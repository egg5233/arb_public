package models

// Phase 15 (PG-LIVE-02) — Drawdown Circuit Breaker model types.
//
// BreakerState is persisted to the Redis HASH `pg:breaker:state` (D-05).
// BreakerTripRecord entries are persisted to the capped LIST `pg:breaker:trips`
// (D-18, capped at 500). All field names + JSON tags are locked by Phase 15
// CONTEXT — downstream plans consume these struct shapes verbatim.

// BreakerState — 5-field Redis HASH at `pg:breaker:state` (D-05).
//
// PaperModeStickyUntil is an int64 unix-ms with sentinels (D-07):
//   - 0           = not sticky (breaker armed-but-not-tripped)
//   - math.MaxInt64 = sticky-until-operator-clears (breaker tripped, no auto-recovery)
//   - any positive < MaxInt64 currently unused; reserved for future timed sticky
//
// The sentinel encoding lets the field roundtrip through Redis as a decimal
// string without float truncation (Pitfall: math.MaxInt64 as float64 loses
// precision). Storage uses strconv.FormatInt(.., 10).
type BreakerState struct {
	PendingStrike        int     `json:"pending_strike"`             // 0 (no pending) or 1 (Strike-1 fired, awaiting Strike-2)
	Strike1Ts            int64   `json:"strike1_ts_ms"`              // unix-ms; 0 when no pending strike
	LastEvalTs           int64   `json:"last_eval_ts_ms"`            // unix-ms of most recent eval tick (heartbeat)
	LastEvalPnLUSDT      float64 `json:"last_eval_pnl_usdt"`         // realized 24h PnL at last eval tick
	PaperModeStickyUntil int64   `json:"paper_mode_sticky_until_ms"` // 0=not sticky; math.MaxInt64=sticky-until-operator (D-07)
}

// BreakerTripRecord — 8-field entry in the capped LIST `pg:breaker:trips` (D-18).
//
// Recovery fields (RecoveryTs, RecoveryOperator) are nullable — populated via
// LSET-backfill on operator-driven recovery. Source distinguishes real trips
// (live) from synthetic test fires (test_fire) and dry-run previews
// (test_fire_dry_run, never persisted per Plan 15-04 design).
type BreakerTripRecord struct {
	TripTs               int64   `json:"trip_ts_ms"`                   // unix-ms when sticky flag was persisted (D-15 step 1)
	TripPnLUSDT          float64 `json:"trip_pnl_usdt"`                // realized 24h PnL that triggered the trip
	Threshold            float64 `json:"threshold_usdt"`               // PriceGapDrawdownLimitUSDT in effect
	RampStage            int     `json:"ramp_stage"`                   // RampController.Snapshot().CurrentStage at trip
	PausedCandidateCount int     `json:"paused_candidate_count"`       // count of candidates with paused_by_breaker set during D-15 step 3
	RecoveryTs           *int64  `json:"recovery_ts_ms,omitempty"`     // nullable; backfilled on recovery
	RecoveryOperator     *string `json:"recovery_operator,omitempty"`  // nullable; operator handle on recovery
	Source               string  `json:"source"`                       // "live" | "test_fire" | "test_fire_dry_run"
}
