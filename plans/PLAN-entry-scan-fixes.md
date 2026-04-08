# PLAN: :35 EntryScan Flow Fixes — v3

**Source**: Codex GPT-5.4 full flow review of EntryScan path
**Scope**: 1 CRITICAL + 4 HIGH + 1 MEDIUM + 1 LOW
**Review**: v1→v5 iterations. v5→C PASS. v6→A NR (stale partial row after flat). v7: explicit close after rollback.

---

## Fix A — CRITICAL: Trim-back failure still activates position with unmatched exposure

**File**: `internal/engine/engine.go` L2178-2187 (caller), L3490-3541 (trim + activate)

**Problem**: Post-loop trim failure only warns, then activates with `minFill`. Unmatched exposure is live and untracked.

**v1 feedback**: Returning error triggers `cleanupFailedPosition` which skips non-zero partials.
**v2 feedback**: `reservation` not in scope inside `executeTradeV2WithPos`; returning nil causes double-commit; SavePosition error ignored.

**v3 feedback**: Manual open caller (L348) also needs sentinel handling. Partial save debounce (10% threshold) means small fills may have no StatusPartial row.

**v4 Fix — Sentinel error pattern + force checkpoint + both callers**:

Step 1: Define sentinel error in engine.go:
```go
var errPartialEntry = errors.New("partial entry: consolidator will reconcile")
```

**v4 feedback**: Force checkpoint save can fail too. If DB has no fill state, sentinel is unsafe.

Step 2: In `executeTradeV2WithPos`, BEFORE post-loop trim (L3490), force-save with retry. If save fails after retries, close both legs and return normal error (not sentinel) — going flat is the only safe fallback when DB is unreachable.
```go
// Force checkpoint before trim — ensures DB has current fill state
pos.LongSize = confirmedLong
pos.ShortSize = confirmedShort
pos.LongEntry = longVWAP
pos.ShortEntry = shortVWAP
pos.EntryNotional = math.Max(longVWAP*confirmedLong, shortVWAP*confirmedShort)
pos.Status = models.StatusPartial
pos.UpdatedAt = time.Now().UTC()

var ckErr error
for attempt := 1; attempt <= 3; attempt++ {
    ckErr = e.db.SavePosition(pos)
    if ckErr == nil {
        break
    }
    e.log.Error("force checkpoint %s attempt %d/3: %v", posID, attempt, ckErr)
    time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
}
if ckErr != nil {
    // DB unreachable — close both legs to go flat.
    e.log.Error("CRITICAL: %s checkpoint failed, closing both legs to go flat", posID)
    var longRem, shortRem float64
    if confirmedLong > 0 {
        longRem = e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, confirmedLong)
    }
    if confirmedShort > 0 {
        shortRem = e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, confirmedShort)
    }
    if longRem > 0 || shortRem > 0 {
        // Could not fully flatten AND DB is unreachable — worst case.
        e.log.Error("ORPHAN EXPOSURE: %s checkpoint failed + close incomplete (longRem=%.6f shortRem=%.6f) — MANUAL INTERVENTION NEEDED", posID, longRem, shortRem)
        pos.FailureReason = "checkpoint_failed_orphan"
        _ = e.db.SavePosition(pos)
        return errPartialEntry // sentinel keeps capital committed, avoids cleanup
    }
    // Fully flattened on exchange. But DB may have a stale non-zero StatusPartial
    // row from debounced saves (L3452). cleanupFailedPosition skips non-zero rows.
    // Explicitly close the position here instead of relying on caller cleanup.
    pos.LongSize = 0
    pos.ShortSize = 0
    pos.Status = models.StatusClosed
    pos.FailureReason = "checkpoint_failed_rollback"
    pos.ExitReason = "entry_failed: DB unreachable, rolled back to flat"
    pos.UpdatedAt = time.Now().UTC()
    _ = e.db.SavePosition(pos)   // best-effort: DB may still be down
    _ = e.db.AddToHistory(pos)
    e.api.BroadcastPositionUpdate(pos)
    e.log.Info("checkpoint failed but exchange flat — position %s closed", posID)
    return fmt.Errorf("checkpoint save failed, rolled back to flat: %w", ckErr)
}
```

