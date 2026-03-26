# Binance API Documentation Reference

## Base URLs

### REST API
- **Production**: `https://fapi.binance.com`
- **Testnet**: `https://demo-fapi.binance.com`
- **Spot/Wallet**: `https://api.binance.com` (for `/sapi/` endpoints)

### WebSocket
- **Production**: `wss://fstream.binance.com`
- **Testnet**: `wss://fstream.binancefuture.com`
- Raw stream: `wss://fstream.binance.com/ws/<streamName>`
- Combined stream: `wss://fstream.binance.com/stream?streams=<stream1>/<stream2>`

## Authentication

### API Key Header
```
X-MBX-APIKEY: <your-api-key>
```

### Signing Requests (HMAC SHA256)
1. Construct `totalParams` = query string + request body
2. Compute HMAC SHA256: `signature = HMAC_SHA256(secretKey, totalParams)`
3. Append `&signature=<signature>` to request

**Required parameters for SIGNED endpoints:**
- `timestamp` — millisecond timestamp of request creation
- `signature` — HMAC SHA256 of totalParams
- `recvWindow` (optional) — milliseconds after timestamp the request is valid (default 5000)

**Timing validation:**
```
if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow) {
  // process request
} else {
  // reject request
}
```

### Security Types
| Type | Description |
|------|------------|
| NONE | No authentication required |
| TRADE | Requires API-Key + signature |
| USER_DATA | Requires API-Key + signature |
| USER_STREAM | Requires API-Key only |
| MARKET_DATA | Requires API-Key only |

## Rate Limits

### IP Rate Limits
- **Weight limit**: 2400 per minute (tracked via `X-MBX-USED-WEIGHT-1M` response header)
- Each endpoint has a specific weight cost
- 429 = rate limited, 418 = IP auto-banned (2 min to 3 days, escalating)

### Order Rate Limits
- **Order limit**: 1200 per minute (tracked via `X-MBX-ORDER-COUNT-1M` header)
- Also tracked per 10s via `X-MBX-ORDER-COUNT-10S`
- Rejected/unsuccessful orders may not count

### WebSocket Limits
- Max 10 incoming messages per second per connection
- Max 1024 streams per connection
- Connection valid for 24 hours
- Server sends ping every 3 min; must respond with pong within 10 min

---

## REST API Endpoints

### Futures (USDS-M) — Market Data

#### Exchange Information
- **Endpoint**: `GET /fapi/v1/exchangeInfo`
- **Weight**: 1
- **Security**: NONE
- **Parameters**: None
- **Response**:
```json
{
  "exchangeFilters": [],
  "rateLimits": [
    {"interval": "MINUTE", "intervalNum": 1, "limit": 2400, "rateLimitType": "REQUEST_WEIGHT"},
    {"interval": "MINUTE", "intervalNum": 1, "limit": 1200, "rateLimitType": "ORDERS"}
  ],
  "serverTime": 1565613908500,
  "assets": [
    {"asset": "USDT", "marginAvailable": true, "autoAssetExchange": "0"}
  ],
  "symbols": [
    {
      "symbol": "BLZUSDT",
      "pair": "BLZUSDT",
      "contractType": "PERPETUAL",
      "deliveryDate": 4133404800000,
      "onboardDate": 1598252400000,
      "status": "TRADING",
      "baseAsset": "BLZ",
      "quoteAsset": "USDT",
      "marginAsset": "USDT",
      "pricePrecision": 5,
      "quantityPrecision": 0,
      "baseAssetPrecision": 8,
      "quotePrecision": 8,
      "underlyingType": "COIN",
      "underlyingSubType": ["STORAGE"],
      "triggerProtect": "0.15",
      "liquidationFee": "0.010000",
      "marketTakeBound": "0.30",
      "filters": [
        {"filterType": "PRICE_FILTER", "maxPrice": "300", "minPrice": "0.0001", "tickSize": "0.0001"},
        {"filterType": "LOT_SIZE", "maxQty": "10000000", "minQty": "1", "stepSize": "1"},
        {"filterType": "MARKET_LOT_SIZE", "maxQty": "590119", "minQty": "1", "stepSize": "1"},
        {"filterType": "MAX_NUM_ORDERS", "limit": 200},
        {"filterType": "MIN_NOTIONAL", "notional": "5.0"},
        {"filterType": "PERCENT_PRICE", "multiplierUp": "1.1500", "multiplierDown": "0.8500", "multiplierDecimal": "4"}
      ],
      "orderTypes": ["LIMIT", "MARKET", "STOP", "STOP_MARKET", "TAKE_PROFIT", "TAKE_PROFIT_MARKET", "TRAILING_STOP_MARKET"],
      "timeInForce": ["GTC", "IOC", "FOK", "GTX"]
    }
  ],
  "timezone": "UTC"
}
```
**Key notes:**
- `pricePrecision` / `quantityPrecision` are NOT tickSize/stepSize — use `filters` instead
- Use `PRICE_FILTER.tickSize` for price step, `LOT_SIZE.stepSize` for quantity step
- `MIN_NOTIONAL.notional` = minimum order value (price * qty)
- `PERCENT_PRICE` filter limits order price relative to mark price

#### Order Book
- **Endpoint**: `GET /fapi/v1/depth`
- **Weight**: Varies by limit — 5/10/20/50→2, 100→5, 500→10, 1000→20
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| limit | INT | NO | Default 500; Valid: 5, 10, 20, 50, 100, 500, 1000 |

