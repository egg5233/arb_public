package risk

import (
	"math"
	"testing"

	"arb/internal/models"
	"arb/pkg/exchange"
)

func TestEffectiveOrderAvailableCapsByMaxTransferOut(t *testing.T) {
	cases := []struct {
		name string
		bal  *exchange.Balance
		want float64
	}{
		{
			name: "raw available when transfer cap unknown",
			bal:  &exchange.Balance{Available: 225.43, MaxTransferOut: 0},
			want: 225.43,
		},
		{
			name: "positive transfer cap is more conservative",
			bal:  &exchange.Balance{Available: 225.43, MaxTransferOut: 52.60},
			want: 52.60,
		},
		{
			name: "authoritative zero means unavailable",
			bal:  &exchange.Balance{Available: 225.43, MaxTransferOut: 0, MaxTransferOutAuthoritative: true},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveOrderAvailable(tc.bal)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("EffectiveOrderAvailable() = %.8f, want %.8f", got, tc.want)
			}
		})
	}
}

func TestApproveCapsSizingByEffectiveOrderAvailable(t *testing.T) {
	cfg := defaultCfg()
	cfg.Leverage = 3
	cfg.CapitalPerLeg = 100
	cfg.MarginSafetyMultiplier = 1.1
	cfg.SlippageBPS = 1000
	cfg.MaxPriceGapBPS = 1000
	cfg.PriceGapFreeBPS = 1000

	ob := &exchange.Orderbook{
		Asks: []exchange.PriceLevel{{Price: 0.000790, Quantity: 1_000_000}},
		Bids: []exchange.PriceLevel{{Price: 0.000784, Quantity: 1_000_000}},
	}
	m, cleanup := newTestManager(t, map[string]exchange.Exchange{
		"bybit": &managerStubExchange{
			name: "bybit",
			futuresBal: &exchange.Balance{
				Total:                       225.43,
				Available:                   225.43,
				MaxTransferOut:              52.60,
				MaxTransferOutAuthoritative: true,
			},
			orderbook: ob,
		},
		"gateio": &managerStubExchange{
			name:       "gateio",
			futuresBal: &exchange.Balance{Total: 232.10, Available: 232.10},
			orderbook:  ob,
		},
	}, cfg)
	defer cleanup()

	approval, err := m.Approve(models.Opportunity{
		Symbol:        "PTBUSDT",
		LongExchange:  "bybit",
		ShortExchange: "gateio",
		Spread:        3.81,
		IntervalHours: 4,
	})
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if !approval.Approved {
		t.Fatalf("Approve rejected: %s", approval.Reason)
	}

	rawAvailableSize := (225.43 * float64(cfg.Leverage) / cfg.MarginSafetyMultiplier) / 0.000787
	if approval.Size >= rawAvailableSize*0.5 {
		t.Fatalf("size %.0f was not capped by MaxTransferOut; raw-available size would be %.0f", approval.Size, rawAvailableSize)
	}

	maxExpected := (52.60 * float64(cfg.Leverage) / cfg.MarginSafetyMultiplier) / 0.000787
	if approval.Size > maxExpected+100 {
		t.Fatalf("size %.0f exceeds effective-available cap %.0f", approval.Size, maxExpected)
	}
}

func TestSimulateApprovalTopUpUsesEffectiveOrderAvailable(t *testing.T) {
	cfg := defaultCfg()
	cfg.Leverage = 3
	cfg.CapitalPerLeg = 100
	cfg.MarginSafetyMultiplier = 2
	cfg.SlippageBPS = 1000
	cfg.MaxPriceGapBPS = 1000
	cfg.PriceGapFreeBPS = 1000

	ob := &exchange.Orderbook{
		Asks: []exchange.PriceLevel{{Price: 0.000790, Quantity: 1_000_000}},
		Bids: []exchange.PriceLevel{{Price: 0.000784, Quantity: 1_000_000}},
	}
	m, cleanup := newTestManager(t, map[string]exchange.Exchange{
		"bybit": &managerStubExchange{
			name: "bybit",
			futuresBal: &exchange.Balance{
				Total:                       225.43,
				Available:                   225.43,
				MaxTransferOut:              52.60,
				MaxTransferOutAuthoritative: true,
			},
			orderbook: ob,
		},
		"gateio": &managerStubExchange{
			name:       "gateio",
			futuresBal: &exchange.Balance{Total: 232.10, Available: 232.10},
			orderbook:  ob,
		},
	}, cfg)
	defer cleanup()

	approval, err := m.SimulateApprovalForPair(
		models.Opportunity{Symbol: "PTBUSDT", Spread: 3.81, IntervalHours: 4},
		"bybit",
		"gateio",
		nil,
		nil,
		&PrefetchCache{TransferablePerExchange: map[string]float64{"bybit": 300}},
	)
	if err != nil {
		t.Fatalf("SimulateApprovalForPair returned error: %v", err)
	}
	if !approval.Approved {
		t.Fatalf("SimulateApprovalForPair rejected: %s", approval.Reason)
	}
	if approval.TopUpApplied["bybit"] < 140 || approval.TopUpApplied["bybit"] > 150 {
		t.Fatalf("bybit TopUpApplied = %.4f, want about 147.4", approval.TopUpApplied["bybit"])
	}
}
