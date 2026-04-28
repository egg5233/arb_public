// Package pricegaptrader — validate.go: shared PriceGapCandidate validator.
//
// Plan 11-03 / T-11-19 pulls the candidate validation surface out of
// internal/api/pricegap_handlers.go so the dashboard handler AND cmd/pg-admin
// (which gains its own candidate-mutation surface in Plan 11-03 Task 2)
// share ONE validator. Drift between the two would let pg-admin write
// candidates that the dashboard would reject — same on-disk file, two
// gatekeepers, contract mismatch. Sharing one function closes that hole.
//
// Caller is expected to have already normalised Symbol/LongExch/ShortExch
// case + whitespace per Pitfall 3 so the validator + storage agree on the
// canonical form. The handler/CLI layer owns normalisation; this validator
// is pure (no mutation).
package pricegaptrader

import (
	"fmt"
	"regexp"

	"arb/internal/models"
)

// canonicalSymbolRE matches the internal canonical symbol format (D-05):
// uppercase alphanumerics ending in USDT.
var canonicalSymbolRE = regexp.MustCompile(`^[A-Z0-9]+USDT$`)

// allowedExch is the closed enum of exchange identifiers the price-gap engine
// supports today (D-06). Phase 10 ships paper-only against the same 6 CEXes
// the rest of the platform talks to.
var allowedExch = map[string]struct{}{
	"binance": {},
	"bybit":   {},
	"gateio":  {},
	"bitget":  {},
	"okx":     {},
	"bingx":   {},
}

// ValidateCandidates enforces D-05..D-11 invariants on a slice of
// PriceGapCandidate. Returns a slice of human-readable error strings, one
// (or more) per failed candidate. Errors are collated rather than fail-fast
// so the dashboard / CLI can surface every issue at once.
//
// Returns nil if the slice is valid (including the empty-slice delete-all
// case — Pitfall 2).
func ValidateCandidates(cs []models.PriceGapCandidate) []string {
	var errs []string
	seen := make(map[string]struct{}, len(cs))
	for i, c := range cs {
		prefix := fmt.Sprintf("candidate[%d] %s/%s/%s", i, c.Symbol, c.LongExch, c.ShortExch)
		if !canonicalSymbolRE.MatchString(c.Symbol) {
			errs = append(errs, prefix+": symbol must match ^[A-Z0-9]+USDT$")
		}
		if _, ok := allowedExch[c.LongExch]; !ok {
			errs = append(errs, prefix+": long_exch invalid")
		}
		if _, ok := allowedExch[c.ShortExch]; !ok {
			errs = append(errs, prefix+": short_exch invalid")
		}
		if c.LongExch == c.ShortExch {
			errs = append(errs, prefix+": long_exch must differ from short_exch")
		}
		if c.ThresholdBps <= 0 || c.ThresholdBps > 10000 {
			errs = append(errs, prefix+": threshold_bps out of range (0,10000]")
		}
		if c.MaxPositionUSDT <= 0 || c.MaxPositionUSDT > 500000 {
			errs = append(errs, prefix+": max_position_usdt out of range (0,500000]")
		}
		if c.ModeledSlippageBps < 0 || c.ModeledSlippageBps > 1000 {
			errs = append(errs, prefix+": modeled_slippage_bps out of range [0,1000]")
		}
		// PG-DIR-01: validate optional direction field. Empty/missing
		// defaults to "pinned" via models.NormalizeDirection in the caller —
		// keep validator pure (no mutation). Unknown values rejected with
		// explicit per-symbol error.
		switch c.Direction {
		case "", models.PriceGapDirectionPinned, models.PriceGapDirectionBidirectional:
			// accepted
		default:
			errs = append(errs, fmt.Sprintf(
				"candidate[%d] %s: direction must be \"pinned\" or \"bidirectional\" (got %q)",
				i, c.Symbol, c.Direction))
		}
		key := c.Symbol + "|" + c.LongExch + "|" + c.ShortExch
		if _, dup := seen[key]; dup {
			errs = append(errs, prefix+": duplicate tuple")
		}
		seen[key] = struct{}{}
	}
	return errs
}
