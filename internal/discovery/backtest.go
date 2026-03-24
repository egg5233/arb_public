package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"arb/internal/models"
)

const lorisHistoricalURL = "https://loris.tools/api/funding/historical"

// backtestResult caches the raw sums from the backtest check.
// Pass/fail is re-evaluated on each read using the current BacktestMinProfit.
type backtestResult struct {
	LongSum   float64 `json:"long_sum"`
	ShortSum  float64 `json:"short_sum"`
	NetProfit float64 `json:"net_profit"`
}

// lorisHistoricalResponse represents the Loris historical funding API response.
type lorisHistoricalResponse struct {
	Symbol  string                       `json:"symbol"`
	Series  map[string][]lorisDataPoint  `json:"series"`
	Notices []string                     `json:"notices"`
}

type lorisDataPoint struct {
	T string  `json:"t"` // ISO 8601 timestamp
	Y float64 `json:"y"` // settlement rate
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
	if cached, err := s.db.Get(cacheKey); err == nil && cached != "" {
		var result backtestResult
		if json.Unmarshal([]byte(cached), &result) == nil {
			if result.NetProfit > s.cfg.BacktestMinProfit {
				return true, ""
			}
			return false, fmt.Sprintf("backtest unprofitable (%dd cached): longSum=%.2f shortSum=%.2f net=%.2f (need >%.2f)",
				days, result.LongSum, result.ShortSum, result.NetProfit, s.cfg.BacktestMinProfit)
		}
	}

	// Call Loris historical API.
	base := strings.TrimSuffix(opp.Symbol, "USDT")
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	params := url.Values{}
	params.Set("symbol", base)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", now.Format(time.RFC3339))
	params.Set("exchanges", opp.LongExchange+","+opp.ShortExchange)

	reqURL := lorisHistoricalURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		s.log.Warn("backtest %s: failed to create request: %v", opp.Symbol, err)
		return true, "" // fail open
	}
	req.Header.Set("User-Agent", "arb-bot/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.log.Warn("backtest %s: API request failed: %v", opp.Symbol, err)
		return true, "" // fail open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.log.Warn("backtest %s: API returned %d: %s", opp.Symbol, resp.StatusCode, string(body))
		return true, "" // fail open
	}

	var historical lorisHistoricalResponse
	if err := json.NewDecoder(resp.Body).Decode(&historical); err != nil {
		s.log.Warn("backtest %s: failed to decode response: %v", opp.Symbol, err)
		return true, "" // fail open
	}

	longSeries := historical.Series[opp.LongExchange]
	shortSeries := historical.Series[opp.ShortExchange]

	if len(longSeries) == 0 || len(shortSeries) == 0 {
		s.log.Warn("backtest %s: empty series (long=%d short=%d)", opp.Symbol, len(longSeries), len(shortSeries))
		return true, "" // fail open — no data
	}

	// Get per-leg intervals for settlement time validation.
	longInterval := s.getLegInterval(opp.LongExchange, opp.Symbol)
	shortInterval := s.getLegInterval(opp.ShortExchange, opp.Symbol)

	// Filter valid settlement times independently per leg.
	longFiltered := filterValidSettlements(longSeries, longInterval)
	shortFiltered := filterValidSettlements(shortSeries, shortInterval)

	// Coverage check: if <50% of expected settlements, skip filter.
	expectedLong := float64(days) * 24.0 / longInterval
	expectedShort := float64(days) * 24.0 / shortInterval
	if float64(len(longFiltered)) < expectedLong*0.5 {
		s.log.Warn("backtest %s: low long coverage %d/%.0f, skipping", opp.Symbol, len(longFiltered), expectedLong)
		return true, ""
	}
	if float64(len(shortFiltered)) < expectedShort*0.5 {
		s.log.Warn("backtest %s: low short coverage %d/%.0f, skipping", opp.Symbol, len(shortFiltered), expectedShort)
		return true, ""
	}

	// Sum independently — do NOT normalize by count.
	var longSumY, shortSumY float64
	for _, p := range longFiltered {
		longSumY += p.Y
	}
	for _, p := range shortFiltered {
		shortSumY += p.Y
	}

	netProfit := shortSumY - longSumY
	pass := netProfit > s.cfg.BacktestMinProfit

	reason := ""
	if !pass {
		reason = fmt.Sprintf("backtest unprofitable (%dd): longSum=%.2f shortSum=%.2f net=%.2f (need >%.2f)",
			days, longSumY, shortSumY, netProfit, s.cfg.BacktestMinProfit)
	}

	// Cache raw sums in Redis with 6h TTL (pass/fail re-evaluated on read).
	result := backtestResult{
		LongSum:   longSumY,
		ShortSum:  shortSumY,
		NetProfit: netProfit,
	}
	if data, err := json.Marshal(result); err == nil {
		s.db.SetWithTTL(cacheKey, string(data), 6*time.Hour)
	}

	s.log.Info("backtest %s (%s/%s %dd): longSum=%.2f shortSum=%.2f net=%.2f pass=%v",
		opp.Symbol, opp.LongExchange, opp.ShortExchange, days, longSumY, shortSumY, netProfit, pass)

	return pass, reason
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

// filterValidSettlements filters data points to only valid settlement times
// and deduplicates by rounded hour.
func filterValidSettlements(series []lorisDataPoint, intervalHours float64) []lorisDataPoint {
	seen := make(map[string]bool) // dedupe by rounded hour key
	var out []lorisDataPoint

	for _, p := range series {
		t, err := time.Parse(time.RFC3339, p.T)
		if err != nil {
			continue
		}
		// Round to nearest hour to handle ±1 min jitter (e.g. 07:59 → 08:00).
		rounded := roundToNearestHour(t)
		if !isValidSettlementHour(rounded, intervalHours) {
			continue
		}
		// Deduplicate by rounded hour.
		key := rounded.Format(time.RFC3339)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// roundToNearestHour rounds a timestamp to the nearest hour.
// e.g. 07:59 → 08:00, 08:01 → 08:00, 08:31 → 09:00
func roundToNearestHour(t time.Time) time.Time {
	if t.Minute() >= 30 {
		return t.Truncate(time.Hour).Add(time.Hour)
	}
	return t.Truncate(time.Hour)
}

// isValidSettlementHour checks if a rounded-to-hour timestamp is a valid
// funding settlement time for the given interval.
func isValidSettlementHour(t time.Time, intervalHours float64) bool {
	h := t.Hour()
	switch {
	case intervalHours <= 1:
		return true // every hour
	case intervalHours <= 4:
		return h%4 == 0 // 0,4,8,12,16,20
	case intervalHours <= 8:
		return h%8 == 0 // 0,8,16
	default:
		return true // unknown interval, accept on-the-hour
	}
}
