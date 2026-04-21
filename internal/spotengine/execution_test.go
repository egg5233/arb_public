package spotengine

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

type closeTestExchange struct {
	orderUpdates     map[string]exchange.OrderUpdate
	orderbook        *exchange.Orderbook
	orderbookEntered chan struct{}
	orderbookRelease <-chan struct{}
	positions        []exchange.Position
	placeCalls       int
}

func (s *closeTestExchange) Name() string { return "stub" }
func (s *closeTestExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) {
	s.placeCalls++
	return "fut-close-1", nil
}
func (s *closeTestExchange) CancelOrder(string, string) error                  { return nil }
func (s *closeTestExchange) GetPendingOrders(string) ([]exchange.Order, error) { return nil, nil }
func (s *closeTestExchange) GetOrderFilledQty(string, string) (float64, error) { return 0, nil }
func (s *closeTestExchange) GetPosition(string) ([]exchange.Position, error)   { return s.positions, nil }
func (s *closeTestExchange) GetAllPositions() ([]exchange.Position, error)     { return nil, nil }
func (s *closeTestExchange) SetLeverage(string, string, string) error          { return nil }
func (s *closeTestExchange) SetMarginMode(string, string) error                { return nil }
func (s *closeTestExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return nil, nil
}
func (s *closeTestExchange) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s *closeTestExchange) GetFundingInterval(string) (time.Duration, error)     { return 0, nil }
func (s *closeTestExchange) GetFuturesBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *closeTestExchange) GetSpotBalance() (*exchange.Balance, error) { return nil, nil }
func (s *closeTestExchange) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s *closeTestExchange) TransferToSpot(string, string) error    { return nil }
func (s *closeTestExchange) TransferToFutures(string, string) error { return nil }
func (s *closeTestExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	if s.orderbookEntered != nil {
		select {
		case s.orderbookEntered <- struct{}{}:
		default:
		}
	}
	if s.orderbookRelease != nil {
		<-s.orderbookRelease
	}
	return s.orderbook, nil
}
func (s *closeTestExchange) StartPriceStream([]string)                   {}
func (s *closeTestExchange) SubscribeSymbol(string) bool                 { return false }
func (s *closeTestExchange) GetBBO(string) (exchange.BBO, bool)          { return exchange.BBO{}, false }
func (s *closeTestExchange) GetPriceStore() *sync.Map                    { return nil }
func (s *closeTestExchange) SubscribeDepth(string) bool                  { return false }
func (s *closeTestExchange) UnsubscribeDepth(string) bool                { return false }
func (s *closeTestExchange) GetDepth(string) (*exchange.Orderbook, bool) { return nil, false }
func (s *closeTestExchange) StartPrivateStream()                         {}
func (s *closeTestExchange) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	upd, ok := s.orderUpdates[orderID]
	return upd, ok
}
func (s *closeTestExchange) SetOrderCallback(func(exchange.OrderUpdate)) {}
func (s *closeTestExchange) SetMetricsCallback(exchange.MetricsCallback) {}
func (s *closeTestExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) {
	return "", nil
}
func (s *closeTestExchange) CancelStopLoss(string, string) error { return nil }
func (s *closeTestExchange) PlaceTakeProfit(exchange.TakeProfitParams) (string, error) {
	return "", nil
}
func (s *closeTestExchange) CancelTakeProfit(string, string) error { return nil }
func (s *closeTestExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *closeTestExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *closeTestExchange) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *closeTestExchange) WithdrawFeeInclusive() bool { return false }
func (s *closeTestExchange) GetWithdrawFee(string, string) (float64, float64, error) {
	return 0, 0, nil
}
func (s *closeTestExchange) EnsureOneWayMode() error      { return nil }
func (s *closeTestExchange) CancelAllOrders(string) error { return nil }
func (s *closeTestExchange) Close()                       {}

type closeTestSpotMargin struct {
	placeCalls   int
	placeSizes   []string
	orderIDs     []string
	queryCalls   int
	queryErrs    []error
	queryStates  []*exchange.SpotMarginOrderStatus
	repayCalls   int
	repayAmounts []string
	marginBal    *exchange.MarginBalance
	onPlace      func(call int, params exchange.SpotMarginOrderParams)
	spotRules    *exchange.SpotOrderRules
	spotRulesErr error
}

