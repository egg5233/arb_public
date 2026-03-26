# Bybit Adapter Audit

**Date**: 2026-03-26
**Files Audited**:
- `pkg/exchange/bybit/adapter.go` (1147 lines)
- `pkg/exchange/bybit/client.go` (217 lines)
- `pkg/exchange/bybit/ws.go` (388 lines)
- `pkg/exchange/bybit/ws_private.go` (263 lines)

**Reference**: `doc/EXCHANGEAPI_BYBIT.md`

---

## Audit Summary

| Category | Status |
|----------|--------|
| Endpoint URLs | PASS |
| Request Parameters | PASS (1 low-severity note) |
| Response Parsing | PASS |
| Authentication (REST) | PASS |
| Authentication (WS) | PASS |
| Response Wrapper | PASS |
| Category Parameter | PASS |
| WebSocket Topics | PASS |
| Enum Values | PASS |
| Position Mode | PASS |
| Error Handling | PASS |

**Overall: PASS** — No critical or high-severity issues found. The adapter is well-aligned with the Bybit v5 API documentation.

---

## Method-by-Method Audit

### REST Client (`client.go`)

#### Authentication & Signing
- **Signature construction** (line 69): `timestamp + apiKey + recvWindow + payload` — matches docs exactly.
- **GET payload**: Query string (sorted, URL-encoded) — correct.
- **POST payload**: JSON body string — correct.
- **HMAC-SHA256** with hex encoding (lowercase) — correct.
- **Headers**: `X-BAPI-API-KEY`, `X-BAPI-SIGN`, `X-BAPI-TIMESTAMP`, `X-BAPI-RECV-WINDOW` — all four required headers present and correct.

#### Response Wrapper
- Unwraps `{retCode, retMsg, result}` (line 156-165) — correct.
- `retCode != 0` treated as error — correct.

#### Retry Logic
- Retryable codes: 10006 (rate limit), 10016 (server error), 10018 (server busy) — all valid per docs.
- Exponential backoff with max 3 retries — reasonable.
- Network error detection (timeout, connection refused, EOF, reset) — correct.

#### Query String Building
- Sorted alphabetically by key — correct (required for deterministic signature).
- URL-encoded — correct.

**Verdict**: PASS

---

### PlaceOrder (`adapter.go:148`)
- **Endpoint**: `POST /v5/order/create` — correct.
- **Params**: `category:"linear"`, `symbol`, `side`, `orderType`, `qty`, `timeInForce` — all correct.
- **Side mapping**: `Buy`/`Sell` (PascalCase) — matches docs.
- **OrderType mapping**: `Market`/`Limit` (PascalCase) — matches docs.
- **TimeInForce mapping**: `GTC`/`IOC`/`FOK`/`PostOnly` — matches docs.
- **Price**: Only sent for limit orders — correct.
- **reduceOnly**: Set when requested — correct.
- **orderLinkId**: Mapped from `ClientOid` — correct (max 36 chars per docs).
- **positionIdx**: Not sent — defaults to 0 (one-way mode), which is correct for this bot's config.
- **Response**: Parses `orderId` from `result` — correct.

**Verdict**: PASS

---

### CancelOrder (`adapter.go:183`)
- **Endpoint**: `POST /v5/order/cancel` — correct.
- **Params**: `category`, `symbol`, `orderId` — correct (docs require either `orderId` or `orderLinkId`).
- **Idempotent handling**: Error 110001 (order does not exist) → returns nil — correct per docs.
- **Error 170213**: Additional idempotent code (spot-related) — defensive, harmless.

**Verdict**: PASS

---

### GetPendingOrders (`adapter.go:203`)
- **Endpoint**: `GET /v5/order/realtime` — correct.
- **Params**: `category`, `symbol`, `openOnly:"0"` — correct. Docs confirm `0` = open orders only (default).
- **Response fields**: `orderId`, `orderLinkId`, `symbol`, `side`, `orderType`, `price`, `qty`, `orderStatus` — all match docs.

**Verdict**: PASS

---

### GetOrderFilledQty (`adapter.go:247`)
- **Endpoint**: `GET /v5/order/realtime` — correct.
- **Params**: `category`, `symbol`, `orderId` — correct.
- **Response**: `cumExecQty` — correct per docs.

**Verdict**: PASS

---

