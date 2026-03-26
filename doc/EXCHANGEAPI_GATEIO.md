# Gate.io API Documentation Reference

> Source: https://www.gate.com/docs/developers/apiv4/en/ (REST API v4.106.47)
> WebSocket: https://www.gate.com/docs/developers/futures/ws/en/

## Base URLs

### REST API
- **Live trading**: `https://api.gateio.ws/api/v4`
- **TestNet**: `https://api-testnet.gateapi.io/api/v4`
- **Futures alternative** (futures only): `https://fx-api.gateio.ws/api/v4`

### WebSocket (Futures)
- **USDT Contract (Live)**: `wss://fx-ws.gateio.ws/v4/ws/usdt`
- **USDT Contract (TestNet)**: `wss://ws-testnet.gate.com/v4/ws/futures/usdt`
- **BTC Contract (Live)**: `wss://fx-ws.gateio.ws/v4/ws/btc`

### WebSocket (Spot)
- **Live**: `wss://api.gateio.ws/ws/v4/`
- **TestNet**: `wss://ws-testnet.gate.com/v4/ws/spot`

---

## Authentication

### Generate API Key
- Create keys on the web console at [APIv4 keys] page
- Each account supports up to 20 API keys
- IP whitelist supported (max 20 IPs per key)

### Permissions
| Product | Read-only | Read-write |
|---------|-----------|------------|
| Spot/Margin | Query orders | Query & place orders |
| Perpetual Contract | Query orders | Query & place orders |
| Delivery Contract | Query orders | Query & place orders |
| Wallet | Query records | Query & transfers |
| Withdrawal | Query records | Query & withdraw |

### Required Headers
| Header | Description |
|--------|-------------|
| `KEY` | API key string |
| `SIGN` | HMAC-SHA512 signature (hex encoded) |
| `Timestamp` | Current Unix timestamp in seconds (max 60s gap from server) |

### Signature Generation
```
Signature String = Request Method + "\n" + Request URL + "\n" + Query String + "\n" + HexEncode(SHA512(Request Body)) + "\n" + Timestamp
```

- **Request Method**: UPPERCASE (GET, POST, DELETE, PUT)
- **Request URL**: Path only, e.g. `/api/v4/futures/orders`
- **Query String**: As-is from URL, empty string if none
- **Request Body Hash**: SHA512 hex of body, or SHA512 hex of empty string if no body (`cf83e1357eefb8bd...`)
- **SIGN**: `HexEncode(HMAC_SHA512(api_secret, signature_string))`

### Python Example
```python
import time, hashlib, hmac

def gen_sign(method, url, query_string=None, payload_string=None):
    key = 'your_api_key'
    secret = 'your_api_secret'
    t = time.time()
    m = hashlib.sha512()
    m.update((payload_string or "").encode('utf-8'))
    hashed_payload = m.hexdigest()
    s = '%s\n%s\n%s\n%s\n%s' % (method, url, query_string or "", hashed_payload, t)
    sign = hmac.new(secret.encode('utf-8'), s.encode('utf-8'), hashlib.sha512).hexdigest()
    return {'KEY': key, 'Timestamp': str(t), 'SIGN': sign}
```

---

## Rate Limits

### REST API
| Market | Endpoint Type | Limit | Based On |
|--------|--------------|-------|----------|
| All public | Public endpoints | 200r/10s per endpoint | IP |
| Wallet | POST /withdrawals | 1r/3s | UID |
| Wallet | POST /wallet/transfers | 80r/10s | UID |
| Wallet | Other private | 200r/10s | UID |
| Spot | Order placement + amend | 10r/s total (UID+Market) | UID |
| Spot | Order cancellation | 200r/s total | UID |
| **Perpetual Futures** | **Order placement + amend** | **100r/s total** | **UID** |
| **Perpetual Futures** | **Order cancellation** | **200r/s total** | **UID** |
| Perpetual Futures | Other private | 200r/10s per endpoint | UID |
| Subaccount | Private endpoints | 80r/10s | UID |

### WebSocket
- Spot: 10r/s (order/amend combined)
- Futures: 100r/s (order/amend/cancel combined)
- Max connections per IP: 300

### Rate Limit Headers
| Header | Description |
|--------|-------------|
| `X-Gate-RateLimit-Requests-Remain` | Remaining requests for current endpoint |
| `X-Gate-RateLimit-Limit` | Current limit |
| `X-Gate-RateLimit-Reset-Timestamp` | Reset timestamp |

