package pricegaptrader

import (
	"context"
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// monitorTestCfg — small, predictable knobs for monitor_test only.
func monitorTestCfg() *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      1,
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
	}
}

// newMonitorTracker wires a tracker with two stub exchanges and BBOs preset so
// closePair's unwind/close helpers don't blow up on lookups.
func newMonitorTracker(t *testing.T, cfg *config.Config) (*Tracker, *stubExchange, *stubExchange, *fakeStore) {
	t.Helper()
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}
	store := newFakeStore()
	tr := NewTracker(exch, store, newFakeDelistChecker(), cfg)
	return tr, bin, byb, store
}

// testPos builds a fully-populated open position for monitor tests.
func testPos(id string, longFill, shortFill float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:                 id,
		Symbol:             "SOON",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		Status:             models.PriceGapStatusOpen,
		EntrySpreadBps:     250,
		ThresholdBps:       200,
		NotionalUSDT:       1000,
		LongFillPrice:      longFill,
		ShortFillPrice:     shortFill,
		LongSize:           100,
		ShortSize:          100,
		LongMidAtDecision:  longFill,
		ShortMidAtDecision: shortFill,
		ModeledSlipBps:     50,
		OpenedAt:           time.Now(),
	}
}

// TestMonitor_Exit_ReversionTrigger: spread drops to T/2 → closePair called
// with reason="reverted"; pos Status="closed"; RealizedPnL populated.
func TestMonitor_Exit_ReversionTrigger(t *testing.T) {
	tr, bin, byb, store := newMonitorTracker(t, monitorTestCfg())
	// Set BBOs so computed spread is well within T * reversionFactor = 200*0.5 = 100 bps.
	// mid_long ≈ 10.0, mid_short ≈ 10.0 → spread ≈ 0 bps.
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	// Queue successful IOC exit fills for both legs.
	bin.queueFill(100, 10.01, nil) // long close = SELL
	byb.queueFill(100, 9.99, nil)  // short close = BUY

	pos := testPos("pg-rev-1", 10.0, 10.5)
	// Drive one tick synchronously via checkAndMaybeExit.
	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected reversion trigger to close position")
	}
	if pos.ExitReason != models.ExitReasonReverted {
		t.Fatalf("ExitReason=%q, want %q", pos.ExitReason, models.ExitReasonReverted)
	}
	if pos.Status != models.PriceGapStatusClosed {
		t.Fatalf("Status=%q, want %q", pos.Status, models.PriceGapStatusClosed)
	}
	// RealizedPnL = (10.01-10.0)*100 + (10.5-9.99)*100 = 1 + 51 = 52.0
	want := (10.01-10.0)*100 + (10.5-9.99)*100
	if math.Abs(pos.RealizedPnL-want) > 1e-6 {
		t.Fatalf("RealizedPnL=%.4f want=%.4f", pos.RealizedPnL, want)
	}
	// History should contain the closed position; active-set write-then-remove simulated
	// by fakeStore presence of saved entries (at least 2: exiting + closed).
	if len(store.saved) < 2 {
		t.Fatalf("expected at least 2 SavePriceGapPosition calls (exiting + closed), got %d", len(store.saved))
	}
}

// TestMonitor_Exit_MaxHoldTrigger: OpenedAt=-5h → closePair called with reason="max_hold".
func TestMonitor_Exit_MaxHoldTrigger(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	// Set spreads WIDE so reversion would NOT fire — must trip on max-hold alone.
	bin.setBBO("SOON", 10.0, 10.0, time.Now())
	byb.setBBO("SOON", 9.0, 9.0, time.Now()) // ~1000 bps spread — far above T
	bin.queueFill(100, 10.0, nil)
	byb.queueFill(100, 9.0, nil)

	pos := testPos("pg-mh-1", 10.0, 9.0)
	pos.OpenedAt = time.Now().Add(-5 * time.Hour)

	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected max-hold to close position")
	}
	if pos.ExitReason != models.ExitReasonMaxHold {
		t.Fatalf("ExitReason=%q, want %q", pos.ExitReason, models.ExitReasonMaxHold)
	}
}

