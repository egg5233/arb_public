// Package pricegaptrader — notify.go defines the Broadcaster DI seam for
// Phase 9 dashboard WebSocket push. The interface is owned by this package
// to preserve the D-02 module boundary (this package depends on nothing above
// it in the dependency graph). Plan 06 adds a concrete implementation in
// the API layer that wraps its WS hub.
package pricegaptrader

import "arb/internal/models"

// PriceGapEvent is the envelope for lifecycle events pushed to the dashboard.
// Type is one of: "entry" | "exit" | "auto_disable".
type PriceGapEvent struct {
	Type     string                   `json:"type"`
	Position *models.PriceGapPosition `json:"position,omitempty"`
	Symbol   string                   `json:"symbol,omitempty"`
	Reason   string                   `json:"reason,omitempty"`
}

// PriceGapCandidateUpdate signals a change in candidate disable state
// (PG-RISK-03 exec-quality auto-disable, operator reversal via pg-admin).
type PriceGapCandidateUpdate struct {
	Symbol     string `json:"symbol"`
	Disabled   bool   `json:"disabled"`
	Reason     string `json:"reason,omitempty"`
	DisabledAt int64  `json:"disabled_at,omitempty"` // unix seconds; 0 when disabled=false
}

// Broadcaster is the outbound-events contract satisfied by the dashboard
// WS hub (see Plan 06). Tests and CLI paths receive NoopBroadcaster{} to
// avoid nil derefs.
type Broadcaster interface {
	BroadcastPriceGapPositions(positions []*models.PriceGapPosition)
	BroadcastPriceGapEvent(evt PriceGapEvent)
	BroadcastPriceGapCandidateUpdate(update PriceGapCandidateUpdate)
}

// NoopBroadcaster is the zero-behavior default used when no WS hub is wired.
// Safe to call on unit-test Trackers and in pg-admin CLI paths.
type NoopBroadcaster struct{}

// BroadcastPriceGapPositions is a no-op.
func (NoopBroadcaster) BroadcastPriceGapPositions([]*models.PriceGapPosition) {}

// BroadcastPriceGapEvent is a no-op.
func (NoopBroadcaster) BroadcastPriceGapEvent(PriceGapEvent) {}

// BroadcastPriceGapCandidateUpdate is a no-op.
func (NoopBroadcaster) BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate) {}
