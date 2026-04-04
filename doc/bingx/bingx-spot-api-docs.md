# BingX Spot Trading API Documentation

Source: https://github.com/BingX-API/api-ai-skills

---

## Repo Usage Quick Reference

- Primary repo use: spot market-data reference only
- Repo symbol format: `BTCUSDT`
- Vendor/API symbol format usually includes a hyphen: `BTC-USDT`
- Most relevant endpoint for this repo: spot best bid/ask (`book ticker`)
- Important repo note: BingX spot is not used for spot-margin trading in this project

# BingX Spot Market Data — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** None (public endpoints) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## 1. Get Spot Trading Symbols

`GET /openApi/spot/v1/common/symbols`

Returns specifications for all available spot trading pairs. Optionally filter by symbol.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USDT`. Omit for all symbols. |

**Response `data.symbols` — array of symbol objects:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair, e.g. `BTC-USDT` |
| `minQty` | float | Minimum order quantity (base asset) |
| `maxQty` | float | Maximum order quantity (base asset) |
| `minNotional` | float | Minimum order value (quote asset) |
| `maxNotional` | float | Maximum order value (quote asset) |
| `status` | integer | Symbol status: `1` = active, `0` = inactive |
| `tickSize` | float | Minimum price increment |
| `stepSize` | float | Minimum quantity increment |

---

## 2. Order Book Depth (v1)

`GET /openApi/spot/v1/market/depth`

Returns current order book bids and asks for a trading pair.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `limit` | integer | No | Number of levels. Default `20`. Max `1000`. |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `bids` | `[string, string][]` | `[price, quantity]` pairs, best bid first |
| `asks` | `[string, string][]` | `[price, quantity]` pairs, best ask first |
| `ts` | integer | Timestamp (ms) |

---

## 3. Order Book Aggregation (v2)

`GET /openApi/spot/v2/market/depth`

Returns an aggregated order book with configurable price precision. Note: this endpoint uses underscore symbol format (e.g. `BTC_USDT`).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair in underscore format, e.g. `BTC_USDT` |
| `depth` | integer | Yes | Number of depth levels to return |
| `type` | string | Yes | Precision type: `step0` (highest precision) through `step5` (lowest) |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `bids` | `[string, string][]` | `[price, quantity]` pairs, best bid first |
| `asks` | `[string, string][]` | `[price, quantity]` pairs, best ask first |
| `ts` | integer | Timestamp (ms) |

---

## 4. Recent Trades

`GET /openApi/spot/v1/market/trades`

Returns the most recent public trades for a symbol.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `limit` | integer | No | Number of trades. Default `100`. Max `500`. |

**Response `data` — array of trade objects:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Trade ID |
| `price` | float | Trade price |
| `qty` | float | Trade quantity (base asset) |
| `time` | integer | Trade timestamp (ms) |
| `buyerMaker` | boolean | `true` if the buyer was the maker |

---

## 5. Kline / Candlestick Data (v2)

`GET /openApi/spot/v2/market/kline`

Returns OHLCV candlestick data for a symbol.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `interval` | string | Yes | Candle interval. See values below. |
| `startTime` | integer | No | Start timestamp (ms). |
| `endTime` | integer | No | End timestamp (ms). |
| `limit` | integer | No | Number of candles. Default `500`. Max `1440`. |

**Valid `interval` values:**
`1m` `3m` `5m` `15m` `30m` `1h` `2h` `4h` `6h` `8h` `12h` `1d` `3d` `1w` `1M`

**Response `data` — array of candles (each candle is an array):**

| Index | Description |
|-------|-------------|
| `[0]` | Open time (ms) |
| `[1]` | Open price |
| `[2]` | High price |
| `[3]` | Low price |
| `[4]` | Close price |
| `[5]` | Volume (base asset) |
| `[6]` | Close time (ms) |
| `[7]` | Quote asset volume |

---

## 6. 24h Ticker Price Change Statistics

`GET /openApi/spot/v1/ticker/24hr`

Returns 24-hour rolling window price statistics. Omit `symbol` to get all trading pairs.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair. Omit for all symbols. |

**Response `data` — array of ticker objects:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `openPrice` | string | Opening price 24h ago |
| `highPrice` | string | Highest price in 24h |
| `lowPrice` | string | Lowest price in 24h |
| `lastPrice` | string | Latest trade price |
| `volume` | string | Base asset volume in 24h |
| `quoteVolume` | string | Quote asset volume in 24h |
| `openTime` | integer | Start of 24h window (ms) |
| `closeTime` | integer | End of 24h window (ms) |
| `bidPrice` | float | Current best bid price |
| `bidQty` | float | Current best bid quantity |
| `askPrice` | float | Current best ask price |
| `askQty` | float | Current best ask quantity |
| `priceChangePercent` | string | Price change percentage over 24h |

---

## 7. Symbol Price Ticker

`GET /openApi/spot/v2/ticker/price`

Returns the latest price trades for a symbol.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |

**Response `data` — array of symbol objects, each containing:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `trades` | array | Array of trade items (see below) |

**Trade item fields:**

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | integer | Trade timestamp (ms) |
| `tradeId` | string | Trade ID |
| `price` | string | Trade price |
| `amount` | string | Trade amount |
| `type` | integer | Trade type: `1` = buy, `2` = sell |
| `volume` | string | Trade volume |

---

## 8. Symbol Order Book Ticker (Best Bid/Ask)

`GET /openApi/spot/v1/ticker/bookTicker`

Returns the best (top-of-book) bid and ask price and quantity for a symbol.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |

**Response `data` — array of order book ticker objects:**

| Field | Type | Description |
|-------|------|-------------|
| `eventType` | string | Data type identifier |
| `time` | integer | Timestamp (ms) |
| `symbol` | string | Trading pair |
| `bidPrice` | string | Best bid price |
| `bidVolume` | string | Best bid quantity |
| `askPrice` | string | Best ask price |
| `askVolume` | string | Best ask quantity |

---

## 9. Historical Klines

`GET /openApi/market/his/v1/kline`

Returns historical candlestick data (can go further back than the standard kline endpoint).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `interval` | string | Yes | Candle interval. Same values as Klines (§5). |
| `startTime` | integer | No | Start timestamp (ms). |
| `endTime` | integer | No | End timestamp (ms). |
| `limit` | integer | No | Default value: 500 Maximum value: 500 |

**Response `data` — array of candles (same format as Klines §5):**

| Index | Description |
|-------|-------------|
| `[0]` | Open time (ms) |
| `[1]` | Open price |
| `[2]` | High price |
| `[3]` | Low price |
| `[4]` | Close price |
| `[5]` | Volume (base asset) |
| `[6]` | Close time (ms) |
| `[7]` | Quote asset volume |

---

## 10. Historical Trade Lookup

`GET /openApi/market/his/v1/trade`

Returns older historical public trades (further back than Recent Trades endpoint).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `limit` | integer | No | Number of trades. Default `100`. Max `500`. |
| `fromId` | string | No | The last recorded trade ID to fetch from. |

**Response `data` — array of historical trade objects:**

| Field | Type | Description |
|-------|------|-------------|
| `tid` | string | Trade ID |
| `t` | integer | Trade time (seconds) |
| `ms` | integer | Milliseconds component of trade time |
| `s` | string | Trading pair |
| `p` | float | Trade price |
| `v` | float | Trade volume |


---

# BingX Spot Trade — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## Place Order

### POST /openApi/spot/v1/trade/order — Place Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair, format `BASE-QUOTE`, e.g. `BTC-USDT` |
| side | string | Yes | Side: `BUY` / `SELL` |
| type | string | Yes | Order type: `MARKET` / `LIMIT` / `TAKE_STOP_LIMIT` / `TAKE_STOP_MARKET` / `TRIGGER_LIMIT` / `TRIGGER_MARKET` |
| quantity | float64 | No | Original quantity, e.g., 0.1BTC |
| quoteOrderQty | float64 | No | Quote order quantity, e.g., 100USDT,if quantity and quoteOrderQty are input at the same time, quantity will be used firs|
| price | float64 | No | Price, e.g., 10000USDT |
| stopPrice | string | Yes | Trigger price; required for `TAKE_STOP_LIMIT`, `TAKE_STOP_MARKET`, `TRIGGER_LIMIT`, `TRIGGER_MARKET` types |
| timeInForce | string | No | Time in force: `GTC` / `IOC` / `FOK` / `PostOnly`; required for `LIMIT`, default `GTC` |
| newClientOrderId | string | No | Only letters, numbers and _,Customized order ID for users, with a limit of characters from 1 to 40. Different orders can|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| symbol | string | Trading pair |
| orderId | long | System order ID |
| transactTime | long | Transaction timestamp (milliseconds) |
| price | string | Order price |
| origQty | string | Order quantity |
| executedQty | string | Filled quantity |
| cummulativeQuoteQty | string | Cumulative filled amount (quote asset) |
| status | string | Order status |
| type | string | Order type |
| side | string | Side |
| clientOrderID | string | Custom order ID |

---

### POST /openApi/spot/v1/trade/batchOrders — Batch Place Orders (up to 5)

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| data | array | Yes | Array of order objects |
| sync | bool | No | sync=false (default false if not filled in): parallel ordering (but all orders need to have the same symbol/side/type), |

Each order object fields: `symbol`, `side`, `type`, `quantity`, `quoteOrderQty`, `price`, `stopPrice`, `timeInForce`, `newClientOrderId`.

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orders | array | Order result list; each entry has the same structure as the Place Order response |

---

## Cancel Order

### POST /openApi/spot/v1/trade/cancel — Cancel Single Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair |
| orderId | long | No | Order ID |
| clientOrderID | string | No | Customized order ID for users, with a limit of characters from 1 to 40. Different orders cannot use the same clientOrder|
| cancelRestrictions | string | No | Restrict cancellation by status: `NEW` / `PENDING` / `PARTIALLY_FILLED` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| symbol | string | Trading pair |
| orderId | long | System order ID |
| price | string | Order price |
| origQty | string | Order quantity |
| executedQty | string | Filled quantity |
| cummulativeQuoteQty | string | Cumulative filled amount |
| status | string | Order status (after cancellation: `CANCELED`) |
| type | string | Order type |
| side | string | Side |
| clientOrderID | string | Custom order ID |

---

### POST /openApi/spot/v1/trade/cancelOrders — Batch Cancel Orders

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair |
| orderIds | string | Yes | Order Ids: for example:orderIds=id1,id2,id3 |
| clientOrderIDs | string | No | Custom order IDs, for example: clientOrderIDs=id1,id2,id3 |
| process | int | No | 0 or 1, default 0,if process=1,will handle valid orderIds partially, and return invalid orderIds in fails list, if proce|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orders | array | Cancel result list; each entry has the same structure as the Cancel Single Order response |

---

### POST /openApi/spot/v1/trade/cancelOpenOrders — Cancel All Open Orders for a Trading Pair

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | No | Trading pair; if omitted, cancels all open orders across all pairs |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orders | array | List of cancelled orders; each entry has the same structure as the Cancel Single Order response |

---

### POST /openApi/spot/v1/trade/cancelAllAfter — Cancel All After (Kill Switch)

Automatically cancels all open orders after a specified timeout. Prevents residual positions after program errors by continuously sending heartbeat requests to reset the countdown.

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| type | string | Yes | Request type: ACTIVATE-Activate, CLOSE-Close |
| timeOut | long | Yes | Timeout duration (seconds); required when `type=ACTIVATE`; range 10–120 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| triggerTime | long | Timestamp (ms) when auto-cancel will trigger; `0` when closed |

---

### POST /openApi/spot/v1/trade/order/cancelReplace — Cancel and Replace Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | The trading pair, for example: BTC-USDT, please use uppercase letters |
| cancelOrderId | long | No | The ID of the order to be canceled |
| cancelRestrictions | string | No | Restrict cancellation by status: `NEW` / `PENDING` / `PARTIALLY_FILLED` |
| side | string | Yes | New order side: `BUY` / `SELL` |
| type | string | Yes | MARKET/LIMIT/TAKE_STOP_LIMIT/TAKE_STOP_MARKET/TRIGGER_LIMIT/TRIGGER_MARKET |
| quantity | float64 | No | New order quantity |
| quoteOrderQty | float64 | No | New order amount (MARKET BUY) |
| price | float64 | No | New order price |
| stopPrice | string | Yes | Trigger price used for TAKE_STOP_LIMIT, TAKE_STOP_MARKET, TRIGGER_LIMIT, TRIGGER_MARKET order types. |
| newClientOrderId | string | No | Custom order ID consisting of letters, numbers, and _. Character length should be between 1-40. Different orders cannot |
| cancelClientOrderID | string | No | The user-defined ID of the order to be canceled, character length limit: 1-40, different orders cannot use the same clie|
| cancelReplaceMode | string | Yes | STOP_ON_FAILURE: If the cancel order fails, it will not continue to place a new order. ALLOW_FAILURE: Regardless of whet|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| cancelResult | string | Cancel result: `SUCCESS` / `FAILURE` |
| newOrderResult | string | New order result: `SUCCESS` / `FAILURE` |
| cancelResponse | object | Cancel response; same structure as Cancel Single Order response |
| newOrderResponse | object | New order response; same structure as Place Order response |

---

## Query Orders

### GET /openApi/spot/v1/trade/query — Query Single Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair |
| orderId | long | No | Order ID |
| clientOrderID | string | No | Customized order ID for users, with a limit of characters from 1 to 40. Different orders cannot use the same clientOrder|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| symbol | string | Trading pair |
| orderId | long | System order ID |
| price | string | Order price |
| origQty | string | Order quantity |
| executedQty | string | Filled quantity |
| cummulativeQuoteQty | string | Cumulative filled amount |
| status | string | Order status: `PENDING` / `NEW` / `PARTIALLY_FILLED` / `FILLED` / `CANCELED` / `FAILED` |
| type | string | Order type |
| side | string | Side |
| time | long | Order placement time (milliseconds) |
| updateTime | long | Last update time (milliseconds) |
| origQuoteOrderQty | string | Original order amount (quote asset) |
| fee | string | Fee amount |
| feeAsset | string | Fee asset |
| clientOrderID | string | Custom order ID |
| stopPrice | string | Trigger price |

---

### GET /openApi/spot/v1/trade/openOrders — Query Current Open Orders

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | No | Trading pair; if omitted, returns all open orders across all pairs |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orders | array | Open order list; each entry has the same structure as the Query Single Order response |

---

### GET /openApi/spot/v1/trade/historyOrders — Query Order History

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | No | Trading pair |
| orderId | long | No | If orderId is set, orders >= orderId. Otherwise, the most recent orders will be returned. |
| startTime | long | No | Start timestamp (milliseconds) |
| endTime | long | No | End timestamp, Unit: ms |
| pageIndex | int | No | Page number, starting from 1; default 1 |
| pageSize | int | No | Page size, must >0,Max 100,If not specified, it defaults to 100. Restriction: pageIndex * pageSize <= 10,000. |
| status | string | No | status: FILLED (fully filled) CANCELED: (canceled) FAILED: (failed) |
| type | string | No | order type: MARKET/LIMIT/TAKE_STOP_LIMIT/TAKE_STOP_MARKET/TRIGGER_LIMIT/TRIGGER_MARKET |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orders | array | Historical order list; each entry has the same structure as the Query Single Order response |
| total | int | Total record count |

---

### GET /openApi/spot/v1/trade/myTrades — Query Trade Fills

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair, e.g. BTC-USDT, please use uppercase letters |
| orderId | long | No | Filter by order ID |
| startTime | long | No | Start timestamp (milliseconds) |
| endTime | long | No | End timestamp (milliseconds) |
| fromId | long | No | Starting trade ID. By default, the latest trade will be retrieved |
| limit | int | No | Number of results; default 500, max 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| fills | array | Trade fill list |
| fills[].symbol | string | Trading pair |
| fills[].id | long | Fill ID |
| fills[].orderId | long | Order ID |
| fills[].price | string | Fill price |
| fills[].qty | string | Fill quantity |
| fills[].quoteQty | string | Fill amount (quote asset) |
| fills[].commission | string | Fee amount |
| fills[].commissionAsset | string | Fee asset |
| fills[].time | long | Fill time (milliseconds) |
| fills[].isBuyer | bool | Whether the user is the buyer |
| fills[].isMaker | bool | Whether the order is a maker order |

---

### GET /openApi/spot/v1/user/commissionRate — Query Commission Rate

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair, e.g. BTC-USDT, please use uppercase letters |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| symbol | string | Trading pair |
| makerCommissionRate | string | Maker commission rate (e.g. `0.001` = 0.1%) |
| takerCommissionRate | string | Taker commission rate |

---

## OCO Orders

OCO (One-Cancels-the-Other) places a limit order and a stop-limit order simultaneously; when one is filled or triggered, the other is automatically cancelled.

### POST /openApi/spot/v1/oco/order — Create OCO Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| symbol | string | Yes | Trading pair, e.g., BTC-USDT, please use uppercase letters |
| side | string | Yes | Side: `BUY` / `SELL` |
| quantity | float64 | Yes | Order quantity (base asset) |
| limitPrice | float64 | Yes | Limit order execution price |
| triggerPrice | float64 | Yes | Stop-limit order trigger price |
| orderPrice | float64 | Yes | Stop-limit order execution price after trigger |
| listClientOrderId | string | No | Custom unique ID for the entire Order List, only supports numeric strings, e.g., "123456" |
| aboveClientOrderId | string | No | Custom order ID for the above order |
| belowClientOrderId | string | No | Custom order ID for the below order |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orderListId | long | OCO group ID |
| contingencyType | string | Contingency type (`OCO`) |
| listStatusType | string | OCO group status |
| listOrderStatus | string | OCO group order status |
| transactionTime | long | Creation time (milliseconds) |
| symbol | string | Trading pair |
| orders | array | Order pair; each entry contains `symbol`, `orderId`, `clientOrderId` |
| orderReports | array | Detailed order info; each entry has the same structure as the Place Order response |

---

### POST /openApi/spot/v1/oco/cancel — Cancel OCO Order

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderId | string | No | The order ID of the limit order or the stop-limit order. Either orderId or clientOrderId must be provided. |
| clientOrderId | string | No | The User-defined order ID of the limit order or the stop-limit order |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orderListId | long | OCO group ID |
| contingencyType | string | Contingency type |
| listStatusType | string | OCO group status |
| listOrderStatus | string | Order status (`ALL_DONE`) |
| transactionTime | long | Cancellation time (milliseconds) |
| symbol | string | Trading pair |
| orders | array | Order pair list |
| orderReports | array | Detailed cancellation info |

---

### GET /openApi/spot/v1/oco/orderList — Query OCO Order Details

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| orderListId | string | No | OCO group ID; at least one of `orderListId` or `listClientOrderId` required |
| clientOrderId | string | No | Custom order ID |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| orderListId | long | OCO group ID |
| contingencyType | string | Contingency type |
| listStatusType | string | List status |
| listOrderStatus | string | Order status |
| listClientOrderId | string | OCO group custom ID |
| transactionTime | long | Creation time (milliseconds) |
| symbol | string | Trading pair |
| orders | array | Order pair list, containing `orderId`, `clientOrderId` |

---

### GET /openApi/spot/v1/oco/openOrderList — Query All Open OCO Orders

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| pageIndex | int64 | Yes | Page number (1-based) |
| pageSize | int64 | Yes | Number of items per page |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| ocoOrders | array | OCO order list; each entry has the same structure as the Query OCO Order Details response |

---

### GET /openApi/spot/v1/oco/historyOrderList — Query Historical OCO Orders

**Request Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| startTime | long | No | Start timestamp (milliseconds) |
| endTime | long | No | End timestamp (milliseconds) |
| pageIndex | int | Yes | Page number, starting from 1; default 1 |
| pageSize | int | Yes | Items per page; default 100, max 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| ocoOrders | array | Historical OCO order list; each entry has the same structure as the Query OCO Order Details response |
| total | int | Total record count |

---

# BingX Spot Account — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## Account and Assets

### 1. Query Spot Account Balance

`GET /openApi/spot/v1/account/balance`

Query spot trading account assets and balances.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| *(none)* | | | |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `balances` | array | Asset list; see fields below |

**`balances` list entry fields:**

| Field | Type | Description |
|-------|------|-------------|
| `asset` | string | Asset symbol, e.g. `USDT`, `BTC` |
| `free` | string | Available balance |
| `locked` | string | Locked balance |

---

### 2. Query Fund Account Balance

`GET /openApi/fund/v1/account/balance`

Query main account (fund account) balance.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `asset` | string | No | Coin name, return all when not transmitted |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `balance` | object | Account balance information |

---

### 3. Asset Overview (All Accounts)

`GET /openApi/account/v1/allAccountBalance`

Query USDT-equivalent asset overview across all account types.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `accountType` | string | No | Account type; returns all if omitted. Options: `sopt`, `stdFutures`, `coinMPerp`, `USDTMPerp`, `copyTrading`, `grid`, `eran`, `c2c` |
| `recvWindow` | int | No | Request validity window (milliseconds) |

**Response `data` — Account asset list (array):**

| Field | Type | Description |
|-------|------|-------------|
| `accountType` | string | Account type |
| `usdtBalance` | string | USDT-equivalent value |

---

### 4. Transfer Assets Between Accounts

`POST /openApi/api/asset/v1/transfer`

Transfer assets between different account types (e.g. spot ↔ perpetual futures).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `asset` | string | Yes | Asset name, e.g. `USDT` |
| `amount` | DECIMAL | Yes | Transfer amount |
| `fromAccount` | string | Yes | fromAccount, fund：Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, US|
| `toAccount` | string | Yes | toAccount, fund:Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, USDT|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `tranId` | string | Transfer record ID |

---

### 5. Query Asset Transfer Records (v3)

`GET /openApi/api/v3/asset/transfer`

Query asset transfer history between accounts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | ENUM | Yes | Transfer direction (at least one of `type` or `tranId` required) |
| `tranId` | int | No | transaction ID, (query by type or tranId) |
| `startTime` | int | No | Starting time1658748648396 |
| `endTime` | int | No | End time (milliseconds) |
| `current` | int | No | current page default1 |
| `size` | int | No | Page size default 10 can not exceed 100 |
| `recvWindow` | int | No | Request validity window (milliseconds), max 5000 |

> **Note:** This endpoint returns `{ total, rows }` directly at the top level, NOT wrapped in the standard `{ code, msg, data }` envelope.

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total record count |
| `rows` | array | Transfer record list |

**`rows` entry fields:**

| Field | Type | Description |
|-------|------|-------------|
| `asset` | string | Asset name |
| `amount` | string | Transfer amount |
| `type` | string | Transfer direction |
| `status` | string | Status: `CONFIRMED`, etc. |
| `tranId` | int | Transfer record ID |
| `timestamp` | int | Transfer timestamp (milliseconds) |

---

### 6. Query Asset Transfer Records (new format)

`GET /openApi/api/v3/asset/transferRecord`

Query asset transfer records with enhanced filtering options.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `fromAccount` | string | Yes* | fromAccount, fund：Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, US|
| `toAccount` | string | Yes* | toAccount, fund:Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, USDT|
| `transferId` | string | No | transaction ID, (query by fromAccount/toAccount or transferId). *Either both `fromAccount`+`toAccount` or `transferId` must be provided. |
| `pageIndex` | int | No | current page default1 |
| `pageSize` | int | No | Page size default 10 can not exceed 100 |
| `startTime` | int | No | Starting time1658748648396 |
| `endTime` | int | No | End time (milliseconds) |

> **Note:** This endpoint returns `{ total, rows }` directly at the top level, NOT wrapped in the standard `{ code, msg, data }` envelope.

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total record count |
| `rows` | array | Transfer record list |

**`rows` entry fields:**

| Field | Type | Description |
|-------|------|-------------|
| `transferId` | string | Transfer record ID |
| `asset` | string | Asset name |
| `amount` | string | Transfer amount |
| `fromAccount` | string | Source account (fund/spot/stdFutures/coinMPerp/USDTMPerp) |
| `toAccount` | string | Target account (fund/spot/stdFutures/coinMPerp/USDTMPerp) |
| `timestamp` | long | Transfer timestamp (milliseconds) |

---

### 7. Query Supported Coins for Transfer

`GET /openApi/api/asset/v1/transfer/supportCoins`

Query the list of assets that support inter-account transfers.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `fromAccount` | string | Yes | fromAccount, fund：Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, US|
| `toAccount` | string | Yes | toAccount, fund:Funding Account spot:Spot Account, stdFutures:Standard Contract, coinMPerp:COIN-M Perpetual Future, USDT|

**Response `data` — Array of supported coins:**

| Field | Type | Description |
|-------|------|-------------|
| `coin` | string | Coin symbol |
| `name` | string | Coin full name |

---

### 8. Internal P2P Transfer (Main Account)

`POST /openApi/wallets/v1/capital/innerTransfer/apply`

Perform internal P2P transfer between main account and other accounts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | Yes | Asset to transfer |
| `amount` | float | Yes | Transfer amount |
| `transferClientId` | string | No | Custom ID for internal transfer by the client, combination of numbers and letters, length less than 100 characters |
| `userAccountType` | int | Yes | Target user account type |
| `userAccount` | string | Yes | Target user account identifier |
| `callingCode` | string | No | Area code for telephone, required when userAccountType=2. |
| `walletType` | int | Yes | Account type, 1 Fund Account; 2 Standard Futures Account; 3 Perpetual Futures Account; 15 Spot Account |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Internal transfer ID |
| `transferClientId` | string | Client-defined transfer ID |

---

### 9. Query Internal Transfer Records

`GET /openApi/wallets/v1/capital/innerTransfer/records`

Query main account internal P2P transfer history.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | Yes | Asset name |
| `id` | string | No | Transfer record ID filter |
| `transferClientId` | string | No | Client's self-defined internal transfer ID. When both platform ID and transferClientId are provided as input, the query |
| `startTime` | int | No | Start time (milliseconds) |
| `endTime` | int | No | End time (milliseconds) |
| `offset` | int | No | Starting offset, default 0 |
| `limit` | int | No | Items per page, default 100, max 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `data` | array | Transfer record list |
| `total` | int | Total record count |

**Record entry fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Transfer ID |
| `coin` | string | Asset name |
| `receiver` | int | Recipient UID |
| `amount` | float | Transfer amount |
| `status` | int | Status: `4` pending, `5` failed, `6` completed |
| `fromUid` | int | Sender UID |
| `recordType` | string | `out` (outgoing) or `in` (incoming) |

---

*For full spot trading operations (place/cancel orders, OCO orders, etc.), see [spot-trade/SKILL.md](../spot-trade/SKILL.md).*

---

# BingX Spot Wallet — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## 1. Deposit Records

### Query Deposit History

`GET /openApi/api/v3/capital/deposit/hisrec`

Rate limit: 10 requests/s per UID. API Key permission: **Read**.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | No | coin name |
| `status` | int | No | Deposit status filter: `0`=In progress, `1`=Not credited, `2`=Wrong amount, `6`=Chain confirmed |
| `startTime` | long | No | Start of query range in milliseconds (e.g., `1658748648396`) |
| `endTime` | long | No | End Time 1658748648396 |
| `offset` | int | No | Pagination offset, default `0` |
| `limit` | int | No | Page size, default `1000`, max `1000` |
| `txId` | long | No | Blockchain transaction ID to filter by |

> **Note:** This endpoint returns a bare JSON array directly, NOT wrapped in the standard `{ code, msg, data }` envelope.

**Response** (array of deposit records):

| Field | Type | Description |
|-------|------|-------------|
| `insertTime` | long | Deposit creation timestamp (milliseconds) |
| `amount` | string | Deposit amount |
| `coin` | string | Coin name |
| `network` | string | Network used for the deposit |
| `address` | string | Deposit address |
| `addressTag` | string | Memo/tag (if applicable) |
| `txId` | string | Blockchain transaction ID |
| `status` | int | Deposit status: `0`=In progress, `1`=Not credited, `2`=Wrong amount, `6`=Confirmed |
| `unlockConfirm` | string | Required confirmations to unlock the deposit |
| `confirmTimes` | string | Current confirmation count / required confirmations (e.g., `"3/12"`) |

---

## 2. Withdrawal Records

### Query Withdrawal History

`GET /openApi/api/v3/capital/withdraw/history`

Rate limit: 10 requests/s per UID. API Key permission: **Read**.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | No | Unique withdrawal record ID |
| `coin` | string | No | Coin name (e.g., `USDT`) |
| `withdrawOrderId` | string | No | Custom ID, if there is none, this field will not be returned,When both the platform ID and withdraw order ID are passed |
| `status` | int | No | Withdrawal status filter: `4`=Under review, `5`=Failed, `6`=Completed |
| `startTime` | long | No | Starting time1658748648396 |
| `endTime` | long | No | End Time 1658748648396 |
| `offset` | int | No | Pagination offset, default `0` |
| `limit` | int | No | Page size, default `1000`, max `1000` |
| `txId` | string | No | Blockchain transaction ID to filter by |

> **Note:** This endpoint returns a bare JSON array directly, NOT wrapped in the standard `{ code, msg, data }` envelope.

**Response** (array of withdrawal records):

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique withdrawal record ID |
| `amount` | string | Withdrawal amount |
| `transactionFee` | string | Network fee charged |
| `coin` | string | Coin name |
| `status` | int | Withdrawal status: `4`=Under review, `5`=Failed, `6`=Completed |
| `address` | string | Destination withdrawal address |
| `addressTag` | string | Memo/tag (if applicable) |
| `txId` | string | Blockchain transaction ID |
| `applyTime` | string | Withdrawal submission timestamp |
| `network` | string | Network used |
| `withdrawOrderId` | string | Custom withdrawal ID (if provided at submission) |
| `info` | string | Additional reason or info (e.g., rejection reason) |
| `confirmNo` | int | Number of blockchain confirmations |

---

## 3. Coin Config

### Query Currency Deposit and Withdrawal Data

`GET /openApi/wallets/v1/capital/config/getall`

Rate limit: 5 requests/s per UID. API Key permission: **Read**.
Returns supported networks, fees, deposit/withdrawal limits, and enable status for each coin.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | No | Coin identification |
| `displayName` | string | No | The platform displays the currency pair name for display only. Unlike coins, coins need to be used for withdrawal and re|

**Response `data`** (array of coin configs):

| Field | Type | Description |
|-------|------|-------------|
| `coin` | string | Coin identifier (e.g., `USDT`) |
| `name` | string | Full coin name (e.g., `TetherUS`) |
| `networkList` | array | List of supported networks for this coin |

**Each `networkList` item:**

| Field | Type | Description |
|-------|------|-------------|
| `network` | string | Network identifier (e.g., `BEP20`, `ERC20`, `TRC20`) |
| `name` | string | Network display name |
| `depositEnable` | bool | Whether deposit is enabled on this network |
| `withdrawEnable` | bool | Whether withdrawal is enabled on this network |
| `withdrawFee` | string | Fixed withdrawal fee on this network |
| `withdrawMin` | string | Minimum withdrawal amount |
| `withdrawMax` | string | Maximum withdrawal amount per transaction |
| `minConfirm` | int | Minimum blockchain confirmations required for deposit |
| `depositDesc` | string | Deposit instructions or notes |
| `withdrawDesc` | string | Withdrawal instructions or notes |
| `specialTips` | string | Special network tips or warnings |
| `isDefault` | bool | Whether this is the default network |
| `contractAddress` | string | Smart contract address for token (if applicable) |

---

## 4. Withdraw

### Initiate a Withdrawal

`POST /openApi/wallets/v1/capital/withdraw/apply`

Rate limit: 2 requests/s per UID. API Key permission: **Withdraw**.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | Yes | Coin name (e.g., `USDT`) |
| `network` | string | No | Network name (e.g., `BEP20`, `ERC20`). Uses the default network if omitted. |
| `address` | string | Yes | Destination withdrawal address |
| `addressTag` | string | No | Tag or memo, some currencies support tag or memo |
| `amount` | float64 | Yes | Withdrawal amount |
| `walletType` | int64 | Yes | Source account: `1`=Fund account, `2`=Standard contract, `3`=Perpetual futures |
| `withdrawOrderId` | string | No | Customer-defined withdrawal ID, a combination of numbers and letters, with a length of less than 100 characters |
| `vaspEntityId` | string | No | Payment platform information, only KYC=KOR (Korean individual users) must pass this field. List values Bithumb, Coinone,|
| `recipientLastName` | string | No | The recipient's surname is in English, and only KYC=KOR (Korean individual users) must pass this field. No need to fill |
| `recipientFirstName` | string | No | The recipient's name in English, only KYC=KOR (Korean individual users) must pass this field. No need to fill in when va|
| `dateOfbirth` | string | No | The payee's date of birth (example 1999-09-09) must be passed as this field only for KYC=KOR (Korean individual users). |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique withdrawal record ID assigned by BingX |

---

## 5. Deposit Address

### Query Main Account Deposit Address

`GET /openApi/wallets/v1/capital/deposit/address`

Rate limit: 2 requests/s per UID. API Key permission: **Read**.
Only available for main (mother) accounts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `coin` | string | Yes | Name of the coin for transfer |
| `offset` | int | No | Starting record number, default `0` |
| `limit` | int | No | Page size, default `100`, max `1000` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total number of deposit address records |
| `data` | array | Array of deposit address objects |

**Each address object:**

| Field | Type | Description |
|-------|------|-------------|
| `coin` | string | Coin name |
| `network` | string | Network identifier |
| `address` | string | Deposit address string |
| `tag` | string | Memo or tag (if applicable for this network) |
| `url` | string | Blockchain explorer URL for this address |

---

## 6. Deposit Risk Control Records

### Query Deposit Risk Control Records

`GET /openApi/wallets/v1/capital/deposit/riskRecords`

Rate limit: 2 requests/s per UID. API Key permission: **Read**.
Returns deposit records that are under risk control review for the main account and its sub-accounts.

**Parameters:**

No additional parameters required beyond common parameters.

**Response `data`** (array of risk-control deposit records):

| Field | Type | Description |
|-------|------|-------------|
| `insertTime` | long | Record creation timestamp (milliseconds) |
| `amount` | string | Deposit amount held under review |
| `coin` | string | Coin name |
| `network` | string | Network of the deposit |
| `address` | string | Deposit address |
| `txId` | string | Blockchain transaction ID |
| `status` | int | Risk control status code |
| `riskReason` | string | Reason the deposit was flagged for review |

---

# BingX Spot WebSocket Market Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-ws.bingx.com/market`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends ping messages; client must reply `Pong` to keep connection alive
- No authentication required for market data streams

