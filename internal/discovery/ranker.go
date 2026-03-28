package discovery

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
)

// Fee schedules for each exchange (v0 with 20% rebate).
// Values are in percentage (e.g. 0.016 means 0.016%).
type feeSchedule struct {
	Maker float64
	Taker float64
}

var exchangeFees = map[string]feeSchedule{
	"binance": {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
	"bybit":   {Maker: 0.02 * 0.8, Taker: 0.055 * 0.8}, // 0.016%, 0.044%
	"okx":     {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
	"bitget":  {Maker: 0.02 * 0.8, Taker: 0.06 * 0.8},  // 0.016%, 0.048%
	"gateio":  {Maker: 0.015 * 0.8, Taker: 0.05 * 0.8}, // 0.012%, 0.04%
	"bingx":   {Maker: 0.02 * 0.8, Taker: 0.05 * 0.8},  // 0.016%, 0.04%
}

// exRate holds a normalized funding rate for one exchange/symbol pair.
type exRate struct {
	exchange    string
	rateBpsH    float64 // bps per hour
	intervalHrs float64
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

			// Loris rates are normalized to 8h equivalent (bps_per_period × 8/interval).
			// Divide by 8 to get bps/hour.
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

		// Step 2: Find best long/short pair.
		var bestLong, bestShort exRate
		var spread float64

		if !s.cfg.AllowMixedIntervals {
			// Group by interval, find best pair within each group,
			// then pick the group with the widest spread.
			// Prevents cross-interval pairs (e.g. 1h vs 4h).
			intervalGroups := make(map[int][]exRate) // key = interval rounded to nearest hour
			for _, r := range rates {
				key := int(math.Round(r.intervalHrs))
				intervalGroups[key] = append(intervalGroups[key], r)
			}
			for _, group := range intervalGroups {
				if len(group) < 2 {
					continue
				}
				gLong, gShort := group[0], group[0]
				for _, r := range group[1:] {
					if r.rateBpsH < gLong.rateBpsH {
						gLong = r
					}
					if r.rateBpsH > gShort.rateBpsH {
						gShort = r
					}
				}
				if gLong.exchange == gShort.exchange {
					continue
				}
				gSpread := gShort.rateBpsH - gLong.rateBpsH
				if gSpread > spread {
					bestLong, bestShort, spread = gLong, gShort, gSpread
				}
			}
		} else {
			// Allow mixed intervals — pick global best long/short.
			// The interval monitor protects live positions by checking spread.
			bestLong = rates[0]
			bestShort = rates[0]
			for _, r := range rates[1:] {
				if r.rateBpsH < bestLong.rateBpsH {
					bestLong = r
				}
				if r.rateBpsH > bestShort.rateBpsH {
					bestShort = r
				}
			}
			if bestLong.exchange != bestShort.exchange {
				spread = bestShort.rateBpsH - bestLong.rateBpsH
			}
		}

		if spread <= 0 {
			continue
		}

		// Step 3: Filter by profitability.
		feesA := exchangeFees[bestLong.exchange]
		feesB := exchangeFees[bestShort.exchange]

		// Entry: taker on both legs (simultaneous IOC). Exit: taker on both legs.
		totalFeePct := feesA.Taker + feesB.Taker + feesA.Taker + feesB.Taker
		totalFeeBps := totalFeePct * 100 // convert percentage to bps

		maxInterval := math.Max(bestLong.intervalHrs, bestShort.intervalHrs)
		if maxInterval <= 0 {
			maxInterval = 8
		}

		// spread is bps/h, so total earned = spread * valueOfTimeHours
		var costRatio float64
		denominator := spread * valueOfTimeHours
		if denominator > 0 {
			costRatio = totalFeeBps / denominator
		} else {
			continue
		}

		if costRatio >= valueOfRatio {
			continue
		}

		// Step 4: Parse OI ranking.
		oiRank := parseOIRank(loris.OIRankings, loris.DefaultOIRank, baseSym)

		// Composite score: spread * (1 + 1/oi_rank).
		score := spread * (1.0 + 1.0/float64(oiRank))

		opp := models.Opportunity{
			Symbol:        symbol,
			LongExchange:  bestLong.exchange,
			ShortExchange: bestShort.exchange,
			LongRate:      bestLong.rateBpsH,
			ShortRate:     bestShort.rateBpsH,
			Spread:        spread, // bps/h
			CostRatio:     costRatio,
			OIRank:        oiRank,
			Score:         score,
			IntervalHours: maxInterval,
			Source:        "loris",
			Timestamp:     now,
		}
		opportunities = append(opportunities, opp)

		// Step 6: Save funding snapshot to Redis.
		s.saveFundingSnapshot(symbol, rates, now)
	}

	// Step 5: Sort by score descending and return top N.
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

// nextFundingTime computes the next funding snapshot time by rounding up
// to the next interval boundary (e.g. 1h→next whole hour, 4h→00/04/08/12/16/20,
// 8h→00/08/16 UTC).
func nextFundingTime(now time.Time, intervalHrs float64) time.Time {
	if intervalHrs <= 0 {
		return time.Time{}
	}
	intervalSec := int64(intervalHrs * 3600)
	unix := now.Unix()
	next := unix - (unix % intervalSec) + intervalSec
	return time.Unix(next, 0).UTC()
}