---

## Symbol Naming Convention

- **Futures**: `BTC_USDT` (underscore-separated, e.g. `ETH_USDT`, `SOL_USDT`)
- **Settle parameter**: `usdt` or `btc` (in URL path, always lowercase)
- **URL pattern**: `/api/v4/futures/{settle}/...` where `{settle}` = `usdt`

---

## REST API Endpoints

### Futures

#### List All Contracts
- **Endpoint**: `GET /api/v4/futures/{settle}/contracts`
- **Auth**: No
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| settle | path | string | yes | `btc` or `usdt` |
| limit | query | integer | no | Max records returned |
| offset | query | integer | no | List offset, starting from 0 |

- **Response** (200): Array of Contract objects
```json
[
  {
    "name": "BTC_USDT",
    "type": "direct",
    "quanto_multiplier": "0.0001",
    "maintenance_rate": "0.005",
    "mark_type": "index",
    "last_price": "38026",
    "mark_price": "37985.6",
    "index_price": "37954.92",
    "funding_rate_indicative": "0.000219",
    "funding_offset": 0,
    "in_delisting": false,
    "risk_limit_base": "1000000",
    "interest_rate": "0.0003",
    "order_price_round": "0.1",
    "order_size_min": "1",
    "order_size_max": "1000000",
    "leverage_min": "1",
    "leverage_max": "100",
    "maker_fee_rate": "-0.00025",
    "taker_fee_rate": "0.00075",
    "funding_rate": "0.002053",
    "funding_interval": 28800,
    "funding_next_apply": 1610035200,
    "funding_impact_value": "60000",
    "position_size": "5223816",
    "trade_size": "28530850594",
    "orders_limit": 50,
    "enable_bonus": true,
    "enable_credit": true,
    "status": "trading",
    "market_order_slip_ratio": "0.05",
    "funding_rate_limit": "0.003"
  }
]
```

**Key Contract Fields**:
- `name`: Contract identifier (e.g. `BTC_USDT`)
- `quanto_multiplier`: Contract multiplier - number of base currency per contract (e.g. `0.0001` = 1 contract = 0.0001 BTC)
- `funding_rate`: Current funding rate
- `funding_interval`: Funding interval in seconds (28800 = 8 hours)
- `funding_next_apply`: Next funding application time (Unix timestamp)
- `order_size_min`/`order_size_max`: Min/max order size in contracts (string type)
- `order_price_round`: Price tick size
- `maker_fee_rate`/`taker_fee_rate`: Fee rates (negative = rebate)
- `mark_price`/`index_price`/`last_price`: Current prices
- `enable_decimal`: Whether contract supports decimal size

#### Get Single Contract
- **Endpoint**: `GET /api/v4/futures/{settle}/contracts/{contract}`
- **Auth**: No
- **Parameters**: settle (path), contract (path, e.g. `BTC_USDT`)
- **Response**: Single Contract object (same fields as above)

#### Order Book
- **Endpoint**: `GET /api/v4/futures/{settle}/order_book`
- **Auth**: No
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| settle | path | string | yes | `usdt` |
| contract | query | string | yes | e.g. `BTC_USDT` |
| interval | query | string | no | Price precision merge. `0` = no merge (default) |
| limit | query | integer | no | Depth levels |
| with_id | query | boolean | no | Return depth update ID |

- **Response** (200):
```json
{
  "id": 123456,
  "current": 1623898993.123,
  "update": 1623898993.121,
  "asks": [
    { "p": "1.52", "s": "100" },
    { "p": "1.53", "s": "40" }
  ],
  "bids": [
    { "p": "1.17", "s": "150" },
    { "p": "1.16", "s": "203" }
  ]
}
```
- `asks`/`bids`: Arrays of `{p: price, s: size}` (both strings)
- Bids sorted high→low, asks sorted low→high

#### Market Trades
- **Endpoint**: `GET /api/v4/futures/{settle}/trades`
- **Auth**: No
- **Parameters**: settle, contract (required), limit, offset, last_id, from, to

- **Response** (200):
```json
[
  {
    "id": 121234231,
    "create_time": 1514764800,
    "contract": "BTC_USDT",
    "size": "-100",
    "price": "100.123"
  }
]
```
- `size`: **Positive = buy, negative = sell**

