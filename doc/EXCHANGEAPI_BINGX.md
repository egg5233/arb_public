# BingX API Documentation Reference

## Base URLs

| Environment | REST API | WebSocket (Swap) |
|---|---|---|
| Production | `https://open-api.bingx.com` | `wss://open-api-swap.bingx.com/swap-market` |
| VST (Testnet) | `https://open-api-vst.bingx.com` | `wss://vst-open-api-ws.bingx.com/swap-market` |

**Server Location**: AWS Singapore (ap-southeast-1), Availability Zones: apse1-az1, apse1-az2, apse1-az3

## Authentication

### API Key Setup
- Create API Keys at BingX website: User Center -> API Management
- Each parent user can create up to **20 API Keys**; each sub-user can also create up to 20
- API Keys have **read-only** permission by default; enable trading permissions separately
- Strongly recommended to configure an **IP whitelist**
- Spot and contract trading permissions are **separate** but share the same API Key

### Signing Requests

All authenticated REST requests must include:
1. `X-BX-APIKEY` in the **request header**
2. `signature` as a **request parameter**
3. `timestamp` (milliseconds) as a request parameter

**Signature Algorithm**: HMAC-SHA256, returned as 64-character lowercase hex string.

**Steps**:
1. Collect all business parameters (excluding `signature`)
2. Add `timestamp` (milliseconds) as a normal parameter
3. Sort all parameters by key in **ASCII ascending order**
4. Build signing string: `key=value&key2=value2&...&timestamp=xxx` (values NOT URL-encoded)
5. Compute `HMAC-SHA256(signingString, secretKey)` to get `signature`

**URL Encoding Rules** (query string only):
- Signature is always calculated from the **unencoded** signing string
- If signing string contains `[` or `{`, URL-encode only parameter **values** (not keys) in the actual request URL
- If no `[` or `{`, values do not need encoding

**Example**:
```
Signing string: recvWindow=0&symbol=BTC-USDT&timestamp=1696751141337
Signature: echo -n 'recvWindow=0&symbol=BTC-USDT&timestamp=1696751141337' | openssl dgst -sha256 -hmac 'SECRET_KEY' -hex
Request: https://open-api.bingx.com/openApi/...?recvWindow=0&symbol=BTC-USDT&timestamp=1696751141337&signature=...
```

**Request Body Endpoints** (application/json): Some endpoints require signature in the JSON body (not query string), mainly sub-account management endpoints. Timestamp and signature go inside the body.

### Request Timing
- Timestamp must be within **5 seconds** of server time (configurable via `recvWindow`, default 5000ms)
- Query server time: `GET /openApi/swap/v2/server/time` -> `{"code":0,"msg":"","data":{"serverTime":1675319535362}}`

## Rate Limits

- Rate limiting is based on **account UID**, each API has its own independent limit
- If exceeded, system auto-rate-limits and **restores after 5 minutes**
- Check rate limit status via HTTP headers:
  - `X-RateLimit-Requests-Remain` — remaining requests
  - `X-RateLimit-Requests-Expire` — window expiration time
- WebSocket: max **200 topics** per connection, max **60 websockets** per IP
- WebSocket subscription rate limit: max **10/s**

**Typical Rate Limits**:

| Category | Rate Limit |
|---|---|
| Market data (contracts, depth, trades, klines, ticker, funding) | 500 req / 10s (IP) |
| Place order | 10/s per UID |
| Cancel order | 10/s per UID |
| Cancel all orders | 5/s per UID |
| Query open orders | 5/s per UID |
| Query account balance | 5/s per UID |
| Query positions | 10/s per UID |
| Query P&L fund flow | 5/s per UID |
| Query commission rate | 5/s per UID |
| Set leverage | 4/s per UID |
| Set position mode | 4/s per UID |
| Query position mode | 2/s per UID |
| Listen key operations | 2/s per UID |

## Response Wrapper Structure

All REST responses follow this wrapper format:
```json
{
  "code": 0,        // 0 = success, non-zero = error
  "msg": "",         // Error message (empty on success)
  "data": { ... }    // Response payload (object or array)
}
```

Some spot endpoints also include `"debugMsg": ""`.

## Numeric Conventions
- Decimal numbers are returned as **strings** to preserve precision
- Integers (trade numbers, order IDs) are returned as numbers without quotes
- Timestamps are in **milliseconds**
- Prices and quantities should be sent as strings to avoid truncation

## Symbol Format
- **Format**: `BTC-USDT` (hyphenated with dash)
- Must include a hyphen "-" in the trading pair symbol
- Case: uppercase recommended (`BTC-USDT` not `btc-usdt`)

---

## REST API Endpoints

### Perpetual Swap — Market Data

