# Bybit API Documentation Reference

## Base URLs

### REST API
- **Mainnet**: `https://api.bybit.com` or `https://api.bytick.com`
- **Testnet**: `https://api-testnet.bybit.com`
- **Regional**: Netherlands: `api.bybit.nl`, Turkey: `api.bybit-tr.com`, Kazakhstan: `api.bybit.kz`, UAE: `api.bybit.ae`, EU: `api.bybit.eu`, Indonesia: `api.bybit.id`

### WebSocket Public
- **Spot**: `wss://stream.bybit.com/v5/public/spot`
- **USDT/USDC Perpetual & Futures**: `wss://stream.bybit.com/v5/public/linear`
- **Inverse**: `wss://stream.bybit.com/v5/public/inverse`
- **Options**: `wss://stream.bybit.com/v5/public/option`

### WebSocket Private
- **Mainnet**: `wss://stream.bybit.com/v5/private`
- **Testnet**: `wss://stream-testnet.bybit.com/v5/private`

### WebSocket Order Entry
- **Mainnet**: `wss://stream.bybit.com/v5/trade`

## Authentication

### HMAC-SHA256 Signing

**Required HTTP Headers:**
| Header | Description |
|--------|-------------|
| `X-BAPI-API-KEY` | Your API key |
| `X-BAPI-TIMESTAMP` | UTC timestamp in milliseconds |
| `X-BAPI-SIGN` | HMAC-SHA256 signature (lowercase hex) |
| `X-BAPI-RECV-WINDOW` | Request validity window in ms (default: 5000) |

**Signature Construction:**
- **GET**: `timestamp + api_key + recv_window + queryString`
- **POST**: `timestamp + api_key + recv_window + jsonBodyString`

Sign the concatenated string with HMAC-SHA256 using your API secret, convert to lowercase hex.

**Example (GET):**
```
# Plain text to sign:
"1658384314791XXXXXXXXXX5000category=option&symbol=BTC-29JUL22-25000-C"

# Result signature:
"410e0f387bafb7afd0f1722c068515e09945610124fa11774da1da857b72f30b"
```

**Timestamp Rule:** `server_time - recv_window <= timestamp < server_time + 1000`

### WebSocket Authentication
```json
{
    "op": "auth",
    "args": ["api_key", expires_timestamp_ms, "signature"]
}
```
Signature: HMAC-SHA256 of `"GET/realtime{expires}"` using API secret.

## Common Response Wrapper

All REST responses use this structure:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {},
    "retExtInfo": {},
    "time": 1671017382656
}
```
- `retCode`: 0 = success
- `retMsg`: "OK", "success", "SUCCESS", or "" = success

## Rate Limits

### IP Limits
- **HTTP**: 600 requests per 5-second window per IP
- **WebSocket**: Max 500 connections per 5-minute window; max 1000 connections per IP for market data (counted separately per Spot/Linear/Inverse/Options)

### API Rate Limits (per UID per second)

**Rate Limit Headers:**
| Header | Description |
|--------|-------------|
| `X-Bapi-Limit-Status` | Remaining requests for current endpoint |
| `X-Bapi-Limit` | Current limit for endpoint |
| `X-Bapi-Limit-Reset-Timestamp` | When limit resets (ms) |

**Trade Endpoints:**
| Method | Path | Linear/Inverse | Option | Spot |
|--------|------|---------------|--------|------|
| POST | `/v5/order/create` | 10/s | 20/s | 10/s |
| POST | `/v5/order/amend` | 10/s | 10/s | 10/s |
| POST | `/v5/order/cancel` | 10/s | 20/s | 10/s |
| POST | `/v5/order/cancel-all` | 10/s | 20/s | 1/s |
| POST | `/v5/order/create-batch` | 10/s | 20/s | 10/s |
| GET | `/v5/order/realtime` | 50/s | 50/s | 50/s |
| GET | `/v5/order/history` | 50/s | 50/s | 50/s |
| GET | `/v5/execution/list` | 50/s | 50/s | 50/s |

**Position Endpoints:**
| Method | Path | Limit |
|--------|------|-------|
| GET | `/v5/position/list` | 50/s |
| GET | `/v5/position/closed-pnl` | 50/s |
| POST | `/v5/position/set-leverage` | 10/s |

**Account Endpoints:**
| Method | Path | Limit |
|--------|------|-------|
| GET | `/v5/account/wallet-balance` | 50/s |
| GET | `/v5/account/fee-rate` (linear) | 10/s |
| GET | `/v5/account/fee-rate` (spot) | 5/s |

**Asset Endpoints:**
| Method | Path | Limit |
|--------|------|-------|
| GET | `/v5/asset/transfer/query-account-coins-balance` | 5/s |
| POST | `/v5/asset/transfer/inter-transfer` | 60/min |

**Batch Instructions:** Batch endpoints have their own rate limits (separate from single order limits). Consumption = number_of_requests × orders_per_request. Max 20 orders per request (linear/inverse/option), 10 for spot.

---

## REST API Endpoints

### Market Data

#### Get Instruments Info
- **Endpoint**: `GET /v5/market/instruments-info`
- **Rate Limit**: Shared with market endpoints
- **Description**: Query instrument specifications for online trading pairs.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `spot`, `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name, e.g. `BTCUSDT` (uppercase) |
| status | false | string | Filter by status. Default: `Trading` only for linear/inverse/spot |
| baseCoin | false | string | Base coin (linear/inverse/option only) |
| limit | false | integer | [1, 1000]. Default: 500 |
| cursor | false | string | Pagination cursor |

