package spotengine

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// mockMaintenanceProvider implements maintenanceRateProvider for testing.
type mockMaintenanceProvider struct {
	rate      float64
	err       error
	callCount int
}

func (m *mockMaintenanceProvider) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	m.callCount++
	return m.rate, m.err
}

// mockExchange is a minimal exchange.Exchange stub for testing.
// It also implements maintenanceRateProvider.
type mockExchange struct {
	exchange.Exchange // embed to satisfy interface (nil methods will panic if called)
	provider          *mockMaintenanceProvider
	contracts         map[string]exchange.ContractInfo
}

func (m *mockExchange) Name() string { return "mock" }

func (m *mockExchange) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	if m.provider != nil {
		return m.provider.GetMaintenanceRate(symbol, notionalUSDT)
	}
	return 0, nil
}

func (m *mockExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	if m.contracts != nil {
		return m.contracts, nil
	}
	return map[string]exchange.ContractInfo{}, nil
}

// mockExchangeNoProvider satisfies exchange.Exchange but does NOT implement
// maintenanceRateProvider (simulates BingX).
type mockExchangeNoProvider struct {
	exchange.Exchange
	contracts map[string]exchange.ContractInfo
}

func (m *mockExchangeNoProvider) Name() string { return "bingx" }

func (m *mockExchangeNoProvider) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return m.contracts, nil
}

// testCfg returns a config with maintenance rate defaults set.
func testCfg() *config.Config {
	cfg := &config.Config{}
	cfg.SpotFuturesMaintenanceDefault = 0.05
	cfg.SpotFuturesMaintenanceCacheTTL = 60
	return cfg
}

// testSpotEngine creates a minimal SpotEngine for maintenance rate testing.
func testSpotEngine(cfg *config.Config, exchanges map[string]exchange.Exchange) *SpotEngine {
	return &SpotEngine{
		exchanges: exchanges,
		cfg:       cfg,
		log:       utils.NewLogger("test-maintenance"),
	}
}

// TestMaintenanceRateCache_HitAndMiss verifies cache returns stored value
// and misses after TTL expiry.
func TestMaintenanceRateCache_HitAndMiss(t *testing.T) {
	cache := newMaintenanceRateCache(50 * time.Millisecond)

	// Miss on empty cache
	_, ok := cache.get("btc")
	if ok {
		t.Error("expected cache miss on empty cache")
	}

	// Set and hit
	cache.set("btc", 0.005)
	rate, ok := cache.get("btc")
	if !ok {
		t.Error("expected cache hit after set")
	}
	if rate != 0.005 {
		t.Errorf("cached rate = %v, want 0.005", rate)
	}

	// Wait for TTL expiry
	time.Sleep(60 * time.Millisecond)
	_, ok = cache.get("btc")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

// TestGetMaintenanceRate_UsesCache verifies that the second call hits cache
// and the adapter is not called twice.
func TestGetMaintenanceRate_UsesCache(t *testing.T) {
	provider := &mockMaintenanceProvider{rate: 0.005}
	mock := &mockExchange{provider: provider}

	e := testSpotEngine(testCfg(), map[string]exchange.Exchange{"mock": mock})

	// First call: adapter should be called
	rate1 := e.getMaintenanceRate("BTCUSDT", "mock", 10000)
	if rate1 != 0.005 {
		t.Errorf("first call rate = %v, want 0.005", rate1)
	}
	if provider.callCount != 1 {
		t.Errorf("adapter call count = %d, want 1", provider.callCount)
	}

	// Second call: should hit cache
	rate2 := e.getMaintenanceRate("BTCUSDT", "mock", 10000)
	if rate2 != 0.005 {
		t.Errorf("second call rate = %v, want 0.005", rate2)
	}
	if provider.callCount != 1 {
		t.Errorf("adapter call count after cache hit = %d, want 1", provider.callCount)
	}
}

// TestGetMaintenanceRate_FallbackToDefault verifies that when rate=0,
// the conservative default (0.05) is returned.
func TestGetMaintenanceRate_FallbackToDefault(t *testing.T) {
	provider := &mockMaintenanceProvider{rate: 0} // returns 0 = unknown
	mock := &mockExchange{
		provider:  provider,
		contracts: map[string]exchange.ContractInfo{"BTCUSDT": {MaintenanceRate: 0}},
	}

	e := testSpotEngine(testCfg(), map[string]exchange.Exchange{"mock": mock})

	rate := e.getMaintenanceRate("BTCUSDT", "mock", 10000)
	if rate != 0.05 {
		t.Errorf("fallback rate = %v, want 0.05 (default)", rate)
	}
}

// TestGetMaintenanceRate_BoundsCheck verifies that negative and >= 1.0
// rates return the default.
func TestGetMaintenanceRate_BoundsCheck(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		wantRate float64
	}{
		{"negative rate", -0.5, 0.05},
		{"rate >= 1.0", 1.5, 0.05},
		{"rate exactly 1.0", 1.0, 0.05},
		{"valid rate", 0.01, 0.01},
		{"zero rate", 0.0, 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockMaintenanceProvider{rate: tt.rate}
			mock := &mockExchange{
				provider:  provider,
				contracts: map[string]exchange.ContractInfo{"BTCUSDT": {MaintenanceRate: 0}},
			}

			e := testSpotEngine(testCfg(), map[string]exchange.Exchange{"mock": mock})

			rate := e.getMaintenanceRate("BTCUSDT", "mock", 10000)
			if rate != tt.wantRate {
				t.Errorf("rate = %v, want %v", rate, tt.wantRate)
			}
		})
	}
}

// TestGetMaintenanceRate_FallbackToContractInfo verifies that when the
// adapter doesn't implement maintenanceRateProvider, the ContractInfo rate is used.
func TestGetMaintenanceRate_FallbackToContractInfo(t *testing.T) {
	noProvider := &mockExchangeNoProvider{
		contracts: map[string]exchange.ContractInfo{
			"BTCUSDT": {MaintenanceRate: 0.008},
		},
	}

	e := testSpotEngine(testCfg(), map[string]exchange.Exchange{"bingx": noProvider})

	rate := e.getMaintenanceRate("BTCUSDT", "bingx", 10000)
	if rate != 0.008 {
		t.Errorf("fallback to ContractInfo rate = %v, want 0.008", rate)
	}
}
