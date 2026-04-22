package pricegaptrader

import (
	"math"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// paperModeTracker constructs a tracker wired with two stub exchanges and
// toggles PriceGapPaperMode via the passed flag. Mirrors newExecTestTracker.
func paperModeTracker(t *testing.T, paper bool) (*Tracker, *stubExchange, *stubExchange, *fakeStore) {
	t.Helper()
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	store := newFakeStore()
	cfg := &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
		PriceGapPollIntervalSec:      1,
		PriceGapMaxHoldMin:           240,
		PriceGapExitReversionFactor:  0.5,
		PriceGapPaperMode:            paper,
	}
	exch := map[string]exchange.Exchange{
		"binance": longEx,
		"gate":    shortEx,
	}
	tr := NewTracker(exch, store, newFakeDelistChecker(), cfg)
	return tr, longEx, shortEx, store
}

// paperModeCand/Det reused across Task 1 + Task 2 paper tests.
func paperModeCand() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 10, // /2 = 5 bps adverse per leg — easy math
	}
}

func paperModeDet() DetectionResult {
	return DetectionResult{
		SpreadBps:    250,
		MidLong:      100.0, // round numbers for exact float asserts
		MidShort:     102.5,
		StalenessSec: 1,
	}
}

// TestPaperMode_OpenPair_NoPlaceOrder — with paper mode ON, full openPair path
// runs but the exchange stub's PlaceOrder is NEVER invoked on either leg.
func TestPaperMode_OpenPair_NoPlaceOrder(t *testing.T) {
	tr, longEx, shortEx, _ := paperModeTracker(t, true)

	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if n := len(longEx.placedOrders()); n != 0 {
		t.Errorf("long PlaceOrder calls = %d, want 0 in paper mode", n)
	}
	if n := len(shortEx.placedOrders()); n != 0 {
		t.Errorf("short PlaceOrder calls = %d, want 0 in paper mode", n)
	}
}

// TestPaperMode_OpenPair_StampsModePaper — pos.Mode must be "paper".
func TestPaperMode_OpenPair_StampsModePaper(t *testing.T) {
	tr, _, _, _ := paperModeTracker(t, true)
	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.Mode != models.PriceGapModePaper {
		t.Errorf("pos.Mode = %q, want %q", pos.Mode, models.PriceGapModePaper)
	}
}

// TestPaperMode_SynthesizedFillPriceBuy — buy leg synth = mid * (1 + modeled/2/10000).
// modeled=10 → adverse = 5 bps above mid. mid=100 → synth=100.05.
func TestPaperMode_SynthesizedFillPriceBuy(t *testing.T) {
	tr, _, _, _ := paperModeTracker(t, true)
	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	wantLong := 100.0 * (1 + 5.0/10_000.0) // 100.05 (buy leg adverse above mid)
	if math.Abs(pos.LongFillPrice-wantLong) > 1e-9 {
		t.Errorf("LongFillPrice = %.9f, want %.9f", pos.LongFillPrice, wantLong)
	}
}

// TestPaperMode_SynthesizedFillPriceSell — sell leg synth = mid * (1 - modeled/2/10000).
// mid=102.5, adverse below mid → 102.5 * (1 - 5/10000) = 102.449...
func TestPaperMode_SynthesizedFillPriceSell(t *testing.T) {
	tr, _, _, _ := paperModeTracker(t, true)
	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	wantShort := 102.5 * (1 - 5.0/10_000.0)
	if math.Abs(pos.ShortFillPrice-wantShort) > 1e-9 {
		t.Errorf("ShortFillPrice = %.9f, want %.9f", pos.ShortFillPrice, wantShort)
	}
}

