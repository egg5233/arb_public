// Package pricegaptrader — breaker_controller.go is Phase 15 Plan 15-03's
// BreakerController state machine for the drawdown circuit breaker
// (PG-LIVE-02).
//
// Two-strike state machine driven by a 5-min ticker (D-01). On each eval tick
// the controller reads BreakerState from Redis, fetches rolling-24h realized
// PnL via *RealizedPnLAggregator (Plan 15-02), and either:
//   - clears a pending strike on PnL recovery (D-08), or
//   - persists Strike-1 on first breach (Pitfall 2 — kill-9 survives), or
//   - trips on Strike-2 when ≥5 min elapsed since Strike-1.
//
// The trip path follows the D-15 atomicity ordering:
//
//	STEP 1 (LOAD-BEARING): persist PaperModeStickyUntil = math.MaxInt64 to
//	        pg:breaker:state. Any failure here aborts the trip — caller
//	        treats as critical. Steps 2-5 are SKIPPED on Step 1 failure.
//	STEP 2: pause every candidate with an active position (chokepoint
//	        Registry.PauseAllOpenCandidates). Failure logged; trip continues.
//	        (Run BEFORE Step 3 so the trip record carries paused_count.)
//	STEP 3: append BreakerTripRecord to pg:breaker:trips. Failure logged;
//	        trip continues.
//	STEP 4: dispatch Telegram critical alert. Failure logged; trip continues.
//	STEP 5: WS broadcast. Failure-impossible (no return on Broadcast).
//
// The ordering is the load-bearing safety property: any partial failure
// beyond Step 1 leaves the engine in paper mode (the safest state) because
// the sticky flag is the engine's authoritative paper-mode source per
// Plan 15-02 IsPaperModeActive chokepoint.
//
// Module boundary (D-15): this file imports only context, fmt, math, sync,
// time, arb/internal/config, arb/internal/models, arb/pkg/utils. No
// internal/api, no internal/database, no internal/notify, no internal/engine,
// no internal/spotengine.
package pricegaptrader

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// BreakerStateStore — narrow interface for BreakerController persistence
// (D-15 boundary). *database.Client satisfies via methods added in Plan 15-01.
// Subsumes BreakerStateLoader from tracker.go for the read path.
type BreakerStateStore interface {
	LoadBreakerState() (models.BreakerState, bool, error)
	SaveBreakerState(state models.BreakerState) error
	AppendBreakerTrip(record models.BreakerTripRecord) error
}

// BreakerNotifier — narrow interface for the trip Telegram dispatch.
// Subset of PriceGapNotifier; *notify.TelegramNotifier and NoopNotifier
// satisfy via the Phase 15 Plan 15-03 Task 1 method addition.
type BreakerNotifier interface {
	NotifyPriceGapBreakerTrip(record models.BreakerTripRecord) error
}

// BreakerWSBroadcaster — narrow interface for the WS broadcast (D-15 step 5).
// *api.Hub satisfies in production via duck typing; tests pass nil or a fake.
type BreakerWSBroadcaster interface {
	BroadcastPriceGapBreakerEvent(eventType string, payload any)
}

// CandidatePauser — narrow interface for the trip-path candidate-pause step
// (D-15 step 2). *Registry satisfies via PauseAllOpenCandidates (Plan 15-03
// Task 1).
type CandidatePauser interface {
	PauseAllOpenCandidates(positions []*models.PriceGapPosition) (int, error)
}

// ActivePositionLister — narrow interface for the trip-path active-positions
// query. *database.Client satisfies via GetActivePriceGapPositions
// (Phase 8 / Phase 14 D-02). Returns full position objects so the trip path
// can extract (symbol, longExch, shortExch) tuples for candidate matching.
type ActivePositionLister interface {
	GetActivePriceGapPositions() ([]*models.PriceGapPosition, error)
}