#### Candlesticks
- **Endpoint**: `GET /api/v4/futures/{settle}/candlesticks`
- **Auth**: No
- **Parameters**: settle, contract (required), from, to, limit, interval, timezone
- **Intervals**: `10s`, `1m`, `5m`, `15m`, `30m`, `1h`, `4h`, `8h`, `1d`, `7d`
- **Note**: Prefix contract with `mark_` for mark price candles, `index_` for index price candles

- **Response** (200):
```json
[
  {
    "t": 1539852480,
    "v": "97151",
    "c": "1.032",
    "h": "1.032",
    "l": "1.032",
    "o": "1.032",
    "sum": "3580"
  }
]
```
- `t`: timestamp, `o/h/l/c`: OHLC, `v`: volume (contracts), `sum`: volume (quote currency)

#### Tickers
- **Endpoint**: `GET /api/v4/futures/{settle}/tickers`
- **Auth**: No
- **Parameters**: settle, contract (optional)

- **Response** (200):
```json
[
  {
    "contract": "BTC_USDT",
    "last": "6432",
    "low_24h": "6278",
    "high_24h": "6790",
    "change_percentage": "4.43",
    "total_size": "32323904",
    "volume_24h": "184040233284",
    "volume_24h_base": "28613220",
    "volume_24h_quote": "184040233284",
    "volume_24h_settle": "28613220",
    "mark_price": "6534",
    "funding_rate": "0.0001",
    "funding_rate_indicative": "0.0001",
    "index_price": "6531",
    "highest_bid": "34089.7",
    "highest_size": "100",
    "lowest_ask": "34217.9",
    "lowest_size": "1000"
  }
]
```

#### Funding Rate History
- **Endpoint**: `GET /api/v4/futures/{settle}/funding_rate`
- **Auth**: No
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| settle | path | string | yes | `usdt` |
| contract | query | string | yes | e.g. `BTC_USDT` |
| limit | query | integer | no | Max records |
| from | query | integer | no | Start timestamp (Unix seconds) |
| to | query | integer | no | End timestamp (Unix seconds) |

- **Response** (200):
```json
[
  {
    "t": 1543968000,
    "r": "0.000157"
  }
]
```
- `t`: Unix timestamp, `r`: funding rate

#### Batch Funding Rates
- **Endpoint**: `POST /api/v4/futures/{settle}/funding_rates`
- **Auth**: No
- **Body**: `{"contracts": ["BTC_USDT", "ETH_USDT"]}`
- **Response**: Array of arrays with contract and data

#### Contract Statistics
- **Endpoint**: `GET /api/v4/futures/{settle}/contract_stats`
- **Auth**: No
- **Parameters**: settle, contract (required), from, interval, limit
- **Response** includes: `lsr_taker`, `lsr_account`, `open_interest`, `long_liq_size`, `short_liq_size`, `mark_price`

---

### Futures Trading

#### Place Order
- **Endpoint**: `POST /api/v4/futures/{settle}/orders`
- **Auth**: Yes
- **Headers**: Optional `x-gate-exptime` (expiration time in ms)
- **Body Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| contract | string | yes | e.g. `BTC_USDT` |
| size | string | yes | **Positive = buy/long, negative = sell/short. Set 0 for close position** |
| price | string | yes | Order price. `"0"` with `tif: "ioc"` = market order |
| tif | string | no | `gtc`, `ioc`, `poc` (post-only), `fok` |
| iceberg | string | no | Display size for iceberg. `"0"` = non-iceberg |
| close | boolean | no | Set true to close position (with size=0) |
| reduce_only | boolean | no | Reduce-only order |
| auto_size | string | no | For dual-mode close: `close_long` or `close_short` |
| text | string | no | Custom ID, prefixed with `t-`, max 28 chars |
| stp_act | string | no | Self-trade prevention: `cn`, `co`, `cb` |
| pos_margin_mode | string | no | `isolated` or `cross` |

- **Response** (201):
```json
{
  "id": 15675394,
  "user": 100000,
  "contract": "BTC_USDT",
  "create_time": 1546569968,
  "size": "6024",
  "iceberg": "0",
  "left": "6024",
  "price": "3765",
  "fill_price": "0",
  "mkfr": "-0.00025",
  "tkfr": "0.00075",
  "tif": "gtc",
  "refu": 0,
  "is_reduce_only": false,
  "is_close": false,
  "is_liq": false,
  "text": "t-my-custom-id",
  "status": "finished",
  "finish_time": 1514764900,
  "finish_as": "cancelled",
  "stp_id": 0,
  "stp_act": "-",
  "amend_text": "-",
  "pos_margin_mode": "isolated"
}
```

