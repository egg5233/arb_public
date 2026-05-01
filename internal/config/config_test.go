package config

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"arb/internal/models"
)

func TestSpotOnlyExchangesDefaultOffAndApplyJSON(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))

	cfg := Load()
	if cfg.SpotFuturesEnableSpotOnlyExchanges {
		t.Fatal("expected SpotFuturesEnableSpotOnlyExchanges default false")
	}

	enabled := true
	cfg.applyJSON(&jsonConfig{SpotFutures: &jsonSpotFutures{EnableSpotOnlyExchanges: &enabled}})
	if !cfg.SpotFuturesEnableSpotOnlyExchanges {
		t.Fatal("expected SpotFuturesEnableSpotOnlyExchanges=true after applyJSON")
	}
}

func TestSpotOnlyExchangesEnvOverride(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
	t.Setenv("SPOT_FUTURES_ENABLE_SPOT_ONLY_EXCHANGES", "true")

	cfg := Load()
	if !cfg.SpotFuturesEnableSpotOnlyExchanges {
		t.Fatal("expected SPOT_FUTURES_ENABLE_SPOT_ONLY_EXCHANGES=true to enable spot-only exchanges")
	}
}

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

// ---------------------------------------------------------------------------
// PriceGapDebugLog (Phase 9 gap-closure Gap #1 / Plan 09-09)
// ---------------------------------------------------------------------------

// TestConfig_PriceGapDebugLog_DefaultOff — the Go zero-value of a freshly
// constructed Config must leave PriceGapDebugLog=false. This is the project
// rollout-pattern default: new observability/risk switches ship OFF.
func TestConfig_PriceGapDebugLog_DefaultOff(t *testing.T) {
	c := &Config{}
	if c.PriceGapDebugLog {
		t.Fatalf("expected PriceGapDebugLog=false (default OFF per CLAUDE.local.md rollout pattern), got true")
	}
}

// TestConfig_PriceGapDebugLog_ApplyJSONTrue — applyJSON with debug_log:true
// must flip the runtime field to true.
func TestConfig_PriceGapDebugLog_ApplyJSONTrue(t *testing.T) {
	cfg := &Config{}
	raw := []byte(`{"price_gap":{"debug_log":true}}`)
	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.applyJSON(&jc)
	if !cfg.PriceGapDebugLog {
		t.Fatalf("expected PriceGapDebugLog=true after applyJSON, got false")
	}
}

// TestConfig_PriceGapDebugLog_ApplyJSONFalse — applyJSON with debug_log:false
// must flip the runtime field back to false even if the runtime value was true.
func TestConfig_PriceGapDebugLog_ApplyJSONFalse(t *testing.T) {
	cfg := &Config{PriceGapDebugLog: true}
	raw := []byte(`{"price_gap":{"debug_log":false}}`)
	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.applyJSON(&jc)
	if cfg.PriceGapDebugLog {
		t.Fatalf("expected PriceGapDebugLog=false after applyJSON, got true")
	}
}