func (s *closeTestSpotMargin) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s *closeTestSpotMargin) MarginRepay(params exchange.MarginRepayParams) error {
	s.repayCalls++
	s.repayAmounts = append(s.repayAmounts, params.Amount)
	return nil
}
func (s *closeTestSpotMargin) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	s.placeCalls++
	s.placeSizes = append(s.placeSizes, params.Size)
	if s.onPlace != nil {
		s.onPlace(s.placeCalls, params)
	}
	if len(s.orderIDs) > 0 {
		orderID := s.orderIDs[0]
		s.orderIDs = s.orderIDs[1:]
		return orderID, nil
	}
	return "spot-close-1", nil
}
func (s *closeTestSpotMargin) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return nil, nil
}
func (s *closeTestSpotMargin) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	if s.marginBal != nil {
		return s.marginBal, nil
	}
	return &exchange.MarginBalance{}, nil
}
func (s *closeTestSpotMargin) GetSpotBBO(string) (exchange.BBO, error) {
	return exchange.BBO{Bid: 100, Ask: 100.1}, nil
}
func (s *closeTestSpotMargin) TransferToMargin(string, string) error   { return nil }
func (s *closeTestSpotMargin) TransferFromMargin(string, string) error { return nil }
func (s *closeTestSpotMargin) GetMarginInterestRateHistory(_ context.Context, _ string, _, _ time.Time) ([]exchange.MarginInterestRatePoint, error) {
	return nil, exchange.ErrHistoricalBorrowNotSupported
}
func (s *closeTestSpotMargin) SpotOrderRules(string) (*exchange.SpotOrderRules, error) {
	return s.spotRules, s.spotRulesErr
}
func (s *closeTestSpotMargin) GetSpotMarginOrder(string, string) (*exchange.SpotMarginOrderStatus, error) {
	s.queryCalls++
	if len(s.queryErrs) > 0 {
		err := s.queryErrs[0]
		s.queryErrs = s.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.queryStates) == 0 {
		return nil, nil
	}
	state := s.queryStates[0]
	s.queryStates = s.queryStates[1:]
	return state, nil
}

func newExecutionTestEngine(t *testing.T) (*SpotEngine, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	return &SpotEngine{
		cfg:       &config.Config{},
		db:        db,
		log:       utils.NewLogger("test"),
		stopCh:    make(chan struct{}),
		exitState: exitState{exiting: make(map[string]bool)},
	}, mr
}

func TestClosePosition_ReusesPendingSpotExitOrderAfterQueryFailure(t *testing.T) {
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
			Bids: []exchange.PriceLevel{{Price: 100, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 102, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		queryErrs: []error{errors.New("temporary spot query failure"), nil},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-close-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  101,
			},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:          "pos-1",
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

	err := engine.ClosePosition(pos, "manual_close", false)
	if err == nil {
		t.Fatal("expected first close attempt to stop on unconfirmed spot fill")
	}

	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition after first attempt: %v", err)
	}
	if stored.PendingSpotExitOrderID != "spot-close-1" {
		t.Fatalf("pending spot exit order = %q, want %q", stored.PendingSpotExitOrderID, "spot-close-1")
	}
	if smExch.placeCalls != 1 {
		t.Fatalf("spot place calls after first attempt = %d, want 1", smExch.placeCalls)
	}

	err = engine.ClosePosition(stored, "manual_close", false)
	if err != nil {
		t.Fatalf("second close attempt failed: %v", err)
	}
	if smExch.placeCalls != 1 {
		t.Fatalf("spot place calls after retry = %d, want 1", smExch.placeCalls)
	}
	if !stored.SpotExitFilled {
		t.Fatal("spot exit should be marked filled after reconciled retry")
	}
	if stored.PendingSpotExitOrderID != "" {
		t.Fatalf("pending spot exit order should be cleared, got %q", stored.PendingSpotExitOrderID)
	}
	if stored.SpotExitPrice != 101 {
		t.Fatalf("spot exit price = %.2f, want 101", stored.SpotExitPrice)
	}
}

