package engine

import (
	"math"
	"testing"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

type plannedMinWithdrawExchange struct {
	exchange.Exchange
	name string
}

func (e *plannedMinWithdrawExchange) Name() string                                { return e.name }
func (e *plannedMinWithdrawExchange) SetMetricsCallback(exchange.MetricsCallback) {}
func (e *plannedMinWithdrawExchange) WithdrawFeeInclusive() bool                  { return false }

func TestExecuteRebalanceFundingPlan_PlannedMinWithdrawOverfundsResidual(t *testing.T) {
	cfg := &config.Config{
		DryRun:            true,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		MarginL4Headroom:  0.05,
		ExchangeAddresses: map[string]map[string]string{"recipient": {"APT": "recipient-apt"}},
	}
	e := &Engine{
		cfg: cfg,
		log: utils.NewLogger("test-planned-minwithdraw"),
		exchanges: map[string]exchange.Exchange{
			"donor":     &plannedMinWithdrawExchange{name: "donor"},
			"recipient": &plannedMinWithdrawExchange{name: "recipient"},
		},
	}

	balances := map[string]rebalanceBalanceInfo{
		"donor":     {futures: 500, futuresTotal: 500},
		"recipient": {futures: 0, futuresTotal: 100},
	}
	needs := map[string]float64{"recipient": 100}
	deficits := []rebalanceDeficit{{exchange: "recipient", amount: 100}}
	planned := []transferStep{
		{From: "donor", To: "recipient", Amount: 95, Chain: "APT", MinWithdraw: 10},
		{From: "donor", To: "recipient", Amount: 10, Chain: "APT", MinWithdraw: 10},
	}

	result := e.executeRebalanceFundingPlan(needs, balances, deficits, planned)

	if got := result.Unfunded["recipient"]; math.Abs(got) > 1e-9 {
		t.Fatalf("recipient should be fully funded by planned min-withdraw overfund, got unfunded %.6f (%v)", got, result.SkipReasons)
	}
}
