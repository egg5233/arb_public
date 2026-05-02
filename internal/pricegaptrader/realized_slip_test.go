package pricegaptrader

// PG-FIX-01 regression tests — guard the realized-slip formula against
// re-introducing the paper-mode override at monitor.go:242-244 and verify
// the BBO-stale fallback (D-01) collapses gracefully.
//
// Tests in this file exist to lock in the fix from Plan 16-01:
//   - Paper-mode realized slip is now non-zero AND can differ from
//     ModeledSlipBps (the override is gone).
//   - When BBO is stale at close (sampleLegs returns error or zero mid),
//     LongMidAtExit/ShortMidAtExit fall back to *MidAtDecision and the
//     realized slip collapses to modeled-only contribution.
//   - Live-mode close path is unchanged (no paper branch in formula).

import (
	"math"
	"testing"
	"time"

	"arb/internal/models"
)

// realizedSlipPos builds a position stamped with the given mode + entry mids.
// LongFillPrice / ShortFillPrice mirror *MidAtDecision so the entry-bps
// contribution is zero — any non-zero realized slip must come from exit-side
// drift (which is exactly what PG-FIX-01 wires up).
func realizedSlipPos(mode string, longMid, shortMid, modeledSlip float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:                 "pg-slip-1",
		Symbol:             "SOON",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		Status:             models.PriceGapStatusOpen,
		EntrySpreadBps:     250,
		ThresholdBps:       200,
		NotionalUSDT:       1000,
		LongFillPrice:      longMid,  // entry fill == entry mid → entry-bps contribution = 0
		ShortFillPrice:     shortMid, // entry fill == entry mid → entry-bps contribution = 0
		LongSize:           100,
		ShortSize:          100,
		LongMidAtDecision:  longMid,
		ShortMidAtDecision: shortMid,
		ModeledSlipBps:     modeledSlip,
		Mode:               mode,
		OpenedAt:           time.Now(),
	}
}

// TestRealizedSlip_PaperNonZero — paper-mode close with mid drift between
// entry and close must produce non-zero RealizedSlipBps that DIFFERS from
// ModeledSlipBps (the override is deleted; formula uses LongMidAtExit /
// ShortMidAtExit captured from live BBO at close).
//
// Setup: entry mid 100/100, exit BBO mid 100.5 (long) / 99.5 (short).
// Paper synth fills at MidAtDecision ± modeled/2 — i.e. 100 - 5/10000*100 =
// 99.95 on long-sell, 100 + 5/10000*100 = 100.05 on short-buy.
// Entry-bps both legs = 0 (entry fill == entry mid).
// Exit-bps long  = (99.95 - 100.5) / 100.5 * 10000 ≈ -54.7 bps
// Exit-bps short = (100.5 - 100.05) / 99.5 * 10000 ≈ +45.2 bps  (note: short formula uses (mid - exit) sign)
// Sum ≈ -9.5 bps; |sum - ModeledSlipBps(=10)| > 0.5 confirmed.
func TestRealizedSlip_PaperNonZero(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	// Exit-side BBO drifts away from entry mid (entry mid was 100/100).
	bin.setBBO("SOON", 100.5, 100.5, time.Now())
	byb.setBBO("SOON", 99.5, 99.5, time.Now())

	pos := realizedSlipPos(models.PriceGapModePaper, 100.0, 100.0, 10)
	tr.closePair(pos, models.ExitReasonReverted)

	if pos.LongMidAtExit == 0 {
		t.Fatalf("LongMidAtExit = 0; closePair must stamp from BBO at close (PG-FIX-01)")
	}
	if pos.ShortMidAtExit == 0 {
		t.Fatalf("ShortMidAtExit = 0; closePair must stamp from BBO at close (PG-FIX-01)")
	}
	if math.Abs(pos.LongMidAtExit-100.5) > 1e-9 {
		t.Errorf("LongMidAtExit = %.6f, want 100.5 (BBO mid at close)", pos.LongMidAtExit)
	}
	if math.Abs(pos.ShortMidAtExit-99.5) > 1e-9 {
		t.Errorf("ShortMidAtExit = %.6f, want 99.5 (BBO mid at close)", pos.ShortMidAtExit)
	}
	if pos.RealizedSlipBps == 0 {
		t.Fatalf("RealizedSlipBps = 0 after paper close with mid drift; override must be removed (PG-FIX-01)")
	}
	if math.Abs(pos.RealizedSlipBps-pos.ModeledSlipBps) <= 0.5 {
		t.Fatalf("RealizedSlipBps (%.4f) too close to ModeledSlipBps (%.4f); override may still be active (PG-FIX-01)",
			pos.RealizedSlipBps, pos.ModeledSlipBps)
	}
}

