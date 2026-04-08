package discovery

import (
	"fmt"
	"math"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/utils"
)

const verifyTimeout = 10 * time.Second

// VerifyRates cross-checks opportunities against exchange-native funding rate
// APIs. Checks interval agreement, rate magnitude (50% tolerance), spread
// magnitude, sign agreement, and spread direction. Also populates NextFunding.
func (s *Scanner) VerifyRates(opportunities []models.Opportunity) []models.Opportunity {
	type result struct {
		idx    int
		opp    models.Opportunity
		pass   bool
		reason string
	}

	results := make([]result, len(opportunities))
	var wg sync.WaitGroup

	for i, opp := range opportunities {
		wg.Add(1)
		go func(idx int, opp models.Opportunity) {
			defer wg.Done()

			type verifyResult struct {
				opp    models.Opportunity
				pass   bool
				reason string
			}
			done := make(chan verifyResult, 1)
			go func() {
				pass, enriched, reason := s.verifyOpportunity(opp)
				done <- verifyResult{opp: enriched, pass: pass, reason: reason}
			}()

			select {
			case vr := <-done:
				results[idx] = result{idx: idx, opp: vr.opp, pass: vr.pass, reason: vr.reason}
			case <-time.After(verifyTimeout):
				s.log.Warn("Verification timed out for %s (%s/%s)", opp.Symbol, opp.LongExchange, opp.ShortExchange)
				results[idx] = result{idx: idx, opp: opp, pass: false, reason: "verification timed out"}
			}
		}(i, opp)
	}

	wg.Wait()

	var verified []models.Opportunity
	for _, r := range results {
		if r.pass {
			verified = append(verified, r.opp)
		} else if r.reason != "" && s.rejStore != nil {
			s.rejStore.AddOpp(r.opp, "verifier", r.reason)
		}
	}
	return verified
}