**Order Status Values**: `open`, `finished`
**finish_as Values**: `filled`, `cancelled`, `liquidated`, `ioc`, `auto_deleveraged`, `reduce_only`, `position_closed`, `stp`, `unknown`

#### List Orders
- **Endpoint**: `GET /api/v4/futures/{settle}/orders`
- **Auth**: Yes
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| contract | query | string | no | Filter by contract |
| status | query | string | yes | `open` or `finished` |
| limit | query | integer | no | Max records |
| offset | query | integer | no | Starting offset |
| last_id | query | string | no | Last record ID for pagination |
| settle | path | string | yes | `usdt` |

- **Response**: Array of FuturesOrder objects
- **Note**: Zero-fill orders cannot be retrieved 10 min after cancellation. Historical orders default to past 6 months.

#### Cancel All Orders
- **Endpoint**: `DELETE /api/v4/futures/{settle}/orders`
- **Auth**: Yes
- **Parameters**: contract (optional), side (`bid`/`ask`), settle
- **Response**: Array of cancelled FuturesOrder objects

#### Get Single Order
- **Endpoint**: `GET /api/v4/futures/{settle}/orders/{order_id}`
- **Auth**: Yes

#### Cancel Single Order
- **Endpoint**: `DELETE /api/v4/futures/{settle}/orders/{order_id}`
- **Auth**: Yes

#### Amend Order
- **Endpoint**: `PUT /api/v4/futures/{settle}/orders/{order_id}`
- **Auth**: Yes

#### Batch Place Orders
- **Endpoint**: `POST /api/v4/futures/{settle}/batch_orders`
- **Auth**: Yes
- **Body**: Array of FuturesOrder objects

#### My Trades
- **Endpoint**: `GET /api/v4/futures/{settle}/my_trades`
- **Auth**: Yes
- **Parameters**: settle, contract, limit, offset, last_id

---

### Futures Positions

#### List Positions
- **Endpoint**: `GET /api/v4/futures/{settle}/positions`
- **Auth**: Yes
- **Parameters**: settle, holding (optional, filter for holding only), position_side

- **Response** (200):
```json
[
  {
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "cross_leverage_limit": "0",
    "pos_margin_mode": "isolated"
  }
]
```

**Key Position Fields**:
- `size`: **Positive = long, negative = short** (string type)
- `leverage`: Isolated margin leverage. `"0"` = cross margin mode
- `cross_leverage_limit`: Cross margin leverage
- `mode`: `single` (one-way), `dual_long`, `dual_short`
- `entry_price`: Average entry price
- `liq_price`: Estimated liquidation price
- `unrealised_pnl`: Unrealized P&L
- `realised_pnl`: Realized P&L
- `pnl_pnl`: Settlement P&L
- `pnl_fund`: Funding fee P&L
- `pnl_fee`: Trading fees
- `margin`: Position margin
- `pos_margin_mode`: `isolated` or `cross`

#### Get Single Position
- **Endpoint**: `GET /api/v4/futures/{settle}/positions/{contract}`
- **Auth**: Yes

#### Update Position Margin
- **Endpoint**: `POST /api/v4/futures/{settle}/positions/{contract}/margin`
- **Auth**: Yes
- **Parameters**: change (margin change amount)

#### Update Position Leverage
- **Endpoint**: `POST /api/v4/futures/{settle}/positions/{contract}/leverage`
- **Auth**: Yes
- **Parameters**: leverage (for isolated, set cross_leverage_limit empty), cross_leverage_limit (for cross, set leverage to 0)

#### Update Risk Limit
- **Endpoint**: `POST /api/v4/futures/{settle}/positions/{contract}/risk_limit`
- **Auth**: Yes

#### Set Dual Mode
- **Endpoint**: `POST /api/v4/futures/{settle}/dual_mode`
- **Auth**: Yes
- **Body**: `{"dual_mode": true}` or `{"dual_mode": false}`

#### Dual Mode Positions
- **Endpoint**: `GET /api/v4/futures/{settle}/dual_comp/positions/{contract}`
- **Auth**: Yes
- Returns both long and short positions in dual mode

---

### Futures Account