func TestClosePosition_RetriesOnlyRemainingSpotQtyAfterPartialExit(t *testing.T) {
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
			Bids: []exchange.PriceLevel{{Price: 100, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 102, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-close-1", "spot-close-2"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-close-1",
				Symbol:    "BTCUSDT",
				Status:    "cancelled",
				FilledQty: 0.4,
				AvgPrice:  101,
			},
			{
				OrderID:   "spot-close-2",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 0.6,
				AvgPrice:  102,
			},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:          "pos-partial",
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

	if err := engine.ClosePosition(pos, "manual_close", false); err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}
	if smExch.placeCalls != 2 {
		t.Fatalf("spot place calls = %d, want 2", smExch.placeCalls)
	}
	if len(smExch.placeSizes) != 2 {
		t.Fatalf("spot place sizes len = %d, want 2", len(smExch.placeSizes))
	}
	if smExch.placeSizes[0] != "1" && smExch.placeSizes[0] != "1.000000" {
		t.Fatalf("first spot place size = %q, want full size", smExch.placeSizes[0])
	}
	if smExch.placeSizes[1] != "0.6" && smExch.placeSizes[1] != "0.600000" {
		t.Fatalf("second spot place size = %q, want remaining size 0.6", smExch.placeSizes[1])
	}
	if !pos.SpotExitFilled {
		t.Fatal("spot exit should be fully marked after retry")
	}
	if pos.SpotExitFilledQty != 1 {
		t.Fatalf("spot exit filled qty = %.2f, want 1", pos.SpotExitFilledQty)
	}
	if pos.PendingSpotExitOrderID != "" {
		t.Fatalf("pending spot exit order should be cleared, got %q", pos.PendingSpotExitOrderID)
	}
	if math.Abs(pos.SpotExitPrice-101.6) > 1e-9 {
		t.Fatalf("spot exit price = %.4f, want 101.6", pos.SpotExitPrice)
	}

	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.SpotExitFilledQty != 1 {
		t.Fatalf("stored spot exit filled qty = %.2f, want 1", stored.SpotExitFilledQty)
	}
}

func TestManualOpen_RejectsDelistedSymbol(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   5,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
		DelistFilterEnabled:       true,
	}
	engine.api = api.NewServer(engine.db, engine.cfg, nil)
	engine.exchanges = map[string]exchange.Exchange{"stub": &closeTestExchange{}}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": &closeTestSpotMargin{}}
	engine.latestOpps = []SpotArbOpportunity{
		{Symbol: "DEGOUSDT", BaseCoin: "DEGO", Exchange: "stub", Direction: "buy_spot_short"},
	}

	// Simulate the discovery poller having written the delist blacklist entry.
	mr.Set("arb:delist:DEGOUSDT", "1")

	err := engine.ManualOpen("DEGOUSDT", "stub", "buy_spot_short")
	if err == nil {
		t.Fatal("expected ManualOpen to block delist-flagged symbol")
	}
	if !strings.Contains(err.Error(), "delist") {
		t.Fatalf("error = %v, want delist-related message", err)
	}
}

func TestManualOpen_RejectsConcurrentEntry(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}
	engine.api = api.NewServer(engine.db, engine.cfg, nil)

	orderbookEntered := make(chan struct{}, 1)
	orderbookRelease := make(chan struct{})
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
		orderbookEntered: orderbookEntered,
		orderbookRelease: orderbookRelease,
	}
	smExch := &closeTestSpotMargin{
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-close-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "buy_spot_short",
		},
	}

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	}()

	select {
	case <-orderbookEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first ManualOpen did not reach orderbook in time")
	}

	err := engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	if err == nil {
		t.Fatal("expected concurrent ManualOpen to fail")
	}
	if !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("concurrent ManualOpen error = %v, want entry-in-progress conflict", err)
	}

	close(orderbookRelease)

	if err := <-firstErrCh; err != nil {
		t.Fatalf("first ManualOpen failed: %v", err)
	}

	active, err := engine.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active positions = %d, want 1", len(active))
	}
	if futExch.placeCalls != 1 {
		t.Fatalf("futures place calls = %d, want 1", futExch.placeCalls)
	}
	if smExch.placeCalls != 1 {
		t.Fatalf("spot place calls = %d, want 1", smExch.placeCalls)
	}
}

func TestManualOpen_FailsClosedWhenEntryLockIsLost(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	prevTTL := spotEntryLockTTL
	spotEntryLockTTL = 150 * time.Millisecond
	t.Cleanup(func() {
		spotEntryLockTTL = prevTTL
	})

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}
	engine.api = api.NewServer(engine.db, engine.cfg, nil)

	orderbookEntered := make(chan struct{}, 1)
	orderbookRelease := make(chan struct{})
	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
		orderbookEntered: orderbookEntered,
		orderbookRelease: orderbookRelease,
	}
	smExch := &closeTestSpotMargin{}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "buy_spot_short",
		},
	}

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	}()

	select {
	case <-orderbookEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first ManualOpen did not reach orderbook in time")
	}

	mr.FastForward(spotEntryLockTTL + 10*time.Millisecond)

	secondLock, ok, err := engine.db.AcquireOwnedLock(spotEntryLockKey, spotEntryLockTTL)
	if err != nil {
		t.Fatalf("AcquireOwnedLock second: %v", err)
	}
	if !ok {
		t.Fatal("expected second lock acquisition after first lease expired")
	}
	defer func() {
		if err := secondLock.Release(); err != nil {
			t.Fatalf("second Release: %v", err)
		}
	}()

	close(orderbookRelease)

	err = <-firstErrCh
	if err == nil {
		t.Fatal("expected first ManualOpen to fail after losing the entry lock")
	}
	if !strings.Contains(err.Error(), "spot entry lock lost") {
		t.Fatalf("first ManualOpen error = %v, want lock-loss failure", err)
	}

	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls = %d, want 0", futExch.placeCalls)
	}
	if smExch.placeCalls != 0 {
		t.Fatalf("spot place calls = %d, want 0", smExch.placeCalls)
	}

	active, err := engine.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active positions = %d, want 0", len(active))
	}
}