// verifyOpportunity checks both legs of an opportunity against exchange APIs.
// Returns (pass, enrichedOpp) — the enriched opp has NextFunding populated.
func (s *Scanner) verifyOpportunity(opp models.Opportunity) (bool, models.Opportunity, string) {
	longExch, longOK := s.exchanges[opp.LongExchange]
	shortExch, shortOK := s.exchanges[opp.ShortExchange]
	if !longOK || !shortOK {
		s.log.Warn("Exchange adapter not found for %s or %s", opp.LongExchange, opp.ShortExchange)
		return false, opp, "exchange adapter not found"
	}

	// Fetch both rates in parallel.
	type rateResult struct {
		rate        *float64
		intervalHrs float64
		maxRate     *float64 // per-period decimal cap from exchange (nil if unavailable)
		minRate     *float64 // per-period decimal floor from exchange (nil if unavailable)
		nextFunding time.Time
		err         error
	}

	var wg sync.WaitGroup
	var longRes, shortRes rateResult

	wg.Add(2)
	go func() {
		defer wg.Done()
		fr, err := longExch.GetFundingRate(opp.Symbol)
		if err != nil {
			longRes.err = err
			return
		}
		longRes.nextFunding = fr.NextFunding
		// Convert exchange rate (decimal) to bps/hour.
		intervalHrs := fr.Interval.Hours()
		if intervalHrs <= 0 {
			intervalHrs = 8
		}
		longRes.intervalHrs = intervalHrs
		longRes.maxRate = fr.MaxRate
		longRes.minRate = fr.MinRate
		rateBpsH := utils.RateToBpsPerHour(fr.Rate*10000, intervalHrs)
		longRes.rate = &rateBpsH
	}()

	go func() {
		defer wg.Done()
		fr, err := shortExch.GetFundingRate(opp.Symbol)
		if err != nil {
			shortRes.err = err
			return
		}
		shortRes.nextFunding = fr.NextFunding
		intervalHrs := fr.Interval.Hours()
		if intervalHrs <= 0 {
			intervalHrs = 8
		}
		shortRes.intervalHrs = intervalHrs
		shortRes.maxRate = fr.MaxRate
		shortRes.minRate = fr.MinRate
		rateBpsH := utils.RateToBpsPerHour(fr.Rate*10000, intervalHrs)
		shortRes.rate = &rateBpsH
	}()

	wg.Wait()

	// If either fetch failed, skip verification but allow the opportunity.
	if longRes.err != nil {
		s.log.Warn("Failed to verify long rate for %s on %s: %v", opp.Symbol, opp.LongExchange, longRes.err)
		return false, opp, fmt.Sprintf("long rate fetch failed: %v", longRes.err)
	}
	if shortRes.err != nil {
		s.log.Warn("Failed to verify short rate for %s on %s: %v", opp.Symbol, opp.ShortExchange, shortRes.err)
		return false, opp, fmt.Sprintf("short rate fetch failed: %v", shortRes.err)
	}

	// Populate NextFunding using the collecting side's settlement time.
	// For mixed-interval pairs (e.g. 1h vs 4h), entering when only the
	// paying side settles means losing money until the collecting side settles.
	if !longRes.nextFunding.IsZero() && !shortRes.nextFunding.IsZero() {
		if *longRes.rate < 0 && *shortRes.rate < 0 {
			// Negative/negative: shorts pay longs → long exchange collects
			opp.NextFunding = longRes.nextFunding
		} else if *longRes.rate >= 0 && *shortRes.rate >= 0 {
			// Positive/positive: longs pay shorts → short exchange collects
			opp.NextFunding = shortRes.nextFunding
		} else {
			// Mixed signs: both sides collect → use earliest (either is beneficial)
			if longRes.nextFunding.Before(shortRes.nextFunding) {
				opp.NextFunding = longRes.nextFunding
			} else {
				opp.NextFunding = shortRes.nextFunding
			}
		}
	} else if !longRes.nextFunding.IsZero() {
		opp.NextFunding = longRes.nextFunding
	} else if !shortRes.nextFunding.IsZero() {
		opp.NextFunding = shortRes.nextFunding
	}

	// --- Interval and magnitude checks (skip for CoinGlass — rates are zero placeholders) ---
	if opp.Source != "coinglass" {
		// Check 1: Per-leg interval agreement.
		// Loris interval must match exchange interval; mismatches (e.g. 4h→1h change)
		// cause rate normalization errors that inflate bps/h values.
		lorisLongInterval := s.getLegInterval(opp.LongExchange, opp.Symbol)
		lorisShortInterval := s.getLegInterval(opp.ShortExchange, opp.Symbol)

		if longRes.intervalHrs > 0 && lorisLongInterval > 0 &&
			math.Abs(lorisLongInterval-longRes.intervalHrs) > 0.5 {
			s.log.Debug("Long interval mismatch for %s on %s: loris=%.0fh exchange=%.0fh",
				opp.Symbol, opp.LongExchange, lorisLongInterval, longRes.intervalHrs)
			return false, opp, fmt.Sprintf("long interval mismatch: loris=%.0fh exchange=%.0fh",
				lorisLongInterval, longRes.intervalHrs)
		}
		if shortRes.intervalHrs > 0 && lorisShortInterval > 0 &&
			math.Abs(lorisShortInterval-shortRes.intervalHrs) > 0.5 {
			s.log.Debug("Short interval mismatch for %s on %s: loris=%.0fh exchange=%.0fh",
				opp.Symbol, opp.ShortExchange, lorisShortInterval, shortRes.intervalHrs)
			return false, opp, fmt.Sprintf("short interval mismatch: loris=%.0fh exchange=%.0fh",
				lorisShortInterval, shortRes.intervalHrs)
		}

		// Check 2: Per-leg rate magnitude (50% tolerance).
		if !rateMagnitudeOK(opp.LongRate, *longRes.rate, 0.50) {
			s.log.Debug("Long rate magnitude divergence for %s on %s: loris=%.2f exchange=%.2f bps/h",
				opp.Symbol, opp.LongExchange, opp.LongRate, *longRes.rate)
			return false, opp, fmt.Sprintf("long rate magnitude divergence: loris=%.2f exchange=%.2f bps/h",
				opp.LongRate, *longRes.rate)
		}
		if !rateMagnitudeOK(opp.ShortRate, *shortRes.rate, 0.50) {
			s.log.Debug("Short rate magnitude divergence for %s on %s: loris=%.2f exchange=%.2f bps/h",
				opp.Symbol, opp.ShortExchange, opp.ShortRate, *shortRes.rate)
			return false, opp, fmt.Sprintf("short rate magnitude divergence: loris=%.2f exchange=%.2f bps/h",
				opp.ShortRate, *shortRes.rate)
		}

		// Check 3: Spread-level magnitude (50% tolerance).
		// Two legs each within tolerance can still produce an overstated net spread.
		exchangeSpread := *shortRes.rate - *longRes.rate
		if !rateMagnitudeOK(opp.Spread, exchangeSpread, 0.50) {
			s.log.Debug("Spread magnitude divergence for %s: loris=%.2f exchange=%.2f bps/h",
				opp.Symbol, opp.Spread, exchangeSpread)
			return false, opp, fmt.Sprintf("spread magnitude divergence: loris=%.2f exchange=%.2f bps/h",
				opp.Spread, exchangeSpread)
		}

		// Check 4: Funding rate cap validation.
		// Reject if Loris rate exceeds exchange's own caps (or default ±150 bps/h).
		const defaultCapBpsH = 150.0 // 1.5%/h fallback when exchange doesn't provide caps

		longMaxBpsH, longMinBpsH := defaultCapBpsH, -defaultCapBpsH
		if longRes.maxRate != nil && longRes.minRate != nil && longRes.intervalHrs > 0 {
			longMaxBpsH = utils.RateToBpsPerHour(*longRes.maxRate*10000, longRes.intervalHrs)
			longMinBpsH = utils.RateToBpsPerHour(*longRes.minRate*10000, longRes.intervalHrs)
		}
		if opp.LongRate > longMaxBpsH || opp.LongRate < longMinBpsH {
			s.log.Debug("Long rate exceeds cap for %s on %s: loris=%.2f bps/h cap=[%.2f, %.2f]",
				opp.Symbol, opp.LongExchange, opp.LongRate, longMinBpsH, longMaxBpsH)
			return false, opp, fmt.Sprintf("long rate exceeds cap: loris=%.2f bps/h cap=[%.2f, %.2f]",
				opp.LongRate, longMinBpsH, longMaxBpsH)
		}

		shortMaxBpsH, shortMinBpsH := defaultCapBpsH, -defaultCapBpsH
		if shortRes.maxRate != nil && shortRes.minRate != nil && shortRes.intervalHrs > 0 {
			shortMaxBpsH = utils.RateToBpsPerHour(*shortRes.maxRate*10000, shortRes.intervalHrs)
			shortMinBpsH = utils.RateToBpsPerHour(*shortRes.minRate*10000, shortRes.intervalHrs)
		}
		if opp.ShortRate > shortMaxBpsH || opp.ShortRate < shortMinBpsH {
			s.log.Debug("Short rate exceeds cap for %s on %s: loris=%.2f bps/h cap=[%.2f, %.2f]",
				opp.Symbol, opp.ShortExchange, opp.ShortRate, shortMinBpsH, shortMaxBpsH)
			return false, opp, fmt.Sprintf("short rate exceeds cap: loris=%.2f bps/h cap=[%.2f, %.2f]",
				opp.ShortRate, shortMinBpsH, shortMaxBpsH)
		}
	}

	// Check sign agreement on long leg (skip if either is near zero).
	if opp.LongRate != 0 && *longRes.rate != 0 && !sameSign(opp.LongRate, *longRes.rate) {
		s.log.Debug("Long rate sign mismatch for %s on %s: loris=%.4f exchange=%.4f",
			opp.Symbol, opp.LongExchange, opp.LongRate, *longRes.rate)
		return false, opp, fmt.Sprintf("long rate sign mismatch: loris=%.4f exchange=%.4f", opp.LongRate, *longRes.rate)
	}

	// Check sign agreement on short leg.
	if opp.ShortRate != 0 && *shortRes.rate != 0 && !sameSign(opp.ShortRate, *shortRes.rate) {
		s.log.Debug("Short rate sign mismatch for %s on %s: loris=%.4f exchange=%.4f",
			opp.Symbol, opp.ShortExchange, opp.ShortRate, *shortRes.rate)
		return false, opp, fmt.Sprintf("short rate sign mismatch: loris=%.4f exchange=%.4f", opp.ShortRate, *shortRes.rate)
	}

	// Verify spread direction: exchange short rate > exchange long rate.
	if *shortRes.rate <= *longRes.rate {
		s.log.Debug("Spread direction mismatch for %s: long(%s)=%.4f short(%s)=%.4f",
			opp.Symbol, opp.LongExchange, *longRes.rate, opp.ShortExchange, *shortRes.rate)
		return false, opp, fmt.Sprintf("spread direction mismatch: long=%.4f short=%.4f", *longRes.rate, *shortRes.rate)
	}

	// Verify alternatives (only when the primary pair passes).
	// Build a temporary Opportunity per alt and recursively call verifyOpportunity.
	// Empty Alternatives to avoid infinite recursion.
	if len(opp.Alternatives) > 0 {
		var verifiedAlts []models.AlternativePair
		for _, alt := range opp.Alternatives {
			altOpp := models.Opportunity{
				Symbol:        opp.Symbol,
				LongExchange:  alt.LongExchange,
				ShortExchange: alt.ShortExchange,
				LongRate:      alt.LongRate,
				ShortRate:     alt.ShortRate,
				Spread:        alt.Spread, // needed for spread-magnitude check
				IntervalHours: alt.IntervalHours,
				OIRank:        opp.OIRank, // inherit for isPersistent() low-OI gate
				Source:        opp.Source,
				// Alternatives intentionally left nil to prevent recursion
			}
			pass, enriched, _ := s.verifyOpportunity(altOpp)
			if pass {
				alt.NextFunding = enriched.NextFunding
				alt.Verified = true
				verifiedAlts = append(verifiedAlts, alt)
			}
		}
		opp.Alternatives = verifiedAlts
	}

	return true, opp, ""
}

// sameSign returns true if both values have the same sign.
func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}

// rateMagnitudeOK checks if two rates are within tolerance of each other.
// Uses absolute difference for small rates (< 5 bps/h) to avoid noise from
// near-zero divisions, and ratio-based comparison for larger rates.
func rateMagnitudeOK(loris, exchange, tolerance float64) bool {
	// Both near zero — allow small absolute difference.
	if math.Abs(exchange) < 5 && math.Abs(loris) < 5 {
		return math.Abs(loris-exchange) < 5
	}
	// Exchange ~0 but Loris is not — check if Loris is small too.
	if math.Abs(exchange) < 0.001 {
		return math.Abs(loris) < 5
	}
	ratio := math.Abs(loris / exchange)
	return ratio >= (1-tolerance) && ratio <= (1+tolerance)
}
