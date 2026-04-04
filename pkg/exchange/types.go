package exchange

import "time"

// MetricsCallback records REST endpoint latency and error outcomes.
type MetricsCallback func(endpoint string, latency time.Duration, err error)

// WSEventType describes an exchange WebSocket lifecycle event.
type WSEventType string

const (
	WSEventConnect    WSEventType = "connect"
	WSEventDisconnect WSEventType = "disconnect"
	WSEventMessage    WSEventType = "message"
)

// WSEvent is emitted by exchange WebSocket clients for health scoring.
type WSEvent struct {
	Type      WSEventType
	Timestamp time.Time
}

// WSMetricsCallback consumes exchange WebSocket lifecycle events.
type WSMetricsCallback func(WSEvent)

// WSMetricsCallbackSetter is implemented by adapters that expose public WS metrics.
type WSMetricsCallbackSetter interface {
	SetWSMetricsCallback(fn WSMetricsCallback)
}

// OrderMetricEventType describes an order lifecycle event used for fill-rate scoring.
type OrderMetricEventType string

const (
	OrderMetricPlaced OrderMetricEventType = "placed"
	OrderMetricFilled OrderMetricEventType = "filled"
)

// OrderMetricEvent is emitted for order placement/fill tracking.
type OrderMetricEvent struct {
	Type      OrderMetricEventType
	OrderID   string
	FilledQty float64
	Timestamp time.Time
}

// OrderMetricsCallback consumes order lifecycle events for health scoring.
type OrderMetricsCallback func(OrderMetricEvent)

// OrderMetricsCallbackSetter is implemented by adapters that expose order metrics.
type OrderMetricsCallbackSetter interface {
	SetOrderMetricsCallback(fn OrderMetricsCallback)
}

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
	Total          float64 // total equity
	Available      float64 // available for new orders
	Frozen         float64 // locked in positions/orders
	Currency       string  // "USDT"
	MarginRatio    float64 // maintenanceMargin / equity; 0 = unknown, 1.0 = liquidation
	MaxTransferOut float64 // max amount that can be transferred out; 0 = unknown (use Available as fallback)
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
	Symbol       string // normalized symbol (e.g. "BTCUSDT"); empty if unknown
	ReduceOnly   bool   // true if this is a reduce-only / close fill (SL, TP, liquidation)
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
	Side     string // "buy" or "sell"
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
	PricePnL   float64 // Raw price P/L (entry vs exit)
	Fees       float64 // Total trading fees (negative = cost)
	Funding    float64 // Total funding fees
	NetPnL     float64 // All-inclusive net PnL
	EntryPrice float64
	ExitPrice  float64
	CloseSize  float64
	Side       string // normalized: "long" or "short"
	CloseTime  time.Time
}

