# PLAN: Codex Review Fixes (5 findings) — v5

**Source**: Codex GPT-5.4 code review of commits 1cd87c5..HEAD
**Scope**: 1 CRITICAL + 1 HIGH + 2 MEDIUM + 1 LOW
**Review**: v5 → Fix 1 NR (caller drops error). v6: caller logs error. ALL fixes now address all review feedback.

---

## Fix 1 — CRITICAL: Consolidator force-close writes partial PnL as reconciled

**File**: `internal/engine/consolidate.go` L517-L562
**Also**: `internal/models/position.go` L72 (InferHasReconciled)

**Problem**: After 6 failed `GetClosePnL` retries, consolidator zeros the missing side, writes partial PnL to history/stats, sets `HasReconciled=true`. One bad exchange response permanently understates PnL.

**Codex v1 feedback**:
1. Just setting `HasReconciled=false` is unstable — `InferHasReconciled()` re-marks true on read
2. No automatic retry path for consolidator-closed positions

**Codex v2 feedback**:
1. Never clears `PartialReconcile` on successful reconciliation — position stays marked partial forever
2. If partial PnL crosses zero after reconcile, win/loss counts from `UpdateStats()` stay wrong

**Revised Fix (v3)**:
1. Add `PartialReconcile bool` field to `ArbitragePosition`
2. When max retries exceeded: set `HasReconciled = false`, `PartialReconcile = true`
3. `InferHasReconciled()` skips inference when `PartialReconcile == true`
4. After force-close, call `reconcilePnL()` async
5. **NEW v3**: `tryReconcilePnL` success paths (both no-diff L920 and update L932) must clear `PartialReconcile = false`
6. **NEW v3**: When reconcile succeeds on a partial-close, use `AdjustStats()` to correct win/loss counts if PnL sign changed (old partial PnL vs new reconciled PnL)

**Changes**:

`internal/models/position.go`:
```go
// Add field to ArbitragePosition struct:
PartialReconcile bool `json:"partial_reconcile,omitempty"` // true when closed with incomplete PnL data

// Update InferHasReconciled:
func (p *ArbitragePosition) InferHasReconciled() {
    if p.HasReconciled || p.PartialReconcile {
        return  // explicit partial marker blocks auto-inference
    }
    // ... rest unchanged
}
```

`internal/engine/consolidate.go` L517-L528, L562:
```go
// After max retries exceeded:
partialClose := false
if !longOK {
    longAgg = exchange.ClosePnL{}
    partialClose = true
}
if !shortOK {
    shortAgg = exchange.ClosePnL{}
    partialClose = true
}

// At L562 (conditional):
if partialClose {
    pos.HasReconciled = false
    pos.PartialReconcile = true
    e.log.Warn("consolidate: %s PARTIAL close (long=%v short=%v), queuing async reconcile",
        pos.ID, longOK, shortOK)
} else {
    pos.HasReconciled = true
}

// After saving position + history + stats, attempt async reconcile:
if partialClose {
    posCopy := *pos
    go e.reconcilePnL(&posCopy)
}
```

`internal/engine/exit.go` — tryReconcilePnL success paths:
```go
// L920-928 (no-diff branch): clear PartialReconcile
if !pos.HasReconciled {
    _ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
        fresh.HasReconciled = true
        fresh.PartialReconcile = false  // NEW v3: clear partial marker
        return true
    })
    // ... (Fix 3: UpdateHistoryEntry)
}

// L932+ (update branch): also clear PartialReconcile in the update callback
fresh.PartialReconcile = false  // NEW v3

// L966 (after AdjustPnL): correct win/loss if partial PnL sign changed
if pos.PartialReconcile {
    oldWon := pos.RealizedPnL > 0
    newWon := reconciledPnL > 0
    if oldWon != newWon {
        if err := e.db.AdjustWinLoss(oldWon, newWon); err != nil {
            e.log.Error("reconcile %s: AdjustWinLoss failed: %v", pos.ID, err)
        }
    }
}
```

`internal/database/state.go` — add `AdjustWinLoss(oldWon, newWon bool)`:
```go
// Swaps one win/loss count when reconciliation changes PnL sign.
// Uses actual Redis field names: "win_count" / "loss_count" (verified at state.go L406).
// Uses pipeline + Exec() to mirror UpdateStats pattern and surface errors.
func (c *Client) AdjustWinLoss(oldWon, newWon bool) error {
    if oldWon == newWon {
        return nil
    }
    ctx := context.Background()
    pipe := c.rdb.Pipeline()
    if oldWon && !newWon {
        pipe.HIncrBy(ctx, keyStats, "win_count", -1)
        pipe.HIncrBy(ctx, keyStats, "loss_count", 1)
    } else {
        pipe.HIncrBy(ctx, keyStats, "loss_count", -1)
        pipe.HIncrBy(ctx, keyStats, "win_count", 1)
    }
    _, err := pipe.Exec(ctx)
    return err
}
```

`web/src/types.ts` — add `partial_reconcile?: boolean` to Position type.

---

## Fix 2 — HIGH: Sequential rebalance passes deficits as needs

**File**: `internal/engine/engine.go` L848-L853
**Also**: `internal/engine/allocator.go` L1363-L1500

**Problem**: `crossDeficits` passed as `crossNeeds` to `executeRebalanceFundingPlan`. The function's L1377 `bal.futures >= need` check fires incorrectly because `need` is already a deficit, not original need.