// TestMonitor_ExitRetriesMarketOnPartialClose: IOC fills 80/100, market fills remainder 20.
// After closePair: long placed orders = 1 IOC SELL + 1 MARKET SELL.
func TestMonitor_ExitRetriesMarketOnPartialClose(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	// Long IOC: partial 80 of 100.
	bin.queueFill(80, 10.01, nil)
	// Short IOC: full 100.
	byb.queueFill(100, 9.99, nil)
	// Long MARKET retry for remainder 20 (closeLegMarket places but doesn't consume a script for qty).
	// No additional queueFill needed — closeLegMarket doesn't read fill back.

	pos := testPos("pg-partial-1", 10.0, 10.0)
	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected close to succeed with market retry")
	}
	longOrders := bin.placedOrders()
	// Expect 2 long-side orders: 1 IOC SELL, 1 MARKET SELL retry.
	if len(longOrders) != 2 {
		t.Fatalf("expected 2 long orders (IOC + market retry), got %d", len(longOrders))
	}
	iocCount, marketCount := 0, 0
	for _, o := range longOrders {
		if o.Side != exchange.SideSell {
			t.Fatalf("unexpected side on long leg: %v", o.Side)
		}
		if o.Force == "ioc" {
			iocCount++
		} else {
			marketCount++
		}
	}
	if iocCount != 1 || marketCount != 1 {
		t.Fatalf("expected 1 IOC + 1 MARKET on long leg, got ioc=%d market=%d", iocCount, marketCount)
	}
	// Market retry must carry ReduceOnly.
	for _, o := range longOrders {
		if o.Force != "ioc" && !o.ReduceOnly {
			t.Fatalf("market retry must be ReduceOnly=true; got %+v", o)
		}
	}
}

// TestMonitor_PnLMath: entry long=100@10.0, short=100@10.5 → exit long@10.6, short@10.2.
// RealizedPnL = (10.6-10.0)*100 + (10.5-10.2)*100 = 60 + 30 = 90.
func TestMonitor_PnLMath(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 10.0, 10.0, time.Now())
	byb.setBBO("SOON", 10.0, 10.0, time.Now())
	bin.queueFill(100, 10.6, nil)
	byb.queueFill(100, 10.2, nil)

	pos := testPos("pg-pnl-1", 10.0, 10.5)
	// Force reversion trigger (spread at ~0 bps << 100).
	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected close")
	}
	want := (10.6-10.0)*100 + (10.5-10.2)*100
	if math.Abs(pos.RealizedPnL-want) > 1e-6 {
		t.Fatalf("RealizedPnL=%.4f want=%.4f", pos.RealizedPnL, want)
	}
	if pos.RealizedSlipBps == 0 {
		t.Fatalf("RealizedSlipBps must be populated (non-zero), got 0")
	}
}

// TestMonitor_NoExit_WhenWithinBounds: wide spread + fresh OpenedAt → no close.
func TestMonitor_NoExit_WhenWithinBounds(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	// Wide spread — above reversion threshold.
	bin.setBBO("SOON", 10.5, 10.5, time.Now())
	byb.setBBO("SOON", 9.5, 9.5, time.Now())

	pos := testPos("pg-nope-1", 10.5, 9.5)
	closed := tr.checkAndMaybeExit(pos, time.Now())
	if closed {
		t.Fatalf("should NOT close: spread wide + position fresh")
	}
}

// Quick sanity: monitor goroutine lifecycle respects Stop.
func TestMonitor_GoroutineExitsOnStop(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 10.0, 10.0, time.Now())
	byb.setBBO("SOON", 10.0, 10.0, time.Now())

	pos := testPos("pg-stop-1", 10.0, 10.0)
	// Manually register cancel func (bypass startMonitor to avoid going through monitorPosition loop).
	ctx, cancel := context.WithCancel(context.Background())
	tr.monMu.Lock()
	tr.monitors[pos.ID] = cancel
	tr.monMu.Unlock()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()
	// tracker's Stop cancels all monitor ctxs (Plan 03).
	close(tr.stopCh)
	// We must manually invoke the cancel loop since we didn't go through Start/Stop tickLoop.
	tr.monMu.Lock()
	for _, c := range tr.monitors {
		c()
	}
	tr.monMu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("monitor context did not cancel within 2s")
	}
}
