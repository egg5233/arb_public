# PLAN: Unified Allocator Post-Impl Fix v2

**Status:** DRAFT v4 — applies Codex v3 5 required changes
**Revision history:**
- v1 → Codex NEEDS REVISION: cap unit mismatch, missing strategyPct fallback, non-existent helper references, capacityMu insufficient (spot race), dispatchUnifiedPerp pending double-create, wrong parity framing, Summary not cached.
- v2 → applied 8 v1 changes; Codex re-review found 5 plan-to-code mismatches.
- v3 → applied 5 v2 changes; Codex re-review found 5 internal inconsistencies.
- v4 → fixes: (1) admitUnifiedWinners pseudocode threads (perpHasOpps, spotHasOpps) and handles Summary error return; (2) NewSpotEngine documented signature uses real `*notify.TelegramNotifier`; only cmd/main.go is the live caller (no test grep hits); (3) pending rollback uses real existing API — `SavePosition(pos)` with `Status=Closed` + `FailureReason="batch_admission_rollback"` + `ExitReason="entry_failed: ..."` + `AddToHistory(pos)` matching legacy abandon at engine.go:2695-2710; no new db helpers needed; (4) Section 5 picks single chosen path: replace `capacityMu` field entirely; (5) Section 8 — db layer untouched (uses existing SavePosition + AddToHistory).
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

### Fix G — Dynamic strategy pct + capOverride threading (units-corrected)

**Files:**
- `internal/engine/unified_capacity.go` — snapshot carries PERCENTAGES (not USDT)
- `internal/engine/unified_entry.go` — `BatchReservationItem.CapOverride` is a percentage passed through

**Live code facts (confirmed via Codex review):**
- `CapitalAllocator.Summary()` at `internal/risk/allocator.go:346-357` is NOT cached; re-computed per call via `loadState()`.
- `Summary().EffectivePerpPct` / `EffectiveSpotPct` can be **zero** until `updateAllocation()` has run at least once.
- `strategyPct()` at `internal/risk/allocator.go:599-621` falls back to static config (`cfg.MaxPerpPerpPct`) when dynamic pct is zero.
- `checkCapsWithOverride()` at `internal/risk/allocator.go:578-583` compares `capOverride` (percentage) against `pct` BEFORE multiplying by `totalCap`. **`CapOverride` is a percentage override, not an absolute USDT ceiling.**
- There is NO existing `computeDynamicPerpCap` / `computeDynamicSpotCap` helper. The real path is `updateAllocation()` at `internal/engine/capital.go:163-224` + `dynamicStrategyPct()` at `internal/engine/capital.go:226-241`.
- Spot manual reserve still passes `0` for CapOverride at `internal/spotengine/execution.go:295` — there is no legacy spot override path to factor.

**Required changes:**

1. `unifiedCapacitySnapshot` carries **percentages**, not USDT caps (one per strategy per tick — not per-candidate):
   ```go
   type unifiedCapacitySnapshot struct {
       TotalCap        float64
       CommittedPerp   float64
       CommittedSpot   float64
       // Effective percentages reproducing strategyPct() fallback semantics
       // at risk/allocator.go:599-621. Zero means "not initialized";
       // caller must fall through to cfg.Max*Pct as static default.
       EffectivePerpPct float64  // from allocator state; 0 if uninitialized
       EffectiveSpotPct float64  // same
       // Per-strategy percentage overrides computed this tick via
       // dynamicStrategyPct() at engine/capital.go:226-241. 0 = no override.
       // Unlike v1 draft, these are NOT per-candidate — they are single
       // per-strategy values applied to ALL reservations in the tick.
       PerpPctOverride  float64  // 0 = no override
       SpotPctOverride  float64  // 0 = no override (new design — no legacy to extract)
   }
   ```

