# BingX Coin-M Futures API Documentation

Source: https://github.com/BingX-API/api-ai-skills

---

# BingX Coin-M (CSwap) Market Data — API Reference

**Base URLs:** see [`references/base-urls.md`](../references/base-urls.md) | **Auth:** HMAC-SHA256 — see [`references/authentication.md`](../references/authentication.md) | **Response:** `{ "code": 0, "msg": "", "data": ... }`

**Common parameters** (apply to all endpoints below):

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| timestamp | int64 | Yes | Request timestamp in milliseconds |
| recvWindow | int64 | No | Request validity window in ms, max 5000 |

**Note:** Market data endpoints do not require HMAC; a `timestamp` query parameter is required for all endpoints.

**Symbol format:** `BASE-USD` (e.g., `BTC-USD`, `ETH-USD`) — coin-margined, **not** USDT-margined.

---

## 1. Contract Information

### Get Contract Specifications

`GET /openApi/cswap/v1/market/contracts`

Returns specifications for all available Coin-M perpetual contracts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD`; omit for all |

**Response `data`:** Array of contract objects.

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair, e.g. `BTC-USD` |
| `pricePrecision` | int | Decimal places for price |
| `minTickSize` | string | Minimum contract value (in USD) |
| `minTradeValue` | string | Minimum trade value (in USD) |
| `minQty` | string | Minimum order quantity (in contracts) |
| `status` | int | `1` = active |
| `timeOnline` | int64 | Contract listing timestamp (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1720074487610,
  "data": [
    {
      "symbol": "BTC-USD",
      "pricePrecision": 1,
      "minTickSize": "10",
      "minTradeValue": "10",
      "minQty": "1.00000000",
      "status": 1,
      "timeOnline": 1713175200000
    }
  ]
}
```

---

## 2. Order Book Depth

### Query Order Book

`GET /openApi/cswap/v1/market/depth`

Returns current bids and asks for a symbol.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `limit` | int64 | No | The number of returned results. The default is 20 if not filled, optional values: 5, 10, 20, 50, 100, 500, 1000. |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `bids` | array | Array of `[price, quantity]` pairs, best bid first |
| `asks` | array | Array of `[price, quantity]` pairs, best ask first |
| `ts` | int64 | Timestamp of the snapshot (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "bids": [
      ["67500.0", "12"],
      ["67498.5", "5"]
    ],
    "asks": [
      ["67502.0", "8"],
      ["67504.0", "20"]
    ],
    "ts": 1720074487610
  }
}
```

---

## 3. K-line / Candlestick Data

### Get K-line Data

`GET /openApi/cswap/v1/market/klines`

Returns OHLCV candlestick data for a symbol.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `interval` | string | Yes | Time interval, optional values are: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M. |
| `startTime` | int64 | No | Start time, the returned result includes the K-line of this time. |
| `endTime` | int64 | No | End time, the returned result does not include the K-line of this time. |
| `limit` | int | No | Number of candles. Default: 500, max: 1440 |

**Interval Enum:**

`1m` `3m` `5m` `15m` `30m` `1h` `2h` `4h` `6h` `12h` `1d` `3d` `1w` `1M`

**Response `data`:** Array of candlestick arrays.

Each element: `[openTime, open, high, low, close, volume, closeTime]`

| Index | Field | Type | Description |
|-------|-------|------|-------------|
| 0 | `openTime` | int64 | Candle open timestamp (ms) |
| 1 | `open` | string | Open price |
| 2 | `high` | string | High price |
| 3 | `low` | string | Low price |
| 4 | `close` | string | Close price |
| 5 | `volume` | string | Base asset volume (in contracts) |
| 6 | `closeTime` | int64 | Candle close timestamp (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": [
    [1720072800000, "67200.0", "67600.0", "67100.0", "67500.0", "1500", 1720076399999],
    [1720076400000, "67500.0", "67800.0", "67400.0", "67750.0", "1200", 1720079999999]
  ]
}
```

---

## 4. Mark Price & Current Funding Rate

### Get Mark Price and Funding Rate

`GET /openApi/cswap/v1/market/premiumIndex`

