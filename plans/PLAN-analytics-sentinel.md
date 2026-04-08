# Plan: Replace ExitFees==0 Sentinel with Explicit Reconciliation Flag
Version: v3
Date: 2026-04-08
Status: APPROVED

## Changes

### Bug (LOW) — ExitFees==0 used as "not reconciled" sentinel, breaks for zero-fee positions
- `internal/models/position.go` (ArbitragePosition struct — add HasReconciled)
- `internal/analytics/aggregator.go:181-184`
- `internal/analytics/snapshot.go:86-91` (RecordPerpClose)
- `internal/analytics/snapshot.go:164-167` (BackfillFromHistory)
- `internal/engine/exit.go:920-933` (tryReconcilePnL — including no-diff early return)
- `internal/engine/consolidate.go:499` (markPositionClosed)
- `internal/database/state.go` (inside GetPosition:142, GetActivePositions:161, GetHistory:278)

## Why

Analytics uses `ExitFees != 0` as sentinel for "has been reconciled." Falls back to `EntryFees` when ExitFees is 0, creating double-count for VIP zero-fee accounts or maker rebate positions.

## How

### Add HasReconciled flag

**position.go:**
```go
HasReconciled bool `json:"has_reconciled,omitempty"` // true when exit PnL reconciled via GetClosePnL
```

### Set flag on ALL reconciliation paths (v3: includes no-diff)

**exit.go tryReconcilePnL ~L920 (no-diff early return):**
```go
if !needsPnLUpdate && !needsFundingUpdate && !needsExitUpdate && !needsBreakdownUpdate {
    // Reconciliation succeeded but values unchanged — still persist the flag.
    if !pos.HasReconciled {
        e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
            fresh.HasReconciled = true
            return true
        })
    }
    return true
}
```

**exit.go ~L933 (inside main UpdatePositionFields callback):**
```go
fresh.HasReconciled = true
```

**consolidate.go ~L499 (markPositionClosed):**
```go
pos.HasReconciled = true
```

### Replace sentinel checks in analytics

**aggregator.go ~L181:**
```go
if p.HasReconciled {
    s.FeesTotal += math.Abs(p.ExitFees)
} else {
    s.FeesTotal += p.EntryFees
}
```

**snapshot.go RecordPerpClose ~L86:**
```go
var perpFees float64
if pos.HasReconciled {
    perpFees = math.Abs(pos.ExitFees)
} else {
    perpFees = pos.EntryFees
}
```

**snapshot.go BackfillFromHistory ~L164:**
```go
fees: func() float64 {
    if p.HasReconciled { return math.Abs(p.ExitFees) }
    return p.EntryFees
}(),
```

### Legacy migration — InferHasReconciled inside DB function bodies (v3 revision)

v2 issues:
- Line numbers inverted (GetPosition is L142, GetActivePositions is L161, GetHistory is L278)
- Must be inside DB function bodies so ALL callers get normalized data (including analytics_handlers.go:56, capital.go:77, spotengine/engine.go:471)
- Need comment distinguishing LongFunding/ShortFunding (exit-only) from FundingCollected (mid-life)

**New method on ArbitragePosition (in position.go):**
```go
// InferHasReconciled backfills HasReconciled for legacy positions
// reconciled before this flag existed. Uses exit-reconciliation breakdown
// fields as evidence. Called on every DB read to normalize legacy data.
//
// Field safety notes:
// - LongFunding/ShortFunding are per-leg EXIT breakdown fields, only set by
//   GetClosePnL during reconciliation. They are NOT the mid-life funding
//   accumulator (FundingCollected). Safe to use as evidence.
// - FundingCollected is updated during position lifetime by the funding
//   tracker. It MUST NOT be used in this check — it would cause false positives
//   on active (un-reconciled) positions.
// - EntryFees is set during entry. NOT used here — same reason.
//
// Ambiguous case: old positions where reconciliation returned all-zero diffs
// (no-diff path at exit.go:920) remain HasReconciled=false. They use EntryFees
// as fallback — same behavior as before this fix. No regression.
func (p *ArbitragePosition) InferHasReconciled() {
    if p.HasReconciled {
        return
    }
    if p.ExitFees != 0 ||
        p.LongTotalFees != 0 || p.ShortTotalFees != 0 ||
        p.BasisGainLoss != 0 ||
        p.LongFunding != 0 || p.ShortFunding != 0 ||
        p.LongClosePnL != 0 || p.ShortClosePnL != 0 {
        p.HasReconciled = true
    }
}
```

**Call inside DB function bodies (state.go):**

**GetPosition (L142) — after unmarshal at L154:**
```go
pos.InferHasReconciled()
```

**GetActivePositions (L161) — after unmarshal in loop:**
```go
pos.InferHasReconciled()
```

**GetHistory (L278) — after unmarshal in loop at L293:**
```go
pos.InferHasReconciled()
```

By placing inside the DB functions, ALL callers are covered:
- `internal/api/analytics_handlers.go:56` (ComputeStrategySummary via GetHistory)
- `internal/engine/capital.go:77` (allocator APR via GetHistory)
- `internal/spotengine/engine.go:471` (spot history via GetHistory)
- `cmd/main.go:265` (BackfillFromHistory via GetHistory)
- Any future callers

## Risk

- **Backward compat**: `omitempty` on bool → missing JSON field deserializes as `false`. `InferHasReconciled()` fixes up on read. No data loss.
- **No-diff race**: If concurrent goroutine sets HasReconciled between read and check at L920, the `if !pos.HasReconciled` guard may trigger a redundant `UpdatePositionFields`. Harmless — callback reads fresh and sets true idempotently. One-time cost per position.
- **Performance**: `InferHasReconciled()` is a trivial field check on every read. Negligible overhead.
- **SpotFuturesPosition**: Does NOT need HasReconciled. Spot analytics use direct fee sums without sentinel pattern (aggregator.go:224, snapshot.go:116). Confirmed.

## Graphify

Relevant nodes: ArbitragePosition (position.go), aggregator (aggregator.go), RecordPerpClose (snapshot.go), BackfillFromHistory (snapshot.go), tryReconcilePnL (exit.go), markPositionClosed (consolidate.go), GetPosition (state.go:142), GetActivePositions (state.go:161), GetHistory (state.go:278).

## Review History
- v1: Codex gpt-5.4 — NEEDS-REVISION — no-diff path misses flag, migration too narrow/wrong location
- v2: Claude code-reviewer — NEEDS-REVISION — line numbers inverted, must be inside DB function body, need LongFunding/FundingCollected distinction comment
- v3: Claude code-reviewer — ALL PASS