---

## Subscription: Trade Detail

Subscribe to real-time trade detail data. Due to multi-threaded push, trade IDs may not be strictly ordered.

### dataType Format

`{symbol}@trade`

Example: `BTC-USDT@trade`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USDT@trade"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@trade` |
| data | Array of trade records |
| data[].T | Trade time (ms) |
| data[].s | Symbol |
| data[].p | Price |
| data[].q | Quantity |
| data[].m | Is buyer maker |

---

## Subscription: K-Line Streams

Subscribe to candlestick/kline data.

### dataType Format

`{symbol}@kline_{interval}`

Example: `BTC-USDT@kline_1min`

### Supported Intervals

`1min`, `3min`, `5min`, `15min`, `30min`, `1h`, `2h`, `4h`, `6h`, `8h`, `12h`, `1d`, `3d`, `1w`, `1M`

> **Note**: Spot uses `min` suffix (e.g., `1min`), unlike swap which uses `m` (e.g., `1m`).

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "BTC-USDT@kline_1min"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-`. K-line interval type (see Supported Intervals above) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@kline_1min` |
| data.s | Symbol |
| data.K.t | Kline start time |
| data.K.T | Kline close time |
| data.K.o | Open |
| data.K.h | High |
| data.K.l | Low |
| data.K.c | Close |
| data.K.v | Volume |