#### Get Contract List (USDT-M Perp Futures Symbols)
- **Endpoint**: `GET /openApi/swap/v2/quote/contracts`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No signature required
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair, e.g. BTC-USDT |
| timestamp | int64 | No | Request timestamp (ms) |
| recvWindow | int64 | No | Request valid window (ms) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| contractId | string | Contract ID |
| symbol | string | Trading pair (e.g. BTC-USDT) |
| quantityPrecision | int64 | Quantity decimal places |
| pricePrecision | int64 | Price decimal places |
| makerFeeRate | float64 | Maker fee rate |
| takerFeeRate | float64 | Taker fee rate |
| tradeMinQuantity | float64 | Min trade quantity (COIN) |
| tradeMinUSDT | float64 | Min trade quantity (USDT) |
| maxLongLeverage | int64 | Max long leverage |
| maxShortLeverage | int64 | Max short leverage |
| currency | string | Settlement currency (e.g. USDT) |
| asset | string | Trading asset (e.g. BTC) |
| status | int64 | 1=online, 25=no-open, 5=pre-online, 0=offline |
| apiStateOpen | string | Can API open positions ("true"/"false") |
| apiStateClose | string | Can API close positions ("true"/"false") |
| launchTime | long | Listing time (ms) |
| maintainTime | long | No-open start time (ms) |
| offTime | long | Offline time (ms) |

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": [
    {
      "contractId": "100",
      "symbol": "BTC-USDT",
      "size": "0",
      "quantityPrecision": 4,
      "pricePrecision": 1,
      "feeRate": 0.0005,
      "makerFeeRate": 0.0002,
      "takerFeeRate": 0.0005,
      "tradeMinLimit": 0,
      "tradeMinQuantity": 0.0001,
      "tradeMinUSDT": 2,
      "maxLongLeverage": 125,
      "maxShortLeverage": 125,
      "currency": "USDT",
      "asset": "BTC",
      "status": 1,
      "apiStateOpen": "true",
      "apiStateClose": "true",
      "launchTime": 1586275200000,
      "maintainTime": 0,
      "offTime": 0
    }
  ]
}
```

#### Order Book
- **Endpoint**: `GET /openApi/swap/v2/quote/depth`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair (e.g. BTC-USDT) |
| limit | int64 | No | Default 20, options: [5, 10, 20, 50, 100, 500, 1000] |

- **Response Fields**: `T` (timestamp ms), `asks` (array of [price, qty]), `bids` (array of [price, qty]), `asksCoin`, `bidsCoin` (quantity in coin units)

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": {
    "T": 1702719083983,
    "bids": [["0.000009854", "483909"], ["0.000009853", "824851"]],
    "asks": [["0.000009860", "578208"], ["0.000009859", "279010"]]
  }
}
```

#### Recent Trades List
- **Endpoint**: `GET /openApi/swap/v2/quote/trades`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair (e.g. BTC-USDT) |
| limit | int64 | No | Default 500, max 1000 |
| timestamp | int64 | Yes | Request timestamp (ms) |

- **Response Fields**: `time` (ms), `isBuyerMaker` (bool), `price` (string), `qty` (string), `quoteQty` (string)

#### Mark Price and Funding Rate
- **Endpoint**: `GET /openApi/swap/v2/quote/premiumIndex`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair (omit for all symbols) |
| timestamp | int64 | Yes | Request timestamp (ms) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| symbol | string | Trading pair |
| markPrice | string | Current mark price |
| indexPrice | string | Index price |
| lastFundingRate | string | Last funding rate (decimal, e.g. "0.00010000") |
| nextFundingTime | int64 | Time until next settlement (ms timestamp) |
| fundingIntervalHours | int64 | Settlement cycle: 1, 2, 4, or 8 hours |
| minFundingRate | string | Lower limit (decimal, e.g. "0.025" = 2.5%) |
| maxFundingRate | string | Upper limit |
| updateTime | int64 | Data update time (ms) |

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": {
    "symbol": "BTC-USDT",
    "markPrice": "69836.0",
    "indexPrice": "69888.9",
    "lastFundingRate": "0.00010000",
    "nextFundingTime": 1773907200000,
    "fundingIntervalHours": 8,
    "minFundingRate": "-0.003000",
    "maxFundingRate": "0.003000",
    "updateTime": 1773878400000
  }
}
```

#### Get Funding Rate History
- **Endpoint**: `GET /openApi/swap/v2/quote/fundingRate`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair |
| startTime | int64 | No | Start time (ms) |
| endTime | int64 | No | End time (ms) |
| limit | int32 | No | Default 100, max 1000 |

- **Response Fields**: `symbol`, `fundingRate` (string), `fundingTime` (ms)

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": [
    {"symbol": "QNT-USDT", "fundingRate": "0.00027100", "fundingTime": 1702713600000},
    {"symbol": "QNT-USDT", "fundingRate": "0.00012800", "fundingTime": 1702684800000}
  ]
}
```

