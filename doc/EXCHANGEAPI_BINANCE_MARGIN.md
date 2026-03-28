# Binance Margin Trading API Documentation Reference

> Scraped from [Binance Margin Trading API Docs](https://developers.binance.com/docs/margin_trading) on 2026-03-28.
> Focus: **Cross Margin** endpoints only (isolated margin parameters noted but not primary).

## Base URLs

### REST API
- **Production**: `https://api.binance.com`
- All margin endpoints use the `/sapi/` prefix on the spot/wallet base URL.

### Authentication
- Same as Binance Spot/Futures: HMAC SHA256 signature via `X-MBX-APIKEY` header.
- All endpoints below require API-Key + signature unless noted otherwise.
- `timestamp` (ms) and `signature` are required on all SIGNED endpoints.
- `recvWindow` (optional, default 5000, max 60000) controls request validity window.

---

## REST API Endpoints

### Cross Margin -- Borrow & Repay

#### Margin Account Borrow/Repay
- **Endpoint**: `POST /sapi/v1/margin/borrow-repay`
- **Weight**: 1500
- **Auth**: MARGIN (API-Key + signature)
- **Description**: Borrow or repay assets in a margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | Yes | Asset to borrow/repay |
| isIsolated | STRING | Yes | "TRUE" for Isolated Margin, "FALSE" for Cross Margin. Default "FALSE" |
| symbol | STRING | Yes | Only required for Isolated Margin |
| amount | STRING | Yes | Amount to borrow or repay |
| type | STRING | Yes | `BORROW` or `REPAY` |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| tranId | long | Transaction ID |

- **Response Example**:
```json
{
  "tranId": 100000001
}
```

- **Notes**:
  - Error `-3045` (INSUFFICIENT_INVENTORY): System doesn't have enough asset to lend.
  - Error `-3006` (EXCEED_MAX_BORROWABLE): Borrow exceeds max. Check via `GET /sapi/v1/margin/maxBorrowable`.
  - Error `-3015` (REPAY_EXCEED_LIABILITY): Repayment exceeds outstanding debt.
  - Error `-3012` (ASSET_ADMIN_BAN_BORROW): Borrowing prohibited for this asset.
  - Error `-3007` (HAS_PENDING_TRANSACTION): Another borrow/repay is processing. Space requests ~100ms apart.

---

#### Query Borrow/Repay Records
- **Endpoint**: `GET /sapi/v1/margin/borrow-repay`
- **Weight**: 10 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query borrow/repay records in margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | No | |
| isolatedSymbol | STRING | No | Symbol in Isolated Margin |
| txId | LONG | No | Transaction ID from POST borrow-repay |
| startTime | LONG | No | |
| endTime | LONG | No | |
| current | LONG | No | Page number, start from 1. Default: 1 |
| size | LONG | No | Default: 10, Max: 100 |
| type | STRING | Yes | `BORROW` or `REPAY` |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| rows[].type | string | AUTO, MANUAL (borrow); MANUAL, AUTO, BNB_AUTO_REPAY, POINT_AUTO_REPAY (repay) |
| rows[].isolatedSymbol | string | Only returned for isolated margin |
| rows[].amount | string | Total amount borrowed/repaid |
| rows[].asset | string | Asset symbol |
| rows[].interest | string | Interest repaid |
| rows[].principal | string | Principal repaid |
| rows[].status | string | PENDING, CONFIRMED, FAILED |
| rows[].timestamp | long | Timestamp (ms) |
| rows[].txId | long | Transaction ID |
| total | int | Total number of records |

- **Response Example**:
```json
{
  "rows": [
    {
      "type": "AUTO",
      "amount": "14.00000000",
      "asset": "BNB",
      "interest": "0.01866667",
      "principal": "13.98133333",
      "status": "CONFIRMED",
      "timestamp": 1563438204000,
      "txId": 2970933056
    }
  ],
  "total": 1
}
```

- **Notes**:
  - `txId` or `startTime` must be sent. `txId` takes precedence.
  - If asset is sent, returns data within 30 days before endTime; if not sent, 7 days.
  - Response in descending order.

---

#### Query Max Borrow
- **Endpoint**: `GET /sapi/v1/margin/maxBorrowable`
- **Weight**: 50 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query maximum borrowable amount for an asset.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | Yes | |
| isolatedSymbol | STRING | No | If not sent, cross margin data returned |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| amount | string | Currently max borrowable amount with sufficient system availability |
| borrowLimit | string | Max borrowable amount limited by account level |

- **Response Example**:
```json
{
  "amount": "1.69248805",
  "borrowLimit": "60"
}
```

---

#### Get Next Hourly Interest Rate
- **Endpoint**: `GET /sapi/v1/margin/next-hourly-interest-rate`
- **Weight**: 100 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Get the next hourly interest rate for specified assets.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| assets | String | Yes | List of assets, separated by commas, up to 20 |
| isIsolated | Boolean | Yes | "TRUE" or "FALSE" |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| asset | string | Asset symbol |
| nextHourlyInterestRate | string | Next hourly interest rate |

- **Response Example**:
```json
[
    {
        "asset": "BTC",
        "nextHourlyInterestRate": "0.00000571"
    },
    {
        "asset": "ETH",
        "nextHourlyInterestRate": "0.00000578"
    }
]
```

---

#### Query Margin Interest Rate History
- **Endpoint**: `GET /sapi/v1/margin/interestRateHistory`
- **Weight**: 1 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query historical interest rate data for a margin asset.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | Yes | |
| vipLevel | INT | No | Default: user's VIP level |
| startTime | LONG | No | Default: 7 days ago |
| endTime | LONG | No | Default: present. Max range: 1 month |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| asset | string | Asset symbol |
| dailyInterestRate | string | Daily interest rate |
| timestamp | long | Timestamp (ms) |
| vipLevel | int | VIP level |

- **Response Example**:
```json
[
    {
        "asset": "BTC",
        "dailyInterestRate": "0.00025000",
        "timestamp": 1611544731000,
        "vipLevel": 1
    }
]
```

---

#### Get Interest History
- **Endpoint**: `GET /sapi/v1/margin/interestHistory`
- **Weight**: 1 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Get interest accrual history.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | No | |
| isolatedSymbol | STRING | No | Isolated symbol; if not sent, cross margin data returned |
| startTime | LONG | No | |
| endTime | LONG | No | |
| current | LONG | No | Page number, start from 1. Default: 1 |
| size | LONG | No | Default: 10, Max: 100 |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| rows[].txId | long | Transaction ID |
| rows[].interestAccuredTime | long | Interest accrued timestamp (ms) |
| rows[].asset | string | Asset symbol |
| rows[].rawAsset | string | Raw asset (not returned for isolated margin) |
| rows[].principal | string | Principal amount |
| rows[].interest | string | Interest charged |
| rows[].interestRate | string | Interest rate |
| rows[].type | string | PERIODIC, ON_BORROW, PERIODIC_CONVERTED, ON_BORROW_CONVERTED, PORTFOLIO |
| rows[].isolatedSymbol | string | Only returned for isolated margin |
| total | int | Total records |

- **Response Example**:
```json
{
  "rows": [
    {
      "txId": 1352286576452864727,
      "interestAccuredTime": 1672160400000,
      "asset": "USDT",
      "rawAsset": "USDT",
      "principal": "45.3313",
      "interest": "0.00024995",
      "interestRate": "0.00013233",
      "type": "ON_BORROW"
    }
  ],
  "total": 1
}
```

- **Notes**:
  - Max interval between startTime and endTime is 30 days.
  - Default: last 7 days if no time range sent.
  - `type` enums: PERIODIC (hourly), ON_BORROW (first charge), PERIODIC_CONVERTED (hourly BNB), ON_BORROW_CONVERTED (first BNB), PORTFOLIO (daily portfolio margin).

---

### Cross Margin -- Orders

#### Margin Account New Order
- **Endpoint**: `POST /sapi/v1/margin/order`
- **Weight**: 6 (UID)
- **Auth**: TRADE (API-Key + signature)
- **Description**: Place a new order for margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | Trading pair (e.g. BTCUSDT) |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| side | ENUM | Yes | `BUY` or `SELL` |
| type | ENUM | Yes | LIMIT, MARKET, STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, TAKE_PROFIT_LIMIT, LIMIT_MAKER |
| quantity | DECIMAL | No | |
| quoteOrderQty | DECIMAL | No | Quote quantity for MARKET orders |
| price | DECIMAL | No | Required for LIMIT orders |
| stopPrice | DECIMAL | No | Used with STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, TAKE_PROFIT_LIMIT |
| newClientOrderId | STRING | No | Unique ID among open orders. Auto-generated if not sent |
| icebergQty | DECIMAL | No | Used with LIMIT, STOP_LOSS_LIMIT, TAKE_PROFIT_LIMIT |
| newOrderRespType | ENUM | No | ACK, RESULT, or FULL. MARKET/LIMIT default to FULL, others to ACK |
| sideEffectType | ENUM | No | NO_SIDE_EFFECT, MARGIN_BUY, AUTO_REPAY, AUTO_BORROW_REPAY. Default: NO_SIDE_EFFECT |
| timeInForce | ENUM | No | GTC, IOC, FOK |
| selfTradePreventionMode | ENUM | No | EXPIRE_TAKER, EXPIRE_MAKER, EXPIRE_BOTH, NONE |
| autoRepayAtCancel | BOOLEAN | No | For MARGIN_BUY/AUTO_BORROW_REPAY orders: repay debt on cancel. Default: true. Suggest "FALSE" for high-frequency |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields (FULL)**:

| Field | Type | Description |
|---|---|---|
| symbol | string | Trading pair |
| orderId | long | Order ID |
| clientOrderId | string | Client order ID |
| transactTime | long | Transaction timestamp (ms) |
| price | string | Order price |
| origQty | string | Original quantity |
| executedQty | string | Executed quantity |
| cummulativeQuoteQty | string | Cumulative quote quantity |
| status | string | NEW, PARTIALLY_FILLED, FILLED, CANCELED, etc. |
| timeInForce | string | GTC, IOC, FOK |
| type | string | Order type |
| side | string | BUY or SELL |
| marginBuyBorrowAmount | decimal | Borrow amount (only if margin trade) |
| marginBuyBorrowAsset | string | Borrowed asset (only if margin trade) |
| isIsolated | boolean | Whether isolated margin |
| selfTradePreventionMode | string | STP mode |
| fills | array | Array of fill objects |
| fills[].price | string | Fill price |
| fills[].qty | string | Fill quantity |
| fills[].commission | string | Commission amount |
| fills[].commissionAsset | string | Commission asset |
| fills[].tradeId | long | Trade ID |

- **Response Example (ACK)**:
```json
{
  "symbol": "BTCUSDT",
  "orderId": 28,
  "clientOrderId": "6gCrw2kRUAF9CvJDGP16IP",
  "isIsolated": true,
  "transactTime": 1507725176595
}
```

- **Response Example (RESULT)**:
```json
{
  "symbol": "BTCUSDT",
  "orderId": 26769564559,
  "clientOrderId": "E156O3KP4gOif65bjuUK5V",
  "transactTime": 1713873075893,
  "price": "0",
  "origQty": "0.001",
  "executedQty": "0.001",
  "cummulativeQuoteQty": "65982.53",
  "status": "FILLED",
  "timeInForce": "GTC",
  "type": "MARKET",
  "side": "SELL",
  "isIsolated": false,
  "selfTradePreventionMode": "EXPIRE_MAKER"
}
```

- **Response Example (FULL)**:
```json
{
  "symbol": "BTCUSDT",
  "orderId": 26769564559,
  "clientOrderId": "E156O3KP4gOif65bjuUK5V",
  "transactTime": 1713873075893,
  "price": "0",
  "origQty": "0.001",
  "executedQty": "0.001",
  "cummulativeQuoteQty": "65.98253",
  "status": "FILLED",
  "timeInForce": "GTC",
  "type": "MARKET",
  "side": "SELL",
  "marginBuyBorrowAmount": 5,
  "marginBuyBorrowAsset": "BTC",
  "isIsolated": false,
  "selfTradePreventionMode": "EXPIRE_MAKER",
  "fills": [
    {
      "price": "65982.53",
      "qty": "0.001",
      "commission": "0.06598253",
      "commissionAsset": "USDT",
      "tradeId": 3570680726
    }
  ]
}
```

- **Notes**:
  - `sideEffectType` controls auto-borrow behavior:
    - `NO_SIDE_EFFECT`: Normal order, no borrowing.
    - `MARGIN_BUY`: Auto-borrow if insufficient balance.
    - `AUTO_REPAY`: Auto-repay debt when selling.
    - `AUTO_BORROW_REPAY`: Combined auto-borrow and auto-repay.
  - Error `-3067` (ASSET_BAN_TRADE): Asset restricted from margin trading.
  - Error `-3027` (NOT_VALID_MARGIN_ASSET): Asset not supported on margin.
  - Error `-3041` (BALANCE_NOT_CLEARED): Insufficient balance.
  - Error `-1015` (TOO_MANY_ORDERS): Order rate limit exceeded.
  - Error `-3064` (EXCEED_PRICE_LIMIT): Limit price outside [-15%, 15%] of index price for certain pairs.

---

#### Margin Account Cancel Order
- **Endpoint**: `DELETE /sapi/v1/margin/order`
- **Weight**: 10 (IP)
- **Auth**: TRADE (API-Key + signature)
- **Description**: Cancel an active order for margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| orderId | LONG | No | |
| origClientOrderId | STRING | No | |
| newClientOrderId | STRING | No | Unique cancel ID. Auto-generated by default |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| symbol | string | Trading pair |
| isIsolated | boolean | Whether isolated margin |
| orderId | string | Order ID |
| origClientOrderId | string | Original client order ID |
| clientOrderId | string | Cancel client order ID |
| price | string | Order price |
| origQty | string | Original quantity |
| executedQty | string | Executed quantity |
| cummulativeQuoteQty | string | Cumulative quote quantity |
| status | string | CANCELED |
| timeInForce | string | GTC, IOC, FOK |
| type | string | Order type |
| side | string | BUY or SELL |

- **Response Example**:
```json
{
  "symbol": "LTCBTC",
  "isIsolated": true,
  "orderId": "28",
  "origClientOrderId": "myOrder1",
  "clientOrderId": "cancelMyOrder1",
  "price": "1.00000000",
  "origQty": "10.00000000",
  "executedQty": "8.00000000",
  "cummulativeQuoteQty": "8.00000000",
  "status": "CANCELED",
  "timeInForce": "GTC",
  "type": "LIMIT",
  "side": "SELL"
}
```

- **Notes**:
  - Either `orderId` or `origClientOrderId` must be sent.
  - Error `-2011` (CANCEL_REJECTED / Unknown order sent): Order already processed or not found.

---

#### Cancel All Open Orders on a Symbol
- **Endpoint**: `DELETE /sapi/v1/margin/openOrders`
- **Weight**: 1
- **Auth**: TRADE (API-Key + signature)
- **Description**: Cancel all active orders on a symbol for margin account. Includes OCO orders.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Example**:
```json
[
  {
    "symbol": "BTCUSDT",
    "isIsolated": true,
    "origClientOrderId": "E6APeyTJvkMvLMYMqu1KQ4",
    "orderId": 11,
    "orderListId": -1,
    "clientOrderId": "pXLV6Hz6mprAcVYpVMTGgx",
    "price": "0.089853",
    "origQty": "0.178622",
    "executedQty": "0.000000",
    "cummulativeQuoteQty": "0.000000",
    "status": "CANCELED",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "side": "BUY",
    "selfTradePreventionMode": "NONE"
  }
]
```

---

#### Query Margin Account's Order
- **Endpoint**: `GET /sapi/v1/margin/order`
- **Weight**: 10 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query a single order by orderId or origClientOrderId.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| orderId | LONG | No | |
| origClientOrderId | STRING | No | |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| clientOrderId | string | Client order ID |
| cummulativeQuoteQty | string | Cumulative quote quantity |
| executedQty | string | Executed quantity |
| icebergQty | string | Iceberg quantity |
| isWorking | boolean | Whether order is on the book |
| orderId | long | Order ID |
| origQty | string | Original quantity |
| price | string | Order price |
| side | string | BUY or SELL |
| status | string | Order status |
| stopPrice | string | Stop price |
| symbol | string | Trading pair |
| isIsolated | boolean | Whether isolated margin |
| time | long | Order creation time (ms) |
| timeInForce | string | GTC, IOC, FOK |
| type | string | Order type |
| selfTradePreventionMode | string | STP mode |
| updateTime | long | Last update time (ms) |

- **Response Example**:
```json
{
  "clientOrderId": "ZwfQzuDIGpceVhKW5DvCmO",
  "cummulativeQuoteQty": "0.00000000",
  "executedQty": "0.00000000",
  "icebergQty": "0.00000000",
  "isWorking": true,
  "orderId": 213205622,
  "origQty": "0.30000000",
  "price": "0.00493630",
  "side": "SELL",
  "status": "NEW",
  "stopPrice": "0.00000000",
  "symbol": "BNBBTC",
  "isIsolated": true,
  "time": 1562133008725,
  "timeInForce": "GTC",
  "type": "LIMIT",
  "selfTradePreventionMode": "NONE",
  "updateTime": 1562133008725
}
```

- **Notes**:
  - Either `orderId` or `origClientOrderId` must be sent.
  - For some historical orders `cummulativeQuoteQty` will be < 0 (data unavailable).

---

#### Query Margin Account's Open Orders
- **Endpoint**: `GET /sapi/v1/margin/openOrders`
- **Weight**: 10 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query all open orders on margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | No | If not sent, returns all symbols |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE". If "TRUE", symbol must be sent |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Example**:
```json
[
  {
    "clientOrderId": "qhcZw71gAkCCTv0t0k8LUK",
    "cummulativeQuoteQty": "0.00000000",
    "executedQty": "0.00000000",
    "icebergQty": "0.00000000",
    "isWorking": true,
    "orderId": 211842552,
    "origQty": "0.30000000",
    "price": "0.00475010",
    "side": "SELL",
    "status": "NEW",
    "stopPrice": "0.00000000",
    "symbol": "BNBBTC",
    "isIsolated": true,
    "time": 1562040170089,
    "timeInForce": "GTC",
    "type": "LIMIT",
    "selfTradePreventionMode": "NONE",
    "updateTime": 1562040170089
  }
]
```

- **Notes**:
  - When all symbols are returned, weight counted equals the number of currently trading symbols.

---

#### Query Margin Account's All Orders
- **Endpoint**: `GET /sapi/v1/margin/allOrders`
- **Weight**: 200 (IP)
- **Rate Limit**: 60 times/min per IP
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query all orders (active, canceled, filled) for margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| orderId | LONG | No | If set, returns orders >= this orderId |
| startTime | LONG | No | |
| endTime | LONG | No | |
| limit | INT | No | Default: 500, Max: 500 |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Example**:
```json
[
  {
    "clientOrderId": "D2KDy4DIeS56PvkM13f8cP",
    "cummulativeQuoteQty": "0.00000000",
    "executedQty": "0.00000000",
    "icebergQty": "0.00000000",
    "isWorking": false,
    "orderId": 41295,
    "origQty": "5.31000000",
    "price": "0.22500000",
    "side": "SELL",
    "status": "CANCELED",
    "stopPrice": "0.18000000",
    "symbol": "BNBBTC",
    "isIsolated": false,
    "time": 1565769338806,
    "timeInForce": "GTC",
    "type": "TAKE_PROFIT_LIMIT",
    "selfTradePreventionMode": "NONE",
    "updateTime": 1565769342148
  }
]
```

- **Notes**:
  - If `orderId` is not set, returns orders within the last 24 hours.
  - Max 24 hours between `startTime` and `endTime`.

---

#### Query Margin Account's Trade List
- **Endpoint**: `GET /sapi/v1/margin/myTrades`
- **Weight**: 10 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query trade history for margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | STRING | Yes | |
| isIsolated | STRING | No | "TRUE" or "FALSE", default "FALSE" |
| orderId | LONG | No | |
| startTime | LONG | No | |
| endTime | LONG | No | |
| fromId | LONG | No | TradeId to fetch from. Default: most recent |
| limit | INT | No | Default: 500, Max: 1000 |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| commission | string | Commission charged |
| commissionAsset | string | Asset used for commission |
| id | long | Trade ID |
| isBestMatch | boolean | Best price match |
| isBuyer | boolean | Whether buyer |
| isMaker | boolean | Whether maker |
| orderId | long | Order ID |
| price | string | Trade price |
| qty | string | Trade quantity |
| symbol | string | Trading pair |
| isIsolated | boolean | Whether isolated margin |
| time | long | Trade timestamp (ms) |

- **Response Example**:
```json
[
  {
    "commission": "0.00006000",
    "commissionAsset": "BTC",
    "id": 34,
    "isBestMatch": true,
    "isBuyer": false,
    "isMaker": false,
    "orderId": 39324,
    "price": "0.02000000",
    "qty": "3.00000000",
    "symbol": "BNBBTC",
    "isIsolated": false,
    "time": 1561973357171
  }
]
```

- **Notes**:
  - If `fromId` is set, returns trades >= that fromId. Otherwise last 24 hours.
  - Max 24 hours between `startTime` and `endTime`.

---

### Cross Margin -- Account

#### Query Cross Margin Account Details
- **Endpoint**: `GET /sapi/v1/margin/account`
- **Weight**: 10 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Get cross margin account details including balances, margin level, and borrow status.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| created | boolean | true = margin account created |
| borrowEnabled | boolean | Whether borrowing is enabled |
| marginLevel | string | Current margin level |
| collateralMarginLevel | string | Collateral margin level |
| totalAssetOfBtc | string | Total asset value in BTC |
| totalLiabilityOfBtc | string | Total liability in BTC |
| totalNetAssetOfBtc | string | Net asset value in BTC |
| TotalCollateralValueInUSDT | string | Total collateral value in USDT |
| totalOpenOrderLossInUSDT | string | Open order unrealized loss in USDT |
| tradeEnabled | boolean | Whether trading is enabled |
| transferInEnabled | boolean | Whether transfer-in is enabled |
| transferOutEnabled | boolean | Whether transfer-out is enabled |
| accountType | string | MARGIN_1 (Cross Classic) or MARGIN_2 (Cross Pro) |
| userAssets | array | Array of asset objects |
| userAssets[].asset | string | Asset symbol |
| userAssets[].borrowed | string | Borrowed amount |
| userAssets[].free | string | Free (available) balance |
| userAssets[].interest | string | Accrued interest |
| userAssets[].locked | string | Locked in orders |
| userAssets[].netAsset | string | Net asset (free + locked - borrowed - interest) |

- **Response Example**:
```json
{
  "created": true,
  "borrowEnabled": true,
  "marginLevel": "11.64405625",
  "collateralMarginLevel": "3.2",
  "totalAssetOfBtc": "6.82728457",
  "totalLiabilityOfBtc": "0.58633215",
  "totalNetAssetOfBtc": "6.24095242",
  "TotalCollateralValueInUSDT": "5.82728457",
  "totalOpenOrderLossInUSDT": "582.728457",
  "tradeEnabled": true,
  "transferInEnabled": true,
  "transferOutEnabled": true,
  "accountType": "MARGIN_1",
  "userAssets": [
    {
      "asset": "BTC",
      "borrowed": "0.00000000",
      "free": "0.00499500",
      "interest": "0.00000000",
      "locked": "0.00000000",
      "netAsset": "0.00499500"
    },
    {
      "asset": "USDT",
      "borrowed": "0.00000000",
      "free": "0.00000000",
      "interest": "0.00000000",
      "locked": "0.00000000",
      "netAsset": "0.00000000"
    }
  ]
}
```

---

#### Adjust Cross Margin Max Leverage
- **Endpoint**: `POST /sapi/v1/margin/max-leverage`
- **Weight**: 3000 (UID)
- **Rate Limit**: 1 time/min per IP
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Adjust cross margin max leverage level.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| maxLeverage | Integer | Yes | 3 or 5 for Cross Margin Classic; 10 (or 20 if compliance allows) for Cross Margin Pro |

- **Response Example**:
```json
{
    "success": true
}
```

- **Notes**:
  - Margin level must be higher than the initial risk ratio of the adjusted leverage (3x = 1.5 ratio, 5x = 1.25 ratio).

---

### Cross Margin -- Transfer

#### Get Cross Margin Transfer History
- **Endpoint**: `GET /sapi/v1/margin/transfer`
- **Weight**: 1 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Get cross margin transfer history.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | No | |
| type | STRING | No | Transfer Type: ROLL_IN, ROLL_OUT |
| startTime | LONG | No | |
| endTime | LONG | No | |
| current | LONG | No | Page number, start from 1. Default: 1 |
| size | LONG | No | Default: 10, Max: 100 |
| isolatedSymbol | STRING | No | Symbol in Isolated Margin |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| rows[].amount | string | Transfer amount |
| rows[].asset | string | Asset symbol |
| rows[].status | string | CONFIRMED, etc. |
| rows[].timestamp | long | Timestamp |
| rows[].txId | long | Transaction ID |
| rows[].type | string | ROLL_IN or ROLL_OUT |
| rows[].transFrom | string | Source: SPOT, FUTURES, CROSS_MARGIN, ISOLATED_MARGIN, FUNDING, etc. |
| rows[].transTo | string | Destination: same enums as transFrom |
| rows[].fromSymbol | string | Source isolated margin symbol (if applicable) |
| rows[].toSymbol | string | Destination isolated margin symbol (if applicable) |
| total | int | Total records |

- **Response Example**:
```json
{
  "rows": [
    {
      "amount": "0.10000000",
      "asset": "BNB",
      "status": "CONFIRMED",
      "timestamp": 1566898617,
      "txId": 5240372201,
      "type": "ROLL_IN",
      "transFrom": "SPOT",
      "transTo": "CROSS_MARGIN"
    }
  ],
  "total": 1
}
```

- **Notes**:
  - Max interval between startTime and endTime is 30 days.
  - Default: last 7 days data.
  - Response in descending order.

---

#### Query Max Transfer-Out Amount
- **Endpoint**: `GET /sapi/v1/margin/maxTransferable`
- **Weight**: 50 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query maximum amount that can be transferred out of margin account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| asset | STRING | Yes | |
| isolatedSymbol | STRING | No | If not sent, cross margin data returned |
| recvWindow | LONG | No | Max 60000 |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| amount | string | Max transferable amount |

- **Response Example**:
```json
{
  "amount": "3.59498107"
}
```

---

### Transfer (Wallet -- Universal Transfer)

#### User Universal Transfer
- **Endpoint**: `POST /sapi/v1/asset/transfer`
- **Weight**: 900 (UID)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Universal transfer between Binance account types (spot, futures, margin, funding, etc.). Requires "Permits Universal Transfer" option enabled on the API key.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| type | ENUM | Yes | Transfer type enum (see list below) |
| asset | STRING | Yes | Asset to transfer |
| amount | DECIMAL | Yes | Amount to transfer |
| fromSymbol | STRING | No | Required for ISOLATEDMARGIN_MARGIN and ISOLATEDMARGIN_ISOLATEDMARGIN |
| toSymbol | STRING | No | Required for MARGIN_ISOLATEDMARGIN and ISOLATEDMARGIN_ISOLATEDMARGIN |
| recvWindow | LONG | No | |
| timestamp | LONG | Yes | |

- **Transfer Type Enums (margin-relevant)**:

| Type | Description |
|---|---|
| MAIN_MARGIN | Spot -> Cross Margin |
| MARGIN_MAIN | Cross Margin -> Spot |
| MAIN_UMFUTURE | Spot -> USDS-M Futures |
| UMFUTURE_MAIN | USDS-M Futures -> Spot |
| MARGIN_UMFUTURE | Cross Margin -> USDS-M Futures |
| UMFUTURE_MARGIN | USDS-M Futures -> Cross Margin |
| MAIN_CMFUTURE | Spot -> COIN-M Futures |
| CMFUTURE_MAIN | COIN-M Futures -> Spot |
| CMFUTURE_MARGIN | COIN-M Futures -> Cross Margin |
| MARGIN_CMFUTURE | Cross Margin -> COIN-M Futures |
| MAIN_FUNDING | Spot -> Funding |
| FUNDING_MAIN | Funding -> Spot |
| FUNDING_UMFUTURE | Funding -> USDS-M Futures |
| UMFUTURE_FUNDING | USDS-M Futures -> Funding |
| MARGIN_FUNDING | Cross Margin -> Funding |
| FUNDING_MARGIN | Funding -> Cross Margin |
| ISOLATEDMARGIN_MARGIN | Isolated Margin -> Cross Margin |
| MARGIN_ISOLATEDMARGIN | Cross Margin -> Isolated Margin |
| ISOLATEDMARGIN_ISOLATEDMARGIN | Isolated Margin -> Isolated Margin |
| MAIN_OPTION | Spot -> Options |
| OPTION_MAIN | Options -> Spot |
| UMFUTURE_OPTION | USDS-M Futures -> Options |
| OPTION_UMFUTURE | Options -> USDS-M Futures |
| MARGIN_OPTION | Cross Margin -> Options |
| OPTION_MARGIN | Options -> Cross Margin |
| FUNDING_OPTION | Funding -> Options |
| OPTION_FUNDING | Options -> Funding |
| MAIN_PORTFOLIO_MARGIN | Spot -> Portfolio Margin |
| PORTFOLIO_MARGIN_MAIN | Portfolio Margin -> Spot |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| tranId | long | Transfer transaction ID |

- **Response Example**:
```json
{
    "tranId": 13526853623
}
```

---

#### Query User Universal Transfer History
- **Endpoint**: `GET /sapi/v1/asset/transfer`
- **Weight**: 1 (IP)
- **Auth**: USER_DATA (API-Key + signature)
- **Description**: Query universal transfer history.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| type | ENUM | Yes | Same transfer type enums as above |
| startTime | LONG | No | |
| endTime | LONG | No | |
| current | INT | No | Page number. Default: 1 |
| size | INT | No | Default: 10, Max: 100 |
| fromSymbol | STRING | No | Required for ISOLATEDMARGIN types |
| toSymbol | STRING | No | Required for ISOLATEDMARGIN types |
| recvWindow | LONG | No | |
| timestamp | LONG | Yes | |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| total | int | Total records |
| rows[].asset | string | Asset symbol |
| rows[].amount | string | Transfer amount |
| rows[].type | string | Transfer type enum |
| rows[].status | string | CONFIRMED, FAILED, PENDING |
| rows[].tranId | long | Transaction ID |
| rows[].timestamp | long | Timestamp (ms) |

- **Response Example**:
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

- **Notes**:
  - Supports query within the last 6 months only.
  - If startTime and endTime not sent, returns last 7 days by default.

---

## Endpoint Quick Reference

| Method | Path | Description | Weight |
|---|---|---|---|
| POST | /sapi/v1/margin/borrow-repay | Borrow or repay | 1500 |
| GET | /sapi/v1/margin/borrow-repay | Query borrow/repay records | 10 |
| GET | /sapi/v1/margin/maxBorrowable | Query max borrow | 50 |
| GET | /sapi/v1/margin/next-hourly-interest-rate | Next hourly interest rate | 100 |
| GET | /sapi/v1/margin/interestRateHistory | Interest rate history | 1 |
| GET | /sapi/v1/margin/interestHistory | Interest charge history | 1 |
| POST | /sapi/v1/margin/order | Place margin order | 6 |
| DELETE | /sapi/v1/margin/order | Cancel margin order | 10 |
| DELETE | /sapi/v1/margin/openOrders | Cancel all open orders | 1 |
| GET | /sapi/v1/margin/order | Query single order | 10 |
| GET | /sapi/v1/margin/openOrders | Query open orders | 10 |
| GET | /sapi/v1/margin/allOrders | Query all orders | 200 |
| GET | /sapi/v1/margin/myTrades | Query trade list | 10 |
| GET | /sapi/v1/margin/account | Query account details | 10 |
| POST | /sapi/v1/margin/max-leverage | Adjust max leverage | 3000 |
| GET | /sapi/v1/margin/transfer | Cross margin transfer history | 1 |
| GET | /sapi/v1/margin/maxTransferable | Max transfer-out amount | 50 |
| POST | /sapi/v1/asset/transfer | Universal transfer | 900 |
| GET | /sapi/v1/asset/transfer | Universal transfer history | 1 |
