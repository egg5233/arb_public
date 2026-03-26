# OKX Adapter Audit

**Date**: 2026-03-26
**Files Audited**:
- `pkg/exchange/okx/adapter.go` (1169 lines)
- `pkg/exchange/okx/client.go` (235 lines)
- `pkg/exchange/okx/ws.go` (328 lines)
- `pkg/exchange/okx/ws_private.go` (198 lines)
- `pkg/exchange/okx/adapter_test.go`

**Reference**: `doc/EXCHANGEAPI_OKX.md`

---

## BUGS FOUND

### BUG-1: GetFundingFees uses wrong filter parameter (`subType: "8"` should be `type: "8"`)
**File**: `adapter.go:957`
**Severity**: HIGH — funding fee queries may return wrong or empty results

The adapter queries funding fees with `"subType": "8"`, but according to OKX docs:
- `type: "8"` = Bill type for Funding Fee
- `subType: "173"` = Funding fee expense, `subType: "174"` = Funding fee income

`subType: "8"` is not a valid subType. The correct filter is either `"type": "8"` (to get all funding fee bills) or `"subType": "173,174"` (to get both expense and income). Current code sends a meaningless subType filter.

```go
// CURRENT (wrong):
"subType": "8",

// FIX (option A — filter by bill type):
"type": "8",

// FIX (option B — filter by both funding subtypes):
// Remove subType param entirely and use type instead
```

### BUG-2: GetFundingFees double-unwraps response envelope
**File**: `adapter.go:966-974`
**Severity**: HIGH — unmarshal always fails, function never returns valid data

The client (`client.go:196`) already unwraps the `{code, data}` envelope and returns the raw `data` array. But `GetFundingFees` tries to unmarshal into a struct with a `Data` field:

```go
// body is already: [{balChg: "...", ts: "..."}, ...]
// But code tries to parse it as: {data: [{...}]}
var resp struct {
    Data []struct {
        BalChg string `json:"balChg"`
        TS     string `json:"ts"`
    } `json:"data"`
}
```

A JSON array cannot unmarshal into a struct — this always returns an error. Compare with `GetClosePnL` (line 1003) which correctly unmarshals the array directly:

```go
// FIX:
var records []struct {
    BalChg string `json:"balChg"`
    TS     string `json:"ts"`
}
if err := json.Unmarshal(body, &records); err != nil { ... }
```

### BUG-3: GetClosePnL does not convert closeTotalPos from contracts to base units
**File**: `adapter.go:1021`
**Severity**: MEDIUM — close size reported in contracts, not base units

All other size fields (positions, fills, order qty) are multiplied by ctVal to convert from contracts to base units. But `closeTotalPos` is used raw:

```go
closeSize, _ := strconv.ParseFloat(r.CloseTotalPos, 64)
// Missing: closeSize *= a.getCtVal(symbol)
```

This is inconsistent with `fetchPositions` (line 378), `GetOrderFilledQty` (line 282), `GetUserTrades` (line 930), and WS order updates (ws_private.go:175), which all multiply by ctVal.

### BUG-4: GetPendingOrders does not convert sz from contracts to base units
**File**: `adapter.go:248`
**Severity**: MEDIUM — order size reported in contracts, not base units

```go
out = append(out, exchange.Order{
    ...
    Size: o.Sz,  // raw contracts, not base units
})
```

Should convert: `sz * ctVal` for consistency.

### BUG-5: Orderbook quantities not converted from contracts to base units
**File**: `adapter.go:751` (REST), `ws.go:281` (WS books5)
**Severity**: LOW — may be intentional if engine expects contract quantities for orderbooks

Per OKX docs: "Quantity is in contracts for derivatives". The adapter returns raw contract quantities without ctVal conversion. This is inconsistent with all other size conversions but may be by design if the engine doesn't use orderbook quantities for sizing.

---

## OBSERVATIONS (not bugs, but worth noting)

### OBS-1: GetFundingRate uses fundingTime instead of nextFundingTime
**File**: `adapter.go:554-555`
**Severity**: LOW

The adapter maps `fundingTime` (current settlement time) to `NextFunding`. After settlement occurs, `fundingTime` is in the past. The API also returns `nextFundingTime` which is always the upcoming settlement, but the adapter doesn't read it.

