# Bitget Cross Margin API Documentation Reference

> Scraped from https://www.bitget.com/api-doc/margin/ on 2026-03-28

## Base URL

`https://api.bitget.com`

## Authentication

All private endpoints require the same auth headers as the main Bitget API (see EXCHANGEAPI_BITGET.md):

| Header | Description |
|--------|-------------|
| `ACCESS-KEY` | API Key |
| `ACCESS-SIGN` | Base64-encoded HMAC-SHA256 signature |
| `ACCESS-TIMESTAMP` | Timestamp in milliseconds |
| `ACCESS-PASSPHRASE` | API Key passphrase |
| `Content-Type` | `application/json` |
| `locale` | `en-US` |

## Account Types for Transfer

| Account Type | Value |
|-------------|-------|
| Spot | `spot` |
| P2P/Funding | `p2p` |
| Coin-M Futures | `coin_futures` |
| USDT-M Futures | `usdt_futures` |
| USDC-M Futures | `usdc_futures` |
| Cross Margin | `crossed_margin` |
| Isolated Margin | `isolated_margin` |

---

## REST API Endpoints

### Common

#### Get Support Currencies
- **Endpoint**: `GET /api/v2/margin/currencies`
- **Rate Limit**: 10 req/1s (IP)
- **Auth**: Yes
- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| symbol | String | Trading pair |
| baseCoin | String | Base currency |
| quoteCoin | String | Quote currency |
| maxCrossedLeverage | String | Cross margin maximum leverage multiples |
| maxIsolatedLeverage | String | Isolated margin maximum leverage multiples |
| warningRiskRatio | String | Warning risk ratio |
| liquidationRiskRatio | String | Liquidation risk ratio |
| minTradeAmount | String | Minimum trading volume |
| maxTradeAmount | String | Maximum trading volume |
| takerFeeRate | String | Taker fee rate |
| makerFeeRate | String | Maker fee rate |
| pricePrecision | String | Price precision (decimal places) |
| quantityPrecision | String | Quantity precision (decimal places) |
| minTradeUSDT | String | Minimum trading volume (USDT) |

---

### Cross Margin -- Account

#### Get Cross Account Assets
- **Endpoint**: `GET /api/v2/margin/crossed/account/assets`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | No | Coin, e.g. `USDT` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| coin | String | Token name |
| totalAmount | String | Total amount |
| available | String | Available amount |
| frozen | String | Assets frozen |
| borrow | String | Borrowed amount |
| interest | String | Interest (minimum payment of interest) |
| net | String | Net assets = available + frozen - borrow - interest |
| coupon | String | Trading bonus |
| cTime | String | Creation time (ms) |
| uTime | String | Update time (ms) |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": [
    {
      "coin": "USDT",
      "totalAmount": "12",
      "available": "2",
      "frozen": "0",
      "borrow": "0.1",
      "interest": "0.000001",
      "net": "0.1",
      "cTime": "1734567744432",
      "uTime": "1734567744432",
      "coupon": "0"
    }
  ]
}
```

---

#### Cross Borrow
- **Endpoint**: `POST /api/v2/margin/crossed/account/borrow`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Borrowing coin |
| borrowAmount | String | Yes | Borrowing amount (up to 8 decimal places) |
| clientid | String | No | Custom order ID |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| loanId | String | Loan order ID |
| coin | String | Borrowing coin |
| borrowAmount | String | Borrowing amount |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1679384491703,
  "data": {
    "loanId": "2342332432",
    "coin": "USDT",
    "borrowAmount": "1.00000000"
  }
}
```

---

#### Cross Repay
- **Endpoint**: `POST /api/v2/margin/crossed/account/repay`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Repayment coin |
| repayAmount | String | Yes | Repayment amount (up to 8 decimal places) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| remainDebtAmount | String | Remaining borrowings |
| repayId | String | Repay ID |
| coin | String | Coin |
| repayAmount | String | Repayment amount |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": {
    "coin": "USDT",
    "repayId": "12313123213",
    "remainDebtAmount": "0.2",
    "repayAmount": "0.1"
  }
}
```

---

#### Cross Flash Repay
- **Endpoint**: `POST /api/v2/margin/crossed/account/flash-repay`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Description**: Repays all cross margin debts. If coin not specified, repays all coins.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | No | Repayment coin. If empty, cross margin account will be fully repaid. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| repayId | String | Repayment ID |
| coin | String | Repayment coin. In case of full repayment, returned with no residual value. |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695619576187,
  "data": {
    "repayId": "3423423",
    "coin": "ETH"
  }
}
```

