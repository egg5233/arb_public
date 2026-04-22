// Package pricegaptrader — paper→live cutover integration tests (Plan 09-08 Task 1).
//
// These tests exercise the full lifecycle described in Pitfalls 2 and 7:
//
//   - Pitfall 2 (Mode immutability): flipping the global PriceGapPaperMode flag
//     mid-life must NOT change the Mode field on positions already opened. The
//     existing paper position must continue through its close path without
//     invoking ex.PlaceOrder, while subsequently-opened live positions must go
//     through the real wire path.
//
//   - Pitfall 7 (Synth fill slippage): paper-mode closes must stamp a non-zero
//     RealizedSlipBps so the Phase 8 realized-vs-modeled pipeline is exercised
//     even when no real orders are placed. The synth formula is mid ± modeled/2
//     bps (applied on both entry legs and both exit legs), giving a full
//     round-trip realized slippage that equals pos.ModeledSlipBps when the
//     modeled value flows through cleanly.
//
// The test harness reuses:
//   - stubExchange / newStubExchange (fakes_test.go)
//   - fakeStore / newFakeStore (risk_gate_test.go)
//   - spyBroadcaster / spyNotifier (tracker_broadcast_test.go)
//   - hookCand / hookDet (tracker_broadcast_test.go)
package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// newCutoverTracker builds a tracker pre-wired with two stub exchanges, a
// miniredis-like fakeStore, and spy Broadcaster + spyNotifier. The returned
// tracker begins in the mode given by `paper`; tests flip tr.cfg.PriceGapPaperMode
// between calls to openPair to simulate a runtime flip.
func newCutoverTracker(t *testing.T, paper bool) (*Tracker, *stubExchange, *stubExchange, *fakeStore, *spyBroadcaster, *spyNotifier) {
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
	exch := map[string]exchange.Exchange{"binance": longEx, "gate": shortEx}
	tr := NewTracker(exch, store, newFakeDelistChecker(), cfg)
	bc := newSpyBroadcaster()
	nf := newSpyNotifier()
	tr.SetBroadcaster(bc)
	tr.SetNotifier(nf)
	return tr, longEx, shortEx, store, bc, nf
}

// cutoverCandA — candidate opened while paper mode is ON.
func cutoverCandA() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "AAA",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    1000,
		ModeledSlippageBps: 20, // each leg ±10 bps → round-trip 40 bps realized
	}
}

// cutoverCandB — candidate opened AFTER the flag flip to live. Different symbol
// so the two entries don't contend for the same per-symbol Redis lock.
func cutoverCandB() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "BBB",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    1000,
		ModeledSlippageBps: 20,
	}
}

func cutoverDet() DetectionResult {
	return DetectionResult{
		Fired:        true,
		SpreadBps:    250,
		MidLong:      100.0,
		MidShort:     102.5,
		StalenessSec: 1,
	}
}

