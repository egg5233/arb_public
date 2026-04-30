// Package pricegaptrader — Phase 12 ActivePositionChecker (D-05).
//
// DBActivePositionChecker reads pg:positions:active SMEMBERS via *database.Client
// and returns true when an active PriceGapPosition matches a candidate's
// (Symbol, LongExch, ShortExch) tuple. Direction is NOT a field on
// PriceGapPosition (only on PriceGapCandidate); auto-promotion only ever
// creates bidirectional candidates per D-18, and we MUST treat ANY active
// position on the configured tuple as a block — direction-mismatch on the
// position side is impossible because PriceGapPosition has no Direction field.
//
// Field-name reality (verified internal/models/pricegap_position.go):
//   - models.PriceGapPosition.LongExchange / ShortExchange — wire-side roles
//     (may swap from configured tuple under PG-DIR-01 inverse fires)
//   - models.PriceGapPosition.CandidateLongExch / CandidateShortExch —
//     CONFIGURED tuple stamped at fire time (Phase 999.1; empty on legacy
//     pre-Phase-999.1 positions)
//
// Phase 10 D-11 explicitly required the active-position guard to match the
// CONFIGURED tuple (NOT the wire-side roles), because a bidirectional fire
// can flip wire-side roles while the candidate identity stays fixed. We use
// CandidateLongExch/CandidateShortExch when present and fall back to
// LongExchange/ShortExchange for legacy positions (Pitfall 5).
//
// Fail-safe behavior (D-05): on Redis read error we return (true, err) so the
// PromotionController treats the candidate as blocked — a transient Redis
// hiccup MUST NOT cause a candidate to be auto-demoted while a position may
// still be open. The controller HOLDS the demote streak at threshold and
// retries on the next cycle.
package pricegaptrader

import (
	"arb/internal/database"
	"arb/internal/models"
)

// DBActivePositionChecker is the production ActivePositionChecker
// implementation backed by *database.Client.
type DBActivePositionChecker struct {
	db *database.Client
}

// NewDBActivePositionChecker constructs the checker. db must be non-nil.
func NewDBActivePositionChecker(db *database.Client) *DBActivePositionChecker {
	return &DBActivePositionChecker{db: db}
}

// IsActiveForCandidate consults pg:positions:active and reports whether any
// active position matches the candidate's (Symbol, LongExch, ShortExch)
// tuple — preferring the CONFIGURED tuple stamped on each position
// (Phase 999.1) and falling back to wire-side roles for legacy positions.
//
// Returns (true, nil) on match (controller skips demote, HOLDS streak).
// Returns (false, nil) on no match (controller proceeds with demote).
// Returns (true, err) on Redis read error (D-05 fail-safe — treat as blocked
// so a transient hiccup does NOT delete a candidate that may have an open
// position; controller HOLDS the demote streak and retries next cycle).
func (g *DBActivePositionChecker) IsActiveForCandidate(c models.PriceGapCandidate) (bool, error) {
	positions, err := g.db.GetActivePriceGapPositions()
	if err != nil {
		return true, err
	}
	for _, p := range positions {
		if p == nil {
			continue
		}
		if p.Symbol != c.Symbol {
			continue
		}
		// Prefer the CONFIGURED tuple (PG-DIR-01) when stamped; fall back to
		// wire-side roles for pre-Phase-999.1 legacy positions.
		longExch := p.CandidateLongExch
		shortExch := p.CandidateShortExch
		if longExch == "" && shortExch == "" {
			longExch = p.LongExchange
			shortExch = p.ShortExchange
		}
		if longExch == c.LongExch && shortExch == c.ShortExch {
			return true, nil
		}
	}
	return false, nil
}