**Response (Linear/Inverse):**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "linear",
        "list": [{
            "symbol": "BTCUSDT",
            "contractType": "LinearPerpetual",
            "status": "Trading",
            "baseCoin": "BTC",
            "quoteCoin": "USDT",
            "launchTime": "1585526400000",
            "deliveryTime": "0",
            "priceScale": "2",
            "leverageFilter": {
                "minLeverage": "1",
                "maxLeverage": "100.00",
                "leverageStep": "0.01"
            },
            "priceFilter": {
                "minPrice": "0.10",
                "maxPrice": "1999999.80",
                "tickSize": "0.10"
            },
            "lotSizeFilter": {
                "maxOrderQty": "1190.000",
                "minOrderQty": "0.001",
                "qtyStep": "0.001",
                "postOnlyMaxOrderQty": "1190.000",
                "maxMktOrderQty": "500.000",
                "minNotionalValue": "5"
            },
            "unifiedMarginTrade": true,
            "fundingInterval": 480,
            "settleCoin": "USDT",
            "upperFundingRate": "0.00375",
            "lowerFundingRate": "-0.00375",
            "isPreListing": false,
            "preListingInfo": null
        }],
        "nextPageCursor": ""
    }
}
```

**Key fields for arb bot:**
- `fundingInterval`: Funding interval in minutes (e.g., 480 = 8 hours)
- `upperFundingRate` / `lowerFundingRate`: Funding rate caps
- `lotSizeFilter.minOrderQty`, `qtyStep`: Order sizing constraints
- `priceFilter.tickSize`: Price precision
- `leverageFilter.maxLeverage`: Max leverage

---

#### Get Tickers
- **Endpoint**: `GET /v5/market/tickers`
- **Rate Limit**: Shared market endpoint

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `spot`, `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name (uppercase) |

**Response (Linear/Inverse):**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "inverse",
        "list": [{
            "symbol": "BTCUSD",
            "lastPrice": "120635.50",
            "indexPrice": "114890.92",
            "markPrice": "114898.43",
            "prevPrice24h": "105595.90",
            "price24hPcnt": "0.142425",
            "highPrice24h": "131309.30",
            "lowPrice24h": "102007.60",
            "prevPrice1h": "119806.10",
            "openInterest": "240113967",
            "openInterestValue": "2089.79",
            "turnover24h": "115.6907",
            "volume24h": "13713832.0000",
            "fundingRate": "0.0001",
            "nextFundingTime": "1760371200000",
            "ask1Size": "9854",
            "bid1Price": "103401.00",
            "ask1Price": "109152.80",
            "bid1Size": "1063",
            "fundingIntervalHour": "8",
            "fundingCap": "0.005"
        }]
    }
}
```

**Key fields:** `fundingRate`, `nextFundingTime`, `lastPrice`, `markPrice`, `indexPrice`, `bid1Price`, `ask1Price`, `bid1Size`, `ask1Size`

---

#### Get Orderbook
- **Endpoint**: `GET /v5/market/orderbook`

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `spot`, `linear`, `inverse`, `option` |
| symbol | true | string | Symbol name (uppercase) |
| limit | false | integer | Spot: [1,200] default 1. Linear/Inverse: [1,500] default 25 |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "s": "BTCUSDT",
        "a": [["65557.7", "16.606555"]],
        "b": [["65485.47", "47.081829"]],
        "ts": 1716863719031,
        "u": 230704,
        "seq": 1432604333,
        "cts": 1716863718905
    }
}
```

Fields: `s` (symbol), `b` (bids: [price, size]), `a` (asks: [price, size]), `ts` (timestamp ms), `u` (update ID), `seq` (cross sequence), `cts` (matching engine timestamp)

---

#### Get Kline
- **Endpoint**: `GET /v5/market/kline`

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | false | string | Default: `linear`. Options: `spot`, `linear`, `inverse` |
| symbol | true | string | Symbol name |
| interval | true | string | `1,3,5,15,30,60,120,240,360,720,D,W,M` |
| start | false | integer | Start timestamp (ms) |
| end | false | integer | End timestamp (ms) |
| limit | false | integer | [1, 1000]. Default: 200 |

