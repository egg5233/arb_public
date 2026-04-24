# PLAN: Per-Leg Tier 1 Skip for Exchanges Without CloseSize

Version: v2
Date: 2026-04-24
Status: DRAFT

## Changelog
- **v2** (2026-04-24): Codex review round 1 — NEEDS-REVISION 4 items. Fixed:
  - #1 control-flow bug (my v1 skipped Tier 1 on Binance leg but Tier 2 sits in
    `if !useTier1` — Binance leg got NO completeness check at all). Added Phase 3
    "Tier 1 partial" fallback using `pos.LongCloseSize`/`pos.ShortCloseSize`
    notional (depth-exit safe).
  - #2 test plan: reference real test names (`TestReconcileRetriesWhenCloseSizePartial`
    etc.); add mixed-leg + sibling-aware + rotation cases; explicit note that
    rotation window keys off `RotationHistory` not `LastRotatedFrom`.
  - #3 risk section: honestly state Binance leg has weaker protection than pure
    non-Binance pairs; the trade-off is acceptable because current behavior is
    "always fail" which is strictly worse.
  - #4 CHANGELOG format to repo convention (`## [0.33.4] - 2026-04-24`); anchor
    "Binance both legs does not exist" to `rankPairs` and `ManualOpen` paths.
- **v1** (2026-04-24): initial draft.

## Problem

Dashboard 交易歷史顯示 fallback 文案「詳細分解僅適用於 v0.27.0 之後平倉的部位」for
recent positions closed on v0.33.3 (e.g. PORTALUSDT 2026-04-24 08:02,
binance→bingx).

Root cause chain (verified by prior Codex review VERDICT=PARTIAL, with the two
corrections applied):

1. `web/src/components/PnLBreakdown.tsx:23-38` — `hasDecomposition` is inferred
   from non-zero breakdown fields. When all zero, renders `hist.dataUnavailable`.

2. Per-leg breakdown fields are written in **two** paths:
   - **Async reconcile** (`internal/engine/exit.go:1272-1335` in `tryReconcilePnL`) — the
     normal post-close path.
   - **Consolidator** (`internal/engine/consolidate.go:685-690`) — not affected.

3. `tryReconcilePnL` has Tier 1 completeness gate at `exit.go:1196-1208` (added
   by commit `8f236707`, SIRENUSDT fix, v0.32.x):
   ```go
   useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
       allSiblingsHaveCloseSize(longSiblings, "long") &&
       allSiblingsHaveCloseSize(shortSiblings, "short")

   if useTier1 {
       longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
       shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
       if longAgg.CloseSize < longExpected-sizeEpsilon ||
          shortAgg.CloseSize < shortExpected-sizeEpsilon {
           return false
       }
   }
   ```

4. Binance `GetClosePnL` (`pkg/exchange/binance/adapter.go:1125-1172`) sums
   `/fapi/v1/income` records (`REALIZED_PNL`, `COMMISSION`, `FUNDING_FEE`).
   Binance income API does not expose `qty` (confirmed against
   `doc/EXCHANGEAPI_BINANCE.md:823-857`), so the returned `exchange.ClosePnL`
   has `CloseSize=0` and `Side=""`.

5. Result: every position with Binance as either leg fails the Tier 1 gate on
   every attempt (1 sync + 3 async retries), leaves all breakdown fields at 0.
   Frontend falls back to `hist.dataUnavailable`.

### Evidence (portalusdt-1776757502428, closed 2026-04-24 08:02)

```
longAgg  Fees=-0.393909 Funding=1.277987 PricePnL=-56.998471 NetPnL=-56.114394 (CloseSize=0)
shortAgg Fees=-0.272256 Funding=-0.334333 PricePnL=56.354500  NetPnL=55.747900  (CloseSize=21466.9)
WARN  incomplete close data (longRawClose=0.000000/21466.900000 shortRawClose=21466.900000/21466.900000), retrying
... attempts 2, 3, 4 identical ...
ERROR reconcile portalusdt-1776757502428: all attempts failed, keeping PnL=-4.9528
```

