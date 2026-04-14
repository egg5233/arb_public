package engine

import (
	"arb/internal/models"
)

// unified_state.go — shared occupancy snapshot used by the unified
// cross-strategy entry selector.
//
// The selector needs a single, atomic view of "what is currently open"
// across both the perp-perp and spot-futures stores so the B&B search
// can:
//   - deduplicate symbols across strategies (one symbol may be open in at
//     most one strategy at a time)
//   - respect the combined MaxPositions ceiling (perp + spot)
//   - respect the spot-only SpotFuturesMaxPositions sub-cap
//
// Per plan Section 9 the non-closed statuses that occupy a slot are:
//   - perp: pending, partial, active, exiting, closing
//   - spot: pending, active, exiting
//
// Those are the only statuses the engine and spot engine guard against
// duplicate entry for today (see `internal/engine/engine.go:2103` and
// `internal/spotengine/execution.go:143`), so the unified occupancy view
// must match exactly — any mismatch would let the selector pick a
// symbol that the last-line safety net then blocks, burning a capital
// reservation.

// unifiedOccupancy is a point-in-time snapshot of slot usage across both
// strategy stores. It is built once per EntryScan tick so the selector
// sees a consistent view during candidate filtering and B&B evaluation.
type unifiedOccupancy struct {
	// ActiveSymbols is the union of non-closed symbols across perp and
	// spot positions. The selector excludes any symbol in this set from
	// its candidate groups so a symbol cannot be opened in a second
	// strategy while a position in it is still live in the first.
	ActiveSymbols map[string]struct{}

	// ActivePerp counts non-closed perp positions
	// (pending + partial + active + exiting + closing).
	ActivePerp int

	// ActiveSpot counts non-closed spot-futures positions
	// (pending + active + exiting).
	ActiveSpot int

	// GlobalSlotsRemaining is cfg.MaxPositions - (ActivePerp + ActiveSpot).
	// Clamped at zero by loadUnifiedOccupancy so the selector can trust
	// non-negativity.
	GlobalSlotsRemaining int

	// SpotSlotsRemaining is cfg.SpotFuturesMaxPositions - ActiveSpot.
	// Clamped at zero. Used by the selector's evaluate callback to reject
	// sets whose spot picks exceed the spot-only sub-cap even when the
	// global ceiling would still allow them.
	SpotSlotsRemaining int
}

// loadUnifiedOccupancy reads the perp and spot position stores and
// returns the current unifiedOccupancy snapshot. Non-closed statuses
// occupy a slot per plan Section 9; any read error from either store
// bubbles up unchanged so the caller can fall back to the legacy path
// (unified selection must never run with a half-built occupancy view).
//
// Intentionally written to match engine.go:2103 (perp) and
// spotengine/execution.go:143 (spot) semantics so the selector's symbol
// deduplication and slot accounting agrees with the last-line safety
// guards — any divergence would let the B&B pick winners the safety
// guard then blocks, burning a preheld reservation.
func (e *Engine) loadUnifiedOccupancy() (*unifiedOccupancy, error) {
	occ := &unifiedOccupancy{
		ActiveSymbols: make(map[string]struct{}),
	}

	perpActive, err := e.db.GetActivePositions()
	if err != nil {
		return nil, err
	}
	for _, p := range perpActive {
		if p == nil {
			continue
		}
		// Non-closed perp statuses occupy a slot.
		// Matches engine.go:2108 (`p.Status != models.StatusClosed`) and
		// plan Section 9: pending/partial/active/exiting/closing.
		if p.Status == models.StatusClosed {
			continue
		}
		occ.ActivePerp++
		if p.Symbol != "" {
			occ.ActiveSymbols[p.Symbol] = struct{}{}
		}
	}

	spotActive, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return nil, err
	}
	for _, sp := range spotActive {
		if sp == nil {
			continue
		}
		// Non-closed spot statuses occupy a slot.
		// Spot only has pending/active/exiting/closed (no partial/closing)
		// per internal/models/spot_position.go:74-77.
		if sp.Status == models.SpotStatusClosed {
			continue
		}
		occ.ActiveSpot++
		if sp.Symbol != "" {
			occ.ActiveSymbols[sp.Symbol] = struct{}{}
		}
	}

	global := e.cfg.MaxPositions - (occ.ActivePerp + occ.ActiveSpot)
	if global < 0 {
		global = 0
	}
	occ.GlobalSlotsRemaining = global

	spotCap := e.cfg.SpotFuturesMaxPositions - occ.ActiveSpot
	if spotCap < 0 {
		spotCap = 0
	}
	occ.SpotSlotsRemaining = spotCap

	return occ, nil
}