#### Get Account
- **Endpoint**: `GET /api/v4/futures/{settle}/accounts`
- **Auth**: Yes
- **Response** includes:
  - `total`: Total balance (classic accounts only)
  - `unrealised_pnl`: Unrealized P&L
  - `position_margin`: Position margin (deprecated)
  - `order_margin`: Initial margin for pending orders
  - `available`: Available balance
  - `currency`: Settlement currency
  - `in_dual_mode`: Whether dual position mode
  - `cross_margin_balance`, `cross_mmr`, `cross_imr`: Cross margin fields

#### Account Book
- **Endpoint**: `GET /api/v4/futures/{settle}/account_book`
- **Auth**: Yes
- **Parameters**: settle, limit, from, to, type
- **Type values**: `dnw` (deposit/withdraw), `pnl`, `fee`, `refr`, `fund` (funding), `point_dnw`, `point_fee`, `point_refr`, `bonus_offset`

---

### Wallet

#### Transfer Between Trading Accounts
- **Endpoint**: `POST /api/v4/wallet/transfers`
- **Auth**: Yes
- **Rate Limit**: 80r/10s
- **Body**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | yes | Currency name (e.g. `USDT`) |
| from | string | yes | Source account |
| to | string | yes | Target account |
| amount | string | yes | Transfer amount |

**Account types**: `spot`, `margin`, `futures`, `delivery`, `cross_margin`, `options`, `earn`

```json
{
  "currency": "USDT",
  "from": "spot",
  "to": "futures",
  "amount": "100"
}
```

- **Response**: `{"tx_id": "..."}`

#### Query Personal Trading Fees
- **Endpoint**: `GET /api/v4/wallet/fee`
- **Auth**: Yes
- **Parameters**: settle (optional)

#### Get Total Balance
- **Endpoint**: `GET /api/v4/wallet/total_balance`
- **Auth**: Yes

---

## WebSocket Streams

### General Format

**Subscribe Request**:
```json
{
  "time": 1545404023,
  "channel": "futures.tickers",
  "event": "subscribe",
  "payload": ["BTC_USDT"],
  "auth": {
    "method": "api_key",
    "KEY": "your_api_key",
    "SIGN": "hmac_sha512_signature"
  }
}
```

**WS Auth Signature**: `channel=<channel>&event=<event>&time=<time>` signed with HMAC-SHA512 using API secret.

**Response Format**:
```json
{
  "time": 1545404023,
  "time_ms": 1545404023123,
  "channel": "futures.tickers",
  "event": "update",
  "result": [...]
}
```

**Decimal Size Support**: Add header `"X-Gate-Size-Decimal": "1"` when connecting to receive string-type decimal sizes.

### Public Channels

#### futures.tickers
- **Subscribe**: `{"channel": "futures.tickers", "event": "subscribe", "payload": ["BTC_USDT"]}`
- **Push Data**:
```json
{
  "channel": "futures.tickers",
  "event": "update",
  "result": [
    {
      "contract": "BTC_USDT",
      "last": "118.4",
      "change_percentage": "0.77",
      "funding_rate": "-0.000114",
      "funding_rate_indicative": "0.01875",
      "mark_price": "118.35",
      "index_price": "118.36",
      "total_size": "73648",
      "volume_24h": "745487577",
      "volume_24h_base": "5526",
      "volume_24h_quote": "1665006",
      "volume_24h_settle": "178",
      "low_24h": "99.2",
      "high_24h": "132.5"
    }
  ]
}
```

#### futures.trades
- **Subscribe**: `{"channel": "futures.trades", "event": "subscribe", "payload": ["BTC_USDT"]}`
- **Push Data**:
```json
{
  "channel": "futures.trades",
  "event": "update",
  "result": [
    {
      "id": 123456,
      "create_time": 1545404023,
      "create_time_ms": 1545404023123,
      "contract": "BTC_USDT",
      "size": "100",
      "price": "6432.10"
    }
  ]
}
```

#### futures.order_book
Order book snapshots at specified levels.
- **Subscribe**: `{"channel": "futures.order_book", "event": "subscribe", "payload": ["BTC_USDT", "20", "0"]}`
  - Params: [contract, level, interval_ms]
  - Levels: `5`, `10`, `20`, `50`, `100`
  - Interval: `0` (realtime), `100`, `1000`

#### futures.book_ticker (Best Bid/Offer)
- **Subscribe**: `{"channel": "futures.book_ticker", "event": "subscribe", "payload": ["BTC_USDT"]}`
- **Push Data**:
```json
{
  "result": {
    "t": 1615366379123,
    "contract": "BTC_USDT",
    "b": "54321.0",
    "B": "100",
    "a": "54322.0",
    "A": "50"
  }
}
```
- `b`/`B`: Best bid price/size, `a`/`A`: Best ask price/size

