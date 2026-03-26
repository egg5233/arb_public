# Adaptive Position Sizing Based on Orderbook Liquidity

## Problem

In `Approve()` (`internal/risk/manager.go`), slippage is checked against a fixed position size computed by `CalculateSize()`. If slippage exceeds `SlippageBPS` (default 5.0 bps), the opportunity is **rejected outright**. For thin orderbooks (small-cap coins), even $100 notional causes excessive slippage, but a smaller size ($30-70) would fit within the limit.

Example from production logs:
```
KATUSDT: slippage too high on bitget: 7.4 bps > 5.0 bps limit  (size=6800, ~$100)
KATUSDT: slippage too high on okx: 5.7 bps > 5.0 bps limit    (size=6759, ~$100)
```

## Solution

Binary search on position size to find the **largest tradeable size** that fits within the slippage limit. Range: `[$10 floor, original size]`. If even $10 exceeds slippage, reject.

## Algorithm: Binary Search

Why binary search:
- `estimateSlippageFromLevels` is **monotonically non-decreasing** in size (verified: more size → deeper levels consumed → higher VWAP deviation)
- No closed-form inverse (piecewise walk over discrete price levels)
- Converges in ~10 iterations (log2 of range), each iteration is O(depth=20) — negligible cost

### Pseudocode

```
findMaxSizeForSlippage(askLevels, bidLevels, longMid, shortMid, maxSize, minSize, slippageBPS, stepSize):

  // Fast path: original size already fits → no regression for liquid pairs
  if slippage(asks, maxSize, longMid) <= limit AND slippage(bids, maxSize, shortMid) <= limit:
    return maxSize

  // Floor check: even minimum is too much → reject
  if slippage(asks, minSize, longMid) > limit OR slippage(bids, minSize, shortMid) > limit:
    return 0

  // Binary search
  lo = minSize, hi = maxSize
  for up to 20 iterations while hi - lo > stepSize:
    mid = RoundToStep((lo + hi) / 2, stepSize)

    // Oscillation guard: if mid rounds down to lo, bump up
    if mid <= lo: mid = lo + stepSize
    if mid >= hi: break

    worstSlippage = max(slippage(asks, mid, longMid), slippage(bids, mid, shortMid))
    if worstSlippage <= limit: lo = mid   // can go bigger
    else: hi = mid                         // must go smaller

  return RoundToStep(lo, stepSize)  // lo is always a valid (passing) size
```

## Detailed Changes

### File: `internal/risk/manager.go` (only file modified)

#### 1. New standalone function: `findMaxSizeForSlippage`

```go
func findMaxSizeForSlippage(
    askLevels []exchange.PriceLevel,   // long side orderbook
    bidLevels []exchange.PriceLevel,   // short side orderbook
    longMid float64,                   // mid price on long exchange
    shortMid float64,                  // mid price on short exchange
    maxSize float64,                   // original size from CalculateSize()
    minSize float64,                   // floor size (from $10 capital floor)
    slippageLimit float64,             // SlippageBPS config value
    stepSize float64,                  // contract step size
) float64
```

- Standalone (not a method) — pure function, easier to unit test
- Insert after `estimateSlippageFromLevels` (~line 392)

#### 2. Modify `Approve()` — replace lines 124-138 (slippage check block)

Current flow (reject on failure):
```
line 125: longSlippage := estimateSlippageFromLevels(longOB.Asks, size, midPrice)
line 126: if longSlippage > limit → REJECT
line 135: shortSlippage := estimateSlippageFromLevels(shortOB.Bids, size, shortMid)
line 136: if shortSlippage > limit → REJECT
```

New flow:
```
1. Compute longSlippage and shortSlippage (same as before)
2. If BOTH within limit → proceed unchanged (fast path, zero regression)
3. If either exceeds limit:
   a. Load contract info from BOTH exchanges (one LoadAllContracts call each)
   b. Use max(longStepSize, shortStepSize) and max(longMinSize, shortMinSize)
   c. Compute minSizeInBase = max(minCapitalFloor * leverage / midPrice, contractMinSize)
   d. Round minSizeInBase UP to step alignment
   e. Call findMaxSizeForSlippage(...)
   f. If returns 0 → reject (name bottleneck exchange in message)
   g. If returns reduced size → update `size` variable, log reduction
4. Recalculate requiredMarginPerLeg = (size * midPrice) / leverage
5. Continue to VWAP gap check (lines 140-173) — uses reduced `size` automatically
```

#### 3. Recalculate `requiredMarginPerLeg` after size reduction

The exposure cap check at lines 181-182 uses `requiredMarginPerLeg`. Without recalculation, it uses the original (larger) value, causing unnecessary rejections.

```go
// After adaptive sizing reduces `size`:
if sizeReduced {
    requiredMarginPerLeg = (size * midPrice) / float64(leverage)
}
```

### No changes to:
- `internal/config/config.go` — no new config fields
- `internal/models/interfaces.go` — `RiskApproval` struct unchanged
- `internal/risk/limits.go` — `minCapitalFloor` ($10) used as-is
- Any other files

## Edge Cases

| Case | Handling |
|------|----------|
| Liquid pair (slippage OK at full size) | Fast path returns immediately, zero overhead |
| `CapitalPerLeg = 0` (auto mode) | Works identically — starts from whatever `CalculateSize` returned |
| Asymmetric liquidity (one exchange deep, one thin) | Binary search checks BOTH sides at each candidate |
| Large `stepSize` relative to range | Oscillation guard: `mid <= lo → mid = lo + stepSize` |
| `stepSize` unknown (LoadAllContracts fails) | Skip adaptive sizing, fall through to original rejection |
| `minSize >= originalSize` | Skip binary search, original check applies |
| `estimateSlippageFromLevels` returns `math.MaxFloat64` | Treated as exceeding limit, binary search narrows downward |
| `RoundToStep` produces 0 | Reject (below minimum) |

## Logging

Size reduced:
```
[adaptive] KATUSDT: size reduced 6800 → 4200 (long=3.1 bps short=4.8 bps, limit=5.0 bps, $100.00 → $61.00 per leg)
```

Minimum size still too much:
```
[adaptive] KATUSDT: rejected — slippage 12.3 bps at minimum size 690 ($10.00) on bitget (limit=5.0 bps)
```

## Verified Assumptions

| Assumption | Status | Evidence |
|------------|--------|----------|
| `estimateSlippageFromLevels` monotonically non-decreasing | Verified | VWAP moves away from mid as more levels consumed |
| `RoundToStep` rounds down | Verified | `math.Floor(value/step) * step` |
| `LoadAllContracts` NOT cached | Verified | All 6 adapters make live API calls every time |
| No other callers of modified code break | Verified | `Approve` callers use `RiskApproval.Size` transparently |
| Margin check (smaller size → less margin) | Verified | Linear relationship: `margin = size * price / leverage` |
| Thread safety | Verified | Pure function, no shared mutable state |

## Performance Impact

- **Liquid pairs**: Zero overhead (fast path check, 2 extra comparisons)
- **Thin orderbooks**: ~10 binary search iterations × 2 `estimateSlippageFromLevels` calls = ~20 O(20) walks = negligible
- **API calls**: +1-2 `LoadAllContracts` calls (only when slippage exceeded). These are uncached live API calls (~50-100ms each). Acceptable since this path was previously a rejection anyway.

## Expected Production Impact

Based on recent logs, these previously-rejected opportunities could now be partially filled:
- KATUSDT (bitget 7.4 bps, okx 5.7 bps) — likely viable at ~60-80% of original size
- Similar thin-orderbook coins that currently get 100% rejection rate