### Current control flow in `tryReconcilePnL` (what makes this tricky)

The gate is one of three mutually-exclusive branches:

```
Phase 1 (if useTier1)   — cross-leg CloseSize completeness check (SIRENUSDT)
Phase 2                 — splitSharedPnL, compute reconciledPnL, diff
Phase 3 (if !useTier1)
  ├─ Tier 2 (pos.LongSize>0 || pos.ShortSize>0) — notional guard using LIVE sizes
  └─ Tier 3 (live sizes zero)                   — longOK && shortOK fallback
```

For depth-exit closes (the common case — PORTALUSDT is a depth-exit), `pos.LongSize`
and `pos.ShortSize` are zeroed by `closeSingleLeg`. `pos.LongCloseSize` /
`pos.ShortCloseSize` are preserved (per the comment at `internal/models/position.go:58-63`).
So under v1's naive "just skip the Binance leg" proposal, the control flow would
have been:
- `useTier1=true` (local fields populated) → enter Tier 1 block → skip Binance leg
  check → pass
- Phase 3 `if !useTier1` → FALSE → Tier 2/3 never runs
- Result: reconcile proceeds with **no completeness check on the Binance leg at all**

Codex flagged this as #1: "skipping one leg and bypassing Tier 2 entirely." v2
adds a proper fallback.

### Why not fix A (populate CloseSize in Binance GetClosePnL)

Codex flagged fix A as unsafe for minimal scope (prior review):
- `since = CreatedAt-1m` (or `lastRotation-1m`; `exit.go:1141-1144`) — userTrades
  in this window includes **entry** fills too. Naively summing `|qty|` would
  double-count.
- Binance `/income` records lack `positionSide`; `GetClosePnL(symbol, since)`
  has no "which side am I" signal at adapter level.
- Proper A fix requires correlating `/income` `tradeId` with `/userTrades` rows
  (doc `EXCHANGEAPI_BINANCE.md:500-520`, `:843-854`) with closer-only derivation.
  That is a larger adapter change deferred to its own plan.

## Solution: Per-Leg Tier 1 + Tier 1 Partial Fallback

Revised design (v2):

1. **Phase 1 (Tier 1)**: keep `useTier1` precondition unchanged. Inside the
   block, apply the completeness check **per leg**: only enforce the check on
   legs whose aggregate reports `CloseSize > 0`. Track which legs bypassed
   the check.

2. **Phase 2**: unchanged (split + diff).

3. **Phase 3 (new "Tier 1 partial")**: if `useTier1 == true` AND at least one
   leg bypassed the Tier 1 check (exchange reported `CloseSize=0`), run a
   notional guard based on `pos.LongCloseSize`/`pos.ShortCloseSize` (depth-exit
   safe — these fields are preserved post-close). If `!useTier1`, existing
   Tier 2/Tier 3 logic runs exactly as today.

Key properties:
- **Binance + non-Binance pair** (e.g. binance↔bingx/bybit/gate/okx/bitget):
  non-Binance leg gets full Tier 1 protection. Binance leg skipped in Tier 1,
  protected by the new Tier 1 partial notional guard.
- **Pure non-Binance pair** (e.g. bybit↔bingx): both legs enforce Tier 1.
  Zero behavioral change. SIRENUSDT protection intact.
- **Binance + Binance pair**: does not exist in this bot. `internal/discovery/ranker.go`
  `rankPairs` (~L177) iterates `rates []exRate` with `i != j` guard; `rates` is
  built from distinct entries of `s.activeExchanges()` (one entry per exchange).
  Manual entry `Engine.ManualOpen` (`internal/engine/engine.go` ~L371-382) only
  picks from scanner-cached opportunities, which come from `rankPairs`. Thus
  long_exchange != short_exchange is enforced at the pair-generation layer.

## Changes

### Change 1: `internal/engine/exit.go` — Tier 1 per-leg + Tier 1 partial fallback