#### futures.order_book_update
Incremental order book updates.
- **Subscribe**: `{"channel": "futures.order_book_update", "event": "subscribe", "payload": ["BTC_USDT", "100ms"]}`
  - Interval: `20ms` (20 levels only), `100ms`, `1000ms`

#### futures.candlesticks
- **Subscribe**: `{"channel": "futures.candlesticks", "event": "subscribe", "payload": ["1m", "BTC_USDT"]}`
  - Params: [interval, contract]

### Private Channels (Auth Required)

#### futures.orders
- **Subscribe**: `{"channel": "futures.orders", "event": "subscribe", "payload": ["user_id", "BTC_USDT"]}`
- **Push Data**: Order objects with status, size, price, fill info

#### futures.usertrades
- **Subscribe**: `{"channel": "futures.usertrades", "event": "subscribe", "payload": ["user_id", "BTC_USDT"]}`

#### futures.positions
- **Subscribe**: `{"channel": "futures.positions", "event": "subscribe", "payload": ["user_id", "BTC_USDT"]}`
- **Push Data**: Position updates with size, entry_price, unrealised_pnl, etc.

#### futures.balances
- **Subscribe**: `{"channel": "futures.balances", "event": "subscribe", "payload": ["user_id"]}`
- **Push Data**: Balance changes with available, total, etc.

#### futures.autoorders
- **Subscribe**: `{"channel": "futures.autoorders", "event": "subscribe", "payload": ["user_id", "BTC_USDT"]}`
- Auto/conditional order updates

---

## Error Codes

### Request/Format Errors
| Label | Meaning |
|-------|---------|
| INVALID_PARAM_VALUE | Invalid parameter value |
| INVALID_PROTOCOL | Invalid protocol |
| INVALID_ARGUMENT | Invalid argument |
| INVALID_REQUEST_BODY | Invalid request body |
| MISSING_REQUIRED_PARAM | Missing required parameter |
| BAD_REQUEST | Invalid request |
| INVALID_CONTENT_TYPE | Invalid Content-Type |
| NOT_FOUND | URL not found |

### Authentication Errors
| Label | Meaning |
|-------|---------|
| INVALID_CREDENTIALS | Invalid credentials |
| INVALID_KEY | Invalid API Key |
| IP_FORBIDDEN | IP not in whitelist |
| READ_ONLY | API key is read-only |
| INVALID_SIGNATURE | Invalid signature |
| MISSING_REQUIRED_HEADER | Missing auth header |
| REQUEST_EXPIRED | Timestamp too far from server time |
| ACCOUNT_LOCKED | Account locked |
| FORBIDDEN | No permission |

### Futures Errors
| Label | Meaning |
|-------|---------|
| USER_NOT_FOUND | No futures account (deposit to initialize) |
| CONTRACT_NOT_FOUND | Contract not found |
| RISK_LIMIT_EXCEEDED | Risk limit exceeded |
| INSUFFICIENT_AVAILABLE | Balance not enough |
| LIQUIDATE_IMMEDIATELY | Operation may cause liquidation |
| LEVERAGE_TOO_HIGH | Leverage too high |
| LEVERAGE_TOO_LOW | Leverage too low |
| ORDER_NOT_FOUND | Order not found |
| ORDER_FINISHED | Order already finished |
| TOO_MANY_ORDERS | Too many open orders |
| POSITION_CROSS_MARGIN | Margin update not allowed in cross margin |
| POSITION_IN_LIQUIDATION | Position being liquidated |
| POSITION_EMPTY | Position is empty |
| PRICE_TOO_DEVIATED | Price deviates too much |
| DUAL_SIDE_NOT_ENABLED | Dual mode not enabled |

### Wallet Errors
| Label | Meaning |
|-------|---------|
| BALANCE_NOT_ENOUGH | Balance not enough |
| MARGIN_BALANCE_NOT_ENOUGH | Margin balance not enough |
| TOO_FAST | Request exceeds frequency limit |
| WITHDRAWAL_OVER_LIMIT | Withdrawal limit exceeded |

---

## Notes for Implementation