Step 3: Post-loop trim failure returns sentinel:
```go
trimFailed := false
if confirmedLong > minFill {
    excess := confirmedLong - minFill
    rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, excess)
    if rem > 0 {
        e.log.Error("TRIM FAILED: %s long excess %.6f on %s", opp.Symbol, rem, opp.LongExchange)
        trimFailed = true
    }
}
if confirmedShort > minFill {
    excess := confirmedShort - minFill
    rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, excess)
    if rem > 0 {
        e.log.Error("TRIM FAILED: %s short excess %.6f on %s", opp.Symbol, rem, opp.ShortExchange)
        trimFailed = true
    }
}

if trimFailed {
    pos.EntryNotional = math.Max(pos.LongEntry*pos.LongSize, pos.ShortEntry*pos.ShortSize)
    pos.FailureReason = "trim_incomplete"
    pos.FailureStage = "post_fill_trim"
    pos.UpdatedAt = time.Now().UTC()
    _ = e.db.SavePosition(pos)  // update with failure metadata
    e.api.BroadcastPositionUpdate(pos)
    return errPartialEntry
}
```

Step 4: Update BOTH callers to handle sentinel:

Batch caller (L2178-2187):
```go
go func(c candidate) {
    defer wg.Done()
    defer c.lock.Release()
    
    err := e.executeTradeV2WithPos(c.opp, c.pos, c.size, c.price, c.gapBPS)
    if errors.Is(err, errPartialEntry) {
        // Partial success — commit capital, skip cleanup
        if cErr := e.commitPerpCapital(c.reservation, c.pos.ID); cErr != nil {
            e.log.Error("capital commit for partial %s: %v", c.opp.Symbol, cErr)
            if e.allocator != nil && e.allocator.Enabled() {
                _ = e.allocator.Reconcile()
            }
        }
        return
    }
    if err != nil {
        e.releasePerpReservation(c.reservation)
        e.log.Error("trade execution failed for %s: %v", c.opp.Symbol, err)
        e.cleanupFailedPosition(c.opp.Symbol, err.Error())
        return
    }
    if cErr := e.commitPerpCapital(c.reservation, c.pos.ID); cErr != nil {
        e.log.Error("capital commit failed for %s: %v", c.opp.Symbol, cErr)
    }
}(c)
```

Manual open caller (L346-357):
```go
go func() {
    defer lock.Release()
    err := e.executeTradeV2WithPos(*opp, pendingPos, approval.Size, approval.Price, approval.GapBPS)
    if errors.Is(err, errPartialEntry) {
        // Partial success — commit capital, DON'T remove position
        if cErr := e.commitPerpCapital(reservation, pendingPos.ID); cErr != nil {
            e.log.Error("manual open: capital commit for partial %s: %v", symbol, cErr)
            if e.allocator != nil && e.allocator.Enabled() {
                _ = e.allocator.Reconcile()
            }
        }
        return
    }
    if err != nil {
        e.releasePerpReservation(reservation)
        e.log.Error("manual open: trade execution failed for %s: %v", symbol, err)
        e.removePendingPosition(pendingPos)
        return
    }
    if cErr := e.commitPerpCapital(reservation, pendingPos.ID); cErr != nil {
        e.log.Error("manual open: capital commit failed for %s: %v", symbol, cErr)
    }
}()
```

In-loop trims (L3315-3322, L3408-3416): keep existing behavior — depth loop continues.

---

## Fix B — HIGH: StatusPartial positions stranded after crash

**File**: `internal/engine/consolidate.go` L119-134

**Problem**: Consolidator only processes `StatusActive`. StatusPartial positions never reconciled.

**v1 feedback**: Use `getExchangePositionSize` (engine.go L3907). Use `markPositionClosed` for bookkeeping.
**v2 feedback**: Promote to Active after trim without checking trim remainder. Entry prices may be missing in partial rows → SL triggers fail.

**v3 Fix**:

