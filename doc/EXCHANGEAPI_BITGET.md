# Bitget API Documentation Reference

## Base URLs

| Type | URL | Description |
|------|-----|-------------|
| REST API | `https://api.bitget.com` | Main Domain |
| WebSocket Public | `wss://ws.bitget.com/v2/ws/public` | Public channels (ticker, depth, trades) |
| WebSocket Private | `wss://ws.bitget.com/v2/ws/private` | Private channels (account, orders, positions) |

## Authentication

### REST API Headers

All private REST requests must include these HTTP headers:

| Header | Description |
|--------|-------------|
| `ACCESS-KEY` | Your API Key string |
| `ACCESS-SIGN` | Base64-encoded HMAC-SHA256 signature |
| `ACCESS-TIMESTAMP` | Timestamp in milliseconds since epoch |
| `ACCESS-PASSPHRASE` | Password set when creating the API Key |
| `Content-Type` | `application/json` (for POST requests) |
| `locale` | Language: `en-US`, `zh-CN` |

### Signature Generation (HMAC-SHA256)

**Sign string format:**
```
timestamp + method.toUpperCase() + requestPath + "?" + queryString + body
```

- `timestamp`: Same as ACCESS-TIMESTAMP (milliseconds)
- `method`: `GET` or `POST` (uppercase)
- `requestPath`: e.g. `/api/v2/mix/order/place-order`
- `queryString`: URL query params (after `?`), omit `?` if empty
- `body`: JSON body string (omit for GET requests)

**Steps:**
1. Concatenate: `timestamp + method + requestPath [+ "?" + queryString] [+ body]`
2. HMAC-SHA256 encrypt with `secretKey`
3. Base64 encode the result

**Example (GET):**
```
16273667805456GET/api/mix/v2/market/depth?limit=20&symbol=BTCUSDT
```

**Example (POST):**
```
16273667805456POST/api/v2/mix/order/place-order{"productType":"usdt-futures","symbol":"BTCUSDT","size":"8","marginMode":"crossed","side":"buy","orderType":"limit","clientOid":"channel#123456"}
```

### WebSocket Login

```json
{
  "op": "login",
  "args": [{
    "apiKey": "<api_key>",
    "passphrase": "<passphrase>",
    "timestamp": "<timestamp_seconds>",
    "sign": "<sign>"
  }]
}
```

**WS Sign:** `CryptoJS.enc.Base64.Stringify(CryptoJS.HmacSHA256(timestamp + 'GET' + '/user/verify', secretKey))`

- `timestamp`: Unix timestamp in **seconds** (not milliseconds like REST)
- `method`: always `GET`
- `requestPath`: always `/user/verify`
- Login expires 30 seconds after timestamp

## Rate Limits

- **Global**: 6000 requests/IP/minute
- **HTTP 429**: Returned when rate limit exceeded

### Endpoint-Specific Limits

| Endpoint Category | Rate Limit |
|-------------------|------------|
| Market Data (GET) | 20 req/sec/IP |
| Account (GET) | 10 req/sec/UID |
| Position (GET all) | 5 req/sec/UID |
| Position (GET single) | 10 req/sec/UID |
| Trade (POST) | 10 req/sec/UID |
| Set Leverage | 5 req/sec/UID |
| Set Margin Mode | 5 req/sec/UID |
| Set Position Mode | 5 req/sec/UID |

### WebSocket Limits

- **Connection limit**: 300 connections/IP/5min, max 100 connections/IP
- **Subscription limit**: 240 subscriptions/hour/connection, max 1000 channels/connection
- **Message limit**: 10 messages/sec/connection
- **Ping/pong**: Send `"ping"` every 30 seconds, expect `"pong"`. Disconnect after 2 min without ping.
- **Recommendation**: Subscribe to < 50 channels per connection

## Response Wrapper Structure

**IMPORTANT**: All Bitget API responses are wrapped in this structure:

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695793701269,
  "data": { ... }
}
```

- `code`: `"00000"` = success, anything else = error
- `msg`: Human-readable message
- `requestTime`: Server timestamp in milliseconds
- `data`: The actual response payload (object, array, or null)

## Product Types

All futures endpoints require a `productType` parameter:

| Value | Description |
|-------|-------------|
| `USDT-FUTURES` | USDT-Margined Futures (settled in USDT) |
| `COIN-FUTURES` | Coin-Margined Futures (settled in crypto) |
| `USDC-FUTURES` | USDC-Margined Futures (settled in USDC) |

## REST API Endpoints

---

### Contract - Market Data

#### Get Contract Config
- **Endpoint**: `GET /api/v2/mix/market/contracts`
- **Rate Limit**: 20 req/sec/IP
- **Auth**: No

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | No | Trading pair, e.g. `BTCUSDT` |
| productType | String | Yes | `USDT-FUTURES`, `COIN-FUTURES`, `USDC-FUTURES` |

**Response:**
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695793701269,
  "data": [{
    "symbol": "BTCUSDT",
    "baseCoin": "BTC",
    "quoteCoin": "USDT",
    "makerFeeRate": "0.0004",
    "takerFeeRate": "0.0006",
    "supportMarginCoins": ["USDT"],
    "minTradeNum": "0.01",
    "volumePlace": "2",
    "pricePlace": "1",
    "sizeMultiplier": "0.01",
    "symbolType": "perpetual",
    "minTradeUSDT": "5",
    "symbolStatus": "normal",
    "fundInterval": "8",
    "minLever": "1",
    "maxLever": "125",
    "maxMarketOrderQty": "220",
    "maxOrderQty": "1200"
  }]
}
```

