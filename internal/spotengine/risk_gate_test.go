package spotengine

import (
	"strings"
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// newRiskGateEngine creates a SpotEngine backed by an in-memory Redis for
// risk-gate unit tests. The caller is responsible for closing the miniredis
// server after the test.
func newRiskGateEngine(t *testing.T, cfg *config.Config) (*SpotEngine, *miniredis.Miniredis) {
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
	return &SpotEngine{
		cfg: cfg,
		db:  db,
		log: utils.NewLogger("test"),
	}, mr
}

// seedActivePosition inserts an active spot position into the test database.
func seedActivePosition(t *testing.T, db *database.Client, symbol, exchange string) {
	t.Helper()
	pos := &models.SpotFuturesPosition{
		ID:       "test-" + symbol + "-" + exchange,
		Symbol:   symbol,
		Exchange: exchange,
		Status:   models.SpotStatusActive,
	}
	if err := db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("seed position: %v", err)
	}
}

// TestCheckRiskGate_DryRunAtCapacity verifies that when both dry-run and
// at-capacity are true, the gate reports at_capacity (not dry_run).
// Regression test for ARB-74.
func TestCheckRiskGate_DryRunAtCapacity(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesDryRun:          true,
		SpotFuturesMaxPositions:    1,
		SpotFuturesPersistenceScans: 0,
	}
	eng, mr := newRiskGateEngine(t, cfg)
	defer mr.Close()

	// Fill capacity with one existing position.
	seedActivePosition(t, eng.db, "ETHUSDT", "binance")

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "at_capacity_1/1" {
		t.Errorf("expected at_capacity reason, got %q", result.Reason)
	}
}

// TestCheckRiskGate_DryRunDuplicate verifies that when both dry-run and
// duplicate are true, the gate reports duplicate (not dry_run).
// Regression test for ARB-74.
func TestCheckRiskGate_DryRunDuplicate(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesDryRun:          true,
		SpotFuturesMaxPositions:    5,
		SpotFuturesPersistenceScans: 0,
	}
	eng, mr := newRiskGateEngine(t, cfg)
	defer mr.Close()

	// Existing position for same symbol on different exchange.
	seedActivePosition(t, eng.db, "BTCUSDT", "binance")

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "duplicate_BTCUSDT_on_binance" {
		t.Errorf("expected duplicate reason, got %q", result.Reason)
	}
}

// TestCheckRiskGate_DryRunCooldown verifies that when both dry-run and
// cooldown are true, the gate reports cooldown (not dry_run).
// Regression test for ARB-74.
func TestCheckRiskGate_DryRunCooldown(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 0,
		SpotFuturesLossCooldownHours: 24,
	}
	eng, mr := newRiskGateEngine(t, cfg)
	defer mr.Close()

	// Set cooldown for the symbol.
	if err := eng.db.SetSpotCooldown("BTCUSDT", cfg.SpotFuturesLossCooldownHours); err != nil {
		t.Fatalf("set cooldown: %v", err)
	}

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "cooldown_BTCUSDT" {
		t.Errorf("expected cooldown reason, got %q", result.Reason)
	}
}

// TestCheckRiskGate_DryRunPersistence verifies that when both dry-run and
// insufficient persistence are true, the gate reports persistence (not dry_run).
// Regression test for ARB-74.
func TestCheckRiskGate_DryRunPersistence(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 3,
	}
	eng, mr := newRiskGateEngine(t, cfg)
	defer mr.Close()

	// Persistence count = 1, required = 3 → insufficient.
	if _, err := eng.db.IncrSpotPersistence("BTCUSDT"); err != nil {
		t.Fatalf("incr persistence: %v", err)
	}

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "persistence_1/3" {
		t.Errorf("expected persistence reason, got %q", result.Reason)
	}
}