#### Kline/Candlestick Data
- **Endpoint**: `GET /openApi/swap/v3/quote/klines`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair |
| interval | string | Yes | Time interval (1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M) |
| startTime | int64 | No | Start time (ms) |
| endTime | int64 | No | End time (ms) |
| limit | int64 | No | Default 500, max 1440 |

- **Response Fields**: `open`, `close`, `high`, `low` (float64), `volume` (float64), `time` (ms)

#### 24hr Ticker Price Change Statistics
- **Endpoint**: `GET /openApi/swap/v2/quote/ticker`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**: `symbol` (optional, omit for all)

- **Response Fields**: `symbol`, `priceChange`, `priceChangePercent`, `lastPrice`, `lastQty`, `highPrice`, `lowPrice`, `volume`, `quoteVolume`, `openPrice`, `openTime`, `closeTime`, `bidPrice`, `bidQty`, `askPrice`, `askQty`

#### Symbol Order Book Ticker (Best Bid/Ask)
- **Endpoint**: `GET /openApi/swap/v2/quote/bookTicker`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No (but requires Professional Futures Trading API key permission)
- **Parameters**: `symbol` (required)

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": {
    "book_ticker": {
      "symbol": "BTC-USDT",
      "bid_price": 42211.1,
      "bid_qty": 12663,
      "ask_price": 42211.8,
      "ask_qty": 128854
    }
  }
}
```

#### Symbol Price Ticker
- **Endpoint**: `GET /openApi/swap/v1/ticker/price`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No
- **Parameters**: `symbol` (optional, omit for all symbols)

- **Response Example**:
```json
{"code": 0, "msg": "", "data": {"symbol": "TIA-USDT", "price": "14.0658", "time": 1702718922941}}
```

---

### Perpetual Swap — Trading

#### Place Order
- **Endpoint**: `POST /openApi/swap/v2/trade/order`
- **Rate Limit**: 10/s per UID
- **Auth**: Yes (signature required)
- **Permission**: Professional Futures Trading
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair (e.g. BTC-USDT) |
| type | string | Yes | Order type (see enum below) |
| side | string | Yes | `BUY` or `SELL` |
| positionSide | string | No | `BOTH` (one-way), `LONG` or `SHORT` (hedge mode). Default: `LONG` |
| reduceOnly | string | No | `true`/`false` (one-way mode only) |
| price | float64 | No | Price (required for LIMIT; trailing distance for TRAILING orders) |
| quantity | float64 | No | Quantity in COIN |
| quoteOrderQty | float64 | No | Quantity in USDT (quantity takes priority if both provided) |
| stopPrice | float64 | No | Trigger price (for STOP/TP orders) |
| priceRate | float64 | No | For TRAILING orders, max 1 |
| workingType | string | No | `MARK_PRICE` (default), `CONTRACT_PRICE`, `INDEX_PRICE` |
| timeInForce | string | No | `PostOnly`, `GTC`, `IOC`, `FOK` |
| clientOrderId | string | No | Custom order ID (1-40 chars, lowercase, unique per order, LIMIT/MARKET only) |
| closePosition | string | No | `true`/`false` — close all position on trigger (STOP_MARKET/TAKE_PROFIT_MARKET only) |
| stopGuaranteed | string | No | `true`/`false`/`cutfee` — guaranteed stop-loss feature |
| activationPrice | float64 | No | For trailing orders |
| positionId | int64 | No | Required in Separate Isolated mode for closing |

**Order Type Enum Values**:
- `LIMIT` — Limit order (requires: quantity, price)
- `MARKET` — Market order (requires: quantity)
- `STOP_MARKET` — Stop market (requires: quantity, stopPrice)
- `TAKE_PROFIT_MARKET` — Take profit market (requires: quantity, stopPrice)
- `STOP` — Stop limit (requires: quantity, stopPrice, price)
- `TAKE_PROFIT` — Take profit limit (requires: quantity, stopPrice, price)
- `TRIGGER_LIMIT` — Trigger limit order (requires: quantity, stopPrice, price)
- `TRIGGER_MARKET` — Trigger market order (requires: quantity, stopPrice)
- `TRAILING_STOP_MARKET` — Trailing stop market
- `TRAILING_TP_SL` — Trailing take profit or stop loss

**Response Fields**: `orderId` (int64), `symbol`, `side`, `positionSide`, `type`, `price`, `quantity`, `stopPrice`, `workingType`, `clientOrderId`, `timeInForce`

#### Cancel Order
- **Endpoint**: `DELETE /openApi/swap/v2/trade/order`
- **Rate Limit**: 10/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair |
| orderId | int64 | No | Order ID (either orderId or clientOrderId required) |
| clientOrderId | string | No | Custom order ID |
| timestamp | int64 | Yes | Request timestamp (ms) |

- **Note**: Cancel API limited to 1/s for the same orderId or clientOrderId. Do not resubmit.

- **Response**: Returns the cancelled order object with full details (status: "CANCELLED")

#### Cancel All Open Orders
- **Endpoint**: `DELETE /openApi/swap/v2/trade/allOpenOrders`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair (omit to cancel all symbols) |
| type | string | Yes | Order type filter |
| timestamp | int64 | Yes | Request timestamp (ms) |

- **Response**: `{ "success": [...orders], "failed": [...errors] }`

#### Close All Positions
- **Endpoint**: `POST /openApi/swap/v2/trade/closeAllPositions`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (optional), `timestamp` (required)

#### Current All Open Orders
- **Endpoint**: `GET /openApi/swap/v2/trade/openOrders`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair (omit for all) |
| type | string | No | Order type filter |
| timestamp | int64 | Yes | Request timestamp (ms) |

- **Response**: Array of order objects with fields: `orderId`, `symbol`, `side`, `positionSide`, `type`, `origQty`, `price`, `executedQty`, `avgPrice`, `cumQuote`, `stopPrice`, `profit`, `commission`, `status`, `time`, `updateTime`, `clientOrderId`, `leverage`, `workingType`, `takeProfit`, `stopLoss`

**Order Status Enum Values**:
- `NEW` — Order accepted
- `PENDING` — Pending trigger
- `PARTIALLY_FILLED` — Partially filled
- `FILLED` — Fully filled
- `CANCELED` — Cancelled
- `CANCELLED` — Cancelled (alternate spelling)
- `EXPIRED` — Expired

#### Query Order Details
- **Endpoint**: `GET /openApi/swap/v2/trade/order`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (required), `orderId` (optional), `clientOrderId` (optional), `timestamp` (required)

#### Query Order History
- **Endpoint**: `GET /openApi/swap/v2/trade/allOrders`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (optional), `orderId` (optional), `startTime` (optional), `endTime` (optional), `limit` (optional, default 500, max 1000), `timestamp` (required)

---

### Perpetual Swap — Position & Leverage

#### Query Margin Type
- **Endpoint**: `GET /openApi/swap/v2/trade/marginType`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (required), `timestamp` (required)
- **Response**: `marginType` field — `CROSSED` or `ISOLATED`

#### Change Margin Type
- **Endpoint**: `POST /openApi/swap/v2/trade/marginType`
- **Rate Limit**: 4/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (required), `marginType` (required: `CROSSED` or `ISOLATED`), `timestamp` (required)

#### Query Leverage and Available Positions
- **Endpoint**: `GET /openApi/swap/v2/trade/leverage`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (required), `timestamp` (required)

#### Set Leverage
- **Endpoint**: `POST /openApi/swap/v2/trade/leverage`
- **Rate Limit**: 4/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | Yes | Trading pair |
| side | string | Yes | `LONG` or `SHORT` |
| leverage | int64 | Yes | Target leverage (1-125 depending on contract) |
| timestamp | int64 | Yes | Request timestamp (ms) |

#### Set Position Mode
- **Endpoint**: `POST /openApi/swap/v1/positionSide/dual`
- **Rate Limit**: 4/s per UID
- **Auth**: Yes
- **Parameters**: `dualSidePosition` (`"true"` = hedge mode, `"false"` = one-way mode), `timestamp`
- **Note**: Can only be changed when there are **no active positions or pending orders**

#### Query Position Mode
- **Endpoint**: `GET /openApi/swap/v1/positionSide/dual`
- **Rate Limit**: 2/s per UID
- **Auth**: Yes
- **Response**: `dualSidePosition` — `"true"` (hedge) or `"false"` (one-way)

---

### Perpetual Swap — Account

#### Query Account Data (Balance)
- **Endpoint**: `GET /openApi/swap/v3/user/balance`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `timestamp` (required)
- **Description**: Get USDC and USDT perpetual account asset information

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| asset | string | Asset type (e.g. "USDT") |
| balance | string | Asset balance |
| equity | string | Net asset value |
| unrealizedProfit | string | Unrealized P&L |
| realisedProfit | string | Realized P&L |
| availableMargin | string | Available margin |
| usedMargin | string | Used margin |
| freezedMargin | string | Frozen margin |

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": [
    {
      "userId": "116***295",
      "asset": "USDT",
      "balance": "194.8212",
      "equity": "196.7431",
      "unrealizedProfit": "1.9219",
      "realisedProfit": "-109.2504",
      "availableMargin": "193.7609",
      "usedMargin": "1.0602",
      "freezedMargin": "0.0000"
    }
  ]
}
```

