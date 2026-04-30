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

// FiredDirection enum (PG-DIR-01) — exact string values for Redis persistence.
// "forward" = configured-direction sign crossed (long_exch cheaper than short_exch).
// "inverse" = opposite-direction sign crossed; executor swapped wire-side leg roles.
const (
	PriceGapFiredForward = "forward"
	PriceGapFiredInverse = "inverse"
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
	ID            string `json:"id"`
	Symbol        string `json:"symbol"`
	LongExchange  string `json:"long_exchange"`  // wire-side role (may swap from configured for inverse fires — PG-DIR-01)
	ShortExchange string `json:"short_exchange"` // wire-side role (may swap from configured for inverse fires — PG-DIR-01)

	// PG-DIR-01: bidirectional fire observability.
	// FiredDirection: "forward" (configured-direction crossed) | "inverse" (opposite sign crossed).
	// CandidateLongExch / CandidateShortExch: the CONFIGURED tuple at fire time —
	// distinct from LongExchange / ShortExchange which are the WIRE-SIDE roles
	// (which differ from the configured tuple when FiredDirection == "inverse").
	// Required for Phase 10 D-11 active-position guard tuple matching (Pitfall 5).
	// Empty on pre-Phase-999.1 records — close path never branches on these
	// (it reads LongExchange/ShortExchange face-value), so legacy decode is safe.
	FiredDirection     string `json:"fired_direction,omitempty"`
	CandidateLongExch  string `json:"candidate_long_exch,omitempty"`
	CandidateShortExch string `json:"candidate_short_exch,omitempty"`

	Status string `json:"status"` // "pending" | "open" | "exiting" | "closed"
	// Mode string `json:"mode"` — Phase 9 D-12: "paper" | "live". Defaults to
	// "live" on unmarshal of pre-Phase-9 records via NormalizeMode.
	Mode string `json:"mode"`

	EntrySpreadBps float64 `json:"entry_spread_bps"`
	ThresholdBps   float64 `json:"threshold_bps"`
	NotionalUSDT   float64 `json:"notional_usdt"`

	LongFillPrice  float64 `json:"long_fill_price"`
	ShortFillPrice float64 `json:"short_fill_price"`
	LongSize       float64 `json:"long_size"`
	ShortSize      float64 `json:"short_size"`

	LongMidAtDecision  float64 `json:"long_mid_at_decision"`
	ShortMidAtDecision float64 `json:"short_mid_at_decision"`

	// D-23 + D-24 (Phase 9): ModeledSlipBps is stamped at entry by openPair
	// (internal/pricegaptrader/execution.go) from cand.ModeledSlippageBps;
	// RealizedSlipBps is stamped at close by closePair
	// (internal/pricegaptrader/monitor.go) as the round-trip realized bps.
	// Both values persist onto every pg:history row, which unblocks Plan 05
	// rolling metrics to read realized-vs-modeled directly from pg:history —
	// no dependency on pg:slippage:* per D-24.
	ModeledSlipBps  float64 `json:"modeled_slippage_bps"`
	RealizedSlipBps float64 `json:"realized_slippage_bps"`
	RealizedPnL     float64 `json:"realized_pnl"`

	ExitReason string    `json:"exit_reason,omitempty"`
	OpenedAt   time.Time `json:"opened_at"`
	ClosedAt   time.Time `json:"closed_at,omitempty"`

	// Phase 14 PG-LIVE-03: Reconcile aggregation key is (position_id, version).
	// 0 on legacy/pre-Phase-14 records (omitted via omitempty); 1 on new closes;
	// increment on amended closes (e.g., late-fill correction).
	Version int `json:"version,omitempty"`

	// Phase 14 D-10: Exchange-side close timestamp. Falls back to ClosedAt
	// (local clock) per D-10 when zero — reconciler flags anomaly_missing_close_ts.
	ExchangeClosedAt time.Time `json:"exchange_closed_at,omitempty"`
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
