package risk

import (
	"testing"

	"arb/internal/config"
	"arb/pkg/exchange"
)

func TestComputeLevel(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
	}

	h := &HealthMonitor{cfg: cfg}

	tests := []struct {
		name     string
		bal      *exchange.Balance
		pnl      float64
		posCount int
		want     HealthLevel
	}{
		{
			name:     "L0: no positions",
			bal:      &exchange.Balance{Total: 100, Available: 100, MarginRatio: 0},
			pnl:      0,
			posCount: 0,
			want:     L0None,
		},
		{
			name:     "L1: positions with positive PnL",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.10},
			pnl:      5.0,
			posCount: 1,
			want:     L1Safe,
		},
		{
			name:     "L2: negative PnL but low margin ratio",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.20},
			pnl:      -2.0,
			posCount: 1,
			want:     L2Low,
		},
		{
			name:     "L3: negative PnL, margin ratio at L3 threshold",
			bal:      &exchange.Balance{Total: 100, Available: 50, MarginRatio: 0.55},
			pnl:      -5.0,
			posCount: 1,
			want:     L3Medium,
		},
		{
			name:     "L4: negative PnL, margin ratio at L4 threshold",
			bal:      &exchange.Balance{Total: 100, Available: 20, MarginRatio: 0.85},
			pnl:      -10.0,
			posCount: 1,
			want:     L4High,
		},
		{
			name:     "L5: critical margin ratio",
			bal:      &exchange.Balance{Total: 100, Available: 5, MarginRatio: 0.96},
			pnl:      -15.0,
			posCount: 1,
			want:     L5Critical,
		},
		{
			name:     "L5: critical even with positive PnL",
			bal:      &exchange.Balance{Total: 100, Available: 3, MarginRatio: 0.97},
			pnl:      1.0,
			posCount: 1,
			want:     L5Critical,
		},
		{
			name:     "L1: zero PnL counts as safe",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.10},
			pnl:      0,
			posCount: 1,
			want:     L1Safe,
		},
		{
			name:     "hybrid fallback: margin ratio from available/total when MarginRatio=0",
			bal:      &exchange.Balance{Total: 100, Available: 40, MarginRatio: 0},
			pnl:      -5.0,
			posCount: 1,
			want:     L3Medium, // 1 - 40/100 = 0.60 >= L3(0.50)
		},
		{
			name:     "skip synthetic fallback when ratio explicitly unavailable",
			bal:      &exchange.Balance{Total: 100, Available: 40, MarginRatio: 0, MarginRatioUnavailable: true},
			pnl:      -5.0,
			posCount: 1,
			want:     L2Low,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.computeLevel(h.normalizeMarginRatio(tt.bal), tt.pnl, tt.posCount)
			if got != tt.want {
				t.Errorf("computeLevel() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestHealthLevelString(t *testing.T) {
	tests := []struct {
		level HealthLevel
		want  string
	}{
		{L0None, "L0-None"},
		{L1Safe, "L1-Safe"},
		{L2Low, "L2-Low"},
		{L3Medium, "L3-Medium"},
		{L4High, "L4-High"},
		{L5Critical, "L5-Critical"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("HealthLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}
