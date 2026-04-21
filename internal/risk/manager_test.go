package risk

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// managerStubExchange is a minimal exchange.Exchange stub for manager tests.
type managerStubExchange struct {
	exchange.Exchange // embed nil for unimplemented methods
	name              string
	futuresBal        *exchange.Balance
	spotBal           *exchange.Balance
	orderbook         *exchange.Orderbook
	contracts         map[string]exchange.ContractInfo
}

func (s *managerStubExchange) Name() string { return s.name }
func (s *managerStubExchange) GetFuturesBalance() (*exchange.Balance, error) {
	if s.futuresBal != nil {
		return s.futuresBal, nil
	}
	return &exchange.Balance{Total: 10000, Available: 10000}, nil
}
func (s *managerStubExchange) GetSpotBalance() (*exchange.Balance, error) {
	if s.spotBal != nil {
		return s.spotBal, nil
	}
	return &exchange.Balance{}, nil
}
func (s *managerStubExchange) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	if s.orderbook != nil {
		return s.orderbook, nil
	}
	return deepOB(), nil
}
func (s *managerStubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	if s.contracts != nil {
		return s.contracts, nil
	}
	return map[string]exchange.ContractInfo{}, nil
}
func (s *managerStubExchange) SetMetricsCallback(fn exchange.MetricsCallback) {}
func (s *managerStubExchange) TransferToFutures(coin string, amount string) error { return nil }
func (s *managerStubExchange) TransferToSpot(coin string, amount string) error    { return nil }

// newTestManager creates a Manager with miniredis and the given exchanges + config.
func newTestManager(t *testing.T, exchanges map[string]exchange.Exchange, cfg *config.Config) (*Manager, func()) {
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
	m := &Manager{
		exchanges:       exchanges,
		db:              db,
		cfg:             cfg,
		log:             utils.NewLogger("test-manager"),
		spreadStability: NewSpreadStabilityChecker(db, cfg),
	}
	return m, func() { mr.Close() }
}

// defaultCfg returns a config that passes all gates when combined with wellFundedExch + deepOB.
// MaxLeverage() is hard-capped at 5, so always use Leverage=5 in fixtures.
func defaultCfg() *config.Config {
	return &config.Config{
		MaxPositions:            10,
		Leverage:                5, // hard cap is 5; using 5 avoids clamp surprises
		CapitalPerLeg:           500,
		MarginSafetyMultiplier:  1.1,
		MarginL4Threshold:       0.80,
		SlippageBPS:             200,
		MaxPriceGapBPS:          500,
		PriceGapFreeBPS:         500,  // disable gap recovery check by default
		MaxGapRecoveryIntervals: 100,
	}
}

// wellFundedExch returns a stub with ample margin (no capital rejection).
func wellFundedExch(name string) *managerStubExchange {
	return &managerStubExchange{
		name:       name,
		futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
	}
}

// deepOB returns an orderbook that will never trigger slippage/gap at small sizes.
// midPrice = (100.1+99.9)/2 = 100.0
func deepOB() *exchange.Orderbook {
	return &exchange.Orderbook{
		Asks: []exchange.PriceLevel{{Price: 100.1, Quantity: 100000}},
		Bids: []exchange.PriceLevel{{Price: 99.9, Quantity: 100000}},
	}
}

func baseOpp() models.Opportunity {
	return models.Opportunity{
		Symbol:        "TESTUSDT",
		LongExchange:  "alpha",
		ShortExchange: "beta",
		Spread:        10.0,
		IntervalHours: 8,
	}
}

