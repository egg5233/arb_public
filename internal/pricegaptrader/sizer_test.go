package pricegaptrader

import "testing"

// Phase 14 sizer stubs — bodies land in plan 14-03.
// Layer-2 hard-ceiling enforcement: even if a config typo escapes layer-1,
// the sizer caps at 1000 USDT/leg.

func TestSizer_MinStageHardCeilingEnforced(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-03")
}

func TestSizer_TypoStage3Of9999_StillSizesAt1000(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-03")
}
