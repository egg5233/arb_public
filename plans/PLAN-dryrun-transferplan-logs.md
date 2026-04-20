# Plan: dryRunTransferPlan — Add Per-Recipient Debug Logs

Version: v2
Date: 2026-04-20
Status: DRAFT

## Review History
- v1 (2026-04-20): Codex review found that log snippets used variable names and line anchors that don't match current source. v2 corrects all 5 log points to use the real locals (`deficit` not `transferNeed`, `bestDonor`/`move` not `donor`/`moveAmount`, etc.) and places infeasible/exit logs at the correct control-flow positions (before existing early returns, not after loop exits).

## Problem

`dryRunTransferPlan` at `internal/engine/allocator.go:931+` decides rebalance feasibility but its current debug log output is insufficient to diagnose why a choice was pruned. Current logs show:

- `dryRun: donor <name> futures→spot <amt> (need=<amt>)` — this is the futures→spot PREP step only
- `dryRun: donor <name> no available after prep+fee (avail=<n> fee=<n>)` — donor depleted
- `dryRun: no donor for <recipient> deficit=<n>` — final infeasibility

What's missing:

- The exact **recipient-by-recipient** flow (which recipient was processed first, what its starting deficit was)
- The **donor-to-recipient net transfer amount** after min-withdraw/fee adjustments (the "futures→spot" log is the prep step, not the actual net amount arriving at the recipient)
- The **donor budget before/after** each move, so one can see where capacity went
- The **per-recipient deficit closeout** — did this recipient end funded, or with residual deficit?

Concrete example: in the 11:45 UTC 2026-04-20 VPS rebalance Iteration 3 (selecting RAVEUSDT + ASTEROIDUSDT), the log shows bitget donating `111.93` and binance donating `89.43`, total `204.50`. But the final message was `no donor for gateio deficit=52.03`. Reading the log it's not possible to tell which recipient consumed Bitget's 111.93, whether there was min-withdraw overfunding, whether ratioDeficit inflated Gate.io's true need, etc. Codex investigation today traced this to alphabetical recipient order (bingx before gateio) + greedy donor selection (bitget → bingx first), but the log alone cannot prove that flow.

This plan adds diagnostic logs so future cases can be diagnosed from log alone, without re-running source inspection.

## Scope

Pure log additions to `internal/engine/allocator.go` inside `dryRunTransferPlan`. NO logic changes, NO algorithm changes, NO test additions. Existing behavior unchanged.

## Changes

### `internal/engine/allocator.go` — `dryRunTransferPlan` (around `:931-1340`)

Add the following `e.log.Debug(...)` calls at the marked points. All are `Debug` level (not `Info`) to avoid polluting normal-op logs; set `DEBUG=true` env var to enable.

1. **After `needs` map is built (`:943`)** — insert the log AFTER line 943 (end of the per-choice needs aggregation loop), before sortedRecipients construction:
   ```go
   e.log.Debug("dryRun: start — choices=%d recipients=%d needs=%v",
       len(choices), len(needs), needs)
   ```

2. **Per-recipient starting state** — insert AFTER `:1061` (end of the `deficit := max(marginDeficit, ratioDeficit)` line) and BEFORE `:1065` (start of donor loop). The real local is named `deficit` (not `transferNeed`):
   ```go
   e.log.Debug("dryRun: recipient %s starts: need=%.4f futures=%.4f futuresTotal=%.4f marginDeficit=%.4f ratioDeficit=%.4f transferNeed=%.4f",
       exch, need, sim[exch].futures, sim[exch].futuresTotal,
       marginDeficit, ratioDeficit, deficit)
   ```
   Also ADD two logging-only counters at the top of this recipient iteration (before line 1065):
   ```go
   moveCount := 0
   totalNetForRecipient := 0.0
   ```

3. **Per donor→recipient move applied** — insert AFTER `:1322` (the `donorBudget[bestDonor] -= ...` line). The real locals are `bestDonor`, `exch`, `move`, `fee`, `deficit`. Compute `donorDebit` and `netCredit` inline from those:
   ```go
   moveCount++
   totalNetForRecipient += move
   donorDebit := move + fee
   netCredit := move
   e.log.Debug("dryRun: moved %.4f from %s to %s (gross=%.4f fee=%.4f netCredit=%.4f donorBudgetAfter=%.4f recipientDeficitAfter=%.4f)",
       move, bestDonor, exch, donorDebit, fee, netCredit, donorBudget[bestDonor], deficit)
   ```

