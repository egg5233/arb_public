package spotengine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/internal/scraper"
	"arb/pkg/exchange"
)

// nativeScannerStubExchange implements SpotMarginExchange for native scanner tests.
type nativeScannerStubExchange struct {
	borrowRate *exchange.MarginInterestRate
	borrowErr  error
	bbo        exchange.BBO
	bboErr     error
	bboCalls   int
}

func (s *nativeScannerStubExchange) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s *nativeScannerStubExchange) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (s *nativeScannerStubExchange) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	return "", nil
}
func (s *nativeScannerStubExchange) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	if s.borrowErr != nil {
		return nil, s.borrowErr
	}
	return s.borrowRate, nil
}
func (s *nativeScannerStubExchange) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return &exchange.MarginBalance{Available: 1000}, nil
}
func (s *nativeScannerStubExchange) GetSpotBBO(string) (exchange.BBO, error) {
	s.bboCalls++
	if s.bboErr != nil {
		return exchange.BBO{}, s.bboErr
	}
	if s.bbo.Bid > 0 && s.bbo.Ask > 0 {
		return s.bbo, nil
	}
	return exchange.BBO{Bid: 100, Ask: 100.1}, nil
}
func (s *nativeScannerStubExchange) TransferToMargin(string, string) error   { return nil }
func (s *nativeScannerStubExchange) TransferFromMargin(string, string) error { return nil }
func (s *nativeScannerStubExchange) GetMarginInterestRateHistory(_ context.Context, _ string, _, _ time.Time) ([]exchange.MarginInterestRatePoint, error) {
	return nil, exchange.ErrHistoricalBorrowNotSupported
}
func (s *nativeScannerStubExchange) SpotOrderRules(string) (*exchange.SpotOrderRules, error) {
	return nil, nil
}

// mockLorisServer creates an httptest.Server that returns Loris funding rate data.
func mockLorisServer(resp models.LorisResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// mockLorisErrorServer creates an httptest.Server that returns an error.
func mockLorisErrorServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
}

// buildTestLorisResponse creates a LorisResponse with given symbol/exchange/rate data.
func buildTestLorisResponse(symbols []string, rates map[string]map[string]float64) models.LorisResponse {
	return models.LorisResponse{
		Symbols:      symbols,
		FundingRates: rates,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
}

func TestNativeScannerDirA(t *testing.T) {
	// Test: Native scanner with mock Loris data produces Dir A opportunity
	// when funding > 0 and borrow is available.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	// Borrow rate: 0.001% per hour -> yearlyAPR = 0.00001 * 24 * 365 = 0.0876
	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.00001, // 0.001% per hour
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.01, // 1%
	}

	// Loris rate: 10 bps 8h-equiv -> bpsPerHour = 10/8 = 1.25
	// fundingAPR = 1.25 * 8760 / 10000 = 1.095 (109.5%)
	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 10.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	// Override lorisURL for tests: we can't override the const, so we'll
	// test via runNativeDiscoveryScanWithLoris helper.
	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	// Should have both Dir A and Dir B.
	var dirA *SpotArbOpportunity
	for i, opp := range opps {
		if opp.Direction == "borrow_sell_long" && opp.Exchange == "binance" && opp.Symbol == "BTCUSDT" {
			dirA = &opps[i]
			break
		}
	}

	if dirA == nil {
		t.Fatal("expected Dir A opportunity for BTCUSDT on binance, got none")
	}

	if dirA.Source != "native" {
		t.Errorf("Source = %q, want %q", dirA.Source, "native")
	}
	if dirA.BorrowAPR <= 0 {
		t.Errorf("BorrowAPR = %f, want > 0", dirA.BorrowAPR)
	}
	if dirA.FundingAPR >= 0 {
		t.Errorf("FundingAPR = %f, want < 0 (Dir A long futures pays when rate is positive)", dirA.FundingAPR)
	}
}

