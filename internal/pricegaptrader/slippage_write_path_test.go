package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/models"
)

// TestSlippageWritePath_WrittenBeforeHistory — after closePair, the history
// entry must carry BOTH ModeledSlipBps (set at entry in openPair) AND
// RealizedSlipBps (set at close here). This is the D-23 guarantee Plan 05
// relies on: no need to read pg:slippage:* to compute realized-vs-modeled.
func TestSlippageWritePath_WrittenBeforeHistory(t *testing.T) {
	tr, bin, byb, store := newMonitorTracker(t, monitorTestCfg())
	// Fresh BBOs so spread ≈ 0 triggers reversion.
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())
	// Queue full-size IOC exit fills for both legs with concrete vwap.
	bin.queueFill(100, 10.01, nil)
	byb.queueFill(100, 9.99, nil)

	pos := testPos("pg-slip-1", 10.0, 10.5)
	pos.ModeledSlipBps = 9.75 // stamped at entry in production via openPair

	closed := tr.checkAndMaybeExit(pos, time.Now())
	if !closed {
		t.Fatalf("expected close")
	}

	// History must have exactly one row and it must carry the modeled + realized fields.
	if len(store.history) != 1 {
		t.Fatalf("history len: got %d, want 1", len(store.history))
	}
	row := store.history[0]
	if row.ModeledSlipBps != 9.75 {
		t.Fatalf("history ModeledSlipBps: got %v, want 9.75 (preserved through close path)", row.ModeledSlipBps)
	}
	// RealizedSlipBps is assigned by closePair as the sum of 4 per-leg bps
	// terms; guarded to be non-zero when mids and fills differ.
	if row.RealizedSlipBps == 0 {
		t.Fatalf("history RealizedSlipBps: got 0, want a measured non-zero value")
	}
	if row.Status != models.PriceGapStatusClosed {
		t.Fatalf("row.Status: got %q, want closed", row.Status)
	}
	if row.ExitReason != models.ExitReasonReverted {
		t.Fatalf("row.ExitReason: got %q, want %q", row.ExitReason, models.ExitReasonReverted)
	}
}

// TestSlippageWritePath_ModeledSurvivesClose — the ModeledSlipBps value from
// entry must not be overwritten at close. closePair writes RealizedSlipBps
// but leaves ModeledSlipBps alone — Plan 05 needs the pairing.
func TestSlippageWritePath_ModeledSurvivesClose(t *testing.T) {
	tr, bin, byb, store := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())
	bin.queueFill(100, 10.01, nil)
	byb.queueFill(100, 9.99, nil)

	pos := testPos("pg-slip-modeled", 10.0, 10.5)
	pos.ModeledSlipBps = 42.0

	_ = tr.checkAndMaybeExit(pos, time.Now())

	if len(store.history) != 1 {
		t.Fatalf("history len: got %d, want 1", len(store.history))
	}
	if store.history[0].ModeledSlipBps != 42.0 {
		t.Fatalf("ModeledSlipBps clobbered by close: got %v, want 42.0", store.history[0].ModeledSlipBps)
	}
}

// TestSlippageWritePath_FieldsOnHistoryNotActive — closePair must remove the
// position from the active set AND add exactly one history row carrying both
// slippage fields. Guards against a future refactor that might write the row
// before stamping realized slip (breaks D-23).
func TestSlippageWritePath_ActiveRemovedAndHistoryStamped(t *testing.T) {
	tr, bin, byb, store := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 9.99, 10.01, time.Now())
	byb.setBBO("SOON", 9.99, 10.01, time.Now())
	bin.queueFill(100, 10.01, nil)
	byb.queueFill(100, 9.99, nil)

	pos := testPos("pg-slip-2", 10.0, 10.5)
	pos.ModeledSlipBps = 5.5

	// Insert into active via SavePriceGapPosition so the post-close
	// RemoveActivePriceGapPosition path is exercised against the fakeStore.
	_ = store.SavePriceGapPosition(pos)

	_ = tr.checkAndMaybeExit(pos, time.Now())

	// Active set must be empty after close.
	active, _ := store.GetActivePriceGapPositions()
	if len(active) != 0 {
		t.Fatalf("active after close: got %d, want 0", len(active))
	}
	// History must have 1 row with both slip fields.
	if len(store.history) != 1 {
		t.Fatalf("history len: got %d, want 1", len(store.history))
	}
	if store.history[0].ModeledSlipBps == 0 || store.history[0].RealizedSlipBps == 0 {
		t.Fatalf("history row slip fields: modeled=%v realized=%v, want both non-zero",
			store.history[0].ModeledSlipBps, store.history[0].RealizedSlipBps)
	}
}