- **Response**:
```json
{
  "lastUpdateId": 1027024,
  "E": 1589436922972,
  "T": 1589436922959,
  "bids": [["4.00000000", "431.00000000"]],
  "asks": [["4.00000200", "12.00000000"]]
}
```
**Note**: Bids/asks are `[price, quantity]` as strings. RPI orders excluded.

#### Mark Price
- **Endpoint**: `GET /fapi/v1/premiumIndex`
- **Weight**: 1 with symbol, 10 without
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | If omitted, returns all symbols as array |

- **Response**:
```json
{
  "symbol": "BTCUSDT",
  "markPrice": "11793.63104562",
  "indexPrice": "11781.80495970",
  "estimatedSettlePrice": "11781.16138815",
  "lastFundingRate": "0.00038246",
  "interestRate": "0.00010000",
  "nextFundingTime": 1597392000000,
  "time": 1597370495002
}
```
**Note**: `lastFundingRate` is the most recent funding rate. `nextFundingTime` is epoch ms.

#### Get Funding Rate History
- **Endpoint**: `GET /fapi/v1/fundingRate`
- **Weight**: Shared 500/5min/IP with `/fapi/v1/fundingInfo`
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| startTime | LONG | NO | Timestamp ms, inclusive |
| endTime | LONG | NO | Timestamp ms, inclusive |
| limit | INT | NO | Default 100, max 1000 |

- **Response**:
```json
[
  {
    "symbol": "BTCUSDT",
    "fundingRate": "-0.03750000",
    "fundingTime": 1570608000000,
    "markPrice": "34287.54619963"
  }
]
```
**Note**: Returns ascending order. If no time params, returns most recent 200 records.

#### Get Funding Rate Info
- **Endpoint**: `GET /fapi/v1/fundingInfo`
- **Weight**: 0 (shared 500/5min/IP with fundingRate)
- **Security**: NONE
- **Parameters**: None
- **Response**:
```json
[
  {
    "symbol": "BLZUSDT",
    "adjustedFundingRateCap": "0.02500000",
    "adjustedFundingRateFloor": "-0.02500000",
    "fundingIntervalHours": 8,
    "disclaimer": false
  }
]
```

#### Symbol Price Ticker V2
- **Endpoint**: `GET /fapi/v2/ticker/price`
- **Weight**: 1 single symbol, 2 all symbols
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | If omitted, returns all as array |

- **Response**:
```json
{"symbol": "BTCUSDT", "price": "6000.01", "time": 1589437530011}
```

#### Symbol Order Book Ticker
- **Endpoint**: `GET /fapi/v1/ticker/bookTicker`
- **Weight**: 2 single symbol, 5 all symbols
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | If omitted, returns all as array |

- **Response**:
```json
{
  "symbol": "BTCUSDT",
  "bidPrice": "4.00000000",
  "bidQty": "431.00000000",
  "askPrice": "4.00000200",
  "askQty": "9.00000000",
  "time": 1589437530011
}
```
**Note**: RPI orders excluded from book ticker.

#### 24hr Ticker Price Change Statistics
- **Endpoint**: `GET /fapi/v1/ticker/24hr`
- **Weight**: 1 single symbol, 40 all symbols
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |

- **Response**:
```json
{
  "symbol": "BTCUSDT",
  "priceChange": "-94.99999800",
  "priceChangePercent": "-95.960",
  "weightedAvgPrice": "0.29628482",
  "lastPrice": "4.00000200",
  "lastQty": "200.00000000",
  "openPrice": "99.00000000",
  "highPrice": "100.00000000",
  "lowPrice": "0.10000000",
  "volume": "8913.30000000",
  "quoteVolume": "15.30000000",
  "openTime": 1499783499040,
  "closeTime": 1499869899040,
  "firstId": 28385,
  "lastId": 28460,
  "count": 76
}
```

#### Kline/Candlestick Data
- **Endpoint**: `GET /fapi/v1/klines`
- **Weight**: [1,100)→1, [100,500)→2, [500,1000]→5, >1000→10
- **Security**: NONE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| interval | ENUM | YES | 1m,3m,5m,15m,30m,1h,2h,4h,6h,8h,12h,1d,3d,1w,1M |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| limit | INT | NO | Default 500, max 1500 |

- **Response**: Array of arrays:
```json
[
  [
    1499040000000,      // Open time
    "0.01634790",       // Open
    "0.80000000",       // High
    "0.01575800",       // Low
    "0.01577100",       // Close
    "148976.11427815",  // Volume
    1499644799999,      // Close time
    "2434.19055334",    // Quote asset volume
    308,                // Number of trades
    "1756.87402397",    // Taker buy base asset volume
    "28.46694368",      // Taker buy quote asset volume
    "17928899.62484339" // Ignore
  ]
]
```

---

### Futures (USDS-M) — Trade

#### New Order
- **Endpoint**: `POST /fapi/v1/order`
- **Weight**: 0 on IP, 1 on 10s order limit, 1 on 1min order limit
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| side | ENUM | YES | BUY, SELL |
| positionSide | ENUM | NO | Default BOTH (One-way); LONG or SHORT (Hedge Mode) |
| type | ENUM | YES | LIMIT, MARKET, STOP, STOP_MARKET, TAKE_PROFIT, TAKE_PROFIT_MARKET, TRAILING_STOP_MARKET |
| timeInForce | ENUM | NO | GTC, IOC, FOK, GTX, GTD |
| quantity | DECIMAL | NO | |
| reduceOnly | STRING | NO | "true"/"false", default "false". Cannot be sent in Hedge Mode |
| price | DECIMAL | NO | |
| newClientOrderId | STRING | NO | Regex: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK" (default), "RESULT" |
| priceMatch | ENUM | NO | OPPONENT, OPPONENT_5/10/20, QUEUE, QUEUE_5/10/20. Cannot be used with price |
| selfTradePreventionMode | ENUM | NO | EXPIRE_TAKER, EXPIRE_MAKER (default), EXPIRE_BOTH |
| goodTillDate | LONG | NO | Required for GTD orders. Must be > now + 600s |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Additional mandatory parameters by type:**

