package pricegaptrader

import (
	"sync"
	"time"

	"arb/pkg/exchange"
)

// stubExchange is a drop-in fake for the full pkg/exchange.Exchange interface.
// Most methods return zero values; the "live" methods GetBBO, PlaceOrder,
// GetOrderFilledQty, GetPosition, LoadAllContracts, Name carry real bodies
// to support later plans' tests.
type stubExchange struct {
	name string

	mu   sync.Mutex
	bbos map[string]*exchange.BBO

	placed        []exchange.PlaceOrderParams
	placeOrderErr error
	placeOrderFn  func(p exchange.PlaceOrderParams) (string, error)

	filledQty  map[string]float64 // orderID -> filled qty
	positions  map[string][]exchange.Position
	contracts  map[string]exchange.ContractInfo
	priceStore sync.Map
}

// Compile-time check — if pkg/exchange adds a method, this line breaks the build
// here instead of failing at test-run-time.
var _ exchange.Exchange = (*stubExchange)(nil)

func newStubExchange(name string) *stubExchange {
	return &stubExchange{
		name:      name,
		bbos:      make(map[string]*exchange.BBO),
		filledQty: make(map[string]float64),
		positions: make(map[string][]exchange.Position),
		contracts: make(map[string]exchange.ContractInfo),
	}
}

// setBBO mutates what GetBBO returns for the given symbol. The ts arg is retained
// so tests can reason about sample timing, though the BBO struct itself carries
// no timestamp — the detector measures staleness via tick wall-clock in the Tracker.
func (s *stubExchange) setBBO(symbol string, bid, ask float64, _ time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bbos[symbol] = &exchange.BBO{Bid: bid, Ask: ask}
}

// ---- Live methods (real behaviour) ------------------------------------------

func (s *stubExchange) Name() string { return s.name }

func (s *stubExchange) GetBBO(symbol string) (exchange.BBO, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bbos[symbol]
	if !ok {
		return exchange.BBO{}, false
	}
	return *b, true
}

func (s *stubExchange) PlaceOrder(p exchange.PlaceOrderParams) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.placed = append(s.placed, p)
	if s.placeOrderFn != nil {
		// Release the lock before calling user fn to avoid deadlocks if the fn
		// re-enters stubExchange.
		s.mu.Unlock()
		id, err := s.placeOrderFn(p)
		s.mu.Lock()
		return id, err
	}
	if s.placeOrderErr != nil {
		return "", s.placeOrderErr
	}
	return "stub-order-id", nil
}

func (s *stubExchange) GetOrderFilledQty(orderID, _ string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filledQty[orderID], nil
}

func (s *stubExchange) GetPosition(symbol string) ([]exchange.Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.positions[symbol], nil
}

func (s *stubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contracts, nil
}

// ---- Zero-value stubs -------------------------------------------------------

func (s *stubExchange) SetMetricsCallback(fn exchange.MetricsCallback)              {}
func (s *stubExchange) CancelOrder(symbol, orderID string) error                    { return nil }
func (s *stubExchange) GetPendingOrders(symbol string) ([]exchange.Order, error)    { return nil, nil }
func (s *stubExchange) GetAllPositions() ([]exchange.Position, error)               { return nil, nil }
func (s *stubExchange) SetLeverage(symbol, leverage, holdSide string) error         { return nil }
func (s *stubExchange) SetMarginMode(symbol, mode string) error                     { return nil }
func (s *stubExchange) GetFundingRate(symbol string) (*exchange.FundingRate, error) { return nil, nil }
func (s *stubExchange) GetFundingInterval(symbol string) (time.Duration, error)     { return 0, nil }
func (s *stubExchange) GetFuturesBalance() (*exchange.Balance, error)               { return &exchange.Balance{}, nil }
func (s *stubExchange) GetSpotBalance() (*exchange.Balance, error)                  { return &exchange.Balance{}, nil }
func (s *stubExchange) Withdraw(p exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s *stubExchange) WithdrawFeeInclusive() bool { return false }
func (s *stubExchange) GetWithdrawFee(coin, chain string) (float64, float64, error) {
	return 0, 0, nil
}
func (s *stubExchange) TransferToSpot(coin, amount string) error    { return nil }
func (s *stubExchange) TransferToFutures(coin, amount string) error { return nil }
func (s *stubExchange) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	return nil, nil
}
func (s *stubExchange) StartPriceStream(symbols []string)    {}
func (s *stubExchange) SubscribeSymbol(symbol string) bool   { return true }
func (s *stubExchange) GetPriceStore() *sync.Map             { return &s.priceStore }
func (s *stubExchange) SubscribeDepth(symbol string) bool    { return true }
func (s *stubExchange) UnsubscribeDepth(symbol string) bool  { return true }
func (s *stubExchange) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	return nil, false
}
func (s *stubExchange) StartPrivateStream()                                       {}
func (s *stubExchange) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	return exchange.OrderUpdate{}, false
}
func (s *stubExchange) SetOrderCallback(fn func(exchange.OrderUpdate))                {}
func (s *stubExchange) PlaceStopLoss(p exchange.StopLossParams) (string, error)       { return "", nil }
func (s *stubExchange) CancelStopLoss(symbol, orderID string) error                   { return nil }
func (s *stubExchange) PlaceTakeProfit(p exchange.TakeProfitParams) (string, error)   { return "", nil }
func (s *stubExchange) CancelTakeProfit(symbol, orderID string) error                 { return nil }
func (s *stubExchange) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *stubExchange) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *stubExchange) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *stubExchange) EnsureOneWayMode() error            { return nil }
func (s *stubExchange) CancelAllOrders(symbol string) error { return nil }
func (s *stubExchange) Close()                              {}

// ---- fakeClock --------------------------------------------------------------

type fakeClock struct{ t time.Time }

func newFakeClock(start time.Time) *fakeClock       { return &fakeClock{t: start} }
func (c *fakeClock) Now() time.Time                 { return c.t }
func (c *fakeClock) Advance(d time.Duration)        { c.t = c.t.Add(d) }
