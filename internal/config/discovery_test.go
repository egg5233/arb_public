package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPriceGapDiscoverySchema covers the 12 sub-tests required by Phase 11
// Plan 01: defaults, validation floors, format rules, JSON round-trip, and
// keepNonZero preservation.
func TestPriceGapDiscoverySchema(t *testing.T) {
	t.Run("Default_DiscoveryEnabledFalse", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapDiscoveryEnabled {
			t.Fatalf("expected PriceGapDiscoveryEnabled=false at defaults, got true")
		}
	})

	t.Run("UniverseExceedsTwentyRejected", func(t *testing.T) {
		c := &Config{PriceGapDiscoveryUniverse: makeUniverse(21)}
		setDiscoveryDefaults(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for universe size 21")
		}
		msg := err.Error()
		if !(strings.Contains(msg, "≤20") || strings.Contains(msg, "max 20") || strings.Contains(msg, "exceeds")) {
			t.Fatalf("expected error mentioning '≤20', 'max 20', or 'exceeds', got %q", msg)
		}
	})

	t.Run("UniverseLowercaseRejected", func(t *testing.T) {
		c := &Config{PriceGapDiscoveryUniverse: []string{"btcusdt"}}
		setDiscoveryDefaults(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for lowercase universe entry")
		}
	})

	t.Run("UniverseSeparatorRejected", func(t *testing.T) {
		c := &Config{PriceGapDiscoveryUniverse: []string{"BTC-USDT"}}
		setDiscoveryDefaults(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for BTC-USDT (separator forbidden)")
		}
	})

	t.Run("DenylistAcceptsTuples", func(t *testing.T) {
		c := &Config{
			PriceGapDiscoveryDenylist: []string{"BTCUSDT", "ETHUSDT@bybit"},
		}
		setDiscoveryDefaults(c)
		if err := validatePriceGapDiscovery(c); err != nil {
			t.Fatalf("expected denylist with valid entries to pass, got %v", err)
		}

		c.PriceGapDiscoveryDenylist = []string{"@bybit"}
		if err := validatePriceGapDiscovery(c); err == nil {
			t.Fatal("expected error for @bybit (empty symbol)")
		}

		c.PriceGapDiscoveryDenylist = []string{"BTCUSDT@"}
		if err := validatePriceGapDiscovery(c); err == nil {
			t.Fatal("expected error for BTCUSDT@ (empty exchange)")
		}
	})

	t.Run("IntervalSecDefaultAndFloor", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapDiscoveryIntervalSec != 300 {
			t.Fatalf("expected default IntervalSec=300, got %d", cfg.PriceGapDiscoveryIntervalSec)
		}
		c := &Config{PriceGapDiscoveryIntervalSec: 30}
		setDiscoveryDefaultsExceptInterval(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for IntervalSec=30 (below floor 60)")
		}
	})

	t.Run("ThresholdBpsDefaultAndFloor", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapDiscoveryThresholdBps != 100 {
			t.Fatalf("expected default ThresholdBps=100, got %d", cfg.PriceGapDiscoveryThresholdBps)
		}
		c := &Config{PriceGapDiscoveryThresholdBps: 5}
		setDiscoveryDefaultsExceptThreshold(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for ThresholdBps=5 (below floor 10)")
		}
	})

	t.Run("MinDepthUSDTDefaultAndFloor", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapDiscoveryMinDepthUSDT != 1000 {
			t.Fatalf("expected default MinDepthUSDT=1000, got %d", cfg.PriceGapDiscoveryMinDepthUSDT)
		}
		c := &Config{PriceGapDiscoveryMinDepthUSDT: -1}
		setDiscoveryDefaultsExceptMinDepth(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for MinDepthUSDT=-1 (negative)")
		}
	})

	t.Run("AutoPromoteScoreDefaultAndFloor", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapAutoPromoteScore != 60 {
			t.Fatalf("expected default AutoPromoteScore=60, got %d", cfg.PriceGapAutoPromoteScore)
		}
		c := &Config{PriceGapAutoPromoteScore: 0}
		setDiscoveryDefaultsExceptScore(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for AutoPromoteScore=0 (below floor 50)")
		}
		c2 := &Config{PriceGapAutoPromoteScore: 101}
		setDiscoveryDefaultsExceptScore(c2)
		if err := validatePriceGapDiscovery(c2); err == nil {
			t.Fatal("expected error for AutoPromoteScore=101 (above 100)")
		}
	})

	t.Run("MaxCandidatesDefaultAndRange", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config.json"))
		cfg := Load()
		if cfg.PriceGapMaxCandidates != 12 {
			t.Fatalf("expected default MaxCandidates=12, got %d", cfg.PriceGapMaxCandidates)
		}
		c := &Config{PriceGapMaxCandidates: 0}
		setDiscoveryDefaultsExceptMax(c)
		err := validatePriceGapDiscovery(c)
		if err == nil {
			t.Fatal("expected error for MaxCandidates=0 (below 1)")
		}
		c2 := &Config{PriceGapMaxCandidates: 51}
		setDiscoveryDefaultsExceptMax(c2)
		if err := validatePriceGapDiscovery(c2); err == nil {
			t.Fatal("expected error for MaxCandidates=51 (above 50)")
		}
	})

	t.Run("RoundTripPreservesAllFields", func(t *testing.T) {
		raw := []byte(`{
            "price_gap": {
                "discovery_enabled": true,
                "discovery_universe": ["BTCUSDT", "ETHUSDT"],
                "discovery_denylist": ["DOGEUSDT", "PEPEUSDT@bybit"],
                "discovery_interval_sec": 600,
                "discovery_threshold_bps": 150,
                "discovery_min_depth_usdt": 2500,
                "auto_promote_score": 75,
                "max_candidates": 20
            }
        }`)
		var jc jsonConfig
		if err := json.Unmarshal(raw, &jc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		cfg := &Config{}
		cfg.applyJSON(&jc)
		if !cfg.PriceGapDiscoveryEnabled {
			t.Fatalf("expected DiscoveryEnabled=true after applyJSON")
		}
		if len(cfg.PriceGapDiscoveryUniverse) != 2 || cfg.PriceGapDiscoveryUniverse[0] != "BTCUSDT" {
			t.Fatalf("universe load mismatch: %v", cfg.PriceGapDiscoveryUniverse)
		}
		if len(cfg.PriceGapDiscoveryDenylist) != 2 {
			t.Fatalf("denylist load mismatch: %v", cfg.PriceGapDiscoveryDenylist)
		}
		if cfg.PriceGapDiscoveryIntervalSec != 600 {
			t.Fatalf("expected IntervalSec=600, got %d", cfg.PriceGapDiscoveryIntervalSec)
		}
		if cfg.PriceGapDiscoveryThresholdBps != 150 {
			t.Fatalf("expected ThresholdBps=150, got %d", cfg.PriceGapDiscoveryThresholdBps)
		}
		if cfg.PriceGapDiscoveryMinDepthUSDT != 2500 {
			t.Fatalf("expected MinDepthUSDT=2500, got %d", cfg.PriceGapDiscoveryMinDepthUSDT)
		}
		if cfg.PriceGapAutoPromoteScore != 75 {
			t.Fatalf("expected AutoPromoteScore=75, got %d", cfg.PriceGapAutoPromoteScore)
		}
		if cfg.PriceGapMaxCandidates != 20 {
			t.Fatalf("expected MaxCandidates=20, got %d", cfg.PriceGapMaxCandidates)
		}

		// Round trip via SaveJSON
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatalf("write seed: %v", err)
		}
		t.Setenv("CONFIG_FILE", path)
		if err := cfg.SaveJSON(); err != nil {
			t.Fatalf("SaveJSON: %v", err)
		}
		out, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read written: %v", err)
		}
		// Re-load and compare.
		var jc2 jsonConfig
		if err := json.Unmarshal(out, &jc2); err != nil {
			t.Fatalf("re-unmarshal: %v", err)
		}
		cfg2 := &Config{}
		cfg2.applyJSON(&jc2)
		if cfg2.PriceGapDiscoveryEnabled != cfg.PriceGapDiscoveryEnabled {
			t.Fatalf("round-trip enabled mismatch")
		}
		if cfg2.PriceGapDiscoveryIntervalSec != cfg.PriceGapDiscoveryIntervalSec {
			t.Fatalf("round-trip interval mismatch: %d vs %d", cfg2.PriceGapDiscoveryIntervalSec, cfg.PriceGapDiscoveryIntervalSec)
		}
		if cfg2.PriceGapAutoPromoteScore != cfg.PriceGapAutoPromoteScore {
			t.Fatalf("round-trip score mismatch: %d vs %d", cfg2.PriceGapAutoPromoteScore, cfg.PriceGapAutoPromoteScore)
		}
		if cfg2.PriceGapMaxCandidates != cfg.PriceGapMaxCandidates {
			t.Fatalf("round-trip max candidates mismatch")
		}
	})

	t.Run("KeepNonZeroTripwireIntact", func(t *testing.T) {
		// keepNonZero must still preserve disk values when new is zero.
		got := keepNonZero(float64(200), 0, "spot_futures.capital_unified_usdt")
		if got != float64(200) {
			t.Fatalf("keepNonZero regression: expected 200, got %v", got)
		}
	})
}