**Codex v1 feedback**: Lower-half (L1496+) is correct cross-exchange logic. Problem is upper-half `bal.futures >= need` check.

**Codex v2 feedback**: Lower-half uses `needs[name]` at L1503/L1642/L1651 for donor surplus calculation. If only deficits are passed, donor exchanges that also need margin get over-drained. Need two separate inputs: full `needs` for donor-reservation + `crossDeficits` for recipient execution.

**Revised Fix (v3)**:
- Promote `deficit` struct from local type inside `executeRebalanceFundingPlan` to package-level type in allocator.go
- Add `precomputedDeficits []deficit` parameter to `executeRebalanceFundingPlan`
- When non-nil, skip the upper-half and use precomputed deficits directly
- Keep full `needs` map for donor surplus calculation in the lower-half
- Sequential rebalance converts `seqDeficit` → `deficit` before calling

**Codex v3 feedback**: `deficit` is defined inside the function, `seqDeficit` is used in engine.go. Types incompatible. Must share or convert.

**Changes**:

`internal/engine/allocator.go` — promote deficit type:
```go
// Move from inside executeRebalanceFundingPlan to package level:
type rebalanceDeficit struct {
    exchange string
    amount   float64
}

// Change signature — add optional precomputed deficits:
func (e *Engine) executeRebalanceFundingPlan(needs map[string]float64, balances map[string]rebalanceBalanceInfo, precomputedDeficits []rebalanceDeficit) {
    var crossDeficits []rebalanceDeficit

    if precomputedDeficits != nil {
        // Sequential rebalance: skip spot→futures phase, use precomputed deficits
        crossDeficits = precomputedDeficits
    } else {
        // Existing upper-half logic (L1370-L1494) — change local `deficit` refs to `rebalanceDeficit`
        for name, need := range needs {
            // ... existing spot→futures + deficit calculation ...
        }
    }

    // Lower-half (L1496+): cross-exchange transfers — unchanged
    // needs[name] still available for donor surplus calculation at L1503
    if len(crossDeficits) == 0 { ... }
    // ... donor selection, withdraw, deposit ...
}
```

`internal/engine/engine.go` L846-L853:
```go
// Convert seqDeficit → rebalanceDeficit explicitly:
precomputed := make([]rebalanceDeficit, len(crossDeficits))
for i, cd := range crossDeficits {
    precomputed[i] = rebalanceDeficit{exchange: cd.exchange, amount: cd.amount}
}
e.executeRebalanceFundingPlan(needs, balances, precomputed)
// 'needs' = original full needs map from rebalanceFunds() for donor surplus calc
```

Update all other call sites to pass `nil` for precomputedDeficits:
```go
e.executeRebalanceFundingPlan(needs, balances, nil)  // existing callers unchanged
```

---

## Fix 3 — MEDIUM: No-diff reconciliation doesn't update history entry

**File**: `internal/engine/exit.go` L920-L928

**Problem**: When reconciliation finds no diff, it sets `HasReconciled=true` on position but not on the history entry.

**Codex review feedback**: `AddToHistory()` uses `LPush` (append), NOT upsert. Must use `UpdateHistoryEntry()` instead.

**Revised Fix**:
```go
// exit.go L920-928:
if !needsPnLUpdate && !needsFundingUpdate && !needsExitUpdate && !needsBreakdownUpdate {
    if !pos.HasReconciled {
        _ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
            fresh.HasReconciled = true
            return true
        })
        // Update history entry too — use UpdateHistoryEntry (NOT AddToHistory which is LPush append)
        pos.HasReconciled = true
        if err := e.db.UpdateHistoryEntry(pos); err != nil {
            e.log.Warn("reconcile %s: failed to update history HasReconciled: %v", pos.ID, err)
        }
    }
    return true
}
```

---

## Fix 4 — MEDIUM: Aggregator test fixtures don't cover HasReconciled (PASS — unchanged)

**File**: `internal/analytics/aggregator_test.go`

**Fix**:
- Add `HasReconciled: true` to existing perp fixtures in `TestComputeStrategySummary`
- Add new test case: `HasReconciled: false` → verify `EntryFees` fallback
- Add new test case: `HasReconciled: true, ExitFees: 0` → verify zero-fee VIP uses 0

---

## Fix 5 — LOW: PnLBreakdown rotation_pnl not in hasDecomposition gate (PASS — unchanged)

**File**: `web/src/components/PnLBreakdown.tsx` L32-L36

**Fix**:
```tsx
const hasDecomposition =
    hasPerLeg ||
    (p.exit_fees != null && p.exit_fees !== 0) ||
    (p.basis_gain_loss != null && p.basis_gain_loss !== 0) ||
    (p.slippage != null && p.slippage !== 0) ||
    (p.rotation_pnl != null && p.rotation_pnl !== 0);  // ADD
```

---

## Implementation Order

1. Fix 1 (CRITICAL) — position.go + consolidate.go
2. Fix 2 (HIGH) — allocator.go + engine.go
3. Fix 3 (MEDIUM) — exit.go
4. Fix 4 (MEDIUM) — aggregator_test.go
5. Fix 5 (LOW) — PnLBreakdown.tsx

## Verification

- `go build ./...` must pass
- `go test ./internal/engine/... ./internal/analytics/...` must pass
- `npm run build` in web/ must pass
