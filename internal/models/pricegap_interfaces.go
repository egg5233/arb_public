package models

import "time"

// PriceGapCandidate — D-08 direction-pinned tuple (PG-05).
// Static config entry describing one tradable long/short exchange pair for a symbol.
type PriceGapCandidate struct {
	Symbol             string  `json:"symbol"`
	LongExch           string  `json:"long_exch"`
	ShortExch          string  `json:"short_exch"`
	ThresholdBps       float64 `json:"threshold_bps"`
	MaxPositionUSDT    float64 `json:"max_position_usdt"`
	ModeledSlippageBps float64 `json:"modeled_slippage_bps"`
	// Direction — Phase 999.1 (PG-DIR-01): "pinned" (default) | "bidirectional".
	// pinned: detector requires positive-direction sign continuity (long_exch
	//   cheaper than short_exch by ≥ T) — closes the latent Phase-8 bug where
	//   barRing.allExceed used math.Abs and silently fired any sign.
	// bidirectional: fires on either sign; executor swaps wire-side leg roles
	//   for inverse fires.
	// Empty/missing decodes as "" and is normalized to "pinned" by
	// NormalizeDirection — preserves backward compat with pre-Phase-999.1
	// candidates persisted in config.json.
	Direction string `json:"direction,omitempty"`
}

// ID returns the canonical candidate identifier. Matches the D-15
// position-ID prefix convention (symbol_longExch_shortExch).
//
// PG-DIR-01 invariant: Direction is a behavior property, NOT identity. Two
// candidates with the same (symbol, long_exch, short_exch) tuple but different
// Direction values produce the same ID() and would be rejected as duplicates
// by Phase 10's validator. Honors the Phase 10 D-11/D-13/D-14 identity rules.
func (c PriceGapCandidate) ID() string {
	return c.Symbol + "_" + c.LongExch + "_" + c.ShortExch
}

// PriceGapDirectionPinned and PriceGapDirectionBidirectional are the canonical
// values for PriceGapCandidate.Direction. Empty string normalizes to "pinned"
// for backward compat with pre-Phase-999.1 candidates (Pitfall 2 mitigation).
const (
	PriceGapDirectionPinned        = "pinned"
	PriceGapDirectionBidirectional = "bidirectional"
)

// NormalizeDirection defaults an empty Direction field to "pinned" so candidates
// persisted before Phase 999.1 continue to fire only on the configured-direction
// sign of the spread (PG-DIR-01 backward-compat invariant). Mirrors the
// NormalizeMode pattern from pricegap_position.go.
func NormalizeDirection(c *PriceGapCandidate) {
	if c == nil {
		return
	}
	if c.Direction == "" {
		c.Direction = PriceGapDirectionPinned
	}
}

// DelistChecker — DI for pricegaptrader (D-02 boundary; no *discovery.Scanner import).
// The concrete *discovery.Scanner (IsDelisted method) satisfies this interface.
// Injected into the tracker to veto entries on symbols flagged for delisting.
type DelistChecker interface {
	IsDelisted(symbol string) bool
}

// PriceGapStore — DI for pricegaptrader (D-02 boundary; no *database.Client import).
// The concrete *database.Client satisfies this interface in Plan 02.
type PriceGapStore interface {
	// Positions
	SavePriceGapPosition(p *PriceGapPosition) error
	GetPriceGapPosition(id string) (*PriceGapPosition, error)
	GetActivePriceGapPositions() ([]*PriceGapPosition, error)
	AddPriceGapHistory(p *PriceGapPosition) error
	RemoveActivePriceGapPosition(id string) error

	// Exec-quality disable flag (D-19, D-20; Phase 9 Pitfall 6)
	// Returns (disabled, reason, disabledAtUnixSec, err). disabledAt is 0 for
	// legacy plain-string values written by pre-Phase-9 pg-admin.
	IsCandidateDisabled(symbol string) (bool, string, int64, error)
	SetCandidateDisabled(symbol, reason string) error
	ClearCandidateDisabled(symbol string) error

	// Slippage rolling window (D-19, D-21; N=10 per PG-RISK-03)
	AppendSlippageSample(candidateID string, sample SlippageSample) error
	GetSlippageWindow(candidateID string, n int) ([]SlippageSample, error)

	// Redis lock reuse (Phase 8 uses "arb:locks:pg:<symbol>" per CONTEXT code_context)
	AcquirePriceGapLock(resource string, ttl time.Duration) (token string, ok bool, err error)
	ReleasePriceGapLock(resource, token string) error
}