**Response:** Array of `[startTime, open, high, low, close, volume, turnover]`

---

#### Get Funding Rate History
- **Endpoint**: `GET /v5/market/funding/history`

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse` |
| symbol | true | string | Symbol name |
| startTime | false | integer | Start timestamp (ms) |
| endTime | false | integer | End timestamp (ms) |
| limit | false | integer | [1, 200]. Default: 200 |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "linear",
        "list": [{
            "symbol": "ETHPERP",
            "fundingRate": "0.0001",
            "fundingRateTimestamp": "1672041600000"
        }]
    }
}
```

---

#### Get Mark Price Kline
- **Endpoint**: `GET /v5/market/mark-price-kline`
- Same params as kline. Returns `[startTime, open, high, low, close]` (no volume).

#### Get Index Price Kline
- **Endpoint**: `GET /v5/market/index-price-kline`
- Same params and response as mark price kline.

---

### Trade

#### Place Order
- **Endpoint**: `POST /v5/order/create`
- **Rate Limit**: 10/s (linear/inverse/spot), 20/s (option)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse`, `spot`, `option` |
| symbol | true | string | Symbol name |
| side | true | string | `Buy`, `Sell` |
| orderType | true | string | `Market`, `Limit` |
| qty | true | string | Order quantity (always base coin for perps/futures) |
| price | false | string | Required for Limit orders |
| timeInForce | false | string | `GTC` (default), `IOC`, `FOK`, `PostOnly` |
| positionIdx | false | integer | `0`: one-way, `1`: hedge Buy side, `2`: hedge Sell side |
| orderLinkId | false | string | Custom order ID (max 36 chars, must be unique) |
| reduceOnly | false | boolean | `true` to reduce position only |
| closeOnTrigger | false | boolean | Close on trigger order |
| takeProfit | false | string | Take profit price |
| stopLoss | false | string | Stop loss price |
| tpTriggerBy | false | string | TP trigger: `MarkPrice`, `IndexPrice`, `LastPrice` (default) |
| slTriggerBy | false | string | SL trigger: `MarkPrice`, `IndexPrice`, `LastPrice` (default) |
| tpslMode | false | string | `Full` (entire position) or `Partial` |
| triggerPrice | false | string | Conditional order trigger price |
| triggerBy | false | string | `LastPrice`, `IndexPrice`, `MarkPrice` |
| triggerDirection | false | integer | `1`: rise, `2`: fall |
| isLeverage | false | integer | Spot only. `0`: spot, `1`: margin |
| marketUnit | false | string | Spot market order unit: `baseCoin`, `quoteCoin` |
| smpType | false | string | Self-match prevention type |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "orderId": "1321003749386327552",
        "orderLinkId": "spot-test-postonly"
    }
}
```

**Important Notes:**
- Order creation is **asynchronous** — use WebSocket to confirm order status
- Max 500 active orders per symbol (perps/futures)
- Conditional orders: max 10 active per symbol
- Market orders are converted to IOC limit orders internally (slippage protection)
- Pass `qty="0"` + `reduceOnly=true` + `closeOnTrigger=true` to close entire position

**Example (USDT Perp one-way open long):**
```json
{
    "category": "linear",
    "symbol": "BTCUSDT",
    "side": "Buy",
    "orderType": "Limit",
    "qty": "1",
    "price": "25000",
    "timeInForce": "GTC",
    "positionIdx": 0,
    "orderLinkId": "usdt-test-01",
    "reduceOnly": false
}
```

---

#### Cancel Order
- **Endpoint**: `POST /v5/order/cancel`
- **Rate Limit**: 10/s (linear/inverse/spot), 20/s (option)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse`, `spot`, `option` |
| symbol | true | string | Symbol name |
| orderId | false | string | Order ID (either orderId or orderLinkId required) |
| orderLinkId | false | string | Custom order ID |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "orderId": "c6f055d9-7f21-4079-913d-e6523a9cfffa",
        "orderLinkId": "linear-004"
    }
}
```

---

#### Amend Order
- **Endpoint**: `POST /v5/order/amend`
- **Rate Limit**: 10/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | Product type |
| symbol | true | string | Symbol name |
| orderId | false | string | Order ID |
| orderLinkId | false | string | Custom order ID |
| qty | false | string | New order qty |
| price | false | string | New order price |
| triggerPrice | false | string | New trigger price |
| takeProfit | false | string | New TP price |
| stopLoss | false | string | New SL price |

---

#### Get Open & Closed Orders
- **Endpoint**: `GET /v5/order/realtime`
- **Rate Limit**: 50/s
- **Description**: Query unfilled or partially filled orders in real-time. Also supports querying recent 500 closed orders.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name |
| baseCoin | false | string | Base coin |
| settleCoin | false | string | Settle coin (linear: either symbol, baseCoin, or settleCoin required) |
| orderId | false | string | Order ID |
| orderLinkId | false | string | Custom order ID |
| openOnly | false | integer | `0` (default): open orders only. `1`: recent 500 closed orders |
| limit | false | integer | [1, 50]. Default: 20 |
| cursor | false | string | Pagination cursor |