The adapter struct reads `FundingTime` but not `NextFundingTime`:
```go
FundingTime     string `json:"fundingTime"`
// Missing: NextFundingTime string `json:"nextFundingTime"`
```

Depending on when the query runs relative to settlement, `fundingTime` could be stale. However, since the bot operates on a scheduler at T-3min, this is likely fine in practice.

### OBS-2: PlaceOrder does not set posSide
**File**: `adapter.go:150-156`
**Severity**: INFO — correct for net_mode

The adapter forces net_mode via `ensureNetMode()` and `EnsureOneWayMode()`, so not setting `posSide` in orders is correct. In net_mode, `posSide` defaults to `net` and is optional. If the account were ever in long/short mode, orders would fail. The guard via `ensureNetMode()` on startup is appropriate.

### OBS-3: TransferToFutures is a no-op
**File**: `adapter.go:772`
**Severity**: INFO — correct for unified account

OKX uses a unified trading account, so no transfer is needed for futures trading. This is correctly documented in the code comment.

---

## VERIFIED CORRECT

### 1. ctVal (Contract Value) Conversion
- `LoadAllContracts()` fetches ctVal from `/api/v5/public/instruments` and caches via `ctValCache.Store(inst.InstID, ctVal)` (line 503)
- `PlaceOrder()`: base units / ctVal = contracts (line 145) -- CORRECT
- `PlaceStopLoss()`: same conversion (line 1071) -- CORRECT
- `GetPosition()` / `fetchPositions()`: contracts * ctVal = base units (line 378) -- CORRECT
- `GetOrderFilledQty()`: accFillSz * ctVal (line 282) -- CORRECT
- `GetUserTrades()`: fillSz * ctVal (line 930) -- CORRECT
- WS order updates: accFillSz * ctVal (ws_private.go:175) -- CORRECT
- `getCtVal()` default of 1.0 is safe for most USDT pairs -- CORRECT
- Contract info minSize/stepSize: multiplied by ctVal (lines 499-500) -- CORRECT
- Test coverage: `TestCtValRoundTrip` covers the round-trip -- GOOD

### 2. instId Format
- `toOKXInstID("BTCUSDT")` -> `"BTC-USDT-SWAP"` -- CORRECT
- `fromOKXInstID("BTC-USDT-SWAP")` -> `"BTCUSDT"` -- CORRECT
- Correctly strips `-SWAP` then removes all dashes

### 3. Response Wrapper
- Client unwraps `{code: "0", msg: "", data: [...]}` envelope (client.go:174-196) -- CORRECT
- Returns `okxResp.Data` (raw JSON array) on success -- CORRECT
- Checks `code != "0"` for errors -- CORRECT
- Extracts inner `sCode/sMsg` for batch operations (client.go:184-193) -- CORRECT
- Per-item sCode/sMsg in PlaceOrder response (adapter.go:185) -- CORRECT

### 4. Endpoint URLs
All 20+ endpoints use `/api/v5/` prefix -- CORRECT:
- Account: `/api/v5/account/balance`, `/positions`, `/config`, `/set-leverage`, `/set-position-mode`, `/trade-fee`, `/bills`, `/positions-history`
- Trade: `/api/v5/trade/order`, `/cancel-order`, `/orders-pending`, `/fills-history`, `/order-algo`, `/cancel-algos`
- Public: `/api/v5/public/instruments`, `/funding-rate`, `/funding-rate-history`
- Market: `/api/v5/market/books`
- Asset: `/api/v5/asset/balances`, `/transfer`, `/withdrawal`

### 5. Authentication (REST)
- Headers: `OK-ACCESS-KEY`, `OK-ACCESS-SIGN`, `OK-ACCESS-TIMESTAMP`, `OK-ACCESS-PASSPHRASE`, `Content-Type: application/json` -- CORRECT
- Timestamp format: `2006-01-02T15:04:05.000Z` (ISO with milliseconds) -- CORRECT
- Signature: `Base64(HMAC-SHA256(timestamp + METHOD + requestPath + body, secretKey))` -- CORRECT
- GET parameters in requestPath (not body) -- CORRECT
- POST body as JSON string in signature -- CORRECT

