package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"arb/internal/models"
)

var (
	backtestCacheTTL            = 24 * time.Hour
	backtestRefreshInterval     = 4 * time.Hour
	backtestRefreshCandidateMin = 10
	backtestInlineFetchLimit    = 3
	backtestPrefetchMinDelay    = 3 * time.Second
	backtestPrefetchMaxDelay    = 30 * time.Second
)

// backtestResult caches the raw sums from the backtest check.
// Pass/fail is re-evaluated on each read using the current BacktestMinProfit.
type backtestResult struct {
	LongSum   float64 `json:"long_sum"`
	ShortSum  float64 `json:"short_sum"`
	NetProfit float64 `json:"net_profit"`
	FetchedAt string  `json:"fetched_at,omitempty"`
}

// backtestFundingHistory checks if a coin's historical funding spread has been
// profitable over the configured lookback period. Returns (pass, reason).
func (s *Scanner) backtestFundingHistory(opp models.Opportunity) (bool, string) {
	days := s.cfg.BacktestDays
	if days <= 0 {
		return true, ""
	}

	// Cache key includes days so changing config doesn't serve stale results.
	cacheKey := fmt.Sprintf("arb:backtest:%s:%s:%s:%d", opp.Symbol, opp.LongExchange, opp.ShortExchange, days)

	// Check Redis cache — raw sums cached, pass/fail re-evaluated with current threshold.
	if result, ok := s.loadBacktestResult(cacheKey); ok {
		if result.NetProfit > s.cfg.BacktestMinProfit {
			return true, ""
		}
		return false, fmt.Sprintf("backtest unprofitable (%dd cached): longSum=%.2f shortSum=%.2f net=%.2f (need >%.2f)",
			days, result.LongSum, result.ShortSum, result.NetProfit, s.cfg.BacktestMinProfit)
	}

	// Cache miss — fail open. Background prefetch will populate for next scan.
	s.log.Info("backtest %s (%s/%s): cache miss, skipping (will prefetch)", opp.Symbol, opp.LongExchange, opp.ShortExchange)
	return true, ""
}

// backtestFundingHistoryRequireCached guarantees the decision is backed by an
// actual cached result. On entry scans, callers may allow an inline fetch on
// cache miss; if that fails, the opportunity is rejected.
func (s *Scanner) backtestFundingHistoryRequireCached(opp models.Opportunity, allowInlineFetch bool) (bool, string) {
	days := s.cfg.BacktestDays
	if days <= 0 {
		return true, ""
	}

	cacheKey := fmt.Sprintf("arb:backtest:%s:%s:%s:%d", opp.Symbol, opp.LongExchange, opp.ShortExchange, days)
	if result, ok := s.loadBacktestResult(cacheKey); ok {
		if result.NetProfit > s.cfg.BacktestMinProfit {
			return true, ""
		}
		return false, fmt.Sprintf("backtest unprofitable (%dd cached): longSum=%.2f shortSum=%.2f net=%.2f (need >%.2f)",
			days, result.LongSum, result.ShortSum, result.NetProfit, s.cfg.BacktestMinProfit)
	}

	if !allowInlineFetch {
		return false, fmt.Sprintf("backtest missing (%dd): waiting for prefetch", days)
	}

	if !s.tryInlineFetchBacktest(opp) {
		return false, fmt.Sprintf("backtest missing (%dd): inline fetch failed", days)
	}

	if result, ok := s.loadBacktestResult(cacheKey); ok {
		if result.NetProfit > s.cfg.BacktestMinProfit {
			return true, ""
		}
		return false, fmt.Sprintf("backtest unprofitable (%dd fetched): longSum=%.2f shortSum=%.2f net=%.2f (need >%.2f)",
			days, result.LongSum, result.ShortSum, result.NetProfit, s.cfg.BacktestMinProfit)
	}
	return false, fmt.Sprintf("backtest missing (%dd): inline fetch returned no cache entry", days)
}

