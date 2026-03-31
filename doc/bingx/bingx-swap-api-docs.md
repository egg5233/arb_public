# BingX Perpetual Swap (USDT-M) API Documentation

Source: https://github.com/BingX-API/api-ai-skills

---

# BingX Swap Market Data — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

Market data endpoints do not require special API key permissions.

---

## 1. Get Contract Info

`GET /openApi/swap/v2/quote/contracts`

Returns specifications for all available perpetual swap contracts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USDT; omit for all |

**Response `data` — array of contract objects:**

| Field | Type | Description |
|-------|------|-------------|
| `contractId` | string | Unique contract identifier |
| `symbol` | string | Trading pair, e.g. `BTC-USDT` |
| `size` | string | Contract size (value per contract) |
| `quantityPrecision` | integer | Decimal places for order quantity |
| `pricePrecision` | integer | Decimal places for price |
| `feeRate` | string | Base fee rate (deprecated, use `makerFeeRate`/`takerFeeRate`) |
| `makerFeeRate` | float | Maker fee rate, e.g. `0.0002` = 0.02% |
| `takerFeeRate` | float | Taker fee rate, e.g. `0.0005` = 0.05% |
| `tradeMinLimit` | integer | Deprecated, use `tradeMinQuantity` |
| `tradeMinQuantity` | float | Minimum order quantity (in base asset), e.g. `0.0001` |
| `tradeMinUSDT` | float | Minimum order value (in USDT), e.g. `2` |
| `maxLongLeverage` | integer | Maximum leverage for long positions, e.g. `125` |
| `maxShortLeverage` | integer | Maximum leverage for short positions, e.g. `125` |
| `currency` | string | Settlement currency, e.g. `USDT` |
| `asset` | string | Base asset, e.g. `BTC` |
| `status` | integer | `1` = active, `0` = inactive |
| `apiStateOpen` | string | API open position status: `"true"` or `"false"` |
| `apiStateClose` | string | API close position status: `"true"` or `"false"` |
| `brokerState` | string | Broker status: `"true"` or `"false"` |
| `launchTime` | integer | Launch time (ms timestamp) |
| `maintainTime` | integer | Maintenance time (ms timestamp, `0` = no maintenance) |
| `offTime` | integer | Offline time (ms timestamp, `0` = not offline) |

---

## 2. Order Book Depth

`GET /openApi/swap/v2/quote/depth`

Returns current order book bids and asks.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `limit` | integer | No | Default 20, optional value:[5, 10, 20, 50, 100, 500, 1000] |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `bids` | `[string, string][]` | `[price, quantity]` pairs, best bid first |
| `asks` | `[string, string][]` | `[price, quantity]` pairs, best ask first |
| `T` | integer | Timestamp (ms) |

---

## 3. Recent Trades

`GET /openApi/swap/v2/quote/trades`

Returns the most recent public trades.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `limit` | integer | No | Number of trades. Default `500`. Max `1000`. |

**Response `data` — array of trade objects:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Trade ID |
| `price` | string | Trade price |
| `qty` | string | Trade quantity |
| `quoteQty` | string | Quote asset quantity |
| `time` | integer | Trade timestamp (ms) |
| `buyerMaker` | boolean | `true` if buyer was the maker |

---

## 4. Mark Price & Premium Index

`GET /openApi/swap/v2/quote/premiumIndex`

Returns mark price, index price, and premium index. Omit `symbol` to get all contracts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |

**Response `data` (single or array):**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `markPrice` | string | Current mark price |
| `indexPrice` | string | Current index price |
| `lastFundingRate` | string | Last funding rate |
| `nextFundingTime` | integer | Next funding time (ms) |
| `time` | integer | Timestamp (ms) |

---

## 5. Funding Rate

`GET /openApi/swap/v2/quote/fundingRate`