### 6. Authentication (WebSocket)
- Login message: `{"op": "login", "args": [{apiKey, passphrase, timestamp, sign}]}` -- CORRECT
- Timestamp: Unix seconds (not ms) -- CORRECT
- Sign: `HMAC-SHA256(timestamp + "GET" + "/users/self/verify")` -> Base64 -- CORRECT
- Validates login response event="login" && code="0" -- CORRECT

### 7. WebSocket
- Public URL: `wss://ws.okx.com:8443/ws/v5/public` -- CORRECT
- Private URL: `wss://ws.okx.com:8443/ws/v5/private` -- CORRECT
- Subscribe format: `{"op": "subscribe", "args": [{"channel": "...", "instId": "..."}]}` -- CORRECT
- Channels: `tickers`, `books5`, `orders` -- CORRECT
- Ping: sends `"ping"` string, handles `"pong"` response -- CORRECT
- Ping interval: 25s (within 30s timeout) -- CORRECT
- Auto-reconnect on error with 5s backoff -- CORRECT

### 8. Request Parameters (spot-checked)
- PlaceOrder: instId, tdMode, side, ordType, sz, px, reduceOnly, clOrdId -- CORRECT
- CancelOrder: instId, ordId -- CORRECT
- SetLeverage: instId, lever, mgnMode -- CORRECT
- GetOrderbook: instId, sz -- CORRECT
- GetFundingRate: instId -- CORRECT

### 9. Response Field Names (spot-checked)
- Positions: instId, posSide, pos, availPos, avgPx, upl, lever, mgnMode, liqPx, markPx -- CORRECT
- Balance: mgnRatio, details[].ccy, details[].eq, details[].availEq, details[].frozenBal -- CORRECT
- Instruments: instId, tickSz, lotSz, minSz, ctVal, state, settleCcy -- CORRECT
- Funding rate: fundingRate, nextFundingRate, fundingTime, maxFundingRate, minFundingRate -- CORRECT
- Order: ordId, clOrdId, fillSz, accFillSz, avgPx, state -- CORRECT
- Fills: tradeId, ordId, instId, side, fillPx, fillSz, fee, feeCcy, ts -- CORRECT

### 10. Error Handling
- Retryable codes: 50011 (rate limit), 50013 (system busy) -- CORRECT
- Exponential backoff retry (1s, 2s, 4s) with max 3 retries -- GOOD
- CancelOrder ignores 51400/51401 (already filled/cancelled) -- CORRECT
- EnsureOneWayMode handles 59000 (can't change with open positions) -- CORRECT

### 11. Margin/Position Mode
- Forces `net_mode` on startup via `ensureNetMode()` -- CORRECT
- `SetMarginMode` is no-op (margin mode set via set-leverage with mgnMode=cross) -- CORRECT
- Correctly handles `posSide="net"` by inferring direction from `pos` sign -- CORRECT

### 12. Transfer Account IDs
- `from: "18"` (trading), `to: "6"` (funding) for TransferToSpot -- CORRECT per docs
- Unified account: TransferToFutures is no-op -- CORRECT

---

## SUMMARY

| Category | Status | Details |
|----------|--------|---------|
| ctVal conversion | PASS (with exceptions) | Core flow correct; missing in orderbook qty, pending orders sz, close PnL closeTotalPos |
| posSide inference | PASS | Correct for net_mode; properly infers direction |
| Bills endpoint | FAIL | Wrong param name (subType vs type) + double envelope unwrap |
| instId format | PASS | Correct BTC-USDT-SWAP conversion |
| Response wrapper | PASS | Properly unwraps {code, data} envelope |
| Endpoint URLs | PASS | All /api/v5/ prefix |
| Request params | MOSTLY PASS | One wrong param name in GetFundingFees |
| Response fields | PASS | All field names match docs |
| WebSocket | PASS | Correct URLs, channels, auth, ping/pong |
| Authentication | PASS | Correct headers, timestamp, signature for both REST and WS |

**Critical bugs**: 2 (BUG-1, BUG-2 — both in GetFundingFees, making it non-functional)
**Medium bugs**: 2 (BUG-3, BUG-4 — size conversion inconsistencies)
**Low bugs**: 1 (BUG-5 — orderbook qty in contracts)
