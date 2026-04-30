package pricegaptrader

import (
	"testing"

	"arb/internal/config"
)

// Phase 14 Plan 14-03 — Sizer tests. Layer-2 hard-ceiling enforcement: even
// if a config typo escapes layer-1 (validatePriceGapLive), the sizer caps
// every sizing call at min(stage_size, hard_ceiling).

func TestSizer_MinStageHardCeilingEnforced(t *testing.T) {
	cfg := &config.Config{
		PriceGapLiveCapital:     true,
		PriceGapStage1SizeUSDT:  100,
		PriceGapStage2SizeUSDT:  500,
		PriceGapStage3SizeUSDT:  1000,
		PriceGapHardCeilingUSDT: 1000,
	}
	s := NewSizer(cfg)

	// stage=2, notional=600 > stage2=500 → caps at stage2.
	if got := s.Cap(600, 2); got != 500 {
		t.Fatalf("expected 500 (stage2 cap), got %v", got)
	}

	// stage=3, notional=200 < stage3=1000 → returns notional unchanged.
	if got := s.Cap(200, 3); got != 200 {
		t.Fatalf("expected 200 (notional unchanged), got %v", got)
	}

	// stage=1, notional=200 > stage1=100 → caps at stage1.
	if got := s.Cap(200, 1); got != 100 {
		t.Fatalf("expected 100 (stage1 cap), got %v", got)
	}

	// Stage out of range → fail-closed 0.
	if got := s.Cap(500, 5); got != 0 {
		t.Fatalf("expected 0 (out-of-range stage), got %v", got)
	}
	if got := s.Cap(500, 0); got != 0 {
		t.Fatalf("expected 0 (out-of-range stage 0), got %v", got)
	}
	if got := s.Cap(500, -1); got != 0 {
		t.Fatalf("expected 0 (negative stage), got %v", got)
	}

	// PriceGapLiveCapital=false → pass-through (paper mode, D-07).
	cfg.PriceGapLiveCapital = false
	if got := s.Cap(9999, 3); got != 9999 {
		t.Fatalf("expected 9999 (paper-mode pass-through), got %v", got)
	}
}

func TestSizer_TypoStage3Of9999_StillSizesAt1000(t *testing.T) {
	// T-14-01 mitigation locked here. Operator typo
	// stage_3_size_usdt: 9999 slipped past validatePriceGapLive (e.g. validator
	// was bypassed or config loaded from older binary). The Sizer is the
	// SECOND layer — must still cap at hard_ceiling=1000.
	cfg := &config.Config{
		PriceGapLiveCapital:     true,
		PriceGapStage1SizeUSDT:  100,
		PriceGapStage2SizeUSDT:  500,
		PriceGapStage3SizeUSDT:  9999, // ← TYPO that escaped layer-1
		PriceGapHardCeilingUSDT: 1000,
	}
	s := NewSizer(cfg)

	if got := s.Cap(2000, 3); got != 1000 {
		t.Fatalf("DEFENSE IN DEPTH FAILED: typo Stage3=9999 should be capped by HardCeiling=1000, got %v", got)
	}
	// And a notional below the ceiling still passes unchanged.
	if got := s.Cap(800, 3); got != 800 {
		t.Fatalf("expected 800 (notional below ceiling unchanged), got %v", got)
	}
}
