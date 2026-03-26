package exchange

import "time"

// Side represents order side.
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// PlaceOrderParams contains parameters for placing an order.
type PlaceOrderParams struct {
	Symbol     string
	Side       Side
	OrderType  string // "limit" or "market"
	Price      string
	Size       string
	Force      string // "gtc", "ioc", "fok", "post_only"
	ReduceOnly bool
	ClientOid  string
}

// Order represents a pending order.
type Order struct {
	OrderID   string
	ClientOid string
	Symbol    string
	Side      string
	OrderType string
	Price     string
	Size      string
	Status    string
}

// Position represents an exchange position.
type Position struct {
	Symbol           string
	HoldSide         string // "long" or "short"
	Total            string
	Available        string
	AverageOpenPrice string
	UnrealizedPL     string
	Leverage         string
	MarginMode       string
	LiquidationPrice string // estimated liquidation price (empty if unavailable)
	MarkPrice        string // current mark price (empty if unavailable)
	FundingFee       string // accumulated funding fee for this position (empty if unavailable)
}

// ContractInfo holds contract specifications for a symbol.
type ContractInfo struct {
	Symbol        string
	MinSize       float64
	StepSize      float64
	MaxSize       float64
	SizeDecimals  int
	PriceStep     float64
	PriceDecimals int
}

// FundingRate holds the current funding rate from an exchange.
type FundingRate struct {
	Symbol      string
	Rate        float64       // current rate (raw, in the exchange's native format)
	NextRate    float64       // predicted next rate (if available)
	Interval    time.Duration // funding interval
	NextFunding time.Time     // next funding timestamp
	MaxRate     *float64      // upper cap, per-period decimal (e.g. 0.025 = 2.5%). nil if unavailable.
	MinRate     *float64      // lower cap, per-period decimal (e.g. -0.025 = -2.5%). nil if unavailable.
}

// Balance holds account balance info.
type Balance struct {
	Total       float64 // total equity
	Available   float64 // available for new orders
	Frozen      float64 // locked in positions/orders
	Currency    string  // "USDT"
	MarginRatio float64 // maintenanceMargin / equity; 0 = unknown, 1.0 = liquidation
}

// Orderbook represents order book depth.
type Orderbook struct {
	Symbol string
	Bids   []PriceLevel // sorted best (highest) first
	Asks   []PriceLevel // sorted best (lowest) first
	Time   time.Time
}

// PriceLevel is a single price level in the order book.
type PriceLevel struct {
	Price    float64
	Quantity float64
}

// BBO represents best bid and offer.
type BBO struct {
	Bid float64
	Ask float64
}

// OrderUpdate represents a real-time order status update from WebSocket.
type OrderUpdate struct {
	OrderID      string
	ClientOID    string
	Status       string
	FilledVolume float64
	AvgPrice     float64
}

// WithdrawParams contains parameters for a withdrawal request.
type WithdrawParams struct {
	Coin    string // "USDT"
	Chain   string // "BEP20" or "APT"
	Address string
	Amount  string
}

// WithdrawResult contains the result of a withdrawal request.
type WithdrawResult struct {
	TxID   string
	Fee    string
	Status string
}

// Trade represents a single fill from exchange trade history.
type Trade struct {
	TradeID  string
	OrderID  string
	Symbol   string
	Side     string  // "buy" or "sell"
	Price    float64
	Quantity float64
	Fee      float64 // always positive (cost)
	FeeCoin  string
	Time     time.Time
}

// FundingPayment represents a single funding fee payment from an exchange.
type FundingPayment struct {
	Amount float64
	Time   time.Time
}

// ClosePnL represents exchange-reported PnL for a closed position.
type ClosePnL struct {
	PricePnL   float64   // Raw price P/L (entry vs exit)
	Fees       float64   // Total trading fees (negative = cost)
	Funding    float64   // Total funding fees
	NetPnL     float64   // All-inclusive net PnL
	EntryPrice float64
	ExitPrice  float64
	CloseSize  float64
	Side       string    // normalized: "long" or "short"
	CloseTime  time.Time
}

// StopLossParams contains parameters for placing a stop-loss (conditional) order.
type StopLossParams struct {
	Symbol       string
	Side         Side   // sell for long SL, buy for short SL
	Size         string
	TriggerPrice string
}

// ExchangeConfig holds the configuration for connecting to an exchange.
type ExchangeConfig struct {
	Exchange   string
	ApiKey     string
	SecretKey  string
	Passphrase string // Required for Bitget, OKX
}

// PermStatus represents the tri-state result of a permission check.
type PermStatus string

const (
	PermGranted PermStatus = "granted"
	PermDenied  PermStatus = "denied"
	PermUnknown PermStatus = "unknown"
)

// PermissionResult holds API key permission check results for an exchange.
type PermissionResult struct {
	Read         PermStatus `json:"read"`
	FuturesTrade PermStatus `json:"futures_trade"`
	Withdraw     PermStatus `json:"withdraw"`
	Transfer     PermStatus `json:"transfer"`
	Method       string     `json:"method"` // "direct", "inferred", "unsupported"
	Error        string     `json:"error,omitempty"`
}

// PermissionChecker is an optional interface for exchanges that support
// API key permission introspection. Use type assertion to check.
type PermissionChecker interface {
	CheckPermissions() PermissionResult
}
