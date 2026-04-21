package risk

import (
	"testing"
	"time"

	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// TestSimulateApprovalForPair_Kind pins RejectionKind population on every
// rejection return path exercised through SimulateApprovalForPair. Step 1 of
// PLAN-allocator-unified.md requires the allocator to classify
// rejections (capital-curable vs not); missing Kind would silently drop
// rescue candidates.
func TestSimulateApprovalForPair_Kind(t *testing.T) {
	tests := []struct {
		name        string
		setupMgr    func(t *testing.T) (*Manager, func())
		opp         models.Opportunity
		long, short string
		wantApprove bool
		wantKind    models.RejectionKind
	}{
		// ----- Config: max positions reached -----
		{
			name: "config_max_positions",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.MaxPositions = 0
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": wellFundedExch("alpha"),
					"beta":  wellFundedExch("beta"),
				}, cfg)
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ----- Config: per-exchange max notional too low to pass $10 floor -----
		// effectiveCap=1.5 leverage=5 → notional floor fires with Config kind.
		{
			name: "config_per_exchange_max_notional_floor",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 1.5 // effectiveCap*lev = 7.5 < 10 → Config
				cfg.Leverage = 5
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ----- Market: empty orderbook on long exchange -----
		{
			name: "market_empty_orderbook_long",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  &exchange.Orderbook{}, // empty
					},
					"beta": wellFundedExch("beta"),
				}, cfg)
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindMarket,
		},
		// ----- Capital: insufficient margin buffer (long leg) -----
		{
			name: "capital_insufficient_margin_buffer_long",
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
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ----- Spread: extreme price gap triggers hard cap -----
		{
			name: "spread_price_gap_hard_cap",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 100000
				cfg.MaxPriceGapBPS = 0.01 // near-zero hard cap
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(), // mid ~100
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 200.1, Quantity: 100000}},
							Bids: []exchange.PriceLevel{{Price: 199.9, Quantity: 100000}},
						},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ----- Capacity: would exceed per-exchange capital cap -----
		{
			name: "capacity_exposure_cap_long",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 1.0
				cfg.MarginL4Threshold = 0.999 // disable post-trade ratio check
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 50, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 50, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindCapacity,
		},
		// ----- Health: exchange health scorer below minimum -----
		{
			name: "health_exchange_score_too_low",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.EnableExchangeHealthScoring = true
				cfg.ExchHealthMinScore = 0.99
				cfg.ExchHealthLatencyMs = 1
				m, cleanup := newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
				scorer := NewExchangeScorer(cfg)
				// very slow latency → latencyScore≈0; overall score < 0.99
				for i := 0; i < 50; i++ {
					scorer.RecordLatency("alpha", "test", 10*time.Second, nil)
				}
				m.SetExchangeScorer(scorer)
				return m, cleanup
			},
			opp:         baseOpp(),
			long:        "alpha",
			short:       "beta",
			wantApprove: false,
			wantKind:    models.RejectionKindHealth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, cleanup := tc.setupMgr(t)
			defer cleanup()

			approval, err := m.SimulateApprovalForPair(tc.opp, tc.long, tc.short, nil, nil, nil)
			if err != nil {
				t.Fatalf("SimulateApprovalForPair error: %v", err)
			}
			if approval == nil {
				t.Fatal("approval is nil")
			}
			if approval.Approved != tc.wantApprove {
				t.Errorf("Approved = %v, want %v (reason=%s)", approval.Approved, tc.wantApprove, approval.Reason)
			}
			if approval.Kind != tc.wantKind {
				t.Errorf("Kind = %v (%s), want %v (%s) (reason=%s)",
					approval.Kind, approval.Kind, tc.wantKind, tc.wantKind, approval.Reason)
			}
		})
	}
}

// TestSimulateApprovalForPair_Kind_Spread_Stability triggers the spread
// stability CV gate (populated Redis history with high volatility) so the
// Kind=Spread return from spreadStability.Check is covered too.
func TestSimulateApprovalForPair_Kind_Spread_Stability(t *testing.T) {
	cfg := defaultCfg()
	cfg.CapitalPerLeg = 100
	cfg.Leverage = 5
	cfg.EnableSpreadStabilityGate = true
	cfg.SpreadVolatilityMinSamples = 2
	cfg.SpreadVolatilityMaxCV = 0.01 // very tight; volatile history will fail

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })

	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	// Seed highly volatile spread history on the pair.
	now := time.Now().UTC()
	for i, spread := range []float64{1.0, 100.0, 1.0, 100.0} {
		opp := models.Opportunity{
			Symbol:        "TESTUSDT",
			LongExchange:  "alpha",
			ShortExchange: "beta",
			Spread:        spread,
			IntervalHours: 8,
		}
		_ = db.AddSpreadHistoryBatch([]models.Opportunity{opp}, now.Add(-time.Duration(i)*time.Hour))
	}

	exchanges := map[string]exchange.Exchange{
		"alpha": &managerStubExchange{
			name:       "alpha",
			futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
			orderbook:  deepOB(),
		},
		"beta": &managerStubExchange{
			name:       "beta",
			futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
			orderbook:  deepOB(),
		},
	}
	m := &Manager{
		exchanges:       exchanges,
		db:              db,
		cfg:             cfg,
		log:             utils.NewLogger("test-manager"),
		spreadStability: NewSpreadStabilityChecker(db, cfg),
	}

	approval, err := m.SimulateApprovalForPair(baseOpp(), "alpha", "beta", nil, nil, nil)
	if err != nil {
		t.Fatalf("SimulateApprovalForPair: %v", err)
	}
	if approval == nil {
		t.Fatal("approval is nil")
	}
	if approval.Approved {
		t.Fatalf("expected rejection, got approved; reason=%s", approval.Reason)
	}
	if approval.Kind != models.RejectionKindSpread {
		t.Errorf("Kind = %v (%s), want Spread (reason=%s)", approval.Kind, approval.Kind, approval.Reason)
	}
}

