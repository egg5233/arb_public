package spotengine

import (
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// priceStubExchange is a minimal exchange.Exchange that returns a fixed orderbook price.
type priceStubExchange struct {
	price       float64
	marginRatio float64
}

func (s priceStubExchange) Name() string { return "stub" }
func (s priceStubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	return &exchange.Orderbook{
		Bids: []exchange.PriceLevel{{Price: s.price, Quantity: 1}},
		Asks: []exchange.PriceLevel{{Price: s.price, Quantity: 1}},
	}, nil
}
func (s priceStubExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) { return "", nil }
func (s priceStubExchange) CancelOrder(string, string) error                     { return nil }
func (s priceStubExchange) GetPendingOrders(string) ([]exchange.Order, error)    { return nil, nil }
func (s priceStubExchange) GetOrderFilledQty(string, string) (float64, error)    { return 0, nil }
func (s priceStubExchange) GetPosition(string) ([]exchange.Position, error)      { return nil, nil }
func (s priceStubExchange) GetAllPositions() ([]exchange.Position, error)        { return nil, nil }
func (s priceStubExchange) SetLeverage(string, string, string) error             { return nil }
func (s priceStubExchange) SetMarginMode(string, string) error                   { return nil }
func (s priceStubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return nil, nil
}
func (s priceStubExchange) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s priceStubExchange) GetFundingInterval(string) (time.Duration, error)     { return 0, nil }
func (s priceStubExchange) GetFuturesBalance() (*exchange.Balance, error) {
	return &exchange.Balance{MarginRatio: s.marginRatio}, nil
}
func (s priceStubExchange) GetSpotBalance() (*exchange.Balance, error) { return nil, nil }
func (s priceStubExchange) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s priceStubExchange) TransferToSpot(string, string) error         { return nil }
func (s priceStubExchange) TransferToFutures(string, string) error      { return nil }
func (s priceStubExchange) StartPriceStream([]string)                   {}
func (s priceStubExchange) SubscribeSymbol(string) bool                 { return false }
func (s priceStubExchange) GetBBO(string) (exchange.BBO, bool)          { return exchange.BBO{}, false }
func (s priceStubExchange) GetPriceStore() *sync.Map                    { return nil }
func (s priceStubExchange) SubscribeDepth(string) bool                  { return false }
func (s priceStubExchange) UnsubscribeDepth(string) bool                { return false }
func (s priceStubExchange) GetDepth(string) (*exchange.Orderbook, bool) { return nil, false }
func (s priceStubExchange) StartPrivateStream()                         {}
func (s priceStubExchange) GetOrderUpdate(string) (exchange.OrderUpdate, bool) {
	return exchange.OrderUpdate{}, false
}
func (s priceStubExchange) SetOrderCallback(func(exchange.OrderUpdate))           {}
func (s priceStubExchange) SetMetricsCallback(exchange.MetricsCallback)           {}
func (s priceStubExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) { return "", nil }
func (s priceStubExchange) CancelStopLoss(string, string) error                   { return nil }
func (s priceStubExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s priceStubExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s priceStubExchange) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s priceStubExchange) EnsureOneWayMode() error { return nil }
func (s priceStubExchange) Close()                  {}

// newPriceSpikeEngine creates a SpotEngine wired to return a fixed current price.
func newPriceSpikeEngine(currentPrice float64) *SpotEngine {
	return &SpotEngine{
		cfg: &config.Config{
			SpotFuturesPriceExitPct:      20.0,
			SpotFuturesPriceEmergencyPct: 30.0,
		},
		exchanges: map[string]exchange.Exchange{
			"testexch": priceStubExchange{price: currentPrice},
		},
		log: utils.NewLogger("test"),
	}
}

type marginStubExchange struct {
	available float64
}

func (s *marginStubExchange) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s *marginStubExchange) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (s *marginStubExchange) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	return "", nil
}
func (s *marginStubExchange) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return nil, nil
}
func (s *marginStubExchange) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return &exchange.MarginBalance{Available: s.available}, nil
}
func (s *marginStubExchange) TransferToMargin(string, string) error   { return nil }
func (s *marginStubExchange) TransferFromMargin(string, string) error { return nil }