**Key Response Fields:**
- `volumePlace`: Decimal places for quantity
- `pricePlace`: Decimal places for price
- `sizeMultiplier`: Order quantity must be a multiple of this
- `minTradeNum`: Minimum order quantity (base coin)
- `minTradeUSDT`: Minimum order value in USDT
- `fundInterval`: Funding interval in hours (`1`, `2`, `4`, `8`)
- `symbolStatus`: `normal`, `maintain`, `limit_open`, `restrictedAPI`, `off`

#### Get Ticker (Single)
- **Endpoint**: `GET /api/v2/mix/market/ticker`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| productType | String | Yes | Product type |

**Response:**
```json
{
  "code": "00000",
  "data": [{
    "symbol": "BTCUSDT",
    "lastPr": "29904.5",
    "askPr": "29904.5",
    "bidPr": "29903.5",
    "bidSz": "0.5091",
    "askSz": "2.2694",
    "high24h": "30100",
    "low24h": "29500",
    "ts": "1695794098184",
    "change24h": "0.013",
    "baseVolume": "12345",
    "quoteVolume": "369000000",
    "usdtVolume": "369000000",
    "openUtc": "29800",
    "changeUtc24h": "0.005",
    "indexPrice": "29132.35",
    "fundingRate": "-0.0007",
    "holdingAmount": "125.6844",
    "open24h": "29500",
    "markPrice": "29905"
  }]
}
```

#### Get All Tickers
- **Endpoint**: `GET /api/v2/mix/market/tickers`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |

Returns same fields as Get Ticker but for all symbols.

#### Get Merge Market Depth (Orderbook)
- **Endpoint**: `GET /api/v2/mix/market/merge-depth`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| precision | String | No | `scale0` (default, unmerged), `scale1`, `scale2`, `scale3` |
| limit | String | No | `1`, `5`, `15`, `50`, `max` (default 100) |

**Response:**
```json
{
  "code": "00000",
  "data": {
    "asks": [["26347.5", "0.25"], ["26348.0", "0.16"]],
    "bids": [["26346.5", "0.16"], ["26346.0", "0.32"]],
    "ts": "1695870968804",
    "scale": "0.1",
    "precision": "scale0",
    "isMaxPrecision": "NO"
  }
}
```

Each entry: `[price, quantity]`

#### Get Mark/Index/Market Prices
- **Endpoint**: `GET /api/v2/mix/market/symbol-price`
- **Rate Limit**: 20 req/sec/UID

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |

**Response:**
```json
{
  "data": [{
    "symbol": "BTCUSDT",
    "price": "26242",
    "indexPrice": "34867",
    "markPrice": "25555",
    "ts": "1695793390482"
  }]
}
```

#### Get Current Funding Rate
- **Endpoint**: `GET /api/v2/mix/market/current-fund-rate`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | No | Trading pair (omit for all) |
| productType | String | Yes | Product type |

**Response:**
```json
{
  "data": [{
    "symbol": "BTCUSDT",
    "fundingRate": "0.000068",
    "fundingRateInterval": "8",
    "nextUpdate": "1743062400000",
    "minFundingRate": "-0.003",
    "maxFundingRate": "0.003"
  }]
}
```

- `fundingRateInterval`: Settlement period in hours (`1`, `2`, `4`, `8`)
- `fundingRate`: Decimal form (0.000068 = 0.0068%)

#### Get Historical Funding Rates
- **Endpoint**: `GET /api/v2/mix/market/history-fund-rate`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| pageSize | String | No | Default 20, max 100 |
| pageNo | String | No | Page number |

**Response:**
```json
{
  "data": [{
    "symbol": "BTCUSDT",
    "fundingRate": "0.0005",
    "fundingTime": "1695776400000"
  }]
}
```

#### Get Next Funding Time
- **Endpoint**: `GET /api/v2/mix/market/funding-time`
- **Rate Limit**: 20 req/sec/IP

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |

**Response:**
```json
{
  "data": [{
    "symbol": "BTCUSDT",
    "nextFundingTime": "1695801600000",
    "ratePeriod": "8"
  }]
}
```

---

### Contract - Account

#### Get Single Account
- **Endpoint**: `GET /api/v2/mix/account/account`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| marginCoin | String | Yes | Margin coin, e.g. `USDT` |

**Response:**
```json
{
  "data": {
    "marginCoin": "USDT",
    "locked": "0",
    "available": "10575.26",
    "crossedMaxAvailable": "10580.56",
    "isolatedMaxAvailable": "10580.56",
    "maxTransferOut": "10572.92",
    "accountEquity": "10582.90",
    "usdtEquity": "10582.90",
    "btcEquity": "0.204885",
    "crossedRiskRate": "0",
    "crossedMarginLeverage": 14,
    "isolatedLongLever": 14,
    "isolatedShortLever": 14,
    "marginMode": "crossed",
    "posMode": "hedge_mode",
    "unrealizedPL": "-267.64",
    "coupon": "0",
    "crossedUnrealizedPL": "-19.48",
    "isolatedUnrealizedPL": "",
    "assetMode": "union"
  }
}
```

**Key Fields:**
- `marginMode`: `isolated` or `crossed`
- `posMode`: `one_way_mode` or `hedge_mode`
- `assetMode`: `union` (multi-asset) or `single`

