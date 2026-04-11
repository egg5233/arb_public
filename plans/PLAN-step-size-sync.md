# Plan: Step Size Strict Synchronization (v9)

## Problem

第一腿部分成交非 common step 倍數時，第二腿無法匹配。

## Solution

1. Entry：粗步長交易所先下單（已實作 ✓）
2. Entry/Exit 第二腿：firstFilled 不 common-tradeable → 補齊第一腿到下一個 common step 倍數
3. 補齊失敗 → rollback(entry) / break-to-market-fallback(exit)

## Changes

### Step 1: Add `RoundUpToStep` helper

**File**: `pkg/utils/math.go`

```go
// RoundUpToStep rounds size UP to the nearest multiple of step.
// If size is already a multiple (within float epsilon), returns size unchanged.
func RoundUpToStep(size, step float64) float64 {
    if step <= 0 {
        return size
    }
    n := size / step
    rounded := math.Ceil(n)
    // Guard float noise: if n is very close to an integer, don't jump up
    if math.Abs(n-math.Round(n)) < 1e-9 {
        rounded = math.Round(n)
    }
    return rounded * step
}
```

### Step 2: Entry — 補齊邏輯

**File**: `internal/engine/engine.go`

**2a. Short-first 分支，第二腿（現有 matchSize <= 0 rollback 的位置）：**

```go
matchSize := e.commonTradeableSize(opp.LongExchange, opp.ShortExchange, opp.Symbol, shortFilled)
if matchSize < shortFilled {
    // Top up first leg to next common step
    ceilSize := utils.RoundUpToStep(shortFilled, stepSize)
    // Verify ceilSize is common-tradeable; if not, search upward.
    // Use epsilon comparison to avoid float mismatch between RoundUpToStep and commonTradeableSize.
    for i := 0; i < 10 && ceilSize > 0; i++ {
        cts := e.commonTradeableSize(opp.LongExchange, opp.ShortExchange, opp.Symbol, ceilSize)
        if cts > 0 && math.Abs(cts-ceilSize) < stepSize*0.01 {
            ceilSize = cts // use the canonical common-tradeable value
            break
        }
        ceilSize += stepSize
    }

    // Guard: don't overshoot remaining target.
    topUpOK := ceilSize <= remaining
    if topUpOK {
        topUp := ceilSize - shortFilled
        topUpStr := e.formatSize(opp.ShortExchange, opp.Symbol, topUp)
        // Guard: formatSize may round to "0"
        if parsed, _ := strconv.ParseFloat(topUpStr, 64); parsed <= 0 {
            topUpOK = false
        } else {
            topUpOID, topUpErr := shortExch.PlaceOrder(exchange.PlaceOrderParams{
                Symbol:    opp.Symbol,
                Side:      exchange.SideSell,
                OrderType: "limit",
                Price:     shortPriceStr,
                Size:      topUpStr,
                Force:     "ioc",
            })
            if topUpErr == nil {
                // Use confirmFillSafe (consistent with first-leg pattern)
                topUpFilled, topUpAvg, topUpCFErr := e.confirmFillSafe(shortExch, topUpOID, opp.Symbol)
                if topUpCFErr != nil {
                    e.log.Warn("depth tick: top-up fill state unknown: %v", topUpCFErr)
                    _ = shortExch.CancelOrder(opp.Symbol, topUpOID)
                    // Check actual exchange position to detect real partial fill.
                    exchSize, exchErr := getExchangePositionSize(shortExch, opp.Symbol, "short")
                    priorTotal := confirmedShort + shortFilled
                    if exchErr == nil && exchSize > priorTotal {
                        actualTopUp := exchSize - priorTotal
                        if actualTopUp > 0 {
                            shortFilled += actualTopUp
                            e.log.Warn("depth tick: top-up unknown but exchange shows %.6f (prior=%.6f), topUp=%.6f",
                                exchSize, priorTotal, actualTopUp)
                            matchSize = e.commonTradeableSize(opp.LongExchange, opp.ShortExchange, opp.Symbol, shortFilled)
                        }
                    }
                } else if topUpFilled > 0 {
                    // VWAP: compute weighted average BEFORE updating shortFilled
                    if topUpAvg > 0 && shortAvg > 0 {
                        shortAvg = (shortAvg*shortFilled + topUpAvg*topUpFilled) / (shortFilled + topUpFilled)
                    } else if topUpAvg > 0 {
                        shortAvg = topUpAvg
                    }
                    shortFilled += topUpFilled
                    matchSize = e.commonTradeableSize(opp.LongExchange, opp.ShortExchange, opp.Symbol, shortFilled)
                    e.log.Info("depth tick: topped up %s by %.6f → total %.6f, matchSize=%.6f",
                        opp.Symbol, topUpFilled, shortFilled, matchSize)
                }
            } else {
                e.log.Warn("depth tick: top-up order failed: %v", topUpErr)
            }
        }
    }

    if matchSize < shortFilled {
        // Still can't match (matchSize=0 or partial) — rollback as last resort
        e.log.Warn("depth tick: top-up failed (matchSize=%.6f < shortFilled=%.6f), rolling back on %s",
            matchSize, shortFilled, opp.ShortExchange)
        rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, shortFilled)
        if rem > 0 {
            e.log.Error("ORPHAN EXPOSURE: %s rollback incomplete, %.6f on %s", opp.Symbol, rem, opp.ShortExchange)
            e.telegram.Send("⚠️ ORPHAN EXPOSURE: %s rollback incomplete, %.6f on %s", opp.Symbol, rem, opp.ShortExchange)
            shortFilled = rem
            longFilled = 0
            abortFillLoop = true
        } else {
            shortConsecFails++
            continue
        }
    }
}
// matchSize > 0 here — proceed with second leg
longSizeStr := e.formatSize(opp.LongExchange, opp.Symbol, matchSize)
```

