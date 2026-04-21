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
}

// ID returns the canonical candidate identifier. Matches the D-15
// position-ID prefix convention (symbol_longExch_shortExch).
func (c PriceGapCandidate) ID() string {
	return c.Symbol + "_" + c.LongExch + "_" + c.ShortExch
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

	// Exec-quality disable flag (D-19, D-20)
	IsCandidateDisabled(symbol string) (bool, string, error)
	SetCandidateDisabled(symbol, reason string) error
	ClearCandidateDisabled(symbol string) error

	// Slippage rolling window (D-19, D-21; N=10 per PG-RISK-03)
	AppendSlippageSample(candidateID string, sample SlippageSample) error
	GetSlippageWindow(candidateID string, n int) ([]SlippageSample, error)

	// Redis lock reuse (Phase 8 uses "arb:locks:pg:<symbol>" per CONTEXT code_context)
	AcquirePriceGapLock(resource string, ttl time.Duration) (token string, ok bool, err error)
	ReleasePriceGapLock(resource, token string) error
}
