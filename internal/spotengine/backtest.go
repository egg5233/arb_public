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
	"arb/pkg/exchange"
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
// Field names match the frontend contract (web/src/pages/Opportunities.tsx).
type SpotBacktestDayBreakdown struct {
	Date       string  `json:"date"`                  // short UTC "MM-DD HH" label
	Bps        float64 `json:"bps"`                   // net bps (Dir B: funding; Dir A: -funding-borrow)
	FundingBps float64 `json:"funding_bps,omitempty"` // Dir A only: raw funding settlement bps
	BorrowBps  float64 `json:"borrow_bps,omitempty"`  // Dir A only: borrow cost bps for this 8h period
}

// SpotBacktestReport is the payload returned by RunSpotBacktestOnDemand.
// Field names match the frontend contract (web/src/pages/Opportunities.tsx).
type SpotBacktestReport struct {
	Symbol          string                     `json:"symbol"`
	Exchange        string                     `json:"exchange"`
	Direction       string                     `json:"direction"`
	Days            int                        `json:"days_lookback"`
	SumBps          float64                    `json:"sum_bps"`
	ProjectedAPR    float64                    `json:"projected_apr"` // decimal, e.g. 0.12 = 12%
	SettlementCount int                        `json:"settlement_count"`
	CoveragePct     float64                    `json:"coverage_pct"` // 0-100 percent
	DayBreakdown    []SpotBacktestDayBreakdown `json:"days"`
	FundingBps      float64                    `json:"funding_bps,omitempty"` // Dir A only: cumulative raw funding
	BorrowBps       float64                    `json:"borrow_bps,omitempty"`  // Dir A only: cumulative borrow cost
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

// launchBacktestPrefetch runs prefetchSpotBacktestData in a tracked goroutine
// so Stop() waits for it to finish before returning.
func (e *SpotEngine) launchBacktestPrefetch(opps []SpotArbOpportunity) {
	if !e.cfg.SpotFuturesBacktestEnabled {
		return
	}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.prefetchSpotBacktestData(opps)
	}()
}

// sleepInterruptible returns early if Stop() is called during the sleep.
// Returns true if the caller should continue; false if shutting down.
func (e *SpotEngine) sleepInterruptible(d time.Duration) bool {
	if d <= 0 {
		return !e.stopping()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-e.stopCh:
		return false
	case <-timer.C:
		return !e.stopping()
	}
}

