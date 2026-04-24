# PLAN: Per-Leg Tier 1 Skip for Exchanges Without CloseSize

Version: v7
Date: 2026-04-24
Status: DRAFT

## Changelog
- **v7** (2026-04-24): Independent Codex review — NEEDS-REVISION 3 items (found
  what the normal review loop missed). Fixed:
  - **Blocker #1 — `reconcileRotationPnL` silently broken by attempt gate**:
    `reconcileRotationPnL` (`exit.go:3246`) calls `tryReconcilePnL(fresh, 1)`
    exactly once with `attempt=1` and no retry wrapper. v6's Phase 3
    attempt gate rejects every Binance-leg partial path on attempt 1, so
    every rotated position whose current leg uses Binance would fail to
    refresh `RealizedPnL` / history when rotation PnL arrives. v7 fixes by
    having `reconcileRotationPnL` pass `attempt=2` (rotation reconciliation
    already happens after an extended delay — rotation itself takes seconds,
    and this call runs after mutex release + GetPosition re-read, so by
    wall-clock time the Binance `/income` has had ample time to finalize
    across all three income types). Plan now lists `reconcileRotationPnL`
    as **modified** rather than unchanged.
  - **Blocker #2 — Tier 1 bypass scoped too broadly**: v6 keyed per-leg
    enforcement off the runtime signal `longAgg.CloseSize > 0`. Codex
    pointed out that Binance is currently the only adapter that omits
    `CloseSize` (`adapter.go:1165-1172`); all others populate it
    (bybit/gateio/okx/bitget/bingx). A future non-Binance parse bug that
    zeros `CloseSize` would silently downgrade that leg out of Tier 1
    protection, contradicting the "pure non-Binance pairs: zero risk"
    claim. v7 mitigates by **explicit exchange-capability check**: skip
    Tier 1 enforcement on a leg only when the exchange is known to not
    expose CloseSize (currently hardcoded set `{"binance"}`, easy to extend
    when adapter semantics change). Any non-Binance leg with
    `CloseSize == 0` is treated as incomplete exchange data (existing
    behavior) — returns false to retry, with an elevated `ERROR`-level log
    so the anomaly is visible to ops. Adds a new exported helper
    `exposesCloseSize(exchName string) bool` in `exit.go` near the
    aggregator.
  - **Blocker #3 — Test coverage missing wrapper + rotation paths**:
    v6 tests only drove `tryReconcilePnL` at attempts 1 and 2 directly. The
    safety argument for the attempt gate depends on the wrapper
    `reconcilePnL`'s real retry timing (2s presleep + 5s/15s/30s delays,
    `exit.go:1099-1123`) and the rotation follow-up path. v7 adds two
    wrapper-level tests (M, N) using a deterministic time-mock or
    patched delay-override so tests don't actually sleep 52s:
    - `TestReconcilePnL_Wrapper_BinanceLong_RetryChainConverges` (M): feeds
      a Binance-leg position through `reconcilePnL`, asserts that attempt
      1 fails (attempt gate), attempt 2 succeeds, final history has
      breakdown populated, 5s/15s/30s retry scaffolding invoked (verified
      via override on the retry delays).
    - `TestReconcileRotationPnL_BinanceLeg_CompletesAfterRotation` (N):
      builds a rotated closed position with a Binance current leg, calls
      `reconcileRotationPnL`, asserts `tryReconcilePnL(fresh, 2)` is
      actually invoked (the v7 attempt-parameter fix) and that the
      history entry has the rotation PnL plus post-rotation per-leg
      breakdown.
- **v6** (2026-04-24): Codex review round 5 — NEEDS-REVISION 1 item (test/design
  consistency). Fixed:
  - Cases C, E, G in the Layer 1 integration test table listed `true` as the
    expected return while the prose above the table said each case calls
    `tryReconcilePnL(pos, 1)`. The v5 design rejects the Tier 1 partial path
    on attempt 1 for every Binance-leg case, so the stated attempt-1
    expectation contradicted the control flow. v6 makes every
    Binance-leg-acceptance case **explicitly attempt-aware**: the expected
    behavior is now "`false` at attempt 1 then `true` at attempt 2" for
    each Binance-leg acceptance case (C, E, G), plus case I ("true (on
    attempt≥2)" was already qualified — updated for clarity and table
    uniformity). Non-Binance cases (A, B) remain attempt-1 tests. Cases F
    and H remain `false` at all attempts (notional mismatch / other-leg
    CloseSize mismatch). Cases J (pre-migration), K (attempt gate
    dedicated), L (residual hazard) are already attempt-aware. Also
    updated the table header and prose wording to say "each case invokes
    tryReconcilePnL at both attempt 1 and attempt 2 and pins expected
    behavior at each; Binance-leg cases require attempt 2 to accept."
