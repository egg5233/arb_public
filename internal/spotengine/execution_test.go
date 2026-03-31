package spotengine

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

type closeTestExchange struct {
	orderUpdates     map[string]exchange.OrderUpdate
	orderbook        *exchange.Orderbook
	orderbookEntered chan struct{}
	orderbookRelease <-chan struct{}
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
func (s *closeTestExchange) GetPosition(string) ([]exchange.Position, error)   { return nil, nil }
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
func (s *closeTestExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) {
	return "", nil
}
func (s *closeTestExchange) CancelStopLoss(string, string) error { return nil }
func (s *closeTestExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *closeTestExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *closeTestExchange) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *closeTestExchange) EnsureOneWayMode() error { return nil }
func (s *closeTestExchange) Close()                  {}

type closeTestSpotMargin struct {
	placeCalls  int
	queryCalls  int
	queryErrs   []error
	queryStates []*exchange.SpotMarginOrderStatus
}

func (s *closeTestSpotMargin) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s *closeTestSpotMargin) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (s *closeTestSpotMargin) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	s.placeCalls++
	return "spot-close-1", nil
}
func (s *closeTestSpotMargin) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return nil, nil
}
func (s *closeTestSpotMargin) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return nil, nil
}
func (s *closeTestSpotMargin) TransferToMargin(string, string) error   { return nil }
func (s *closeTestSpotMargin) TransferFromMargin(string, string) error { return nil }
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
		cfg: &config.Config{},
		db:  db,
		log: utils.NewLogger("test"),
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

func TestManualOpen_RejectsConcurrentEntry(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{
		SpotFuturesMaxPositions:       1,
		SpotFuturesLeverage:           3,
		SpotFuturesUnifiedAcctMaxUSDT: 100,
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