Returns the current mark price and funding rate. If `symbol` is omitted, returns data for all contracts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD`. Omit to get all. |

**Response `data`:** Single object (when symbol provided) or array (when omitted).

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `markPrice` | string | Current mark price |
| `indexPrice` | string | Current index price |
| `lastFundingRate` | string | Most recent funding rate (e.g., `"0.0001"` = 0.01%) |
| `nextFundingTime` | int64 | Next funding settlement timestamp (ms) |
| `time` | int64 | Data timestamp (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "symbol": "BTC-USD",
    "markPrice": "67523.5",
    "indexPrice": "67510.0",
    "lastFundingRate": "0.0001",
    "nextFundingTime": 1720080000000,
    "time": 1720074487610
  }
}
```

---

## 5. Open Interest Statistics

### Get Open Interest

`GET /openApi/cswap/v1/market/openInterest`

Returns the total open interest for a symbol.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `openInterest` | string | Total open interest (in contracts) |
| `time` | int64 | Data timestamp (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "symbol": "BTC-USD",
    "openInterest": "52341",
    "time": 1720074487610
  }
}
```

---

## 6. 24hr Ticker Price Change Statistics

### Get 24h Ticker

`GET /openApi/cswap/v1/market/ticker`

Returns 24-hour rolling price change statistics. If `symbol` is omitted, returns data for all contracts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD`. Omit to get all. |

**Response `data`:** Single object (when symbol provided) or array (when omitted).

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `lastPrice` | string | Latest traded price |
| `openPrice` | string | Open price 24 hours ago |
| `highPrice` | string | 24h highest price |
| `lowPrice` | string | 24h lowest price |
| `volume` | string | 24h trading volume (in contracts) |
| `priceChange` | string | Absolute price change over 24h |
| `priceChangePercent` | string | Percentage price change over 24h |
| `time` | int64 | Data timestamp (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "symbol": "BTC-USD",
    "lastPrice": "67523.5",
    "openPrice": "66800.0",
    "highPrice": "68100.0",
    "lowPrice": "66500.0",
    "volume": "98432",
    "priceChange": "723.5",
    "priceChangePercent": "1.083",
    "time": 1720074487610
  }
}
```

---

## Common Error Codes

| Code | Description |
|------|-------------|
| `0` | Success |
| `100400` | Invalid parameter |
| `100500` | Internal server error |
| `100503` | Server busy, retry later |


---

# BingX Coin-M (CSwap) Trade — API Reference

**Base URL (Production Live):** `https://open-api.bingx.com` (fallback: `https://open-api.bingx.pro`)
**Base URL (Production Simulated):** `https://open-api-vst.bingx.com` (fallback: `https://open-api-vst.bingx.pro`)
**Authentication:** All endpoints require HMAC SHA256 signature. See [`references/authentication.md`](../references/authentication.md).
**Response format:** `{ "code": 0, "msg": "", "data": <payload> }` — `code: 0` indicates success.
**Symbol format:** `BASE-USD` (e.g., `BTC-USD`, `ETH-USD`) — coin-margined, NOT USDT-margined.

---

## 1. Place Order

### Place a New Order

`POST /openApi/cswap/v1/trade/order`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `side` | string | Yes | Buying and selling direction SELL, BUY |
| `positionSide` | string | No | Position direction, single position must fill in BOTH, two-way position can only choose LONG or SHORT, if it is empty, t|
| `type` | string | Yes | Order type: `MARKET`, `LIMIT`, `STOP_MARKET`, `STOP`, `TAKE_PROFIT_MARKET`, `TAKE_PROFIT` |
| `quantity` | float64 | No | Order quantity in contracts. Required unless `closePosition: true` |
| `price` | float | No | Commission price |
| `stopPrice` | float | No | Trigger price. Required for `STOP_MARKET`, `STOP`, `TAKE_PROFIT_MARKET`, `TAKE_PROFIT` |
| `timeInForce` | string | No | Effective method, currently supports GTC, IOC, FOK and PostOnly |
| `clientOrderId` | string | No | Custom order ID, 1–40 chars |
| `workingType` | string | No | Trigger price source: `MARK_PRICE` or `CONTRACT_PRICE` (default) |
| `takeProfit` | string | No | Attached take-profit (see TP/SL Object, only on `MARKET`/`LIMIT`) |
| `stopLoss` | string | No | Attached stop-loss (see TP/SL Object, only on `MARKET`/`LIMIT`) |

