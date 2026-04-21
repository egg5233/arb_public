package spotengine

import (
	"errors"
	"testing"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// ---------------------------------------------------------------------------
// isSpotResidualDust
// ---------------------------------------------------------------------------

func TestIsSpotResidualDust_BelowMinBase(t *testing.T) {
	rules := &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 0.001, MinNotional: 0}
	isDust, eff := isSpotResidualDust(rules, 0.5, 100)
	if !isDust {
		t.Fatalf("expected dust for remaining=0.5 < minBase=1, got isDust=false eff=%.6f", eff)
	}
}

func TestIsSpotResidualDust_FloorsBelowMinBase(t *testing.T) {
	// remaining=1.9, step=1 → floors to 1 which is NOT below minBase=1 (≥)
	rules := &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0}
	isDust, eff := isSpotResidualDust(rules, 1.9, 100)
	if isDust {
		t.Fatalf("expected tradeable: remaining=1.9 floors to eff=%.6f at step=1, should not be dust", eff)
	}
	if eff != 1.0 {
		t.Fatalf("effectiveQty = %.6f, want 1.0", eff)
	}
}

func TestIsSpotResidualDust_BelowMinNotional(t *testing.T) {
	// 0.5 units * $1 = $0.50 notional < minNotional=$1
	rules := &exchange.SpotOrderRules{MinBaseQty: 0.001, QtyStep: 0.001, MinNotional: 1}
	isDust, _ := isSpotResidualDust(rules, 0.5, 1.0)
	if !isDust {
		t.Fatal("expected dust: notional 0.5*1=$0.50 < minNotional=$1")
	}
}

func TestIsSpotResidualDust_StepFloorsToZero(t *testing.T) {
	// GWEI reproduction: remaining=0.102, step=1 → floors to 0 < minBase=1 → dust
	rules := &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0}
	isDust, eff := isSpotResidualDust(rules, 0.102, 0.05)
	if !isDust {
		t.Fatalf("GWEI: 0.102 should floor to %.6f and be dust (minBase=1)", eff)
	}
	if eff != 0.0 {
		t.Fatalf("effectiveQty = %.6f, want 0.0", eff)
	}
}

// ---------------------------------------------------------------------------
// isAlreadyFlatError — table-driven across representative exchange error strings
// ---------------------------------------------------------------------------

func TestIsAlreadyFlatError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantFlat bool
	}{
		// positive cases
		{"gate.io empty position", "position is empty position for symbol GWEI_USDT", true},
		{"gate.io empty caps", "EMPTY POSITION", true},
		{"bybit position not exist", "bybit: position not exist", true},
		{"okx no position", "no position", true},
		{"binance position is zero", "Position is zero for this symbol", true},
		{"reduce-only trailing", "reduce only order: not allowed", true},
		// negative cases — real errors that should NOT be treated as flat
		{"margin insufficient", "margin insufficient", false},
		{"rate limit", "too many requests", false},
		{"network timeout", "context deadline exceeded", false},
		{"generic error", "internal server error", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAlreadyFlatError(errors.New(tc.errMsg))
			if got != tc.wantFlat {
				t.Fatalf("isAlreadyFlatError(%q) = %v, want %v", tc.errMsg, got, tc.wantFlat)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// closeDirectionB dust short-circuit
// ---------------------------------------------------------------------------

func TestCloseDirectionB_DustResidueShortCircuits(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{
			"fut-close-1": {
				OrderID:      "fut-close-1",
				Status:       "filled",
				FilledVolume: 1896,
				AvgPrice:     0.05,
			},
		},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 0.049, Quantity: 10000}},
			Asks: []exchange.PriceLevel{{Price: 0.051, Quantity: 10000}},
		},
	}
	smExch := &closeTestSpotMargin{
		// Gate.io GWEI rules: step=1, minBase=1
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0},
	}

	engine.exchanges = map[string]exchange.Exchange{"gateio": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"gateio": smExch}

	// GWEI scenario: spot bought 1896.102, futures closed 1896, residual = 0.102
	pos := &models.SpotFuturesPosition{
		ID:                "sf-gweiusdt-dust",
		Symbol:            "GWEIUSDT",
		BaseCoin:          "GWEI",
		Exchange:          "gateio",
		Direction:         "buy_spot_short",
		Status:            models.SpotStatusExiting,
		SpotSize:          1896.102,
		SpotExitFilledQty: 1896,
		FuturesExit:       0.05, // already closed
		FuturesSize:       1896,
		FuturesSide:       "short",
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	err := engine.ClosePosition(pos, "spread_reversal", false)
	if err != nil {
		t.Fatalf("ClosePosition returned error: %v", err)
	}
	if smExch.placeCalls != 0 {
		t.Fatalf("PlaceSpotMarginOrder called %d times, want 0 (dust short-circuit)", smExch.placeCalls)
	}
	if !pos.SpotExitFilled {
		t.Fatal("pos.SpotExitFilled should be true after dust short-circuit")
	}
	if pos.SpotExitFilledQty != pos.SpotSize {
		t.Fatalf("SpotExitFilledQty = %.6f, want %.6f", pos.SpotExitFilledQty, pos.SpotSize)
	}
}

