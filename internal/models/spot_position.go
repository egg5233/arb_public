package models

import "time"

// SpotFuturesPosition represents a delta-neutral position with a spot margin
// leg and a futures leg on the same exchange.
type SpotFuturesPosition struct {
	ID        string `json:"id"`
	Symbol    string `json:"symbol"`
	BaseCoin  string `json:"base_coin"`
	Exchange  string `json:"exchange"`
	Direction string `json:"direction"` // "borrow_sell_long" or "buy_spot_short"
	Status    string `json:"status"`    // pending, active, exiting, closed

	// Spot leg
	SpotSize       float64 `json:"spot_size"`
	SpotEntryPrice float64 `json:"spot_entry_price"`
	SpotExitPrice  float64 `json:"spot_exit_price"`

	// Futures leg
	FuturesSize  float64 `json:"futures_size"`
	FuturesEntry float64 `json:"futures_entry"`
	FuturesExit  float64 `json:"futures_exit"`
	FuturesSide  string  `json:"futures_side"` // "long" or "short"
	HedgeIntact  bool    `json:"-"`
	HedgeBroken  bool    `json:"hedge_broken,omitempty"`

	// Borrow (Direction A only)
	BorrowAmount     float64 `json:"borrow_amount"`
	BorrowRateHourly float64 `json:"borrow_rate_hourly"`
	InterestPaid     float64 `json:"interest_paid"`

	// Monitor state (updated by monitorLoop)
	LastBorrowRateCheck time.Time  `json:"last_borrow_rate_check"`
	CurrentBorrowAPR    float64    `json:"current_borrow_apr"`
	BorrowCostAccrued   float64    `json:"borrow_cost_accrued"`
	NegativeYieldSince  *time.Time `json:"negative_yield_since,omitempty"`
	FundingAPR          float64    `json:"funding_apr"` // entry-time funding APR for yield comparison
	FeePct              float64    `json:"fee_pct"`     // entry-time one-time fee cost for reference

	// Live economics snapshot (updated by monitorLoop each tick)
	CurrentFundingAPR  float64    `json:"current_funding_apr"`
	CurrentFeePct      float64    `json:"current_fee_pct"`
	CurrentNetYieldAPR float64    `json:"current_net_yield_apr"`
	YieldDataSource    string     `json:"yield_data_source"` // "live_scan" or "entry_fallback"
	YieldSnapshotAt    *time.Time `json:"yield_snapshot_at,omitempty"`

	// P&L tracking
	FundingCollected float64 `json:"funding_collected"`
	EntryFees        float64 `json:"entry_fees"`
	ExitFees         float64 `json:"exit_fees"`
	RealizedPnL      float64 `json:"realized_pnl"`
	NotionalUSDT     float64 `json:"notional_usdt"`

	// Exit tracking
	ExitReason                 string     `json:"exit_reason,omitempty"`
	ExitTriggeredAt            *time.Time `json:"exit_triggered_at,omitempty"`
	ExitCompletedAt            *time.Time `json:"exit_completed_at,omitempty"`
	PeakPriceMovePct           float64    `json:"peak_price_move_pct"`
	MarginUtilizationPct       float64    `json:"margin_utilization_pct"`
	PendingEntryOrderID        string     `json:"pending_entry_order_id,omitempty"`         // accepted spot entry order awaiting confirmation / hedge completion
	PendingFuturesEntryOrderID string     `json:"pending_futures_entry_order_id,omitempty"` // accepted futures entry order awaiting confirmation after spot leg already confirmed (e.g. bitget client error on fill query)
	SpotExitFilledQty          float64    `json:"spot_exit_filled_qty,omitempty"`           // cumulative confirmed spot exit quantity across retries / partial IOC fills
	SpotExitFilled             bool       `json:"spot_exit_filled,omitempty"`               // true once the entire spot exit quantity is confirmed flat
	PendingSpotExitOrderID     string     `json:"pending_spot_exit_order_id,omitempty"`     // accepted spot exit order being reconciled; prevents duplicate close orders on retry
	PendingRepay               bool       `json:"pending_repay,omitempty"`                  // true when trade legs closed but margin repay still outstanding
	PendingRepayRetryAt        *time.Time `json:"pending_repay_retry_at,omitempty"`         // earliest time to retry repay (e.g. after Bybit blackout)
	ExitRetryCount             int        `json:"exit_retry_count,omitempty"`               // number of monitor-initiated exit retries

	// Timing
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SpotFutures position status constants.
const (
	SpotStatusPending = "pending"
	SpotStatusActive  = "active"
	SpotStatusExiting = "exiting"
	SpotStatusClosed  = "closed"
)

func (p *SpotFuturesPosition) SyncHedgeState() {
	if p == nil {
		return
	}
	p.HedgeIntact = !p.HedgeBroken
}

func (p *SpotFuturesPosition) MarkHedgeBroken() {
	if p == nil {
		return
	}
	p.HedgeBroken = true
	p.HedgeIntact = false
}