2. `buildCapacitySnapshot` reproduces `strategyPct()` fallback semantics — NOT raw `Summary()`. Real `dynamicStrategyPct` signature requires opp-presence booleans:

   ```go
   // Real signature (verified at internal/engine/capital.go:226-241):
   //   func (e *Engine) dynamicStrategyPct(strategy risk.Strategy, perpHasOpps, spotHasOpps bool) float64
   //
   // NOT a 1-arg version. v3 must thread perpHasOpps/spotHasOpps through.
   
   func (e *Engine) buildCapacitySnapshot(perpHasOpps, spotHasOpps bool) (*unifiedCapacitySnapshot, error) {
       // Summary() returns (*CapitalSummary, error) per allocator.go:346-357 — must handle both.
       summary, err := e.allocator.Summary()
       if err != nil {
           return nil, fmt.Errorf("allocator summary: %w", err)
       }
       
       // Reproduce strategyPct() fallback: if dynamic is 0, fall back to static config
       effectivePerp := summary.EffectivePerpPct
       if effectivePerp == 0 { effectivePerp = e.cfg.MaxPerpPerpPct }
       effectiveSpot := summary.EffectiveSpotPct
       if effectiveSpot == 0 { effectiveSpot = e.cfg.MaxSpotFuturesPct }
       
       // Compute overrides via the SAME helper legacy perp uses, with opp-presence
       // inputs determined from this tick's actual candidate gathering.
       perpOverride := e.dynamicStrategyPct(risk.StrategyPerpPerp, perpHasOpps, spotHasOpps)
       spotOverride := e.dynamicStrategyPct(risk.StrategySpotFutures, perpHasOpps, spotHasOpps)
       
       return &unifiedCapacitySnapshot{
           TotalCap:         summary.TotalCap,
           CommittedPerp:    summary.ByStrategy[risk.StrategyPerpPerp],
           CommittedSpot:    summary.ByStrategy[risk.StrategySpotFutures],
           EffectivePerpPct: effectivePerp,
           EffectiveSpotPct: effectiveSpot,
           PerpPctOverride:  perpOverride,
           SpotPctOverride:  spotOverride,
       }, nil
   }
   ```
   
   Caller must supply `perpHasOpps = len(perpOpps) > 0` and `spotHasOpps = len(spotCands) > 0` from `runUnifiedEntrySelection` before calling `buildCapacitySnapshot`.

3. `makeUnifiedEvaluator.evaluate(keys)` computes caps using percentage math matching `checkCapsWithOverride:578-583`:
   ```go
   func (s *unifiedCapacitySnapshot) strategyCap(strategy risk.Strategy) float64 {
       var pct, override float64
       switch strategy {
       case risk.StrategyPerpPerp:
           pct, override = s.EffectivePerpPct, s.PerpPctOverride
       case risk.StrategySpotFutures:
           pct, override = s.EffectiveSpotPct, s.SpotPctOverride
       }
       // Match checkCapsWithOverride: override is a percentage; max(pct, override)
       // is applied BEFORE multiplying by totalCap.
       effectivePct := pct
       if override > effectivePct { effectivePct = override }
       return s.TotalCap * effectivePct
   }
   ```
   Inside `evaluate(keys)`:
   ```go
   perpCap := snapshot.strategyCap(risk.StrategyPerpPerp)
   spotCap := snapshot.strategyCap(risk.StrategySpotFutures)
   // sum committed + candidate-set exposures per strategy, check against cap
   ```

4. `BatchReservationItem.CapOverride` threading — pass the percentage override:
   ```go
   items = append(items, risk.BatchReservationItem{
       Strategy:    risk.StrategyPerpPerp,
       Exposures:   exposures,
       CapOverride: snapshot.PerpPctOverride,  // percentage, NOT USDT
   })
   // same for spot with snapshot.SpotPctOverride
   ```

**Spot-override note:** there is no legacy spot `CapOverride` path to factor (Codex point 3). Fix G introduces spot percentage override as a **new behavioral extension** — do NOT claim "symmetric extraction from engine.go:1306". Document as new, justify in CHANGELOG.

**Acceptance:** evaluator's per-strategy ceiling = `checkCapsWithOverride()`'s ceiling for the same inputs.

### Fix H — Cross-strategy admission serialization + pending-position handoff

**Live code facts (confirmed via Codex review):**
- Legacy perp `executeArbitrage` holds `capacityMu` across `engine.go:2126-2427` (snapshot → approval → reservation → pending-save), releases BEFORE execution goroutines at `engine.go:2424-2455`.
- Legacy perp `ManualOpen` also acquires `capacityMu` at `engine.go:407-485`.
- `reducePosition` does NOT use `capacityMu`.
- **Spot manual/selected entry uses `spotEntryLock`** (`internal/spotengine/execution.go:143-157`, `:292-306`, `selected_entry.go:237-305`), NOT `capacityMu`. A `capacityMu`-only fix still races cross-strategy.
- `dispatchUnifiedPerp()` at `internal/engine/unified_entry.go:592-599` creates its OWN pending position. If admission also persists pending, dispatch creates a duplicate.