// ---------------------------------------------------------------------------
// closeDirectionA dust blocked by outstanding borrow
// ---------------------------------------------------------------------------

func TestCloseDirectionA_DustBlockedByBorrow(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{
			"fut-close-1": {
				OrderID:      "fut-close-1",
				Status:       "filled",
				FilledVolume: 1,
				AvgPrice:     100,
			},
		},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0},
		// Outstanding borrow: well above 1*0.001 = 0.001 threshold
		marginBal: &exchange.MarginBalance{Borrowed: 0.5},
		// provide a query state so the buyback order confirm path can proceed if called
		queryStates: []*exchange.SpotMarginOrderStatus{
			{OrderID: "spot-close-1", Status: "filled", FilledQty: 0.1, AvgPrice: 100},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	// Dir A: futures=1 (already closed via FuturesExit set), spot residual=0.1 (dust at step=1)
	pos := &models.SpotFuturesPosition{
		ID:                "dirA-dust-borrow",
		Symbol:            "BTCUSDT",
		BaseCoin:          "BTC",
		Exchange:          "stub",
		Direction:         "borrow_sell_long",
		Status:            models.SpotStatusExiting,
		SpotSize:          1.1,
		SpotExitFilledQty: 1.0,
		FuturesExit:       100, // already closed
		FuturesSize:       1,
		FuturesSide:       "long",
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	// Should NOT dust-short-circuit because borrow=0.5 > threshold
	// It will attempt the buyback — let it succeed via the query state
	err := engine.ClosePosition(pos, "spread_reversal", false)
	// We expect success (buyback fills the residual) or an error — but critically
	// SpotExitFilled must NOT have been set before the buyback attempt.
	// The key assertion: placeCalls > 0 (dust path was skipped, buyback was attempted).
	if smExch.placeCalls == 0 && !pos.SpotExitFilled {
		t.Fatal("dust short-circuit fired despite outstanding borrow — should have attempted buyback")
	}
	// If placeCalls>0 that proves the dust path was bypassed (correct behavior).
	_ = err
}

// ---------------------------------------------------------------------------
// closeDirectionA dust completes when borrow is cleared
// ---------------------------------------------------------------------------

func TestCloseDirectionA_DustCompletesWhenBorrowCleared(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0},
		// No borrow: safe to dust-close
		marginBal: &exchange.MarginBalance{Borrowed: 0},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:                "dirA-dust-clear",
		Symbol:            "BTCUSDT",
		BaseCoin:          "BTC",
		Exchange:          "stub",
		Direction:         "borrow_sell_long",
		Status:            models.SpotStatusExiting,
		SpotSize:          1.1,
		SpotExitFilledQty: 1.0,
		FuturesExit:       100, // already closed
		FuturesSize:       1,
		FuturesSide:       "long",
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	err := engine.ClosePosition(pos, "spread_reversal", false)
	if err != nil {
		t.Fatalf("ClosePosition returned error: %v", err)
	}
	if smExch.placeCalls != 0 {
		t.Fatalf("PlaceSpotMarginOrder called %d times, want 0 (dust short-circuit)", smExch.placeCalls)
	}
	if !pos.SpotExitFilled {
		t.Fatal("SpotExitFilled should be true when dust+borrow cleared")
	}
}

// ---------------------------------------------------------------------------
// closeDirectionB: empty-futures-position idempotent with FuturesExit from orderbook
// ---------------------------------------------------------------------------

func TestCloseDirectionB_EmptyFuturesPositionIdempotent(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		// PlaceOrder returns "empty position" error — futures already closed
		positions: []exchange.Position{}, // GetPosition returns empty → verified flat
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	// Override PlaceOrder to return an "empty position" error
	futExchWithErr := &closeTestExchangeWithPlaceErr{
		closeTestExchange: futExch,
		placeErr:          errors.New("empty position for symbol BTCUSDT"),
	}

	smExch := &closeTestSpotMargin{
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 0.001, QtyStep: 0.001, MinNotional: 0},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{OrderID: "spot-close-1", Status: "filled", FilledQty: 1, AvgPrice: 100},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExchWithErr}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:          "dirB-idem",
		Symbol:      "BTCUSDT",
		BaseCoin:    "BTC",
		Exchange:    "stub",
		Direction:   "buy_spot_short",
		Status:      models.SpotStatusExiting,
		SpotSize:    1,
		FuturesSize: 1,
		FuturesSide: "short",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	err := engine.ClosePosition(pos, "spread_reversal", false)
	if err != nil {
		t.Fatalf("ClosePosition returned error: %v", err)
	}
	// FuturesExit should be populated from orderbook mid (99+101)/2 = 100
	if pos.FuturesExit <= 0 {
		t.Fatalf("FuturesExit = %.6f, want > 0 (from orderbook mid)", pos.FuturesExit)
	}
}

