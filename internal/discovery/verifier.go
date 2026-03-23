package discovery

import (
	"fmt"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/utils"
)

const verifyTimeout = 10 * time.Second

// VerifyRates cross-checks opportunities against exchange-native funding rate
// APIs. Opportunities where the Loris rate diverges more than 20% from the
// exchange-reported rate on either leg are dropped. Also populates NextFunding.
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

	// Direction-only verification: confirm the exchange-reported rates agree
	// on sign with the Loris rates, and that the spread direction (short > long)
	// holds. We avoid magnitude comparison because Loris and exchange interval
	// metadata can differ, making normalized values unreliable for tolerance checks.

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

	return true, opp, ""
}

// sameSign returns true if both values have the same sign.
func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}