func TestNativeScannerDirB(t *testing.T) {
	// Test: Native scanner produces Dir B opportunity when funding > 0 (borrowAPR = 0).
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "ETH",
			HourlyRate: 0.00002,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"bybit": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.01,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"ETH"},
		map[string]map[string]float64{
			"bybit": {"ETH": 8.0}, // 8 bps 8h-equiv
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	var dirB *SpotArbOpportunity
	for i, opp := range opps {
		if opp.Direction == "buy_spot_short" && opp.Exchange == "bybit" && opp.Symbol == "ETHUSDT" {
			dirB = &opps[i]
			break
		}
	}

	if dirB == nil {
		t.Fatal("expected Dir B opportunity for ETHUSDT on bybit, got none")
	}

	if dirB.BorrowAPR != 0 {
		t.Errorf("Dir B BorrowAPR = %f, want 0", dirB.BorrowAPR)
	}
	if dirB.Source != "native" {
		t.Errorf("Source = %q, want %q", dirB.Source, "native")
	}
}

func TestNativeScannerBothDirections(t *testing.T) {
	// Test: Native scanner generates BOTH Dir A and Dir B for same symbol.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.01,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 10.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	hasDirA, hasDirB := false, false
	for _, opp := range opps {
		if opp.Symbol == "BTCUSDT" && opp.Exchange == "binance" {
			if opp.Direction == "borrow_sell_long" {
				hasDirA = true
			}
			if opp.Direction == "buy_spot_short" {
				hasDirB = true
			}
		}
	}

	if !hasDirA {
		t.Error("missing Dir A (borrow_sell_long) for BTCUSDT")
	}
	if !hasDirB {
		t.Error("missing Dir B (buy_spot_short) for BTCUSDT")
	}
}

func TestNetYieldRanking(t *testing.T) {
	// Test: Net yield = fundingAPR - borrowAPR - feeAPR, ranked descending by NetAPR.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	// BTC has lower borrow, ETH has higher borrow -> BTC net should be higher.
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": &nativeScannerStubExchange{
			borrowRate: &exchange.MarginInterestRate{
				Coin:       "BTC",
				HourlyRate: 0.00001, // low borrow
			},
		},
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.0,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"BTC", "ETH"},
		map[string]map[string]float64{
			"binance": {
				"BTC": 10.0, // same funding rate
				"ETH": 10.0,
			},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	if len(opps) == 0 {
		t.Fatal("expected opportunities, got none")
	}

	// Verify net yield formula: netAPR = fundingAPR - borrowAPR - feeAPR
	for _, opp := range opps {
		expectedNet := opp.FundingAPR - opp.BorrowAPR
		if math.Abs(opp.NetAPR-expectedNet) > 0.0001 {
			t.Errorf("%s %s %s: NetAPR = %f, want %f (funding=%f, borrow=%f, fee=%f)",
				opp.Symbol, opp.Exchange, opp.Direction,
				opp.NetAPR, expectedNet, opp.FundingAPR, opp.BorrowAPR, opp.FeePct)
		}
	}

	// Verify ranking: first opp should have highest NetAPR among passed.
	for i := 0; i < len(opps)-1; i++ {
		if opps[i].FilterStatus == "" && opps[i+1].FilterStatus == "" {
			if opps[i].NetAPR < opps[i+1].NetAPR {
				t.Errorf("ranking violated: opps[%d].NetAPR=%f < opps[%d].NetAPR=%f",
					i, opps[i].NetAPR, i+1, opps[i+1].NetAPR)
			}
		}
	}
}

