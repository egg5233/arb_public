package spotengine

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

type spotBacktestRoundTripper func(*http.Request) (*http.Response, error)

func (f spotBacktestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newSpotBacktestEngine(t *testing.T) (*SpotEngine, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	return &SpotEngine{
		cfg:    &config.Config{},
		db:     db,
		log:    utils.NewLogger("test-spot-backtest"),
		client: &http.Client{Timeout: 5 * time.Second},
		stopCh: make(chan struct{}),
	}, mr
}

func writeSpotBacktestResult(t *testing.T, db *database.Client, key string, result spotBacktestResult) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := db.SetWithTTL(key, string(data), time.Hour); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
}

func TestSpotBacktestDirB_CacheMissFailOpen(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 10.0

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "buy_spot_short"}
	pass, _ := e.backtestDirB(opp)
	if !pass {
		t.Fatal("cache miss should fail open (pass=true)")
	}
}

func TestSpotBacktestDirB_CacheHitPass(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 10.0

	key := spotBacktestCacheKey("BTCUSDT", "binance", 7)
	writeSpotBacktestResult(t, e.db, key, spotBacktestResult{
		SumBps: 50.0, Settlements: 21, Coverage: 1.0,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	})

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "buy_spot_short"}
	pass, reason := e.backtestDirB(opp)
	if !pass {
		t.Fatalf("cached result above threshold should pass, got reason %q", reason)
	}
}

func TestSpotBacktestDirB_CacheHitFail(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 100.0

	key := spotBacktestCacheKey("BTCUSDT", "binance", 7)
	writeSpotBacktestResult(t, e.db, key, spotBacktestResult{
		SumBps: 5.0, Settlements: 21, Coverage: 1.0,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	})

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "buy_spot_short"}
	pass, reason := e.backtestDirB(opp)
	if pass {
		t.Fatal("cached result below threshold should fail")
	}
	if !strings.Contains(reason, "need >100.00") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestSpotBacktestCacheKeyFormat(t *testing.T) {
	key := spotBacktestCacheKey("ETHUSDT", "bybit", 7)
	expected := "arb:spot_backtest:ETHUSDT:bybit:buy_spot_short:7"
	if key != expected {
		t.Fatalf("cache key = %q, want %q", key, expected)
	}
}

func TestFetchAndCacheSpotBacktest_PopulatesCache(t *testing.T) {
	origMinDelay := spotBacktestPrefetchMinDelay
	origMaxDelay := spotBacktestPrefetchMaxDelay
	spotBacktestPrefetchMinDelay = 0
	spotBacktestPrefetchMaxDelay = 0
	defer func() {
		spotBacktestPrefetchMinDelay = origMinDelay
		spotBacktestPrefetchMaxDelay = origMaxDelay
	}()

	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 1
	e.cfg.SpotFuturesBacktestMinProfit = 0

	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			body := `{
				"symbol":"BTC",
				"series":{
					"binance":[
						{"t":"2026-04-02T00:00:00Z","y":3.0},
						{"t":"2026-04-02T08:00:00Z","y":3.0},
						{"t":"2026-04-02T16:00:00Z","y":3.0}
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

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "buy_spot_short"}
	if !e.fetchAndCacheSpotBacktest(opp) {
		t.Fatal("fetchAndCacheSpotBacktest should return true on success")
	}

	key := spotBacktestCacheKey("BTCUSDT", "binance", 1)
	result, found := e.loadSpotBacktestResult(key)
	if !found {
		t.Fatal("expected cache entry to be populated")
	}
	if result.SumBps != 9.0 {
		t.Fatalf("expected SumBps=9.0, got %.2f", result.SumBps)
	}
	if result.Settlements != 3 {
		t.Fatalf("expected Settlements=3, got %d", result.Settlements)
	}
}

func TestPrefetchSpotBacktestData_SkipsDirA(t *testing.T) {
	origMinDelay := spotBacktestPrefetchMinDelay
	origMaxDelay := spotBacktestPrefetchMaxDelay
	spotBacktestPrefetchMinDelay = 0
	spotBacktestPrefetchMaxDelay = 0
	defer func() {
		spotBacktestPrefetchMinDelay = origMinDelay
		spotBacktestPrefetchMaxDelay = origMaxDelay
	}()

	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestEnabled = true
	e.cfg.SpotFuturesBacktestDays = 1

	var requested []string
	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			requested = append(requested, req.URL.Query().Get("symbol"))
			body := `{"symbol":"ETH","series":{"binance":[
				{"t":"2026-04-02T00:00:00Z","y":3.0},
				{"t":"2026-04-02T08:00:00Z","y":3.0},
				{"t":"2026-04-02T16:00:00Z","y":3.0}
			]}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			}, nil
		}),
	}

	opps := []SpotArbOpportunity{
		{Symbol: "BTCUSDT", Exchange: "binance", Direction: "borrow_sell_long"}, // Dir A — must be skipped
		{Symbol: "ETHUSDT", Exchange: "binance", Direction: "buy_spot_short"},   // Dir B — must be fetched
	}
	e.prefetchSpotBacktestData(opps)

	if len(requested) != 1 || requested[0] != "ETH" {
		t.Fatalf("expected one Loris request for Dir B (ETH), got %v", requested)
	}
}