Current code path (abbreviated):

```go
// Phase 1 (:1189-1208)
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
    if longAgg.CloseSize < longExpected-sizeEpsilon || shortAgg.CloseSize < shortExpected-sizeEpsilon {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data ..., retrying", ...)
        return false
    }
}

// Phase 2 (:1210-1231) — splitSharedPnL, compute reconciledPnL + diff
...
diff := reconciledPnL - oldPnL

// Phase 3 (:1233-1254) — Tier 2 / Tier 3 only if !useTier1
if !useTier1 {
    if pos.LongSize > 0 || pos.ShortSize > 0 {
        // Tier 2
        longNotional := pos.LongEntry * pos.LongSize
        shortNotional := pos.ShortEntry * pos.ShortSize
        notional := math.Max(longNotional, shortNotional)
        if notional > 0 && math.Abs(diff) > notional {
            return false
        }
    } else {
        // Tier 3: fallback — longOK && shortOK already checked earlier
    }
}
```

Target code:

```go
// Phase 1 (:1189-1208) — per-leg Tier 1
const sizeEpsilon = 1e-6
longSiblings := e.siblingsFor(pos, "long")
shortSiblings := e.siblingsFor(pos, "short")
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

// Track which legs had their Tier 1 check enforced (vs skipped because the
// exchange's close-PnL endpoint doesn't expose size — currently Binance /income).
longTier1Enforced := false
shortTier1Enforced := false

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")

    // Per-leg enforcement: only enforce where the aggregate reports size.
    // CloseSize=0 from the aggregate means the exchange's close-PnL endpoint
    // does not expose per-close size (Binance /fapi/v1/income).
    longTier1Enforced = longAgg.CloseSize > 0
    shortTier1Enforced = shortAgg.CloseSize > 0

    longIncomplete := longTier1Enforced && longAgg.CloseSize < longExpected-sizeEpsilon
    shortIncomplete := shortTier1Enforced && shortAgg.CloseSize < shortExpected-sizeEpsilon

    if longIncomplete || shortIncomplete {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f longEnforced=%v shortEnforced=%v), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected,
            longTier1Enforced, shortTier1Enforced)
        return false
    }

    if !longTier1Enforced || !shortTier1Enforced {
        e.log.Info("reconcile %s [attempt %d]: Tier 1 partial (longEnforced=%v shortEnforced=%v) — Tier 1 partial notional guard will apply post-split",
            pos.ID, attempt, longTier1Enforced, shortTier1Enforced)
    }
}

// Phase 2 (unchanged) — splitSharedPnL, compute reconciledPnL, diff
...
diff := reconciledPnL - oldPnL

// Phase 3 — Tier 2/Tier 3 when !useTier1, OR Tier 1 partial fallback when at
// least one leg bypassed Tier 1.
if !useTier1 {
    if pos.LongSize > 0 || pos.ShortSize > 0 {
        // Tier 2 (unchanged)
        longNotional := pos.LongEntry * pos.LongSize
        shortNotional := pos.ShortEntry * pos.ShortSize
        notional := math.Max(longNotional, shortNotional)
        if notional > 0 && math.Abs(diff) > notional {
            e.log.Warn("reconcile %s [attempt %d]: pre-migration diff %.4f exceeds notional %.4f, retrying",
                pos.ID, attempt, diff, notional)
            return false
        }
    } else {
        // Tier 3 (unchanged)
        e.log.Warn("reconcile %s [attempt %d]: pre-migration depth-exit, no size info — relying on longOK && shortOK",
            pos.ID, attempt)
    }
} else if !longTier1Enforced || !shortTier1Enforced {
    // Tier 1 partial — at least one leg was skipped in Tier 1 because the
    // exchange did not expose CloseSize. Apply a notional guard using
    // pos.LongEntry*pos.LongCloseSize (rather than LIVE pos.LongSize, which
    // is zeroed after depth-exit close). This is strictly stronger than what
    // the unreached Tier 3 longOK-shortOK fallback would provide for this
    // close shape.
    longNotional := pos.LongEntry * pos.LongCloseSize
    shortNotional := pos.ShortEntry * pos.ShortCloseSize
    notional := math.Max(longNotional, shortNotional)
    if notional > 0 && math.Abs(diff) > notional {
        e.log.Warn("reconcile %s [attempt %d]: Tier 1 partial diff %.4f exceeds close-notional %.4f (longEnforced=%v shortEnforced=%v), retrying",
            pos.ID, attempt, diff, notional, longTier1Enforced, shortTier1Enforced)
        return false
    }
}
```