| Type | Required Params |
|------|----------------|
| LIMIT | timeInForce, quantity, price |
| MARKET | quantity |

- **Response**:
```json
{
  "clientOrderId": "testOrder",
  "cumQty": "0",
  "cumQuote": "0",
  "executedQty": "0",
  "orderId": 22542179,
  "avgPrice": "0.00000",
  "origQty": "10",
  "price": "0",
  "reduceOnly": false,
  "side": "BUY",
  "positionSide": "SHORT",
  "status": "NEW",
  "stopPrice": "9300",
  "closePosition": false,
  "symbol": "BTCUSDT",
  "timeInForce": "GTD",
  "type": "TRAILING_STOP_MARKET",
  "origType": "TRAILING_STOP_MARKET",
  "updateTime": 1566818724722,
  "workingType": "CONTRACT_PRICE",
  "priceProtect": false,
  "priceMatch": "NONE",
  "selfTradePreventionMode": "NONE",
  "goodTillDate": 1693207680000
}
```

#### Place Multiple Orders (Batch)
- **Endpoint**: `POST /fapi/v1/batchOrders`
- **Weight**: 5 on IP, 5 on 10s order limit, 1 on 1min order limit
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| batchOrders | LIST\<JSON\> | YES | Max 5 orders |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Example**: `/fapi/v1/batchOrders?batchOrders=[{"type":"LIMIT","timeInForce":"GTC","symbol":"BTCUSDT","side":"BUY","price":"10001","quantity":"0.001"}]`

**Note**: Batch orders processed concurrently; matching order not guaranteed. Response order matches request order.

#### Cancel Order
- **Endpoint**: `DELETE /fapi/v1/order`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| orderId | LONG | NO | Either orderId or origClientOrderId required |
| origClientOrderId | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

#### Cancel All Open Orders
- **Endpoint**: `DELETE /fapi/v1/allOpenOrders`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**: `{"code": 200, "msg": "The operation of cancel all open order is done."}`

#### Query Order
- **Endpoint**: `GET /fapi/v1/order`
- **Weight**: 1
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| orderId | LONG | NO | Either orderId or origClientOrderId required |
| origClientOrderId | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Note**: Orders not found if: (CANCELED/EXPIRED with no fills AND created > 3 days ago) OR (created > 90 days ago)

#### Query Current All Open Orders
- **Endpoint**: `GET /fapi/v1/openOrders`
- **Weight**: 1 with symbol, 40 without
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

#### Query Current Open Order
- **Endpoint**: `GET /fapi/v1/openOrder`
- **Weight**: 1
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| orderId | LONG | NO | Either orderId or origClientOrderId required |
| origClientOrderId | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Note**: If order has been filled or cancelled, returns "Order does not exist" error.

#### Account Trade List
- **Endpoint**: `GET /fapi/v1/userTrades`
- **Weight**: 5
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| orderId | LONG | NO | |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| fromId | LONG | NO | Trade id to fetch from |
| limit | INT | NO | Default 500, max 1000 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
[
  {
    "buyer": false,
    "commission": "-0.07819010",
    "commissionAsset": "USDT",
    "id": 698759,
    "maker": false,
    "orderId": 25851813,
    "price": "7819.01",
    "qty": "0.002",
    "quoteQty": "15.63802",
    "realizedPnl": "-0.91539999",
    "side": "SELL",
    "positionSide": "SHORT",
    "symbol": "BTCUSDT",
    "time": 1569514978020
  }
]
```
**Note**: Max 7-day range between startTime/endTime. Only last 6 months queryable.

#### Change Margin Type
- **Endpoint**: `POST /fapi/v1/marginType`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| marginType | ENUM | YES | ISOLATED, CROSSED |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**: `{"code": 200, "msg": "success"}`

#### Change Position Mode
- **Endpoint**: `POST /fapi/v1/positionSide/dual`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| dualSidePosition | STRING | YES | "true" = Hedge Mode; "false" = One-way Mode |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

#### Change Initial Leverage
- **Endpoint**: `POST /fapi/v1/leverage`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| leverage | INT | YES | 1 to 125 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
{"leverage": 21, "maxNotionalValue": "1000000", "symbol": "BTCUSDT"}
```

#### Position Information V3
- **Endpoint**: `GET /fapi/v3/positionRisk`
- **Weight**: 5
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response** (One-way mode):
```json
[
  {
    "symbol": "ADAUSDT",
    "positionSide": "BOTH",
    "positionAmt": "30",
    "entryPrice": "0.385",
    "breakEvenPrice": "0.385077",
    "markPrice": "0.41047590",
    "unRealizedProfit": "0.76427700",
    "liquidationPrice": "0",
    "isolatedMargin": "0",
    "notional": "12.31427700",
    "marginAsset": "USDT",
    "isolatedWallet": "0",
    "initialMargin": "0.61571385",
    "maintMargin": "0.08004280",
    "positionInitialMargin": "0.61571385",
    "openOrderInitialMargin": "0",
    "adl": 2,
    "bidNotional": "0",
    "askNotional": "0",
    "updateTime": 1720736417660
  }
]
```
**Note**: Only symbols with position or open orders are returned. In Hedge Mode, LONG and SHORT returned separately.

