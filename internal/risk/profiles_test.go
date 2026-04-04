package risk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"arb/internal/config"
)

func TestProfileApplicationBalanced(t *testing.T) {
	cfg := &config.Config{}
	if err := ApplyProfile(cfg, "balanced"); err != nil {
		t.Fatalf("ApplyProfile(balanced): %v", err)
	}
	if cfg.MaxPositions != 3 {
		t.Errorf("MaxPositions: got %d, want 3", cfg.MaxPositions)
	}
	if cfg.Leverage != 3 {
		t.Errorf("Leverage: got %d, want 3", cfg.Leverage)
	}
	if cfg.MaxCostRatio != 0.50 {
		t.Errorf("MaxCostRatio: got %f, want 0.50", cfg.MaxCostRatio)
	}
	if cfg.SpotFuturesMinNetYieldAPR != 0.10 {
		t.Errorf("MinNetYieldAPR: got %f, want 0.10", cfg.SpotFuturesMinNetYieldAPR)
	}
	if cfg.MaxPerpPerpPct != 0.50 {
		t.Errorf("MaxPerpPerpPct: got %f, want 0.50", cfg.MaxPerpPerpPct)
	}
	if cfg.MaxSpotFuturesPct != 0.50 {
		t.Errorf("MaxSpotFuturesPct: got %f, want 0.50", cfg.MaxSpotFuturesPct)
	}
	if cfg.SizeMultiplier != 1.0 {
		t.Errorf("SizeMultiplier: got %f, want 1.0", cfg.SizeMultiplier)
	}
	if cfg.RiskProfile != "balanced" {
		t.Errorf("RiskProfile: got %q, want %q", cfg.RiskProfile, "balanced")
	}
}

func TestProfileApplicationConservative(t *testing.T) {
	cfg := &config.Config{}
	if err := ApplyProfile(cfg, "conservative"); err != nil {
		t.Fatalf("ApplyProfile(conservative): %v", err)
	}
	if cfg.MaxPositions != 1 {
		t.Errorf("MaxPositions: got %d, want 1", cfg.MaxPositions)
	}
	if cfg.Leverage != 2 {
		t.Errorf("Leverage: got %d, want 2", cfg.Leverage)
	}
	if cfg.MaxCostRatio != 0.30 {
		t.Errorf("MaxCostRatio: got %f, want 0.30", cfg.MaxCostRatio)
	}
	if cfg.SpotFuturesMinNetYieldAPR != 0.15 {
		t.Errorf("MinNetYieldAPR: got %f, want 0.15", cfg.SpotFuturesMinNetYieldAPR)
	}
	if cfg.MaxPerpPerpPct != 0.60 {
		t.Errorf("MaxPerpPerpPct: got %f, want 0.60", cfg.MaxPerpPerpPct)
	}
	if cfg.MaxSpotFuturesPct != 0.40 {
		t.Errorf("MaxSpotFuturesPct: got %f, want 0.40", cfg.MaxSpotFuturesPct)
	}
	if cfg.SizeMultiplier != 0.7 {
		t.Errorf("SizeMultiplier: got %f, want 0.7", cfg.SizeMultiplier)
	}
	if cfg.RiskProfile != "conservative" {
		t.Errorf("RiskProfile: got %q, want %q", cfg.RiskProfile, "conservative")
	}
}

func TestProfileApplicationAggressive(t *testing.T) {
	cfg := &config.Config{}
	if err := ApplyProfile(cfg, "aggressive"); err != nil {
		t.Fatalf("ApplyProfile(aggressive): %v", err)
	}
	if cfg.MaxPositions != 5 {
		t.Errorf("MaxPositions: got %d, want 5", cfg.MaxPositions)
	}
	if cfg.Leverage != 5 {
		t.Errorf("Leverage: got %d, want 5", cfg.Leverage)
	}
	if cfg.MaxCostRatio != 0.70 {
		t.Errorf("MaxCostRatio: got %f, want 0.70", cfg.MaxCostRatio)
	}
	if cfg.SpotFuturesMinNetYieldAPR != 0.05 {
		t.Errorf("MinNetYieldAPR: got %f, want 0.05", cfg.SpotFuturesMinNetYieldAPR)
	}
	if cfg.MaxPerpPerpPct != 0.40 {
		t.Errorf("MaxPerpPerpPct: got %f, want 0.40", cfg.MaxPerpPerpPct)
	}
	if cfg.MaxSpotFuturesPct != 0.60 {
		t.Errorf("MaxSpotFuturesPct: got %f, want 0.60", cfg.MaxSpotFuturesPct)
	}
	if cfg.SizeMultiplier != 1.3 {
		t.Errorf("SizeMultiplier: got %f, want 1.3", cfg.SizeMultiplier)
	}
	if cfg.RiskProfile != "aggressive" {
		t.Errorf("RiskProfile: got %q, want %q", cfg.RiskProfile, "aggressive")
	}
}