Returns the current funding rate and next funding time.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USDT` |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int32 | No | default: 100 maximum: 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `fundingRate` | string | Current funding rate (e.g. `"0.0001"`) |
| `fundingTime` | integer | Current funding time (ms) |
| `nextFundingTime` | integer | Next funding settlement time (ms) |

---

## 6. Kline / Candlestick Data

`GET /openApi/swap/v3/quote/klines`

Returns OHLCV candlestick data.

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
| `[8]` | Number of trades |
| `[9]` | Taker buy base asset volume |
| `[10]` | Taker buy quote asset volume |

---

## 7. Open Interest

`GET /openApi/swap/v2/quote/openInterest`

Returns the total open interest for a contract.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `openInterest` | string | Total open interest (in base asset) |
| `symbol` | string | Trading pair |
| `time` | integer | Timestamp (ms) |

---

## 8. 24h Ticker Price Change Statistics

`GET /openApi/swap/v2/quote/ticker`

Returns 24-hour rolling window price statistics. Omit `symbol` to get all contracts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |

**Response `data` (single object or array):**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `priceChange` | string | Price change over 24h |
| `priceChangePercent` | string | Price change percentage over 24h |
| `lastPrice` | string | Latest trade price |
| `lastQty` | string | Quantity of last trade |
| `openPrice` | string | Opening price 24h ago |
| `highPrice` | string | Highest price in 24h |
| `lowPrice` | string | Lowest price in 24h |
| `volume` | string | Base asset volume in 24h |
| `quoteVolume` | string | Quote asset volume in 24h |
| `openTime` | integer | Start of 24h window (ms) |
| `closeTime` | integer | End of 24h window (ms) |
| `askPrice` | string | Current best ask price |
| `askQty` | string | Current best ask quantity |
| `bidPrice` | string | Current best bid price |
| `bidQty` | string | Current best bid quantity |

---

## 9. Best Bid/Ask Price (Book Ticker)

`GET /openApi/swap/v2/quote/bookTicker`

Returns the best (top-of-book) bid and ask price and quantity. Omit `symbol` to get all contracts.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |

**Response `data` (single object or array):**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `bidPrice` | string | Best bid price |
| `bidQty` | string | Best bid quantity |
| `askPrice` | string | Best ask price |
| `askQty` | string | Best ask quantity |
| `time` | integer | Timestamp (ms) |

---

## 10. Historical Trades

`GET /openApi/swap/v1/market/historicalTrades`

Query historical transaction records for a trading pair.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `fromId` | int64 | No | Starting trade ID. Default returns most recent records |
| `limit` | int | No | Number of results, default 50, max 100 |

**Response `data` (array):**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Transaction ID |
| `price` | string | Transaction price |
| `qty` | string | Transaction quantity |
| `quoteQty` | string | Turnover |
| `time` | int64 | Transaction time (ms) |
| `isBuyerMaker` | bool | Whether the buyer is the maker |

---

## 11. Mark Price Kline/Candlestick Data

`GET /openApi/swap/v1/market/markPriceKlines`

Query mark price kline/candlestick data.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `interval` | string | Yes | time interval, refer to field description |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int64 | No | Default 500, max 1440 |

**Response `data` (array):**

| Field | Type | Description |
|-------|------|-------------|
| `open` | float64 | Open price |
| `close` | float64 | Close price |
| `high` | float64 | High price |
| `low` | float64 | Low price |
| `volume` | float64 | Transaction volume |
| `time` | int64 | K-line timestamp (ms) |

---

## 12. Symbol Price Ticker

`GET /openApi/swap/v1/ticker/price`

Get the latest price for a symbol or all symbols.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT. If omitted, returns all symbols |

**Response `data` (single object or array):**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `price` | string | Latest price |
| `time` | int64 | Matching engine time (ms) |

---

## 13. Trading Rules

`GET /openApi/swap/v1/tradingRules`

Query trading rules and limits for a contract.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT. Please use uppercase letters. If not provided, information for all trading pairs will be returned |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `minSizeCoin` | string | Minimum order quantity in coin |
| `minSizeUsd` | string | Minimum order amount in USDT |
| `maxNumOrder` | string | Maximum open orders for this contract |
| `protectionThreshold` | string | Spread protection threshold |
| `buyMaxPrice` | string | Upper limit ratio for limit buy price |
| `buyMinPrice` | string | Lower limit ratio for limit buy price |
| `sellMaxPrice` | string | Upper limit ratio for limit sell price |
| `sellMinPrice` | string | Lower limit ratio for limit sell price |
| `marketRatio` | string | Price tolerance ratio for market orders |

---

## 14. Get Server Time

`GET /openApi/swap/v2/server/time`

Get the server timestamp.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `serverTime` | int64 | Server time in milliseconds |

---


---

# BingX Perpetual Swap Trade — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## I. Place Orders

### 1. Place Order

`POST /openApi/swap/v2/trade/order`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `side` | string | Yes | buying and selling direction SELL, BUY |
| `positionSide` | string | No | Position direction, required for single position as BOTH, for both long and short positions only LONG or SHORT can be ch|
| `type` | string | Yes | Order type: `MARKET`, `LIMIT`, `STOP_MARKET`, `STOP`, `TAKE_PROFIT_MARKET`, `TAKE_PROFIT`, `TRAILING_STOP_MARKET`, `TRAILING_TP_SL` |
| `quantity` | float | No | Original quantity, only support units by COIN ,Ordering with quantity U is not currently supported. |
| `quoteOrderQty` | float | No | Quote order quantity, e.g., 100USDT,if quantity and quoteOrderQty are input at the same time, quantity will be used firs|
| `price` | float | No | Price, represents the trailing stop distance in TRAILING_STOP_MARKET and TRAILING_TP_SL |
| `stopPrice` | float | No | Trigger price (required for `STOP_MARKET`, `STOP`, `TAKE_PROFIT_MARKET`, `TAKE_PROFIT`) |
| `timeInForce` | string | No | Time in force: `GTC`, `IOC`, `FOK`, `PostOnly`; required for `LIMIT`, default `GTC` |
| `clientOrderId` | string | No | Custom order ID, 1–40 characters, converted to lowercase by the system; must be unique per order; **supported for `MARKET` and `LIMIT` only** |
| `workingType` | string | No | Trigger price source: `MARK_PRICE` (mark price, **default**), `CONTRACT_PRICE` (last trade price), or `INDEX_PRICE` (index price); must be `CONTRACT_PRICE` when `stopGuaranteed=true` or `cutfee` |
| `stopGuaranteed` | string | No | true: Enables the guaranteed stop-loss and take-profit feature; false: Disables the feature; cutfee: Enable the guaranteed stop loss function and enable the VIP guaranteed stop loss fee reduction function. When stopGuaranteed is true or cutfee, the quantity field does not take effect. The guaranteed stop-loss feature is not enabled by default. Supported order types include: STOP_MARKET / TAKE_PROFIT_MARKET / STOP / TAKE_PROFIT / TRIGGER_LIMIT / TRIGGER_MARKET. |
| `closePosition` | string | No | `true` closes all positions in the specified direction on trigger; cannot be used with `quantity` |
| `reduceOnly` | string | No | true, false; Default value is false for single position mode; This parameter is not accepted for both long and short pos|
| `activationPrice` | float | No | Activation price (for `TRAILING_STOP_MARKET`; defaults to current market price if omitted) |
| `priceRate` | float | No | Callback rate (required for `TRAILING_STOP_MARKET` and `TRAILING_TP_SL`, e.g. `0.05` = 5%) |
| `stopLoss` | string | No | Stop-loss attached order (may only be attached to `MARKET` / `LIMIT` orders; see structure below) |
| `takeProfit` | string | No | Support setting take profit while placing an order. Only supports type: TAKE_PROFIT_MARKET/TAKE_PROFIT |
| `positionId` | int | No | In the Separate Isolated mode, closing a position must be transmitted |

**stopLoss / takeProfit object structure:**

```json
{
  "type": "STOP_MARKET",       // stopLoss: "STOP_MARKET" or "STOP"
                               // takeProfit: "TAKE_PROFIT_MARKET" or "TAKE_PROFIT"
  "stopPrice": 29000,          // Trigger price (required)
  "price": 28900,              // Limit execution price (required when type is "STOP" or "TAKE_PROFIT")
  "workingType": "MARK_PRICE", // Trigger price source: "MARK_PRICE" or "CONTRACT_PRICE"
  "stopGuaranteed": false      // Whether to guarantee fill
}
```

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int | System order ID (numeric — may lose precision in JavaScript for large values) |
| `orderID` | string | System order ID (string — use this in JavaScript/TypeScript to avoid precision loss) |
| `symbol` | string | Trading pair |
| `side` | string | Order side |
| `positionSide` | string | Position side |
| `type` | string | Order type |
| `origQty` | string | Order quantity |
| `price` | string | Order price |
| `stopPrice` | string | Trigger price |
| `workingType` | string | Trigger price source |
| `status` | string | Order status: `NEW`, `PARTIALLY_FILLED`, `FILLED`, `CANCELED`, `EXPIRED` |
| `clientOrderId` | string | Custom order ID |

> **Important:** BingX order IDs can exceed JavaScript's `Number.MAX_SAFE_INTEGER`. Always use the string `orderID` field (capital "ID") instead of the numeric `orderId` field when working in JavaScript/TypeScript to avoid precision loss.

---

### 2. Test Place Order

`POST /openApi/swap/v2/trade/order/test`

Parameters are identical to the Place Order endpoint. No actual trade is executed; used to validate signature and parameter correctness. Response structure is the same as Place Order.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `side` | string | Yes | buying and selling direction SELL, BUY |
| `type` | string | Yes | Order type: `MARKET`, `LIMIT`, `STOP_MARKET`, `STOP`, `TAKE_PROFIT_MARKET`, `TAKE_PROFIT`, `TRAILING_STOP_MARKET`, `TRAILING_TP_SL` |
| `positionSide` | string | No | Position direction, required for single position as BOTH, for both long and short positions only LONG or SHORT can be ch|
| `reduceOnly` | string | No | true, false; Default value is false for single position mode; This parameter is not accepted for both long and short pos|
| `price` | float64 | No | Price, represents the trailing stop distance in TRAILING_STOP_MARKET and TRAILING_TP_SL |
| `quantity` | float64 | No | Original quantity, only support units by COIN ,Ordering with quantity U is not currently supported. |
| `stopPrice` | float64 | No | Trigger price (required for stop/take-profit types) |
| `priceRate` | float64 | No | For type: TRAILING_STOP_MARKET or TRAILING_TP_SL; Maximum: 1 |
| `stopLoss` | string | No | Support setting stop loss while placing an order. Only supports type: STOP_MARKET/STOP |
| `takeProfit` | string | No | Support setting take profit while placing an order. Only supports type: TAKE_PROFIT_MARKET/TAKE_PROFIT |
| `clientOrderId` | string | No | Customized order ID for users, with a limit of characters from 1 to 40. Different orders cannot use the same clientOrder|
| `timeInForce` | string | No | Time in force: `GTC`, `IOC`, `FOK`, `PostOnly` |
| `closePosition` | string | No | true, false; all position squaring after triggering, only support STOP_MARKET and TAKE_PROFIT_MARKET; not used with quan|
| `activationPrice` | float64 | No | Used with TRAILING_STOP_MARKET or TRAILING_TP_SL orders, default as the latest price(supporting different workingType) |
| `stopGuaranteed` | string | No | true: Enables the guaranteed stop-loss and take-profit feature; false: Disables the feature; cutfee: Enable the guaranteed stop loss function and enable the VIP guaranteed stop loss fee reduction function. When stopGuaranteed is true or cutfee, the quantity field does not take effect. The guaranteed stop-loss feature is not enabled by default. Supported order types include: STOP_MARKET: Market stop-loss order / TAKE_PROFIT_MARKET: Market take-profit order / STOP: Limit stop-loss order / TAKE_PROFIT: Limit take-profit order / TRIGGER_LIMIT: Stop-limit order with trigger / TRIGGER_MARKET: Market order with trigger for stop-loss. |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair, e.g. `BTC-USDT` |
| `orderId` | int64 | Order ID |
| `side` | string | Order direction: `BUY` / `SELL` |
| `positionSide` | string | `BOTH`, `LONG`, or `SHORT` |
| `type` | string | Order type |
| `clientOrderId` | string | Custom order ID (if provided) |
| `workingType` | string | Trigger price type, e.g. `MARK_PRICE` |

---

### 3. Batch Place Orders

`POST /openApi/swap/v2/trade/batchOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `batchOrders` | LIST<Order> | Yes | Array of orders, up to 5; each order uses the same structure as Place Order |