#### Modify Isolated Position Margin
- **Endpoint**: `POST /fapi/v1/positionMargin`
- **Weight**: 1
- **Security**: TRADE
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| positionSide | ENUM | NO | Default BOTH; LONG/SHORT for Hedge Mode |
| amount | DECIMAL | YES | |
| type | INT | YES | 1 = Add margin, 2 = Reduce margin |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

---

### Futures (USDS-M) — Account

#### Futures Account Balance V3
- **Endpoint**: `GET /fapi/v3/balance`
- **Weight**: 5
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
[
  {
    "accountAlias": "SgsR",
    "asset": "USDT",
    "balance": "122607.35137903",
    "crossWalletBalance": "23.72469206",
    "crossUnPnl": "0.00000000",
    "availableBalance": "23.72469206",
    "maxWithdrawAmount": "23.72469206",
    "marginAvailable": true,
    "updateTime": 1617939110373
  }
]
```

#### Account Information V3
- **Endpoint**: `GET /fapi/v3/account`
- **Weight**: 5
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response** (single-asset mode):
```json
{
  "totalInitialMargin": "0.00000000",
  "totalMaintMargin": "0.00000000",
  "totalWalletBalance": "103.12345678",
  "totalUnrealizedProfit": "0.00000000",
  "totalMarginBalance": "103.12345678",
  "totalPositionInitialMargin": "0.00000000",
  "totalOpenOrderInitialMargin": "0.00000000",
  "totalCrossWalletBalance": "103.12345678",
  "totalCrossUnPnl": "0.00000000",
  "availableBalance": "103.12345678",
  "maxWithdrawAmount": "103.12345678",
  "assets": [
    {
      "asset": "USDT",
      "walletBalance": "23.72469206",
      "unrealizedProfit": "0.00000000",
      "marginBalance": "23.72469206",
      "maintMargin": "0.00000000",
      "initialMargin": "0.00000000",
      "positionInitialMargin": "0.00000000",
      "openOrderInitialMargin": "0.00000000",
      "crossWalletBalance": "23.72469206",
      "crossUnPnl": "0.00000000",
      "availableBalance": "23.72469206",
      "maxWithdrawAmount": "23.72469206",
      "updateTime": 1625474304765
    }
  ],
  "positions": [
    {
      "symbol": "BTCUSDT",
      "positionSide": "BOTH",
      "positionAmt": "1.000",
      "unrealizedProfit": "0.00000000",
      "isolatedMargin": "0.00000000",
      "notional": "0",
      "isolatedWallet": "0",
      "initialMargin": "0",
      "maintMargin": "0",
      "updateTime": 0
    }
  ]
}
```

#### User Commission Rate
- **Endpoint**: `GET /fapi/v1/commissionRate`
- **Weight**: 20
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
{
  "symbol": "BTCUSDT",
  "makerCommissionRate": "0.0002",
  "takerCommissionRate": "0.0004",
  "rpiCommissionRate": "0.00005"
}
```

#### Account Configuration
- **Endpoint**: `GET /fapi/v1/accountConfig`
- **Weight**: 5
- **Security**: USER_DATA
- **Response**:
```json
{
  "feeTier": 0,
  "canTrade": true,
  "canDeposit": true,
  "canWithdraw": true,
  "dualSidePosition": true,
  "updateTime": 0,
  "multiAssetsMargin": false,
  "tradeGroupId": -1
}
```

#### Symbol Configuration
- **Endpoint**: `GET /fapi/v1/symbolConfig`
- **Weight**: 5
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
[
  {
    "symbol": "BTCUSDT",
    "marginType": "CROSSED",
    "isAutoAddMargin": false,
    "leverage": 21,
    "maxNotionalValue": "1000000"
  }
]
```

#### Notional and Leverage Brackets
- **Endpoint**: `GET /fapi/v1/leverageBracket`
- **Weight**: 1
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
[
  {
    "symbol": "ETHUSDT",
    "notionalCoef": 1.50,
    "brackets": [
      {
        "bracket": 1,
        "initialLeverage": 75,
        "notionalCap": 10000,
        "notionalFloor": 0,
        "maintMarginRatio": 0.0065,
        "cum": 0.0
      }
    ]
  }
]
```

#### Get Current Position Mode
- **Endpoint**: `GET /fapi/v1/positionSide/dual`
- **Weight**: 30
- **Security**: USER_DATA
- **Response**: `{"dualSidePosition": true}` (true = Hedge Mode, false = One-way)

#### Get Current Multi-Assets Mode
- **Endpoint**: `GET /fapi/v1/multiAssetsMargin`
- **Weight**: 30
- **Security**: USER_DATA
- **Response**: `{"multiAssetsMargin": true}`

