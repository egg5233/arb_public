# OKX API Documentation Reference

## Base URLs

### Production
- **REST**: `https://www.okx.com`
- **WebSocket Public**: `wss://ws.okx.com:8443/ws/v5/public`
- **WebSocket Private**: `wss://ws.okx.com:8443/ws/v5/private`
- **WebSocket Business**: `wss://ws.okx.com:8443/ws/v5/business`

### Demo Trading
- **REST**: `https://www.okx.com` (add header `x-simulated-trading: 1`)
- **WebSocket Public**: `wss://wspap.okx.com:8443/ws/v5/public`
- **WebSocket Private**: `wss://wspap.okx.com:8443/ws/v5/private`

### Regional Domains
- US/AU users (registered on app.okx.com): use `us.okx.com`
- EU users (registered on my.okx.com): use `eea.okx.com`

---

## Authentication

### Required Headers (all private REST endpoints)
| Header | Description |
|--------|-------------|
| `OK-ACCESS-KEY` | API key as String |
| `OK-ACCESS-SIGN` | Base64-encoded HMAC SHA256 signature |
| `OK-ACCESS-TIMESTAMP` | ISO format UTC timestamp, e.g. `2020-12-08T09:08:57.715Z` |
| `OK-ACCESS-PASSPHRASE` | Passphrase set when creating API key |
| `Content-Type` | `application/json` |

### Signature Algorithm
```
prehash = timestamp + METHOD + requestPath + body
signature = Base64(HMAC-SHA256(prehash, secretKey))
```

- **timestamp**: Same as `OK-ACCESS-TIMESTAMP` header (ISO millisecond format)
- **METHOD**: Uppercase, e.g. `GET`, `POST`
- **requestPath**: Full path including query string for GET, e.g. `/api/v5/account/balance?ccy=BTC`
- **body**: JSON string for POST requests; omit for GET requests
- **GET request parameters are part of requestPath, NOT body**

### WebSocket Authentication
```json
{
  "op": "login",
  "args": [{
    "apiKey": "your-api-key",
    "passphrase": "your-passphrase",
    "timestamp": "1704876947",
    "sign": "base64-hmac-sha256-signature"
  }]
}
```
- **timestamp**: Unix epoch in **seconds** (not milliseconds like REST)
- **sign**: `Base64(HMAC-SHA256(timestamp + 'GET' + '/users/self/verify', secretKey))`
- Method is always `GET`, path is always `/users/self/verify`
- Login expires 30 seconds after timestamp

---

## Rate Limits

### General Rules
- Public unauthenticated REST: based on **IP address**
- Private REST: based on **User ID** (sub-accounts have individual User IDs)
- WebSocket login/subscribe: 480 requests per hour per connection
- WebSocket connection: 3 requests per second per IP
- Trading-related APIs: rate limits shared across REST and WebSocket

### Key Endpoint Limits
| Endpoint | Rate Limit | Rule |
|----------|-----------|------|
| GET /account/balance | 10 req/2s | User ID |
| GET /account/positions | 10 req/2s | User ID |
| GET /account/config | 5 req/2s | User ID |
| POST /account/set-leverage | 20 req/2s | User ID |
| POST /account/set-position-mode | 5 req/2s | User ID |
| GET /account/trade-fee | 5 req/2s | User ID |
| GET /account/bills | 5 req/s | User ID |
| POST /trade/order | 60 req/2s | User ID + Instrument ID |
| POST /trade/cancel-order | 60 req/2s | User ID + Instrument ID |
| POST /trade/close-position | 20 req/2s | User ID + Instrument ID |
| GET /trade/order | 60 req/2s | User ID + Instrument ID |
| GET /trade/fills | 60 req/2s | User ID |
| GET /public/instruments | 20 req/2s | User ID + instType |
| GET /public/funding-rate | 10 req/2s | IP + Instrument ID |
| GET /public/funding-rate-history | 10 req/2s | IP + Instrument ID |
| GET /public/mark-price | 10 req/2s | IP + Instrument ID |
| GET /public/open-interest | 20 req/2s | IP + Instrument ID |
| GET /public/price-limit | 20 req/2s | IP |
| GET /public/position-tiers | 10 req/2s | IP |
| GET /market/ticker | 20 req/2s | IP |
| GET /market/tickers | 20 req/2s | IP |
| GET /market/books | 40 req/2s | IP |
| GET /market/candles | 40 req/2s | IP |
| POST /asset/transfer | 2 req/s | User ID + Currency |
| GET /asset/balances | 6 req/s | User ID |

### Sub-account Rate Limit
- Max 1000 order requests per 2 seconds (new + amend orders counted)
- Error code `50061` when exceeded

---

## Instrument Naming Convention

### instId Format
| Type | Format | Example |
|------|--------|---------|
| Spot | `{BASE}-{QUOTE}` | `BTC-USDT` |
| Perpetual Swap | `{BASE}-{QUOTE}-SWAP` | `BTC-USDT-SWAP` |
| Futures | `{BASE}-{QUOTE}-{EXPIRY}` | `BTC-USDT-250808` |
| Option | `{BASE}-{QUOTE}-{EXPIRY}-{STRIKE}-{C/P}` | `BTC-USD-220309-33000-C` |

### instType Values
- `SPOT` - Spot trading
- `MARGIN` - Margin trading
- `SWAP` - Perpetual futures
- `FUTURES` - Expiry futures
- `OPTION` - Options
- `ANY` - All types (for WS subscriptions)

### instFamily Examples
| Contract Type | uly | instFamily | settleCcy | SWAP instId |
|--------------|-----|-----------|-----------|-------------|
| USDT-margined | BTC-USDT | BTC-USDT | USDT | BTC-USDT-SWAP |
| USDC-margined | BTC-USDC | BTC-USDC | USDC | BTC-USDC-SWAP |
| Coin-margined | BTC-USD | BTC-USD | BTC | BTC-USD-SWAP |

### Contract Type (ctType)
- `linear` - USDT/USDC margined contracts
- `inverse` - Coin-margined contracts

---

## Response Wrapper

**All OKX API responses** use this standard wrapper:
```json
{
  "code": "0",
  "msg": "",
  "data": [...]
}
```
- `code`: `"0"` means success, any other value is an error
- `msg`: Error message (empty on success)
- `data`: **Always an array**, even for single items
- Some batch endpoints use `sCode`/`sMsg` per-item instead of top-level `code`/`msg`

---

## REST API Endpoints

### Trading Account

#### GET /api/v5/account/balance
Get trading account balance for all currencies with non-zero balance.

