package risk

import (
	"math"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

func TestComputeLevel(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
	}

	h := &HealthMonitor{cfg: cfg}

	tests := []struct {
		name     string
		bal      *exchange.Balance
		pnl      float64
		posCount int
		want     HealthLevel
	}{
		{
			name:     "L0: no positions",
			bal:      &exchange.Balance{Total: 100, Available: 100, MarginRatio: 0},
			pnl:      0,
			posCount: 0,
			want:     L0None,
		},
		{
			name:     "L1: positions with positive PnL",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.10},
			pnl:      5.0,
			posCount: 1,
			want:     L1Safe,
		},
		{
			name:     "L2: negative PnL but low margin ratio",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.20},
			pnl:      -2.0,
			posCount: 1,
			want:     L2Low,
		},
		{
			name:     "L3: negative PnL, margin ratio at L3 threshold",
			bal:      &exchange.Balance{Total: 100, Available: 50, MarginRatio: 0.55},
			pnl:      -5.0,
			posCount: 1,
			want:     L3Medium,
		},
		{
			name:     "L4: negative PnL, margin ratio at L4 threshold",
			bal:      &exchange.Balance{Total: 100, Available: 20, MarginRatio: 0.85},
			pnl:      -10.0,
			posCount: 1,
			want:     L4High,
		},
		{
			name:     "L5: critical margin ratio",
			bal:      &exchange.Balance{Total: 100, Available: 5, MarginRatio: 0.96},
			pnl:      -15.0,
			posCount: 1,
			want:     L5Critical,
		},
		{
			name:     "L5: critical even with positive PnL",
			bal:      &exchange.Balance{Total: 100, Available: 3, MarginRatio: 0.97},
			pnl:      1.0,
			posCount: 1,
			want:     L5Critical,
		},
		{
			name:     "L1: zero PnL counts as safe",
			bal:      &exchange.Balance{Total: 100, Available: 80, MarginRatio: 0.10},
			pnl:      0,
			posCount: 1,
			want:     L1Safe,
		},
		{
			name:     "hybrid fallback: margin ratio from available/total when MarginRatio=0",
			bal:      &exchange.Balance{Total: 100, Available: 40, MarginRatio: 0},
			pnl:      -5.0,
			posCount: 1,
			want:     L3Medium, // 1 - 40/100 = 0.60 >= L3(0.50)
		},
		{
			name:     "skip synthetic fallback when ratio explicitly unavailable",
			bal:      &exchange.Balance{Total: 100, Available: 40, MarginRatio: 0, MarginRatioUnavailable: true},
			pnl:      -5.0,
			posCount: 1,
			want:     L2Low,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.computeLevel(h.normalizeMarginRatio(tt.bal), tt.pnl, tt.posCount)
			if got != tt.want {
				t.Errorf("computeLevel() = %s, want %s", got, tt.want)
			}
		})
	}
}

// healthStubExchange is a minimal exchange.Exchange stub for health monitor tests.
type healthStubExchange struct {
	exchange.Exchange // embed nil for unimplemented methods
	bal               *exchange.Balance
	positions         []exchange.Position
}

func (s *healthStubExchange) Name() string { return "stub" }
func (s *healthStubExchange) GetFuturesBalance() (*exchange.Balance, error) {
	return s.bal, nil
}
func (s *healthStubExchange) GetSpotBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *healthStubExchange) GetPosition(string) ([]exchange.Position, error) {
	return s.positions, nil
}
func (s *healthStubExchange) GetAllPositions() ([]exchange.Position, error) {
	return s.positions, nil
}

// newTestHealthMonitor creates a HealthMonitor with miniredis for testing.
func newTestHealthMonitor(t *testing.T, cfg *config.Config, exchanges map[string]exchange.Exchange) (*HealthMonitor, *database.Client, *miniredis.Miniredis) {
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
	h := &HealthMonitor{
		exchanges:    exchanges,
		db:           db,
		cfg:          cfg,
		log:          utils.NewLogger("test-health"),
		states:       make(map[string]*ExchangeHealth),
		liqTrend:     NewLiqTrendTracker(cfg),
		actionCh:     make(chan HealthAction, 20),
		stopCh:       make(chan struct{}),
		orphanCounts: make(map[string]int),
	}
	return h, db, mr
}

