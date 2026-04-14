# PLAN: Unified Allocator Post-Impl Fix v2

**Status:** DRAFT v1 — pending Codex review
**Author:** claude
**Date:** 2026-04-14
**Trigger:** Codex post-fix review (#30b7ce58) found 2 blocking issues in v0.33.0 implementation that i5's review missed.

---

## 1. Context

v0.33.0 shipped unified cross-strategy allocator on branch `feat/unified-entry-selection` (commit `e5b31d61`). Post-impl Codex review identified 2 real bugs + 2 test gaps:

1. **Static strategy cap divergence from live allocator**: `makeUnifiedEvaluator` uses static `cfg.MaxPerpPerpPct`, but live `CapitalAllocator.checkCapsWithOverride` uses dynamic `strategyPct()` (from CA-04 dynamic shifting) plus optional `capOverride`. Unified selector never threads non-zero overrides into `BatchReservationItem` (`unified_entry.go:342` + `:229` always pass `CapOverride: 0`). Solver can admit sets `ReserveBatch` later rejects, or reject sets the live allocator would accept.

2. **capacityMu serialization bypass**: Legacy `executeArbitrage` holds `e.capacityMu` across active-read → slot count → approval → reservation → pending-save (`engine.go:2122` area). Unified path takes snapshot (`unified_state.go:70`), then reserves + dispatches winners in parallel (`unified_entry.go:228-299`) without acquiring `capacityMu`. Concurrent `ManualOpen` (from dashboard) or automated entry can land between snapshot and pending save → slot count drift + preheld reservations burned on winners that are no longer admissible.

**Test gaps:**
- `TestUnifiedEntry_DispatchHonorsPreheldReservation` fabricates a reservation and calls `dispatchUnifiedSpot` directly — doesn't prove `runUnifiedEntrySelection → ReserveBatch → batch.Items[key] → OpenSelectedEntry` end-to-end.
- Parity suite hits `solveAllocator`/`greedyAllocatorSeed` directly, not `runPoolAllocator` or `dryRunTransferPlan`. Fee-cache interaction, override/revalidation untested.

---

## 2. Goal

Align unified selection's cap decisions with the live allocator's actual cap enforcement, and close the entry-admission race window. No user-visible behavior change when flag is OFF.

---

## 3. Fixes

### Fix G — Dynamic strategy pct + capOverride threading

**Files:**
- `internal/engine/unified_capacity.go` — snapshot must carry dynamic strategy percentages
- `internal/engine/unified_entry.go` — `exposuresForChoice` + `BatchReservationItem` must thread `CapOverride`

**Current state (broken):**
```go
// unified_capacity.go:72 — uses static config:
perpCap := s.TotalCap * s.cfg.MaxPerpPerpPct
spotCap := s.TotalCap * s.cfg.MaxSpotFuturesPct

// unified_entry.go:229 — always zero override:
items = append(items, risk.BatchReservationItem{
    ...
    CapOverride: 0, // always zero — never unlocks CA-04 dynamic headroom
})
```

**Required change (match live `checkCapsWithOverride` at `risk/allocator.go:560-599`):**

1. Extend `unifiedCapacitySnapshot` to carry the dynamic effective percentages:
   ```go
   type unifiedCapacitySnapshot struct {
       // existing fields unchanged
       TotalCap           float64
       CommittedPerp      float64
       CommittedSpot      float64
       // NEW fields replacing static caps:
       EffectivePerpPct   float64  // allocator.strategyPct(StrategyPerpPerp) at snapshot time
       EffectiveSpotPct   float64  // allocator.strategyPct(StrategySpotFutures) at snapshot time
       // Per-strategy cap overrides keyed by candidate key (populated in step 2 below)
       PerpCapOverride    float64  // 0 = no override; >0 = allocator-computed override from CA-04
       SpotCapOverride    float64  // same for spot
   }
   ```

2. `buildCapacitySnapshot` queries `allocator.Summary()` which already exposes `EffectivePerpPct` / `EffectiveSpotPct`. Use those directly instead of `cfg.MaxPerpPerpPct`.

3. `makeUnifiedEvaluator.evaluate(keys)` computes strategy ceilings as:
   ```go
   perpCap := s.TotalCap * s.EffectivePerpPct
   spotCap := s.TotalCap * s.EffectiveSpotPct
   // Plus override allowance: if candidate has a valid per-strategy override
   // (e.g. CA-04 has freed spot headroom), add that on top
   if s.PerpCapOverride > perpCap { perpCap = s.PerpCapOverride }
   if s.SpotCapOverride > spotCap { spotCap = s.SpotCapOverride }
   ```
   Note: override is a **ceiling** not an addition — match `checkCapsWithOverride` semantics: `effectiveCap := max(normalCap, override)`.

4. `exposuresForChoice` + `BatchReservationItem` construction: when the selector decides to unlock headroom for a candidate, pass the corresponding override via `CapOverride`. Query allocator for the same override the legacy perp path uses: `engine.go:1306` already computes dynamic cap via `computeDynamicCap(result)` → `ReserveWithCap(..., dynamicCap)`. Factor that computation so unified path calls the same helper:
   ```go
   perpOverride := e.computeDynamicPerpCap(snapshot) // new helper factored from engine.go:1306
   spotOverride := e.computeDynamicSpotCap(snapshot) // symmetric for spot
   items = append(items, risk.BatchReservationItem{
       Strategy: risk.StrategyPerpPerp,
       Exposures: exposures,
       CapOverride: perpOverride,  // was hardcoded 0
   })
   ```

**Acceptance:** solver admits exactly the sets `ReserveBatch` accepts; snapshot ↔ enforcement cap semantics byte-identical.

### Fix H — capacityMu serialization for unified path

**Files:**
- `internal/engine/unified_entry.go` — wrap snapshot → reserve → dispatch in `capacityMu` critical section
- `internal/engine/engine.go` (already has capacityMu field) — no struct change

**Current state (broken):**
```go
// unified_entry.go:~200 — snapshot taken without capacityMu:
snapshot, err := e.buildCapacitySnapshot()
...
// ReserveBatch + dispatch fire without capacityMu — race with ManualOpen
```

**Required change:**

`runUnifiedEntrySelection` must hold `e.capacityMu` across these steps (same scope as legacy `executeArbitrage` at `engine.go:2122`):

```go
func (e *Engine) runUnifiedEntrySelection() error {
    // ... readiness gate + log + initial opp gathering (no lock) ...
    
    // Begin admission critical section
    e.capacityMu.Lock()
    defer e.capacityMu.Unlock()
    
    // Take snapshot under lock — guarantees active-count + reservation state frozen
    occupancy, err := e.loadUnifiedOccupancy()
    if err != nil { return err }
    snapshot, err := e.buildCapacitySnapshot()
    if err != nil { return err }
    
    // Build candidates, solve, reserve, persist-pending — all under lock
    choices := e.buildUnifiedChoices(perpOpps, spotCands, occupancy, snapshot)
    winners := solveGroupedSearch(...)
    batch, err := e.allocator.ReserveBatch(items)
    if err != nil { return err }
    
    // Persist pending positions under lock (symmetry with legacy at engine.go:2122+)
    if err := e.persistPendingPositions(winners, batch); err != nil {
        e.allocator.ReleaseBatch(batch)
        return err
    }
    
    // Release lock BEFORE long-running dispatch (same pattern as legacy)
    e.capacityMu.Unlock()
    defer e.capacityMu.Lock()  // re-lock before defer Unlock fires (or restructure)
    
    // Dispatch perp + spot winners in parallel — no lock needed, pending record protects
    e.dispatchAllWinners(winners, batch)
    return nil
}
```

**Refactor note:** the `defer/Unlock/Lock` gymnastics is ugly. Cleaner version (preferred):
```go
func (e *Engine) runUnifiedEntrySelection() error {
    // ... readiness + opps ...
    
    winners, batch, err := e.admitUnifiedWinners(perpOpps, spotCands)  // holds capacityMu internally
    if err != nil { return err }
    if len(winners) == 0 { return nil }
    
    // Dispatch outside admission lock
    return e.dispatchAllWinners(winners, batch)
}

func (e *Engine) admitUnifiedWinners(perpOpps []models.Opportunity, spotCands []*models.SpotEntryPlan) ([]unifiedEntryChoice, *risk.BatchReservation, error) {
    e.capacityMu.Lock()
    defer e.capacityMu.Unlock()
    
    occupancy, err := e.loadUnifiedOccupancy()
    // ... snapshot, build, solve, reserve, persist-pending ...
    return winners, batch, nil
}
```

**Acceptance:** unified path's admission window is serialized against legacy `ManualOpen` / `executeArbitrage` / L4-flatten paths that also acquire `capacityMu`. Pending position records visible to concurrent admission checks the moment the batch reservation succeeds.

### Fix I — E2E preheld reservation test

**File:** `internal/engine/unified_entry_test.go` (extend)

Replace fabricated-reservation test with real orchestrator flow:

```go
func TestUnifiedEntry_E2EPreheldReservationDoesNotDoubleReserve(t *testing.T) {
    // Setup: miniredis allocator, stub spot executor that records reservation pointer on OpenSelectedEntry
    // Stub perp executor that records reservation pointer on dispatchUnifiedPerp
    // Run: runUnifiedEntrySelection with 2 perp + 1 spot opp
    // Assert:
    //   - ReserveBatch called exactly once
    //   - batch.Items contains 3 entries (2 perp + 1 spot)
    //   - Each winner's dispatcher received the pre-existing reservation (not a new one)
    //   - Allocator.Reserve() never called during dispatch (spy with counter)
}
```

### Fix J — runPoolAllocator parity test

**File:** `internal/engine/allocator_parity_test.go` (extend)

```go
func TestRunPoolAllocator_EndToEndParityWithRebalance(t *testing.T) {
    // Setup: 5 candidates, mixed exchanges, with dryRunTransferPlan producing
    // real transfer fees for some paths (not all zero-fee)
    // Run runPoolAllocator BEFORE and AFTER the selection_core refactor
    // (frozen fixture of expected output must match exact choices + fees + transfer plan)
}
```

This exercises `runPoolAllocator → dryRunTransferPlan → solveAllocator → selection_core` full stack, not just the B&B in isolation.

---

## 4. Test Plan

| Test | File | What it verifies |
|---|---|---|
| `TestUnifiedEntry_SnapshotUsesEffectivePctNotStatic` | `unified_capacity_test.go` | snapshot reads `allocator.Summary().EffectivePerpPct`, not `cfg.MaxPerpPerpPct` |
| `TestUnifiedEntry_EvaluatorHonorsCapOverride` | `unified_entry_test.go` | when snapshot has non-zero override, evaluator uses override as ceiling |
| `TestUnifiedEntry_BatchReservationItemIncludesCapOverride` | `unified_entry_test.go` | `BatchReservationItem.CapOverride` populated from `computeDynamicPerpCap` / `computeDynamicSpotCap` |
| `TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen` | `unified_entry_test.go` | while unified admission holds `capacityMu`, a concurrent `ManualOpen` call blocks until admission finishes (use goroutine + sync.WaitGroup + miniredis) |
| `TestUnifiedEntry_E2EPreheldReservationDoesNotDoubleReserve` | `unified_entry_test.go` | See Fix I |
| `TestRunPoolAllocator_EndToEndParityWithRebalance` | `allocator_parity_test.go` | See Fix J |

---

## 5. Rollback

All 4 fixes are additive or local — no schema changes. Rollback = `git revert <fix commit>`. Flag-gated behavior unchanged; legacy path unaffected.

---

## 6. Risks

| Risk | Mitigation |
|---|---|
| Dynamic pct query adds Redis latency to unified snapshot | Snapshot built once per EntryScan tick. Allocator.Summary() already cached ~1s. Unchanged TPS. |
| capacityMu held longer than legacy (admission now covers B&B solve) | B&B has 30s timeout; realistic admission < 3s. Compared to legacy admission ~500ms, worst-case 6× longer. Acceptable for entry cadence. If measured > 5s, add pre-admission candidate count cap. |
| computeDynamicPerpCap helper refactor affects legacy rebalance | Wrap existing logic 1:1; parity test catches drift. |
| E2E preheld test brittle due to goroutine timing | Use `sync.WaitGroup` + explicit test-only hooks, not time.Sleep. |

---

## 7. Acceptance Criteria

- [ ] `TestUnifiedEntry_SnapshotUsesEffectivePctNotStatic` pass
- [ ] `TestUnifiedEntry_EvaluatorHonorsCapOverride` pass
- [ ] `TestUnifiedEntry_BatchReservationItemIncludesCapOverride` pass
- [ ] `TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen` pass
- [ ] `TestUnifiedEntry_E2EPreheldReservationDoesNotDoubleReserve` pass
- [ ] `TestRunPoolAllocator_EndToEndParityWithRebalance` pass
- [ ] All pre-existing tests still pass (no regression)
- [ ] `go build` / `go vet` clean
- [ ] Codex re-review on the fix branch returns SHIP

---

## 8. Files Changed

**New:**
- `internal/engine/unified_capacity_test.go` (if not already present)

**Modified:**
- `internal/engine/unified_capacity.go` — snapshot fields + dynamic pct
- `internal/engine/unified_entry.go` — capacityMu wrap + CapOverride threading + admitUnifiedWinners helper
- `internal/engine/engine.go` — factor `computeDynamicPerpCap` / `computeDynamicSpotCap` helpers
- `internal/engine/unified_entry_test.go` — 4 new tests
- `internal/engine/allocator_parity_test.go` — 1 new test
- `VERSION` → `0.33.1` (patch)
- `CHANGELOG.md` — 0.33.1 entry

Estimated ~600 LoC delta.