// TestPaperMode_OrderIDPrefix — the synth-fill orderID must begin with "paper_".
// We expose this via a test hook: after Task 1, placeLeg returns a fillResult
// whose orderID carries the "paper_" prefix. We exercise placeLeg directly so
// we don't have to smuggle the ID through the pos struct.
func TestPaperMode_OrderIDPrefix(t *testing.T) {
	tr, longEx, _, _ := paperModeTracker(t, true)
	fr := tr.placeLeg(longEx, "SOON", exchange.SideBuy, 100, 6, "ioc", 100.0, 10.0)
	if fr.err != nil {
		t.Fatalf("paper placeLeg err: %v", fr.err)
	}
	if !strings.HasPrefix(fr.orderID, "paper_") {
		t.Errorf("orderID = %q, want prefix paper_", fr.orderID)
	}
	if n := len(longEx.placedOrders()); n != 0 {
		t.Errorf("PlaceOrder calls in paper placeLeg = %d, want 0", n)
	}
}

// TestPaperMode_AcquiresLock — paper entry must still acquire the Redis lock.
// If the lock is forced busy, paper mode must return an error (NOT silently
// proceed). This preserves single-tracker determinism (threat T-09-16).
func TestPaperMode_AcquiresLock(t *testing.T) {
	tr, _, _, store := paperModeTracker(t, true)
	store.forceLockBusy = true

	_, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err == nil {
		t.Fatal("expected lock-busy error in paper mode, got nil")
	}
}

// TestLiveMode_OpenPair_Unchanged — regression: with paper OFF, PlaceOrder is
// invoked exactly once per leg (live path byte-for-byte).
func TestLiveMode_OpenPair_Unchanged(t *testing.T) {
	tr, longEx, shortEx, _ := paperModeTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)

	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if n := len(longEx.placedOrders()); n != 1 {
		t.Errorf("long PlaceOrder calls = %d, want exactly 1 in live mode", n)
	}
	if n := len(shortEx.placedOrders()); n != 1 {
		t.Errorf("short PlaceOrder calls = %d, want exactly 1 in live mode", n)
	}
}

// TestLiveMode_StampsModeLive — pos.Mode must be "live" when paper=false.
func TestLiveMode_StampsModeLive(t *testing.T) {
	tr, longEx, shortEx, _ := paperModeTracker(t, false)
	longEx.queueFill(100, 1.00, nil)
	shortEx.queueFill(100, 1.025, nil)
	pos, err := tr.openPair(paperModeCand(), 100, paperModeDet())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pos.Mode != models.PriceGapModeLive {
		t.Errorf("pos.Mode = %q, want %q", pos.Mode, models.PriceGapModeLive)
	}
}

// ---- Task 2: exit-path paper mode tests ------------------------------------

// newMonitorPaperTracker — like newMonitorTracker but also honors a paper flag
// on the tracker config so we can exercise closePair with either mode.
func newMonitorPaperTracker(t *testing.T, paper bool) (*Tracker, *stubExchange, *stubExchange, *fakeStore) {
	t.Helper()
	cfg := monitorTestCfg()
	cfg.PriceGapPaperMode = paper
	return newMonitorTracker(t, cfg)
}

// paperTestPos builds a position stamped with the given mode.
func paperTestPos(mode string, longFill, shortFill float64, modeledSlip float64) *models.PriceGapPosition {
	p := testPos("pg-paper-1", longFill, shortFill)
	p.Mode = mode
	p.ModeledSlipBps = modeledSlip
	return p
}

// TestPaperMode_ClosePair_NoPlaceOrder — pos.Mode="paper" must NOT invoke PlaceOrder.
func TestPaperMode_ClosePair_NoPlaceOrder(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, true)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	pos := paperTestPos(models.PriceGapModePaper, 10.0, 10.5, 50)
	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected reversion trigger to close paper position")
	}
	if pos.Status != models.PriceGapStatusClosed {
		t.Fatalf("Status = %q, want closed", pos.Status)
	}
	if n := len(bin.placedOrders()); n != 0 {
		t.Errorf("long PlaceOrder calls = %d, want 0 for paper close", n)
	}
	if n := len(byb.placedOrders()); n != 0 {
		t.Errorf("short PlaceOrder calls = %d, want 0 for paper close", n)
	}
}

