// Package pricegaptrader — Strategy 4 (cross-exchange price-gap arbitrage) MVP tracker.
//
// Module boundary (per CONTEXT §D-02): this package MUST NOT import
// internal/engine/ or internal/spotengine/. Allowed: pkg/exchange,
// pkg/utils, internal/models, internal/config, internal/database.
//
// Data path (Option A per RESEARCH §Summary): in-memory 1m OHLC bars built
// from BBO samples at a 30s tick cadence (2 samples per bar). Option B
// (adding GetKlines to the 35-method Exchange interface) was REJECTED because
// it requires 4-adapter REST work for a signal-to-measurement ratio that is
// fine with BBO sampling at T=200 bps events. Rationale: RESEARCH §Summary,
// CONTEXT §D-02 "minimal surface", §D-07 bar-history reset on restart.
//
// Spread formula (D-06): (mid_long - mid_short) / mid_avg × 10_000 bps, where
// mid_avg = (mid_long + mid_short) / 2 and each mid is (bid+ask)/2 of the
// corresponding exchange's BBO.
//
// Freshness gate: BBO from the WebSocket price stream is considered fresh
// when GetBBO returns ok=true. pkg/exchange.BBO currently carries no timestamp,
// so the detector measures staleness as the wall-clock interval between ticks
// for a given candidate; if that interval exceeds cfg.PriceGapKlineStalenessSec
// then the sample is rejected as stale. See detector.go for the gate logic.
package pricegaptrader

