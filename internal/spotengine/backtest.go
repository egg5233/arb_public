package spotengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"arb/internal/discovery"
)

var (
	spotBacktestCacheTTL         = 24 * time.Hour
	spotBacktestRefreshInterval  = 4 * time.Hour
	spotBacktestPrefetchMinDelay = 3 * time.Second
	spotBacktestPrefetchMaxDelay = 30 * time.Second
)

// spotBacktestResult caches the Dir B funding sum in Redis.
// Pass/fail is re-evaluated on each read using the current SpotFuturesBacktestMinProfit.
type spotBacktestResult struct {
	SumBps      float64 `json:"sum_bps"`
	Settlements int     `json:"settlements"`
	Coverage    float64 `json:"coverage"` // ratio of observed vs expected settlements
	FetchedAt   string  `json:"fetched_at,omitempty"`
}

func (r spotBacktestResult) isStale(now time.Time) bool {
	if r.FetchedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, r.FetchedAt)
	if err != nil {
		return true
	}
	return now.Sub(t) >= spotBacktestRefreshInterval
}

// SpotBacktestDayBreakdown is a single settlement entry in the on-demand report.
type SpotBacktestDayBreakdown struct {
	Timestamp string  `json:"timestamp"`
	FundingY  float64 `json:"funding_y"` // raw bps from Loris
}

// SpotBacktestReport is the payload returned by RunSpotBacktestOnDemand.
type SpotBacktestReport struct {
	Symbol      string                     `json:"symbol"`
	Exchange    string                     `json:"exchange"`
	Direction   string                     `json:"direction"`
	Days        int                        `json:"days"`
	SumBps      float64                    `json:"sum_bps"`
	NetAPR      float64                    `json:"net_apr"`
	Settlements int                        `json:"settlements"`
	Coverage    float64                    `json:"coverage"`
	Breakdown   []SpotBacktestDayBreakdown `json:"breakdown"`
}

func spotBacktestCacheKey(symbol, exchange string, days int) string {
	return fmt.Sprintf("arb:spot_backtest:%s:%s:buy_spot_short:%d", symbol, exchange, days)
}

// backtestDirB checks the Redis cache for a Dir B historical backtest result.
// Fails open on cache miss — background prefetch will populate for the next scan.
func (e *SpotEngine) backtestDirB(opp SpotArbOpportunity) (bool, string) {
	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return true, ""
	}

	cacheKey := spotBacktestCacheKey(opp.Symbol, opp.Exchange, days)
	if result, ok := e.loadSpotBacktestResult(cacheKey); ok {
		if result.SumBps > e.cfg.SpotFuturesBacktestMinProfit {
			return true, ""
		}
		return false, fmt.Sprintf("backtest unprofitable (%dd cached): sumBps=%.2f (need >%.2f)",
			days, result.SumBps, e.cfg.SpotFuturesBacktestMinProfit)
	}

	e.log.Info("spot backtest %s (%s): cache miss, fail-open (will prefetch)", opp.Symbol, opp.Exchange)
	return true, ""
}

func (e *SpotEngine) loadSpotBacktestResult(cacheKey string) (spotBacktestResult, bool) {
	cached, err := e.db.Get(cacheKey)
	if err != nil || cached == "" {
		return spotBacktestResult{}, false
	}
	var result spotBacktestResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return spotBacktestResult{}, false
	}
	return result, true
}

// fetchAndCacheSpotBacktest fetches Dir B historical funding from Loris and caches the result.
// Returns true on success or benign skip (no data, low coverage), false on API error/429.
func (e *SpotEngine) fetchAndCacheSpotBacktest(opp SpotArbOpportunity) bool {
	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return true
	}

	base := strings.TrimSuffix(opp.Symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := discovery.FetchLorisHistoricalSeries(e.client, base, []string{opp.Exchange}, start, now)
	if err != nil {
		if errors.Is(err, discovery.ErrLorisRateLimited) {
			e.log.Warn("spot backtest prefetch %s (%s): 429 rate limited", opp.Symbol, opp.Exchange)
		} else {
			e.log.Warn("spot backtest prefetch %s (%s): %v", opp.Symbol, opp.Exchange, err)
		}
		return false
	}

	series := historical.Series[opp.Exchange]
	if len(series) == 0 {
		return true // no data — not an error
	}

	// Futures settle every 8h; use fixed interval for Dir B.
	filtered := discovery.FilterValidSettlements(series, 8.0)
	expectedSettlements := float64(days) * 24.0 / 8.0
	coverage := float64(len(filtered)) / expectedSettlements
	if coverage < 0.5 {
		return true // insufficient coverage, skip without caching
	}

	var sumY float64
	for _, p := range filtered {
		sumY += p.Y
	}

	result := spotBacktestResult{
		SumBps:      sumY,
		Settlements: len(filtered),
		Coverage:    coverage,
		FetchedAt:   now.Format(time.RFC3339),
	}
	if data, err := json.Marshal(result); err == nil {
		e.db.SetWithTTL(spotBacktestCacheKey(opp.Symbol, opp.Exchange, days), string(data), spotBacktestCacheTTL)
	}

	e.log.Info("spot backtest prefetch %s (%s %dd): sumBps=%.2f settlements=%d coverage=%.0f%%",
		opp.Symbol, opp.Exchange, days, sumY, len(filtered), coverage*100)
	return true
}

