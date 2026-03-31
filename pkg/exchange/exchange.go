package exchange

import (
	"sync"
	"time"
)

// Exchange is the unified interface that all exchange adapters must implement.
type Exchange interface {
	// Identity
	Name() string
	SetMetricsCallback(fn MetricsCallback)

	// Orders
	PlaceOrder(req PlaceOrderParams) (orderID string, err error)
	CancelOrder(symbol, orderID string) error
	GetPendingOrders(symbol string) ([]Order, error)
	GetOrderFilledQty(orderID, symbol string) (float64, error)

	// Positions
	GetPosition(symbol string) ([]Position, error)
	GetAllPositions() ([]Position, error)

	// Account Config
	SetLeverage(symbol string, leverage string, holdSide string) error
	SetMarginMode(symbol string, mode string) error

	// Contract Info
	LoadAllContracts() (map[string]ContractInfo, error)

	// Funding Rate
	GetFundingRate(symbol string) (*FundingRate, error)
	GetFundingInterval(symbol string) (time.Duration, error)

	// Account — Balance
	// GetFuturesBalance returns the futures/trading account balance (used by the engine).
	GetFuturesBalance() (*Balance, error)
	// GetSpotBalance returns the spot/funding account balance (withdrawable funds).
	GetSpotBalance() (*Balance, error)
	// Withdraw & Transfer
	Withdraw(params WithdrawParams) (*WithdrawResult, error)
	// TransferToSpot moves funds from the trading/futures account to the
	// spot/funding account so they can be withdrawn. No-op on exchanges
	// where withdrawals already come from the main balance (e.g. Binance, Gate.io).
	TransferToSpot(coin string, amount string) error
	// TransferToFutures moves funds from spot/funding to the futures/trading
	// account. No-op on unified-account exchanges (OKX, Bybit).
	TransferToFutures(coin string, amount string) error

	// Orderbook
	GetOrderbook(symbol string, depth int) (*Orderbook, error)

	// WebSocket: Prices
	StartPriceStream(symbols []string)
	SubscribeSymbol(symbol string) bool
	GetBBO(symbol string) (BBO, bool)
	GetPriceStore() *sync.Map

	// WebSocket: Depth (top-5 orderbook)
	SubscribeDepth(symbol string) bool
	UnsubscribeDepth(symbol string) bool
	GetDepth(symbol string) (*Orderbook, bool)

	// WebSocket: Private
	StartPrivateStream()
	GetOrderUpdate(orderID string) (OrderUpdate, bool)
	SetOrderCallback(fn func(OrderUpdate))

	// Stop-Loss (conditional orders)
	PlaceStopLoss(params StopLossParams) (orderID string, err error)
	CancelStopLoss(symbol, orderID string) error

	// Trade History
	GetUserTrades(symbol string, startTime time.Time, limit int) ([]Trade, error)

	// Funding Fee History
	GetFundingFees(symbol string, since time.Time) ([]FundingPayment, error)

	// Close PnL — exchange-reported position-level PnL after close
	GetClosePnL(symbol string, since time.Time) ([]ClosePnL, error)

	// Account Setup — ensure cross margin + one-way position mode
	EnsureOneWayMode() error

	// Close terminates all WebSocket connections for graceful shutdown.
	Close()
}
