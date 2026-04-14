package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyJSON_StrategyScanMinutesRejectZero(t *testing.T) {
	// Minute 0 collides with normalScan at :00 and is never a valid typed scan
	// minute. Setting 0 in config.json should NOT overwrite defaults.
	cfg := &Config{
		EntryScanMinute:     40,
		ExitScanMinute:      30,
		RotateScanMinute:    35,
		RebalanceScanMinute: 10,
	}

	zero := 0
	jc := &jsonConfig{
		Strategy: &jsonStrategy{
			EntryScanMinute:     &zero,
			ExitScanMinute:      &zero,
			RotateScanMinute:    &zero,
			RebalanceScanMinute: &zero,
		},
	}

	cfg.applyJSON(jc)

	if cfg.EntryScanMinute != 40 {
		t.Fatalf("expected entry_scan_minute=40 (default preserved), got %d", cfg.EntryScanMinute)
	}
	if cfg.ExitScanMinute != 30 {
		t.Fatalf("expected exit_scan_minute=30 (default preserved), got %d", cfg.ExitScanMinute)
	}
	if cfg.RotateScanMinute != 35 {
		t.Fatalf("expected rotate_scan_minute=35 (default preserved), got %d", cfg.RotateScanMinute)
	}
	if cfg.RebalanceScanMinute != 10 {
		t.Fatalf("expected rebalance_scan_minute=10 (default preserved), got %d", cfg.RebalanceScanMinute)
	}
}

func TestApplyJSON_FundRebalanceDoesNotOverrideStrategy(t *testing.T) {
	cfg := &Config{RebalanceScanMinute: 10}

	strategyMinute := 35
	fundMinute := 20
	jc := &jsonConfig{
		Strategy: &jsonStrategy{
			RebalanceScanMinute: &strategyMinute,
		},
		Fund: &jsonFund{
			RebalanceScanMinute: &fundMinute,
		},
	}

	cfg.applyJSON(jc)

	if cfg.RebalanceScanMinute != 35 {
		t.Fatalf("expected strategy rebalance_scan_minute to win, got %d", cfg.RebalanceScanMinute)
	}
}

func TestApplyJSON_FundRebalanceFallbackWhenStrategyMissing(t *testing.T) {
	cfg := &Config{RebalanceScanMinute: 10}

	fundMinute := 20
	jc := &jsonConfig{
		Fund: &jsonFund{
			RebalanceScanMinute: &fundMinute,
		},
	}

	cfg.applyJSON(jc)

	if cfg.RebalanceScanMinute != 20 {
		t.Fatalf("expected fund rebalance_scan_minute=20 to apply as fallback, got %d", cfg.RebalanceScanMinute)
	}
}

// TestConfig_RoundTripUnifiedEntrySelectionFlag verifies that the
// allocation.enable_unified_entry_selection JSON key round-trips through
// Load() -> in-memory Config -> SaveJSON() -> Load() without loss.
func TestConfig_RoundTripUnifiedEntrySelectionFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	initial := `{
  "allocation": {
    "enable_unified_entry_selection": true
  }
}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)
	// Ensure env override does not mask JSON-driven load.
	t.Setenv("ENABLE_UNIFIED_ENTRY_SELECTION", "")

	cfg := Load()
	if !cfg.EnableUnifiedEntrySelection {
		t.Fatalf("expected EnableUnifiedEntrySelection=true after JSON load, got false")
	}

	// Save the in-memory config back to disk, then reload.
	if err := cfg.SaveJSON(); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	// Verify the raw JSON file contains the expected key with a true value.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}
	allocRaw, ok := decoded["allocation"].(map[string]interface{})
	if !ok {
		t.Fatalf("saved config missing allocation block: %#v", decoded["allocation"])
	}
	flagVal, ok := allocRaw["enable_unified_entry_selection"]
	if !ok {
		t.Fatalf("saved config missing enable_unified_entry_selection key")
	}
	if flagVal != true {
		t.Fatalf("saved config enable_unified_entry_selection: got %#v, want true", flagVal)
	}

	// Reload via Load(); flag must survive.
	reloaded := Load()
	if !reloaded.EnableUnifiedEntrySelection {
		t.Fatalf("reloaded EnableUnifiedEntrySelection lost after SaveJSON round-trip")
	}

	// Negative round-trip: toggle to false via applyJSON, reload must be false.
	cfg2 := &Config{EnableUnifiedEntrySelection: true}
	jc := &jsonConfig{
		Allocation: &jsonAllocation{
			EnableUnifiedEntrySelection: boolPtr(false),
		},
	}
	cfg2.applyJSON(jc)
	if cfg2.EnableUnifiedEntrySelection {
		t.Fatalf("applyJSON with false did not toggle the flag off")
	}
}

// TestConfig_EnvOverrideUnifiedEntrySelection verifies that the
// ENABLE_UNIFIED_ENTRY_SELECTION env var overrides JSON when set to true/1.
func TestConfig_EnvOverrideUnifiedEntrySelection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	// JSON explicitly sets the flag to false; env var must override to true.
	initial := `{
  "allocation": {
    "enable_unified_entry_selection": false
  }
}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("ENABLE_UNIFIED_ENTRY_SELECTION", "true")

	cfg := Load()
	if !cfg.EnableUnifiedEntrySelection {
		t.Fatalf("expected env var override to set EnableUnifiedEntrySelection=true, got false")
	}

	// Also accept "1" form.
	t.Setenv("ENABLE_UNIFIED_ENTRY_SELECTION", "1")
	cfg = Load()
	if !cfg.EnableUnifiedEntrySelection {
		t.Fatalf("expected env var override with '1' to set EnableUnifiedEntrySelection=true, got false")
	}

	// Empty env var must NOT override JSON (JSON wins -> false).
	t.Setenv("ENABLE_UNIFIED_ENTRY_SELECTION", "")
	cfg = Load()
	if cfg.EnableUnifiedEntrySelection {
		t.Fatalf("expected empty env var to leave JSON value (false), got true")
	}
}

// boolPtr is a small helper used only in tests within this package.
func boolPtr(b bool) *bool { return &b }