**Response fields:** `orderId`, `orderLinkId`, `symbol`, `price`, `qty`, `side`, `positionIdx`, `orderStatus`, `cancelType`, `rejectReason`, `avgPrice`, `leavesQty`, `cumExecQty`, `cumExecValue`, `cumExecFee`, `timeInForce`, `orderType`, `stopOrderType`, `triggerPrice`, `takeProfit`, `stopLoss`, `tpslMode`, `reduceOnly`, `closeOnTrigger`, `createdTime`, `updatedTime`

**orderStatus values:**
- Open: `New`, `PartiallyFilled`, `Untriggered`
- Closed: `Rejected`, `PartiallyFilledCanceled` (spot only), `Filled`, `Cancelled`, `Triggered`, `Deactivated`

---

#### Get Order History
- **Endpoint**: `GET /v5/order/history`
- **Rate Limit**: 50/s
- **Description**: Query order history (up to 2 years). Same response fields as Get Open Orders.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | Product type |
| symbol | false | string | Symbol name |
| orderId | false | string | Order ID |
| orderLinkId | false | string | Custom order ID |
| orderStatus | false | string | Filter by status |
| startTime | false | integer | Start timestamp (ms). Max 7-day range |
| endTime | false | integer | End timestamp (ms) |
| limit | false | integer | [1, 50]. Default: 20 |
| cursor | false | string | Pagination cursor |

---

#### Get Trade History (Executions)
- **Endpoint**: `GET /v5/execution/list`
- **Rate Limit**: 50/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | Product type |
| symbol | false | string | Symbol name |
| orderId | false | string | Order ID |
| startTime | false | integer | Start timestamp (ms). Max 7-day range |
| endTime | false | integer | End timestamp (ms) |
| execType | false | string | Execution type filter |
| limit | false | integer | [1, 100]. Default: 50 |
| cursor | false | string | Pagination cursor |

**Response fields:** `symbol`, `orderId`, `orderLinkId`, `side`, `orderPrice`, `orderQty`, `leavesQty`, `orderType`, `execFee`, `execId`, `execPrice`, `execQty`, `execType`, `execValue`, `execTime`, `feeCurrency`, `isMaker`, `feeRate`, `markPrice`, `closedSize`, `seq`

---

#### Batch Place Order
- **Endpoint**: `POST /v5/order/create-batch`
- **Rate Limit**: 10/s (linear/spot/inverse), 20/s (option)
- **Max orders**: 20 (linear/inverse/option), 10 (spot)

```json
{
    "category": "linear",
    "request": [
        {"symbol": "BTCUSDT", "side": "Buy", "orderType": "Limit", "qty": "0.01", "price": "25000", "timeInForce": "GTC"},
        {"symbol": "ETHUSDT", "side": "Sell", "orderType": "Limit", "qty": "0.1", "price": "2000", "timeInForce": "GTC"}
    ]
}
```

#### Batch Cancel Order
- **Endpoint**: `POST /v5/order/cancel-batch`
- Same structure with `orderId` or `orderLinkId` per request item.

---

### Position

#### Get Position Info
- **Endpoint**: `GET /v5/position/list`
- **Rate Limit**: 50/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name. Returns data regardless of having position if passed |
| settleCoin | false | string | Settle coin. Linear: either symbol or settleCoin required |
| limit | false | integer | [1, 200]. Default: 20 |
| cursor | false | string | Pagination cursor |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [{
            "positionIdx": 0,
            "riskId": 1,
            "riskLimitValue": "150",
            "symbol": "BTCUSD",
            "side": "Sell",
            "size": "300",
            "avgPrice": "27464.50441675",
            "positionValue": "0.01092319",
            "leverage": "10",
            "markPrice": "28224.50",
            "liqPrice": "",
            "takeProfit": "0.00",
            "stopLoss": "0.00",
            "unrealisedPnl": "-0.00029413",
            "curRealisedPnl": "0.00013123",
            "cumRealisedPnl": "-0.00096902",
            "seq": 5723621632,
            "isReduceOnly": false,
            "createdTime": "1676538056258",
            "updatedTime": "1697673600012"
        }],
        "nextPageCursor": "",
        "category": "inverse"
    }
}
```

**Key fields:** `positionIdx`, `symbol`, `side` ("Buy"=long, "Sell"=short, ""=empty), `size` (always positive), `avgPrice`, `positionValue`, `leverage`, `markPrice`, `liqPrice`, `unrealisedPnl`, `curRealisedPnl`, `cumRealisedPnl`

---

#### Set Leverage
- **Endpoint**: `POST /v5/position/set-leverage`
- **Rate Limit**: 10/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse` |
| symbol | true | string | Symbol name |
| buyLeverage | true | string | [1, maxLeverage]. One-way: must equal sellLeverage |
| sellLeverage | true | string | [1, maxLeverage] |

