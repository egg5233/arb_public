package models

import "time"

// Discoverer abstracts the discovery/scanning module. The engine depends on
// this interface rather than on the concrete *discovery.Scanner so that
// implementations can be swapped for testing.
type Discoverer interface {
	// Scan triggers an immediate discovery scan and returns ranked opportunities.
	Scan() []Opportunity

	// GetOpportunities returns the latest cached set of ranked opportunities
	// from the most recent polling cycle.
	GetOpportunities() []Opportunity
}

// SymbolPair identifies a specific exchange pair for targeted re-scanning.
// Used by the entry fallback path when allocator overrides need re-validation.
type SymbolPair struct {
	Symbol        string
	LongExchange  string
	ShortExchange string
}

// SymbolRescanner can re-scan specific symbols through the full scanner pipeline
// with fresh data. Implemented by the concrete Scanner. Engine uses type
// assertion to check if the Discoverer also supports this capability.
type SymbolRescanner interface {
	RescanSymbols(pairs []SymbolPair) []Opportunity
}

// RiskChecker abstracts pre-trade risk assessment. The engine calls Approve
// before executing any trade.
type RiskChecker interface {
	// Approve evaluates whether an opportunity should be traded. It returns
	// an approval decision that includes the recommended position size and
	// price, or a rejection reason.
	Approve(opp Opportunity) (*RiskApproval, error)

	// ApproveWithReserved is like Approve but subtracts already-reserved
	// margin (from prior approvals in the same batch) before checking
	// balance sufficiency. This prevents concurrent margin over-commitment
	// when multiple candidates are approved sequentially then executed in
	// parallel. Pass nil to behave identically to Approve.
	ApproveWithReserved(opp Opportunity, reserved map[string]float64) (*RiskApproval, error)
}

// RejectionKind classifies why a risk approval was rejected so callers can
// decide whether the rejection is curable by cross-exchange transfer.
// Zero value (RejectionKindNone) applies to approved opportunities.
type RejectionKind int

const (
	RejectionKindNone     RejectionKind = iota // Approved=true
	RejectionKindCapital                        // curable by transfer: insufficient capital / margin buffer / post-trade ratio / notional floor
	RejectionKindCapacity                       // NOT curable: per-exchange capital cap
	RejectionKindMarket                         // NOT curable: empty orderbook
	RejectionKindSpread                         // NOT curable: slippage / gap / stability
	RejectionKindHealth                         // NOT curable: exchange health scorer
	RejectionKindConfig                         // NOT curable: max positions / exchange not configured
)

func (k RejectionKind) String() string {
	switch k {
	case RejectionKindCapital:
		return "capital"
	case RejectionKindCapacity:
		return "capacity"
	case RejectionKindMarket:
		return "market"
	case RejectionKindSpread:
		return "spread"
	case RejectionKindHealth:
		return "health"
	case RejectionKindConfig:
		return "config"
	default:
		return "none"
	}
}

// RiskApproval holds the result of a RiskChecker.Approve call.
type RiskApproval struct {
	Approved          bool          `json:"approved"`
	Kind              RejectionKind `json:"kind"`                  // rejection classification
	Size              float64       `json:"size"`
	Reason            string        `json:"reason"`
	Price             float64       `json:"price"`
	GapBPS            float64       `json:"gap_bps"`               // cross-exchange price gap at approval time
	// RequiredMargin is max(LongMarginNeeded, ShortMarginNeeded) with the
	// safety buffer already applied (for reservation purposes).
	// IMPORTANT: this value already includes the safety buffer — do NOT
	// multiply by MarginSafetyMultiplier again at call sites.
	RequiredMargin    float64       `json:"required_margin"`       // max(long,short) margin with safety buffer (for reservation)
	LongMarginNeeded  float64       `json:"long_margin_needed"`    // per-leg margin needed on long exchange (with buffer)
	ShortMarginNeeded float64       `json:"short_margin_needed"`   // per-leg margin needed on short exchange (with buffer)
}

// RiskAlert represents an alert emitted by the risk monitor for dashboard
// display. Matches the event channel type from plan section 3.11.
type RiskAlert struct {
	Type       string `json:"type"`
	PositionID string `json:"position_id"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// StateStore abstracts the persistence layer (Redis). Modules depend on this
// interface rather than on the concrete *database.Client.
type StateStore interface {
	// Position CRUD
	SavePosition(pos *ArbitragePosition) error
	GetPosition(id string) (*ArbitragePosition, error)
	GetActivePositions() ([]*ArbitragePosition, error)
	UpdatePositionStatus(id, status string) error

	// History
	AddToHistory(pos *ArbitragePosition) error
	GetHistory(limit int) ([]*ArbitragePosition, error)

	// Funding
	SaveFundingSnapshot(snap *FundingSnapshot) error
	GetLatestFunding(symbol string) (*FundingSnapshot, error)

	// Stats
	UpdateStats(pnl float64, won bool) error
	GetStats() (map[string]string, error)

	// Balance cache
	SaveBalance(exchange string, balance float64) error
	GetBalance(exchange string) (float64, error)

	// Distributed locks
	AcquireLock(resource string, ttl time.Duration) (bool, error)
	ReleaseLock(resource string) error
}