// StopLossParams contains parameters for placing a stop-loss (conditional) order.
type StopLossParams struct {
	Symbol       string
	Side         Side // sell for long SL, buy for short SL
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

// ---------------------------------------------------------------------------
// Spot Margin (borrow-and-sell) types
// ---------------------------------------------------------------------------

// MarginBorrowParams contains parameters for borrowing a coin on spot margin.
type MarginBorrowParams struct {
	Coin   string // e.g. "BTC"
	Amount string // quantity to borrow
}

// MarginRepayParams contains parameters for repaying a borrowed coin.
type MarginRepayParams struct {
	Coin   string // e.g. "BTC"
	Amount string // quantity to repay (include interest)
}

// ErrRepayBlackout is returned when a repay attempt is blocked by a
// scheduled exchange maintenance/blackout window. RetryAfter indicates
// the earliest time the caller should retry.
type ErrRepayBlackout struct {
	RetryAfter time.Time
	Message    string
}

func (e *ErrRepayBlackout) Error() string { return e.Message }

// SpotMarginOrderParams contains parameters for placing a spot margin order.
type SpotMarginOrderParams struct {
	Symbol     string // e.g. "BTCUSDT"
	Side       Side   // buy or sell
	OrderType  string // "limit" or "market"
	Price      string // required for limit orders
	Size       string // quantity in base coin (required for limit + market sell)
	QuoteSize  string // quantity in quote coin / USDT (required for market buy on some exchanges)
	Force      string // "gtc", "ioc", "fok"
	AutoBorrow bool   // if true, exchange auto-borrows on sell
	AutoRepay  bool   // if true, exchange auto-repays on buy
	ClientOid  string
}

// MarginInterestRate holds the borrow interest rate for a coin.
type MarginInterestRate struct {
	Coin       string
	HourlyRate float64 // per-hour rate as decimal (e.g. 0.000005)
	DailyRate  float64 // per-day rate as decimal (if available)
}

// MarginBalance holds spot margin account info for a coin.
type MarginBalance struct {
	Coin          string
	TotalBalance  float64 // total holdings
	Available     float64 // available (not frozen)
	Borrowed      float64 // outstanding loan principal
	Interest      float64 // accrued interest
	NetBalance    float64 // total - borrowed - interest
	MaxBorrowable float64 // maximum additional borrowable
}

// SpotMarginOrderStatus is a normalized view of a spot margin order state.
type SpotMarginOrderStatus struct {
	OrderID   string
	Symbol    string
	Status    string
	FilledQty float64
	AvgPrice  float64
	// Fee deducted from the received asset (e.g., BTC commission on a BTC buy).
	// When > 0, actual received = FilledQty - FeeDeducted.
	FeeDeducted float64
}

// SpotMarginExchange is an optional interface for exchanges that support
// spot margin borrowing (borrow-sell-buyback-repay). Use type assertion to check.
// BingX does not implement this interface.
type SpotMarginExchange interface {
	// MarginBorrow borrows a coin on spot margin.
	MarginBorrow(params MarginBorrowParams) error

	// MarginRepay repays a borrowed coin (amount should include accrued interest).
	MarginRepay(params MarginRepayParams) error

	// PlaceSpotMarginOrder places a buy or sell order on spot margin.
	PlaceSpotMarginOrder(params SpotMarginOrderParams) (orderID string, err error)

	// GetSpotBBO returns the current best bid/offer for the spot market.
	GetSpotBBO(symbol string) (BBO, error)

	// GetMarginInterestRate returns the current borrow interest rate for a coin.
	GetMarginInterestRate(coin string) (*MarginInterestRate, error)

	// GetMarginBalance returns spot margin account info for a coin.
	GetMarginBalance(coin string) (*MarginBalance, error)

	// TransferToMargin moves funds from the main/futures account to the margin account.
	// Used by Dir A (borrow-sell-long) on separate-account exchanges.
	TransferToMargin(coin string, amount string) error

	// TransferFromMargin moves funds from the margin account back to main/futures.
	TransferFromMargin(coin string, amount string) error
}

// SpotMarginOrderQuerier is an optional interface for exchanges that can
// query native spot margin order state. The spot-futures exit flow uses this
// to reconcile accepted spot close orders without routing through futures
// order endpoints.
type SpotMarginOrderQuerier interface {
	GetSpotMarginOrder(orderID, symbol string) (*SpotMarginOrderStatus, error)
}

// TradingFee holds the authenticated user's maker/taker fee rates.
type TradingFee struct {
	MakerRate float64 // decimal fraction, e.g. 0.0002 = 0.02%
	TakerRate float64 // decimal fraction, e.g. 0.0004 = 0.04%
}

// TradingFeeProvider is optionally implemented by exchanges that support
// querying the authenticated user's trading fee tier.
type TradingFeeProvider interface {
	// GetTradingFee returns the current user's maker/taker fee rates.
	// Rates are decimal fractions (e.g. 0.0004 = 0.04%).
	GetTradingFee() (*TradingFee, error)
}

// FlashRepayer is an optional interface for exchanges that support flash-repay
// (exchange converts collateral to repay borrow in one step, no buy order needed).
// Used by Dir A close to skip the spot buyback entirely.
type FlashRepayer interface {
	FlashRepay(coin string) (repayID string, err error)
}
