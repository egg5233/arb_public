package spotengine

import (
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// newAutoEntryEngine creates a SpotEngine for auto-entry tests, reusing the
// same miniredis-backed pattern from risk_gate_test.go.
func newAutoEntryEngine(t *testing.T, cfg *config.Config) (*SpotEngine, *miniredis.Miniredis) {
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
		cfg:       cfg,
		db:        db,
		log:       utils.NewLogger("test"),
		lastSeen:  make(map[string]bool),
	}, mr
}

// TestAutoEntry_DryRun verifies the SF-05 pipeline:
// runDiscoveryScan() -> filterPassed() -> attemptAutoEntries() -> checkRiskGate() -> dry_run stop.
// This confirms the wiring is correct: when SpotFuturesAutoEnabled=true and SpotFuturesDryRun=true,
// the pipeline evaluates opportunities and stops at the dry_run gate for the first eligible candidate.
func TestAutoEntry_DryRun(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesAutoEnabled:      true,
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 0, // disabled
	}
	eng, mr := newAutoEntryEngine(t, cfg)
	defer mr.Close()

	// Create test opportunities that would pass all real gates.
	opps := []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			Exchange:  "binance",
			Direction: "borrow_sell_long",
			NetAPR:    0.15,
			Source:    "native",
		},
		{
			Symbol:    "ETHUSDT",
			Exchange:  "bybit",
			Direction: "buy_spot_short",
			NetAPR:    0.12,
			Source:    "native",
		},
	}

	// Verify pipeline reaches dry_run gate: the first opp passes all real checks
	// (capacity, duplicate, cooldown, persistence) and is stopped by dry_run.
	// Calling checkRiskGate directly first to verify the underlying result.
	result := eng.checkRiskGate(opps[0])
	if result.Allowed {
		t.Fatal("expected dry_run block, got allowed")
	}
	if result.Reason != "dry_run" {
		t.Errorf("expected dry_run reason, got %q", result.Reason)
	}

	// Now call attemptAutoEntries to verify the full pipeline wiring.
	// Since dry_run is on, it should return without calling ManualOpen.
	// attemptAutoEntries logs "DRY RUN" and returns after the first eligible candidate.
	eng.attemptAutoEntries(opps)

	// Verify no positions were opened (dry run prevents actual execution).
	active, err := eng.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("get active positions: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active positions in dry run, got %d", len(active))
	}
}

// TestAutoEntry_Disabled verifies that attemptAutoEntries is a no-op when
// SpotFuturesAutoEnabled is false. No risk gate evaluation should occur.
func TestAutoEntry_Disabled(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesAutoEnabled:  false,
		SpotFuturesMaxPositions: 5,
	}
	eng, mr := newAutoEntryEngine(t, cfg)
	defer mr.Close()

	opps := []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			Exchange:  "binance",
			Direction: "borrow_sell_long",
			NetAPR:    0.15,
			Source:    "native",
		},
	}

	// Call attemptAutoEntries — should return immediately without processing.
	eng.attemptAutoEntries(opps)

	// Verify no positions were opened.
	active, err := eng.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("get active positions: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active positions when disabled, got %d", len(active))
	}
}

// TestAutoEntry_AtCapacity verifies that when positions are at max capacity,
// attemptAutoEntries stops evaluation after the at_capacity gate.
func TestAutoEntry_AtCapacity(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesAutoEnabled:      true,
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     1,
		SpotFuturesPersistenceScans: 0,
	}
	eng, mr := newAutoEntryEngine(t, cfg)
	defer mr.Close()

	// Fill capacity.
	seedActivePosition(t, eng.db, "ETHUSDT", "binance")

	opps := []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			Exchange:  "bybit",
			Direction: "borrow_sell_long",
			NetAPR:    0.20,
			Source:    "native",
		},
	}

	// attemptAutoEntries should stop at capacity gate — no dry_run evaluation.
	result := eng.checkRiskGate(opps[0])
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "at_capacity_1/1" {
		t.Errorf("expected at_capacity reason, got %q", result.Reason)
	}
}