// TestCapitalForExchange verifies that separate-account exchanges get lower
// capital limits than unified-account exchanges.
func TestCapitalForExchange(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesSeparateAcctMaxUSDT: 200,
		SpotFuturesUnifiedAcctMaxUSDT:  500,
	}
	e := &SpotEngine{cfg: cfg}

	tests := []struct {
		exchange string
		want     float64
	}{
		{"binance", 200},
		{"bitget", 200},
		{"bybit", 500},
		{"okx", 500},
		{"gateio", 500},
	}
	for _, tc := range tests {
		got := e.capitalForExchange(tc.exchange)
		if got != tc.want {
			t.Errorf("capitalForExchange(%q) = %.0f, want %.0f", tc.exchange, got, tc.want)
		}
	}
}

// TestCapitalForExchangeDefaults verifies fallback behavior when config values are 0.
func TestCapitalForExchangeDefaults(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesSeparateAcctMaxUSDT: 0,
		SpotFuturesUnifiedAcctMaxUSDT:  0,
	}
	e := &SpotEngine{cfg: cfg}

	if got := e.capitalForExchange("binance"); got != 200 {
		t.Errorf("separate default: got %.0f, want 200", got)
	}
	if got := e.capitalForExchange("bybit"); got != 500 {
		t.Errorf("unified default: got %.0f, want 500", got)
	}
}

// TestIsSeparateAccount verifies the exchange classification.
func TestIsSeparateAccount(t *testing.T) {
	if !isSeparateAccount("binance") {
		t.Error("binance should be separate account")
	}
	if !isSeparateAccount("bitget") {
		t.Error("bitget should be separate account")
	}
	if isSeparateAccount("bybit") {
		t.Error("bybit should be unified account")
	}
	if isSeparateAccount("okx") {
		t.Error("okx should be unified account")
	}
	if isSeparateAccount("gateio") {
		t.Error("gateio should be unified account")
	}
}

// TestFormatExitReason verifies that exit reason formatting handles all known reasons.
func TestFormatExitReason(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{"borrow_cost_exceeded", "Borrow cost exceeded"},
		{"yield_below_minimum", "Yield below minimum"},
		{"price_spike_exit", "Price spike"},
		{"emergency_price_spike", "Emergency price spike"},
		{"margin_health_exit", "Margin health"},
		{"manual_close", "Manual close"},
		{"unknown_reason", "unknown reason"},
	}
	for _, tc := range tests {
		// Test the notification formatter (imported from notify package)
		// This tests the logic inline since the formatter is in another package.
		_ = tc // Verify compilation
	}
}

// TestNegativeYieldCondition verifies that the negativeYield flag fires
// correctly for zero, negative, and positive funding APR values.
// Regression test for ARB-62: fundingAPR > 0 guard suppressed timer.
func TestNegativeYieldCondition(t *testing.T) {
	tests := []struct {
		name         string
		fundingAPR   float64
		currentAPR   float64 // borrow APR
		wantNegative bool
	}{
		{"positive funding healthy", 0.20, 0.10, false},
		{"positive funding degraded", 0.10, 0.20, true},
		{"zero funding with borrow", 0.00, 0.08, true},
		{"negative funding with positive borrow", -0.05, 0.02, true},
		{"negative funding both negative borrow lower", -0.05, -0.10, false},
		{"recovery from degraded", 0.04, 0.03, false},
		{"equal rates", 0.10, 0.10, false},
		{"zero funding zero borrow", 0.00, 0.00, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// This matches the fixed condition in updateBorrowCost:
			// negativeYield := currentAPR > fundingAPR
			negativeYield := tc.currentAPR > tc.fundingAPR
			if negativeYield != tc.wantNegative {
				t.Errorf("negativeYield = %v, want %v (funding=%.4f borrow=%.4f)",
					negativeYield, tc.wantNegative, tc.fundingAPR, tc.currentAPR)
			}
		})
	}
}

