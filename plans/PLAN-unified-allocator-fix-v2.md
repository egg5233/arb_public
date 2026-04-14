# PLAN: Unified Allocator Post-Impl Fix v2

**Status:** DRAFT v15 — applies Codex v14 1 required change (line-number correction in history bullet)
**Revision history:**
- v1 → Codex NEEDS REVISION: cap unit mismatch, missing strategyPct fallback, non-existent helper references, capacityMu insufficient (spot race), dispatchUnifiedPerp pending double-create, wrong parity framing, Summary not cached.
- v2 → applied 8 v1 changes; Codex re-review found 5 plan-to-code mismatches.
- v3 → applied 5 v2 changes; Codex re-review found 5 internal inconsistencies.
- v4 → applied 5 v3 changes; Codex re-review found 3 spec inconsistencies.
- v5 → applied 3 v4 changes; Codex re-review found 5 deeper issues.
- v6 → applied 5 v5 changes; Codex re-review found 6 residuals.
- v7 → applied 6 v6 changes; Codex re-review found 3 residuals.
- v8 → applied 3 v7 changes; Codex re-review found 2 blockers + 3 citation drifts.
- v9 → applied 5 v8 changes (2 blockers + 3 citation drifts); Codex re-review found 5 plan-text inconsistencies.
- v10 → applied 5 v9 changes; Codex re-review (v10 re-dispatch with embedded plan after v5/v10 cache-mismatch issue) confirmed 5/5 landed; found 5 new plan-text inconsistencies.
- v11 → fixes per codex v10 verbatim: (1) ManualOpen snippet no longer uses `defer e.admissionUnlock()` — explicit unlock after persist; (2) Fix H constructor-plumbing sentence rewritten to remove `*Engine`-ref contradiction; (3) AGENTS.md change note rewritten to match documented spot lock order; (4) "Section 9" references replaced with "Fix H unified occupancy model"; (5) lock-duration risk row updated to reflect full in-lock scope.
- v12 → applied 3 v11 changes; Codex re-review confirmed 3/3 landed; found 3 new plan-text polish items.
- v13 → applied 3 v12 changes; Codex re-review confirmed 3/3 landed; found 1 remaining type-name drift.
- v14 → fixes per codex v13 verbatim: `*risk.Reservation` → `*risk.CapitalReservation` in Fix H pseudocode (2 sites: lines 250 + 298). Matches live type at `internal/risk/allocator.go:34`. Codex stated: "After that fix, I do not see any remaining plan-text inconsistencies in v13."
- v15 → codex v14 identified stale line numbers in v14 history bullet (said `249`/`297` but actual lines were `250`/`298` after content shift). Corrected in this entry.
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

Align unified selection's cap decisions with the live allocator's actual cap enforcement, and close the entry-admission race window.

