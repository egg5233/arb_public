package strategy

import (
	"math"
	"testing"
)

func TestEVFormulas(t *testing.T) {
	tests := []struct {
		name string
		got  EVBreakdown
		want float64
	}{
		{
			name: "dir b positive funding pays shorts",
			got:  DirBEV(2.0, 16, 8),
			want: 0,
		},
		{
			name: "dir a positive published funding hurts long futures",
			got:  DirAEV(2.0, 16, 0.0876, 8),
			want: -4.10,
		},
		{
			name: "perp perp spread minus fees and rotation",
			got:  PPEV(3.0, 1.0, 16, 1.68, 8),
			want: -0.01,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if math.Abs(tt.got.NetBpsH-tt.want) > 0.01 {
				t.Fatalf("NetBpsH = %.4f, want %.4f", tt.got.NetBpsH, tt.want)
			}
		})
	}
}
