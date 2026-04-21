# Cross-Engine Interference Audit (v0.32.13)

**Task:** 260415-34e
**Scope:** All exchange-level queries that may see positions/balances owned by the *other* engine (perp-perp ↔ spot-futures), across `internal/engine/`, `internal/spotengine/`, `internal/risk/`, and shared WS callback paths.
**Method:** READ-ONLY static analysis of every call site listed in the plan inventory (20 `getExchangePositionSize`, 5+ `GetAllPositions`, ~15 `GetFuturesBalance`, `ownOrders`, `slIndex`, `GetPosition`, WS fill callbacks).
**Already-fixed (excluded from findings count):**
- `consolidate.go:287-297` — size-mismatch detection uses `sfSizeOffset` (FIXED v0.32.13)
- `exit.go:2786` — rotation post-close verification with step-size pre-check + re-open on abort (FIXED v0.32.13)

---

## Summary Table

| # | Severity | Location | Symptom | Fix class |
|---|----------|----------|---------|-----------|
| 1 | HIGH | `engine.go:1577` (SL Method 2 verify) | False SL trigger → emergency close of healthy PP position when SF closes its futures leg on same exchange+symbol+side | Subtract `sfSizeOffset` or gate on SF-owned orderIDs |
| 2 | HIGH | `engine.go:1822` (orphan-cleanup verify) | Orphan close repeatedly "not flat" because SF leg is visible → position reverted to Active forever; or premature close of SF's futures leg if sizes happen to overlap | Subtract `sfSizeOffset` |
| 3 | HIGH | `consolidate.go:480` / `:500` (markPositionClosed close-leg) | Consolidator issues `closeFullyWithRetry` against SIZE THAT INCLUDES SF'S FUTURES LEG → closes SF's hedge, strips SF position of its futures side, causing live spot-margin-only exposure | Subtract `sfSizeOffset` before using `actualSize` for close |
| 4 | HIGH | `consolidate.go:524` / `:532` (markPositionClosed verify) | Post-close verification sees residual SF leg → keeps PP position Active with ghost `LongSize/ShortSize` forever | Subtract `sfSizeOffset` before dust check |
| 5 | HIGH | `consolidate.go:842-843` (reconcilePartialPosition) | Partial-entry reconciliation sees SF leg → mis-promotes to Active / trims SF leg / sets wrong `LongSize`/`ShortSize` | Subtract `sfSizeOffset` |
| 6 | HIGH | `exit.go:1893-1894` (closePositionWithMode verify) | Post-close verify sees SF leg → position reverted to Active with `LongSize = SF's size` and stale SLs reattached against a phantom position | Subtract `sfSizeOffset` |
| 7 | HIGH | `engine.go:3614` / `:3674` / `:3841` / `:3898` (entry confirmFill fallback) | On `confirmFillSafe` failure during entry, re-reads exchange size and stores it as PP's `LongSize`/`ShortSize` — silently absorbs SF's futures leg into the PP record | Subtract `sfSizeOffset`, or prefer delta-from-pre-entry snapshot |
| 8 | MEDIUM | `engine.go:1927,1946` (updateFundingCollected) | Hourly funding tracker sums `FundingFee` / `UnrealizedPL` from ALL positions on symbol → attributes SF's funding+UPL to PP position | Filter by side only after subtracting SF total; or use per-orderID funding history |
| 9 | MEDIUM | `risk/monitor.go:130` (checkAll prefetch → checkPosition) | `checkPositionBalance` / `checkUnrealizedPnL` / `checkLiquidationDistance` consume ALL positions on exchange, not just PP's → false "balance drift" / wrong UPL / wrong liq distance when SF has a leg | Filter prefetched array by side *and* subtract SF offset before comparison |
| 10 | MEDIUM | `risk/health.go:552,563` (orphan-leg detection) | Uses `GetAllPositions` to detect orphan PP legs — sees SF legs as PP legs when sides match, counter increments, eventually triggers orphan close of an SF futures leg | Subtract `sfSizeOffset` on `findLegSize` |
| 11 | MEDIUM | `risk/health.go:179,308` (L-tier calculation + `getLegPnL`) | Per-exchange L0–L5 tier uses `GetFuturesBalance.MarginRatio` (includes SF margin) but `totalPnL` loop iterates only PP positions → tier triggered by SF exposure but reduction actions only target PP positions (L4 reduces PP to save margin SF is consuming) | Either include SF positions in `totalPnL` (already done for count), or track SF margin separately |
| 12 | MEDIUM | `risk/manager.go:131,252,259,283,290` (approve / approveRotation) | Pre-trade capital check uses `bal.Available` — SF's open borrows/hedge already consumed part of this. PP may over-commit when SF has a large hedge open; or under-commit (benign). Conversely, `ensureFuturesBalance` sweeps spot → futures, which can *starve* SF's planned spot-margin entries that expect quote currency in spot wallet | Reserve an SF-reserved portion of `bal.Available` before use |
| 13 | MEDIUM | `engine.go:609,1700,1743,2148,3534,3540` (rebalance / L4 re-check / L5 transfer / prefetch / depth-tick cap) | `GetFuturesBalance.Available` is unified pool (Bybit UTA, OKX, Gate.io) — reflects combined PP + SF usage. Rebalance may move funds *away* from an exchange where SF needs them; depth-tick margin cap clamps PP size using an Available that SF has also earmarked | Introduce an "engine-available" computation: `bal.Available - sfReservedMargin(exchName)` |
| 14 | MEDIUM | `allocator.go:1566,1593,1766,1832,1860` (rebalanceFunds donor/recipient flow) | Rebalance decides donors/surplus using unified balances. On unified exchanges (Bybit UTA, OKX, Gate.io) it can transfer funds *out* of SF's working capital pool. `MaxTransferOut` is exchange-authoritative so limits damage, but in-memory surplus math doesn't know SF is reserving funds | Subtract SF reservation from donor `futures.Available` / `futuresTotal` *before* surplus computation |
| 15 | MEDIUM | `spotengine/exit_manager.go:160,185` (margin_health_exit) | SF's exit trigger uses `GetFuturesBalance().MarginRatio` (unified pool). When PP has 3 large positions on same exchange, PP's margin stress pushes ratio >95% and triggers **SF emergency exit** — even though SF's own hedge is fine | Subtract PP-attributable margin before comparison; or check per-position `Position.MarginRatio` instead |
| 16 | MEDIUM | `spotengine/execution.go:1106` (pendingEntryFuturesPosition) | Pending-entry reconciliation reads ALL positions on symbol+side → may absorb a PP leg as "SF hedge that was opened". If PP has an active long on the same symbol+exchange, SF promotes to Active with PP's size/entry as its `FuturesSize`/`FuturesEntry` | Match against the pre-entry snapshot delta, or filter via the just-placed `orderID` only (query by order, not by position) |
| 17 | LOW | `consolidate.go:173` (orphan scan) | `GetAllPositions` iterates all exchange positions; an SF futures leg is excluded via `spotFuturesKeys[key]` filter (correct today), but only if Dir A/Dir B→side mapping is right. Symbol/side mismatch (e.g. future rename) would make SF legs appear as orphan PP positions and get market-closed. Defence-in-depth concern | Add sanity check: if key matches an SF position *by symbol alone*, log & skip rather than close |
| 18 | LOW | `engine.go:1647-1650` (WS `SetOrderCallback`) | Single private-fills WS callback per exchange, installed by PP engine. SF futures fills flow through this callback; SF doesn't register its own. SF fills reach `handleSLFill` and are filtered out by method 1 (`slIndex` miss) + method 2 `remaining > 0` check. Works today but relies on the method-2 verify subtracting SF offset (finding 1) | Once finding 1 is fixed, this is safe. Document the dependency |
| 19 | LOW | `engine.go:1532` (`ownOrders` map) | `ownOrders` is PP-only. SF orders never go in, so when SF places a reduce-only futures close, `handleSLFill` won't short-circuit via `ownOrders`. It proceeds to method-2 verify. Same dependency as finding 18 | Share an `ownOrders` map between engines; or have SF register its own callback that drains SF fills before PP sees them |
| 20 | LOW | `engine.go:4643,4653` (getUnrealizedPnL) | Same pattern as finding 8 but for a helper used only in log lines — cosmetic only | Subtract SF offset when logging |
| 21 | LOW | `api/spot_handlers.go:628,649` (dashboard handler) | `GetAllPositions` for UI display — reads both engines' positions. Cosmetic: SF panel may show a PP position. Verify filtering | Filter by spot-futures key like consolidator does |

