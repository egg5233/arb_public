package spotengine

import (
	"fmt"
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// basisStubExchange returns a fixed bid/ask orderbook for basis calculation tests.
type basisStubExchange struct {
	priceStubExchange
	bid float64
	ask float64
	err error // if non-nil, GetOrderbook returns this error
}

func (s basisStubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &exchange.Orderbook{
		Bids: []exchange.PriceLevel{{Price: s.bid, Quantity: 1}},
		Asks: []exchange.PriceLevel{{Price: s.ask, Quantity: 1}},
	}, nil
}

// newBasisGateEngine creates a SpotEngine with miniredis and configurable exchange stub.
func newBasisGateEngine(t *testing.T, cfg *config.Config, exchStub exchange.Exchange) (*SpotEngine, *miniredis.Miniredis) {
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
	exchanges := map[string]exchange.Exchange{}
	if exchStub != nil {
		exchanges["testexch"] = exchStub
	}
	return &SpotEngine{
		cfg:       cfg,
		db:        db,
		exchanges: exchanges,
		log:       utils.NewLogger("test"),
	}, mr
}

// ---------------------------------------------------------------------------
// TestBasisGateEntry verifies that the basis gate rejects entries when the
// spread-based basis proxy exceeds MaxBasisPct, allows entries when below,
// is skipped entirely when disabled, and is fail-closed on errors.
// ---------------------------------------------------------------------------

func TestBasisGateEntry(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		bid        float64
		ask        float64
		maxBasis   float64
		exchErr    error  // force GetOrderbook error
		wantReason string // expected rejection reason (empty = allowed)
	}{
		{
			name:       "basis exceeds threshold: rejected",
			enabled:    true,
			bid:        99.5,
			ask:        100.5, // spread = 1.0, mid = 100.0, basis = 1.0%
			maxBasis:   0.5,
			wantReason: "basis_1.00%>0.50%",
		},
		{
			name:       "basis at threshold: allowed",
			enabled:    true,
			bid:        99.75,
			ask:        100.25, // spread = 0.50, mid = 100.0, basis = 0.50%
			maxBasis:   0.5,
			wantReason: "", // 0.50% <= 0.5% — allowed
		},
		{
			name:       "basis below threshold: allowed",
			enabled:    true,
			bid:        99.95,
			ask:        100.05, // spread = 0.10, mid = 100.0, basis = 0.10%
			maxBasis:   0.5,
			wantReason: "",
		},
		{
			name:       "gate disabled: wide basis allowed",
			enabled:    false,
			bid:        99.0,
			ask:        101.0, // spread = 2.0, basis = 2.0%
			maxBasis:   0.5,
			wantReason: "",
		},
		{
			name:       "exchange error: fail-closed",
			enabled:    true,
			bid:        100.0,
			ask:        100.0,
			maxBasis:   0.5,
			exchErr:    fmt.Errorf("connection timeout"),
			wantReason: "basis_check_error",
		},
		{
			name:       "default maxBasis when config is 0",
			enabled:    true,
			bid:        99.5,
			ask:        100.5, // basis = 1.0%, default threshold = 0.5%
			maxBasis:   0,     // should fall back to 0.5
			wantReason: "basis_1.00%>0.50%",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				SpotFuturesEnableBasisGate:  tc.enabled,
				SpotFuturesMaxBasisPct:      tc.maxBasis,
				SpotFuturesMaxPositions:     5,
				SpotFuturesPersistenceScans: 0,
			}
			stub := basisStubExchange{bid: tc.bid, ask: tc.ask, err: tc.exchErr}
			eng, mr := newBasisGateEngine(t, cfg, stub)
			defer mr.Close()

			opp := SpotArbOpportunity{
				Symbol:    "BTCUSDT",
				Exchange:  "testexch",
				Direction: "buy_spot_short",
				NetAPR:    0.5,
			}

			result := eng.checkRiskGate(opp)

			if tc.wantReason == "" {
				// Should be allowed (unless dry-run is on).
				if !result.Allowed {
					t.Errorf("expected allowed, got blocked: reason=%q", result.Reason)
				}
			} else {
				if result.Allowed {
					t.Error("expected blocked, got allowed")
				}
				if result.Reason != tc.wantReason {
					t.Errorf("reason = %q, want %q", result.Reason, tc.wantReason)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBasisGateBeforeDryRun verifies that the basis gate (check 5) fires
// BEFORE dry-run (check 6). When both are active and basis is too wide,
// the rejection reason must be "basis_*" not "dry_run".
// ---------------------------------------------------------------------------

func TestBasisGateBeforeDryRun(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesEnableBasisGate:  true,
		SpotFuturesMaxBasisPct:      0.5,
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 0,
	}
	stub := basisStubExchange{bid: 99.0, ask: 101.0} // basis = 2.0%
	eng, mr := newBasisGateEngine(t, cfg, stub)
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{
		Symbol:   "BTCUSDT",
		Exchange: "testexch",
		NetAPR:   0.5,
	})

	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "basis_2.00%>0.50%" {
		t.Errorf("expected basis rejection before dry_run, got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// TestBasisGateDryRunPassthrough verifies that when basis check passes but
// dry-run is enabled, the rejection reason is "dry_run" (not basis).
// ---------------------------------------------------------------------------

func TestBasisGateDryRunPassthrough(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesEnableBasisGate:  true,
		SpotFuturesMaxBasisPct:      0.5,
		SpotFuturesDryRun:           true,
		SpotFuturesMaxPositions:     5,
		SpotFuturesPersistenceScans: 0,
	}
	stub := basisStubExchange{bid: 99.99, ask: 100.01} // basis = 0.02%
	eng, mr := newBasisGateEngine(t, cfg, stub)
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{
		Symbol:   "BTCUSDT",
		Exchange: "testexch",
		NetAPR:   0.5,
	})

	if result.Allowed {
		t.Fatal("expected dry_run block, got allowed")
	}
	if result.Reason != "dry_run" {
		t.Errorf("expected dry_run reason (basis passed), got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// TestCalculateEntryBasis verifies the basis calculation helper directly.
// ---------------------------------------------------------------------------

func TestCalculateEntryBasis(t *testing.T) {
	tests := []struct {
		name    string
		bid     float64
		ask     float64
		wantPct float64
		wantErr bool
	}{
		{
			name:    "normal spread",
			bid:     99.95,
			ask:     100.05,
			wantPct: 0.1, // (0.10/100.0)*100 = 0.1%
		},
		{
			name:    "wide spread",
			bid:     99.0,
			ask:     101.0,
			wantPct: 2.0, // (2.0/100.0)*100 = 2.0%
		},
		{
			name:    "zero spread",
			bid:     100.0,
			ask:     100.0,
			wantPct: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := &SpotEngine{
				cfg: &config.Config{},
				exchanges: map[string]exchange.Exchange{
					"testexch": basisStubExchange{bid: tc.bid, ask: tc.ask},
				},
				log: utils.NewLogger("test"),
			}

			got, err := eng.calculateEntryBasis("BTCUSDT", "testexch")
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			diff := got - tc.wantPct
			if diff > 0.01 || diff < -0.01 {
				t.Errorf("basis = %.4f%%, want ~%.4f%%", got, tc.wantPct)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCalculateEntryBasis_ExchangeNotFound verifies fail-closed when exchange
// does not exist.
// ---------------------------------------------------------------------------

func TestCalculateEntryBasis_ExchangeNotFound(t *testing.T) {
	eng := &SpotEngine{
		cfg:       &config.Config{},
		exchanges: map[string]exchange.Exchange{},
		log:       utils.NewLogger("test"),
	}

	_, err := eng.calculateEntryBasis("BTCUSDT", "nonexistent")
	if err == nil {
		t.Error("expected error for missing exchange, got nil")
	}
}
