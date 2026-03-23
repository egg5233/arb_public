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

// RiskChecker abstracts pre-trade risk assessment. The engine calls Approve
// before executing any trade.
type RiskChecker interface {
	// Approve evaluates whether an opportunity should be traded. It returns
	// an approval decision that includes the recommended position size and
	// price, or a rejection reason.
	Approve(opp Opportunity) (*RiskApproval, error)
}

// RiskApproval holds the result of a RiskChecker.Approve call.
type RiskApproval struct {
	Approved bool    `json:"approved"`
	Size     float64 `json:"size"`
	Reason   string  `json:"reason"`
	Price    float64 `json:"price"`
	GapBPS   float64 `json:"gap_bps"` // cross-exchange price gap at approval time
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