**Findings count:** 21 new (FIXED sites excluded). HIGH=7, MEDIUM=9 (treating rebalance/allocator as one class of 2, manager margin as 1), LOW=5.

---

## Detailed Findings

### Finding 1: SL Method-2 verify can false-trigger on SF futures-leg closes

- **Location:** `internal/engine/engine.go:1577` (inside `handleSLFill`, method 2)
- **Function:** `handleSLFill` → `getExchangePositionSize(exch, pos.Symbol, leg)`
- **What it reads:** Total exchange position on `(exchName, symbol, leg-side)` after a reduce-only fill arrives via WS.
- **Why it's wrong:** When SF closes its futures leg on the same exchange and same side as a PP position's leg, the fill is ReduceOnly, is NOT in `ownOrders` (PP-only map), and the SL index doesn't have its orderID. The code falls into method-2: it looks up PP's active position on that exchange+symbol, then calls `getExchangePositionSize`. If the PP leg is still fully open, `remaining > 0` and the code correctly skips — BUT if PP's leg is SMALLER than SF's leg and the exchange reports totals by side, `remaining = PP + SF_residual > 0` still correctly skips. The danger path is when SF closes **both** legs simultaneously on unified-sided exchanges where PP also holds the same side: PP's leg is fully intact but the WS fill looks like a reduce of the combined total. Code calls `remaining = combinedSize`; with SF partially done, `remaining > 0` skips. With SF fully done, `remaining = PP.size > 0` skips. The problematic case is when PP has already quietly exited (e.g. consolidator path just completed) and SF fires a reduce-only fill before `unregisterSLOrders` runs — then `remaining = 0` and emergency-close fires for a *phantom* PP position that is already closed, causing duplicate bookkeeping / double PnL reconcile attempts.
- **Severity:** HIGH (edge case but emits `closePositionEmergency` on a closed position; concurrent with SF exit it can also race into closing SF's other side)
- **Impact:** Potential duplicate close paths; Telegram SL alerts fired for non-SL events; possible cancellation of SF's own orphan TP/SL orders by downstream `CancelAllOrders`
- **Suggested fix:** Before method-2 verify, subtract `sfSizeOffset` (same pattern as `consolidate.go:296`). Or, better, build a shared `ownOrders` map that BOTH engines populate so SF reduce-only fills are filtered before method-2 even runs.
- **Already fixed in v0.32.13?** No

### Finding 2: Orphan-cleanup post-close verify sees SF leg

- **Location:** `internal/engine/engine.go:1822`
- **Function:** `handleOrphanClose` → `getExchangePositionSize(exch, pos.Symbol, action.OrphanSide)`
- **What it reads:** Remaining size on the orphan side after PP market-closed what it thought was the orphan leg.
- **Why it's wrong:** If the "orphan" side coincides with an SF futures leg on the same exchange+side, after PP closes its portion, `remaining = SF.size > 0`. The code logs "ORPHAN EXPOSURE" and reverts the position to `StatusActive`, assigning `LongSize/ShortSize = remaining` — i.e. it imports SF's size into the PP record. On the next consolidator cycle, `siblingTotals` mismatch will try to close "the remainder", hitting SF's leg with `closeFullyWithRetry`.
- **Severity:** HIGH
- **Impact:** Live SF hedge leg can be market-closed, leaving SF's spot margin position unhedged until SF's monitor detects the problem (~60s worst case, depending on config). Delta-neutral invariant violated for up to a full scan interval.
- **Suggested fix:** Load `sfSizeOffset` here (reuse the consolidator's helper) and subtract before dust check.
- **Already fixed in v0.32.13?** No

### Finding 3: markPositionClosed closes SF's futures leg

- **Location:** `internal/engine/consolidate.go:480` (long path), `:500` (short path)
- **Function:** `markPositionClosed` → `getExchangePositionSize(longExch, pos.Symbol, "long")` then `closeFullyWithRetry(... , actualSize)`
- **What it reads:** Exchange-total size on the leg that's about to be closed, using the returned size as the close quantity.
- **Why it's wrong:** `actualSize` includes SF's futures leg. The code then calls `closeFullyWithRetry(exch, symbol, side, actualSize)` — which issues market reduceOnly orders for `actualSize`. Exchange-side reduceOnly will cap at total exposure, so at worst PP closes both its own remainder AND SF's hedge.
- **Severity:** HIGH
- **Impact:** SF's futures hedge unexpectedly flattens. SF monitor will detect the imbalance and emergency-exit the spot leg, but the unhedged window is non-zero and may lock in borrow costs without the funding offset.
- **Suggested fix:** Same as finding 1 — subtract `sfSizeOffset` before using `actualSize`. The `markPositionClosed` function doesn't currently receive the offset map; either plumb it through as a new parameter, or rebuild it locally from `GetActiveSpotPositions`.
- **Already fixed in v0.32.13?** No

### Finding 4: markPositionClosed verify keeps PP stuck Active

- **Location:** `internal/engine/consolidate.go:524` (long), `:532` (short)
- **Function:** `markPositionClosed` — post-close verification
- **What it reads:** Exchange size after close attempt; dust-check to decide "confirmed flat".
- **Why it's wrong:** If SF still has its hedge open on this symbol+side, `rem > minSize` → `longDust=false` → CRITICAL log + `keeping active`. PP position is now a ghost Active record with its actual legs closed but local `LongSize`/`ShortSize` still > 0.
- **Severity:** HIGH
- **Impact:** Ghost position leaks allocator capacity forever; dashboard shows wrong data; funding/PnL tracking continues on a closed position.
- **Suggested fix:** Same `sfSizeOffset` subtraction before `isDust` check.
- **Already fixed in v0.32.13?** No

### Finding 5: reconcilePartialPosition mis-promotes using SF size

- **Location:** `internal/engine/consolidate.go:842-843`
- **Function:** `reconcilePartialPosition`
- **What it reads:** Exchange totals per leg to decide: promote to Active, close one-sided, or mark failed.
- **Why it's wrong:** If PP has a `StatusPartial` position on `BTCUSDT` (short leg filled, long failed) and SF holds a `BTCUSDT` long hedge on the same exchange, `longActual = SF.longSize > 0` and `shortActual = PP.shortSize > 0`. Code takes the "both legs present" branch, trims excess (cuts into SF!), and promotes PP to Active with `LongSize = minFill = min(SF.size, PP.size)`. PP now claims SF's long leg.
- **Severity:** HIGH
- **Impact:** SF's hedge partially or fully closed; PP position claims phantom long leg with wrong `LongEntry` (BBO at promote time, not SF's actual entry). On PP exit, both legs' reduceOnly closes will over-close.
- **Suggested fix:** `sfSizeOffset` subtraction before the promote/trim logic.
- **Already fixed in v0.32.13?** No

### Finding 6: closePositionWithMode post-close verify → phantom revert

- **Location:** `internal/engine/exit.go:1893-1894`
- **Function:** `closePositionWithMode` verification
- **What it reads:** Per-leg exchange size after PP exit to confirm flat.
- **Why it's wrong:** Exact same pattern as the rotation bug fixed in v0.32.13 at `exit.go:2786`. If SF leg is on same exchange+side, `actualLong/actualShort > 0`, `notFlat = true`, and code reverts PP to Active with `LongSize = SF.size`. Worse, it reattaches SL orders for this phantom size.
- **Severity:** HIGH
- **Impact:** PP position never actually closes from local perspective; SL orders placed for an amount PP doesn't own — next reduceOnly SL trigger will close SF's leg.
- **Suggested fix:** The twin of the v0.32.13 rotation fix. Subtract `sfSizeOffset`; if combined remainder equals the SF-known offset, treat as flat.
- **Already fixed in v0.32.13?** No (this is the direct twin of the FIXED rotation bug)

### Finding 7: Entry fallback absorbs SF size into PP record

- **Location:** `internal/engine/engine.go:3614`, `:3674`, `:3841`, `:3898` (all in `executeArbitrage` depth-fill paths)
- **Function:** depth-fill first-leg and top-up `confirmFillSafe` failure recovery
- **What it reads:** Exchange size on `(shortExch, Symbol, "short")` or `(longExch, Symbol, "long")` used as `pos.ShortSize` / `pos.LongSize` / as `priorTotal` reference for top-up delta.
- **Why it's wrong:** Two sub-cases:
  1. First-leg `confirmFill` failed: code queries `getExchangePositionSize`, and if `exchSize > 0` writes it directly as the PP position size (`f.ShortSize = exchSize` at line 3621; `f.LongSize = exchSize` at 3848). An SF hedge pre-existing on the same side+exchange contributes to `exchSize`, so PP records SF's size + PP's fill.
  2. Top-up branch uses `priorTotal := confirmedShort + shortFilled` and then `actualTopUp := exchSize - priorTotal`. If SF opened between snapshots, `exchSize` jumps and `actualTopUp` is falsely inflated.
- **Severity:** HIGH (happens at entry time; PP writes an incorrect size to Redis immediately)
- **Impact:** PP owns phantom size from day one. Depth/exit/SL all operate on wrong size. On exit, over-closes SF's hedge.
- **Suggested fix:** Snapshot the exchange size *before* placing the IOC order, then compute `delta = post - pre` rather than using `post` directly. Also subtract `sfSizeOffset` as defence-in-depth.
- **Already fixed in v0.32.13?** No

### Finding 8: updateFundingCollected attributes SF funding/UPL to PP

- **Location:** `internal/engine/engine.go:1927` (long), `:1946` (short)
- **Function:** `updateFundingCollected` — hourly funding tracker
- **What it reads:** All positions on symbol, filters by `HoldSide == "long"|"short"`, sums `FundingFee` and `UnrealizedPL` fields from each `Position` row.
- **Why it's wrong:** Each exchange may return multiple rows for the same symbol+side (hedge mode) or one combined row (one-way mode). Even in one-way mode, when SF and PP share a side, the single row reports combined totals. PP absorbs SF's funding fees into `pos.FundingCollected` and SF's unrealized PnL into `pos.LongUnrealizedPnL`/`ShortUnrealizedPnL`.
- **Severity:** MEDIUM (cosmetic in hedge mode where rows are distinct; real P&L misstatement in one-way mode)
- **Impact:** PP's analytics PnL and funding counts are inflated/deflated by SF activity. Analytics PnL attribution (Phase 4) is wrong. Dashboard shows misleading funding collected.
- **Suggested fix:** Prefer `GetFundingFees(symbol, since)` history API (already the fallback) which is scoped to the account's funding events by time — still not engine-filtered, but combined with `pos.CreatedAt` it at least doesn't double-count across engines if they entered at different times. Best fix: apportion by size ratio using `sfSizeOffset`.
- **Already fixed in v0.32.13?** No

### Finding 9: risk/monitor sees cross-engine positions

- **Location:** `internal/risk/monitor.go:130` → consumed at `:175` fallback and at `checkPosition` (`checkPositionBalance`, `checkUnrealizedPnL`, `checkLiquidationDistance`).
- **Function:** `Monitor.checkAll` prefetches `GetAllPositions()` per exchange and passes through `getCachedPositions`.
- **What it reads:** Every position on every exchange with an active PP position.
- **Why it's wrong:** The per-position helper filters by `p.Symbol == symbol` only — NOT by engine ownership. Downstream `checkPositionBalance` compares `sum(p.Total)` against `pos.LongSize + pos.ShortSize`; when SF has a leg on the same symbol, the sum is inflated → false "balance drift" alert. `checkUnrealizedPnL` aggregates `UnrealizedPL` on that symbol → includes SF's UPL → false PnL alerts. `checkLiquidationDistance` reads `LiquidationPrice` from possibly the SF leg's row.
- **Severity:** MEDIUM
- **Impact:** Noisy false alerts; may also emit HealthAction events that push risk engine into incorrect tier decisions.
- **Suggested fix:** After filtering by symbol, also subtract `sfSizeOffset[exch:symbol:side]` before comparison in each check.
- **Already fixed in v0.32.13?** No

### Finding 10: risk/health orphan detection sees SF legs

- **Location:** `internal/risk/health.go:552,563` (`detectOrphanLegs`)
- **Function:** orphan-leg scanner; uses `findLegSize` to get per-side size from `GetAllPositions`
- **What it reads:** Exchange sizes per (symbol, side) for every active PP position.
- **Why it's wrong:** `findLegSize` sums across all rows with matching side. If the PP long leg has been partially closed but SF has a long leg open, `longSize = SF.size > 0` → not orphan. Conversely, if PP long is open but SF has a leg on PP's short side, `shortSize = SF.size > 0` → not orphan. Both false-negatives (orphan not detected) AND false-positives (when PP both legs truly open but SF *absent* and sizes don't match expected; trend monitor could get confused).
- **Severity:** MEDIUM
- **Impact:** Orphan detection delayed or missed; the subsequent `HealthAction{Type: "close_orphan"}` would (once dispatched) hit finding 2.
- **Suggested fix:** `findLegSize` should consume the SF offsets map. Plumb it from `HealthMonitor.checkAll` similarly to consolidator.
- **Already fixed in v0.32.13?** No

### Finding 11: Health tier uses unified margin, reduces only PP

- **Location:** `internal/risk/health.go:179` (balance), `:308` (`getLegPnL`)
- **Function:** `checkExchangeHealth`, `handleL3/L4/L5`
- **What it reads:** `GetFuturesBalance().MarginRatio` is the exchange-wide margin ratio. `getLegPnL` reads `GetPosition(symbol)` per PP position.
- **Why it's wrong:** The margin ratio in `bal` reflects *both engines'* consumption (unified pool on Bybit UTA / OKX / Gate.io). The `totalPnL` loop sums only PP positions' leg UPL. `computeLevel` decides tier from `marginRatio` (combined) + `totalPnL` (PP-only). When SF has a big losing hedge, marginRatio rises → L4/L5 triggered → `handleL4` passes `exchPositions` (PP-only) + `exchSpotPositions` to `h.handleL4`. Good news: L4/L5 handlers DO get `exchSpotPositions` (lines 250-252) — but the L3 handler at line 248 only gets PP positions (`exchPositions`), so L3 transfer logic ignores SF exposure.
- **Severity:** MEDIUM
- **Impact:** L3 transfers funds into an exchange thinking margin is dedicated to PP, when actually SF is consuming it. Transferred funds get absorbed into SF's margin requirement, PP's margin ratio doesn't improve, loop continues. Also `getLegPnL` at `:308` sums `UnrealizedPL` for PP's `side` but across all rows with that side — includes SF UPL (same class as finding 9).
- **Suggested fix:** Pass SF positions to `handleL3` too, or compute an "SF-adjusted margin ratio" separate from `bal.MarginRatio` for tier decisions that only drive PP reductions.
- **Already fixed in v0.32.13?** No

### Finding 12: risk/manager approval uses unified Available

- **Location:** `internal/risk/manager.go:131` (ApproveRotation), `:252/:259` (approveInternal balance fetch), `:283/:290` (post-ensure re-fetch)
- **Function:** Pre-trade approval — capital check uses `bal.Available`.
- **What it reads:** `GetFuturesBalance` on both legs.
- **Why it's wrong:** On unified-account exchanges (Bybit UTA, OKX, Gate.io), `bal.Available` is shared across PP + SF. PP may approve an entry that consumes capacity SF has already earmarked (SF reserves via the `CapitalAllocator`, not via exchange-side locks, so SF's reservation is invisible to `GetFuturesBalance`).
- **Severity:** MEDIUM (usually caught downstream by margin-cap check in depth-tick, but can still cause failed orders and retries)
- **Impact:** PP entries fail at exchange with INSUFFICIENT errors when SF is holding most of the unified pool; wasted API calls; noisy logs.
- **Suggested fix:** Subtract `allocator.SFReservedMargin(exchName)` from `bal.Available` before the approval check. Allocator already tracks this.
- **Already fixed in v0.32.13?** No

### Finding 13: Engine direct balance reads are cross-contaminated

- **Location:**
  - `engine.go:609` (rebalance snapshot)
  - `engine.go:1700` (L4 post-reduce re-check)
  - `engine.go:1743` (L5 safety transfer remainder)
  - `engine.go:2148` (entry-scan prefetch)
  - `engine.go:3534` / `:3540` (depth-tick margin cap per PP entry)
- **Function:** Various PP engine flows using raw `Available`/`Total`/`MarginRatio` from `GetFuturesBalance`.
- **What it reads:** Exchange's unified futures balance (on unified accounts, this is the whole pool).
- **Why it's wrong:** Same class as finding 12: "available" overstates what PP can safely use when SF has reservations.
  - Depth-tick at `:3534-3540` caps per-tick size by `(Available * lev) / price` — can blow past SF's allocated share, causing later SF orders to fail.
  - L5 safety transfer at `:1743-1751` calls `TransferToSpot` on the *full* `bal.Available` — this includes SF's earmarked margin, so after L5 fires SF has no futures margin for its open hedge → forced liquidation of SF.
  - L4 re-check at `:1700-1708` compares `bal.MarginRatio` — same combined ratio.
- **Severity:** MEDIUM to HIGH for L5 case specifically (L5 could liquidate SF hedges as a side effect of PP emergency response).
- **Impact:** Capacity over-commit at entry, SF-hostile L5 safety transfer.
- **Suggested fix:** Introduce `engineAvailable(bal, exchName) = bal.Available - sfReservedMargin(exchName)` helper. Use everywhere PP treats "available" as its own capacity.
- **Already fixed in v0.32.13?** No

### Finding 14: allocator (rebalance) uses unified totals

- **Location:** `internal/engine/allocator.go:1566, 1593, 1766, 1832, 1860`
- **Function:** `rebalanceFunds` donor selection, transfer execution, recipient polling.
- **What it reads:** `GetFuturesBalance` on donor/recipient; unified-donor path explicitly uses futures balance as the withdrawable pool.
- **Why it's wrong:** Donor surplus math in `rebalanceFunds` (see the section around `:1566`) calls `capByMarginHealth(donorBal) - needs[bestDonor]` where `donorBal` is the combined futures view. Unified donors' `maxTransferOut` is exchange-authoritative (so physical damage is capped), but the allocator can still transfer funds *away* from an exchange that SF is about to need (e.g. SF autoentry scheduled in next scan will find less available and skip).
- **Severity:** MEDIUM
- **Impact:** SF autoentries skipped due to "insufficient balance" right after PP rebalance cycle; SF monitor may also misinterpret the margin shift.
- **Suggested fix:** Subtract `SFCommittedOnExchange(name)` from donor `futures/spot/futuresTotal` before adding to `surplus` pool.
- **Already fixed in v0.32.13?** No

### Finding 15: SF exit_manager margin trigger fires on PP stress

- **Location:** `internal/spotengine/exit_manager.go:160` (Dir A futures margin check), `:185` (Dir B)
- **Function:** `checkExitTriggers` margin-health trigger.
- **What it reads:** `futExch.GetFuturesBalance().MarginRatio` — unified pool ratio.
- **Why it's wrong:** When PP holds large positions on the same exchange, their drawdown moves the unified `MarginRatio` past SF's `SpotFuturesMarginExitPct` (default 85%) / `SpotFuturesMarginEmergencyPct` (default 95%). Code emits `margin_health_exit` and `isEmergency=true` at 95% — SF emergency-exits a healthy spot-futures position because PP is in trouble.
- **Severity:** MEDIUM (deterministic: any time PP hits L4, SF on same exchange will exit)
- **Impact:** SF gives up profitable funding collection during PP stress events. Real loss if spot-margin borrow costs were already paid and position exits before collecting funding. PP's L4 reduce-50% + SF's emergency-exit compound the response — both fire in the same tick.
- **Suggested fix:** Use per-position `Position.MarginRatio` from `GetPosition(symbol)` if adapters expose it, or use `GetPosition` to compute "SF-only margin ratio = SF leg notional / (SF-allocated capital)". The capital allocator already knows SF's share.
- **Already fixed in v0.32.13?** No

### Finding 16: SF pending-entry recovery absorbs PP leg

- **Location:** `internal/spotengine/execution.go:1106`
- **Function:** `pendingEntryFuturesPosition` — called during SF pending-entry reconciliation (`reconcilePendingEntry`).
- **What it reads:** `exch.GetPosition(symbol)` filtered by side; returns first row whose `|qty| > tolerance`.
- **Why it's wrong:** If PP already has a position on the same symbol+side+exchange, this reads PP's `Total` and `AverageOpenPrice` and assigns them to the pending SF entry. SF position now claims PP's leg as its futures hedge; later SF exit will reduceOnly against PP's leg.
- **Severity:** MEDIUM (rare — requires SF to have placed order, failed to confirm, then PP has/gets a leg on the same exchange/side before SF's monitor tick). Combined with finding 7 on PP side, this is a bilateral risk.
- **Impact:** SF owns phantom leg. On SF exit, closes PP's leg.
- **Suggested fix:** Scope reconciliation to the specific `orderID` SF placed (query by order status, not by symbol-position). All adapters support order-status queries; SF already does `confirmFuturesFill(orderID)` in the happy path — the pending-entry recovery path just needs the same discipline.
- **Already fixed in v0.32.13?** No

### Finding 17: Consolidator orphan scan relies on spotFuturesKeys mapping correctness

- **Location:** `internal/engine/consolidate.go:173` (and the filter at `:189`)
- **Function:** Orphan scan — `GetAllPositions()` across all exchanges, filtered by `spotFuturesKeys`.
- **What it reads:** All exchange positions; excludes keys in `spotFuturesKeys`.
- **Why it's wrong:** The filter derives `spotFuturesKeys` from SF's `Direction` → side mapping: `buy_spot_short → short`, else `long`. This is correct today. However, any future SF direction variant or data-corruption case would lead the consolidator to treat an SF futures leg as an orphan and market-close it.
- **Severity:** LOW (defensive concern — current code is correct)
- **Impact:** Would cause SF hedge unexpected-close if mapping ever drifts.
- **Suggested fix:** Add a secondary filter: if `(name, ep.Symbol)` matches ANY SF position for that exchange regardless of side, log at WARN and skip the orphan close. Safer fail-open.
- **Already fixed in v0.32.13?** No

### Finding 18: Single WS private-fills callback per exchange

- **Location:** `internal/engine/engine.go:1647-1650` — `setupSLCallbacks`
- **Function:** Registers one callback per exchange; all private fills stream into PP's `slFillCh`.
- **What it reads:** Every `OrderUpdate` from the private WS (PP + SF orders).
- **Why it's wrong:** SF does not register its own callback. The callback interface is single-consumer (`SetOrderCallback` stores a function; newer calls overwrite). SF fills still reach PP's `handleSLFill`; they're filtered out only by the method-1 (`slIndex` miss) and method-2 (`remaining > 0` or `ownOrders` check). Correctness depends on those filters being sound — see findings 1, 19.
- **Severity:** LOW (current behaviour is correct *if* findings 1 and 19 are fixed; without them this is part of the HIGH attack surface)
- **Impact:** Coupling between engines via shared callback.
- **Suggested fix:** Fan-out dispatcher: `SetOrderCallback` once, route each fill to both engines by orderID prefix/map lookup. Or have SF register its own `SetOrderCallback` and make the adapter support a slice of callbacks.
- **Already fixed in v0.32.13?** No

### Finding 19: ownOrders is PP-only

- **Location:** `internal/engine/engine.go:1532` (consumer), `:106-109` (definition). Populated by PP at every `PlaceOrder` call.
- **Function:** Method-2 SL filter — looks up `ownOrders` to skip known bot-initiated reduceOnly fills.
- **What it reads:** Order IDs PP has placed.
- **Why it's wrong:** SF's reduce-only fills (futures-leg close on SF exit) are NOT in `ownOrders`, so method 2 runs the full `getExchangePositionSize` verify path. Today this is caught by `remaining > 0` when PP's leg is still open. If PP has just quietly exited and registered the close before SF's reduce-only fill arrives, `remaining = 0` → PP emits emergency-close on the already-closed position (finding 1).
- **Severity:** LOW (scenario requires precise timing)
- **Impact:** Same as finding 1 edge case.
- **Suggested fix:** Promote `ownOrders` to a shared map populated by both engines. Spot-futures `PlaceOrder` calls in `execution.go` and `exit_manager.go` need to write to it.
- **Already fixed in v0.32.13?** No

### Finding 20: getUnrealizedPnL cosmetic cross-contamination

- **Location:** `internal/engine/engine.go:4643` (long), `:4653` (short)
- **Function:** `getUnrealizedPnL` helper used for logging.
- **What it reads:** Per-side UPL from `GetPosition(symbol)`.
- **Why it's wrong:** Sums SF + PP UPL on same symbol+side. But it's only used in log lines (grep shows callers are Info-level logs around exit decisions).
- **Severity:** LOW (cosmetic)
- **Impact:** Misleading log line only.
- **Suggested fix:** Same offset subtraction pattern, or document as best-effort log metric.
- **Already fixed in v0.32.13?** No

### Finding 21: API spot handlers list all positions

- **Location:** `internal/api/spot_handlers.go:628, 649`
- **Function:** Dashboard "spot" tab position list.
- **What it reads:** `GetAllPositions()` across all exchanges.
- **Why it's wrong:** If handler doesn't filter by `spotFuturesKeys`, PP positions may appear in the spot tab. This is a display concern only (no trading actions taken).
- **Severity:** LOW (cosmetic, but can mislead user)
- **Impact:** Dashboard clutter.
- **Suggested fix:** Apply the same spotFuturesKeys filter the consolidator uses.
- **Already fixed in v0.32.13?** No (not verified whether filter is already present; flagged for review)

---

## WebSocket Cross-Contamination Analysis

**Fill-callback flow (all exchanges):**
1. Adapter registers exactly one `orderCallback` field (set via `SetOrderCallback`).
2. PP's `setupSLCallbacks` (engine.go:1644) iterates all exchanges and installs a non-blocking sender into `slFillCh`.
3. SF registers NO private-WS callback.
4. When SF places a futures order (`execution.go` and `exit_manager.go`), its fill events nonetheless flow through PP's callback → `slFillCh` → `handleSLFill`.

**Filter chain for an SF fill reaching `handleSLFill`:**
- Method 1: `slIndex[exchange:orderID]` — SF orderIDs are never inserted, so miss.
- ReduceOnly check: SF closes are often reduceOnly=true, so this gate passes.
- `ownOrders` check: SF orderIDs are never inserted, so miss.
- Symbol-match: if any PP active position exists on `(exchName, symbol)` on the side matching the fill's direction, code proceeds to verify.
- Verify: `getExchangePositionSize` returns SF residual (if SF not fully closed) OR 0 (if SF fully flat + PP also flat). The 0 case is the problematic path (finding 1).

**Conclusion:** WS callback layer has no hard isolation. Every finding (1, 18, 19) depends on the verify step in `handleSLFill` being correct, which itself depends on cross-engine offset subtraction. Highest-leverage fix: share `ownOrders` between engines and have SF register its own callback (dispatcher pattern).

---

## Shared Risk Layer Analysis

### risk/manager.go
- Uses `GetFuturesBalance` for capital approval (findings 12, 13). The 11-point check's capital gate is the primary injection point where PP over-commits on unified accounts.
- `ensureFuturesBalance` sweeps spot → futures, which can starve SF autoentries that expect spot balance (for borrow collateral in Dir A). Documented at line 942 but not SF-aware.
- No direct `GetAllPositions` usage — approvals use `db.GetActivePositions` (PP record) for exposure math, which is self-consistent.

### risk/health.go
- Per-exchange tier (L0-L5) reads unified `MarginRatio` but drives PP-only reductions for L3; L4 and L5 correctly receive both `exchPositions` and `exchSpotPositions` since Phase 6 (line 250-252). Gap: L3 and trend handler at `handleTrend` (line 340) only pass PP positions.
- Orphan-leg scan uses `GetAllPositions` unfiltered (finding 10).

### risk/allocator.go (CapitalAllocator)
- Abstract capital accounting (SF reservations tracked). No exchange API calls here — this layer is clean. It's the *consumers* of its `ReservedMargin`/`Committed` values that don't subtract them from exchange balance views (findings 12, 13, 14).

---

## Priority Matrix

### P0 — Fix immediately (HIGH severity, can cause wrong trades or loss)

| # | Title | Rationale |
|---|-------|-----------|
| 3 | markPositionClosed closes SF leg | Direct market-close of SF hedge |
| 6 | closePositionWithMode post-close verify | Direct twin of v0.32.13 rotation fix — same bug class, unfixed path |
| 7 | Entry fallback absorbs SF size | Writes wrong size to PP record at entry time; poisons all downstream logic |
| 2 | Orphan-cleanup verify | PP imports SF size, next consolidator closes SF leg |
| 5 | reconcilePartialPosition | Trims/promotes against SF leg during partial-entry recovery |
| 4 | markPositionClosed verify ghost | Creates ghost Active positions that never close |
| 1 | SL Method-2 verify edge case | Rare but can trigger unintended emergency close |

### P1 — Fix soon (MEDIUM severity, wrong decisions, eventual self-correction possible)

| # | Title | Rationale |
|---|-------|-----------|
| 15 | SF exit_manager triggers on PP margin | Deterministic: any PP L4 triggers SF exit on same exchange |
| 13 | L5 safety transfer drains SF margin | L5 side effect can liquidate SF hedge |
| 11 | Health tier L3 ignores SF | L3 transfer loop wastes cycles / moves funds incorrectly |
| 16 | SF pending-entry absorbs PP leg | Bilateral twin of finding 7 |
| 12 | risk/manager approval over-commits | Causes failed orders + retries on unified accounts |
| 14 | Rebalance allocator uses unified totals | Starves SF of capacity |
| 9 | risk/monitor cross-engine UPL | Noisy false alerts |
| 10 | health orphan detection | Delayed/missed detection + would hit finding 2 if dispatched |
| 8 | updateFundingCollected attribution | P&L misstatement (cosmetic→real impact in analytics) |

### P2 — Fix later (LOW severity, cosmetic/defensive)

| # | Title | Rationale |
|---|-------|-----------|
| 17 | Consolidator orphan mapping drift | Defensive hardening |
| 18 | Single WS callback | Foundation for cleaner fixes to 1/19 |
| 19 | ownOrders PP-only | Part of WS cross-contamination class |
| 20 | getUnrealizedPnL log math | Cosmetic |
| 21 | Dashboard spot handler filter | Cosmetic |

---

## Recommended Fix Strategy

**Option A: Per-site `sfSizeOffset` subtraction (what v0.32.13 did)**
- Pros: Minimal code change per site; matches the landed fix style
- Cons: Must remember every site; easy to regress; 7 HIGH + ~5 MEDIUM sites to touch + every new site added in future
- Verdict: Acceptable only as a short-term patch for P0 items

**Option B: Wrapper helper — `getEnginePositionSize(engine, exch, sym, side)`**
- Introduce in `internal/engine/` (and mirror in `internal/spotengine/`) a helper that:
  1. Calls `exch.GetPosition(sym)` once
  2. Loads SF offset map (cached per scan cycle) or PP offset map
  3. Returns the engine-scoped size
- Replace all `getExchangePositionSize` callers in both engines.
- Similar wrapper for `getEngineFuturesBalance(bal, exchName, engine)` that subtracts the other engine's reserved margin (available via `CapitalAllocator.CommittedByExchange(strategy, exchName)`).
- Pros: Single source of truth for cross-engine offset; natural place for defensive logging
- Cons: Moderate refactor, must plumb SF position map into all call sites (but that's already done in consolidator — generalize the pattern)
- **Verdict: Recommended.** This is the right mid-term fix. Roughly 2-3 days of focused work, bounded blast radius.

**Option C: Architectural — tag positions at adapter level**
- Add a `SourceTag` or `ClientOrderIDPrefix` parameter to `PlaceOrderParams`; have adapters record it in each `Position` row.
- Engines filter by tag when reading positions.
- Pros: Clean separation, future-proof for additional engines
- Cons: Requires adapter changes across all 6 exchanges; not all exchanges support per-position tagging; would touch the 35-method Exchange interface; test burden is high
- Verdict: Long-term direction, not appropriate for the current interference bug window

**Proposed sequencing:**

1. **Immediate patch (P0, Option A):** Apply `sfSizeOffset` subtraction to the 7 HIGH sites. This is mechanical, each site is ≤10 lines, mirrors the v0.32.13 fix. Ship as v0.32.14.
2. **Short-term refactor (P0+P1, Option B):** Introduce `getEnginePositionSize` + `getEngineFuturesBalance` helpers; migrate all P0+P1 sites. Ship as v0.33.0.
3. **WS isolation (P2 findings 18, 19):** Promote `ownOrders` to shared state, convert adapter callback to fan-out. Enables fully safe finding 1 resolution.
4. **Long-term (Option C):** Reserve for a v1.x architectural refresh, separate milestone.

**Testing priorities:**
- Add integration-style test fixtures that place overlapping PP + SF positions on a mocked exchange and exercise every P0 code path (markPositionClosed, closePositionWithMode verify, reconcilePartial, orphan-cleanup, entry confirmFill fallback).
- For P1, add unit tests for the allocator-aware `getEngineFuturesBalance` helper and the SF exit_manager's per-position margin ratio.

---

## Notes

- The word "exchange-total" here refers to what `exch.GetPosition(symbol)` returns — i.e., the sum of all rows with matching side. This is exchange-wide, not engine-filtered.
- `sfSizeOffset` referenced throughout is the map built in `consolidate.go:123-140` from `db.GetActiveSpotPositions()`. Any fix should reuse that construction (or extract to a shared helper).
- All findings are static analysis only — not runtime-confirmed. Some edge cases (especially finding 1's timing window) may not trigger in practice.
- No source code was modified during this audit.
