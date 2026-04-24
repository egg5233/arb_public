# PLAN: Per-Leg Tier 1 Skip for Exchanges Without CloseSize

Version: v1
Date: 2026-04-24
Status: DRAFT

## Problem

Dashboard 交易歷史顯示 fallback 文案「詳細分解僅適用於 v0.27.0 之後平倉的部位」 for recent
positions closed on v0.33.3 (e.g. PORTALUSDT 2026-04-24 08:02, binance→bingx).

Root cause chain (verified by Codex review, VERDICT=PARTIAL with two corrections):

1. `web/src/components/PnLBreakdown.tsx:23-38` — `hasDecomposition` is inferred from
   non-zero breakdown fields (`LongTotalFees`, `ShortTotalFees`, `LongFunding`,
   `ShortFunding`, `LongClosePnL`, `ShortClosePnL`, `ExitFees`, `BasisGainLoss`,
   `Slippage`, `RotationPnL`). When all zero, renders `hist.dataUnavailable`.

2. Per-leg breakdown fields are written in **two** paths:
   - **Async reconcile** (`internal/engine/exit.go:1272-1335` in `tryReconcilePnL`) — the
     normal post-close path.
   - **Consolidator** (`internal/engine/consolidate.go:685-690`) — not affected by this bug.

3. `tryReconcilePnL` has Tier 1 completeness gate at `exit.go:1196-1208` (added by
   commit `8f236707`, SIRENUSDT fix, v0.32.x):
   ```go
   useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
       allSiblingsHaveCloseSize(longSiblings, "long") &&
       allSiblingsHaveCloseSize(shortSiblings, "short")

   if useTier1 {
       longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
       shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
       if longAgg.CloseSize < longExpected-sizeEpsilon || shortAgg.CloseSize < shortExpected-sizeEpsilon {
           e.log.Warn("reconcile ... incomplete close data ..., retrying")
           return false
       }
   }
   ```

4. Binance `GetClosePnL` (`pkg/exchange/binance/adapter.go:1125-1172`) sums
   `/fapi/v1/income` records (`REALIZED_PNL`, `COMMISSION`, `FUNDING_FEE`). Binance
   income API does not expose `qty`, so the returned `exchange.ClosePnL` has
   `CloseSize=0` (zero default) and `Side=""`. Confirmed against
   `doc/EXCHANGEAPI_BINANCE.md:823-857`.

5. Result: every position with Binance as either leg fails the Tier 1 gate on
   every attempt (1 sync + 3 async retries), logs
   `all attempts failed, keeping PnL=-4.9528`, and leaves all breakdown fields at 0.
   Frontend falls back to `hist.dataUnavailable`.

### Evidence (portalusdt-1776757502428, closed 2026-04-24 08:02)

```
longAgg  Fees=-0.393909 Funding=1.277987 PricePnL=-56.998471 NetPnL=-56.114394 (CloseSize=0)
shortAgg Fees=-0.272256 Funding=-0.334333 PricePnL=56.354500  NetPnL=55.747900  (CloseSize=21466.9)
WARN incomplete close data (longRawClose=0.000000/21466.900000 shortRawClose=21466.900000/21466.900000), retrying
... attempts 2, 3, 4 identical ...
ERROR reconcile portalusdt-1776757502428: all attempts failed, keeping PnL=-4.9528
```

PnL/Fees/Funding/PricePnL values are all correct. Only `CloseSize=0` on the
Binance leg causes the gate to fail.

### Why not fix A (populate CloseSize in Binance GetClosePnL)

Codex flagged this as unsafe for quick-fix scope:
- `since = CreatedAt-1m` (or `lastRotation-1m`) — userTrades in this window includes
  **entry** fills, not only closes. Naively summing `|qty|` would double-count.
- `GetClosePnL(symbol, since)` has no "which side am I" signal at the adapter level;
  Binance income records also lack a `positionSide` field.
- Proper A fix requires correlating `/income` `tradeId` with `/userTrades` rows
  (which do have `qty`, `realizedPnl`, `positionSide`). That is a larger adapter
  change with its own risk surface.

This plan defers A and takes the minimal change that unblocks the UI while
preserving the SIRENUSDT protection.

## Solution: Per-Leg Tier 1 Skip