**TP/SL Object structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `TAKE_PROFIT_MARKET`, `TAKE_PROFIT`, `STOP_MARKET`, or `STOP` |
| `stopPrice` | float | Yes | Trigger price |
| `price` | float | Conditional | Limit execution price (required for `TAKE_PROFIT` / `STOP` types) |
| `workingType` | string | No | `MARK_PRICE` or `CONTRACT_PRICE` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int64 | System-generated order ID |
| `symbol` | string | Trading pair |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG`, `SHORT`, or `BOTH` |
| `type` | string | Order type |
| `price` | float | Order price (0 for market orders) |
| `quantity` | int | Order quantity in contracts |
| `stopPrice` | float | Trigger price (0 if not applicable) |
| `workingType` | string | Trigger price source |
| `timeInForce` | string | Time in force |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "orderId": 1809841379603398656,
    "symbol": "BTC-USD",
    "positionSide": "LONG",
    "side": "BUY",
    "type": "LIMIT",
    "price": 65000,
    "quantity": 1,
    "stopPrice": 0,
    "workingType": "",
    "timeInForce": "GTC"
  }
}
```

---

## 2. Cancel Order

### Cancel an Order

`DELETE /openApi/cswap/v1/trade/cancelOrder`

Cancel an order that is currently in a pending state.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `orderId` | int64 | No | Order ID |
| `clientOrderId` | string | No | Custom order ID. Required if `orderId` not provided |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | int64 | Cancelled order ID |
| `symbol` | string | Trading pair |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | Position direction |
| `type` | string | Order type |
| `quantity` | int | Quantity in contracts |
| `price` | float | Order price |
| `executedQty` | string | Filled quantity |
| `avgPrice` | string | Average fill price |
| `status` | string | `CANCELLED` |
| `time` | int64 | Order creation time (ms) |
| `updateTime` | int64 | Last update time (ms) |

---

## 3. Cancel All Open Orders

### Cancel All Open Orders for a Symbol

`POST /openApi/cswap/v1/trade/allOpenOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | array | Array of successfully cancelled order objects (same fields as Cancel Order response) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1720501468364,
  "data": {
    "success": [
      {
        "symbol": "BTC-USD",
        "orderId": "1809845251327672320",
        "side": "BUY",
        "positionSide": "LONG",
        "type": "LIMIT",
        "quantity": 1,
        "price": "65000",
        "executedQty": "0",
        "avgPrice": "0",
        "status": "CANCELLED",
        "time": 1720335707872,
        "updateTime": 1720335707912
      }
    ]
  }
}
```

---

## 4. Close All Positions

### Close All Open Positions

`POST /openApi/cswap/v1/trade/closeAllPositions`

Closes all currently open positions using market orders.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD`. If omitted, closes all positions across all symbols |

**Response `data`:** Empty object `{}` on success.

---

## 5. Query Open Orders

### Get All Open Orders

`GET /openApi/cswap/v1/trade/openOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USD,When not filled, query all pending orders. When fill|

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | Array of open order objects |