func TestManualOpen_PersistsPendingEntryUntilSpotConfirmationRecovers(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}
	engine.api = api.NewServer(engine.db, engine.cfg, nil)

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
		queryErrs: []error{errors.New("temporary spot query failure")},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-close-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "buy_spot_short",
		},
	}

	err := engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	if err == nil {
		t.Fatal("expected ManualOpen to return pending confirmation")
	}
	if !strings.Contains(err.Error(), "pending confirmation") {
		t.Fatalf("ManualOpen error = %v, want pending confirmation", err)
	}
	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls after pending entry = %d, want 0", futExch.placeCalls)
	}

	active, err := engine.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions after pending entry: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active positions after pending entry = %d, want 1", len(active))
	}
	if active[0].Status != models.SpotStatusPending {
		t.Fatalf("pending position status = %q, want %q", active[0].Status, models.SpotStatusPending)
	}
	if active[0].PendingEntryOrderID != "spot-close-1" {
		t.Fatalf("pending entry order id = %q, want %q", active[0].PendingEntryOrderID, "spot-close-1")
	}

	engine.monitorTick()

	recovered, err := engine.db.GetSpotPosition(active[0].ID)
	if err != nil {
		t.Fatalf("GetSpotPosition after recovery: %v", err)
	}
	if recovered.Status != models.SpotStatusActive {
		t.Fatalf("recovered position status = %q, want %q", recovered.Status, models.SpotStatusActive)
	}
	if recovered.PendingEntryOrderID != "" {
		t.Fatalf("pending entry order id should be cleared, got %q", recovered.PendingEntryOrderID)
	}
	if recovered.FuturesSize != 1 {
		t.Fatalf("futures size = %.2f, want 1", recovered.FuturesSize)
	}
	if futExch.placeCalls != 1 {
		t.Fatalf("futures place calls after recovery = %d, want 1", futExch.placeCalls)
	}
}

func TestManualOpen_PendingEntryReconcilesAllocatorExposureToActualNotional(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		EnableCapitalAllocator:    true,
		MaxTotalExposureUSDT:      1000,
		MaxPerpPerpPct:            1,
		MaxSpotFuturesPct:         1,
		MaxPerExchangePct:         1,
		ReservationTTLSec:         300,
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}
	engine.allocator = risk.NewCapitalAllocator(engine.db, engine.cfg)
	engine.api = api.NewServer(engine.db, engine.cfg, nil)

	futExch := &closeTestExchange{
		orderUpdates: map[string]exchange.OrderUpdate{
			"fut-close-1": {
				OrderID:      "fut-close-1",
				Status:       "filled",
				FilledVolume: 0.5,
				AvgPrice:     100,
			},
		},
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		queryErrs: []error{errors.New("temporary spot query failure")},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-close-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 0.5,
				AvgPrice:  100,
			},
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "buy_spot_short",
		},
	}

	err := engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	if err == nil || !strings.Contains(err.Error(), "pending confirmation") {
		t.Fatalf("ManualOpen error = %v, want pending confirmation", err)
	}

	summary, err := engine.allocator.Summary()
	if err != nil {
		t.Fatalf("allocator summary after pending entry: %v", err)
	}
	if got := summary.ByExchange["stub"]; got != 100 {
		t.Fatalf("allocator exposure after pending entry = %.2f, want 100", got)
	}

	engine.monitorTick()

	summary, err = engine.allocator.Summary()
	if err != nil {
		t.Fatalf("allocator summary after recovery: %v", err)
	}
	if got := summary.ByExchange["stub"]; got != 50 {
		t.Fatalf("allocator exposure after recovery = %.2f, want 50", got)
	}
}

