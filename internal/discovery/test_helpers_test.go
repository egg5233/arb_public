package discovery

import (
	"sync"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// stubExchange is a minimal exchange.Exchange implementation for testing.
// All methods are no-ops or return zero values.
type stubExchange struct{ name string }

func (s stubExchange) Name() string                                         { return s.name }
func (s stubExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) { return "", nil }
func (s stubExchange) CancelOrder(string, string) error                     { return nil }
func (s stubExchange) GetPendingOrders(string) ([]exchange.Order, error)    { return nil, nil }
func (s stubExchange) GetOrderFilledQty(string, string) (float64, error)    { return 0, nil }
func (s stubExchange) GetPosition(string) ([]exchange.Position, error)      { return nil, nil }
func (s stubExchange) GetAllPositions() ([]exchange.Position, error)        { return nil, nil }
func (s stubExchange) SetLeverage(string, string, string) error             { return nil }
func (s stubExchange) SetMarginMode(string, string) error                   { return nil }
func (s stubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return nil, nil
}
func (s stubExchange) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s stubExchange) GetFundingInterval(string) (time.Duration, error)     { return 0, nil }
func (s stubExchange) GetFuturesBalance() (*exchange.Balance, error)        { return nil, nil }
func (s stubExchange) GetSpotBalance() (*exchange.Balance, error)           { return nil, nil }
func (s stubExchange) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s stubExchange) TransferToSpot(string, string) error                   { return nil }
func (s stubExchange) TransferToFutures(string, string) error                { return nil }
func (s stubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) { return nil, nil }
func (s stubExchange) StartPriceStream([]string)                             {}
func (s stubExchange) SubscribeSymbol(string) bool                           { return false }
func (s stubExchange) GetBBO(string) (exchange.BBO, bool)                    { return exchange.BBO{}, false }
func (s stubExchange) GetPriceStore() *sync.Map                              { return nil }
func (s stubExchange) SubscribeDepth(string) bool                              { return false }
func (s stubExchange) UnsubscribeDepth(string) bool                            { return false }
func (s stubExchange) GetDepth(string) (*exchange.Orderbook, bool)             { return nil, false }
func (s stubExchange) StartPrivateStream()                                     {}
func (s stubExchange) GetOrderUpdate(string) (exchange.OrderUpdate, bool) {
	return exchange.OrderUpdate{}, false
}
func (s stubExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s stubExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) { return "", nil }
func (s stubExchange) CancelStopLoss(string, string) error                   { return nil }
func (s stubExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}

// makeNilExchangeMap builds a map[string]exchange.Exchange with stub implementations.
func makeNilExchangeMap(names ...string) map[string]exchange.Exchange {
	m := make(map[string]exchange.Exchange, len(names))
	for _, n := range names {
		m[n] = stubExchange{name: n}
	}
	return m
}

// newTestLogger returns a logger suitable for tests.
func newTestLogger() *utils.Logger {
	return utils.NewLogger("test")
}
