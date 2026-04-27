package models

import "testing"

// TestPriceGapCandidate_NormalizeDirection_Default — empty Direction defaults
// to "pinned" so pre-Phase-999.1 candidates persisted without the field
// continue to fire only on configured-direction sign continuity.
func TestPriceGapCandidate_NormalizeDirection_Default(t *testing.T) {
	c := PriceGapCandidate{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit"}
	NormalizeDirection(&c)
	if c.Direction != PriceGapDirectionPinned {
		t.Fatalf("got %q want %q", c.Direction, PriceGapDirectionPinned)
	}
}

// TestPriceGapCandidate_NormalizeDirection_PreservesExplicit — an explicit
// Direction must NOT be clobbered by NormalizeDirection.
func TestPriceGapCandidate_NormalizeDirection_PreservesExplicit(t *testing.T) {
	c := PriceGapCandidate{Direction: PriceGapDirectionBidirectional}
	NormalizeDirection(&c)
	if c.Direction != PriceGapDirectionBidirectional {
		t.Fatalf("clobbered explicit bidirectional: got %q", c.Direction)
	}
}

// TestPriceGapCandidate_NormalizeDirection_NilSafe — nil receiver must not panic.
func TestPriceGapCandidate_NormalizeDirection_NilSafe(t *testing.T) {
	NormalizeDirection(nil) // must not panic
}

// TestPriceGapCandidate_ID_DirectionNotInKey — Direction is a behavior property,
// NOT identity. Two candidates with the same tuple but different Direction
// values produce the same ID() (Phase 10 D-11 invariant).
func TestPriceGapCandidate_ID_DirectionNotInKey(t *testing.T) {
	a := PriceGapCandidate{Symbol: "X", LongExch: "a", ShortExch: "b", Direction: PriceGapDirectionPinned}
	b := PriceGapCandidate{Symbol: "X", LongExch: "a", ShortExch: "b", Direction: PriceGapDirectionBidirectional}
	if a.ID() != b.ID() {
		t.Fatalf("Direction must not affect ID(): a=%q b=%q", a.ID(), b.ID())
	}
}