func TestManualOpen_CleansUpAcceptedSpotOrderWhenPendingEntrySaveFails(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	workingDB := engine.db
	failRedis := miniredis.RunT(t)
	failingDB, err := database.New(failRedis.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New failingDB: %v", err)
	}
	failRedis.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}

	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-entry-1", "spot-cleanup-1"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-entry-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
			{
				OrderID:   "spot-cleanup-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
		},
		onPlace: func(call int, _ exchange.SpotMarginOrderParams) {
			if call == 1 {
				engine.db = failingDB
			}
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "buy_spot_short",
		},
	}

	err = engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	if err == nil {
		t.Fatal("expected ManualOpen to fail when pending entry save fails")
	}
	if !strings.Contains(err.Error(), "pending entry save failed") {
		t.Fatalf("ManualOpen error = %v, want pending entry save failure", err)
	}
	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls = %d, want 0", futExch.placeCalls)
	}
	if smExch.placeCalls != 2 {
		t.Fatalf("spot place calls = %d, want 2 (entry + cleanup)", smExch.placeCalls)
	}

	active, err := workingDB.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	// Pending entry record remains because engine.db points at failed Redis,
	// so abandonPendingEntry cannot update the record to closed.
	if len(active) != 1 {
		t.Fatalf("active positions = %d, want 1 (pending entry retained when abandon fails)", len(active))
	}
}

func TestManualOpen_ReversesAndRepaysBorrowWhenPendingEntrySaveFails(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	workingDB := engine.db
	failRedis := miniredis.RunT(t)
	failingDB, err := database.New(failRedis.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New failingDB: %v", err)
	}
	failRedis.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}

	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-entry-1", "spot-cleanup-1"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-entry-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
			{
				OrderID:   "spot-cleanup-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
		},
		onPlace: func(call int, _ exchange.SpotMarginOrderParams) {
			if call == 1 {
				engine.db = failingDB
			}
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "borrow_sell_long",
		},
	}

	err = engine.ManualOpen("BTCUSDT", "stub", "borrow_sell_long")
	if err == nil {
		t.Fatal("expected ManualOpen to fail when pending entry save fails")
	}
	if !strings.Contains(err.Error(), "pending entry save failed") {
		t.Fatalf("ManualOpen error = %v, want pending entry save failure", err)
	}
	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls = %d, want 0", futExch.placeCalls)
	}
	if smExch.placeCalls != 2 {
		t.Fatalf("spot place calls = %d, want 2 (entry + cleanup)", smExch.placeCalls)
	}
	// With auto-borrow, repay is handled by the buyback order's AutoRepay flag,
	// not a separate MarginRepay call.
	if smExch.repayCalls != 0 {
		t.Fatalf("repay calls = %d, want 0 (auto-repay via buyback order)", smExch.repayCalls)
	}

	active, err := workingDB.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active positions = %d, want 1 (pending entry retained when abandon fails)", len(active))
	}
}

func TestManualOpen_ReportsManualInterventionWhenCleanupOrderOnlyPartiallyFills(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	failRedis := miniredis.RunT(t)
	failingDB, err := database.New(failRedis.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New failingDB: %v", err)
	}
	failRedis.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}

	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-entry-1", "spot-cleanup-1"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-entry-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
			{
				OrderID:   "spot-cleanup-1",
				Symbol:    "BTCUSDT",
				Status:    "cancelled",
				FilledQty: 0.4,
				AvgPrice:  100,
			},
		},
		onPlace: func(call int, _ exchange.SpotMarginOrderParams) {
			if call == 1 {
				engine.db = failingDB
			}
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "borrow_sell_long",
		},
	}

	err = engine.ManualOpen("BTCUSDT", "stub", "borrow_sell_long")
	if err == nil {
		t.Fatal("expected ManualOpen to fail when cleanup order only partially fills")
	}
	if !strings.Contains(err.Error(), "manual intervention required") {
		t.Fatalf("ManualOpen error = %v, want manual intervention requirement", err)
	}
	if smExch.placeCalls != 2 {
		t.Fatalf("spot place calls = %d, want 2 (entry + cleanup)", smExch.placeCalls)
	}
	if smExch.repayCalls != 0 {
		t.Fatalf("repay calls = %d, want 0 when cleanup is incomplete", smExch.repayCalls)
	}
}