#### Get Income History
- **Endpoint**: `GET /fapi/v1/income`
- **Weight**: 30
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| incomeType | STRING | NO | TRANSFER, REALIZED_PNL, FUNDING_FEE, COMMISSION, etc. |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| page | INT | NO | |
| limit | INT | NO | Default 100, max 1000 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Income types**: TRANSFER, WELCOME_BONUS, REALIZED_PNL, FUNDING_FEE, COMMISSION, INSURANCE_CLEAR, REFERRAL_KICKBACK, COMMISSION_REBATE, API_REBATE, CONTEST_REWARD, CROSS_COLLATERAL_TRANSFER, OPTIONS_PREMIUM_FEE, OPTIONS_SETTLE_PROFIT, INTERNAL_TRANSFER, AUTO_EXCHANGE, DELIVERED_SETTELMENT, COIN_SWAP_DEPOSIT, COIN_SWAP_WITHDRAW, POSITION_LIMIT_INCREASE_FEE, FEE_RETURN, BFUSD_REWARD

- **Response**:
```json
[
  {
    "symbol": "BTCUSDT",
    "incomeType": "COMMISSION",
    "income": "-0.01000000",
    "asset": "USDT",
    "info": "COMMISSION",
    "time": 1570636800000,
    "tranId": 9689322392,
    "tradeId": "2059192"
  }
]
```
**Note**: Only last 3 months of data. Default 7-day window if no time params.

---

### Spot/Wallet

#### User Universal Transfer
- **Endpoint**: `POST /sapi/v1/asset/transfer`
- **Weight**: 900 (per UID)
- **Security**: USER_DATA
- **Requirement**: Must enable "Permits Universal Transfer" for API Key
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | ENUM | YES | Transfer type (see below) |
| asset | STRING | YES | e.g., "USDT" |
| amount | DECIMAL | YES | |
| fromSymbol | STRING | NO | Required for ISOLATEDMARGIN types |
| toSymbol | STRING | NO | Required for ISOLATEDMARGIN types |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Key transfer types for arb bot:**

| Type | Description |
|------|------------|
| MAIN_UMFUTURE | Spot → USDS-M Futures |
| UMFUTURE_MAIN | USDS-M Futures → Spot |
| MAIN_FUNDING | Spot → Funding |
| FUNDING_MAIN | Funding → Spot |
| FUNDING_UMFUTURE | Funding → USDS-M Futures |
| UMFUTURE_FUNDING | USDS-M Futures → Funding |

- **Response**: `{"tranId": 13526853623}`

#### Query Universal Transfer History
- **Endpoint**: `GET /sapi/v1/asset/transfer`
- **Weight**: 1 (per IP)
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | ENUM | YES | |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| current | INT | NO | Page, default 1 |
| size | INT | NO | Default 10, max 100 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
{
  "total": 2,
  "rows": [
    {
      "asset": "USDT",
      "amount": "1",
      "type": "MAIN_UMFUTURE",
      "status": "CONFIRMED",
      "tranId": 11415955596,
      "timestamp": 1544433328000
    }
  ]
}
```
**Note**: `status` values: CONFIRMED, FAILED, PENDING. Only last 6 months queryable.

#### All Coins Info
- **Endpoint**: `GET /sapi/v1/capital/config/getall`
- **Weight**: 10 (per IP)
- **Security**: USER_DATA
- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

- **Response**:
```json
[
  {
    "coin": "USDT",
    "depositAllEnable": true,
    "withdrawAllEnable": true,
    "name": "TetherUS",
    "free": "34941.1",
    "locked": "0",
    "freeze": "0",
    "withdrawing": "0",
    "trading": true,
    "networkList": [
      {
        "network": "ETH",
        "coin": "USDT",
        "withdrawIntegerMultiple": "0.000001",
        "isDefault": true,
        "depositEnable": true,
        "withdrawEnable": true,
        "withdrawFee": "3.2",
        "withdrawMin": "10",
        "withdrawMax": "9999999999",
        "minConfirm": 6,
        "unLockConfirm": 64,
        "withdrawTag": false,
        "estimatedArrivalTime": 2,
        "contractAddress": "0xdac17f958d2ee523a2206206994597c13d831ec7"
      }
    ]
  }
]
```

---

## WebSocket Streams

### Connection Info
- Base URL: `wss://fstream.binance.com`
- Raw stream: `/ws/<streamName>` (e.g., `wss://fstream.binance.com/ws/btcusdt@markPrice`)
- Combined stream: `/stream?streams=<stream1>/<stream2>`
- Combined payloads wrapped: `{"stream":"<streamName>","data":<rawPayload>}`
- All symbols **lowercase** in stream names
- Connection valid 24 hours
- Ping every 3 min from server; pong required within 10 min
- Max 10 incoming messages/sec, max 1024 streams per connection

### Subscribe/Unsubscribe
```json
// Subscribe
{"method": "SUBSCRIBE", "params": ["btcusdt@aggTrade", "btcusdt@depth"], "id": 1}

// Unsubscribe
{"method": "UNSUBSCRIBE", "params": ["btcusdt@depth"], "id": 312}

// List subscriptions
{"method": "LIST_SUBSCRIPTIONS", "id": 3}
```

### Public Streams

#### Mark Price Stream
- **Stream**: `<symbol>@markPrice` (3s) or `<symbol>@markPrice@1s` (1s)
- **All symbols**: `!markPrice@arr` or `!markPrice@arr@1s`
- **Payload**:
```json
{
  "e": "markPriceUpdate",
  "E": 1562305380000,
  "s": "BTCUSDT",
  "p": "11794.15000000",
  "ap": "11794.15000000",
  "i": "11784.62659091",
  "P": "11784.25641265",
  "r": "0.00038167",
  "T": 1562306400000
}
```
| Field | Description |
|-------|------------|
| p | Mark price |
| ap | Mark price moving average |
| i | Index price |
| P | Estimated settle price |
| r | Funding rate |
| T | Next funding time |