**Required changes:**

1. **Cross-strategy admission lock — concrete plumbing.** Single `admissionMu` shared across perp + spot admission paths.

   **Live spot-engine constructor signature (verified at internal/spotengine/engine.go:73-80):**
   ```go
   func NewSpotEngine(
       exchanges map[string]exchange.Exchange,
       db *database.Client,
       apiSrv *api.Server,
       cfg *config.Config,
       allocator *risk.CapitalAllocator,
       telegram *notify.TelegramNotifier,  // NOT *notify.Telegram
   ) *SpotEngine
   ```
   `SpotEngine` has NO `*Engine` reference. Plumbing must add one.

   **Plumbing changes (chosen path):** pass `*sync.Mutex` (the shared admission lock) into `NewSpotEngine`. Avoids adding `*Engine` ref (cycle risk).

   - **`internal/engine/engine.go`** — Engine struct adds `admissionMu *sync.Mutex` field (pointer, so spot can share). Initialized as `admissionMu: &sync.Mutex{}` in `NewEngine`. Replace `capacityMu` field entirely (legacy `capacityMu` callers below).
   - **Replace ALL `e.capacityMu.*` calls in `internal/engine/engine.go`** at:
     - `:407-485` `ManualOpen` — `e.capacityMu.Lock/Unlock` → `e.admissionMu.Lock/Unlock`
     - `:2126-2427` `executeArbitrage` — same
     - Any other `capacityMu` call sites (audit via grep)
   - **`internal/spotengine/engine.go`** — `NewSpotEngine` adds `admissionMu *sync.Mutex` parameter. SpotEngine struct stores it. New helper:
     ```go
     func (e *SpotEngine) admissionLock()   { e.admissionMu.Lock() }
     func (e *SpotEngine) admissionUnlock() { e.admissionMu.Unlock() }
     ```
   - **`internal/spotengine/execution.go:143-157,292-306`** `ManualOpen` — wrap admission window (reservation + pending save) with `e.admissionLock(); defer e.admissionUnlock()`. Replace existing spot-internal lock if scope overlaps; keep `spotEntryLock` ONLY if it serves a different purpose (per-symbol vs cross-strategy). Audit needed.
   - **`internal/spotengine/selected_entry.go:237-305`** `OpenSelectedEntry` — same admission wrapping. Note: when called from unified dispatcher, lock is ALREADY held by caller — must use a re-entrant pattern OR caller releases before calling. Decision: caller (`runUnifiedEntrySelection`) releases admissionMu BEFORE dispatch; spot's `OpenSelectedEntry` re-acquires for its own dispatch admission. NOT nested.
   - **`cmd/main.go:432`** — update `NewSpotEngine` call to pass `eng.AdmissionMutex()` (new accessor on Engine returning `*sync.Mutex`):
     ```go
     // engine.go new accessor:
     func (e *Engine) AdmissionMutex() *sync.Mutex { return e.admissionMu }
     
     // cmd/main.go:432:
     spotEng = spotengine.NewSpotEngine(exchanges, db, apiSrv, cfg, allocator, tg, eng.AdmissionMutex())
     ```
   - **Test callers of `NewSpotEngine`:** grep confirms only `cmd/main.go:432` is the live caller; no test file constructs `NewSpotEngine` directly. If the implementation later adds new spotengine tests that need a real `NewSpotEngine`, they must construct `&sync.Mutex{}` to pass. Existing spotengine tests use direct struct construction with stub fields, NOT `NewSpotEngine` — verify before assuming wider impact.

   **Lock ordering invariant:** all admission paths acquire `admissionMu` at TOP level, no nesting. Unified `runUnifiedEntrySelection` releases BEFORE dispatch goroutines spawn (matches legacy `executeArbitrage` releasing before exec at `engine.go:2424-2455`).