func TestManualOpen_PersistsManualRecoveryPositionWhenCleanupOnlyPartiallyFills(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	workingDB := engine.db
	failRedis := miniredis.RunT(t)
	failingDB, err := database.New(failRedis.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New failingDB: %v", err)
	}
	failRedis.Close()

	engine.cfg = &config.Config{
		EnableCapitalAllocator:    true,
		MaxTotalExposureUSDT:      1000,
		MaxPerpPerpPct:            1,
		MaxSpotFuturesPct:         1,
		MaxPerExchangePct:         1,
		ReservationTTLSec:         300,
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
	}
	engine.allocator = risk.NewCapitalAllocator(workingDB, engine.cfg)

	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-entry-1", "spot-cleanup-1"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-entry-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
			{
				OrderID:   "spot-cleanup-1",
				Symbol:    "BTCUSDT",
				Status:    "cancelled",
				FilledQty: 0.4,
				AvgPrice:  100,
			},
		},
		onPlace: func(call int, _ exchange.SpotMarginOrderParams) {
			if call == 1 {
				engine.db = failingDB
				return
			}
			if call == 2 {
				engine.db = workingDB
			}
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "borrow_sell_long",
		},
	}

	err = engine.ManualOpen("BTCUSDT", "stub", "borrow_sell_long")
	if err == nil {
		t.Fatal("expected ManualOpen to require manual recovery when cleanup only partially fills")
	}
	if !strings.Contains(err.Error(), "manual intervention required") {
		t.Fatalf("ManualOpen error = %v, want manual intervention requirement", err)
	}

	active, err := workingDB.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active positions = %d, want 1", len(active))
	}
	pos := active[0]
	if pos.Status != models.SpotStatusPending {
		t.Fatalf("manual recovery status = %q, want %q", pos.Status, models.SpotStatusPending)
	}
	if pos.ExitReason != spotEntryManualRecoveryReason {
		t.Fatalf("manual recovery reason = %q, want %q", pos.ExitReason, spotEntryManualRecoveryReason)
	}
	if pos.PendingEntryOrderID != "" {
		t.Fatalf("manual recovery pending entry order = %q, want empty", pos.PendingEntryOrderID)
	}
	if pos.PendingSpotExitOrderID != "spot-cleanup-1" {
		t.Fatalf("manual recovery cleanup order = %q, want %q", pos.PendingSpotExitOrderID, "spot-cleanup-1")
	}
	if math.Abs(pos.SpotSize-0.6) > 1e-9 {
		t.Fatalf("manual recovery spot size = %.6f, want 0.600000", pos.SpotSize)
	}
	if math.Abs(pos.BorrowAmount-0.6) > 1e-9 {
		t.Fatalf("manual recovery borrow amount = %.6f, want 0.600000", pos.BorrowAmount)
	}
	if math.Abs(pos.NotionalUSDT-60) > 1e-9 {
		t.Fatalf("manual recovery notional = %.2f, want 60.00", pos.NotionalUSDT)
	}

	summary, err := engine.allocator.Summary()
	if err != nil {
		t.Fatalf("allocator summary after manual recovery = %v", err)
	}
	if got := summary.ByExchange["stub"]; got != 60 {
		t.Fatalf("allocator exposure after manual recovery = %.2f, want 60.00", got)
	}

	engine.monitorTick()
	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls after manual recovery = %d, want 0", futExch.placeCalls)
	}
}

func TestManualOpen_RetainsPendingRecordWhenManualRecoverySaveFails(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	workingDB := engine.db
	failRedis := miniredis.RunT(t)
	failingDB, err := database.New(failRedis.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New failingDB: %v", err)
	}
	failRedis.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:   1,
		SpotFuturesLeverage:       3,
		SpotFuturesCapitalUnified: 100,
		EnableCapitalAllocator:    true,
		MaxTotalExposureUSDT:      1000,
		MaxPerpPerpPct:            1,
		MaxSpotFuturesPct:         1,
		MaxPerExchangePct:         1,
	}
	engine.allocator = risk.NewCapitalAllocator(engine.db, engine.cfg)

	futExch := &closeTestExchange{
		orderbook: &exchange.Orderbook{
			Bids: []exchange.PriceLevel{{Price: 99, Quantity: 1}},
			Asks: []exchange.PriceLevel{{Price: 101, Quantity: 1}},
		},
	}
	smExch := &closeTestSpotMargin{
		orderIDs: []string{"spot-entry-1", "spot-cleanup-1"},
		queryStates: []*exchange.SpotMarginOrderStatus{
			{
				OrderID:   "spot-entry-1",
				Symbol:    "BTCUSDT",
				Status:    "filled",
				FilledQty: 1,
				AvgPrice:  100,
			},
			{
				OrderID:   "spot-cleanup-1",
				Symbol:    "BTCUSDT",
				Status:    "cancelled",
				FilledQty: 0.4,
				AvgPrice:  100,
			},
		},
		onPlace: func(call int, _ exchange.SpotMarginOrderParams) {
			if call == 1 {
				engine.db = failingDB
			}
		},
	}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}
	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			Exchange:  "stub",
			Direction: "borrow_sell_long",
		},
	}

	err = engine.ManualOpen("BTCUSDT", "stub", "borrow_sell_long")
	if err == nil {
		t.Fatal("expected ManualOpen to fail when manual recovery save fails")
	}
	if !strings.Contains(err.Error(), "could not be persisted") {
		t.Fatalf("ManualOpen error = %v, want manual recovery persistence failure", err)
	}

	active, err := workingDB.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("active positions = %d, want 1", len(active))
	}
	if active[0].Status != models.SpotStatusPending {
		t.Fatalf("retained position status = %q, want %q", active[0].Status, models.SpotStatusPending)
	}
	if active[0].PendingEntryOrderID != "" {
		t.Fatalf("retained pending entry order = %q, want empty preflight checkpoint", active[0].PendingEntryOrderID)
	}

	summary, err := engine.allocator.Summary()
	if err != nil {
		t.Fatalf("allocator summary after failed manual recovery save: %v", err)
	}
	if got := summary.ByExchange["stub"]; got != 60 {
		t.Fatalf("allocator exposure after failed manual recovery save = %.2f, want 60.00 (remaining after partial cleanup)", got)
	}
	if summary.Reservations != 0 {
		t.Fatalf("allocator reservations after failed manual recovery save = %d, want 0", summary.Reservations)
	}
	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls after failed manual recovery save = %d, want 0", futExch.placeCalls)
	}
}

