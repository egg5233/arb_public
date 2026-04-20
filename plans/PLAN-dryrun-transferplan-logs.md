# Plan: dryRunTransferPlan — Add Per-Recipient Debug Logs

Version: v1
Date: 2026-04-20
Status: DRAFT

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

1. **At function entry, after computing `needs` map** (around `:943`, after the needs aggregation loop):
   ```go
   e.log.Debug("dryRun: start — choices=%d recipients=%d needs=%v",
       len(choices), len(needs), needs)
   ```
   Shows the raw `needs` map (symmetric `requiredMargin` per exchange) at entry.

2. **At start of Pass 2 recipient loop, before per-recipient processing** (around `:1035-1040`, the main cross-exchange transfer loop):
   For each recipient iteration, log starting state:
   ```go
   // inside the recipient for-loop, at the top
   e.log.Debug("dryRun: recipient %s starts: need=%.4f futures=%.4f futuresTotal=%.4f marginDeficit=%.4f ratioDeficit=%.4f transferNeed=%.4f",
       exch, need, sim[exch].futures, sim[exch].futuresTotal,
       marginDeficit, ratioDeficit, transferNeed)
   ```
   Where `transferNeed = max(marginDeficit, ratioDeficit)`. This exposes the post-trade-ratio headroom calculation that can inflate actual deficit vs raw `marginDeficit`.

3. **After each donor-to-recipient move is applied** (around `:1320-1322`, where donor budget is decremented):
   ```go
   e.log.Debug("dryRun: moved %.4f from %s to %s (fee=%.4f netCredit=%.4f donorBudgetAfter=%.4f recipientDeficitAfter=%.4f)",
       moveAmount, donor, exch, fee, netCredit, donorBudget[donor], remainingDeficit)
   ```
   Where `moveAmount` is the gross amount transferred (before fee), `netCredit` is what arrives at recipient, `remainingDeficit` is recipient's remaining need. Makes the donor→recipient routing explicit.

4. **When a recipient closes out (either funded or infeasible)** (inside the same loop, after the donor loop exits per recipient):
   ```go
   if recipientDeficit <= 0 {
       e.log.Debug("dryRun: recipient %s funded — consumed %d donor moves, totalNet=%.4f",
           exch, moveCount, totalNetForRecipient)
   } else {
       e.log.Debug("dryRun: recipient %s INFEASIBLE — residualDeficit=%.4f after %d donor moves",
           exch, recipientDeficit, moveCount)
   }
   ```
   Makes it explicit whether each recipient ended funded or not, without having to infer from absence of the existing "no donor for X" message.

5. **At function exit, before return** (around `:1335+`):
   ```go
   e.log.Debug("dryRun: end — feasible=%v steps=%d totalFee=%.4f",
       result.Feasible, len(result.Steps), totalFee)
   ```
   Summary one-liner.

The implementer may use slightly different local-variable names depending on what's already in scope at each log point. The intent is to capture the listed fields; exact formatting is flexible but must include all named fields.

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