- **v5** (2026-04-24): Codex review round 4 — NEEDS-REVISION 1 item (internal
  consistency). Fixed:
  - Case L ("commission-only snapshot") previously claimed `false at any
    attempt` but the v4 design only rejects on attempt 1. On attempt 2+ a
    commission-only Binance snapshot with `|diff|` inside the notional bound
    would be accepted. v5 makes a choice: **keep the two-layer design (empty
    guard + attempt gate + notional guard) without adding a third "strict
    non-empty on all three fields" guard**, and **rewrite case L** to match
    the actual behavior: `false` at attempt 1 (attempt gate), accept at
    attempt ≥ 2 if `|diff|` fits notional. Case L is now explicitly labeled
    as exercising the **residual hazard** that the plan accepts as
    documented risk. Rationale: adding a strict "all three income types
    non-zero" gate would false-reject genuinely-zero PnL closes (rare but
    legitimate), and would not fully defeat a pathological /income ordering
    where REALIZED_PNL or FUNDING_FEE is delayed indefinitely (`attempt 2+`
    still exhausts retries). The attempt gate + notional guard is the
    chosen trade-off: it catches the common pre-T+5s partial snapshots,
    accepts the rare mid-settlement-at-T+7s+ case as a known
    residual hazard to be monitored via logged metrics, and avoids over-
    rejecting legitimate zero-PnL closes.
- **v4** (2026-04-24): Codex review round 3 — NEEDS-REVISION 1 item. Fixed:
  - #2 (still open): the empty-aggregate guard rejects only fully-zero
    snapshots. Codex pointed out that Binance `GetClosePnL` makes three
    separate `/fapi/v1/income` queries (`REALIZED_PNL`, `COMMISSION`,
    `FUNDING_FEE` at `adapter.go:1128-1162`) and always synthesises one
    `ClosePnL` record even when only one income type has appeared — a
    commission-only snapshot would pass the empty-aggregate guard and
    suppress the retry chain. The docs do not guarantee ordering between
    income types. Mitigation: promote the formerly-optional **attempt-count
    gate** into the plan. Tier 1 partial fallback now rejects on attempt 1
    regardless of the empty-aggregate check, forcing at least one 5s retry
    delay so `/income` has a chance to finalize across all three income
    types. The 2s pre-attempt-1 sleep (`reconcilePnL:1105`) + 5s attempt-1
    retry delay + 15s + 30s together give 52s of budget before all attempts
    exhaust — matches pre-v3 behavior for Binance-settlement timing.
- **v3** (2026-04-24): Codex review round 2 — NEEDS-REVISION 2 items. Fixed:
  - #1 test coverage: added real `tryReconcilePnL` integration tests using the
    existing miniredis + Engine harness (pattern from `engine_stage2_salvage_test.go:75-135`).
    New stub exchange `reconcileStubExchange` returns controlled `GetClosePnL`
    records per test case. The inline logic tests are kept as fast-path
    coverage but no longer the sole coverage — the decision matrix the
    runtime executes is now exercised end-to-end.
  - #2 retry short-circuit hazard: added an **empty-aggregate guard** in Phase 1.
    When a leg reports `CloseSize==0`, require at least one of
    `PricePnL/Fees/Funding` to be non-zero. If all four values are zero on a
    `CloseSize==0` leg, treat the exchange data as unfinalized and return
    `false` to trigger the 5s/15s/30s retry chain — preserving the
    finalization semantics at `exit.go:1162-1186`. Risk section now explicitly
    calls out this hazard and the mitigation.
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

