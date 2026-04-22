package exchange

import "testing"

func TestDetectPrefixMultiplier(t *testing.T) {
	tests := []struct {
		in       string
		wantBare string
		wantMult float64
	}{
		{"1000PEPE", "PEPE", 1000},
		{"10000QUBIC", "QUBIC", 10000},
		{"1000000BABYDOGE", "BABYDOGE", 1_000_000},
		{"10000000AIDOGE", "AIDOGE", 10_000_000},
		{"1INCH", "1INCH", 1},
		{"0G", "0G", 1},
		{"2Z", "2Z", 1},
		{"4", "4", 1},
		{"BTC", "BTC", 1},
		{"", "", 1},
		{"1000", "1000", 1},
		{"123ABC", "123ABC", 1},
		{"1000000", "1000000", 1},
	}

	for _, tc := range tests {
		gotBare, gotMult := DetectPrefixMultiplier(tc.in)
		if gotBare != tc.wantBare || gotMult != tc.wantMult {
			t.Fatalf("%q -> (%q, %v), want (%q, %v)", tc.in, gotBare, gotMult, tc.wantBare, tc.wantMult)
		}
	}
}