**Response:** Empty result on success.

---

#### Switch Position Mode
- **Endpoint**: `POST /v5/position/switch-mode`
- Switches between one-way and hedge mode.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear` |
| symbol | false | string | Symbol name (higher priority than coin) |
| coin | false | string | Coin name |
| mode | true | integer | `0`: Merged Single (one-way), `3`: Both Sides (hedge) |

---

#### Set Trading Stop (TP/SL)
- **Endpoint**: `POST /v5/position/trading-stop`

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear`, `inverse` |
| symbol | true | string | Symbol name |
| tpslMode | true | string | `Full` or `Partial` |
| positionIdx | true | integer | `0`, `1`, or `2` |
| takeProfit | false | string | TP price. `0` to cancel |
| stopLoss | false | string | SL price. `0` to cancel |
| tpTriggerBy | false | string | TP trigger type |
| slTriggerBy | false | string | SL trigger type |
| tpSize | false | string | TP size (Partial mode only) |
| slSize | false | string | SL size (Partial mode only) |
| tpOrderType | false | string | `Market` (default), `Limit` |
| slOrderType | false | string | `Market` (default), `Limit` |
| tpLimitPrice | false | string | Limit price for TP (Partial + Limit only) |
| slLimitPrice | false | string | Limit price for SL (Partial + Limit only) |

---

#### Get Closed PnL
- **Endpoint**: `GET /v5/position/closed-pnl`
- **Rate Limit**: 50/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `linear` (USDT/USDC contract) |
| symbol | false | string | Symbol name |
| startTime | false | integer | Start timestamp (ms). Max 7-day range |
| endTime | false | integer | End timestamp (ms) |
| limit | false | integer | [1, 100]. Default: 50 |
| cursor | false | string | Pagination cursor |

**Response fields:** `symbol`, `orderId`, `side`, `qty`, `orderPrice`, `orderType`, `execType`, `closedSize`, `avgEntryPrice`, `avgExitPrice`, `closedPnl`, `leverage`, `openFee`, `closeFee`, `fillCount`, `createdTime`, `updatedTime`

---

### Account

#### Get Wallet Balance
- **Endpoint**: `GET /v5/account/wallet-balance`
- **Rate Limit**: 50/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| accountType | true | string | `UNIFIED` |
| coin | false | string | Coin name(s), comma-separated. e.g., `USDT,USDC` |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [{
            "totalEquity": "3.31216591",
            "accountIMRate": "0",
            "totalMarginBalance": "3.00326056",
            "totalInitialMargin": "0",
            "accountType": "UNIFIED",
            "totalAvailableBalance": "3.00326056",
            "accountMMRate": "0",
            "totalPerpUPL": "0",
            "totalWalletBalance": "3.00326056",
            "totalMaintenanceMargin": "0",
            "coin": [{
                "coin": "BTC",
                "equity": "0.00102964",
                "usdValue": "36.70759517",
                "walletBalance": "0.00102964",
                "unrealisedPnl": "0",
                "cumRealisedPnl": "-0.00000973",
                "locked": "0",
                "marginCollateral": true,
                "collateralSwitch": true,
                "borrowAmount": "0.0",
                "spotBorrow": "0",
                "totalOrderIM": "0",
                "totalPositionIM": "0",
                "totalPositionMM": "0",
                "bonus": "0"
            }]
        }]
    }
}
```

**Key fields:** `totalEquity` (USD), `totalWalletBalance` (USD), `totalAvailableBalance` (USD), `totalPerpUPL` (USD), per-coin: `walletBalance`, `equity`, `unrealisedPnl`, `locked`

---

#### Get Account Info
- **Endpoint**: `GET /v5/account/info`
- **Rate Limit**: Shared

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "marginMode": "REGULAR_MARGIN",
        "updatedTime": "1697078946000",
        "unifiedMarginStatus": 4,
        "isMasterTrader": false,
        "spotHedgingStatus": "OFF"
    }
}
```

**marginMode values:** `ISOLATED_MARGIN`, `REGULAR_MARGIN`, `PORTFOLIO_MARGIN`

---

#### Get Fee Rate
- **Endpoint**: `GET /v5/account/fee-rate`
- **Rate Limit**: 10/s (linear/inverse), 5/s (spot/option)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| category | true | string | `spot`, `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name (for linear/inverse/spot) |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [{
            "symbol": "ETHUSDT",
            "takerFeeRate": "0.0006",
            "makerFeeRate": "0.0001"
        }]
    }
}
```

---

### Asset

