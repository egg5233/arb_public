// Package pricegaptrader — sizer.go is Phase 14 Plan 14-03's notional-cap
// primitive (T-14-01 mitigation, layer 2). Defense in depth: even if
// validatePriceGapLive at config-load slipped a typo through (e.g.
// stage_3_size_usdt: 9999), this sizer caps notional at the hard ceiling
// every time it is consulted. risk_gate.go Gate 6 enforces the same min()
// independently — that is the second-layer redundancy on the sizing path.
package pricegaptrader

import "arb/internal/config"

// Sizer caps Strategy 4 notional sizing per the live-capital ramp.
//
// When PriceGapLiveCapital=false → returns notional unchanged (paper-mode
// pass-through, matches D-07).
// When PriceGapLiveCapital=true → returns min(notional, stage_size, hard_ceiling).
// When stage is out of [1,3] range → returns 0 (fail-closed; T-14-07).
type Sizer struct {
	cfg *config.Config
}

// NewSizer constructs a Sizer backed by the given live-capital config snapshot.
// cfg may be nil (tests) — Cap then returns notional unchanged regardless of stage.
func NewSizer(cfg *config.Config) *Sizer {
	return &Sizer{cfg: cfg}
}

// Cap returns min(notional, stage_size, hard_ceiling) when live capital is
// enabled; otherwise returns notional unchanged (paper-mode pass-through).
//
// Stage out of range (s < 1 or s > 3) returns 0 — fail-closed because
// min(0, hard_ceiling) = 0 means ALL proposals at that stage will be rejected
// downstream (per-position cap in risk_gate Gate 3 already catches notional
// over the candidate's MaxPositionUSDT, and Gate 6 catches notional over the
// ramp budget). A 0-stage proposal cannot pass any further gate, which is
// safer than allowing it through.
func (s *Sizer) Cap(notional float64, stage int) float64 {
	if s.cfg == nil || !s.cfg.PriceGapLiveCapital {
		return notional
	}
	stageSize := s.stageSize(stage)
	if stageSize <= 0 {
		return 0
	}
	cap := stageSize
	if s.cfg.PriceGapHardCeilingUSDT > 0 && s.cfg.PriceGapHardCeilingUSDT < cap {
		// min(stage_size, hard_ceiling)
		cap = s.cfg.PriceGapHardCeilingUSDT
	}
	if notional < cap {
		return notional
	}
	return cap
}

// stageSize returns the configured per-leg notional for stage. Stages outside
// [1,3] return 0 to signal fail-closed (Cap callers convert to 0 result).
func (s *Sizer) stageSize(stage int) float64 {
	switch stage {
	case 1:
		return s.cfg.PriceGapStage1SizeUSDT
	case 2:
		return s.cfg.PriceGapStage2SizeUSDT
	case 3:
		return s.cfg.PriceGapStage3SizeUSDT
	default:
		return 0
	}
}