**Flag-off behavior note (v10 clarification):** The unified entry selection path (`runUnifiedEntrySelection`) remains flag-gated behind `EnableUnifiedEntrySelection` and has NO user-visible behavior change when the flag is OFF. However, Fix H's spot-side admission changes apply to `SpotEngine.ManualOpen` and `OpenSelectedEntry` REGARDLESS of the unified flag, because they are wired independently of unified selection (`cmd/main.go:432-438`). Those changes are documented as intentional behavior changes in Section 6 (Risks).

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
       // CapitalSummary has NO TotalCap field (allocator.go:48-55); the real total-cap source is
       // cfg.MaxTotalExposureUSDT used inside checkCapsWithOverride() at allocator.go:561-583.
       summary, err := e.allocator.Summary()
       if err != nil {
           return nil, fmt.Errorf("allocator summary: %w", err)
       }
       
       // Reproduce strategyPct() fallback: if dynamic is 0, fall back to static config
       effectivePerp := summary.EffectivePerpPct
       if effectivePerp == 0 { effectivePerp = e.cfg.MaxPerpPerpPct }
       effectiveSpot := summary.EffectiveSpotPct
       if effectiveSpot == 0 { effectiveSpot = e.cfg.MaxSpotFuturesPct }
       
       // Overrides: reuse the live allocator.DynamicStrategyPct API with its real signature
       // (allocator.go:785-830): DynamicStrategyPct(strategy, perpHasOpps, spotHasOpps, committed).
       // Pass `committed` from the already-loaded summary so no extra loadState() happens.
       // No allocator API change needed.
       committed := summary.ByStrategy  // map[risk.Strategy]float64 from the single Summary call
       perpOverride := e.allocator.DynamicStrategyPct(risk.StrategyPerpPerp, perpHasOpps, spotHasOpps, committed)
       spotOverride := e.allocator.DynamicStrategyPct(risk.StrategySpotFutures, perpHasOpps, spotHasOpps, committed)
       
       return &unifiedCapacitySnapshot{
           TotalCap:         e.cfg.MaxTotalExposureUSDT,  // NOT summary.TotalCap (doesn't exist)
           CommittedPerp:    summary.ByStrategy[risk.StrategyPerpPerp],
           CommittedSpot:    summary.ByStrategy[risk.StrategySpotFutures],
           EffectivePerpPct: effectivePerp,
           EffectiveSpotPct: effectiveSpot,
           PerpPctOverride:  perpOverride,
           SpotPctOverride:  spotOverride,
       }, nil
   }
   ```
   
   Caller in `runUnifiedEntrySelection` MUST call `e.updateAllocation()` BEFORE `buildCapacitySnapshot` (matches legacy non-unified path at `engine.go:1276-1288` — without this, dynamic pcts stay 0/stale).
   
   Caller must supply `perpHasOpps = len(perpOpps) > 0` and `spotHasOpps = len(spotPlans) > 0` from `runUnifiedEntrySelection` before calling `buildCapacitySnapshot`.

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
   `SpotEngine` has NO `*Engine` reference today, so the shared admission lock must be plumbed explicitly. Chosen path: pass `*sync.Mutex` into `NewSpotEngine` (avoids adding a `*Engine` reference and its cycle risk).

   - **`internal/engine/engine.go`** — Engine struct adds `admissionMu sync.Mutex` field as a **value** (not pointer). Replace `capacityMu` field entirely. Expose accessor `func (e *Engine) AdmissionMutex() *sync.Mutex { return &e.admissionMu }` that returns the address. Value field means existing `&Engine{}` test literals keep working without explicit init.
   - **Replace ALL `e.capacityMu.*` calls in `internal/engine/engine.go`** at:
     - `:407-485` `ManualOpen` — `e.capacityMu.Lock/Unlock` → `e.admissionMu.Lock/Unlock`
     - `:2126-2427` `executeArbitrage` — same
     - Any other `capacityMu` call sites (audit via grep)
   - **`internal/spotengine/engine.go`** — `NewSpotEngine` adds `admissionMu *sync.Mutex` parameter. SpotEngine struct stores it. **Nil-safe helpers** (existing `&SpotEngine{}` test literals pass nil — must not panic):
     ```go
     func (e *SpotEngine) admissionLock() {
         if e.admissionMu != nil { e.admissionMu.Lock() }
     }
     func (e *SpotEngine) admissionUnlock() {
         if e.admissionMu != nil { e.admissionMu.Unlock() }
     }
     ```
     Without cross-strategy serialization (i.e. in tests with nil admissionMu), spot admission falls back to its existing spotEntryLock — safe because tests don't have a concurrent Engine instance.
   - **`internal/spotengine/occupancy.go` (NEW FILE)** — add spot-side unified-occupancy helper that MIRRORS `internal/engine/unified_state.go:70-128` semantics so spot admission agrees with unified admission:
     ```go
     // loadUnifiedAdmission reads the perp and spot position stores and returns
     // a point-in-time view for spot-side admission (ManualOpen / OpenSelectedEntry).
     // Mirrors engine.loadUnifiedOccupancy — MUST be called under admissionMu.
     func (e *SpotEngine) loadUnifiedAdmission() (
         activePerp int,
         activeSpot int,
         occupiedSymbols map[string]struct{},
         err error,
     ) {
         occupiedSymbols = make(map[string]struct{})
         perpPos, err := e.db.GetActivePositions()
         if err != nil { return 0, 0, nil, fmt.Errorf("GetActivePositions: %w", err) }
         for _, p := range perpPos {
             if p == nil || p.Status == models.StatusClosed { continue }
             activePerp++
             if p.Symbol != "" { occupiedSymbols[p.Symbol] = struct{}{} }
         }
         spotPos, err := e.db.GetActiveSpotPositions()
         if err != nil { return 0, 0, nil, fmt.Errorf("GetActiveSpotPositions: %w", err) }
         for _, sp := range spotPos {
             if sp == nil || sp.Status == models.SpotStatusClosed { continue }
             activeSpot++
             if sp.Symbol != "" { occupiedSymbols[sp.Symbol] = struct{}{} }
         }
         return activePerp, activeSpot, occupiedSymbols, nil
     }
     ```
     Status filters match `engine.loadUnifiedOccupancy` exactly. Must stay in-sync — any divergence lets spot admission approve what unified admission would block.

   - **`internal/spotengine/execution.go:143-157`** `ManualOpen` — REPLACE the spot-only duplicate + capacity checks. Acquire `admissionMu` BEFORE reading occupancy (NOT just reserve+save window — Codex v8 blocker 1):
     ```go
     e.admissionLock()
     // NOTE: defer NOT used — lock spans duplicate/capacity → reserve → persist ONLY;
     // release explicitly BEFORE long-running execution (leverage set, orderbook
     // fetch, order placement). ALL post-lock failure paths must release the lock
     // AND any owned reservation via the local helper below.
     
     var reservation *risk.CapitalReservation  // nil until reserveSpotCapital succeeds
     failAdmission := func(err error) error {
         if reservation != nil { e.releaseSpotReservation(reservation) }
         e.admissionUnlock()
         return err
     }
     
     activePerp, activeSpot, occupied, lerr := e.loadUnifiedAdmission()
     if lerr != nil { return failAdmission(fmt.Errorf("load unified admission: %w", lerr)) }
     if _, ok := occupied[symbol]; ok {
         return failAdmission(fmt.Errorf("position for %s already open (cross-strategy)", symbol))
     }
     // Combined global cap (matches unified admission at engine/unified_state.go:115-119)
     if activePerp + activeSpot >= e.cfg.MaxPositions {
         return failAdmission(fmt.Errorf("at global capacity (%d/%d)", activePerp+activeSpot, e.cfg.MaxPositions))
     }
     // Spot-only sub-cap
     if activeSpot >= e.cfg.SpotFuturesMaxPositions {
         return failAdmission(fmt.Errorf("at spot capacity (%d/%d)", activeSpot, e.cfg.SpotFuturesMaxPositions))
     }
     // --- Reservation + persist (still under admissionMu) ---
     var rerr error
     reservation, rerr = e.reserveSpotCapital(exchName, plannedNotional, 0)
     if rerr != nil {
         // reservation is nil here; failAdmission will just unlock
         return failAdmission(fmt.Errorf("capital allocator rejected: %w", rerr))
     }
     if err := requireEntryLock("leverage setup"); err != nil {
         return failAdmission(err)  // releases reservation + unlocks
     }
     if err := e.persistPendingEntry(entryPos, ""); err != nil {
         return failAdmission(fmt.Errorf("failed to persist pending entry: %w", err))
     }
     // --- Admission complete: release lock before long-running execution ---
     e.admissionUnlock()
     // reservation intentionally NOT released here — commitSpotFromPreheld or
     // releaseSpotReservation happens in the post-execution error handling path.
     // ... long-running execution: SetLeverage, GetOrderbook, PlaceOrder, etc. — NOT under admissionMu ...
     ```
     Remove the pre-existing `GetActiveSpotPositions` + `SpotFuturesMaxPositions` check at `:143-157` (superseded). Keep `spotEntryLock` only if it serves a DIFFERENT purpose (e.g., per-symbol orderbook dedup inside `requireEntryLock`); cross-strategy serialization now covered by `admissionMu`.

   - **`internal/spotengine/selected_entry.go:272-289`** `OpenSelectedEntry` — SAME pattern. Acquire `admissionMu` BEFORE the duplicate+capacity checks (`:272-289`). Lock spans: duplicate/cap checks → reservation setup (`:297-305`, uses preheld OR self-reserve) → `persistPendingEntry` (`:419`). Release `admissionMu` AFTER `persistPendingEntry` returns. Release BEFORE long-running execution (leverage set, orderbook fetch, order placement).
     ```go
     e.admissionLock()
     // NOTE: defer NOT used — release happens explicitly after persist, before execution.
     // ALL post-lock failure paths route through failAdmission to release lock +
     // any owned self-reserved reservation.
     
     var selfReservation *risk.CapitalReservation  // only set if we fall back to reserveSpotCapital
     failAdmission := func(err error) error {
         if selfReservation != nil { e.releaseSpotReservation(selfReservation) }
         e.admissionUnlock()
         return err
     }
     
     activePerp, activeSpot, occupied, lerr := e.loadUnifiedAdmission()
     if lerr != nil { return failAdmission(fmt.Errorf("load unified admission: %w", lerr)) }
     if _, ok := occupied[symbol]; ok {
         return failAdmission(fmt.Errorf("OpenSelectedEntry: %s already open (cross-strategy)", symbol))
     }
     if activePerp + activeSpot >= e.cfg.MaxPositions {
         return failAdmission(fmt.Errorf("OpenSelectedEntry: at global capacity (%d/%d)", activePerp+activeSpot, e.cfg.MaxPositions))
     }
     if activeSpot >= e.cfg.SpotFuturesMaxPositions {
         return failAdmission(fmt.Errorf("OpenSelectedEntry: at spot capacity (%d/%d)", activeSpot, e.cfg.SpotFuturesMaxPositions))
     }
     // --- Reservation (preheld or self-reserve fallback — still under admissionMu) ---
     reservation := preheld
     if reservation == nil {
         var rerr error
         selfReservation, rerr = e.reserveSpotCapital(exchName, plan.PlannedNotionalUSDT, capOverride)
         if rerr != nil {
             return failAdmission(fmt.Errorf("OpenSelectedEntry: capital allocator rejected: %w", rerr))
         }
         reservation = selfReservation
     }
     if err := e.persistPendingEntry(...); err != nil {
         // if preheld: caller (runUnifiedEntrySelection) owns the reservation and
         // will release via ReleaseBatch; if self-reserved: failAdmission releases it.
         return failAdmission(fmt.Errorf("OpenSelectedEntry: persist pending: %w", err))
     }
     // --- Admission complete ---
     e.admissionUnlock()
     // ... long-running execution (leverage, orderbook, orders) — NOT under admissionMu ...
     ```
     When called from `runUnifiedEntrySelection` dispatch, caller releases `admissionMu` BEFORE calling `OpenSelectedEntry` (dispatch runs post-admission — matches legacy `executeArbitrage` at `engine.go:2424-2455`). `OpenSelectedEntry` then re-acquires on its own admission window. NOT nested; NOT reentrant.
   - **`cmd/main.go:432`** — update `NewSpotEngine` call to pass `eng.AdmissionMutex()` (new accessor on Engine returning `*sync.Mutex`):
     ```go
     // engine.go new accessor:
     func (e *Engine) AdmissionMutex() *sync.Mutex { return &e.admissionMu }
     
     // cmd/main.go:432:
     spotEng = spotengine.NewSpotEngine(exchanges, db, apiSrv, cfg, allocator, tg, eng.AdmissionMutex())
     ```
   - **Test callers of `NewSpotEngine`:** grep confirms only `cmd/main.go:432` is the live caller; no test file constructs `NewSpotEngine` directly. If the implementation later adds new spotengine tests that need a real `NewSpotEngine`, they must construct `&sync.Mutex{}` to pass. Existing spotengine tests use direct struct construction with stub fields, NOT `NewSpotEngine` — verify before assuming wider impact.

   **Lock ordering invariant (v10 clarification):** `admissionMu` is NEVER nested across callers. No caller that holds `admissionMu` invokes a function that re-acquires it. `runUnifiedEntrySelection` releases `admissionMu` BEFORE dispatch goroutines spawn (matches legacy `executeArbitrage` releasing before exec at `engine.go:2424-2455`); dispatched `OpenSelectedEntry` re-acquires on its own.
   
   **Interaction with existing `spotEntryLock`:** The pre-existing `spotEntryLock` (per-symbol orderbook / entry dedup in `internal/spotengine/execution.go:67-78` and `selected_entry.go:237-248`) remains the OUTER lock in spot paths — effective acquisition order is `spotEntryLock` → `admissionMu`. This is safe: no caller holds `admissionMu` while trying to acquire `spotEntryLock` (the inverse order would require a perp path to call into spot, which does not happen). `spotEntryLock`'s purpose (per-symbol orderbook dedup inside `requireEntryLock`) is orthogonal to `admissionMu`'s purpose (cross-strategy admission atomicity), so both coexist without deadlock risk.

2. `runUnifiedEntrySelection` structure with helper:
   ```go
   func (e *Engine) runUnifiedEntrySelection() error {
       // readiness gate, log, opp gathering (no lock)
       // Preserve the existing tier-3 override-salvage wrapper. Do NOT bypass to raw
       // consumeOverridesAndEnrichOpps — that loses the stale-override → original-opps
       // fallback path (engine/unified_entry.go:119-120, :322-331).
       originalPerpOpps := e.discovery.GetOpportunities()
       perpOpps, tier := e.selectUnifiedPerpOpps(originalPerpOpps)
       
       // Spot: ListEntryCandidates returns []SpotEntryCandidate (no plans yet);
       // BuildEntryPlan turns each into SpotEntryPlan with sizing/fees/borrow checks.
       // Plans are built BEFORE admission lock (read-only, network / exchange-metadata queries).
       spotRaw := e.spotEntry.ListEntryCandidates(...)
       spotPlans := make([]*models.SpotEntryPlan, 0, len(spotRaw))
       for _, c := range spotRaw {
           plan, err := e.spotEntry.BuildEntryPlan(c)
           if err != nil {
               e.log.Warn("BuildEntryPlan %s/%s: %v", c.Exchange, c.Symbol, err)
               continue
           }
           spotPlans = append(spotPlans, plan)
       }
       
       winners, batch, pendingRecs, err := e.admitUnifiedWinners(perpOpps, spotPlans)
       if err != nil { return err }
       if len(winners) == 0 { return nil }
       
       return e.dispatchUnifiedWinners(winners, batch, pendingRecs)
   }
   
   func (e *Engine) admitUnifiedWinners(
       perpOpps []models.Opportunity,
       spotPlans []*models.SpotEntryPlan,  // plans, NOT raw candidates (BuildEntryPlan ran before admission)
   ) (winners []*unifiedEntryChoice, batch *risk.BatchReservation, pending map[string]*models.ArbitragePosition, err error) {
       // CRITICAL: all admission decisions (occupancy, approval revalidation, solver,
       // reserve, pending-save) must happen under this lock. Candidate gathering
       // BEFORE admissionMu is OK (opps/spotPlans are read-only), but any
       // feasibility check that reads the active set or allocator state must be
       // in-lock.
       e.admissionMu.Lock()
       defer e.admissionMu.Unlock()
       
       // Must call updateAllocation() before snapshot — without it, dynamic pcts stay stale.
       e.updateAllocation()
       
       occupancy, occErr := e.loadUnifiedOccupancy()
       if occErr != nil {
           return nil, nil, nil, fmt.Errorf("loadUnifiedOccupancy: %w", occErr)
       }
       perpHasOpps := len(perpOpps) > 0
       spotHasOpps := len(spotPlans) > 0
       snapshot, snapErr := e.buildCapacitySnapshot(perpHasOpps, spotHasOpps)
       if snapErr != nil {
           return nil, nil, nil, fmt.Errorf("buildCapacitySnapshot: %w", snapErr)
       }
       choices := e.buildUnifiedChoices(perpOpps, spotPlans, occupancy, snapshot)
       
       // Use live allocator timeout (cfg.AllocatorTimeoutMs with 30ms fallback),
       // NOT hardcoded 30s. Matches current unified_entry.go:207-212.
       timeout := time.Duration(e.cfg.AllocatorTimeoutMs) * time.Millisecond
       if timeout <= 0 { timeout = 30 * time.Millisecond }
       winnerKeys := solveGroupedSearch(choices, occupancy.GlobalSlotsRemaining, timeout, makeUnifiedEvaluator(choices, occupancy, snapshot))
       winners = filterWinners(choices, winnerKeys)
       
       // In-lock cross-strategy revalidation: re-read perp + spot position stores,
       // enforce cross-strategy symbol exclusion and global cfg.MaxPositions before
       // reserve. Blocks race where a concurrent ManualOpen (perp or spot) consumed
       // the last slot or opened the same symbol between snapshot and here.
       finalOccupancy, rerr := e.loadUnifiedOccupancy()
       if rerr != nil { return nil, nil, nil, rerr }
       winners = e.filterWinnersAgainstOccupancy(winners, finalOccupancy)  // drop any that now collide
       if len(winners) == 0 { return nil, nil, nil, nil }  // all raced — nothing to reserve
       
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
                   // Route peer cleanup through abandonPendingPerp (single source of truth).
                   // That helper does: Status=Closed + FailureReason + FailureStage +
                   // ExitReason + AddToHistory + SavePosition + Broadcast (nil-guarded).
                   for _, prevPos := range pending {
                       e.abandonPendingPerp(prevPos, "batch_admission_rollback: peer save failed")
                   }
                   e.allocator.ReleaseBatch(batch)
                   return nil, nil, nil, err
               }
               if e.api != nil {
                   e.api.BroadcastPositionUpdate(pos)  // preserve broadcast (unified_entry.go:597-599 guarded)
               }
               pending[w.Key] = pos
           }
       }
       return winners, batch, pending, nil
   }
   ```

3. **Pending lifecycle preservation** — moving pending-save to admission must preserve ALL current side effects:
   
   a. **Broadcast after admission save (nil-guarded).** Current `dispatchUnifiedPerp` does `BroadcastPositionUpdate(pendingPos)` at `unified_entry.go:597-599` AFTER a `if e.api != nil` guard. After move, `admitUnifiedWinners` must broadcast each created pending record with the SAME nil guard (test harness constructs `&Engine{}` with `api=nil` at `unified_entry_test.go:115-120`):
      ```go
      if err := e.db.SavePosition(pos); err != nil { /* rollback via abandonPendingPerp... */ }
      if e.api != nil {
          e.api.BroadcastPositionUpdate(pos)  // nil-guarded, preserves tests
      }
      pending[w.Key] = pos
      ```
   
   b. **`dispatchUnifiedPerp` new signature consumes pre-created pos:**
      ```go
      func (e *Engine) dispatchUnifiedPerp(
          w *unifiedEntryChoice,
          batch *risk.BatchReservation,
          pos *models.ArbitragePosition,  // pre-created during admission, non-nil
      ) error {
          // use pos directly; do NOT call db.SavePosition again for pending
          // continue from executeTradeV2WithPos as today
      }
      ```
   
   c. **Early-return cleanup (CRITICAL).** All pre-execute early-return paths must now mark the pre-created pending as failed (legacy abandon pattern). These are:
      - **Acquire-lock error** at current `unified_entry.go:579` — must call `abandonPendingPerp(pos, "entry_lock_acquire_error")` before return
      - **Lock busy / already held** at `:580-582` — same abandonment
      - **`cfg.DryRun`** at `:586` — same abandonment (pending should never survive a dry-run tick)
      - **Any other pre-`executeTradeV2WithPos` abort** (pre-flight checks, market-data unavailable, etc.) — same
      
      New helper:
      ```go
      func (e *Engine) abandonPendingPerp(pos *models.ArbitragePosition, reason string) {
          pos.Status = models.StatusClosed
          pos.FailureReason = reason
          pos.FailureStage = deriveFailureStage(reason)
          pos.ExitReason = "entry_failed: " + reason
          pos.UpdatedAt = time.Now().UTC()
          if err := e.db.AddToHistory(pos); err != nil {
              e.log.Error("abandonPendingPerp: history %s: %v", pos.ID, err)
          }
          if err := e.db.SavePosition(pos); err != nil {
              e.log.Error("abandonPendingPerp: save %s: %v", pos.ID, err)
          }
          if e.api != nil {
              e.api.BroadcastPositionUpdate(pos)  // nil-guarded; preserves test harness with api=nil
          }
      }
      ```
   
   d. **Regression test** (added to Section 4): `TestUnifiedEntry_EarlyReturnPathsCleanupPendingRecord` — covers acquire-lock error, lock busy, dry-run paths; asserts no orphan pending record survives in the active set.

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
//   - miniredis-backed CapitalAllocator (real implementation — enables Redis key inspection)
//   - stubSpotEntryExecutor blocks inside OpenSelectedEntry on a channel (so the test
//     can snapshot Redis BEFORE runUnifiedEntrySelection reaches ReleaseBatch)
//   - 1 or more spot candidates, 0 perp (or perp disabled) to isolate the handoff
//
// Run runUnifiedEntrySelection in a goroutine.
//
// Key fact: Redis reserved keys are `risk:capital:reserved:{reservationID}` where
// reservationID is generated (not candidate key). Only batch.Items is keyed by
// candidate key. Reservations are created on ReserveBatch and DELETED by
// ReleaseBatch (internal/risk/batch_reservation.go:195-240) which
// runUnifiedEntrySelection calls at unified_entry.go:291-295 after dispatch.
//
// Test lifecycle (two snapshot points):
//   MID-DISPATCH (inside blocked OpenSelectedEntry):
//     1. miniredis has exactly len(winners) keys matching prefix
//        `risk:capital:reserved:` (allocatorReservationPref per allocator.go:30).
//     2. Stub recorded preheld != nil for each winner.
//     3. For each recorded preheld.ID, a Redis key `risk:capital:reserved:{preheld.ID}`
//        exists. batch.Items[candidateKey].ID equals preheld.ID (links candidate to reservation).
//     4. NO committed keys (prefix `risk:capital:committed:`) for these reservations yet.
//   After unblocking the stub + waiting for runUnifiedEntrySelection to return:
//     5a. If stub returned without calling Commit:
//         All reserved keys from snapshot 1 are GONE (ReleaseBatch deleted them).
//         No committed keys exist.
//     5b. If stub explicitly called allocator.Commit(preheld, fakePosID, exposures):
//         Reserved keys are GONE (Commit deletes the reserved key per
//         allocator.go:148-200). Committed totals under
//         `risk:capital:committed:{strategy}:{exchange}` increased by exposure amounts.
//
// Why observable Redis vs spy: Engine/SpotEngine hold concrete *risk.CapitalAllocator
// fields (not interfaces), so a spy would require an interface refactor. Redis state
// is already observable via miniredis and matches the live contract.
// Complements: internal/spotengine/selected_entry_test.go:677-724 (inner preheld branch).
```