---

## Subscription: Market Depth

Push limited depth information every 300ms. Default level 20.

### dataType Format

`{symbol}@depth{level}`

Example: `BTC-USDT@depth50`

### Supported Levels

`5`, `10`, `20`, `50`, `100`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USDT@depth50"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-`. Depth level: 5, 10, 20, 50, 100 |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@depth50` |
| data.bids | `[[price, qty], ...]` sorted descending |
| data.asks | `[[price, qty], ...]` sorted ascending |
| data.T | Timestamp (ms) |

---

## Subscription: 24-Hour Price Change

Push every 1000ms.

### dataType Format

`{symbol}@ticker`

Example: `BTC-USDT@ticker`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USDT@ticker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@ticker`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@ticker` |
| data.s | Symbol |
| data.c | Latest price |
| data.h | 24h high |
| data.l | 24h low |
| data.v | 24h volume |
| data.p | Price change |
| data.P | Price change percent |

---

## Subscription: Latest Trade Price

Real-time push.

### dataType Format

`{symbol}@lastPrice`

Example: `BTC-USDT@lastPrice`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USDT@lastPrice"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@lastPrice`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@lastPrice` |
| data.s | Symbol |
| data.c | Latest price |
| data.T | Timestamp (ms) |

---

## Subscription: Best Order Book (Book Ticker)

