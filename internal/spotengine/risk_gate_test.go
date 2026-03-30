package spotengine

import (
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
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