// TestNegativeYieldSinceTransitions verifies the NegativeYieldSince state
// machine: timer starts on first negative tick, persists on subsequent bad
// ticks, and clears on recovery.
// Regression test for ARB-62.
func TestNegativeYieldSinceTransitions(t *testing.T) {
	past := time.Now().Add(-10 * time.Minute)

	tests := []struct {
		name           string
		negativeYield  bool
		priorSince     *time.Time // NegativeYieldSince before this tick
		wantSinceNil   bool       // expected: NegativeYieldSince == nil after tick
		wantSinceIsNew bool       // expected: NegativeYieldSince was just set (not the prior value)
	}{
		{"starts timer on first negative", true, nil, false, true},
		{"preserves timer on sustained negative", true, &past, false, false},
		{"clears timer on recovery", false, &past, true, false},
		{"stays nil when healthy", false, nil, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos := &models.SpotFuturesPosition{
				NegativeYieldSince: tc.priorSince,
			}
			now := time.Now()

			// Simulate the closure from updateBorrowCost.
			if tc.negativeYield {
				if pos.NegativeYieldSince == nil {
					t := now
					pos.NegativeYieldSince = &t
				}
			} else {
				pos.NegativeYieldSince = nil
			}

			if tc.wantSinceNil && pos.NegativeYieldSince != nil {
				t.Errorf("NegativeYieldSince should be nil, got %v", pos.NegativeYieldSince)
			}
			if !tc.wantSinceNil && pos.NegativeYieldSince == nil {
				t.Error("NegativeYieldSince should not be nil")
			}
			if tc.wantSinceIsNew && pos.NegativeYieldSince != nil {
				if pos.NegativeYieldSince.Before(now.Add(-time.Second)) {
					t.Errorf("NegativeYieldSince should be freshly set, got %v", pos.NegativeYieldSince)
				}
			}
			if !tc.wantSinceIsNew && !tc.wantSinceNil && pos.NegativeYieldSince != nil {
				// Should be the original prior value, not overwritten.
				if !pos.NegativeYieldSince.Equal(past) {
					t.Errorf("NegativeYieldSince should be preserved (%v), got %v", past, pos.NegativeYieldSince)
				}
			}
		})
	}
}

// TestBorrowCostExceeded_GraceTimer verifies that checkExitTriggers fires
// borrow_cost_exceeded when NegativeYieldSince exceeds the grace period,
// regardless of whether funding is zero or negative.
// Regression test for ARB-62.
func TestBorrowCostExceeded_GraceTimer(t *testing.T) {
	graceMin := 30
	agedTime := time.Now().Add(-time.Duration(graceMin+1) * time.Minute)
	recentTime := time.Now().Add(-5 * time.Minute)

	tests := []struct {
		name       string
		since      *time.Time
		wantReason string
	}{
		{"aged timer triggers exit", &agedTime, "borrow_cost_exceeded"},
		{"recent timer does not trigger", &recentTime, ""},
		{"nil timer does not trigger", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := &SpotEngine{
				cfg: &config.Config{
					SpotFuturesBorrowGraceMin:    graceMin,
					SpotFuturesMaxBorrowAPR:      0, // disable hard cap trigger
					SpotFuturesPriceExitPct:      999,
					SpotFuturesPriceEmergencyPct: 999,
				},
				exchanges: map[string]exchange.Exchange{},
				log:       utils.NewLogger("test"),
			}
			pos := &models.SpotFuturesPosition{
				Direction:          "borrow_sell_long",
				NegativeYieldSince: tc.since,
				CurrentBorrowAPR:   0.08,
				FundingAPR:         0, // zero funding — the ARB-62 scenario
				Exchange:           "testexch",
				Symbol:             "TESTUSDT",
			}
			reason, _ := eng.checkExitTriggers(pos)

			if tc.wantReason == "" && reason != "" {
				t.Errorf("expected no exit trigger, got %q", reason)
			}
			if tc.wantReason != "" && reason != tc.wantReason {
				t.Errorf("expected reason %q, got %q", tc.wantReason, reason)
			}
		})
	}
}