### Response Format
- Gate.io returns **direct JSON** (no wrapper object like `{code: 0, data: ...}`)
- Arrays return `[]` directly, objects return `{}` directly
- Error responses: `{"label": "ERROR_CODE", "message": "description"}`
- Non-2xx status codes indicate errors

### Symbol Format
- Futures: `BTC_USDT` (underscore separator)
- Internal conversion: `BTCUSDT` → `BTC_USDT`
- The `settle` parameter in URL path is always lowercase: `usdt` or `btc`

### Size Convention
- **Positive size = long/buy, negative size = short/sell**
- Size is in **contracts**, not base currency
- Use `quanto_multiplier` from contract info to convert: `base_amount = size * quanto_multiplier`
- Since v4.106.0, all size fields are **string type** (may contain decimals)
- Add `X-Gate-Size-Decimal: 1` header for decimal support

### Dual Position Mode
- Enable/disable via `POST /futures/{settle}/dual_mode`
- In dual mode, positions have `mode`: `dual_long` or `dual_short`
- Use `auto_size`: `close_long` or `close_short` when closing
- In single mode, `mode` = `single`

### Position Margin Modes
- `isolated`: Isolated margin (leverage field indicates leverage)
- `cross`: Cross margin (leverage = "0", use cross_leverage_limit)

### Funding Rate
- Default interval: 28800 seconds (8 hours)
- `funding_rate` in contract/ticker: current rate
- `funding_next_apply`: Next funding timestamp
- `funding_rate_indicative`: Predicted next rate (deprecated, use funding_rate)

### Time Fields
- All timestamps are Unix timestamps in seconds
- May be integer, float, or string format
- WebSocket includes `time_ms` for millisecond precision

### Pagination
- Two methods: `page`+`limit` or `offset`+`limit`
- Default limit: 100, max: 1000
- Response headers: `X-Pagination-Limit`, `X-Pagination-Offset`, `X-Pagination-Total`

### Order Text Field
- Custom order ID: prefix with `t-`, max 28 chars after prefix
- Allowed chars: `0-9`, `A-Z`, `a-z`, `_`, `-`, `.`
- Internal prefixes: `web`, `api`, `app`, `liquidation`, etc.

### Market Orders
- Set `price: "0"` with `tif: "ioc"` for market order
- `market_order_slip_ratio`: Max slippage ratio for market orders

### WebSocket Connection
- Max 300 connections per IP
- Use protocol-level ping/pong to keep alive
- Application-level: send `{"channel": "futures.ping"}`, receive `futures.pong`
- Auth: sign `channel=<channel>&event=<event>&time=<time>` with HMAC-SHA512

---

## Appendix: Additional Endpoints (Patched)

### Position Close History

#### Query Position Close History
- **Endpoint**: `GET /api/v4/futures/{settle}/position_close`
- **Auth**: Yes
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| settle | path | string | yes | `btc` or `usdt` |
| contract | query | string | no | Futures contract, return related data only if specified |
| limit | query | integer | no | Maximum number of records returned in a single list |
| offset | query | integer | no | List offset, starting from 0 |
| from | query | integer(int64) | no | Start timestamp (Unix seconds). Defaults to data start time of range returned by `to` and `limit` |
| to | query | integer(int64) | no | End timestamp (Unix seconds). Defaults to current time |
| side | query | string | no | Query side: `long` or `short` |
| pnl | query | string | no | Query profit or loss |

- **Response** (200): Array of position close records
```json
[
  {
    "time": 1546487347,
    "pnl": "0.00013",
    "pnl_pnl": "0.00011",
    "pnl_fund": "0.00001",
    "pnl_fee": "0.00001",
    "side": "long",
    "contract": "BTC_USDT",
    "text": "web",
    "max_size": "100",
    "accum_size": "100",
    "first_open_time": 1546487347,
    "long_price": "2026.87",
    "short_price": "2544.4"
  }
]
```

**Response Fields**:
- `time`: Position close time (Unix timestamp, float)
- `contract`: Futures contract (e.g. `BTC_USDT`)
- `side`: Position side — `long` or `short`
- `pnl`: Total PnL
- `pnl_pnl`: PnL from position P/L
- `pnl_fund`: PnL from funding fees
- `pnl_fee`: PnL from transaction fees
- `text`: Source of close order (see order `text` field for values)
- `max_size`: Max trade size
- `accum_size`: Cumulative closed position volume
- `first_open_time`: First open time (Unix timestamp)
- `long_price`: When side is `long`, opening average price; when side is `short`, closing average price
- `short_price`: When side is `long`, closing average price; when side is `short`, opening average price