Change the Tier 1 gate in `tryReconcilePnL` so each leg's completeness check
is only enforced when that leg's aggregate reports a usable `CloseSize` (`>0`).
When an aggregate reports `CloseSize==0` (exchange does not expose size — Binance
via `/fapi/v1/income`), skip the gate **for that leg only**. The other leg still
gets full Tier 1 protection if it has real size data, and Tier 2 (notional diff)
still runs for the position as a whole after split.

Key properties:
- Binance-only positions (e.g. binance↔bingx/bybit/gate/okx/bitget): Binance leg
  skipped, other leg checked. Falls through to split + Tier 2 notional guard.
- Pure non-Binance positions (e.g. bybit↔bingx): both legs still enforce Tier 1.
  No regression on SIRENUSDT protection for those pairs.
- Binance on both legs: skipped on both, relies on Tier 2 only. This is worse
  than today only for the edge case the SIRENUSDT gate was designed to catch,
  but since neither leg reports CloseSize, Tier 1 literally has nothing to check.
  Tier 2 notional guard still catches gross corruption.

## Changes

### Change 1: `internal/engine/exit.go` — Tier 1 per-leg gate

Current (`exit.go:1189-1208`):
```go
// H2.3 Phase 1: pre-split Tier 1 completeness gate.
// When pos AND all siblings have CloseSize populated, require raw
// exchange-aggregated CloseSize to match the sum of expected sizes.
// Prevents accepting partial exchange data when local PnL was contaminated.
const sizeEpsilon = 1e-6
longSiblings := e.siblingsFor(pos, "long")
shortSiblings := e.siblingsFor(pos, "short")
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
    if longAgg.CloseSize < longExpected-sizeEpsilon || shortAgg.CloseSize < shortExpected-sizeEpsilon {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected)
        return false
    }
}
```