func TestMonitorTick_DoesNotHedgePreflightPendingEntry(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{}
	smExch := &closeTestSpotMargin{}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:           "preflight-1",
		Symbol:       "BTCUSDT",
		BaseCoin:     "BTC",
		Exchange:     "stub",
		Direction:    "buy_spot_short",
		Status:       models.SpotStatusPending,
		SpotSize:     1,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		NotionalUSDT: 100,
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	engine.monitorTick()

	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls = %d, want 0 for preflight checkpoint", futExch.placeCalls)
	}
	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.Status != models.SpotStatusPending {
		t.Fatalf("status = %q, want %q", stored.Status, models.SpotStatusPending)
	}
}

func TestMonitorTick_ReusesExistingFuturesHedgeForPendingEntry(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	futExch := &closeTestExchange{
		positions: []exchange.Position{
			{
				Symbol:           "BTCUSDT",
				HoldSide:         "short",
				Total:            "1",
				AverageOpenPrice: "100",
			},
		},
	}
	smExch := &closeTestSpotMargin{}

	engine.exchanges = map[string]exchange.Exchange{"stub": futExch}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{"stub": smExch}

	pos := &models.SpotFuturesPosition{
		ID:                  "recover-hedge-1",
		Symbol:              "BTCUSDT",
		BaseCoin:            "BTC",
		Exchange:            "stub",
		Direction:           "buy_spot_short",
		Status:              models.SpotStatusPending,
		SpotSize:            1,
		SpotEntryPrice:      100,
		NotionalUSDT:        100,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
		PendingEntryOrderID: "",
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	engine.monitorTick()

	if futExch.placeCalls != 0 {
		t.Fatalf("futures place calls = %d, want 0 when hedge already exists", futExch.placeCalls)
	}
	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.Status != models.SpotStatusActive {
		t.Fatalf("status = %q, want %q", stored.Status, models.SpotStatusActive)
	}
	if stored.FuturesSize != 1 {
		t.Fatalf("futures size = %.2f, want 1", stored.FuturesSize)
	}
	if stored.FuturesEntry != 100 {
		t.Fatalf("futures entry = %.2f, want 100", stored.FuturesEntry)
	}
}

func TestManualClose_ClearsManualRecoveryWhenExchangeIsFlat(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		EnableCapitalAllocator: true,
		MaxTotalExposureUSDT:   1000,
		MaxPerpPerpPct:         1,
		MaxSpotFuturesPct:      1,
		MaxPerExchangePct:      1,
	}
	engine.allocator = risk.NewCapitalAllocator(engine.db, engine.cfg)

	pos := &models.SpotFuturesPosition{
		ID:                     "manual-recovery-1",
		Symbol:                 "BTCUSDT",
		BaseCoin:               "BTC",
		Exchange:               "stub",
		Direction:              "borrow_sell_long",
		Status:                 models.SpotStatusPending,
		ExitReason:             spotEntryManualRecoveryReason,
		SpotSize:               0.6,
		BorrowAmount:           0.6,
		NotionalUSDT:           60,
		PendingSpotExitOrderID: "spot-cleanup-1",
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
		SpotEntryPrice:         100,
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}
	res, err := engine.allocator.Reserve(risk.StrategySpotFutures, map[string]float64{"stub": 60})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := engine.allocator.Commit(res, pos.ID, map[string]float64{"stub": 60}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"stub": &closeTestSpotMargin{
			queryStates: []*exchange.SpotMarginOrderStatus{
				{
					OrderID:   "spot-cleanup-1",
					Symbol:    "BTCUSDT",
					Status:    "cancelled",
					FilledQty: 0.6,
					AvgPrice:  100,
				},
			},
			marginBal: &exchange.MarginBalance{},
		},
	}

	if err := engine.ManualClose(pos.ID); err != nil {
		t.Fatalf("ManualClose: %v", err)
	}

	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.Status != models.SpotStatusClosed {
		t.Fatalf("status = %q, want %q", stored.Status, models.SpotStatusClosed)
	}
	if stored.ExitCompletedAt == nil {
		t.Fatal("ExitCompletedAt should be set")
	}

	active, err := engine.db.GetActiveSpotPositions()
	if err != nil {
		t.Fatalf("GetActiveSpotPositions: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active positions = %d, want 0", len(active))
	}

	summary, err := engine.allocator.Summary()
	if err != nil {
		t.Fatalf("allocator summary: %v", err)
	}
	if got := summary.ByExchange["stub"]; got != 0 {
		t.Fatalf("allocator exposure after clear = %.2f, want 0", got)
	}
}