// exposesCloseSize declares which exchanges populate CloseSize in their
// GetClosePnL response. Binance is the only current adapter that omits it
// (/fapi/v1/income has no qty field). When an exchange NOT in this deny-list
// returns CloseSize=0, that is treated as exchange/parse anomaly — retry and
// log at ERROR level so ops can investigate.
exposesCloseSize := func(name string) bool {
    // Only Binance is exempt. Kept as an inline closure near the gate so the
    // reviewer sees the scope at the same place as the gate logic.
    return name != "binance"
}

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")

    // Detect non-Binance leg with CloseSize==0 (exchange anomaly).
    // Handled as incomplete data regardless of aggregate contents; retry.
    if exposesCloseSize(pos.LongExchange) && longAgg.CloseSize == 0 {
        e.log.Error("reconcile %s [attempt %d]: long exchange %s expected to expose CloseSize but got 0 — retrying as incomplete (longAgg=%+v)",
            pos.ID, attempt, pos.LongExchange, longAgg)
        return false
    }
    if exposesCloseSize(pos.ShortExchange) && shortAgg.CloseSize == 0 {
        e.log.Error("reconcile %s [attempt %d]: short exchange %s expected to expose CloseSize but got 0 — retrying as incomplete (shortAgg=%+v)",
            pos.ID, attempt, pos.ShortExchange, shortAgg)
        return false
    }

    // Per-leg enforcement: enforce where either the aggregate reports size OR
    // the exchange is known to expose CloseSize (i.e. runtime zero is an
    // anomaly already handled above — so here, if exposesCloseSize=true, we
    // must have CloseSize>0). Skip enforcement only when the exchange is in
    // the exempt deny-list (currently Binance).
    longTier1Enforced = exposesCloseSize(pos.LongExchange)
    shortTier1Enforced = exposesCloseSize(pos.ShortExchange)

    longIncomplete := longTier1Enforced && longAgg.CloseSize < longExpected-sizeEpsilon
    shortIncomplete := shortTier1Enforced && shortAgg.CloseSize < shortExpected-sizeEpsilon

    if longIncomplete || shortIncomplete {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f longEnforced=%v shortEnforced=%v), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected,
            longTier1Enforced, shortTier1Enforced)
        return false
    }

    // Empty-aggregate guard (anti retry-short-circuit):
    // When a leg has CloseSize==0 (exchange doesn't expose size), we cannot
    // use size to detect "exchange data not finalized yet". Instead, require
    // that at least one of (PricePnL, Fees, Funding) is non-zero on that leg —
    // a fully-zero aggregate on a CloseSize==0 leg almost certainly means
    // /fapi/v1/income hasn't settled the close yet. Return false to trigger
    // the 5s/15s/30s retry chain (matches the finalization semantics at
    // exit.go:1162-1186). Genuinely zero-PnL closes are extremely rare — at
    // typical fee rates, a completed close always generates a non-zero
    // COMMISSION record.
    longUnsettled := !longTier1Enforced && longAgg.PricePnL == 0 && longAgg.Fees == 0 && longAgg.Funding == 0
    shortUnsettled := !shortTier1Enforced && shortAgg.PricePnL == 0 && shortAgg.Fees == 0 && shortAgg.Funding == 0
    if longUnsettled || shortUnsettled {
        e.log.Warn("reconcile %s [attempt %d]: leg aggregate looks unfinalized (longUnsettled=%v shortUnsettled=%v longAgg=%+v shortAgg=%+v), retrying",
            pos.ID, attempt, longUnsettled, shortUnsettled, longAgg, shortAgg)
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
    // exchange did not expose CloseSize. Apply two guards:
    //
    // (1) Attempt-count gate: reject on attempt 1 (sync, immediate). Binance
    //     GetClosePnL makes three separate /fapi/v1/income queries
    //     (REALIZED_PNL, COMMISSION, FUNDING_FEE) with no documented
    //     atomicity between them — a commission-only or realized-PnL-only
    //     snapshot would satisfy the notional bound below and be accepted
    //     as terminal success, defeating the 5s/15s/30s retry chain. Force
    //     at least one retry delay (+5s beyond the existing 2s pre-attempt-1
    //     sleep at reconcilePnL:1105) so all three income types have a
    //     chance to post.
    //
    // (2) Notional guard: |diff| must not exceed the larger close-notional
    //     (pos.LongEntry*pos.LongCloseSize or pos.ShortEntry*pos.ShortCloseSize).
    //     Live pos.LongSize/ShortSize are zeroed post-close, so we use the
    //     preserved CloseSize fields (see position.go:58-63).
    if attempt < 2 {
        e.log.Info("reconcile %s [attempt %d]: Tier 1 partial path — deferring acceptance until attempt 2 so /income has time to finalize (longEnforced=%v shortEnforced=%v)",
            pos.ID, attempt, longTier1Enforced, shortTier1Enforced)
        return false
    }

    longNotional := pos.LongEntry * pos.LongCloseSize
    shortNotional := pos.ShortEntry * pos.ShortCloseSize
    notional := math.Max(longNotional, shortNotional)
    if notional > 0 && math.Abs(diff) > notional {
        e.log.Warn("reconcile %s [attempt %d]: Tier 1 partial diff %.4f exceeds close-notional %.4f (longEnforced=%v shortEnforced=%v), retrying",
            pos.ID, attempt, diff, notional, longTier1Enforced, shortTier1Enforced)
        return false
    }

    // Acceptance log line for VPS monitoring of residual hazard
    // (commission-only or other mid-settlement snapshots that slip past
    // the empty guard + attempt gate but fit within the notional bound).
    // Grep-able label: "Tier 1 partial accepted".
    e.log.Info("reconcile %s [attempt %d]: Tier 1 partial accepted (longEnforced=%v shortEnforced=%v longAgg={Fees=%.4f Funding=%.4f PricePnL=%.4f} shortAgg={Fees=%.4f Funding=%.4f PricePnL=%.4f} diff=%.4f notional=%.4f)",
        pos.ID, attempt, longTier1Enforced, shortTier1Enforced,
        longAgg.Fees, longAgg.Funding, longAgg.PricePnL,
        shortAgg.Fees, shortAgg.Funding, shortAgg.PricePnL,
        diff, notional)
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

**Also modified (v7):**
- `reconcileRotationPnL` at `exit.go:3246`: change the single
  `tryReconcilePnL(fresh, 1)` call to `tryReconcilePnL(fresh, 2)`. Rationale:
  rotation PnL reconciliation runs after rotation execution completes and
  after a mutex round-trip — by wall-clock time, all three Binance `/income`
  income types have had ample time to post. Passing attempt=2 bypasses the
  Phase 3 attempt gate while preserving all other guards (empty-aggregate,
  notional). Without this change, every rotated Binance-leg position would
  fail rotation reconcile silently under v7.
- One-line change with a comment explaining why attempt=2 is safe here.

### Change 2: Regression tests

Two layers:

**Layer 1 — Integration tests against real `tryReconcilePnL`**

New file `internal/engine/reconcile_tier1_per_leg_integration_test.go`. Follows
the pattern established by `internal/engine/engine_stage2_salvage_test.go:75-135`
(`newMiniredisDB` + `buildStage2SalvageEngine`).

Helpers:
```go
// newReconcileStubExchange returns a fullStubExchange (or adapted shape)
// whose GetClosePnL returns a caller-controlled []exchange.ClosePnL. Set
// records = nil to simulate an empty response; set records to a Binance-shape
// single record with Side="" to simulate Binance; set records to a per-side
// slice to simulate bybit/bingx/gate/okx/bitget.
type reconcileStubExchange struct {
    fullStubExchange
    closePnLFn func(symbol string, since time.Time) ([]exchange.ClosePnL, error)
}

func (s *reconcileStubExchange) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
    if s.closePnLFn != nil {
        return s.closePnLFn(symbol, since)
    }
    return nil, nil
}

// buildReconcileEngine wires an Engine with miniredis DB + two stub exchanges
// (long, short). Injects a pre-closed position into history + active store
// with the given fields. Returns the Engine and the position.
func buildReconcileEngine(t *testing.T, longClose, shortClose []exchange.ClosePnL,
    posOpts ...func(*models.ArbitragePosition)) (*Engine, *models.ArbitragePosition) { ... }
```

Integration test cases. Non-Binance cases invoke `tryReconcilePnL(pos, 1)` and
assert the attempt-1 result. Binance-leg cases (any row with a `CloseSize:0,
Side:""` record) invoke `tryReconcilePnL(pos, 1)` then `tryReconcilePnL(pos, 2)`
and assert **both** results; the v5 attempt-count gate rejects every Binance-leg
acceptance on attempt 1 by design. Each test asserts return values and post-call
state from `e.db.GetHistory` / `e.db.GetPosition`.

| # | Name                                                   | long exchange records                         | short exchange records                       | Expected attempt-1 | Expected attempt-2 | Expected state after acceptance |
|---|--------------------------------------------------------|-----------------------------------------------|----------------------------------------------|--------------------|--------------------|-----|
| A | `TestTryReconcile_NonBinancePair_Complete`             | `[{Side:"long", CloseSize:100, PricePnL:-5, Fees:-0.1, Funding:0}]`  | `[{Side:"short", CloseSize:100, PricePnL:5, Fees:-0.1, Funding:0}]`  | `true`             | (not tested — already accepted) | HasReconciled=true, LongTotalFees/ShortTotalFees/LongClosePnL/ShortClosePnL populated, UpdateHistoryEntry called |
| B | `TestTryReconcile_NonBinancePair_LongSizeShort`        | `[{Side:"long", CloseSize:80, ...}]`          | `[{Side:"short", CloseSize:100, ...}]`       | `false`            | `false`            | fields unchanged (attempt-1 test is sufficient; attempt-2 asserted for belt-and-braces) |
| C | `TestTryReconcile_BinanceLong_Settled`                 | `[{Side:"", CloseSize:0, PricePnL:-57, Fees:-0.4, Funding:1.3}]`    | `[{Side:"short", CloseSize:100, ...}]`       | `false` (attempt gate) | `true`             | breakdown populated on attempt 2 — proves the fix unblocks PORTALUSDT-shape closes |
| D | `TestTryReconcile_BinanceLong_Unfinalized`             | `[{Side:"", CloseSize:0, PricePnL:0, Fees:0, Funding:0}]`           | `[{Side:"short", CloseSize:100, ...}]`       | `false` (empty-aggregate guard trips) | `false` (empty-aggregate guard still trips — Phase 1 runs before Phase 3 attempt gate, so this case never reaches attempt-gate path) | unchanged |
| E | `TestTryReconcile_BinanceShort_Settled`                | `[{Side:"long", CloseSize:100, ...}]`         | `[{Side:"", CloseSize:0, PricePnL:2, ...}]`  | `false` (attempt gate) | `true`             | short-leg Binance path works symmetrically |
| F | `TestTryReconcile_BinanceLong_PartialExceedsNotional`  | `[{Side:"", CloseSize:0, PricePnL:999999, Fees:-0.4, Funding:0}]`   | `[{Side:"short", CloseSize:100, PricePnL:-2, ...}]` | `false` (attempt gate) | `false` (notional guard trips) | unchanged |
| G | `TestTryReconcile_BinanceLong_PartialWithinNotional`   | `[{Side:"", CloseSize:0, PricePnL:-55, Fees:-0.4, Funding:1.2}]`    | `[{Side:"short", CloseSize:100, PricePnL:55, ...}]` | `false` (attempt gate) | `true`             | Tier 1 partial notional guard does not trip on attempt 2; breakdown populated |
| H | `TestTryReconcile_BinanceLong_CloseSizeMismatchOnShort`| `[{Side:"", CloseSize:0, PricePnL:-55, ...}]`                       | `[{Side:"short", CloseSize:80, ...}]`        | `false` (short-leg Phase 1 incomplete) | `false` (Phase 1 short incomplete still trips before Phase 3; attempt gate never reached) | unchanged |
| I | `TestTryReconcile_SiblingAware_BinanceLong`            | shared long exchange, `pos.LongCloseSize=100`, sibling `LongCloseSize=100`, longExpected=200, Binance records return `[{Side:"", CloseSize:0, PricePnL:-55, ...}]` | short has real size | `false` (attempt gate)                  | `true`             | proves the Binance leg's skip does not cause the sibling-sum logic to malfunction; `longExpected` is still computed as 200 but long-leg Tier 1 is skipped, so sibling sum matters only for the non-skipped leg's check |
| J | `TestTryReconcile_PreMigration_BinanceLong`            | same records as C but `pos.LongCloseSize=0, pos.ShortCloseSize=0` (pre-migration) | ... | `true` on attempt 1 (flows to existing Tier 2/3; `!useTier1` path unaffected by attempt gate) | (not tested) | proves `!useTier1` branch still works; this change does not affect pre-migration path |
| K | `TestTryReconcile_BinanceLong_AttemptGate` (v4)        | `[{Side:"", CloseSize:0, PricePnL:-55, Fees:-0.4, Funding:1.2}]` (settled)  | `[{Side:"short", CloseSize:100, ...}]` (settled) | `false`           | `true`             | dedicated attempt-gate regression — the gate is the reason C/E/G/I must not accept on attempt 1 |
| L | `TestTryReconcile_BinanceLong_CommissionOnlySnapshot_ResidualHazard` (v5) | `[{Side:"", CloseSize:0, PricePnL:0, Fees:-0.4, Funding:0}]` (mid-settlement: COMMISSION-only, persists across attempts)  | settled | `false` (attempt gate) | `true` if `|diff| <= max(close-notional)` (residual hazard path); test parameterizes the notional and diff so that the accept path is deterministic | **Pins the residual hazard explicitly.** The test documents the known design gap: empty-aggregate guard passes (Fees non-zero), attempt gate rejects attempt 1, attempt 2 accepts if `|diff|` fits notional. Asserts log lines: `Tier 1 partial path — deferring acceptance` at attempt 1 AND `Tier 1 partial accepted` at attempt 2. VPS ops can grep for the latter to monitor mid-settlement acceptances; if production shows this mode firing on actually-unsettled data, the follow-up stability-check mitigation (documented in Risk) is promoted. |
| M | `TestReconcilePnL_Wrapper_BinanceLong_RetryChainConverges` (v7)            | Binance-leg position, settled `/income` data returned at every attempt | settled                                            | (wrapper-level test; asserts end-state, not per-attempt return) | After `reconcilePnL(pos)` completes, history entry has per-leg breakdown populated and `HasReconciled=true`. Test patches the async retry delays (via internal `reconcilePnLDelays` override or similar) to 1ms so the test doesn't actually sleep 52s. Asserts `tryReconcilePnL` was invoked at least twice (attempt 1 rejected by gate, attempt 2 accepted). Proves the wrapper's retry chain works end-to-end, not just the inner function. |
| N | `TestReconcileRotationPnL_BinanceLeg_CompletesAfterRotation` (v7)          | Binance-leg closed rotated position with non-empty `RotationHistory` and nil `RotationRecord.PnL` on one entry | settled                                | (wrapper-level test; asserts end-state) | After `reconcileRotationPnL` completes, `RotationHistory[i].PnL` is set AND the subsequent `tryReconcilePnL(fresh, 2)` call (per v7 fix at exit.go:3246) succeeds and updates history breakdown. Asserts the attempt parameter passed is 2 (mock or spy on `tryReconcilePnL` call). Proves the v7 rotation fix unblocks Binance-leg rotated positions. |

Each integration test asserts:
1. `tryReconcilePnL` return value matches expectation at attempt 1 (always) and
   attempt 2 (for Binance-leg cases and anywhere the table specifies an
   attempt-2 expectation).
2. On `true` return: history entry has non-zero per-leg breakdown fields and
   `HasReconciled=true`.
3. On `false` return: history entry retains prior (empty) breakdown fields.
4. For cases C, E, G, I, K, L: after the attempt-1 call, `HasReconciled` must
   remain `false` (attempt gate or Phase 1 guard rejected) — proving the
   attempt-1 rejection does not leak partial state into history.

**Layer 2 — Inline logic tests (fast-path, for decision-matrix completeness)**

Keep a smaller `reconcile_tier1_per_leg_inline_test.go` that exercises the
decision math without DB scaffolding. Useful to catch off-by-one / boolean-
logic regressions without the full harness cost. Cases mirror the matrix in
the runtime:

| # | Decision                          | Input                                              | Expected |
|---|-----------------------------------|----------------------------------------------------|----------|
| 1 | `longTier1Enforced`               | `longAgg.CloseSize=100`                            | true     |
| 2 | `longTier1Enforced`               | `longAgg.CloseSize=0`                              | false    |
| 3 | `longIncomplete`                  | `longTier1Enforced=true, longAgg.CloseSize=80, longExpected=100` | true |
| 4 | `longIncomplete`                  | `longTier1Enforced=false, longAgg.CloseSize=0, longExpected=100` | false |
| 5 | `longUnsettled`                   | `longTier1Enforced=false, PricePnL=0, Fees=0, Funding=0`        | true |
| 6 | `longUnsettled`                   | `longTier1Enforced=false, PricePnL=-5, Fees=0, Funding=0`       | false |
| 7 | Partial fallback notional bound   | `diff=999, pos.LongEntry*pos.LongCloseSize=240, pos.ShortEntry*pos.ShortCloseSize=240` | retries |
| 8 | Partial fallback within bound     | `diff=1.5, notional=240`                                        | passes |

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

- **Blast radius**: small. Single function, ~45 net added lines in `exit.go`
  (Phase 1 per-leg + empty-aggregate guard, Phase 3 partial fallback), no
  schema change, no persistent-state migration. Two new test files.
- **Rollback**: revert commit. No persistent state touched.
- **Correctness risk (honest)**:
  - Pure non-Binance pairs: zero risk. Same code path as today.
  - Binance-leg pairs: Tier 1 protection on the Binance leg is replaced by
    (a) empty-aggregate guard (Phase 1) + (b) Tier 1 partial notional guard
    (Phase 3, `|diff| <= max(pos.LongEntry*pos.LongCloseSize,
    pos.ShortEntry*pos.ShortCloseSize)`). Combined, this is **weaker** than
    per-leg CloseSize match. A Binance-side corruption with non-zero
    aggregate values but wrong magnitudes that keeps the cross-leg diff
    within one leg's close-notional slips through. At typical sizes the
    notional bound is hundreds of USDT, so only gross corruption is caught.
    **Deliberate trade-off** — today's behavior is "retry all 4 attempts,
    fail, keep local PnL, breakdown never populated" for every Binance-leg
    position, strictly worse than v3's weaker-but-functional check.
  - **Retry-short-circuit hazard (addressed in v4)**: Codex flagged that a
    partial `/fapi/v1/income` snapshot with the `|diff|` bound satisfied
    could be accepted as terminal success, suppressing the 5s/15s/30s retries
    whose purpose is to wait for finalization (see `exit.go:1162-1186`:
    "exchange may not have finalized the position yet"). v4 addresses this
    with two complementary guards:
    1. **Empty-aggregate guard (Phase 1)**: rejects any `CloseSize==0` leg
       whose `PricePnL`, `Fees`, and `Funding` are all zero. Catches the
       pre-settlement case where `/income` has returned no records yet.
    2. **Attempt-count gate (Phase 3 partial fallback)**: rejects the Tier 1
       partial path on attempt 1 regardless of how complete the aggregate
       looks. Binance `GetClosePnL` makes three separate `/income` queries
       with no documented atomicity (`adapter.go:1128-1162`), so a
       commission-only snapshot (fees posted, PnL+funding still pending)
       would pass the empty-aggregate guard. The attempt gate forces at
       least one 5s retry delay (plus the 2s pre-attempt-1 sleep at
       `reconcilePnL:1105`) before acceptance, giving all three income
       types a chance to post.
    3. **Retry budget**: pre-sleep 2s + attempt 1 rejection + 5s +
       attempt 2 (now eligible) + 15s + attempt 3 + 30s + attempt 4 = 52s
       total before all attempts exhaust. Matches Binance's typical
       /income latency (observed <10s post-fill in practice; spec does not
       guarantee).
    4. **Residual hazard**: if at attempt 2 the snapshot is still partial
       (e.g., commission-only) AND `|diff|` fits within the notional bound,
       it will be accepted. The integration test matrix (cases K, L) pins
       the expected behavior and emits a labelled log so VPS observation
       can detect this mode if it occurs. A further tightening
       (stability check: require aggregate unchanged between attempts 1 and
       2) is available as a follow-up if the attempt gate proves
       insufficient under production load.
  - SIRENUSDT protection preservation: verified — the SIRENUSDT bug (H1:
    synthetic retrySecondLeg price + H2: reconcile guard + H3: SL/TP
    detection) fired on a non-Binance pair (SIRENUSDT was bybit↔bingx).
    Pure non-Binance pairs still hit full Tier 1 in v3.
- **Review-acknowledged weaknesses**:
  - When both legs are Binance-shape (CloseSize=0), Tier 1 partial notional
    guard is the only completeness check. The bot does not produce such
    pairs (see behavior matrix for Binance×Binance entry), so this is
    hypothetical.
  - `pos.LongCloseSize` is used as the notional basis. For pre-migration
    positions (`useTier1==false`), this branch does not fire — existing
    Tier 2 / Tier 3 runs unchanged. So this introduces no regression for
    positions that predate CloseSize tracking.

## Acceptance

- `go build ./...` passes.
- `go test ./internal/engine/...` passes (all existing tests, plus new
  `reconcile_tier1_per_leg_test.go`).
- After deploy, a Binance-leg close produces `UpdateHistoryEntry succeeded` in
  logs and the trade-history card shows `進場利差 / 已收資金費 / 多方價格 /
  空方價格 / 平倉原因` and the per-leg breakdown table (Fees / Funding /
  ClosePnL / Subtotal / TotalPnL / APR).