2. `runUnifiedEntrySelection` structure with helper:
   ```go
   func (e *Engine) runUnifiedEntrySelection() error {
       // readiness gate, log, opp gathering (no lock)
       perpOpps, tier := e.consumeOverridesAndEnrichOpps(...)
       spotCands := e.spotEntry.ListEntryCandidates(...)
       
       winners, batch, pendingRecs, err := e.admitUnifiedWinners(perpOpps, spotCands)
       if err != nil { return err }
       if len(winners) == 0 { return nil }
       
       return e.dispatchUnifiedWinners(winners, batch, pendingRecs)
   }
   
   func (e *Engine) admitUnifiedWinners(
       perpOpps []models.Opportunity,
       spotCands []*models.SpotEntryPlan,
   ) (winners []*unifiedEntryChoice, batch *risk.BatchReservation, pending map[string]*models.ArbitragePosition, err error) {
       e.admissionMu.Lock()
       defer e.admissionMu.Unlock()
       
       occupancy, _ := e.loadUnifiedOccupancy()
       perpHasOpps := len(perpOpps) > 0
       spotHasOpps := len(spotCands) > 0
       snapshot, snapErr := e.buildCapacitySnapshot(perpHasOpps, spotHasOpps)
       if snapErr != nil {
           return nil, nil, nil, fmt.Errorf("buildCapacitySnapshot: %w", snapErr)
       }
       choices := e.buildUnifiedChoices(perpOpps, spotCands, occupancy, snapshot)
       
       winnerKeys := solveGroupedSearch(choices, occupancy.GlobalSlotsRemaining, 30*time.Second, makeUnifiedEvaluator(choices, occupancy, snapshot))
       winners = filterWinners(choices, winnerKeys)
       
       // Reserve atomically
       items := buildBatchItems(winners, snapshot)
       batch, err = e.allocator.ReserveBatch(items)
       if err != nil { return nil, nil, nil, err }
       
       // Create and persist perp pending records INSIDE the lock (moved from dispatchUnifiedPerp).
       // Uses existing db.SavePosition (writes any status including Pending) — no new helpers.
       pending = make(map[string]*models.ArbitragePosition)
       for _, w := range winners {
           if w.Strategy == risk.StrategyPerpPerp {
               pos := newPendingPerpPosition(w)  // extracted from unified_entry.go:592-599
               // pos.Status = StatusPending set by newPendingPerpPosition
               if err := e.db.SavePosition(pos); err != nil {
                   // Rollback: mark previously-saved pending records as failed using
                   // the legacy abandon pattern (engine.go:2695-2710): set Status=Closed
                   // + FailureReason + ExitReason, AddToHistory, then SavePosition.
                   // Without this, the pending records would orphan in the active set
                   // and falsely consume slot accounting.
                   for prevKey, prevPos := range pending {
                       prevPos.Status = models.StatusClosed
                       prevPos.FailureReason = "batch_admission_rollback: peer save failed"
                       prevPos.ExitReason = "entry_failed: batch_admission_rollback"
                       prevPos.UpdatedAt = time.Now().UTC()
                       if histErr := e.db.AddToHistory(prevPos); histErr != nil {
                           e.log.Error("admit rollback: history write failed for %s (%s): %v", prevKey, prevPos.ID, histErr)
                       }
                       if saveErr := e.db.SavePosition(prevPos); saveErr != nil {
                           e.log.Error("admit rollback: close-state save failed for %s (%s): %v", prevKey, prevPos.ID, saveErr)
                       }
                   }
                   e.allocator.ReleaseBatch(batch)
                   return nil, nil, nil, err
               }
               pending[w.Key] = pos
           }
       }
       return winners, batch, pending, nil
   }
   ```

3. **Remove pending-position creation from `dispatchUnifiedPerp`** (`internal/engine/unified_entry.go:592-599`). Dispatch now consumes the pre-created `pending[w.Key]` position. New dispatch signature:
   ```go
   func (e *Engine) dispatchUnifiedPerp(
       w *unifiedEntryChoice,
       batch *risk.BatchReservation,
       pos *models.ArbitragePosition,  // pre-created during admission, non-nil
   ) error {
       // use pos directly; do NOT call db.SavePendingPosition again
       // continue from executeTradeV2WithPos as today
   }
   ```

4. Admission lock released BEFORE dispatch goroutines fire — match legacy pattern at `engine.go:2424-2455`.