#### Get Account List
- **Endpoint**: `GET /api/v2/mix/account/accounts`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |

Returns array of account info for all margin coins under the product type.

#### Get Account Bills
- **Endpoint**: `GET /api/v2/mix/account/bill`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |
| coin | String | No | Currency filter |
| businessType | String | No | Filter by business type |
| startTime | String | No | Start time (ms) |
| endTime | String | No | End time (ms), max 30 day span |
| limit | String | No | Max 100, default 20 |
| idLessThan | String | No | Pagination: before this bill ID |

**Key businessType values:**
- `open_long`, `open_short`, `close_long`, `close_short`
- `contract_settle_fee` (funding fee)
- `trans_from_exchange`, `trans_to_exchange` (transfers)
- `force_close_long`, `force_close_short` (liquidation)
- `append_margin`, `reduce_margin`

#### Change Margin Mode
- **Endpoint**: `POST /api/v2/mix/account/set-margin-mode`
- **Rate Limit**: 5 req/sec/UID
- **Auth**: Yes
- **Note**: Cannot change when open positions or orders exist

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| marginCoin | String | Yes | Margin coin (capitalized) |
| marginMode | String | Yes | `isolated` or `crossed` |

#### Change Leverage
- **Endpoint**: `POST /api/v2/mix/account/set-leverage`
- **Rate Limit**: 5 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| marginCoin | String | Yes | Margin coin (capitalized) |
| leverage | String | No | Leverage (for cross-margin, or one-way isolated) |
| longLeverage | String | No | Long leverage (hedge-mode isolated only) |
| shortLeverage | String | No | Short leverage (hedge-mode isolated only) |
| holdSide | String | No | `long` or `short` (required for hedge-mode isolated) |

**Note:** In cross-margin mode, use `leverage` (not `longLeverage`/`shortLeverage`).

#### Change Position Mode
- **Endpoint**: `POST /api/v2/mix/account/set-position-mode`
- **Rate Limit**: 5 req/sec/UID
- **Auth**: Yes
- **Note**: Cannot change when positions/orders exist under the product type

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |
| posMode | String | Yes | `one_way_mode` or `hedge_mode` |

---

### Contract - Position

#### Get Single Position
- **Endpoint**: `GET /api/v2/mix/position/single-position`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |
| symbol | String | Yes | Trading pair |
| marginCoin | String | Yes | Margin coin (capitalized) |

#### Get All Positions
- **Endpoint**: `GET /api/v2/mix/position/all-position`
- **Rate Limit**: 5 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| productType | String | Yes | Product type |
| marginCoin | String | No | Margin coin filter |

**Response (both endpoints):**
```json
{
  "data": [{
    "marginCoin": "USDT",
    "symbol": "BTCUSDT",
    "holdSide": "short",
    "openDelegateSize": "0",
    "marginSize": "103.19",
    "available": "0.0155",
    "locked": "0",
    "total": "0.0155",
    "leverage": "14",
    "achievedProfits": "8.605",
    "openPriceAvg": "88505.23",
    "marginMode": "crossed",
    "posMode": "hedge_mode",
    "unrealizedPL": "-72.82",
    "liquidationPrice": "5737867.86",
    "keepMarginRate": "0.004",
    "markPrice": "93203.4",
    "marginRatio": "0.00285",
    "breakEvenPrice": "85283.07",
    "totalFee": "-56.83",
    "deductedFee": "0.924",
    "assetMode": "union",
    "autoMargin": "off",
    "takeProfit": "",
    "stopLoss": "",
    "cTime": "1766103799183",
    "uTime": "1767682800537"
  }]
}
```

**Key Fields:**
- `holdSide`: `long` or `short`
- `total`: Total position size = `available` + `locked`
- `available`: Size that can be closed
- `openPriceAvg`: Average entry price
- `totalFee`: Accumulated funding fees during position
- `deductedFee`: Accumulated transaction fees during position
- `achievedProfits`: Realized PnL (excludes funding fee and tx fee)
- `liquidationPrice`: If <= 0, low risk / no liquidation price
- `marginRatio`: Maintenance margin rate (0.1 = 10%)

---

### Contract - Trade

#### Place Order
- **Endpoint**: `POST /api/v2/mix/order/place-order`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair, e.g. `ETHUSDT` |
| productType | String | Yes | Product type |
| marginMode | String | Yes | `isolated` or `crossed` |
| marginCoin | String | Yes | Margin coin (capitalized) |
| size | String | Yes | Amount in base coin |
| price | String | No | Required for `limit` orders |
| side | String | Yes | `buy` or `sell` |
| tradeSide | String | No | Required in hedge-mode: `open` or `close` |
| orderType | String | Yes | `limit` or `market` |
| force | String | No | `ioc`, `fok`, `gtc` (default), `post_only`. Required for limit orders |
| clientOid | String | No | Custom order ID |
| reduceOnly | String | No | `YES` or `NO` (default). One-way mode only |

**Hedge-mode order sides:**
- Open long: `side=buy`, `tradeSide=open`
- Close long: `side=buy`, `tradeSide=close`
- Open short: `side=sell`, `tradeSide=open`
- Close short: `side=sell`, `tradeSide=close`

**One-way mode:** `side=buy` or `side=sell`, ignore `tradeSide`

**Response:**
```json
{
  "code": "00000",
  "data": {
    "clientOid": "121211212122",
    "orderId": "121211212122"
  }
}
```