**Rate Limit**: 10 req/2s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | No | Currency or currencies (comma-separated, max 20), e.g. `BTC,ETH` |

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "totalEq": "55837.43556134779",
    "adjEq": "55415.624719833286",
    "uTime": "1705474164160",
    "details": [{
      "ccy": "USDT",
      "eq": "4992.890093622894",
      "cashBal": "4850.435693622894",
      "availBal": "4834.317093622894",
      "availEq": "4834.3170936228935",
      "frozenBal": "158.573",
      "ordFrozen": "0",
      "upl": "-7.545600000000006",
      "eqUsd": "4991.542013297616",
      "uTime": "1705449605015"
    }]
  }],
  "msg": ""
}
```

**Key Response Fields**:
| Field | Description |
|-------|-------------|
| totalEq | Total equity in USD |
| adjEq | Adjusted/effective equity in USD |
| details[].ccy | Currency |
| details[].eq | Equity of currency |
| details[].cashBal | Cash balance |
| details[].availBal | Available balance |
| details[].availEq | Available equity |
| details[].frozenBal | Frozen balance |
| details[].upl | Unrealized PnL |
| details[].eqUsd | Equity in USD |

---

#### GET /api/v5/account/positions
Get positions of the trading account. Returns positions with non-zero quantity.

**Rate Limit**: 10 req/2s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | No | MARGIN, SWAP, FUTURES, OPTION |
| instId | String | No | Instrument ID, e.g. BTC-USDT-SWAP |
| posId | String | No | Position ID |

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| instId | Instrument ID |
| instType | Instrument type |
| posSide | Position side: `long`, `short`, `net` |
| pos | Quantity of positions (in contracts for derivatives) |
| availPos | Position that can be closed |
| avgPx | Average open price |
| upl | Unrealized PnL (by mark price) |
| uplRatio | Unrealized PnL ratio |
| lever | Leverage |
| liqPx | Estimated liquidation price |
| markPx | Latest mark price |
| margin | Margin (isolated only) |
| mgnRatio | Maintenance margin ratio |
| mmr | Maintenance margin requirement |
| imr | Initial margin requirement |
| notionalUsd | Notional value in USD |
| adl | ADL indicator (0-5, lower = weaker) |
| last | Latest traded price |
| ccy | Currency used for margin |
| mgnMode | Margin mode: `cross` or `isolated` |
| pnl | Realized PnL |
| fee | Accumulated fee (negative = charged) |
| fundingFee | Accumulated funding fee |

**posSide values**:
- `net` - Net mode (default, single direction)
- `long` - Long position (long/short mode)
- `short` - Short position (long/short mode)

---

#### GET /api/v5/account/config
Get current account configuration.

**Rate Limit**: 5 req/2s | **Permission**: Read

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| acctLv | Account mode: `1`=Spot, `2`=Futures, `3`=Multi-currency margin, `4`=Portfolio margin |
| posMode | Position mode: `long_short_mode` or `net_mode` |
| autoLoan | Auto borrow coins: `true`/`false` |
| uid | Account ID |
| mainUid | Main account ID (same as uid if master account) |
| level | User level, e.g. `Lv1` |

---

#### POST /api/v5/account/set-position-mode
Set position mode for FUTURES/SWAP.

**Rate Limit**: 5 req/2s | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| posMode | String | Yes | `long_short_mode` or `net_mode` |

---

#### POST /api/v5/account/set-leverage
Set leverage.

**Rate Limit**: 20 req/2s | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Conditional | Instrument ID (required for isolated mode, cross SWAP) |
| ccy | String | Conditional | Currency (for cross MARGIN at currency level) |
| lever | String | Yes | Leverage value |
| mgnMode | String | Yes | `cross` or `isolated` |
| posSide | String | Conditional | `long` or `short` (only for isolated + long/short mode) |

**Example (cross SWAP)**:
```json
{
  "instId": "BTC-USDT-SWAP",
  "lever": "5",
  "mgnMode": "cross"
}
```

---

#### GET /api/v5/account/trade-fee
Get fee rates.

**Rate Limit**: 5 req/2s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | SPOT, MARGIN, SWAP, FUTURES, OPTION |
| instId | String | No | Instrument ID (SPOT/MARGIN) |
| instFamily | String | No | Instrument family (FUTURES/SWAP/OPTION) |

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| maker | Maker fee rate (negative = fee charged) |
| taker | Taker fee rate |
| level | Fee rate level |
| feeGroup[].taker | Taker fee per group |
| feeGroup[].maker | Maker fee per group |
| feeGroup[].groupId | Fee group ID |

---

#### GET /api/v5/account/bills
Get bills details (last 7 days).

**Rate Limit**: 5 req/s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | No | SPOT, MARGIN, SWAP, FUTURES, OPTION |
| instId | String | No | Instrument ID |
| ccy | String | No | Bill currency |
| mgnMode | String | No | `isolated` or `cross` |
| ctType | String | No | `linear` or `inverse` |
| type | String | No | Bill type |
| subType | String | No | Bill subtype |
| after/before | String | No | Pagination by bill ID |
| limit | String | No | Max 100, default 100 |

**Key Bill Types (type)**:
- `2` - Trade
- `5` - Liquidation
- `8` - Funding fee
- `9` - Transfer

**Key Bill SubTypes (subType)**:
- `1` - Buy
- `2` - Sell
- `3` - Open long
- `4` - Open short
- `5` - Close long
- `6` - Close short
- `173` - Funding fee expense
- `174` - Funding fee income

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| billId | Bill ID |
| instType | Instrument type |
| instId | Instrument ID |
| type | Bill type |
| subType | Bill subtype |
| sz | Quantity |
| px | Price (trade price, liquidation price, delivery price, or mark price depending on subType) |
| pnl | Profit and loss |
| fee | Fee (negative = charged, positive = rebate) |
| bal | Balance after change |
| balChg | Balance change amount |
| ccy | Currency |
| ordId | Order ID |
| ts | Timestamp |
| fillTime | Last filled time |
| tradeId | Last traded ID |
| execType | `T` = taker, `M` = maker |
| from | Remitting account (`6`=funding, `18`=trading) |
| to | Beneficiary account (`6`=funding, `18`=trading) |

---

### Trade

#### POST /api/v5/trade/order
Place a new order.

**Rate Limit**: 60 req/2s | **Rule**: User ID + Instrument ID | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| tdMode | String | Yes | Trade mode: `cross`, `isolated`, `cash` |
| side | String | Yes | `buy` or `sell` |
| ordType | String | Yes | Order type (see below) |
| sz | String | Yes | Quantity to buy or sell |
| px | String | Conditional | Price (required for limit orders) |
| posSide | String | Conditional | Position side: `long`, `short`, `net` (required in long/short mode) |
| ccy | String | No | Margin currency (for isolated MARGIN in Futures mode) |
| clOrdId | String | No | Client order ID (up to 32 chars) |
| tag | String | No | Order tag (up to 16 chars) |
| reduceOnly | Boolean | No | Reduce-only order (default `false`) |
| tgtCcy | String | No | Target currency for SPOT market orders: `base_ccy` or `quote_ccy` |

**ordType Values**:
| Value | Description |
|-------|-------------|
| `market` | Market order |
| `limit` | Limit order |
| `post_only` | Post-only order |
| `fok` | Fill-or-kill |
| `ioc` | Immediate-or-cancel |
| `optimal_limit_ioc` | Market order with IOC (FUTURES/SWAP only) |

**posSide Rules**:
- **net mode**: Default is `net`, optional
- **long/short mode**: **Required** for FUTURES/SWAP. Use `long` for buy/close-short, `short` for sell/close-long

**tdMode Values**:
| Value | Description |
|-------|-------------|
| `cross` | Cross margin |
| `isolated` | Isolated margin |
| `cash` | Non-margin (Spot) |

**Response**:
```json
{
  "code": "0",
  "data": [{
    "ordId": "312269865356374016",
    "clOrdId": "oktswap6",
    "tag": "",
    "ts": "1695190491421",
    "sCode": "0",
    "sMsg": ""
  }],
  "msg": ""
}
```

---

#### POST /api/v5/trade/cancel-order
Cancel an order.

**Rate Limit**: 60 req/2s | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| ordId | String | Conditional | Order ID (either ordId or clOrdId required) |
| clOrdId | String | Conditional | Client Order ID |

---

#### POST /api/v5/trade/close-position
Close all positions for an instrument.

**Rate Limit**: 20 req/2s | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| mgnMode | String | Yes | `cross` or `isolated` |
| posSide | String | Conditional | Position side (required in long/short mode): `long` or `short` |
| ccy | String | Conditional | Margin currency (for closing cross MARGIN in Futures mode) |
| autoCxl | Boolean | No | Auto-cancel pending close orders (default `false`) |
| clOrdId | String | No | Client order ID |

**Example**:
```json
{
  "instId": "BTC-USDT-SWAP",
  "mgnMode": "cross",
  "posSide": "long"
}
```

---

#### GET /api/v5/trade/order
Get order details.

**Rate Limit**: 60 req/2s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| ordId | String | Conditional | Order ID |
| clOrdId | String | Conditional | Client Order ID |

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| ordId | Order ID |
| clOrdId | Client order ID |
| instId | Instrument ID |
| instType | Instrument type |
| ordType | Order type |
| side | `buy` or `sell` |
| posSide | Position side |
| state | `live`, `partially_filled`, `filled`, `canceled` |
| px | Order price |
| sz | Order size |
| fillPx | Last fill price |
| fillSz | Last fill size |
| accFillSz | Accumulated fill size |
| avgPx | Average fill price |
| fee | Accumulated fee |
| feeCcy | Fee currency |
| pnl | Profit and loss |
| lever | Leverage |
| tdMode | Trade mode |

---

#### GET /api/v5/trade/fills
Get recent fills (last 3 days).

**Rate Limit**: 60 req/2s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | No | SPOT, MARGIN, SWAP, FUTURES, OPTION |
| instId | String | No | Instrument ID |
| ordId | String | No | Order ID |
| after/before | String | No | Pagination by billId |
| limit | String | No | Max 100, default 100 |

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| instId | Instrument ID |
| instType | Instrument type |
| ordId | Order ID |
| tradeId | Trade ID |
| billId | Bill ID |
| fillPx | Fill price |
| fillSz | Fill size |
| fillPnl | Fill PnL (for closing positions) |
| side | `buy` or `sell` |
| posSide | Position side |
| fee | Fee |
| feeCcy | Fee currency |
| feeRate | Fee rate |
| execType | `T` = taker, `M` = maker |
| fillTime | Fill timestamp |
| ts | Data timestamp |
| subType | Transaction type (1=buy, 2=sell, 3=open long, 4=open short, 5=close long, 6=close short) |

---

### Public Data

#### GET /api/v5/public/instruments
Get available instruments info. **Also available at** `GET /api/v5/account/instruments` (authenticated, includes user-specific fields).

**Rate Limit**: 20 req/2s | **Rule**: IP (public) or User ID + instType (account)

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | SPOT, MARGIN, SWAP, FUTURES, OPTION |
| instFamily | String | Conditional | Required for OPTION |
| instId | String | No | Specific instrument ID |

**Response Key Fields**:
| Field | Description |
|-------|-------------|
| instId | Instrument ID, e.g. `BTC-USDT-SWAP` |
| instType | Instrument type |
| instFamily | Instrument family, e.g. `BTC-USDT` |
| uly | Underlying, e.g. `BTC-USDT` |
| baseCcy | Base currency (SPOT/MARGIN only) |
| quoteCcy | Quote currency (SPOT/MARGIN only) |
| settleCcy | Settlement/margin currency (FUTURES/SWAP/OPTION) |
| **ctVal** | **Contract value** (FUTURES/SWAP/OPTION) — **CRITICAL for size conversion** |
| **ctMult** | **Contract multiplier** (FUTURES/SWAP/OPTION) |
| **ctValCcy** | **Contract value currency** (FUTURES/SWAP/OPTION) |
| ctType | Contract type: `linear` or `inverse` |
| tickSz | Tick size (minimum price increment) |
| lotSz | Lot size (minimum order size increment) |
| minSz | Minimum order size |
| maxLmtSz | Max limit order size |
| maxMktSz | Max market order size |
| lever | Max leverage |
| state | `live`, `suspend`, `preopen`, `test` |
| listTime | Listing time |
| expTime | Expiry time |

**CRITICAL: ctVal (Contract Value)**

The `ctVal` field determines how many units of the underlying asset one contract represents.

- For `BTC-USDT-SWAP` (linear): `ctVal` = `"0.01"` means 1 contract = 0.01 BTC
- For `BTC-USD-SWAP` (inverse): `ctVal` = `"100"` means 1 contract = 100 USD
- **To convert quantity to contracts**: `contracts = quantity / ctVal`
- **To convert contracts to quantity**: `quantity = contracts * ctVal`
- **Different contracts have different ctVal values** — always check via this endpoint
- **Bug history**: 181 contracts were affected by incorrect ctVal conversion when ctVal != 1

**Example Response (SWAP)**:
```json
{
  "instId": "BTC-USDT-SWAP",
  "instType": "SWAP",
  "ctVal": "0.01",
  "ctMult": "1",
  "ctValCcy": "BTC",
  "ctType": "linear",
  "settleCcy": "USDT",
  "tickSz": "0.1",
  "lotSz": "1",
  "minSz": "1",
  "lever": "125",
  "state": "live"
}
```

---

#### GET /api/v5/public/funding-rate
Get current funding rate for a SWAP instrument.

**Rate Limit**: 10 req/2s | **Rule**: IP + Instrument ID

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID (SWAP only), e.g. `BTC-USDT-SWAP`. Use `ANY` for all swaps. |

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "instId": "BTC-USDT-SWAP",
    "instType": "SWAP",
    "fundingRate": "0.0000182221218054",
    "fundingTime": "1743609600000",
    "nextFundingTime": "1743638400000",
    "minFundingRate": "-0.00375",
    "maxFundingRate": "0.00375",
    "settState": "settled",
    "settFundingRate": "0.0000145824401745",
    "method": "current_period",
    "formulaType": "noRate",
    "ts": "1743588686291"
  }],
  "msg": ""
}
```