#### Create Internal Transfer
- **Endpoint**: `POST /v5/asset/transfer/inter-transfer`
- **Rate Limit**: 60/min

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| transferId | true | string | UUID (manually generated) |
| coin | true | string | Coin name (uppercase) |
| amount | true | string | Transfer amount |
| fromAccountType | true | string | Source account type |
| toAccountType | true | string | Destination account type |

**Account types:** `UNIFIED`, `FUND`, `CONTRACT`, `SPOT`

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "transferId": "42c0cfb0-6bca-c242-bc76-4e6df6cbab16",
        "status": "SUCCESS"
    }
}
```

---

#### Get All Coins Balance
- **Endpoint**: `GET /v5/asset/transfer/query-account-coins-balance`
- **Rate Limit**: 5/s

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| accountType | true | string | Account type (e.g., `UNIFIED`, `FUND`) |
| coin | false | string | Coin name. **Mandatory for UNIFIED**, supports up to 10 coins |
| memberId | false | string | Sub-account user ID (master API key only) |

**Response:**
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "memberId": "XXXX",
        "accountType": "FUND",
        "balance": [{
            "coin": "USDC",
            "transferBalance": "0",
            "walletBalance": "0",
            "bonus": ""
        }]
    }
}
```

---

## WebSocket Streams

### Connection Management
- **Heartbeat**: Send `{"op": "ping"}` every 20 seconds
- **Max alive time**: Default 10 minutes without ping/data. Configurable via `max_active_time` param (30s-600s): `wss://stream.bybit.com/v5/private?max_active_time=1m`
- **Subscribe**: `{"op": "subscribe", "args": ["topic1", "topic2"]}`
- **Unsubscribe**: `{"op": "unsubscribe", "args": ["topic1"]}`
- **Args limit**: Max 21,000 characters per public connection

### Public Streams

#### Orderbook
- **Topic**: `orderbook.{depth}.{symbol}` (e.g., `orderbook.1.BTCUSDT`)
- **Depths**: Linear/Inverse: 1 (10ms), 50 (20ms), 200 (100ms), 1000 (200ms). Spot: same. Option: 25 (20ms), 100 (100ms)
- **Type**: `snapshot` (initial + level 1 always), `delta` (incremental updates)

```json
{
    "topic": "orderbook.50.BTCUSDT",
    "type": "snapshot",
    "ts": 1672304484978,
    "data": {
        "s": "BTCUSDT",
        "b": [["16493.50", "0.006"], ["16493.00", "0.100"]],
        "a": [["16611.00", "0.029"], ["16612.00", "0.213"]],
        "u": 18521288,
        "seq": 7961638724
    },
    "cts": 1672304484976
}
```

**Delta processing:** size=0 → delete entry; new price → insert; existing price → update value.

---

#### Trade
- **Topic**: `publicTrade.{symbol}`
- **Push**: Real-time

```json
{
    "topic": "publicTrade.BTCUSDT",
    "type": "snapshot",
    "ts": 1672304486868,
    "data": [{
        "T": 1672304486865,
        "s": "BTCUSDT",
        "S": "Buy",
        "v": "0.001",
        "p": "16578.50",
        "L": "PlusTick",
        "i": "20f43950-d8dd-5b31-9112-a178eb6023af",
        "BT": false,
        "seq": 1783284617
    }]
}
```

Fields: `T` (trade timestamp ms), `s` (symbol), `S` (side), `v` (size), `p` (price), `L` (direction: PlusTick/MinusTick/ZeroPlusTick/ZeroMinusTick), `i` (trade ID)

---

#### Ticker
- **Topic**: `tickers.{symbol}`
- **Push**: Derivatives 100ms, Spot 50ms
- **Type**: `snapshot` + `delta` (Linear/Inverse), `snapshot` only (Spot/Option)

```json
{
    "topic": "tickers.BTCUSDT",
    "type": "snapshot",
    "data": {
        "symbol": "BTCUSDT",
        "lastPrice": "66666.60",
        "markPrice": "66666.60",
        "indexPrice": "115418.19",
        "fundingRate": "-0.005",
        "nextFundingTime": "1760342400000",
        "bid1Price": "66666.60",
        "bid1Size": "23789.165",
        "ask1Price": "66666.70",
        "ask1Size": "23775.469",
        "openInterest": "492373.72",
        "volume24h": "73191.3870",
        "turnover24h": "4936790807.6521",
        "highPrice24h": "79266.30",
        "lowPrice24h": "65076.90",
        "prevPrice24h": "79206.20",
        "price24hPcnt": "-0.158315",
        "fundingIntervalHour": "8",
        "fundingCap": "0.005"
    },
    "cs": 9532239429,
    "ts": 1760325052630
}
```

**Note:** Delta messages only contain changed fields. Missing fields = unchanged.

---

#### Kline
- **Topic**: `kline.{interval}.{symbol}` (e.g., `kline.5.BTCUSDT`)
- **Push**: 1-60s
- **Intervals**: `1,3,5,15,30,60,120,240,360,720,D,W,M`

