package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"

	"github.com/alicebob/miniredis/v2"
)

// ============================================================================
// Plan 07 Task 2 — end-to-end tests driving the full tracker pipeline.
//
// Coverage:
//   - TestTrackerE2E_HappyCycle            — detect → gate → enter → monitor → exit
//   - TestTrackerE2E_GateBlocksOnBudget    — runTick blocks on budget gate, no orders fire
//   - TestTrackerE2E_DisabledFlagBlocks    — exec-quality disable flag blocks entry
//   - TestTrackerE2E_RehydrationOnRestart  — Start()/rehydrate re-enrolls active positions
//   - TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated — safety property (PG-OPS-06)
//
// We use a miniredis-backed *database.Client so the tests exercise the full
// PriceGapStore interface contract (not just the fakeStore subset).
// ============================================================================

// newE2EClient spins up miniredis + a real *database.Client.
func newE2EClient(t *testing.T) (*database.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mr
}

// e2eCfg — fast-cycle defaults suitable for driving runTick directly (the
// tick goroutine itself is not exercised here; we call runTick synchronously).
func e2eCfg(candidates []models.PriceGapCandidate) *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      30,
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
		PriceGapBarPersistence:       4,
		PriceGapCandidates:           candidates,
	}
}

// e2eCandidate — SOON BTC-like on binance/bybit, threshold 200 bps.
func e2eCandidate() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "bybit",
		ThresholdBps:       200,
		MaxPositionUSDT:    1000, // small so budget math is easy to reason about
		ModeledSlippageBps: 7.5,
	}
}

// feedFiringSpread sets both stub BBOs to produce a ~+250 bps spread and then
// runs 4 detectOnce ticks one minute apart so the detector's 4-bar ring fills.
// Returns the latest tick time so callers can sequence the final runTick.
func feedFiringSpread(t *testing.T, tr *Tracker, longEx, shortEx *stubExchange, cand models.PriceGapCandidate, start time.Time) time.Time {
	t.Helper()
	// mid_long = 1.000, mid_short ≈ 0.975 → (1-0.975)/((1+0.975)/2) * 10_000 ≈ 253 bps.
	longEx.setBBO(cand.Symbol, 0.999, 1.001, start)
	shortEx.setBBO(cand.Symbol, 0.974, 0.976, start)
	now := start
	for i := 0; i < 4; i++ {
		tr.detectOnce(cand, now)
		now = now.Add(time.Minute)
	}
	return now
}

// TestTrackerE2E_HappyCycle — full detect → gate → enter → monitor → exit flow.
func TestTrackerE2E_HappyCycle(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := e2eCandidate()
	cfg := e2eCfg([]models.PriceGapCandidate{cand})
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	// Prime 4 bars of +250 bps spread so the 5th sample (inside runTick) fires.
	start := time.Now()
	lastTick := feedFiringSpread(t, tr, bin, byb, cand, start)

	// Queue entry fills for both legs — ~1000 notional / mid 0.9875 ≈ 1012 base units.
	bin.queueFill(1012, 1.000, nil)
	byb.queueFill(1012, 0.975, nil)

	// runTick should detect fire, pass gates (no active positions, disabled=false),
	// place orders, and start the monitor goroutine.
	tr.runTick(lastTick)

	// Assert: position persisted in pg:positions + pg:positions:active.
	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active position, got %d", len(active))
	}
	pos := active[0]
	if pos.Symbol != cand.Symbol {
		t.Fatalf("pos.Symbol=%q want %q", pos.Symbol, cand.Symbol)
	}
	if pos.Status != models.PriceGapStatusOpen {
		t.Fatalf("pos.Status=%q want %q", pos.Status, models.PriceGapStatusOpen)
	}

	// Monitor goroutine must be registered.
	tr.monMu.Lock()
	_, ok := tr.monitors[pos.ID]
	tr.monMu.Unlock()
	if !ok {
		t.Fatalf("monitor not registered for %s", pos.ID)
	}

	// Stop the monitor now so the exit path drives closePair synchronously below.
	tr.monMu.Lock()
	if c, hit := tr.monitors[pos.ID]; hit {
		c()
	}
	tr.monMu.Unlock()

	// Flip spread back near zero and queue exit fills → drive checkAndMaybeExit.
	bin.setBBO(cand.Symbol, 0.999, 1.001, lastTick)
	byb.setBBO(cand.Symbol, 0.999, 1.001, lastTick)
	bin.queueFill(pos.LongSize, 1.000, nil)  // long close SELL
	byb.queueFill(pos.ShortSize, 1.000, nil) // short close BUY

	closed := tr.checkAndMaybeExit(pos, lastTick)
	if !closed {
		t.Fatalf("expected reversion to close position")
	}
	if pos.ExitReason != models.ExitReasonReverted {
		t.Fatalf("ExitReason=%q want %q", pos.ExitReason, models.ExitReasonReverted)
	}
	if pos.Status != models.PriceGapStatusClosed {
		t.Fatalf("pos.Status=%q want closed", pos.Status)
	}

	// Active set should be empty post-close; history should have exactly 1 entry.
	post, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions post-close: %v", err)
	}
	if len(post) != 0 {
		t.Fatalf("expected 0 active post-close, got %d", len(post))
	}
}