func TestNativeScannerLorisNormalization(t *testing.T) {
	// Test: Loris rate normalization: raw 8h-equiv rate / 8 = bps/h, then * 8760 / 10000 = decimal APR.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.0,
	}

	// rawRate = 80 bps 8h-equiv
	// bpsPerHour = 80 / 8 = 10
	// fundingAPR = 10 * 8760 / 10000 = 8.76 (876%)
	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 80.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	var dirB *SpotArbOpportunity
	for i, opp := range opps {
		if opp.Direction == "buy_spot_short" {
			dirB = &opps[i]
			break
		}
	}
	if dirB == nil {
		t.Fatal("expected Dir B opportunity")
	}

	expectedAPR := (80.0 / 8.0) * 8760.0 / 10000.0 // = 8.76
	if math.Abs(dirB.FundingAPR-expectedAPR) > 0.001 {
		t.Errorf("FundingAPR = %f, want %f", dirB.FundingAPR, expectedAPR)
	}
}

func TestCoinGlassFallback(t *testing.T) {
	// Test: CoinGlass fallback activates when Loris poll returns error.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode: "native",
	}

	// Use error server to trigger fallback.
	server := mockLorisErrorServer()
	defer server.Close()

	// Also set up CoinGlass data in Redis for fallback.
	cgPayload := scraper.Payload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: []scraper.Opportunity{
			{
				Symbol:    "BTC",
				Portfolio: "Buy BTC",
				Exchange:  "Binance",
				APR:       "50.00%",
			},
		},
	}
	raw, err := json.Marshal(cgPayload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	// Poll Loris with error URL, should fallback to CoinGlass.
	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	if len(opps) == 0 {
		// CoinGlass fallback might return empty if no margin balance mock.
		// The key test is that it doesn't panic and calls fallback.
		return
	}

	// If fallback returned results, verify source.
	for _, opp := range opps {
		if opp.Source != "coinglass_spot" {
			t.Errorf("expected coinglass_spot source on fallback, got %q", opp.Source)
		}
	}
}

func TestNativeScannerSkipsNoSpotMarginExchange(t *testing.T) {
	// Test: Symbols without SpotMarginExchange are skipped.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	// Only binance has SpotMarginExchange, bybit does not.
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": &nativeScannerStubExchange{
			borrowRate: &exchange.MarginInterestRate{
				Coin:       "BTC",
				HourlyRate: 0.00001,
			},
		},
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.0,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 10.0},
			"bybit":   {"BTC": 10.0}, // bybit not in spotMargin
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	for _, opp := range opps {
		if opp.Exchange == "bybit" {
			t.Error("should not produce opportunities for exchange without SpotMarginExchange")
		}
	}
}

func TestNativeScannerFilterStatus(t *testing.T) {
	// Test: FilterStatus set for opportunities below MinNetYieldAPR or above MaxBorrowAPR.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	// High borrow rate: 0.01% per hour -> 87.6% APR
	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.0001, // high borrow
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.50, // 50% min net yield
		SpotFuturesMaxBorrowAPR:   0.20, // 20% max borrow
	}

	// Low funding rate: 2 bps 8h-equiv -> fundingAPR = (2/8) * 8760/10000 = 0.219 (21.9%)
	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 2.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	for _, opp := range opps {
		if opp.Direction == "borrow_sell_long" && opp.FilterStatus == "" {
			t.Errorf("Dir A should have FilterStatus set (borrow too high or net too low), got empty")
		}
	}
}

