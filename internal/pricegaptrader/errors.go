package pricegaptrader

import "errors"

// Typed sentinels for gate denials. Exit reason persisted per D-13
// uses plain strings (ExitReasonRiskGate) — these sentinels are for
// in-process error propagation and log-line classification.
var (
	ErrPriceGapDisabled             = errors.New("pricegap: tracker disabled")
	ErrPriceGapCandidateDisabled    = errors.New("pricegap: candidate exec-quality disabled")
	ErrPriceGapMaxConcurrent        = errors.New("pricegap: max concurrent positions reached")
	ErrPriceGapPerPositionCap       = errors.New("pricegap: per-position notional cap exceeded")
	ErrPriceGapBudgetExceeded       = errors.New("pricegap: budget exceeded")
	ErrPriceGapGateConcentrationCap = errors.New("pricegap: gate concentration cap exceeded")
	ErrPriceGapDelistedLeg          = errors.New("pricegap: leg delisted or halted")
	ErrPriceGapStaleBBO             = errors.New("pricegap: BBO staler than threshold")
	ErrPriceGapCircuitBreaker       = errors.New("pricegap: circuit breaker open")
	ErrPriceGapDuplicateCandidate   = errors.New("pricegap: candidate already has an active position")

	// Phase 14 Plan 14-03 (PG-LIVE-01) — Gate 6 ramp sentinels.

	// ErrPriceGapRampExceeded — Gate 6 ramp rejection. Returned when
	// requestedNotionalUSDT > min(stageSize, hardCeiling) AND PriceGapLiveCapital=true.
	ErrPriceGapRampExceeded = errors.New("pricegap: ramp exceeded — notional > min(stage_size, hard_ceiling)")

	// ErrPriceGapRampStateUnavailable — Gate 6 fail-closed when ramp state
	// read fails AND live_capital=true. Refuses to size live capital from
	// unknown ramp state; matches T-14-12 fail-closed posture.
	ErrPriceGapRampStateUnavailable = errors.New("pricegap: ramp state unavailable — refusing to size live capital")
)