#### Individual Symbol Book Ticker Stream
- **Stream**: `<symbol>@bookTicker`
- **Update speed**: Real-time
- **Payload**:
```json
{
  "e": "bookTicker",
  "u": 400900217,
  "E": 1568014460893,
  "T": 1568014460891,
  "s": "BNBUSDT",
  "b": "25.35190000",
  "B": "31.21000000",
  "a": "25.36520000",
  "A": "40.66000000"
}
```
| Field | Description |
|-------|------------|
| u | Order book updateId |
| b | Best bid price |
| B | Best bid qty |
| a | Best ask price |
| A | Best ask qty |

#### All Book Tickers Stream
- **Stream**: `!bookTicker`
- **Update speed**: 5s
- Same payload as individual book ticker

#### Kline/Candlestick Stream
- **Stream**: `<symbol>@kline_<interval>` (e.g., `btcusdt@kline_1m`)
- **Update speed**: 250ms
- **Payload**:
```json
{
  "e": "kline",
  "E": 1638747660000,
  "s": "BTCUSDT",
  "k": {
    "t": 1638747660000,
    "T": 1638747719999,
    "s": "BTCUSDT",
    "i": "1m",
    "f": 100,
    "L": 200,
    "o": "0.0010",
    "c": "0.0020",
    "h": "0.0025",
    "l": "0.0015",
    "v": "1000",
    "n": 100,
    "x": false,
    "q": "1.0000",
    "V": "500",
    "Q": "0.500",
    "B": "123456"
  }
}
```
| Field | Description |
|-------|------------|
| x | Is this kline closed? |
| v | Base asset volume |
| q | Quote asset volume |

#### Aggregate Trade Stream
- **Stream**: `<symbol>@aggTrade`
- **Update speed**: 100ms
- **Payload**:
```json
{
  "e": "aggTrade",
  "E": 123456789,
  "s": "BTCUSDT",
  "a": 5933014,
  "p": "0.001",
  "q": "100",
  "f": 100,
  "l": 105,
  "T": 123456785,
  "m": true
}
```
| Field | Description |
|-------|------------|
| a | Aggregate trade ID |
| p | Price |
| q | Quantity |
| m | Is buyer the market maker? |

#### Individual Symbol Ticker Stream
- **Stream**: `<symbol>@ticker`
- **Update speed**: 2000ms
- 24hr rolling window statistics

#### Partial Book Depth Stream
- **Stream**: `<symbol>@depth<levels>` or `<symbol>@depth<levels>@500ms` or `<symbol>@depth<levels>@100ms`
- **Levels**: 5, 10, 20
- **Update speed**: 250ms, 500ms, or 100ms
- **Payload**:
```json
{
  "e": "depthUpdate",
  "E": 1571889248277,
  "T": 1571889248276,
  "s": "BTCUSDT",
  "U": 390497796,
  "u": 390497878,
  "pu": 390497794,
  "b": [["7403.89", "0.002"]],
  "a": [["7405.96", "3.340"]]
}
```

#### Diff Book Depth Stream
- **Stream**: `<symbol>@depth` or `<symbol>@depth@500ms` or `<symbol>@depth@100ms`
- **Update speed**: 250ms, 500ms, 100ms
- Same payload format as partial book depth

### Private Streams (User Data)

#### User Data Stream Management
- **Start**: `POST /fapi/v1/listenKey` → `{"listenKey": "..."}`
- **Keepalive**: `PUT /fapi/v1/listenKey` (extend 60 min)
- **Close**: `DELETE /fapi/v1/listenKey`
- **Weight**: 1 each
- **Security**: USER_STREAM (API Key only, no signature)
- **Connect**: `wss://fstream.binance.com/ws/<listenKey>`
- ListenKey valid 60 min, must keepalive. If expired, use POST to recreate.

#### Event: ACCOUNT_UPDATE (Balance & Position)
- **Event type**: `ACCOUNT_UPDATE`
- **Triggers**: Balance or position changes (NOT unfilled/cancelled orders)
- **Payload**:
```json
{
  "e": "ACCOUNT_UPDATE",
  "E": 1564745798939,
  "T": 1564745798938,
  "a": {
    "m": "ORDER",
    "B": [
      {
        "a": "USDT",
        "wb": "122624.12345678",
        "cw": "100.12345678",
        "bc": "50.12345678"
      }
    ],
    "P": [
      {
        "s": "BTCUSDT",
        "pa": "0",
        "ep": "0.00000",
        "bep": "0",
        "cr": "200",
        "up": "0",
        "mt": "isolated",
        "iw": "0.00000000",
        "ps": "BOTH"
      }
    ]
  }
}
```
| Field | Description |
|-------|------------|
| m | Reason: ORDER, FUNDING_FEE, DEPOSIT, WITHDRAW, MARGIN_TRANSFER, etc. |
| B[].a | Asset |
| B[].wb | Wallet balance |
| B[].cw | Cross wallet balance |
| B[].bc | Balance change (except PnL & commission) |
| P[].s | Symbol |
| P[].pa | Position amount |
| P[].ep | Entry price |
| P[].bep | Breakeven price |
| P[].up | Unrealized PnL |
| P[].mt | Margin type (isolated/cross) |
| P[].iw | Isolated wallet |
| P[].ps | Position side (BOTH/LONG/SHORT) |

**Funding fee note**: When FUNDING_FEE occurs on crossed position, only balance B is pushed (no position P). For isolated position, both B and P are pushed.

