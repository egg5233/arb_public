# Plan: PnL Reconcile Safety + Analytics Fee Sign Fix
Version: v8
Date: 2026-04-08
Status: IMPLEMENTED

## Changes

### Bug A — consolidate.go: PnL zeroed when GetClosePnL fails
- `internal/engine/consolidate.go:478-504`

### Bug B — analytics + UI: ExitFees sign mismatch
- `internal/analytics/aggregator.go:181-184`
- `internal/analytics/snapshot.go:84-96` (RecordPerpClose)
- `internal/analytics/snapshot.go:106-113` (RecordSpotClose — no change, ExitFees already positive)
- `internal/analytics/snapshot.go:161-163` (BackfillFromHistory — same sign bug)
- `web/src/components/PnLBreakdown.tsx:58-59` (per-leg path: raw long_total_fees/short_total_fees with mixed signs)
- `web/src/components/PnLBreakdown.tsx:116` (fallback path: raw exit_fees with mixed signs)

### Bug C — BasisGainLoss semantic fix
- `internal/engine/exit.go:936` (tryReconcilePnL)
- `internal/engine/consolidate.go:500-501` (markPositionClosed)

## Why

**Bug A:** `aggregateClosePnLBySide` returns `(ClosePnL, bool)`. The `ok` flag is discarded with `_`. When both API calls fail, PnL/FundingCollected are overwritten to 0. A position that earned $50 shows $0.

**Bug B:** `ClosePnL.Fees` has mixed sign conventions across exchanges: Binance/OKX return negative (cost), Bybit/Bitget return positive (cost). `ExitFees` inherits this mixed sign. `EntryFees` from `UserTrade.Fee` is always positive (engine.go L3662). Aggregator adds both to `FeesTotal` without normalizing → mixed signs. UI has two rendering paths: (1) per-leg path (PnLBreakdown.tsx:58-59) renders raw `long_total_fees`/`short_total_fees` with `cc()` coloring — Bybit positive fees show green instead of red; (2) fallback path (PnLBreakdown.tsx:116) renders raw `exit_fees`. Note: `negative: true` in the fallback path only forces red text color (line 134), it does NOT add a minus sign — the raw value is displayed as-is via `fmt()`.

**Bug C (v7 revision):** v6 proposed `pricePnL - RotationPnL` to bypass fee-sign ambiguity. Codex review found that `RotationPnL` is accumulated from rotated-leg `NetPnL` (exit.go:2367, includes fees+funding), not `PricePnL`. Subtracting net rotation from price-only current legs mixes types. The correct fix: `BasisGainLoss = pricePnL` (current legs only, no RotationPnL subtraction). RotationPnL is tracked in its own field. Decomposition from `reconciledPnL` is impossible because fee signs differ per exchange and per-rotation price/funding/fee breakdown isn't stored.

## How

### Bug A — Guard PnL write with ok flags (unchanged from v6)

```go
// consolidate.go ~L478:
longAgg, longOK := aggregateClosePnLBySide(longPnLs, "long")
shortAgg, shortOK := aggregateClosePnLBySide(shortPnLs, "short")

// Guard: require BOTH sides to have PnL data before overwriting.
// Matches exit.go tryReconcilePnL pattern — partial data is worse than stale data.
if !longOK || !shortOK {
    e.log.Warn("consolidate: %s missing PnL (long=%v short=%v), keeping existing PnL=%.4f funding=%.4f",
        pos.ID, longOK, shortOK, pos.RealizedPnL, pos.FundingCollected)
    // Do NOT set sizes/status/UpdatedAt — no SavePosition follows, so mutations are discarded.
    // Position stays Active in Redis → consolidator retries next cycle.
    return
}

longAgg = e.splitSharedPnL(longAgg, pos, "long")
shortAgg = e.splitSharedPnL(shortAgg, pos, "short")
// ... rest unchanged
```

