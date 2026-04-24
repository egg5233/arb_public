package pricegaptrader

import (
	"fmt"
	"sync"
	"time"

	"arb/pkg/exchange"
)

// fillScript — scripted response tuple pushed by tests to drive PlaceOrder +
// GetOrderFilledQty. On a queued script: err != nil makes PlaceOrder return
// the error (no orderID); err == nil makes PlaceOrder mint a sequential
// orderID and GetOrderFilledQty later returns the scripted (filled, vwap).
type fillScript struct {
	filled float64
	vwap   float64
	err    error
}

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

	// Plan 05: scripted fills — tests enqueue via queueFill; PlaceOrder pops
	// the next script. Per-order fill recorded under orderIdFill for
	// GetOrderFilledQty lookup (keyed by minted orderID).
	scripts        []fillScript
	orderIDCounter int
	orderIDFill    map[string]fillScript

	filledQty  map[string]float64 // orderID -> filled qty (legacy direct-set path)
	positions  map[string][]exchange.Position
	contracts  map[string]exchange.ContractInfo
	priceStore sync.Map

	// Plan 09-10 (Gap #2): SubscribeSymbol call tracking. subs counts per-symbol
	// calls; subscribeFn, if non-nil, overrides the default `return true`.
	subs         map[string]int
	subscribeFn  func(symbol string) bool
}

// Compile-time check — if pkg/exchange adds a method, this line breaks the build
// here instead of failing at test-run-time.
var _ exchange.Exchange = (*stubExchange)(nil)

func newStubExchange(name string) *stubExchange {
	return &stubExchange{
		name:        name,
		bbos:        make(map[string]*exchange.BBO),
		filledQty:   make(map[string]float64),
		orderIDFill: make(map[string]fillScript),
		positions:   make(map[string][]exchange.Position),
		contracts:   make(map[string]exchange.ContractInfo),
		subs:        make(map[string]int),
	}
}

// queueFill enqueues a scripted PlaceOrder+GetOrderFilledQty response.
// Scripts are consumed FIFO. If err is non-nil, PlaceOrder returns it (no
// orderID minted); otherwise PlaceOrder mints a sequential orderID and
// GetOrderFilledQty returns (filled, nil) for that orderID.
func (s *stubExchange) queueFill(filled, vwap float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts = append(s.scripts, fillScript{filled: filled, vwap: vwap, err: err})
}

// placedOrders returns a copy of the orders recorded by PlaceOrder.
func (s *stubExchange) placedOrders() []exchange.PlaceOrderParams {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]exchange.PlaceOrderParams, len(s.placed))
	copy(out, s.placed)
	return out
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
	s.placed = append(s.placed, p)
	if s.placeOrderFn != nil {
		// Release the lock before calling user fn to avoid deadlocks if the fn
		// re-enters stubExchange.
		s.mu.Unlock()
		id, err := s.placeOrderFn(p)
		return id, err
	}
	// Plan 05 scripted path — consume the next fillScript if queued.
	if len(s.scripts) > 0 {
		sc := s.scripts[0]
		s.scripts = s.scripts[1:]
		if sc.err != nil {
			s.mu.Unlock()
			return "", sc.err
		}
		s.orderIDCounter++
		oid := fmt.Sprintf("%s-ord-%d", s.name, s.orderIDCounter)
		s.orderIDFill[oid] = sc
		s.mu.Unlock()
		return oid, nil
	}
	if s.placeOrderErr != nil {
		s.mu.Unlock()
		return "", s.placeOrderErr
	}
	s.mu.Unlock()
	return "stub-order-id", nil
}

func (s *stubExchange) GetOrderFilledQty(orderID, _ string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Scripted path takes priority when the orderID was minted by queueFill.
	if sc, ok := s.orderIDFill[orderID]; ok {
		return sc.filled, nil
	}
	return s.filledQty[orderID], nil
}

// GetOrderVwap — test-only optional interface hit (pricegaptrader.vwapReader).
// Returns the scripted vwap for orderIDs minted by queueFill. Production
// adapters do NOT implement this; only the stub does, to drive exact exit
// PnL math in monitor_test.go.
func (s *stubExchange) GetOrderVwap(orderID string) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc, ok := s.orderIDFill[orderID]; ok {
		return sc.vwap, true
	}
	return 0, false
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
func (s *stubExchange) SubscribeSymbol(symbol string) bool {
	s.mu.Lock()
	s.subs[symbol]++
	fn := s.subscribeFn
	s.mu.Unlock()
	if fn != nil {
		return fn(symbol)
	}
	return true
}
func (s *stubExchange) subscribeCount(symbol string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subs[symbol]
}
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