**Order object fields:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `orderId` | string | System order ID |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG`, `SHORT`, or `BOTH` |
| `type` | string | Order type |
| `quantity` | int | Quantity in contracts |
| `price` | string | Order price |
| `executedQty` | string | Filled quantity |
| `avgPrice` | string | Average fill price |
| `stopPrice` | string | Trigger price |
| `status` | string | `Pending`, `PartiallyFilled`, etc. |
| `time` | int64 | Order creation time (ms) |
| `updateTime` | int64 | Last update time (ms) |
| `clientOrderId` | string | Custom order ID |
| `takeProfit` | object | Take-profit settings |
| `stopLoss` | object | Stop-loss settings |
| `workingType` | string | Trigger price source |

---

## 6. Query Order Detail

### Get a Single Order

`GET /openApi/cswap/v1/trade/orderDetail`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `orderId` | int64 | No | Order ID |
| `clientOrderId` | string | No | Custom order ID. Required if `orderId` not provided |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `order` | object | Order detail object (same fields as open orders; see section 5) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "order": {
      "symbol": "SOL-USD",
      "orderId": "1816342420721254400",
      "side": "BUY",
      "positionSide": "Long",
      "type": "LIMIT",
      "quantity": 1,
      "price": "150",
      "executedQty": "0",
      "avgPrice": "0.000",
      "stopPrice": "",
      "profit": "0.0000",
      "commission": "0.0000",
      "status": "Pending",
      "time": 1721884753767,
      "updateTime": 1721884753786,
      "clientOrderId": "",
      "workingType": "MARK_PRICE",
      "takeProfit": {
        "type": "TAKE_PROFIT",
        "quantity": 0,
        "stopPrice": 0,
        "price": 0,
        "workingType": "MARK_PRICE",
        "stopGuaranteed": ""
      },
      "stopLoss": {
        "type": "STOP",
        "quantity": 0,
        "stopPrice": 0,
        "price": 0,
        "workingType": "MARK_PRICE",
        "stopGuaranteed": ""
      }
    }
  }
}
```

---

## 7. Query Order History

### Get Historical Orders

`GET /openApi/cswap/v1/trade/orderHistory`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | There must be a hyphen/ "-" in the trading pair symbol. eg: BTC-USD.If no symbol is specified, it will query the histori|
| `orderId` | int64 | No | Only return subsequent orders, and return the latest order by default |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int | Yes | Number of results (default: 100, max: 1000) |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | Array of historical order objects (same fields as open orders; see section 5) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "data": {
    "orders": [
      {
        "symbol": "SOL-USD",
        "orderId": "1816002957423951872",
        "side": "BUY",
        "positionSide": "LONG",
        "type": "LIMIT",
        "quantity": 1,
        "price": "150.000",
        "executedQty": "0.00000000",
        "avgPrice": "0.000",
        "status": "Filled",
        "time": 1721803819000,
        "updateTime": 1721803856000,
        "workingType": "MARK_PRICE"
      }
    ]
  }
}
```

---

## 8. Query Trade Fill History

### Get All Fill Orders (Trade History)

`GET /openApi/cswap/v1/trade/allFillOrders`

> **Note:** `orderId` is required for Coin-M futures. Provide the order ID to retrieve its fill records.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `orderId` | string | Yes | Order ID |
| `pageIndex` | int64 | No | Page number |
| `pageSize` | int64 | No | Number per page, default 100, max 1000 |

**Response `data`:** Array of fill objects directly.

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | string | Order ID |
| `symbol` | string | Trading pair |
| `type` | string | Order type |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG` or `SHORT` |
| `tradeId` | string | Unique fill/trade ID |
| `volume` | string | Fill quantity in contracts |
| `tradePrice` | string | Fill price |
| `amount` | string | Fill notional value in USD |
| `realizedPnl` | string | Realized profit and loss |
| `commission` | string | Trading fee (negative = fee paid) |
| `currency` | string | Settlement asset (e.g., `BTC`) |
| `buyer` | bool | `true` if this is the buy side |
| `maker` | bool | `true` if this was a maker fill |
| `tradeTime` | int64 | Fill time (ms) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1722147756019,
  "data": [
    {
      "orderId": "1817441228670648320",
      "symbol": "SOL-USD",
      "type": "MARKET",
      "side": "BUY",
      "positionSide": "LONG",
      "tradeId": "97244554",
      "volume": "2",
      "tradePrice": "182.652",
      "amount": "20.00000000",
      "realizedPnl": "0.00000000",
      "commission": "-0.00005475",
      "currency": "SOL",
      "buyer": true,
      "maker": false,
      "tradeTime": 1722146730000
    }
  ]
}
```

---

## 9. Query Liquidation Orders

### Get Liquidation / ADL Orders

`GET /openApi/cswap/v1/trade/forceOrders`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. `BTC-USD`. If omitted, returns all symbols |
| `autoCloseType` | string | No | `LIQUIDATION` (liquidation orders) or `ADL` (auto-deleveraging orders) |
| `startTime` | int64 | No | Start time in milliseconds |
| `endTime` | int64 | No | End time in milliseconds |
| `limit` | int | No | Number of results (default: 50, max: 100) |
| `recvWindow` | int64 | No | Request validity window in milliseconds |
| `timestamp` | int64 | Yes | Request timestamp in milliseconds |

**Response `data`:** Array of liquidation order objects.

| Field | Type | Description |
|-------|------|-------------|
| `orderId` | string | Order ID |
| `symbol` | string | Trading pair |
| `type` | string | Order type |
| `side` | string | `BUY` or `SELL` |
| `positionSide` | string | `LONG` or `SHORT` |
| `price` | string | Order price |
| `quantity` | float64 | Quantity in contracts |
| `stopPrice` | string | Trigger price |
| `workingType` | string | Trigger price source |
| `status` | string | Order status |
| `avgPrice` | string | Average fill price |
| `executedQty` | string | Filled quantity |
| `profit` | string | Realized profit |
| `commission` | string | Trading fee |
| `time` | int64 | Order creation time (ms) |
| `updateTime` | string | Last update time |

---

## 10. Query Leverage

### Get Current Leverage

`GET /openApi/cswap/v1/trade/leverage`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `recvWindow` | int64 | No | Request validity window in milliseconds |
| `timestamp` | int64 | Yes | Request timestamp in milliseconds |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `longLeverage` | int | Current long position leverage |
| `shortLeverage` | int | Current short position leverage |
| `maxLongLeverage` | int | Maximum allowed long leverage |
| `maxShortLeverage` | int | Maximum allowed short leverage |
| `availableLongVol` | string | Available long position volume (contracts) |
| `availableShortVol` | string | Available short position volume (contracts) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1720683803391,
  "data": {
    "symbol": "SOL-USD",
    "longLeverage": 5,
    "shortLeverage": 5,
    "maxLongLeverage": 50,
    "maxShortLeverage": 50,
    "availableLongVol": "4000000",
    "availableShortVol": "4000000"
  }
}
```

