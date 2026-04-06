# 孤兒條件單清理計劃 v2

> Status: **PLAN — 等審查通過後實施**
> v2: 根據 Codex 審查 + adapter 代碼 + doc/ API 文件修正

## 問題
Bot 平倉後沒有取消該 symbol 的 algo/conditional orders（TP/SL），導致：
1. 殘留條件單阻擋 SetMarginMode（Binance -4067 錯誤）
2. 條件單可能在未來錯誤觸發

## 修復方案
新增 `CancelAllOrders(symbol string) error` 到 Exchange interface，6 個 adapter 全部實作。在平倉完成後呼叫清理。

## Exchange interface (pkg/exchange/exchange.go)
```go
// CancelAllOrders cancels all open orders (regular + conditional/algo) for a symbol.
CancelAllOrders(symbol string) error
```

## 各 adapter 實作（6 個，含 Bybit）

### Binance (pkg/exchange/binance/adapter.go)
- client 有 `Delete` 方法 (client.go:148)
- 取消普通: `DELETE /fapi/v1/allOpenOrders` params: `{"symbol": symbol}`
- 取消 algo: `DELETE /fapi/v1/algoOpenOrders` params: `{"symbol": symbol}`
- doc 參考: doc/binance/binance-usds-futures-api-docs.md L7149, L7194
```go
func (b *Adapter) CancelAllOrders(symbol string) error {
    b.client.Delete("/fapi/v1/allOpenOrders", map[string]string{"symbol": symbol})
    b.client.Delete("/fapi/v1/algoOpenOrders", map[string]string{"symbol": symbol})
    return nil // best-effort, errors logged inside client
}
```

### BingX (pkg/exchange/bingx/adapter.go) — Codex APPROVE
- client 有 `Delete` 方法 (client.go:126)
- symbol 轉換: `toBingXSymbol` (adapter.go:132)
- 取消所有（含 STOP_MARKET/TAKE_PROFIT_MARKET): `DELETE /openApi/swap/v2/trade/allOpenOrders` params: `{"symbol": bxSym}`
- doc 參考: doc/bingx/bingx-swap-api-docs.md L604
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
    bxSym := toBingXSymbol(symbol)
    a.client.Delete("/openApi/swap/v2/trade/allOpenOrders", map[string]string{"symbol": bxSym})
    return nil
}
```

### Bitget (pkg/exchange/bitget/adapter.go)
- client.Post 接受 `map[string]string` (client.go:90)
- symbol 直接使用，無轉換函數 (adapter.go:106)
- 取消 plan orders: `POST /api/v2/mix/order/cancel-plan-order` body: `{"symbol": symbol, "productType": "USDT-FUTURES"}`
- 取消普通: `POST /api/v2/mix/order/batch-cancel-orders` body: `{"symbol": symbol, "productType": "USDT-FUTURES"}`
- doc 參考: doc/bitget/bitget-futures-api-docs.md L2353, L2396
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
    a.client.Post("/api/v2/mix/order/cancel-plan-order", map[string]string{
        "symbol": symbol, "productType": "USDT-FUTURES",
    })
    a.client.Post("/api/v2/mix/order/batch-cancel-orders", map[string]string{
        "symbol": symbol, "productType": "USDT-FUTURES",
    })
    return nil
}
```

### Gate.io (pkg/exchange/gateio/adapter.go)
- client baseURL 已含 `/api/v4` (client.go:18: `defaultBaseURL = "https://api.gateio.ws/api/v4"`)
- **路徑不要再加 /api/v4**，直接用 `/futures/usdt/...`
- client 有 `Delete` 方法 (client.go:111)
- symbol 轉換: `toGateSymbol` (adapter.go:122) → e.g. "PLAY_USDT"
- 取消條件單: `DELETE /futures/usdt/price_orders` params: `{"contract": gtSym}`
- 取消普通: `DELETE /futures/usdt/orders` params: `{"contract": gtSym}`
- doc 參考: doc/gate/gate-perpetual-futures-api-docs.md L7901
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
    gtSym := toGateSymbol(symbol)
    a.client.Delete("/futures/usdt/price_orders", map[string]string{"contract": gtSym})
    a.client.Delete("/futures/usdt/orders", map[string]string{"contract": gtSym})
    return nil
}
```

### OKX (pkg/exchange/okx/adapter.go)
- symbol 轉換: `toOKXInstID` (adapter.go:183) → e.g. "BTC-USDT-SWAP"
- 需 query-then-cancel（沒有 cancel-all-by-symbol for algo）
- 查 algo: `GET /api/v5/trade/orders-algo-pending` params: `instId, ordType=conditional`
- 取消 algo: `POST /api/v5/trade/cancel-algos` body: `[{algoId, instId}]`（最多 10 個）
- 取消普通: 先查 `GET /api/v5/trade/orders-pending` params: `instId, instType=SWAP`，再逐個取消
- doc 參考: doc/okx/okx-order-book-trading-api-docs.md L5694
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
    instID := toOKXInstID(symbol)
    
    // 1. Cancel algo/conditional orders (TP/SL)
    // Existing adapter pattern: CancelStopLoss (adapter.go:1467) already uses
    // POST /api/v5/trade/cancel-algos with []map[string]interface{}{{instId, algoId}}
    // We query pending algos, then batch cancel.
    algoData, err := a.client.Get("/api/v5/trade/orders-algo-pending", map[string]string{
        "instId": instID, "ordType": "conditional",
    })
    if err == nil {
        var algoResp []struct {
            AlgoID string `json:"algoId"`
            InstID string `json:"instId"`
        }
        if json.Unmarshal(algoData, &algoResp) == nil && len(algoResp) > 0 {
            // Batch cancel (max 10 per request, matching OKX doc)
            for i := 0; i < len(algoResp); i += 10 {
                end := i + 10
                if end > len(algoResp) {
                    end = len(algoResp)
                }
                batch := make([]map[string]interface{}, 0, end-i)
                for _, algo := range algoResp[i:end] {
                    batch = append(batch, map[string]interface{}{
                        "instId": algo.InstID,
                        "algoId": algo.AlgoID,
                    })
                }
                a.client.Post("/api/v5/trade/cancel-algos", batch) // best-effort
            }
        }
    }
    
    // 2. Cancel regular pending orders
    ordData, err := a.client.Get("/api/v5/trade/orders-pending", map[string]string{
        "instId": instID, "instType": "SWAP",
    })
    if err == nil {
        var ordResp []struct {
            OrdID  string `json:"ordId"`
            InstID string `json:"instId"`
        }
        if json.Unmarshal(ordData, &ordResp) == nil {
            for _, ord := range ordResp {
                a.client.Post("/api/v5/trade/cancel-order", map[string]interface{}{
                    "instId": ord.InstID,
                    "ordId":  ord.OrdID,
                }) // best-effort
            }
        }
    }
    
    return nil
}
```