func TestProfileApplicationInvalidName(t *testing.T) {
	cfg := &config.Config{
		MaxPositions: 99,
		RiskProfile:  "original",
	}
	if err := ApplyProfile(cfg, "invalid_name"); err == nil {
		t.Fatal("expected error for invalid profile name")
	}
	// Config should be unchanged
	if cfg.MaxPositions != 99 {
		t.Errorf("MaxPositions changed to %d after invalid profile", cfg.MaxPositions)
	}
	if cfg.RiskProfile != "original" {
		t.Errorf("RiskProfile changed to %q after invalid profile", cfg.RiskProfile)
	}
}

func TestProfileBundledFields(t *testing.T) {
	fields := ProfileBundledFields()
	if len(fields) == 0 {
		t.Fatal("ProfileBundledFields returned empty slice")
	}
	// Should contain known paths
	expected := map[string]bool{
		"fund.max_positions":                 false,
		"fund.leverage":                      false,
		"strategy.discovery.max_cost_ratio":  false,
		"spot_futures.min_net_yield_apr":     false,
		"risk.max_perp_perp_pct":            false,
		"risk.max_spot_futures_pct":          false,
		"allocation.size_multiplier":         false,
	}
	for _, f := range fields {
		if _, ok := expected[f]; ok {
			expected[f] = true
		}
	}
	for path, found := range expected {
		if !found {
			t.Errorf("expected %q in ProfileBundledFields, not found", path)
		}
	}
}

func TestConfigAllocationRoundTrip(t *testing.T) {
	// Create a temp config file with allocation values
	configPath := filepath.Join(t.TempDir(), "config.json")
	initial := `{
  "allocation": {
    "enable_unified_capital": true,
    "total_capital_usdt": 5000.0,
    "risk_profile": "aggressive",
    "allocation_lookback_days": 14,
    "allocation_floor_pct": 0.25,
    "allocation_ceiling_pct": 0.75,
    "size_multiplier": 1.5
  }
}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := config.Load()

	if !cfg.EnableUnifiedCapital {
		t.Error("EnableUnifiedCapital not loaded from JSON")
	}
	if cfg.TotalCapitalUSDT != 5000.0 {
		t.Errorf("TotalCapitalUSDT: got %f, want 5000", cfg.TotalCapitalUSDT)
	}
	if cfg.RiskProfile != "aggressive" {
		t.Errorf("RiskProfile: got %q, want aggressive", cfg.RiskProfile)
	}
	if cfg.AllocationLookbackDays != 14 {
		t.Errorf("AllocationLookbackDays: got %d, want 14", cfg.AllocationLookbackDays)
	}
	if cfg.AllocationFloorPct != 0.25 {
		t.Errorf("AllocationFloorPct: got %f, want 0.25", cfg.AllocationFloorPct)
	}
	if cfg.AllocationCeilingPct != 0.75 {
		t.Errorf("AllocationCeilingPct: got %f, want 0.75", cfg.AllocationCeilingPct)
	}
	if cfg.SizeMultiplier != 1.5 {
		t.Errorf("SizeMultiplier: got %f, want 1.5", cfg.SizeMultiplier)
	}

	// SaveJSON should persist them back
	if err := cfg.SaveJSON(); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	// Re-read and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	alloc, ok := raw["allocation"].(map[string]interface{})
	if !ok {
		t.Fatal("allocation section missing in saved JSON")
	}
	if v, ok := alloc["total_capital_usdt"].(float64); !ok || v != 5000.0 {
		t.Errorf("saved total_capital_usdt: got %v", alloc["total_capital_usdt"])
	}
	if v, ok := alloc["risk_profile"].(string); !ok || v != "aggressive" {
		t.Errorf("saved risk_profile: got %v", alloc["risk_profile"])
	}
}

func TestConfigAllocationDefaults(t *testing.T) {
	// Use empty config file to test defaults
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := config.Load()

	if cfg.EnableUnifiedCapital != false {
		t.Error("EnableUnifiedCapital default should be false")
	}
	if cfg.TotalCapitalUSDT != 0 {
		t.Errorf("TotalCapitalUSDT default: got %f, want 0", cfg.TotalCapitalUSDT)
	}
	if cfg.RiskProfile != "balanced" {
		t.Errorf("RiskProfile default: got %q, want balanced", cfg.RiskProfile)
	}
	if cfg.AllocationLookbackDays != 7 {
		t.Errorf("AllocationLookbackDays default: got %d, want 7", cfg.AllocationLookbackDays)
	}
	if cfg.AllocationFloorPct != 0.20 {
		t.Errorf("AllocationFloorPct default: got %f, want 0.20", cfg.AllocationFloorPct)
	}
	if cfg.AllocationCeilingPct != 0.80 {
		t.Errorf("AllocationCeilingPct default: got %f, want 0.80", cfg.AllocationCeilingPct)
	}
	if cfg.SizeMultiplier != 1.0 {
		t.Errorf("SizeMultiplier default: got %f, want 1.0", cfg.SizeMultiplier)
	}
}