// prefetchSpotBacktestData pre-warms the backtest cache for Dir B opportunities.
// Only one prefetch runs at a time (prefetchMu). Rate-limited to avoid 429s from Loris.
func (e *SpotEngine) prefetchSpotBacktestData(opps []SpotArbOpportunity) {
	if !e.cfg.SpotFuturesBacktestEnabled {
		return
	}
	if !e.prefetchMu.TryLock() {
		return
	}
	defer e.prefetchMu.Unlock()

	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return
	}

	// Check global backoff.
	e.spotBackoffMu.RLock()
	backoff := e.spotBackoffUntil
	e.spotBackoffMu.RUnlock()
	if time.Now().Before(backoff) {
		e.log.Info("spot backtest prefetch: skipping, backoff until %s", backoff.Format("15:04:05"))
		return
	}

	// Filter to Dir B only, deduplicate by symbol+exchange.
	seen := make(map[string]bool)
	var toFetch []SpotArbOpportunity
	now := time.Now().UTC()
	for _, opp := range opps {
		if opp.Direction != "buy_spot_short" {
			continue
		}
		k := opp.Symbol + ":" + opp.Exchange
		if seen[k] {
			continue
		}
		seen[k] = true
		cacheKey := spotBacktestCacheKey(opp.Symbol, opp.Exchange, days)
		if result, ok := e.loadSpotBacktestResult(cacheKey); ok && !result.isStale(now) {
			continue
		}
		toFetch = append(toFetch, opp)
	}

	if len(toFetch) == 0 {
		return
	}

	e.log.Info("spot backtest prefetch: %d to fetch", len(toFetch))

	fetched := 0
	for _, opp := range toFetch {
		time.Sleep(nextSpotBacktestDelay())

		e.spotBackoffMu.RLock()
		backoff = e.spotBackoffUntil
		e.spotBackoffMu.RUnlock()
		if time.Now().Before(backoff) {
			e.log.Warn("spot backtest prefetch: hit backoff, stopping after %d/%d", fetched, len(toFetch))
			return
		}

		if !e.fetchAndCacheSpotBacktest(opp) {
			e.spotBackoffMu.Lock()
			e.spotBackoffUntil = time.Now().Add(60 * time.Second)
			e.spotBackoffMu.Unlock()
			e.log.Warn("spot backtest prefetch: error, backing off 60s after %d/%d", fetched, len(toFetch))
			return
		}
		fetched++
	}
	e.log.Info("spot backtest prefetch: completed %d/%d", fetched, len(toFetch))
}

func nextSpotBacktestDelay() time.Duration {
	if spotBacktestPrefetchMaxDelay <= spotBacktestPrefetchMinDelay {
		return spotBacktestPrefetchMinDelay
	}
	span := spotBacktestPrefetchMaxDelay - spotBacktestPrefetchMinDelay + time.Second
	return spotBacktestPrefetchMinDelay + time.Duration(rand.Int63n(int64(span)))
}

// RunSpotBacktestOnDemand fetches fresh historical funding data and returns a
// per-settlement breakdown. Only Direction B (buy_spot_short) is supported.
func (e *SpotEngine) RunSpotBacktestOnDemand(ctx context.Context, symbol, exchange, direction string, days int) (*SpotBacktestReport, error) {
	if direction != "buy_spot_short" {
		return nil, fmt.Errorf("backtest not yet supported for direction %q (requires historical borrow-rate source)", direction)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if days <= 0 {
		days = e.cfg.SpotFuturesBacktestDays
	}
	if days <= 0 {
		days = 7
	}

	base := strings.TrimSuffix(symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := discovery.FetchLorisHistoricalSeries(e.client, base, []string{exchange}, start, now)
	if err != nil {
		return nil, fmt.Errorf("loris fetch: %w", err)
	}

	series := historical.Series[exchange]
	filtered := discovery.FilterValidSettlements(series, 8.0)

	expectedSettlements := float64(days) * 24.0 / 8.0
	coverage := 0.0
	if expectedSettlements > 0 {
		coverage = float64(len(filtered)) / expectedSettlements
	}

	var sumY float64
	breakdown := make([]SpotBacktestDayBreakdown, 0, len(filtered))
	for _, p := range filtered {
		sumY += p.Y
		breakdown = append(breakdown, SpotBacktestDayBreakdown{
			Timestamp: p.T,
			FundingY:  p.Y,
		})
	}

	// Annualize: average daily bps × 365 ÷ 10000
	netAPR := 0.0
	if days > 0 {
		netAPR = (sumY / float64(days)) * 365.0 / 10000.0
	}

	return &SpotBacktestReport{
		Symbol:      symbol,
		Exchange:    exchange,
		Direction:   direction,
		Days:        days,
		SumBps:      sumY,
		NetAPR:      netAPR,
		Settlements: len(filtered),
		Coverage:    coverage,
		Breakdown:   breakdown,
	}, nil
}