// breakerAggregator — narrow interface for the rolling-24h PnL primitive.
// *RealizedPnLAggregator (Plan 15-02) satisfies. Tests inject a fake.
type breakerAggregator interface {
	Realized24h(ctx context.Context, now time.Time) (float64, error)
}

// BreakerController is the drawdown circuit-breaker state machine. Daemon
// goroutine (Run) is spawned by Tracker.Start when PriceGapBreakerEnabled=true
// (Plan 15-04 wires production). All mutation operations are serialized by
// the eval tick — single goroutine consumes from time.Ticker, no concurrent
// trip path possible.
type BreakerController struct {
	cfg        *config.Config
	store      BreakerStateStore
	aggregator breakerAggregator
	notifier   BreakerNotifier

	// Best-effort side-effect dependencies (D-15 steps 2/3/5). Nil-safe;
	// missing wiring degrades the trip cosmetically (no candidate pause /
	// no record / no WS) but does NOT block Step 1 sticky persistence.
	ws        BreakerWSBroadcaster
	ramp      RampSnapshotter
	registry  CandidatePauser
	positions ActivePositionLister

	log *utils.Logger
	mu  sync.RWMutex
}

// NewBreakerController constructs a BreakerController. Required deps: cfg,
// store, aggregator, notifier, log. Best-effort deps (ws, ramp, registry,
// positions) are wired post-construction via setters so tests + production
// bootstrap order can keep the constructor stable.
func NewBreakerController(
	cfg *config.Config,
	store BreakerStateStore,
	aggregator breakerAggregator,
	notifier BreakerNotifier,
	log *utils.Logger,
) *BreakerController {
	if log == nil {
		log = utils.NewLogger("pg-breaker")
	}
	return &BreakerController{
		cfg:        cfg,
		store:      store,
		aggregator: aggregator,
		notifier:   notifier,
		log:        log,
	}
}

// SetWSBroadcaster wires the WS hub. nil-safe; the trip path elides Step 5
// when the broadcaster is nil. Plan 15-04 wires *api.Hub from cmd/main.go.
func (bc *BreakerController) SetWSBroadcaster(ws BreakerWSBroadcaster) { bc.ws = ws }

// SetRamp wires the ramp controller's Snapshot reader. nil-safe; the trip
// record records ramp_stage=0 when the ramp is not wired. Plan 15-04 wires
// *RampController.
func (bc *BreakerController) SetRamp(r RampSnapshotter) { bc.ramp = r }

// SetRegistry wires the chokepoint-write registry. nil-safe; the trip path
// elides Step 2 (candidate pause) when nil. Plan 15-04 wires *Registry.
func (bc *BreakerController) SetRegistry(p CandidatePauser) { bc.registry = p }

// SetPositions wires the active-positions reader. nil-safe; the trip path
// elides Step 2 when nil (no positions to enumerate). Plan 15-04 wires
// *database.Client.
func (bc *BreakerController) SetPositions(p ActivePositionLister) { bc.positions = p }