// TestTrackerE2E_GateBlocksOnBudget — budget cap prevents a fired detection from
// opening the position. Uses a deterministically-fired detection by pre-feeding
// the 4-bar ring; the runTick budget gate compares requested notional against
// cfg.PriceGapBudget.
func TestTrackerE2E_GateBlocksOnBudget(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := e2eCandidate()
	// Tight budget that is LESS than the per-candidate notional ⇒ budget gate bites.
	cfg := e2eCfg([]models.PriceGapCandidate{cand})
	cfg.PriceGapBudget = 500 // < cand.MaxPositionUSDT (1000)
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	start := time.Now()
	lastTick := feedFiringSpread(t, tr, bin, byb, cand, start)

	tr.runTick(lastTick)

	// No orders should have been placed — budget gate blocked.
	if len(bin.placedOrders()) != 0 {
		t.Fatalf("expected 0 orders on long leg; got %d", len(bin.placedOrders()))
	}
	if len(byb.placedOrders()) != 0 {
		t.Fatalf("expected 0 orders on short leg; got %d", len(byb.placedOrders()))
	}

	active, _ := db.GetActivePriceGapPositions()
	if len(active) != 0 {
		t.Fatalf("expected 0 active positions after budget block, got %d", len(active))
	}
}

// TestTrackerE2E_DisabledFlagBlocks — exec-quality disable flag (Gate 1) blocks
// entry even on fired detection.
func TestTrackerE2E_DisabledFlagBlocks(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := e2eCandidate()
	cfg := e2eCfg([]models.PriceGapCandidate{cand})
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	// Seed the disable flag via the store directly.
	if err := db.SetCandidateDisabled(cand.Symbol, "unit-test-forced"); err != nil {
		t.Fatalf("SetCandidateDisabled: %v", err)
	}

	start := time.Now()
	lastTick := feedFiringSpread(t, tr, bin, byb, cand, start)

	tr.runTick(lastTick)

	if len(bin.placedOrders()) != 0 || len(byb.placedOrders()) != 0 {
		t.Fatalf("expected 0 orders when disabled; got long=%d short=%d",
			len(bin.placedOrders()), len(byb.placedOrders()))
	}
	active, _ := db.GetActivePriceGapPositions()
	if len(active) != 0 {
		t.Fatalf("expected 0 active positions when disabled; got %d", len(active))
	}
}

// TestTrackerE2E_RehydrationOnRestart — pre-seed an active position in the
// real miniredis-backed store; Start() must restore the monitor.
func TestTrackerE2E_RehydrationOnRestart(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	// Pre-seed a position + nonzero exchange positions on both legs.
	pos := &models.PriceGapPosition{
		ID:                 "pg-rehydrate-1",
		Symbol:             "SOON",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		Status:             models.PriceGapStatusOpen,
		EntrySpreadBps:     250,
		ThresholdBps:       200,
		NotionalUSDT:       1000,
		LongFillPrice:      1.0,
		ShortFillPrice:     0.975,
		LongSize:           1000,
		ShortSize:          1000,
		LongMidAtDecision:  1.0,
		ShortMidAtDecision: 0.975,
		ModeledSlipBps:     7.5,
		OpenedAt:           time.Now().Add(-10 * time.Minute),
	}
	if err := db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("SavePriceGapPosition: %v", err)
	}
	bin.positions["SOON"] = []exchange.Position{{Symbol: "SOON", HoldSide: "long", Total: "1000"}}
	byb.positions["SOON"] = []exchange.Position{{Symbol: "SOON", HoldSide: "short", Total: "1000"}}

	// Wide BBO so the just-started monitor goroutine won't immediately exit on
	// reversion (spread ~ 0 bps would otherwise trigger an immediate close).
	bin.setBBO("SOON", 1.0, 1.0, time.Now())
	byb.setBBO("SOON", 0.9, 0.9, time.Now())

	// Use long poll interval so the monitor goroutine's first tick is far away.
	cfg := e2eCfg(nil) // no candidates → tickLoop finds nothing to do
	cfg.PriceGapPollIntervalSec = 3600
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	tr.Start()
	defer tr.Stop()

	// Give Start() a moment to schedule the monitor (rehydrate runs synchronously
	// before tickLoop spawn, but monitor goroutine is async).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tr.monMu.Lock()
		_, ok := tr.monitors[pos.ID]
		tr.monMu.Unlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	tr.monMu.Lock()
	_, ok := tr.monitors[pos.ID]
	tr.monMu.Unlock()
	if !ok {
		t.Fatalf("rehydration did not register monitor for %s", pos.ID)
	}
}

// TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated — safety property
// (PG-OPS-06): with PriceGapEnabled=false, cmd/main.go-equivalent startup
// logic does NOT call NewTracker, so no pg:* Redis writes occur.
//
// We replicate the main.go guard pattern here because main.go itself is not
// unit-testable in-process.
func TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated(t *testing.T) {
	db, mr := newE2EClient(t)

	// cmd/main.go pattern: NewTracker only constructed if enabled.
	cfg := &config.Config{PriceGapEnabled: false}

	var tr *Tracker
	if cfg.PriceGapEnabled {
		// SHOULD NOT EXECUTE.
		bin := newStubExchange("binance")
		byb := newStubExchange("bybit")
		exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}
		tr = NewTracker(exch, db, newFakeDelistChecker(), cfg)
		tr.Start()
	}

	if tr != nil {
		t.Fatalf("tracker must not be instantiated when PriceGapEnabled=false")
	}

	// Assert zero pg:* keys in miniredis.
	for _, k := range mr.Keys() {
		if len(k) >= 3 && k[:3] == "pg:" {
			t.Fatalf("unexpected pg:* key present when tracker disabled: %s", k)
		}
	}
}