> Note: `batchOrders` is a JSON array. URL-encode the parameter value when it contains `[` or `{`.

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | List of successfully placed orders |
| `errors` | array | List of failed orders with error details |

---

## II. Cancel Orders

### 4. Cancel Single Order

`DELETE /openApi/swap/v2/trade/order`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `orderId` | int | No | Order ID |
| `clientOrderId` | string | No | Customized order ID for users, with a limit of characters from 1 to 40. The system will convert this field to lowercase.|

**Response `data`:** Details of the cancelled order (same fields as Place Order response).

---

### 5. Batch Cancel Orders

`DELETE /openApi/swap/v2/trade/batchOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `orderIdList` | LIST<int64> | No | system order number, up to 10 orders [1234567,2345678] |
| `clientOrderIdList` | LIST<string> | No | Customized order ID for users, up to 10 orders ["abc1234567","abc2345678"]. The system will convert this field to lowerc|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | array | List of successfully cancelled orders |
| `failed` | array | List of failed cancellations with error details |

---

### 6. Cancel All Open Orders

`DELETE /openApi/swap/v2/trade/allOpenOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT,if you do not fill this field,will delete all type |
| `type` | string | No | LIMIT: Limit Order / MARKET: Market Order / STOP_MARKET: Stop Market Order / TAKE_PROFIT_MARKET: Take Profit Market Orde|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | array | List of successfully cancelled orders |
| `failed` | array | List of failed cancellations with error details |

---

### 7. Cancel All After (Kill Switch)

`POST /openApi/swap/v2/trade/cancelAllAfter`

Starts or stops a countdown timer. When the timer expires, all open orders are automatically cancelled. Useful to prevent orders from lingering after a network disconnection.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Request type: ACTIVATE-Activate, CLOSE-Close |
| `timeOut` | int | Yes | Countdown duration (seconds), range 10–120 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `triggerTime` | int | Timestamp (ms) when auto-cancel will trigger; returns `0` when `CLOSE` |
| `status` | string | Operation result: `ACTIVATED`, `CLOSED`, or `FAILED` |
| `note` | string | Additional information |

---

## III. Order Queries

### 8. Query Single Open Order Status

`GET /openApi/swap/v2/trade/openOrder`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `orderId` | int | No | Order ID |
| `clientOrderId` | string | No | Customized order ID for users, with a limit of characters from 1 to 40. Different orders cannot use the same clientOrder|

**Response `data`:** Open order object with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int | Order ID |
| `symbol` | string | Trading pair |
| `side` | string | Order side |
| `positionSide` | string | Position side |
| `type` | string | Order type |
| `origQty` | string | Order quantity |
| `executedQty` | string | Filled quantity |
| `price` | string | Order price |
| `avgPrice` | string | Average fill price |
| `status` | string | Order status: `NEW`, `PARTIALLY_FILLED`, `FILLED`, `CANCELED`, `EXPIRED` |
| `time` | int | Order placement time (milliseconds) |
| `updateTime` | int | Last update time (milliseconds) |

---

### 9. Query Order Details

`GET /openApi/swap/v2/trade/order`

Query an order by orderId or clientOrderId. Can retrieve orders in any status (including filled or cancelled).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `orderId` | int64 | No | Order ID |
| `clientOrderId` | string | No | Customized order ID for users, with a limit of characters from 1 to 40. The system will convert this field to lowercase.|

