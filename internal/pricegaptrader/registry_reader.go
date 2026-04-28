// Package pricegaptrader — RegistryReader interface (Phase 11 / D-13).
//
// RegistryReader is the read-only view of the CandidateRegistry chokepoint
// (PG-DISC-04). It exposes ONLY Get + List so that downstream consumers
// (Phase 11 scanner) cannot mutate cfg.PriceGapCandidates at compile time.
//
// Phase 12 swaps consumers from RegistryReader → *Registry to enable
// auto-promotion. Phase 11 keeps everyone on RegistryReader.
//
// Module-boundary contract (T-11-07): this file MUST NOT import either of
// the live trading engines or any package that does. See CLAUDE.md "Module
// boundaries" section for the allow-list.
package pricegaptrader

import "arb/internal/models"

// RegistryReader is the read-only contract over the candidate registry.
// Mutators (Add/Update/Delete/Replace) are deliberately omitted; Plan 02 will
// add them on the concrete *Registry type and Phase 12 will swap consumers
// over once auto-promotion lands. Until then, downstream packages can only
// observe the registry — never mutate it.
type RegistryReader interface {
	// Get returns the candidate at index idx, or false if idx is out of range.
	// Index is the position inside cfg.PriceGapCandidates as of the last
	// successful registry mutation; reads are snapshot-consistent under cfg.mu.
	Get(idx int) (models.PriceGapCandidate, bool)

	// List returns a defensive copy of the current candidate slice. Callers
	// may iterate and mutate the returned slice freely without affecting the
	// registry's internal state.
	List() []models.PriceGapCandidate
}
