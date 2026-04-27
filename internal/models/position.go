package models

import "time"

// ArbitragePosition represents a live delta-neutral position across two exchanges.
type ArbitragePosition struct {
	ID                 string           `json:"id"`
	Symbol             string           `json:"symbol"`
	LongExchange       string           `json:"long_exchange"`
	ShortExchange      string           `json:"short_exchange"`
	LongOrderID        string           `json:"long_order_id"`
	ShortOrderID       string           `json:"short_order_id"`
	LongSize           float64          `json:"long_size"`
	ShortSize          float64          `json:"short_size"`
	LongEntry          float64          `json:"long_entry"`
	ShortEntry         float64          `json:"short_entry"`
	LongExit           float64          `json:"long_exit"`
	ShortExit          float64          `json:"short_exit"`
	Status             string           `json:"status"` // pending, partial, active, exiting, closing, closed
	EntrySpread        float64          `json:"entry_spread"`
	FundingCollected   float64          `json:"funding_collected"`
	RealizedPnL        float64          `json:"realized_pnl"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	NextFunding        time.Time        `json:"next_funding"`
	CurrentSpread      float64          `json:"current_spread,omitempty"`    // live funding spread in bps/h
	RotationPnL        float64          `json:"rotation_pnl,omitempty"`      // accumulated PnL from closing rotated legs
	AllExchanges       []string         `json:"all_exchanges,omitempty"`     // all exchanges used (including rotated-away)
	LongSLOrderID      string           `json:"long_sl_order_id,omitempty"`  // stop-loss order ID on long exchange
	ShortSLOrderID     string           `json:"short_sl_order_id,omitempty"` // stop-loss order ID on short exchange
	LongTPOrderID      string           `json:"long_tp_order_id,omitempty"`  // take-profit order ID on long exchange
	ShortTPOrderID     string           `json:"short_tp_order_id,omitempty"` // take-profit order ID on short exchange
	LastRotatedFrom    string           `json:"last_rotated_from,omitempty"` // exchange we rotated away from
	LastRotatedAt      time.Time        `json:"last_rotated_at,omitempty"`   // when last rotation happened
	RotationCount      int              `json:"rotation_count,omitempty"`    // total rotations for this position
	ReversalCount      int              `json:"reversal_count,omitempty"`    // spread reversal occurrences (for tolerance)
	ZeroSpreadCount    int              `json:"zero_spread_count,omitempty"` // consecutive zero-spread occurrences
	EntryFees          float64          `json:"entry_fees,omitempty"`        // total entry trading fees (both legs)
	ExitFees           float64          `json:"exit_fees,omitempty"`         // total trading fees both legs open+close (from reconciliation)
	BasisGainLoss      float64          `json:"basis_gain_loss,omitempty"`   // price-based P/L excluding funding and fees
	Slippage           float64          `json:"slippage,omitempty"`          // estimated slippage from BBO at order time
	LongTotalFees      float64          `json:"long_total_fees,omitempty"`   // total trading fees for long leg (open+close)
	ShortTotalFees     float64          `json:"short_total_fees,omitempty"`  // total trading fees for short leg (open+close)
	LongFunding        float64          `json:"long_funding,omitempty"`      // funding collected on long leg
	ShortFunding       float64          `json:"short_funding,omitempty"`     // funding collected on short leg
	LongClosePnL       float64          `json:"long_close_pnl,omitempty"`    // price movement PnL on long leg (PricePnL)
	ShortClosePnL      float64          `json:"short_close_pnl,omitempty"`   // price movement PnL on short leg (PricePnL)
	EntryNotional      float64          `json:"entry_notional,omitempty"`    // max(long_entry*long_size, short_entry*short_size) at open
	ExitReason         string           `json:"exit_reason,omitempty"`       // why the position was closed
	FailureReason      string           `json:"failure_reason,omitempty"`    // why it failed (e.g. "circuit breaker", "insufficient balance", "depth timeout")
	FailureStage       string           `json:"failure_stage,omitempty"`     // at what stage (e.g. "depth_subscribe", "depth_fill", "order_placement")
	LongUnrealizedPnL  float64          `json:"long_unrealized_pnl,omitempty"`
	ShortUnrealizedPnL float64          `json:"short_unrealized_pnl,omitempty"`
	RotationHistory    []RotationRecord `json:"rotation_history,omitempty"`
	HasReconciled      bool             `json:"has_reconciled,omitempty"`    // true once PnL reconciliation has run (exit or consolidate)
	PartialReconcile   bool             `json:"partial_reconcile,omitempty"` // true when closed with incomplete PnL data

	// LongCloseSize / ShortCloseSize — current intended full close size.
	// Set at entry-fill completion. Updated on legitimate size-changing active paths
	// (partial-close revert, rotation partial, startup merge).
	// NOT modified by depth-exit zeroing. Used by reconcile completeness gate.
	LongCloseSize  float64 `json:"long_close_size,omitempty"`
	ShortCloseSize float64 `json:"short_close_size,omitempty"`
}

// InferHasReconciled back-fills HasReconciled for legacy positions that were
// closed before the field existed. Uses exit-reconciliation breakdown fields
// as evidence — any non-zero value proves reconciliation ran.
//
// Field safety notes:
//   - LongFunding/ShortFunding are per-leg EXIT breakdown fields, only set by
//     GetClosePnL during reconciliation. They are NOT the mid-life funding
//     accumulator (FundingCollected). Safe to use as evidence.
//   - FundingCollected is updated during position lifetime by the funding
//     tracker. It MUST NOT be used in this check — would cause false positives.
//   - EntryFees is set during entry. NOT used here — same reason.
//
// Ambiguous case: old positions where reconciliation returned all-zero diffs
// remain HasReconciled=false. They use EntryFees as fallback — same as before.
func (p *ArbitragePosition) InferHasReconciled() {
	if p.HasReconciled || p.PartialReconcile {
		return
	}
	if p.ExitFees != 0 ||
		p.LongTotalFees != 0 || p.ShortTotalFees != 0 ||
		p.BasisGainLoss != 0 ||
		p.LongFunding != 0 || p.ShortFunding != 0 ||
		p.LongClosePnL != 0 || p.ShortClosePnL != 0 {
		p.HasReconciled = true
	}
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
