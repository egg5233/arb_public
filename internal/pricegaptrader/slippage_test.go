package pricegaptrader

import (
	"strings"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// slippageTestCfg — minimal knobs for slippage_test.
func slippageTestCfg() *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
	}
}

func newSlipTestTracker(t *testing.T, store *fakeStore) *Tracker {
	t.Helper()
	exch := map[string]exchange.Exchange{
		"binance": newStubExchange("binance"),
		"bybit":   newStubExchange("bybit"),
	}
	return NewTracker(exch, store, newFakeDelistChecker(), slippageTestCfg())
}

func slipPos(realized, modeled float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:              "p-test",
		Symbol:          "SOON",
		LongExchange:    "binance",
		ShortExchange:   "bybit",
		RealizedSlipBps: realized,
		ModeledSlipBps:  modeled,
	}
}

// TestSlippage_BelowTenSamples_DoesNothing — 9 samples at 4× realized/modeled
// must NOT trigger auto-disable because the window is not yet full.
func TestSlippage_BelowTenSamples_DoesNothing(t *testing.T) {
	store := newFakeStore()
	tr := newSlipTestTracker(t, store)
	for i := 0; i < 9; i++ {
		tr.recordSlippageAndMaybeDisable(slipPos(200, 50))
	}
	if _, disabled := store.disabledSetWith["SOON"]; disabled {
		t.Fatalf("must NOT disable with only 9 samples")
	}
}

// TestSlippage_TenSamples_UnderThreshold — 10 samples with mean realized=95
// vs mean modeled=50 → ratio 1.9x < 2x → must NOT disable.
func TestSlippage_TenSamples_UnderThreshold(t *testing.T) {
	store := newFakeStore()
	tr := newSlipTestTracker(t, store)
	for i := 0; i < 10; i++ {
		tr.recordSlippageAndMaybeDisable(slipPos(95, 50))
	}
	if _, disabled := store.disabledSetWith["SOON"]; disabled {
		t.Fatalf("must NOT disable at 1.9x (under 2x threshold)")
	}
}

// TestSlippage_TenSamples_JustOverThreshold — 10 samples at realized=101
// vs modeled=50 → ratio 2.02x > 2x → MUST disable with reason containing
// "auto_disable".
func TestSlippage_TenSamples_JustOverThreshold(t *testing.T) {
	store := newFakeStore()
	tr := newSlipTestTracker(t, store)
	for i := 0; i < 10; i++ {
		tr.recordSlippageAndMaybeDisable(slipPos(101, 50))
	}
	reason, disabled := store.disabledSetWith["SOON"]
	if !disabled {
		t.Fatalf("expected SOON to be auto-disabled after 10 samples at 2.02x")
	}
	if !strings.Contains(reason, "auto_disable") {
		t.Fatalf("disable reason=%q must contain 'auto_disable'", reason)
	}
}

// TestSlippage_ZeroModeled_Guard — 10 samples with modeled=0, realized=100
// must NOT disable (divide-by-zero guard; no modeled signal means no meaningful
// ratio to compare against).
func TestSlippage_ZeroModeled_Guard(t *testing.T) {
	store := newFakeStore()
	tr := newSlipTestTracker(t, store)
	for i := 0; i < 10; i++ {
		tr.recordSlippageAndMaybeDisable(slipPos(100, 0))
	}
	if _, disabled := store.disabledSetWith["SOON"]; disabled {
		t.Fatalf("zero modeled must NOT trigger disable (divide-by-zero guard)")
	}
}

// TestSlippage_ExactlyAtTwoX_DoesNotDisable — realized=100, modeled=50 → ratio=2.00x
// Rule uses strict > (D-19: "mean(realized) > 2 * mean(modeled)"), so boundary is PASS.
func TestSlippage_ExactlyAtTwoX_DoesNotDisable(t *testing.T) {
	store := newFakeStore()
	tr := newSlipTestTracker(t, store)
	for i := 0; i < 10; i++ {
		tr.recordSlippageAndMaybeDisable(slipPos(100, 50))
	}
	if _, disabled := store.disabledSetWith["SOON"]; disabled {
		t.Fatalf("ratio exactly 2.0x must NOT disable (strict > threshold)")
	}
}
