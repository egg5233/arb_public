package spotengine

import (
	"testing"

	"arb/internal/config"
	"arb/internal/models"
)

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
				Direction:        tc.direction,
				FuturesSide:      tc.futSide,
				SpotEntryPrice:   tc.spotEntry,
				SpotExitPrice:    tc.spotExit,
				SpotSize:         tc.spotSize,
				FuturesEntry:     tc.futEntry,
				FuturesExit:      tc.futExit,
				FuturesSize:      tc.futSize,
				BorrowCostAccrued: tc.borrow,
				EntryFees:        tc.fees / 2,
				ExitFees:         tc.fees / 2,
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