**Response `data`:** Order object with the same fields as "Query Single Open Order Status".

---

### 10. Query All Current Open Orders

`GET /openApi/swap/v2/trade/openOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT,When not filled, query all pending orders. When fil|
| `type` | string | No | LIMIT: Limit Order / MARKET: Market Order / STOP_MARKET: Stop Market Order / TAKE_PROFIT_MARKET: Take Profit Market Orde|

**Response `data`:** Array of open order objects; each object has the same fields as "Query Single Open Order Status".

---

### 11. Order History

`GET /openApi/swap/v2/trade/allOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT.If no symbol is specified, it will query the histor|
| `currency` | string | No | Settlement currency: `USDT` or `USDC` |
| `orderId` | int | No | Only return subsequent orders, and return the latest order by default |
| `startTime` | int | No | Start time (millisecond timestamp) |
| `endTime` | int | No | End time (millisecond timestamp) |
| `limit` | int | Yes | Number of results, default 500, max 1000 |

> **Note:** When using `startTime` and `endTime`, the query range must not exceed **7 days**. The server returns error `109400` if the range is larger.

**Response `data`:** Array of historical orders; each object has the same fields as "Query Single Open Order Status".

---

### 12. Liquidation / Force Close Order Query

`GET /openApi/swap/v2/trade/forceOrders`

Query forced liquidation orders triggered by liquidation (LIQUIDATION) or auto-deleveraging (ADL).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `currency` | string | No | Settlement currency: `USDT` or `USDC` |
| `autoCloseType` | string | No | `LIQUIDATION` or `ADL` |
| `startTime` | int | No | Start time (millisecond timestamp) |
| `endTime` | int | No | End time (millisecond timestamp) |
| `limit` | int | No | The number of returned result sets The default value is 50, the maximum value is 100 |

**Response `data`:** Array of forced liquidation orders; same fields as "Order History".

---

### 13. Trade Fill History

`GET /openApi/swap/v2/trade/allFillOrders`

Query individual trade fills for the account, returning detailed information for each actual execution (including fees and realized PnL).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tradingUnit` | string | Yes | Trading unit, optional values: COIN,CONT; COIN directly represent assets such as BTC and ETH, and CONT represents the nu|
| `startTs` | int | Yes | Start time (millisecond timestamp) |
| `endTs` | int | Yes | End time (millisecond timestamp) |
| `orderId` | int | No | If orderId is provided, only the filled orders of that orderId are returned |
| `currency` | string | No | Settlement currency: `USDT` or `USDC` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `tradeId` | int | Trade fill ID |
| `symbol` | string | Trading pair |
| `orderId` | int | Order ID |
| `side` | string | Fill side |
| `price` | string | Fill price |
| `qty` | string | Fill quantity |
| `realizedPnl` | string | Realized PnL |
| `fee` | string | Fee (negative value = paid out) |
| `time` | int | Fill time (milliseconds) |

---

## IV. Position Management

### 14. Close All Positions

`POST /openApi/swap/v2/trade/closeAllPositions`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, for example: BTC-USDT, please use capital letters. |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | array | List of successfully closed position IDs |
| `failed` | array | List of positions that failed to close with error details |

---

### 15. Close Position by positionId

`POST /openApi/swap/v1/trade/closePosition`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `positionId` | string | Yes | Position ID, will close the position with market price |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int | Order ID generated by the position close |
| `positionId` | string | ID of the closed position |

---

### 16. Adjust Isolated Margin

`POST /openApi/swap/v2/trade/positionMargin`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `amount` | float | Yes | Margin adjustment amount (USDT) |
| `positionSide` | string | No | Position side: `LONG` or `SHORT` |
| `positionId` | int | No | Position ID, if it is filled, the system will use the positionId first |
| `type` | int | Yes | adjustment direction 1: increase isolated margin, 2: decrease isolated margin |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `amount` | float | Adjustment amount |
| `type` | int | Adjustment direction |

---

## V. Leverage and Mode Settings

### 17. Query Margin Mode

`GET /openApi/swap/v2/trade/marginType`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `marginType` | string | Current margin mode: `ISOLATED` or `CROSSED` |

---

### 18. Switch Margin Mode

`POST /openApi/swap/v2/trade/marginType`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `marginType` | string | Yes | Target margin mode: `ISOLATED`, `CROSSED`, or `SEPARATE_ISOLATED` (isolated-split mode — allows multiple independent isolated positions for the same pair; `positionId` required when closing) |

**Response `data`:** Empty object (success returns `code: 0`).

---

### 19. Query Leverage and Available Position

`GET /openApi/swap/v2/trade/leverage`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `longLeverage` | int | Current long leverage |
| `shortLeverage` | int | Current short leverage |
| `maxLongLeverage` | int | Maximum long leverage |
| `maxShortLeverage` | int | Maximum short leverage |
| `availableLongVol` | string | Available quantity to open long |
| `availableShortVol` | string | Available quantity to open short |

---

### 20. Set Leverage

`POST /openApi/swap/v2/trade/leverage`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USDT |
| `side` | string | Yes | Side to set: `LONG`, `SHORT` (hedge mode) or `BOTH` (one-way mode) |
| `leverage` | int | Yes | Leverage multiplier |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `leverage` | int | Leverage after update |
| `availableVol` | string | Available quantity to open at current leverage |
| `maxVol` | string | Maximum openable quantity |

---

### 21. Query Position Mode

`GET /openApi/swap/v1/positionSide/dual`

**Parameters:** No additional parameters beyond [common parameters](#common-parameters).

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `dualSidePosition` | bool | `true` = hedge (dual-side) mode; `false` = one-way mode |

---

### 22. Set Position Mode

`POST /openApi/swap/v1/positionSide/dual`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `dualSidePosition` | true | Yes | `"true"` switches to hedge mode; `"false"` switches to one-way mode |

> Note: Ensure there are no open orders or positions before switching position mode, otherwise an error will be returned.

**Response `data`:** Empty object (success returns `code: 0`).

---

## VI. Order Modification and Replacement

### 23. Amend Order

`POST /openApi/swap/v1/trade/amend`

Modifies the quantity of an existing open order. The order must still be open (`NEW` or `PARTIALLY_FILLED`).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `orderId` | string | Yes | System order ID (at least one of `orderId` or `clientOrderId` required) |
| `clientOrderId` | string | Yes | Custom order ID (at least one of `orderId` or `clientOrderId` required) |
| `quantity` | float | Yes | New order quantity after amendment |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | string | Order ID |
| `clientOrderId` | string | Custom order ID |
| `symbol` | string | Trading pair |
| `quantity` | string | Updated order quantity |

---

### 24. Cancel and Replace Order

`POST /openApi/swap/v1/trade/cancelReplace`

Atomic operation: cancels an existing open order and immediately submits a new order, eliminating the time-window risk between cancel and re-submit.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USDT` |
| `cancelOrderId` | int | No | System order ID to cancel (at least one of `cancelOrderId` or `cancelClientOrderId` required) |
| `cancelClientOrderId` | string | No | The original client-defined order ID to be canceled. The system will convert this field to lowercase. Either cancelClien|
| `side` | string | Yes | buying and selling direction SELL, BUY |
| `positionSide` | string | Yes | Position direction, required for single position as BOTH, for both long and short positions only LONG or SHORT can be ch|
| `type` | string | Yes | LIMIT: Limit Order / MARKET: Market Order / STOP_MARKET: Stop Market Order / TAKE_PROFIT_MARKET: Take Profit Market Orde|
| `quantity` | float | No | Original quantity, only support units by COIN ,Ordering with quantity U is not currently supported. |
| `price` | float | No | Price, represents the trailing stop distance in TRAILING_STOP_MARKET and TRAILING_TP_SL |
| `cancelReplaceMode` | string | Yes | STOP_ON_FAILURE: If the order cancellation fails, the replacement order will not continue. ALLOW_FAILURE: Regardless of whether the cancellation succeeds or not, a new order is placed. |
| `cancelRestrictions` | string | No | ONLY_NEW: If the order status is NEW, the cancellation will succeed. ONLY_PENDING: If the order status is PENDING, the cancellation will succeed. ONLY_PARTIALLY_FILLED: If the order status is PARTIALLY_FILLED, the cancellation will succeed. |
| `reduceOnly` | string | No | true, false; Default value is false for single position mode; This parameter is not accepted for both long and short position mode. |
| `stopPrice` | float64 | No | Trigger price (for stop/take-profit types) |
| `priceRate` | float64 | No | For type: TRAILING_STOP_MARKET or TRAILING_TP_SL ; Maximum: 1 |
| `workingType` | string | No | StopPrice trigger price types: MARK_PRICE, CONTRACT_PRICE,  default MARK_PRICE. When the type is STOP or STOP_MARKET, an|
| `stopLoss` | string | No | Support setting stop loss while placing an order. Only supports type: STOP_MARKET/STOP |
| `takeProfit` | string | No | Support setting take profit while placing an order. Only supports type: TAKE_PROFIT_MARKET/TAKE_PROFIT |
| `clientOrderId` | string | No | Customized order ID for users, with a limit of characters from 1 to 40. The system will convert this field to lowercase.|
| `closePosition` | string | No | true, false; all position squaring after triggering, only support STOP_MARKET and TAKE_PROFIT_MARKET; not used with quan|
| `activationPrice` | float64 | No | Used with TRAILING_STOP_MARKET or TRAILING_TP_SL  orders, default as the latest price(supporting different workingType) |
| `stopGuaranteed` | string | No | true: Enables the guaranteed stop-loss and take-profit feature; false: Disables the feature. The guaranteed stop-loss fe|
| `timeInForce` | string | No | Time in force: `GTC`, `IOC`, `FOK`, `PostOnly` |
| `positionId` | int64 | No | In the Separate Isolated mode, closing a position must be transmitted |