---

#### Get Cross Flash Repay Result
- **Endpoint**: `POST /api/v2/margin/crossed/account/query-flash-repay-status`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| idList | List | Yes | Set of IDs for flash repay requests (max 100 IDs) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| repayId | String | Repayment ID |
| status | String | Repayment result (e.g. `FINISH`) |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1679384491703,
  "data": [
    {
      "repayId": "2342332432",
      "status": "FINISH"
    }
  ]
}
```

---

#### Get Cross Risk Rate
- **Endpoint**: `GET /api/v2/margin/crossed/account/risk-rate`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| riskRateRatio | String | Risk ratio (total assets / total liabilities under cross mode) |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": {
    "riskRateRatio": "0"
  }
}
```

---

#### Get Cross Max Borrowable Amount
- **Endpoint**: `GET /api/v2/margin/crossed/account/max-borrowable-amount`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Borrowing coin, e.g. `BTC` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| maxBorrowableAmount | String | Maximum borrow amount (changes in real time) |
| coin | String | Coin |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": {
    "coin": "USDT",
    "maxBorrowableAmount": "3976070.21616"
  }
}
```

---

#### Get Cross Max Transfer Out Amount
- **Endpoint**: `GET /api/v2/margin/crossed/account/max-transfer-out-amount`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Token name |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| coin | String | Coin |
| maxTransferOutAmount | String | Maximum transferable amount |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": {
    "coin": "USDT",
    "maxTransferOutAmount": "11"
  }
}
```

---

#### Get Cross Interest Rate and Max Borrowable
- **Endpoint**: `GET /api/v2/margin/crossed/interest-rate-and-limit`
- **Rate Limit**: 10 req/1s (IP)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Coin, e.g. `BTC`, `ETH` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| coin | String | Coin |
| leverage | String | Leverage (default 3, tiers: 3, 5, 10) |
| transferable | Boolean | Transfer supported (`true`/`false`) |
| borrowable | Boolean | Borrowable (`true`/`false`) |
| dailyInterestRate | String | Non-VIP daily interest rate |
| annualInterestRate | String | Non-VIP APR |
| maxBorrowableAmount | String | Maximum borrow amount |
| vipList | Array | VIP level details |
| > level | String | VIP level |
| > limit | String | VIP limit |
| > dailyInterestRate | String | VIP daily interest rate |
| > annualInterestRate | String | VIP APR |
| > discountRate | String | VIP discount (1 = 100% = no discount; 0.97 = 97% of original rate) |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695167748916,
  "data": [
    {
      "transferable": true,
      "leverage": "3",
      "coin": "ETH",
      "borrowable": true,
      "dailyInterestRate": "0.0005",
      "annualInterestRate": "0.05",
      "maxBorrowableAmount": "100000",
      "vipList": [
        {
          "level": "0",
          "limit": "1000",
          "dailyInterestRate": "0.00001",
          "annualInterestRate": "0.01",
          "discountRate": "1"
        }
      ]
    }
  ]
}
```

---

#### Get Cross Tier Configuration
- **Endpoint**: `GET /api/v2/margin/crossed/tier-data`
- **Rate Limit**: 10 req/1s (IP)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | Yes | Coin |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| tier | String | Tier level |
| leverage | String | Effective leverage (global default: 3x) |
| coin | String | Coin |
| maxBorrowableAmount | String | Maximum borrow amount |
| maintainMarginRate | String | Maintenance margin rate |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695167748916,
  "data": [
    {
      "tier": "1",
      "leverage": "3",
      "coin": "ETH",
      "maxBorrowableAmount": "6",
      "maintainMarginRate": "0.05"
    }
  ]
}
```