### Bybit (pkg/exchange/bybit/adapter.go) — 新增
- client **沒有 Delete 方法**，只有 Get 和 Post (client.go:101,106)
- Bybit cancel-all 用 POST（不是 DELETE）: `POST /v5/order/cancel-all`
- params: `{"category": "linear", "symbol": symbol}`
- orderFilter 可選: "Order"=普通, "StopOrder"=條件單, 不傳=全部
- doc 參考: doc/bybit/bybit-order-api-docs.md L943
```go
func (a *Adapter) CancelAllOrders(symbol string) error {
    a.client.Post("/v5/order/cancel-all", map[string]string{
        "category": "linear", "symbol": symbol,
    })
    return nil
}
```

## 呼叫位置

根據 Codex 審查，正確的 terminal close paths：

| 位置 | 路徑 | 說明 |
|------|------|------|
| exit.go ~L745 | depth exit 完成 | 主要平倉路徑 — `AddToHistory` (L734) + log "depth-exit closed" (L745) 之後，兩邊交易所都 cancel |
| exit.go ~L1575 | closePositionV2 完成 | smart/emergency exit 路徑 — `AddToHistory` 之後 |
| exit.go ~L2192 | rotation 舊腿平倉後 | `closeFullyWithRetry` (L2192) 完成後，只對 oldExch（被替換的交易所）cancel |
| consolidate.go ~L498 | markPositionClosed 內 | `AddToHistory` 之後，清理缺腿倉位 |
| consolidate.go ~L241 | untracked orphan close | `closeFullyWithRetry` (L241) 完成後，加 `exch.CancelAllOrders(ep.Symbol)` |

**不加入 entry rollback (engine.go cleanupFailedPosition L2368)**: 該函數只在 `LongSize==0 && ShortSize==0` 時執行（開倉完全失敗），此時不會有 SL/TP 掛單殘留。

**Rotation 注意**: exit.go L2391-2427 已用 `CancelStopLoss`/`CancelTakeProfit` 按 orderID 取消舊 TP/SL。但 Binance algo orders 用不同的 algoId 命名空間，可能不被 CancelStopLoss 覆蓋。所以 L2200 還需呼叫 `CancelAllOrders` 補漏。

**Rotation 注意**: exit.go L2391-2427 已用 `CancelStopLoss`/`CancelTakeProfit` 按 orderID 取消舊 TP/SL。但 algo orders (如 Binance conditional) 可能不走同一個 cancel path。Rotation 後也應呼叫 CancelAllOrders 清理舊 exchange 上的殘留 algo orders。

**呼叫方式**: goroutine（不阻塞主流程），兩邊交易所都要清
```go
go func() {
    if err := longExch.CancelAllOrders(symbol); err != nil {
        e.log.Warn("cancel orphan orders on %s failed: %v", longExchName, err)
    }
    if err := shortExch.CancelAllOrders(symbol); err != nil {
        e.log.Warn("cancel orphan orders on %s failed: %v", shortExchName, err)
    }
}()
```

## Binance -4067 暫時處理

**不在 SetMarginMode 加 -4067 return nil**（Codex 指出不安全）。
改為：在 SetMarginMode 遇到 -4067 時，先呼叫 CancelAllOrders 清理，然後重試一次：
```go
func (b *Adapter) SetMarginMode(symbol string, mode string) error {
    _, err := b.client.Post("/fapi/v1/marginType", params)
    if err != nil {
        if isAPIError(err, -4046) {
            return nil // already set
        }
        if isAPIError(err, -4067) {
            // Has open orders blocking change — cancel and retry
            b.CancelAllOrders(symbol)
            _, err = b.client.Post("/fapi/v1/marginType", params)
            if err != nil && isAPIError(err, -4046) {
                return nil
            }
            return err
        }
        return fmt.Errorf("SetMarginMode: %w", err)
    }
    return nil
}
```

## 風險
- 低 — CancelAllOrders 在平倉後才跑，用 goroutine 不阻塞
- 各 API call 失敗只 warn
- Binance -4067 retry 比直接 return nil 更安全
