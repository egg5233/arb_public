package notify

import (
	"testing"
	"time"
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
	n.NotifyAutoEntry(nil, 0.1)  // should not panic
	n.NotifyAutoExit(nil, "", 0, 0)
	n.NotifyEmergencyClose(nil, "", 0)
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