4. **Per-recipient closeout** — TWO log points:
   - **Infeasible closeout**: insert BEFORE the existing `return dryRunResult{...}` early-return at `:1084-1085` (the "no donor" path). Can't wait until "after loop exits" because the function returns immediately:
     ```go
     e.log.Debug("dryRun: recipient %s INFEASIBLE — residualDeficit=%.4f after %d donor moves, totalNet=%.4f",
         exch, deficit, moveCount, totalNetForRecipient)
     return dryRunResult{Feasible: false, ...existing fields...}
     ```
   - **Funded closeout**: insert AFTER the inner donor while-loop exits normally (deficit reached ≤ 0) and BEFORE line 1324 (or wherever the next recipient iteration begins). Real condition is `deficit <= 0`:
     ```go
     e.log.Debug("dryRun: recipient %s funded — consumed %d donor moves, totalNet=%.4f",
         exch, moveCount, totalNetForRecipient)
     ```

5. **Function exit** — there is NO `result` local in current code; the function returns `dryRunResult{...}` literals directly at `:1365-1370` (success) and at each early-return point. For the SUCCESS path, insert BEFORE line 1365:
   ```go
   e.log.Debug("dryRun: end — feasible=true steps=%d totalFee=%.4f",
       len(steps), totalFee)
   ```
   For EARLY-RETURN paths (at `:1084-1085`, `:1361` per review, plus any other `return dryRunResult{Feasible: false, ...}` site), insert BEFORE each:
   ```go
   e.log.Debug("dryRun: end — feasible=false reason=%s",
       <short reason string for this specific failure site>)
   ```
   Use short reason strings like `"no_donor_for_recipient"`, `"post_ratio_infeasible"`, etc. matching the existing infeasibility log message at that site.

**Implementer guidance**: before editing, `grep -n "dryRunResult{" internal/engine/allocator.go` to enumerate every return site in `dryRunTransferPlan`. Every infeasible return gets an end-log line. The success path (the last `return` before function close) gets the `feasible=true` summary.

Exact line numbers above are from `efd6006d`; implementer re-verifies against current HEAD before editing. The intent of each log is stated; formatting flexible but must include all named fields.

## Why Debug, Not Info

- Rebalance runs every 60 minutes (rotateScan at :45). Per-iteration logs per-recipient per-donor could produce ~20-50 extra lines per rebalance → ~500-1200 lines/day at Info, polluting `logs/arb.log`.
- Diagnostic use is rare; enable `DEBUG=true` only when investigating a specific rebalance decision.
- Consistent with existing `dryRun:` log lines which are already `[DEBUG]` level.

## Files to Change

| File | Change |
|---|---|
| `internal/engine/allocator.go` | Add 5 `e.log.Debug(...)` calls inside `dryRunTransferPlan` at the 5 points above. No other changes. |

## Risks

| Risk | Mitigation |
|---|---|
| Log lines leak into Info by mistake | Use `e.log.Debug(...)` explicitly. |
| Local-variable names in the log format string don't exist at the log point | Implementer adapts to real variable names in scope; intent of each log is stated in the snippet. |
| Performance cost of formatting long log strings when DEBUG=false | `e.log.Debug(...)` short-circuits when debug disabled (verify `pkg/utils/logging.go` Logger.Debug early-return). If it doesn't, wrap each call in `if e.cfg.Debug { ... }`. |

## Tests

None required for this plan (pure logging). Existing `dryRunTransferPlan` tests (if any) remain valid.

## Rollout

- Version: next sequence after v0.32.33 (= v0.32.34).
- CHANGELOG: "debug: add per-recipient/per-donor trace logs inside dryRunTransferPlan for diagnosing rank-first Tier-1 prune decisions".
- Rollback: single-commit revert removes the logs.

## Out of Scope

- Actual algorithm improvements (recipient ordering, min-cost flow matching, feasible-subset prune) — separate follow-up plan.
- Any logic change to dryRunTransferPlan.
- Upgrade from Debug to Info.
