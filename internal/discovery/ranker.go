package discovery

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
)

// FeeSchedule holds maker/taker fee rates for an exchange.
// Values are in percentage (e.g. 0.016 means 0.016%).
type FeeSchedule struct {
	Maker float64
	Taker float64
}

var exchangeFees = map[string]FeeSchedule{
	"binance": {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
	"bybit":   {Maker: 0.02 * 0.8, Taker: 0.055 * 0.8}, // 0.016%, 0.044%
	"okx":     {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
	"bitget":  {Maker: 0.02 * 0.8, Taker: 0.06 * 0.8},  // 0.016%, 0.048%
	"gateio":  {Maker: 0.015 * 0.8, Taker: 0.05 * 0.8}, // 0.012%, 0.04%
	"bingx":   {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
}

// getEffectiveFees returns exchange fees with dynamic overrides applied.
func (s *Scanner) getEffectiveFees() map[string]FeeSchedule {
	s.dynamicFeesMu.RLock()
	hasDynamic := len(s.dynamicFees) > 0
	s.dynamicFeesMu.RUnlock()

	if !hasDynamic {
		return exchangeFees
	}
	return s.GetExchangeFees()
}

// exRate holds a normalized funding rate for one exchange/symbol pair.
type exRate struct {
	exchange    string
	rateBpsH    float64 // bps per hour
	intervalHrs float64
}

type rankedPair struct {
	long          exRate
	short         exRate
	spread        float64
	costRatio     float64
	score         float64
	intervalHours float64
}

// RankOpportunities scores and ranks arbitrage opportunities from Loris data.
func (s *Scanner) RankOpportunities(loris *models.LorisResponse) []models.Opportunity {
	now := time.Now().UTC()
	valueOfTimeHours := s.cfg.MinHoldTime.Hours()
	valueOfRatio := s.cfg.MaxCostRatio

	var opportunities []models.Opportunity

	for _, baseSym := range loris.Symbols {
		// Loris uses bare symbols (e.g. "BTC"); exchange contracts use "BTCUSDT".
		symbol := strings.ToUpper(baseSym) + "USDT"

		// Step 1: Collect and normalize rates to bps-per-hour for supported exchanges.
		var rates []exRate
		for _, exch := range s.activeExchanges() {
			exRates, ok := loris.FundingRates[exch]
			if !ok {
				continue
			}
			rawRate, ok := exRates[baseSym]
			if !ok {
				continue
			}

			// Verify the symbol actually exists as a futures contract on this exchange.
			if !s.hasContract(exch, symbol) {
				continue
			}

			// Determine funding interval in hours (default 8h).
			intervalHrs := 8.0
			if intervals, ok := loris.FundingIntervals[exch]; ok {
				if iv, ok := intervals[baseSym]; ok && iv > 0 {
					intervalHrs = iv
				}
			}

			// Loris rates are normalized to 8h equivalent. Divide by 8 to get bps/hour.
			rateBpsH := rawRate / 8.0
			rates = append(rates, exRate{
				exchange:    exch,
				rateBpsH:    rateBpsH,
				intervalHrs: intervalHrs,
			})
		}

		// Need at least 2 exchanges to form a pair.
		if len(rates) < 2 {
			continue
		}

		fees := s.getEffectiveFees()
		pairs := rankPairs(rates, valueOfTimeHours, valueOfRatio, s.cfg.AllowMixedIntervals, fees)
		if len(pairs) == 0 {
			continue
		}

		topPairs := s.cfg.TopPairsPerSymbol
		if topPairs <= 0 {
			topPairs = 1
		}
		if len(pairs) > topPairs {
			pairs = pairs[:topPairs]
		}

		// Step 4: Parse OI ranking.
		oiRank := parseOIRank(loris.OIRankings, loris.DefaultOIRank, baseSym)
		bestPair := pairs[0]

		opp := models.Opportunity{
			Symbol:        symbol,
			LongExchange:  bestPair.long.exchange,
			ShortExchange: bestPair.short.exchange,
			LongRate:      bestPair.long.rateBpsH,
			ShortRate:     bestPair.short.rateBpsH,
			Spread:        bestPair.spread,
			CostRatio:     bestPair.costRatio,
			OIRank:        oiRank,
			Score:         bestPair.spread * (1.0 + 1.0/float64(oiRank)),
			IntervalHours: bestPair.intervalHours,
			Source:        "loris",
			Timestamp:     now,
		}
		if len(pairs) > 1 {
			opp.Alternatives = make([]models.AlternativePair, 0, len(pairs)-1)
			for _, pair := range pairs[1:] {
				opp.Alternatives = append(opp.Alternatives, models.AlternativePair{
					LongExchange:  pair.long.exchange,
					ShortExchange: pair.short.exchange,
					LongRate:      pair.long.rateBpsH,
					ShortRate:     pair.short.rateBpsH,
					Spread:        pair.spread,
					CostRatio:     pair.costRatio,
					Score:         pair.spread * (1.0 + 1.0/float64(oiRank)),
					IntervalHours: pair.intervalHours,
				})
			}
		}
		opportunities = append(opportunities, opp)

		// Save funding snapshot to Redis.
		s.saveFundingSnapshot(symbol, rates, now)
	}

	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].Score > opportunities[j].Score
	})

	topN := s.cfg.TopOpportunities
	if topN <= 0 {
		topN = 5
	}
	if len(opportunities) > topN {
		opportunities = opportunities[:topN]
	}

	return opportunities
}