#### Query Position Data
- **Endpoint**: `GET /openApi/swap/v2/user/positions`
- **Rate Limit**: 10/s per UID
- **Auth**: Yes
- **Parameters**: `symbol` (optional, omit for all), `timestamp` (required)

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| positionId | string | Position ID |
| symbol | string | Trading pair |
| positionSide | string | `LONG` / `SHORT` |
| isolated | bool | true=isolated, false=cross |
| positionAmt | string | Position amount |
| availableAmt | string | Available amount |
| unrealizedProfit | string | Unrealized P&L |
| realisedProfit | string | Realized P&L |
| initialMargin | string | Initial margin |
| margin | string | Margin |
| avgPrice | string | Average entry price |
| liquidationPrice | float64 | Liquidation price |
| leverage | int64 | Leverage |
| positionValue | string | Position value |
| markPrice | string | Mark price |
| riskRate | string | Risk rate (100% = liquidation) |
| maxMarginReduction | string | Max margin reduction |
| pnlRatio | string | Unrealized P&L ratio |
| updateTime | int64 | Update time (ms) |

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": [
    {
      "positionId": "1735*****52",
      "symbol": "BNB-USDT",
      "currency": "USDT",
      "positionAmt": "0.20",
      "availableAmt": "0.20",
      "positionSide": "SHORT",
      "isolated": true,
      "avgPrice": "246.43",
      "initialMargin": "9.7914",
      "leverage": 5,
      "unrealizedProfit": "-0.0653",
      "realisedProfit": "-0.0251",
      "liquidationPrice": 294.16914617776246
    }
  ]
}
```

#### Get Account Profit and Loss Fund Flow
- **Endpoint**: `GET /openApi/swap/v2/user/income`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair |
| incomeType | string | No | Income type filter (TRANSFER, REALIZED_PNL, FUNDING_FEE, TRADING_FEE, INSURANCE_CLEAR, TRIAL_FUND, ADL, SYSTEM_DEDUCTION) |
| startTime | int64 | No | Start time |
| endTime | int64 | No | End time |
| limit | int64 | No | Default 100, max 1000 |

- **Response Fields**: `symbol`, `incomeType`, `income` (string, positive=inflow, negative=outflow), `asset`, `info`, `time` (ms), `tranId`, `tradeId`

#### Query Trading Commission Rate
- **Endpoint**: `GET /openApi/swap/v2/user/commissionRate`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": {
    "commission": {
      "takerCommissionRate": 0.0005,
      "makerCommissionRate": 0.0002
    }
  }
}
```