// TestCheckRiskGate_DryRunFullyEligible verifies that when all real gates
// pass and dry-run is enabled, the gate returns dry_run as the reason.
// This confirms dry-run only triggers AFTER all other checks pass.
// Regression test for ARB-74.
func TestCheckRiskGate_DryRunFullyEligible(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 0, // disabled
	}
	eng, mr := newRiskGateEngine(t, cfg)
	defer mr.Close()

	// No active positions, no cooldown, no persistence requirement → all clear.
	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit", NetAPR: 0.5})
	if result.Allowed {
		t.Fatal("expected dry_run block, got allowed")
	}
	if result.Reason != "dry_run" {
		t.Errorf("expected dry_run reason, got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Maintenance Rate Gate Tests (Plan 06-02)
// ---------------------------------------------------------------------------

// newMaintenanceGateEngine creates a SpotEngine with mock exchange supporting
// maintenanceRateProvider for maintenance gate testing. Backed by miniredis.
func newMaintenanceGateEngine(t *testing.T, cfg *config.Config, maintRate float64) (*SpotEngine, *miniredis.Miniredis) {
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

	provider := &mockMaintenanceProvider{rate: maintRate}
	mock := &mockExchange{provider: provider}

	return &SpotEngine{
		cfg:       cfg,
		db:        db,
		log:       utils.NewLogger("test"),
		exchanges: map[string]exchange.Exchange{"bybit": mock},
	}, mr
}

// TestMaintenanceRateGate_RejectsHighRate verifies GUAUSDT scenario:
// 30% maintenance at 3x leverage → survivable 3.3% < 30% threshold → REJECTED.
func TestMaintenanceRateGate_RejectsHighRate(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesMaxPositions:         5,
		SpotFuturesPersistenceScans:     0,
		SpotFuturesEnableMaintenanceGate: true,
		SpotFuturesLeverage:             3,
		SpotFuturesCapitalSeparate:      200,
		SpotFuturesCapitalUnified:       500,
		SpotFuturesMaintenanceDefault:   0.05,
		SpotFuturesMaintenanceCacheTTL:  60,
	}
	eng, mr := newMaintenanceGateEngine(t, cfg, 0.30) // 30% maintenance
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "GUAUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected GUAUSDT at 30% maintenance / 3x to be rejected")
	}
	if !strings.Contains(result.Reason, "maintenance_survivable_") {
		t.Errorf("expected maintenance_survivable reason, got %q", result.Reason)
	}
}

// TestMaintenanceRateGate_AllowsLowRate verifies BTCUSDT scenario:
// 0.5% maintenance at 3x leverage → survivable 32.8% >= 30% threshold → ALLOWED.
func TestMaintenanceRateGate_AllowsLowRate(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesMaxPositions:         5,
		SpotFuturesPersistenceScans:     0,
		SpotFuturesDryRun:              false,
		SpotFuturesEnableMaintenanceGate: true,
		SpotFuturesLeverage:             3,
		SpotFuturesCapitalSeparate:      200,
		SpotFuturesCapitalUnified:       500,
		SpotFuturesMaintenanceDefault:   0.05,
		SpotFuturesMaintenanceCacheTTL:  60,
	}
	eng, mr := newMaintenanceGateEngine(t, cfg, 0.005) // 0.5% maintenance
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "BTCUSDT", Exchange: "bybit"})
	if !result.Allowed {
		t.Errorf("expected BTCUSDT at 0.5%% maintenance / 3x to be allowed, got reason %q", result.Reason)
	}
}

// TestMaintenanceRateGate_SkippedWhenDisabled verifies that when
// EnableMaintenanceGate = false, entries pass even with 30% rate.
func TestMaintenanceRateGate_SkippedWhenDisabled(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesMaxPositions:         5,
		SpotFuturesPersistenceScans:     0,
		SpotFuturesDryRun:              false,
		SpotFuturesEnableMaintenanceGate: false, // gate disabled
		SpotFuturesLeverage:             3,
		SpotFuturesCapitalSeparate:      200,
		SpotFuturesCapitalUnified:       500,
		SpotFuturesMaintenanceDefault:   0.05,
		SpotFuturesMaintenanceCacheTTL:  60,
	}
	eng, mr := newMaintenanceGateEngine(t, cfg, 0.30) // 30% maintenance, but gate disabled
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "GUAUSDT", Exchange: "bybit"})
	if !result.Allowed {
		t.Errorf("expected entry allowed when gate disabled, got reason %q", result.Reason)
	}
}