#### Event: ORDER_TRADE_UPDATE (Order Updates)
- **Event type**: `ORDER_TRADE_UPDATE`
- **Payload**:
```json
{
  "e": "ORDER_TRADE_UPDATE",
  "E": 1568879465651,
  "T": 1568879465650,
  "o": {
    "s": "BTCUSDT",
    "c": "TEST",
    "S": "SELL",
    "o": "TRAILING_STOP_MARKET",
    "f": "GTC",
    "q": "0.001",
    "p": "0",
    "ap": "0",
    "sp": "7103.04",
    "x": "NEW",
    "X": "NEW",
    "i": 8886774,
    "l": "0",
    "z": "0",
    "L": "0",
    "N": "USDT",
    "n": "0",
    "T": 1568879465650,
    "t": 0,
    "b": "0",
    "a": "9.91",
    "m": false,
    "R": false,
    "wt": "CONTRACT_PRICE",
    "ot": "TRAILING_STOP_MARKET",
    "ps": "LONG",
    "cp": false,
    "AP": "7476.89",
    "cr": "5.0",
    "pP": false,
    "rp": "0",
    "V": "EXPIRE_TAKER",
    "pm": "OPPONENT",
    "gtd": 0
  }
}
```
| Field | Description |
|-------|------------|
| s | Symbol |
| c | Client order ID |
| S | Side (BUY/SELL) |
| o | Order type |
| f | Time in force |
| q | Original quantity |
| p | Original price |
| ap | Average price |
| x | Execution type: NEW, CANCELED, CALCULATED, EXPIRED, TRADE, AMENDMENT |
| X | Order status: NEW, PARTIALLY_FILLED, FILLED, CANCELED, EXPIRED, EXPIRED_IN_MATCH |
| i | Order ID |
| l | Last filled quantity |
| z | Filled accumulated quantity |
| L | Last filled price |
| N | Commission asset |
| n | Commission |
| T | Order trade time |
| t | Trade ID |
| m | Is maker side? |
| R | Is reduce only? |
| ps | Position side |
| rp | Realized profit |

**Liquidation indicators**:
- `c` starts with "autoclose-": liquidation order
- `c` = "adl_autoclose": ADL auto close
- `c` starts with "settlement_autoclose-": settlement for delisting/delivery

#### Event: ACCOUNT_CONFIG_UPDATE (Leverage Change)
- **Event type**: `ACCOUNT_CONFIG_UPDATE`
- **Leverage change payload**:
```json
{
  "e": "ACCOUNT_CONFIG_UPDATE",
  "E": 1611646737479,
  "T": 1611646737476,
  "ac": {"s": "BTCUSDT", "l": 25}
}
```
- **Multi-assets mode change payload**:
```json
{
  "e": "ACCOUNT_CONFIG_UPDATE",
  "E": 1611646737479,
  "T": 1611646737476,
  "ai": {"j": true}
}
```

---

## Enum Definitions

### Symbol/Contract
| Enum | Values |
|------|--------|
| Symbol type | FUTURE |
| Contract type | PERPETUAL, CURRENT_MONTH, NEXT_MONTH, CURRENT_QUARTER, NEXT_QUARTER, PERPETUAL_DELIVERING |
| Contract status | PENDING_TRADING, TRADING, PRE_DELIVERING, DELIVERING, DELIVERED, PRE_SETTLE, SETTLING, CLOSE |

### Order
| Enum | Values |
|------|--------|
| Order status | NEW, PARTIALLY_FILLED, FILLED, CANCELED, REJECTED, EXPIRED, EXPIRED_IN_MATCH |
| Order type | LIMIT, MARKET, STOP, STOP_MARKET, TAKE_PROFIT, TAKE_PROFIT_MARKET, TRAILING_STOP_MARKET |
| Side | BUY, SELL |
| Position side | BOTH, LONG, SHORT |
| Time in force | GTC, IOC, FOK, GTX (Post Only), GTD (Good Till Date), RPI (Retail Price Improvement) |
| Working type | MARK_PRICE, CONTRACT_PRICE |
| Response type | ACK, RESULT |
| STP mode | EXPIRE_TAKER, EXPIRE_BOTH, EXPIRE_MAKER |
| Price match | NONE, OPPONENT, OPPONENT_5/10/20, QUEUE, QUEUE_5/10/20 |

### Kline Intervals
`1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M`

### Rate Limit Types
| Type | Example |
|------|---------|
| REQUEST_WEIGHT | 2400/min |
| ORDERS | 1200/min |

### Filters
| Filter | Key Fields |
|--------|-----------|
| PRICE_FILTER | minPrice, maxPrice, tickSize |
| LOT_SIZE | minQty, maxQty, stepSize |
| MARKET_LOT_SIZE | minQty, maxQty, stepSize |
| MAX_NUM_ORDERS | limit (e.g., 200) |
| MIN_NOTIONAL | notional (e.g., "5.0") |
| PERCENT_PRICE | multiplierUp, multiplierDown, multiplierDecimal |

---

## Error Codes

### 10xx — General Server/Network
| Code | Name | Description |
|------|------|-------------|
| -1000 | UNKNOWN | Unknown error processing request |
| -1001 | DISCONNECTED | Internal error; unable to process |
| -1002 | UNAUTHORIZED | Not authorized |
| -1003 | TOO_MANY_REQUESTS | Rate limit exceeded |
| -1006 | UNEXPECTED_RESP | Unexpected response from message bus |
| -1007 | TIMEOUT | Timeout waiting for backend |
| -1008 | REQUEST_THROTTLED | Server overloaded. Reduce-only/close-position orders exempt |
| -1015 | TOO_MANY_ORDERS | Order rate limit exceeded |
| -1021 | INVALID_TIMESTAMP | Timestamp outside recvWindow |
| -1022 | INVALID_SIGNATURE | Invalid signature |