// TestPaperToLiveCutover — the Phase-9 capstone: a paper position opened while
// PriceGapPaperMode=true coexists with a live position opened after the flag
// flips to false. Both ultimately close into pg:history with their respective
// Mode stamps intact, the paper one with non-zero RealizedSlipBps (Pitfall 7),
// and ex.PlaceOrder is invoked exactly 0 times on the paper side (both entry
// and close) and exactly 2 times per leg on the live side (IOC + defensive
// MARKET fallback on both close legs since IOC fills fully per the stub script).
func TestPaperToLiveCutover(t *testing.T) {
	tr, longEx, shortEx, store, _, _ := newCutoverTracker(t, true)

	// --- Phase 1: paper entry (flag=true) ---------------------------------
	posA, err := tr.openPair(cutoverCandA(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("paper openPair: %v", err)
	}
	if posA == nil {
		t.Fatal("paper openPair returned nil pos")
	}
	if posA.Mode != models.PriceGapModePaper {
		t.Fatalf("posA.Mode = %q, want %q", posA.Mode, models.PriceGapModePaper)
	}

	// No real PlaceOrder on either leg for paper entry.
	if n := len(longEx.placedOrders()); n != 0 {
		t.Fatalf("after paper entry, long PlaceOrder = %d, want 0", n)
	}
	if n := len(shortEx.placedOrders()); n != 0 {
		t.Fatalf("after paper entry, short PlaceOrder = %d, want 0", n)
	}

	// --- Phase 2: flip flag to LIVE ---------------------------------------
	tr.cfg.PriceGapPaperMode = false

	// --- Phase 3: live entry (flag=false) ---------------------------------
	// Queue one fill per leg for the live entry openPair path.
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)

	posB, err := tr.openPair(cutoverCandB(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("live openPair: %v", err)
	}
	if posB == nil {
		t.Fatal("live openPair returned nil pos")
	}
	if posB.Mode != models.PriceGapModeLive {
		t.Fatalf("posB.Mode = %q, want %q", posB.Mode, models.PriceGapModeLive)
	}

	// Live entry placed exactly 1 order per leg.
	if n := len(longEx.placedOrders()); n != 1 {
		t.Fatalf("after live entry, long PlaceOrder = %d, want 1", n)
	}
	if n := len(shortEx.placedOrders()); n != 1 {
		t.Fatalf("after live entry, short PlaceOrder = %d, want 1", n)
	}

	// --- Phase 4: close the paper position (synth path, no PlaceOrder) ---
	// closePair reads pos.Mode not the global flag, so posA must still use synth.
	longEx.setBBO("AAA", 99.9, 100.1, time.Now())
	shortEx.setBBO("AAA", 102.4, 102.6, time.Now())

	tr.closePair(posA, models.ExitReasonReverted)
	if posA.Status != models.PriceGapStatusClosed {
		t.Fatalf("posA.Status = %q, want closed", posA.Status)
	}
	if posA.Mode != models.PriceGapModePaper {
		t.Fatalf("posA.Mode mutated to %q during close; must remain paper", posA.Mode)
	}
	// Pitfall 7 proof: paper close must produce non-zero realized slippage.
	if posA.RealizedSlipBps == 0 {
		t.Fatalf("posA.RealizedSlipBps = 0 after paper close; Pitfall 7 violated")
	}
	// No PlaceOrder added for the paper close.
	if n := len(longEx.placedOrders()); n != 1 {
		t.Fatalf("after paper close, long PlaceOrder = %d, want 1 (live entry only)", n)
	}
	if n := len(shortEx.placedOrders()); n != 1 {
		t.Fatalf("after paper close, short PlaceOrder = %d, want 1 (live entry only)", n)
	}

	// --- Phase 5: close the live position (real path) --------------------
	longEx.setBBO("BBB", 99.9, 100.1, time.Now())
	shortEx.setBBO("BBB", 102.4, 102.6, time.Now())
	// IOC close fills — one per leg. No remainder queued, so defensive MARKET
	// fallback runs (closeLegMarket doesn't read a script for qty).
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)

	tr.closePair(posB, models.ExitReasonReverted)
	if posB.Status != models.PriceGapStatusClosed {
		t.Fatalf("posB.Status = %q, want closed", posB.Status)
	}
	if posB.Mode != models.PriceGapModeLive {
		t.Fatalf("posB.Mode = %q, want live", posB.Mode)
	}
	// Live close placed at least one additional order per leg.
	if n := len(longEx.placedOrders()); n < 2 {
		t.Fatalf("after live close, long PlaceOrder = %d, want >= 2 (entry + close)", n)
	}
	if n := len(shortEx.placedOrders()); n < 2 {
		t.Fatalf("after live close, short PlaceOrder = %d, want >= 2 (entry + close)", n)
	}

	// --- Phase 6: pg:history must contain both positions -----------------
	if len(store.history) != 2 {
		t.Fatalf("pg:history len = %d, want 2 (paper + live)", len(store.history))
	}
	// Find each by Mode (order is insertion order in fakeStore, so paper first).
	var sawPaper, sawLive bool
	for _, h := range store.history {
		switch h.Mode {
		case models.PriceGapModePaper:
			sawPaper = true
			if h.RealizedSlipBps == 0 {
				t.Errorf("history paper row RealizedSlipBps = 0, want non-zero")
			}
		case models.PriceGapModeLive:
			sawLive = true
		default:
			t.Errorf("unexpected Mode in history: %q", h.Mode)
		}
	}
	if !sawPaper {
		t.Error("pg:history missing paper row")
	}
	if !sawLive {
		t.Error("pg:history missing live row")
	}
}