// TestMaintenanceRateGate_LeverageScaling verifies the threshold formula
// threshold = 90% / leverage at different leverage levels.
func TestMaintenanceRateGate_LeverageScaling(t *testing.T) {
	tests := []struct {
		name      string
		leverage  int
		maintRate float64
		wantAllow bool
		desc      string
	}{
		{"2x_allowed", 2, 0.01, true, "2x: survivable=49%, threshold=45% → allowed"},
		{"2x_rejected", 2, 0.10, false, "2x: survivable=40%, threshold=45% → rejected"},
		{"3x_allowed", 3, 0.005, true, "3x: survivable=32.8%, threshold=30% → allowed"},
		{"3x_rejected", 3, 0.30, false, "3x: survivable=3.3%, threshold=30% → rejected"},
		{"4x_allowed", 4, 0.01, true, "4x: survivable=24%, threshold=22.5% → allowed"},
		{"4x_rejected", 4, 0.05, false, "4x: survivable=20%, threshold=22.5% → rejected"},
		{"5x_allowed", 5, 0.01, true, "5x: survivable=19%, threshold=18% → allowed"},
		{"5x_rejected", 5, 0.05, false, "5x: survivable=15%, threshold=18% → rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SpotFuturesMaxPositions:         5,
				SpotFuturesPersistenceScans:     0,
				SpotFuturesDryRun:              false,
				SpotFuturesEnableMaintenanceGate: true,
				SpotFuturesLeverage:             tt.leverage,
				SpotFuturesCapitalSeparate:      200,
				SpotFuturesCapitalUnified:       500,
				SpotFuturesMaintenanceDefault:   0.05,
				SpotFuturesMaintenanceCacheTTL:  60,
			}
			eng, mr := newMaintenanceGateEngine(t, cfg, tt.maintRate)
			defer mr.Close()

			result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "TESTUSDT", Exchange: "bybit"})
			if result.Allowed != tt.wantAllow {
				t.Errorf("%s: allowed=%v, want %v (reason=%q)", tt.desc, result.Allowed, tt.wantAllow, result.Reason)
			}
		})
	}
}

// TestMaintenanceRateGate_ZeroRateUsesDefault verifies that when rate=0
// (unknown), the default 0.05 is used.
// 3x: survivable = 33.3% - 5% = 28.3% < 30% threshold → REJECTED.
func TestMaintenanceRateGate_ZeroRateUsesDefault(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesMaxPositions:         5,
		SpotFuturesPersistenceScans:     0,
		SpotFuturesEnableMaintenanceGate: true,
		SpotFuturesLeverage:             3,
		SpotFuturesCapitalSeparate:      200,
		SpotFuturesCapitalUnified:       500,
		SpotFuturesMaintenanceDefault:   0.05,
		SpotFuturesMaintenanceCacheTTL:  60,
	}
	eng, mr := newMaintenanceGateEngine(t, cfg, 0) // rate=0 → default 0.05
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "UNKNOWNUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected rejection when rate=0 uses default 5% at 3x (28.3% < 30%)")
	}
	if !strings.Contains(result.Reason, "maintenance_survivable_") {
		t.Errorf("expected maintenance_survivable reason, got %q", result.Reason)
	}
}

// TestMaintenanceRateGate_BeforeDryRun verifies that maintenance gate (check 6)
// fires before dry-run (check 7). When both would trigger, maintenance wins.
func TestMaintenanceRateGate_BeforeDryRun(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesMaxPositions:         5,
		SpotFuturesPersistenceScans:     0,
		SpotFuturesDryRun:              true, // dry-run enabled
		SpotFuturesEnableMaintenanceGate: true,
		SpotFuturesLeverage:             3,
		SpotFuturesCapitalSeparate:      200,
		SpotFuturesCapitalUnified:       500,
		SpotFuturesMaintenanceDefault:   0.05,
		SpotFuturesMaintenanceCacheTTL:  60,
	}
	eng, mr := newMaintenanceGateEngine(t, cfg, 0.30) // high rate → rejected
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{Symbol: "GUAUSDT", Exchange: "bybit"})
	if result.Allowed {
		t.Fatal("expected rejection")
	}
	// Maintenance should fire before dry-run
	if !strings.Contains(result.Reason, "maintenance_survivable_") {
		t.Errorf("expected maintenance reason (not dry_run), got %q", result.Reason)
	}
}
