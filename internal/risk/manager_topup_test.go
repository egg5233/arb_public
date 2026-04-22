package risk

import (
	"testing"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// TestSimulator_TopUpAppliedPopulated verifies that when cross-exchange top-up
// is used to inflate balance so a dryRun approval passes, TopUpApplied carries
// the per-exchange amount for the rebalance router to detect.
func TestSimulator_TopUpAppliedPopulated(t *testing.T) {
	m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
		"gateio": {Available: 50, Total: 50},
		"bitget": {Available: 50, Total: 50},
	})
	cache := &PrefetchCache{
		Balances: map[string]*exchange.Balance{
			"gateio": {Available: 50, Total: 50},
			"bitget": {Available: 50, Total: 50},
		},
		Orderbooks: map[string]*exchange.Orderbook{
			"gateio:METUSDT": newTestOrderbook(0.188),
			"bitget:METUSDT": newTestOrderbook(0.188),
		},
		TransferablePerExchange: map[string]float64{
			"gateio": 200,
			"bitget": 200,
		},
		ActivePositions: nil,
	}
	opp := models.Opportunity{
		Symbol:        "METUSDT",
		LongExchange:  "gateio",
		ShortExchange: "bitget",
		Spread:        3.87,
		IntervalHours: 4,
	}

	approval, err := m.SimulateApprovalForPair(opp, "gateio", "bitget", nil, nil, cache)
	if err != nil {
		t.Fatalf("SimulateApprovalForPair: %v", err)
	}
	if !approval.Approved {
		t.Fatalf("expected approved with top-up, got rejected: %s", approval.Reason)
	}
	if len(approval.TopUpApplied) == 0 {
		t.Fatal("TopUpApplied empty — rebalance router cannot detect top-up dependency")
	}
	if approval.TopUpApplied["gateio"] <= 0 {
		t.Errorf("TopUpApplied[gateio] = %.2f, expected >0", approval.TopUpApplied["gateio"])
	}
	if approval.TopUpApplied["bitget"] <= 0 {
		t.Errorf("TopUpApplied[bitget] = %.2f, expected >0", approval.TopUpApplied["bitget"])
	}
}

// TestSimulator_TopUpAppliedEmptyWhenSufficient verifies that when real balance
// is sufficient, TopUpApplied stays empty (approval is honest).
func TestSimulator_TopUpAppliedEmptyWhenSufficient(t *testing.T) {
	m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
		"gateio": {Available: 500, Total: 500},
		"bitget": {Available: 500, Total: 500},
	})
	cache := &PrefetchCache{
		Balances: map[string]*exchange.Balance{
			"gateio": {Available: 500, Total: 500},
			"bitget": {Available: 500, Total: 500},
		},
		Orderbooks: map[string]*exchange.Orderbook{
			"gateio:METUSDT": newTestOrderbook(0.188),
			"bitget:METUSDT": newTestOrderbook(0.188),
		},
		TransferablePerExchange: map[string]float64{
			"gateio": 200,
			"bitget": 200,
		},
		ActivePositions: nil,
	}
	opp := models.Opportunity{
		Symbol:        "METUSDT",
		LongExchange:  "gateio",
		ShortExchange: "bitget",
		Spread:        3.87,
		IntervalHours: 4,
	}

	approval, err := m.SimulateApprovalForPair(opp, "gateio", "bitget", nil, nil, cache)
	if err != nil {
		t.Fatalf("SimulateApprovalForPair: %v", err)
	}
	if !approval.Approved {
		t.Fatalf("expected approved (real balance sufficient): %s", approval.Reason)
	}
	if len(approval.TopUpApplied) != 0 {
		t.Errorf("TopUpApplied non-empty when real balance sufficient: %v", approval.TopUpApplied)
	}
}

// TestExecutor_TopUpAppliedNeverPopulated verifies real Approve (dryRun=false)
// never populates TopUpApplied, to keep non-simulator callers insulated.
func TestExecutor_TopUpAppliedNeverPopulated(t *testing.T) {
	m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
		"gateio": {Available: 500, Total: 500},
		"bitget": {Available: 500, Total: 500},
	})
	opp := models.Opportunity{
		Symbol:        "METUSDT",
		LongExchange:  "gateio",
		ShortExchange: "bitget",
		Spread:        3.87,
		IntervalHours: 4,
	}

	approval, err := m.Approve(opp)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approval.TopUpApplied != nil {
		t.Errorf("Executor path populated TopUpApplied (must be nil, not empty map): %v", approval.TopUpApplied)
	}
}
