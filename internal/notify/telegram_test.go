package notify

import (
	"testing"
	"time"

	"arb/internal/models"
)

func TestNewTelegramNilOnEmpty(t *testing.T) {
	if n := NewTelegram("", ""); n != nil {
		t.Error("expected nil for empty bot token")
	}
	if n := NewTelegram("token", ""); n != nil {
		t.Error("expected nil for empty chat ID")
	}
	if n := NewTelegram("", "123"); n != nil {
		t.Error("expected nil for empty bot token with chat ID")
	}
	if n := NewTelegram("token", "123"); n == nil {
		t.Error("expected non-nil for valid token and chat ID")
	}
}

func TestNilSafeNotify(t *testing.T) {
	// All notification methods should be safe to call on nil receiver.
	var n *TelegramNotifier
	n.NotifyAutoEntry(nil, 0.1) // should not panic
	n.NotifyAutoExit(nil, "", 0, 0)
	n.NotifyEmergencyClose(nil, "", 0)

	// New perp-perp methods (nil-safe).
	n.NotifySLTriggered(nil, "", "")
	n.NotifyEmergencyClosePerp("", "", 0)
	n.NotifyConsecutiveAPIErrors("", 0, nil)
	n.NotifyLossLimitBreached("", 0, 0, 0, 0)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{90 * time.Minute, "1h30m"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{5 * time.Minute, "5m"},
		{0, "0m"},
	}
	for _, tc := range tests {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestFormatExitReason(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{"borrow_cost_exceeded", "Borrow cost exceeded"},
		{"borrow_rate_spike", "Borrow rate spike"},
		{"yield_below_minimum", "Yield below minimum"},
		{"price_spike_exit", "Price spike"},
		{"emergency_price_spike", "Emergency price spike"},
		{"margin_health_exit", "Margin health"},
		{"manual_close", "Manual close"},
		{"unknown_reason", "unknown reason"},
	}
	for _, tc := range tests {
		got := formatExitReason(tc.reason)
		if got != tc.want {
			t.Errorf("formatExitReason(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestCooldown(t *testing.T) {
	tg := NewTelegram("tok", "123")
	if tg == nil {
		t.Fatal("NewTelegram returned nil")
	}

	t0 := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)

	// First call should return true (no prior send).
	if !tg.checkCooldownAt("test_event", t0) {
		t.Error("first checkCooldownAt should return true")
	}

	// Within 5 minutes should return false.
	if tg.checkCooldownAt("test_event", t0.Add(2*time.Minute)) {
		t.Error("checkCooldownAt within 5m should return false")
	}

	// After 5 minutes should return true again.
	if !tg.checkCooldownAt("test_event", t0.Add(6*time.Minute)) {
		t.Error("checkCooldownAt after 6m should return true")
	}
}

func TestCooldownIndependent(t *testing.T) {
	tg := NewTelegram("tok", "123")
	if tg == nil {
		t.Fatal("NewTelegram returned nil")
	}

	t0 := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)

	// Both different event types should return true at the same time.
	if !tg.checkCooldownAt("sl_triggered", t0) {
		t.Error("sl_triggered should pass cooldown")
	}
	if !tg.checkCooldownAt("api_errors:binance", t0) {
		t.Error("api_errors:binance should pass cooldown independently")
	}

	// But same event type should be blocked.
	if tg.checkCooldownAt("sl_triggered", t0.Add(1*time.Minute)) {
		t.Error("sl_triggered should be blocked within cooldown window")
	}
}

func TestNotifySLTriggeredFormat(t *testing.T) {
	tg := NewTelegram("tok", "123")
	if tg == nil {
		t.Fatal("NewTelegram returned nil")
	}

	pos := &models.ArbitragePosition{
		ID:            "test-pos-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		LongSize:      0.5,
		ShortSize:     0.5,
	}

	// Should not panic even though send will fail (invalid bot token).
	tg.NotifySLTriggered(pos, "long", "binance")
}

func TestNotifyLossLimitBreachedFormat(t *testing.T) {
	tg := NewTelegram("tok", "123")
	if tg == nil {
		t.Fatal("NewTelegram returned nil")
	}

	// Should not panic.
	tg.NotifyLossLimitBreached("daily", 150.0, 100.0, 300.0, 500.0)
}
