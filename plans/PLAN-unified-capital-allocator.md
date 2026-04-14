# PLAN: Unified Cross-Strategy Entry Selection (perp-perp + spot-futures)

**Status:** DRAFT v8 — applies Codex v7 documentation fixes
**Author:** claude
**Date:** 2026-04-14
**Revision history:**
- v1 → Codex NEEDS REVISION: wrong architecture, wrong resource model, wrong scoring, wrong direction labels, missing spot engine wiring, missing batch reservation, missing cross-strategy dedup, missing config fields.
- v2 → applies v1 guidance: extracted search core, reframed selector, added SpotEntryPlan, aligned horizon to MinHoldTime, added batch reservation, added symbol dedup.
- v3 → Codex v2 PASS on sections 2/5/7/8; NEEDS REVISION on 1/3/4/6/9/10. v3 applied 6 prescriptive fixes (override application, SpotEntryPlan reshape, borrow cap-then-round, PlannedNotionalUSDT scoring, EntryScanMinute, test names).
- v4 → Codex v3 PASS on 2/3/4/6/7; NEEDS REVISION on 1/5/8/9/10. v4 applied: hard gate + log state, unified vs separate borrow rule, attemptAutoEntries signature, spot status vocabulary, wiring checklist.
- v5 → Codex v4 PASS on 2/4/6/7/9/10; NEEDS REVISION on 1/3/5/8. v5 applied: readiness gate promoted, callback corrected, Dir A >0 guard, test file path/name fixes.
- v6 → Codex v5 PASS on 2/4/5/6/7/9; NEEDS REVISION on 1/3/8/10. v6 applied: readiness gate full normalization, override rescan helper, regression tests, test rename.
- v7 → Codex v6 PASS on 1/2/4/5/6/7/8(gating)/9/10; NEEDS REVISION on section 3 override consume-once design. v7 applied: consumeOverridesAndEnrichOpps helper, applyAllocatorOverridesWithState pure rename, regression tests.
- v8 → Codex v7 PASS on 1/2/4/5/6/7/9/10; NEEDS REVISION only on section 3 doc consistency. Fixes applied: (3) section 3 flow step updated from `applyAllocatorOverrides(perpOpps)` to `consumeOverridesAndEnrichOpps(perpOpps)` to match section 8 helper; (3) section 3 opening paragraph corrected — engine.go:640-642 (write) + 531-536 (stale-clear) is rebalance WRITE path, engine.go:1026-1030, :1248, :1275-1292 is EntryScan CONSUME path.

---

## 1. Scope, Preconditions, and Flags

**Goal.** Given current capital across all exchanges, maximize expected PnL over `cfg.MinHoldTime` (default 16h) by picking the optimal subset of mixed perp-perp and spot-futures candidates via a single cross-strategy entry selector. Runs at `cfg.EntryScanMinute` only (default `40`, configurable).

**User example.** 5 ranked candidates `{spot, perp, perp, perp, spot}`: the solver must pick the subset (could be 2, 3, …) that maximizes combined USDT value over `cfg.MinHoldTime` under per-exchange, per-strategy, and global slot constraints.

**Preconditions (HARD readiness gate, enforced at OWNERSHIP SWITCHES not only inside selector).**

Define a single helper used at every switch point:
```go
func (e *Engine) unifiedOwnerReady() bool {
    return e.cfg.EnableUnifiedEntrySelection &&
           e.cfg.EnableCapitalAllocator &&
           e.allocator != nil &&
           e.allocator.Enabled()
}
```

Apply at **three** places (not just one):

1. **EntryScan ownership switch** (`engine.go` EntryScan branch) — only divert from legacy `executeArbitrage` when `unifiedOwnerReady()`:
   ```go
   case discovery.EntryScan:
       if e.unifiedOwnerReady() {
           if err := e.runUnifiedEntrySelection(); err != nil { ... }
           return
       }
       // fall through to legacy perp entry
       e.executeArbitrage(entryOpps, perpCap)
   ```
   Without this outer gate, `EnableUnifiedEntrySelection=true` + allocator off would suppress legacy perp AND fail the inner reserve — both pathways broken.

2. **Spot auto-entry ownership switch** (`spotengine/autoentry.go:10`) — only suppress legacy spot when `unifiedOwnerReady()`:
   ```go
   func (e *SpotEngine) attemptAutoEntries(opps []SpotArbOpportunity) {
       if e.unifiedOwnerReady() {
           return   // unified owner installed and allocator ready — skip legacy
       }
       // legacy code unchanged
   }
   ```
   Needs a symmetric `SpotEngine.unifiedOwnerReady()` helper reading the same cfg + `e.allocator.Enabled()`.

3. **Inner defense-in-depth** (inside `runUnifiedEntrySelection`) — WARN log + early return if somehow reached with allocator off (covers race between cfg reload and dispatch):
   ```go
   if !e.cfg.EnableCapitalAllocator || e.allocator == nil || !e.allocator.Enabled() {
       e.log.Warn("unified entry: skipped — capital allocator not ready (cfg=%v, alloc=%v, enabled=%v)",
           e.cfg.EnableCapitalAllocator, e.allocator!=nil, e.allocator!=nil && e.allocator.Enabled())
       return nil
   }
   ```

Reason: live `Reserve`/`Commit` paths silently no-op when allocator is off (`internal/risk/allocator.go:92-103`, `internal/engine/capital.go:33-50`, `internal/spotengine/capital.go:9-15`); without the outer gates, flipping `EnableUnifiedEntrySelection=true` while allocator is off would suppress BOTH legacy paths and the "preheld winners" guarantee evaporates.

**Other preconditions:**
- Reservation state must be clean (no stale reservations) at selector entry — covered by existing TTL.
- Spot engine is running (`cmd.spotEng`) and subscribed to opportunities.

**New feature flag.**
- `allocation.enable_unified_entry_selection` (bool, default `false`)
  - When `true` AND `unifiedOwnerReady()` is true: the new selector owns `EntryScan` dispatch (at `cfg.EntryScanMinute`) for both strategies. If the flag is true but readiness fails, legacy paths run AS-IS (defense against misconfiguration).
  - When `false`: existing paths run as today (perp `executeArbitrage(opps)`; spot `autoentry.attemptAutoEntries` loop)

