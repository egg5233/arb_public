# Plan: Merge Rebalance Transfers — Map Accumulate + Batch Execute (v6)

## Problem

同 donor→recipient 分多筆 withdraw，多付手續費。

## Solution

Inner loop 不呼叫 Withdraw，改為累加到 `map[donor→recipient]`。迴圈結束後合併同方向，一次 Withdraw。

## Known Tradeoffs (已知且可接受)

1. **Fee overcharge in surplus**: 迴圈中每次 `surplus -= netAmount + fee`，但合併後只付一次 fee。Surplus 被低估 → donor 容量被保守估計 → **安全**（寧可少轉不會多轉）
2. **Spot balance tracking**: 迴圈中 `bi.spot -= (netAmount + fee - movedToSpot)` 假設 withdraw 已發生，但實際 money 還在 spot。後續迭代看到的 spot 偏低 → 可能觸發不必要的 futures→spot → **安全**（futures→spot 免費）
3. **Failed merged withdraw 無法 retry 不同 donor**: 合併後一筆失敗 = 整段失敗 + rollback。比分筆失敗少了 "換 donor 重試" 的機會 → **可接受**（金額小、失敗率低、rollback 恢復狀態）
5. **Donor failure discovered late**: 如果 donor withdraw 在批次執行時才失敗，迴圈已結束無法改選 donor。但每個 donor→recipient 是獨立的 key，一個失敗不影響其他 → **可接受**（跟 issue 3 同理）
4. **dryRun 不同步**: dryRun path 不合併 → 計畫和執行的 log 數量不同 → **可接受**（dryRun 只影響 log，不影響實際轉帳）

## Changes

**File**: `internal/engine/allocator.go`

### 1. 宣告 batch map（~line 1252，迴圈前）

```go
type batchedWd struct {
    donor, recipient string
    chain, destAddr  string
    netTotal         float64
    fee              float64
    isGross          bool
    movedToSpot      float64
}
batchedWds := map[string]*batchedWd{} // key: "donor->recipient"
```

### 2. Inner loop：替換 Withdraw 為累加（~line 1490-1542）

把 line 1490-1542（Withdraw call + error handling + recordTransfer + pendingDeposits + surplus/balance update）替換為：

```go
// Accumulate into batch instead of immediate withdraw
wdKey := bestDonor + "->" + exchName
if bw, ok := batchedWds[wdKey]; ok {
    bw.netTotal += netAmount
    bw.movedToSpot += movedToSpot
} else {
    batchedWds[wdKey] = &batchedWd{
        donor: bestDonor, recipient: exchName,
        chain: chain, destAddr: destAddr,
        netTotal: netAmount, fee: fee,
        isGross: isGross, movedToSpot: movedToSpot,
    }
}

// Keep surplus/balance updates for correct donor selection in subsequent iterations
surplus[bestDonor] -= netAmount + fee
bi := balances[bestDonor]
bi.futures -= movedToSpot
bi.spot -= (netAmount + fee - movedToSpot)
if bi.spot < 0 {
    bi.spot = 0
}
balances[bestDonor] = bi
remaining -= netAmount
```

注意：`recordTransfer`、`pendingDeposits`、`pendingStartBal` 移到 Step 3。

### 3. 迴圈後：批次執行（在 deposit polling 前，~line 1546）

```go
// Execute batched withdrawals in deterministic order (sorted by key)
batchKeys := make([]string, 0, len(batchedWds))
for k := range batchedWds {
    batchKeys = append(batchKeys, k)
}
sort.Strings(batchKeys)
for _, bk := range batchKeys {
    bw := batchedWds[bk]
    withdrawAmtForAPI := bw.netTotal
    if bw.isGross {
        withdrawAmtForAPI = bw.netTotal + bw.fee
    }
    amtStr := fmt.Sprintf("%.4f", withdrawAmtForAPI)
    e.log.Info("rebalance: batched withdraw %s->%s net=%.2f fee=%.4f amount=%.2f via %s",
        bw.donor, bw.recipient, bw.netTotal, bw.fee, withdrawAmtForAPI, bw.chain)

    result, err := e.exchanges[bw.donor].Withdraw(exchange.WithdrawParams{
        Coin:    "USDT",
        Chain:   bw.chain,
        Address: bw.destAddr,
        Amount:  amtStr,
    })
    if err != nil {
        e.log.Error("rebalance: batched withdraw from %s failed: %v", bw.donor, err)
        if bw.movedToSpot > 0 {
            rollbackStr := fmt.Sprintf("%.4f", bw.movedToSpot)
            e.log.Info("rebalance: rollback %s spot->futures %s USDT", bw.donor, rollbackStr)
            if bw.donor == "bingx" {
                time.Sleep(2 * time.Second)
            }
            if rbErr := e.exchanges[bw.donor].TransferToFutures("USDT", rollbackStr); rbErr != nil {
                e.log.Error("rebalance: rollback failed: %v", rbErr)
            }
        }
        continue
    }

    recipientReceives := bw.netTotal
    if bw.isGross {
        recipientReceives = withdrawAmtForAPI - bw.fee
    }
    e.log.Info("rebalance: withdraw from %s txid=%s (apiAmt=%.2f, recipient=%.2f, fee=%.4f, gross=%v) -> %s",
        bw.donor, result.TxID, withdrawAmtForAPI, recipientReceives, bw.fee, bw.isGross, bw.recipient)
    e.recordTransfer(bw.donor, bw.recipient, "USDT", bw.chain, amtStr, result.Fee, result.TxID, "completed", "rebalance")

    if _, exists := pendingStartBal[bw.recipient]; !exists {
        if uc, ok := e.exchanges[bw.recipient].(interface{ IsUnified() bool }); ok && uc.IsUnified() {
            if fb, err := e.exchanges[bw.recipient].GetFuturesBalance(); err == nil {
                pendingStartBal[bw.recipient] = fb.Available
            } else {
                pendingStartBal[bw.recipient] = balances[bw.recipient].spot
            }
        } else {
            pendingStartBal[bw.recipient] = balances[bw.recipient].spot
        }
    }
    pendingDeposits[bw.recipient] += recipientReceives
}
```

## Files Changed

| File | Change |
|------|--------|
| `internal/engine/allocator.go` | ~line 1252: 加 batchedWds map；~line 1490-1542: 替換為累加；~line 1546 前: 批次執行 |

## Example

Before: bingx→bitget 15.19 + 5.00 = 2 次 withdraw, 0.02 APT fee
After: bingx→bitget 20.17 = 1 次 withdraw, 0.01 APT fee