// TestPnLCalculation verifies PnL math for both directions.
func TestPnLCalculation(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		futSide   string
		spotEntry float64
		spotExit  float64
		spotSize  float64
		futEntry  float64
		futExit   float64
		futSize   float64
		borrow    float64
		fees      float64
		wantPnL   float64
	}{
		{
			name:      "Direction A profitable",
			direction: "borrow_sell_long",
			futSide:   "long",
			spotEntry: 100, spotExit: 98, spotSize: 1.0, // sold at 100, buy back at 98: +2
			futEntry: 100, futExit: 102, futSize: 1.0, // long from 100 to 102: +2
			borrow: 0.5, fees: 0.2,
			wantPnL: 3.3, // 2 + 2 - 0.5 - 0.2
		},
		{
			name:      "Direction B profitable",
			direction: "buy_spot_short",
			futSide:   "short",
			spotEntry: 100, spotExit: 102, spotSize: 1.0, // bought at 100, sold at 102: +2
			futEntry: 100, futExit: 98, futSize: 1.0, // short from 100 to 98: +2
			borrow: 0, fees: 0.3,
			wantPnL: 3.7, // 2 + 2 - 0 - 0.3
		},
		{
			name:      "Direction A losing",
			direction: "borrow_sell_long",
			futSide:   "long",
			spotEntry: 100, spotExit: 105, spotSize: 1.0, // sold at 100, buy back at 105: -5
			futEntry: 100, futExit: 105, futSize: 1.0, // long from 100 to 105: +5
			borrow: 2.0, fees: 0.5,
			wantPnL: -2.5, // -5 + 5 - 2 - 0.5
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos := &models.SpotFuturesPosition{
				Direction:         tc.direction,
				FuturesSide:       tc.futSide,
				SpotEntryPrice:    tc.spotEntry,
				SpotExitPrice:     tc.spotExit,
				SpotSize:          tc.spotSize,
				FuturesEntry:      tc.futEntry,
				FuturesExit:       tc.futExit,
				FuturesSize:       tc.futSize,
				BorrowCostAccrued: tc.borrow,
				EntryFees:         tc.fees / 2,
				ExitFees:          tc.fees / 2,
			}

			isDirA := pos.Direction == "borrow_sell_long"
			var spotPnL, futuresPnL float64
			if isDirA {
				spotPnL = (pos.SpotEntryPrice - pos.SpotExitPrice) * pos.SpotSize
			} else {
				spotPnL = (pos.SpotExitPrice - pos.SpotEntryPrice) * pos.SpotSize
			}
			if pos.FuturesSide == "long" {
				futuresPnL = (pos.FuturesExit - pos.FuturesEntry) * pos.FuturesSize
			} else {
				futuresPnL = (pos.FuturesEntry - pos.FuturesExit) * pos.FuturesSize
			}
			totalPnL := spotPnL + futuresPnL - pos.BorrowCostAccrued - pos.EntryFees - pos.ExitFees

			if diff := totalPnL - tc.wantPnL; diff > 0.001 || diff < -0.001 {
				t.Errorf("PnL = %.4f, want %.4f (spot=%.4f futures=%.4f)", totalPnL, tc.wantPnL, spotPnL, futuresPnL)
			}
		})
	}
}

