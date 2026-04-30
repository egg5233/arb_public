// Package pricegaptrader — reconcile_record.go declares the typed schema for
// pg:reconcile:daily:{date} (Phase 14 PG-LIVE-03).
//
// Field declaration order is the JSON marshal order in encoding/json — DO NOT
// REORDER without re-running TestReconcile_Idempotency_ByteEqual at -count=100.
// All fields are typed structs/slices (no Go map types) so json.Marshal produces byte-identical
// output for byte-identical inputs (D-04 idempotency contract).
package pricegaptrader

import "time"

// reconcileSchemaVersion is the on-disk schema version stamped into every
// DailyReconcileRecord.SchemaVersion. Bump when adding/removing fields; an
// older daemon reading a newer record can detect the mismatch and skip
// (rather than corrupting the ramp signal with partial decode).
const reconcileSchemaVersion = 1

// DailyReconcileRecord — typed schema for pg:reconcile:daily:{date}.
//
// Field order is the JSON marshal order. D-04 idempotency: same inputs ->
// byte-identical Marshal output. T-14-06 mitigation: typed-struct only, no
// maps in any field; positions sorted by (ExchangeClosedAt, ID) ASC by the
// Reconciler before marshal; FlaggedIDs sorted ASC by the Reconciler before
// marshal.
type DailyReconcileRecord struct {
	SchemaVersion int                      `json:"schema_version"`
	Date          string                   `json:"date"`        // YYYY-MM-DD (UTC)
	ComputedAt    time.Time                `json:"computed_at"` // RFC3339, frozen via nowFn for tests
	Totals        DailyReconcileTotals     `json:"totals"`
	Positions     []DailyReconcilePosition `json:"positions"` // sorted by (ExchangeClosedAt, ID) ASC
	Anomalies     DailyReconcileAnomalies  `json:"anomalies"`
}

// DailyReconcileTotals — aggregate metrics for the day. NetClean is the
// upstream signal consumed by RampController in Plan 14-03 (D-05: realized
// PnL >= 0 AND positions_closed >= 1).
type DailyReconcileTotals struct {
	RealizedPnLUSDT float64 `json:"realized_pnl_usdt"`
	PositionsClosed int     `json:"positions_closed"`
	Wins            int     `json:"wins"`
	Losses          int     `json:"losses"`
	NetClean        bool    `json:"net_clean"`     // D-05: realized_pnl_usdt >= 0 AND positions_closed >= 1
	SkippedCount    int     `json:"skipped_count"` // T-14-11: positions failed to load — surfaced for ops review
}

// DailyReconcilePosition — per-position contribution to the day's record.
// FallbackLocalCloseAt is true when ExchangeClosedAt was zero on the source
// PriceGapPosition and the Reconciler used ClosedAt (local clock) instead per
// D-10. AnomalyMissingCloseTs mirrors that condition into the anomaly flags.
type DailyReconcilePosition struct {
	ID                    string    `json:"id"`
	Version               int       `json:"version"`
	ExchangeClosedAt      time.Time `json:"exchange_closed_at"`      // populated; falls back to ClosedAt when source is zero (D-10)
	FallbackLocalCloseAt  bool      `json:"fallback_local_close_at"` // true when ExchangeClosedAt was zero on source
	RealizedPnLUSDT       float64   `json:"realized_pnl_usdt"`
	RealizedSlippageBps   float64   `json:"realized_slippage_bps"`
	AnomalyHighSlippage   bool      `json:"anomaly_high_slippage"`
	AnomalyMissingCloseTs bool      `json:"anomaly_missing_close_ts"`
}

// DailyReconcileAnomalies — aggregate anomaly counters + flagged IDs. The
// FlaggedIDs slice is sorted ASC by the Reconciler before marshal for
// determinism (T-14-06). Initialized to []string{} (never nil) so the JSON
// output is "[]" rather than "null" — preserves byte-equality across
// no-anomaly days.
type DailyReconcileAnomalies struct {
	HighSlippageCount   int      `json:"high_slippage_count"`
	MissingCloseTsCount int      `json:"missing_close_ts_count"`
	FlaggedIDs          []string `json:"flagged_ids"` // sorted ASC for determinism
}