// Run is the daemon loop. Spawned by Tracker.Start when
// PriceGapBreakerEnabled=true. Mirrors RampController.Run shape exactly.
//
// On disabled cfg the loop is a no-op (skips immediately). The interval has a
// safety floor of 60s — any cfg < 60 silently uses 300s (matches Phase 14
// "default-then-override" pattern). Graceful shutdown on ctx.Done().
func (bc *BreakerController) Run(ctx context.Context) {
	if !bc.cfg.PriceGapBreakerEnabled {
		bc.log.Info("breaker: PriceGapBreakerEnabled=false, daemon skipped")
		return
	}
	interval := bc.cfg.PriceGapBreakerIntervalSec
	if interval < 60 {
		interval = 300 // safety floor
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	bc.log.Info("breaker: daemon started, interval=%ds, threshold=%.2f USDT",
		interval, bc.cfg.PriceGapDrawdownLimitUSDT)
	for {
		select {
		case <-ctx.Done():
			bc.log.Info("breaker: daemon stopping")
			return
		case t := <-ticker.C:
			if err := bc.evalTick(ctx, t); err != nil {
				bc.log.Warn("breaker eval tick: %v", err)
			}
		}
	}
}

// evalTick runs one eval cycle. Implements CONTEXT D-01 through D-08:
//
//	D-01: 5-min cadence (caller sets the ticker; tick is single-shot here)
//	D-03: whole-tick blackout suppression — no PnL fetch, no state mutation
//	      while wall-clock minute is in [:04, :05:30). REUSES inBybitBlackout
//	      from scanner.go (Pitfall 4 — single source of truth).
//	D-04: pending Strike-1 survives a blackout-suppressed tick.
//	D-05: state persisted to pg:breaker:state HASH on every state change.
//	D-08: PnL recovery (>= threshold) clears pending Strike-1.
//	Boot-guard (Discretion): missing state → fresh init (permissive).
//	Defensive ≥5min: even when a confirming breach arrives, the elapsed
//	      check ensures two ticks were >= 5 min apart (matches D-01 cadence).
func (bc *BreakerController) evalTick(ctx context.Context, now time.Time) error {
	if !bc.cfg.PriceGapBreakerEnabled {
		return nil // safety re-check (covers race during cfg reload)
	}
	// D-03: whole-tick blackout suppression — REUSES inBybitBlackout
	// from scanner.go (Pitfall 4: single source of truth).
	if inBybitBlackout(now) {
		return nil
	}

	state, exists, err := bc.store.LoadBreakerState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	// Boot guard (Discretion): missing state → fresh init (permissive,
	// unlike Phase 14 ramp). A fresh breaker is armed-but-not-tripped — a
	// safe cold-start.
	if !exists {
		state = models.BreakerState{}
	}
	// Already tripped (sticky flag non-zero) — nothing to evaluate.
	// Operator must run recovery to clear.
	if state.PaperModeStickyUntil != 0 {
		return nil
	}

	pnl, err := bc.aggregator.Realized24h(ctx, now)
	if err != nil {
		return fmt.Errorf("aggregate: %w", err)
	}

	state.LastEvalTs = now.UnixMilli()
	state.LastEvalPnLUSDT = pnl

	threshold := bc.cfg.PriceGapDrawdownLimitUSDT
	inBreach := pnl < threshold

	if !inBreach {
		// D-08: clear pending strike on recovery.
		if state.PendingStrike == 1 {
			bc.log.Info("breaker: PnL recovered (24h=%.2f >= threshold=%.2f), strike cleared",
				pnl, threshold)
			state.PendingStrike = 0
			state.Strike1Ts = 0
		}
		return bc.store.SaveBreakerState(state)
	}

	// In breach.
	if state.PendingStrike == 0 {
		// First strike. Pitfall 2: persist immediately so kill -9 cannot lose it.
		state.PendingStrike = 1
		state.Strike1Ts = now.UnixMilli()
		bc.log.Warn("breaker: STRIKE 1 (24h=%.2f < threshold=%.2f), awaiting confirmation tick ≥5min away",
			pnl, threshold)
		return bc.store.SaveBreakerState(state)
	}

	// Pending strike + breach. Defensive ≥5min check covers the case
	// where eval is invoked manually or with a sub-cadence interval (or
	// while the cadence is being changed). The 5-min spacing is the
	// load-bearing two-strike rule — this guard is independent of the
	// ticker interval.
	elapsed := now.UnixMilli() - state.Strike1Ts
	if elapsed < 5*60*1000 {
		bc.log.Info("breaker: pending strike + breach (24h=%.2f), but elapsed %dms < 5min — defensive skip",
			pnl, elapsed)
		return bc.store.SaveBreakerState(state)
	}

	// Strike 2 — TRIP per D-15 ordering.
	return bc.trip(ctx, state, pnl, "live")
}

// trip executes the D-15 trip ordering. Step 1 is the load-bearing safety
// property: any partial failure beyond Step 1 leaves the engine in paper
// mode (the safest state) because the sticky flag is observed by the
// IsPaperModeActive chokepoint (Plan 15-02).
//
// Reordered vs. CONTEXT D-15 prose: Step 2 (candidate pause) runs BEFORE
// Step 3 (append trip record) so the record can carry paused_candidate_count.
// CONTEXT D-15 ordering allowed this swap — it preserves the safety
// invariant (sticky persisted first) while improving observability.
func (bc *BreakerController) trip(ctx context.Context, state models.BreakerState, pnl24h float64, source string) error {
	_ = ctx // reserved for future ctx-aware Redis path

	// Snapshot ramp stage (best-effort; defaults to 0 if not wired).
	rampStage := 0
	if bc.ramp != nil {
		rampStage = bc.ramp.Snapshot().CurrentStage
	}

	// === STEP 1: Sticky flag persisted FIRST. Load-bearing safety property (D-15). ===
	// Any partial failure beyond this point leaves engine in safest state (forced paper mode).
	state.PendingStrike = 0
	state.Strike1Ts = 0
	state.PaperModeStickyUntil = math.MaxInt64
	if err := bc.store.SaveBreakerState(state); err != nil {
		// Step 1 failure is critical — abort. Do NOT proceed to steps 2-5.
		bc.log.Error("breaker: TRIP FAILED at step 1 (sticky flag NOT persisted): %v", err)
		return fmt.Errorf("trip step 1 (save sticky): %w", err)
	}
	bc.log.Warn("breaker: TRIPPED — paper_mode_sticky_until=MaxInt64 persisted (24h=%.2f, threshold=%.2f, source=%s)",
		pnl24h, bc.cfg.PriceGapDrawdownLimitUSDT, source)

	// === Steps 2-5: best-effort. Failures logged but do NOT undo sticky. ===
	record := models.BreakerTripRecord{
		TripTs:      time.Now().UnixMilli(),
		TripPnLUSDT: pnl24h,
		Threshold:   bc.cfg.PriceGapDrawdownLimitUSDT,
		RampStage:   rampStage,
		Source:      source,
	}

	// STEP 2: pause all candidates with active positions (run before Step 3
	// so we can include the count in the trip record).
	if bc.registry != nil && bc.positions != nil {
		active, posErr := bc.positions.GetActivePriceGapPositions()
		if posErr != nil {
			bc.log.Warn("breaker: TRIP step 2 GetActivePriceGapPositions: %v", posErr)
		} else {
			count, pauseErr := bc.registry.PauseAllOpenCandidates(active)
			if pauseErr != nil {
				bc.log.Warn("breaker: TRIP step 2 PauseAllOpenCandidates: %v", pauseErr)
			} else {
				record.PausedCandidateCount = count
			}
		}
	}

	// STEP 3: append trip record to capped LIST.
	if err := bc.store.AppendBreakerTrip(record); err != nil {
		bc.log.Warn("breaker: TRIP step 3 AppendBreakerTrip: %v", err)
	}

	// STEP 4: Telegram critical alert.
	if bc.notifier != nil {
		if err := bc.notifier.NotifyPriceGapBreakerTrip(record); err != nil {
			bc.log.Warn("breaker: TRIP step 4 NotifyPriceGapBreakerTrip: %v", err)
		}
	}

	// STEP 5: WS broadcast.
	if bc.ws != nil {
		bc.ws.BroadcastPriceGapBreakerEvent("pg.breaker.trip", record)
	}

	return nil
}

// Snapshot returns the current BreakerState (for dashboard read endpoint;
// Plan 15-04 wires it via API). Read-only — does NOT mutate state.
func (bc *BreakerController) Snapshot() (models.BreakerState, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	state, exists, err := bc.store.LoadBreakerState()
	if err != nil {
		return models.BreakerState{}, err
	}
	if !exists {
		return models.BreakerState{}, nil
	}
	return state, nil
}