// TestHealthMonitorSpotPositions_IncludedInCheckAll verifies that checkAll
// fetches both perp and spot positions and includes spot positions in
// position count for health level calculation.
func TestHealthMonitorSpotPositions_IncludedInCheckAll(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		L4ReduceFraction:  0.50,
	}
	exchanges := map[string]exchange.Exchange{
		"bybit": &healthStubExchange{
			bal: &exchange.Balance{
				Total: 100, Available: 5, MarginRatio: 0.96,
			},
		},
	}
	h, db, mr := newTestHealthMonitor(t, cfg, exchanges)
	defer mr.Close()

	// Seed a spot position on bybit.
	now := time.Now().UTC()
	spotPos := &models.SpotFuturesPosition{
		ID:           "spot-1",
		Symbol:       "TESTUSDT",
		Exchange:     "bybit",
		Status:       models.SpotStatusActive,
		Direction:    "buy_spot_short",
		FuturesSide:  "short",
		FuturesEntry: 100.0,
		NotionalUSDT: 500.0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.SaveSpotPosition(spotPos); err != nil {
		t.Fatalf("save spot position: %v", err)
	}

	// Run checkAll — at L5, it should emit a close action.
	h.checkAll()

	// Verify that action was emitted.
	select {
	case action := <-h.actionCh:
		if action.Type != "close" {
			t.Errorf("action type = %q, want %q", action.Type, "close")
		}
		// Verify SpotPositions field is populated.
		if len(action.SpotPositions) == 0 {
			t.Error("action.SpotPositions should be populated with spot positions")
		}
		if len(action.SpotPositions) > 0 && action.SpotPositions[0].ID != "spot-1" {
			t.Errorf("action.SpotPositions[0].ID = %q, want %q", action.SpotPositions[0].ID, "spot-1")
		}
	default:
		t.Error("expected health action to be emitted, got none")
	}
}

// TestHealthMonitor_L4ReduceIncludesSpotPositions verifies that L4 reduce
// actions include spot positions in the SpotPositions field.
func TestHealthMonitor_L4ReduceIncludesSpotPositions(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		L4ReduceFraction:  0.50,
	}
	exchanges := map[string]exchange.Exchange{
		"bybit": &healthStubExchange{
			bal: &exchange.Balance{
				Total: 100, Available: 15, MarginRatio: 0.85,
			},
			// Need some position for PnL to be negative.
			positions: []exchange.Position{
				{Symbol: "BTCUSDT", HoldSide: "long", Total: "0.1", UnrealizedPL: "-10.0"},
			},
		},
	}
	h, db, mr := newTestHealthMonitor(t, cfg, exchanges)
	defer mr.Close()

	// Seed a perp-perp position so PnL is negative.
	perpPos := &models.ArbitragePosition{
		ID:            "perp-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "bybit",
		ShortExchange: "binance",
		Status:        models.StatusActive,
	}
	if err := db.SavePosition(perpPos); err != nil {
		t.Fatalf("save perp position: %v", err)
	}

	// Seed a spot position.
	now := time.Now().UTC()
	spotPos := &models.SpotFuturesPosition{
		ID:           "spot-l4",
		Symbol:       "ETHUSDT",
		Exchange:     "bybit",
		Status:       models.SpotStatusActive,
		Direction:    "buy_spot_short",
		NotionalUSDT: 300.0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.SaveSpotPosition(spotPos); err != nil {
		t.Fatalf("save spot position: %v", err)
	}

	h.checkAll()

	select {
	case action := <-h.actionCh:
		if action.Type != "reduce" {
			t.Errorf("action type = %q, want %q", action.Type, "reduce")
		}
		if len(action.SpotPositions) == 0 {
			t.Error("L4 action should include spot positions in SpotPositions field")
		}
	default:
		t.Error("expected L4 reduce action, got none")
	}
}

// TestHealthMonitor_L5CloseIncludesAllSpotPositions verifies that L5 emergency
// close includes ALL spot positions on the exchange.
func TestHealthMonitor_L5CloseIncludesAllSpotPositions(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		L4ReduceFraction:  0.50,
	}
	exchanges := map[string]exchange.Exchange{
		"bybit": &healthStubExchange{
			bal: &exchange.Balance{
				Total: 100, Available: 3, MarginRatio: 0.97,
			},
		},
	}
	h, db, mr := newTestHealthMonitor(t, cfg, exchanges)
	defer mr.Close()

	now := time.Now().UTC()
	for _, id := range []string{"spot-a", "spot-b"} {
		pos := &models.SpotFuturesPosition{
			ID:           id,
			Symbol:       "TESTUSDT",
			Exchange:     "bybit",
			Status:       models.SpotStatusActive,
			Direction:    "buy_spot_short",
			NotionalUSDT: 200.0,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.SaveSpotPosition(pos); err != nil {
			t.Fatalf("save spot position %s: %v", id, err)
		}
	}

	h.checkAll()

	select {
	case action := <-h.actionCh:
		if action.Type != "close" {
			t.Errorf("action type = %q, want %q", action.Type, "close")
		}
		if len(action.SpotPositions) != 2 {
			t.Errorf("L5 action should include all 2 spot positions, got %d", len(action.SpotPositions))
		}
	default:
		t.Error("expected L5 close action, got none")
	}
}