---

### Conditional / Price-Triggered Orders

#### Create Price-Triggered Order
- **Endpoint**: `POST /api/v4/futures/{settle}/price_orders`
- **Auth**: Yes
- **Body Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| initial | body | object | yes | Order parameters |
| initial.contract | body | string | yes | Futures contract (e.g. `BTC_USDT`) |
| initial.size | body | integer(int64) | no | Number of contracts. `0` = full close. Positive/negative indicates direction for partial close |
| initial.price | body | string | yes | Order price. `"0"` = market price |
| initial.close | body | boolean | no | Set `true` for full close in single-position mode |
| initial.tif | body | string | no | Time in force: `gtc` (default), `ioc`. Market orders only support `ioc` |
| initial.text | body | string | no | Order source: `web`, `api`, `app` |
| initial.reduce_only | body | boolean | no | When `true`, ensures order only closes/reduces position |
| initial.auto_size | body | string | no | Hedge mode full close (size=0): `close_long` or `close_short`. Not required for one-way mode or partial close |
| trigger | body | object | yes | Trigger condition |
| trigger.strategy_type | body | integer(int32) | no | `0` = price trigger (default), `1` = price spread trigger |
| trigger.price_type | body | integer(int32) | no | `0` = latest trade price, `1` = mark price, `2` = index price |
| trigger.price | body | string | yes | Trigger price value |
| trigger.rule | body | integer(int32) | yes | `1` = trigger when price >= trigger price (trigger price must > last price). `2` = trigger when price <= trigger price (trigger price must < last price) |
| trigger.expiration | body | integer | no | Max wait time in seconds. Order cancelled if timeout |
| order_type | body | string | no | Take-profit/stop-loss type (see values below) |
| settle | path | string | yes | `btc` or `usdt` |

**order_type Values**:
- `close-long-order`: Order TP/SL, close long position
- `close-short-order`: Order TP/SL, close short position
- `close-long-position`: Position TP/SL, close all long positions
- `close-short-position`: Position TP/SL, close all short positions
- `plan-close-long-position`: Position plan TP/SL, close all or partial long
- `plan-close-short-position`: Position plan TP/SL, close all or partial short

**Note**: The two `close-*-order` types are read-only and cannot be passed in requests.

- **Response** (201):
```json
{
  "id": 1432329
}
```

**Request Body Example**:
```json
{
  "initial": {
    "contract": "BTC_USDT",
    "size": 100,
    "price": "5.03"
  },
  "trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "3000",
    "rule": 1,
    "expiration": 86400
  },
  "order_type": "close-long-order"
}
```

#### Cancel Single Conditional Order
- **Endpoint**: `DELETE /api/v4/futures/{settle}/price_orders/{order_id}`
- **Auth**: Yes
- **Parameters**:

| Name | In | Type | Required | Description |
|------|-----|------|----------|-------------|
| settle | path | string | yes | `btc` or `usdt` |
| order_id | path | integer(int64) | yes | Order ID returned when order was successfully created |

- **Response** (200): `FuturesPriceTriggeredOrder` object
```json
{
  "initial": {
    "contract": "BTC_USDT",
    "size": 100,
    "price": "5.03"
  },
  "trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "3000",
    "rule": 1,
    "expiration": 86400
  },
  "id": 1283293,
  "user": 1234,
  "create_time": 1514764800,
  "finish_time": 1514764900,
  "trade_id": 13566,
  "status": "finished",
  "finish_as": "cancelled",
  "reason": "",
  "order_type": "close-long-order"
}
```

#### FuturesPriceTriggeredOrder Schema Reference

```json
{
  "initial": {
    "contract": "string",
    "size": 0,
    "price": "string",
    "close": false,
    "tif": "gtc",
    "text": "string",
    "reduce_only": false,
    "auto_size": "string",
    "is_reduce_only": true,
    "is_close": true
  },
  "trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "string",
    "rule": 1,
    "expiration": 0
  },
  "id": 0,
  "user": 0,
  "create_time": 0,
  "finish_time": 0,
  "trade_id": 0,
  "status": "open",
  "finish_as": "cancelled",
  "reason": "string",
  "order_type": "string",
  "me_order_id": 0
}
```

**Status Values**: `open`, `finished`, `inactive`, `invalid`
**finish_as Values**: `cancelled`, `succeeded`, `failed`, `expired`