---

### Spot / Wallet

#### Query Spot Assets (Balance)
- **Endpoint**: `GET /openApi/spot/v1/account/balance`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes
- **Parameters**: `timestamp` (required), `recvWindow` (optional)

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": {
    "balances": [
      {"asset": "USDT", "free": "566773.193402631", "locked": "244.186162653"},
      {"asset": "BTC", "free": "0.5", "locked": "0"}
    ]
  }
}
```

#### Asset Transfer (Between Accounts)
- **Endpoint**: `POST /openApi/api/v3/asset/transfer`
- **Rate Limit**: 10/s per UID
- **Auth**: Yes
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| type | ENUM | Yes | Transfer type |
| asset | string | Yes | Currency name (e.g. USDT) |
| amount | DECIMAL | Yes | Transfer amount |
| timestamp | int64 | Yes | Timestamp (ms) |

**Transfer Type Enum Values**:
- `FUND_SFUTURES` — Fund Account -> Perpetual Futures
- `SFUTURES_FUND` — Perpetual Futures -> Fund Account
- `FUND_PFUTURES` — Fund Account -> Standard Futures
- `PFUTURES_FUND` — Standard Futures -> Fund Account
- `SFUTURES_PFUTURES` — Perpetual -> Standard
- `PFUTURES_SFUTURES` — Standard -> Perpetual

#### Asset Transfer Records
- **Endpoint**: `GET /openApi/api/v3/asset/transfer`
- **Rate Limit**: 10/s per UID
- **Auth**: Yes
- **Parameters**: `type` (required, or `tranId`), `startTime`, `endTime`, `current` (page, default 1), `size` (page size, default 10, max 100)

- **Response**: `{ "total": N, "rows": [{ "asset", "amount", "type", "status": "CONFIRMED", "tranId", "timestamp" }] }`

#### Main Account Internal Transfer
- **Endpoint**: `POST /openApi/wallets/v1/capital/innerTransfer/apply`
- **Rate Limit**: 2/s per UID
- **Auth**: Yes (Master Account Only, Withdraw permission)
- **Parameters**: `coin`, `userAccountType` (1=UID, 2=phone, 3=email), `userAccount`, `amount`, `walletType` (1=Fund, 2=Standard Futures, 3=Perpetual Futures, 4=Spot)

#### Query Fund Account Assets
- **Endpoint**: `GET /openApi/wallets/v1/capital/balance`
- **Rate Limit**: 5/s per UID
- **Auth**: Yes

#### Deposit Records
- **Endpoint**: `GET /openApi/wallets/v1/capital/deposit/hisrec`
- **Auth**: Yes

#### Withdraw Records
- **Endpoint**: `GET /openApi/wallets/v1/capital/withdraw/history`
- **Auth**: Yes

---

## WebSocket Streams

### Connection Rules
- All data is **GZIP compressed** — must decompress before use
- Heartbeat: server sends `Ping` every 5 seconds, client must respond with `Pong`
- Max **200 topics** per connection
- Max **60 websocket connections** per IP
- Subscription rate limit: **10/s**

### Subscribe/Unsubscribe Format
```json
// Subscribe
{"id": "unique-id", "reqType": "sub", "dataType": "<symbol>@<channel>"}