// prefetchSpotBacktestData pre-warms the backtest cache for Dir B opportunities.
// Only one prefetch runs at a time (prefetchMu). Rate-limited to avoid 429s from Loris.
// Rate-limit cooldown is package-level (discovery.LorisBackoffUntil) and shared
// with the perp-perp prefetcher — a 429 seen by either one suppresses both.
func (e *SpotEngine) prefetchSpotBacktestData(opps []SpotArbOpportunity) {
	if !e.cfg.SpotFuturesBacktestEnabled {
		return
	}
	if !e.prefetchMu.TryLock() {
		return
	}
	defer e.prefetchMu.Unlock()

	if e.stopping() {
		return
	}

	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return
	}

	// Check shared Loris 429 backoff.
	if active, until := discovery.LorisBackoffUntil(); active {
		e.log.Info("spot backtest prefetch: skipping, shared Loris backoff until %s", until.Format("15:04:05"))
		return
	}

	// Collect Dir B and supported Dir A opps, deduplicate by symbol+exchange+direction.
	seen := make(map[string]bool)
	var toFetch []SpotArbOpportunity
	now := time.Now().UTC()
	for _, opp := range opps {
		switch opp.Direction {
		case "buy_spot_short":
			k := opp.Symbol + ":" + opp.Exchange + ":b"
			if seen[k] {
				continue
			}
			seen[k] = true
			cacheKey := spotBacktestCacheKey(opp.Symbol, opp.Exchange, days)
			if result, ok := e.loadSpotBacktestResult(cacheKey); ok && !result.isStale(now) {
				continue
			}
			toFetch = append(toFetch, opp)
		case "borrow_sell_long":
			if !e.canRunDirABacktest(opp.Exchange) {
				continue
			}
			if _, hasAdapter := e.spotMargin[opp.Exchange]; !hasAdapter {
				continue
			}
			k := opp.Symbol + ":" + opp.Exchange + ":a"
			if seen[k] {
				continue
			}
			seen[k] = true
			cacheKey := spotBacktestDirACacheKey(opp.Symbol, opp.Exchange, days)
			if result, ok := e.loadSpotBacktestDirAResult(cacheKey); ok && !result.isStale(now) {
				continue
			}
			toFetch = append(toFetch, opp)
		}
	}

	if len(toFetch) == 0 {
		return
	}

	e.log.Info("spot backtest prefetch: %d to fetch", len(toFetch))

	fetched := 0
	for _, opp := range toFetch {
		if !e.sleepInterruptible(nextSpotBacktestDelay()) {
			e.log.Info("spot backtest prefetch: shutdown during delay, stopping after %d/%d", fetched, len(toFetch))
			return
		}

		if active, _ := discovery.LorisBackoffUntil(); active {
			e.log.Warn("spot backtest prefetch: hit shared backoff, stopping after %d/%d", fetched, len(toFetch))
			return
		}

		var ok bool
		switch opp.Direction {
		case "borrow_sell_long":
			ok = e.fetchAndCacheSpotBacktestDirA(context.Background(), opp)
		default:
			ok = e.fetchAndCacheSpotBacktest(opp)
		}
		if !ok {
			// FetchLorisHistoricalSeries auto-triggers backoff on 429; do it
			// here too so non-429 errors we treat as rate-limiting also cool down.
			discovery.TriggerLorisBackoff()
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

// RunSpotBacktestOnDemand fetches fresh historical data and returns a per-settlement
// breakdown. Dir B (buy_spot_short) uses Loris funding only. Dir A (borrow_sell_long)
// combines Loris funding + exchange historical borrow rates; only supported on
// binance, bybit, and gateio.
func (e *SpotEngine) RunSpotBacktestOnDemand(ctx context.Context, symbol, exchName, direction string, days int) (*SpotBacktestReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if days <= 0 {
		days = e.cfg.SpotFuturesBacktestDays
	}
	if days <= 0 {
		days = 7
	}

	switch direction {
	case "buy_spot_short":
		return e.runBacktestDirBOnDemand(ctx, symbol, exchName, days)
	case "borrow_sell_long":
		if !e.canRunDirABacktest(exchName) {
			return nil, fmt.Errorf("Dir A backtest not yet supported on %q", exchName)
		}
		return e.runBacktestDirAOnDemand(ctx, symbol, exchName, days)
	default:
		return nil, fmt.Errorf("unknown direction %q", direction)
	}
}

func (e *SpotEngine) runBacktestDirBOnDemand(ctx context.Context, symbol, exchName string, days int) (*SpotBacktestReport, error) {
	base := strings.TrimSuffix(symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := discovery.FetchLorisHistoricalSeries(e.client, base, []string{exchName}, start, now)
	if err != nil {
		return nil, fmt.Errorf("loris fetch: %w", err)
	}

	series := historical.Series[exchName]
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
			Date: formatSettlementLabel(p.T),
			Bps:  p.Y,
		})
	}

	projectedAPR := 0.0
	if days > 0 {
		projectedAPR = (sumY / float64(days)) * 365.0 / 10000.0
	}

	return &SpotBacktestReport{
		Symbol:          symbol,
		Exchange:        exchName,
		Direction:       "buy_spot_short",
		Days:            days,
		SumBps:          sumY,
		ProjectedAPR:    projectedAPR,
		SettlementCount: len(filtered),
		CoveragePct:     coverage * 100.0,
		DayBreakdown:    breakdown,
	}, nil
}