**Key Fields**:
| Field | Description |
|-------|-------------|
| fundingRate | Current funding rate |
| fundingTime | Current settlement time (Unix ms) |
| nextFundingTime | Next settlement time (Unix ms) |
| settState | `processing` or `settled` |
| settFundingRate | Funding rate used for settlement |
| minFundingRate | Lower limit of funding rate |
| maxFundingRate | Upper limit of funding rate |
| method | `current_period` |

**Note**: Funding rate frequency can be adjusted (8h/6h/4h/2h/1h). Check `fundingTime` vs `nextFundingTime` to determine the interval.

---

#### GET /api/v5/public/funding-rate-history
Get funding rate history (up to 3 months).

**Rate Limit**: 10 req/2s | **Rule**: IP + Instrument ID

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID (SWAP only) |
| before/after | String | No | Pagination by fundingTime |
| limit | String | No | Max 400, default 400 |

**Response Fields**:
| Field | Description |
|-------|-------------|
| fundingRate | Predicted funding rate |
| realizedRate | Actual funding rate |
| fundingTime | Settlement time |
| method | `current_period` or `next_period` |

---

#### GET /api/v5/public/mark-price
Get mark price.

**Rate Limit**: 10 req/2s | **Rule**: IP + Instrument ID

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | MARGIN, SWAP, FUTURES, OPTION |
| instFamily | String | No | Instrument family |
| instId | String | No | Instrument ID |

