package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
