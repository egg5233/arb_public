package spotengine

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"arb/pkg/exchange"
)

// Locks in the borrow-coverage floor added after codex audit 14064872.
// Without this floor, partial CoinGlass/native borrow history during
// bootstrap would silently produce misleading NetBps (missing hours default
// to zero via the borrowByHour map lookup), and the result would cache for
// 24h.

func TestFetchAndCacheSpotBacktestDirA_SparseBorrow_NoCacheWrite(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7

	// 7 day window → need ≥ 84 borrow hours (50% of 168) to pass the floor.
	// Seed only 24 hourly points (simulating 24h after scraper started).
	baseTime, _ := time.Parse(time.RFC3339, "2026-04-20T00:00:00Z")
	var sparseBorrow []exchange.MarginInterestRatePoint
	for i := 0; i < 24; i++ {
		sparseBorrow = append(sparseBorrow, exchange.MarginInterestRatePoint{
			Timestamp:  baseTime.Add(time.Duration(i) * time.Hour),
			HourlyRate: 0.00001,
		})
	}
	mock := &mockDirASpotMargin{history: sparseBorrow}
	e.spotMargin = map[string]exchange.SpotMarginExchange{"binance": mock}

	// Full Loris funding series (one settlement every 8h for 7 days).
	lorisBody := `{"symbol":"BTC","series":{"binance":[`
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h += 8 {
			t := baseTime.Add(time.Duration(d*24+h) * time.Hour)
			if d > 0 || h > 0 {
				lorisBody += ","
			}
			lorisBody += `{"t":"` + t.Format(time.RFC3339) + `","y":5.0}`
		}
	}
	lorisBody += "]}}"
	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(lorisBody)),
			}, nil
		}),
	}

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "borrow_sell_long"}
	ok := e.fetchAndCacheSpotBacktestDirA(context.Background(), opp)
	if !ok {
		t.Fatal("fetchAndCacheSpotBacktestDirA should return true (fail-open) on low coverage, got false")
	}

	// Critical: no cache entry should have been written. If it was, subsequent
	// backtestDirA calls would return misleading results for 24h.
	cacheKey := spotBacktestDirACacheKey("BTCUSDT", "binance", 7)
	if _, ok := e.loadSpotBacktestDirAResult(cacheKey); ok {
		t.Fatal("low coverage must NOT write cache — would pollute Dir A filter for up to 24h")
	}
}

func TestFetchAndCacheSpotBacktestDirA_FullBorrow_WritesCache(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7

	// Full 168 hourly borrow points → 100% coverage.
	baseTime, _ := time.Parse(time.RFC3339, "2026-04-20T00:00:00Z")
	var fullBorrow []exchange.MarginInterestRatePoint
	for i := 0; i < 168; i++ {
		fullBorrow = append(fullBorrow, exchange.MarginInterestRatePoint{
			Timestamp:  baseTime.Add(time.Duration(i) * time.Hour),
			HourlyRate: 0.00001,
		})
	}
	mock := &mockDirASpotMargin{history: fullBorrow}
	e.spotMargin = map[string]exchange.SpotMarginExchange{"binance": mock}

	lorisBody := `{"symbol":"BTC","series":{"binance":[`
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h += 8 {
			t := baseTime.Add(time.Duration(d*24+h) * time.Hour)
			if d > 0 || h > 0 {
				lorisBody += ","
			}
			lorisBody += `{"t":"` + t.Format(time.RFC3339) + `","y":5.0}`
		}
	}
	lorisBody += "]}}"
	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(lorisBody)),
			}, nil
		}),
	}

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "borrow_sell_long"}
	if !e.fetchAndCacheSpotBacktestDirA(context.Background(), opp) {
		t.Fatal("full coverage should succeed")
	}
	cacheKey := spotBacktestDirACacheKey("BTCUSDT", "binance", 7)
	if _, ok := e.loadSpotBacktestDirAResult(cacheKey); !ok {
		t.Fatal("full coverage should write cache")
	}
}

func TestRunBacktestDirAOnDemand_SparseBorrow_ReturnsError(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()

	// Only 10 hourly points over a 7-day request window — way below 50% floor.
	baseTime, _ := time.Parse(time.RFC3339, "2026-04-20T00:00:00Z")
	var sparseBorrow []exchange.MarginInterestRatePoint
	for i := 0; i < 10; i++ {
		sparseBorrow = append(sparseBorrow, exchange.MarginInterestRatePoint{
			Timestamp:  baseTime.Add(time.Duration(i) * time.Hour),
			HourlyRate: 0.00001,
		})
	}
	mock := &mockDirASpotMargin{history: sparseBorrow}
	e.spotMargin = map[string]exchange.SpotMarginExchange{"binance": mock}

	lorisBody := `{"symbol":"BTC","series":{"binance":[{"t":"2026-04-15T00:00:00Z","y":5.0}]}}`
	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(lorisBody)),
			}, nil
		}),
	}

	_, err := e.RunSpotBacktestOnDemand(context.Background(), "BTCUSDT", "binance", "borrow_sell_long", 7)
	if err == nil {
		t.Fatal("on-demand with sparse borrow should return error, not misleading NetBps with zero'd hours")
	}
	if !strings.Contains(err.Error(), "insufficient borrow history") {
		t.Fatalf("error should mention insufficient borrow history, got: %v", err)
	}
}