func (e *SpotEngine) runBacktestDirAOnDemand(ctx context.Context, symbol, exchName string, days int) (*SpotBacktestReport, error) {
	smExch, ok := e.spotMargin[exchName]
	if !ok {
		return nil, fmt.Errorf("no spot margin adapter for %q", exchName)
	}

	base := strings.TrimSuffix(symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := discovery.FetchLorisHistoricalSeries(e.client, base, []string{exchName}, start, now)
	if err != nil {
		return nil, fmt.Errorf("loris fetch: %w", err)
	}

	borrowSeries, err := e.loadBorrowHistoryWithFallback(ctx, smExch, exchName, base, start, now)
	if err != nil {
		return nil, fmt.Errorf("borrow rate history: %w", err)
	}

	// Build hourly borrow lookup for O(1) per-settlement attribution.
	borrowByHour := make(map[time.Time]float64, len(borrowSeries))
	for _, bp := range borrowSeries {
		h := bp.Timestamp.UTC().Truncate(time.Hour)
		borrowByHour[h] += bp.HourlyRate
	}

	series := historical.Series[exchName]
	filtered := discovery.FilterValidSettlements(series, 8.0)

	expectedSettlements := float64(days) * 24.0 / 8.0
	coverage := 0.0
	if expectedSettlements > 0 {
		coverage = float64(len(filtered)) / expectedSettlements
	}

	var totalFundingBps, totalBorrowBps float64
	breakdown := make([]SpotBacktestDayBreakdown, 0, len(filtered))
	for _, p := range filtered {
		t, _ := time.Parse(time.RFC3339, p.T)
		t = t.UTC()

		// Sum hourly borrow rates for the 8h window ending at this settlement.
		var settleBorrowBps float64
		for h := 1; h <= 8; h++ {
			hour := t.Add(-time.Duration(h) * time.Hour).Truncate(time.Hour)
			settleBorrowBps += borrowByHour[hour] * 10000
		}

		settleFundingBps := p.Y
		netBps := -settleFundingBps - settleBorrowBps
		totalFundingBps += settleFundingBps
		totalBorrowBps += settleBorrowBps

		breakdown = append(breakdown, SpotBacktestDayBreakdown{
			Date:       formatSettlementLabel(p.T),
			Bps:        netBps,
			FundingBps: settleFundingBps,
			BorrowBps:  settleBorrowBps,
		})
	}

	// Dir A net: long futures pays positive funding; borrow is always a cost.
	netBps := -totalFundingBps - totalBorrowBps
	projectedAPR := 0.0
	if days > 0 {
		projectedAPR = (netBps / float64(days)) * 365.0 / 10000.0
	}

	return &SpotBacktestReport{
		Symbol:          symbol,
		Exchange:        exchName,
		Direction:       "borrow_sell_long",
		Days:            days,
		SumBps:          netBps,
		ProjectedAPR:    projectedAPR,
		SettlementCount: len(filtered),
		CoveragePct:     coverage * 100.0,
		DayBreakdown:    breakdown,
		FundingBps:      totalFundingBps,
		BorrowBps:       totalBorrowBps,
	}, nil
}

// formatSettlementLabel converts an ISO-8601 timestamp into a short "MM-DD HH"
// UTC label for the frontend breakdown table. Falls back to the first 10 chars
// if parsing fails (e.g. "2026-04-18").
func formatSettlementLabel(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		if len(ts) > 10 {
			return ts[:10]
		}
		return ts
	}
	return t.UTC().Format("01-02 15")
}

// ---- Dir A (borrow_sell_long) backtest ----

// spotBacktestDirAResult caches Dir A funding + borrow sums in Redis.
// NetBps = -FundingBps - BorrowBps: Dir A long-futures pays positive funding
// and always pays borrow, so profitable Dir A has negative cumulative funding.
type spotBacktestDirAResult struct {
	FundingBps  float64 `json:"funding_bps"`
	BorrowBps   float64 `json:"borrow_bps"`
	NetBps      float64 `json:"net_bps"`
	Settlements int     `json:"settlements"`
	Coverage    float64 `json:"coverage"`
	FetchedAt   string  `json:"fetched_at,omitempty"`
}

func (r spotBacktestDirAResult) isStale(now time.Time) bool {
	if r.FetchedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, r.FetchedAt)
	if err != nil {
		return true
	}
	return now.Sub(t) >= spotBacktestRefreshInterval
}

func spotBacktestDirACacheKey(symbol, exchName string, days int) string {
	return fmt.Sprintf("arb:spot_backtest:%s:%s:borrow_sell_long:%d", symbol, exchName, days)
}

