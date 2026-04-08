# Plan: Consolidator Flat Position Stuck Fix
Version: v4
Date: 2026-04-08
Status: APPROVED

## Changes

### Bug — Flat positions (both sizes=0 on exchange) stuck as Active forever
- `internal/engine/consolidate.go:260-350` (consolidatePosition)
- `internal/engine/consolidate.go:478-484` (PnL guard retry limit)

## Why

**Gap 1:** `consolidatePosition()` checks `if pos.LongSize > 0 && longSize == 0` (L313) and `if pos.ShortSize > 0 && shortSize == 0` (L332). When both local sizes are 0, conditions are FALSE. Returns at L387 without closing.

**Gap 2:** PnL guard at L481-484 returns early when `GetClosePnL` fails, with no retry limit or fallback. Retries every 5 minutes indefinitely.

## How

### Fix 1 — Detect exchange-flat positions (v3: correct placement + correct guard)

v2 issues:
- Insertion point before sibling guard at L299 → would fire on shared-leg positions
- `exitActive` guard doesn't protect L4 reduce (reducePosition runs synchronously without setting exitActive)

**v3 approach:** Insert AFTER the sibling guard return at L299. For L4 reduce protection, use `entryActive` check instead (the engine sets `entryActive` for the symbol during health actions), or simply check if the position's sizes were recently updated.

