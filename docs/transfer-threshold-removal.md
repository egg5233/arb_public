# Transfer Threshold Removal Plan

## Background
allocator.go 中有 14 處硬編碼的轉帳門檻（10 USDT 或 1.0 USDT），阻止了小額跨交易所轉帳。
例如 gateio 只需 3 USDT 就能開倉，但 10 USDT 門檻擋住了轉帳。

## 改動清單 — allocator.go（14 處）

### isAllocatorFundingFeasible（feasibility 模擬）
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L303 | `remaining > 10` | `remaining > 0` | 迴圈終止條件 |
| L309 | `netPossible <= 10` | `netPossible <= 0` | 跳過 fee 吃掉全部的 donor |

### findAllocatorDonor
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L343 | `avail <= 10` | `avail <= 0` | 跳過無 surplus 的 donor |

### estimateAllocatorTransferCost（模擬函數，不動真錢）
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L472 | `transferAmt > 10` | `transferAmt > 0` | 加入 crossDeficits |
| L480 | `remaining > 10` | `remaining > 0` | 模擬迴圈 |
| L503 | `netPossible <= 10` | `netPossible <= 0` | 同 L309 |

### findAllocatorDonorWithCache
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L547 | `avail <= 10` | `avail <= 0` | 同 L343 |

### executeRebalanceFundingPlan — bal.futures >= need 分支
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L750 | `transferAmt > 10` | `transferAmt > 0` | spot 不夠時加入 crossDeficits |

### executeRebalanceFundingPlan — bal.futures < need 分支
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L771 | `transferAmt > 10` | `transferAmt > 0` | spot 扣完後剩餘加入 crossDeficits |

### executeRebalanceFundingPlan — cross-exchange 迴圈
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L798 | `remaining > 10` | `remaining > 0` | 迴圈終止條件 |
| L802 | `s <= 10` | `s <= 0` | 跳過無 surplus 的 donor |
| L862 | `contribution < 10` | `contribution <= 0` | net-fee 扣完 fee 後防負數（注意用 <= 不是 <）|
| L968 | `netAmount < 10` | `netAmount <= 0` | 提幣前最後防線，防負數/零金額 |

### deposit 確認後 spot→futures
| 行號 | 原始 | 改成 | 用途 |
|------|------|------|------|
| L1049 | `totalPending < 1.0` | `totalPending <= 0` | deposit 到帳後做 spot→futures |

## 不動的（3 處）
| 行號 | 內容 | 原因 |
|------|------|------|
| L468 | `actualTransfer >= 1.0` | 同交易所 spot→futures 精度控制 |
| L714 | `extra >= 1.0` | 同交易所 ratio-relief 精度控制 |
| L748 | `actualTransfer < 1.0` | 同交易所 spot→futures 精度控制 |

## 不動的（health.go）
| 行號 | 內容 | 原因 |
|------|------|------|
| L394 | `amount < 1.0` | L3 health 轉帳最低限額，非 allocator 路徑 |

## 搭配修復
1. manager.go — dryRun inflation 加 post-trade ratio 觸發條件
2. allocator.go — executeRebalanceFundingPlan continue bug（ratio-relief 失敗時加入 crossDeficits）
3. allocator.go — skipOuterTransfer 移除 binance 硬編碼
4. manager.go — PrefetchCache 加 TransferablePerExchange
5. allocator.go — simulateTransferPlan 窮舉驗證

## 安全性
- 無 infinite loop 風險：所有迴圈有 donor 耗盡退出條件
- 無負數提幣風險：L862 用 `<= 0` 擋住 fee > contribution
- 極小金額提幣：交易所 API 自己會擋不合法的金額
