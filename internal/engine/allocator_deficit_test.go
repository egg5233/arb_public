package engine

import (
	"math"
	"testing"
)

// computeDeficit implements the max(marginDeficit, ratioDeficit) formula from Bug A.
// Parameters mirror the three call sites in allocator.go / engine.go.
func computeDeficit(totalNeed, available, marginSafetyMultiplier, futuresTotal, marginL4Threshold, marginEpsilon float64) float64 {
	// --- marginDeficit ---
	marginDeficit := totalNeed - available
	if marginDeficit < 0 {
		marginDeficit = 0
	}

	// --- actualMargin (denominator safety) ---
	actualMargin := totalNeed
	if marginSafetyMultiplier > 0 {
		actualMargin = totalNeed / marginSafetyMultiplier
	}
	if actualMargin <= 0 {
		actualMargin = totalNeed
	}

	// --- ratioDeficit ---
	targetRatio := marginL4Threshold - marginEpsilon
	freeTarget := 1.0 - targetRatio
	var ratioDeficit float64
	if freeTarget > 0 && futuresTotal > 0 {
		ratioDeficit = (freeTarget*futuresTotal - available + actualMargin) / targetRatio
		if ratioDeficit < 0 {
			ratioDeficit = 0
		}
	}

	// --- max ---
	result := marginDeficit
	if ratioDeficit > result {
		result = ratioDeficit
	}
	return result
}