import (
	"context"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// Tracker orchestrates BBO sampling → detection → (later plans) risk → entry → monitor → exit.
// In Plan 03 only the skeleton + detector state live here; tickLoop is a stub that
// drains stopCh. Plans 04/05/06 wire dispatch, risk, entry, exit.
type Tracker struct {
	exchanges map[string]exchange.Exchange
	db        models.PriceGapStore
	delist    models.DelistChecker
	cfg       *config.Config
	log       *utils.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
	exitWG sync.WaitGroup

	// In-memory bar state (D-07: resets on restart; key = candidate.ID()).
	barsMu sync.RWMutex
	bars   map[string]*candidateBars

	// Active position monitor registry — posID -> cancel func for exit goroutine.
	// `monitors` is the legacy Stop()-path keyed by posID. `monitorHandles` adds
	// a seq token so the exit-cleanup defer in monitorPosition can detect
	// "my entry was replaced by a newer startMonitor" and skip delete.
	monMu           sync.Mutex
	monitors        map[string]context.CancelFunc
	monitorHandles  map[string]monitorHandle

	// Circuit breaker — consecutive PlaceOrder failures across any leg (D-10).
	failMu   sync.Mutex
	failures int
	paused   bool

	// Entry serialization — per-symbol locks to avoid double-fire across ticks.
	entryMu       sync.Mutex
	entryInFlight map[string]bool

	// Broadcaster DI seam (Phase 9, Plan 06 wires the WS hub implementation).
	// Defaults to NoopBroadcaster{} in NewTracker so callers without a hub
	// (tests, CLI) never risk nil deref.
	broadcaster Broadcaster

	// PriceGapNotifier DI seam (Phase 9 Plan 06) — Telegram alerts on
	// entry / exit / risk-block / auto-disable. Defaults to NoopNotifier{}
	// so callers without Telegram configured never risk nil deref.
	notifier PriceGapNotifier

	// Heartbeat throttle (Plan 06 D-06 / Pitfall 3): BroadcastPriceGapPositions
	// is rate-limited to at most one call per `priceGapPositionsMinInterval`
	// (2s). Called (a) from tickLoop every ~2s, and (b) right after every
	// entry / exit hook. The mutex guards lastPositionsBroadcast ONLY — the
	// broadcast itself happens outside the critical section.
	positionsBroadcastMu   sync.Mutex
	lastPositionsBroadcast time.Time
}

// NewTracker constructs a Tracker with injected dependencies. The store and
// delist-checker are interfaces to preserve the D-02 module boundary —
// *database.Client and *discovery.Scanner satisfy them respectively.
func NewTracker(
	exchanges map[string]exchange.Exchange,
	db models.PriceGapStore,
	delist models.DelistChecker,
	cfg *config.Config,
) *Tracker {
	return &Tracker{
		exchanges:     exchanges,
		db:            db,
		delist:        delist,
		cfg:           cfg,
		log:           utils.NewLogger("pg-tracker"),
		stopCh:        make(chan struct{}),
		bars:           make(map[string]*candidateBars),
		monitors:       make(map[string]context.CancelFunc),
		monitorHandles: make(map[string]monitorHandle),
		entryInFlight:  make(map[string]bool),
		broadcaster:    NoopBroadcaster{},
		notifier:       NoopNotifier{},
	}
}

// SetBroadcaster swaps the broadcaster (Phase 9 wiring; the main binary calls
// this from cmd/main.go after api.Server is constructed). Passing nil reverts
// to NoopBroadcaster so downstream code can always call methods safely.
func (t *Tracker) SetBroadcaster(b Broadcaster) {
	if b == nil {
		t.broadcaster = NoopBroadcaster{}
		return
	}
	t.broadcaster = b
}

// SetNotifier swaps the Telegram notifier (Phase 9 Plan 06 wiring; cmd/main.go
// calls this after notify.NewTelegram). Passing nil reverts to NoopNotifier so
// downstream hooks can always call methods safely even when Telegram is
// unconfigured (no bot token / no chat id).
func (t *Tracker) SetNotifier(n PriceGapNotifier) {
	if n == nil {
		t.notifier = NoopNotifier{}
		return
	}
	t.notifier = n
}

// priceGapPositionsMinInterval is the minimum gap between consecutive
// BroadcastPriceGapPositions calls (T-09-26 throttle). Per-event
// BroadcastPriceGapEvent is unthrottled — event rate is bounded by trading
// cadence, not noise.
const priceGapPositionsMinInterval = 2 * time.Second

// maybeBroadcastPositions emits the full active-positions list to the
// dashboard WS hub, but at most once per priceGapPositionsMinInterval.
//
// Called from three sites:
//  1. After every openPair hook (so the UI sees a new entry immediately).
//  2. After every closePair hook (so the UI sees the exit immediately).
//  3. From the heartbeat ticker in tickLoop (so the UI stays in sync even
//     with zero trading activity).
//
// Throttle rationale (T-09-26, Pitfall 3): a tick or a storm of exits can
// otherwise fan out identical full-list pushes back-to-back. The per-event
// stream (entry/exit/auto_disable) carries the actual news; pg_positions is
// just the reconciliation snapshot.
func (t *Tracker) maybeBroadcastPositions() {
	t.positionsBroadcastMu.Lock()
	if time.Since(t.lastPositionsBroadcast) < priceGapPositionsMinInterval {
		t.positionsBroadcastMu.Unlock()
		return
	}
	t.lastPositionsBroadcast = time.Now()
	t.positionsBroadcastMu.Unlock()

	active, err := t.db.GetActivePriceGapPositions()
	if err != nil {
		// Non-fatal: log and drop; next heartbeat will retry.
		t.log.Warn("pricegap: maybeBroadcastPositions GetActivePriceGapPositions: %v", err)
		return
	}
	t.broadcaster.BroadcastPriceGapPositions(active)
}

// Start spawns the tick goroutine + rehydrates active positions.
// Wired from cmd/main.go conditional on cfg.PriceGapEnabled (Plan 07).
//
// Order (Plan 07): rehydrate() runs BEFORE the tick goroutine spawns so any
// restored positions are already enrolled in monitors; otherwise the first
// tick could race rehydration and see an empty active set for budget gating.
func (t *Tracker) Start() {
	t.log.Info("Price-gap tracker starting (candidates=%d, budget=$%.0f, enabled=%v)",
		len(t.cfg.PriceGapCandidates), t.cfg.PriceGapBudget, t.cfg.PriceGapEnabled)
	t.rehydrate()
	t.wg.Add(1)
	go t.tickLoop()
}

// Stop signals goroutines to exit and waits; graceful shutdown per D-03.
func (t *Tracker) Stop() {
	t.log.Info("Price-gap tracker stopping")
	close(t.stopCh)
	t.wg.Wait()
	// Cancel any active exit monitors.
	t.monMu.Lock()
	for _, cancel := range t.monitors {
		cancel()
	}
	t.monMu.Unlock()
	t.exitWG.Wait()
}

// tickLoop runs the detect → gate → enter → monitor dispatch at a fixed
// PriceGapPollIntervalSec cadence (D-05). Started by Start() after rehydrate.
//
// Startup offset (RESEARCH §Pitfall 2 — Bybit blackout): the first tick fires
// after ~7s of startup then settles onto the interval. A pure time.NewTicker
// would land the first tick exactly `interval` after boot, which — when
// launched at certain minute boundaries — can push the first BBO-read into
// the Bybit :04–:05:30 blackout window. The 7s offset pushes us safely off
// that boundary on fresh starts (the subsequent ticker cadence is
// unaffected; subsequent ticks are whatever the system clock gives us).
func (t *Tracker) tickLoop() {
	defer t.wg.Done()

	interval := time.Duration(t.cfg.PriceGapPollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	firstTick := time.After(7 * time.Second)
	var ticker *time.Ticker

	// Heartbeat ticker (Plan 06 D-06): pushes pg_positions every 2s so the
	// dashboard stays in sync even when no entries/exits fire. Runs concurrently
	// with the poll-cadence ticker; the throttle in maybeBroadcastPositions
	// guarantees the per-event path and the heartbeat path do not duplicate.
	heartbeat := time.NewTicker(priceGapPositionsMinInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-t.stopCh:
			if ticker != nil {
				ticker.Stop()
			}
			return
		case <-firstTick:
			ticker = time.NewTicker(interval)
			t.runTick(time.Now())
		case now := <-chanTick(ticker):
			t.runTick(now)
		case <-heartbeat.C:
			t.maybeBroadcastPositions()
		}
	}
}

// chanTick is a nil-safe helper so the select in tickLoop can read from a
// ticker that hasn't been created yet (before the first startup tick fires).
// A nil channel blocks forever in select, which is exactly what we want.
func chanTick(tk *time.Ticker) <-chan time.Time {
	if tk == nil {
		return nil
	}
	return tk.C
}

// runTick evaluates every configured candidate for one poll cycle:
//   detectOnce → if fired, preEntry gates → if approved, openPair + startMonitor.
//
// Circuit-breaker respect: when the execution circuit is open (5+ consecutive
// PlaceOrder failures per D-10), we skip the entire tick so no new orders
// fire until the operator clears the breaker. Monitors on existing positions
// still run on their own goroutines.
//
// Per-candidate sizing: notional = cand.MaxPositionUSDT at fire time (D-08
// direction-only; future enhancement could size proportional to spread
// magnitude — out of scope for Phase 8 MVP).
//
// Local active-view update: after openPair succeeds, we append the new
// position to the local `active` slice so the NEXT candidate in the same
// tick sees the up-to-date concurrency count for budget / max-concurrent /
// gate-concentration gating. Without this, back-to-back opens in one tick
// could each see the pre-tick active set and both pass gates that only
// allow one of them.
func (t *Tracker) runTick(now time.Time) {
	if t.isCircuitOpen() {
		return
	}
	active, err := t.db.GetActivePriceGapPositions()
	if err != nil {
		t.log.Warn("pricegap: runTick GetActivePriceGapPositions failed: %v", err)
		return
	}
	for _, cand := range t.cfg.PriceGapCandidates {
		det := t.detectOnce(cand, now)
		if !det.Fired {
			continue
		}

		notional := cand.MaxPositionUSDT

		gate := t.preEntry(cand, notional, det, active)
		if gate.Err != nil {
			t.log.Info("pricegap: %s gate-blocked: %s", cand.Symbol, gate.Reason)
			continue
		}

		// Convert notional USDT → per-leg base-asset size using mid at decision.
		mid := (det.MidLong + det.MidShort) / 2.0
		if mid <= 0 {
			continue
		}
		sizeBase := notional / mid

		pos, err := t.openPair(cand, sizeBase, det)
		if err != nil {
			t.log.Warn("pricegap: openPair %s failed: %v", cand.Symbol, err)
			continue
		}
		if pos != nil {
			t.startMonitor(pos)
			active = append(active, pos)
			t.log.Info("pricegap: opened %s spread=%.1fbps notional=%.0f",
				pos.ID, det.SpreadBps, pos.NotionalUSDT)
		}
	}
}

