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

// TestPriceGapPosition_LegacyDecode — pre-Phase-999.1 records (no PG-DIR-01
// fields) decode cleanly with zero-value FiredDirection/CandidateLongExch/
// CandidateShortExch. Close path is safe because monitor.closePair reads
// pos.LongExchange/pos.ShortExchange face-value, never branches on the new
// fields (A2 verified — no re-derivation).
func TestPriceGapPosition_LegacyDecode(t *testing.T) {
	legacy := []byte(`{"id":"old","symbol":"BTCUSDT","long_exchange":"binance","short_exchange":"bybit","status":"closed"}`)
	var p PriceGapPosition
	if err := json.Unmarshal(legacy, &p); err != nil {
		t.Fatalf("legacy unmarshal: %v", err)
	}
	if p.FiredDirection != "" {
		t.Fatalf("expected empty FiredDirection on legacy decode, got %q", p.FiredDirection)
	}
	if p.CandidateLongExch != "" {
		t.Fatalf("expected empty CandidateLongExch on legacy decode, got %q", p.CandidateLongExch)
	}
	if p.CandidateShortExch != "" {
		t.Fatalf("expected empty CandidateShortExch on legacy decode, got %q", p.CandidateShortExch)
	}
	// Wire-side roles must still decode face-value.
	if p.LongExchange != "binance" || p.ShortExchange != "bybit" {
		t.Fatalf("wire-side roles wrong: long=%q short=%q", p.LongExchange, p.ShortExchange)
	}
}

// TestPriceGapPosition_DirectionFields_RoundTrip — bidirectional inverse fire
// records both wire-side roles AND configured tuple. JSON round-trip preserves
// the divergence (LongExchange != CandidateLongExch when fired inverse).
func TestPriceGapPosition_DirectionFields_RoundTrip(t *testing.T) {
	orig := &PriceGapPosition{
		ID:                 "pg_X_a_b_1",
		Symbol:             "X",
		LongExchange:       "bybit",   // wire-side (swapped from configured)
		ShortExchange:      "binance", // wire-side (swapped from configured)
		CandidateLongExch:  "binance", // configured
		CandidateShortExch: "bybit",   // configured
		FiredDirection:     PriceGapFiredInverse,
		Status:             PriceGapStatusOpen,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PriceGapPosition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.FiredDirection != PriceGapFiredInverse {
		t.Fatalf("FiredDirection round-trip: got %q want %q", got.FiredDirection, PriceGapFiredInverse)
	}
	if got.CandidateLongExch != "binance" || got.CandidateShortExch != "bybit" {
		t.Fatalf("configured tuple lost: long=%q short=%q", got.CandidateLongExch, got.CandidateShortExch)
	}
	if got.LongExchange != "bybit" || got.ShortExchange != "binance" {
		t.Fatalf("wire-side roles lost: long=%q short=%q", got.LongExchange, got.ShortExchange)
	}
}