```json
{
    "topic": "kline.5.BTCUSDT",
    "data": [{
        "start": 1672324800000,
        "end": 1672325099999,
        "interval": "5",
        "open": "16649.5",
        "close": "16677",
        "high": "16677",
        "low": "16608",
        "volume": "2.081",
        "turnover": "34666.4005",
        "confirm": false,
        "timestamp": 1672324988882
    }],
    "ts": 1672324988882,
    "type": "snapshot"
}
```

`confirm`: `true` = candle closed, `false` = still open/updating.

---

### Private Streams

#### Position
- **Topic**: `position` (all categories) or `position.linear`, `position.inverse`, `position.option`
- **Note**: Triggers on every order create/amend/cancel (even without position change)

```json
{
    "id": "1003076014fb7eedb-c7e6-45d6-a8c1-270f0169171a",
    "topic": "position",
    "creationTime": 1697682317044,
    "data": [{
        "positionIdx": 2,
        "riskId": 1,
        "riskLimitValue": "2000000",
        "symbol": "BTCUSDT",
        "side": "",
        "size": "0",
        "entryPrice": "0",
        "leverage": "10",
        "breakEvenPrice": "93556.73034991",
        "positionValue": "0",
        "markPrice": "28184.5",
        "positionIM": "0",
        "positionMM": "0",
        "takeProfit": "0",
        "stopLoss": "0",
        "trailingStop": "0",
        "unrealisedPnl": "0",
        "curRealisedPnl": "1.26",
        "cumRealisedPnl": "-25.06579337",
        "liqPrice": "0",
        "category": "linear",
        "positionStatus": "Normal",
        "adlRankIndicator": 0,
        "autoAddMargin": 0,
        "seq": 8327597863,
        "isReduceOnly": false,
        "createdTime": "1694402496913",
        "updatedTime": "1697682317038"
    }]
}
```

---

#### Execution
- **Topic**: `execution` (all) or `execution.spot`, `execution.linear`, `execution.inverse`, `execution.option`

```json
{
    "topic": "execution",
    "id": "386825804_BTCUSDT_140612148849382",
    "creationTime": 1746270400355,
    "data": [{
        "category": "linear",
        "symbol": "BTCUSDT",
        "closedSize": "0.5",
        "execFee": "26.3725275",
        "execId": "0ab1bdf7-4219-438b-b30a-32ec863018f7",
        "execPrice": "95900.1",
        "execQty": "0.5",
        "execType": "Trade",
        "execValue": "47950.05",
        "feeRate": "0.00055",
        "markPrice": "95901.48",
        "leavesQty": "0",
        "orderId": "9aac161b-8ed6-450d-9cab-c5cc67c21784",
        "orderPrice": "94942.5",
        "orderQty": "0.5",
        "orderType": "Market",
        "side": "Sell",
        "execTime": "1746270400353",
        "isMaker": false,
        "seq": 140612148849382,
        "execPnl": "0.05",
        "createType": "CreateByUser",
        "feeCurrency": "USDT"
    }]
}
```

---

#### Order
- **Topic**: `order` (all) or `order.spot`, `order.linear`, `order.inverse`, `order.option`

```json
{
    "id": "5923240c6880ab-c59f-420b-9adb-3639adc9dd90",
    "topic": "order",
    "creationTime": 1672364262474,
    "data": [{
        "symbol": "ETH-30DEC22-1400-C",
        "orderId": "5cf98598-39a7-459e-97bf-76ca765ee020",
        "side": "Sell",
        "orderType": "Market",
        "cancelType": "UNKNOWN",
        "price": "72.5",
        "qty": "1",
        "timeInForce": "IOC",
        "orderStatus": "Filled",
        "orderLinkId": "",
        "reduceOnly": false,
        "leavesQty": "",
        "cumExecQty": "1",
        "cumExecValue": "75",
        "avgPrice": "75",
        "positionIdx": 0,
        "cumExecFee": "0.358635",
        "closedPnl": "0",
        "createdTime": "1672364262444",
        "updatedTime": "1672364262457",
        "rejectReason": "EC_NoError",
        "category": "option",
        "smpType": "None"
    }]
}
```

---

#### Wallet
- **Topic**: `wallet`
- **Note**: No snapshot on subscription. Unrealised PnL changes do NOT trigger events.