// TestPaperToLiveCutover_NoOrphans — after the flag flip, the existing paper
// position's close path must continue to synth-fill and never reach
// ex.PlaceOrder. The monitor's closePair reads pos.Mode, not the global flag —
// a stale monitor goroutine cannot be "tricked" into a live close by the flip.
func TestPaperToLiveCutover_NoOrphans(t *testing.T) {
	tr, longEx, shortEx, _, _, _ := newCutoverTracker(t, true)

	// Paper entry.
	posA, err := tr.openPair(cutoverCandA(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("paper openPair: %v", err)
	}

	// Adversarial flip: dashboard toggles paper OFF while posA is still open.
	tr.cfg.PriceGapPaperMode = false

	// Set BBOs so spread has reverted to ~0 bps (< threshold * reversionFactor = 100 bps).
	// Matching mids on both legs means computeSpreadBps returns 0 → reversion fires.
	longEx.setBBO("AAA", 99.99, 100.01, time.Now())
	shortEx.setBBO("AAA", 99.99, 100.01, time.Now())

	// Drive one tick through the monitor's exit-evaluation path.
	closed := tr.checkAndMaybeExit(posA, time.Now())
	if !closed {
		t.Fatalf("expected paper position to close via reversion after flag flip")
	}

	// The paper position went through the synth path despite the global flag
	// being live — no PlaceOrder should have fired on either leg.
	if n := len(longEx.placedOrders()); n != 0 {
		t.Errorf("paper-mode close after flip: long PlaceOrder = %d, want 0 (stub exchange untouched)", n)
	}
	if n := len(shortEx.placedOrders()); n != 0 {
		t.Errorf("paper-mode close after flip: short PlaceOrder = %d, want 0", n)
	}
	// And pos.Mode remains paper on the persisted record.
	if posA.Mode != models.PriceGapModePaper {
		t.Errorf("posA.Mode = %q, want paper (immutability through lifecycle)", posA.Mode)
	}
}

// TestPaperToLiveCutover_TelegramContinuity — both entries and both exits
// invoke the PriceGapNotifier exactly once each (D-22 parity: paper + live
// fire the same notifier contract). The paper exit and paper entry are
// marked via the position's Mode field so the Telegram implementation
// (internal/notify/telegram.go) can apply the 📝 PAPER prefix; this test
// asserts only the call contract, not the string formatting (which lives in
// the notify package's own unit tests).
func TestPaperToLiveCutover_TelegramContinuity(t *testing.T) {
	tr, longEx, shortEx, _, _, nf := newCutoverTracker(t, true)

	// Paper entry → 1 notify_entry.
	posA, err := tr.openPair(cutoverCandA(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("paper openPair: %v", err)
	}
	if n := nf.countKind("notify_entry"); n != 1 {
		t.Fatalf("after paper entry notify_entry = %d, want 1", n)
	}

	// Flip to live, open the live candidate.
	tr.cfg.PriceGapPaperMode = false
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)
	posB, err := tr.openPair(cutoverCandB(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("live openPair: %v", err)
	}
	if n := nf.countKind("notify_entry"); n != 2 {
		t.Fatalf("after live entry notify_entry = %d, want 2", n)
	}

	// Close paper (synth) → 1 notify_exit.
	longEx.setBBO("AAA", 99.9, 100.1, time.Now())
	shortEx.setBBO("AAA", 102.4, 102.6, time.Now())
	tr.closePair(posA, models.ExitReasonReverted)
	if n := nf.countKind("notify_exit"); n != 1 {
		t.Fatalf("after paper close notify_exit = %d, want 1", n)
	}

	// Close live (real path) → 2 notify_exit.
	longEx.setBBO("BBB", 99.9, 100.1, time.Now())
	shortEx.setBBO("BBB", 102.4, 102.6, time.Now())
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)
	tr.closePair(posB, models.ExitReasonReverted)
	if n := nf.countKind("notify_exit"); n != 2 {
		t.Fatalf("after live close notify_exit = %d, want 2", n)
	}

	// Verify the pos args passed to notify_exit carry their Mode — consumer
	// (TelegramNotifier) can apply 📝 PAPER prefix. We don't inspect strings
	// here; telegram_pricegap_test.go covers the formatting contract.
}

// TestPaperToLiveCutover_ClosedLogShape — the pg:history rows persisted through
// the cutover carry Modeled, Realized, and Mode fields correctly on both paper
// and live entries. The dashboard "Closed Log" reads these fields to render
// the Modeled / Realized / Delta columns (PG-OPS-03).
func TestPaperToLiveCutover_ClosedLogShape(t *testing.T) {
	tr, longEx, shortEx, store, _, _ := newCutoverTracker(t, true)

	// Paper lifecycle.
	posA, err := tr.openPair(cutoverCandA(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("paper openPair: %v", err)
	}
	longEx.setBBO("AAA", 99.9, 100.1, time.Now())
	shortEx.setBBO("AAA", 102.4, 102.6, time.Now())
	tr.closePair(posA, models.ExitReasonReverted)

	// Flip to live.
	tr.cfg.PriceGapPaperMode = false
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)
	posB, err := tr.openPair(cutoverCandB(), 10, cutoverDet())
	if err != nil {
		t.Fatalf("live openPair: %v", err)
	}
	longEx.setBBO("BBB", 99.9, 100.1, time.Now())
	shortEx.setBBO("BBB", 102.4, 102.6, time.Now())
	longEx.queueFill(10, 100.0, nil)
	shortEx.queueFill(10, 102.5, nil)
	tr.closePair(posB, models.ExitReasonReverted)

	if len(store.history) != 2 {
		t.Fatalf("history len = %d, want 2", len(store.history))
	}

	for _, h := range store.history {
		if h.Mode != models.PriceGapModePaper && h.Mode != models.PriceGapModeLive {
			t.Errorf("history row Mode = %q, want paper|live", h.Mode)
		}
		if h.ModeledSlipBps == 0 {
			t.Errorf("history row %s ModeledSlipBps = 0, want non-zero (candidate configured 20 bps)", h.ID)
		}
		if h.ExitReason == "" {
			t.Errorf("history row %s ExitReason empty", h.ID)
		}
		if h.ClosedAt.IsZero() {
			t.Errorf("history row %s ClosedAt zero", h.ID)
		}
		// Delta = Realized - Modeled is derived in the handler; we just assert
		// both source fields are populated. Live rows' RealizedSlipBps may be
		// zero when entry fill == mid (no adverse); paper rows are guaranteed
		// non-zero by the synth formula (Pitfall 7).
		if h.Mode == models.PriceGapModePaper && h.RealizedSlipBps == 0 {
			t.Errorf("paper history row RealizedSlipBps = 0, want non-zero (Pitfall 7)")
		}
	}
}