**Behavior matrix (summarized in the test table below):**

| Pair                         | long Tier 1 | short Tier 1 | Phase 3 path        |
|------------------------------|-------------|--------------|----------------------|
| non-Binance × non-Binance    | enforced    | enforced     | (no guard — both Tier 1 OK) |
| Binance long, non-Binance    | skipped     | enforced     | Tier 1 partial notional guard |
| non-Binance long, Binance    | enforced    | skipped      | Tier 1 partial notional guard |
| Binance × Binance            | n/a — pair layer does not produce this (rankPairs `i != j`, single exchange entry per `activeExchanges()`). |
| pre-migration (`useTier1=false`) | unchanged   | unchanged    | Existing Tier 2 / Tier 3 |

**Non-goals / deliberately unchanged:**
- `splitSharedPnL`, `aggregateClosePnLBySide`, Phase 2 math untouched.
- Binance `GetClosePnL` untouched — fix A deferred.
- `tryReconcilePnL` return/retry semantics unchanged (still return `false` to
  retry on incomplete/mismatched).
- Consolidator path (`consolidate.go`) unchanged — it does not use the Tier 1
  gate and was already writing breakdown fields correctly for its own close
  path.
- `reconcileRotationPnL` (`exit.go:3167-3185` per Codex review) unchanged — it
  already accepts empty-Side records without a Tier 1 size gate.

### Change 2: Regression tests

Add new file `internal/engine/reconcile_tier1_per_leg_test.go`. Keeps
`sirenusdt_fixes_test.go` focused on its original fix and follows the
established "inline logic test, no DB scaffolding" pattern used by e.g.
`TestReconcileRetriesWhenCloseSizePartial` (`sirenusdt_fixes_test.go:391-411`).

Testable surfaces (inline):

```go
// Tier 1 per-leg decision logic (replicates the Phase 1 block):
//   longTier1Enforced := longAgg.CloseSize > 0
//   shortTier1Enforced := shortAgg.CloseSize > 0
//   longIncomplete := longTier1Enforced && longAgg.CloseSize < longExpected-sizeEpsilon
//   shortIncomplete := shortTier1Enforced && shortAgg.CloseSize < shortExpected-sizeEpsilon
//   incomplete := longIncomplete || shortIncomplete

// Tier 1 partial fallback decision (replicates Phase 3 else-if):
//   anyLegSkipped := !longTier1Enforced || !shortTier1Enforced
//   longNotional := pos.LongEntry * pos.LongCloseSize
//   shortNotional := pos.ShortEntry * pos.ShortCloseSize
//   notional := math.Max(longNotional, shortNotional)
//   trips := notional > 0 && math.Abs(diff) > notional
```

Table-driven test cases:

| # | Name                                          | long agg CloseSize | short agg CloseSize | longExpected | shortExpected | Tier 1 result |
|---|-----------------------------------------------|--------------------|---------------------|--------------|---------------|----------------|
| A | `BothSizedComplete`                           | 100                | 100                 | 100          | 100           | pass           |
| B | `LongSizedShort`                              | 80                 | 100                 | 100          | 100           | retry          |
| C | `ShortSizedShort`                             | 100                | 80                  | 100          | 100           | retry          |
| D | `LongUnreportedShortComplete` (Binance long)  | 0                  | 100                 | 100          | 100           | pass, long skipped  |
| E | `LongUnreportedShortShort` (Binance long)     | 0                  | 80                  | 100          | 100           | retry (short still enforced) |
| F | `ShortUnreportedLongComplete` (Binance short) | 100                | 0                   | 100          | 100           | pass, short skipped |
| G | `BothUnreported` (hypothetical only)          | 0                  | 0                   | 100          | 100           | pass, both skipped  |