// fetchAndCacheBacktest calls the Loris historical API for a single opportunity
// and stores the result in Redis. Returns true if successful, false on error/429.
func (s *Scanner) fetchAndCacheBacktest(opp models.Opportunity) bool {
	days := s.cfg.BacktestDays
	if days <= 0 {
		return true
	}

	cacheKey := fmt.Sprintf("arb:backtest:%s:%s:%s:%d", opp.Symbol, opp.LongExchange, opp.ShortExchange, days)

	base := strings.TrimSuffix(opp.Symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := FetchLorisHistoricalSeries(s.client, base, []string{opp.LongExchange, opp.ShortExchange}, start, now)
	if err != nil {
		if errors.Is(err, ErrLorisRateLimited) {
			s.log.Warn("backtest prefetch %s: 429 rate limited", opp.Symbol)
		} else {
			s.log.Warn("backtest prefetch %s: request failed: %v", opp.Symbol, err)
		}
		return false
	}

	longSeries := historical.Series[opp.LongExchange]
	shortSeries := historical.Series[opp.ShortExchange]

	if len(longSeries) == 0 || len(shortSeries) == 0 {
		return true // no data, but not an error
	}

	// Filter and sum.
	longInterval := s.getLegInterval(opp.LongExchange, opp.Symbol)
	shortInterval := s.getLegInterval(opp.ShortExchange, opp.Symbol)
	longFiltered := FilterValidSettlements(longSeries, longInterval)
	shortFiltered := FilterValidSettlements(shortSeries, shortInterval)

	expectedLong := float64(days) * 24.0 / longInterval
	expectedShort := float64(days) * 24.0 / shortInterval
	if float64(len(longFiltered)) < expectedLong*0.5 || float64(len(shortFiltered)) < expectedShort*0.5 {
		return true // low coverage, skip
	}

	var longSumY, shortSumY float64
	for _, p := range longFiltered {
		longSumY += p.Y
	}
	for _, p := range shortFiltered {
		shortSumY += p.Y
	}

	netProfit := shortSumY - longSumY
	result := backtestResult{
		LongSum:   longSumY,
		ShortSum:  shortSumY,
		NetProfit: netProfit,
		FetchedAt: now.Format(time.RFC3339),
	}
	if data, err := json.Marshal(result); err == nil {
		s.db.SetWithTTL(cacheKey, string(data), backtestCacheTTL)
	}

	s.log.Info("backtest prefetch %s (%s/%s %dd): longSum=%.2f shortSum=%.2f net=%.2f",
		opp.Symbol, opp.LongExchange, opp.ShortExchange, days, longSumY, shortSumY, netProfit)
	return true
}

// prefetchBacktestData pre-warms the backtest cache for verified opportunities.
// Runs with rate-limiting delays to avoid 429s from Loris API.
// Only one prefetch can run at a time (prefetchMu).
func (s *Scanner) prefetchBacktestData(opps []models.Opportunity) {
	if !s.prefetchMu.TryLock() {
		return // another prefetch is already running
	}
	defer s.prefetchMu.Unlock()

	days := s.cfg.BacktestDays
	if days <= 0 {
		return
	}

	// Check global backoff (shared across perp-perp and spot-futures engines).
	if active, until := LorisBackoffUntil(); active {
		s.log.Info("backtest prefetch: skipping, rate limit backoff until %s", until.Format("15:04:05"))
		return
	}

	candidates := s.backtestRefreshCandidates(opps)
	if len(candidates) == 0 {
		return
	}

	// Deduplicate by cache key.
	type oppKey struct{ symbol, long, short string }
	seen := make(map[oppKey]bool)
	var toFetch []models.Opportunity
	fresh := 0
	stale := 0
	now := time.Now().UTC()
	for _, opp := range candidates {
		k := oppKey{opp.Symbol, opp.LongExchange, opp.ShortExchange}
		if seen[k] {
			continue
		}
		seen[k] = true
		cacheKey := fmt.Sprintf("arb:backtest:%s:%s:%s:%d", opp.Symbol, opp.LongExchange, opp.ShortExchange, days)
		if result, ok := s.loadBacktestResult(cacheKey); ok {
			if !result.isStale(now) {
				fresh++
				continue
			}
			stale++
		}
		toFetch = append(toFetch, opp)
	}

	if len(toFetch) == 0 {
		return
	}

	s.log.Info("backtest prefetch: %d to fetch (%d stale), %d fresh, %d candidates",
		len(toFetch), stale, fresh, len(seen))

	fetched := 0
	for _, opp := range toFetch {
		// Random delay between requests to avoid bursting the historical API.
		delay := nextBacktestPrefetchDelay()
		time.Sleep(delay)

		// Re-check backoff (might have been tripped by another path, incl. the spot engine).
		if active, _ := LorisBackoffUntil(); active {
			s.log.Warn("backtest prefetch: hit backoff, stopping after %d/%d", fetched, len(toFetch))
			return
		}

		if !s.fetchAndCacheBacktest(opp) {
			// On failure (likely 429), set global backoff. FetchLorisHistoricalSeries
			// auto-triggers on 429; TriggerLorisBackoff here covers non-429 errors
			// (timeout, decode failure) that we treat as rate-limiting too.
			TriggerLorisBackoff()
			s.log.Warn("backtest prefetch: 429/error, backing off 60s after %d/%d", fetched, len(toFetch))
			return
		}
		fetched++
	}

	s.log.Info("backtest prefetch: completed %d/%d", fetched, len(toFetch))
}

func (s *Scanner) tryInlineFetchBacktest(opp models.Opportunity) bool {
	if active, until := LorisBackoffUntil(); active {
		s.log.Warn("backtest entry fetch %s (%s/%s): skipped, rate limit backoff until %s",
			opp.Symbol, opp.LongExchange, opp.ShortExchange, until.Format("15:04:05"))
		return false
	}

	s.log.Info("backtest entry fetch %s (%s/%s): cache miss, fetching inline",
		opp.Symbol, opp.LongExchange, opp.ShortExchange)
	if s.fetchAndCacheBacktest(opp) {
		return true
	}

	TriggerLorisBackoff()
	return false
}

func (s *Scanner) loadBacktestResult(cacheKey string) (backtestResult, bool) {
	cached, err := s.db.Get(cacheKey)
	if err != nil || cached == "" {
		return backtestResult{}, false
	}

	var result backtestResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return backtestResult{}, false
	}
	return result, true
}

