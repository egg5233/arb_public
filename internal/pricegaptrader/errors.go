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
)