**Response**:
```json
{
  "data": [{
    "instType": "SWAP",
    "instId": "BTC-USDT-SWAP",
    "markPx": "87234.5",
    "ts": "1597026383085"
  }]
}
```

---

#### GET /api/v5/public/price-limit
Get buy/sell price limits.

**Rate Limit**: 20 req/2s | **Rule**: IP

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |

**Response**:
```json
{
  "data": [{
    "instId": "BTC-USDT-SWAP",
    "buyLmt": "17057.9",
    "sellLmt": "16388.9",
    "ts": "1597026383085",
    "enabled": true
  }]
}
```

---

#### GET /api/v5/public/open-interest
Get total open interest.

**Rate Limit**: 20 req/2s | **Rule**: IP + Instrument ID

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | SWAP, FUTURES, OPTION |
| instId | String | No | Instrument ID |

**Response Fields**: `oi` (contracts), `oiCcy` (coins), `oiUsd` (USD)

---

#### GET /api/v5/public/position-tiers
Get position tier information (leverage tiers).

**Rate Limit**: 10 req/2s | **Rule**: IP

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | MARGIN, SWAP, FUTURES, OPTION |
| tdMode | String | Yes | `cross` or `isolated` |
| instFamily | String | Conditional | Required for SWAP/FUTURES/OPTION |
| instId | String | Conditional | For MARGIN |

**Response Fields**: `tier`, `minSz`, `maxSz`, `mmr`, `imr`, `maxLever`

---

### Market Data

#### GET /api/v5/market/tickers
Get all tickers for an instrument type.

**Rate Limit**: 20 req/2s | **Rule**: IP

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | Yes | SPOT, SWAP, FUTURES, OPTION |
| instFamily | String | No | Instrument family |

**Response Example**:
```json
{
  "data": [{
    "instType": "SWAP",
    "instId": "BTC-USDT-SWAP",
    "last": "87234.5",
    "lastSz": "1",
    "askPx": "87235.0",
    "askSz": "11",
    "bidPx": "87234.0",
    "bidSz": "5",
    "open24h": "86000",
    "high24h": "88000",
    "low24h": "85500",
    "volCcy24h": "2222",
    "vol24h": "2222",
    "ts": "1597026383085"
  }]
}
```

#### GET /api/v5/market/ticker
Get single ticker.

**Parameters**: `instId` (required)

---

#### GET /api/v5/market/books
Get order book.

**Rate Limit**: 40 req/2s | **Rule**: IP

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| sz | String | No | Depth per side (max 400, default 1) |

**Response**:
```json
{
  "data": [{
    "asks": [["41006.8", "0.60038921", "0", "1"]],
    "bids": [["41006.3", "0.30178218", "0", "2"]],
    "ts": "1629966436396",
    "seqId": 3235851742
  }]
}
```

**Array format**: `[price, quantity, deprecated("0"), numOrders]`
- Quantity is in contracts for derivatives, base currency for spot

---

#### GET /api/v5/market/candles
Get candlestick data (latest 1440 entries).

**Rate Limit**: 40 req/2s | **Rule**: IP

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| bar | String | No | Bar size: `1m/3m/5m/15m/30m/1H/2H/4H/6H/12H/1D/1W/1M` etc. Default `1m` |
| after/before | String | No | Pagination by timestamp |
| limit | String | No | Max 300, default 100 |

**Response array**: `[ts, open, high, low, close, vol, volCcy, volCcyQuote, confirm]`

---

### Funding Account

#### GET /api/v5/asset/balances
Get funding account balances.

**Rate Limit**: 6 req/s | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | No | Currency (comma-separated, max 20) |

**Response**:
```json
{
  "data": [{
    "ccy": "USDT",
    "bal": "37.11827078",
    "availBal": "37.11827078",
    "frozenBal": "0"
  }]
}
```

---

#### POST /api/v5/asset/transfer
Transfer funds between accounts.

**Rate Limit**: 2 req/s | **Rule**: User ID + Currency | **Permission**: Trade

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | Yes | Currency, e.g. `USDT` |
| amt | String | Yes | Amount to transfer |
| from | String | Yes | `6` = Funding account, `18` = Trading account |
| to | String | Yes | `6` = Funding account, `18` = Trading account |
| type | String | No | `0` = within account (default), `1` = master→sub, `2` = sub→master, `3` = sub→master (sub key), `4` = sub→sub |
| subAcct | String | Conditional | Sub-account name (required for type 1/2/4) |

**Example (funding → trading)**:
```json
{
  "ccy": "USDT",
  "amt": "1.5",
  "from": "6",
  "to": "18"
}
```

**Response**:
```json
{
  "data": [{
    "transId": "754147",
    "ccy": "USDT",
    "from": "6",
    "amt": "0.1",
    "to": "18"
  }]
}
```

---

## WebSocket Streams

### Connection Management
- **Ping/Pong**: Send string `'ping'`, expect `'pong'` response
- **Timeout**: Connection breaks after 30 seconds without data
- **Keepalive**: Send ping within N < 30 seconds of last message

### Subscribe/Unsubscribe Format
```json
{
  "op": "subscribe",
  "args": [{
    "channel": "tickers",
    "instId": "BTC-USDT-SWAP"
  }]
}
```

### Public Channels

#### Tickers Channel
Real-time ticker updates.

**Subscribe**:
```json
{"op": "subscribe", "args": [{"channel": "tickers", "instId": "BTC-USDT-SWAP"}]}
```

**Push Data**:
```json
{
  "arg": {"channel": "tickers", "instId": "BTC-USDT-SWAP"},
  "data": [{
    "instType": "SWAP",
    "instId": "BTC-USDT-SWAP",
    "last": "87234.5",
    "lastSz": "1",
    "askPx": "87235.0",
    "askSz": "11",
    "bidPx": "87234.0",
    "bidSz": "5",
    "open24h": "86000",
    "high24h": "88000",
    "low24h": "85500",
    "vol24h": "2222",
    "volCcy24h": "2222",
    "ts": "1597026383085"
  }]
}
```

