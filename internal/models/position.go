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
