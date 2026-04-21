package engine

import (
	"math"
	"testing"

	"arb/internal/config"
)

func TestCapByMarginHealth_GateioIncident(t *testing.T) {
	e := &Engine{cfg: &config.Config{MarginL4Threshold: 0.80, MarginL5Threshold: 0.95}}
	bal := rebalanceBalanceInfo{
		futures:      64.93,
		futuresTotal: 259.50,
		marginRatio:  0.0588,
		hasPositions: true,
	}
	got := e.capByMarginHealth(bal)
	// usage cap: 259.50 - (259.50-64.93)/0.80 = 16.2875
	// maint cap: 259.50 - 0.0588*259.50/0.95 = 243.444
	// min = 16.2875
	want := 16.29
	if math.Abs(got-want) > 0.5 {
		t.Errorf("capByMarginHealth gateio: got %.2f want ~%.2f", got, want)
	}
}

func TestCapByMarginHealth_NoPositions(t *testing.T) {
	e := &Engine{cfg: &config.Config{MarginL4Threshold: 0.80, MarginL5Threshold: 0.95}}
	bal := rebalanceBalanceInfo{futures: 100, futuresTotal: 100, hasPositions: false}
	if got := e.capByMarginHealth(bal); got != math.MaxFloat64 {
		t.Errorf("expected MaxFloat64 for no positions, got %v", got)
	}
}

func TestCapByMarginHealth_FullyDrawn(t *testing.T) {
	e := &Engine{cfg: &config.Config{MarginL4Threshold: 0.80, MarginL5Threshold: 0.95}}
	bal := rebalanceBalanceInfo{
		futures: 0, futuresTotal: 100, marginRatio: 0.50, hasPositions: true,
	}
	// frozen=100, capUsage = 100 - 100/0.80 = -25 -> clamped to 0
	if got := e.capByMarginHealth(bal); got != 0 {
		t.Errorf("expected 0 cap when fully drawn, got %v", got)
	}
}

func TestRebalanceUsageRatio(t *testing.T) {
	cases := []struct {
		name string
		bal  rebalanceBalanceInfo
		want float64
	}{
		{"zero total", rebalanceBalanceInfo{futuresTotal: 0, futures: 0}, 0},
		{"empty", rebalanceBalanceInfo{futuresTotal: 100, futures: 100}, 0},
		{"fully used", rebalanceBalanceInfo{futuresTotal: 100, futures: 0}, 1.0},
		{"gateio incident", rebalanceBalanceInfo{futuresTotal: 259.50, futures: 64.93}, 0.7498},
		{"negative futures", rebalanceBalanceInfo{futuresTotal: 100, futures: -5}, 1.0},
		{"futures > total (edge)", rebalanceBalanceInfo{futuresTotal: 100, futures: 120}, 0},
	}
	for _, c := range cases {
		got := rebalanceUsageRatio(c.bal)
		if math.Abs(got-c.want) > 1e-3 {
			t.Errorf("%s: got %.4f want %.4f", c.name, got, c.want)
		}
	}
}