---

## 11. Set Leverage

### Change Leverage

`POST /openApi/cswap/v1/trade/leverage`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `side` | string | Yes | For dual-position mode, the leverage rate of long or short positions. LONG represents long position, SHORT represents sh|
| `leverage` | string | Yes | New leverage value (e.g., `10`, `20`) |
| `recvWindow` | int64 | No | Request validity window in milliseconds |
| `timestamp` | int64 | Yes | Request timestamp in milliseconds |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `longLeverage` | int | Updated long leverage |
| `shortLeverage` | int | Updated short leverage |

---

## 12. Query Margin Type

### Get Current Margin Mode

`GET /openApi/cswap/v1/trade/marginType`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `marginType` | string | `CROSSED` (cross margin) or `ISOLATED` (isolated margin) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1721966069132,
  "data": {
    "symbol": "SOL-USD",
    "marginType": "CROSSED"
  }
}
```

---

## 13. Set Margin Type

### Change Margin Mode

`POST /openApi/cswap/v1/trade/marginType`

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `marginType` | string | Yes | Margin type, e.g., ISOLATED, CROSSED |
| `recvWindow` | int64 | No | Request validity window in milliseconds |
| `timestamp` | int64 | Yes | Request timestamp in milliseconds |

**Response `data`:** Empty object `{}` on success.

---

## 14. Adjust Position Margin

### Add or Remove Isolated Margin

`POST /openApi/cswap/v1/trade/positionMargin`

Adjusts the margin amount for an isolated position. Only applicable when margin type is `ISOLATED`.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | Yes | Trading pair, e.g. `BTC-USD` |
| `positionSide` | string | Yes | `LONG` or `SHORT` |
| `amount` | float | Yes | Margin funds |
| `type` | int | Yes | Margin adjustment type: `1` = add, `2` = reduce |

**Response `data`:** Empty object `{}` on success.

---

## 15. Query Commission Rate

### Get Trading Commission Rate

`GET /openApi/cswap/v1/user/commissionRate`

**Parameters:**

No endpoint-specific parameters.

**Response `data`:**

| Field | Type | Description |
|-------|------|-------------|
| `takerCommissionRate` | string | Taker fee rate (e.g., `"0.0005"` = 0.05%) |
| `makerCommissionRate` | string | Maker fee rate (e.g., `"0.0002"` = 0.02%) |

**Example response:**

```json
{
  "code": 0,
  "msg": "",
  "timestamp": 1721365261438,
  "data": {
    "takerCommissionRate": "0.0005",
    "makerCommissionRate": "0.0002"
  }
}
```

---

## 16. Query Account Assets

`GET /openApi/cswap/v1/user/balance`

Query coin-margined account asset balances.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USD; omit for all assets |

**Response `data` (array):**

| Field | Type | Description |
|-------|------|-------------|
| `asset` | string | Asset name |
| `balance` | string | Total balance |
| `equity` | string | Asset net value |
| `unrealizedProfit` | string | Unrealized profit and loss |
| `availableMargin` | string | Available margin |
| `usedMargin` | string | Used margin |
| `freezedMargin` | string | Frozen margin |
| `shortUid` | string | User UID |

---

## 17. Get Current Positions

`GET /openApi/cswap/v1/user/positions`

Query current open positions for coin-margined contracts.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `symbol` | string | No | Trading pair, e.g. BTC-USD; omit for all positions |

**Response `data` (array):**

| Field | Type | Description |
|-------|------|-------------|
| `symbol` | string | Trading pair |
| `positionId` | string | Position ID |
| `positionSide` | string | Position direction: `LONG` or `SHORT` |
| `isolated` | bool | `true` = isolated margin; `false` = cross margin |
| `positionAmt` | string | Position quantity |
| `availableAmt` | string | Closeable quantity |
| `unrealizedProfit` | string | Unrealized profit and loss |
| `initialMargin` | string | Initial margin |
| `liquidationPrice` | float64 | Liquidation price |
| `avgPrice` | string | Average entry price |
| `leverage` | int32 | Leverage |
| `markPrice` | string | Mark price |
| `riskRate` | string | Risk rate |
| `maxMarginReduction` | string | Maximum margin reduction |
| `updateTime` | int64 | Last update time (ms) |

---

## Common Error Codes

| Code | Description |
|------|-------------|
| `0` | Success |
| `100001` | Authentication error — check API key and signature |
| `100400` | Invalid parameter — check required fields and formats |
| `100202` | Insufficient funds / margin |
| `100421` | Symbol restricted from API trading |
| `80016` | Order not found |
| `80017` | Order not found |
| `80012` | Margin is not enough |
| `100500` | Internal server error — retry later |
| `100503` | Server busy — retry later |

---

# BingX Coin-M WebSocket Market Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-cswap-ws.bingx.com/market`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends ping messages; client must reply `Pong` to keep connection alive
- No authentication required for market data streams
- **Symbol format:** `BTC-USD` (not `BTC-USDT`)

