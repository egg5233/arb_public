# Plan: Pool Allocator Executor Honors Planned Steps
Version: v3
Date: 2026-04-18
Status: REVIEWING

## Incident

2026-04-18 14:45 UTC on v0.32.25. Pool allocator selected SIGNUSDT gateio/bingx. `simulation passed`, gateio→bingx 132.55 USDT executed, override dropped (kept=0), entry tier-3 rejected. 132.55 stranded.

## Root cause (Codex-verified, corrected from v1)

**`dryRunTransferPlan` models donor-drain correctly** — it DOES subtract `move + fee` from donor's sim balance before L4 check (allocator.go:1206-1221). Not a projection bug.

**Real bug**: sim produces `Steps` (donor→recipient assignments) but caller throws them away. Executor re-runs greedy donor selection with only `needs` (allocator.go:1460-1507), possibly picking different donors than sim validated as feasible.

In the 14:45 case: sim likely validated a plan using different donors (or different amount distribution). Executor re-allocated and drained gateio, breaking gateio's own-leg feasibility.

Same bug class as v0.32.24 Bug B (sequential planner/executor mismatch), just in pool allocator path.

## Change (Option B — executor honors Steps)

### 0. `internal/engine/allocator.go:~68` — extend `transferStep` with `MinWithdraw` (v3 addition)

The executor's late safety check at `allocator.go:1733, 1749` uses `minWd` to guard against live-capped amounts dropping below exchange minimum. Planned branch must carry this value from sim.

BEFORE:
```go
type transferStep struct {
    From, To string
    Amount   float64
    Fee      float64
    Chain    string
}
```

AFTER:
```go
type transferStep struct {
    From        string
    To          string
    Amount      float64
    Fee         float64
    Chain       string
    MinWithdraw float64
}
```

And when `steps` is populated in `dryRunTransferPlan` (~`allocator.go:1221`):

BEFORE:
```go
steps = append(steps, transferStep{From: bestDonor, To: exch, Amount: move, Fee: fee, Chain: chain})
```

AFTER:
```go
steps = append(steps, transferStep{
    From:        bestDonor,
    To:          exch,
    Amount:      move,
    Fee:         fee,
    Chain:       chain,
    MinWithdraw: minWd,
})
```

### 1. `internal/engine/allocator.go:~255` — extend `allocatorSelection`

BEFORE:
```go
type allocatorSelection struct {
    choices        []allocatorChoice
    needs          map[string]float64
    totalBaseValue float64
    feasible       bool
}
```

AFTER:
```go
type allocatorSelection struct {
    choices        []allocatorChoice
    needs          map[string]float64
    totalBaseValue float64
    feasible       bool
    plan           dryRunResult
}
```

`plan` carries the `Steps []transferStep` from simulation.

### 2. Populate `plan` after dry-run passes in `runPoolAllocator`

Locate the place where `simulation passed` log fires and `allocSel` is constructed. Assign `allocSel.plan = dryRunResult` before returning.

### 3. `internal/engine/engine.go:633` — pass planned steps

BEFORE:
```go
result := e.executeRebalanceFundingPlan(allocSel.needs, balances, nil)
```

AFTER:
```go
result := e.executeRebalanceFundingPlan(allocSel.needs, balances, nil, allocSel.plan.Steps)
```

### 4. `internal/engine/allocator.go` — extend `executeRebalanceFundingPlan` signature

BEFORE:
```go
func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) rebalanceExecutionResult {
```

AFTER:
```go
func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit, plannedSteps []transferStep) rebalanceExecutionResult {
```

### 5. Sequential fallback caller update (Bug C context)

Sequential path also calls `executeRebalanceFundingPlan`. Pass `nil` for plannedSteps (sequential has its own dry-run prune per v0.32.24 Bug B; greedy-with-steps binding is pool-only for now).

Existing sequential call site at `engine.go:~1020` (per v0.32.24 Bug C):
```go
result := e.executeRebalanceFundingPlan(needs, balances, precomputed)
```

Update to:
```go
result := e.executeRebalanceFundingPlan(needs, balances, precomputed, nil)
```

### 6. Executor loop — honor plannedSteps when present

At `internal/engine/allocator.go:1460-1507` (the outer `for i := range crossDeficits` loop + inner greedy donor selection `for remaining > 0`):