```json
{
    "id": "592324d2bce751-ad38-48eb-8f42-4671d1fb4d4e",
    "topic": "wallet",
    "creationTime": 1700034722104,
    "data": [{
        "accountIMRate": "0",
        "accountMMRate": "0",
        "totalEquity": "10262.91335023",
        "totalWalletBalance": "9684.46297164",
        "totalMarginBalance": "9684.46297164",
        "totalAvailableBalance": "9556.6056555",
        "totalPerpUPL": "0",
        "totalInitialMargin": "0",
        "totalMaintenanceMargin": "0",
        "accountType": "UNIFIED",
        "coin": [{
            "coin": "BTC",
            "equity": "0.00102964",
            "usdValue": "36.70759517",
            "walletBalance": "0.00102964",
            "unrealisedPnl": "0",
            "cumRealisedPnl": "-0.00000973",
            "locked": "0",
            "collateralSwitch": true,
            "marginCollateral": true,
            "spotBorrow": "0"
        }]
    }]
}
```

---

## Error Codes

### HTTP Status Codes
| Code | Description |
|------|-------------|
| 400 | Bad request |
| 401 | Invalid authentication |
| 403 | Forbidden (IP rate limit or US IP) |
| 404 | Path not found or category mismatch |
| 429 | System-level frequency protection |

### Common API Error Codes (retCode)
| Code | Description |
|------|-------------|
| 0 | OK - Success |
| 10000 | Server Timeout |
| 10001 | Request parameter error |
| 10002 | Request time exceeds time window |
| 10003 | API key invalid |
| 10004 | Invalid signature |
| 10005 | Permission denied |
| 10006 | Too many visits (rate limit exceeded) |
| 10007 | User authentication failed |
| 10009 | IP has been banned |
| 10010 | Unmatched IP (API key IP whitelist) |
| 10016 | Server error |
| 10018 | IP rate limit exceeded |

### Trade Error Codes
| Code | Description |
|------|-------------|
| 110001 | Order does not exist |
| 110003 | Order price exceeds allowable range |
| 110004 | Wallet balance insufficient |
| 110007 | Available balance insufficient |
| 110008 | Order completed or cancelled |
| 110009 | Stop orders exceed max limit |
| 110012 | Insufficient available balance |
| 110013 | Cannot set leverage due to risk limit |
| 110020 | Max 500 active orders |
| 110022 | Cannot increase order quantity |
| 110024 | Cannot switch position mode with existing position |
| 110025 | Position mode not modified |
| 110026 | Margin mode not modified |
| 110030 | Duplicate orderId |
| 110040 | Order will trigger forced liquidation |
| 110043 | Leverage not modified |
| 110072 | Duplicate orderLinkId |
| 110094 | Order notional value below lower limit |

---

## Notes for Implementation

### Response Wrapper
All responses: `{retCode, retMsg, result{}, retExtInfo{}, time}`

### Category Parameter
- `linear`: USDT perpetual, USDT Futures, USDC perpetual, USDC Futures
- `inverse`: Inverse perpetual, Inverse Futures
- `spot`: Spot trading
- `option`: Options

### Symbol Format
- **No separator**: `BTCUSDT`, `ETHUSDT` (uppercase only)
- No suffix needed (unlike OKX's `-SWAP`)
- No underscore (unlike Gate.io's `BTC_USDT`)

### Position Mode
- `positionIdx=0`: One-way mode (MergedSingle)
- `positionIdx=1`: Hedge mode Buy side
- `positionIdx=2`: Hedge mode Sell side
- Switch via `POST /v5/position/switch-mode` with `mode=0` (one-way) or `mode=3` (hedge)

### Unified Trading Account (UTA)
- All Bybit accounts are now UTA (Unified Trading Account)
- Single `UNIFIED` account type for all derivatives + spot margin
- `FUND` account type for funding/wallet operations
- Transfer between accounts via `/v5/asset/transfer/inter-transfer`

### Key Enum Values

**side**: `Buy`, `Sell`

**orderType**: `Market`, `Limit`

**timeInForce**: `GTC`, `IOC`, `FOK`, `PostOnly`

**orderStatus**: `New`, `PartiallyFilled`, `Untriggered`, `Rejected`, `PartiallyFilledCanceled`, `Filled`, `Cancelled`, `Triggered`, `Deactivated`

**triggerBy**: `LastPrice`, `IndexPrice`, `MarkPrice`

**cancelType**: `CancelByUser`, `CancelByReduceOnly`, `CancelByPrepareLiq`, `CancelByPrepareAdl`, `CancelByAdmin`, `CancelBySettle`, `CancelByTpSlTsClear`, `CancelBySmp`

**execType**: `Trade`, `AdlTrade`, `Funding`, `BustTrade`, `Delivery`, `Settle`, `BlockTrade`

**contractType**: `InversePerpetual`, `LinearPerpetual`, `LinearFutures`, `InverseFutures`

**accountType**: `UNIFIED`, `FUND`

### Funding Rate
- Default interval: 8 hours (480 minutes), configurable per symbol via `fundingInterval` field
- Query current rate via tickers endpoint (`fundingRate`, `nextFundingTime`)
- Query history via `/v5/market/funding/history`
- Caps available in `upperFundingRate`/`lowerFundingRate` (instruments-info) and `fundingCap` (tickers)