#### Cancel Order
- **Endpoint**: `POST /api/v2/mix/order/cancel-order`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair |
| productType | String | Yes | Product type |
| marginCoin | String | No | Margin coin (capitalized) |
| orderId | String | No | Order ID (either orderId or clientOid required) |
| clientOid | String | No | Custom order ID |

#### Batch Cancel Orders
- **Endpoint**: `POST /api/v2/mix/order/batch-cancel-orders`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderIdList | List | No | Max 50 items. Requires `symbol` if set |
| > orderId | String | No | Order ID (either orderId or clientOid required) |
| > clientOid | String | No | Custom order ID |
| symbol | String | No | Required when `orderIdList` is set |
| productType | String | Yes | Product type |
| marginCoin | String | No | Margin coin |

**Response:**
```json
{
  "data": {
    "successList": [{"orderId": "123", "clientOid": "abc"}],
    "failureList": [{"orderId": "456", "clientOid": "def", "errorMsg": "notExistend", "errorCode": "40102"}]
  }
}
```

#### Get Order Detail
- **Endpoint**: `GET /api/v2/mix/order/detail`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | Yes | Trading pair (capitalized) |
| productType | String | Yes | Product type |
| orderId | String | No | Order ID (either required) |
| clientOid | String | No | Custom order ID (either required) |

**Response key fields:**
- `state`/`status`: `live`, `partially_filled`, `filled`, `canceled`
- `side`: `buy`, `sell`
- `posSide`: `long`, `short`, `net`
- `tradeSide`: `open`, `close`
- `orderType`: `limit`, `market`
- `force`: `ioc`, `fok`, `gtc`, `post_only`
- `priceAvg`: Average filled price
- `baseVolume`: Filled quantity

#### Get Order Fill Details
- **Endpoint**: `GET /api/v2/mix/order/fills`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderId | String | No | Order ID filter |
| symbol | String | No | Trading pair |
| productType | String | Yes | Product type |
| startTime | String | No | Start time (ms), max 3 month span |
| endTime | String | No | End time (ms) |
| limit | String | No | Max 100, default 100 |
| idLessThan | String | No | Pagination: before this trade ID |

**Response:**
```json
{
  "data": {
    "fillList": [{
      "tradeId": "123",
      "symbol": "ethusdt",
      "orderId": "121212",
      "price": "1900",
      "baseVolume": "1",
      "feeDetail": [{
        "deduction": "yes",
        "feeCoin": "BGB",
        "totalDeductionFee": "-0.0171",
        "totalFee": "-0.0171"
      }],
      "side": "buy",
      "quoteVolume": "1902",
      "profit": "102",
      "enterPointSource": "api",
      "tradeSide": "close",
      "posMode": "hedge_mode",
      "tradeScope": "taker",
      "cTime": "1627293509612"
    }],
    "endId": "123"
  }
}
```

**IMPORTANT - feeDetail structure:**
The `feeDetail` field is an **array of objects**, not a simple value. Each element contains:
- `deduction`: Whether fee deduction voucher was used (`"yes"` / `"no"`)
- `feeCoin`: Fee currency (e.g. `"BGB"`, `"USDT"`)
- `totalDeductionFee`: Deducted fee amount
- `totalFee`: **Total transaction fee** (this is the actual fee to use)

**Note:** Always access `feeDetail[0].totalFee` to get the transaction fee. This caused a previous bug when parsing incorrectly.

#### Get History Orders
- **Endpoint**: `GET /api/v2/mix/order/orders-history`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes
- **Note**: Only supports data within 90 days

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderId | String | No | Filter by order ID |
| clientOid | String | No | Filter by custom order ID |
| symbol | String | No | Trading pair |
| productType | String | Yes | Product type |
| startTime | String | No | Start time (ms) |
| endTime | String | No | End time (ms) |
| limit | String | No | Max 100, default 100 |
| idLessThan | String | No | Pagination |

**Response:**
```json
{
  "data": {
    "entrustedList": [{
      "symbol": "ethusdt",
      "size": "100",
      "orderId": "123",
      "clientOid": "12321",
      "baseVolume": "12.1",
      "fee": "-0.00854",
      "price": "1900",
      "priceAvg": "1903",
      "status": "filled",
      "side": "buy",
      "force": "gtc",
      "totalProfits": "0",
      "posSide": "long",
      "marginCoin": "usdt",
      "leverage": "20",
      "marginMode": "crossed",
      "tradeSide": "open",
      "posMode": "hedge_mode",
      "orderType": "limit",
      "orderSource": "normal",
      "cTime": "1627293504612",
      "uTime": "1627293505612"
    }],
    "endId": "123"
  }
}
```

**Note**: History orders response wraps the list in `entrustedList` (not just `data`).

---

### Spot - Account/Wallet

#### Get Account Assets (Spot Balance)
- **Endpoint**: `GET /api/v2/spot/account/assets`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| coin | String | No | Token name, e.g. `USDT` |
| assetType | String | No | `hold_only` (default) or `all` |

**Response:**
```json
{
  "data": [{
    "coin": "usdt",
    "available": "1000",
    "frozen": "0",
    "locked": "0",
    "limitAvailable": "0",
    "uTime": "1622697148"
  }]
}
```

#### Transfer Between Accounts
- **Endpoint**: `POST /api/v2/spot/wallet/transfer`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| fromType | String | Yes | Source account type |
| toType | String | Yes | Destination account type |
| amount | String | Yes | Amount to transfer |
| coin | String | Yes | Currency |
| symbol | String | Yes | Required for isolated margin transfers |
| clientOid | String | No | Custom order ID (idempotent) |