---

## Subscription: Trade Detail

Subscribe to real-time trade detail data.

### dataType Format

`{symbol}@trade`

Example: `BTC-USD@trade`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USD@trade"
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
| dataType | `BTC-USD@trade` |
| data | Array of trade records |
| data[].T | Trade time (ms) |
| data[].s | Symbol |
| data[].p | Price |
| data[].q | Quantity |
| data[].m | Is buyer maker |

---

## Subscription: Latest Trade Price

Real-time push of latest transaction price.

### dataType Format

`{symbol}@lastPrice`

Example: `BTC-USD@lastPrice`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USD@lastPrice"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USD@lastPrice`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@lastPrice` |
| data.s | Symbol |
| data.c | Latest price |
| data.T | Timestamp (ms) |

---

## Subscription: Mark Price

Real-time push of mark price.

### dataType Format

`{symbol}@markPrice`

Example: `BTC-USD@markPrice`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USD@markPrice"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USD@markPrice`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@markPrice` |
| data.s | Symbol |
| data.p | Mark price |
| data.T | Timestamp (ms) |

---

## Subscription: Limited Depth

Subscribe to limited order book depth.

### dataType Format

`{symbol}@depth{level}`

Example: `BTC-USD@depth5`

### Supported Levels

`5`, `10`, `20`, `50`, `100`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USD@depth5"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-`. Depth levels: 5, 10, 20, 50, 100 |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@depth5` |
| data.bids | `[[price, qty], ...]` sorted descending |
| data.asks | `[[price, qty], ...]` sorted ascending |
| data.T | Timestamp (ms) |