> Other new order parameters follow the same rules as Place Order (§1).

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `cancelResult` | string | Cancel result: `SUCCESS` or `FAILED` |
| `newOrderResult` | string | New order result: `SUCCESS` or `FAILED` |
| `cancelOrderId` | int | ID of the cancelled order |
| `newOrderId` | int | ID of the new order |

---

### 25. Batch Cancel and Replace Orders (batchCancelReplace)

`POST /openApi/swap/v1/trade/batchCancelReplace`

Batch atomic operation: simultaneously cancels multiple open orders and submits multiple new orders. Each operation executes independently; partial failures do not affect other orders.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `batchOrders` | string | Yes | A batch of orders, string form of LIST<OrderRequest> |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `result` | array | List of results for each operation; same fields as Cancel and Replace response |

---

## VII. TWAP Orders

### 26. Place TWAP Order

`POST /openApi/swap/v1/twap/order`

Place a Time-Weighted Average Price (TWAP) order that splits a large order into smaller child orders executed at regular intervals.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `side` | string | Yes | Buying and selling direction SELL, BUY |
| `positionSide` | string | Yes | `LONG` or `SHORT` |
| `priceType` | string | Yes | `constant` (price interval) or `percentage` (slippage) |
| `priceVariance` | string | Yes | Price difference in USDT (constant) or slippage percentage (percentage) |
| `triggerPrice` | string | Yes | Trigger price, this price is the condition that limits the execution of strategy orders. For buying, when the market pri|
| `interval` | int64 | Yes | After the strategic order is split, the time interval for order placing is between 5-120s. |
| `amountPerOrder` | string | Yes | The quantity of a single order. After the strategy order is split, the maximum order quantity for a single order。 |
| `totalAmount` | string | Yes | The total number of orders. The total trading volume of strategy orders, which may be split into multiple order executio|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `mainOrderId` | string | TWAP main order ID |

---

### 27. Cancel TWAP Order

`POST /openApi/swap/v1/twap/cancelOrder`

Cancel an active TWAP order.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mainOrderId` | string | Yes | TWAP order ID to cancel |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `mainOrderId` | string | Cancelled TWAP order ID |

---

### 28. Query TWAP Open Orders

`GET /openApi/swap/v1/twap/openOrders`

Query currently active TWAP orders.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USDT; omit for all |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `mainOrderId` | string | TWAP main order ID |
| `symbol` | string | Trading pair |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG` or `SHORT` |
| `priceType` | string | `constant` or `percentage` |
| `priceVariance` | string | Price variance value |
| `triggerPrice` | string | Trigger price |
| `interval` | int64 | Interval in seconds |
| `amountPerOrder` | string | Quantity per child order |
| `totalAmount` | string | Total volume |
| `executedQty` | string | Executed quantity |
| `status` | string | Order status |

---

### 29. Query TWAP Historical Orders

`GET /openApi/swap/v1/twap/historyOrders`

Query historical (completed/cancelled) TWAP orders.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USDT; omit for all |
| `pageIndex` | int64 | Yes | Page number, min 1 |
| `pageSize` | int64 | Yes | Page size, max 1000 |
| `startTime` | int64 | Yes | Start time in milliseconds |
| `endTime` | int64 | Yes | End time in milliseconds |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total number of records |
| `list` | array | Array of TWAP order objects (same fields as open orders plus completion info) |

---

### 30. TWAP Order Details

