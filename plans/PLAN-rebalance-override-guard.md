# Plan: Rebalance Allocator Override Guard
Version: v4
Date: 2026-04-12
Status: ALL PASS

## Problem (verified by 3 independent Codex passes)

`rebalanceFunds()` at `internal/engine/engine.go:627-642` calls the void executor `executeRebalanceFundingPlan()` and then **unconditionally stores** `e.allocOverrides[c.symbol] = c` — regardless of whether any real cross-exchange transfer happened.

### Production incident (2026-04-12 03:45-03:55 UTC)
- :45 rebalance allocator greedy-selected 3 overrides: SIRENUSDT (bitget/bingx), PIPPINUSDT (okx/gateio), 4USDT (binance/bingx)
- Between :45:50 and :55:01: ZERO cross-exchange transfer log (no Withdraw / no cross-exchange TransferToSpot)
- bitget balance unchanged at :55 (still 50.14)
- :55 entry scan used tier-2-rebalance-overrides:
  - SIREN rejected: post-trade margin 0.94 > L4 threshold 0.80 on bitget
  - 4USDT dropped at allocator filter
  - Only PIPPIN executed (the one that didn't need cross-exchange funding)

### Root cause details
1. `executeRebalanceFundingPlan(needs, balances, nil)` has no return value (`allocator.go:1101`)
2. `dryRunResult.Steps` are generated but never consumed — executor recomputes from aggregated `needs` (`allocator.go:1081-1085`, dead state)
3. Caller at `engine.go:632-642` has no status check between executor call and override storage
4. `SimulateApproval` inflates balances with theoretical `TransferablePerExchange` (`manager.go:333-374`) — allocator feasibility is based on SIMULATED post-transfer balances, not real ones
5. Override cleanup: only on consumption (`engine.go:1008-1010`, `1256-1258`) — no failure-path cleanup

Plausible (not proven) execution path for this incident: `crossDeficits` emerged empty inside the executor and hit the early return at `allocator.go:1254-1256`, because the allocator simulation already included theoretical transferables. That would explain why no skip logs fired.

### Consumers of `allocOverrides` (audit)
Two consumers, both must still work correctly:
- `engine.go:1007-1011` — `applyAllocatorOverrides()` in normal entry scan path
- `engine.go:1256-1279` — fallback re-scan path (zero-opportunity override fallback)

### Callers of `executeRebalanceFundingPlan` (audit)
Two call sites, both must be updated for the signature change:
- `engine.go:632` — pool-allocator success branch (MAIN FIX POINT)
- `engine.go:995` — sequential-rebalance fallback, delegates to executor after its own deficit computation

## Changes

### 1. `internal/engine/allocator.go` — new result type

Add near top (after `dryRunResult` definition, ~line 66):
```go
// rebalanceExecutionResult reports the POST-EXECUTION state so callers can
// revalidate allocator choices against reality. PostBalances reflects actual
// funds after all successful transfers (not simulated).
type rebalanceExecutionResult struct {
    PostBalances map[string]rebalanceBalanceInfo // exchange -> actual balance after executor returns
    Unfunded     map[string]float64              // exchange -> deficit not covered (for diagnostics)
    SkipReasons  map[string]string               // exchange -> human-readable skip reason (diagnostic only)
}

// cloneRebalanceBalances returns a shallow copy of the map; rebalanceBalanceInfo
// is a value type so per-key writes on the copy are independent.
func cloneRebalanceBalances(src map[string]rebalanceBalanceInfo) map[string]rebalanceBalanceInfo {
    dst := make(map[string]rebalanceBalanceInfo, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}
```

### 2. `internal/engine/allocator.go:1101` — executor signature + population

- Change signature:
  ```go
  // Before
  func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) {

  // After
  func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) rebalanceExecutionResult {
  ```

- At function start: `result := rebalanceExecutionResult{PostBalances: cloneRebalanceBalances(balances), Unfunded: map[string]float64{}, SkipReasons: map[string]string{}}`

- Every time a `Withdraw` succeeds and subsequent `TransferToFutures` on the recipient completes: update `result.PostBalances[recipient]` with new futures balance (re-read from adapter or add the delta).

- Every time a `TransferToSpot` on the donor completes: update `result.PostBalances[donor]` similarly (futures decreased, spot increased).

- **CRITICAL**: Every time a local spot→futures relief succeeds in the executor's upper half at `allocator.go:1130-1156` and `allocator.go:1220-1245`, update `result.PostBalances[exchange]` with the new futures/spot values. Without this, `keepFundedChoices` will false-drop choices that became fundable via local relief alone — which is the dominant path in the 2026-04-12 incident where `crossDeficits` was empty at `:1254` early return.

- Every skip branch (11 gates listed in Codex verification pass 3): record `result.Unfunded[recipient] = remainingDeficit` and `result.SkipReasons[recipient] = "<branch message>"`.

- `allocator.go:1254-1256` early return:
  ```go
  if len(crossDeficits) == 0 {
      e.log.Info("rebalance: no cross-exchange transfers needed (crossDeficits empty after local fund relief)")
      return result
  }
  ```

- `allocator.go:1644` normal completion:
  ```go
  e.log.Info("rebalance: complete (unfunded=%d skipReasons=%d)", len(result.Unfunded), len(result.SkipReasons))
  return result
  ```

### 3. `internal/engine/engine.go:995` — sequential fallback caller

Sequential fallback doesn't use allocOverrides; it just needs the signature match:
```go
_ = e.executeRebalanceFundingPlan(needs, balances, precomputed)
```
Keep behavior unchanged for the sequential path. No filtering needed here because sequential path doesn't populate `allocOverrides`.

### 4. `internal/engine/engine.go:632-642` — pool-allocator: filter overrides via deterministic replay

Replace:
```go
e.executeRebalanceFundingPlan(allocSel.needs, balances, nil)

e.allocOverrides = make(map[string]allocatorChoice, len(allocSel.choices))
for _, c := range allocSel.choices {
    e.allocOverrides[c.symbol] = c
}
```
With:
```go
result := e.executeRebalanceFundingPlan(allocSel.needs, balances, nil)

kept := e.keepFundedChoices(allocSel.choices, result.PostBalances)

e.allocOverrideMu.Lock()
e.allocOverrides = kept
e.allocOverrideMu.Unlock()

dropped := len(allocSel.choices) - len(kept)
if dropped > 0 {
    e.log.Warn("rebalance: dropped %d/%d allocator overrides after executor outcome (remaining=%d)",
        dropped, len(allocSel.choices), len(kept))
    for _, c := range allocSel.choices {
        if _, ok := kept[c.symbol]; ok {
            continue
        }
        reasonLong := result.SkipReasons[c.longExchange]
        reasonShort := result.SkipReasons[c.shortExchange]
        e.log.Info("rebalance: dropped override %s (%s/%s): longReason=%q shortReason=%q",
            c.symbol, c.longExchange, c.shortExchange, reasonLong, reasonShort)
    }
}
e.log.Info("rebalance: stored %d allocator overrides for entry scan", len(kept))
```

### 5. New helper: `keepFundedChoices` + `canReserveAllocatorChoice` + `reserveAllocatorChoice`

Location: `internal/engine/allocator.go` (near other choice helpers). Delete earlier plan's `choiceNeedsTransfer` / `choiceRecipientsFunded` — replaced by deterministic replay.

```go
// keepFundedChoices replays allocator choices in order against the executor's
// post-execution balances, reserving capacity per kept choice. Choices that
// cannot be satisfied after real transfers are dropped. This mirrors the same
// per-leg budget math the allocator uses during selection, ensuring we don't
// keep overrides that rely on unfunded exchanges (partial or full).
func (e *Engine) keepFundedChoices(choices []allocatorChoice, post map[string]rebalanceBalanceInfo) map[string]allocatorChoice {
    work := cloneRebalanceBalances(post)
    kept := make(map[string]allocatorChoice, len(choices))
    for _, c := range choices {
        if !e.canReserveAllocatorChoice(work, c) {
            continue
        }
        e.reserveAllocatorChoice(work, c)
        kept[c.symbol] = c
    }
    return kept
}

// canReserveAllocatorChoice reports whether both legs of the choice still have
// enough futures capacity in the working balance map. Mirrors the allocator's
// own `needs` accounting in allocator.go:231-232 / 666-667 where requiredMargin
// is added to EACH exchange independently (it's max(long,short) buffered, not
// a total).
func (e *Engine) canReserveAllocatorChoice(work map[string]rebalanceBalanceInfo, c allocatorChoice) bool {
    long, okL := work[c.longExchange]
    short, okS := work[c.shortExchange]
    if !okL || !okS {
        return false
    }

    // Buffered sufficiency check: matches allocator's `needs` accounting.
    if long.futures < c.requiredMargin || short.futures < c.requiredMargin {
        return false
    }

    // Unbuffered projected-ratio check: matches dryRunTransferPlan's
    // post-trade ratio semantics so replay stays consistent with the
    // decision the allocator already made.
    margin := c.requiredMargin
    if e.cfg.MarginSafetyMultiplier > 0 {
        margin = c.requiredMargin / e.cfg.MarginSafetyMultiplier
    }
    return e.replayProjectedRatioOK(long, margin) && e.replayProjectedRatioOK(short, margin)
}

// postTradeMarginRatio is a NEW shared helper (extracted from the inline math
// currently embedded in dryRunTransferPlan at allocator.go:1052-1077). Both
// dryRunTransferPlan and replayProjectedRatioOK must call it for consistency.
// Returns ok=false only when the ratio breaches L4.
func (e *Engine) postTradeMarginRatio(bal rebalanceBalanceInfo, margin float64) (ratio float64, ok bool) {
    if bal.futuresTotal <= 0 {
        return 0, true
    }
    futures := bal.futures - margin
    if futures < 0 {
        futures = 0
    }
    ratio = 1 - futures/bal.futuresTotal
    return ratio, ratio < e.cfg.MarginL4Threshold
}

// replayProjectedRatioOK is a thin wrapper over postTradeMarginRatio used by
// keepFundedChoices during the post-execution replay.
func (e *Engine) replayProjectedRatioOK(bal rebalanceBalanceInfo, margin float64) bool {
    _, ok := e.postTradeMarginRatio(bal, margin)
    return ok
}

// dryRunTransferPlan refactor (allocator.go:1052-1077) — replace the inline
// ratio formula with a call to the new shared helper, keeping behavior
// identical:
//
//   for name := range involved {
//       bal := sim[name]
//       if bal.futuresTotal <= 0 {
//           continue
//       }
//       ratio, ok := e.postTradeMarginRatio(bal, 0)
//       postRatios[name] = ratio
//       if !ok {
//           e.log.Debug("dryRun: %s post-ratio=%.4f >= L4=%.4f, infeasible",
//               name, ratio, e.cfg.MarginL4Threshold)
//           return dryRunResult{Feasible: false}
//       }
//   }

// reserveAllocatorChoice subtracts requiredMargin from BOTH legs (not
// requiredMargin/2) to match allocator.go:231-232 / 666-667 semantics.
func (e *Engine) reserveAllocatorChoice(work map[string]rebalanceBalanceInfo, c allocatorChoice) {
    long := work[c.longExchange]
    short := work[c.shortExchange]
    long.futures -= c.requiredMargin
    short.futures -= c.requiredMargin
    work[c.longExchange] = long
    work[c.shortExchange] = short
}
```

**requiredMargin semantics (confirmed, not guessed)**: per `manager.go:718-728`, `approval.RequiredMargin = max(longMarginWithBuffer, shortMarginWithBuffer)`. The allocator treats it as a per-exchange buffered need at `allocator.go:231-232` and `:666-667` (`needs[longExchange] += requiredMargin; needs[shortExchange] += requiredMargin`). The v2 `perLeg := requiredMargin / 2` was WRONG and would under-reserve, causing kept overrides that should be dropped. v3 aligns with real allocator math: reserve `requiredMargin` on EACH leg.

**Exact-parity alternative**: if `allocatorChoice` can be extended with explicit `LongMarginNeeded` / `ShortMarginNeeded` fields sourced from `approval.LongMarginWithBuffer` / `ShortMarginWithBuffer`, the replay gains exact parity with `risk.Approve` instead of using the `max(…)` surrogate. This is a second-pass improvement — not required for v3 landing but noted for backlog.

### 6. Telemetry + tests

- New INFO log at executor completion showing `len(PostBalances)`, `len(Unfunded)`, per-exchange funded deltas.
- Unit tests (new `internal/engine/allocator_override_test.go`):
  1. Executor returns empty crossDeficits (all pre-funded) → `keepFundedChoices` keeps all choices based on pre-exec balances reflected in `PostBalances`.
  2. All cross-exchange transfers succeed → `keepFundedChoices` keeps all choices, working balance tracks reservations correctly.
  3. One recipient fails (simulated `SkipReasons[X] = "no donor found"`, `PostBalances[X]` unchanged from pre-exec state) → choices requiring X are dropped, others kept.
  4. Partial funding on shared recipient: two choices both need X, X received enough for one choice only. First choice reserves, second cannot → first kept, second dropped.
  5. Fallback re-scan path (engine.go:1256-1279) still processes overrides correctly with the new filtered map.
  6. **Local spot→futures relief only** (regression for v3): `crossDeficits` empty because local relief succeeded; `PostBalances` reflects post-relief state; override should be KEPT. This is the exact 2026-04-12 incident scenario inverted — if local relief had been enough, override should survive.
  7. requiredMargin semantics: reserving a choice with `requiredMargin=60` subtracts 60 from BOTH legs (not 30).

## Why

Three independent Codex passes converged on this diagnosis, and a review pass explicitly flagged problems with the v1 `Funded[exchange] > 0` filter:
- Allocator plans use simulated post-transfer balances (`TransferablePerExchange`)
- Executor receives aggregated `needs` and may early-return without any cross-exchange transfer
- Caller stores symbol-granular overrides after a void call
- Next :55 entry scan consumes stale overrides → forces trades on unfunded exchanges → L4 rejection / margin violations
- `Funded` presence check is too coarse for multi-choice shared recipients or partial funding

## Risk

- **`requiredMargin` semantics**: CONFIRMED per `allocator.go:231-232`, `:666-667`, and `manager.go:718-728` — the allocator adds `requiredMargin` to each exchange's `needs` independently (it's `max(longBuffered, shortBuffered)`, not a total). Replay reserves the full amount on both legs. No further confirmation needed before landing.
- **Post-execution balance freshness**: executor must re-fetch balances after each successful `TransferToFutures` to reflect reality. If it just adds deltas, drift from fees can accumulate.
- **False drops from split-account exchanges**: unified vs isolated margin accounts may report balances differently. The cloned `rebalanceBalanceInfo` must carry whatever the allocator itself uses for its feasibility check; do not substitute a different source.
- **Fallback re-scan path**: `engine.go:1256-1279` reads the same `allocOverrides` map. Filtering at storage time means the fallback only sees kept choices — desired behavior.
- **`SkipReasons` is diagnostic only**: not the source of truth for retention. A choice might be kept even if SkipReasons records an issue, because the working balance still has enough from other transfers or unchanged exchanges.
- **No change to `SimulateApproval`**: the theoretical transferable inflation stays in place for the allocator's forward-looking feasibility — we only gate overrides on reality afterwards.
- **No change to `dryRunTransferPlan.Steps`**: dead field remains but not harmful.
- **Concurrent rebalance**: `run()` is single-goroutine (`engine.go:1151-1285`) — no race. Still guard `allocOverrides` writes with `allocOverrideMu` (already does; keep).