```go
// After the main StatusActive loop (L134), add:
for _, pos := range positions {
    if pos.Status != models.StatusPartial {
        continue
    }
    if busySymbols[pos.LongExchange+":"+pos.Symbol] || busySymbols[pos.ShortExchange+":"+pos.Symbol] {
        continue
    }
    e.reconcilePartialPosition(pos)
}
```

New function with v2 feedback fixes:
```go
func (e *Engine) reconcilePartialPosition(pos *models.ArbitragePosition) {
    longExch, lok := e.exchanges[pos.LongExchange]
    shortExch, sok := e.exchanges[pos.ShortExchange]
    if !lok || !sok {
        return
    }

    longActual, longErr := getExchangePositionSize(longExch, pos.Symbol, "long")
    shortActual, shortErr := getExchangePositionSize(shortExch, pos.Symbol, "short")
    if longErr != nil || shortErr != nil {
        e.log.Warn("reconcilePartial %s: query failed (long=%v short=%v)", pos.ID, longErr, shortErr)
        return
    }

    // Both zero → close as failed entry
    if longActual == 0 && shortActual == 0 {
        e.log.Info("reconcilePartial %s: both sides zero, closing", pos.ID)
        e.markPartialClosed(pos, "partial_zero_on_reconcile", "entry_failed: no fills on reconcile")
        return
    }

    // One zero → close non-zero leg, then close position
    if longActual == 0 || shortActual == 0 {
        e.log.Warn("reconcilePartial %s: one-sided (long=%.6f short=%.6f)", pos.ID, longActual, shortActual)
        if longActual > 0 {
            rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, longActual)
            if rem > 0 {
                e.log.Error("ORPHAN: %s long %.6f on %s after partial close", pos.Symbol, rem, pos.LongExchange)
                return // don't close position if orphan remains — retry next cycle
            }
        }
        if shortActual > 0 {
            rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, shortActual)
            if rem > 0 {
                e.log.Error("ORPHAN: %s short %.6f on %s after partial close", pos.Symbol, rem, pos.ShortExchange)
                return
            }
        }
        e.markPartialClosed(pos, "partial_one_sided", "entry_failed: one-sided partial")
        return
    }

    // Both have fills → trim to matched
    minFill := math.Min(longActual, shortActual)
    trimOK := true
    if longActual > minFill {
        rem := e.closeFullyWithRetry(longExch, pos.Symbol, exchange.SideSell, longActual-minFill)
        if rem > 0 {
            e.log.Error("reconcilePartial %s: long trim incomplete, %.6f remaining", pos.ID, rem)
            trimOK = false
        }
    }
    if shortActual > minFill {
        rem := e.closeFullyWithRetry(shortExch, pos.Symbol, exchange.SideBuy, shortActual-minFill)
        if rem > 0 {
            e.log.Error("reconcilePartial %s: short trim incomplete, %.6f remaining", pos.ID, rem)
            trimOK = false
        }
    }

    if !trimOK {
        // Update sizes to reflect actual state, stay as partial, retry next cycle
        pos.LongSize = longActual  // will be rechecked next consolidation
        pos.ShortSize = shortActual
        pos.UpdatedAt = time.Now().UTC()
        _ = e.db.SavePosition(pos)
        return
    }

    // Trim succeeded — promote to active
    pos.LongSize = minFill
    pos.ShortSize = minFill
    pos.Status = models.StatusActive
    pos.FailureReason = ""
    pos.FailureStage = ""

    // Entry prices: use saved values if available, otherwise query BBO
    if pos.LongEntry <= 0 {
        if bbo, ok := longExch.GetBBO(pos.Symbol); ok && bbo.Ask > 0 {
            pos.LongEntry = bbo.Ask
        }
    }
    if pos.ShortEntry <= 0 {
        if bbo, ok := shortExch.GetBBO(pos.Symbol); ok && bbo.Bid > 0 {
            pos.ShortEntry = bbo.Bid
        }
    }
    pos.EntryNotional = math.Max(pos.LongEntry*pos.LongSize, pos.ShortEntry*pos.ShortSize)
    pos.UpdatedAt = time.Now().UTC()
    _ = e.db.SavePosition(pos)
    e.api.BroadcastPositionUpdate(pos)

    // Only attach SL if entry prices are valid
    if pos.LongEntry > 0 && pos.ShortEntry > 0 {
        e.attachStopLosses(pos)
    } else {
        e.log.Warn("reconcilePartial %s: promoted but no entry prices — SL skipped", pos.ID)
    }
    e.log.Info("reconcilePartial %s: promoted to active (size=%.6f)", pos.ID, minFill)
}

// markPartialClosed handles close bookkeeping for a failed partial position.
// Uses same pattern as markPositionClosed but without PnL reconciliation.
func (e *Engine) markPartialClosed(pos *models.ArbitragePosition, reason, exitReason string) {
    pos.Status = models.StatusClosed
    pos.FailureReason = reason
    pos.ExitReason = exitReason
    pos.UpdatedAt = time.Now().UTC()
    _ = e.db.SavePosition(pos)
    _ = e.db.AddToHistory(pos)
    e.releasePerpPosition(pos.ID)
    e.api.BroadcastPositionUpdate(pos)
}
```