`GET /openApi/swap/v1/twap/orderDetail`

Query detailed information about a specific TWAP order, including child order execution records.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mainOrderId` | string | Yes | TWAP order ID |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `mainOrderId` | string | TWAP main order ID |
| `symbol` | string | Trading pair |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG` or `SHORT` |
| `totalAmount` | string | Total volume |
| `executedQty` | string | Executed quantity |
| `status` | string | Order status |
| `childOrders` | array | Array of child order execution records |

---

## VIII. Multi-Assets Mode

### 31. Query Multi-Assets Mode

`GET /openApi/swap/v1/trade/assetMode`

Query the current multi-assets mode setting.

**Parameters:** No additional parameters beyond [common parameters](#common-parameters).

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `assetMode` | string | `singleAssetMode` or `multiAssetsMode` |

---

### 32. Switch Multi-Assets Mode

`POST /openApi/swap/v1/trade/assetMode`

Switch between single-asset mode and multi-assets mode. Cannot switch when there are open positions or orders.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `assetMode` | string | Yes | `singleAssetMode` or `multiAssetsMode` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `assetMode` | string | Updated asset mode |

---

### 33. Query Multi-Assets Rules

`GET /openApi/swap/v1/trade/multiAssetsRules`

Query multi-assets mode rules including supported margin assets and their discount rates.

**Parameters:** No additional parameters beyond [common parameters](#common-parameters).

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `rules` | array | List of margin asset rules |

---

### 34. Query Multi-Assets Margin

`GET /openApi/swap/v1/user/marginAssets`

Query margin assets and their valuations under multi-assets mode.

**Parameters:** No additional parameters beyond [common parameters](#common-parameters).

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `assets` | array | List of margin asset details |

---

## IX. Additional Endpoints

### 35. All Orders (V2)

`GET /openApi/swap/v1/trade/fullOrder`

Query all orders (open, filled, cancelled) with extended filter options. Supports pagination.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USDT; omit for all pairs |
| `orderId` | int64 | No | Only return orders after this ID |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int | Yes | Number of results, default 500, max 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | Array of order objects (same fields as Query All Orders) |

---

### 36. Query Historical Transaction Details

`GET /openApi/swap/v2/trade/fillHistory`

Query historical fill/transaction details with pagination support.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `currency` | string | No | `USDC` or `USDT` |
| `orderId` | int64 | No | If orderId is provided, only the filled orders of that orderId are returned |
| `lastFillId` | int64 | No | Last tradeId from previous query, default 0 |
| `startTs` | int64 | Yes | Start timestamp in milliseconds |
| `endTs` | int64 | Yes | End timestamp in milliseconds |
| `pageIndex` | int64 | No | Page number, default 1 |
| `pageSize` | int64 | No | The size of each page must be greater than 0, the maximum value is 1000, if you do not fill in, then the default 50 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `fill_orders` | array | Array of fill detail objects |

---

### 37. Query Position History

`GET /openApi/swap/v1/trade/positionHistory`

Query closed position history with pagination.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `currency` | string | No | `USDC` or `USDT` |
| `positionId` | int64 | No | Position ID; omit for all position histories of the pair |
| `startTs` | int64 | Yes | Start timestamp in milliseconds, max span 3 months |
| `endTs` | int64 | Yes | End timestamp in milliseconds, max span 3 months |
| `pageIndex` | int64 | No | Page number, default 1 |
| `pageSize` | int64 | No | Page size, must be greater than 0, maximum value is 100, if not provided, the default is 1000 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total records |
| `list` | array | Array of historical position objects |

---

### 38. Isolated Margin Change History

`GET /openApi/swap/v1/positionMargin/history`

Query the margin adjustment history for isolated-margin positions.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `positionId` | string | Yes | Position ID |
| `startTime` | int64 | Yes | Start timestamp in milliseconds |
| `endTime` | int64 | Yes | End timestamp in milliseconds |
| `pageIndex` | int64 | Yes | Page number, default 1 |
| `pageSize` | int64 | Yes | Page size, max 100 |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total records |
| `list` | array | Array of margin change records |

---

### 39. Position and Maintenance Margin Ratio

`GET /openApi/swap/v1/maintMarginRatio`

Query the maintenance margin ratio tiers for a trading pair.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `tiers` | array | Array of margin ratio tier objects |

---

### 40. Automatic Margin Addition

`POST /openApi/swap/v1/trade/autoAddMargin`

Enable or disable automatic margin addition for a position in hedge mode.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `positionId` | int64 | Yes | Position ID |
| `functionSwitch` | string | Yes | `true` to enable, `false` to disable |
| `amount` | string | No | Margin amount in USDT (when enabling) |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | Whether the operation succeeded |

---

### 41. One-Click Reverse Position

`POST /openApi/swap/v1/trade/reverse`

Reverse a position direction with one click. Supports immediate or trigger-based reversal.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | `Reverse` (immediate) or `TriggerReverse` (planned) |
| `symbol` | string | Yes | Trading pair, e.g. BTC-USDT |
| `triggerPrice` | string | No | Trigger price (required for `TriggerReverse`) |
| `workingType` | string | No | `MARK_PRICE` or `CONTRACT_PRICE` (required for `TriggerReverse`) |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int | Reverse order ID |

---

### 42. Apply VST

`POST /openApi/swap/v2/trade/getVst`

Apply for or adjust Virtual Simulated Trading (VST) balance.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `adjustType` | string | No | `0` to increase, `1` to decrease |
| `amount` | int64 | No | Adjustment amount |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `balance` | string | Updated VST balance |

---

# BingX Swap Account — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

---

## 1. Query Account Balance

`GET /openApi/swap/v3/user/balance`

Returns balance, equity, margin, and PnL summary for the perpetual futures account.

**Request Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|

**Response `data` — array of balance objects:**

| Field | Type | Description |
|-------|------|-------------|
| `userId` | string | User ID (partially masked) |
| `asset` | string | Settlement asset, e.g. `USDT` |
| `balance` | string | Total wallet balance |
| `equity` | string | Net equity (balance + unrealized PnL) |
| `unrealizedProfit` | string | Total unrealized profit and loss |
| `realisedProfit` | string | Total realized profit and loss |
| `availableMargin` | string | Available margin for new orders |
| `usedMargin` | string | Margin currently in use |
| `freezedMargin` | string | Frozen/reserved margin |

**Example response:**

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

---

## 2. Query Position Data

`GET /openApi/swap/v2/user/positions`

Returns current open positions with PnL, liquidation price, leverage, and margin info.

**Request Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USDT`. Omit for all positions. |

**Response `data` — array of position objects:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair, e.g. `BTC-USDT` |
| `positionId` | string | Position ID |
| `positionSide` | string | Position direction: `LONG` or `SHORT` |
| `isolated` | bool | `true` = isolated margin mode; `false` = cross margin |
| `positionAmt` | string | Total position amount (in base asset) |
| `availableAmt` | string | Amount available to close |
| `unrealizedProfit` | string | Unrealized profit and loss |
| `realisedProfit` | string | Realized profit and loss |
| `initialMargin` | string | Initial margin used for this position |
| `liquidationPrice` | float64 | Estimated liquidation price |
| `avgPrice` | string | Average open price |
| `leverage` | int | Leverage multiplier |
| `positionValue` | string | Notional value of the position |
| `currency` | string | Settlement currency, e.g. `USDT` |

**Example response:**

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
      "liquidationPrice": 294.17
    }
  ]
}
```

---

## 3. Query User Commission Rate

`GET /openApi/swap/v2/user/commissionRate`

Returns the current user's taker and maker fee rates.

**Request Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|

**Response `data.commission`:**

| Field | Type | Description |
|-------|------|-------------|
| `takerCommissionRate` | float64 | Taker fee rate (e.g. `0.0005` = 0.05%) |
| `makerCommissionRate` | float64 | Maker fee rate (e.g. `0.0002` = 0.02%) |

**Example response:**

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

## 4. Query Fund Flow (Income)

`GET /openApi/swap/v2/user/income`

Returns fund flow history for the perpetual futures account.

> - If neither `startTime` nor `endTime` is provided, only the **last 7 days** of data is returned.
> - If `incomeType` is not provided, all types are returned.
> - Only the **last 3 months** of data is retained.

**Request Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USDT` |
| `incomeType` | string | No | Fund flow type (see enum below) |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int64 | No | Number of results. Default `100`, max `1000` |

**`incomeType` Enum:**

| Value | Description |
|-------|-------------|
| `TRANSFER` | Transfer |
| `REALIZED_PNL` | Realized profit and loss |
| `FUNDING_FEE` | Funding fee |
| `TRADING_FEE` | Trading fee |
| `INSURANCE_CLEAR` | Liquidation |
| `TRIAL_FUND` | Trial fund |
| `ADL` | Auto-deleveraging |
| `SYSTEM_DEDUCTION` | System deduction |
| `GTD_PRICE` | Guaranteed price |

**Response `data` — array of income records:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair, e.g. `BTC-USDT` |
| `incomeType` | string | Fund flow type |
| `income` | string | Amount. Positive = inflow, negative = outflow |
| `asset` | string | Asset, e.g. `USDT` |
| `info` | string | Remarks, varies by flow type |
| `time` | int64 | Timestamp in milliseconds |
| `tranId` | string | Transfer ID |
| `tradeId` | string | Original trade ID that triggered this flow |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": [
    {
      "symbol": "LDO-USDT",
      "incomeType": "FUNDING_FEE",
      "income": "-0.0292",
      "asset": "USDT",
      "info": "Funding Fee",
      "time": 1702713615000,
      "tranId": "170***6*2_3*9_20***97",
      "tradeId": "170***6*2_3*9_20***97"
    }
  ]
}
```

---

## 5. Export Fund Flow

`GET /openApi/swap/v2/user/income/export`

Exports fund flow records as an **Excel file** (binary response, not JSON).

> This endpoint returns a binary Excel file, not a JSON response. Handle accordingly (e.g. write to a `.xlsx` file).

**Request Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USDT` |
| `incomeType` | string | No | Fund flow type, optional values:REALIZED_PNL FUNDING_FEE TRADING_FEE INSURANCE_CLEAR TRIAL_FUND ADL SYSTEM_DEDUCTION |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int | No | Number of results. Default `100`, max `1000` |

**Response:** Excel binary file (`.xlsx`). The HTTP response body is the file content directly.

**Example (download to file):**

```typescript
const res = await fetch(url, { headers: { "X-BX-APIKEY": apiKey, "X-SOURCE-KEY": "BX-AI-SKILL" } });
const buffer = await res.arrayBuffer();
fs.writeFileSync("fund_flow.xlsx", Buffer.from(buffer));
```


---

# BingX Swap WebSocket Market Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-swap.bingx.com/swap-market`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends `Ping`; client must reply `Pong` to keep connection alive
- No authentication required for market data streams

---

## Subscribe Market Depth Data

Push limited order book depth information.

### dataType Format

`{symbol}@depth{level}@{interval}`

Examples: `BTC-USDT@depth20@200ms`, `SOL-USDT@depth100@500ms`

BTC-USDT and ETH-USDT support 200ms push interval; other contracts use 500ms.

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "BTC-USDT@depth5@500ms"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-`. Depth level: 5, 10, 20, 50, 100. Interval: 200ms, 500ms |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | Subscription channel identifier |
| data.bids | Bid side: `[[price, qty], ...]` sorted descending by price |
| data.asks | Ask side: `[[price, qty], ...]` sorted ascending by price |
| data.T | Timestamp (ms) |

---

## Subscribe the Latest Trade Detail

Real-time push of trade details.

### dataType Format

`{symbol}@trade`

Example: `BTC-USDT@trade`

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "BTC-USDT@trade"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@trade`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@trade` |
| data.T | Trade timestamp (ms) |
| data.s | Symbol |
| data.p | Trade price |
| data.q | Trade quantity |
| data.m | Is buyer the market maker |

---

## Subscribe K-Line Data

Subscribe to candlestick/kline data for a trading pair.

### dataType Format

`{symbol}@kline_{interval}`

Example: `BTC-USDT@kline_1m`

### Supported Intervals

`1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `2h`, `4h`, `6h`, `8h`, `12h`, `1d`, `3d`, `1w`, `1M`

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "BTC-USDT@kline_1m"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-`. K-line interval type (see Supported Intervals above) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@kline_1m` |
| data.s | Symbol |
| data.K.t | Kline start time (ms) |
| data.K.T | Kline close time (ms) |
| data.K.o | Open price |
| data.K.h | High price |
| data.K.l | Low price |
| data.K.c | Close price |
| data.K.v | Volume |

---

## Subscribe to 24-Hour Price Changes

Push every 1 second.

### dataType Format

`{symbol}@ticker`

Example: `BTC-USDT@ticker`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USDT@ticker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
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

## Subscribe to Latest Price Changes

Real-time push of latest trade price.

### dataType Format

`{symbol}@lastPrice`

Example: `BTC-USDT@lastPrice`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USDT@lastPrice"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@lastPrice`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@lastPrice` |
| data.s | Symbol |
| data.c | Latest price |
| data.T | Timestamp (ms) |

---

## Subscribe to Latest Mark Price Changes

Real-time push of mark price.

### dataType Format

`{symbol}@markPrice`

Example: `BTC-USDT@markPrice`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USDT@markPrice"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@markPrice`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@markPrice` |
| data.s | Symbol |
| data.p | Mark price |
| data.T | Timestamp (ms) |

---

## Subscribe to Book Ticker Streams

Push every 200ms. Best bid and ask price/quantity.

### dataType Format

`{symbol}@bookTicker`

Example: `BTC-USDT@bookTicker`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USDT@bookTicker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
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

## Incremental Depth Information

BTC-USDT and ETH-USDT push every 200ms; other trading pairs push every 800ms.

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
| id | string | Yes | Subscription ID (UUID recommended) |
| reqType | string | Yes | `sub` to subscribe, `unsub` to unsubscribe |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USDT@incrDepth`) |

### How to Maintain Incremental Depth Locally

1. After subscribing, first message has `action: "all"` — full depth snapshot with `lastUpdateId`
2. Subsequent messages have `action: "update"` — incremental updates. The Nth update's `lastUpdateId` should equal `(N-1)th lastUpdateId + 1`
3. If `lastUpdateId` is not continuous, reconnect or cache last 3 incremental updates and try to merge
4. For each incremental update, compare with local depth:
   - Price not in local depth → Add
   - Quantity is 0 → Remove
   - Quantity differs → Update
5. After traversal, update local depth cache and `lastUpdateId`

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USDT@incrDepth` |
| data.action | `all` (full snapshot) or `update` (incremental) |
| data.lastUpdateId | Sequence ID for continuity check |
| data.bids | `[[price, qty], ...]` |
| data.asks | `[[price, qty], ...]` |
| data.T | Timestamp (ms) |


---

# BingX Swap WebSocket Account Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-swap.bingx.com/swap-market?listenKey=<key>`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends `Ping`; client must reply `Pong` to keep connection alive
- **No channel subscription needed** — all events are pushed automatically after connecting with listenKey
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

Extends validity to 60 minutes from this call. Recommended: call every 30 minutes.

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

## listenKey Expired Push

Pushed when the current connection's listenKey expires. After receiving this, reconnect with a new listenKey.

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| e | string | Event type: `listenKeyExpired` |
| E | number | Event time (ms) |
| listenKey | string | The expired listenKey |

### Example

```json
{
  "e": "listenKeyExpired",
  "E": 1676964520421,
  "listenKey": "53c1067059c5401e216ec0562f4e9741f49c3c18239a743653d844a50c4db6c0"
}
```

---

## Account Balance and Position Update (ACCOUNT_UPDATE)

Pushed when account information changes (balance, positions). Not pushed if order status change doesn't affect account/positions.

### Subscription

N/A — Auto-pushed after connecting with listenKey. No explicit subscription needed.

### Trigger Reasons (field `m`)

- `DEPOSIT` — Deposit
- `WITHDRAW` — Withdrawal
- `ORDER` — Order fill
- `FUNDING_FEE` — Funding fee

### FUNDING_FEE Special Rules

- **Isolated position**: Only pushes affected asset balance (B) and the specific position (P) where funding fee occurred
- **Cross position**: Only pushes affected asset balance (B), no position info (P)

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| e | string | `ACCOUNT_UPDATE` |
| E | number | Event time (ms) |
| T | number | Push timestamp (ms) |
| a.m | string | Trigger reason |
| a.B | array | Balance updates |
| a.B[].a | string | Asset name (e.g., `USDT`) |
| a.B[].wb | string | Wallet balance |
| a.B[].cw | string | Cross wallet balance |
| a.B[].bc | string | Balance change |
| a.P | array | Position updates |
| a.P[].s | string | Symbol |
| a.P[].pa | string | Position amount |
| a.P[].ep | string | Entry price |
| a.P[].up | string | Unrealized PnL |
| a.P[].mt | string | Margin type (`cross`/`isolated`) |
| a.P[].ps | string | Position side (`LONG`/`SHORT`/`BOTH`) |

---

## Order Update (ORDER_TRADE_UPDATE)

Pushed when an order is created, filled, or changes status.

### Subscription

N/A — Auto-pushed after connecting with listenKey. No explicit subscription needed.

### Order Direction

- `BUY` — Buy
- `SELL` — Sell

### Order Types

- `MARKET` — Market order
- `LIMIT` — Limit order
- `STOP` — Stop loss limit
- `STOP_MARKET` — Stop loss market
- `TAKE_PROFIT` — Take profit limit
- `TAKE_PROFIT_MARKET` — Take profit market
- `TRIGGER_LIMIT` — Trigger limit order
- `TRIGGER_MARKET` — Trigger market order
- `TRAILING_STOP_MARKET` — Trailing stop market
- `TRAILING_TP_SL` — Trailing TP/SL
- `LIQUIDATION` — Liquidation order

### Execution Types

- `NEW` — New order
- `TRADE` — Trade/fill
- `CANCELED` — Canceled
- `EXPIRED` — Expired
- `CALCULATED` — ADL or liquidation

### Order Status

- `NEW` — Active
- `PARTIALLY_FILLED` — Partially filled
- `FILLED` — Fully filled
- `CANCELED` — Canceled
- `EXPIRED` — Expired

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| e | string | `ORDER_TRADE_UPDATE` |
| E | number | Event time (ms) |
| T | number | Order update time (ms) |
| o.s | string | Symbol |
| o.c | string | Client order ID |
| o.S | string | Side (`BUY`/`SELL`) |
| o.o | string | Order type |
| o.q | string | Original quantity |
| o.p | string | Order price |
| o.ap | string | Average fill price |
| o.x | string | Execution type |
| o.X | string | Order status |
| o.i | number | Order ID |
| o.l | string | Last filled quantity |
| o.z | string | Cumulative filled quantity |
| o.L | string | Last fill price |
| o.n | string | Commission |
| o.N | string | Commission asset |
| o.T | number | Order trade time (ms) |
| o.ps | string | Position side |
| o.rp | string | Realized profit |

---

## Account Config Update (ACCOUNT_CONFIG_UPDATE)

Pushed when leverage or margin mode changes. Full data pushed once on connection, then every 5 seconds.

### Subscription

N/A — Auto-pushed after connecting with listenKey. No explicit subscription needed.

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| e | string | `ACCOUNT_CONFIG_UPDATE` |
| E | number | Event time (ms) |
| ac.s | string | Symbol |
| ac.l | number | Long position leverage |
| ac.S | number | Short position leverage |
| ac.mt | string | Margin type (`cross`/`isolated`) |


---