// makeUniverse generates n synthetic canonical-form symbols.
func makeUniverse(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = "AAA" + string(rune('A'+i%26)) + "USDT"
	}
	return out
}

// setDiscoveryDefaults populates all 8 fields with valid defaults so that
// validation focuses on whichever field a sub-test mutates.
func setDiscoveryDefaults(c *Config) {
	c.PriceGapDiscoveryIntervalSec = 300
	c.PriceGapDiscoveryThresholdBps = 100
	c.PriceGapDiscoveryMinDepthUSDT = 1000
	c.PriceGapAutoPromoteScore = 60
	c.PriceGapMaxCandidates = 12
}

func setDiscoveryDefaultsExceptInterval(c *Config) {
	c.PriceGapDiscoveryThresholdBps = 100
	c.PriceGapDiscoveryMinDepthUSDT = 1000
	c.PriceGapAutoPromoteScore = 60
	c.PriceGapMaxCandidates = 12
}

func setDiscoveryDefaultsExceptThreshold(c *Config) {
	c.PriceGapDiscoveryIntervalSec = 300
	c.PriceGapDiscoveryMinDepthUSDT = 1000
	c.PriceGapAutoPromoteScore = 60
	c.PriceGapMaxCandidates = 12
}

func setDiscoveryDefaultsExceptMinDepth(c *Config) {
	c.PriceGapDiscoveryIntervalSec = 300
	c.PriceGapDiscoveryThresholdBps = 100
	c.PriceGapAutoPromoteScore = 60
	c.PriceGapMaxCandidates = 12
}

func setDiscoveryDefaultsExceptScore(c *Config) {
	c.PriceGapDiscoveryIntervalSec = 300
	c.PriceGapDiscoveryThresholdBps = 100
	c.PriceGapDiscoveryMinDepthUSDT = 1000
	c.PriceGapMaxCandidates = 12
}

func setDiscoveryDefaultsExceptMax(c *Config) {
	c.PriceGapDiscoveryIntervalSec = 300
	c.PriceGapDiscoveryThresholdBps = 100
	c.PriceGapDiscoveryMinDepthUSDT = 1000
	c.PriceGapAutoPromoteScore = 60
}