func TestComputeDeficit(t *testing.T) {
	const (
		marginL4Headroom       = 0.05
		marginSafetyMultiplier = 2.0
		marginL4Threshold      = 0.80
	)

	tests := []struct {
		name        string
		totalNeed   float64
		available   float64
		futuresTotal float64
		wantMin     float64 // result must be >= wantMin
		wantMax     float64 // result must be <= wantMax
		// which deficit should dominate
		wantDominant string // "margin", "ratio", or "none"
	}{
		{
			// Small futuresTotal keeps ratioDeficit tiny; large need-avail gap makes marginDeficit win.
			// totalNeed=100, available=50 → marginDeficit=50
			// futuresTotal=20, targetRatio=0.75, freeTarget=0.25
			// ratioDeficit = (0.25*20 - 50 + 50) / 0.75 = 5/0.75 ≈ 6.67
			// max(50, 6.67) = 50 → margin wins
			name:         "marginDeficit > ratioDeficit (small futuresTotal, large gap)",
			totalNeed:    100,
			available:    50,
			futuresTotal: 20,
			wantMin:      49.9,
			wantMax:      50.1,
			wantDominant: "margin",
		},
		{
			// Large futuresTotal pushes ratioDeficit above marginDeficit.
			// totalNeed=5, available=48 → marginDeficit=0 (avail>need)
			// futuresTotal=1000, targetRatio=0.75, freeTarget=0.25, actualMargin=2.5
			// ratioDeficit = (0.25*1000 - 48 + 2.5) / 0.75 = 204.5/0.75 ≈ 272.67
			// max(0, 272.67) = 272.67 → ratio wins
			name:         "ratioDeficit > marginDeficit (large futuresTotal, near L4 threshold)",
			totalNeed:    5,
			available:    48,
			futuresTotal: 1000,
			wantMin:      271,
			wantMax:      274,
			wantDominant: "ratio",
		},
		{
			// avail=200 >> need=50, futuresTotal small → both deficits are 0.
			// marginDeficit = 50 - 200 = -150 → clamped to 0
			// ratioDeficit = (0.25*30 - 200 + 25) / 0.75 = (7.5 - 200 + 25)/0.75 < 0 → clamped to 0
			name:         "no deficit (avail >= need, ratio safe)",
			totalNeed:    50,
			available:    200,
			futuresTotal: 30,
			wantMin:      0,
			wantMax:      0,
			wantDominant: "none",
		},
		{
			// futuresTotal = 0 → ratioDeficit branch is skipped, falls back to marginDeficit only.
			// totalNeed=80, available=50 → marginDeficit=30
			// ratioDeficit=0 (futuresTotal==0 guard)
			// max(30, 0) = 30
			name:         "edge: futuresTotal = 0 (ratioDeficit must be 0)",
			totalNeed:    80,
			available:    50,
			futuresTotal: 0,
			wantMin:      29.9,
			wantMax:      30.1,
			wantDominant: "margin",
		},
		{
			// marginSafetyMultiplier = 0 → actualMargin falls back to totalNeed.
			// totalNeed=100, available=80, multiplier=0 → actualMargin=100 (fallback)
			// marginDeficit = 100 - 80 = 20
			// futuresTotal=200, targetRatio=0.75, freeTarget=0.25
			// ratioDeficit = (0.25*200 - 80 + 100) / 0.75 = 70/0.75 ≈ 93.33
			// max(20, 93.33) → ratio wins, result ≈ 93.33
			name:                "edge: marginSafetyMultiplier = 0 (actualMargin falls back to totalNeed)",
			totalNeed:           100,
			available:           80,
			futuresTotal:        200,
			wantMin:             92,
			wantMax:             95,
			wantDominant:        "ratio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiplier := marginSafetyMultiplier
			if tt.name == "edge: marginSafetyMultiplier = 0 (actualMargin falls back to totalNeed)" {
				multiplier = 0
			}

			got := computeDeficit(tt.totalNeed, tt.available, multiplier, tt.futuresTotal, marginL4Threshold, marginL4Headroom)

			// Check range
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeDeficit() = %.4f, want in [%.4f, %.4f]",
					got, tt.wantMin, tt.wantMax)
			}

			// Check dominance
			marginD := tt.totalNeed - tt.available
			if marginD < 0 {
				marginD = 0
			}
			actualM := tt.totalNeed / multiplier
			if actualM <= 0 {
				actualM = tt.totalNeed
			}
			targetRatio := marginL4Threshold - marginL4Headroom
			freeTarget := 1.0 - targetRatio
			var ratioD float64
			if freeTarget > 0 && tt.futuresTotal > 0 {
				ratioD = (freeTarget*tt.futuresTotal - tt.available + actualM) / targetRatio
				if ratioD < 0 {
					ratioD = 0
				}
			}

			switch tt.wantDominant {
			case "margin":
				if marginD <= ratioD && !(math.Abs(marginD-ratioD) < 1e-9) {
					t.Errorf("expected marginDeficit (%.4f) to dominate ratioDeficit (%.4f)", marginD, ratioD)
				}
			case "ratio":
				if ratioD <= marginD && !(math.Abs(marginD-ratioD) < 1e-9) {
					t.Errorf("expected ratioDeficit (%.4f) to dominate marginDeficit (%.4f)", ratioD, marginD)
				}
			case "none":
				if got != 0 {
					t.Errorf("expected zero deficit, got %.4f", got)
				}
			}
		})
	}
}

// TestComputeDeficitNeverNegative verifies the formula never returns a negative value
// across a range of adversarial inputs.
func TestComputeDeficitNeverNegative(t *testing.T) {
	const (
		marginL4Headroom       = 0.05
		marginSafetyMultiplier = 2.0
		marginL4Threshold      = 0.80
	)

	cases := [][4]float64{
		// {totalNeed, available, futuresTotal, multiplier}
		{0, 0, 0, 2.0},
		{0, 100, 500, 2.0},
		{1, 1, 1, 2.0},
		{50, 200, 1000, 2.0},
		{0.001, 0.001, 0, 2.0},
		{100, 100, 0, 0},     // multiplier=0 edge + no futures
		{0, 0, 1000, 0},      // zero need, large futures, multiplier=0
	}

	for _, c := range cases {
		totalNeed, available, futuresTotal, mult := c[0], c[1], c[2], c[3]
		got := computeDeficit(totalNeed, available, mult, futuresTotal, marginL4Threshold, marginL4Headroom)
		if got < 0 {
			t.Errorf("computeDeficit(need=%.3f, avail=%.3f, futures=%.3f, mult=%.1f) = %.6f, want >= 0",
				totalNeed, available, futuresTotal, mult, got)
		}
	}
}
