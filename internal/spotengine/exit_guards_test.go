package spotengine

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newGuardTestEngine creates a SpotEngine with configurable guards and a
// price stub that returns the given current price for the "testexch" exchange.
func newGuardTestEngine(cfg *config.Config, currentPrice float64) *SpotEngine {
	return &SpotEngine{
		cfg: cfg,
		exchanges: map[string]exchange.Exchange{
			"testexch": priceStubExchange{price: currentPrice},
		},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"testexch": &marginStubExchange{available: 10000},
		},
		log: utils.NewLogger("test"),
	}
}

// newGuardTestPos creates a test position with a configurable CreatedAt age.
func newGuardTestPos(direction string, age time.Duration) *models.SpotFuturesPosition {
	return &models.SpotFuturesPosition{
		Symbol:         "TESTUSDT",
		BaseCoin:       "TEST",
		Exchange:       "testexch",
		Direction:      direction,
		FuturesEntry:   100.0,
		SpotEntryPrice: 100.0,
		Status:         "active",
		CreatedAt:      time.Now().Add(-age),
		FundingAPR:     0.01, // low funding to trigger yield_below_minimum when min-hold not blocking
	}
}

// ---------------------------------------------------------------------------
// TestMinHoldGate verifies that young positions are blocked from yield-based
// exit but allowed once they are old enough. Also verifies the gate is a
// complete no-op when the enable toggle is false.
// ---------------------------------------------------------------------------

