package engine

import (
	"time"

	"arb/internal/models"
	"arb/internal/risk"
)

// spotEntryExecutor is the contract the unified cross-strategy entry selector
// uses to pull spot-futures candidates and dispatch preheld winners.
//
// The Engine intentionally depends on this narrow interface rather than on
// the concrete *spotengine.SpotEngine to keep the selector decoupled from
// the spot engine's internal state. Concrete implementation lives in
// internal/spotengine/selected_entry.go and is installed on the Engine via
// Engine.SetSpotEntryExecutor at startup (before Engine.Start so the EntryScan
// handler never observes a nil executor).
//
// Contract:
//   - ListEntryCandidates returns a read-only projection of the latest
//     post-filter spot opportunities, dropping entries older than maxAge.
//     The selector passes 2 * cfg.SpotDiscoveryInterval to match perp
//     freshness semantics.
//   - BuildEntryPlan sizes a candidate against live per-exchange capital,
//     futures step size and (for Dir A on unified accounts) MaxBorrowable,
//     returning a feasibility-checked plan with PlannedNotionalUSDT set to
//     the exact USDT that will be reserved.
//   - OpenSelectedEntry dispatches a preheld reservation via the same
//     execution primitives ManualOpen uses, WITHOUT re-reserving capital and
//     WITHOUT consulting the latestOpps cache (the plan carries everything
//     execution needs). capOverride mirrors risk.ReserveWithCap semantics;
//     zero means no override. preheld must be the reservation produced by
//     risk.CapitalAllocator.ReserveBatch for this candidate key.
type spotEntryExecutor interface {
	// ListEntryCandidates returns post-filter spot arbitrage candidates
	// captured no earlier than maxAge ago.
	ListEntryCandidates(maxAge time.Duration) []models.SpotEntryCandidate

	// BuildEntryPlan sizes a candidate into a reservable plan or returns a
	// non-nil error when the candidate fails any feasibility gate (e.g.
	// below futures min size, below budget floor, BingX with no margin API).
	BuildEntryPlan(c models.SpotEntryCandidate) (*models.SpotEntryPlan, error)

	// OpenSelectedEntry opens the delta-neutral spot-futures position using
	// a preheld batch reservation. Returns an error if execution fails; the
	// selector is responsible for releasing preheld reservations whose
	// dispatch errors.
	OpenSelectedEntry(plan *models.SpotEntryPlan, capOverride float64, preheld *risk.CapitalReservation) error
}
