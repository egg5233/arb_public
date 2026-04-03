package models

import "time"

// ArbitragePosition represents a live delta-neutral position across two exchanges.
type ArbitragePosition struct {
	ID               string    `json:"id"`
	Symbol           string    `json:"symbol"`
	LongExchange     string    `json:"long_exchange"`
	ShortExchange    string    `json:"short_exchange"`
	LongOrderID      string    `json:"long_order_id"`
	ShortOrderID     string    `json:"short_order_id"`
	LongSize         float64   `json:"long_size"`
	ShortSize        float64   `json:"short_size"`
	LongEntry        float64   `json:"long_entry"`
	ShortEntry       float64   `json:"short_entry"`
	LongExit         float64   `json:"long_exit"`
	ShortExit        float64   `json:"short_exit"`
	Status           string    `json:"status"` // pending, partial, active, exiting, closing, closed
	EntrySpread      float64   `json:"entry_spread"`
	FundingCollected float64   `json:"funding_collected"`
	RealizedPnL      float64   `json:"realized_pnl"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	NextFunding      time.Time `json:"next_funding"`
	CurrentSpread    float64   `json:"current_spread,omitempty"`    // live funding spread in bps/h
	RotationPnL      float64   `json:"rotation_pnl,omitempty"`      // accumulated PnL from closing rotated legs
	AllExchanges     []string  `json:"all_exchanges,omitempty"`     // all exchanges used (including rotated-away)
	LongSLOrderID    string    `json:"long_sl_order_id,omitempty"`  // stop-loss order ID on long exchange
	ShortSLOrderID   string    `json:"short_sl_order_id,omitempty"` // stop-loss order ID on short exchange
	LastRotatedFrom  string    `json:"last_rotated_from,omitempty"` // exchange we rotated away from
	LastRotatedAt    time.Time `json:"last_rotated_at,omitempty"`   // when last rotation happened
	RotationCount    int       `json:"rotation_count,omitempty"`    // total rotations for this position
	ReversalCount    int       `json:"reversal_count,omitempty"`    // spread reversal occurrences (for tolerance)
	ZeroSpreadCount  int       `json:"zero_spread_count,omitempty"` // consecutive zero-spread occurrences
	EntryFees        float64   `json:"entry_fees,omitempty"`        // total entry trading fees (both legs)
	ExitFees         float64   `json:"exit_fees,omitempty"`         // total exit trading fees (both legs)
	BasisGainLoss    float64   `json:"basis_gain_loss,omitempty"`   // price-based P/L excluding funding and fees
	Slippage         float64   `json:"slippage,omitempty"`          // estimated slippage from BBO at order time
	ExitReason         string           `json:"exit_reason,omitempty"`         // why the position was closed
	LongUnrealizedPnL  float64          `json:"long_unrealized_pnl,omitempty"`
	ShortUnrealizedPnL float64          `json:"short_unrealized_pnl,omitempty"`
	RotationHistory    []RotationRecord `json:"rotation_history,omitempty"`
}

// RotationRecord captures a single leg rotation event for the position timeline.
type RotationRecord struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	LegSide   string    `json:"leg_side"` // "long" or "short"
	PnL       *float64  `json:"pnl"`      // nil = not yet reconciled, 0 = real zero
	Timestamp time.Time `json:"timestamp"`
}

// PositionStatus constants.
const (
	StatusPending = "pending"
	StatusPartial = "partial"
	StatusActive  = "active"
	StatusExiting = "exiting"
	StatusClosing = "closing"
	StatusClosed  = "closed"
)