---

## Fix C — HIGH: Post-fill SavePosition failure leaves orphan fills

**File**: `internal/engine/engine.go` L3537-3541

**v1 feedback**: SavePosition is idempotent (Redis HSet). Retry is fine.
**v2 feedback**: Returning error triggers reservation release (L2180) + cleanupFailedPosition (L2182) which breaks filled partials. Manual open path does `removePendingPosition` which marks row closed even with fills.

**v3 feedback**: Same sentinel gap as Fix A (manual open). Also, debounced save may not have persisted StatusPartial.

**v4 feedback**: Same checkpoint failure hole as Fix A.

**v5 Fix**:
Fix A Step 2 now retries the force checkpoint and goes flat on failure. If the checkpoint succeeded but the final StatusActive save fails, the DB has a valid StatusPartial row. Retry final save, then return `errPartialEntry`.

```go
// L3537-3541: replace single save with retry
pos.Status = models.StatusActive
pos.UpdatedAt = time.Now().UTC()
var saveErr error
for attempt := 1; attempt <= 3; attempt++ {
    saveErr = e.db.SavePosition(pos)
    if saveErr == nil {
        break
    }
    e.log.Error("save active position %s attempt %d/3: %v", posID, attempt, saveErr)
    time.Sleep(time.Duration(attempt) * time.Second)
}
if saveErr != nil {
    // Force checkpoint (Fix A Step 2) already succeeded — DB has StatusPartial.
    // (If checkpoint had failed, we would have gone flat and returned normal error.)
    // Return sentinel — caller commits capital, consolidator reconciles.
    e.log.Error("CRITICAL: %s filled but final save failed — StatusPartial in DB for consolidator", posID)
    return errPartialEntry
}
```

---

## Fix D — HIGH: confirmFill treats unknown as zero-fill

**File**: `internal/engine/engine.go` L3569-3617

**Problem**: REST failure → return (0, 0). Unknown and confirmed-zero conflated.

**v1 feedback**: 13 callers across entry/exit/rotation/consolidator. Each needs different unknown handling. Changing signature affects all.

**v2 Fix**:
Add a separate `confirmFillSafe` wrapper that returns error for unknown state. Existing `confirmFill` stays unchanged for backwards compatibility. Only entry-path callers switch to `confirmFillSafe`.

```go
// New wrapper — only used by entry path
func (e *Engine) confirmFillSafe(exch exchange.Exchange, orderID, symbol string) (float64, float64, error) {
    filled, avg := e.confirmFill(exch, orderID, symbol)
    if filled == 0 && avg == 0 {
        // Check if this was a REST failure or a genuine zero fill.
        // Re-query REST to distinguish.
        restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
        if restErr != nil {
            return 0, 0, fmt.Errorf("fill state unknown for %s on %s: %w", orderID, exch.Name(), restErr)
        }
        if restFilled > 0 {
            return restFilled, 0, nil // filled but no avg price
        }
        return 0, 0, nil // confirmed zero
    }
    return filled, avg, nil
}
```

