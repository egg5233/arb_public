// active_position_checker_test.go — Phase 12 Plan 03 Task 1 (D-05).
//
// Tests for DBActivePositionChecker — the ActivePositionChecker implementation
// that consults pg:positions:active SMEMBERS via *database.Client and matches
// the (Symbol, LongExch, ShortExch, Direction) tuple against persisted
// PriceGapPositions.
//
// IMPORTANT MAPPING NOTE (verified against internal/models/pricegap_position.go):
//
//	models.PriceGapPosition exposes the WIRE-SIDE roles as
//	`LongExchange` / `ShortExchange`, NOT `LongExch` / `ShortExch`.
//
//	The PG-DIR-01 (Phase 999.1) `CandidateLongExch` / `CandidateShortExch`
//	fields stamp the CONFIGURED tuple (which differs from wire-side roles
//	when FiredDirection == "inverse"). Phase 10 D-11 explicitly required the
//	guard to match the CONFIGURED tuple — so we prefer those when present
//	and fall back to LongExchange/ShortExchange for legacy positions.
//
//	Direction is NOT a field on PriceGapPosition (only on PriceGapCandidate).
//	The controller passes `"bidirectional"` (D-18) — auto-promotion only ever
//	creates bidirectional candidates, so we MUST skip the demote whenever an
//	active position exists for the configured tuple regardless of direction.
package pricegaptrader

import (
	"testing"

	"arb/internal/models"
)

// TestDBActivePositionChecker_MatchesConfiguredTuple — the simple case: an
// active position with matching configured-tuple fields blocks the demote.
func TestDBActivePositionChecker_MatchesConfiguredTuple(t *testing.T) {
	db, _ := newE2EClient(t)
	checker := NewDBActivePositionChecker(db)

	// Persist a "forward fire" position — wire-side roles == configured roles.
	pos := &models.PriceGapPosition{
		ID:                 "test-1",
		Symbol:             "BTCUSDT",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		CandidateLongExch:  "binance",
		CandidateShortExch: "bybit",
		FiredDirection:     models.PriceGapFiredForward,
		Status:             models.PriceGapStatusOpen,
	}
	if err := db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	cand := models.PriceGapCandidate{
		Symbol:    "BTCUSDT",
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}
	blocked, err := checker.IsActiveForCandidate(cand)
	if err != nil {
		t.Fatalf("IsActiveForCandidate err=%v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked=true (active position exists for tuple); got false")
	}
}

// TestDBActivePositionChecker_NoMatchOnDifferentExchanges — different short
// exchange does NOT match.
func TestDBActivePositionChecker_NoMatchOnDifferentExchanges(t *testing.T) {
	db, _ := newE2EClient(t)
	checker := NewDBActivePositionChecker(db)

	pos := &models.PriceGapPosition{
		ID:                 "test-2",
		Symbol:             "BTCUSDT",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		CandidateLongExch:  "binance",
		CandidateShortExch: "bybit",
		Status:             models.PriceGapStatusOpen,
	}
	if err := db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	cand := models.PriceGapCandidate{
		Symbol:    "BTCUSDT",
		LongExch:  "binance",
		ShortExch: "gate", // different short
		Direction: models.PriceGapDirectionBidirectional,
	}
	blocked, err := checker.IsActiveForCandidate(cand)
	if err != nil {
		t.Fatalf("IsActiveForCandidate err=%v", err)
	}
	if blocked {
		t.Fatal("expected blocked=false (different short exchange); got true")
	}
}

// TestDBActivePositionChecker_NoMatchOnDifferentSymbol — different symbol does
// NOT match.
func TestDBActivePositionChecker_NoMatchOnDifferentSymbol(t *testing.T) {
	db, _ := newE2EClient(t)
	checker := NewDBActivePositionChecker(db)

	pos := &models.PriceGapPosition{
		ID:                 "test-3",
		Symbol:             "BTCUSDT",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		CandidateLongExch:  "binance",
		CandidateShortExch: "bybit",
		Status:             models.PriceGapStatusOpen,
	}
	if err := db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	cand := models.PriceGapCandidate{
		Symbol:    "ETHUSDT", // different symbol
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}
	blocked, err := checker.IsActiveForCandidate(cand)
	if err != nil {
		t.Fatalf("IsActiveForCandidate err=%v", err)
	}
	if blocked {
		t.Fatal("expected blocked=false (different symbol); got true")
	}
}

// TestDBActivePositionChecker_EmptyActiveSet — no active positions → not
// blocked.
func TestDBActivePositionChecker_EmptyActiveSet(t *testing.T) {
	db, _ := newE2EClient(t)
	checker := NewDBActivePositionChecker(db)

	cand := models.PriceGapCandidate{
		Symbol:    "BTCUSDT",
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}
	blocked, err := checker.IsActiveForCandidate(cand)
	if err != nil {
		t.Fatalf("IsActiveForCandidate err=%v", err)
	}
	if blocked {
		t.Fatal("expected blocked=false (no active positions); got true")
	}
}

// TestDBActivePositionChecker_LegacyPositionUsesWireSideRoles — pre-Phase-999.1
// positions have empty CandidateLongExch/CandidateShortExch fields. The guard
// MUST fall back to LongExchange/ShortExchange for those rows so we don't
// accidentally demote a candidate while a legacy position is open.
func TestDBActivePositionChecker_LegacyPositionUsesWireSideRoles(t *testing.T) {
	db, _ := newE2EClient(t)
	checker := NewDBActivePositionChecker(db)

	pos := &models.PriceGapPosition{
		ID:            "legacy-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		// CandidateLongExch + CandidateShortExch left empty (pre-999.1 row).
		Status: models.PriceGapStatusOpen,
	}
	if err := db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("save position: %v", err)
	}

	cand := models.PriceGapCandidate{
		Symbol:    "BTCUSDT",
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}
	blocked, err := checker.IsActiveForCandidate(cand)
	if err != nil {
		t.Fatalf("IsActiveForCandidate err=%v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked=true (legacy position with matching wire-side roles); got false")
	}
}