### GetPosition / GetAllPositions (`adapter.go:280-303`)
- **Endpoint**: `GET /v5/position/list` — correct.
- **GetPosition params**: `category:"linear"`, `symbol` — correct.
- **GetAllPositions params**: `category:"linear"`, `settleCoin:"USDT"` — correct (docs: "either symbol or settleCoin required" for linear).
- **Response fields**: `symbol`, `side`, `size`, `avgPrice`, `unrealisedPnl`, `leverage`, `tradeMode`, `positionValue`, `liqPrice`, `markPrice` — all match docs.
- **Side mapping**: `Buy` = long, `Sell` = short, empty positions (size=0) skipped — correct per docs ("Buy"=long, "Sell"=short, ""=empty).
- **Margin mode**: `tradeMode` 0=cross, 1=isolated — correct.

**Verdict**: PASS

---

### SetLeverage (`adapter.go:361`)
- **Endpoint**: `POST /v5/position/set-leverage` — correct.
- **Params**: `category`, `symbol`, `buyLeverage`, `sellLeverage` (both set to same value) — correct. Docs: "One-way: must equal sellLeverage".
- **Error 110043**: Leverage not modified → no-op — correct per docs.

**Verdict**: PASS

---

### SetMarginMode (`adapter.go:381`)
- **Endpoint**: `POST /v5/position/switch-isolated` — valid Bybit v5 endpoint (not in local docs but correct).
- **Params**: `category`, `symbol`, `tradeMode` (0=cross, 1=isolated), `buyLeverage`, `sellLeverage` — correct (Bybit requires leverage when switching).
- **Error 110026**: Already in requested mode → no-op — correct per docs.
- **Error 100028**: Unified trading account (margin mode managed at account level) → no-op — correct.

**Verdict**: PASS

---

### LoadAllContracts (`adapter.go:411`)
- **Endpoint**: `GET /v5/market/instruments-info` — correct.
- **Params**: `category:"linear"` — correct.
- **Response fields**: `symbol`, `status`, `lotSizeFilter.minOrderQty`, `lotSizeFilter.maxOrderQty`, `lotSizeFilter.qtyStep`, `priceFilter.tickSize`, `fundingInterval` — all match docs.
- **Filtering**: Only `Trading` status — correct (docs: "Default: Trading only for linear").

| Issue | Severity | Detail |
|-------|----------|--------|
| Missing pagination | LOW | Default `limit=500`. If Bybit has >500 linear instruments, subsequent pages are not fetched. Currently ~450 linear pairs, so this works but is fragile. |

**Verdict**: PASS (with note)

---

### GetFundingRate (`adapter.go:479`)
- **Endpoint**: `GET /v5/market/tickers` (for rate) + `GET /v5/market/instruments-info` (for interval/caps) — correct.
- **Ticker fields**: `fundingRate`, `nextFundingTime` — correct per docs.
- **Instrument fields**: `fundingInterval` (minutes), `upperFundingRate`, `lowerFundingRate` — correct per docs.
- **fundingInterval**: Parsed as minutes, converted to `time.Duration` — correct (docs: "480 = 8 hours").

**Verdict**: PASS

---

### GetFundingInterval (`adapter.go:548`)
- **Endpoint**: `GET /v5/market/instruments-info` — correct.
- **Response**: `fundingInterval` as `json.Number` → parsed as float64 minutes — correct.

**Verdict**: PASS

---

### GetFuturesBalance (`adapter.go:581`)
- **Endpoint**: `GET /v5/account/wallet-balance` — correct.
- **Params**: `accountType:"UNIFIED"`, `coin:"USDT"` — correct per docs.
- **Response fields**: `accountMMRate`, `coin[].equity`, `coin[].availableToWithdraw`, `coin[].locked` — correct. (`availableToWithdraw` exists in actual API response, not shown in local docs sample).
- **Fallback logic**: If `availableToWithdraw=0` but `equity>0`, uses `equity - locked` — defensive and correct for unified accounts.

**Verdict**: PASS

---

### GetSpotBalance (`adapter.go:633`)
- **Endpoint**: `GET /v5/asset/transfer/query-account-coins-balance` — correct.
- **Params**: `accountType:"FUND"`, `coin:"USDT"` — correct per docs.
- **Response fields**: `balance[].coin`, `balance[].transferBalance`, `balance[].walletBalance` — match docs.

**Verdict**: PASS

---