**Out of scope (this plan).**
- Cross-strategy rotation mid-life (spot→perp or perp→spot leg swap)
- Dynamic `effectivePerpPct` / `effectiveSpotPct` tuning (CA-03/CA-04) — unchanged
- Rebalance planner awareness of spot commitments (`rebalanceFunds` stays rebalance-only)

---

## 2. Current Boundaries To Preserve

Nothing in the following list may change as a side effect of this plan:

| Invariant | Why preserved |
|---|---|
| `internal/engine/allocator.go:195` `runPoolAllocator` is **rebalance-only** | It depends on transfer planning and post-transfer override replay; not an entry-time subset solver |
| `internal/engine/allocator.go:764` `dryRunTransferPlan` scope | Rebalance feasibility only |
| `internal/engine/allocator.go:1205` `executeRebalanceFundingPlan` | Transfer executor; not an entry dispatcher |
| `internal/engine/engine.go:632` rebalance override storage | Feeds EntryScan with allocator-picked perp pairs |
| `internal/engine/engine.go:2103` perp duplicate-symbol guard | Last-line safety net for perp execution |
| `internal/spotengine/execution.go:143` spot duplicate / capacity guard | Last-line safety net for spot execution |
| `internal/spotengine/monitor.go:448` `lookupCurrentOpp` | Private monitor path; not selector |
| `internal/spotengine/execution.go:62` `ManualOpen` | Dashboard manual entry path |
| `internal/engine/engine.go:2521` `MergeExistingDuplicates` | Persistence cleanup, not selector policy |
| `internal/risk/allocator.go:95,148,220` legacy `Reserve/Commit/ReleaseReservation` APIs | Kept for legacy callers |
| `internal/risk/allocator.go:435` current spot committed exposure = `pos.NotionalUSDT` | Unit semantics unchanged |

---

## 3. New Cross-Strategy Entry Architecture

Selector is an **independent module** from rebalance solving/execution, but it still **consumes** the rebalance-produced `allocOverrides` when building perp candidate intake. **Rebalance writes** `e.allocOverrides` at `engine.go:640-642` (with stale-clear at `:531-536`) during `RebalanceScan`/`RotateScan` handlers. **EntryScan consumes** overrides at `engine.go:1026-1030` (non-empty opps tier-2 patch path) and `:1275-1292` (empty opps + non-empty overrides → `RescanSymbols` v0.32.8 fallback), both acquiring `allocOverrideMu` and clearing once. Unified selection reuses the same consume-once semantics via a single shared helper (see section 8). The selector **does not mutate** rebalance or any existing allocator code paths.

**Flow.**
```
EntryScanMinute tick fires
 → gather freshly prefiltered perp opps (existing discovery output)
 → perpOpps, tier = consumeOverridesAndEnrichOpps(perpOpps)   # consume-once helper
                                                               # → non-empty + overrides → tier-2 patch
                                                               # → empty + overrides    → RescanSymbols (v0.32.8)
                                                               # → empty overrides      → return unchanged
 → gather freshly prefiltered spot candidates (ListEntryCandidates, new)
 → build perpDispatchRequest[] and SpotEntryPlan[]       # per-candidate feasibility
 → groupChoicesBySymbol                                  # cross-strategy dedup
 → solveGroupedSearch                                    # generic B&B, selection_core.go
 → ReserveBatch                                          # atomic prehold
 → dispatch perp winners via executeArbitrage(preheld=true path)
 → dispatch spot winners via OpenSelectedEntry (no latestOpps lookup)
 → release any unused preholds on failure
```

**New files.**
- `internal/engine/selection_core.go` — generic grouped B&B search; no allocator/transfer logic
- `internal/engine/unified_entry.go` — cross-strategy selector + dispatcher at EntryScan
- `internal/engine/unified_state.go` — `unifiedOccupancy` (active symbols, slot counts across both stores)
- `internal/engine/unified_value.go` — spot scoring (`scoreSpotEntry`) aligned with perp's `computeAllocatorBaseValue`
- `internal/engine/spot_entry_iface.go` — `spotEntryExecutor` interface (`Engine` depends only on this, not concrete `SpotEngine`)
- `internal/models/spot_entry.go` — `SpotEntryCandidate`, `SpotEntryPlan`
- `internal/spotengine/selected_entry.go` — `ListEntryCandidates`, `BuildEntryPlan`, `OpenSelectedEntry`
- `internal/risk/batch_reservation.go` — `BatchReservationItem`, `BatchReservation`, `ReserveBatch`, `ReleaseBatch`

**Engine wiring change.**
- `internal/engine/engine.go:121,289` currently only holds `spotCloseCallback`. Add `spotEntry spotEntryExecutor` field.
- Add setter:
  ```go
  func (e *Engine) SetSpotEntryExecutor(exec spotEntryExecutor) {
      e.spotEntry = exec
  }
  ```
- `cmd/main.go:412-437` startup order must be rewritten to:
  ```
  construct spotEng
   → eng.SetSpotCloseCallback(spotEng.ClosePosition)   # existing — live signature
                                                       # func(*SpotFuturesPosition, string, bool) error
                                                       # NOT spotEng.ManualClose (takes positionID string)
   → eng.SetSpotEntryExecutor(spotEng)                 # NEW (must be before eng.Start)
   → spotEng.Start()                                   # can move before eng.Start too
   → eng.Start()
  ```
  Current order (`eng.Start()` at `cmd/main.go:412` before `spotEng.Start()` + callbacks at `:426-437`) would let `Engine.run()` dispatch `EntryScan` before `spotEntry` is set. Either fix the order, OR add a nil-guard in the `EntryScan` branch:
  ```go
  if e.unifiedOwnerReady() && e.spotEntry != nil { ... }
  ```
  Preferred: fix the order to remove the nil-guard crutch.

---

## 4. Shared Search Core Extraction

Move only the generic search/B&B into a new package-level helper so both the rebalance allocator and the entry selector can call it without sharing rebalance-specific state.