Real-time push.

### dataType Format

`{symbol}@bookTicker`

Example: `BTC-USDT@bookTicker`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USDT@bookTicker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@bookTicker`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@bookTicker` |
| data.s | Symbol |
| data.b | Best bid price |
| data.B | Best bid quantity |
| data.a | Best ask price |
| data.A | Best ask quantity |
| data.T | Timestamp (ms) |

---

## Subscription: Incremental and Full Depth

Push incremental depth of 1000 levels every 500ms.

### dataType Format

`{symbol}@incrDepth`

Example: `BTC-USDT@incrDepth`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USDT@incrDepth"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@incrDepth`) |

### How to Maintain Incremental Depth Locally

1. First message: `action: "all"` — full snapshot with `lastUpdateId`
2. Subsequent: `action: "update"` — incremental. Nth update's `lastUpdateId` = (N-1)th + 1
3. If not continuous: reconnect or cache last 3 and try merge
4. Compare with local depth: Add / Remove (qty=0) / Update
5. Update cache and `lastUpdateId`

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@incrDepth` |
| data.action | `all` or `update` |
| data.lastUpdateId | Sequence ID |
| data.bids | `[[price, qty], ...]` |
| data.asks | `[[price, qty], ...]` |
| data.T | Timestamp (ms) |


---

# BingX Spot WebSocket Account Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-ws.bingx.com/market?listenKey=<key>`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends ping messages; client must reply `Pong` to keep connection alive
- **Explicit channel subscription required** — send subscribe messages for each event type
- Listen Key valid for 1 hour; extend every 30 minutes via PUT REST API