---

### Cross Margin -- Trade

#### Cross Place Order
- **Endpoint**: `POST /api/v2/margin/crossed/place-order`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderType | String | Yes | Order type: `limit` (limit price), `market` (market price) |
| price | String | No | Price (required for limit orders) |
| loanType | String | Yes | Margin order mode: `normal` (normal order), `autoLoan` (auto-borrow), `autoRepay` (auto-repay), `autoLoanAndRepay` (auto-borrow and auto-repay) |
| force | String | Yes | Time in force (invalid for market orders): `gtc` (good till canceled), `post_only`, `fok` (fill or kill), `ioc` (immediate or cancel) |
| baseSize | String | No | Must fill for limit and market sell. Quantity of base currency (left coin). |
| quoteSize | String | No | Must fill for market buy. Quantity of quote currency (right coin). |
| clientOid | String | No | Custom ID. Idempotency time is 6 hours, only valid when orders are unfilled. |
| side | String | Yes | Direction: `sell`, `buy` |
| stpMode | String | No | STP Mode (default `none`): `none`, `cancel_taker`, `cancel_maker`, `cancel_both` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| orderId | String | Order ID |
| clientOid | String | Custom ID |

- **Response Example**:
```json
{
  "code": "00000",
  "data": {
    "orderId": "121211212122",
    "clientOid": "121211212122"
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

---

#### Cross Batch Place Orders
- **Endpoint**: `POST /api/v2/margin/crossed/batch-place-order`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderList | Array | Yes | Array of order objects (same fields as place-order below) |
| > orderType | String | Yes | `limit` or `market` |
| > price | String | No | Price |
| > loanType | String | Yes | `normal`, `autoLoan`, `autoRepay`, `autoLoanAndRepay` |
| > force | String | Yes | `gtc`, `post_only`, `fok`, `ioc` |
| > baseSize | String | No | Quantity of base currency |
| > quoteSize | String | No | Quantity of quote currency |
| > clientOid | String | No | Custom ID |
| > side | String | Yes | `sell` or `buy` |
| > stpMode | String | No | `none`, `cancel_taker`, `cancel_maker`, `cancel_both` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| successList | Array | Successful order array |
| > orderId | String | Order ID |
| > clientOid | String | Client custom ID |
| failureList | Array | Failed order array |
| > clientOid | String | Client custom ID |
| > errorMsg | String | Error information |

- **Response Example**:
```json
{
  "code": "00000",
  "data": {
    "successList": [
      {
        "orderId": "121211212122",
        "clientOid": "121211212122"
      }
    ],
    "failureList": [
      {
        "clientOid": "121211212122",
        "errorMsg": "Order Cancelled"
      }
    ]
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

---

#### Cross Cancel Order
- **Endpoint**: `POST /api/v2/margin/crossed/cancel-order`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderId | String | No | Order ID (either orderId or clientOid required) |
| clientOid | String | No | Client custom ID (either orderId or clientOid required) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| orderId | String | Order ID |
| clientOid | String | Custom ID |

- **Response Example**:
```json
{
  "code": "00000",
  "data": {
    "orderId": "121211212122",
    "clientOid": "BITGET#121211212122"
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

---

#### Cross Batch Cancel Orders
- **Endpoint**: `POST /api/v2/margin/crossed/batch-cancel-order`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderIdList | List | Yes | Order ID list (either orderId or clientOid per entry) |
| > orderId | String | No | Order ID |
| > clientOid | String | No | Client custom ID |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| successList | Array | Successful cancellation array |
| > orderId | String | Order ID |
| > clientOid | String | Client custom ID |
| failureList | Array | Failed cancellation array |
| > orderId | String | Order ID |
| > clientOid | String | Client custom ID |
| > errorMsg | String | Error information |

- **Response Example**:
```json
{
  "code": "00000",
  "data": {
    "successList": [
      {
        "orderId": "121211212122",
        "clientOid": "BITGET#121211212122"
      }
    ],
    "failureList": [
      {
        "orderId": "121211212122",
        "clientOid": "BITGET#121211212122",
        "errorMsg": "Order Cancelled"
      }
    ]
  },
  "msg": "success",
  "requestTime": 1627293504612
}
```

---

#### Get Cross Open Orders
- **Endpoint**: `GET /api/v2/margin/crossed/open-orders`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderId | String | No | Order ID |
| clientOid | String | No | Client custom ID |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp) |
| limit | String | No | Number of results. Default: 100 |
| idLessThan | String | No | Pagination cursor. Pass last orderId from previous query for next page. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| orderList | Array | Array of order objects |
| > symbol | String | Trading pair |
| > orderId | String | Order ID |
| > clientOid | String | Client custom ID |
| > size | String | Filled quantity |
| > priceAvg | String | Average fill price |
| > amount | String | Filled amount |
| > force | String | Time in force: `gtc`, `post_only`, `fok`, `ioc` |
| > price | String | Order price |
| > enterPointSource | String | Order source: `WEB`, `API`, `SYS`, `ANDROID`, `IOS` |
| > status | String | Order status: `live`, `partial_fill`, `filled`, `cancelled`, `reject` |
| > side | String | Direction: `sell`, `buy`, `liquidation-buy`, `liquidation-sell`, `systemRepay-buy`, `systemRepay-sell` |
| > baseSize | String | Quantity of base currency |
| > quoteSize | String | Quantity of quote currency |
| > orderType | String | `limit` or `market` |
| > cTime | String | Creation time (ms) |
| > uTime | String | Updated time (ms) |
| > loanType | String | `normal`, `autoLoan`, `autoRepay`, `autoLoanAndRepay` |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636829522,
  "data": {
    "orderList": [
      {
        "orderId": "121211212122",
        "symbol": "BTCUSDT",
        "orderType": "limit",
        "enterPointSource": "API",
        "clientOid": "myClientOid001",
        "loanType": "normal",
        "price": "32111",
        "side": "buy",
        "status": "live",
        "baseSize": "0.01",
        "quoteSize": "1000",
        "priceAvg": "32111",
        "size": "0.01",
        "amount": "1000",
        "force": "gtc",
        "cTime": "1695629859821",
        "uTime": "1695629890839"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

---

#### Get Cross History Orders
- **Endpoint**: `GET /api/v2/margin/crossed/history-orders`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderId | String | No | Order ID |
| enterPointSource | String | No | Order source: `WEB`, `API`, `SYS`, `ANDROID`, `IOS` |
| clientOid | String | No | Client custom ID |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last endId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| orderList | Array | Array of order objects |
| > symbol | String | Trading pair |
| > orderId | String | Order ID |
| > clientOid | String | Client custom ID |
| > size | String | Filled quantity |
| > priceAvg | String | Average fill price |
| > amount | String | Filled volume |
| > force | String | Time in force: `gtc`, `post_only`, `fok`, `ioc` |
| > price | String | Order price |
| > enterPointSource | String | Order source: `WEB`, `API`, `SYS`, `ANDROID`, `IOS` |
| > status | String | Status: `live`, `partially_fill`, `filled`, `cancelled`, `reject` |
| > side | String | Direction: `sell`, `buy`, `liquidation-buy`, `liquidation-sell`, `systemRepay-buy`, `systemRepay-sell` |
| > baseSize | String | Quantity of base currency |
| > quoteSize | String | Quantity of quote currency |
| > orderType | String | `limit` or `market` |
| > cTime | String | Creation time (ms) |
| > uTime | String | Updated time (ms) |
| > loanType | String | `normal`, `autoLoan`, `autoRepay`, `autoLoanAndRepay` |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636829522,
  "data": {
    "orderList": [
      {
        "orderId": "121211212122",
        "symbol": "BTCUSDT",
        "orderType": "limit",
        "enterPointSource": "API",
        "clientOid": "myClientOid001",
        "loanType": "normal",
        "price": "32111",
        "side": "buy",
        "status": "filled",
        "baseSize": "0.01",
        "quoteSize": "1000",
        "priceAvg": "32111",
        "size": "0.01",
        "amount": "1000",
        "force": "gtc",
        "cTime": "1695629859821",
        "uTime": "1695629890839"
      }
    ],
    "maxId": "121211212122",
    "minId": "121211212122"
  }
}
```

---

#### Get Cross Order Fills
- **Endpoint**: `GET /api/v2/margin/crossed/fills`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | String | Yes | Trading pair, e.g. `BTCUSDT` |
| orderId | String | No | Order ID |
| idLessThan | String | No | Pagination cursor. Pass last fillId from previous query. |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| fills | Array | Array of fill objects |
| > symbol | String | Trading pair |
| > orderId | String | Order ID |
| > tradeId | String | Transaction detail ID |
| > size | String | Filled quantity |
| > priceAvg | String | Fill price |
| > amount | String | Filled amount |
| > side | String | Direction: `sell`, `buy`, `liquidation_buy`, `liquidation_sell`, `system_repay_buy`, `system_repay_sell` |
| > orderType | String | `limit` or `market` |
| > tradeScope | String | `taker` or `maker` |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| > feeDetail | Object | Transaction fee details |
| >> deduction | String | Discount applied (`yes`/`no`) |
| >> feeCoin | String | Fee coin |
| >> totalDeductionFee | String | Total discounted transaction fee |
| >> totalFee | String | Total transaction fee |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636829522,
  "data": {
    "fills": [
      {
        "orderId": "121211212122",
        "tradeId": "121211212122",
        "orderType": "limit",
        "side": "buy",
        "priceAvg": "32111",
        "size": "0.01",
        "amount": "1000",
        "tradeScope": "taker",
        "cTime": "1695629859821",
        "uTime": "1695629890839",
        "feeDetail": {
          "deduction": "yes",
          "feeCoin": "BGB",
          "totalDeductionFee": "-0.017118519726",
          "totalFee": "-0.017118519726"
        }
      }
    ],
    "maxId": "121211212122",
    "minId": "121211212122"
  }
}
```

---

#### Get Cross Liquidation Orders
- **Endpoint**: `GET /api/v2/margin/crossed/liquidation-order`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| type | String | No | Type: `swap` (swap), `place_order` (default) |
| symbol | String | No | Trading pair, e.g. `BTCUSDT`. Only effective when type=`place_order`. Default: all symbols |
| fromCoin | String | No | Swap from coin. Only effective when type=`swap` |
| toCoin | String | No | Swap to coin. Only effective when type=`swap` |
| startTime | String | No | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last endId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of liquidation records |
| > orderId | String | Order ID |
| > symbol | String | Trading pair (only when type=`place_order`) |
| > orderType | String | Order type: `market` (only when type=`place_order`) |
| > side | String | `liquidation_sell` or `liquidation_buy` (only when type=`place_order`) |
| > priceAvg | String | Filled price (only when type=`place_order`) |
| > price | String | Order price (only when type=`place_order`) |
| > fillSize | String | Filled quantity (only when type=`place_order`) |
| > size | String | Order quantity (only when type=`place_order`) |
| > amount | String | Filled amount (only when type=`place_order`) |
| > fromCoin | String | Source currency (only when type=`swap`) |
| > fromSize | String | Source currency size (only when type=`swap`) |
| > toCoin | String | Target currency (only when type=`swap`) |
| > toSize | String | Target currency size (only when type=`swap`) |
| > cTime | String | Creation time (ms) |
| > uTime | String | Updated time (ms) |
| idLessThan | String | Pagination cursor for next page |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1708654712083,
  "data": {
    "resultList": [
      {
        "symbol": "BTCUSDT",
        "orderType": "market",
        "side": "liquidation-sell",
        "priceAvg": "43024.762472",
        "price": "43024.762472",
        "fillSize": "1",
        "size": "1",
        "amount": "43024.762472",
        "orderId": "xxxx",
        "fromCoin": "",
        "toCoin": "",
        "fromSize": "",
        "toSize": "",
        "cTime": "1705474028200",
        "uTime": "1705474028398"
      }
    ],
    "idLessThan": "1131405566368010241"
  }
}
```

---

### Cross Margin -- Order Records

#### Get Cross Borrow History
- **Endpoint**: `GET /api/v2/margin/crossed/borrow-history`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| loanId | String | No | Borrowing ID (exact match) |
| coin | String | No | Coin |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last loanId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of borrow records |
| > loanId | String | Loan ID |
| > coin | String | Borrowed coin |
| > borrowAmount | String | Borrowing amount |
| > borrowType | String | Type: `auto_loan` (auto borrow), `manual_loan` (manual borrow) |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636829522,
  "data": {
    "resultList": [
      {
        "loanId": "1",
        "coin": "USDT",
        "borrowAmount": "12.1",
        "borrowType": "manual_loan",
        "cTime": "1695629859821",
        "uTime": "1695629890839"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

---

#### Get Cross Repay History
- **Endpoint**: `GET /api/v2/margin/crossed/repay-history`
- **Rate Limit**: 10 req/1s (IP)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| repayId | String | No | Repayment ID |
| coin | String | No | Coin |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last repayId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of repay records |
| > repayId | String | Repayment ID |
| > coin | String | Repayment coin |
| > repayPrincipal | String | Repayment principal |
| > repayAmount | String | Total repayment amount |
| > repayInterest | String | Repayment interest |
| > repayType | String | Type: `auto_repay`, `manual_repay`, `liq_repay` (liquidation), `force_repay` (compulsory) |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636829522,
  "data": {
    "resultList": [
      {
        "repayId": "1",
        "coin": "USDT",
        "repayAmount": "12.1",
        "repayType": "manual_repay",
        "repayInterest": "0.0001",
        "repayPrincipal": "0.1",
        "cTime": "1695629859821",
        "uTime": "1695629890839"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

---

#### Get Cross Interest History
- **Endpoint**: `GET /api/v2/margin/crossed/interest-history`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | String | No | Coin |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last loanId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of interest records |
| > interestId | String | Interest record ID |
| > interestAmount | String | Interest amount |
| > dailyInterestRate | String | Daily interest rate |
| > interstType | String | Interest type: `first` (initial borrowing), `scheduled` (scheduled interest) |
| > interestCoin | String | Interest coin |
| > loanCoin | String | Borrowing coin |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1668134626684,
  "data": {
    "minId": "1",
    "maxId": "1",
    "resultList": [
      {
        "interestId": "1",
        "loanCoin": "USDT",
        "interestCoin": "USDT",
        "dailyInterestRate": "0.00001",
        "interestAmount": "1.2",
        "interstType": "first",
        "uTime": "1668134458717",
        "cTime": "1668134458717"
      }
    ]
  }
}
```

---

#### Get Cross Liquidation History
- **Endpoint**: `GET /api/v2/margin/crossed/liquidation-history`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last liqId from previous query. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of liquidation records |
| > liqId | String | Liquidation ID |
| > liqStartTime | String | Liquidation start time (ms) |
| > liqEndTime | String | Liquidation end time (ms) |
| > liqRiskRatio | String | Risk ratio at liquidation |
| > totalAssets | String | Total assets at liquidation (USDT) |
| > totalDebt | String | Total debt at liquidation (USDT) |
| > liqFee | String | Liquidation transaction fees |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1668134626684,
  "data": {
    "minId": "1",
    "maxId": "1",
    "resultList": [
      {
        "liqId": "1",
        "liqStartTime": "1356756456873",
        "liqEndTime": "1356756456873",
        "liqRiskRatio": "0.1",
        "totalAssets": "154",
        "totalDebt": "123",
        "liqFee": "31.1",
        "uTime": "1668134458717",
        "cTime": "1668134458717"
      }
    ]
  }
}
```

---

#### Get Cross Financial History
- **Endpoint**: `GET /api/v2/margin/crossed/financial-records`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| marginType | String | No | Capital flow type (see enum values below) |
| coin | String | No | Coin |
| startTime | String | Yes | Start time (Unix ms timestamp) |
| endTime | String | No | End time (Unix ms timestamp). Max interval: 90 days |
| limit | String | No | Number of results. Default: 100, max: 500 |
| idLessThan | String | No | Pagination cursor. Pass last marginId from previous query. |

**marginType enum values**:
| Value | Description |
|---|---|
| `transfer_in` | Assets transferred in |
| `transfer_out` | Assets transferred out |
| `borrow` | Borrow |
| `repay` | Repay |
| `liquidation_fee` | Liquidation fee |
| `compensate` | Collateral shortfall compensation from risk fund |
| `deal_in` | Trade and deposit (buy) |
| `deal_out` | Trade and withdraw (sell) |
| `confiscated` | Deduction for collateral shortfall |
| `exchange_in` | Exchange income (from system account) |
| `exchange_out` | Exchange expense (to system account) |
| `sys_exchange_in` | System account exchange income |
| `sys_exchange_out` | System account exchange expense |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| resultList | Array | Array of financial records |
| > coin | String | Coin |
| > marginId | String | Capital flow ID |
| > marginType | String | Capital flow type (same enum as request) |
| > amount | String | Capital flow amount |
| > balance | String | Account balance |
| > fee | String | Transaction fee |
| > cTime | String | Creation time (ms) |
| > uTime | String | Update time (ms) |
| maxId | String | Max ID in result set |
| minId | String | Min ID in result set |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1668134626684,
  "data": {
    "minId": "1",
    "maxId": "1",
    "resultList": [
      {
        "marginId": "1",
        "amount": "10.12",
        "coin": "USDT",
        "balance": "156",
        "fee": "0",
        "marginType": "transfer_in",
        "uTime": "1668134458717",
        "cTime": "1668134458717"
      }
    ]
  }
}
```

---

### Spot Wallet -- Transfer (for Margin Transfers)

#### Wallet Transfer
- **Endpoint**: `POST /api/v2/spot/wallet/transfer`
- **Rate Limit**: 10 req/1s (UID)
- **Auth**: Yes (signature required)
- **Description**: Transfer assets between accounts. Use `crossed_margin` as fromType/toType for cross margin transfers.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| fromType | String | Yes | Source account type: `spot`, `p2p`, `coin_futures`, `usdt_futures`, `usdc_futures`, `crossed_margin`, `isolated_margin` |
| toType | String | Yes | Destination account type: `spot`, `p2p`, `coin_futures`, `usdt_futures`, `usdc_futures`, `crossed_margin`, `isolated_margin` |
| amount | String | Yes | Amount to transfer |
| coin | String | Yes | Currency of transfer |
| symbol | String | Yes | Required when transferring to/from isolated margin account |
| clientOid | String | No | Custom order ID. Must be unique. Duplicate clientOid returns existing transfer result. |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| transferId | String | Transfer ID |
| clientOid | String | Custom order ID |

- **Response Example**:
```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1683875302853,
  "data": {
    "transferId": "123456",
    "clientOid": "x123"
  }
}
```

---

## Endpoint Quick Reference

| Category | Method | Endpoint | Rate Limit |
|---|---|---|---|
| **Account** | GET | `/api/v2/margin/crossed/account/assets` | 10/s (UID) |
| **Account** | POST | `/api/v2/margin/crossed/account/borrow` | 10/s (UID) |
| **Account** | POST | `/api/v2/margin/crossed/account/repay` | 10/s (UID) |
| **Account** | POST | `/api/v2/margin/crossed/account/flash-repay` | 10/s (UID) |
| **Account** | POST | `/api/v2/margin/crossed/account/query-flash-repay-status` | 10/s (UID) |
| **Account** | GET | `/api/v2/margin/crossed/account/risk-rate` | 10/s (UID) |
| **Account** | GET | `/api/v2/margin/crossed/account/max-borrowable-amount` | 10/s (UID) |
| **Account** | GET | `/api/v2/margin/crossed/account/max-transfer-out-amount` | 10/s (UID) |
| **Account** | GET | `/api/v2/margin/crossed/interest-rate-and-limit` | 10/s (IP) |
| **Account** | GET | `/api/v2/margin/crossed/tier-data` | 10/s (IP) |
| **Trade** | POST | `/api/v2/margin/crossed/place-order` | 10/s (UID) |
| **Trade** | POST | `/api/v2/margin/crossed/batch-place-order` | 10/s (UID) |
| **Trade** | POST | `/api/v2/margin/crossed/cancel-order` | 10/s (UID) |
| **Trade** | POST | `/api/v2/margin/crossed/batch-cancel-order` | 10/s (UID) |
| **Trade** | GET | `/api/v2/margin/crossed/open-orders` | 10/s (UID) |
| **Trade** | GET | `/api/v2/margin/crossed/history-orders` | 10/s (UID) |
| **Trade** | GET | `/api/v2/margin/crossed/fills` | 10/s (UID) |
| **Trade** | GET | `/api/v2/margin/crossed/liquidation-order` | 10/s (UID) |
| **Records** | GET | `/api/v2/margin/crossed/borrow-history` | 10/s (UID) |
| **Records** | GET | `/api/v2/margin/crossed/repay-history` | 10/s (IP) |
| **Records** | GET | `/api/v2/margin/crossed/interest-history` | 10/s (UID) |
| **Records** | GET | `/api/v2/margin/crossed/liquidation-history` | 10/s (UID) |
| **Records** | GET | `/api/v2/margin/crossed/financial-records` | 10/s (UID) |
| **Common** | GET | `/api/v2/margin/currencies` | 10/s (IP) |
| **Transfer** | POST | `/api/v2/spot/wallet/transfer` | 10/s (UID) |

## Key Enums

### loanType (Margin Order Mode)
| Value | Description |
|---|---|
| `normal` | Normal order (no auto borrow/repay) |
| `autoLoan` | Auto-borrow on order placement |
| `autoRepay` | Auto-repay on order fill |
| `autoLoanAndRepay` | Auto-borrow and auto-repay |

### orderType
| Value | Description |
|---|---|
| `limit` | Limit price order |
| `market` | Market price order |

### force (Time in Force)
| Value | Description |
|---|---|
| `gtc` | Good till canceled |
| `post_only` | Post only (maker only) |
| `fok` | Fill or kill |
| `ioc` | Immediate or cancel |

### side
| Value | Description |
|---|---|
| `buy` | Buy |
| `sell` | Sell |
| `liquidation-buy` | Settlement buy |
| `liquidation-sell` | Settlement sell |
| `systemRepay-buy` | System repay buy |
| `systemRepay-sell` | System repay sell |

### Order Status
| Value | Description |
|---|---|
| `live` | New order |
| `partial_fill` | Partially filled |
| `filled` | Fully filled |
| `cancelled` | Cancelled |
| `reject` | Rejected |

### borrowType
| Value | Description |
|---|---|
| `auto_loan` | Auto borrow |
| `manual_loan` | Manual borrow |

### repayType
| Value | Description |
|---|---|
| `auto_repay` | Automatic repayment |
| `manual_repay` | Manual repayment |
| `liq_repay` | Liquidation repay |
| `force_repay` | Compulsory repayment |

### enterPointSource (Order Source)
| Value | Description |
|---|---|
| `WEB` | Website |
| `API` | API |
| `SYS` | System (usually liquidation) |
| `ANDROID` | Android app |
| `IOS` | iOS app |

### stpMode (Self-Trade Prevention)
| Value | Description |
|---|---|
| `none` | No STP (default) |
| `cancel_taker` | Cancel taker order |
| `cancel_maker` | Cancel maker order |
| `cancel_both` | Cancel both orders |

## Response Format

All responses follow this structure:

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": { ... }
}
```

- `code`: `"00000"` indicates success. Any other value is an error.
- `msg`: Human-readable message.
- `requestTime`: Server timestamp (ms).
- `data`: Response payload (object or array depending on endpoint).

## Pagination

Most list endpoints use cursor-based pagination:

1. First request: omit `idLessThan`
2. Subsequent requests: pass the last ID from previous response as `idLessThan`
3. Default limit: 100 (most endpoints support max 500)
4. Time range: most endpoints support max 90-day interval between `startTime` and `endTime`