// Unsubscribe
{"id": "unique-id", "reqType": "unsub", "dataType": "<symbol>@<channel>"}

// Success confirmation
{"id": "unique-id", "code": 0, "msg": ""}
```

### Public Channels (Swap)

#### Order Book Depth
- **dataType**: `<symbol>@depth<level>@<interval>`
- **Example**: `BTC-USDT@depth5@500ms`, `BTC-USDT@depth20@200ms`
- **Levels**: 5, 20, 50, 100
- **Intervals**: 200ms (BTC-USDT, ETH-USDT), 500ms (other contracts)
- **Push Data**:
```json
{
  "code": 0,
  "dataType": "BTC-USDT@depth5@500ms",
  "ts": 1759996742928,
  "data": {
    "bids": [["121825.7", "7.7622"], ["121825.6", "0.0060"]],
    "asks": [["121827.3", "0.0025"], ["121827.1", "0.0010"]]
  }
}
```

#### Latest Trade Detail
- **dataType**: `<symbol>@trade`
- **Example**: `BTC-USDT@trade`
- **Push frequency**: Real-time
- **Push Data**:
```json
{
  "code": 0,
  "dataType": "BTC-USDT@trade",
  "data": [
    {"q": "0.0162", "p": "121934.9", "T": 1759994162450, "m": false, "s": "BTC-USDT"},
    {"q": "0.0551", "p": "121926.9", "T": 1759994162450, "m": true, "s": "BTC-USDT"}
  ]
}
```
- Fields: `q` (volume), `p` (price), `T` (trade time ms), `m` (true=buyer is maker), `s` (symbol)

#### K-Line Data
- **dataType**: `<symbol>@kline_<interval>`
- **Example**: `BTC-USDT@kline_1m`
- **Intervals**: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
- **Push Data**:
```json
{
  "code": 0,
  "dataType": "BTC-USDT@kline_1m",
  "s": "BTC-USDT",
  "data": [{"c": "121697.7", "o": "121684.1", "h": "121697.7", "l": "121638.7", "v": "5.8244", "T": 1759997460000}]
}
```
- Fields: `c` (close), `o` (open), `h` (high), `l` (low), `v` (volume), `T` (timestamp)

#### 24-Hour Ticker
- **dataType**: `<symbol>@ticker`
- **Example**: `BTC-USDT@ticker`
- **Push frequency**: Every 1 second
- **Push Data**:
```json
{
  "code": 0,
  "dataType": "BTC-USDT@ticker",
  "data": {
    "e": "24hTicker",
    "E": 1759998119388,
    "s": "BTC-USDT",
    "p": "-262.0",
    "P": "-0.21",
    "c": "121621.7",
    "L": "0.0010",
    "h": "124152.9",
    "l": "121412.0",
    "v": "17030.6463",
    "q": "210028.10",
    "o": "121883.7",
    "O": 1759998027573,
    "C": 1759998119403,
    "A": "121621.7",
    "a": "11.3115",
    "B": "121621.5",
    "b": "6.4426"
  }
}
```
- Fields: `e` (event type), `E` (event time), `s` (symbol), `p` (price change), `P` (% change), `c` (last price), `L` (last qty), `h` (high), `l` (low), `v` (volume), `q` (turnover USDT), `o` (open), `O`/`C` (open/close time), `A`/`a` (ask price/qty), `B`/`b` (bid price/qty)

#### Latest Price
- **dataType**: `<symbol>@lastPrice`
- **Push frequency**: Real-time
- **Push Data**:
```json
{"code": 0, "dataType": "BTC-USDT@lastPrice", "data": {"e": "lastPriceUpdate", "E": 1760001029063, "s": "BTC-USDT", "c": "121485.9"}}
```

#### Mark Price
- **dataType**: `<symbol>@markPrice`
- **Push frequency**: Real-time
- **Push Data**:
```json
{"code": 0, "dataType": "BTC-USDT@markPrice", "data": {"e": "markPriceUpdate", "E": 1760001628357, "s": "BTC-USDT", "p": "121570.0"}}
```

#### Book Ticker (Best Bid/Ask)
- **dataType**: `<symbol>@bookTicker`
- **Push frequency**: Every 200ms
- **Push Data**:
```json
{
  "code": 0,
  "dataType": "BTC-USDT@bookTicker",
  "data": {
    "e": "bookTicker",
    "u": 578534658,
    "E": 1760001840686,
    "T": 1760001840687,
    "s": "BTC-USDT",
    "b": "121584.1",
    "B": "18.7084",
    "a": "121584.3",
    "A": "4.9602"
  }
}
```
- Fields: `u` (update ID), `E` (event push time), `T` (match time), `b`/`B` (bid price/qty), `a`/`A` (ask price/qty)

### Private Channels (Swap)

Private WebSocket channels require a **Listen Key**. Connect to:
```
wss://open-api-swap.bingx.com/swap-market?listenKey=<your-listen-key>
```

No channel subscription needed — all private events are pushed automatically after connecting with listenKey.

#### Listen Key Management

| Operation | Method | Endpoint | Rate Limit |
|---|---|---|---|
| Generate | POST | `/openApi/user/auth/userDataStream` | 2/s |
| Extend (60min) | PUT | `/openApi/user/auth/userDataStream` | 2/s |
| Close | DELETE | `/openApi/user/auth/userDataStream` | 2/s |

- Listen key valid for **1 hour**; recommended to extend every **30 minutes**
- Generate requires `X-BX-APIKEY` header
- Extend/Close requires `listenKey` parameter

#### Order Update Push
- **Event**: Pushed when order status changes
- **Push Data Fields**:

| Field | Type | Description |
|---|---|---|
| s | string | Symbol (e.g. LINK-USDT) |
| c | string | Client order ID |
| i | int64 | Order ID |
| S | string | Side: `BUY` / `SELL` |
| o | string | Order type (LIMIT, MARKET, etc.) |
| q | string | Order quantity |
| p | string | Order price |
| sp | string | Stop/trigger price |
| ap | string | Average filled price |
| x | string | Execution type: `NEW`, `TRADE`, `CANCELED`, `EXPIRED` |
| X | string | Current order status: `NEW`, `PARTIALLY_FILLED`, `FILLED`, `CANCELED` |
| N | string | Fee asset (e.g. USDT) |
| n | string | Fee amount (may be negative) |
| T | int64 | Trade time (ms) |
| wt | string | Trigger price type: `MARK_PRICE` / `CONTRACT_PRICE` / `INDEX_PRICE` |
| ps | string | Position side: `LONG` / `SHORT` / `BOTH` |
| rp | string | Realized PnL for this trade |
| z | string | Cumulative filled quantity |
| sg | boolean | Guaranteed TP/SL enabled |
| ro | boolean | Reduce-only flag |
| td | string | Trade ID |

#### Account Balance and Position Update Push
- **Event Type**: `ACCOUNT_UPDATE`
- Pushed when account info changes (funds, positions)
- Only pushed for the symbols/assets that changed
- **Reason field** `m`: `DEPOSIT`, `WITHDRAW`, `ORDER`, `FUNDING_FEE`
- For FUNDING_FEE in cross mode: only pushes asset balance (B), not positions
- For FUNDING_FEE in isolated mode: pushes both relevant asset balance (B) and affected position (P)

---

## Error Codes

### HTTP Error Codes

| Code | Description |
|---|---|
| 400 | Bad Request — Invalid request format |
| 401 | Unauthorized — Invalid API Key |
| 403 | Forbidden — No access (or network firewall ban) |
| 404 | Not Found |
| 418 | IP banned (continued access after 429) |
| 429 | Too Many Requests — Rate limited |
| 500 | Internal Server Error |
| 504 | Gateway Timeout |

### Common Error Codes (All Endpoints)

| Code | Description |
|---|---|
| 100001 | Signature verification failed |
| 100004 | API key lacks trading permission |
| 100400 | Request routing error (bad path or method) |
| 100404 | API path does not exist or wrong method |
| 100410 | Rate limit exceeded |
| 100412 | Missing signature parameter |
| 100413 | API Key incorrect or missing X-BX-APIKEY header |
| 100419 | Request IP not in whitelist |
| 100421 | Timestamp null or mismatch with server time |
| 100500 | System busy |

### Futures Error Codes (Key Ones)

| Code | Description |
|---|---|
| 101201 | Position too large for requested leverage |
| 101202 | Order value below contract minimum |
| 101204 | Insufficient margin |
| 101205 | No position exists to close |
| 101206 | Insufficient available balance |
| 101209 | Max position value for leverage reached |
| 101211 | Order price out of allowed range |
| 101214 | Funding fee settlement in progress |
| 101215 | PostOnly order would have been filled immediately |
| 101290 | Reduce Only order exceeds current position size |
| 101400 | Order parameter validation failed |
| 101414 | Leverage exceeds limit |
| 101419 | Pending orders reached upper limit |
| 101429 | Order exceeds position limit |
| 101460 | Long order price must be > liquidation price |
| 101461 | Short order price must be < liquidation price |
| 101481 | clientOrderID already used |
| 104103 | Cannot switch position mode while positions/orders exist |
| 109201 | Duplicate order number submitted |
| 109400 | Invalid request parameters (symbol format, missing params, etc.) |
| 109403 | Blocked by risk control |
| 109420 | No position exists for symbol |
| 109421 | Order does not exist |
| 109425 | Trading pair does not exist |
| 109429 | Too many invalid requests, temporarily restricted |
| 109500 | Internal server error |
| 110203 | Insufficient closing amount |
| 110206 | TP/SL orders reached max limit |

### Spot Error Codes

| Code | Description |
|---|---|
| 100001 | Signature verification failed |
| 100202 | Insufficient balance |
| 100400 | Invalid parameter |
| 100440 | Order price deviates from market price |
| 100500 | Internal server error |
| 100503 | Server busy |

---

## Notes for Implementation

### Response Wrapper
- All responses wrapped in `{"code": 0, "msg": "", "data": ...}`
- Check `code == 0` for success; non-zero indicates error
- Some spot endpoints also include `debugMsg`

### Symbol Format
- **BingX uses**: `BTC-USDT` (hyphen-separated)
- **Mapping from internal format**: `BTCUSDT` -> `BTC-USDT` (insert hyphen before USDT/USDC)
- Must use uppercase

### Key Differences from Other Exchanges
1. **Signature**: HMAC-SHA256 with sorted query params (similar to Binance but uses `X-BX-APIKEY` header instead of `X-MBX-APIKEY`)
2. **WebSocket compression**: Uses GZIP (not raw/deflate) — must decompress all WS messages
3. **WebSocket heartbeat**: Text-based `Ping`/`Pong` (not binary frames)
4. **Funding rate intervals**: Configurable per contract (1h, 2h, 4h, 8h) — check `fundingIntervalHours` field
5. **Cancel order**: Uses `DELETE` method (not POST)
6. **Position mode**: Hedge mode (dual) / One-way mode (single) — set via `/openApi/swap/v1/positionSide/dual`
7. **Quantity**: Only COIN quantity supported for orders (not USDT quantity via `quantity` param), use `quoteOrderQty` for USDT-denominated
8. **Order IDs**: Very large int64 values — may need string handling in some languages

### Enum Values Summary

**Order Side**: `BUY`, `SELL`

**Position Side**: `LONG`, `SHORT`, `BOTH`

**Order Type**: `LIMIT`, `MARKET`, `STOP_MARKET`, `TAKE_PROFIT_MARKET`, `STOP`, `TAKE_PROFIT`, `TRIGGER_LIMIT`, `TRIGGER_MARKET`, `TRAILING_STOP_MARKET`, `TRAILING_TP_SL`

**Time in Force**: `GTC`, `IOC`, `FOK`, `PostOnly`

**Margin Type**: `CROSSED`, `ISOLATED`

**Working Type**: `MARK_PRICE`, `CONTRACT_PRICE`, `INDEX_PRICE`

**Order Status**: `NEW`, `PENDING`, `PARTIALLY_FILLED`, `FILLED`, `CANCELED`, `CANCELLED`, `EXPIRED`

**Income Type**: `TRANSFER`, `REALIZED_PNL`, `FUNDING_FEE`, `TRADING_FEE`, `INSURANCE_CLEAR`, `TRIAL_FUND`, `ADL`, `SYSTEM_DEDUCTION`

**Transfer Type**: `FUND_SFUTURES`, `SFUTURES_FUND`, `FUND_PFUTURES`, `PFUTURES_FUND`, `SFUTURES_PFUTURES`, `PFUTURES_SFUTURES`

**Wallet Type** (for internal transfer): 1=Fund Account, 2=Standard Futures, 3=Perpetual Futures, 4=Spot Account

### WebSocket dataType Patterns (Swap)
- Depth: `<symbol>@depth<level>@<interval>` (e.g. `BTC-USDT@depth5@500ms`)
- Trade: `<symbol>@trade`
- Kline: `<symbol>@kline_<interval>` (e.g. `BTC-USDT@kline_1m`)
- Ticker: `<symbol>@ticker`
- Last Price: `<symbol>@lastPrice`
- Mark Price: `<symbol>@markPrice`
- Book Ticker: `<symbol>@bookTicker`
- Incremental Depth: `<symbol>@depth@500ms` (incremental updates)

### Default Fee Rates
- Maker: 0.0002 (0.02%)
- Taker: 0.0005 (0.05%)
- Fees may vary by VIP level
