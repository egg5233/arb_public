# Plan: Sequential Rebalance Fallback — Cross-Exchange Transfer Gap + Hardcoded Floor
Version: v3
Date: 2026-04-08
Status: APPROVED

## Changes

### Bug A (HIGH) — Sequential fallback never executes cross-exchange withdrawals
- `internal/engine/engine.go:544-842` (sequential rebalance path)
- `internal/engine/engine.go:836-841` (crossDeficits accumulated but abandoned)

### Bug B (MEDIUM) — Hardcoded 10.0 USDT floor instead of minWithdraw
- `internal/engine/allocator.go:586-588` (isAllocatorFundingFeasible)
- `internal/engine/allocator.go:1242-1248` (simulateTransferPlan)

## Why

**Bug A:** When `EnablePoolAllocator=false` (default, config.go:514), `rebalanceFunds()` takes the sequential path (engine.go:544). The path accumulates unmet cross-exchange deficits into `crossDeficits` (L810, L831), then logs "rebalance: complete" and returns at L841 without executing any Withdraw.

**Bug B:** `isAllocatorFundingFeasible()` at L586 and `simulateTransferPlan()` at L1242 both hardcode 10.0 USDT floor.

## How

### Bug A — Convert crossDeficits to needs map and call executeRebalanceFundingPlan (v3)

v2 used `needs :=` which shadows the existing `needs` variable at engine.go:562. v3 uses a different variable name.

**Verified:** `executeRebalanceFundingPlan` signature at allocator.go:1327:
```go
func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo)
```
Sequential path builds `balances` as `map[string]rebalanceBalanceInfo` at engine.go:498. Types match.

**engine.go ~L836:**
```go
if len(crossDeficits) == 0 {
    e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
    return
}

// Convert crossDeficits to needs format and delegate to the existing
// cross-exchange executor (same path the pool allocator uses).
crossNeeds := make(map[string]float64, len(crossDeficits))
for _, cd := range crossDeficits {
    crossNeeds[cd.name] = cd.amount
}
e.log.Info("rebalance: %d exchanges need cross-exchange funding, delegating to allocator executor", len(crossNeeds))
e.executeRebalanceFundingPlan(crossNeeds, balances)

e.log.Info("rebalance: complete")
```

Uses `crossNeeds` (not `needs`) to avoid shadowing the existing variable at L562.

### Bug B — Replace hardcoded 10.0 with donor-specific minWd (v3)

v2 had two errors: (1) wrong `findAllocatorDonor` call signature, (2) referenced non-existent `getWithdrawFeeWithCache` and `e.feeCache`.

**Verified signatures:**
- `findAllocatorDonor(recipient string, donorGross map[string]float64, balances map[string]rebalanceBalanceInfo) (string, float64, float64, bool)` — returns (donor, available, fee, ok)
- `findAllocatorDonor` internally calls `GetWithdrawFee("USDT", chain)` at L638, which returns `(fee, minWd, err)`. But `minWd` is NOT returned to the caller — only `(donor, available, fee, ok)`.
- There is no `e.feeCache` field. Fee cache is local to `findAllocatorDonorWithCache` (allocator.go ~L964).
- `getWithdrawFeeWithCache` does not exist.

**v3 approach for L586 (isAllocatorFundingFeasible):**

The cleanest fix is to have `findAllocatorDonor` also return `minWd`, or add a separate lookup. Since changing the return signature of `findAllocatorDonor` touches many call sites, the simpler approach: call `GetWithdrawFee` separately after finding the donor.

```go
for exchName, deficit := range deficits {
    remaining := deficit
    for remaining > 0 {
        donor, available, fee, ok := e.findAllocatorDonor(exchName, donorGross, balances)
        if !ok {
            return false
        }
        
        // Floor deficit to donor's minWithdraw (replaces hardcoded 10.0).
        // findAllocatorDonor already used GetWithdrawFee internally to get `fee`,
        // so we need the chain to query minWd. Use same chain selection logic.
        if remaining < 10.0 {
            // Only bother looking up minWd when we'd hit the old floor.
            e.cfg.RLock()
            addrs := e.cfg.ExchangeAddresses[exchName]
            e.cfg.RUnlock()
            var chain string
            for _, c := range []string{"APT", "BEP20"} {
                if addrs[c] != "" { chain = c; break }
            }
            if chain != "" {
                if _, minWd, err := e.exchanges[donor].GetWithdrawFee("USDT", chain); err == nil && remaining < minWd {
                    remaining = minWd
                }
            }
        }
        
        netPossible := available - fee
        if netPossible <= 0 {
            donorGross[donor] = 0
            continue
        }
        move := math.Min(remaining, netPossible)
        remaining -= move
        donorGross[donor] -= move + fee
    }
}
```

**Note:** This calls `GetWithdrawFee` a second time (it was already called inside `findAllocatorDonor`). Since the exchange adapter caches this internally and it only fires when `remaining < 10.0`, the overhead is negligible.

**allocator.go ~L1242 (simulateTransferPlan):**

Same approach — direct `GetWithdrawFee` call:
```go
move := math.Min(deficit, bestSurplus)

// Floor to donor's minWithdraw (replaces hardcoded 10.0).
e.cfg.RLock()
addrs := e.cfg.ExchangeAddresses[exch]
e.cfg.RUnlock()
var simChain string
for _, c := range []string{"APT", "BEP20"} {
    if addrs[c] != "" { simChain = c; break }
}
minWd := 10.0 // fallback
if simChain != "" {
    if _, mw, err := e.exchanges[bestDonor].GetWithdrawFee("USDT", simChain); err == nil {
        minWd = mw
    }
}
if move < minWd {
    if bestSurplus < minWd {
        e.log.Debug("allocator: simXfer %s→%s bestSurplus=%.2f < minWd=%.2f, infeasible", bestDonor, exch, bestSurplus, minWd)
        return false, 0
    }
    move = minWd
}
```

## Risk

- Bug A: `executeRebalanceFundingPlan` is battle-tested on the pool allocator path. `crossNeeds` maps exchange→deficit amount, same semantic as allocator needs. Safe reuse.
- Bug B: Extra `GetWithdrawFee` calls are cached by adapters and only fire for small deficits. Negligible overhead.
- Bug B: Chain selection logic duplicated from `findAllocatorDonor` (L617-624). Could extract to helper, but keeping inline for minimal diff.

## Graphify

Relevant nodes: rebalanceFunds (engine.go), executeRebalanceFundingPlan (allocator.go:1327), findAllocatorDonor (allocator.go:609), isAllocatorFundingFeasible (allocator.go), simulateTransferPlan (allocator.go), seqDeficit (engine.go), rebalanceBalanceInfo (allocator.go:38).

## Review History
- v1: Codex gpt-5.4 — NEEDS-REVISION — custom helpers wrong, floor removal unsafe, no need for toggle
- v2: Claude code-reviewer — NEEDS-REVISION — needs variable shadowing, findAllocatorDonor wrong signature, getWithdrawFeeWithCache doesn't exist
- v3: Claude code-reviewer — ALL PASS