**Acceptance:**
- Unified admission serialized with perp `ManualOpen` / `executeArbitrage` AND spot `ManualOpen` / `OpenSelectedEntry`.
- Perp dispatch never creates a duplicate pending record — always consumes one from the `pending` map.
- No deadlock: all callers acquire `admissionMu` at same logical level (not nested).

### Fix I — E2E preheld reservation test (spot-only, uses existing stub)

**Live code facts (per Codex):**
- Spot HAS a working stub: `stubSpotEntryExecutor` at `internal/engine/unified_entry_test.go:25-76` already lets us spy on `OpenSelectedEntry` calls.
- Perp does NOT have a dispatch hook: `dispatchUnifiedPerp()` at `internal/engine/unified_entry.go:567-625` calls `executeTradeV2WithPos()` directly. Adding a perp hook requires production-surface change for test-only use.
- The ORIGINAL gap (Codex post-fix review) is: orchestrator → `ReserveBatch` → `batch.Items[key]` → `OpenSelectedEntry` handoff. Spot-only E2E covers this gap.

**Required test (spot-only scope):**

`TestUnifiedEntry_E2ESpotPreheldHandoff` in `internal/engine/unified_entry_test.go`:
```go
// Setup:
//   - miniredis-backed CapitalAllocator
//   - stubSpotEntryExecutor spying OpenSelectedEntry(plan, capOverride, preheld *risk.CapitalReservation)
//   - 1 or more spot candidates, 0 perp (or perp disabled) to isolate the handoff
// Run: e.runUnifiedEntrySelection()
// Assert:
//   1. allocator.ReserveBatch called exactly once (count via allocator spy)
//   2. batch.Items contains one reservation per spot winner keyed by candidate key
//   3. each OpenSelectedEntry call received preheld != nil, matching batch.Items[c.Key]
//   4. allocator.Reserve (single) never called during dispatch (count must stay 0 after admission)
// Complements: internal/spotengine/selected_entry_test.go:677-724 which verifies
//   the inner preheld branch of OpenSelectedEntry itself.
```

**Perp E2E note (non-blocking):** if later we need perp E2E coverage, add a narrow test-only dispatch hook patterned after `rescannerOverride` (`internal/engine/consume_overrides.go:16-27`). NOT in scope for v2 — Codex recommends spot-only for this fix.

### Fix J — Richer allocator fixture with non-zero transfer fees (frozen, no before/after)

**Live code facts (per Codex):**
- Existing parity suite pattern is frozen-fixture at `internal/engine/allocator_parity_test.go:12-30`. "Before/after" framing in v1 was wrong.
- `runPoolAllocator` full-stack test requires `buildAllocatorCandidates()` (`allocator.go:195-240`) + `risk.SimulateApprovalForPair()` (`allocator.go:416-470`). No existing test override exists, so stubbing risk manager + exchanges is needed — expensive.
- Minimum fixture to trigger non-zero transfer fees: selected choice leaves recipient deficit after local spot→futures pass (`allocator.go:846-918`), enters cross-exchange Pass 2 (`allocator.go:926-1045`), with recipient chain address configured and donor stub returning valid `GetWithdrawFee()`.

**Required test (frozen fixture, not before/after):**

`TestAllocator_FrozenFixtureNonZeroTransferFees` in `internal/engine/allocator_parity_test.go`:
```go
// Fixture:
//   - 2 candidates: one requires a cross-exchange transfer whose withdraw fee > 0
//   - Recipient exchange configured with a chain address + min withdraw
//   - Donor stub returning a concrete GetWithdrawFee() response
//   - Pre-computed GOLDEN output: exact expected choices + transfer plan + fee cache entries
// Run: solveAllocator + dryRunTransferPlan end-to-end (full stack when possible)
// Assert: choices match golden; total fee matches golden; fee-cache populated
// Purpose: exercise fee-cache + cross-exchange transfer path, which frozen
//   existing fixtures don't cover (they use transferable=10000 everywhere).
```

**If full `runPoolAllocator` harness proves too expensive (Codex alternative):** test `dryRunTransferPlan()` directly with a fixture that triggers cross-exchange transfer with non-zero fee. This still exercises the fee-cache + transfer-fee path without requiring the full B&B + risk manager + exchange stubs.