Update entry-path callers (6 sites in depth fill loop) to use `confirmFillSafe`:
- First-leg zero + error (unknown): cancel order + do NOT proceed to second leg
- Second-leg zero + error (unknown) after first filled: do NOT auto-rollback, log ORPHAN WARNING, save as StatusPartial

Exit/rotation/consolidator callers keep using `confirmFill` unchanged — they already have their own recovery paths.

---

## Fix E — HIGH: Allocator override stale → fallback to tier-3 bypasses funding

**File**: `internal/engine/engine.go` L868-933, L1093-1108

**v1 feedback**: `hasAllocatorOverrides` doesn't exist. Use `applyAllocatorOverrides` return value to signal whether overrides existed.

**v2 Fix**:
Change `applyAllocatorOverrides` to return `(filtered []models.Opportunity, hadOverrides bool)`:

```go
// L868: change return signature
func (e *Engine) applyAllocatorOverrides(opps []models.Opportunity) ([]models.Opportunity, bool) {
    overrides := e.allocOverrides
    e.allocOverrides = nil

    if overrides == nil || len(overrides) == 0 {
        return nil, false  // no overrides existed
    }
    
    // ... existing filter logic ...
    
    if len(filtered) == 0 {
        e.log.Warn("entry: all allocator overrides stale, no opps passed filter")
        return nil, true  // overrides existed but all stale
    }
    return filtered, true
}
```

Update caller at L1093-1108:
```go
// Tier 2: allocator overrides
if len(entryOpps) == 0 {
    patched, hadOverrides := e.applyAllocatorOverrides(result.Opps)
    if len(patched) > 0 {
        tier = "tier-2-rebalance-overrides"
        entryOpps = patched
    } else if hadOverrides {
        // Allocator ran but all overrides stale — block tier-3 fallback
        tier = "blocked-stale-overrides"
        e.log.Warn("entry: allocator ran but overrides stale, skipping tier-3 to avoid unfunded entries")
        // Skip executeArbitrage — entryOpps stays empty
    }
}

// Tier 3: rank-based — only if allocator did NOT run
if len(entryOpps) == 0 && tier == "none" {
    tier = "tier-3-rank-based"
    entryOpps = result.Opps
}
```

---

## Fix F — MEDIUM: Allocator commit failure not properly handled

**File**: `internal/engine/engine.go` L2185-2187

**v1 feedback**: `allocator.Commit` already retries 8 watch conflicts internally (L137+). Outer retry is redundant. Use `Reconcile()` (allocator.go L367) as fallback.

**v2 Fix**:
On commit failure, trigger `Reconcile()` which rebuilds allocator state from DB positions:

```go
// L2185-2187:
if err := e.commitPerpCapital(c.reservation, c.pos.ID); err != nil {
    e.log.Error("capital commit failed for %s: %v — triggering allocator reconcile", c.opp.Symbol, err)
    if e.allocator != nil && e.allocator.Enabled() {
        if rErr := e.allocator.Reconcile(); rErr != nil {
            e.log.Error("allocator reconcile after commit failure: %v", rErr)
        }
    }
}
```

---

## Fix G — LOW: buildOppsFromAllocatorChoices is dead code (PASS — unchanged)

**File**: `internal/engine/engine.go` L938-994

**Fix**: Remove the entire function. No callers exist.

---

## Implementation Order

1. Fix D (HIGH) — confirmFillSafe wrapper (prerequisite for Fix A)
2. Fix A (CRITICAL) — trim failure handling
3. Fix B (HIGH) — partial position reconciliation in consolidator
4. Fix C (HIGH) — SavePosition retry
5. Fix E (HIGH) — allocator stale override block
6. Fix F (MEDIUM) — Reconcile on commit failure
7. Fix G (LOW) — dead code removal

## Verification

- `go build ./...` must pass
- `go test ./internal/engine/... ./internal/risk/...` must pass
- Manual review of trim failure + partial reconciliation paths