## Review History
- v1: Codex (gpt-5.4 high) — NEEDS-REVISION — `Funded[]>0` filter too coarse, `c.neededExchange` undefined (compile error), missed 2nd caller at engine.go:995, missed fallback re-scan consumer, helpers don't mirror executor's local-relief math
- v2: Codex (gpt-5.4 high, resumed thread) — NEEDS-REVISION — v2 addressed all v1 findings, but 2 substantive gaps remained: (a) `perLeg = requiredMargin / 2` math was wrong — allocator uses `requiredMargin` as per-exchange buffered need (max(long,short) not total); (b) `PostBalances` plan only updated on cross-exchange moves — missed local spot→futures relief at allocator.go:1130-1156 / 1220-1245, which is the exact path in the 2026-04-12 incident
- v3: Codex (gpt-5.4 high, resumed thread) — NEEDS-REVISION — all v1/v2 findings addressed, but 1 blocker remained: `replayProjectedRatioOK` still a placeholder referencing nonexistent fields (`futuresUsed`, `riskL4Threshold`); no shared post-trade ratio helper exists today in allocator.go:1052-1077, must be factored out. Also stale "must confirm requiredMargin" note in Risk section contradicted v3 body.
- v4: Codex (gpt-5.4 high, resumed thread) — **ALL PASS** — shared postTradeMarginRatio helper matches real fields, replayProjectedRatioOK no longer placeholder, local relief accounted for in PostBalances, requiredMargin semantics confirmed aligned with allocator.go:231/manager.go:718, both callers and both consumers covered. No remaining gaps.