---

## Listen Key REST APIs

### Generate Listen Key

`POST /openApi/user/auth/userDataStream`

Rate limit: 2/s per UID

**Headers:**

| Header | Value | Required |
|--------|-------|----------|
| X-BX-APIKEY | Your API Key | Yes |
| X-SOURCE-KEY | BX-AI-SKILL | Yes |

**Response:**

```json
{
  "listenKey": "a8ea75681542e66f1a50a1616dd06ed77dab61baa0c296bca03a9b13ee5f2dd7"
}
```

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `listenKey` | string | Listen Key for WebSocket authentication |

### Extend Listen Key Validity

`PUT /openApi/user/auth/userDataStream`

Rate limit: 2/s per UID

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| listenKey | string | Yes | The listen key to extend |

**Response `data`:** Standard `{ code, msg }` envelope with empty `data`. HTTP status codes: `200` Success, `204` No request parameters, `404` listenKey does not exist.

### Delete Listen Key

`DELETE /openApi/user/auth/userDataStream`

Rate limit: 2/s per UID

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| listenKey | string | Yes | The listen key to delete |

**Response `data`:** Standard `{ code, msg }` envelope with empty `data`. HTTP status codes: `200` Success, `204` No request parameters, `404` listenKey does not exist.

---

## Subscription: Order Update (spot.executionReport)

