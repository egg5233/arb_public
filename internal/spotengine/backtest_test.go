package spotengine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
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

	// Dir A binance is supported but the engine has no spotMargin adapter configured,
	// so it should be excluded from toFetch (no extra Loris call).
	if len(requested) != 1 || requested[0] != "ETH" {
		t.Fatalf("expected one Loris request for Dir B (ETH), got %v", requested)
	}
}

// ---- Dir A tests ----

// mockDirASpotMargin is a minimal SpotMarginExchange for Dir A backtest tests.
type mockDirASpotMargin struct {
	history []exchange.MarginInterestRatePoint
	err     error
}

func (m *mockDirASpotMargin) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (m *mockDirASpotMargin) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (m *mockDirASpotMargin) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	return "", nil
}
func (m *mockDirASpotMargin) GetSpotBBO(string) (exchange.BBO, error) {
	return exchange.BBO{Bid: 100, Ask: 100.1}, nil
}
func (m *mockDirASpotMargin) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return &exchange.MarginInterestRate{HourlyRate: 0.00001}, nil
}
func (m *mockDirASpotMargin) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return &exchange.MarginBalance{Available: 1000}, nil
}
func (m *mockDirASpotMargin) TransferToMargin(string, string) error   { return nil }
func (m *mockDirASpotMargin) TransferFromMargin(string, string) error { return nil }
func (m *mockDirASpotMargin) GetMarginInterestRateHistory(_ context.Context, _ string, _, _ time.Time) ([]exchange.MarginInterestRatePoint, error) {
	return m.history, m.err
}

func writeDirABacktestResult(t *testing.T, db *database.Client, key string, result spotBacktestDirAResult) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := db.SetWithTTL(key, string(data), time.Hour); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
}

func TestSpotBacktestDirA_CacheMissFailOpen(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 10.0

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "borrow_sell_long"}
	pass, _ := e.backtestDirA(opp)
	if !pass {
		t.Fatal("Dir A cache miss should fail open (pass=true)")
	}
}

func TestSpotBacktestDirA_CacheHitPass(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = -50.0 // Dir A profitable if net > -50 bps

	key := spotBacktestDirACacheKey("BTCUSDT", "binance", 7)
	writeDirABacktestResult(t, e.db, key, spotBacktestDirAResult{
		NetBps: -10.0, FundingBps: 30.0, BorrowBps: 20.0,
		Settlements: 21, Coverage: 1.0,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	})

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance"}
	pass, reason := e.backtestDirA(opp)
	if !pass {
		t.Fatalf("netBps=-10 above threshold -50, should pass, got reason %q", reason)
	}
}

