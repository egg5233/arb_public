package discovery

import (
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

// newTestScanner creates a Scanner backed by an in-memory Redis for filter tests.
func newTestScanner(t *testing.T, cfg *config.Config) (*Scanner, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("database.New: %v", err)
	}
	s := NewScanner(makeNilExchangeMap("bybit", "binance", "gateio"), db, cfg)
	return s, mr
}

func TestAgreementFilter_BlockedBybitLegDropped(t *testing.T) {
	cfg := &config.Config{}
	cfg.EnableAgreementSkiplist = true
	s, mr := newTestScanner(t, cfg)
	defer mr.Close()

	// Block bybit/HOODUSDT.
	s.SetAgreementBlock("bybit", "HOODUSDT", "110126: must sign")

	opps := []models.Opportunity{
		{Symbol: "HOODUSDT", LongExchange: "bybit", ShortExchange: "binance"},
		{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "gateio"},
	}

	result := s.applyEntryFilters(opps, EntryScan)
	if len(result) != 1 {
		t.Fatalf("expected 1 opp after filter, got %d", len(result))
	}
	if result[0].Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT to pass, got %s", result[0].Symbol)
	}
}

func TestAgreementFilter_NonBybitPairKept(t *testing.T) {
	cfg := &config.Config{}
	cfg.EnableAgreementSkiplist = true
	s, mr := newTestScanner(t, cfg)
	defer mr.Close()

	// Block bybit/HOODUSDT — but not binance/HOODUSDT.
	s.SetAgreementBlock("bybit", "HOODUSDT", "110126: must sign")

	// Pair with no Bybit leg should NOT be dropped.
	opps := []models.Opportunity{
		{Symbol: "HOODUSDT", LongExchange: "binance", ShortExchange: "gateio"},
	}

	result := s.applyEntryFilters(opps, EntryScan)
	if len(result) != 1 {
		t.Fatalf("expected 1 opp to pass (non-Bybit pair), got %d", len(result))
	}
	if result[0].Symbol != "HOODUSDT" {
		t.Errorf("expected HOODUSDT pair to pass, got %s", result[0].Symbol)
	}
}

func TestAgreementFilter_SwitchOff_NoFilter(t *testing.T) {
	cfg := &config.Config{}
	cfg.EnableAgreementSkiplist = false // switch OFF — filter must be a no-op
	s, mr := newTestScanner(t, cfg)
	defer mr.Close()

	// Even with a block set, filter should not drop anything when switch is OFF.
	s.SetAgreementBlock("bybit", "HOODUSDT", "110126: must sign")

	opps := []models.Opportunity{
		{Symbol: "HOODUSDT", LongExchange: "bybit", ShortExchange: "binance"},
		{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "gateio"},
	}

	result := s.applyEntryFilters(opps, EntryScan)
	if len(result) != 2 {
		t.Fatalf("expected 2 opps when switch OFF (no filter), got %d", len(result))
	}
}

func TestAgreementFilter_ShortLegBlocked(t *testing.T) {
	cfg := &config.Config{}
	cfg.EnableAgreementSkiplist = true
	s, mr := newTestScanner(t, cfg)
	defer mr.Close()

	// Block bybit on the short leg.
	s.SetAgreementBlock("bybit", "SOONUSDT", "110126: must sign")

	opps := []models.Opportunity{
		{Symbol: "SOONUSDT", LongExchange: "binance", ShortExchange: "bybit"},
	}

	result := s.applyEntryFilters(opps, EntryScan)
	if len(result) != 0 {
		t.Fatalf("expected 0 opps when short leg blocked, got %d", len(result))
	}
}
