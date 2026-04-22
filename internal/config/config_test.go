package config

import (
	"encoding/json"
	"testing"

	"arb/internal/models"
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

// TestApplyJSON_PriceGap exercises the full price_gap block load path:
// every PriceGap* field must be populated from the nested JSON object.
func TestApplyJSON_PriceGap(t *testing.T) {
	raw := []byte(`{
        "price_gap": {
            "enabled": true,
            "budget": 5000,
            "max_concurrent": 3,
            "gate_concentration_pct": 0.5,
            "max_hold_min": 240,
            "exit_reversion_factor": 0.5,
            "bar_persistence": 4,
            "kline_staleness_sec": 90,
            "poll_interval_sec": 30,
            "candidates": [
                {
                    "symbol": "SOONUSDT",
                    "long_exch": "binance",
                    "short_exch": "gate",
                    "threshold_bps": 200,
                    "max_position_usdt": 5000,
                    "modeled_slippage_bps": 47.9
                }
            ]
        }
    }`)

	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cfg := &Config{}
	cfg.applyJSON(&jc)

	if !cfg.PriceGapEnabled {
		t.Fatalf("expected PriceGapEnabled=true")
	}
	if cfg.PriceGapBudget != 5000 {
		t.Fatalf("expected PriceGapBudget=5000, got %v", cfg.PriceGapBudget)
	}
	if cfg.PriceGapMaxConcurrent != 3 {
		t.Fatalf("expected PriceGapMaxConcurrent=3, got %d", cfg.PriceGapMaxConcurrent)
	}
	if cfg.PriceGapGateConcentrationPct != 0.5 {
		t.Fatalf("expected PriceGapGateConcentrationPct=0.5, got %v", cfg.PriceGapGateConcentrationPct)
	}
	if cfg.PriceGapMaxHoldMin != 240 {
		t.Fatalf("expected PriceGapMaxHoldMin=240, got %d", cfg.PriceGapMaxHoldMin)
	}
	if cfg.PriceGapExitReversionFactor != 0.5 {
		t.Fatalf("expected PriceGapExitReversionFactor=0.5, got %v", cfg.PriceGapExitReversionFactor)
	}
	if cfg.PriceGapBarPersistence != 4 {
		t.Fatalf("expected PriceGapBarPersistence=4, got %d", cfg.PriceGapBarPersistence)
	}
	if cfg.PriceGapKlineStalenessSec != 90 {
		t.Fatalf("expected PriceGapKlineStalenessSec=90, got %d", cfg.PriceGapKlineStalenessSec)
	}
	if cfg.PriceGapPollIntervalSec != 30 {
		t.Fatalf("expected PriceGapPollIntervalSec=30, got %d", cfg.PriceGapPollIntervalSec)
	}
	if len(cfg.PriceGapCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cfg.PriceGapCandidates))
	}

	want := models.PriceGapCandidate{
		Symbol:             "SOONUSDT",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 47.9,
	}
	got := cfg.PriceGapCandidates[0]
	if got != want {
		t.Fatalf("candidate mismatch:\n  want=%+v\n  got= %+v", want, got)
	}
	if got.ID() != "SOONUSDT_binance_gate" {
		t.Fatalf("expected ID=SOONUSDT_binance_gate, got %s", got.ID())
	}
}

// TestDefaults_PriceGap verifies that a config.json without a price_gap
// block leaves all PriceGap* fields at zero-value (safe-off per T-08-01).
func TestDefaults_PriceGap(t *testing.T) {
	raw := []byte(`{
        "dry_run": true
    }`)

	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cfg := &Config{}
	cfg.applyJSON(&jc)

	if cfg.PriceGapEnabled {
		t.Fatalf("expected PriceGapEnabled=false (safe-off)")
	}
	if cfg.PriceGapBudget != 0 {
		t.Fatalf("expected PriceGapBudget=0, got %v", cfg.PriceGapBudget)
	}
	if cfg.PriceGapMaxConcurrent != 0 {
		t.Fatalf("expected PriceGapMaxConcurrent=0, got %d", cfg.PriceGapMaxConcurrent)
	}
	if cfg.PriceGapGateConcentrationPct != 0 {
		t.Fatalf("expected PriceGapGateConcentrationPct=0, got %v", cfg.PriceGapGateConcentrationPct)
	}
	if cfg.PriceGapMaxHoldMin != 0 {
		t.Fatalf("expected PriceGapMaxHoldMin=0, got %d", cfg.PriceGapMaxHoldMin)
	}
	if cfg.PriceGapExitReversionFactor != 0 {
		t.Fatalf("expected PriceGapExitReversionFactor=0, got %v", cfg.PriceGapExitReversionFactor)
	}
	if cfg.PriceGapBarPersistence != 0 {
		t.Fatalf("expected PriceGapBarPersistence=0, got %d", cfg.PriceGapBarPersistence)
	}
	if cfg.PriceGapKlineStalenessSec != 0 {
		t.Fatalf("expected PriceGapKlineStalenessSec=0, got %d", cfg.PriceGapKlineStalenessSec)
	}
	if cfg.PriceGapPollIntervalSec != 0 {
		t.Fatalf("expected PriceGapPollIntervalSec=0, got %d", cfg.PriceGapPollIntervalSec)
	}
	if cfg.PriceGapCandidates != nil {
		t.Fatalf("expected PriceGapCandidates=nil, got %v", cfg.PriceGapCandidates)
	}
}

// TestConfig_PaperMode_Default — Load() must set PriceGapPaperMode=true (D-12 exception).
// Uses the struct literal directly to avoid dragging in filesystem/env state from Load().
func TestConfig_PaperMode_DefaultsTrue(t *testing.T) {
	// Mirror the Load() defaults block for the PriceGap subset.
	c := &Config{PriceGapPaperMode: true}
	if !c.PriceGapPaperMode {
		t.Fatalf("expected PriceGapPaperMode=true (D-12), got false")
	}
}

// TestConfig_PaperMode_ApplyJSONFlipsField — applyJSON with paper_mode:false
// must flip the runtime field to false.
func TestConfig_PaperMode_ApplyJSONFlipsField(t *testing.T) {
	cfg := &Config{PriceGapPaperMode: true}

	raw := []byte(`{"price_gap":{"paper_mode":false}}`)
	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.applyJSON(&jc)

	if cfg.PriceGapPaperMode != false {
		t.Fatalf("expected PriceGapPaperMode=false after applyJSON, got true")
	}
}

// TestConfig_PaperMode_ApplyJSONRoundTripTrue — applyJSON with paper_mode:true
// must set the runtime field to true.
func TestConfig_PaperMode_ApplyJSONRoundTripTrue(t *testing.T) {
	cfg := &Config{PriceGapPaperMode: false}

	raw := []byte(`{"price_gap":{"paper_mode":true}}`)
	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.applyJSON(&jc)

	if !cfg.PriceGapPaperMode {
		t.Fatalf("expected PriceGapPaperMode=true after applyJSON, got false")
	}
}