Pushed when a spot order is created, partially filled, fully filled, or canceled.

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "spot.executionReport"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Fixed value: `spot.executionReport` |

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| dataType | string | `spot.executionReport` |
| data.e | string | Event type |
| data.E | number | Event time (ms) |
| data.s | string | Symbol |
| data.S | string | Side (`BUY`/`SELL`) |
| data.o | string | Order type (`LIMIT`/`MARKET`) |
| data.q | string | Original quantity |
| data.p | string | Order price |
| data.x | string | Execution type (`NEW`/`TRADE`/`CANCELED`/`EXPIRED`) |
| data.X | string | Order status (`NEW`/`PARTIALLY_FILLED`/`FILLED`/`CANCELED`/`EXPIRED`) |
| data.i | number | Order ID |
| data.l | string | Last filled quantity |
| data.z | string | Cumulative filled quantity |
| data.L | string | Last fill price |
| data.n | string | Commission |
| data.N | string | Commission asset |
| data.T | number | Trade time (ms) |

---

## Subscription: Account Balance Update (ACCOUNT_UPDATE)

Pushed when account balance changes.

### Subscription Request

```json
{
  "id": "gdfg2311-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "ACCOUNT_UPDATE"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Fixed value: `ACCOUNT_UPDATE` |

### Trigger Reasons (field `m`)

- `DEPOSIT` — Deposit
- `WITHDRAW` — Withdrawal
- `ORDER` — Order fill
- `FUNDING_FEE` — Funding fee
- `WITHDRAW_REJECT` — Withdrawal rejected
- `ADJUSTMENT` — Adjustment
- `INSURANCE_CLEAR` — Insurance clear
- `ADMIN_DEPOSIT` — Admin deposit
- `ADMIN_WITHDRAW` — Admin withdrawal
- `MARGIN_TRANSFER` — Margin transfer
- `MARGIN_TYPE_CHANGE` — Margin type change
- `ASSET_TRANSFER` — Asset transfer
- `AUTO_EXCHANGE` — Auto exchange

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| e | string | Event type: `ACCOUNT_UPDATE` |
| E | number | Event time (ms) |
| T | number | Matching time (ms) |
| a.m | string | Trigger reason |
| a.B | array | Balance updates |
| a.B[].a | string | Asset name (e.g., `USDT`, `BTC`) |
| a.B[].wb | string | Wallet balance |
| a.B[].cw | string | Cross wallet balance |
| a.B[].bc | string | Balance change |


---
