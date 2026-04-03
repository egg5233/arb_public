package risk

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"

	"github.com/alicebob/miniredis/v2"
)

func setupLossLimitTest(t *testing.T) (*LossLimitChecker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EnableLossLimits:    true,
		DailyLossLimitUSDT:  100.0,
		WeeklyLossLimitUSDT: 300.0,
	}
	checker := NewLossLimitChecker(db, cfg, nil, nil)
	return checker, mr
}

// TestLossLimit24h verifies that the 24h rolling window correctly sums losses
// and transitions from allowed to blocked when the daily threshold is exceeded.
func TestLossLimit24h(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	now := time.Now().UTC()

	// Record 3 loss events in last 24h totaling -$80
	checker.RecordClosedPnL("pos-1", -30.0, "BTCUSDT", now.Add(-6*time.Hour))
	checker.RecordClosedPnL("pos-2", -25.0, "ETHUSDT", now.Add(-3*time.Hour))
	checker.RecordClosedPnL("pos-3", -25.0, "SOLUSDT", now.Add(-1*time.Hour))

	// CheckLimits with $100 daily limit should NOT be blocked (net = -80)
	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked, but got blocked: %+v", status)
	}
	if status.DailyLoss > -70 || status.DailyLoss < -90 {
		t.Fatalf("expected daily loss around -80, got %.2f", status.DailyLoss)
	}

	// Record another -$30 event (total = -110, exceeds $100 daily limit)
	checker.RecordClosedPnL("pos-4", -30.0, "BTCUSDT", now.Add(-30*time.Minute))

	blocked, status = checker.CheckLimits()
	if !blocked {
		t.Fatalf("expected blocked after exceeding daily limit, got not blocked: %+v", status)
	}
	if status.BreachType != "daily" {
		t.Fatalf("expected breach_type=daily, got %q", status.BreachType)
	}
}

// TestLossLimit7d verifies that the 7d rolling window correctly sums losses
// across multiple days and transitions to blocked on weekly threshold breach.
func TestLossLimit7d(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	now := time.Now().UTC()

	// Record loss events across 5 days totaling -$250
	// Set daily limit high so we don't trigger daily breach
	checker.cfg.DailyLossLimitUSDT = 1000.0

	checker.RecordClosedPnL("pos-1", -50.0, "BTCUSDT", now.Add(-5*24*time.Hour))
	checker.RecordClosedPnL("pos-2", -50.0, "ETHUSDT", now.Add(-4*24*time.Hour))
	checker.RecordClosedPnL("pos-3", -50.0, "SOLUSDT", now.Add(-3*24*time.Hour))
	checker.RecordClosedPnL("pos-4", -50.0, "XRPUSDT", now.Add(-2*24*time.Hour))
	checker.RecordClosedPnL("pos-5", -50.0, "ADAUSDT", now.Add(-1*24*time.Hour))

	// CheckLimits with $300 weekly limit should NOT be blocked (net = -250)
	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked, but got blocked: %+v", status)
	}
	if status.WeeklyLoss > -240 || status.WeeklyLoss < -260 {
		t.Fatalf("expected weekly loss around -250, got %.2f", status.WeeklyLoss)
	}

	// Add -$60 more (total = -310, exceeds $300 weekly limit)
	checker.RecordClosedPnL("pos-6", -60.0, "DOGEUSDT", now.Add(-12*time.Hour))

	blocked, status = checker.CheckLimits()
	if !blocked {
		t.Fatalf("expected blocked after exceeding weekly limit, got not blocked: %+v", status)
	}
	if status.BreachType != "weekly" {
		t.Fatalf("expected breach_type=weekly, got %q", status.BreachType)
	}
}

// TestLossLimitBlocks verifies that entries are halted when losses exceed the daily limit.
func TestLossLimitBlocks(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	now := time.Now().UTC()

	// Record losses exceeding daily limit
	checker.RecordClosedPnL("pos-1", -60.0, "BTCUSDT", now.Add(-2*time.Hour))
	checker.RecordClosedPnL("pos-2", -60.0, "ETHUSDT", now.Add(-1*time.Hour))

	// Total = -120, daily limit = 100 -> should be blocked
	blocked, status := checker.CheckLimits()
	if !blocked {
		t.Fatalf("expected blocked with -120 loss against 100 limit, got not blocked: %+v", status)
	}
	if !status.Breached {
		t.Fatal("expected Breached=true")
	}
	if status.BreachType != "daily" {
		t.Fatalf("expected breach_type=daily, got %q", status.BreachType)
	}
}

// TestLossLimitAllows verifies that net PnL considers both wins and losses.
// A mix of wins and losses netting to -$40 with $100 daily limit should allow entries.
func TestLossLimitAllows(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	now := time.Now().UTC()

	// Record mix of wins and losses
	checker.RecordClosedPnL("pos-1", -80.0, "BTCUSDT", now.Add(-5*time.Hour))
	checker.RecordClosedPnL("pos-2", 40.0, "ETHUSDT", now.Add(-3*time.Hour)) // win!
	// Net = -40, daily limit = 100 -> should NOT be blocked

	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked with net -40 against 100 limit, but got blocked: %+v", status)
	}
	if status.DailyLoss > -30 || status.DailyLoss < -50 {
		t.Fatalf("expected daily loss around -40, got %.2f", status.DailyLoss)
	}
}

// TestLossLimitPrune verifies that events older than 8 days are pruned on write.
func TestLossLimitPrune(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	now := time.Now().UTC()

	// Record event 9 days ago
	checker.RecordClosedPnL("pos-old", -100.0, "BTCUSDT", now.Add(-9*24*time.Hour))

	// Record a recent event (this triggers prune of events older than 8 days from "now")
	checker.RecordClosedPnL("pos-new", -10.0, "ETHUSDT", now)

	// The old event should have been pruned. Only the recent -10 should remain.
	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked after prune, but got blocked: %+v", status)
	}
	// Weekly loss should only include the recent event
	if status.WeeklyLoss < -15 || status.WeeklyLoss > -5 {
		t.Fatalf("expected weekly loss around -10 after prune, got %.2f", status.WeeklyLoss)
	}
}

// TestLossLimitDisabled verifies that when EnableLossLimits is false,
// CheckLimits always returns not blocked.
func TestLossLimitDisabled(t *testing.T) {
	checker, _ := setupLossLimitTest(t)
	checker.cfg.EnableLossLimits = false
	now := time.Now().UTC()

	// Record losses that would exceed limits
	checker.RecordClosedPnL("pos-1", -200.0, "BTCUSDT", now.Add(-1*time.Hour))

	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked when disabled, got blocked: %+v", status)
	}
	if status.Enabled {
		t.Fatal("expected Enabled=false when disabled")
	}
}

// TestLossLimitNilSafe verifies that nil LossLimitChecker methods don't panic.
func TestLossLimitNilSafe(t *testing.T) {
	var checker *LossLimitChecker
	// Should not panic
	checker.RecordClosedPnL("pos-1", -100.0, "BTCUSDT", time.Now())

	blocked, status := checker.CheckLimits()
	if blocked {
		t.Fatalf("expected not blocked from nil checker, got blocked: %+v", status)
	}
}
