package models

import (
	"encoding/json"
	"testing"
)

// TestPriceGapPosition_Mode_RoundTrip — Mode="paper" survives JSON marshal/unmarshal.
func TestPriceGapPosition_Mode_RoundTrip(t *testing.T) {
	orig := &PriceGapPosition{
		ID:     "pg_TEST_x_y_1",
		Symbol: "TESTUSDT",
		Status: PriceGapStatusOpen,
		Mode:   PriceGapModePaper,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PriceGapPosition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Mode != PriceGapModePaper {
		t.Fatalf("Mode round-trip: got %q, want %q", got.Mode, PriceGapModePaper)
	}
}

// TestPriceGapPosition_Mode_DefaultsLiveOnLegacyRecord — unmarshaling a pre-Phase-9
// record (no "mode" key) followed by NormalizeMode must yield Mode="live".
func TestPriceGapPosition_Mode_DefaultsLiveOnLegacyRecord(t *testing.T) {
	legacy := []byte(`{"id":"legacy","symbol":"SOONUSDT","status":"open"}`)
	var got PriceGapPosition
	if err := json.Unmarshal(legacy, &got); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if got.Mode != "" {
		t.Fatalf("pre-normalize Mode: got %q, want empty", got.Mode)
	}
	NormalizeMode(&got)
	if got.Mode != PriceGapModeLive {
		t.Fatalf("post-normalize Mode: got %q, want %q", got.Mode, PriceGapModeLive)
	}
}

// TestPriceGapPosition_Mode_NormalizePreservesExplicitPaper — an explicit Mode="paper"
// must NOT be overwritten by NormalizeMode.
func TestPriceGapPosition_Mode_NormalizePreservesExplicitPaper(t *testing.T) {
	p := &PriceGapPosition{Mode: PriceGapModePaper}
	NormalizeMode(p)
	if p.Mode != PriceGapModePaper {
		t.Fatalf("NormalizeMode clobbered paper: got %q", p.Mode)
	}
}

// TestPriceGapPosition_Mode_NilSafe — NormalizeMode on nil must not panic.
func TestPriceGapPosition_Mode_NilSafe(t *testing.T) {
	NormalizeMode(nil) // must not panic
}