```go
// internal/engine/selection_core.go (new)

// searchChoice is the abstract unit fed to the solver.
type searchChoice struct {
    Key       string   // stable unique id
    GroupKey  string   // symbol — mutual exclusion group
    ValueUSDT float64  // objective; higher is better
    SlotCost  int      // positions consumed (1 per entry)
}

// solveGroupedSearch returns the best subset of keys that:
//   - consumes at most maxSlots slot-cost total
//   - picks at most one choice per GroupKey
//   - maximizes sum of ValueUSDT subject to `evaluate` returning feasible=true
//
// The evaluate callback is where exchange-capacity / strategy-cap / post-trade
// ratio checks live. It is called many times during branch exploration.
//
// Pure search — no rebalance, no transfer, no reservation side effects.
func solveGroupedSearch(
    groups map[string][]searchChoice,
    maxSlots int,
    timeout time.Duration,
    evaluate func(keys []string) (scoreUSDT float64, feasible bool),
) []string {
    // extracted from existing allocator.go solveAllocator + greedy seed
}
```

**Existing code changes.**
- `internal/engine/allocator.go:515` `solveAllocator` — refactor to call `solveGroupedSearch(...)` with a rebalance-specific `evaluate`. Behavior must remain byte-identical for perp rebalance.
- `internal/engine/allocator.go:582` `greedyAllocatorSeed` — move greedy seed into `selection_core.go` as helper used by both callers.

**Tests required.**
- Parity test: rebalance pathway after refactor produces same choices as before on a frozen fixture (guards against regression in allocator.go).

---

## 5. Spot Candidate Plan and Borrow Feasibility

Spot candidates are built from freshly-filtered spot opps and sized per exchange capital, mirroring current execution logic exactly.

```go
// internal/models/spot_entry.go (new)

type SpotEntryCandidate struct {
    Symbol     string    // "BTCUSDT"
    BaseCoin   string    // "BTC"
    Exchange   string
    Direction  string    // "borrow_sell_long" | "buy_spot_short"
    FundingAPR float64   // signed annualized decimal (perp funding on short leg, or long leg for Dir A)
    BorrowAPR  float64   // annualized decimal; 0 for Dir B
    FeePct     float64   // one-time round-trip decimal
    Timestamp  time.Time
}

type SpotEntryPlan struct {
    Candidate                SpotEntryCandidate
    CapitalBudgetUSDT        float64 // from e.capitalForExchange(exchange) — raw budget, pre-cap
    MidPrice                 float64
    PlannedBaseSize          float64 // AFTER Dir A MaxBorrowable cap AND futures step rounding
    PlannedNotionalUSDT      float64 // PlannedBaseSize * MidPrice — the USDT actually reserved
    FuturesMarginUSDT        float64 // PlannedNotionalUSDT / cfg.SpotFuturesLeverage
    MaxBorrowableBase        float64 // 0 when not applicable (Dir B or non-borrow)
    RequiresInternalTransfer bool    // true only on Binance/Bitget in current live code
    TransferTarget           string  // "margin" for Dir A on separate, "spot" for Dir B on separate, "" otherwise
}
```

**Direction mapping (authoritative, from `internal/spotengine/discovery.go:215-216`):**
- `borrow_sell_long` = **Dir A**: long futures + short spot (borrow spot, sell)
- `buy_spot_short` = **Dir B**: short futures + long spot (buy spot)

**BuildEntryPlan** lives in `internal/spotengine/selected_entry.go` and uses:
- `SpotEngine.capitalForExchange(exchange)` — existing at `engine.go:520`, respects `SpotFuturesCapitalSeparate` vs `SpotFuturesCapitalUnified`
- `GetMarginBalance(baseCoin).MaxBorrowable` — existing via `SpotMarginExchange` interface (`pkg/exchange/types.go:326-377`)
- Mid-price snapshot from the **futures orderbook BBO** that live `ManualOpen` uses (`spotengine/execution.go:166-178`) — NOT a generic spot BBO; ensures planning-time price matches execution-time price exactly

**Feasibility rules by account type** (matches live `spotengine/engine.go:455-460` + `execution.go:24-31`):
- **Unified (Bybit UTA / OKX / Gate.io)**: single pool; treat `PlannedNotionalUSDT + FuturesMarginUSDT` as a single debit against `bi.futures` (which represents the unified wallet). `RequiresInternalTransfer=false`, `TransferTarget=""`. Bybit / OKX transfer methods are no-op per `pkg/exchange/bybit/margin.go:335,417-421` / `pkg/exchange/okx/margin.go:304,381-393`; Gate.io transfer methods are also no-op per `pkg/exchange/gateio/margin.go:534-541`.
- **Separate (Binance / Bitget)**: `capitalForExchange` returns USDT drawn from futures wallet via internal transfer at execution time. Feasibility check: `bi.futures >= PlannedNotionalUSDT + FuturesMarginUSDT`. `RequiresInternalTransfer=true`, `TransferTarget="margin"` for Dir A, `"spot"` for Dir B. Do NOT require pre-existing spot balance.
- **BingX**: no spot margin → filter out at candidate builder (no adapter implements `SpotMarginExchange`).

**Dir A borrow feasibility (matches live `execution.go:189-264` exactly).**

The rule depends on account type because live execution differs:

**Unified accounts (Bybit / OKX / Gate.io)** — single pool, no transfer needed. Matches live `execution.go:229-237` exactly (only caps when MaxBorrowable > 0):
```
rawSize := plan.CapitalBudgetUSDT / plan.MidPrice
if plan.MaxBorrowableBase > 0 && rawSize > plan.MaxBorrowableBase {
    rawSize = plan.MaxBorrowableBase                   // CAP only when MaxBorrowable known
}
// When MaxBorrowableBase <= 0 (unknown), do NOT cap — execution caps post-poll
plan.PlannedBaseSize = futuresStepSize(exch, symbol, rawSize)  // round DOWN
plan.PlannedNotionalUSDT = plan.PlannedBaseSize * plan.MidPrice
```