**Scope note:** this is a NEW fixture, added alongside existing parity tests at `allocator_parity_test.go:12-30`. Does NOT replace them. Does NOT require "before refactor" state (refactor already landed in v0.33.0).

---

## 4. Test Plan

| Test | File | What it verifies |
|---|---|---|
| `TestUnifiedEntry_SnapshotUsesStrategyPctFallback` | `unified_capacity_test.go` | when `Summary().EffectivePerpPct == 0`, snapshot falls back to `cfg.MaxPerpPerpPct` (matches `strategyPct()` at allocator.go:599-621) |
| `TestUnifiedEntry_SnapshotThreadsOppPresenceToDynamicPct` | `unified_capacity_test.go` | `dynamicStrategyPct` called with `(strategy, perpHasOpps, spotHasOpps)` reflecting tick's actual candidate set |
| `TestUnifiedEntry_EvaluatorHonorsPctOverride` | `unified_entry_test.go` | when snapshot has non-zero `PerpPctOverride`, evaluator computes ceiling as `TotalCap * max(EffectivePerpPct, PerpPctOverride)` (percentage units, not USDT) |
| `TestUnifiedEntry_BatchReservationItemThreadsPctOverride` | `unified_entry_test.go` | `BatchReservationItem.CapOverride` carries `snapshot.PerpPctOverride` / `SpotPctOverride` through to `ReserveBatch` |
| `TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen` | `unified_entry_test.go` | while unified admission holds `admissionMu`, concurrent perp `ManualOpen` AND spot `ManualOpen` block until admission finishes (use goroutine + sync.WaitGroup + miniredis + shared `*sync.Mutex` injected into both engines) |
| `TestUnifiedEntry_PendingPositionRollbackOnPartialFailure` | `unified_entry_test.go` | when 2 perp pending saves succeed and 3rd fails, all 3 are deleted before returning; batch released; no orphan rows |
| `TestUnifiedEntry_DispatchUsesPreCreatedPendingNotNew` | `unified_entry_test.go` | `dispatchUnifiedPerp` consumes `pending[w.Key]`, never calls `db.SavePendingPosition` itself (spy with counter on save) |
| `TestUnifiedEntry_E2ESpotPreheldHandoff` | `unified_entry_test.go` | See Fix I — orchestrator → ReserveBatch → batch.Items[key] → OpenSelectedEntry (spot-only, uses existing stubSpotEntryExecutor) |
| `TestAllocator_FrozenFixtureNonZeroTransferFees` | `allocator_parity_test.go` | See Fix J — frozen fixture exercises Pass-2 cross-exchange transfer with non-zero withdraw fee |

---

## 5. Rollback

All 4 fixes are additive or local — no schema changes. Rollback = `git revert <fix commit>`. Flag-gated behavior unchanged; legacy path unaffected.

Note: Fix H **replaces** `capacityMu` field with `admissionMu *sync.Mutex` (single chosen path — not "rename or add alongside"). Legacy perp callers `ManualOpen` (`engine.go:407-485`) and `executeArbitrage` (`engine.go:2126-2427`) update in the same commit. No coexistence period.

---

## 6. Risks

| Risk | Mitigation |
|---|---|
| Dynamic pct query adds Redis latency (per-tick) | `Summary()` is NOT cached today (`risk/allocator.go:346-357` — corrected from v1 which wrongly claimed 1s cache). Each call re-computes via `loadState()`. Snapshot built ONCE per EntryScan tick, so latency impact = 1× `loadState()` per tick. Measure in practice; if > 500ms, add explicit per-tick cache. |
| `admissionMu` held longer than legacy `capacityMu` (admission covers B&B) | B&B has 30s timeout; realistic admission < 3s. Legacy admission ~500ms. Worst-case 6× longer. Acceptable for 10-minute EntryScan cadence. Measure post-deploy; add candidate count cap if > 5s. |
| `admissionMu` replaces `capacityMu` — lock-ordering mistakes | No nested locks (single admission lock at top level). All callers must acquire at same logical level. Enforced by removing `capacityMu` field from Engine and replacing all call sites atomically in one commit. |
| Cross-engine admission requires `Engine` → `SpotEngine` lock sharing | CHOSEN: pass `*sync.Mutex` into `NewSpotEngine` constructor (`internal/spotengine/engine.go:73-102`). Engine exposes `AdmissionMutex()` accessor; cmd/main.go:432 wires it. Avoids `*Engine` reference cycle. All test callers must add the arg. |
| `dispatchUnifiedPerp` pending-removal breaks existing callers | Audit: only the unified path calls `dispatchUnifiedPerp`; legacy perp uses different dispatch. Single caller — low risk. |
| Frozen fixture parity test (Fix J) requires concrete exchange/risk stubs | Use existing test-doubles in `internal/engine/*_test.go`. If stubs don't exist for `SimulateApprovalForPair`, add a thin test-only interface override OR test `dryRunTransferPlan` directly. |

