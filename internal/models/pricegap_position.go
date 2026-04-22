package models

import "time"

// Price-gap tracker status constants.
const (
	PriceGapStatusPending = "pending"
	PriceGapStatusOpen    = "open"
	PriceGapStatusExiting = "exiting"
	PriceGapStatusClosed  = "closed"
)

// ExitReason enum (D-13) — exact string values for Redis persistence.
const (
	ExitReasonReverted    = "reverted"
	ExitReasonMaxHold     = "max_hold"
	ExitReasonManual      = "manual"
	ExitReasonRiskGate    = "risk_gate"
	ExitReasonExecQuality = "exec_quality"
	ExitReasonOrphan      = "recovered_orphan" // rehydration pitfall #3 per RESEARCH.md
)

// Price-gap mode constants (Phase 9, D-12).
// Default on unmarshal of pre-Phase-9 records is "live" — see NormalizeMode.
const (
	PriceGapModeLive  = "live"
	PriceGapModePaper = "paper"
)

// PriceGapPosition — per D-14 persisted under pg:pos:{id}, D-15 ID format.
// Delta-neutral cross-exchange price-gap position (Strategy 4).
type PriceGapPosition struct {
	ID             string `json:"id"`
	Symbol         string `json:"symbol"`
	LongExchange   string `json:"long_exchange"`
	ShortExchange  string `json:"short_exchange"`
	Status         string `json:"status"` // "pending" | "open" | "exiting" | "closed"
	Mode           string `json:"mode"`   // "paper" | "live" (Phase 9 D-12); defaults to "live" on unmarshal of pre-Phase-9 records via NormalizeMode.

	EntrySpreadBps float64 `json:"entry_spread_bps"`
	ThresholdBps   float64 `json:"threshold_bps"`
	NotionalUSDT   float64 `json:"notional_usdt"`

	LongFillPrice  float64 `json:"long_fill_price"`
	ShortFillPrice float64 `json:"short_fill_price"`
	LongSize       float64 `json:"long_size"`
	ShortSize      float64 `json:"short_size"`

	LongMidAtDecision  float64 `json:"long_mid_at_decision"`
	ShortMidAtDecision float64 `json:"short_mid_at_decision"`

	ModeledSlipBps  float64 `json:"modeled_slippage_bps"`
	RealizedSlipBps float64 `json:"realized_slippage_bps"`
	RealizedPnL     float64 `json:"realized_pnl"`

	ExitReason string    `json:"exit_reason,omitempty"`
	OpenedAt   time.Time `json:"opened_at"`
	ClosedAt   time.Time `json:"closed_at,omitempty"`
}

// NormalizeMode defaults an empty Mode field to "live" so records persisted
// before Phase 9 (which did not carry a Mode key) continue to route through the
// live-trading path. Intentionally a free function (not UnmarshalJSON) so that
// test fixtures constructing PriceGapPosition literals without Mode still work
// unchanged; callers reading from Redis apply this at decode time.
func NormalizeMode(p *PriceGapPosition) {
	if p == nil {
		return
	}
	if p.Mode == "" {
		p.Mode = PriceGapModeLive
	}
}

// SlippageSample is one observation of modeled-vs-realized slippage in bps.
// Used by the rolling window (PG-RISK-03) to trigger exec-quality disable.
type SlippageSample struct {
	PositionID string    `json:"position_id"`
	Realized   float64   `json:"realized_bps"`
	Modeled    float64   `json:"modeled_bps"`
	Timestamp  time.Time `json:"ts"`
}