// TestPaperMode_CloseLegMarket_Synth — closeLegMarket under paper mode must
// synthesize a fill (no PlaceOrder). Smoke-tested via placedOrders count after
// calling closePair with a paper pos.
func TestPaperMode_CloseLegMarket_Synth(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, true)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	pos := paperTestPos(models.PriceGapModePaper, 10.0, 10.5, 50)
	tr.closePair(pos, models.ExitReasonReverted)
	// No PlaceOrder may be invoked on either leg for a paper close.
	if n := len(bin.placedOrders()); n != 0 {
		t.Errorf("long PlaceOrder calls = %d, want 0", n)
	}
	if n := len(byb.placedOrders()); n != 0 {
		t.Errorf("short PlaceOrder calls = %d, want 0", n)
	}
}

// TestPaperMode_RealizedSlippageNonzero — synth uses mid ± modeled/2 so the
// realized-bps pipeline produces a non-zero value (Pitfall 7).
func TestPaperMode_RealizedSlippageNonzero(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, true)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	pos := paperTestPos(models.PriceGapModePaper, 10.0, 10.5, 50) // modeled=50 bps
	tr.closePair(pos, models.ExitReasonReverted)
	if pos.RealizedSlipBps == 0 {
		t.Fatalf("RealizedSlipBps = 0; paper synth must produce non-zero (Pitfall 7)")
	}
}

// TestPaperMode_ModeImmutable — flipping the global PriceGapPaperMode flag
// mid-life must NOT change pos.Mode; closePair reads pos.Mode, never the flag.
func TestPaperMode_ModeImmutable(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, true)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	pos := paperTestPos(models.PriceGapModePaper, 10.0, 10.5, 50)

	// Adversary flips the global flag to LIVE after the paper position is already open.
	tr.cfg.PriceGapPaperMode = false

	tr.closePair(pos, models.ExitReasonReverted)
	if pos.Mode != models.PriceGapModePaper {
		t.Fatalf("pos.Mode mutated to %q; must remain paper through lifecycle", pos.Mode)
	}
	// And critically: no real PlaceOrder fired despite global flag flipping to live.
	if n := len(bin.placedOrders()); n != 0 {
		t.Errorf("long PlaceOrder calls = %d; paper position must not place live orders after flag flip", n)
	}
	if n := len(byb.placedOrders()); n != 0 {
		t.Errorf("short PlaceOrder calls = %d; paper position must not place live orders after flag flip", n)
	}
}

// TestLiveMode_CloseUnchanged — pos.Mode="live" still calls PlaceOrder (regression).
func TestLiveMode_CloseUnchanged(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, false)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())
	bin.queueFill(100, 10.01, nil)
	byb.queueFill(100, 9.99, nil)

	pos := paperTestPos(models.PriceGapModeLive, 10.0, 10.5, 50)
	tr.closePair(pos, models.ExitReasonReverted)
	if n := len(bin.placedOrders()); n < 1 {
		t.Errorf("long PlaceOrder calls = %d, want >= 1 for live close", n)
	}
	if n := len(byb.placedOrders()); n < 1 {
		t.Errorf("short PlaceOrder calls = %d, want >= 1 for live close", n)
	}
}

// TestPaperMode_ExitReasonPropagated — reason string must land on pos.ExitReason
// in both paper and live modes (regression coverage).
func TestPaperMode_ExitReasonPropagated(t *testing.T) {
	tr, bin, byb, _ := newMonitorPaperTracker(t, true)
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())

	pos := paperTestPos(models.PriceGapModePaper, 10.0, 10.5, 50)
	tr.closePair(pos, models.ExitReasonMaxHold)
	if pos.ExitReason != models.ExitReasonMaxHold {
		t.Errorf("ExitReason = %q, want %q", pos.ExitReason, models.ExitReasonMaxHold)
	}
}