### 11xx — Request Issues
| Code | Name | Description |
|------|------|-------------|
| -1100 | ILLEGAL_CHARS | Illegal characters in parameter |
| -1101 | TOO_MANY_PARAMETERS | Too many parameters |
| -1102 | MANDATORY_PARAM_EMPTY | Mandatory parameter missing/empty |
| -1103 | UNKNOWN_PARAM | Unknown parameter sent |
| -1111 | BAD_PRECISION | Precision over maximum |
| -1115 | INVALID_TIF | Invalid timeInForce |
| -1116 | INVALID_ORDER_TYPE | Invalid orderType |
| -1117 | INVALID_SIDE | Invalid side |
| -1121 | BAD_SYMBOL | Invalid symbol |
| -1125 | INVALID_LISTEN_KEY | ListenKey doesn't exist; recreate with POST |

### 20xx — Processing Issues
| Code | Name | Description |
|------|------|-------------|
| -2010 | NEW_ORDER_REJECTED | Order rejected |
| -2011 | CANCEL_REJECTED | Cancel rejected |
| -2013 | NO_SUCH_ORDER | Order does not exist |
| -2015 | REJECTED_MBX_KEY | Invalid API key, IP, or permissions |
| -2018 | BALANCE_NOT_SUFFICIENT | Insufficient balance |
| -2019 | MARGIN_NOT_SUFFICIENT | Insufficient margin |
| -2020 | UNABLE_TO_FILL | Unable to fill |
| -2021 | ORDER_WOULD_IMMEDIATELY_TRIGGER | Order would immediately trigger |
| -2022 | REDUCE_ONLY_REJECT | ReduceOnly order conflicts with existing orders |
| -2024 | POSITION_NOT_SUFFICIENT | Position not sufficient |
| -2025 | MAX_OPEN_ORDER_EXCEEDED | Max open order limit reached |
| -2027 | MAX_LEVERAGE_RATIO | Exceeded max position at current leverage |

### 40xx — Filters/Other
| Code | Name | Description |
|------|------|-------------|
| -4001 | PRICE_LESS_THAN_ZERO | Price < 0 |
| -4003 | QTY_LESS_THAN_ZERO | Quantity < 0 |
| -4014 | PRICE_NOT_INCREASED_BY_TICK_SIZE | Price not on tick |
| -4023 | QTY_NOT_INCREASED_BY_STEP_SIZE | Qty not on step |
| -4028 | INVALID_LEVERAGE | Invalid leverage value |
| -4046 | NO_NEED_TO_CHANGE_MARGIN_TYPE | Already that margin type |
| -4047 | THERE_EXISTS_OPEN_ORDERS | Cannot change margin type with open orders |
| -4048 | THERE_EXISTS_QUANTITY | Cannot change margin type with position |
| -4055 | AMOUNT_MUST_BE_POSITIVE | Amount must be positive |
| -4059 | NO_NEED_TO_CHANGE_POSITION_SIDE | Already that position mode |
| -4061 | POSITION_SIDE_NOT_MATCH | Order position side doesn't match settings |

---

## Notes for Implementation

### Field Format Quirks
- **All numeric values are strings** in REST responses (prices, quantities, rates)
- **Timestamps are milliseconds** (epoch ms), not seconds
- **Boolean fields**: Some use actual booleans (`true`/`false`), others use string `"true"`/`"false"` (notably `dualSidePosition`, `reduceOnly` in requests)
- **Order book**: Bids/asks are `[price_string, qty_string]` arrays
- **Kline data**: Returns array of arrays, NOT objects — positional indexing required

### Critical Implementation Notes
1. **tickSize vs pricePrecision**: Always use `PRICE_FILTER.tickSize` from exchangeInfo, NOT `pricePrecision`
2. **stepSize vs quantityPrecision**: Always use `LOT_SIZE.stepSize`, NOT `quantityPrecision`
3. **PERCENT_PRICE filter**: Order prices are validated against mark price, not last price
4. **MIN_NOTIONAL**: For MARKET orders, notional is checked against mark price * quantity
5. **WebSocket vs REST**: Use WebSocket for real-time data to reduce rate limit pressure
6. **Position modes**: One-way (positionSide=BOTH) vs Hedge (LONG/SHORT) — affects order parameters
7. **Reduce-only**: Cannot use `reduceOnly` in Hedge Mode — use `positionSide` instead
8. **Funding rate**: `lastFundingRate` in premiumIndex is a decimal (0.0001 = 0.01%)
9. **Order rate limit**: IP-based for weight limits, account-based for order limits
10. **Batch orders**: Max 5 per batch, processed concurrently (no guaranteed order)
11. **User data stream**: ListenKey must be kept alive every 60 min; 24h max connection
12. **Transfer endpoint**: Uses `/sapi/v1/` prefix (Spot API), not `/fapi/v1/`
13. **HTTP 503 handling**: "Unknown error" does NOT mean failure — check order status before retrying
14. **WebSocket symbols**: Must be lowercase (e.g., `btcusdt@markPrice`, not `BTCUSDT@markPrice`)
15. **Commission rate**: `makerCommissionRate`/`takerCommissionRate` are decimal (0.0004 = 0.04%)
