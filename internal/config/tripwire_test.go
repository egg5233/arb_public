package config

import (
	"testing"
)

func TestKeepNonZero_PreservesDiskValueWhenNewIsZero(t *testing.T) {
	// Simulates the live incident: on-disk had capital_unified_usdt=200, some
	// path tries to write 0. Tripwire should return the disk value.
	got := keepNonZero(float64(200), 0, "spot_futures.capital_unified_usdt")
	if got != float64(200) {
		t.Errorf("expected disk value 200 preserved, got %v", got)
	}
	gotInt := keepNonZero(float64(1), 0, "spot_futures.max_positions")
	if gotInt != float64(1) {
		t.Errorf("expected disk int 1 preserved, got %v", gotInt)
	}
}

func TestKeepNonZero_AllowsZeroWhenDiskWasZero(t *testing.T) {
	// If disk was 0 too, zero is the real value. No tripwire fire.
	got := keepNonZero(float64(0), 0, "spot_futures.capital_unified_usdt")
	if got != 0 {
		t.Errorf("expected 0 to pass through when disk was 0, got %v", got)
	}
}

func TestKeepNonZero_AllowsZeroWhenDiskWasMissing(t *testing.T) {
	// json.Unmarshal leaves missing keys as the nil interface{}. Zero passes.
	got := keepNonZero(nil, 0, "spot_futures.max_positions")
	if got != 0 {
		t.Errorf("expected 0 to pass through when disk was nil, got %v", got)
	}
}

func TestKeepNonZero_PassesThroughNonZeroWrites(t *testing.T) {
	// Normal updates: disk=200, new=500. New wins.
	got := keepNonZero(float64(200), float64(500), "spot_futures.capital_unified_usdt")
	if got != float64(500) {
		t.Errorf("expected new value 500, got %v", got)
	}
	gotUpgrade := keepNonZero(float64(1), 2, "spot_futures.max_positions")
	if gotUpgrade != 2 {
		t.Errorf("expected new value 2, got %v", gotUpgrade)
	}
}

func TestKeepNonZero_HandlesMixedNumericTypes(t *testing.T) {
	// json.Unmarshal gives float64, but in-memory config fields can be int.
	// The helper must handle both on either side.
	for _, tc := range []struct {
		name     string
		existing interface{}
		newVal   interface{}
		want     interface{}
	}{
		{"float-disk-int-zero", float64(200), 0, float64(200)},
		{"float-disk-int64-zero", float64(200), int64(0), float64(200)},
		{"float-disk-float64-zero", float64(200), float64(0), float64(200)},
		{"int-disk-zero-attempt", 3, 0, 3},
		{"non-numeric-disk-passes", "not-a-number", 0, 0}, // not comparable → pass
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := keepNonZero(tc.existing, tc.newVal, "test")
			if got != tc.want {
				t.Errorf("existing=%v newVal=%v: got %v, want %v", tc.existing, tc.newVal, got, tc.want)
			}
		})
	}
}