### TransferToFutures / TransferToSpot (`adapter.go:671-688`)
- **TransferToFutures**: No-op — correct (Bybit unified account, trading balance already available).
- **TransferToSpot endpoint**: `POST /v5/asset/transfer/inter-transfer` — correct.
- **Params**: `transferId` (UUID), `coin`, `amount`, `fromAccountType:"UNIFIED"`, `toAccountType:"FUND"` — correct per docs.
- **UUID generation**: Random UUID v4 — correct (docs: "manually generated").

**Verdict**: PASS

---

### Withdraw (`adapter.go:690`)
- **Endpoint**: `POST /v5/asset/withdraw/create` — valid Bybit endpoint (not in local docs).
- **Chain mapping**: BEP20→BSC, APT→APTOS — correct Bybit chain names.
- **Response**: Parses `id` — correct.

**Verdict**: PASS

---

### GetOrderbook (`adapter.go:731`)
- **Endpoint**: `GET /v5/market/orderbook` — correct.
- **Params**: `category:"linear"`, `symbol`, `limit` — correct.
- **Response fields**: `s` (symbol), `b` (bids `[price, size]`), `a` (asks), `ts` (timestamp ms) — all match docs exactly.

**Verdict**: PASS

---

### GetUserTrades (`adapter.go:869`)
- **Endpoint**: `GET /v5/execution/list` — correct.
- **Params**: `category:"linear"`, `symbol`, `startTime`, `limit` — correct.
- **Limit**: Clamped to max 100 — correct per docs (`[1, 100]`).
- **Response fields**: `execId`, `orderId`, `symbol`, `side`, `execPrice`, `execQty`, `execFee`, `feeCurrency`, `execTime` — all match docs.
- **Fee abs()**: `if fee < 0 { fee = -fee }` — defensive normalization.

**Verdict**: PASS

---

### GetFundingFees (`adapter.go:926`)
- **Endpoint**: `GET /v5/account/transaction-log` — valid Bybit endpoint (not in local docs).
- **Params**: `category:"linear"`, `symbol`, `type:"SETTLEMENT"`, `startTime`, `limit:"50"` — correct.
- **Response**: Parses `funding` and `transactionTime` — correct.

**Verdict**: PASS

---

### GetClosePnL (`adapter.go:963`)
- **Endpoint**: `GET /v5/position/closed-pnl` — correct.
- **Params**: `category:"linear"`, `symbol`, `startTime`, `limit:"50"` — correct (docs: limit `[1, 100]`).
- **Response fields**: `closedPnl`, `cumEntryValue`, `cumExitValue`, `avgEntryPrice`, `avgExitPrice`, `openFee`, `closeFee`, `closedSize`, `side`, `updatedTime` — all match docs.
- **Side normalization**: `Buy` close = was short, `Sell` close = was long — correct.

**Verdict**: PASS

---

### PlaceStopLoss (`adapter.go:1045`)
- **Endpoint**: `POST /v5/order/create` with `orderFilter:"StopOrder"` — correct.
- **Params**: `category`, `symbol`, `side`, `orderType:"Market"`, `qty`, `triggerPrice`, `triggerDirection`, `triggerBy:"MarkPrice"`, `reduceOnly:"true"`, `timeInForce:"GTC"` — correct.
- **triggerDirection**: `2` = price falls below (long SL), `1` = price rises above (short SL) — correct per docs.

**Verdict**: PASS

---

### CancelStopLoss (`adapter.go:1082`)
- **Endpoint**: `POST /v5/order/cancel` with `orderFilter:"StopOrder"` — correct.
- **Idempotent**: Same error handling as CancelOrder (110001, 170213) — correct.

**Verdict**: PASS

---

### EnsureOneWayMode (`adapter.go:1101`)
- **Endpoint**: `POST /v5/position/switch-mode` — correct.
- **Params**: `category:"linear"`, `mode:"0"` (MergedSingle), `coin:"USDT"` — correct per docs (`0` = one-way, `3` = hedge).
- **Error 110025**: Position mode not modified → no-op — correct per docs.
- **Fallback**: If switch fails, checks current mode via position query — defensive and correct.

**Verdict**: PASS

---

### CheckPermissions (`adapter.go:50`)
- **Endpoint**: `GET /v5/user/query-api` — valid Bybit endpoint (not in local docs).
- **Response**: Checks `readOnly` flag and `permissions` map for `ContractTrade`, `Wallet` — correct.
- **Logic**: `readOnly=1` → deny all trade/wallet; otherwise check `ContractTrade` for trade, `Wallet.Withdraw`/`Wallet.AccountTransfer` — correct.

**Verdict**: PASS

---

