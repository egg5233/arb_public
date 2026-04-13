# PLAN: Stuck-Active Position Dust Fix (NATGASUSDT incident)

**Status:** REVISION 10 — pending Codex re-review
**Author:** claude
**Date:** 2026-04-13
**Trigger:** Live position `natgasusdt-1775865302363` stuck Active for 3 days with `LongSize=ShortSize=7.1e-15` dust; 2 manual closes from dashboard failed; consolidator skipped every 5 min.
**Revision history:**
- v1 → Codex NEEDS REVISION: dustThreshold scope, Fix C safety arg, acceptance criterion, Fix B impact mis-stated.
- v2 → Apply Codex required changes 1-3 + add Fix D (NextFunding stale on zero-rate, found while diagnosing TSLAUSDT).
- v3 → Codex v2 PASS for A/B/D/E; NEEDS REVISION on Fix C (concurrent close bookkeeping race between L4 full-flatten and consolidator). Add Fix F (CAS-guarded final close), narrow Fix D acceptance, fix doc nits.
- v4 → Codex v3 PASS for B/C/D/E; NEEDS REVISION on Fix F (UpdatePositionFields is not real CAS — Get→Save with no lock; status gate too narrow rejects StatusExiting preemption callers; field-copy too thin missing close bundle) + Fix A doc nit (`executeDepthExit` not `executeSmartClose`). v4 switches Fix F to AcquireOwnedLock + StatusActive|StatusExiting claim + re-read-after-save bookkeeping.
- v5 → Codex v4 PASS for outer lock / lock placement / Fix A / B/C/D/E; NEEDS REVISION on Fix F: (1) markPositionClosed predicate too broad (text says Active|Exiting but snippet accepts anything not Closed/Closing); (2) used nonexistent fields `LongRealizedPnL`/`ShortRealizedPnL`, real names are `LongClosePnL`/`ShortClosePnL`; (3) reread failure aborts and drops post-close side effects (CancelAllOrders, releasePerpPosition, lossLimiter, cooldowns, reconcilePnL). v5 tightens predicate, fixes field names, adds reread retry+fallback with explicit side-effect preservation.
- v6 → Codex v5 PASS for predicate(markPositionClosed)/field-names/reread-retry; NEEDS REVISION on: (1) `closePositionWithMode` predicate still too narrow — `GetActivePositions()` returns ALL non-closed records and `checkDelistPositions()` only skips Closing/Closed, so Pending/Partial are legitimate callers; (2) `markPositionClosed` bundle missing `UpdatedAt`; (3) `closePositionWithMode` bundle still has placeholder; (4) wrong function signatures (`releasePerpPosition` takes ID not pos; `reconcilePnL` not `partialReconcile`; `RecordClosedPnL` 4 args; no `writeCooldown`, real is `discovery.SetSymbolCooldown`/`SetReEnterCooldown`; close-path `reconcilePnL` is synchronous not goroutine).
- v7 → Codex v6 PASS for predicate/markPositionClosed bundle/signatures; NEEDS REVISION on: (1) `closePositionWithMode` phase-2 wrote ExitFees/LongClosePnL/ShortClosePnL — those belong to `reconcilePnL`; pre-writing them would trip `InferHasReconciled` (`models/position.go:73-83`) and skip the actual reconcile; (2) `CancelAllOrders` must stay BEFORE SavePosition (`exit.go:1913-1916`), not moved to post-save bookkeeping (avoids canceling orders on a re-used symbol); (3) `markPositionClosed` `UpdatedAt = now` line is redundant since `UpdatePositionFields` auto-bumps (`state.go:217-228`); (4) consolidator async `reconcilePnL` is conditional on `partialClose`/`PartialReconcile`, not unconditional (`consolidate.go:577-583, 613-615`); (5) test wording: "GetPosition stub" → "tiny test hook".
- v8 → Codex v7 PASS for phase-2 mirror/CancelAllOrders ordering/markPositionClosed UpdatedAt+reconcile; NEEDS REVISION on: (1) reread fallback uses stale local `pos` — phase-2 mutate only applied to `fresh` inside `UpdatePositionFields`, fallback `bookkeepingPos = pos` has none of the close-bundle changes; (2) same issue in markPositionClosed (UpdatedAt missing on fallback); (3) reread-fallback test must assert fallback object is closed snapshot, not just call count; (4) field-preservation test wording for UpdatedAt: "approximately now" not "match local"; (5) Manual integration Path 1 expectation wrong: `executeDepthExit` fully-flat branch, not `markPositionClosed`. v8 syncs local pos with close bundle BEFORE reread, tightens test assertions, fixes Path 1.
- v9 → Codex v8 PASS for both local-pos-syncs / Manual integration / A-E; NEEDS REVISION only on test wording: (1) close-path fallback test must assert via observable sinks (history entry shows `Status=closed`+RealizedPnL+LongExit/ShortExit), not direct BroadcastPositionUpdate/reconcilePnL argument capture; (2) "10 calls total" needs explicit preconditions (non-nil loss limiter + negative PnL + LossCooldownHours>0 + ReEnterCooldownHours>0); (3) markPositionClosed fallback test also must assert closed snapshot via history. v9 reworks 3 test descriptions accordingly.
- v10 → Codex v9 PASS for tests/preconditions list/cross-fix; ONE wording fix: `RecordClosedPnL` is gated by `lossLimiter != nil` only (loss_limit.go:42, exit.go:1932), NOT by `realizedPnL < 0`. Negative PnL only gates `SetSymbolCooldown` (exit.go:1946).

---

## 1. Goal

Stop positions from being stranded in `Active` status when both legs are effectively closed on the exchange. Four fixes that together break the failure chain, let manual close finish the job for NATGAS without DB surgery, and keep `NextFunding` accurate during zero-rate periods.