BEFORE:
```go
surplus := map[string]float64{}
for name := range e.exchanges {
    surplus[name] = e.allocatorDonorGrossCapacity(name, balances[name], needs[name])
}
...
for i := range crossDeficits {
    remaining := crossDeficits[i].amount
    exchName := crossDeficits[i].exchange
    origDeficit := remaining
    for remaining > 0 {
        var bestDonor string
        var bestSurplus float64
        for name, s := range surplus {
            if name == exchName || s <= 0 {
                continue
            }
            if balances[name].marginRatio >= e.cfg.MarginL4Threshold {
                continue
            }
            if s > bestSurplus || (s == bestSurplus && (bestDonor == "" || name < bestDonor)) {
                bestDonor = name
                bestSurplus = s
            }
        }
```

AFTER:
```go
surplus := map[string]float64{}
for name := range e.exchanges {
    surplus[name] = e.allocatorDonorGrossCapacity(name, balances[name], needs[name])
}

plannedByRecipient := map[string][]transferStep{}
for _, step := range plannedSteps {
    plannedByRecipient[step.To] = append(plannedByRecipient[step.To], step)
}

for i := range crossDeficits {
    remaining := crossDeficits[i].amount
    exchName := crossDeficits[i].exchange
    origDeficit := remaining
    planned := plannedByRecipient[exchName]
    plannedIdx := 0

    for remaining > 0 {
        var bestDonor, chain string
        var bestSurplus, fee, minWd, contribution float64

        if len(plannedSteps) > 0 {
            if plannedIdx >= len(planned) {
                result.SkipReasons[exchName] = fmt.Sprintf("planned steps exhausted (remaining=%.2f)", remaining)
                break
            }
            step := planned[plannedIdx]
            plannedIdx++

            if step.To != exchName {
                result.SkipReasons[exchName] = fmt.Sprintf("planned step routed to %s, expected %s", step.To, exchName)
                break
            }

            bestDonor = step.From
            bestSurplus = surplus[bestDonor]
            fee = step.Fee
            minWd = step.MinWithdraw
            contribution = step.Amount
            chain = step.Chain

            if contribution > remaining {
                contribution = remaining
            }
            if contribution > bestSurplus {
                contribution = bestSurplus
            }
        } else {
            // Existing greedy donor-selection block (fallback when no plan)
            for name, s := range surplus {
                if name == exchName || s <= 0 {
                    continue
                }
                if balances[name].marginRatio >= e.cfg.MarginL4Threshold {
                    continue
                }
                if s > bestSurplus || (s == bestSurplus && (bestDonor == "" || name < bestDonor)) {
                    bestDonor = name
                    bestSurplus = s
                }
            }
            // ... existing GetWithdrawFee / contribution computation follows
        }

        // Remaining logic unchanged: recipient-address lookup, withdraw prep, batching,
        // live-balance caps, PostBalances mutation.
        // (chain/fee/minWd/contribution now set by either planned or greedy branch.)
```

The branch-and-common-tail structure preserves existing recipient address resolution, withdraw API call, PostBalances mutation.

## Tests

1. `internal/engine/allocator_override_test.go::TestExecuteRebalanceFundingPlan_HonorsPlannedSteps` — verify executor dispatches via `plannedSteps[*].From` (not greedy re-pick).
2. Dry-run unit test proving `Steps` non-empty with `From/To/Amount/Fee/Chain` populated after `simulation passed`.
3. Incident-style regression: gateio would be simulated as donor for bingx, but planned amount=~80 (less than observed 132); executor honors planned amount; `keepFundedChoices` retains override for SIGNUSDT.

## Risk

- Signature change `executeRebalanceFundingPlan` → 2 callers (pool + sequential). Both updated.
- Greedy fallback preserved when `plannedSteps == nil` → sequential path unchanged behaviorally.
- `plan dryRunResult` on `allocatorSelection` — ensure other consumers of `allocatorSelection` don't break; likely only `runPoolAllocator` return + `rebalanceFunds` consumer.
- Empty/partial plan edge: if `plannedByRecipient[recipient]` is empty but `crossDeficits` has the recipient, executor logs `SkipReasons` and breaks — same behavior as greedy "no donor found".

## Version
Bump to `v0.32.26`.

## Review History
- v1: Codex NEEDS-REVISION — my root cause was wrong (L4 projection already models donor-drain). Real bug: sim's Steps discarded, executor re-greedy. Chose Option B; provided concrete BEFORE/AFTER for signature + executor loop.
- v2: Codex NEEDS-REVISION — `minWd = 0` in planned branch breaks executor's late min-withdraw safety check at allocator.go:1733, 1749.
- v3: REVIEWING — added section 0: extend `transferStep` with `MinWithdraw` field, populate in dryRunTransferPlan, carry into planned branch. Planned branch now uses `step.MinWithdraw` instead of zero.