retrySecondLeg 呼叫傳 matchSize（不變，已在 v7.1 實作）。

**2b. Long-first 分支：** 同樣邏輯，方向相反（top up long leg with BUY）。

### Step 3: Exit — 補齊邏輯

**File**: `internal/engine/exit.go`

**3a. LongFirst 分支，第二腿（現有 break 的位置）：**

```go
matchSize := e.commonTradeableSize(pos.LongExchange, pos.ShortExchange, pos.Symbol, firstFilled)
if matchSize < firstFilled {
    // Top up: close more on the first-leg exchange
    ceilSize := utils.RoundUpToStep(firstFilled, exitStepSize)
    // Verify ceilSize is common-tradeable; if not, search upward.
    for i := 0; i < 10 && ceilSize > 0; i++ {
        cts := e.commonTradeableSize(pos.LongExchange, pos.ShortExchange, pos.Symbol, ceilSize)
        if cts > 0 && math.Abs(cts-ceilSize) < exitStepSize*0.01 {
            ceilSize = cts
            break
        }
        ceilSize += exitStepSize
    }

    // Guard: don't over-close past what's left
    longRemAfterFirst := totalLong - closedLong
    shortRemaining := totalShort - closedShort
    // Cap: don't top up past the paired (short) leg's remaining either
    topUpOK := ceilSize-firstFilled <= longRemAfterFirst && ceilSize <= shortRemaining
    if topUpOK {
        topUp := ceilSize - firstFilled
        topUpStr := e.formatSize(pos.LongExchange, pos.Symbol, topUp)
        if parsed, _ := strconv.ParseFloat(topUpStr, 64); parsed <= 0 {
            topUpOK = false
        } else {
            topUpOID, topUpErr := longExch.PlaceOrder(exchange.PlaceOrderParams{
                Symbol:     pos.Symbol,
                Side:       exchange.SideSell,
                OrderType:  "limit",
                Price:      sellPrice,
                Size:       topUpStr,
                Force:      "ioc",
                ReduceOnly: true,
            })
            if topUpErr == nil {
                topUpFilled, topUpAvg := e.confirmFill(longExch, topUpOID, pos.Symbol)
                if topUpFilled > 0 {
                    if topUpAvg > 0 {
                        longVWAPSum += topUpAvg * topUpFilled
                    }
                    firstFilled += topUpFilled
                    closedLong += topUpFilled
                    matchSize = e.commonTradeableSize(pos.LongExchange, pos.ShortExchange, pos.Symbol, firstFilled)
                    e.log.Info("depth exit %s: topped up long close by %.6f → total %.6f, matchSize=%.6f",
                        pos.ID, topUpFilled, firstFilled, matchSize)
                }
            } else {
                e.log.Warn("depth exit %s: top-up order failed: %v", pos.ID, topUpErr)
            }
        }
    }

    if matchSize <= 0 || matchSize < firstFilled {
        e.log.Warn("depth exit %s: top-up failed, breaking to market fallback", pos.ID)
        break
    }
}
buySize := e.formatSize(pos.ShortExchange, pos.Symbol, matchSize)
```

Exit VWAP 直接加入 `longVWAPSum += topUpAvg * topUpFilled`，跟現有的 VWAP 累加方式一致（line 593-594）。不需要重新算加權平均，因為 exit 最後用 `longVWAPSum / closedLong` 算（line 781）。

**3b. ShortFirst 分支：** 同樣邏輯（top up short close with BUY, reduce-only）。

## Files Changed

| File | Change |
|------|--------|
| `pkg/utils/math.go` | Add `RoundUpToStep()` |
| `internal/engine/engine.go` | Entry 第二腿：先補齊，失敗才 rollback |
| `internal/engine/exit.go` | Exit 第二腿：先補齊，失敗才 break |

## Codex v8+v9 Issues Addressed

| Issue | Fix |
|-------|-----|
| ceilSize > remaining overshoot | Entry: `if ceilSize > remaining` → round down. Exit: check `longRemAfterFirst` |
| confirmFill vs confirmFillSafe | Entry top-up 用 `confirmFillSafe`（一致） |
| confirmFillSafe unknown 未處理 partial | Unknown 時查 exchange position，偵測實際成交並更新 shortFilled |
| VWAP double-count | Entry: 先算加權平均再更新 shortFilled。Exit: 用 `longVWAPSum +=` 累加（跟現有一致） |
| topUp formatSize → "0" | `ParseFloat` 檢查，0 → `topUpOK = false`，跳過下單 |
| RoundUpToStep float noise | epsilon guard: `Abs(n-Round(n)) < 1e-9` → 不多跳一步 |
| Entry rollback 條件不完整 | `matchSize <= 0` → `matchSize < shortFilled`（跟 exit 一致） |
