// Package pricegaptrader — validate_test.go: shared validation tests for
// PriceGapCandidate. Plan 11-03 Task 2 extracted validatePriceGapCandidates
// out of internal/api/pricegap_handlers.go so handlers.go AND cmd/pg-admin
// share one validation surface (drift defense — T-11-19).
package pricegaptrader

import (
	"strings"
	"testing"

	"arb/internal/models"
)

// validCandidate returns a baseline candidate that passes ValidateCandidates
// — tests override one field at a time for negative cases.
func validCandidate() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "BTCUSDT",
		LongExch:           "binance",
		ShortExch:          "bybit",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 5,
	}
}

func TestValidateCandidates_AcceptsValid(t *testing.T) {
	c := validCandidate()
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) != 0 {
		t.Fatalf("valid candidate produced errors: %v", errs)
	}
}

func TestValidateCandidates_RejectsBadSymbol(t *testing.T) {
	c := validCandidate()
	c.Symbol = "btc/usdt"
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 {
		t.Fatalf("bad symbol passed validation")
	}
	if !strings.Contains(strings.Join(errs, ";"), "symbol must match") {
		t.Errorf("error msg missing symbol regex hint: %v", errs)
	}
}

func TestValidateCandidates_RejectsUnknownExch(t *testing.T) {
	c := validCandidate()
	c.LongExch = "kraken"
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "long_exch invalid") {
		t.Fatalf("expected long_exch invalid: %v", errs)
	}
}

func TestValidateCandidates_RejectsSameExch(t *testing.T) {
	c := validCandidate()
	c.ShortExch = c.LongExch
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "must differ") {
		t.Fatalf("expected long_exch must differ from short_exch: %v", errs)
	}
}

func TestValidateCandidates_RejectsBadThreshold(t *testing.T) {
	c := validCandidate()
	c.ThresholdBps = 0
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "threshold_bps") {
		t.Fatalf("expected threshold_bps range error: %v", errs)
	}
}

func TestValidateCandidates_RejectsBadMaxPosition(t *testing.T) {
	c := validCandidate()
	c.MaxPositionUSDT = 600000 // > 500000 cap
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "max_position_usdt") {
		t.Fatalf("expected max_position_usdt range error: %v", errs)
	}
}

func TestValidateCandidates_RejectsBadSlippage(t *testing.T) {
	c := validCandidate()
	c.ModeledSlippageBps = 1500
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "modeled_slippage_bps") {
		t.Fatalf("expected modeled_slippage_bps range error: %v", errs)
	}
}

func TestValidateCandidates_RejectsBadDirection(t *testing.T) {
	c := validCandidate()
	c.Direction = "wonky"
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "direction must be") {
		t.Fatalf("expected direction error: %v", errs)
	}
}

func TestValidateCandidates_AcceptsEmptyDirection(t *testing.T) {
	c := validCandidate()
	c.Direction = "" // default → pinned
	errs := ValidateCandidates([]models.PriceGapCandidate{c})
	if len(errs) != 0 {
		t.Fatalf("empty direction must be accepted (default→pinned): %v", errs)
	}
}

func TestValidateCandidates_DetectsDuplicateTuple(t *testing.T) {
	c1 := validCandidate()
	c2 := validCandidate()
	errs := ValidateCandidates([]models.PriceGapCandidate{c1, c2})
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, ";"), "duplicate") {
		t.Fatalf("expected duplicate tuple error: %v", errs)
	}
}

func TestValidateCandidates_CollatesAllErrors(t *testing.T) {
	bad := validCandidate()
	bad.Symbol = "lower"
	bad.LongExch = "notExch"
	bad.ThresholdBps = 0
	errs := ValidateCandidates([]models.PriceGapCandidate{bad})
	if len(errs) < 3 {
		t.Fatalf("expected ≥3 collated errors, got %d: %v", len(errs), errs)
	}
}