func TestManagerApprove_RejectionKinds(t *testing.T) {
	// With leverage=5 (hard cap) and CapitalPerLeg=500, midPrice=100:
	//   size = capPerLeg*lev / price = 500*5/100 = 25
	//   longMarginPerLeg = 25 * 100 / 5 = 500
	//   longMarginWithBuffer = 500 * safetyMultiplier

	tests := []struct {
		name        string
		setupMgr    func(t *testing.T) (*Manager, func())
		opp         models.Opportunity
		wantApprove bool
		wantKind    models.RejectionKind
	}{
		// ---- line 234: Config — long exchange not configured ----
		{
			name: "line234_long_exchange_not_configured",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"beta": wellFundedExch("beta"),
					// alpha absent
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ---- line 240: Config — short exchange not configured ----
		{
			name: "line240_short_exchange_not_configured",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": wellFundedExch("alpha"),
					// beta absent
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ---- line 227: Config — max positions reached ----
		{
			name: "line227_max_positions_reached",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.MaxPositions = 0
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": wellFundedExch("alpha"),
					"beta":  wellFundedExch("beta"),
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ---- line 421: Market — empty orderbook on long exchange ----
		{
			name: "line421_empty_orderbook_long",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  &exchange.Orderbook{},
					},
					"beta": wellFundedExch("beta"),
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindMarket,
		},
		// ---- line 430: Capital — insufficient capital for minimum position size ----
		// Use CapitalPerLeg=0 + near-zero balance → auto-sizing yields 0
		{
			name: "line430_insufficient_capital",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 0
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 0.001, Available: 0.001},
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 0.001, Available: 0.001},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 438: notionalRejectionKind — Capital kind (effectiveCap*leverage >= 10) ----
		// effectiveCap=500, leverage=5 → 2500 >= 10 → Capital
		// Trigger: size*longMid < 10. With capPerLeg=500,lev=5,price=P: size=2500/P
		// longNotional = size*P = 2500. Always >= 10, so we need size itself < 10/P.
		// Use capPerLeg=1 (1*5=5 < 10) → Config kind. Need capPerLeg >= 2 for Capital.
		// capPerLeg=2: effectiveCap*lev=10 ≥ 10 → Capital. size=10/P, notional=10: passes!
		// capPerLeg=1.5: effectiveCap*lev=7.5 < 10 → Config.
		// For Capital: use capPerLeg=2. size=2*5/P=10/P. notional=10/P*P=10: passes.
		// We need notional < 10. size must be smaller. Use stepSize contract + small minSize
		// that rounds down to 0 — but that yields line 430 not 438.
		// Best approach: use very high price so notional drops below $10.
		// capPerLeg=500, lev=5 → effectiveCap*lev=2500 >= 10 → Capital kind.
		// size = 500*5/price. notional = size*price = 2500. Always >= 10.
		// So we can't trigger notional floor with capPerLeg=500.
		// Use capPerLeg=1.5 → effectiveCap*lev=7.5 < 10 → Config kind (line438 Config case).
		// For Capital kind with notional floor, need: effectiveCap*lev >= 10 AND size*price < 10.
		// size = capPerLeg*lev/price (from maxPositionValue), and notional = size*price = capPerLeg*lev.
		// capPerLeg*lev is always >= 10 when effectiveCap*lev >= 10 → notional = capPerLeg*lev >= 10.
		// Therefore: line 438 Capital kind is only reachable when capPerLeg=0 (auto-sizing).
		// In auto-sizing: size = available*lev/(slots*2) / price, notional = available*lev/(slots*2).
		// If available*lev/(slots*2) < 10 but available > 0 → size > 0 but notional < 10.
		// effectiveCap=0 → notionalRejectionKind(0, lev) → Capital kind.
		{
			name: "line438_long_notional_floor_capital_kind",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 0 // auto-sizing
				cfg.MaxPositions = 10
				// available=1, lev=5, slots=10: maxPosVal = 1*5/(10*2) = 0.25 → size = 0.0025 @ price=100
				// notional = 0.0025 * 100 = 0.25 < 10 → triggers notional floor
				// effectiveCap=0 → notionalRejectionKind(0,5) → Capital
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 1000, Available: 1},
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 1000, Available: 1},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 438: notionalRejectionKind — Config kind (effectiveCap*leverage < 10) ----
		// effectiveCap=1.5, leverage=5 → 7.5 < 10 → Config
		// size = 1.5*5/100 = 0.075; notional = 0.075*100 = 7.5 < 10 → triggers line438
		{
			name: "line438_long_notional_floor_config_kind",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 1.5
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
			wantApprove: false,
			wantKind:    models.RejectionKindConfig,
		},
		// ---- line 450: Capital — insufficient margin buffer on long ----
		// With capPerLeg=100, lev=5, price=100: size=5, longMarginPerLeg=100, buffer=100*safetyMult
		// safetyMultiplier=2.0: buffer=200. Available=50 < 200 → rejection.
		// But sizing uses maxFromBalance = available*lev/safetyMult = 50*5/2=125 capped.
		// size = min(100*5, 125)/100 = min(500,125)/100 = 1.25
		// longMarginPerLeg = 1.25*100/5 = 25; buffer = 25*2 = 50. Available=50 == 50: passes!
		// Need available < buffer. Try available=49: buffer = 25*2=50 > 49 → triggers.
		// But capPerLeg=100: maxPosVal = min(100*5=500, 49*5/2=122.5) = 122.5 → size=1.225
		// longMarginPerLeg=1.225*100/5=24.5; buffer=49 > 49: no, equal → passes.
		// Use safetyMultiplier=3: buffer = size*price/lev * 3. Need available < buffer.
		// Let available=X, safetyMult=3: size=min(500, X*5/3)/100
		// For X=50: size=min(500, 83.3)/100=0.833; margin=16.7; buffer=50.1 > 50 → triggers.
		{
			name: "line450_insufficient_margin_buffer_long",
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
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 465: Capital — post-trade margin ratio (long leg) ----
		// Need projectedRatio = 1 - (avail-margin)/total >= MarginL4Threshold
		// Use a very tight L4 threshold so the ratio easily exceeds it.
		// capPerLeg=100, lev=5, price=100: size depends on available.
		// Long: Total=100, Available=99, MarginL4Threshold=0.05 (very tight)
		// size = min(100*5=500, 99*5/1.1=450) / 100 = 4.5
		// longMarginPerLeg = 4.5*100/5 = 90
		// projectedAvail = 99 - 90 = 9; projectedRatio = 1 - 9/100 = 0.91 >= 0.05 → triggers
		{
			name: "line465_post_trade_ratio_long",
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
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 490: Market — empty orderbook on short exchange ----
		{
			name: "line490_empty_orderbook_short",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": wellFundedExch("alpha"),
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  &exchange.Orderbook{},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindMarket,
		},
		// ---- line 503: notionalRejectionKind — Capital kind (short leg) ----
		// Same logic as line438 Capital case but short leg.
		// effectiveCap=0 (auto-sizing), small balance → size*shortMid < 10.
		// long OB at price 100 passes long notional; need short OB at same or close price.
		// With auto-sizing and available=1: notional=0.25 < 10, effectiveCap=0 → Capital.
		// Long notional check: size*longMid = 0.0025*100 = 0.25 < 10 → long check fires at line 438.
		// To reach line 503, long notional must pass. Use CapitalPerLeg > 0 so long notional = capPerLeg*lev >= 10.
		// capPerLeg=5, lev=5: effectiveCap*lev=25 >= 10 → Capital kind.
		// longNotional = size*longMid = 5*5 = 25 >= 10 ✓
		// shortMid must give shortNotional < 10: size=25/100=0.25 → shortNotional=0.25*shortMid
		// Need 0.25*shortMid < 10 → shortMid < 40. Use shortMid=0.1.
		{
			name: "line503_short_notional_floor_capital_kind",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 5
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 1.0 // disable buffer check
				cfg.MarginL4Threshold = 0.999    // disable post-trade ratio check
				cfg.SlippageBPS = 100000         // disable slippage check
				cfg.MaxPriceGapBPS = 100000      // disable gap check
				cfg.PriceGapFreeBPS = 100000     // disable gap recovery check
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(), // longMid=100
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							// shortMid = 0.095; size=5*5/100=0.25; shortNotional=0.25*0.095=0.024 < 10
							Asks: []exchange.PriceLevel{{Price: 0.1, Quantity: 10000000}},
							Bids: []exchange.PriceLevel{{Price: 0.09, Quantity: 10000000}},
						},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 503: notionalRejectionKind — Config kind (short leg) ----
		// effectiveCap=1.5, lev=5 → 7.5 < 10 → Config. But long notional = 1.5*5=7.5 < 10 → fires at 438.
		// To reach line 503 with Config kind: need long notional >= 10 but effectiveCap*lev < 10.
		// long notional = capPerLeg*lev regardless of price (= maxPosVal). If capPerLeg*lev < 10, long fires first.
		// This site is structurally unreachable via the Config-kind path alone through normal approval flow:
		// whenever effectiveCap*lev < 10, the long leg check at line 438 fires first.
		// We test the Config branch of notionalRejectionKind via line 438 Config case (same helper, same logic).
		// This row exercises it directly as a unit test via TestNotionalRejectionKind below.
		// Placeholder: re-exercise the line438 Config path with short leg having deep OB.
		{
			name: "line503_short_notional_floor_config_kind_via_long",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 1.5 // effectiveCap*lev=7.5 < 10 → Config
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
			wantApprove: false,
			wantKind:    models.RejectionKindConfig, // fires at line 438 long check first; same notionalRejectionKind branch
		},
		// ---- line 512: Capital — insufficient margin buffer on short ----
		// Mirror of line 450 but for short leg. Long has ample balance; short is tight.
		{
			name: "line512_insufficient_margin_buffer_short",
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
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 525: Capital — post-trade margin ratio (short leg) ----
		// Long passes; short fails ratio. Mirror of line 465.
		{
			name: "line525_post_trade_ratio_short",
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
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapital,
		},
		// ---- line 563: Spread — slippage too high (no adaptive path: no contract info) ----
		{
			name: "line563_slippage_too_high_no_adaptive",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 0.001 // near-zero → any spread triggers
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						// Skewed OB → high slippage; no contracts so stepSize=0 → no adaptive path
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 110, Quantity: 10000}},
							Bids: []exchange.PriceLevel{{Price: 90, Quantity: 10000}},
						},
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 110, Quantity: 10000}},
							Bids: []exchange.PriceLevel{{Price: 90, Quantity: 10000}},
						},
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 588: Spread — slippage too high at minimum size (adaptive exhausted) ----
		{
			name: "line588_slippage_too_high_at_min_size",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 0.001 // near-zero limit
				contracts := map[string]exchange.ContractInfo{
					"TESTUSDT": {Symbol: "TESTUSDT", StepSize: 1, MinSize: 1},
				}
				// Shallow OB with wide spread → even minSize=1 has huge slippage
				skewedOB := &exchange.Orderbook{
					Asks: []exchange.PriceLevel{{Price: 200, Quantity: 0.001}},
					Bids: []exchange.PriceLevel{{Price: 50, Quantity: 0.001}},
				}
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  skewedOB,
						contracts:  contracts,
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  skewedOB,
						contracts:  contracts,
					},
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 613: Spread — price gap too high (absolute hard cap) ----
		{
			name: "line613_price_gap_too_high",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 100000  // allow slippage
				cfg.MaxPriceGapBPS = 0.01 // near-zero hard cap
				// long ~100, short ~200 → enormous gap
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook:  deepOB(), // mid=100
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
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 627: Spread — price gap with zero funding spread ----
		{
			name: "line627_price_gap_zero_spread",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 100000
				cfg.MaxPriceGapBPS = 100000 // pass hard cap
				cfg.PriceGapFreeBPS = 1     // very tight; any gap > 1 bps triggers recovery check
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						// long VWAP ask at 101; short VWAP bid at 100; gap = 1 bps > PriceGapFreeBPS(1)
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 101, Quantity: 100000}},
							Bids: []exchange.PriceLevel{{Price: 99, Quantity: 100000}},
						},
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 100.1, Quantity: 100000}},
							Bids: []exchange.PriceLevel{{Price: 99.9, Quantity: 100000}},
						},
					},
				}, cfg)
			},
			opp: models.Opportunity{
				Symbol:        "TESTUSDT",
				LongExchange:  "alpha",
				ShortExchange: "beta",
				Spread:        0,  // zero → triggers line 627
				IntervalHours: 8,
			},
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 639: Spread — gap recovery intervals exceeded ----
		{
			name: "line639_gap_recovery_exceeded",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.SlippageBPS = 100000
				cfg.MaxPriceGapBPS = 100000
				cfg.PriceGapFreeBPS = 1           // any gap > 1 bps triggers recovery check
				cfg.MaxGapRecoveryIntervals = 0.001 // near-zero → recovery always exceeds
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 110, Quantity: 100000}},
							Bids: []exchange.PriceLevel{{Price: 90, Quantity: 100000}},
						},
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 10000, Available: 10000},
						orderbook: &exchange.Orderbook{
							Asks: []exchange.PriceLevel{{Price: 100.1, Quantity: 100000}},
							Bids: []exchange.PriceLevel{{Price: 99.9, Quantity: 100000}},
						},
					},
				}, cfg)
			},
			opp: models.Opportunity{
				Symbol:        "TESTUSDT",
				LongExchange:  "alpha",
				ShortExchange: "beta",
				Spread:        1.0, // positive → passes zero-spread check
				IntervalHours: 8,
			},
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 649: Spread — spread stability rejection ----
		{
			name: "line649_spread_stability",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.EnableSpreadStabilityGate = true
				cfg.SpreadVolatilityMinSamples = 2
				cfg.SpreadVolatilityMaxCV = 0.01 // tight CV gate; alternating 1/100 has CV ~0.97
				mr, err := miniredis.Run()
				if err != nil {
					t.Fatalf("miniredis: %v", err)
				}
				db, err := database.New(mr.Addr(), "", 0)
				if err != nil {
					mr.Close()
					t.Fatalf("database.New: %v", err)
				}
				now := time.Now().UTC()
				// Seed highly volatile spread history (CV >> 0.01)
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
				t.Cleanup(func() { mr.Close() })
				return m, func() {}
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindSpread,
		},
		// ---- line 657: Health — exchange health too low (long) ----
		// Record only failed/slow requests on alpha so its latency score drops below threshold.
		// With ExchHealthLatencyMs=1 (1ms threshold) and latency=10s, latencyScore=clamp(1-10000,0,1)=0.
		// UptimeScore and FillRateScore default to 1.0 (no WS/order data).
		// Score = 0*0.3 + 1*0.4 + 1*0.3 = 0.7. Use minScore=0.99 → 0.7 < 0.99 → rejected.
		{
			name: "line657_health_too_low_long",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.EnableExchangeHealthScoring = true
				cfg.ExchHealthMinScore = 0.99   // require near-perfect
				cfg.ExchHealthLatencyMs = 1     // 1ms threshold — 10s latency → score=0
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
				// Record many very slow requests on alpha → latencyScore=0 → overall score~0.7
				for i := 0; i < 50; i++ {
					scorer.RecordLatency("alpha", "test", 10*time.Second, nil)
				}
				m.SetExchangeScorer(scorer)
				return m, cleanup
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindHealth,
		},
		// ---- line 663: Health — exchange health too low (short) ----
		// alpha scores above threshold; beta has degraded latency → fails.
		{
			name: "line663_health_too_low_short",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.EnableExchangeHealthScoring = true
				cfg.ExchHealthMinScore = 0.99
				cfg.ExchHealthLatencyMs = 1 // 1ms threshold
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
				// alpha: record fast latencies → latencyScore=1.0, overall score=1.0 ≥ 0.99
				for i := 0; i < 50; i++ {
					scorer.RecordLatency("alpha", "test", 1*time.Microsecond, nil)
				}
				// beta: record very slow latencies → latencyScore=0 → overall score~0.7 < 0.99
				for i := 0; i < 50; i++ {
					scorer.RecordLatency("beta", "test", 10*time.Second, nil)
				}
				m.SetExchangeScorer(scorer)
				return m, cleanup
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindHealth,
		},
		// ---- line 697: Capacity — would exceed % capital cap (long) ----
		// totalCapital = alpha.Total + beta.Total; maxExposure = 60% * total
		// longExposure = longMarginPerLeg = size * longMid / lev
		// With capPerLeg=100, lev=5, price=100: size=5, longMarginPerLeg=100
		// totalCapital must be small enough that 100 > maxExposure.
		// maxExposure = 0.6 * totalCapital; need 100 > 0.6*T → T < 166.7
		// Use alpha.Total=50, beta.Total=50 → total=100; maxExposure=60 < 100 → triggers.
		// But available must be enough for sizing: available=10000 (separate from Total).
		{
			name: "line697_capacity_cap_long",
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
			wantApprove: false,
			wantKind:    models.RejectionKindCapacity,
		},
		// ---- line 702: Capacity — would exceed % capital cap (short) ----
		// Seed an existing position with short on beta to inflate shortExposure past cap
		// while long stays under cap.
		{
			name: "line702_capacity_cap_short",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				cfg.CapitalPerLeg = 100
				cfg.Leverage = 5
				cfg.MarginSafetyMultiplier = 1.0
				cfg.MarginL4Threshold = 0.999 // disable post-trade ratio check
				// alpha.Total large so long exposure fraction stays small;
				// beta.Total large but existing beta position inflates shortExposure.
				// totalCapital = 200 + 200 = 400; maxExposure = 0.6*400 = 240
				// shortExposure starts at longMarginPerLeg = 100 (new leg)
				// + existing position on beta short: 10*100/5 = 200. Total shortExposure=300 > 240.
				m, cleanup := newTestManager(t, map[string]exchange.Exchange{
					"alpha": &managerStubExchange{
						name:       "alpha",
						futuresBal: &exchange.Balance{Total: 200, Available: 10000},
						orderbook:  deepOB(),
					},
					"beta": &managerStubExchange{
						name:       "beta",
						futuresBal: &exchange.Balance{Total: 200, Available: 10000},
						orderbook:  deepOB(),
					},
				}, cfg)
				// Seed existing position: short on beta with ShortSize=10, ShortEntry=100
				// shortExposure from existing = 10*100/5 = 200
				_ = m.db.SavePosition(&models.ArbitragePosition{
					ID:            "existing-1",
					Symbol:        "BTCUSDT",
					LongExchange:  "alpha",
					ShortExchange: "beta",
					LongSize:      0.1, LongEntry: 100,
					ShortSize:  10, ShortEntry: 100,
					Status: models.StatusActive,
				})
				return m, cleanup
			},
			opp:         baseOpp(),
			wantApprove: false,
			wantKind:    models.RejectionKindCapacity,
		},
		// ---- approved case: all checks pass ----
		{
			name: "approved_all_checks_pass",
			setupMgr: func(t *testing.T) (*Manager, func()) {
				cfg := defaultCfg()
				return newTestManager(t, map[string]exchange.Exchange{
					"alpha": wellFundedExch("alpha"),
					"beta":  wellFundedExch("beta"),
				}, cfg)
			},
			opp:         baseOpp(),
			wantApprove: true,
			wantKind:    models.RejectionKindNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := tt.setupMgr(t)
			defer cleanup()

			approval, err := m.SimulateApproval(tt.opp, nil)
			if err != nil {
				t.Fatalf("SimulateApproval error: %v", err)
			}
			if approval.Approved != tt.wantApprove {
				t.Errorf("Approved = %v, want %v (reason: %s)", approval.Approved, tt.wantApprove, approval.Reason)
			}
			if approval.Kind != tt.wantKind {
				t.Errorf("Kind = %v (%s), want %v (%s) (reason: %s)",
					approval.Kind, approval.Kind, tt.wantKind, tt.wantKind, approval.Reason)
			}
		})
	}
}

// TestNotionalRejectionKind pins the helper logic directly without going through approveInternal.
func TestNotionalRejectionKind(t *testing.T) {
	cases := []struct {
		effectiveCap float64
		leverage     int
		want         models.RejectionKind
	}{
		{0, 5, models.RejectionKindCapital},   // effectiveCap=0 → Capital
		{500, 5, models.RejectionKindCapital}, // 500*5=2500 >= 10 → Capital
		{1.5, 5, models.RejectionKindConfig},  // 1.5*5=7.5 < 10 → Config
		{0.9, 5, models.RejectionKindConfig},  // 0.9*5=4.5 < 10 → Config
		{2.0, 5, models.RejectionKindCapital}, // 2.0*5=10 >= 10 → Capital
		{0.1, 5, models.RejectionKindConfig},  // 0.1*5=0.5 < 10 → Config
	}
	for _, c := range cases {
		got := notionalRejectionKind(c.effectiveCap, c.leverage)
		if got != c.want {
			t.Errorf("notionalRejectionKind(%.2f, %d) = %v, want %v", c.effectiveCap, c.leverage, got, c.want)
		}
	}
}