---

---

## 7. Acceptance Criteria

- [ ] `TestUnifiedEntry_SnapshotUsesStrategyPctFallback` pass
- [ ] `TestUnifiedEntry_SnapshotThreadsOppPresenceToDynamicPct` pass
- [ ] `TestUnifiedEntry_EvaluatorHonorsPctOverride` pass
- [ ] `TestUnifiedEntry_BatchReservationItemThreadsPctOverride` pass
- [ ] `TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen` pass (covers BOTH perp and spot ManualOpen)
- [ ] `TestUnifiedEntry_PendingPositionRollbackOnPartialFailure` pass
- [ ] `TestUnifiedEntry_DispatchUsesPreCreatedPendingNotNew` pass
- [ ] `TestUnifiedEntry_E2ESpotPreheldHandoff` pass
- [ ] `TestAllocator_FrozenFixtureNonZeroTransferFees` pass
- [ ] All pre-existing tests still pass (no regression)
- [ ] `go build` / `go vet` clean
- [ ] Codex re-review on the fix branch returns SHIP

---

## 8. Files Changed

**New:**
- `internal/engine/unified_capacity_test.go` — Fix G snapshot tests

**Modified:**
- `internal/engine/unified_capacity.go` — snapshot carries percentages (`EffectivePerpPct`, `EffectiveSpotPct`, `PerpPctOverride`, `SpotPctOverride`); strategyPct() fallback; `buildCapacitySnapshot(perpHasOpps, spotHasOpps) (*unifiedCapacitySnapshot, error)` handles Summary error
- `internal/engine/unified_entry.go` — `admissionMu` wrap (replaces capacityMu); pct override threading via `BatchReservationItem.CapOverride`; `admitUnifiedWinners` helper returns `pending map`; pending rollback uses existing `SavePosition + AddToHistory` (legacy abandon pattern engine.go:2695-2710); `dispatchUnifiedPerp` consumes pre-created pending instead of creating new
- `internal/engine/engine.go` — REPLACE `capacityMu` field with `admissionMu *sync.Mutex` (pointer for sharing); `AdmissionMutex()` accessor; update all call sites at `:407-485` (`ManualOpen`) and `:2126-2427` (`executeArbitrage`)
- `internal/spotengine/engine.go` — `NewSpotEngine` adds `admissionMu *sync.Mutex` parameter (after existing `*notify.TelegramNotifier`); SpotEngine struct stores it; `admissionLock()/admissionUnlock()` helpers
- `internal/spotengine/execution.go` — `ManualOpen` (`:143-157,292-306`) wraps admission window with `admissionLock()`; audit `spotEntryLock` for scope overlap
- `internal/spotengine/selected_entry.go` — `OpenSelectedEntry` (`:237-305`) acquires `admissionLock()` for its own admission (NOT nested with caller's lock — caller releases first)
- `cmd/main.go:432` — pass `eng.AdmissionMutex()` to `NewSpotEngine` constructor (only live caller; no test callers per grep)
- `internal/engine/unified_entry_test.go` — 7 new tests (see Section 4 table)
- `internal/engine/allocator_parity_test.go` — 1 new test (`TestAllocator_FrozenFixtureNonZeroTransferFees`)
- `VERSION` → `0.33.1` (patch)
- `CHANGELOG.md` — 0.33.1 entry

**NOT modified (uses existing APIs):**
- `internal/database/state.go` — pending rollback uses existing `SavePosition` + `AddToHistory` per legacy abandon at `engine.go:2695-2710`. No new persistence helpers needed.

Estimated ~800 LoC delta (admissionMu plumbing across spotengine + tests).
