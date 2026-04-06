package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestSaveJSON_PreservesExistingExchangeCredentialsWhenRuntimeEmpty(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := `{
  "dry_run": false,
  "exchanges": {
    "binance": {
      "api_key": "original-key",
      "secret_key": "original-secret",
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	enabled := true
	cfg := &Config{
		DryRun:         true,
		BinanceEnabled: &enabled,
	}
	if err := cfg.SaveJSON(); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	exchanges, ok := saved["exchanges"].(map[string]interface{})
	if !ok {
		t.Fatalf("exchanges missing or wrong type: %#v", saved["exchanges"])
	}
	binance, ok := exchanges["binance"].(map[string]interface{})
	if !ok {
		t.Fatalf("binance missing or wrong type: %#v", exchanges["binance"])
	}
	if got := binance["api_key"]; got != "original-key" {
		t.Fatalf("expected api_key to be preserved, got %#v", got)
	}
	if got := binance["secret_key"]; got != "original-secret" {
		t.Fatalf("expected secret_key to be preserved, got %#v", got)
	}
	if got := saved["dry_run"]; got != true {
		t.Fatalf("expected dry_run update to persist, got %#v", got)
	}

	backupData, err := os.ReadFile(configPath + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != original {
		t.Fatalf("backup mismatch: got %q", string(backupData))
	}
}

func TestSaveJSON_PreservesOnDiskExchangeCredentialsWhenRuntimeIsStale(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := `{
  "dry_run": false,
  "exchanges": {
    "binance": {
      "api_key": "file-new-key",
      "secret_key": "file-new-secret",
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	enabled := true
	cfg := &Config{
		DryRun:           true,
		BinanceAPIKey:    "runtime-old-key",
		BinanceSecretKey: "runtime-old-secret",
		BinanceEnabled:   &enabled,
	}
	if err := cfg.SaveJSON(); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	exchanges, ok := saved["exchanges"].(map[string]interface{})
	if !ok {
		t.Fatalf("exchanges missing or wrong type: %#v", saved["exchanges"])
	}
	binance, ok := exchanges["binance"].(map[string]interface{})
	if !ok {
		t.Fatalf("binance missing or wrong type: %#v", exchanges["binance"])
	}
	if got := binance["api_key"]; got != "file-new-key" {
		t.Fatalf("expected api_key to preserve on-disk value, got %#v", got)
	}
	if got := binance["secret_key"]; got != "file-new-secret" {
		t.Fatalf("expected secret_key to preserve on-disk value, got %#v", got)
	}
	if got := saved["dry_run"]; got != true {
		t.Fatalf("expected dry_run update to persist, got %#v", got)
	}
}

func TestSaveJSON_FailsWhenBackupCannotBeWritten(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := `{
  "dry_run": false,
  "exchanges": {
    "binance": {
      "api_key": "original-key",
      "secret_key": "original-secret",
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Mkdir(configPath+".bak", 0755); err != nil {
		t.Fatalf("mkdir backup blocker: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := &Config{DryRun: true}
	err := cfg.SaveJSON()
	if err == nil {
		t.Fatal("expected backup write failure")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "write backup config") {
		t.Fatalf("expected backup error, got %v", err)
	}

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("expected config to remain unchanged, got %q", string(data))
	}
}
