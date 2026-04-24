package risk

import (
	"testing"

	"arb/pkg/exchange"

	"arb/internal/models"
)

// TestManagerCapitalRejection_FieldPopulation pins the prerequisite contract
// for the Pass 1 rescue path: every RejectionKindCapital approval produced
// by the insufficient-margin-buffer and post-trade-ratio sites must carry
// non-zero Size, Price, RequiredMargin, LongMarginNeeded, and ShortMarginNeeded
// so rebalance.Pass1 can size transfers and score the rescue candidate.
// If a future refactor strips these fields, Pass 1 drops the candidate as
// capital_unpriced and the 2026-04-20 15:45 donor-idle bug reappears.
func TestManagerCapitalRejection_FieldPopulation(t *testing.T) {
	tests := []struct {
		name     string
		setupMgr func(t *testing.T) (*Manager, func())
		// whichLeg is "long" when shortMarginWithBuffer cannot yet be known
		// (rejection fires before shortOB is fetched), and "short" when both
		// legs are computed.
		whichLeg string
	}{
		{
			name: "long_leg_margin_buffer",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 3.0
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 1000, Available: 50},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			whichLeg: "long",
		},
		{
			name: "long_leg_post_trade_ratio",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 1.0
				cfg.MarginL4Threshold = 0.05
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 100, Available: 99},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			whichLeg: "long",
		},
		{
			name: "short_leg_margin_buffer",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 3.0
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 1000, Available: 50},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			whichLeg: "short",
		},
		{
			name: "short_leg_post_trade_ratio",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 1.0
				cfg.MarginL4Threshold = 0.05
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 100, Available: 99},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			whichLeg: "short",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, cleanup := tc.setupMgr(t)
			defer cleanup()

			approval, err := m.Approve(baseOpp())
			if err != nil {
				t.Fatalf("Approve returned err: %v", err)
			}
			if approval == nil {
				t.Fatal("Approve returned nil approval")
			}
			if approval.Approved {
				t.Fatalf("expected rejection, got approved; reason=%s", approval.Reason)
			}
			if approval.Kind != models.RejectionKindCapital {
				t.Fatalf("expected RejectionKindCapital, got %s (reason=%s)", approval.Kind.String(), approval.Reason)
			}

			if approval.Size <= 0 {
				t.Errorf("Size must be > 0 for Pass 1 rescue pricing, got %v (reason=%s)", approval.Size, approval.Reason)
			}
			if approval.Price <= 0 {
				t.Errorf("Price must be > 0 for Pass 1 rescue pricing, got %v (reason=%s)", approval.Price, approval.Reason)
			}
			if approval.RequiredMargin <= 0 {
				t.Errorf("RequiredMargin must be > 0, got %v (reason=%s)", approval.RequiredMargin, approval.Reason)
			}
			if approval.LongMarginNeeded <= 0 {
				t.Errorf("LongMarginNeeded must be > 0 (pricedCapitalRejection fills from other side), got %v (reason=%s)", approval.LongMarginNeeded, approval.Reason)
			}
			if approval.ShortMarginNeeded <= 0 {
				t.Errorf("ShortMarginNeeded must be > 0 (pricedCapitalRejection fills from other side), got %v (reason=%s)", approval.ShortMarginNeeded, approval.Reason)
			}

			// RequiredMargin must equal max(long, short) so reservation uses the
			// heavier leg.
			var want float64
			if approval.LongMarginNeeded > approval.ShortMarginNeeded {
				want = approval.LongMarginNeeded
			} else {
				want = approval.ShortMarginNeeded
			}
			if approval.RequiredMargin != want {
				t.Errorf("RequiredMargin=%v want max(long=%v, short=%v)=%v (reason=%s)",
					approval.RequiredMargin, approval.LongMarginNeeded, approval.ShortMarginNeeded, want, approval.Reason)
			}
		})
	}
}