func rankPairs(rates []exRate, valueOfTimeHours, valueOfRatio float64, allowMixed bool, fees map[string]FeeSchedule) []rankedPair {
	var pairs []rankedPair
	for i := range rates {
		for j := range rates {
			if i == j {
				continue
			}
			long := rates[i]
			short := rates[j]
			if !allowMixed && int(math.Round(long.intervalHrs)) != int(math.Round(short.intervalHrs)) {
				continue
			}
			spread := short.rateBpsH - long.rateBpsH
			if spread <= 0 {
				continue
			}
			costRatio := pairCostRatio(long.exchange, short.exchange, spread, valueOfTimeHours, fees)
			if costRatio >= valueOfRatio {
				continue
			}
			intervalHours := math.Max(long.intervalHrs, short.intervalHrs)
			if intervalHours <= 0 {
				intervalHours = 8
			}
			pairs = append(pairs, rankedPair{
				long:          long,
				short:         short,
				spread:        spread,
				costRatio:     costRatio,
				score:         spread,
				intervalHours: intervalHours,
			})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].costRatio < pairs[j].costRatio
		}
		return pairs[i].score > pairs[j].score
	})
	return pairs
}

func pairCostRatio(longExch, shortExch string, spread, valueOfTimeHours float64, fees map[string]FeeSchedule) float64 {
	feesA := fees[longExch]
	feesB := fees[shortExch]
	totalFeePct := feesA.Taker + feesB.Taker + feesA.Taker + feesB.Taker
	totalFeeBps := totalFeePct * 100
	denominator := spread * valueOfTimeHours
	if denominator <= 0 {
		return math.MaxFloat64
	}
	return totalFeeBps / denominator
}

// parseOIRank extracts the OI rank integer for a symbol from the Loris response.
func parseOIRank(rankings map[string]interface{}, defaultRank string, symbol string) int {
	if rankings == nil {
		return parseRankString(defaultRank)
	}
	val, ok := rankings[symbol]
	if !ok {
		return parseRankString(defaultRank)
	}

	switch v := val.(type) {
	case string:
		return parseRankString(v)
	case float64:
		return int(v)
	default:
		return parseRankString(defaultRank)
	}
}

// parseRankString parses rank strings like "1", "2", "500+".
func parseRankString(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "+")
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 500
	}
	return n
}

// saveFundingSnapshot persists the current funding rates to Redis.
func (s *Scanner) saveFundingSnapshot(symbol string, rates []exRate, ts time.Time) {
	snap := &models.FundingSnapshot{
		Symbol:    symbol,
		Timestamp: ts,
		Rates:     make([]models.FundingRate, 0, len(rates)),
	}
	for _, r := range rates {
		snap.Rates = append(snap.Rates, models.FundingRate{
			Exchange:    r.exchange,
			Symbol:      symbol,
			RateBpsH:    r.rateBpsH,
			Interval:    time.Duration(r.intervalHrs) * time.Hour,
			NextFunding: nextFundingTime(ts, r.intervalHrs),
			UpdatedAt:   ts,
		})
	}
	if err := s.db.SaveFundingSnapshot(snap); err != nil {
		s.log.Error("Failed to save funding snapshot for %s: %v", symbol, err)
	}
}

// nextFundingTime computes the next funding snapshot time by rounding up to the
// next interval boundary.
func nextFundingTime(now time.Time, intervalHrs float64) time.Time {
	if intervalHrs <= 0 {
		return time.Time{}
	}
	intervalSec := int64(intervalHrs * 3600)
	unix := now.Unix()
	next := unix - (unix % intervalSec) + intervalSec
	return time.Unix(next, 0).UTC()
}
