package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

func rehydrateTestCfg() *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      30, // long so monitor goroutine won't fire during test
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
	}
}

func newRehydrateTracker(t *testing.T, store *fakeStore) (*Tracker, *stubExchange, *stubExchange) {
	t.Helper()
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}
	return NewTracker(exch, store, newFakeDelistChecker(), rehydrateTestCfg()), bin, byb
}

func rehydratePos(id, symbol string) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:             id,
		Symbol:         symbol,
		LongExchange:   "binance",
		ShortExchange:  "bybit",
		Status:         models.PriceGapStatusOpen,
		ThresholdBps:   200,
		LongSize:       100,
		ShortSize:      100,
		OpenedAt:       time.Now().Add(-10 * time.Minute),
	}
}

// TestRehydrate_ReEnrollsActive: 2 active positions with nonzero legs on stub
// → 2 monitor goroutines registered in tracker.monitors. Active set preserved.
func TestRehydrate_ReEnrollsActive(t *testing.T) {
	store := newFakeStore()
	store.active = []*models.PriceGapPosition{
		rehydratePos("pg-r-1", "AAA"),
		rehydratePos("pg-r-2", "BBB"),
	}
	tr, bin, byb := newRehydrateTracker(t, store)
	bin.positions["AAA"] = []exchange.Position{{Symbol: "AAA", HoldSide: "long", Total: "100"}}
	byb.positions["AAA"] = []exchange.Position{{Symbol: "AAA", HoldSide: "short", Total: "100"}}
	bin.positions["BBB"] = []exchange.Position{{Symbol: "BBB", HoldSide: "long", Total: "100"}}
	byb.positions["BBB"] = []exchange.Position{{Symbol: "BBB", HoldSide: "short", Total: "100"}}

	tr.rehydrate()

	tr.monMu.Lock()
	defer tr.monMu.Unlock()
	if _, ok := tr.monitors["pg-r-1"]; !ok {
		t.Fatalf("pg-r-1 monitor not registered after rehydrate")
	}
	if _, ok := tr.monitors["pg-r-2"]; !ok {
		t.Fatalf("pg-r-2 monitor not registered after rehydrate")
	}
	if len(tr.monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(tr.monitors))
	}
	if len(store.history) != 0 {
		t.Fatalf("no positions should be orphaned: history len=%d", len(store.history))
	}
	// Clean up spawned goroutines.
	for _, c := range tr.monitors {
		c()
	}
}

// TestRehydrate_OrphansZeroSizeLeg: 1 position where exchange shows zero-size
// on the long leg → position is stamped ExitReasonOrphan, moved to history,
// removed from active set, no monitor spawned.
func TestRehydrate_OrphansZeroSizeLeg(t *testing.T) {
	store := newFakeStore()
	store.active = []*models.PriceGapPosition{
		rehydratePos("pg-orphan-1", "CCC"),
	}
	tr, bin, byb := newRehydrateTracker(t, store)
	// Binance has NO position for CCC (empty slice == zero-size).
	bin.positions["CCC"] = nil
	byb.positions["CCC"] = []exchange.Position{{Symbol: "CCC", HoldSide: "short", Total: "100"}}

	tr.rehydrate()

	tr.monMu.Lock()
	monitorCount := len(tr.monitors)
	tr.monMu.Unlock()
	if monitorCount != 0 {
		t.Fatalf("orphaned position must NOT spawn monitor; got %d", monitorCount)
	}
	if len(store.history) != 1 {
		t.Fatalf("expected 1 history entry (orphan), got %d", len(store.history))
	}
	if store.history[0].ExitReason != models.ExitReasonOrphan {
		t.Fatalf("ExitReason=%q want %q", store.history[0].ExitReason, models.ExitReasonOrphan)
	}
	if store.history[0].Status != models.PriceGapStatusClosed {
		t.Fatalf("Status=%q want closed", store.history[0].Status)
	}
	if len(store.removedActive) != 1 || store.removedActive[0] != "pg-orphan-1" {
		t.Fatalf("expected RemoveActivePriceGapPosition('pg-orphan-1'); got %v", store.removedActive)
	}
}

// TestRehydrate_IdempotentOnSecondCall: calling rehydrate twice still leaves
// one monitor per position (the second call cancels+replaces the first).
func TestRehydrate_IdempotentOnSecondCall(t *testing.T) {
	store := newFakeStore()
	store.active = []*models.PriceGapPosition{
		rehydratePos("pg-idemp-1", "DDD"),
	}
	tr, bin, byb := newRehydrateTracker(t, store)
	bin.positions["DDD"] = []exchange.Position{{Symbol: "DDD", HoldSide: "long", Total: "100"}}
	byb.positions["DDD"] = []exchange.Position{{Symbol: "DDD", HoldSide: "short", Total: "100"}}

	tr.rehydrate()
	tr.rehydrate()

	tr.monMu.Lock()
	defer tr.monMu.Unlock()
	if len(tr.monitors) != 1 {
		t.Fatalf("expected 1 monitor after 2 rehydrates, got %d", len(tr.monitors))
	}
	for _, c := range tr.monitors {
		c()
	}
}
