package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBacktestFundingHistoryLegacyCacheUsesCurrentThreshold(t *testing.T) {
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		BacktestDays:      3,
		BacktestMinProfit: 0,
	}
	scanner := NewScanner(nil, db, cfg)

	key := "arb:backtest:BTCUSDT:binance:bybit:3"
	legacy := `{"long_sum":1.25,"short_sum":2.50,"net_profit":1.25}`
	if err := db.SetWithTTL(key, legacy, time.Hour); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	opp := models.Opportunity{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"}
	pass, _ := scanner.backtestFundingHistory(opp)
	if !pass {
		t.Fatal("legacy cache entry should pass when net profit exceeds threshold")
	}

	cfg.BacktestMinProfit = 2
	pass, reason := scanner.backtestFundingHistory(opp)
	if pass {
		t.Fatal("legacy cache entry should fail when threshold increases above cached net profit")
	}
	if !strings.Contains(reason, "need >2.00") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestBacktestFundingHistoryRequireCachedFailsOnMiss(t *testing.T) {
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{BacktestDays: 3}
	scanner := NewScanner(nil, db, cfg)

	opp := models.Opportunity{Symbol: "ONTUSDT", LongExchange: "binance", ShortExchange: "bingx"}

	pass, reason := scanner.backtestFundingHistoryRequireCached(opp, false)
	if pass {
		t.Fatal("cache miss should fail closed when cached backtest is required and inline fetch is disabled")
	}
	if !strings.Contains(reason, "waiting for prefetch") {
		t.Fatalf("unexpected reason: %q", reason)
	}

	pass, _ = scanner.backtestFundingHistory(opp)
	if !pass {
		t.Fatal("cache miss should still fail open for non-entry scans")
	}
}

func TestBacktestFundingHistoryRequireCachedFetchesInlineOnMiss(t *testing.T) {
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		BacktestDays:      1,
		BacktestMinProfit: 0,
	}
	scanner := NewScanner(nil, db, cfg)

	opp := models.Opportunity{Symbol: "ONTUSDT", LongExchange: "binance", ShortExchange: "bingx"}
	scanner.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"symbol":"ONT",
				"series":{
					"binance":[
						{"t":"2026-04-02T00:00:00Z","y":1.0},
						{"t":"2026-04-02T08:00:00Z","y":1.0},
						{"t":"2026-04-02T16:00:00Z","y":1.0}
					],
					"bingx":[
						{"t":"2026-04-02T00:00:00Z","y":2.0},
						{"t":"2026-04-02T08:00:00Z","y":2.0},
						{"t":"2026-04-02T16:00:00Z","y":2.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			}, nil
		}),
	}

	pass, reason := scanner.backtestFundingHistoryRequireCached(opp, true)
	if !pass {
		t.Fatalf("inline fetch should satisfy entry backtest on cache miss, got reason %q", reason)
	}

	cacheKey := "arb:backtest:ONTUSDT:binance:bingx:1"
	if _, ok := scanner.loadBacktestResult(cacheKey); !ok {
		t.Fatal("expected inline fetch to populate cache")
	}
}

func TestPrefetchBacktestDataRefreshesStaleButSkipsFresh(t *testing.T) {
	origMinDelay := backtestPrefetchMinDelay
	origMaxDelay := backtestPrefetchMaxDelay
	origRefreshInterval := backtestRefreshInterval
	backtestPrefetchMinDelay = 0
	backtestPrefetchMaxDelay = 0
	backtestRefreshInterval = 4 * time.Hour
	defer func() {
		backtestPrefetchMinDelay = origMinDelay
		backtestPrefetchMaxDelay = origMaxDelay
		backtestRefreshInterval = origRefreshInterval
	}()

	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		BacktestDays: 1,
		MaxPositions: 1,
	}
	scanner := NewScanner(nil, db, cfg)

	freshKey := "arb:backtest:BTCUSDT:binance:bybit:1"
	fresh := backtestResult{
		LongSum:   1,
		ShortSum:  2,
		NetProfit: 1,
		FetchedAt: time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	writeBacktestResult(t, db, freshKey, fresh)

	staleKey := "arb:backtest:ETHUSDT:binance:bybit:1"
	stale := backtestResult{
		LongSum:   1,
		ShortSum:  2,
		NetProfit: 1,
		FetchedAt: time.Now().UTC().Add(-6 * time.Hour).Format(time.RFC3339),
	}
	writeBacktestResult(t, db, staleKey, stale)

	var requested []string
	scanner.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requested = append(requested, req.URL.Query().Get("symbol"))
			body := `{
				"symbol":"ETH",
				"series":{
					"binance":[
						{"t":"2026-04-02T00:00:00Z","y":1.0},
						{"t":"2026-04-02T08:00:00Z","y":1.0},
						{"t":"2026-04-02T16:00:00Z","y":1.0}
					],
					"bybit":[
						{"t":"2026-04-02T00:00:00Z","y":2.0},
						{"t":"2026-04-02T08:00:00Z","y":2.0},
						{"t":"2026-04-02T16:00:00Z","y":2.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			}, nil
		}),
	}

	scanner.prefetchBacktestData([]models.Opportunity{
		{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"},
		{Symbol: "ETHUSDT", LongExchange: "binance", ShortExchange: "bybit"},
	})

	if len(requested) != 1 || requested[0] != "ETH" {
		t.Fatalf("expected exactly one refresh request for stale ETH cache, got %v", requested)
	}

	updated, ok := scanner.loadBacktestResult(staleKey)
	if !ok {
		t.Fatal("expected refreshed stale cache entry to exist")
	}
	if updated.FetchedAt == "" {
		t.Fatal("expected refreshed cache entry to include fetched_at")
	}
	if updated.NetProfit != 3 {
		t.Fatalf("expected refreshed net profit 3.0, got %.2f", updated.NetProfit)
	}
}

func TestBacktestRefreshCandidatesUsesActionableSlice(t *testing.T) {
	cfg := &config.Config{MaxPositions: 2}
	scanner := NewScanner(nil, nil, cfg)

	var opps []models.Opportunity
	for i := 0; i < 12; i++ {
		opps = append(opps, models.Opportunity{Symbol: fmt.Sprintf("SYM%02dUSDT", i)})
	}

	candidates := scanner.backtestRefreshCandidates(opps)
	if len(candidates) != 10 {
		t.Fatalf("expected floor limit of 10 candidates, got %d", len(candidates))
	}
	if candidates[0].Symbol != opps[0].Symbol || candidates[9].Symbol != opps[9].Symbol {
		t.Fatal("expected candidate selection to preserve the top-ranked ordering")
	}
}

func writeBacktestResult(t *testing.T, db *database.Client, key string, result backtestResult) {
	t.Helper()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := db.SetWithTTL(key, string(data), time.Hour); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
}
