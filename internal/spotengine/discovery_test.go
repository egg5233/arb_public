package spotengine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/internal/scraper"
	"arb/pkg/exchange"
)

// TestDiscoveryMaintenanceRate_PopulatedFromContracts verifies that the
// native scanner populates MaintenanceRate on SpotArbOpportunity from
// ContractInfo via the getMaintenanceRate pathway.
func TestDiscoveryMaintenanceRate_PopulatedFromContracts(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	// Stub exchange that also implements maintenanceRateProvider.
	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "BTC",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	// Provide a mock exchange with known ContractInfo.MaintenanceRate.
	engine.exchanges = map[string]exchange.Exchange{
		"binance": &mockExchange{
			contracts: map[string]exchange.ContractInfo{
				"BTCUSDT": {MaintenanceRate: 0.005},
			},
		},
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:         "native",
		SpotFuturesMinNetYieldAPR:      0.0,
		SpotFuturesCapitalSeparate:     200,
		SpotFuturesLeverage:            3,
		SpotFuturesMaintenanceDefault:  0.05,
		SpotFuturesMaintenanceCacheTTL: 60,
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
	if len(opps) == 0 {
		t.Fatal("expected opportunities, got none")
	}

	for _, opp := range opps {
		if opp.MaintenanceRate != 0.005 {
			t.Errorf("%s %s: MaintenanceRate = %v, want 0.005", opp.Symbol, opp.Direction, opp.MaintenanceRate)
		}
	}
}

// TestDiscoveryMaintenanceRate_ZeroWhenContractMissing verifies that when
// the contract is not in LoadAllContracts, the maintenance rate falls back
// to the configured default (not zero) which is the safe-display behavior.
func TestDiscoveryMaintenanceRate_ZeroWhenContractMissing(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "XYZ",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	// Exchange has empty contracts map -> MaintenanceRate from ContractInfo = 0 -> falls back to default.
	engine.exchanges = map[string]exchange.Exchange{
		"binance": &mockExchange{
			contracts: map[string]exchange.ContractInfo{},
		},
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:         "native",
		SpotFuturesMinNetYieldAPR:      0.0,
		SpotFuturesCapitalSeparate:     200,
		SpotFuturesLeverage:            3,
		SpotFuturesMaintenanceDefault:  0.05,
		SpotFuturesMaintenanceCacheTTL: 60,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"XYZ"},
		map[string]map[string]float64{
			"binance": {"XYZ": 10.0},
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)
	if len(opps) == 0 {
		t.Fatal("expected opportunities, got none")
	}

	// When contract is missing, getMaintenanceRate falls back to default 0.05.
	for _, opp := range opps {
		if opp.MaintenanceRate != 0.05 {
			t.Errorf("%s %s: MaintenanceRate = %v, want 0.05 (default)", opp.Symbol, opp.Direction, opp.MaintenanceRate)
		}
	}
}

// TestDiscoveryMaintenanceRate_NotUsedForScoring verifies that MaintenanceRate
// does NOT affect FilterStatus (display only per D-15).
func TestDiscoveryMaintenanceRate_NotUsedForScoring(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	stubExch := &nativeScannerStubExchange{
		borrowRate: &exchange.MarginInterestRate{
			Coin:       "GUA",
			HourlyRate: 0.00001,
		},
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"binance": stubExch,
	}
	// Very high maintenance rate (30% — GUAUSDT-like case).
	engine.exchanges = map[string]exchange.Exchange{
		"binance": &mockExchange{
			contracts: map[string]exchange.ContractInfo{
				"GUAUSDT": {MaintenanceRate: 0.30},
			},
		},
	}
	engine.cfg = &config.Config{
		SpotFuturesScannerMode:         "native",
		SpotFuturesMinNetYieldAPR:      0.0,
		SpotFuturesCapitalSeparate:     200,
		SpotFuturesLeverage:            3,
		SpotFuturesMaintenanceDefault:  0.05,
		SpotFuturesMaintenanceCacheTTL: 60,
	}

	lorisResp := buildTestLorisResponse(
		[]string{"GUA"},
		map[string]map[string]float64{
			"binance": {"GUA": 50.0}, // high funding to pass yield filter
		},
	)

	server := mockLorisServer(lorisResp)
	defer server.Close()

	opps := engine.runNativeDiscoveryScanWithURL(server.URL)

	// Find Dir B (buy_spot_short) — should have no FilterStatus despite high maintenance rate.
	for _, opp := range opps {
		if opp.Direction == "buy_spot_short" && opp.Symbol == "GUAUSDT" {
			if opp.MaintenanceRate != 0.30 {
				t.Errorf("MaintenanceRate = %v, want 0.30", opp.MaintenanceRate)
			}
			if opp.FilterStatus != "" {
				t.Errorf("FilterStatus = %q, want empty (maintenance_rate is display only)", opp.FilterStatus)
			}
			return
		}
	}
	t.Fatal("expected Dir B GUAUSDT opportunity")
}

func TestRunDiscoveryScan_KeepsActivePositionWithNonPositiveFunding(t *testing.T) {
	tests := []struct {
		name string
		apr  string
		want float64
	}{
		{name: "zero funding", apr: "0.00%", want: 0},
		{name: "negative funding", apr: "-5.00%", want: -0.05},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, mr := newExecutionTestEngine(t)
			defer mr.Close()

			engine.cfg = &config.Config{SpotFuturesScannerMode: "coinglass"}
			engine.spotMargin = map[string]exchange.SpotMarginExchange{
				"binance": &marginStubExchange{},
			}

			pos := &models.SpotFuturesPosition{
				ID:        "pos-1",
				Symbol:    "BTCUSDT",
				BaseCoin:  "BTC",
				Exchange:  "binance",
				Direction: "buy_spot_short",
				Status:    models.SpotStatusActive,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := engine.db.SaveSpotPosition(pos); err != nil {
				t.Fatalf("SaveSpotPosition: %v", err)
			}

			payload := scraper.Payload{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data: []scraper.Opportunity{
					{
						Symbol:    "BTC",
						Portfolio: "Buy BTC",
						Exchange:  "Binance",
						APR:       tc.apr,
					},
				},
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
				t.Fatalf("SetWithTTL: %v", err)
			}

			opps := engine.runDiscoveryScan()
			if len(opps) != 1 {
				t.Fatalf("runDiscoveryScan returned %d opportunities, want 1", len(opps))
			}

			opp := opps[0]
			if opp.FundingAPR != tc.want {
				t.Fatalf("FundingAPR = %.4f, want %.4f", opp.FundingAPR, tc.want)
			}
			if !strings.Contains(opp.FilterStatus, "funding") {
				t.Fatalf("FilterStatus = %q, want funding filter", opp.FilterStatus)
			}

			engine.oppsMu.Lock()
			engine.latestOpps = opps
			engine.oppsMu.Unlock()
			if _, found := engine.lookupCurrentOpp("BTCUSDT", "binance", "buy_spot_short"); !found {
				t.Fatal("lookupCurrentOpp did not retain active position with non-positive funding")
			}
		})
	}
}