Actually, the L4 reduce path (`handleReduce` at engine.go ~L1467) calls `reducePosition` synchronously on the health-action goroutine, then calls `UpdatePositionFields` which writes new sizes atomically. The consolidator reads from Redis (`GetActivePositions`), so it sees either pre-reduce or post-reduce sizes — never a partially-written state. The transient window is only in-memory (`pos` object), not in Redis. Since `consolidatePosition` is called with positions read from Redis, and the exchange query at L268-269 is real-time, the scenario is: Redis has old sizes, exchange has 0 (reduce already executed on exchange but `UpdatePositionFields` hasn't persisted yet). This window is milliseconds. Adding a "recently updated" check is sufficient.

**consolidate.go — after L299 return (after sibling guard), before L302:**
```go
// Detect exchange-flat positions: both exchange sizes are 0 but status Active.
// This catches positions where exit reduced both legs but consolidator
// didn't mark them closed before sizes were zeroed.
if longSize == 0 && shortSize == 0 && pos.Status == models.StatusActive {
    // Guard: skip if position was updated very recently (within 60s).
    // Protects against recent consolidator size syncs (L372 sets UpdatedAt)
    // and recent entry/exit activity. Note: reducePosition does NOT set
    // UpdatedAt, so this guard does not cover the L4 reduce window directly.
    // That window is milliseconds in practice and not a practical concern.
    if time.Since(pos.UpdatedAt) < 60*time.Second {
        e.log.Debug("consolidate: %s exchange-flat but recently updated (%.0fs ago), skipping",
            pos.ID, time.Since(pos.UpdatedAt).Seconds())
        return
    }
    
    e.log.Warn("consolidate: %s exchange-flat (both sizes=0) but Active for %.0fs, attempting close",
        pos.ID, time.Since(pos.UpdatedAt).Seconds())
    // Set local sizes to 0 so markPositionClosed skips close orders (legs already flat).
    pos.LongSize = 0
    pos.ShortSize = 0
    e.markPositionClosed(pos, "exchange-flat detected by consolidator")
    return
}
```

**Why `time.Since(pos.UpdatedAt) < 60s` is sufficient:**
- The 5-minute consolidator cycle means a position must be flat for at least one cycle before detection
- The 60s guard protects against recent consolidator size syncs (L372 sets `UpdatedAt`) and recent entry/exit activity
- **Note:** `reducePosition` (exit.go:1393-1400) does NOT set `UpdatedAt` — its `UpdatePositionFields` callback only sets `LongSize`/`ShortSize`. The 60s guard therefore does NOT cover the L4 reduce transient window. However, that window is milliseconds (between PlaceOrder fill and UpdatePositionFields persist) and is not a practical concern
- No dependency on `exitActive` — avoids the issue that L4 reduce doesn't set `exitActive`

**Why no dust tolerance needed for this check:**
- Exchange `longSize`/`shortSize` from `getExchangePositionSize` returns exactly 0 for flat positions
- Dust positions (0.0001 BTC) return non-zero — they won't trigger `== 0`
- Dust handling is a separate concern (existing missing-leg detection + close retry handles it)

### Fix 2 — Retry limit with full close bookkeeping (v3: correct delete placement + init)

v2 issues:
- `delete(consolidateRetries)` unreachable on success path
- `consolidateRetries` map not initialized (nil map panic)

**New field + initialization:**
```go
// Engine struct:
consolidateRetries map[string]int

// NewEngine():
consolidateRetries: make(map[string]int),
```

**consolidate.go ~L478 (the PnL guard):**
```go
const maxConsolidateRetries = 6 // 6 × 5 min = 30 min

longAgg, longOK := aggregateClosePnLBySide(longPnLs, "long")
shortAgg, shortOK := aggregateClosePnLBySide(shortPnLs, "short")

if !longOK || !shortOK {
    e.consolidateRetries[pos.ID]++
    retries := e.consolidateRetries[pos.ID]
    
    if retries <= maxConsolidateRetries {
        e.log.Warn("consolidate: %s missing PnL (long=%v short=%v) attempt %d/%d, keeping existing PnL=%.4f funding=%.4f",
            pos.ID, longOK, shortOK, retries, maxConsolidateRetries, pos.RealizedPnL, pos.FundingCollected)
        return
    }
    
    // Max retries exceeded — use partial data, fall through to full close path.
    e.log.Warn("consolidate: %s max retries (%d) exceeded, force-closing with partial PnL",
        pos.ID, maxConsolidateRetries)
    delete(e.consolidateRetries, pos.ID)
    
    if !longOK {
        longAgg = exchange.ClosePnL{}
    }
    if !shortOK {
        shortAgg = exchange.ClosePnL{}
    }
    // Fall through to splitSharedPnL → reconciledPnL → full close bookkeeping
    // (SavePosition, AddToHistory, UpdateStats, releasePerpPosition, BroadcastPositionUpdate)
} else {
    // Both sides OK — clear any retry counter.
    delete(e.consolidateRetries, pos.ID)
}

longAgg = e.splitSharedPnL(longAgg, pos, "long")
shortAgg = e.splitSharedPnL(shortAgg, pos, "short")
// ... rest of close path unchanged
```

**Key v3 fixes:**
- `delete` is in the `else` branch (success path) — properly clears counter when both sides OK
- `delete` also called in the max-retries branch — cleans up before fall-through
- `consolidateRetries` initialized in `NewEngine()` — no nil map panic
- Fall-through reaches `splitSharedPnL` (L487) which handles zero inputs safely (confirmed: just multiplies by ratio)
- Fall-through continues to full bookkeeping at L526-537

## Risk

- Fix 1: 60s time guard is conservative. If the bot was stopped for >60s and restarted, a genuinely flat position would be closed immediately — this is correct behavior.
- Fix 1: No `exitActive` dependency — avoids the L4 reduce issue entirely.
- Fix 2: Force-close with partial/zero PnL is inaccurate but better than stuck forever. Logged as warning for operator visibility.
- Fix 2: `splitSharedPnL` with zero ClosePnL: ratio calculation uses `max()` denominator guard, so division by zero is safe.

## Graphify

Relevant nodes: consolidatePosition (consolidate.go:260), markPositionClosed (consolidate.go:392), aggregateClosePnLBySide (exit.go), splitSharedPnL (exit.go:1040), reducePosition (engine.go), handleReduce (engine.go).

## Review History
- v1: Codex gpt-5.4 — NEEDS-REVISION — local sizes wrong, L4 reduce transient, bookkeeping incomplete
- v2: Claude code-reviewer — NEEDS-REVISION — insertion before sibling guard, exitActive insufficient for L4, delete unreachable, nil map
- v3: Claude code-reviewer — NEEDS-REVISION — Fix 1 UpdatedAt rationale wrong (reducePosition doesn't set it); Fix 2 PASS
- v4: Claude code-reviewer — ALL PASS — rationale correction verified