**Account types for fromType/toType:**
- `spot`: Spot account
- `p2p`: P2P/funding account
- `coin_futures`: Coin-M futures account
- `usdt_futures`: USDT-M futures account
- `usdc_futures`: USDC-M futures account
- `crossed_margin`: Cross margin account
- `isolated_margin`: Isolated margin account

**Response:**
```json
{
  "data": {
    "transferId": "123456",
    "clientOid": "x123"
  }
}
```

#### Get Spot Account Bills
- **Endpoint**: `GET /api/v2/spot/account/bills`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| coin | String | No | Token name |
| groupType | String | No | `deposit`, `withdraw`, `transaction`, `transfer`, etc. |
| businessType | String | No | `DEPOSIT`, `WITHDRAW`, `BUY`, `SELL`, `TRANSFER_IN`, `TRANSFER_OUT`, etc. |
| startTime | String | No | Start time (ms), max 90 day span |
| endTime | String | No | End time (ms) |
| limit | String | No | Default 100, max 500 |

#### Get Transfer Records
- **Endpoint**: `GET /api/v2/spot/account/transferRecords`
- **Rate Limit**: 20 req/sec/UID
- **Auth**: Yes

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| coin | String | Yes | Token name |
| fromType | String | No | Account type filter |
| startTime | String | No | Start time (ms) |
| endTime | String | No | End time (ms), max 90 day span |
| limit | String | No | Default 100, max 500 |

---

## WebSocket Streams

### Connection & Authentication

**Public channels** (no auth): `wss://ws.bitget.com/v2/ws/public`
**Private channels** (auth required): `wss://ws.bitget.com/v2/ws/private`

**Subscribe format:**
```json
{
  "op": "subscribe",
  "args": [{
    "instType": "USDT-FUTURES",
    "channel": "<channel_name>",
    "instId": "<symbol_or_default>"
  }]
}
```

**Unsubscribe format:**
```json
{
  "op": "unsubscribe",
  "args": [{ "instType": "...", "channel": "...", "instId": "..." }]
}
```

### Public Channels

#### Ticker Channel
- **Channel**: `ticker`
- **instType**: `USDT-FUTURES`, `COIN-FUTURES`, `USDC-FUTURES`
- **instId**: Symbol, e.g. `BTCUSDT`
- **Push frequency**: 300-400ms on change

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "ticker", "instId": "BTCUSDT"}]
}
```

**Push data:**
```json
{
  "action": "snapshot",
  "arg": {"instType": "USDT-FUTURES", "instId": "BTCUSDT", "channel": "ticker"},
  "data": [{
    "lastPr": "87673.6",
    "symbol": "BTCUSDT",
    "indexPrice": "87714.07",
    "bidPr": "87673.6",
    "askPr": "87673.7",
    "bidSz": "6.91",
    "askSz": "14.33",
    "high24h": "88022.1",
    "low24h": "86542.5",
    "change24h": "0.00743",
    "baseVolume": "17398.16",
    "quoteVolume": "1521198076.61",
    "fundingRate": "0.000055",
    "nextFundingTime": "1766678400000",
    "holdingAmount": "28135.5456",
    "markPrice": "87673.7",
    "open24h": "87027.0",
    "ts": "1766674540816"
  }],
  "ts": 1766674540817
}
```

#### Depth Channel (Order Book)
- **Channel**: `books` (all levels), `books1` (1 level), `books5` (5 levels), `books15` (15 levels)
- **Push frequency**: `books`/`books5`/`books15`: 150ms; `books1`: 10ms
- **Action**: `books` sends `snapshot` first, then `update`; others always `snapshot`

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "books5", "instId": "BTCUSDT"}]
}
```

**Push data:**
```json
{
  "action": "snapshot",
  "arg": {"instType": "USDT-FUTURES", "channel": "books5", "instId": "BTCUSDT"},
  "data": [{
    "asks": [["27000.5", "8.760"], ["27001.0", "0.400"]],
    "bids": [["27000.0", "2.710"], ["26999.5", "1.460"]],
    "checksum": 0,
    "seq": 123,
    "ts": "1695716059516"
  }],
  "ts": 1695716059516
}
```

**Checksum (for `books` channel):**
Build string: `bid1[price:amount]:ask1[price:amount]:bid2[price:amount]:ask2[price:amount]...` (up to 25 levels), then calculate CRC32 (32-bit signed integer).

#### Trade Channel (Public Trades)
- **Channel**: `trade`
- **Push**: Real-time

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "trade", "instId": "BTCUSDT"}]
}
```

**Push data:**
```json
{
  "action": "snapshot",
  "data": [{
    "ts": "1695716760565",
    "price": "27000.5",
    "size": "0.001",
    "side": "buy",
    "tradeId": "1111111111"
  }]
}
```

#### Candlestick Channel
- **Channel**: `candle1m`, `candle5m`, `candle15m`, `candle30m`, `candle1H`, `candle4H`, `candle12H`, `candle1D`, `candle1W`, `candle1M`, etc.
- **Push**: Once per second during trades, otherwise at granularity intervals

**Push data array format:** `[timestamp, open, high, low, close, baseVolume, quoteVolume, usdtVolume]`

### Private Channels

#### Account Channel
- **Channel**: `account`
- **instId**: `default` (all coins)
- **Triggers**: Transfer, voucher deposit, order fills

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "account", "coin": "default"}]
}
```