func TestSpotBacktestDirA_CacheHitFail(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 0.0 // require net > 0 (unlikely for Dir A)

	key := spotBacktestDirACacheKey("BTCUSDT", "binance", 7)
	writeDirABacktestResult(t, e.db, key, spotBacktestDirAResult{
		NetBps: -30.0, FundingBps: 20.0, BorrowBps: 10.0,
		Settlements: 21, Coverage: 1.0,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	})

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance"}
	pass, reason := e.backtestDirA(opp)
	if pass {
		t.Fatal("netBps=-30 below threshold 0, should fail")
	}
	if !strings.Contains(reason, "need >0.00") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestSpotBacktestDirACacheKeyFormat(t *testing.T) {
	key := spotBacktestDirACacheKey("ETHUSDT", "bybit", 7)
	expected := "arb:spot_backtest:ETHUSDT:bybit:borrow_sell_long:7"
	if key != expected {
		t.Fatalf("Dir A cache key = %q, want %q", key, expected)
	}
}

// TestBacktestDirASignMath verifies the critical sign convention:
// Dir A is LONG futures + SHORT spot. Long futures PAYS when funding > 0.
// Positive funding + positive borrow = both costs → netBps must be NEGATIVE.
func TestBacktestDirASignMath(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 1
	e.cfg.SpotFuturesBacktestMinProfit = -9999

	// Use fixed timestamps at valid 8h settlement hours (0h, 8h, 16h UTC).
	// Three funding settlements of +5 bps each (positive = cost to long futures).
	// Borrow: 24 hourly points at 0.0001/h → 0.0001 * 10000 = 1 bps/h each.
	const lorisBody = `{"symbol":"BTC","series":{"binance":[` +
		`{"t":"2026-04-18T00:00:00Z","y":5.0},` +
		`{"t":"2026-04-18T08:00:00Z","y":5.0},` +
		`{"t":"2026-04-18T16:00:00Z","y":5.0}` +
		`]}}`

	hourlyRate := 0.0001 // 0.01%/h → 1 bps/h
	baseTime, _ := time.Parse(time.RFC3339, "2026-04-18T16:00:00Z")
	var borrowPts []exchange.MarginInterestRatePoint
	for i := 0; i < 24; i++ {
		borrowPts = append(borrowPts, exchange.MarginInterestRatePoint{
			Timestamp:  baseTime.Add(-time.Duration(i) * time.Hour),
			HourlyRate: hourlyRate,
		})
	}

	mock := &mockDirASpotMargin{history: borrowPts}
	e.spotMargin = map[string]exchange.SpotMarginExchange{"binance": mock}
	e.client = &http.Client{
		Transport: spotBacktestRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(lorisBody)),
			}, nil
		}),
	}

	opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "binance", Direction: "borrow_sell_long"}
	if !e.fetchAndCacheSpotBacktestDirA(context.Background(), opp) {
		t.Fatal("fetchAndCacheSpotBacktestDirA returned false")
	}

	key := spotBacktestDirACacheKey("BTCUSDT", "binance", 1)
	result, found := e.loadSpotBacktestDirAResult(key)
	if !found {
		t.Fatal("expected Dir A cache entry")
	}

	// Positive funding = cost to long futures. Positive borrow = cost.
	// Both costs → net must be negative.
	// Exact expected values:
	//   FundingBps: 3 settlements × 5.0 = 15.0
	//   BorrowBps:  24h × 0.0001/h × 10000 = 24.0
	//   NetBps:     -15.0 - 24.0 = -39.0
	const (
		eps            = 1e-9
		wantFundingBps = 15.0
		wantBorrowBps  = 24.0
		wantNetBps     = -39.0
	)
	if diff := result.FundingBps - wantFundingBps; diff > eps || diff < -eps {
		t.Fatalf("FundingBps = %.6f, want %.6f", result.FundingBps, wantFundingBps)
	}
	if diff := result.BorrowBps - wantBorrowBps; diff > eps || diff < -eps {
		t.Fatalf("BorrowBps = %.6f, want %.6f", result.BorrowBps, wantBorrowBps)
	}
	if diff := result.NetBps - wantNetBps; diff > eps || diff < -eps {
		t.Fatalf("NetBps = %.6f, want %.6f (sign-math bug: expected -FundingBps-BorrowBps)", result.NetBps, wantNetBps)
	}
}

func TestSpotBacktestDirA_UnsupportedExchange(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestDays = 7
	e.cfg.SpotFuturesBacktestMinProfit = 999.0 // impossibly high threshold

	for _, exchName := range []string{"okx", "bitget", "bingx"} {
		opp := SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: exchName, Direction: "borrow_sell_long"}
		pass, reason := e.backtestDirA(opp)
		if !pass {
			t.Errorf("unsupported exchange %q should fail-open, got reason %q", exchName, reason)
		}
	}
}

func TestExchangeSupportsDirABacktest(t *testing.T) {
	supported := []string{"binance", "bybit", "gateio"}
	unsupported := []string{"okx", "bitget", "bingx", ""}

	for _, ex := range supported {
		if !exchangeSupportsDirABacktest(ex) {
			t.Errorf("%q should be supported for Dir A backtest", ex)
		}
	}
	for _, ex := range unsupported {
		if exchangeSupportsDirABacktest(ex) {
			t.Errorf("%q should NOT be supported for Dir A backtest", ex)
		}
	}
}