#### Order Book Channels
| Channel | Description | Update Frequency |
|---------|-------------|-----------------|
| `books` | Full 400-depth, incremental | 100ms |
| `books5` | Top 5 levels, snapshot | 100ms |
| `books50-l2-tbt` | Top 50, tick-by-tick | Real-time |
| `books-l2-tbt` | Full depth, tick-by-tick | Real-time |
| `bbo-tbt` | Best bid/offer, tick-by-tick | Real-time |

**Subscribe**:
```json
{"op": "subscribe", "args": [{"channel": "books5", "instId": "BTC-USDT-SWAP"}]}
```

#### Mark Price Channel
```json
{"op": "subscribe", "args": [{"channel": "mark-price", "instId": "BTC-USDT-SWAP"}]}
```

#### Funding Rate Channel
```json
{"op": "subscribe", "args": [{"channel": "funding-rate", "instId": "BTC-USDT-SWAP"}]}
```

#### Instruments Channel
Subscribe by instType to get instrument state changes:
```json
{"op": "subscribe", "args": [{"channel": "instruments", "instType": "SWAP"}]}
```

---

### Private Channels (require login)

#### Account Channel
Account balance and position updates.

```json
{"op": "subscribe", "args": [{"channel": "account"}]}
```

**Push data includes**: `totalEq`, `adjEq`, `details[]` with per-currency balances (same fields as REST GET /account/balance).

#### Positions Channel
Position updates.

```json
{"op": "subscribe", "args": [{"channel": "positions", "instType": "SWAP"}]}
```

**Push data includes**: All position fields (same as REST GET /account/positions).

#### Orders Channel
Order status updates.

```json
{"op": "subscribe", "args": [{"channel": "orders", "instType": "ANY"}]}
```

**Push data includes**: `ordId`, `clOrdId`, `instId`, `side`, `posSide`, `ordType`, `sz`, `px`, `state`, `fillPx`, `fillSz`, `avgPx`, `fee`, `pnl`, `lever`, `accFillSz`.

#### Fills Channel (via Business WebSocket)
Trade fills in real-time.

```json
{"op": "subscribe", "args": [{"channel": "fills", "instType": "ANY"}]}
```
**Note**: Fills channel uses the **Business** WebSocket endpoint (`/ws/v5/business`), not Private.

#### Balance and Positions Channel
Combined balance + position updates in a single push.

```json
{"op": "subscribe", "args": [{"channel": "balance_and_position"}]}
```

### WebSocket Order Placement
Orders can also be placed via WebSocket (shares rate limit with REST):

```json
{
  "id": "1512",
  "op": "order",
  "args": [{
    "side": "buy",
    "instId": "BTC-USDT-SWAP",
    "tdMode": "cross",
    "ordType": "limit",
    "sz": "1",
    "px": "50000"
  }]
}
```

---

## Error Codes

### General Class
| Code | HTTP | Message |
|------|------|---------|
| 0 | 200 | Success |
| 1 | 200 | Operation failed |
| 2 | 200 | Bulk operation partially succeeded |
| 50000 | 400 | Body for POST request cannot be empty |
| 50001 | 503 | Service temporarily unavailable |
| 50002 | 400 | JSON syntax error |
| 50004 | 400 | API endpoint request timeout |
| 50011 | 200/429 | Rate limit reached |
| 50013 | 429 | Systems are busy |
| 50014 | 400 | Parameter {param0} can not be empty |
| 50026 | 500 | System error |
| 50061 | 200 | Sub-account rate limit reached |

### API/Auth Class
| Code | HTTP | Message |
|------|------|---------|
| 50100 | 400 | API frozen |
| 50101 | 401 | APIKey does not match current environment |
| 50102 | 401 | Timestamp request expired |
| 50103 | 401 | OK-ACCESS-KEY cannot be empty |
| 50104 | 401 | OK-ACCESS-PASSPHRASE cannot be empty |
| 50105 | 401 | OK-ACCESS-PASSPHRASE incorrect |
| 50106 | 401 | OK-ACCESS-SIGN cannot be empty |
| 50107 | 401 | OK-ACCESS-TIMESTAMP cannot be empty |
| 50110 | 401 | IP not in whitelist |
| 50111 | 401 | Invalid OK-ACCESS-KEY |
| 50112 | 401 | Invalid OK-ACCESS-TIMESTAMP |
| 50113 | 401 | Invalid signature |

### Trade Class
| Code | HTTP | Message |
|------|------|---------|
| 51000 | 400 | Parameter error |
| 51001 | 200 | Instrument ID doesn't exist |
| 51002 | 200 | Instrument ID doesn't match underlying |
| 51003 | 200 | Either client order ID or order ID required |
| 51004 | 200 | Order amount exceeds position limit |
| 51006 | 200 | Order price exceeds limit |
| 51008 | 200 | Insufficient balance |
| 51009 | 200 | Order cancelled |
| 51010 | 200 | Order already cancelled |
| 51011 | 200 | Order already filled |
| 51012 | 200 | Insufficient balance for margin |
| 51020 | 200 | Order not found |
| 51024 | 200 | Reduce-only order not allowed |
| 51101 | 200 | txn. details not found |

---

## Notes for Implementation

### sz (Size) Field Semantics
- **For FUTURES/SWAP**: `sz` is in **number of contracts** (not coins)
- **For SPOT**: `sz` is in **base currency** (for limit orders) or configurable via `tgtCcy`
- To convert: `coinAmount = contracts * ctVal` (for linear contracts)
- **Always fetch ctVal from Get Instruments** before calculating order sizes

### ctVal (Contract Value) — Critical for Size Conversion
- Each instrument has a unique `ctVal` that defines how much of the underlying one contract represents
- **Linear contracts** (e.g., BTC-USDT-SWAP): `ctVal` = "0.01" means 1 contract = 0.01 BTC. To buy 0.1 BTC worth, order `sz` = 10
- **Inverse contracts** (e.g., BTC-USD-SWAP): `ctVal` = "100" means 1 contract = 100 USD
- **Bug note**: A recent bug affected 181 contracts because ctVal was assumed to be 1 when it wasn't

### posSide (Position Side) — Logic for Long/Short
- In **net mode** (`posMode: net_mode`): posSide is always `net` or omitted
- In **long/short mode** (`posMode: long_short_mode`):
  - Opening long: `side: "buy"`, `posSide: "long"`
  - Opening short: `side: "sell"`, `posSide: "short"`
  - Closing long: `side: "sell"`, `posSide: "long"`
  - Closing short: `side: "buy"`, `posSide: "short"`
- **Bug note**: posSide inference was a recent bug — always explicitly set posSide in long/short mode

### Bills Endpoint — Close PnL Field Mapping
- Use `GET /api/v5/account/bills` with `subType` filter for close PnL
- SubType `5` = Close long, `6` = Close short
- The `pnl` field contains the realized PnL
- The `px` field contains the trade price for fills (subType 1-6)
- For funding fees: subType `173` = expense, `174` = income
- **Bug note**: Field mapping between bills and close PnL was a recent bug

### Account Types
- `6` = Funding account (for deposits, withdrawals)
- `18` = Trading account (for trading)
- Transfer between them using `POST /api/v5/asset/transfer`