// exchangeSupportsDirABacktest returns true for the three exchanges that expose
// public historical borrow-rate APIs: Binance, Bybit, Gate.io. This is the
// "native" list — OKX and Bitget can still run Dir A via the CoinGlass fallback
// when SpotFuturesBacktestCoinGlassFallback is enabled (see canRunDirABacktest).
func exchangeSupportsDirABacktest(exchName string) bool {
	switch exchName {
	case "binance", "bybit", "gateio":
		return true
	default:
		return false
	}
}

// exchangeSupportsCoinGlassDirAFallback returns true for the exchanges whose
// historical borrow rates are available via the CoinGlass /MarginFee scraper
// (/var/solana/data/coinGlass/fetch_margin_fee.js) when the native API doesn't
// expose them.
func exchangeSupportsCoinGlassDirAFallback(exchName string) bool {
	switch exchName {
	case "okx", "bitget":
		return true
	default:
		return false
	}
}

// canRunDirABacktest decides whether Dir A historical backtest can run on this
// exchange given the current config. True when the adapter has a native history
// API, OR when the CoinGlass fallback is enabled and the exchange has CoinGlass
// coverage.
func (e *SpotEngine) canRunDirABacktest(exchName string) bool {
	if exchangeSupportsDirABacktest(exchName) {
		return true
	}
	if e.cfg.SpotFuturesBacktestCoinGlassFallback && exchangeSupportsCoinGlassDirAFallback(exchName) {
		return true
	}
	return false
}

// loadBorrowHistoryWithFallback returns the historical borrow-rate series for
// (exchName, baseCoin) over [start, end]. It prefers the adapter's native
// GetMarginInterestRateHistory; if that returns ErrHistoricalBorrowNotSupported
// and the CoinGlass fallback is enabled, it reads from the scraper-populated
// Redis list instead. Returns (nil, nil) with no error if the adapter has no
// native support AND the fallback is disabled / empty — callers treat that as
// cache-miss / skip.
func (e *SpotEngine) loadBorrowHistoryWithFallback(
	ctx context.Context,
	smExch exchange.SpotMarginExchange,
	exchName, baseCoin string,
	start, end time.Time,
) ([]exchange.MarginInterestRatePoint, error) {
	series, err := smExch.GetMarginInterestRateHistory(ctx, baseCoin, start, end)
	if err == nil {
		return series, nil
	}
	if !errors.Is(err, exchange.ErrHistoricalBorrowNotSupported) {
		return nil, err
	}
	// Adapter has no native support. Try CoinGlass fallback if enabled.
	if !e.cfg.SpotFuturesBacktestCoinGlassFallback {
		return nil, err // preserve the sentinel for upstream handling
	}
	cgPoints, cgErr := e.db.GetCoinGlassMarginFeeHistory(exchName, baseCoin, start, end)
	if cgErr != nil {
		e.log.Warn("Dir A CoinGlass fallback read %s (%s): %v", baseCoin, exchName, cgErr)
		return nil, err // fall back to the original sentinel
	}
	if len(cgPoints) == 0 {
		// Key missing or no data in range — treat as unsupported for this scan.
		return nil, err
	}
	out := make([]exchange.MarginInterestRatePoint, 0, len(cgPoints))
	for _, p := range cgPoints {
		out = append(out, exchange.MarginInterestRatePoint{
			Timestamp:  p.Timestamp,
			HourlyRate: p.HourlyRate,
		})
	}
	e.log.Info("Dir A CoinGlass fallback %s (%s): %d points", baseCoin, exchName, len(out))
	return out, nil
}

// backtestDirA checks the Redis cache for a Dir A historical backtest result.
// Unsupported exchanges fail-open immediately. Cache misses also fail-open.
func (e *SpotEngine) backtestDirA(opp SpotArbOpportunity) (bool, string) {
	if !e.canRunDirABacktest(opp.Exchange) {
		return true, ""
	}
	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return true, ""
	}
	cacheKey := spotBacktestDirACacheKey(opp.Symbol, opp.Exchange, days)
	if result, ok := e.loadSpotBacktestDirAResult(cacheKey); ok {
		if result.NetBps > e.cfg.SpotFuturesBacktestMinProfit {
			return true, ""
		}
		return false, fmt.Sprintf("backtest unprofitable (%dd cached): netBps=%.2f (need >%.2f)",
			days, result.NetBps, e.cfg.SpotFuturesBacktestMinProfit)
	}
	e.log.Info("spot backtest Dir A %s (%s): cache miss, fail-open (will prefetch)", opp.Symbol, opp.Exchange)
	return true, ""
}