**In scope (this plan):**
- Fix A: Bug 1 — depth-exit finalizer dust snap
- Fix B: Bug 3 — PnL sanity fallback when notional is dust
- Fix C: Bug 4 — consolidator UpdatedAt bypass when exchange-flat
- Fix D: TSLAUSDT — `NextFunding` not advanced when funding rate = 0
- Fix E (added per Codex v1 change #3): mirror dust semantics in `consolidate.go:493-496` (`markPositionClosed`) so consolidator auto-close works without manual close fallback
- Fix F (added per Codex v2): CAS-guard the final closed write in `markPositionClosed` and `closePositionWithMode` to prevent double-close bookkeeping when L4 full-flatten races consolidator

**Out of scope (deferred to a follow-up):**
- Bug 2 (dust origin in `reducePosition` raw subtraction at exit.go:1707-1718) — defense-in-depth on L4 reduce path, not on dashboard manual-close path. Track separately.
- `closePositionWithMode:1848` dust gate — sits on manual-close fallback + emergency close, not the current NATGAS root cause but worth a follow-up audit.
- Other stuck paths (orphan cleanup engine.go:1822-1833) — same snap-to-zero idea applies; track separately.

---

## 2. Background (incident timeline)

Verified from `/root/arb/logs/arb_2026041[012].log`:

```
04-10 23:55  entry long=52.3 bitget short=52.3 binance, entry_spread=+6.4681
04-13 00:30  exit check: spread reversal -0.19 (reversal 1/1, tolerating)
04-13 01:30  exit check: spread reversal -1.235 (reversal 2, exiting)
04-13 01:30  depth exit started; binance fill 52.3; bitget top-up failed → market fallback
04-13 01:30  CRITICAL: NOT fully flat — longRem=0 shortRem=0 — reverted to active
             (← position record left with LongSize=ShortSize=7.1e-15)
04-13 02:30  retry: longSize=0 shortSize=0, "remaining 0 < step 0.1, done"
             → still PARTIAL revert; PnL 3.9060 zeroed (notional=0 sanity check)
04-13 04:30  same retry pattern
04-13 06:55  USER manual close ×2 → same depth path → same PARTIAL revert
04-13 06:55+ consolidator: "exchange-flat but recently updated (10s ago), skipping"
             (every 5 min, indefinitely)
```

TSLAUSDT separate incident (2026-04-13 ~08:30):
- `next_funding=2026-04-13T08:00:00Z` already 30 min in the past
- Both legs funding rate = 0 → `fundingAccrued=0` → `fundingChanged=false`
- `NextFunding` advancement gated by `if fundingChanged` → never advances → settlement protection window in `checkSpreadReversal` becomes wrong

Codex review v1 (xhigh) confirmed all four original bugs and ranked Bug 1 as root cause. Codex review v2 of this plan: NEEDS REVISION (3 required changes + Fix B impact correction).

---

## 3. Fixes

### Fix A — Bug 1: snap dust to zero in depth-exit finalizer

**File:** `internal/engine/exit.go`
**Sites:**
- `~900-902` — compute `longRemainder`/`shortRemainder`
- `~952-956` — DB partial revert path
- `~1010-1013` — local/broadcast partial revert path
- (the fully-flat branch at `937-945` already persists exact zero — no extra snap needed there)

**Current behavior:**
```go
longRemainder := totalLong - closedLong
shortRemainder := totalShort - closedShort
fullyFlat := longRemainder <= 0 && shortRemainder <= 0
// 7.1e-15 fails the gate even though depth loop already declared "done"
// (loop stop conditions at exit.go:402-417 and 515-523 use step OR min, not exact zero)
```

**New behavior — REVISED per Codex change #1:**

The threshold must align with the depth-exit loop's own done-criteria: `remaining < step OR remaining < min`. So `dustThreshold` = `max(stepSize, minSize)`. When contract metadata is missing, fall back to the existing repo idiom: treat the remainder as dust when `formatSize(...) == 0` (the same rule already used in `engine.go:3129-3138` and `exit.go:3102-3105`/`3140-3145`).

```go
// dustThreshold returns the minimum tradable size for this exchange/symbol.
// Aligned with depth-exit loop done-criteria (exit.go:402-417, 515-523):
// loop stops when remaining < step OR < min, so finalizer must accept the same.
func (e *Engine) dustThreshold(exchName, symbol string) (float64, bool) {
    if e.contracts != nil {
        if exContracts, ok := e.contracts[exchName]; ok {
            if ci, ok := exContracts[symbol]; ok {
                t := math.Max(ci.StepSize, ci.MinSize)
                if t > 0 {
                    return t, true
                }
            }
        }
    }
    return 0, false  // caller falls back to formatSize-zero rule
}

// isDust returns true when remainder is below tradable threshold.
// Two-tier check matching existing repo behavior:
//   1. If contract metadata available: remainder < max(step, min)
//   2. Else: formatSize-rounded remainder == 0 (same as engine.go:3129-3138)
func (e *Engine) isDust(exchName, symbol string, remainder float64) bool {
    if remainder <= 0 {
        return true
    }
    if t, ok := e.dustThreshold(exchName, symbol); ok {
        return remainder < t
    }
    formatted := e.formatSize(exchName, symbol, remainder)
    if v, _ := strconv.ParseFloat(formatted, 64); v <= 0 {
        return true
    }
    return false
}
```

Apply at the finalizer:
```go
longRemainder  := totalLong - closedLong
shortRemainder := totalShort - closedShort

if e.isDust(pos.LongExchange,  pos.Symbol, longRemainder)  { longRemainder  = 0 }
if e.isDust(pos.ShortExchange, pos.Symbol, shortRemainder) { shortRemainder = 0 }

fullyFlat := longRemainder <= 0 && shortRemainder <= 0
```

**Critical: only snap remainders, NOT `closedLong`/`closedShort`.**
Per Codex (`exit.go:864-918`): closedLong/closedShort feed VWAP/PnL math; snapping them would invent fills.

**Persist sites** (952-956 / 1010-1013): when reverting active, also snap `pos.LongSize`/`pos.ShortSize` to 0 if `isDust(...)` so the persisted record never carries dust to the next retry. The closed-branch persist at 937-945 already writes exact 0 — no change.

**Effect:** depth exit with dust remainders now hits `fullyFlat=true` and takes the existing fully-flat close branch at `exit.go:931-1008` (inside `executeDepthExit`, NOT `executeSmartClose` which is at `exit.go:2014-2137`), instead of the partial revert at `952-956`/`1010-1013`. NATGAS unblocks on next manual close.

---

### Fix B — Bug 3: PnL sanity fallback when notional is dust

**File:** `internal/engine/exit.go`
**Sites:** `~923-926` (depth-exit), `~1894-1897` (`closePositionWithMode`)

**Current behavior:**
```go
notional := math.Max(pos.LongEntry*totalLong, pos.ShortEntry*totalShort)
if math.Abs(realizedPnL) > 2*notional {
    e.log.Error("PnL %.4f exceeds 2x notional %.4f — close prices suspect, zeroing", realizedPnL, notional)
    realizedPnL = 0
}
// On dust retries: totalLong=totalShort=7e-15 → notional=0 → any pnl > 0 zeroed
```

**New behavior — pattern already used in `reconcilePnL` at exit.go:1155-1165:**
```go
notional := math.Max(pos.LongEntry*totalLong, pos.ShortEntry*totalShort)

// Fallback: when current residual sizes are dust/zero, the sanity check
// has no signal. Use the position's stored EntryNotional, or skip if both unknown.
// EntryNotional is "max(long_entry*long_size, short_entry*short_size) at open"
// per internal/models/position.go:48 — exactly what we want.
if notional <= 0 {
    if pos.EntryNotional > 0 {
        notional = pos.EntryNotional
        e.log.Warn("PnL sanity: residual notional=0, using EntryNotional=%.4f", notional)
    } else {
        // Both unknown — skip the sanity gate. Better to record a wrong PnL
        // than to silently zero a real one (Codex change: this affects stats/cooldowns,
        // not just display — see exit.go:978-1000 and 1928-1959).
        e.log.Warn("PnL sanity: notional=0 and EntryNotional=0, skipping sanity check (PnL=%.4f)", realizedPnL)
        goto skipSanity
    }
}
if math.Abs(realizedPnL) > 2*notional {
    e.log.Error("PnL %.4f exceeds 2x notional %.4f — close prices suspect, zeroing", realizedPnL, notional)
    realizedPnL = 0
}
skipSanity:
```

`goto` label pattern is acceptable in this codebase (already in `exit.go:424-430`, `exit.go:824-828`, `engine.go:2721-2728`, `engine.go:2798-2802`).

Apply same change to `closePositionWithMode` at `~1894-1897`. Codex audit found no third site with this pattern.

**Effect (corrected from v1):** Restored PnL accuracy on dust retries. **NOT cosmetic** — this is on the realized-PnL → stats / cooldowns / loss-tracking pipeline (`exit.go:978-1000`, `exit.go:1928-1959`).

---

### Fix C — Bug 4: consolidator UpdatedAt bypass when exchange-flat

**File:** `internal/engine/consolidate.go`
**Sites:** `317-325` (the 60s skip)

**Current behavior:**
```go
if longSize == 0 && shortSize == 0 && pos.Status == models.StatusActive {
    if time.Since(pos.UpdatedAt) < 60*time.Second {
        e.log.Debug("consolidate: %s exchange-flat but recently updated (%.0fs ago), skipping", ...)
        return
    }
    ...markPositionClosed("exchange-flat detected by consolidator")
}
```

**New behavior — REVISED per Codex change #2 (corrected safety argument):**

The bypass is safe because:
1. Both exchange reads return 0 (consolidate.go:280-281). Exchange truth, not local guess.
2. `markPositionClosed` re-verifies flat before saving closed state (consolidate.go:473-496). Defense in depth.
3. Position with dust LocalSize but exchange-zero is functionally already closed; nothing to protect.

NOT relying on the `busySymbols`/`exitActive` claim from v1 — that claim was too strong because **L4 reduce path bypasses both maps** (`engine.go:1685-1691` calls `reducePosition` at `exit.go:1707-1718` which writes sizes without claiming `exitActive`/`entryActive`). Codex caught this.

```go
if longSize == 0 && shortSize == 0 && pos.Status == models.StatusActive {
    // Drop the 60s UpdatedAt skip for this branch.
    //
    // Why this is safe:
    //   - Both exchange reads returned 0 (consolidate.go:280-281). Exchange-truth.
    //   - markPositionClosed re-verifies via getExchangePositionSize before
    //     persisting closed state (consolidate.go:473-496).
    //
    // Why UpdatedAt was wrong: UpdatePositionFields auto-bumps UpdatedAt on
    // every successful mutate (database/state.go:217-228), and several writers
    // touch active positions for non-size reasons (risk-monitor CurrentSpread,
    // exit-check NextFunding/ReversalCount/ZeroSpreadCount, funding tracker
    // FundingCollected/UnrealizedPnL, rotation/reconcile metadata). UpdatedAt
    // is therefore not a "size changed recently" signal.
    //
    // Note: L4 reduce (engine.go:1685-1691 → reducePosition exit.go:1707-1718)
    // does not claim exitActive/entryActive but writes sizes. With this bypass,
    // a window exists where consolidator sees exchange-zero immediately after
    // L4 closes both legs but L4 hasn't finished its bookkeeping. The
    // markPositionClosed re-verify guards against this (it would re-read
    // exchange and see zero anyway → safe to close).
    e.log.Warn("consolidate: %s exchange-flat (both sizes=0) but Active for %.0fs, attempting close",
        pos.ID, time.Since(pos.UpdatedAt).Seconds())
    pos.LongSize = 0
    pos.ShortSize = 0
    e.markPositionClosed(pos, "exchange-flat detected by consolidator")
    return
}
```

**Effect:** consolidator becomes the genuine safety net. Combined with Fix E below, NATGAS auto-closes within one consolidator tick after deploy without needing manual click.

---

### Fix E — markPositionClosed dust semantics (NEW per Codex change #3)

**File:** `internal/engine/consolidate.go`
**Site:** `493-496`

**Current behavior:**
```go
if longRemaining > 0 || shortRemaining > 0 {
    e.log.Error("consolidate: cannot close %s — verify-flat failed", pos.ID)
    return  // keep position active
}
```

If `getExchangePositionSize` returns microdust (e.g. exchange returns "0.00000001" due to its own rounding), `markPositionClosed` refuses to close even though Fix C said exchange-zero. This breaks the Fix C acceptance criterion of "auto-close within one consolidator tick".

**New behavior:** apply same `isDust` from Fix A.

```go
if !e.isDust(pos.LongExchange,  pos.Symbol, longRemaining) ||
   !e.isDust(pos.ShortExchange, pos.Symbol, shortRemaining) {
    e.log.Error("consolidate: cannot close %s — verify-flat failed (long=%.6e short=%.6e)",
        pos.ID, longRemaining, shortRemaining)
    return  // keep position active
}
// Both within dust threshold → safe to close.
```

**Effect:** consolidator auto-close path no longer blocked by exchange-side rounding dust. Acceptance criterion holds.

---

### Fix F — Per-position close lock + status claim + re-read bookkeeping (REVISED per Codex v3)

**Files/sites:**
- `internal/engine/consolidate.go:473-496` and `:552-615` (`markPositionClosed` — persists Closed, history, stats)
- `internal/engine/exit.go:1755-1760` and `:1918-1959` (`closePositionWithMode` / `closePositionEmergency` — same)

**Why this matters (Codex v2 finding):**
Fix C drops the consolidator's `UpdatedAt` guard and relies on exchange-flat re-verify in `markPositionClosed`. But re-verify only proves leg flatness — it does NOT serialize close bookkeeping. Realistic race:

```
T0  L4 reduce decides to full-flatten position P
T1  reducePosition() zeroes pos.LongSize/ShortSize at exit.go:1711-1734
    (writes via UpdatePositionFields → status still Active)
T2  Consolidator tick fires; reads exchange = both 0; status=Active; Fix C bypasses
    UpdatedAt guard; calls markPositionClosed
T3  markPositionClosed re-verifies exchange-flat → OK → SavePosition(Closed) +
    AddToHistory + UpdateStats   ← bookkeeping write #1
T4  L4 path continues to closePositionEmergency()
T5  closePositionEmergency unconditionally SavePosition(Closing/Closed) +
    AddToHistory + UpdateStats   ← bookkeeping write #2 (DUPLICATE)
```

Result: `arb:history` list has two entries for one trade; `arb:stats` counters incremented twice; potentially mismatched realized PnL between the two records.

**Why v3's predicate-only approach was wrong (Codex v3 finding):**
v3 proposed using `UpdatePositionFields` predicate as a "CAS". But `UpdatePositionFields` (`database/state.go:217-228`) is `GetPosition → mutate → SavePosition` with **no Redis WATCH, no Lua compare, no lock**. Two contenders can both read `StatusActive`, both predicate returns true, both `SavePosition(Closed)` — and both still go on to call `AddToHistory` / `UpdateStats`. It is a stale-copy guard, not a CAS. The repo already provides real serialization: `AcquireOwnedLock` (`internal/database/locks.go:56-78`) — Redis SET NX + TTL + Lua-script release.

**Fix F v4 design — three layers:**
1. **Outer:** per-position Redis lock via `AcquireOwnedLock("close:"+pos.ID, 30s)` — only one closer at a time.
2. **Middle:** `UpdatePositionFields` predicate as inner stale-copy guard against status flips between lock acquisition and save.
3. **Bookkeeping:** re-read the saved record from Redis before `AddToHistory` / `UpdateStats` / `BroadcastPositionUpdate` so we always work from the persisted truth (eliminates field-copy-completeness risk).

**`markPositionClosed` (consolidate.go:473-615):**
```go
// Outer: per-position close lock (Redis, true serialization).
lock, ok, err := e.db.AcquireOwnedLock("close:"+pos.ID, 30*time.Second)
if err != nil {
    e.log.Error("consolidate: failed to acquire close lock for %s: %v", pos.ID, err)
    return
}
if !ok {
    e.log.Info("consolidate: %s close lock held by another path, skipping", pos.ID)
    return
}
defer lock.Release()

// (existing dust verify + close bundle population stays here — see consolidate.go:473-596.
//  The plan does NOT reduce field copy. All fields currently set on `pos` before
//  SavePosition continue to be set: RealizedPnL, FundingCollected, ExitFees,
//  BasisGainLoss, per-leg fees/funding/PnL, exit prices, HasReconciled,
//  PartialReconcile, sizes, status, ExitReason, etc.)

// Middle: stale-copy guard. Accept ONLY StatusActive or StatusExiting.
// Reject everything else (including Pending, Partial, Closing, Closed) —
// a generic "not Closed" gate would admit unexpected states for no benefit.
saved := false
err = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
    if fresh.Status != models.StatusActive && fresh.Status != models.StatusExiting {
        return false
    }
    // Copy the FULL close bundle. Field set MUST match what the existing
    // markPositionClosed body assigns at consolidate.go:559-595.
    // Field names verified against internal/models/position.go:42-56:
    //   - LongClosePnL  / ShortClosePnL  (NOT LongRealizedPnL / ShortRealizedPnL)
    //   - LongTotalFees / ShortTotalFees
    //   - LongFunding   / ShortFunding
    fresh.Status            = models.StatusClosed
    fresh.LongSize          = 0
    fresh.ShortSize         = 0
    fresh.ExitReason        = pos.ExitReason
    fresh.LongExit          = pos.LongExit
    fresh.ShortExit         = pos.ShortExit
    fresh.RealizedPnL       = pos.RealizedPnL
    fresh.FundingCollected  = pos.FundingCollected
    fresh.ExitFees          = pos.ExitFees
    fresh.BasisGainLoss     = pos.BasisGainLoss
    fresh.LongTotalFees     = pos.LongTotalFees
    fresh.ShortTotalFees    = pos.ShortTotalFees
    fresh.LongFunding       = pos.LongFunding
    fresh.ShortFunding      = pos.ShortFunding
    fresh.LongClosePnL      = pos.LongClosePnL       // NOT LongRealizedPnL
    fresh.ShortClosePnL     = pos.ShortClosePnL      // NOT ShortRealizedPnL
    fresh.HasReconciled     = pos.HasReconciled
    fresh.PartialReconcile  = pos.PartialReconcile
    // NOTE: do NOT set fresh.UpdatedAt — UpdatePositionFields auto-bumps it
    // at state.go:217-228. Adding it here would be redundant.
    saved = true
    return true
})
if err != nil { e.log.Error(...); return }
if !saved {
    e.log.Info("consolidate: %s already closed by another path, skipping bookkeeping", pos.ID)
    return
}

// Sync local `pos` with the just-persisted close bundle so the reread fallback
// path has the same closed-state record as Redis. Without this, fallback would
// broadcast/history a stale Active record.
pos.Status     = models.StatusClosed
pos.LongSize   = 0
pos.ShortSize  = 0
pos.UpdatedAt  = time.Now().UTC()   // mirror the auto-bump done by UpdatePositionFields
// (other close-bundle fields were already on local pos before the predicate copy)

// Bookkeeping: re-read persisted truth, with retry-then-fallback. Reread failure
// MUST NOT abort — that would drop history + releasePerpPosition + reconcilePnL.
var bookkeepingPos *models.ArbitragePosition
for attempt := 0; attempt < 3; attempt++ {
    if updated, gerr := e.db.GetPosition(pos.ID); gerr == nil && updated != nil {
        bookkeepingPos = updated
        break
    }
    time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
}
if bookkeepingPos == nil {
    e.log.Warn("consolidate: %s reread failed after retries — using local pos for bookkeeping", pos.ID)
    bookkeepingPos = pos    // pos is now in-sync with persisted close bundle (above)
}

// Run ALL existing post-close side effects from consolidate.go:577-615.
// EXACT signatures verified against repo (capital.go:178-184, exit.go:1932-1959):
e.db.AddToHistory(bookkeepingPos)
e.db.UpdateStats(bookkeepingPos.RealizedPnL, bookkeepingPos.RealizedPnL > 0)
e.releasePerpPosition(bookkeepingPos.ID)             // signature: takes posID string, NOT *Position
// reconcilePnL kickoff is CONDITIONAL — only when this was a partial close
// (consolidate.go:577-583 sets PartialReconcile=true; line 613-615 launches goroutine).
// Full closes have already reconciled inline.
if bookkeepingPos.PartialReconcile {
    posCopy := *bookkeepingPos
    go e.reconcilePnL(&posCopy)
}
e.api.BroadcastPositionUpdate(bookkeepingPos)
```

**`closePositionWithMode` / `closePositionEmergency` (exit.go:1755-1760, 1903-1959):**

Same pattern with two phases. **Claim gate must accept everything except `StatusClosing` and `StatusClosed`** (i.e. Active, Exiting, Pending, Partial all valid). Reasoning per Codex v5:
- `GetActivePositions()` returns ALL non-closed records (`database/state.go:119-140`)
- `checkDelistPositions()` only skips Closing/Closed before calling `closePositionEmergency()` (`engine.go:1450-1476`)
- Therefore Pending and Partial are legitimate callers; v5's narrower `Active|Exiting` gate would silently no-op them

```go
// Outer: per-position close lock.
lock, ok, err := e.db.AcquireOwnedLock("close:"+pos.ID, 30*time.Second)
if err != nil { ... return err }
if !ok {
    e.log.Info("close %s lock held by another path, skipping", pos.ID)
    return nil
}
defer lock.Release()

// Phase 1: claim → Closing. Reject only terminal states (Closing/Closed).
// Mirrors checkDelistPositions filter at engine.go:1450-1476.
claimed := false
err = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
    if fresh.Status == models.StatusClosing || fresh.Status == models.StatusClosed {
        return false   // already closing/closed
    }
    // Active, Exiting, Pending, Partial — all valid claim sources.
    fresh.Status = models.StatusClosing
    if pos.ExitReason != "" { fresh.ExitReason = pos.ExitReason }
    fresh.UpdatedAt = time.Now().UTC()
    claimed = true
    return true
})
if err != nil { ... return err }
if !claimed {
    e.log.Info("close %s: already Closing/Closed, no-op", pos.ID)
    return nil
}

// ... existing exchange close orders + PnL math run here (longClose, shortClose, realizedPnL etc.) ...

// CancelAllOrders MUST run BEFORE phase-2 SavePosition (mirror exit.go:1913-1916).
// Reason: prevents canceling orders on a re-used symbol if a new entry races
// in after the close write but before the cancels.
longExch.CancelAllOrders(pos.Symbol)
shortExch.CancelAllOrders(pos.Symbol)

// Phase 2: final Closed write. STRICT mirror of exit.go:1902-1918 — only the
// fields the existing pre-SavePosition block sets. Do NOT add ExitFees,
// LongClosePnL, ShortClosePnL here — those belong to reconcilePnL. Pre-writing
// them would trip InferHasReconciled (models/position.go:73-83) and skip the
// real reconcile pass.
saved := false
err = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
    if fresh.Status == models.StatusClosed { return false }
    // Preserve pre-set exit prices when computed close price is 0 (existing behavior at exit.go:1903-1908).
    // The caller may have populated LongExit/ShortExit before invoking close
    // (e.g. recovery / preemption paths that already know exit prices).
    if longClose > 0  { fresh.LongExit  = longClose }
    if shortClose > 0 { fresh.ShortExit = shortClose }
    fresh.RealizedPnL = realizedPnL
    fresh.Status      = models.StatusClosed
    // NOTE: do NOT set fresh.UpdatedAt — UpdatePositionFields auto-bumps it.
    // NOTE: do NOT set ExitReason here — caller/claim path already set it
    // (exit.go:287, :1733; engine.go:1475, :1620, :1727-1732).
    saved = true
    return true
})
if err != nil { ... return err }
if !saved {
    e.log.Info("close %s: status already Closed, skipping bookkeeping", pos.ID)
    return nil
}

// Sync local `pos` with the just-persisted close bundle so the reread fallback
// has the same closed-state record as Redis (mirror of phase-2 mutator above).
if longClose > 0  { pos.LongExit  = longClose }
if shortClose > 0 { pos.ShortExit = shortClose }
pos.RealizedPnL = realizedPnL
pos.Status      = models.StatusClosed
pos.UpdatedAt   = time.Now().UTC()    // mirror auto-bump

// Bookkeeping with retry-then-fallback. MUST preserve all current post-close
// side effects from exit.go:1913-1959 — losing them would be a regression.
var bookkeepingPos *models.ArbitragePosition
for attempt := 0; attempt < 3; attempt++ {
    if updated, gerr := e.db.GetPosition(pos.ID); gerr == nil && updated != nil {
        bookkeepingPos = updated
        break
    }
    time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
}
if bookkeepingPos == nil {
    e.log.Warn("close %s: reread failed after retries — using local pos for bookkeeping", pos.ID)
    bookkeepingPos = pos
}

// Run remaining post-close side effects with EXACT repo signatures (verified per Codex v6):
e.db.AddToHistory(bookkeepingPos)                                     // existing
e.db.UpdateStats(bookkeepingPos.RealizedPnL, bookkeepingPos.RealizedPnL > 0)  // existing
// loss limiter — 4 args per loss_limit.go:42-53:
e.lossLimiter.RecordClosedPnL(bookkeepingPos.ID, bookkeepingPos.RealizedPnL, bookkeepingPos.Symbol, time.Now().UTC())
e.releasePerpPosition(bookkeepingPos.ID)                              // takes posID string
// Cooldowns — real names per exit.go:1946-1954:
e.discovery.SetSymbolCooldown(bookkeepingPos.Symbol, ...)
e.discovery.SetReEnterCooldown(bookkeepingPos.Symbol, ...)
e.api.BroadcastPositionUpdate(bookkeepingPos)
// reconcilePnL is SYNCHRONOUS in close path (exit.go:1957-1959), NOT a goroutine:
posCopy := *bookkeepingPos
e.reconcilePnL(&posCopy)
return nil
```

**Lock placement notes:**
- Acquire `close:<posID>` lock OUTSIDE `exitMu` to avoid lock-order inversion with the exit goroutine map.
- For preemption callers (L4/L5/delist/SL): existing pattern is `cancelExitGoroutine(...)` returns first, THEN call closePositionEmergency. Lock acquisition happens inside the close call, so no change needed at call sites.
- 30s TTL is generous (close orders rarely take >10s); falls back via TTL if process crashes mid-close.

**Effect:** Exactly one path wins the lock + claim. The loser short-circuits at the lock or at the claim predicate. Exactly one `AddToHistory` + one `UpdateStats` + one `BroadcastPositionUpdate` per closed position, regardless of how many paths race. Existing preemption flows that go through `StatusExiting` still work because the claim gate accepts both.

---

### Fix D — NextFunding stale on zero-rate (TSLAUSDT)

**File:** `internal/engine/engine.go`
**Site:** `1997-2014` (`updateFundingCollected`)

**Current behavior:**
```go
if fundingChanged || uplChanged {
    nextFunding := e.computeNextFunding(...)
    UpdatePositionFields(...)
        if fundingChanged {
            fresh.FundingCollected = fundingAccrued
            if !fresh.NextFunding.IsZero() && time.Now().UTC().After(fresh.NextFunding) {
                fresh.NextFunding = nextFunding   // ← only runs when funding was credited
            }
        }
        fresh.LongUnrealizedPnL = longUPL
        fresh.ShortUnrealizedPnL = shortUPL
}
```

`fundingChanged := fundingAccrued != 0 && fundingAccrued != pos.FundingCollected`

When both legs have rate=0 (e.g. TSLAUSDT during quiet period), no funding fee → `fundingAccrued=0` → `fundingChanged=false` → `NextFunding` never advances even when `uplChanged=true`.

**New behavior:** lift `NextFunding` advance OUT of `if fundingChanged`.

```go
if fundingChanged || uplChanged {
    nextFunding := e.computeNextFunding(...)
    UpdatePositionFields(...)
        if fundingChanged {
            fresh.FundingCollected = fundingAccrued
        }
        // Always advance NextFunding when settlement passed, regardless of
        // whether funding was credited this cycle. Zero-rate periods otherwise
        // leave NextFunding stuck in the past, breaking the ±10min settlement
        // protection window in checkSpreadReversal (exit.go:1419-1428).
        if !fresh.NextFunding.IsZero() && time.Now().UTC().After(fresh.NextFunding) {
            fresh.NextFunding = nextFunding
        }
        fresh.LongUnrealizedPnL = longUPL
        fresh.ShortUnrealizedPnL = shortUPL
}
```

**Do NOT change `fundingChanged` definition.** The `fundingAccrued != 0` part exists because some exchange APIs transiently return 0 for FundingFee; relaxing it would risk overwriting `FundingCollected` with stale 0.

**Edge case acknowledged:** if `uplChanged` is also false (extremely flat market, UPL hasn't moved between ticks), the outer block is skipped and `NextFunding` still doesn't advance. This is rare but real. **Not patched in this plan** — would require a third path "advance NextFunding only" with no other side effects. Track as follow-up.

**Effect:** `NextFunding` reflects actual next settlement during zero-rate periods. Settlement protection window in `checkSpreadReversal` works correctly.

---

## 4. Test Plan

### Unit (Go test, no Redis)

`internal/engine/exit_dust_test.go` (new) — Fix A + Fix B:
- `TestDepthExitFinalizer_DustSnapsToZero_StepBased` — remainder `1e-7`, step=`0.001` → snapped, fullyFlat=true.
- `TestDepthExitFinalizer_DustSnapsToZero_MinBased` — remainder `0.05`, step=0, min=`0.1` → snapped, fullyFlat=true.
- `TestDepthExitFinalizer_NoContractMetadata_FormatSizeZero` — no contract entry; `formatSize(...)` rounds to 0 → snapped (covers Codex change #1 fallback).
- `TestDepthExitFinalizer_NoContractMetadata_FormatSizeNonZero` — no contract entry; `formatSize(...)` returns "0.123" → NOT snapped, revert active.
- `TestDepthExitFinalizer_AboveStep_StillRevert` — remainder=`0.5`, step=`0.001`, min=`0.1` → not dust, revert active (no false positive).
- `TestDepthExitFinalizer_DustPersistedAsZero` — confirm Redis-stored LongSize is exact 0, not `1e-7`.
- `TestDepthExitFinalizer_ClosedFillsNotSnapped` — `closedLong`/`closedShort` are NOT modified by dust logic (Codex critical note: VWAP correctness).
- `TestPnLSanity_DustNotionalUsesEntryNotional` — totalLong=0, EntryNotional=142, PnL=3.9 → preserved.
- `TestPnLSanity_BothZeroSkipsCheck` — notional=0 AND EntryNotional=0 → PnL preserved (warn logged).
- `TestPnLSanity_NormalNotionalUnchanged` — notional=147, PnL=2 → sanity gate runs normally (regression guard).

`internal/engine/consolidate_dust_test.go` (new) — Fix C + Fix E:
- `TestConsolidate_ExchangeFlatBypasses60sGuard` — UpdatedAt=10s ago, both exchange sizes=0 → markPositionClosed called.
- `TestConsolidate_ExchangeOneLegPresentStill60sGuarded` — longSize=0, shortSize=10 → not in flat branch, behavior unchanged.
- `TestConsolidate_UpdatedAtStarvationReproduction` — simulate risk-monitor write loop bumping UpdatedAt every 30s; with v1 code, position stays Active forever; with Fix C, closes within one tick (covers Codex test gap).
- `TestMarkPositionClosed_AcceptsExchangeDust` — `getExchangePositionSize` returns `1e-7` < step → closed.
- `TestMarkPositionClosed_RejectsRealRemainder` — returns 0.5 → still rejected (regression guard).
- `TestConsolidate_ExchangeFlat_DoesNotDoubleCloseDuringConcurrentL4FullFlatten` (Fix F) — **deterministic** race: use `miniredis` + a synchronization point (channel/sleep injected via interface) so both goroutines provably reach `AcquireOwnedLock` before either proceeds. Assert exactly one history append + one stats update + one Closed-status write across both paths. Targets `consolidate.go:317-335`/`595-607` and `exit.go:1711-1734`/`1755-1760`/`1918-1959`.
- `TestClosePositionWithMode_NoOpsIfAlreadyClosed` (Fix F) — pre-set status=Closed, call closePositionWithMode, assert no history/stats writes.
- `TestClosePositionWithMode_AcceptsStatusExiting` (Fix F, **regression guard**) — pre-set status=Exiting (the preemption path state), call closePositionWithMode, assert close completes successfully and bookkeeping runs once. Without this test, v3's overly-narrow `StatusActive`-only gate would silently no-op the entire L4/L5/delist/SL preemption flow.
- `TestMarkPositionClosed_PreservesFullCloseBundle` (Fix F, **field-copy regression guard**) — populate `pos.RealizedPnL`, `FundingCollected`, `ExitFees`, `BasisGainLoss`, per-leg fees/funding/PnL, `HasReconciled`, `PartialReconcile` before calling markPositionClosed. After close, re-read from DB and assert: (a) all populated fields match the local pre-call values; (b) `UpdatedAt` is **approximately now** (within a few seconds, NOT equal to local pre-call value — `UpdatePositionFields` auto-bumps it). Catches the "thin field copy" risk.
- `TestClosePositionWithMode_AcceptsStatusPartial` (Fix F, **regression guard**) — pre-set status=Partial, call closePositionWithMode, assert close completes successfully and bookkeeping runs once. Without this test, an overly-narrow `Active|Exiting`-only gate would silently no-op delist/recovery flows for partial positions.
- `TestClosePositionWithMode_AcceptsStatusPending` (Fix F, **regression guard**) — same, with Pending. Confirms the broad gate handles pre-entry-failure cleanup paths.
- `TestClosePositionWithMode_PreservesPreSetExitPrice` (Fix F) — pre-populate `pos.LongExit=1.5`, then call closePositionWithMode where computed `longClose=0`. Assert the saved record retains `LongExit=1.5` (not overwritten with 0). Mirrors existing behavior at exit.go:1903-1908.
- `TestMarkPositionClosed_RereadFailureFallsBackToLocal` (Fix F) — use a small test-only hook around the post-save reread (NOT a `GetPosition` stub or interface refactor) that forces 3 consecutive failures after SavePosition. Assert: (a) bookkeeping (AddToHistory, UpdateStats, releasePerpPosition, conditional reconcilePnL when PartialReconcile) still runs using local `pos` as fallback; (b) the **history entry is a closed snapshot** — assert via reading back from the history list that `Status=Closed`, `LongSize=0`, `ShortSize=0`, `UpdatedAt` is approximately now. History is the observable sink (`AddToHistory` writes to Redis at state.go) — does not require a second hook on Broadcast/reconcile to verify the fallback object.
- `TestClosePositionWithMode_RereadFailureFallsBackToLocal` (Fix F) — same hook approach for close path. **Preconditions** (must be set up explicitly in test for the "10 calls total" expectation to hold): non-nil `e.lossLimiter` (gates `RecordClosedPnL` at exit.go:1932, per loss_limit.go:42), `realizedPnL < 0` (gates `SetSymbolCooldown` at exit.go:1946 — NOT RecordClosedPnL), `cfg.LossCooldownHours > 0` (also gates `SetSymbolCooldown` at exit.go:1946), `cfg.ReEnterCooldownHours > 0` (gates `SetReEnterCooldown` at exit.go:1951). Assert: (a) all 9 post-claim side effects + 1 sync reconcilePnL = 10 total calls; (b) the **history entry asserts the fallback is a closed snapshot** — read back from history list and confirm `Status=Closed`, `RealizedPnL` matches expected, `LongExit`/`ShortExit` match per pre-set guards (>0 → set, =0 → preserved). If you need to also assert the exact object passed to `BroadcastPositionUpdate` or `reconcilePnL`, add a second tiny test-only bookkeeping hook; the reread-failure hook alone does not expose those arguments.
- `TestSideEffects_ExactlyOnceUnderRace` (Fix F) — combine the race scenario with the same hook. Assert each side-effect counter = 1 across both racing goroutines (lock-loser must short-circuit at AcquireOwnedLock before reaching any side effect).

`internal/engine/funding_nextupdate_test.go` (new) — Fix D:
- `TestUpdateFundingCollected_AdvancesNextFundingOnZeroRate` — both rates=0, uplChanged=true, NextFunding in past → advanced.
- `TestUpdateFundingCollected_AdvancesNextFundingOnNonZeroRate` — fundingChanged=true → advanced (regression guard).
- `TestUpdateFundingCollected_DoesNotAdvanceWhenNotPassedYet` — NextFunding still in future → unchanged.

### Manual integration (VPS, after deploy)

1. Confirm pre-deploy state:
   - `/api/positions` shows NATGAS LongSize=ShortSize=7.1e-15 dust
   - `/api/positions` shows TSLA next_funding stuck at past timestamp
2. Deploy.
3. **Path 1 — manual close NATGAS**:
   - Press manual close on dashboard.
   - Expected: `manual close requested` → `exit goroutine started` → depth done → `executeDepthExit` fully-flat close branch (`exit.go:931-1008`) takes inline close (no PARTIAL revert, no `markPositionClosed` — that's only the consolidator auto-recovery path) → position disappears from `/api/positions`, appears in `/api/history` with `status=closed`, `realized_pnl != 0` (Fix B).
   - Verify Bitget + Binance positions for NATGASUSDT actually 0 via `cmd/balance` or `cmd/closepos --check` (Codex test gap: don't trust dashboard alone).
4. **Path 2 — wait for consolidator** (alternative validation, do this FIRST if testing the auto-recovery path):
   - Wait ≤ 5 min after deploy.
   - Expected: `consolidate: NATGAS exchange-flat ... attempting close` → `markPositionClosed` succeeds → status=closed.
5. **TSLA Fix D**:
   - Within 1 funding cycle (~hourly), confirm `next_funding` advances past current time.
6. **Smoke** — at least one healthy entry+exit cycle on a small position to confirm no false-positive dust snap.

---

## 5. Rollback

All six fixes are conservative additive edits in three production files (`internal/engine/exit.go`, `internal/engine/consolidate.go`, `internal/engine/engine.go`). Rollback = `git revert <commit>`. No schema changes, no Redis migration, no config flags. If a regression appears:
- Fix A regression (premature close): would manifest as positions closing with non-trivial residual (look for `markPositionClosed` calls when remainders were >> step). Revert.
- Fix B regression (PnL preserved when it shouldn't be): WARN logs make it discoverable. Revert if false readings appear.
- Fix C/E regression (consolidator closes in-flight position): mitigated by exchange-zero precondition + markPositionClosed re-verify. If observed (very unlikely), revert.
- Fix D regression (NextFunding advances incorrectly): would manifest as `checkSpreadReversal` skipping when it shouldn't. Revert.

---

## 6. Risks & open questions

| Risk | Mitigation |
|---|---|
| `dustThreshold` returns tradable-size threshold but exchange returns sub-tradable dust on close. Real symbols' step/min on the trades we run? | Funding-arb USDT-perps: step typically 0.001-1, min 0.001-1. `7.1e-15` is many orders below all. |
| Fix B fallback masks a real prices-suspect bug | Logged as WARN, easy to grep. Codex confirmed the alternative (silent zero) is worse — affects stats/cooldowns. |
| Fix C bypass races L4 reduce window | `markPositionClosed` re-verify via exchange query. L4 reduce window is "milliseconds in practice" per existing comment (consolidate.go:322). Race window is sub-100ms. |
| Fix D edge case: rare flat market where uplChanged is also false | Acknowledged. Would require independent NextFunding-only path. Not blocking; track follow-up. |
| Five fixes in one PR | All address the same incident family. Codex review v1 explicitly recommended A+C+E together; B independent but discovered alongside; D found during diagnosis. Splitting risks shipping partial fix that doesn't unblock NATGAS. |
| `closePositionWithMode:1848` and L4 `reducePosition:1707` still have raw dust bug | Out of scope for this plan but explicitly tracked. Manual-close fallback path (`closePositionWithMode`) only fires when depth fails entirely; primary depth-exit path is fixed by Fix A. |

---

## 7. Acceptance Criteria

- [ ] Unit tests pass: `go test ./internal/engine/...`
- [ ] Build green: `make build`
- [ ] NATGAS position closes successfully — either on next manual close (via Fix A) OR within 1 consolidator tick after deploy (via Fix C + Fix E + Fix F together)
- [ ] Verified at exchange level (not just dashboard) that NATGAS Bitget + Binance positions are 0
- [ ] **No double bookkeeping**: history list contains exactly one entry for the closed position; stats counters incremented exactly once
- [ ] **Preemption flow regression**: at least one emergency/fallback close (L4 or L5 or delist or SL) succeeds end-to-end from `StatusExiting` (validated post-deploy by checking logs for `closePositionEmergency` completing without "no-op" skip)
- [ ] TSLA `next_funding` advances on the next funding-tracker pass where `NextFunding` is already in the past AND (`fundingChanged` OR `uplChanged`) is true. The both-flags-false case remains follow-up work (acknowledged limitation)
- [ ] No regression in existing exit/consolidate tests
- [ ] CHANGELOG updated with version bump (likely v0.32.12)
- [ ] Codex post-implementation review PASS

---

## 8. Files changed

- `internal/engine/exit.go` — Fix A (helper + 4 sites: compute, persist×2, no-touch on closed-fills), Fix B (2 sites), Fix F (closePositionWithMode/Emergency CAS at 1755-1760, 1918-1959)
- `internal/engine/consolidate.go` — Fix C (1 site), Fix E (1 site), Fix F (markPositionClosed CAS at 473-496, 595-609)
- `internal/engine/engine.go` — Fix D (1 site)
- `internal/engine/exit_dust_test.go` (new)
- `internal/engine/consolidate_dust_test.go` (new)
- `internal/engine/funding_nextupdate_test.go` (new)
- `VERSION`
- `CHANGELOG.md`
