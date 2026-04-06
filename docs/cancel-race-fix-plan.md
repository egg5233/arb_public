# CancelAllOrders Race Condition 修復計劃

> Status: **PLAN — 等審查通過後實施**

## 問題
CancelAllOrders 用 goroutine 異步執行，在 SavePosition(StatusClosed) 之後。
如果 ReEnterCooldownHours=0，存在 race window：
1. Position 標記 closed → symbol 可用
2. 新入場掃描開倉 + 設 TP/SL
3. 異步 goroutine 才執行 CancelAllOrders → 把新倉的 TP/SL 刪掉

## 修復
把 CancelAllOrders 改為**同步**，在 SavePosition(StatusClosed) **之前**執行。
cancel 完成後才標記 closed → 新入場才看到 symbol 可用 → 不會 race。

## 改動位置（5 處）

### Site 1: exit.go depth exit (~L749)
```go
// BEFORE (異步, 在 AddToHistory 之後):
go func(sym string, le, se exchange.Exchange) {
    le.CancelAllOrders(sym)
    se.CancelAllOrders(sym)
}(pos.Symbol, longExch, shortExch)

// AFTER (同步, 在 AddToHistory 之前):
longExch.CancelAllOrders(pos.Symbol)
shortExch.CancelAllOrders(pos.Symbol)
// ... then AddToHistory, SavePosition, etc.
```

### Site 2: exit.go closePositionV2 (~L1604)
同理，改為同步，在 AddToHistory 之前。

### Site 3: exit.go rotation (~L2210)
Rotation 特殊 — position 不 close，只換腿。沒有 race（symbol 一直被佔用）。
**保持 goroutine** — 不阻塞 rotation 流程，且 symbol 仍被當前 position 佔用。

### Site 4: consolidate.go markPositionClosed (~L514)
同理，改為同步，在 AddToHistory 之前。

### Site 5: consolidate.go orphan close (~L245)
同理，改為同步，在 closeFullyWithRetry 之後立即執行。

## 總結

| Site | 改動 | 原因 |
|------|------|------|
| 1 depth exit | goroutine → 同步, 移到 UpdatePositionFields(StatusClosed) 前 (~L700) | 防 race — UpdatePositionFields 內部呼叫 SavePosition → SRem keyPositionsActive |
| 2 closePositionV2 | goroutine → 同步, 移到 SavePosition 前 (L1577) | 防 race — SavePosition 釋放 symbol |
| 3 rotation | 保持 goroutine | symbol 被佔用，無 race |
| 4 consolidate close | goroutine → 同步, 移到 **SavePosition 前** (L496) | 防 race — SavePosition 會 SRem keyPositionsActive，那才是 symbol 釋放點 |
| 5 orphan close | goroutine → 同步 | 防 race |

## 性能影響
- CancelAllOrders 是 HTTP call，同步會增加平倉延遲
- 正常情況 200-500ms，最差情況 30-45s（OKX query-then-cancel + 另一個交易所都慢）
- 可接受 — 平倉不是 latency-sensitive 操作
- Rotation 保持異步因為它需要快速進入新腿

## 風險
- 低 — 只改 goroutine 為同步，不改邏輯
- CancelAllOrders 失敗不影響平倉流程（return nil）