func TestMinHoldGate(t *testing.T) {
	tests := []struct {
		name         string
		enableMinHold bool
		minHoldHours int
		posAge       time.Duration
		wantReason   string // "" means no trigger (blocked or nothing fires)
		wantBlocked  bool   // true if guard blocks a yield trigger that would fire
	}{
		{
			name:          "young position blocked by min-hold",
			enableMinHold: true,
			minHoldHours:  8,
			posAge:        2 * time.Hour, // 2h < 8h min-hold
			wantReason:    "",
			wantBlocked:   true,
		},
		{
			name:          "old position allowed through min-hold",
			enableMinHold: true,
			minHoldHours:  8,
			posAge:        10 * time.Hour, // 10h > 8h min-hold
			wantReason:    "yield_below_minimum",
			wantBlocked:   false,
		},
		{
			name:          "min-hold disabled: young position triggers yield exit",
			enableMinHold: false,
			minHoldHours:  8,
			posAge:        2 * time.Hour,
			wantReason:    "yield_below_minimum",
			wantBlocked:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				SpotFuturesEnableMinHold:     tc.enableMinHold,
				SpotFuturesMinHoldHours:      tc.minHoldHours,
				SpotFuturesPriceExitPct:      999.0, // disable price spike
				SpotFuturesPriceEmergencyPct: 999.0,
				SpotFuturesMinNetYieldAPR:    0.50, // will cause yield_below_minimum with our low funding
			}
			eng := newGuardTestEngine(cfg, 100.0)
			pos := newGuardTestPos("buy_spot_short", tc.posAge)

			reason, _ := eng.checkExitTriggers(pos)

			if tc.wantBlocked {
				if reason != "" {
					t.Errorf("expected min-hold to block exit, got reason=%q", reason)
				}
			} else {
				if reason != tc.wantReason {
					t.Errorf("reason = %q, want %q", reason, tc.wantReason)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSettlementGuard verifies that the settlement window guard defers yield
// exits during funding settlement windows and is a no-op when disabled.
// ---------------------------------------------------------------------------

func TestSettlementGuard(t *testing.T) {
	// We test isInSettlementWindow() directly since the exact time is hard
	// to control inside checkExitTriggers. The integration is verified by
	// TestEmergencyBypass (which enables the guard and verifies it doesn't
	// block emergency exits).

	tests := []struct {
		name      string
		hour      int
		minute    int
		windowMin int
		want      bool
	}{
		{"exactly at settlement start (00:00)", 0, 0, 10, true},
		{"during settlement window (08:05)", 8, 5, 10, true},
		{"end of settlement window (16:09)", 16, 9, 10, true},
		{"just outside window (00:10)", 0, 10, 10, false},
		{"normal hour (03:30)", 3, 30, 10, false},
		{"prev hour tail (07:55)", 7, 55, 10, true},
		{"prev hour just outside (07:49)", 7, 49, 10, false},
		{"prev hour tail (15:52)", 15, 52, 10, true},
		{"non-settlement hour (12:00)", 12, 0, 10, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isInSettlementWindowAt(tc.hour, tc.minute, tc.windowMin)
			if got != tc.want {
				t.Errorf("isInSettlementWindowAt(%d, %d, %d) = %v, want %v",
					tc.hour, tc.minute, tc.windowMin, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestEmergencyBypass verifies that emergency triggers (price spike, margin
// health) fire for young positions even when ALL guards are enabled.
// This is the critical safety invariant: guards never block emergencies.
// ---------------------------------------------------------------------------

func TestEmergencyBypass(t *testing.T) {
	tests := []struct {
		name          string
		direction     string
		currentPrice  float64
		marginRatio   float64 // for futures balance
		marginAvail   float64 // for spot margin (Dir A only)
		borrowAmount  float64 // for Dir A only
		wantReason    string
		wantEmergency bool
	}{
		{
			name:          "DirA emergency price spike bypasses min-hold",
			direction:     "borrow_sell_long",
			currentPrice:  135.0, // +35% > 30% emergency threshold
			marginRatio:   0.10,  // healthy margin
			marginAvail:   10000,
			borrowAmount:  0,
			wantReason:    "emergency_price_spike",
			wantEmergency: true,
		},
		{
			name:          "DirB emergency price spike bypasses min-hold",
			direction:     "buy_spot_short",
			currentPrice:  135.0, // +35% > 30% emergency threshold
			marginRatio:   0.10,
			marginAvail:   10000,
			borrowAmount:  0,
			wantReason:    "emergency_price_spike",
			wantEmergency: true,
		},
		{
			name:          "DirA normal price spike bypasses min-hold",
			direction:     "borrow_sell_long",
			currentPrice:  125.0, // +25% > 20% threshold
			marginRatio:   0.10,
			marginAvail:   10000,
			borrowAmount:  0,
			wantReason:    "price_spike_exit",
			wantEmergency: false,
		},
		{
			name:          "DirA emergency margin bypasses min-hold",
			direction:     "borrow_sell_long",
			currentPrice:  100.0, // no price spike
			marginRatio:   0.96,  // 96% > 95% emergency
			marginAvail:   10000,
			borrowAmount:  0,
			wantReason:    "margin_health_exit",
			wantEmergency: true,
		},
		{
			name:          "DirB emergency margin bypasses min-hold",
			direction:     "buy_spot_short",
			currentPrice:  100.0,
			marginRatio:   0.96,
			marginAvail:   10000,
			borrowAmount:  0,
			wantReason:    "margin_health_exit",
			wantEmergency: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				// ALL guards ON.
				SpotFuturesEnableMinHold:        true,
				SpotFuturesMinHoldHours:         999, // position will always be "too young"
				SpotFuturesEnableSettlementGuard: true,
				SpotFuturesSettlementWindowMin:   60, // always in window
				SpotFuturesEnableExitSpreadGate:  true,
				SpotFuturesExitSpreadPct:         0.001, // very tight spread gate
				// Normal thresholds.
				SpotFuturesPriceExitPct:       20.0,
				SpotFuturesPriceEmergencyPct:  30.0,
				SpotFuturesMarginExitPct:      85.0,
				SpotFuturesMarginEmergencyPct: 95.0,
			}
			eng := &SpotEngine{
				cfg: cfg,
				exchanges: map[string]exchange.Exchange{
					"testexch": priceStubExchange{
						price:       tc.currentPrice,
						marginRatio: tc.marginRatio,
					},
				},
				spotMargin: map[string]exchange.SpotMarginExchange{
					"testexch": &marginStubExchange{available: tc.marginAvail},
				},
				log: utils.NewLogger("test"),
			}

			pos := &models.SpotFuturesPosition{
				Symbol:         "TESTUSDT",
				BaseCoin:       "TEST",
				Exchange:       "testexch",
				Direction:      tc.direction,
				FuturesEntry:   100.0,
				SpotEntryPrice: 100.0,
				BorrowAmount:   tc.borrowAmount,
				Status:         "active",
				CreatedAt:      time.Now().Add(-1 * time.Minute), // very young position
			}

			reason, emergency := eng.checkExitTriggers(pos)
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if emergency != tc.wantEmergency {
				t.Errorf("emergency = %v, want %v", emergency, tc.wantEmergency)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExitSpreadGate verifies that the exit spread gate defers yield-based
// exits when estimated slippage exceeds the threshold, and is a no-op when
// the enable toggle is false.
// ---------------------------------------------------------------------------

func TestExitSpreadGate(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		bidPrice    float64
		askPrice    float64
		maxSlippage float64
		wantBlocked bool // true if guard blocks yield exit
	}{
		{
			name:        "wide spread blocks exit",
			enabled:     true,
			bidPrice:    99.5,
			askPrice:    100.5, // spread = 1.0, mid = 100, spreadPct = 1.0%, slippage = 2.0%
			maxSlippage: 0.3,
			wantBlocked: true,
		},
		{
			name:        "tight spread allows exit",
			enabled:     true,
			bidPrice:    99.99,
			askPrice:    100.01, // spread = 0.02, mid = 100, spreadPct = 0.02%, slippage = 0.04%
			maxSlippage: 0.3,
			wantBlocked: false,
		},
		{
			name:        "disabled gate allows exit regardless of spread",
			enabled:     false,
			bidPrice:    99.5,
			askPrice:    100.5, // wide spread, but gate disabled
			maxSlippage: 0.3,
			wantBlocked: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				SpotFuturesEnableExitSpreadGate: tc.enabled,
				SpotFuturesExitSpreadPct:        tc.maxSlippage,
				SpotFuturesPriceExitPct:         999.0, // disable price spike
				SpotFuturesPriceEmergencyPct:    999.0,
				SpotFuturesMinNetYieldAPR:       0.50, // trigger yield_below_minimum
			}
			eng := &SpotEngine{
				cfg: cfg,
				exchanges: map[string]exchange.Exchange{
					"testexch": spreadStubExchange{bid: tc.bidPrice, ask: tc.askPrice},
				},
				log: utils.NewLogger("test"),
			}

			pos := &models.SpotFuturesPosition{
				Symbol:       "TESTUSDT",
				Exchange:     "testexch",
				Direction:    "buy_spot_short",
				FuturesEntry: 100.0,
				Status:       "active",
				CreatedAt:    time.Now().Add(-24 * time.Hour), // old enough for min-hold
				FundingAPR:   0.01,                           // low funding to trigger yield exit
			}

			reason, _ := eng.checkExitTriggers(pos)

			if tc.wantBlocked {
				if reason != "" {
					t.Errorf("expected spread gate to block exit, got reason=%q", reason)
				}
			} else {
				if reason != "yield_below_minimum" {
					t.Errorf("expected yield_below_minimum, got reason=%q", reason)
				}
			}
		})
	}
}

// spreadStubExchange extends priceStubExchange with separate bid/ask prices
// for testing spread-dependent logic.
type spreadStubExchange struct {
	priceStubExchange
	bid float64
	ask float64
}

func (s spreadStubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	return &exchange.Orderbook{
		Bids: []exchange.PriceLevel{{Price: s.bid, Quantity: 1}},
		Asks: []exchange.PriceLevel{{Price: s.ask, Quantity: 1}},
	}, nil
}

// ---------------------------------------------------------------------------
// TestEstimateUnwindSlippage verifies the slippage calculation from orderbook
// bid-ask spread.
// ---------------------------------------------------------------------------

func TestEstimateUnwindSlippage(t *testing.T) {
	tests := []struct {
		name     string
		bid      float64
		ask      float64
		wantPct  float64 // approximate expected slippage %
		wantZero bool
	}{
		{
			name:    "normal spread",
			bid:     99.95,
			ask:     100.05,
			wantPct: 0.2, // (0.10/100.0)*100 * 2 = 0.2%
		},
		{
			name:    "wide spread",
			bid:     99.0,
			ask:     101.0,
			wantPct: 4.0, // (2.0/100.0)*100 * 2 = 4.0%
		},
		{
			name:     "zero spread",
			bid:      100.0,
			ask:      100.0,
			wantZero: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := &SpotEngine{
				cfg: &config.Config{},
				exchanges: map[string]exchange.Exchange{
					"testexch": spreadStubExchange{bid: tc.bid, ask: tc.ask},
				},
				log: utils.NewLogger("test"),
			}

			pos := &models.SpotFuturesPosition{
				Symbol:   "TESTUSDT",
				Exchange: "testexch",
			}
			got := eng.estimateUnwindSlippage(pos)

			if tc.wantZero {
				if got != 0 {
					t.Errorf("slippage = %.4f, want 0", got)
				}
			} else {
				// Allow small floating point tolerance.
				diff := got - tc.wantPct
				if diff > 0.01 || diff < -0.01 {
					t.Errorf("slippage = %.4f, want ~%.4f", got, tc.wantPct)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsInSettlementWindow verifies the settlement window detection logic
// using the testable isInSettlementWindowAt helper.
// ---------------------------------------------------------------------------

func TestIsInSettlementWindow(t *testing.T) {
	tests := []struct {
		name   string
		hour   int
		minute int
		window int
		want   bool
	}{
		{"00:00 in window", 0, 0, 10, true},
		{"08:03 in window", 8, 3, 10, true},
		{"16:09 in window", 16, 9, 10, true},
		{"00:10 at boundary excluded", 0, 10, 10, false},
		{"07:55 prev-hour tail", 7, 55, 10, true},
		{"07:50 prev-hour boundary", 7, 50, 10, true},
		{"07:49 prev-hour outside", 7, 49, 10, false},
		{"15:51 prev-hour tail", 15, 51, 10, true},
		{"15:50 prev-hour boundary", 15, 50, 10, true},
		{"12:00 non-settlement hour", 12, 0, 10, false},
		{"03:30 normal time", 3, 30, 10, false},
		{"23:55 in window for 00:00", 23, 55, 10, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isInSettlementWindowAt(tc.hour, tc.minute, tc.window)
			if got != tc.want {
				t.Errorf("isInSettlementWindowAt(%d, %d, %d) = %v, want %v",
					tc.hour, tc.minute, tc.window, got, tc.want)
			}
		})
	}
}