---

## Subscription: Best Bid and Ask (Book Ticker)

Real-time push.

### dataType Format

`{symbol}@bookTicker`

Example: `BTC-USD@bookTicker`

### Subscription Request

```json
{
  "id": "24dd0e35-56a4-4f7a-af8a-394c7060909c",
  "reqType": "sub",
  "dataType": "BTC-USD@bookTicker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USD@bookTicker`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@bookTicker` |
| data.s | Symbol |
| data.b | Best bid price |
| data.B | Best bid quantity |
| data.a | Best ask price |
| data.A | Best ask quantity |
| data.T | Timestamp (ms) |

---

## Subscription: K-Line Data

Subscribe to candlestick/kline data.

### dataType Format

`{symbol}@kline_{interval}`

Example: `BTC-USD@kline_1m`

### Supported Intervals

`1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `2h`, `4h`, `6h`, `8h`, `12h`, `1d`, `3d`, `1w`, `1M`

### Subscription Request

```json
{
  "id": "e745cd6d-d0f6-4a70-8d5a-043e4c741b40",
  "reqType": "sub",
  "dataType": "BTC-USD@kline_1m"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-`. K-line interval (see Supported Intervals above) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@kline_1m` |
| data.s | Symbol |
| data.K.t | Kline start time |
| data.K.T | Kline close time |
| data.K.o | Open |
| data.K.h | High |
| data.K.l | Low |
| data.K.c | Close |
| data.K.v | Volume |

---

## Subscription: 24-Hour Price Change

Push every 1000ms.

### dataType Format

`{symbol}@ticker`

Example: `BTC-USD@ticker`

### Subscription Request

```json
{
  "id": "975f7385-7f28-4ef1-93af-df01cb9ebb53",
  "reqType": "sub",
  "dataType": "BTC-USD@ticker"
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| id | string | Yes | Subscription ID |
| reqType | string | Yes | `sub` / `unsub` |
| dataType | string | Yes | Symbol must contain `-` (e.g., `BTC-USD@ticker`) |

### Response Fields

| Field | Description |
|-------|-------------|
| dataType | `BTC-USD@ticker` |
| data.s | Symbol |
| data.c | Latest price |
| data.h | 24h high |
| data.l | 24h low |
| data.v | 24h volume |
| data.p | Price change |
| data.P | Price change percent |


---

# BingX Coin-M WebSocket Account Data — API Reference

## Connection

**WebSocket URL:** `wss://open-api-cswap-ws.bingx.com/market?listenKey=<key>`

- All messages are GZIP compressed; client must decompress before parsing
- Server sends ping messages; client must reply `Pong` to keep connection alive
- **No channel subscription needed** — all events are pushed automatically after connecting with listenKey
- Listen Key valid for 1 hour; extend every 30 minutes via PUT REST API
- **Symbol format:** `BTC-USD` (not `BTC-USDT`)

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

Pushed when account information changes (balance, positions).

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
| a.B[].a | string | Asset name (e.g., `BTC`, `ETH`) |
| a.B[].wb | string | Wallet balance |
| a.B[].cw | string | Cross wallet balance |
| a.B[].bc | string | Balance change |
| a.P | array | Position updates |
| a.P[].s | string | Symbol (e.g., `BTC-USD`) |
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
- `STOP` — Stop loss order
- `TAKE_PROFIT` — Take profit order
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

Pushed when leverage or margin mode changes.

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