// TestRealizedSlip_PaperBBOStaleFallback — when sampleLegs cannot produce a
// mid at close (no BBO ever set on the stub), LongMidAtExit / ShortMidAtExit
// must fall back to *MidAtDecision (D-01 / Pitfall 10). The slip formula then
// collapses to modeled-only contribution since synth fills are exactly
// MidAtDecision ± modeled/2 against an exit reference of MidAtDecision.
//
// Sum: |0 + 0 + (5)| + |0 + 0 + (5)| = 10 bps == ModeledSlipBps.
func TestRealizedSlip_PaperBBOStaleFallback(t *testing.T) {
	tr, _, _, _ := newMonitorTracker(t, monitorTestCfg())
	// Intentionally do NOT call setBBO on either stub — sampleLegs will return
	// an error ("long GetBBO not populated"), which the closePair fix must
	// handle by falling back to *MidAtDecision.

	pos := realizedSlipPos(models.PriceGapModePaper, 100.0, 100.0, 10)
	tr.closePair(pos, models.ExitReasonReverted)

	if math.Abs(pos.LongMidAtExit-pos.LongMidAtDecision) > 1e-9 {
		t.Errorf("BBO-stale fallback: LongMidAtExit = %.6f, want LongMidAtDecision (%.6f) (D-01)",
			pos.LongMidAtExit, pos.LongMidAtDecision)
	}
	if math.Abs(pos.ShortMidAtExit-pos.ShortMidAtDecision) > 1e-9 {
		t.Errorf("BBO-stale fallback: ShortMidAtExit = %.6f, want ShortMidAtDecision (%.6f) (D-01)",
			pos.ShortMidAtExit, pos.ShortMidAtDecision)
	}
	if math.Abs(pos.RealizedSlipBps-pos.ModeledSlipBps) > 0.5 {
		t.Errorf("BBO-stale fallback: RealizedSlipBps = %.4f, want ~ModeledSlipBps (%.4f) (D-01)",
			pos.RealizedSlipBps, pos.ModeledSlipBps)
	}
}

// TestRealizedSlip_LiveStillWorks — live-mode close with mid drift must
// continue to produce a non-zero RealizedSlipBps reflecting actual fills
// against entry/exit mids; no paper-mode override branch.
//
// Setup: live entry at mid 100/100; live close fills queued at 100.4 (long
// sell) and 99.6 (short buy); exit BBO mid 100.5/99.5. Entry-bps = 0 (entry
// fill == entry mid). Live exit fill prices come from the queued fills
// (vwapReader returns scripted vwap).
//   exit-long  = (100.4 - 100.5) / 100.5 * 10000 ≈ -9.95 bps
//   exit-short = (99.5 - 99.6) / 99.5 * 10000   ≈ -10.05 bps
//   sum ≈ -20 bps → non-zero, no override branch.
func TestRealizedSlip_LiveStillWorks(t *testing.T) {
	tr, bin, byb, _ := newMonitorTracker(t, monitorTestCfg())
	bin.setBBO("SOON", 100.5, 100.5, time.Now())
	byb.setBBO("SOON", 99.5, 99.5, time.Now())
	bin.queueFill(100, 100.4, nil) // long close = SELL
	byb.queueFill(100, 99.6, nil)  // short close = BUY

	pos := realizedSlipPos(models.PriceGapModeLive, 100.0, 100.0, 10)
	tr.closePair(pos, models.ExitReasonReverted)

	if pos.RealizedSlipBps == 0 {
		t.Fatalf("RealizedSlipBps = 0 on live close with mid drift; live formula regressed")
	}
	// Live formula uses MidAtExit(=100.5/99.5) as exit reference; result is
	// well-separated from ModeledSlipBps(=10) so we can assert no override.
	if math.Abs(pos.RealizedSlipBps-pos.ModeledSlipBps) <= 0.5 {
		t.Errorf("Live RealizedSlipBps (%.4f) too close to ModeledSlipBps (%.4f); override leaked into live path",
			pos.RealizedSlipBps, pos.ModeledSlipBps)
	}
	if pos.LongMidAtExit == 0 || pos.ShortMidAtExit == 0 {
		t.Errorf("Live close did not stamp MidAtExit; LongMidAtExit=%.6f ShortMidAtExit=%.6f",
			pos.LongMidAtExit, pos.ShortMidAtExit)
	}
}