func (r backtestResult) isStale(now time.Time) bool {
	if backtestRefreshInterval <= 0 {
		return false
	}
	if r.FetchedAt == "" {
		return true
	}

	fetchedAt, err := time.Parse(time.RFC3339, r.FetchedAt)
	if err != nil {
		return true
	}
	return now.Sub(fetchedAt) >= backtestRefreshInterval
}

func (s *Scanner) backtestRefreshCandidates(opps []models.Opportunity) []models.Opportunity {
	limit := backtestRefreshCandidateMin
	if cfgLimit := s.cfg.MaxPositions * 2; cfgLimit > limit {
		limit = cfgLimit
	}
	if limit <= 0 || len(opps) <= limit {
		return opps
	}
	return opps[:limit]
}

func nextBacktestPrefetchDelay() time.Duration {
	if backtestPrefetchMaxDelay <= backtestPrefetchMinDelay {
		if backtestPrefetchMinDelay < 0 {
			return 0
		}
		return backtestPrefetchMinDelay
	}

	span := backtestPrefetchMaxDelay - backtestPrefetchMinDelay + time.Second
	return backtestPrefetchMinDelay + time.Duration(rand.Int63n(int64(span)))
}

// getLegInterval returns the funding interval in hours for a specific exchange+symbol.
// Falls back to 8h if not found.
func (s *Scanner) getLegInterval(exchange, symbol string) float64 {
	s.intervalsMu.RLock()
	defer s.intervalsMu.RUnlock()

	base := strings.TrimSuffix(symbol, "USDT")
	if intervals, ok := s.lastLorisIntervals[exchange]; ok {
		// Try both BASE and BASEUSDT keys (Loris may use either).
		if h, ok := intervals[base]; ok && h > 0 {
			return h
		}
		if h, ok := intervals[symbol]; ok && h > 0 {
			return h
		}
	}
	return 8 // default fallback
}