**Separate accounts (Binance / Bitget) Dir A** — execution transfers USDT to margin FIRST, then polls `MaxBorrowable`:
```
rawSize := plan.CapitalBudgetUSDT / plan.MidPrice
plan.PlannedBaseSize = futuresStepSize(exch, symbol, rawSize)  // round only
plan.PlannedNotionalUSDT = plan.PlannedBaseSize * plan.MidPrice
// DO NOT cap by pre-transfer MaxBorrowableBase here — execution polls AFTER
// transfer and caps at that point (execution.go:189-237). Pre-transfer
// MaxBorrowable can be near-zero on Binance/Bitget.
plan.MaxBorrowableBase = 0   // not authoritative pre-transfer
```

**Dir B (all account types)** — no borrow:
```
rawSize := plan.CapitalBudgetUSDT / plan.MidPrice
plan.PlannedBaseSize = futuresStepSize(exch, symbol, rawSize)
plan.PlannedNotionalUSDT = plan.PlannedBaseSize * plan.MidPrice
plan.MaxBorrowableBase = 0
```

**Common rejection criteria (matches `execution.go:255-264`):**
```
futMin := futuresMinSize(exch, symbol)   // 0 if unknown
budgetFloor := math.Max(plan.CapitalBudgetUSDT * 0.10, 5.0)

if plan.PlannedBaseSize <= 0:                      drop
if futMin > 0 && plan.PlannedBaseSize < futMin:    drop
if plan.PlannedNotionalUSDT < budgetFloor:         drop  // NOT venue min
```

`budgetFloor = max(budget*10%, 5.0)` matches live `ManualOpen` rejection. Do NOT use a "venue min notional" — that is not how live code rejects.

**Existing code changes.**
- `internal/spotengine/discovery.go:424` — current borrow filter (`MaxBorrowable > 0`) is size-agnostic. Keep at discovery; selector does size-aware check in `BuildEntryPlan`.
- `internal/spotengine/execution.go:189-210,221-237` — preserve existing transfer-to-margin / transfer-to-spot / post-transfer `MaxBorrowable` poll loop. `OpenSelectedEntry` calls the same execution primitives.

**Must NOT change.**
- `internal/engine/allocator.go:38` `rebalanceBalanceInfo` struct — do not add spot-borrow fields; that type stays rebalance-transfer state.
- `internal/spotengine/execution.go:191-210` transfer behavior.

---

## 6. Objective Normalization and Value Formula

Both perp and spot must produce USDT value over the same horizon so the solver can compare them.

**Horizon:** `cfg.MinHoldTime.Hours()` (default 16h, set at `internal/config/config.go:31`).

**Perp value** (unchanged, uses existing `computeAllocatorBaseValue` at `allocator.go:651`).

**Spot value** (new, `internal/engine/unified_value.go`):
```go
type SpotValueBreakdown struct {
    Direction      string
    HoldHours      float64  // cfg.MinHoldTime.Hours()
    NotionalUSDT   float64  // plan.PlannedNotionalUSDT — NOT CapitalBudgetUSDT
    FundingAPR     float64  // decimal fraction, NOT /100
    BorrowAPR      float64  // decimal fraction, NOT /100
    GrossCarryAPR  float64  // FundingAPR - BorrowAPR   (Dir A carries borrow cost; Dir B: BorrowAPR=0)
    FeePct         float64  // one-time round-trip decimal
    GrossCarryUSDT float64  // GrossCarryAPR * HoldHours/8760 * NotionalUSDT
    FeeUSDT        float64  // FeePct * NotionalUSDT
    NetValueUSDT   float64  // GrossCarryUSDT - FeeUSDT
}

func scoreSpotEntry(plan *models.SpotEntryPlan, cfg *config.Config) SpotValueBreakdown {
    h := cfg.MinHoldTime.Hours()
    n := plan.PlannedNotionalUSDT   // the actual USDT that will be reserved/committed
    gross := (plan.Candidate.FundingAPR - plan.Candidate.BorrowAPR) * h / 8760 * n
    fee := plan.Candidate.FeePct * n
    return SpotValueBreakdown{..., NotionalUSDT: n, NetValueUSDT: gross - fee}
}
```

**Rules:**
- APR and FeePct are **decimal fractions** — no `/100`
- `NetAPR` from `SpotArbOpportunity` is NOT used here (it does not include fees in current code — `discovery.go:247,274,450`). Selector does its own math from FundingAPR/BorrowAPR/FeePct.
- Sign of `FundingAPR`:
  - Dir A (`borrow_sell_long`): funding on LONG futures leg → positive expected value when long receives funding
  - Dir B (`buy_spot_short`): funding on SHORT futures leg → positive expected value when short receives funding
  - `scoreSpotEntry` treats `GrossCarryAPR` as absolute (always positive-expected after correct direction selection at discovery time)

---

## 7. Batch Reservation / Prehold Semantics

Selector must be able to atomically hold capital for multiple winners before any is dispatched, to preserve the "selected subset = executed subset" guarantee.

```go
// internal/risk/batch_reservation.go (new)

type BatchReservationItem struct {
    Key         string            // candidate key (for later Commit or Release)
    Strategy    Strategy
    Exposures   map[string]float64 // same units as Reserve()
    CapOverride float64            // 0 = no override
}

type BatchReservation struct {
    ID        string
    Items     map[string]*CapitalReservation  // key -> individual reservation
    CreatedAt time.Time
    ExpiresAt time.Time
}

func (a *CapitalAllocator) ReserveBatch(items []BatchReservationItem) (*BatchReservation, error) {
    // 1. Fetch committed state once.
    // 2. Validate cumulative caps and per-exchange caps against the batch.
    // 3. If any item fails, return error; NO partial persistence.
    // 4. Otherwise persist every reservation atomically (Redis MULTI or equivalent).
}

func (a *CapitalAllocator) ReleaseBatch(batch *BatchReservation) error {
    // Release every uncommitted reservation in the batch (ignore committed ones).
}
```