Tier 1 partial fallback cases (only fire when ≥1 leg skipped):

| #  | Name                                     | skipped leg(s) | diff | long close-notional | short close-notional | result |
|----|------------------------------------------|----------------|------|---------------------|----------------------|--------|
| H  | `PartialWithinNotional`                  | long only      | 1.23 | 240                 | 240                  | pass   |
| I  | `PartialExceedsNotional`                 | long only      | 999  | 240                 | 240                  | retry  |
| J  | `PartialZeroNotional`                    | long only      | 5.0  | 0                   | 0                    | pass (notional gate doesn't fire when notional==0 — matches existing Tier 2) |

Sibling-aware mixed-leg cases:

| #  | Name                                       | Coverage |
|----|--------------------------------------------|----------|
| K  | `MixedLegWithSibling`                      | pos.LongCloseSize=100, sibling long CloseSize=100 → longExpected=200; longAgg.CloseSize=0 (Binance) → skipped; sibling logic still flows through `sumSiblingCloseSize` correctly because its result is used in computing longExpected but not in the skip decision. |

Rotation case:

| #  | Name                                       | Coverage |
|----|--------------------------------------------|----------|
| L  | `RotationNoopOnTier1PerLeg`                | Build a position with non-empty `RotationHistory`. Confirm `since` derivation (`exit.go:1141-1144`) still keys off `RotationHistory[last].Timestamp-1m`. Note in the test comment: `LastRotatedFrom` is used elsewhere for event-routing but `tryReconcilePnL` does NOT read it; `reconcileRotationPnL` (`exit.go:3167-3185`) is a separate method. |

Existing tests MUST continue to pass:
- `TestAllSiblingsHaveCloseSizeTrueWhenAllPopulated` (`sirenusdt_fixes_test.go:333`)
- `TestAllSiblingsHaveCloseSizeFalseWhenOneMissing` (`:348`)
- `TestSumSiblingCloseSizeAggregates` (`:360`)
- `TestReconcileAcceptsDepthExitWithCloseSizeMatch` (`:377`)
- `TestReconcileRetriesWhenAggregationFails` (`:381`)
- `TestReconcileRetriesWhenCloseSizePartial` (`:391`)
- `TestReconcileAcceptsSharedPositionCompleteClose` (`:413`)
- `TestReconcileRetriesSharedPositionPartial` (`:434`)
- `TestReconcileMixedHistoryFallsThroughToTier2` (`:449`)
- `TestReconcilePreMigrationNormalCloseRetainsNotionalGuard` (`:466`)
- `TestReconcileDepthExitPreMigrationFallback` (`:500`)

These cover the original Tier 1/Tier 2/Tier 3 contract; the v2 change is additive
(new per-leg decision + new partial fallback) and does not alter the existing
branches when both legs report `CloseSize > 0`.

### Change 3: VERSION + CHANGELOG

- `VERSION`: `0.33.3` → `0.33.4`.
- `CHANGELOG.md` (match repo format, see `CHANGELOG.md:5-14`):
  ```
  ## [0.33.4] - 2026-04-24

  ### Fixed
  - **Reconcile Tier 1 gate no longer blocks Binance-leg positions** —
    `tryReconcilePnL` at `internal/engine/exit.go:1196-1208` required both legs'
    exchange-reported `CloseSize` to match local expected size, but Binance's
    `GetClosePnL` at `pkg/exchange/binance/adapter.go:1125-1172` aggregates
    `/fapi/v1/income` records which have no `qty` field, so Binance-leg records
    always reported `CloseSize=0`. Every position with a Binance leg failed all
    4 reconcile attempts, leaving per-leg breakdown fields at zero, which the
    dashboard (`web/src/components/PnLBreakdown.tsx:23-38`) rendered as the
    "詳細分解僅適用於 v0.27.0 之後平倉的部位" fallback. Fix: per-leg Tier 1
    enforcement — legs whose aggregate reports `CloseSize=0` are skipped in
    Tier 1, and a new "Tier 1 partial" notional fallback based on
    `pos.LongCloseSize` / `pos.ShortCloseSize` runs post-split to guard gross
    corruption on the skipped leg. Pure non-Binance pairs retain full Tier 1
    protection (regression tests cover the mixed- and pure-pair cases).
  ```

## Out of scope

1. **Fix A** (populate `CloseSize` in Binance `GetClosePnL` via `/fapi/v1/userTrades`
   correlated with income `tradeId`). Deferred — requires closer-only derivation
   and side inference per trade. Own plan.
2. **Frontend fix** (switch `hasDecomposition` to read `p.has_reconciled` /
   `p.partial_reconcile`). Independent design fragility flagged by Codex but not
   required to unblock the current symptom — the current fix restores the
   backend data and the UI inference works again.
3. **Historical backfill** for already-closed Binance-leg positions that missed
   reconciliation. Those keep the fallback text; no retroactive fix.

## Rollout

1. Merge to main, bump VERSION, update CHANGELOG.
2. GitHub Release with binary (per `feedback_always_release.md`), then VPS
   `/update`.
3. Verify on VPS: next Binance-leg close shows `UpdateHistoryEntry succeeded`
   in logs and full breakdown card on dashboard.
4. Monitor for one full cycle before declaring success.

## Risk assessment (honest)

- **Blast radius**: small. Single function, ~30 net added lines in `exit.go`,
  no schema change, no persistent-state migration. New test file only.
- **Rollback**: revert commit. No persistent state touched.
- **Correctness risk (honest)**: 
  - Pure non-Binance pairs: zero risk. Same code path as today.
  - Binance-leg pairs: Tier 1 protection on the Binance leg is replaced by
    Tier 1 partial notional guard (`|diff| <= max(pos.LongEntry*pos.LongCloseSize,
    pos.ShortEntry*pos.ShortCloseSize)`). This is **weaker** than per-leg
    CloseSize match: a Binance-side corruption that kept the cross-leg diff
    within one leg's close-notional would slip through. In practice the
    notional bound is large (hundreds of USDT at typical sizes), so only gross
    corruption is caught. **This is a deliberate trade-off**: today's behavior
    on every Binance-leg position is "retry all 4 attempts, fail, keep local
    PnL, breakdown never populated" — strictly worse. The v2 fallback at least
    catches gross errors and unblocks the UI.
  - SIRENUSDT protection preservation: verified — the SIRENUSDT bug (H1:
    synthetic retrySecondLeg price + H2: reconcile guard + H3: SL/TP detection)
    fires on a non-Binance pair (SIRENUSDT was bybit↔bingx). Pure non-Binance
    pairs still hit full Tier 1 in v2.
- **Review-acknowledged weaknesses**:
  - When both legs are Binance-shape (CloseSize=0), Tier 1 partial notional
    guard is the only completeness check. The bot does not produce such pairs
    (see behavior matrix for Binance×Binance entry), so this is hypothetical.
  - `pos.LongCloseSize` is used as the notional basis. For pre-migration
    positions (`useTier1==false`), this branch does not fire — existing Tier 2
    / Tier 3 runs unchanged. So this introduces no regression for positions
    that predate CloseSize tracking.

## Acceptance

- `go build ./...` passes.
- `go test ./internal/engine/...` passes (all existing tests, plus new
  `reconcile_tier1_per_leg_test.go`).
- After deploy, a Binance-leg close produces `UpdateHistoryEntry succeeded` in
  logs and the trade-history card shows `進場利差 / 已收資金費 / 多方價格 /
  空方價格 / 平倉原因` and the per-leg breakdown table (Fees / Funding /
  ClosePnL / Subtotal / TotalPnL / APR).