### Account Modes
| acctLv | Mode | Description |
|--------|------|-------------|
| 1 | Spot mode | Basic spot trading |
| 2 | Futures mode | Isolated margin per contract |
| 3 | Multi-currency margin | Cross margin across currencies |
| 4 | Portfolio margin | Cross margin with portfolio risk |

### Order Limits
- Max 4,000 pending orders per account
- Max 500 pending orders per instrument
- Max 100 TP/SL orders per instrument
- Max 500 trigger orders
- Taker orders can match up to 1000 maker orders before being cancelled

### WebSocket Specifics
- Max 30 connections per channel per sub-account
- 480 subscribe/unsubscribe/login requests per hour per connection
- Connection limit: 3 new connections per second per IP
- Ping keepalive: send `"ping"` string, expect `"pong"` string
- Auto-disconnect after 30 seconds without data

---

## Appendix: Additional Endpoints (Patched)

### Trading Account — Positions History

#### GET /api/v5/account/positions-history
Retrieve the updated position data for the last 3 months. Returns in reverse chronological order using `uTime`.

**Rate Limit**: 10 req/2s | **Rule**: User ID | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | No | Instrument type: `MARGIN`, `SWAP`, `FUTURES`, `OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USD-SWAP` |
| mgnMode | String | No | Margin mode: `cross`, `isolated` |
| type | String | No | Type of latest close position: `1`=Close partially, `2`=Close all, `3`=Liquidation, `4`=Partial liquidation, `5`=ADL (not fully closed), `6`=ADL (fully closed) |
| posId | String | No | Position ID. Expires if >30 days after last full close; new posId assigned. |
| after | String | No | Pagination — records earlier than requested `uTime` (Unix ms) |
| before | String | No | Pagination — records newer than requested `uTime` (Unix ms) |
| limit | String | No | Max 100, default 100. All records with same `uTime` returned together. |

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "cTime": "1654177169995",
    "ccy": "BTC",
    "closeAvgPx": "29786.5999999789081085",
    "closeTotalPos": "1",
    "instId": "BTC-USD-SWAP",
    "instType": "SWAP",
    "lever": "10.0",
    "mgnMode": "cross",
    "openAvgPx": "29783.8999999995535393",
    "openMaxPos": "1",
    "realizedPnl": "0.001",
    "fee": "-0.0001",
    "fundingFee": "0",
    "liqPenalty": "0",
    "pnl": "0.0011",
    "pnlRatio": "0.000906447858888",
    "posId": "452587086133239818",
    "posSide": "long",
    "direction": "long",
    "triggerPx": "",
    "type": "1",
    "uTime": "1654177174419",
    "uly": "BTC-USD",
    "nonSettleAvgPx": "",
    "settledPnl": ""
  }],
  "msg": ""
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| instType | String | Instrument type |
| instId | String | Instrument ID |
| mgnMode | String | Margin mode: `cross`, `isolated` |
| type | String | Latest close type: `1`=Partial close, `2`=Close all, `3`=Liquidation, `4`=Partial liquidation, `5`=ADL |
| cTime | String | Created time of position (Unix ms) |
| uTime | String | Updated time of position (Unix ms) |
| openAvgPx | String | Average open price. Cross-margin expiry futures updates at settlement. |
| nonSettleAvgPx | String | Non-settlement entry price (only reflects open/increase avg price). Only for cross FUTURES. |
| closeAvgPx | String | Average closing price |
| posId | String | Position ID |
| openMaxPos | String | Max quantity of position |
| closeTotalPos | String | Position's cumulative closed volume |
| realizedPnl | String | Realized PnL. Formula: `realizedPnl = pnl + fee + fundingFee + liqPenalty + settledPnl` |
| settledPnl | String | Accumulated settled PnL (by settlement price). Only for cross FUTURES. |
| pnl | String | Profit and loss (excluding fee) |
| pnlRatio | String | Realized P&L ratio |
| fee | String | Accumulated fee (negative = charged, positive = rebate) |
| fundingFee | String | Accumulated funding fee |
| liqPenalty | String | Accumulated liquidation penalty (negative when present) |
| posSide | String | Position side: `long`, `short`, `net` |
| lever | String | Leverage |
| direction | String | Direction: `long`, `short`. Only for MARGIN/FUTURES/SWAP/OPTION. |
| triggerPx | String | Trigger mark price. Has value when type is 3/4/5; empty for type 1/2. |
| uly | String | Underlying |
| ccy | String | Currency used for margin |

---

### Trade — Historical Fills (Last 3 Months)

#### GET /api/v5/trade/fills-history
Retrieve transaction details from the last 3 months.

**Rate Limit**: 10 req/2s | **Rule**: User ID | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | **Yes** | Instrument type: `SPOT`, `MARGIN`, `SWAP`, `FUTURES`, `OPTION` |
| instFamily | String | No | Instrument family. Applicable to FUTURES/SWAP/OPTION. |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| ordId | String | No | Order ID |
| subType | String | No | Transaction subtype (see below) |
| after | String | No | Pagination — records earlier than requested `billId` |
| before | String | No | Pagination — records newer than requested `billId` |
| begin | String | No | Filter begin timestamp (Unix ms) |
| end | String | No | Filter end timestamp (Unix ms) |
| limit | String | No | Max 100, default 100 |

**Key subType Values**:
| Value | Description |
|-------|-------------|
| 1 | Buy |
| 2 | Sell |
| 3 | Open long |
| 4 | Open short |
| 5 | Close long |
| 6 | Close short |
| 100-107 | Liquidation variants |
| 112-113 | Delivery long/short |
| 125-128 | ADL variants |

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "side": "buy",
    "fillSz": "0.00192834",
    "fillPx": "51858",
    "fillPxVol": "",
    "fillFwdPx": "",
    "fee": "-0.00000192834",
    "fillPnl": "0",
    "ordId": "680800019749904384",
    "feeRate": "-0.001",
    "instType": "SPOT",
    "fillPxUsd": "",
    "instId": "BTC-USDT",
    "clOrdId": "",
    "posSide": "net",
    "billId": "680800019754098688",
    "subType": "1",
    "fillMarkVol": "",
    "tag": "",
    "fillTime": "1708587373361",
    "execType": "T",
    "fillIdxPx": "",
    "tradeId": "744876980",
    "fillMarkPx": "",
    "feeCcy": "BTC",
    "ts": "1708587373362",
    "tradeQuoteCcy": "USDT"
  }],
  "msg": ""
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tradeId | String | Last trade ID |
| ordId | String | Order ID (always `""` for block trading) |
| clOrdId | String | Client Order ID (always `""` for block trading) |
| billId | String | Bill ID |
| subType | String | Transaction type |
| tag | String | Order tag |
| fillPx | String | Last filled price |
| fillSz | String | Last filled quantity |
| fillIdxPx | String | Index price at trade execution |
| fillPnl | String | Last filled PnL (for closing position orders; `0` otherwise) |
| fillPxVol | String | Implied volatility when filled (options only) |
| fillPxUsd | String | Options price in USD when filled (options only) |
| fillMarkVol | String | Mark volatility when filled (options only) |
| fillFwdPx | String | Forward price when filled (options only) |
| fillMarkPx | String | Mark price when filled (FUTURES/SWAP/OPTION) |
| side | String | Order side: `buy`, `sell` |
| posSide | String | Position side: `long`, `short`, `net` |
| execType | String | Liquidity: `T`=taker, `M`=maker |
| feeCcy | String | Fee/rebate currency |
| fee | String | Fee amount (negative = charged, positive = rebate) |
| feeRate | String | Fee rate (SPOT/MARGIN only) |
| ts | String | Data generation time (Unix ms) |
| fillTime | String | Trade time (Unix ms) |
| tradeQuoteCcy | String | Quote currency for trading |

