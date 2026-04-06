package config

import (
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