**Retry behavior:** The early-return path does NOT call SavePosition, so the position stays StatusActive/StatusPartial in Redis. The consolidator WILL retry on the next 5-minute cycle. This is correct — the API may succeed on retry. If the API keeps failing, PnL stays at the last written value (from exit.go's tryReconcilePnL or a previous successful consolidation), which is better than zeroing.

### Bug B — Normalize fee sign (v7: added UI fix)

**aggregator.go ~L181:**
```go
if p.ExitFees != 0 {
    s.FeesTotal += math.Abs(p.ExitFees) // ExitFees has mixed signs across exchanges, normalize to positive
} else {
    s.FeesTotal += p.EntryFees // already positive
}
```

**snapshot.go RecordPerpClose ~L85:**
```go
perpFees := pos.ExitFees
if perpFees != 0 {
    perpFees = math.Abs(perpFees) // normalize mixed-sign to positive
} else {
    perpFees = pos.EntryFees
}
```

**snapshot.go RecordSpotClose ~L113:**
```go
FeesTotal: pos.EntryFees + pos.ExitFees,  // NO change needed — spot ExitFees is always positive
```
Verified: `SpotFuturesPosition.ExitFees` is computed as `size * price * feeRate` (execution.go L1858, L1861) — always positive. No `math.Abs` needed for spot path.

**snapshot.go BackfillFromHistory ~L161:**
```go
fees: func() float64 {
    if p.ExitFees != 0 { return math.Abs(p.ExitFees) }
    return p.EntryFees
}(),
```

**Import note:** Only `snapshot.go` needs `import "math"` added. `aggregator.go` already has it (L4).

**PnLBreakdown.tsx per-leg path ~L58-59 (NEW in v8):**
```tsx
const lFees = -Math.abs(p.long_total_fees ?? 0);
const sFees = -Math.abs(p.short_total_fees ?? 0);
```
Fees are always a cost — normalize to negative so `cc()` renders them red and `fmt()` shows a minus sign. `Math.abs` handles both Binance (negative raw) and Bybit (positive raw). The subtotal at L64 (`lSub = lFees + lFund + lClose`) now consistently subtracts fees regardless of exchange.

**PnLBreakdown.tsx fallback path ~L116 (NEW in v7, corrected in v8):**
```tsx
{ label: t('hist.exitFees'), value: -Math.abs(p.exit_fees ?? 0), negative: true },
```
`negative: true` only forces red text (L134: `cell.negative ? 'text-red-400' : cc(cell.value)`), it does NOT add a minus sign. `-Math.abs()` ensures the displayed value is always negative (cost). `fmt()` renders it as `$-X.XXXX`.

### Bug C — BasisGainLoss: current-leg PricePnL only (v7 revision)

**v6 approach (REJECTED by Codex):** `pricePnL - RotationPnL` — mixes price-only with net-PnL types.

**v7 approach:** `BasisGainLoss = pricePnL` (current legs only).

`RotationPnL` is NOT subtracted because:
1. `RotationPnL` is `NetPnL` (includes rotation fees + rotation funding), not `PricePnL`
2. Per-rotation price/funding/fee breakdown is not stored — cannot decompose
3. `RotationPnL` is already tracked in its own field (`position.go:27`) and displayed separately
4. `BasisGainLoss` definition (position.go:40): "price-based P/L excluding funding and fees" — this matches current-leg PricePnL exactly

**Fix at exit.go ~L936 and consolidate.go ~L500:**

```go
// Replace:
// pricePnL := longAgg.PricePnL + shortAgg.PricePnL
// pos.BasisGainLoss = pricePnL - pos.RotationPnL
// With:
pos.BasisGainLoss = longAgg.PricePnL + shortAgg.PricePnL
```

Both locations (exit.go tryReconcilePnL and consolidate.go markPositionClosed) get the same change. No new imports needed.

**Accounting note:** For positions with rotations, the sum of displayed components (BasisGainLoss + FundingCollected + ExitFees + RotationPnL) may not exactly equal RealizedPnL because rotation PnL cannot be decomposed into price/funding/fee. This is a pre-existing data model limitation, not introduced by this fix. The gap is the rotation's funding + fees component, which is embedded in RotationPnL but can't be attributed to the separate breakdown fields.

## Risk

- Bug A early-return: PnL won't be recorded if both APIs fail. Early-return does NOT call SavePosition, so position stays Active — consolidator WILL retry on next cycle. Keeping stale PnL is better than zeroing.
- Bug B math.Abs: changes analytics total for reconciled positions. Historical data in Redis already has mixed signs — can't retroactively fix. Only new positions consistent.
- Bug B UI: Math.abs changes display for positions where exit_fees was stored as negative. This is a visual correction, not a data change.
- Bug C: Removing RotationPnL subtraction changes BasisGainLoss values for rotated positions. For non-rotated positions (RotationPnL=0), no change. For rotated positions, BasisGainLoss will increase by the RotationPnL amount (typically small relative to total PnL).
- Edge: ExitFees genuinely 0 after reconciliation (exchange didn't report fees). Falls through to EntryFees. ExitFees "already contains total fees (open+close)" per comment — so using EntryFees as fallback may double-count. This is a pre-existing design assumption, not introduced by this fix.

## Graphify

Relevant nodes: ClosePnL (types.go), aggregateClosePnLBySide (exit.go:996), tryReconcilePnL (exit.go), consolidate functions. EntryFees written at engine.go:3683 from UserTrade.Fee (positive). Consolidator filters StatusActive/StatusPartial only (L63, L81, L120). RotationPnL accumulated from NetPnL at exit.go:2367.

## Review History
- v1: Claude code-reviewer — NEEDS-REVISION — missing one-side warning log, missing UpdatedAt, EntryFees sign unverified, snapshot.go fixes unspecified, consolidator retry claim wrong
- v2: Claude code-reviewer — NEEDS-REVISION — SpotFuturesPosition.ExitFees sign unverified, math import note imprecise
- v3: Claude code-reviewer — NEEDS-REVISION — BackfillFromHistory (L161) missed, retry prose wrong
- v4: Skeptic reviewer — BLOCK — partial reconciliation (one side ok) corrupts PnL, must require BOTH sides
- v5: Claude code-reviewer — BLOCK — math.Abs(totalFees) is wrong, would break Binance BasisGainLoss
- v6: Claude code-reviewer — ALL PASS — Codex review found 2 issues
- v6-codex: Codex gpt-5.4 — NEEDS-REVISION — (1) RotationPnL is NetPnL not PricePnL, `pricePnL - RotationPnL` mixes types; (2) PnLBreakdown.tsx renders raw exit_fees with mixed signs
- v7-codex: Codex gpt-5.4 — NEEDS-REVISION — (1) `negative: true` only forces red text, not minus sign; (2) per-leg path renders raw long_total_fees/short_total_fees with mixed signs (green for Bybit positive fees)
- v8-codex: Codex gpt-5.4 — ALL PASS — approach is sound, -Math.abs handles both sign conventions, -0 edge case safe (fmt returns '-'). SpotPositions.tsx:305 also uses exit_fees but is spot path (always positive). Ready to implement.