// TestConfig_PriceGapDebugLog_ApplyJSONAbsentPreserves — applyJSON with an empty
// price_gap block (debug_log key omitted) must leave the existing runtime value
// unchanged. This is the pointer-optional contract: absent ≠ false.
func TestConfig_PriceGapDebugLog_ApplyJSONAbsentPreserves(t *testing.T) {
	cfg := &Config{PriceGapDebugLog: true}
	raw := []byte(`{"price_gap":{}}`)
	var jc jsonConfig
	if err := json.Unmarshal(raw, &jc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.applyJSON(&jc)
	if !cfg.PriceGapDebugLog {
		t.Fatalf("expected PriceGapDebugLog=true preserved when debug_log key absent, got false")
	}
}

// TestConfig_PriceGapDebugLog_SaveJSONRoundTrip — marshal via jsonPriceGap with
// a pointer to the runtime value, unmarshal via applyJSON, and confirm the
// value survives the round-trip for both true and false. This proves SaveJSON
// emits the debug_log key (via jsonPriceGap) in the same shape applyJSON reads.
func TestConfig_PriceGapDebugLog_SaveJSONRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		val  bool
	}{
		{"true", true},
		{"false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Emit via jsonPriceGap (the same shape SaveJSON's raw-map writer uses).
			src := jsonPriceGap{DebugLog: &tc.val}
			wrap := jsonConfig{PriceGap: &jsonPriceGap{DebugLog: src.DebugLog}}
			buf, err := json.Marshal(wrap)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			// Round-trip: unmarshal, apply, verify.
			var jc jsonConfig
			if err := json.Unmarshal(buf, &jc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			cfg := &Config{PriceGapDebugLog: !tc.val} // start on the opposite
			cfg.applyJSON(&jc)
			if cfg.PriceGapDebugLog != tc.val {
				t.Fatalf("round-trip lost value: want %v, got %v", tc.val, cfg.PriceGapDebugLog)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 14 (PG-LIVE-01, PG-LIVE-03) — validatePriceGapLive defense-layer-1
// ---------------------------------------------------------------------------

// defaultLiveConfig returns a Config populated with all Phase 14 defaults so
// individual tests can flip a single field to assert that the validator
// rejects exactly that violation.
func defaultLiveConfig() *Config {
	return &Config{
		PriceGapPaperMode:          true,
		PriceGapLiveCapital:        false,
		PriceGapStage1SizeUSDT:     100,
		PriceGapStage2SizeUSDT:     500,
		PriceGapStage3SizeUSDT:     1000,
		PriceGapHardCeilingUSDT:    1000,
		PriceGapAnomalySlippageBps: 50,
		PriceGapCleanDaysToPromote: 7,
	}
}

// TestValidatePriceGapLive_RejectsLiveWithPaper — D-06: live capital must
// fail-fast at config-load when paper mode is also on.
func TestValidatePriceGapLive_RejectsLiveWithPaper(t *testing.T) {
	c := defaultLiveConfig()
	c.PriceGapLiveCapital = true
	c.PriceGapPaperMode = true
	if err := validatePriceGapLive(c); err == nil {
		t.Fatal("expected error when LiveCapital=true AND PaperMode=true (D-06), got nil")
	}
}

// TestValidatePriceGapLive_RejectsCeilingAbove1000 — v2.2 hard ceiling 1000
// per CONTEXT Hard Contracts; even if defense-layer-2 (sizer) caps at 1000,
// layer 1 must reject before any code path runs.
func TestValidatePriceGapLive_RejectsCeilingAbove1000(t *testing.T) {
	c := defaultLiveConfig()
	c.PriceGapHardCeilingUSDT = 1500
	if err := validatePriceGapLive(c); err == nil {
		t.Fatal("expected error when HardCeiling > 1000, got nil")
	}
}

// TestValidatePriceGapLive_RejectsStageOrderViolation — Stage1<=Stage2<=Stage3
// is the asymmetric ratchet invariant (PG-LIVE-01). A misordered config
// (Stage1=600 with Stage2=500) must be caught at load.
func TestValidatePriceGapLive_RejectsStageOrderViolation(t *testing.T) {
	c := defaultLiveConfig()
	c.PriceGapStage1SizeUSDT = 600
	c.PriceGapStage2SizeUSDT = 500
	if err := validatePriceGapLive(c); err == nil {
		t.Fatal("expected error when Stage1=600 > Stage2=500, got nil")
	}
}

// TestValidatePriceGapLive_AcceptsDefaults — default values produced by
// Load() must satisfy the validator. This is the no-regression backstop:
// a config.json without a Phase 14 block must not fail validation.
func TestValidatePriceGapLive_AcceptsDefaults(t *testing.T) {
	c := defaultLiveConfig()
	if err := validatePriceGapLive(c); err != nil {
		t.Fatalf("validator rejected defaults: %v", err)
	}
}

// TestValidatePriceGapLive_RejectsTypoStage3At9999 — RESEARCH Q10 explicit
// scenario: operator types 9999 instead of 1000 in stage_3_size_usdt. Layer-1
// must catch this before the sizer (layer-2) runs.
func TestValidatePriceGapLive_RejectsTypoStage3At9999(t *testing.T) {
	c := defaultLiveConfig()
	c.PriceGapStage3SizeUSDT = 9999 // typo
	if err := validatePriceGapLive(c); err == nil {
		t.Fatal("expected error when Stage3=9999 > HardCeiling=1000, got nil")
	}
}

// TestValidatePriceGapLive_RejectsAnomalyBpsOutOfRange — D-09 default 50;
// range [0,500]. Negative or above 500 must be rejected.
func TestValidatePriceGapLive_RejectsAnomalyBpsOutOfRange(t *testing.T) {
	for _, val := range []float64{-1, 501, 9999} {
		c := defaultLiveConfig()
		c.PriceGapAnomalySlippageBps = val
		if err := validatePriceGapLive(c); err == nil {
			t.Fatalf("expected error when AnomalySlippageBps=%v, got nil", val)
		}
	}
}

// TestValidatePriceGapLive_RejectsCleanDaysOutOfRange — default 7; range
// [1,30]. Zero or above 30 must be rejected.
func TestValidatePriceGapLive_RejectsCleanDaysOutOfRange(t *testing.T) {
	for _, val := range []int{0, 31, 365} {
		c := defaultLiveConfig()
		c.PriceGapCleanDaysToPromote = val
		if err := validatePriceGapLive(c); err == nil {
			t.Fatalf("expected error when CleanDaysToPromote=%d, got nil", val)
		}
	}
}

// TestPriceGapLiveDTORoundTrip — JSON DTO with Phase 14 fields:
//   - Empty block: applyJSON installs the Phase 14 defaults (100/500/1000/1000/50/7/false).
//   - Explicit values: applyJSON preserves them verbatim.
func TestPriceGapLiveDTORoundTrip(t *testing.T) {
	t.Run("empty_block_installs_defaults", func(t *testing.T) {
		raw := []byte(`{"price_gap":{}}`)
		var jc jsonConfig
		if err := json.Unmarshal(raw, &jc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		cfg := &Config{}
		cfg.applyJSON(&jc)
		if cfg.PriceGapLiveCapital {
			t.Fatalf("expected LiveCapital=false default, got true")
		}
		if cfg.PriceGapStage1SizeUSDT != 100 {
			t.Fatalf("expected Stage1=100, got %v", cfg.PriceGapStage1SizeUSDT)
		}
		if cfg.PriceGapStage2SizeUSDT != 500 {
			t.Fatalf("expected Stage2=500, got %v", cfg.PriceGapStage2SizeUSDT)
		}
		if cfg.PriceGapStage3SizeUSDT != 1000 {
			t.Fatalf("expected Stage3=1000, got %v", cfg.PriceGapStage3SizeUSDT)
		}
		if cfg.PriceGapHardCeilingUSDT != 1000 {
			t.Fatalf("expected HardCeiling=1000, got %v", cfg.PriceGapHardCeilingUSDT)
		}
		if cfg.PriceGapAnomalySlippageBps != 50 {
			t.Fatalf("expected AnomalySlippageBps=50, got %v", cfg.PriceGapAnomalySlippageBps)
		}
		if cfg.PriceGapCleanDaysToPromote != 7 {
			t.Fatalf("expected CleanDaysToPromote=7, got %d", cfg.PriceGapCleanDaysToPromote)
		}
	})

	t.Run("explicit_values_preserved", func(t *testing.T) {
		raw := []byte(`{
            "price_gap": {
                "live_capital": true,
                "stage_1_size_usdt": 200,
                "stage_2_size_usdt": 600,
                "stage_3_size_usdt": 950,
                "hard_ceiling_usdt": 1000,
                "anomaly_slippage_bps": 75,
                "clean_days_to_promote": 14
            }
        }`)
		var jc jsonConfig
		if err := json.Unmarshal(raw, &jc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		cfg := &Config{}
		cfg.applyJSON(&jc)
		if !cfg.PriceGapLiveCapital {
			t.Fatalf("expected LiveCapital=true, got false")
		}
		if cfg.PriceGapStage1SizeUSDT != 200 {
			t.Fatalf("expected Stage1=200, got %v", cfg.PriceGapStage1SizeUSDT)
		}
		if cfg.PriceGapStage2SizeUSDT != 600 {
			t.Fatalf("expected Stage2=600, got %v", cfg.PriceGapStage2SizeUSDT)
		}
		if cfg.PriceGapStage3SizeUSDT != 950 {
			t.Fatalf("expected Stage3=950, got %v", cfg.PriceGapStage3SizeUSDT)
		}
		if cfg.PriceGapHardCeilingUSDT != 1000 {
			t.Fatalf("expected HardCeiling=1000, got %v", cfg.PriceGapHardCeilingUSDT)
		}
		if cfg.PriceGapAnomalySlippageBps != 75 {
			t.Fatalf("expected AnomalySlippageBps=75, got %v", cfg.PriceGapAnomalySlippageBps)
		}
		if cfg.PriceGapCleanDaysToPromote != 14 {
			t.Fatalf("expected CleanDaysToPromote=14, got %d", cfg.PriceGapCleanDaysToPromote)
		}
	})
}

// ---------------------------------------------------------------------------
// Phase 15 (PG-LIVE-02) — Drawdown circuit breaker validator
// ---------------------------------------------------------------------------

// TestValidatePriceGapLive_BreakerFields exercises the Phase 15 breaker
// validator extension to validatePriceGapLive. Default OFF (disabled=false)
// must accept any field values without inspection. When enabled, the
// validator rejects positive limits (D-06: limit is absolute USDT, must be
// negative-or-zero) and out-of-range intervals (60s..3600s per D-01).
func TestValidatePriceGapLive_BreakerFields(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		limit     float64
		interval  int
		wantError bool
	}{
		// Default OFF — any values accepted (master switch is the gate).
		{"disabled_zeros_accepted", false, 0, 0, false},
		{"disabled_positive_limit_accepted", false, 50, 30, false},
		{"disabled_huge_interval_accepted", false, -100, 99999, false},

		// Enabled — limit validation.
		{"enabled_limit_zero_accepted", true, 0, 300, false},
		{"enabled_limit_negative_accepted", true, -50, 300, false},
		{"enabled_limit_positive_rejected", true, 50, 300, true},
		{"enabled_limit_positive_one_rejected", true, 0.1, 300, true},

		// Enabled — interval range.
		{"enabled_interval_default_300_accepted", true, -50, 300, false},
		{"enabled_interval_floor_60_accepted", true, -50, 60, false},
		{"enabled_interval_ceiling_3600_accepted", true, -50, 3600, false},
		{"enabled_interval_below_floor_rejected", true, -50, 59, true},
		{"enabled_interval_above_ceiling_rejected", true, -50, 3601, true},
		{"enabled_interval_zero_rejected", true, -50, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := defaultLiveConfig()
			c.PriceGapBreakerEnabled = tc.enabled
			c.PriceGapDrawdownLimitUSDT = tc.limit
			c.PriceGapBreakerIntervalSec = tc.interval
			err := validatePriceGapLive(c)
			if tc.wantError && err == nil {
				t.Fatalf("expected error (enabled=%v, limit=%v, interval=%d), got nil",
					tc.enabled, tc.limit, tc.interval)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("expected no error (enabled=%v, limit=%v, interval=%d), got: %v",
					tc.enabled, tc.limit, tc.interval, err)
			}
		})
	}
}
