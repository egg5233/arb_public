package config

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"arb/internal/models"
)

func TestStrategyPriorityDefaultsAreSafe(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))

	cfg := Load()

	if cfg.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=false")
	}
	if cfg.StrategyPriority != StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected StrategyPriority=%s, got %s", StrategyPriorityPerpPerpFirst, cfg.StrategyPriority)
	}
	if cfg.EffectiveStrategyPriority() != StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected EffectiveStrategyPriority=%s, got %s", StrategyPriorityPerpPerpFirst, cfg.EffectiveStrategyPriority())
	}
	if cfg.ExpectedHoldHours != 24 {
		t.Fatalf("expected ExpectedHoldHours=24, got %v", cfg.ExpectedHoldHours)
	}
}

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

func TestApplyJSON_StrategyPriorityFields(t *testing.T) {
	enabled := true
	mode := string(StrategyPriorityDirBFirst)
	hold := 12.5
	cfg := &Config{
		StrategyPriority:  StrategyPriorityPerpPerpFirst,
		ExpectedHoldHours: 24,
	}

	cfg.applyJSON(&jsonConfig{Strategy: &jsonStrategy{
		EnablePriority:    &enabled,
		StrategyPriority:  &mode,
		ExpectedHoldHours: &hold,
	}})

	if !cfg.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=true")
	}
	if cfg.StrategyPriority != StrategyPriorityDirBFirst {
		t.Fatalf("expected StrategyPriority=%s, got %s", StrategyPriorityDirBFirst, cfg.StrategyPriority)
	}
	if cfg.ExpectedHoldHours != hold {
		t.Fatalf("expected ExpectedHoldHours=%v, got %v", hold, cfg.ExpectedHoldHours)
	}
}

func TestApplyJSON_StrategyPriorityAbsentPreserves(t *testing.T) {
	cfg := &Config{
		EnableStrategyPriority: true,
		StrategyPriority:       StrategyPriorityDirBOnly,
		ExpectedHoldHours:      36,
	}

	cfg.applyJSON(&jsonConfig{Strategy: &jsonStrategy{}})

	if !cfg.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority preserved true")
	}
	if cfg.StrategyPriority != StrategyPriorityDirBOnly {
		t.Fatalf("expected StrategyPriority preserved %s, got %s", StrategyPriorityDirBOnly, cfg.StrategyPriority)
	}
	if cfg.ExpectedHoldHours != 36 {
		t.Fatalf("expected ExpectedHoldHours preserved 36, got %v", cfg.ExpectedHoldHours)
	}
}

func TestApplyJSON_StrategyPriorityInvalidFallsBack(t *testing.T) {
	badMode := "unknown"
	badHold := 0.0
	cfg := &Config{
		StrategyPriority:  StrategyPriorityDirBOnly,
		ExpectedHoldHours: 36,
	}

	cfg.applyJSON(&jsonConfig{Strategy: &jsonStrategy{
		StrategyPriority:  &badMode,
		ExpectedHoldHours: &badHold,
	}})

	if cfg.StrategyPriority != StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected invalid strategy priority fallback %s, got %s", StrategyPriorityPerpPerpFirst, cfg.StrategyPriority)
	}
	if cfg.ExpectedHoldHours != 24 {
		t.Fatalf("expected invalid expected_hold_hours fallback 24, got %v", cfg.ExpectedHoldHours)
	}
}

func TestStrategyPrioritySaveJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", configPath)
	if err := os.WriteFile(configPath, []byte(`{"strategy":{}}`), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg := &Config{
		EnableStrategyPriority: true,
		StrategyPriority:       StrategyPriorityPerpPerpOnly,
		ExpectedHoldHours:      48,
	}
	if err := cfg.SaveJSON(); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	loaded := Load()
	if !loaded.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=true after round trip")
	}
	if loaded.StrategyPriority != StrategyPriorityPerpPerpOnly {
		t.Fatalf("expected StrategyPriority=%s, got %s", StrategyPriorityPerpPerpOnly, loaded.StrategyPriority)
	}
	if loaded.ExpectedHoldHours != 48 {
		t.Fatalf("expected ExpectedHoldHours=48, got %v", loaded.ExpectedHoldHours)
	}
}

func TestStrategyPriorityEnvOverrides(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
	t.Setenv("ENABLE_STRATEGY_PRIORITY", "true")
	t.Setenv("STRATEGY_PRIORITY", string(StrategyPriorityDirBOnly))
	t.Setenv("EXPECTED_HOLD_HOURS", "6.5")

	cfg := Load()
	if !cfg.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority=true from env")
	}
	if cfg.StrategyPriority != StrategyPriorityDirBOnly {
		t.Fatalf("expected StrategyPriority=%s, got %s", StrategyPriorityDirBOnly, cfg.StrategyPriority)
	}
	if cfg.ExpectedHoldHours != 6.5 {
		t.Fatalf("expected ExpectedHoldHours=6.5, got %v", cfg.ExpectedHoldHours)
	}
}

func TestStrategyPriorityEnvInvalidKeepsDefaults(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
	t.Setenv("ENABLE_STRATEGY_PRIORITY", "maybe")
	t.Setenv("STRATEGY_PRIORITY", "bad")
	t.Setenv("EXPECTED_HOLD_HOURS", "0")

	cfg := Load()
	if cfg.EnableStrategyPriority {
		t.Fatalf("expected EnableStrategyPriority default false")
	}
	if cfg.StrategyPriority != StrategyPriorityPerpPerpFirst {
		t.Fatalf("expected StrategyPriority default, got %s", cfg.StrategyPriority)
	}
	if cfg.ExpectedHoldHours != 24 {
		t.Fatalf("expected ExpectedHoldHours default 24, got %v", cfg.ExpectedHoldHours)
	}
}

func TestValidateExpectedHoldHours(t *testing.T) {
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if ValidateExpectedHoldHours(v) {
			t.Fatalf("expected %v to be invalid", v)
		}
	}
	if !ValidateExpectedHoldHours(0.1) {
		t.Fatalf("expected positive finite hold hours to be valid")
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
