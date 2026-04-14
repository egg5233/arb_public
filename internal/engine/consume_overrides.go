package engine

import (
	"arb/internal/models"
)

// symbolRescanner is a narrow rescanner contract used by
// consumeOverridesAndEnrichOpps so tests can inject a lightweight stub
// without building a real *discovery.Scanner. The concrete
// *discovery.Scanner satisfies this interface out of the box (see
// internal/discovery/scanner.go:1104).
type symbolRescanner interface {
	RescanSymbols(pairs []models.SymbolPair) []models.Opportunity
}

// rescanner returns the interface used by consumeOverridesAndEnrichOpps
// to drive the v0.32.8 fallback path. Priority:
//   - e.rescannerOverride (set by tests via setRescannerForTest)
//   - e.discovery (production path)
//
// The override field is declared in engine.go; a nil override falls
// through to the real scanner. Keeping the indirection out of the
// hot path means production dispatch is byte-identical to the live
// implementation.
func (e *Engine) rescanner() symbolRescanner {
	if e.rescannerOverride != nil {
		return e.rescannerOverride
	}
	if e.discovery == nil {
		return nil
	}
	return e.discovery
}

// consume_overrides.go — single shared helper consumeOverridesAndEnrichOpps
// used by BOTH the legacy EntryScan dispatcher and the new unified
// cross-strategy entry selector so the `allocOverrides` map produced by
// `rebalanceFunds()` at `engine.go:640-642` is consumed (read + cleared)
// exactly once per EntryScan tick. Two callers cannot each try to consume
// the same overrides map — that would split the consume-once invariant and
// let stale choices bleed into the next scan cycle.
//
// Per plan Section 8 this helper covers both live behaviors that used to
// be split across two inline blocks:
//
//  1. tier-2 override patching (engine.go:1036-1128 logic)
//     — non-empty perp opps plus non-empty overrides: patch each opp's
//       exchange pair to the allocator's choice (only when the choice
//       still exists as a verified alternative in the CURRENT scan).
//       Delegates to the pure helper `applyAllocatorOverridesWithState`.
//
//  2. v0.32.8 RescanSymbols fallback (engine.go:1283-1300 logic)
//     — empty perp opps plus non-empty overrides: re-scan only the
//       allocator-chosen symbols to salvage the transferred funds.
//       Delegates to `e.discovery.RescanSymbols` with a SymbolPair list.
//
//  3. no-op fall-through
//     — empty overrides: return perpOpps unchanged with tier tag "none".
//
// The helper acquires `e.allocOverrideMu` exactly once; no caller is
// allowed to read `e.allocOverrides` directly. Both the legacy
// `executeArbitrage` path and the new `runUnifiedEntrySelection` call
// this helper at the top of their perp intake stage.

// consumeOverridesAndEnrichOpps reads and clears e.allocOverrides
// under allocOverrideMu, then dispatches to the appropriate branch:
//
//   - overrides empty:          return perpOpps unchanged, tier="none"
//   - perpOpps non-empty:       apply tier-2 patching; tier="tier-2-override-patch"
//                               when patching actually happened, else "none"
//   - perpOpps empty:           call discovery.RescanSymbols for the override
//                               pairs; tier="tier-2-override-fallback" when at
//                               least one symbol re-scanned, else
//                               "tier-2-override-fallback-empty"
//
// Returns the enriched/rescanned opp list and a human-readable tier tag
// for observability. On the empty-overrides fast path the same opps slice
// is returned (no defensive copy) — callers must not mutate the slice
// after this point.
func (e *Engine) consumeOverridesAndEnrichOpps(perpOpps []models.Opportunity) (out []models.Opportunity, tier string) {
	// Consume once, under the lock — matches live behavior at
	// engine.go:1027-1030 (tier-2 branch) and :1275-1278 (fallback branch).
	e.allocOverrideMu.Lock()
	overrides := e.allocOverrides
	e.allocOverrides = nil
	e.allocOverrideMu.Unlock()

	if len(overrides) == 0 {
		// No allocator guidance for this tick — pass opps through as-is.
		// Mirrors the live behavior where applyAllocatorOverrides returns
		// (nil, false) and the caller falls through to tier-3 ranking.
		return perpOpps, "none"
	}

	if len(perpOpps) > 0 {
		// Non-empty opps: run the tier-2 override patch logic (formerly
		// inline engine.go:1036-1128). The pure helper returns (patchedOpps,
		// didPatch); translate the boolean into the tier tag expected by
		// the rest of the entry chain.
		patched, didPatch := e.applyAllocatorOverridesWithState(perpOpps, overrides)
		if didPatch && len(patched) > 0 {
			return patched, "tier-2-override-patch"
		}
		if didPatch {
			// Overrides existed but every one was stale. Return nil + a
			// tier tag the caller can log; legacy EntryScan will then
			// decide whether to fall through to tier-3 on its own.
			return nil, "tier-2-override-patch"
		}
		// No overrides matched (unreachable given len(overrides) > 0, but
		// preserved for defense in depth) — treat like the empty branch.
		return perpOpps, "none"
	}

	// Empty perp opps + non-empty overrides — v0.32.8 RescanSymbols
	// salvage path. Translate each allocatorChoice into a SymbolPair and
	// drive a targeted re-scan.
	pairs := make([]models.SymbolPair, 0, len(overrides))
	for symbol, choice := range overrides {
		pairs = append(pairs, models.SymbolPair{
			Symbol:        symbol,
			LongExchange:  choice.longExchange,
			ShortExchange: choice.shortExchange,
		})
	}

	e.log.Info("entry: discovery returned 0 opps but %d allocator overrides exist, re-scanning", len(overrides))
	rs := e.rescanner()
	var fallbackOpps []models.Opportunity
	if rs != nil {
		fallbackOpps = rs.RescanSymbols(pairs)
	}
	if len(fallbackOpps) > 0 {
		e.log.Info("entry: override fallback: %d/%d passed re-scan", len(fallbackOpps), len(pairs))
		return fallbackOpps, "tier-2-override-fallback"
	}
	e.log.Info("entry: override fallback: all %d symbols failed re-scan", len(pairs))
	return nil, "tier-2-override-fallback-empty"
}