**Existing changes.**
- `internal/engine/capital.go:36-54` perp reserve path — extend to accept a preheld reservation instead of reserving again during dispatch. New helper `commitExistingReservation(res, posID, amount)`.
- `internal/spotengine/capital.go:9-15` spot reserve path — symmetric: add `capOverride` param (matches perp's `ReserveWithCap`).
- `internal/spotengine/capital.go` — new `commitSpotFromPreheld(res, posID, amount)`.

**Exposure units (spot).** `BatchReservationItem.Exposures` for spot candidates uses `plan.PlannedNotionalUSDT` keyed by `plan.Candidate.Exchange`. This matches the live `reserveSpotCapital(exchange, notional)` semantics at `internal/spotengine/execution.go:255` and the committed exposure semantic at `internal/risk/allocator.go:428-435` (spot commits store `pos.NotionalUSDT`). Do NOT use `CapitalBudgetUSDT` — the solver must compare against the value that will actually be reserved.

**Exposure units (perp).** Unchanged: per-leg futures margin by exchange, matching existing `engine/capital.go:36-74` path.

**Must NOT change.**
- `internal/risk/allocator.go:95,148,220` existing single-reservation APIs — other legacy callers depend on them.
- `internal/risk/allocator.go:102` `ReserveWithCap` — batch builds on this, does not replace it.
- `internal/risk/allocator.go:435` spot committed exposure semantic (`pos.NotionalUSDT`).

**Guarantee statement.**
With `ReserveBatch`, the selector guarantees "all preheld winners will either be executed or released; no partial capital leak." Without batch, v2 falls back to per-candidate reserve with per-candidate rollback; this plan adopts **batch** to avoid that downgrade.

---

## 8. EntryScan Wiring and Spot Auto-Entry Ownership

**Unified selection replaces both perp and spot entry pathways at `cfg.EntryScanMinute` ONLY when `unifiedOwnerReady()` is true. If the flag is on but readiness fails, legacy paths run.**

```go
// internal/engine/engine.go (EntryScan handler)
case discovery.EntryScan:
    if e.unifiedOwnerReady() {
        if err := e.runUnifiedEntrySelection(); err != nil {
            e.log.Error("unified entry: %v", err)
        }
        return
    }
    // flag off OR readiness not met — legacy perp entry runs unchanged,
    // including the v0.32.8 zero-opp override salvage path below
    e.executeArbitrage(entryOpps, perpCap)
```

**Zero-opportunity override salvage + tier-2 override patching (MUST preserve — v0.32.8 behavior).**
Live EntryScan at `engine.go:1036-1128` applies tier-2 override patching to non-empty opps, and `engine.go:1272-1299` rescans `allocOverrides` via `discovery.RescanSymbols()` when opps is empty but overrides exist. Both behaviors MUST be preserved in the unified path.

**Design — single consume-once helper that handles both branches.** The live code consumes `e.allocOverrides` under `allocOverrideMu` exactly once; we cannot split that consume into two calls. Factor a single helper used by BOTH legacy EntryScan and `runUnifiedEntrySelection`:

```go
// internal/engine/engine.go (new helper — refactored from live lines 1026-1128 + 1272-1299)
//
// consumeOverridesAndEnrichOpps acquires allocOverrideMu, reads+clears
// e.allocOverrides exactly once, then either:
//   1. perpOpps non-empty → apply tier-2 override patching (engine.go:1036-1128 logic)
//   2. perpOpps empty + overrides exist → RescanSymbols fallback (engine.go:1272-1299 logic)
//   3. overrides empty → return perpOpps unchanged
//
// Returns the enriched/rescanned opp list and a tier tag for logging.
// Live state type is map[string]allocatorChoice (internal/engine/allocator.go:168-178).
func (e *Engine) consumeOverridesAndEnrichOpps(perpOpps []models.Opportunity) (out []models.Opportunity, tier string) {
    e.allocOverrideMu.Lock()
    overrides := e.allocOverrides
    e.allocOverrides = nil
    e.allocOverrideMu.Unlock()

    if len(overrides) == 0 {
        return perpOpps, "none"
    }

    if len(perpOpps) > 0 {
        // Non-empty: apply tier-2 patching (move existing engine.go:1036-1128 logic here).
        // Returns (patchedOpps, didPatch bool); translate to tier tag.
        patched, didPatch := e.applyAllocatorOverridesWithState(perpOpps, overrides)
        if didPatch {
            return patched, "tier-2-override-patch"
        }
        return perpOpps, "none"
    }

    // Empty opps: v0.32.8 RescanSymbols fallback (move engine.go:1283-1300 logic here).
    var pairs []models.SymbolPair
    for symbol, choice := range overrides {
        pairs = append(pairs, models.SymbolPair{
            Symbol:        symbol,
            LongExchange:  choice.longExchange,
            ShortExchange: choice.shortExchange,
        })
    }
    fallbackOpps := e.discovery.RescanSymbols(pairs)
    if len(fallbackOpps) > 0 {
        return fallbackOpps, "tier-2-override-fallback"
    }
    return nil, "tier-2-override-fallback-empty"
}
```

**Required refactor of existing live code.** Rename the current `applyAllocatorOverrides` (signature `([]models.Opportunity, bool)`, consumes state via `allocOverrideMu`) to a **pure** helper `applyAllocatorOverridesWithState(opps, overrides) ([]models.Opportunity, bool)` that takes overrides as a parameter instead of reading from engine state. Live callers at `engine.go:1026-1128` in the legacy path must be updated to:
```go
// was: patched, didPatch := e.applyAllocatorOverrides(result.Opps)
// becomes:
patchedOpps, tierTag := e.consumeOverridesAndEnrichOpps(result.Opps)
// legacy tier chain then uses patchedOpps and tierTag
```

Both legacy EntryScan and `runUnifiedEntrySelection` call `consumeOverridesAndEnrichOpps(perpOpps)` exactly once at the perp intake stage. The lock is acquired inside the helper, never outside; no caller may read `e.allocOverrides` directly.

Call at top of `runUnifiedEntrySelection` perp intake:
```go
perpOpps, overrideTier := e.consumeOverridesAndEnrichOpps(perpOpps)
// log tier for observability
```

**Spot auto-entry subsumption (readiness-gated, not flag-only).**
- `internal/spotengine/autoentry.go:10` live signature is `func (e *SpotEngine) attemptAutoEntries(opps []SpotArbOpportunity)`. Add a readiness-gated early return at the top:
  ```go
  func (e *SpotEngine) attemptAutoEntries(opps []SpotArbOpportunity) {
      if e.unifiedOwnerReady() {
          return   // unified owner installed and allocator ready — skip legacy
      }
      // existing code unchanged below — runs when flag off OR readiness fails
  }
  ```
- Add symmetric `SpotEngine.unifiedOwnerReady()` helper reading `e.cfg.EnableUnifiedEntrySelection && e.cfg.EnableCapitalAllocator && e.allocator != nil && e.allocator.Enabled()`.
- The discovery loop calls `attemptAutoEntries(opps)` from 3 places (`spotengine/engine.go:196-197, 214-215, 232-233`). Single early return covers all three.

**Spot accessor (new, replaces raw `latestOpps` access).**
```go
// internal/spotengine/selected_entry.go
func (e *SpotEngine) ListEntryCandidates(maxAge time.Duration) []models.SpotEntryCandidate {
    opps := e.getLatestOpps()
    out := []models.SpotEntryCandidate{}
    for _, o := range opps {
        if o.FilterStatus != "" { continue }
        if time.Since(o.Timestamp) > maxAge { continue }
        out = append(out, models.SpotEntryCandidate{...})  // projection only
    }
    return out
}
```

**Freshness window:** reuse `2 * SpotDiscoveryInterval` (matching existing perp freshness at `engine.go:1220`). `ListEntryCandidates` is the **only** reader-facing accessor; raw `latestOpps` stays private (matches current private cache/getter split at `spotengine/engine.go:34-36, 345-350`).

**Direct open (no `latestOpps` lookup):**
```go
func (e *SpotEngine) OpenSelectedEntry(
    plan *models.SpotEntryPlan,
    capOverride float64,
    preheld *risk.CapitalReservation,
) error {
    // Same execution primitives as ManualOpen — but takes SpotEntryPlan directly,
    // avoids latestOpps symbol/exchange/direction lookup.
    // Uses preheld reservation via commitSpotFromPreheld; no new Reserve call.
}
```

**Must NOT change.**
- `internal/spotengine/execution.go:62` `ManualOpen` — still used by dashboard manual entry.
- `internal/spotengine/monitor.go:448` `lookupCurrentOpp` — monitoring only.

---

## 9. Duplicate-Symbol and Slot Policy

Global across strategies. Symbol dedup happens **inside** the selector via `GroupKey`; engine-side guards remain as last-line safety.

```go
// internal/engine/unified_state.go (new)
type unifiedOccupancy struct {
    ActiveSymbols        map[string]struct{} // union of all non-closed perp + spot symbols. Perp non-closed = pending/partial/active/exiting/closing; Spot non-closed = pending/active/exiting (no partial/closing in spot).
    ActivePerp           int
    ActiveSpot           int
    GlobalSlotsRemaining int                 // cfg.MaxPositions - (ActivePerp + ActiveSpot)
    SpotSlotsRemaining   int                 // cfg.SpotFuturesMaxPositions - ActiveSpot
}

func (e *Engine) loadUnifiedOccupancy() (*unifiedOccupancy, error) {
    perpActive, _ := e.db.GetActivePositions()        // perp non-closed: pending/partial/active/exiting/closing (models/position.go:97-102)
    spotActive, _ := e.db.GetActiveSpotPositions()    // spot non-closed: pending/active/exiting only (models/spot_position.go:74-77)
    // union symbols; count slot usage
}
```

**Policy summary.**
- `MaxPositions` is the combined ceiling (perp + spot).
- `SpotFuturesMaxPositions` further caps spot-only.
- One symbol may be open in **at most one strategy** at a time.
- `SlotCost = 1` for every candidate.

**Grouping rule in selector:**
```go
func groupChoicesBySymbol(
    perp []perpDispatchRequest,
    spot []*models.SpotEntryPlan,
) map[string][]searchChoice {
    // One GroupKey per symbol, candidates from either strategy.
    // Symbols already occupied (in unifiedOccupancy.ActiveSymbols) get zero candidates.
}
```

**Must NOT change.**
- `internal/engine/engine.go:2103` perp dup guard — safety net.
- `internal/spotengine/execution.go:143` spot dup guard — safety net.
- `internal/engine/engine.go:2521` `MergeExistingDuplicates` — unrelated persistence concern.

---

## 10. Config, API, and Dashboard Wiring

**Config (additions only):**
- `allocation.enable_unified_entry_selection` (bool, default `false`) — new; JSON key `enable_unified_entry_selection`; env var `ENABLE_UNIFIED_ENTRY_SELECTION`.

**Config (reuse, no change):**
- `risk.enable_capital_allocator`, `allocation.enable_unified_capital` (existing; sizing, not selection)
- `fund.max_positions`, `spot_futures.max_positions`
- `spot_futures.auto_enabled` (becomes "standalone-mode only" when `unifiedOwnerReady()` is true)
- `spot_futures.capital_separate_usdt`, `spot_futures.capital_unified_usdt`, `spot_futures.leverage`
- `discovery.min_hold_time_hours`
- `risk.max_perp_perp_pct`, `risk.max_spot_futures_pct`, `risk.max_per_exchange_pct`, `risk.reservation_ttl_sec`

**Live config wiring touchpoints to update (`internal/config/config.go`):**
- `Config` struct: add `EnableUnifiedEntrySelection bool` field
- `jsonAllocation` struct (`config.go:281-288`): add `EnableUnifiedEntrySelection *bool` matching JSON key
- `applyJSON` allocation block (`config.go:1234-1252`): apply the new field if non-nil
- Defaults (`config.go:567+`): set default `false`
- Save path (`config.go:1544-1551`): persist new field
- Env override (`config.go:1838-1846`): read `ENABLE_UNIFIED_ENTRY_SELECTION`

**Live API wiring touchpoints to update (`internal/api/handlers.go`):**
- `configAllocationResponse` struct (`handlers.go:307-314`): add `EnableUnifiedEntrySelection bool` JSON field
- `allocationUpdate` struct (`handlers.go:632-639`): add `EnableUnifiedEntrySelection *bool` for POST
- `buildConfigResponse` (`handlers.go:751-758` + `:1461-1487`): emit field
- POST apply path (`handlers.go:751-758`): copy POST value into `Config`
- Flat Redis hash `fields[...]` map (`handlers.go:1637-1643`): persist `enable_unified_entry_selection` key

**Live dashboard wiring touchpoints to update (`web/src/pages/Config.tsx`):**
- New toggle under allocator section (alongside existing `enable_unified_capital` at `Config.tsx:1633-1647`):
  - i18n key: `cfg.alloc.enableUnifiedEntrySelection` (label)
  - i18n key: `cfg.alloc.enableUnifiedEntrySelectionDesc` (helper text)
- Standalone spot auto-entry toggle (`Config.tsx:1209-1215`): disable + tooltip when `unifiedOwnerReady()` would return true (flag on AND allocator enabled — dashboard can read both flags from `/api/config`)
  - i18n key: `cfg.sf.autoEnabledUnifiedOwner` (tooltip text "subsumed by unified entry selection")

**Live i18n wiring touchpoints (BOTH locale files MUST stay in sync):**
- `web/src/i18n/en.ts`:
  - Add `'cfg.alloc.enableUnifiedEntrySelection': 'Unified entry selection (beta)'`
  - Add `'cfg.alloc.enableUnifiedEntrySelectionDesc': 'Single B&B picks best subset across perp + spot at EntryScan'`
  - Add `'cfg.sf.autoEnabledUnifiedOwner': 'Subsumed by unified entry selection'`
  - Mirror at `:434-435, :489-490` (existing allocation/spot-futures sections)
- `web/src/i18n/zh-TW.ts`:
  - Add Traditional Chinese translations for the same 3 keys
  - Mirror at `:431-432, :485-486`

---

## 11. Test Plan

**Unit (new files).**

`internal/engine/selection_core_test.go`:
- `TestSolveGroupedSearch_PicksBestSubsetUnderSlotCap`
- `TestSolveGroupedSearch_OneChoicePerGroup` (mutual exclusion)
- `TestSolveGroupedSearch_EvaluateFeasibilityRejection`
- `TestSolveGroupedSearch_TimeoutFallsBackToGreedy`

`internal/engine/unified_entry_test.go`:
- `TestUnifiedEntry_UserFiveCandidateExample` — reproduces `{spot, perp, perp, perp, spot}` → `{#1 spot, #2 perp}` selection
- `TestUnifiedEntry_DuplicateSymbolDedupAcrossStrategies`
- `TestUnifiedEntry_RespectsMaxPositionsCombinedCap`
- `TestUnifiedEntry_RespectsSpotOnlyCapIndependently`
- `TestUnifiedEntry_DispatchHonorsPreheldReservation` (no double-reserve)
- `TestUnifiedEntry_ReleasesPreheldOnDispatchError`
- `TestUnifiedEntry_SymbolAlreadyActiveIsExcluded`
- `TestUnifiedEntry_LoadUnifiedOccupancy_BlocksAllNonClosedPositions`
- `TestUnifiedEntry_SkipsStandaloneSpotAutoEntryWhenUnifiedEnabled`
- `TestUnifiedEntry_RequiresSpotEntryExecutorBeforeDispatch` (nil-guard or startup-order regression)
- `TestConsumeOverridesAndEnrichOpps_NonEmptyOppsAppliesTier2Patch` — non-empty opps + non-empty overrides → tier-2 patch applied (engine.go:1036-1128 logic preserved)
- `TestConsumeOverridesAndEnrichOpps_EmptyOppsCallsRescanSymbols` — empty opps + non-empty overrides → `discovery.RescanSymbols()` called with correct SymbolPair list (v0.32.8 engine.go:1272-1299 preserved)
- `TestConsumeOverridesAndEnrichOpps_EmptyOverridesReturnsOppsUnchanged` — overrides cleared each invocation; subsequent call with same overrides is no-op
- `TestConsumeOverridesAndEnrichOpps_AcquiresAllocOverrideMuOnceAndClears` — concurrent callers see consume-once semantics
- `TestUnifiedEntry_LegacyPathRunsWhenFlagOnButAllocatorDisabled` — readiness gate must fall through to `executeArbitrage`, not suppress legacy
- `TestAttemptAutoEntries_LegacyPathRunsWhenFlagOnButAllocatorDisabled` — symmetric on spot side

`internal/engine/unified_value_test.go`:
- `TestScoreSpotEntry_DirA_DeductsBorrowAPRAndFees`
- `TestScoreSpotEntry_DirB_NoBorrowCost`
- `TestScoreSpotEntry_UsesMinHoldTimeNot8h`
- `TestScoreSpotEntry_APRDecimalNotPercent`

`internal/risk/batch_reservation_test.go`:
- `TestReserveBatch_AtomicAllOrNothing`
- `TestReserveBatch_RespectsPerStrategyCap`
- `TestReserveBatch_RespectsPerExchangeCap`
- `TestReleaseBatch_KeepsCommittedReservations`
- `TestReleaseBatch_ReleasesOnlyUncommitted`

`internal/config/config_test.go` (extend existing):
- `TestConfig_RoundTripUnifiedEntrySelectionFlag` — JSON load/save preserves `enable_unified_entry_selection`
- `TestConfig_EnvOverrideUnifiedEntrySelection` — `ENABLE_UNIFIED_ENTRY_SELECTION=true` overrides JSON

`internal/api/config_handlers_test.go` (extend existing; matches repo pattern `TestHandleConfig_*RoundTrip`):
- `TestHandleConfig_UnifiedEntrySelectionRoundTrip`
- `TestHandleConfig_UnifiedEntrySelectionPostRoundTrip`

`internal/spotengine/selected_entry_test.go`:
- `TestListEntryCandidates_FiltersFilterStatusAndStale`
- `TestBuildEntryPlan_DirA_BorrowCheckAgainstBaseUnits`
- `TestBuildEntryPlan_DirA_CapsToMaxBorrowableAndRoundsToFuturesStep` (replaces old "FailsBorrowCheckWhenSizeExceeds")
- `TestBuildEntryPlan_DirA_RejectsWhenRoundedSizeBelowMin`
- `TestBuildEntryPlan_DirA_RejectsWhenNotionalBelowBudgetFloor` (rule is `max(budget*0.10, 5.0)` — NOT venue min)
- `TestBuildEntryPlan_DirB_NoBorrowNeeded`
- `TestBuildEntryPlan_BinanceBitget_SetInternalTransferFlag` (replaces old "SeparateAccount_SetsInternalTransferFlag")
- `TestBuildEntryPlan_GateioUnified_NoInternalTransfer`
- `TestBuildEntryPlan_BybitUTA_NoInternalTransfer`
- `TestBuildEntryPlan_OKXUnified_NoInternalTransfer`
- `TestBuildEntryPlan_BingXFilteredOut`
- `TestBuildEntryPlan_UsesPlannedNotionalForScoreAndReserve`
- `TestOpenSelectedEntry_NoLatestOppsLookup`
- `TestOpenSelectedEntry_UsesPreheldReservationNoReserveCall`

**Regression (shared-core extraction).**

`internal/engine/allocator_parity_test.go`:
- `TestRunPoolAllocator_AfterRefactorSameOutputAsBaseline` — frozen fixture, byte-identical choices.
- `TestGreedyAllocatorSeed_AfterRefactorSameOutput`.

**Integration (live VPS, post-deploy, `unifiedOwnerReady()==true` — both flag and capital allocator on).**
1. `/api/config` shows `enable_unified_entry_selection: true`.
2. Log shows `unified entry: picked N=<n> (perp=<p> spot=<s>) value=<USDT>`.
3. At least one cycle with both perp + spot candidates → both types in picked subset.
4. Symbol duplicate across strategies blocked — confirm via log `unified entry: skip <symbol> already active across strategies`.
5. Flag flip to false → perp runs `executeArbitrage`, spot runs `attemptAutoEntries` as before (regression).
6. ReserveBatch atomicity: force one cap breach scenario, confirm zero partial reservations persisted.

**Acceptance.**
- [ ] `go build` / `go vet` / `go test ./...` pass
- [ ] Parity tests pass (shared-core extraction zero behavior drift)
- [ ] User's 5-candidate example reproduces `{#1, #2}` selection
- [ ] Live dry-run: `unifiedOwnerReady()==true` for one cycle produces expected log signature; NO perp/spot positions duplicated
- [ ] Flag=false: legacy paths unchanged (baseline)
- [ ] CHANGELOG updated; VERSION → 0.33.0 (minor)
- [ ] Codex post-impl review PASS
- [ ] i5 post-impl review PASS (on branch)

---

## 12. Rollout and Rollback

**Rollout.**
1. Ship with `enable_unified_entry_selection=false` in production `config.json`.
2. Verify legacy paths unchanged (regression gate).
3. Enable in staging / dashboard toggle, observe 3 full EntryScan cycles (at `cfg.EntryScanMinute`).
4. Enable in production for 1 cycle, confirm expected behavior.
5. Leave on; monitor for 24h.

**Operational verification log line** (emit at start of `runUnifiedEntrySelection`, AFTER the hard gate passes):
```
[engine] unified entry: tick at minute=<actual_minute> (cfg.EntryScanMinute=<configured>) unified_entry=on capital_allocator=on
```
Operators verify the active path AND the capital-allocator dependency without assuming the default schedule.

**Rollback.**
- Primary: flip `enable_unified_entry_selection=false` via dashboard. Takes effect next EntryScan tick (at `cfg.EntryScanMinute`).
- Secondary: `git revert <commit>` — no schema/Redis migration; all new code is behind the flag.

**Downgrade path if batch reservation has a bug.**
- Disable flag → falls back to legacy per-candidate reserve. Both paths coexist, maintained for the rollout period only. Post-validation (2 weeks), remove legacy dual-path code in a follow-up cleanup PR.

---

## 13. Files Changed Summary

**New.**
- `internal/engine/selection_core.go`
- `internal/engine/unified_entry.go`
- `internal/engine/unified_state.go`
- `internal/engine/unified_value.go`
- `internal/engine/spot_entry_iface.go`
- `internal/models/spot_entry.go`
- `internal/spotengine/selected_entry.go`
- `internal/risk/batch_reservation.go`
- 7 test files above

**Modified.**
- `internal/engine/allocator.go` — refactor `solveAllocator` + `greedyAllocatorSeed` to call `selection_core.go`; rebalance behavior preserved
- `internal/engine/engine.go` — add `spotEntry` field, `SetSpotEntryExecutor`, `unifiedOwnerReady()` helper, EntryScan branch using `unifiedOwnerReady()`, refactor tier-2 override consume+patch+rescan into single `consumeOverridesAndEnrichOpps()` helper (shared by legacy and unified paths; rename existing `applyAllocatorOverrides` to pure `applyAllocatorOverridesWithState(opps, overrides)` taking overrides as parameter); reuse freshness window constant
- `internal/engine/capital.go` — `commitExistingReservation` helper
- `internal/spotengine/capital.go` — `capOverride` param, `commitSpotFromPreheld`
- `internal/spotengine/autoentry.go` — skip when `unifiedOwnerReady()` is true (readiness-gated, not flag-only)
- `internal/spotengine/engine.go` — add symmetric `SpotEngine.unifiedOwnerReady()` helper
- `internal/spotengine/engine.go` — no structural changes required beyond reusing `capitalForExchange`
- `internal/config/config.go` — new `EnableUnifiedEntrySelection bool` + JSON key
- `internal/api/handlers.go` — surface new flag in `/api/config`
- `web/src/pages/Config.tsx` — toggle UI; grey-out standalone auto-entry when `unifiedOwnerReady()` would be true (dashboard computes from `allocation.enable_unified_entry_selection` + `risk.enable_capital_allocator`)
- `cmd/main.go` — call `SetSpotEntryExecutor(spotEng)`
- `VERSION` → `0.33.0`
- `CHANGELOG.md`

**Not changed.**
- `internal/risk/allocator.go` existing single-reservation APIs
- `internal/engine/allocator.go` `runPoolAllocator` / `dryRunTransferPlan` / `executeRebalanceFundingPlan`
- `internal/engine/engine.go:632` rebalance override path
- `internal/spotengine/execution.go` `ManualOpen` / transfer logic / borrow poll loop
- `internal/engine/engine.go:2103`, `internal/spotengine/execution.go:143`, `internal/engine/engine.go:2521` safety-net guards

---