## WebSocket Audit

### Public WebSocket (`ws.go`)

- **URL**: `wss://stream.bybit.com/v5/public/linear` — correct per docs.
- **Ping**: Every 20 seconds (`{"op":"ping"}`) — correct per docs ("send ping every 20 seconds").
- **Subscribe format**: `{"op":"subscribe","args":["tickers.BTCUSDT"]}` — correct per docs.

#### Ticker Stream
- **Topic**: `tickers.{symbol}` — correct.
- **Fields parsed**: `symbol`, `bid1Price`, `ask1Price` — correct per docs.
- **Handling**: Updates BBO in priceStore — correct.

#### Orderbook Stream
- **Topic**: `orderbook.50.{symbol}` — correct (50-level depth, 20ms push).
- **Data fields**: `b` (bids), `a` (asks), `s` (symbol), `u` (update ID), `ts` (timestamp) — all match docs.
- **Snapshot handling**: Stores full orderbook — correct.
- **Delta handling**: size=0 → delete, else upsert — correct per docs ("size=0 -> delete entry").
- **Sorting**: Bids descending, asks ascending after delta application — correct.

| Issue | Severity | Detail |
|-------|----------|--------|
| Comment mismatch | INFO | Line 327 comment says "top-5 orderbook depth" but actually subscribes to `orderbook.50` (50 levels). Cosmetic only, no runtime impact. |

**Verdict**: PASS

---

### Private WebSocket (`ws_private.go`)

- **URL**: `wss://stream.bybit.com/v5/private` — correct per docs.

#### Authentication
- **Expires**: `now + 10000` ms (10 seconds) — correct.
- **Signature message**: `"GET/realtime" + expiresStr` — correct per docs.
- **HMAC-SHA256**: Correct.
- **Auth message**: `{"op":"auth","args":[apiKey, expires, signature]}` — correct. `expires` sent as int64, `signature` as string — matches docs format.
- **Auth response**: Checks `success` field — correct.

#### Order Stream
- **Topic**: `"order"` — correct per docs (subscribes to all categories).
- **Fields parsed**: `symbol`, `orderId`, `orderLinkId`, `orderStatus`, `cumExecQty`, `avgPrice` — all match docs.
- **Status normalization**: `New`→`new`, `PartiallyFilled`→`partially_filled`, `Filled`→`filled`, `Cancelled`→`cancelled`, `Rejected`→`rejected`, `Deactivated`→`deactivated` — correct.
- **Fill callback**: Triggered on `status=filled && filledVolume>0` — correct.

| Issue | Severity | Detail |
|-------|----------|--------|
| Missing `PartiallyFilledCanceled` | INFO | Not normalized in `normalizeOrderStatus()`. Per docs, this status is spot-only — not relevant for `linear` category. No impact. |

**Verdict**: PASS

---

## Low-Severity Notes

### 1. POST Body String Serialization
**Location**: `client.go:122-128`
**Detail**: `bodyParams` is `map[string]string`, so all JSON values are serialized as strings (e.g., `"reduceOnly":"true"` instead of `"reduceOnly":true`). The Bybit API expects boolean/integer types per documentation, but accepts string representations in practice (confirmed by live operation).
**Impact**: None (system is live and working). Bybit's API is lenient with type coercion.
**Severity**: LOW

### 2. LoadAllContracts Missing Pagination
**Location**: `adapter.go:411`
**Detail**: Only fetches first page with default `limit=500`. Bybit currently has ~450 linear pairs, so this works. If Bybit exceeds 500 linear instruments, new contracts would be silently missed.
**Severity**: LOW — monitor and add pagination cursor handling if Bybit's linear count approaches 500.

### 3. Endpoints Not in Local Docs
The following valid Bybit v5 endpoints are used but not documented in `EXCHANGEAPI_BYBIT.md`:
- `GET /v5/user/query-api` (CheckPermissions)
- `POST /v5/position/switch-isolated` (SetMarginMode)
- `POST /v5/asset/withdraw/create` (Withdraw)
- `GET /v5/account/transaction-log` (GetFundingFees)

**Severity**: INFO — consider adding these to the local docs for completeness.

---

## Conclusion

The Bybit adapter is **well-implemented** and correctly follows the Bybit v5 API specification. All endpoint URLs, parameter names, response field names, authentication, WebSocket topics, and enum values are correct. No critical or high-severity issues were found. The two low-severity items (string serialization and missing pagination) are non-blocking and the system operates correctly in production.