**Perp E2E note (non-blocking):** if later we need perp E2E coverage, add a narrow test-only dispatch hook patterned after `rescannerOverride` (`internal/engine/consume_overrides.go:16-27`). NOT in scope for v2 — Codex recommends spot-only for this fix.

### Fix J — Richer allocator fixture with non-zero transfer fees (frozen, no before/after)

**Live code facts (per Codex):**
- Existing parity suite pattern is frozen-fixture at `internal/engine/allocator_parity_test.go:12-30`. "Before/after" framing in v1 was wrong.
- `runPoolAllocator` full-stack test requires `buildAllocatorCandidates()` (`internal/engine/allocator.go:416-508`) + `risk.SimulateApprovalForPair()`. `runPoolAllocator` itself is at `internal/engine/allocator.go:195-243`. No existing test override exists, so stubbing risk manager + exchanges is needed — expensive.
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
| `TestSpotManualOpen_CrossStrategySymbolExclusion` | `internal/spotengine/execution_test.go` | active perp position on `BTCUSDT` BLOCKS spot `ManualOpen(BTCUSDT)` via `loadUnifiedAdmission()` cross-strategy symbol check (error message includes "cross-strategy") |
| `TestSpotManualOpen_CombinedGlobalCapacity` | `internal/spotengine/execution_test.go` | active perp consumes last global slot (`cfg.MaxPositions`); spot `ManualOpen` rejected with "at global capacity" even when `SpotFuturesMaxPositions` has headroom |
| `TestSpotManualOpen_AdmissionLockBlocksStaleReads` | `internal/spotengine/execution_test.go` | concurrent goroutine A holds `admissionMu` + inserts a perp position; spot `ManualOpen` goroutine B starts WHILE A still holds the lock (B must BLOCK waiting on admissionMu); goroutine A releases the lock; B then resumes, observes the new perp position via `loadUnifiedAdmission()`, and rejects. Proves admissionMu prevents the pre-lock stale-read race (B's admission read happens AFTER lock acquisition, NOT before). Synchronize via shared channel or `sync.WaitGroup`. |
| `TestSpotOpenSelectedEntry_CrossStrategySymbolExclusion` | `internal/spotengine/selected_entry_test.go` | active perp on `BTCUSDT` blocks `OpenSelectedEntry(plan[BTCUSDT], preheld)` even if preheld reservation is valid; error bubbles up before reservation state mutated |
| `TestLoadUnifiedAdmission_PerpOnly` | `internal/spotengine/occupancy_test.go` | with only perp positions (mix of closed + non-closed), `loadUnifiedAdmission()` returns correct `activePerp`, `activeSpot=0`, and `occupiedSymbols` matching non-closed perp symbols |
| `TestLoadUnifiedAdmission_SpotOnly` | `internal/spotengine/occupancy_test.go` | with only spot positions (mix of closed + non-closed), returns `activePerp=0`, correct `activeSpot`, `occupiedSymbols` matching non-closed spot symbols |
| `TestLoadUnifiedAdmission_CrossStrategyUnion` | `internal/spotengine/occupancy_test.go` | with perp BTCUSDT + spot ETHUSDT both active, returns `activePerp=1`, `activeSpot=1`, `occupiedSymbols = {BTCUSDT, ETHUSDT}` — mirrors `engine.loadUnifiedOccupancy` semantics |
| `TestUnifiedEntry_PendingPositionRollbackOnPartialFailure` | `unified_entry_test.go` | when 2 perp pending saves succeed and 3rd fails, the 2 prior records get close-and-history rollback (Status=Closed + FailureReason + FailureStage + ExitReason + AddToHistory + SavePosition), batch released, no orphans in active set |
| `TestUnifiedEntry_DispatchUsesPreCreatedPendingNotNew` | `unified_entry_test.go` | `dispatchUnifiedPerp` consumes `pending[w.Key]`; never calls `db.SavePosition` for a NEW pending record itself (spy on SavePosition counts only the admission-time saves) |
| `TestUnifiedEntry_EarlyReturnPathsCleanupPendingRecord` | `unified_entry_test.go` | pre-execute early-return paths (acquire-lock error / lock busy / `cfg.DryRun`) all call `abandonPendingPerp` — no orphan pending row survives in the active set |
| `TestUnifiedEntry_E2ESpotPreheldHandoff` | `unified_entry_test.go` | See Fix I — orchestrator → ReserveBatch → batch.Items[key] → OpenSelectedEntry (spot-only, uses existing stubSpotEntryExecutor) |
| `TestAllocator_FrozenFixtureNonZeroTransferFees` | `allocator_parity_test.go` | See Fix J — frozen fixture exercises Pass-2 cross-exchange transfer with non-zero withdraw fee |

---

## 5. Rollback

All 4 fixes are additive or local — no schema changes. Rollback = `git revert <fix commit>`. Unified entry selection (`runUnifiedEntrySelection`) remains flag-gated and unchanged when flag is OFF. Note: Fix H's spot admission changes (`ManualOpen` / `OpenSelectedEntry` now enforce cross-strategy exclusion + combined cap) apply regardless of flag — rollback via revert restores the spot-only admission semantics.

Note: Fix H **replaces** `capacityMu` field with `admissionMu sync.Mutex` VALUE (single chosen path — not "rename or add alongside"; not a pointer). `AdmissionMutex() *sync.Mutex` accessor returns `&e.admissionMu`. Legacy perp callers `ManualOpen` (`engine.go:407-485`) and `executeArbitrage` (`engine.go:2126-2427`) update in the same commit. No coexistence period.

---

## 6. Risks

| Risk | Mitigation |
|---|---|
| Dynamic pct query adds Redis latency (per-tick) | `Summary()` is NOT cached today (`risk/allocator.go:346-357` — corrected from v1 which wrongly claimed 1s cache). Each call re-computes via `loadState()`. Snapshot built ONCE per EntryScan tick, so latency impact = 1× `loadState()` per tick. Measure in practice; if > 500ms, add explicit per-tick cache. |
| `admissionMu` held longer than legacy `capacityMu` (admission covers B&B) | B&B uses `cfg.AllocatorTimeoutMs` with `30ms` fallback (live: `internal/engine/unified_entry.go:207-212`). In-lock scope per Fix H pseudocode: `updateAllocation()` + `loadUnifiedOccupancy()` + `buildCapacitySnapshot()` + `buildUnifiedChoices()` + solver (`solveGroupedSearch` with 30ms fallback) + winner revalidation + `ReserveBatch` + perp pending-save. Realistic end-to-end: Redis I/O (summary + occupancy reads + reserve + saves) typically < 200ms; solver bounded at 30ms; choice-building + revalidation < 50ms. Expected p50 < 300ms, p95 < 600ms — same order of magnitude as legacy admission (~500ms). Measure post-deploy; add candidate-count cap if p95 > 2s. Solver timeout already bounds worst-case tail. |
| `admissionMu` replaces `capacityMu` — lock-ordering mistakes | Invariant: `admissionMu` is NEVER nested across callers (no caller holding `admissionMu` invokes a function that re-acquires it). Unified dispatcher releases `admissionMu` before calling `OpenSelectedEntry`, which re-acquires for its own admission window. NOTE: `spotEntryLock` (pre-existing per-symbol orderbook/entry dedup) remains the OUTER lock in spot paths (`internal/spotengine/execution.go:67-78`, `internal/spotengine/selected_entry.go:237-248`); actual spot acquisition order is `spotEntryLock` → `admissionMu`. This is safe because no caller holds `admissionMu` while trying to acquire `spotEntryLock` (no inverse order possible). |
| Cross-engine admission requires `Engine` → `SpotEngine` lock sharing | CHOSEN: pass `*sync.Mutex` into `NewSpotEngine` constructor (`internal/spotengine/engine.go:73-80`). Engine exposes `AdmissionMutex()` accessor; cmd/main.go:432 wires it. Avoids `*Engine` reference cycle. Grep confirms only cmd/main.go is the live caller — no test callers to update. |
| `dispatchUnifiedPerp` pending-removal breaks existing callers | Audit: only the unified path calls `dispatchUnifiedPerp`; legacy perp uses different dispatch. Single caller — low risk. |
| Frozen fixture parity test (Fix J) requires concrete exchange/risk stubs | Use existing test-doubles in `internal/engine/*_test.go`. If stubs don't exist for `SimulateApprovalForPair`, add a thin test-only interface override OR test `dryRunTransferPlan` directly. |
| Spot admission now blocks cross-strategy symbol duplicates (BEHAVIOR CHANGE) | Previously `spotengine/execution.go:143-152` only blocked same-symbol spot-vs-spot. Fix H adds perp-vs-spot exclusion via `loadUnifiedAdmission()`. Ops note: if a user manually opens perp `BTCUSDT`, spot `ManualOpen(BTCUSDT)` now rejects (was allowed). Unified selector already populates the occupancy set at `internal/engine/unified_state.go:89-92` (perp symbols) and `:109-112` (spot symbols), and applies exclusion at `internal/engine/unified_entry.go:152-153,168-169,428-430,472-474`; so unified entry is unaffected. Document in CHANGELOG + release notes. |
| Spot admission now enforces combined global `cfg.MaxPositions` (BEHAVIOR CHANGE) | Previously spot paths only checked `cfg.SpotFuturesMaxPositions`. Fix H adds combined `cfg.MaxPositions` check matching unified admission. Ops note: if perp + spot total already at `cfg.MaxPositions`, new spot `ManualOpen` rejects (was allowed). Intentional — aligns with Fix H unified occupancy model (`loadUnifiedAdmission` mirrors `engine.loadUnifiedOccupancy`). |

---

---

## 7. Acceptance Criteria

- [ ] `TestUnifiedEntry_SnapshotUsesStrategyPctFallback` pass
- [ ] `TestUnifiedEntry_SnapshotThreadsOppPresenceToDynamicPct` pass
- [ ] `TestUnifiedEntry_EvaluatorHonorsPctOverride` pass
- [ ] `TestUnifiedEntry_BatchReservationItemThreadsPctOverride` pass
- [ ] `TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen` pass (covers BOTH perp and spot ManualOpen)
- [ ] `TestSpotManualOpen_CrossStrategySymbolExclusion` pass
- [ ] `TestSpotManualOpen_CombinedGlobalCapacity` pass
- [ ] `TestSpotManualOpen_AdmissionLockBlocksStaleReads` pass
- [ ] `TestSpotOpenSelectedEntry_CrossStrategySymbolExclusion` pass
- [ ] `TestLoadUnifiedAdmission_PerpOnly` pass
- [ ] `TestLoadUnifiedAdmission_SpotOnly` pass
- [ ] `TestLoadUnifiedAdmission_CrossStrategyUnion` pass
- [ ] `TestUnifiedEntry_PendingPositionRollbackOnPartialFailure` pass
- [ ] `TestUnifiedEntry_DispatchUsesPreCreatedPendingNotNew` pass
- [ ] `TestUnifiedEntry_EarlyReturnPathsCleanupPendingRecord` pass
- [ ] `TestUnifiedEntry_E2ESpotPreheldHandoff` pass
- [ ] `TestAllocator_FrozenFixtureNonZeroTransferFees` pass
- [ ] All pre-existing tests still pass (no regression)
- [ ] `go build` / `go vet` clean
- [ ] Codex re-review on the fix branch returns SHIP

---

## 8. Files Changed

**New:**
- `internal/engine/unified_capacity_test.go` — Fix G snapshot tests
- `internal/spotengine/occupancy.go` — `loadUnifiedAdmission()` helper (mirrors `engine.loadUnifiedOccupancy`)
- `internal/spotengine/occupancy_test.go` — unit tests for unified-occupancy helper (perp-only, spot-only, cross-strategy)

**Modified:**
- `internal/engine/unified_capacity.go` — snapshot carries percentages (`EffectivePerpPct`, `EffectiveSpotPct`, `PerpPctOverride`, `SpotPctOverride`); strategyPct() fallback; `buildCapacitySnapshot(perpHasOpps, spotHasOpps) (*unifiedCapacitySnapshot, error)` handles Summary error
- `internal/engine/unified_entry.go` — `admissionMu` wrap (replaces capacityMu); pct override threading via `BatchReservationItem.CapOverride`; `admitUnifiedWinners` helper returns `pending map`; pending rollback uses existing `SavePosition + AddToHistory` (legacy abandon pattern engine.go:2695-2710); `dispatchUnifiedPerp` consumes pre-created pending instead of creating new
- `internal/engine/engine.go` — REPLACE `capacityMu` field with `admissionMu sync.Mutex` VALUE; `AdmissionMutex() *sync.Mutex` accessor returns `&e.admissionMu`; update all call sites at `:407-485` (`ManualOpen`) and `:2126-2427` (`executeArbitrage`)
- `internal/spotengine/engine.go` — `NewSpotEngine` adds `admissionMu *sync.Mutex` parameter (after existing `*notify.TelegramNotifier`); SpotEngine struct stores it; `admissionLock()/admissionUnlock()` helpers
- `internal/spotengine/execution.go` — `ManualOpen` acquires `admissionMu` BEFORE `:143-157` duplicate/capacity checks (not just reserve+save); REPLACE spot-only `GetActiveSpotPositions`+`SpotFuturesMaxPositions` check with `loadUnifiedAdmission()`-based combined-cap + cross-strategy symbol exclusion; lock spans through `:292-306` reservation + persist; audit `spotEntryLock` for scope overlap (likely kept for per-symbol orderbook dedup, NOT cross-strategy)
- `internal/spotengine/selected_entry.go` — `OpenSelectedEntry` acquires `admissionMu` BEFORE `:272-289` duplicate/capacity checks; uses `loadUnifiedAdmission()`; lock scope covers reservation setup (`:297-305`) + `persistPendingEntry` (`:419`); releases BEFORE long-running execution. NOT nested with unified-dispatcher lock (caller releases first per Fix H invariant)
- `cmd/main.go:432` — pass `eng.AdmissionMutex()` to `NewSpotEngine` constructor (only live caller; no test callers per grep)
- `AGENTS.md` — concurrency section update: `capacityMu` → `admissionMu` rename; note that `admissionMu` is a shared `*sync.Mutex` between `Engine` (value field owner) and `SpotEngine` (holds pointer). Lock-order rules: (a) `admissionMu` is NEVER nested across callers — no caller holding `admissionMu` calls a function that re-acquires it; (b) spot paths keep `spotEntryLock` as the OUTER lock (pre-existing per-symbol orderbook dedup at `internal/spotengine/execution.go:67-78` and `selected_entry.go:237-248`), so effective spot acquisition order is `spotEntryLock → admissionMu`; no caller holds `admissionMu` while acquiring `spotEntryLock` (inverse order impossible), so no deadlock. Perp paths (`ManualOpen`, `executeArbitrage`, unified `runUnifiedEntrySelection`) have no `spotEntryLock` interaction — `admissionMu` is their sole admission lock.
- `internal/engine/unified_entry_test.go` — 7 new tests (see Section 4 table)
- `internal/engine/allocator_parity_test.go` — 1 new test (`TestAllocator_FrozenFixtureNonZeroTransferFees`)
- `VERSION` → `0.33.1` (patch)
- `CHANGELOG.md` — 0.33.1 entry

**NOT modified (uses existing APIs):**
- `internal/database/state.go` — pending rollback uses existing `SavePosition` + `AddToHistory` per legacy abandon at `engine.go:2695-2710`. No new persistence helpers needed.

Estimated ~800 LoC delta (admissionMu plumbing across spotengine + tests).