// TestHealthMonitor_MixedPositionsAggregateCorrectly verifies that an exchange
// with both perp and spot positions has the correct total position count.
func TestHealthMonitor_MixedPositionsAggregateCorrectly(t *testing.T) {
	cfg := &config.Config{
		MarginL3Threshold: 0.50,
		MarginL4Threshold: 0.80,
		MarginL5Threshold: 0.95,
		L4ReduceFraction:  0.50,
	}
	exchanges := map[string]exchange.Exchange{
		"bybit": &healthStubExchange{
			bal: &exchange.Balance{
				Total: 100, Available: 15, MarginRatio: 0.85,
			},
			positions: []exchange.Position{
				{Symbol: "BTCUSDT", HoldSide: "long", Total: "0.1", UnrealizedPL: "-5.0"},
				{Symbol: "ETHUSDT", HoldSide: "short", Total: "1.0", UnrealizedPL: "-3.0"},
			},
		},
	}
	h, db, mr := newTestHealthMonitor(t, cfg, exchanges)
	defer mr.Close()

	// Seed 2 perp-perp positions.
	for _, pos := range []*models.ArbitragePosition{
		{ID: "perp-btc", Symbol: "BTCUSDT", LongExchange: "bybit", ShortExchange: "binance", Status: models.StatusActive},
		{ID: "perp-eth", Symbol: "ETHUSDT", LongExchange: "binance", ShortExchange: "bybit", Status: models.StatusActive},
	} {
		if err := db.SavePosition(pos); err != nil {
			t.Fatalf("save perp position: %v", err)
		}
	}

	// Seed 1 spot position.
	now := time.Now().UTC()
	spotPos := &models.SpotFuturesPosition{
		ID:           "spot-mix",
		Symbol:       "SOLUSDT",
		Exchange:     "bybit",
		Status:       models.SpotStatusActive,
		Direction:    "buy_spot_short",
		NotionalUSDT: 200.0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.SaveSpotPosition(spotPos); err != nil {
		t.Fatalf("save spot position: %v", err)
	}

	h.checkAll()

	// L4 (margin=0.85) with 3 total positions (2 perp + 1 spot).
	// The health state should show 3 positions total.
	state := h.states["bybit"]
	if state == nil {
		t.Fatal("bybit health state should not be nil")
	}
	// posIDs should include both perp and spot positions.
	if len(state.Positions) != 3 {
		t.Errorf("state.Positions count = %d, want 3 (2 perp + 1 spot)", len(state.Positions))
	}
}

func TestHealthLevelString(t *testing.T) {
	tests := []struct {
		level HealthLevel
		want  string
	}{
		{L0None, "L0-None"},
		{L1Safe, "L1-Safe"},
		{L2Low, "L2-Low"},
		{L3Medium, "L3-Medium"},
		{L4High, "L4-High"},
		{L5Critical, "L5-Critical"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("HealthLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestCapitalUsageRatio(t *testing.T) {
	cases := []struct {
		name string
		bal  *exchange.Balance
		want float64
	}{
		{"nil", nil, 0},
		{"zero total", &exchange.Balance{Total: 0, Available: 0}, 0},
		{"empty account", &exchange.Balance{Total: 100, Available: 100}, 0},
		{"fully used", &exchange.Balance{Total: 100, Available: 0}, 1.0},
		{"half used", &exchange.Balance{Total: 100, Available: 50}, 0.5},
		{"avail > total (edge)", &exchange.Balance{Total: 100, Available: 110}, 0},
		{"negative avail", &exchange.Balance{Total: 100, Available: -1}, 1.0},
		{"gateio incident", &exchange.Balance{Total: 259.50, Available: 64.93}, 0.7498},
	}
	for _, c := range cases {
		got := CapitalUsageRatio(c.bal)
		if math.Abs(got-c.want) > 1e-3 {
			t.Errorf("%s: got %.4f want %.4f", c.name, got, c.want)
		}
	}
}

func TestLiquidationRiskRatio(t *testing.T) {
	cases := []struct {
		name string
		bal  *exchange.Balance
		want float64
	}{
		{"nil", nil, 0},
		{"unavailable flag", &exchange.Balance{MarginRatio: 0.5, MarginRatioUnavailable: true}, 0},
		{"normal", &exchange.Balance{MarginRatio: 0.0588}, 0.0588},
		{"high risk", &exchange.Balance{MarginRatio: 0.85}, 0.85},
		{"negative (sentinel)", &exchange.Balance{MarginRatio: -1}, 0},
		{"zero", &exchange.Balance{MarginRatio: 0}, 0},
	}
	for _, c := range cases {
		got := LiquidationRiskRatio(c.bal)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %.4f want %.4f", c.name, got, c.want)
		}
	}
}