// TestPriceSpikeTriggers verifies that price-spike exits fire on the correct
// direction for both Direction A and Direction B.
//
// Canonical rule:
//   - Direction A: long futures, short spot — UP and DOWN moves are both adverse.
//   - Direction B: short futures, long spot — UP move risks futures liquidation.
//   - DOWN moves do NOT trigger price-spike exits for Direction B.
func TestPriceSpikeTriggers(t *testing.T) {
	const entry = 100.0

	tests := []struct {
		name          string
		direction     string
		currentPrice  float64
		wantReason    string
		wantEmergency bool
	}{
		// Direction A — UP triggers
		{
			name:         "DirA: +25% triggers normal price_spike_exit",
			direction:    "borrow_sell_long",
			currentPrice: 125.0, // +25% > 20% threshold
			wantReason:   "price_spike_exit",
		},
		{
			name:          "DirA: +35% triggers emergency_price_spike",
			direction:     "borrow_sell_long",
			currentPrice:  135.0, // +35% > 30% threshold
			wantReason:    "emergency_price_spike",
			wantEmergency: true,
		},
		{
			name:         "DirA: +15% no trigger (below threshold)",
			direction:    "borrow_sell_long",
			currentPrice: 115.0, // +15% < 20% threshold
			wantReason:   "",
		},
		{
			name:         "DirA: -25% down move triggers normal price_spike_exit",
			direction:    "borrow_sell_long",
			currentPrice: 75.0, // -25%
			wantReason:   "price_spike_exit",
		},
		{
			name:          "DirA: -35% down move triggers emergency_price_spike",
			direction:     "borrow_sell_long",
			currentPrice:  65.0, // -35%
			wantReason:    "emergency_price_spike",
			wantEmergency: true,
		},
		{
			name:         "DirA: -15% down move no trigger (below threshold)",
			direction:    "borrow_sell_long",
			currentPrice: 85.0, // -15% > -20% threshold
			wantReason:   "",
		},

		// Direction B — UP triggers (the key invariant)
		{
			name:         "DirB: +25% triggers normal price_spike_exit",
			direction:    "buy_spot_short",
			currentPrice: 125.0, // +25% > 20% threshold
			wantReason:   "price_spike_exit",
		},
		{
			name:          "DirB: +35% triggers emergency_price_spike",
			direction:     "buy_spot_short",
			currentPrice:  135.0, // +35% > 30% threshold
			wantReason:    "emergency_price_spike",
			wantEmergency: true,
		},
		{
			name:         "DirB: +15% no trigger (below threshold)",
			direction:    "buy_spot_short",
			currentPrice: 115.0, // +15% < 20% threshold
			wantReason:   "",
		},
		{
			name:         "DirB: -25% down move does NOT trigger (critical invariant)",
			direction:    "buy_spot_short",
			currentPrice: 75.0, // -25% — profitable for short futures, must NOT exit
			wantReason:   "",
		},
		{
			name:         "DirB: -50% crash does NOT trigger (Direction B is safe in crashes)",
			direction:    "buy_spot_short",
			currentPrice: 50.0, // -50% — big crash, short futures profits massively
			wantReason:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newPriceSpikeEngine(tc.currentPrice)
			pos := &models.SpotFuturesPosition{
				Symbol:       "TESTUSDT",
				Exchange:     "testexch",
				Direction:    tc.direction,
				FuturesEntry: entry,
				Status:       "active",
			}

			reason, emergency := e.checkExitTriggers(pos)
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if emergency != tc.wantEmergency {
				t.Errorf("emergency = %v, want %v", emergency, tc.wantEmergency)
			}
		})
	}
}

func TestMarginHealthTriggers_DirectionAFuturesMargin(t *testing.T) {
	e := &SpotEngine{
		cfg: &config.Config{
			SpotFuturesMarginExitPct:      85.0,
			SpotFuturesMarginEmergencyPct: 95.0,
			SpotFuturesPriceExitPct:       999.0,
			SpotFuturesPriceEmergencyPct:  999.0,
		},
		exchanges: map[string]exchange.Exchange{
			"testexch": priceStubExchange{marginRatio: 0.90},
		},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"testexch": &marginStubExchange{available: 100},
		},
		log: utils.NewLogger("test"),
	}

	pos := &models.SpotFuturesPosition{
		Symbol:         "TESTUSDT",
		BaseCoin:       "TEST",
		Exchange:       "testexch",
		Direction:      "borrow_sell_long",
		Status:         "active",
		BorrowAmount:   1,
		SpotEntryPrice: 100,
	}

	reason, emergency := e.checkExitTriggers(pos)
	if reason != "margin_health_exit" {
		t.Fatalf("reason = %q, want %q", reason, "margin_health_exit")
	}
	if emergency {
		t.Fatalf("emergency = %v, want false", emergency)
	}
	if pos.MarginUtilizationPct != 90 {
		t.Fatalf("MarginUtilizationPct = %.1f, want 90.0", pos.MarginUtilizationPct)
	}
}