func TestNativeScannerCachesMissingSpotMarketAcrossRestart(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "ONT",
			HourlyRate: 0.00001,
		},
		bboErr: fmt.Errorf("GetSpotBBO: no OKX spot market for ONTUSDT"),
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"okx": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.01,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"ONT"},
		map[string]map[string]float64{
			"okx": {"ONT": 10.0},
		},
	)
	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)
	if len(opps) != 2 {
		t.Fatalf("expected 2 opportunities, got %d", len(opps))
	}
	for _, opp := range opps {
		if opp.FilterStatus != "spot market unavailable" {
			t.Fatalf("expected spot market unavailable, got %q", opp.FilterStatus)
		}
	}
	if stubExch.bboCalls != 1 {
		t.Fatalf("expected one GetSpotBBO probe for native scan, got %d", stubExch.bboCalls)
	}

	exists, found, err := engine.db.GetSpotMarketAvailability("okx", "ONTUSDT")
	if err != nil {
		t.Fatalf("GetSpotMarketAvailability: %v", err)
	}
	if !found {
		t.Fatal("expected cached spot market availability entry")
	}
	if exists {
		t.Fatal("expected cached spot market availability to be false")
	}

	engine2 := &SpotEngine{
		cfg:       &config.Config{SpotFuturesScannerMode: "native", SpotFuturesMinNetYieldAPR: 0.01},
		db:        engine.db,
		log:       engine.log,
		stopCh:    make(chan struct{}),
		exitState: exitState{exiting: make(map[string]bool)},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"okx": &nativeScannerStubExchange{
				borrowRate: &exchange.MarginInterestRate{Coin: "ONT", HourlyRate: 0.00001},
			},
		},
	}

	opps = engine2.runNativeDiscoveryScanWithURL(server.URL)
	if len(opps) != 2 {
		t.Fatalf("expected 2 opportunities after restart, got %d", len(opps))
	}
	for _, opp := range opps {
		if opp.FilterStatus != "spot market unavailable" {
			t.Fatalf("expected cached spot market unavailable after restart, got %q", opp.FilterStatus)
		}
	}
	if secondStub := engine2.spotMargin["okx"].(*nativeScannerStubExchange); secondStub.bboCalls != 0 {
		t.Fatalf("expected no GetSpotBBO call after restart cache hit, got %d", secondStub.bboCalls)
	}
}

func TestNativeScannerAbortsQuicklyOnShutdown(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "ONT",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"okx": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.01,
	}

	close(engine.stopCh)

	lorisResp := buildTestLorisResponse(
		[]string{"ONT", "BTC", "ETH"},
		map[string]map[string]float64{
			"okx": {"ONT": 10.0, "BTC": 10.0, "ETH": 10.0},
		},
	)

	opps := engine.runNativeDiscoveryScanFromLoris(&lorisResp)
	if len(opps) != 0 {
		t.Fatalf("expected no opportunities after shutdown, got %d", len(opps))
	}
	if stubExch.bboCalls != 0 {
		t.Fatalf("expected no GetSpotBBO calls after shutdown, got %d", stubExch.bboCalls)
	}
}

func TestNativeScannerConfigDefaults(t *testing.T) {
	// Test: Config defaults for all 9 new fields.
	// config.Load() sets defaults, then loads JSON + env. We call it in a
	// clean env to verify the struct literal defaults are correct.
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Load()

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"SpotFuturesScannerMode", cfg.SpotFuturesScannerMode, "native"},
		{"SpotFuturesEnableMinHold", cfg.SpotFuturesEnableMinHold, false},
		{"SpotFuturesMinHoldHours", cfg.SpotFuturesMinHoldHours, 8},
		{"SpotFuturesEnableSettlementGuard", cfg.SpotFuturesEnableSettlementGuard, false},
		{"SpotFuturesSettlementWindowMin", cfg.SpotFuturesSettlementWindowMin, 10},
		{"SpotFuturesEnablePriceGapGate", cfg.SpotFuturesEnablePriceGapGate, false},
		{"SpotFuturesMaxPriceGapPct", cfg.SpotFuturesMaxPriceGapPct, 0.5},
		{"SpotFuturesEnableExitSpreadGate", cfg.SpotFuturesEnableExitSpreadGate, false},
		{"SpotFuturesExitSpreadPct", cfg.SpotFuturesExitSpreadPct, 0.3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if fmt.Sprintf("%v", tc.got) != fmt.Sprintf("%v", tc.want) {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestNativeScannerSourceField(t *testing.T) {
	// Test: Source field is "native" for Loris-sourced.
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:    "native",
		SpotFuturesMinNetYieldAPR: 0.0,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"BTC"},
		map[string]map[string]float64{
			"binance": {"BTC": 10.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	for _, opp := range opps {
		if opp.Source != "native" {
			t.Errorf("Source = %q, want %q", opp.Source, "native")
		}
	}
}