**Push data:**
```json
{
  "action": "snapshot",
  "data": [{
    "marginCoin": "USDT",
    "frozen": "0.00000000",
    "available": "11.98545761",
    "maxOpenPosAvailable": "11.98545761",
    "maxTransferOut": "11.98545761",
    "equity": "11.98545761",
    "usdtEquity": "11.985457617660",
    "crossedRiskRate": "0",
    "unrealizedPL": "0.000000000000"
  }]
}
```

#### Position Channel
- **Channel**: `positions`
- **instId**: `default` (all symbols)
- **Triggers**: Open/close orders created, filled, or canceled

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "positions", "instId": "default"}]
}
```

**Push data:**
```json
{
  "action": "snapshot",
  "data": [{
    "posId": "1",
    "instId": "ETHUSDT",
    "marginCoin": "USDT",
    "marginSize": "9.5",
    "marginMode": "crossed",
    "holdSide": "short",
    "posMode": "hedge_mode",
    "total": "0.1",
    "available": "0.1",
    "frozen": "0",
    "openPriceAvg": "1900",
    "leverage": 20,
    "achievedProfits": "0",
    "unrealizedPL": "0",
    "unrealizedPLR": "0",
    "liquidationPrice": "5788.108",
    "keepMarginRate": "0.005",
    "breakEvenPrice": "24778.97",
    "totalFee": "1.45",
    "deductedFee": "0.388",
    "markPrice": "2500",
    "cTime": "1695649246169",
    "uTime": "1695711602568"
  }]
}
```

#### Order Channel
- **Channel**: `orders`
- **instId**: `default` (all) or specific symbol

```json
{
  "op": "subscribe",
  "args": [{"instType": "USDT-FUTURES", "channel": "orders", "instId": "default"}]
}
```

**Push data key fields:**
- `orderId`, `clientOid`: Order identifiers
- `price`, `size`: Order price/quantity
- `status`: `live`, `partially_filled`, `filled`, `canceled`
- `side`: `buy`, `sell`
- `posSide`: `long`, `short`, `net`
- `tradeSide`: `open`, `close`
- `orderType`: `limit`, `market`
- `leverage`, `marginMode`, `posMode`
- `accBaseVolume`: Total filled quantity
- `priceAvg`: Average filled price
- `fillPrice`: Latest fill price
- `feeDetail`: Array of `{feeCoin, fee}` objects
- `cancelReason`: `normal_cancel`, `stp_cancel`

---

## Error Codes

### Common Auth Errors

| Code | Message |
|------|---------|
| 40001 | ACCESS_KEY cannot be empty |
| 40002 | ACCESS_SIGN cannot be empty |
| 40005 | Invalid ACCESS_TIMESTAMP |
| 40006 | Invalid ACCESS_KEY |
| 40008 | Request timestamp expired |
| 40009 | Sign signature error |
| 40011 | ACCESS_PASSPHRASE cannot be empty |
| 40012 | apikey/password is incorrect |
| 40018 | Invalid IP |
| 40036 | passphrase is error |

### Trading Errors

| Code | Message |
|------|---------|
| 40072 | Symbol is invalid or not supported |
| 40102 | Symbol does not exist |
| 40304 | clientOid length cannot exceed 50 |
| 40305 | clientOid length cannot exceed 64 |
| 40306 | Batch processing orders max 20 |
| 40309 | The contract has been removed |
| 40706 | Wrong order price |
| 40714 | No direct margin call allowed |
| 40754 | Balance not enough |
| 40755 | Not enough open positions available |
| 40757 | Not enough position available |
| 40761 | Total number of unfilled orders too high |
| 40762 | Order size > max open size |
| 40844 | Contract under temporary maintenance |
| 40845 | Contract has been removed |

### Position/Margin Errors

| Code | Message |
|------|---------|
| 22001 | Leverage exceeds symbol maximum |
| 22002 | Order amount exceeds position limit |
| 22004 | Position does not exist |
| 22005 | Symbol does not support cross mode |
| 22034 | Less than minimum order amount |
| 22038 | Quantity must be integral multiple of {0} |
| 40730 | Cannot adjust leverage with open positions/orders |
| 40872 | Failed to adjust position: holding position or order |

### General Errors

| Code | Message |
|------|---------|
| 429 | Too many requests (rate limited) |
| 00001 | startTime and endTime interval too large |
| 40015 | System abnormal, try again later |
| 40017 | Parameter verification failed |
| 40019 | Parameter cannot be empty |
| 40020 | Parameter error |
| 40200 | Server upgrade, try again later |

---

## Notes for Implementation

### Data Wrapper Structure
All responses wrapped in `{code, msg, data, requestTime}`. Always check `code == "00000"` for success.

### Symbol Format
Bitget uses plain symbols: `BTCUSDT`, `ETHUSDT` (no separators, no suffixes). Same format for REST and WebSocket.

### Key Enum Values

**Position Mode (`posMode`):**
- `one_way_mode`: One-way position mode
- `hedge_mode`: Hedge (two-way) position mode

**Margin Mode (`marginMode`):**
- `isolated`: Isolated margin
- `crossed`: Cross margin

**Order Side (`side`):**
- `buy`, `sell`

**Trade Side (`tradeSide`, hedge-mode only):**
- `open`: Open position
- `close`: Close position

**Position Side (`posSide`):**
- `long`: Long position (hedge-mode)
- `short`: Short position (hedge-mode)
- `net`: Net position (one-way mode)

**Order Type (`orderType`):**
- `limit`: Limit order
- `market`: Market order

**Time in Force (`force`):**
- `gtc`: Good till canceled (default)
- `ioc`: Immediate or cancel
- `fok`: Fill or kill
- `post_only`: Post only (maker)

**Order Status:**
- `live`: New order, waiting for match
- `partially_filled`: Partially filled
- `filled`: Fully filled
- `canceled`: Canceled

### feeDetail Structure in Fill Responses
**CRITICAL**: The `feeDetail` field in order fills is an **array**, not a flat value:
```json
"feeDetail": [{
  "deduction": "yes",
  "feeCoin": "BGB",
  "totalDeductionFee": "-0.017",
  "totalFee": "-0.017"
}]
```
Access fee via `feeDetail[0].totalFee`. Previous bugs were caused by not properly unwrapping this array.

### WebSocket Push Actions
- `snapshot`: Full state (first push, or channels like `books5`)
- `update`: Incremental update (only for `books` channel)

### Funding Rate Format
- Returned as decimal: `0.000068` = 0.0068%
- `fundingRateInterval`: Hours between settlements (1, 2, 4, 8)

### Timestamp Format
- All timestamps are Unix milliseconds (13 digits) unless noted
- Exception: WebSocket login uses seconds (10 digits)

---

## Appendix: Additional Endpoints (Patched)

---

### Contract - Plan/Trigger Orders

#### Place Plan Order (Trigger/Conditional Order)
- **Endpoint**: `POST /api/v2/mix/order/place-plan-order`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

Place a trigger order, trailing stop order, or stop-loss/take-profit order.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| planType | String | Yes | `normal_plan`: Trigger order, `track_plan`: Trailing stop order |
| symbol | String | Yes | Trading pair, e.g. `ETHUSDT` |
| productType | String | Yes | `USDT-FUTURES`, `COIN-FUTURES`, `USDC-FUTURES` |
| marginMode | String | Yes | `isolated` or `crossed` |
| marginCoin | String | Yes | Margin coin (capitalized) |
| size | String | Yes | Amount (base coin) |
| price | String | No | Order price. Required for `limit` orders with `normal_plan`. Must be empty for `track_plan` |
| callbackRatio | String | No | Callback rate for trailing stop orders only (max 10) |
| triggerPrice | String | Yes | Trigger price |
| triggerType | String | Yes | `mark_price` or `fill_price` (latest price) |
| side | String | Yes | `buy` or `sell` |
| tradeSide | String | No | Required in hedge-mode: `open` or `close` |
| orderType | String | Yes | `limit` or `market`. For `track_plan`, must be `market` |
| clientOid | String | No | Custom order ID |
| reduceOnly | String | No | `YES` or `NO` (default). One-way mode only |
| stopSurplusTriggerPrice | String | No | Take-profit trigger price (for `normal_plan`) or TP percentage (for `track_plan`, 0.01–999.99) |
| stopSurplusExecutePrice | String | No | Take-profit execute price. Empty/0 = market order. Must be empty for `track_plan` |
| stopSurplusTriggerType | String | No | `fill_price` or `mark_price`. Required when `stopSurplusTriggerPrice` is set |
| stopLossTriggerPrice | String | No | Stop-loss trigger price (for `normal_plan`) or SL percentage (for `track_plan`, 0.01–999.99) |
| stopLossExecutePrice | String | No | Stop-loss execute price. Empty/0 = market order. Must be empty for `track_plan` |
| stopLossTriggerType | String | No | `fill_price` or `mark_price`. Required when `stopLossTriggerPrice` is set |
| stpMode | String | No | STP mode: `none` (default), `cancel_taker`, `cancel_maker`, `cancel_both` |

**Response:**
```json
{
  "code": "00000",
  "data": {
    "orderId": "121212121212",
    "clientOid": "BITGET#121212121212"
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

#### Cancel Plan Order
- **Endpoint**: `POST /api/v2/mix/order/cancel-plan-order`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

Cancel trigger/plan orders. Can cancel by `productType` + `symbol`, or by order ID list.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderIdList | List | No | Trigger order ID list. If passed, `symbol` must also be set |
| > orderId | String | No | Trigger order ID (either `orderId` or `clientOid` required) |
| > clientOid | String | No | Custom trigger order ID |
| symbol | String | No | Trading pair, e.g. `ETHUSDT` |
| productType | String | Yes | `USDT-FUTURES`, `COIN-FUTURES`, `USDC-FUTURES` |
| marginCoin | String | No | Margin coin (capitalized) |
| planType | String | No | `normal_plan` (default), `profit_plan`, `loss_plan`, `pos_profit`, `pos_loss`, `moving_plan` |

**Response:**
```json
{
  "code": "00000",
  "data": {
    "successList": [
      { "orderId": "121212121212", "clientOid": "123" }
    ],
    "failureList": [
      { "orderId": "3", "clientOid": "123", "errorMsg": "notExistend" }
    ]
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

**Response Fields:**
- `successList`: Array of successfully cancelled orders (`orderId`, `clientOid`)
- `failureList`: Array of failed cancellations (`orderId`, `clientOid`, `errorMsg`)

---

### Contract - Trade (Additional)

#### Get Pending Orders
- **Endpoint**: `GET /api/v2/mix/order/orders-pending`
- **Rate Limit**: 10 req/sec/UID
- **Auth**: Yes

Query all existing pending (open) orders.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderId | String | No | Order ID filter (takes precedence over `clientOid`) |
| clientOid | String | No | Custom order ID filter |
| symbol | String | No | Trading pair, e.g. `ETHUSDT` |
| productType | String | Yes | `USDT-FUTURES`, `COIN-FUTURES`, `USDC-FUTURES` |
| status | String | No | `live` (default, not filled yet), `partially_filled` |
| idLessThan | String | No | Pagination: before this ID (older data) |
| startTime | String | No | Start time (ms), max 3 month span |
| endTime | String | No | End time (ms), max 3 month span |
| limit | String | No | Max 100, default 100 |

**Response:**
```json
{
  "code": "00000",
  "data": {
    "entrustedList": [{
      "symbol": "ethusdt",
      "size": "100",
      "orderId": "123",
      "clientOid": "12321",
      "baseVolume": "12.1",
      "fee": "",
      "price": "1900",
      "priceAvg": "1903",
      "status": "partially_filled",
      "side": "buy",
      "force": "gtc",
      "totalProfits": "0",
      "posSide": "long",
      "marginCoin": "usdt",
      "quoteVolume": "22001.21",
      "leverage": "20",
      "marginMode": "cross",
      "enterPointSource": "api",
      "tradeSide": "open",
      "posMode": "hedge_mode",
      "orderType": "limit",
      "orderSource": "normal",
      "reduceOnly": "NO",
      "cTime": "1627293504612",
      "uTime": "1627293505612"
    }],
    "endId": "123"
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

**Key Response Fields:**
- `entrustedList`: Array of pending orders
- `status`: `live` (not filled) or `partially_filled`
- `side`: `buy` or `sell`
- `tradeSide`: `open` or `close`
- `force`: `ioc`, `fok`, `gtc`, `post_only`
- `enterPointSource`: `WEB`, `API`, `SYS`, `ANDROID`, `IOS`
- `priceAvg`: Average fill price (empty when `live`)
- `baseVolume`: Filled quantity so far
- `endId`: Pagination cursor for `idLessThan`

---

### Contract - Position (Additional)

#### Get Historical Position
- **Endpoint**: `GET /api/v2/mix/position/history-position`
- **Rate Limit**: 20 req/sec/UID
- **Auth**: Yes
- **Note**: Only supports data within 3 months

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | String | No | Trading pair filter |
| productType | String | No | Default `USDT-FUTURES`. Ignored if `symbol` is set |
| idLessThan | String | No | Pagination: before this ID (older data) |
| startTime | String | No | Start time (ms), max 3 month span |
| endTime | String | No | End time (ms), max 3 month span |
| limit | String | No | Default 20, max 100 |

**Response:**
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1312312312321,
  "data": {
    "list": [{
      "positionId": "xxxxxxxxxxx",
      "marginCoin": "USDT",
      "symbol": "BTCUSDT",
      "holdSide": "long",
      "openAvgPrice": "32000",
      "closeAvgPrice": "32500",
      "marginMode": "isolated",
      "openTotalPos": "0.01",
      "closeTotalPos": "0.01",
      "pnl": "14.1",
      "netProfit": "12.1",
      "totalFunding": "0.1",
      "openFee": "0.01",
      "closeFee": "0.01",
      "posMode": "one_way_mode",
      "ctime": "1988824171000",
      "utime": "1988824171000"
    }],
    "endId": "23423432423423234"
  }
}
```

**Key Response Fields:**
- `list`: Array of historical position records
- `holdSide`: `long` or `short`
- `posMode`: `one_way_mode` or `hedge_mode`
- `marginMode`: `isolated` or `crossed`
- `openAvgPrice`: Average entry price
- `closeAvgPrice`: Average exit price
- `pnl`: Realized profit and loss
- `netProfit`: Net profit (after fees and funding)
- `totalFunding`: Accumulated funding costs
- `openFee`: Total fee for position opening
- `closeFee`: Total fee for position closing
- `openTotalPos`: Accumulated opening amount
- `closeTotalPos`: Accumulated closing amount
- `endId`: Pagination cursor for `idLessThan`

**Note:** Response wraps the list in `data.list` (not `data.entrustedList` like order history).

---

### Spot - Wallet (Additional)

#### Withdrawal
- **Endpoint**: `POST /api/v2/spot/wallet/withdrawal`
- **Rate Limit**: 5 req/sec/UID
- **Auth**: Yes
- **Note**: Withdrawal address must be added in the address book on the Bitget web UI first

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| coin | String | Yes | Coin name, e.g. `USDT` |
| transferType | String | Yes | `on_chain`: On-chain withdrawal, `internal_transfer`: Internal transfer |
| address | String | Yes | Withdrawal address (chain address for `on_chain`, or UID/email/mobile for `internal_transfer`) |
| chain | String | No | Chain network, e.g. `trc20`, `erc20`. Required when `transferType` is `on_chain` |
| innerToType | String | No | Address type for internal transfers: `uid` (default), `email`, `mobile` |
| areaCode | String | No | Required when `innerToType` is `mobile` |
| tag | String | No | Address tag (required for some coins like EOS) |
| size | String | Yes | Withdrawal amount |
| remark | String | No | Note/memo |
| clientOid | String | No | Custom order ID (idempotent) |

**Response:**
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695808949356,
  "data": {
    "orderId": "123",
    "clientOid": "123"
  }
}
```

**Response Fields:**
- `orderId`: Withdrawal order ID
- `clientOid`: Custom order ID