func TestManualClose_RejectsManualRecoveryWhileCleanupOrderIsLive(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	pos := &models.SpotFuturesPosition{
		ID:                     "manual-recovery-live-order",
		Symbol:                 "BTCUSDT",
		BaseCoin:               "BTC",
		Exchange:               "stub",
		Direction:              "borrow_sell_long",
		Status:                 models.SpotStatusPending,
		ExitReason:             spotEntryManualRecoveryReason,
		SpotSize:               0.6,
		BorrowAmount:           0.6,
		NotionalUSDT:           60,
		PendingSpotExitOrderID: "spot-cleanup-live",
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"stub": &closeTestSpotMargin{
			queryStates: []*exchange.SpotMarginOrderStatus{
				{
					OrderID:   "spot-cleanup-live",
					Symbol:    "BTCUSDT",
					Status:    "live",
					FilledQty: 0,
				},
			},
			marginBal: &exchange.MarginBalance{},
		},
	}

	err := engine.ManualClose(pos.ID)
	if err == nil {
		t.Fatal("expected ManualClose to reject active cleanup order")
	}
	if !strings.Contains(err.Error(), "still active") {
		t.Fatalf("ManualClose error = %v, want active cleanup order rejection", err)
	}

	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.Status != models.SpotStatusPending {
		t.Fatalf("status = %q, want %q", stored.Status, models.SpotStatusPending)
	}
}

func TestManualClose_RejectsManualRecoveryWhileBaseBalanceIsLocked(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	pos := &models.SpotFuturesPosition{
		ID:                     "manual-recovery-locked-balance",
		Symbol:                 "BTCUSDT",
		BaseCoin:               "BTC",
		Exchange:               "stub",
		Direction:              "borrow_sell_long",
		Status:                 models.SpotStatusPending,
		ExitReason:             spotEntryManualRecoveryReason,
		SpotSize:               0.6,
		BorrowAmount:           0.6,
		NotionalUSDT:           60,
		PendingSpotExitOrderID: "spot-cleanup-locked",
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}
	if err := engine.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}

	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"stub": &closeTestSpotMargin{
			queryStates: []*exchange.SpotMarginOrderStatus{
				{
					OrderID:   "spot-cleanup-locked",
					Symbol:    "BTCUSDT",
					Status:    "cancelled",
					FilledQty: 0.4,
				},
			},
			marginBal: &exchange.MarginBalance{
				TotalBalance: 0.2,
				Available:    0,
				Borrowed:     0,
				Interest:     0,
			},
		},
	}

	err := engine.ManualClose(pos.ID)
	if err == nil {
		t.Fatal("expected ManualClose to reject locked base balance")
	}
	if !strings.Contains(err.Error(), "manual recovery still open on exchange") {
		t.Fatalf("ManualClose error = %v, want locked balance rejection", err)
	}

	stored, err := engine.db.GetSpotPosition(pos.ID)
	if err != nil {
		t.Fatalf("GetSpotPosition: %v", err)
	}
	if stored.Status != models.SpotStatusPending {
		t.Fatalf("status = %q, want %q", stored.Status, models.SpotStatusPending)
	}
}

func TestManualOpen_RejectsFilteredOpportunity(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.latestOpps = []SpotArbOpportunity{
		{
			Symbol:       "BTCUSDT",
			BaseCoin:     "BTC",
			Exchange:     "stub",
			Direction:    "buy_spot_short",
			FilterStatus: "margin unavailable",
		},
	}

	err := engine.ManualOpen("BTCUSDT", "stub", "buy_spot_short")
	if err == nil {
		t.Fatal("expected filtered opportunity to be rejected")
	}
	if !strings.Contains(err.Error(), "is filtered") {
		t.Fatalf("ManualOpen error = %v, want filtered rejection", err)
	}
}