**Note**: For recent 3-day data, prefer `GET /api/v5/trade/fills` (higher rate limit: 60 req/2s).

---

### Trade — Pending Orders

#### GET /api/v5/trade/orders-pending
Retrieve all incomplete orders under the current account.

**Rate Limit**: 60 req/2s | **Rule**: User ID | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instType | String | No | Instrument type: `SPOT`, `MARGIN`, `SWAP`, `FUTURES`, `OPTION` |
| instFamily | String | No | Instrument family (FUTURES/SWAP/OPTION) |
| instId | String | No | Instrument ID, e.g. `BTC-USD-200927` |
| ordType | String | No | Order type (comma-separated): `market`, `limit`, `post_only`, `fok`, `ioc`, `optimal_limit_ioc`, `mmp`, `mmp_and_post_only`, `op_fok` |
| state | String | No | State: `live`, `partially_filled` |
| after | String | No | Pagination — records earlier than requested `ordId` |
| before | String | No | Pagination — records newer than requested `ordId` |
| limit | String | No | Max 100, default 100 |

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "accFillSz": "0",
    "avgPx": "",
    "cTime": "1724733617998",
    "category": "normal",
    "clOrdId": "",
    "fee": "0",
    "feeCcy": "BTC",
    "fillPx": "",
    "fillSz": "0",
    "fillTime": "",
    "instId": "BTC-USDT",
    "instType": "SPOT",
    "lever": "",
    "ordId": "1752588852617379840",
    "ordType": "post_only",
    "pnl": "0",
    "posSide": "net",
    "px": "13013.5",
    "reduceOnly": "false",
    "side": "buy",
    "slOrdPx": "",
    "slTriggerPx": "",
    "slTriggerPxType": "",
    "source": "",
    "state": "live",
    "stpMode": "cancel_maker",
    "sz": "0.001",
    "tag": "",
    "tdMode": "cash",
    "tpOrdPx": "",
    "tpTriggerPx": "",
    "tpTriggerPxType": "",
    "tradeId": "",
    "uTime": "1724733617998"
  }],
  "msg": ""
}
```

**Response Key Fields**:
| Field | Type | Description |
|-------|------|-------------|
| instType | String | Instrument type |
| instId | String | Instrument ID |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID |
| tag | String | Order tag |
| px | String | Price |
| pxUsd | String | Options price in USD (options only) |
| pxVol | String | Implied volatility (options only) |
| sz | String | Quantity to buy or sell |
| pnl | String | PnL (for closing position orders; `0` otherwise) |
| ordType | String | Order type |
| side | String | Order side: `buy`, `sell` |
| posSide | String | Position side |
| tdMode | String | Trade mode: `cross`, `isolated`, `cash` |
| accFillSz | String | Accumulated fill quantity |
| fillPx | String | Last filled price |
| tradeId | String | Last trade ID |
| fillSz | String | Last filled quantity |
| fillTime | String | Last filled time |
| avgPx | String | Average filled price (`""` if none) |
| state | String | State: `live`, `partially_filled` |
| lever | String | Leverage (MARGIN/FUTURES/SWAP) |
| tpTriggerPx | String | Take-profit trigger price |
| tpTriggerPxType | String | TP trigger price type: `last`, `index`, `mark` |
| tpOrdPx | String | Take-profit order price |
| slTriggerPx | String | Stop-loss trigger price |
| slTriggerPxType | String | SL trigger price type: `last`, `index`, `mark` |
| slOrdPx | String | Stop-loss order price |
| feeCcy | String | Fee currency |
| fee | String | Fee amount |
| rebateCcy | String | Rebate currency |
| rebate | String | Rebate amount |
| source | String | Order source (`6`=trigger, `7`=TP/SL, `13`=algo, `25`=trailing) |
| category | String | Category: `normal` |
| reduceOnly | String | Reduce-only: `true` or `false` |
| uTime | String | Update time (Unix ms) |
| cTime | String | Creation time (Unix ms) |

---

### Trade — Query Single Order

#### GET /api/v5/trade/order
Get order details. *(Already documented in main section — included here for reference completeness.)*

**Rate Limit**: 60 req/2s | **Rule**: User ID + Instrument ID | **Permission**: Read

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID |
| ordId | String | Conditional | Order ID (either `ordId` or `clOrdId` required) |
| clOrdId | String | Conditional | Client Order ID |

Response fields are the same as GET /api/v5/trade/orders-pending plus additional filled-state fields: `cancelSource`, `cancelSourceReason`.

---

### Trade — Place Algo Order (Stop-Loss / Take-Profit)

#### POST /api/v5/trade/order-algo
Place algo orders including: trigger, conditional (TP/SL), OCO, chase, trailing stop, and TWAP orders.

**Rate Limit**: 20 req/2s | **Rule**: User ID + Instrument ID (Options: User ID + Instrument Family) | **Permission**: Trade

**Common Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| tdMode | String | Yes | Trade mode: `cross`, `isolated`, `cash` |
| side | String | Yes | Order side: `buy`, `sell` |
| ordType | String | Yes | Order type: `conditional`, `oco`, `trigger`, `move_order_stop`, `twap`, `chase` |
| posSide | String | Conditional | Position side (required in long/short mode): `long`, `short` |
| sz | String | Conditional | Quantity. Either `sz` or `closeFraction` required. |
| tag | String | No | Order tag (up to 16 chars) |
| algoClOrdId | String | No | Client-supplied Algo ID (up to 32 chars) |
| tgtCcy | String | No | Target currency for SPOT market buy: `base_ccy`, `quote_ccy` |
| closeFraction | String | Conditional | Fraction of position to close (only `1` supported = full close). Only for FUTURES/SWAP with `conditional`/`oco`. |
| reduceOnly | Boolean | No | Reduce-only order (default `false`) |

**TP/SL Parameters** (for `ordType` = `conditional` or `oco`):
| Name | Type | Required | Description |
|------|------|----------|-------------|
| tpTriggerPx | String | No | Take-profit trigger price |
| tpTriggerPxType | String | No | TP trigger type: `last` (default), `index`, `mark` |
| tpOrdPx | String | No | TP order price (`-1` = market price) |
| tpOrdKind | String | No | TP order kind: `condition` (default), `limit` |
| slTriggerPx | String | No | Stop-loss trigger price |
| slTriggerPxType | String | No | SL trigger type: `last` (default), `index`, `mark` |
| slOrdPx | String | No | SL order price (`-1` = market price) |
| cxlOnClosePos | Boolean | No | Cancel TP/SL when position fully closed (default `false`). Requires `reduceOnly=true`. |

**Request Example (Conditional TP/SL)**:
```json
{
  "instId": "BTC-USDT-SWAP",
  "tdMode": "cross",
  "side": "buy",
  "ordType": "conditional",
  "sz": "2",
  "posSide": "short",
  "slTriggerPx": "99000",
  "slOrdPx": "-1",
  "slTriggerPxType": "mark"
}
```

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "algoClOrdId": "order1234",
    "algoId": "1836487817828872192",
    "clOrdId": "",
    "sCode": "0",
    "sMsg": "",
    "tag": ""
  }],
  "msg": ""
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | Event result code (`0` = success) |
| sMsg | String | Rejection message (empty on success) |
| clOrdId | String | Client Order ID *(deprecated)* |
| tag | String | Order tag |

**Note**: When placing net TP/SL (`ordType=conditional`) with both TP and SL params, only SL logic executes; TP is ignored.

---

### Trade — Cancel Algo Orders

#### POST /api/v5/trade/cancel-algos
Cancel unfilled algo orders. Max 10 orders per request. Request body is an **array** of objects.

**Rate Limit**: 20 req/2s | **Rule**: User ID + Instrument ID (Options: User ID + Instrument Family) | **Permission**: Trade

**Request Body** (Array of objects):
| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| algoId | String | Conditional | Algo ID. Either `algoId` or `algoClOrdId` required. If both, `algoId` used. |
| algoClOrdId | String | Conditional | Client-supplied Algo ID |

**Request Example**:
```json
[
  {
    "algoId": "590919993110396111",
    "instId": "BTC-USDT"
  },
  {
    "algoId": "590920138287841222",
    "instId": "BTC-USDT"
  }
]
```

**Response Example**:
```json
{
  "code": "0",
  "data": [{
    "algoClOrdId": "",
    "algoId": "1836489397437468672",
    "clOrdId": "",
    "sCode": "0",
    "sMsg": "",
    "tag": ""
  }],
  "msg": ""
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID *(deprecated)* |
| sCode | String | Event result code (`0` = success) |
| sMsg | String | Rejection message (empty on success) |
| clOrdId | String | Client Order ID *(deprecated)* |
| tag | String | Order tag *(deprecated)* |

---

### Funding Account — Withdrawal

#### POST /api/v5/asset/withdrawal
Withdraw assets from funding account. Only supports verified addresses (set via WEB/APP). Common sub-accounts do not support withdrawal.

**Rate Limit**: 6 req/s | **Rule**: User ID | **Permission**: Withdraw

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | Yes | Currency, e.g. `USDT` |
| amt | String | Yes | Withdrawal amount (fee not included; reserve sufficient tx fees) |
| dest | String | Yes | Withdrawal method: `3`=internal transfer, `4`=on-chain withdrawal |
| toAddr | String | Yes | Destination address. For `dest=4`: wallet address (some formatted as `address:tag`). For `dest=3`: UID, email, phone, or login account name. |
| toAddrType | String | No | Address type: `1`=wallet/email/phone/name, `2`=UID (only for `dest=3`) |
| chain | String | Conditional | Chain name, e.g. `USDT-TRC20`, `USDT-ERC20`. Defaults to main chain. Required for on-chain withdrawal if multiple chains available. |
| areaCode | String | Conditional | Phone area code (e.g. `86`). Required if `toAddr` is a phone number. |
| clientId | String | No | Client-supplied ID (up to 32 chars) |
| rcvrInfo | Object | Conditional | Recipient info (required for specific entity users on-chain withdrawal) |

**rcvrInfo Object**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| walletType | String | Yes | `exchange` or `private` |
| exchId | String | Conditional | Exchange ID (for `walletType=exchange`; use `0` if not in list) |
| rcvrFirstName | String | Conditional | Receiver's first name |
| rcvrLastName | String | Conditional | Receiver's last name |
| rcvrCountry | String | Conditional | Recipient country (English name or ISO 3166-1 code) |
| rcvrCountrySubDivision | String | Conditional | State/province |
| rcvrTownName | String | Conditional | City |
| rcvrStreetName | String | Conditional | Street address |

**Request Example (on-chain)**:
```json
{
  "amt": "100",
  "dest": "4",
  "ccy": "USDT",
  "chain": "USDT-TRC20",
  "toAddr": "TXtvfb7cdrn6VX9H49mgio8bUxZ3DGfvYF"
}
```

**Request Example (internal transfer)**:
```json
{
  "amt": "10",
  "dest": "3",
  "ccy": "USDT",
  "toAddr": "user@example.com"
}
```

**Response Example**:
```json
{
  "code": "0",
  "msg": "",
  "data": [{
    "amt": "0.1",
    "wdId": "67485",
    "ccy": "BTC",
    "clientId": "",
    "chain": "BTC-Bitcoin"
  }]
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ccy | String | Currency |
| chain | String | Chain name, e.g. `USDT-ERC20` |
| amt | String | Withdrawal amount |
| wdId | String | Withdrawal ID |
| clientId | String | Client-supplied ID |

---

### Trading Account — Bills Details (Completeness Patch)

#### GET /api/v5/account/bills
*(Already documented in main section. This patch adds missing parameters and response fields from the official docs.)*

**Additional Request Parameters** (not in main section):
| Name | Type | Required | Description |
|------|------|----------|-------------|
| begin | String | No | Filter with begin timestamp (Unix ms), e.g. `1597026383085` |
| end | String | No | Filter with end timestamp (Unix ms), e.g. `1597026383085` |

**Additional Response Fields** (not in main section):
| Field | Type | Description |
|-------|------|-------------|
| posBalChg | String | Change in balance amount at the position level |
| posBal | String | Balance at the position level |
| interest | String | Interest |
| notes | String | Notes |
| earnAmt | String | Auto earn amount (only when type is `381`) |
| earnApr | String | Auto earn APR (only when type is `381`) |
| fillIdxPx | String | Index price at trade execution |
| fillMarkPx | String | Mark price when filled (FUTURES/SWAP/OPTIONS) |
| fillPxVol | String | Implied volatility when filled (options only) |
| fillPxUsd | String | Options price in USD when filled (options only) |
| fillMarkVol | String | Mark volatility when filled (options only) |
| fillFwdPx | String | Forward price when filled (options only) |

**Bills Archive Endpoint** — For data beyond 7 days (up to 3 months), use:

#### GET /api/v5/account/bills-archive
Same parameters and response fields as `GET /api/v5/account/bills`, but covers the last 3 months.

**Rate Limit**: 5 req/2s | **Rule**: User ID | **Permission**: Read

**Note on `px` field semantics by subType**:
- SubTypes 1-6 (trade fills): `px` = trade filled price
- SubTypes 100-107 (liquidation): `px` = liquidation price
- SubTypes 112-113 (delivery): `px` = delivery price
- SubTypes 170-172 (exercise): `px` = exercise price
- SubTypes 173-174 (funding fee): `px` = mark price

**Funding Fee in Bills**:
- SubType `173` = Funding fee expense → check `pnl` field for fee payment
- SubType `174` = Funding fee income