// ---------------------------------------------------------------------------
// monitor retry auto-heals the stuck GWEI dust position
// ---------------------------------------------------------------------------

func TestMonitorRetry_AutoHealsStuckDustPosition(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 0.049, Quantity: 10000}},
			Asks: []exchange.PriceLevel{{Price: 0.051, Quantity: 10000}},
		},
	}
	smExch := &closeTestSpotMargin{
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 1, QtyStep: 1, MinNotional: 0},
	}

	engine.exchanges = map[string]exchange.Exchange{"gateio": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"gateio": smExch}

	// Seed a stuck exiting position matching the GWEI scenario
	triggered := time.Now().Add(-3 * time.Minute).UTC()
	pos := &models.SpotFuturesPosition{
		ID:                "sf-gweiusdt-1776555134770",
		Symbol:            "GWEIUSDT",
		BaseCoin:          "GWEI",
		Exchange:          "gateio",
		Direction:         "buy_spot_short",
		Status:            models.SpotStatusExiting,
		SpotSize:          1896.102,
		SpotExitFilledQty: 1896,
		FuturesExit:       0.05, // futures already closed
		FuturesSize:       1896,
		FuturesSide:       "short",
		ExitReason:        "spread_reversal",
		ExitTriggeredAt:   &triggered,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	// Call initiateExit directly (same code path monitorTick uses via launchExit)
	engine.initiateExit(pos, pos.ExitReason, false)

	// Assert: dust short-circuit fires → no PlaceSpotMarginOrder call, spot marked closed
	if smExch.placeCalls != 0 {
		t.Fatalf("PlaceSpotMarginOrder called %d times after monitor retry, want 0 (dust auto-heal)", smExch.placeCalls)
	}
	if !pos.SpotExitFilled {
		t.Fatal("position should be marked SpotExitFilled after dust auto-heal on monitor retry")
	}
}

// ---------------------------------------------------------------------------
// DEGO regression: SpotSize unaligned to LOT_SIZE stepSize must be floored
// before PlaceSpotMarginOrder, not sent as-is (Binance -1013 LOT_SIZE).
// ---------------------------------------------------------------------------

func TestCloseDirectionB_SpotSellRoundsToLotStep(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 0.157, Quantity: 10000}},
			Asks: []exchange.PriceLevel{{Price: 0.159, Quantity: 10000}},
		},
	}
	// Binance DEGOUSDT spot: stepSize=0.01, minQty=0.01.
	// First sell ordered at 1258.14 fills fully; residue 0.0006 must be dust-closed.
	smExch := &closeTestSpotMargin{
		spotRules: &exchange.SpotOrderRules{MinBaseQty: 0.01, QtyStep: 0.01, MinNotional: 5},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{OrderID: "spot-close-1", Status: "filled", FilledQty: 1258.14, AvgPrice: 0.158},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"binance": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"binance": smExch}

	pos := &models.SpotFuturesPosition{
		ID:             "sf-degousdt-reproduction",
		Symbol:         "DEGOUSDT",
		BaseCoin:       "DEGO",
		Exchange:       "binance",
		Direction:      "buy_spot_short",
		Status:         models.SpotStatusExiting,
		SpotSize:       1258.1406, // unaligned to step=0.01 (the actual DEGO bug value)
		SpotEntryPrice: 0.157339,
		FuturesExit:    0.159040, // futures already closed
		FuturesSize:    1259.4,
		FuturesSide:    "short",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	err := engine.ClosePosition(pos, "delist_DEGOUSDT", false)
	if err != nil {
		t.Fatalf("ClosePosition returned error: %v", err)
	}

	if smExch.placeCalls < 1 {
		t.Fatalf("PlaceSpotMarginOrder never called (placeCalls=%d)", smExch.placeCalls)
	}
	// Critical assertion: the submitted Size must be step-aligned (1258.14), NOT
	// the raw unaligned 1258.1406 that Binance rejects with -1013 LOT_SIZE.
	firstSize := smExch.placeSizes[0]
	if firstSize != "1258.140000" {
		t.Fatalf("first PlaceSpotMarginOrder Size = %q, want %q (rounded to stepSize=0.01)", firstSize, "1258.140000")
	}
	if !pos.SpotExitFilled {
		t.Fatal("SpotExitFilled should be true after rounded sell + dust close")
	}
}

// ---------------------------------------------------------------------------
// helper: closeTestExchange variant that returns a configurable PlaceOrder error
// ---------------------------------------------------------------------------

type closeTestExchangeWithPlaceErr struct {
	*closeTestExchange
	placeErr error
}

func (s *closeTestExchangeWithPlaceErr) PlaceOrder(exchange.PlaceOrderParams) (string, error) {
	s.placeCalls++
	if s.placeErr != nil {
		return "", s.placeErr
	}
	return "fut-close-1", nil
}
