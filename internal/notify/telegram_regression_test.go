package notify

import (
	"errors"
	"strings"
	"testing"
	"time"

	"arb/internal/models"
)

// This file pins the byte-for-byte output format of every pre-Phase-9 Notify*
// method. It is the guardrail against accidental refactors that alter an
// existing perp-perp or spot-futures alert body while new PriceGap notifiers
// are being added (Phase 9, Plan 04, T-09-20).
//
// If a legitimate message change is required, update both the golden string
// below AND the corresponding user-facing copywriting decision in the plan
// before merging.

func TestRegression_NotifyAutoEntry(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.SpotFuturesPosition{
		Symbol:       "BTCUSDT",
		Direction:    "borrow_sell_long",
		Exchange:     "binance",
		NotionalUSDT: 500.0,
	}
	tg.NotifyAutoEntry(pos, 0.123)
	sink.waitForMessages(t, 1)
	want := "*Auto-Entry*\nSymbol: `BTCUSDT`\nDirection: Dir A (borrow+sell+long)\nExchange: binance\nSize: 500.00 USDT\nExpected Yield: 12.3% APR"
	if got := sink.last(); got != want {
		t.Errorf("NotifyAutoEntry drift:\n got: %q\nwant: %q", got, want)
	}

	// Dir B variant.
	tg2, sink2 := newStubTelegram(t)
	pos2 := &models.SpotFuturesPosition{
		Symbol:       "ETHUSDT",
		Direction:    "buy_spot_short",
		Exchange:     "bybit",
		NotionalUSDT: 250.0,
	}
	tg2.NotifyAutoEntry(pos2, 0.05)
	sink2.waitForMessages(t, 1)
	if !strings.Contains(sink2.last(), "Dir B (buy+short)") {
		t.Errorf("NotifyAutoEntry Dir B label drift: %q", sink2.last())
	}
}

func TestRegression_NotifyAutoExit(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.SpotFuturesPosition{Symbol: "BTCUSDT", Exchange: "binance"}
	tg.NotifyAutoExit(pos, "borrow_cost_exceeded", 12.3456, 90*time.Minute)
	sink.waitForMessages(t, 1)
	want := "*Auto-Exit*\nSymbol: `BTCUSDT`\nReason: Borrow cost exceeded\nPnL: +12.3456 USDT\nDuration: 1h30m\nExchange: binance"
	if got := sink.last(); got != want {
		t.Errorf("NotifyAutoExit drift:\n got: %q\nwant: %q", got, want)
	}

	// Negative PnL path (no leading '+').
	tg2, sink2 := newStubTelegram(t)
	tg2.NotifyAutoExit(pos, "manual_close", -5.0, time.Hour)
	sink2.waitForMessages(t, 1)
	if !strings.Contains(sink2.last(), "PnL: -5.0000 USDT") {
		t.Errorf("NotifyAutoExit negative PnL drift: %q", sink2.last())
	}
}

func TestRegression_NotifyEmergencyClose(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.SpotFuturesPosition{Symbol: "BTCUSDT", Exchange: "binance"}
	tg.NotifyEmergencyClose(pos, "emergency_price_spike", -25.5)
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *EMERGENCY CLOSE*\nSymbol: `BTCUSDT`\nTrigger: Emergency price spike\nPnL: -25.5000 USDT\nExchange: binance"
	if got := sink.last(); got != want {
		t.Errorf("NotifyEmergencyClose drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegression_NotifySLTriggered(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.ArbitragePosition{Symbol: "BTCUSDT"}
	tg.NotifySLTriggered(pos, "long", "binance")
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *SL Triggered*\nSymbol: `BTCUSDT`\nExchange: binance\nLeg: long\nEmergency close initiated"
	if got := sink.last(); got != want {
		t.Errorf("NotifySLTriggered drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegression_NotifyEmergencyClosePerp(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyEmergencyClosePerp("binance", "L5", 3)
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *L5 Emergency Action*\nExchange: binance\nPositions affected: 3\nMargin health critical — reducing exposure"
	if got := sink.last(); got != want {
		t.Errorf("NotifyEmergencyClosePerp drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegression_NotifyConsecutiveAPIErrors(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyConsecutiveAPIErrors("binance", 5, errors.New("timeout"))
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *API Error Alert*\nExchange: binance\nConsecutive failures: 5\nLast error: timeout"
	if got := sink.last(); got != want {
		t.Errorf("NotifyConsecutiveAPIErrors drift:\n got: %q\nwant: %q", got, want)
	}

	// nil error path.
	tg2, sink2 := newStubTelegram(t)
	tg2.NotifyConsecutiveAPIErrors("bybit", 3, nil)
	sink2.waitForMessages(t, 1)
	if !strings.Contains(sink2.last(), "Last error: <nil>") {
		t.Errorf("NotifyConsecutiveAPIErrors nil-error drift: %q", sink2.last())
	}
}

func TestRegression_NotifyLossLimitBreached(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyLossLimitBreached("daily", 150.0, 100.0, 300.0, 500.0)
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *Loss Limit Breached (daily)*\nDaily: 150.00 / 100.00 USDT\nWeekly: 300.00 / 500.00 USDT\nNew entries halted. Existing positions continue."
	if got := sink.last(); got != want {
		t.Errorf("NotifyLossLimitBreached drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegression_NotifySpotHedgeBroken(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.SpotFuturesPosition{
		ID:          "pos-1",
		Symbol:      "BTCUSDT",
		Exchange:    "binance",
		FuturesSide: "short",
		FuturesSize: 0.5,
	}
	tg.NotifySpotHedgeBroken(pos, "short", 0.3)
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *SF HEDGE BROKEN*\nPosition: `pos-1`\nSymbol: `BTCUSDT`\nExchange: binance\nRecorded: short 0.500000\nExchange: short 0.300000\nManual intervention required"
	if got := sink.last(); got != want {
		t.Errorf("NotifySpotHedgeBroken drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegression_NotifySpotCloseBlocked(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.SpotFuturesPosition{
		ID:       "pos-1",
		Symbol:   "BTCUSDT",
		Exchange: "binance",
	}
	tg.NotifySpotCloseBlocked(pos, "repay_blackout")
	sink.waitForMessages(t, 1)
	want := "\xE2\x9A\xA0 *SF CLOSE BLOCKED*\nPosition: `pos-1`\nSymbol: `BTCUSDT`\nExchange: binance\nReason: repay blackout\nHedge marked broken; manual intervention required"
	if got := sink.last(); got != want {
		t.Errorf("NotifySpotCloseBlocked drift:\n got: %q\nwant: %q", got, want)
	}
}

// TestRegression_AllNilSafe pins the nil-receiver invariant for every existing
// Notify* method. New methods are covered in telegram_pricegap_test.go.
func TestRegression_AllNilSafe(t *testing.T) {
	var tg *TelegramNotifier
	tg.Send("fmt %d", 1)
	tg.NotifyAutoEntry(nil, 0)
	tg.NotifyAutoExit(nil, "", 0, 0)
	tg.NotifyEmergencyClose(nil, "", 0)
	tg.NotifySpotHedgeBroken(nil, "", 0)
	tg.NotifySpotCloseBlocked(nil, "")
	tg.NotifySLTriggered(nil, "", "")
	tg.NotifyEmergencyClosePerp("", "", 0)
	tg.NotifyConsecutiveAPIErrors("", 0, nil)
	tg.NotifyLossLimitBreached("", 0, 0, 0, 0)
}
