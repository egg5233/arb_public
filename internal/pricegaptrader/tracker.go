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
	monMu    sync.Mutex
	monitors map[string]context.CancelFunc

	// Circuit breaker — consecutive PlaceOrder failures across any leg (D-10).
	failMu   sync.Mutex
	failures int
	paused   bool

	// Entry serialization — per-symbol locks to avoid double-fire across ticks.
	entryMu       sync.Mutex
	entryInFlight map[string]bool
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
		bars:          make(map[string]*candidateBars),
		monitors:      make(map[string]context.CancelFunc),
		entryInFlight: make(map[string]bool),
	}
}

// Start spawns the tick goroutine + rehydrates active positions.
// Wired from cmd/main.go conditional on cfg.PriceGapEnabled (Plan 07).
func (t *Tracker) Start() {
	t.log.Info("Price-gap tracker starting (candidates=%d, budget=$%.0f, enabled=%v)",
		len(t.cfg.PriceGapCandidates), t.cfg.PriceGapBudget, t.cfg.PriceGapEnabled)
	t.wg.Add(1)
	go t.tickLoop()
	// rehydration + per-position monitor spawn: implemented in Plan 06/07
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

// tickLoop is a stub in this plan — Plan 05 wires the detector→risk→entry dispatch.
// The stub drains stopCh so Start/Stop shutdown is testable.
func (t *Tracker) tickLoop() {
	defer t.wg.Done()
	<-t.stopCh
}

// candidateBars placeholder — the real definition (with barRing + lastSampleAt)
// lives in detector.go (Task 2). Kept here as a forward declaration so the
// tracker skeleton in Task 1 compiles standalone; Task 2 replaces this with
// the detector.go definition.
type candidateBars struct{}