Target (per-leg skip for exchanges that don't expose CloseSize):
```go
// H2.3 Phase 1: pre-split Tier 1 completeness gate.
// When pos AND all siblings have CloseSize populated, require raw
// exchange-aggregated CloseSize to match the sum of expected sizes.
// Prevents accepting partial exchange data when local PnL was contaminated.
//
// Per-leg skip: Binance's GetClosePnL is /fapi/v1/income-based and returns
// CloseSize=0 (income API has no qty field). Enforce the gate only on legs
// whose aggregate reports CloseSize>0. Legs with CloseSize=0 fall through to
// the post-split Tier 2 notional guard.
const sizeEpsilon = 1e-6
longSiblings := e.siblingsFor(pos, "long")
shortSiblings := e.siblingsFor(pos, "short")
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")

    // Per-leg enforcement: only check legs whose aggregate reports size.
    // CloseSize=0 indicates the exchange's close-PnL endpoint does not expose
    // per-close size (currently Binance /fapi/v1/income).
    longCheckApplies := longAgg.CloseSize > 0
    shortCheckApplies := shortAgg.CloseSize > 0

    longIncomplete := longCheckApplies && longAgg.CloseSize < longExpected-sizeEpsilon
    shortIncomplete := shortCheckApplies && shortAgg.CloseSize < shortExpected-sizeEpsilon

    if longIncomplete || shortIncomplete {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f longChecked=%v shortChecked=%v), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected,
            longCheckApplies, shortCheckApplies)
        return false
    }

    // Informational: at least one leg's size check was skipped (exchange does
    // not expose CloseSize). Tier 2 notional guard still runs post-split.
    if !longCheckApplies || !shortCheckApplies {
        e.log.Info("reconcile %s [attempt %d]: Tier 1 skipped on leg(s) with CloseSize=0 (longChecked=%v shortChecked=%v) — Tier 2 will apply",
            pos.ID, attempt, longCheckApplies, shortCheckApplies)
    }
}
```

**Behavior:**
- Non-Binance both legs: unchanged (both checks apply).
- Binance one leg + non-Binance other: Binance leg skipped, other leg enforced.
- Binance both legs: both skipped — Tier 1 block does nothing, flow proceeds to
  split + Tier 2 notional guard. Matches old pre-SIRENUSDT behavior for
  Binance↔Binance (which is not a configuration that actually exists in this
  bot — Binance is one exchange).

**Non-goals / deliberately unchanged:**
- `splitSharedPnL`, `aggregateClosePnLBySide`, Tier 2, Tier 3 are untouched.
- `Binance GetClosePnL` is untouched — fix A deferred to a future plan.
- `tryReconcilePnL` return values and retry semantics are unchanged.

### Change 2: Regression test

Add new test file (or extend `internal/engine/sirenusdt_fixes_test.go`). Preferred
location: new file `internal/engine/reconcile_tier1_per_leg_test.go` to keep the
SIRENUSDT file focused on its fix.

Test cases (table-driven, inline Tier 1 logic test matching the existing
`TestSirenusdtTier1*` style — no DB):

| Case | long CloseSize (agg) | short CloseSize (agg) | long expected | short expected | Expected outcome |
|------|----------------------|------------------------|---------------|----------------|------------------|
| A: both sized, both complete | 100 | 100 | 100 | 100 | gate passes |
| B: both sized, long short | 80 | 100 | 100 | 100 | gate fails (retry) |
| C: both sized, short short | 100 | 80 | 100 | 100 | gate fails (retry) |
| D: long CloseSize=0 (Binance), short complete | 0 | 100 | 100 | 100 | gate passes (long skip) |
| E: long CloseSize=0 (Binance), short short | 0 | 80 | 100 | 100 | gate fails (short enforced) |
| F: short CloseSize=0 (Binance), long complete | 100 | 0 | 100 | 100 | gate passes (short skip) |
| G: both CloseSize=0 | 0 | 0 | 100 | 100 | gate passes (both skip) |

Existing tests MUST continue to pass unchanged:
- `TestSirenusdtTier1Gate` (A pattern).
- `TestSirenusdtTier1MultipleClosers` (sibling-aware variant).
- `TestSirenusdtTier1SiblingMissingCloseSize` (pre-migration sibling fallback).
- `TestSirenusdtTier1PreMigrationNoCloseSize` (pos without CloseSize).

### Change 3: VERSION + CHANGELOG

- `VERSION`: bump from `0.33.3` → `0.33.4`.
- `CHANGELOG.md`: new entry:
  ```
  ## 0.33.4 — 2026-04-24

  ### Fixed
  - Reconcile Tier 1 gate no longer blocks Binance-leg positions. The gate now
    enforces CloseSize completeness per leg; legs whose exchange close-PnL
    endpoint does not expose size (Binance /fapi/v1/income) are skipped
    individually, while the other leg's check still runs. Tier 2 notional guard
    continues to protect the position as a whole. Restores `詳細分解` /
    Detailed breakdown on the trade history page for positions closed with a
    Binance leg.
  ```

## Out of scope

1. **Fix A** (populate `CloseSize` in Binance `GetClosePnL` via `/fapi/v1/userTrades`
   correlated with income `tradeId`). Deferred to a separate plan — requires
   closer-only derivation, side inference per trade, and its own review.
2. **Frontend fix** (switch `hasDecomposition` to use `p.has_reconciled` /
   `p.partial_reconcile`). Codex flagged this as an independent design
   fragility but not required to unblock the current symptom. Deferred — the
   current fix restores the data and the UI inference works again.
3. **Historical backfill** for already-closed Binance-leg positions that missed
   reconciliation. Those will stay on fallback text; no retroactive fix here.

## Rollout

1. Merge to main, bump VERSION, update CHANGELOG.
2. Release binary to GitHub (per `feedback_always_release.md`), then VPS `/update`
   pull.
3. Verify on VPS: next Binance-leg close should reconcile successfully and show
   full breakdown on dashboard.
4. Monitor for one full cycle (≥1h) before declaring success.

## Risk assessment

- **Blast radius**: tiny. Single function in `exit.go`, ~15-line change, no data
  migration, no schema change.
- **Rollback**: revert commit; no persistent state affected.
- **Correctness risk**: low — skipping a check on a leg whose exchange does not
  expose size is strictly more permissive than today; no new false positives
  possible. Tier 2 notional guard still catches catastrophic corruption.
- **SIRENUSDT regression risk**: zero for pure non-Binance pairs. For mixed
  Binance-leg pairs, the exposed surface is strictly the Binance leg's Tier 1
  protection, which is already ineffective today (it always fires a false
  "incomplete" warning). No real protection is lost.

## Acceptance

- Build passes (`go build ./...`).
- All existing tests green, including SIRENUSDT tests.
- New table-driven regression test covers cases A-G above.
- After deploy, a Binance-leg position closing produces
  `UpdateHistoryEntry succeeded` in logs and the trade-history card shows
  `進場利差 / 已收資金費 / 多方價格 / 空方價格` plus the per-leg breakdown
  (Fees / Funding / ClosePnL / Subtotal / TotalPnL / APR).