func (e *SpotEngine) loadSpotBacktestDirAResult(cacheKey string) (spotBacktestDirAResult, bool) {
	cached, err := e.db.Get(cacheKey)
	if err != nil || cached == "" {
		return spotBacktestDirAResult{}, false
	}
	var result spotBacktestDirAResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return spotBacktestDirAResult{}, false
	}
	return result, true
}

// fetchAndCacheSpotBacktestDirA fetches Loris historical funding and exchange
// historical borrow rates for a Dir A opportunity, then caches the result.
// Returns true on success or benign skip, false on API error/429.
func (e *SpotEngine) fetchAndCacheSpotBacktestDirA(ctx context.Context, opp SpotArbOpportunity) bool {
	days := e.cfg.SpotFuturesBacktestDays
	if days <= 0 {
		return true
	}
	smExch, ok := e.spotMargin[opp.Exchange]
	if !ok {
		return true
	}

	base := strings.TrimSuffix(opp.Symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	historical, err := discovery.FetchLorisHistoricalSeries(e.client, base, []string{opp.Exchange}, start, now)
	if err != nil {
		if errors.Is(err, discovery.ErrLorisRateLimited) {
			e.log.Warn("spot backtest Dir A prefetch %s (%s): 429 rate limited", opp.Symbol, opp.Exchange)
		} else {
			e.log.Warn("spot backtest Dir A prefetch %s (%s): loris: %v", opp.Symbol, opp.Exchange, err)
		}
		return false
	}

	borrowSeries, err := e.loadBorrowHistoryWithFallback(ctx, smExch, opp.Exchange, base, start, now)
	if err != nil {
		if errors.Is(err, exchange.ErrHistoricalBorrowNotSupported) {
			// Neither native adapter nor CoinGlass fallback produced data.
			e.log.Info("spot backtest Dir A %s (%s): historical borrow unavailable, skipping", opp.Symbol, opp.Exchange)
			return true
		}
		e.log.Warn("spot backtest Dir A prefetch %s (%s): borrow history: %v", opp.Symbol, opp.Exchange, err)
		return false
	}

	fundingSeries := historical.Series[opp.Exchange]
	if len(fundingSeries) == 0 {
		return true
	}

	filtered := discovery.FilterValidSettlements(fundingSeries, 8.0)
	expectedSettlements := float64(days) * 24.0 / 8.0
	coverage := float64(len(filtered)) / expectedSettlements
	if coverage < 0.5 {
		return true
	}

	var sumFundingBps float64
	for _, p := range filtered {
		sumFundingBps += p.Y
	}

	// Each borrow point covers one hour; convert hourly rate to bps.
	var sumBorrowBps float64
	for _, p := range borrowSeries {
		sumBorrowBps += p.HourlyRate * 10000
	}

	// Dir A net: long futures pays positive funding; borrow is always a cost.
	netBps := -sumFundingBps - sumBorrowBps

	result := spotBacktestDirAResult{
		FundingBps:  sumFundingBps,
		BorrowBps:   sumBorrowBps,
		NetBps:      netBps,
		Settlements: len(filtered),
		Coverage:    coverage,
		FetchedAt:   now.Format(time.RFC3339),
	}
	if data, err := json.Marshal(result); err == nil {
		e.db.SetWithTTL(spotBacktestDirACacheKey(opp.Symbol, opp.Exchange, days), string(data), spotBacktestCacheTTL)
	}

	e.log.Info("spot backtest Dir A prefetch %s (%s %dd): net=%.2f bps (funding=%.2f borrow=%.2f) settlements=%d coverage=%.0f%%",
		opp.Symbol, opp.Exchange, days, netBps, sumFundingBps, sumBorrowBps, len(filtered), coverage*100)
	return true
}