func TestCoinGlassDiscoveryCachesMissingSpotMarketAcrossRestart(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{SpotFuturesScannerMode: "coinglass"}
	stub := &nativeScannerStubExchange{
		bboErr: fmt.Errorf("GetSpotBBO: no OKX spot market for ONTUSDT"),
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"okx": stub,
	}

	payload := scraper.Payload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: []scraper.Opportunity{
			{
				Symbol:    "ONT",
				Portfolio: "Buy ONT",
				Exchange:  "OKX",
				APR:       "12.50%",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	opps := engine.runDiscoveryScan()
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}
	if opps[0].FilterStatus != "spot market unavailable" {
		t.Fatalf("expected spot market unavailable, got %q", opps[0].FilterStatus)
	}
	if stub.bboCalls != 1 {
		t.Fatalf("expected one GetSpotBBO probe for CoinGlass scan, got %d", stub.bboCalls)
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
		cfg:       &config.Config{SpotFuturesScannerMode: "coinglass"},
		db:        engine.db,
		log:       engine.log,
		stopCh:    make(chan struct{}),
		exitState: exitState{exiting: make(map[string]bool)},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"okx": &nativeScannerStubExchange{},
		},
	}

	opps = engine2.runDiscoveryScan()
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity after restart, got %d", len(opps))
	}
	if opps[0].FilterStatus != "spot market unavailable" {
		t.Fatalf("expected cached spot market unavailable after restart, got %q", opps[0].FilterStatus)
	}
	if secondStub := engine2.spotMargin["okx"].(*nativeScannerStubExchange); secondStub.bboCalls != 0 {
		t.Fatalf("expected no GetSpotBBO call after restart cache hit, got %d", secondStub.bboCalls)
	}
}

func TestCoinGlassDiscoveryAbortsQuicklyOnShutdown(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{SpotFuturesScannerMode: "coinglass"}
	stub := &nativeScannerStubExchange{}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"okx": stub,
	}

	payload := scraper.Payload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: []scraper.Opportunity{
			{Symbol: "ONT", Portfolio: "Buy ONT", Exchange: "OKX", APR: "12.50%"},
			{Symbol: "BTC", Portfolio: "Buy BTC", Exchange: "OKX", APR: "12.50%"},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	close(engine.stopCh)

	opps := engine.runDiscoveryScan()
	if len(opps) != 0 {
		t.Fatalf("expected no opportunities after shutdown, got %d", len(opps))
	}
	if stub.bboCalls != 0 {
		t.Fatalf("expected no GetSpotBBO calls after shutdown, got %d", stub.bboCalls)
	}
}
