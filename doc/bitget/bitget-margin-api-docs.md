# Bitget Margin Trading API Documentation

Crawled from https://www.bitget.com/api-doc/margin/

---

# Common

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Support Currencies

Frequency limit:10 times/1s (IP)

### Description

### HTTP Request

- GET /api/v2/margin/currencies

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/currencies"
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

Response Example

```json
{
    "code": "00000",
    "msg": "success",
    "requestTime": 1679383565084,
    "data": [
        {
            "symbol": "ETHUSDT",
            "baseCoin": "ETH",
            "quoteCoin": "USDT",
            "maxCrossedLeverage": "3",
            "maxIsolatedLeverage": "10",
            "warningRiskRatio": "0.80000000",
            "liquidationRiskRatio": "1.00000000",
            "minTradeAmount": "0.00010000",
            "maxTradeAmount": "10000.00000000",
            "takerFeeRate": "0.00100000",
            "makerFeeRate": "0.00100000",
            "pricePrecision": "4",
            "quantityPrecision": "4",
            "minTradeUSDT": "5.00000000",
            "isBorrowable": true,
            "userMinBorrow": "0.00000001",
            "status": "1",
            "isIsolatedBaseBorrowable": true,
            "isIsolatedQuoteBorrowable": true,
            "isCrossBorrowable": true
        }
    ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| baseCoin | String | Base currency |
| quoteCoin | String | Quote currency |
| maxCrossedLeverage | String | Cross margin maximum leverage multiples |
| maxIsolatedLeverage | String | Cross margin maximum leverage multiples |
| warningRiskRatio | String | Warning risk ratio |
| liquidationRiskRatio | String | Liquidation risk ratio |
| minTradeAmount | String | Minimum trading volume |
| maxTradeAmount | String | Maximum trading volume |
| takerFeeRate | String | Taker rates |
| makerFeeRate | String | Maker rates |
| pricePrecision | String | Pricing precision |
| quantityPrecision | String | Amount precision |
| minTradeUSDT | String | Minimum trading volume (USDT) |
| ~~isBorrowable~~ | String | Borrowable or not(ignore) |
| userMinBorrow | String | Minimum borrowing |
| status | String | 1: tradable 2: under temporary maintenance |
| isIsolatedBaseBorrowable | String | isolate base coin borrowable or not? |
| isIsolatedQuoteBorrowable | String | isolate quote coin borrowable or not? |
| isCrossBorrowable | String | corss borrowable or not? |

---

# Cross Margin — Account

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Borrow

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/account/borrow

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/account/borrow"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"coin": "USDT","borrowAmount": "1"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Borrowing coin |
| borrowAmount | String | Yes | Borrowing amount (up to 8 decimal places) |
| clientid | String | No | 客户自定义订单ID |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanId | String | Loan order ID |
| coin | String | Borrowing coin |
| borrowAmount | String | Borrowing amount |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Flash Repay

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/account/flash-repay

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/account/flash-repay" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '{"coin": "BTC"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | No | Repayment coin for the cross margin<br>If you don't fill it, then cross margin account will be fully repaid. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695619576187,
  "data":
    {
      "repayId": "3423423",
      "coin": "ETH"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| repayId | String | Repayment id |
| coin | String | Repayment coin. In case of full repayment, the coin will be returned with no residual value. |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Max Borrowable

Frequency limit: 10 times/1s (UID)

### Description

Get max borrowable

### HTTP Request

- GET /api/v2/margin/crossed/account/max-borrowable-amount

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/account/max-borrowable-amount?coin=USDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Borrowing coins, such as BTC |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| maxBorrowableAmount | String | Maximum borrow amount (amount changes in real time) |
| coin | String | Coin |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Repay

Frequency limit: 10 times/1s (UID)

### Description

Repay

### HTTP Request

- POST /api/v2/margin/crossed/account/repay

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/account/repay" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '{"coin": "USDT", "repayAmount":"0.1"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Repayment coin |
| repayAmount | String | Yes | Number of repayments (up to 8 decimal places) |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| remainDebtAmount | String | Remaining borrowings |
| repayId | String | repay ID |
| coin | String | Coin |
| repayAmount | String | Repayment amount |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Tier Configuration

Frequency limit:10 times/1s (IP)

### Description

This interface will determine the user's VIP level based on the User ID sending the request, and then return the tier information based on the VIP level.

### HTTP Request

- GET /api/v2/margin/crossed/tier-data

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/tier-data?coin=ETH"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Coin |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| tier | String | Tier |
| leverage | String | Effective leverage, global default: 3x |
| coin | String | Coin |
| maxBorrowableAmount | String | Maximum borrow |
| maintainMarginRate | String | Maintenance margin rate |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Account Assets

Frequency limit: 10 times/1s (UID)

### Description

Getting account asset

### HTTP Request

- GET /api/v2/margin/crossed/account/assets

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/account/assets?coin=USDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | No | Coin, like USDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": [{
    "coin": "USDT",
    "totalAmount": "12",
    "available": "2",
    "frozen": "0",
    "borrow": "0.1",
    "interest": "0.000001",
    "net": "0.1",
    "cTime":"1734567744432",
    "uTime":"1734567744432",
    "coupon": "0"
  }]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| coin | String | Token name |
| totalAmount | String | Total amount |
| available | String | Available amount |
| frozen | String | Assets frozen |
| borrow | String | Borrow |
| interest | String | Interest, Interest-only payments with a minimum payment of interest. |
| net | String | Net assets = available + frozen − borrow − interest. Liquidation is triggered when the Maintenance Margin Ratio (MMR) is reached. |
| coupon | String | Trading bonus |
| cTime | String | Creation time |
| uTime | String | Update time |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Interest Rate and Max Borrowable

Frequency limit:10 times/1s (IP)

### Description

This interface will determine the user's VIP level based on the User ID sending the request, and then return information such as interest rates and limits based on the VIP level.

### HTTP Request

- GET /api/v2/margin/crossed/interest-rate-and-limit

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/interest-rate-and-limit?coin=ETH"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Trading pairs, like BTC, ETH |

Response Example

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
                    "level":"0",
                    "limit":"1000",
                    "dailyInterestRate":"0.00001",
                    "annualInterestRate":"0.01",
                    "discountRate":"1"
                }
            ]
        }
    ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| coin | String | Coins, such as BTC, ETH |
| leverage | String | Leverage <br>The default value is 3, and it has three tiers: 3, 5, and 10. |
| transferable | Boolean | Transfer supported?<br>true: Transferable<br>false: Not transferable |
| borrowable | Boolean | Borrowable or not?<br>true: Borrowable<br>false: Not borrowable |
| dailyInterestRate | String | Non-VIP daily interest rate |
| annualInterestRate | String | Non-VIP APR |
| maxBorrowableAmount | String | Maximum borrow |
| vipList | Array | VIP level |
| > level | String | VIP level |
| > limit | String | VIP limit |
| > dailyInterestRate | String | VIP daily interest rate |
| > annualInterestRate | String | VIP APR |
| > discountRate | String | VIP discount: 1 is for 100%, i.e. no discount; 0.97 is for 97% of the original rate |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Max Transferable

Frequency limit: 10 times/1s (UID)

### Description

Get max transferable

### HTTP Request

- GET /api/v2/margin/crossed/account/max-transfer-out-amount

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/account/max-transfer-out-amount?coin=USDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | Yes | Token name |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| coin | String | Coin |
| maxTransferOutAmount | String | Maximum transferable amount |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Risk Rate

Frequency limit: 10 times/1s (UID)

### Description

Get cross risk rate

### HTTP Request

- GET /api/v2/margin/crossed/account/risk-rate

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/account/risk-rate" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| N/A |  |  |  |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| riskRate | String | Risk ratio (total assets/total liabilities under cross mode) |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Flash Repay Result

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/account/query-flash-repay-status

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/account/query-flash-repay-status" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '{"idList": ["13423423"]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| idList | List | Yes | Set of ids for close position requests (Max. 100 ids) |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1679384491703,
  "data": [{
    "repayId": "2342332432",
    "status": "FINISH"
  }]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| repayId | String | Repayment ID |
| status | String | Repayment result |

---

# Cross Margin — Trade

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Batch Cancel Orders

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/batch-cancel-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/batch-cancel-order"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"symbol": "BTCUSDT","orderIdList": [{"orderId":"11232132134"},{"clientOid":"mytestOid"}]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderIdList | List | yes | Order ID list <br>Either orderId or clientOid |
| > orderId | String | No | Order ID <br>Either orderId or clientOid |
| > clientOid | String | No | Client customized ID <br>Either orderId or clientOid |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| successList | Array | Successful order array |
| > orderId | String | Order ID |
| > clientOid | String | Client customized ID |
| failureList | Array | Failed order array |
| > orderId | String | Order ID |
| > clientOid | String | Client customized ID |
| > errorMsg | String | Error information |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Batch Orders

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/batch-place-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/batch-place-order"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"symbol": "BTCUSDT","orderList": [{"side":"buy", "orderType":"market", "force":"gtc", "quoteSize":"10000", "loanType":"normal"}]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderList | Array | Yes | Order entries |
| orderType | String | Yes | Order type<br>limit: limit price<br>market: market price |
| price | String | No | Price |
| loanType | String | Yes | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |
| force | String | Yes | Time in force (invalid when `orderType` is `market`)<br>gtc: normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| baseSize | String | No | Must fill limit and market sell. Sell order presents quantity of based currency (the left coin). |
| quoteSize | String | No | Must fill market buy. Buy order presents quantity of quote currency (the right coin). |
| clientOid | String | No | Customized ID |
| side | String | Yes | Direction<br>sell: Sell<br>buy: Buy |
| stpMode | String | No | STP Mode, default `none`<br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| successList | Array | Successful order array |
| > orderId | String | Order ID |
| > clientOid | String | Client customized ID |
| failureList | Array | Failed order array |
| > clientOid | String | Client customized ID |
| > errorMsg | String | Error information |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Cancel Order

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/crossed/cancel-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/cancel-order"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"symbol": "BTCUSDT","orderId": "12234234321432"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderId | String | No | Order ID<br>Either orderId or clientOid |
| clientOid | String | No | Client customized ID<br>Either orderId or clientOid |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| clientOid | String | Customized ID |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross Place Order

Rate Limit: 10 req/sec/UID

### Description

### HTTP Request

- POST /api/v2/margin/crossed/place-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/crossed/place-order"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"symbol": "BTCUSDT", "side":"buy", "orderType":"market", "force":"gtc", "quoteSize":"10000", "loanType":"normal"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderType | String | Yes | Order type<br>limit: limit price<br>market: market price |
| price | String | No | Price |
| loanType | String | Yes | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |
| force | String | Yes | Time in force (invalid when `orderType` is `market`)<br>gtc: Normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| baseSize | String | No | Must fill limit and market sell. Sell order presents quantity of based currency (the left coin). |
| quoteSize | String | No | Must fill market buy. Buy order presents quantity of quote currency (the right coin). |
| clientOid | String | No | Customized ID<br>The idempotency time is 6 hours, only valid when orders are unfilled. |
| side | String | Yes | Direction<br>sell: Sell<br>buy: Buy |
| stpMode | String | No | STP Mode, default `none`<br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| clientOid | String | Customized ID |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Current Orders

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/open-orders

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/open-orders?symbol=BTCUSDT&limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderId | String | No | Order ID |
| clientOid | String | No | Client customized ID |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp |
| limit | String | No | Number of quiries<br>Default: 100 entries |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last loanId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| orderId | String | Order no. |
| clientOid | String | Client customized ID |
| size | String | Filled quantity |
| priceAvg | String | Order price |
| amount | String | Filled quantity |
| force | String | Order bot<br>gtc: normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| price | String | Order price |
| enterPointSource | String | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| status | String | Order status<br>`live`: New order<br>`partial_fill`: Partially filled<br>`filled`: Fully filled<br>`cancelled`: The order is cancelled<br>`reject`: Rejected |
| side | String | Direction<br>`sell`: Sell<br>`buy`: Buy<br>`liquidation-buy`: Settlement – buy<br>`liquidation-sell`: Settlement – sell<br>`systemRepay-buy`: Repay by system – buy<br>`systemRepay-selll`: Repay by system – sell |
| baseSize | String | Quantity of the base currency |
| quoteSize | String | Quantity of the quote currency |
| orderType | String | Order type<br>limit: limit price<br>market: market price |
| cTime | String | Creation time |
| uTime | String | Updated time |
| loanType | String | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Order Fills

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/fills

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/fills?symbol=BTCUSDT&limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderId | String | No | Order ID |
| idLessThan | String | No | Match order ID, relative parameters of turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last fillId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| orderId | String | Order no. |
| tradeId | String | Transaction detail ID |
| size | String | Filled quantity |
| priceAvg | String | Order price |
| amount | String | Filled quantity |
| side | String | Direction<br>sell: Sell<br>buy: Buy<br>liquidation\_buy: Settlement – buy<br>liquidation\_sell: Settlement – sell<br>system\_repay\_buy: Repay by system – buy<br>system\_repay\_sell: Repay by system – sell |
| orderType | String | Order type<br>limit: limit price<br>market: market price |
| tradeScope | String | Trader tag<br>taker: Taker<br>maker: Maker |
| cTime | String | Creation time, millisecond timestamp |
| uTime | String | Update time, millisecond timestamp |
| feeDetail | Array | Transaction fee details |
| > deduction | String | Discount or not |
| > feecoinCode | String | Coin for transaction fee |
| > totalDeductionFee | String | Total discounted transaction fee |
| > totalFee | String | Total transaction fee |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross History Orders

Rate limit:10 req/sec/UID

### Description

### HTTP Request

- GET /api/v2/margin/crossed/history-orders

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/history-orders?symbol=BTCUSDT&limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |
| orderId | String | No | Order ID |
| enterPointSource | String | No | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| clientOid | String | No | Client customized ID |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages<br>It's not needed in the first query. When querying data in the second page and the data beyond, the last endId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| orderId | String | Order ID |
| clientOid | String | Client customized ID |
| size | String | Filled quantity |
| priceAvg | String | Order price |
| amount | String | Filled volume |
| force | String | Time in force<br>`gtc`: normal limit order, good till canceled<br>`post_only`: Post only<br>`fok`: Fill or kill<br>`ioc`: Immediate or cancel |
| price | String | Order price |
| enterPointSource | String | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| status | String | Order status<br>`live`: New order<br>`partially_fill`: Partially filled<br>`filled`: Fully filled<br>`cancelled`: Cancelled<br>`reject`: Rejected |
| side | String | Direction<br>`sell`: Sell<br>`buy`: Buy<br>`liquidation-buy`: Settlement – buy<br>`liquidation-sell`: Settlement – sell<br>`systemRepay-buy`: Repay by system – buy<br>`systemRepay-selll`: Repay by system – sell |
| baseSize | String | Quantity of the base currency |
| quoteSize | String | Quantity of the quote currency |
| orderType | String | Order type<br>limit: limit price<br>market: market price |
| cTime | String | Creation time |
| uTime | String | Updated time |
| loanType | String | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |

---

# Cross Margin — Records

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Financial History

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/financial-records

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/financial-record?coin=USDT&limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| marginType | String | No | Capital flow type<br>transfer\_in: assets transferred in<br>transfer\_out: assets transferred out<br>borrow: borrow<br>repay: repay<br>liquidation\_fee: liquidation fee<br>compensate: collateral shortfall compensation from risk fund<br>deal\_in: trade and deposit (buy)<br>deal\_out: trade and withdraw (sell)<br>confiscated: deduction for collateral shortfall<br>exchange\_in: exchange income, charged from the system account<br>exchange\_out: exchange expense, charged to the system account<br>sys\_exchange\_in: exchange income of the system account, with exchange expense appearing at the same time<br>sys\_exchange\_out: exchange expense of the system account, with exchange income appearing at the same time |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last marginId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| coin | String | Coin |
| marginId | String | Capital flow ID |
| marginType | String | Capital flow type<br>transfer\_in: assets transferred in<br>transfer\_out: assets transferred out<br>borrow: borrow<br>repay: repay<br>liquidation\_fee: liquidation fee<br>compensate: collateral shortfall compensation from risk fund<br>deal\_in: trade and deposit (buy)<br>deal\_out: trade and withdraw (sell)<br>confiscated: deduction for collateral shortfall<br>exchange\_in: exchange income, charged from the system account<br>exchange\_out: exchange expense, charged to the system account<br>sys\_exchange\_in: exchange income of the system account, with exchange expense appearing at the same time<br>sys\_exchange\_out: exchange expense of the system account, with exchange income appearing at the same time |
| amount | String | Capital flow amount |
| balance | String | Account balance |
| fee | String | Transaction fee details |
| cTime | String | Creation time, millisecond timestamp |
| uTime | String | Update time, millisecond timestamp |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Interest History

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/interest-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/interest-history?coin=USDT&limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last loanId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| interestId | String | ID |
| interestAmount | String | Interest amount |
| dailyInterestRate | String | Daily interest rate |
| interstType | String | Interest type<br>first interest on initial borrowing<br>scheduled: scheduled interest |
| interestCoin | String | Interest coin |
| loanCoin | String | Borrowing coin |
| cTime | String | Creation time |
| uTime | String | Update time, millisecond timestamp |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Liquidation History

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/liquidation-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/liquidation-history?limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last liqId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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
                "liqFee":"31.1",
                "uTime": "1668134458717",
                "cTime": "1668134458717"
            }
        ]
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| liqId | String | Liquidation ID |
| liqStartTime | String | Liquidation start time |
| liqEndTime | String | Liquidation end time |
| liqRiskRatio | String | Risk ratio in liquidation |
| totalAssets | String | Total assets in liquidation<br>In USDT |
| totalDebt | String | Total debt in liquidation<br>In USDT |
| liqFee | String | Liquidation transaction fees |
| cTime | String | Creation time, millisecond timestamp |
| uTime | String | Update time, millisecond timestamp |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Borrow History

Frequency limit:10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/borrow-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/borrow-history?limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| loanId | String | No | Borrowing ID (exact match of single item) |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last loanId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanId | String | ID |
| coin | String | Borrowed coin |
| borrowAmount | String | Borrowing amount |
| borrowType | String | Type of borrowing<br>`auto_loan`: auto borrow<br>`manual_loan`: manual borrow |
| cTime | String | Creation time, millisecond timestamp |
| uTime | String | Update time, millisecond timestamp |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Cross Repay History

Frequency limit:10 times/1s (IP)

### Description

### HTTP Request

- GET /api/v2/margin/crossed/repay-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/crossed/repay-history?limit=20&startTime=1693205171000&endTime=1694155571000" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| repayId | String | No | Repayment ID |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamp |
| endTime | String | No | End time, Unix millisecond timestamp<br>Maximum interval between start time and end time is 90 days |
| limit | String | No | Number of quiries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. The first query is not passed. When querying data in the second page and the data beyond, the last repayId returned in the last query is used, and the result will return data with a value less than this one; the query response time will be shortened. |

Response Example

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

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| repayId | String | Repayment ID |
| coin | String | Repayment coin |
| repayPrincipal | String | Repayment principal |
| repayAmount | String | Total repayment |
| repayInterest | String | Repayment interest |
| repayType | String | Repayment type<br>auto\_repay automatic repayment<br>manual\_repay manual repayment<br>liq\_repay liquidate and repay<br>force\_repay compulsory repayment |
| cTime | String | Creation time, millisecond timestamp |
| uTime | String | Update time, millisecond timestamp |

---

# Cross Margin — WebSocket Public

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Margin Index Price

### Description

Subscribe margin trading pair index price

Request Example

```bash
{
    "args":[
        {
            "channel":"index-price",
            "instType":"MARGIN",
            "instId":"default"
        }
    ],
    "op":"subscribe"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| op | String | Yes | Operation<br>subscribe Subscribe<br>unsubscribe Unsubscribe |
| args | List<Object> | Yes | List of channels to request subscription |
| > instType | String | Yes | Product type: MARGIN |
| > channel | String | Yes | Channel name |
| > instId | String | Yes | Product ID<br>Only support: `defualt`(all symbols) |

Response Example

```json
{
    "event":"subscribe",
    "arg":{
        "instType":"MARGIN",
        "channel":"index-price",
        "instId":"default"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| event | String | Event |
| arg | Object | Subscribed channels |
| > instType | String | Product type: MARGIN |
| > channel | String | Channel name |
| > instId | String | Product ID<br>`default` |
| code | String | Error code, returned only on error |
| msg | String | Error message |

Push Data

```json
{
    "action":"snapshot",
    "arg":{
        "instType":"MARGIN",
        "channel":"index-price",
        "instId":"default"
    },
    "data":[
        {
            "symbol":"BTCUSDT",
            "baseCoin":"BTC",
            "quoteCoin":"USDT",
            "indexPrice":"25702.4",
            "ts":"1695880514609"
        }
    ],
    "ts":1695880514622
}
```

### Push Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| arg | Object | Channels with successful subscription |
| > instType | String | Product type: MARGIN |
| > channel | String | Channel name |
| > instId | String | Product ID |
| action | String | Push data action, `snapshot` or `update` |
| data | List<Object> | Subscribed data |
| > baseCoin | String | Base currency |
| > indexPrice | String | Index price |
| > quoteCoin | String | Quote currency |
| > symbol | String | Trading pair |
| > ts | String | > timestamp |

---

# Cross Margin — WebSocket Private

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross-Margin Orders Channel

### Description

Request Example

```bash
{
    "args":[
        {
            "channel":"orders-crossed",
            "instId":"BTCUSDT",
            "instType":"MARGIN"
        }
    ],
    "op":"subscribe"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| op | String | Yes | Operation<br>`subscribe`: Subscribe<br>`unsubscribe`: Unsubscribe |
| args | List<Object> | Yes | List of channels to request subscription |
| > instType | String | Yes | Product type: MARGIN |
| > channel | String | Yes | Channel name |
| > instId | String | Yes | Product ID<br>e.g. ETHUSDT |

Response Example

```json
{
    "event":"subscribe",
    "arg":{
        "instType":"MARGIN",
        "channel":"orders-crossed",
        "instId":"BTCUSDT"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| event | String | Event |
| arg | Object | Subscribed channels |
| > instType | String | Product type: MARGIN |
| > channel | String | Channel name |
| > instId | String | Product ID<br>e.g. ETHUSDT |
| code | String | Error code, returned only on error |
| msg | String | Error message |

Push Data

```json
{
    "action":"snapshot",
    "arg":{
        "instType":"MARGIN",
        "channel":"orders-crossed",
        "instId":"BTCUSDT"
    },
    "data":[
        {
            "force":"gtc",
            "orderType":"market",
            "price":"0.000000000",
            "quoteSize":"0.000000000",
            "side":"buy",
            "feeDetail":[
                {
                    "feeCoin":"USDT",
                    "deduction":"no",
                    "totalDeductionFee":"0",
                    "totalFee":"0.01538693"
                }
            ],
            "enterPointSource":"web",
            "status":"partially_filled",
            "baseSize":"0.000000000",
            "cTime":"1695881543701",
            "uTime":"1695881543701",
            "clientOid":"2000000000",
            "fillPrice":"26426.800000000",
            "baseVolume":"0.000200000",
            "fillTotalAmount":"5.285360000",
            "loanType":"normal",
            "orderId":"1",
            "stpMode": "cancel_taker"
        }
    ],
    "ts":1695881543805
}
```

### Push Data Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| arg | Object | Successful channel subscriptions |
| > instType | String | Product type: MARGIN |
| > channel | String | Channel name |
| > instId | String | Product ID, e.g. ETHUSDT |
| action | String | Push data action, `snapshot` or `update` |
| data | List<Object> | Subscribed data |
| > baseSize | String | Number of base coins |
| > cTime | String | Creation time, Unix millisecond timestamp |
| > uTime | String | Update time, Unix millisecond timestamp |
| > clientOid | String | Client Unique Identifier |
| > fillPrice | String | Sale price |
| > baseVolume | String | Filled quantity |
| > fillTotalAmount | String | sum of money sold |
| > loanType | String | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |
| > orderId | String | Order ID |
| > orderType | String | Order type<br>limit: limit order<br>market |
| > price | String | Order price |
| > quoteSize | String | Number of denominated coins |
| > side | String | Methods of Sale and Purchase |
| > feeDetail | List<Object> | Transaction fee of the order |
| >> deduction | String | Is discount |
| >> totalDeductionFee | String | Fee of discount |
| >> totalFee | String | Order transaction fee, the transaction fee charged by the platform from the user. |
| >> feeCoin | String | The currency of the transaction fee |
| > enterPointSource | String | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| > status | String | Order status<br>`live`:new order;<br>`partially_filled`:partially filled;<br>`filled`:full filled;<br>`cancelled`: cancelled; |
| > symbol | String | Trading pair |
| > force | String | Order Strategy |
| > stpMode | String | STP Mode <br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |
| ts | Long | Timestamp ms |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cross-Margin Account Channel

### Description

Request Example

```bash
{
    "args":[
        {
            "channel":"account-crossed",
            "coin":"default",
            "instType":"MARGIN"
        }
    ],
    "op":"subscribe"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| op | String | Yes | Operation<br>subscribe Subscribe<br>unsubscribe Unsubscribe |
| args | List<Object> | Yes | List of channels to request subscription |
| > instType | String | Yes | Product type: `MARGIN` |
| > channel | String | Yes | Channel name: `account-crossed` |
| > coin | String | Yes | Coin name，`default` represents all the coins，Only default is supported nows |

Response Example

```json
{
    "event":"subscribe",
    "arg":{
        "instType":"MARGIN",
        "channel":"account-crossed",
        "coin":"default"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| event | String | Event |
| arg | Object | Subscribed channels |
| > instType | String | Product type: `MARGIN` |
| > channel | String | Channel name: `account-crossed` |
| > coin | String | `default` |
| code | String | Error code, returned only on error |
| msg | String | Error message |

Push Data

```json
{
    "action":"snapshot",
    "arg":{
        "instType":"MARGIN",
        "channel":"account-crossed",
        "coin":"default"
    },
    "data":[
        {
            "uTime":"1695881380854",
            "id":"1",
            "coin":"USDT",
            "available":"31092.45075078",
            "borrow":"0.00000000",
            "frozen":"6.00000000",
            "interest":"0.00000000",
            "coupon":"0.00000000"
        },
        {
            "uTime":"1695881380854",
            "id":"1",
            "coin":"BTC",
            "available":"0.00000000",
            "borrow":"0.99990096",
            "frozen":"0.00000000",
            "interest":"0.00416626",
            "coupon":"0.00000000"
        }
    ],
    "ts":1695881380856
}
```

### Push Data Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| arg | Object | Successful channel subscriptions |
| > instType | String | Product type: `MARGIN` |
| > channel | String | Channel name: `account-crossed` |
| > coin | String | `default` |
| action | String | Push data action, `snapshot` or `update` |
| data | List<Object> | Subscribed data |
| > available | String | Available amount |
| > borrow | String | Borrow amount |
| > coin | String | Coin name |
| > frozen | String | Amount frozen |
| > coupon | String | Coupon |
| > id | String | ID |
| > interest | String | Interest |
| > uTime | String | Updated time |

---

# Isolated Margin — Account

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Account Asset

Frequency limit: 10 times/1s (UID)

### Description

Getting account asset

### HTTP Request

- GET /api/v2/margin/isolated/account/assets

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/account/assets?symbol=BTCUSDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | No | Trading pairs, like BTCUSDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1694588056713,
  "data": [
    {
      "symbol": "BTCUSDT",
      "coin": "BTC",
      "totalAmount": "19876.1798394",
      "available": "19876.1798394",
      "frozen": "0",
      "borrow": "0",
      "interest": "0",
      "net": "19876.1798394",
      "coupon": "0",
      "cTime": "1692085312298",
      "uTime": "1694569802151"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| coin | String | Token name |
| symbol | String | Trading pair name |
| totalAmount | String | Total amount |
| available | String | Available amount |
| frozen | String | Assets frozen |
| borrow | String | Borrow |
| interest | String | Interest, Interest-only payments with a minimum payment of interest. |
| net | String | Net assets = available + frozen − borrow − interest. Liquidation is triggered when the risk ratio is reached 1. |
| coupon | String | Trading bonus |
| cTime | String | Creation time |
| uTime | String | Update time |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Max Transferable

Frequency limit: 10 times/1s (UID)

### Description

Get Isolated Max Transferable Amount

### HTTP Request

- GET /api/v2/margin/isolated/account/max-transfer-out-amount

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/account/max-transfer-out-amount?symbol=BTCUSDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695618856403,
  "data": {
    "baseCoin": "BTC",
    "symbol": "BTCUSDT",
    "baseCoinMaxTransferOutAmount": "199999",
    "quoteCoin": "USDT",
    "quoteCoinMaxTransferOutAmount": "1000000"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| baseCoin | String | Base coin |
| quoteCoin | String | Quote coin |
| baseCoinMaxTransferOutAmount | String | Maximum transferable amount for base coin |
| quoteCoinMaxTransferOutAmount | String | Maximum transferable amount for quote coin |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Borrow

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/account/borrow

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/account/borrow?symbol=USDT"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"coin": "USDT","borrowAmount": "1","symbol": "BTCUSDT"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Borrowing trading pairs, like BTCUSDT |
| coin | String | Yes | Borrowing coins, such as BTC |
| borrowAmount | String | Yes | Borrowing amount (up to 8 decimal places) |
| clientOid | String | No | Client customized ID |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1679384491703,
  "data": {
    "loanId":"123412412452345",
    "symbol": "BTCUSDT",
    "coin": "USDT",
    "borrowAmount": "1.00000000"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanId | String | Loan ID |
| coin | String | Borrowing coin |
| borrowAmount | String | Borrowing amount |
| symbol | String | Borrowing trading pair |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Flash Repay

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/account/flash-repay

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/account/flash-repay" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '{"symbolList": ["ETHUSDT"]}"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbolList | List | No | Trading pair array under isolated mode<br>If it is not filled, all trading pair will be confirmed by default.<br>Up to 100 trading pairs in one request |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695619576187,
  "data": [
    {
      "repayId": "3423423",
      "symbol": "ETHUSDT",
      "result": "success"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| repayId | String | Repayment id |
| symbol | String | Spot margin trading pairs |
| result | String | success Repayment request successful<br>failure Failure to repay |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Interest Rate and Max Borrowable

Frequency limit:10 times/1s (UID)

### Description

This interface will determine the user's VIP level based on the User ID sending the request, and then return information such as interest rates and limits based on the VIP level.

### HTTP Request

- GET /api/v2/margin/isolated/interest-rate-and-limit

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/interest-rate-and-limit?symbol=BTCUSDT"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1694579779768,
  "data": [
    {
      "symbol": "BTCUSDT",
      "leverage": "5",
      "baseCoin": "BTC",
      "baseTransferable": true,
      "baseBorrowable": true,
      "baseDailyInterestRate": "0.00007113",
      "baseAnnuallyInterestRate": "0.02596245",
      "baseMaxBorrowableAmount": "0",
      "baseVipList": [
        {
          "level": "0",
          "dailyInterestRate": "0.00007333",
          "limit": "0",
          "annuallyInterestRate": "0.02676545",
          "discountRate": "1"
        }
      ],
      "quoteCoin": "USDT",
      "quoteTransferable": true,
      "quoteBorrowable": true,
      "quoteDailyInterestRate": "0.00011963",
      "quoteAnnuallyInterestRate": "0.04366495",
      "quoteMaxBorrowableAmount": "0",
      "quoteList": [
        {
          "level": "0",
          "dailyInterestRate": "0.00012333",
          "limit": "0",
          "annuallyInterestRate": "0.04501545",
          "discountRate": "1"
        }
      ]
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| baseCoin | String | Base currency |
| leverage | String | Leverage: default: 10 |
| baseTransferable | Boolean | Is the base currency transferable?<br>true: Transferable<br>false: Not transferable |
| baseBorrowable | Boolean | Whether or not the base currency can be borrowed?<br>true: Borrowable<br>false: Not borrowable |
| baseDailyInterestRate | String | Non-VIP daily interest rate of the base currency |
| baseAnnuallyInterestRate | String | Non-VIP APR of the base currency |
| baseMaxBorrowableAmount | String | Maximum borrow in base currency |
| baseVipList | Array | Base currency VIP |
| > level | String | VIP level |
| > limit | String | VIP limit |
| > dailyInterestRate | String | VIP daily interest rate |
| > annuallyInterestRate | String | VIP APR |
| > discountRate | String | VIP discount: 1 is for 100%, i.e. no discount; 0.97 is for 97% of the original rate |
| quoteCoin | String | Quote currency |
| quoteTransferInAble | Boolean | Whether the quote currency is transferable |
| quoteTransferable | Boolean | Whether the quote currency is borrowable |
| quoteBorrowable | String | Non-VIP daily interest rate of the quote currency |
| quoteAnnuallyInterestRate | String | Non-VIP APR of the quote currency |
| quoteMaxBorrowableAmount | String | Non-VIP maximum borrow in quote currency |
| quoteList | Array | Quote currency VIP |
| > level | String | VIP level |
| > limit | String | VIP limit |
| > dailyInterestRate | String | VIP daily interest rate |
| > annuallyInterestRate | String | VIP APR |
| > discountRate | String | VIP discount: 1 is for 100%, i.e. no discount; 0.97 is for 97% of the original rate |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Max Borrowable

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/account/max-borrowable-amount

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/account/max-borrowable-amount?symbol=BTCUSDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695636742119,
  "data": {
    "symbol": "ETHUSDT",
    "baseCoin": "ETH",
    "baseCoinMaxBorrowAmount": "10.1401916",
    "quoteCoin": "USDT",
    "quoteCoinMaxBorrowAmount": "3976070.21616"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| baseCoin | String | Left coin |
| baseCoinMaxBorrowAmount | String | Maximum borrow amount of left coins (amount changes in real time) |
| quoteCoin | String | Right coin |
| quoteCoinMaxBorrowAmount | String | Maximum borrow amount of right coins |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Repay

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/account/repay

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/account/repay" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"  -d '{"coin": "USDT","repayAmount": "1","symbol":"BTCUSDT"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| repayAmount | String | Yes | Number of repayments, up to 8 decimal places |
| coin | String | Yes | Repayment coin |
| symbol | String | Yes | Repayment trading pairs, like BTCUSDT |
| clientOid | String | No | Client customized ID |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1694588859956,
  "data": {
    "remainDebtAmount": "0",
    "symbol": "BTCUSDT",
    "repayId":"1234234234234",
    "coin": "USDT",
    "repayAmount": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| remainDebtAmount | String | Remaining borrowings |
| repayId | String | Repayment ID |
| symbol | String | Trading pair |
| coin | String | Coin |
| repayAmount | String | Repayment amount |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Risk Rate

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/account/risk-rate

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/account/risk-rate?symbol=BTCUSDT" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | No | Trading pairs, like BTCUSDT |
| pageNum | String | No | Page number, Default: 1 |
| pageSize | String | No | Size per page, default 100, maximum 500 |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1694590583906,
  "data": [
    {
      "symbol": "BTCUSDT",
      "riskRateRatio": "0"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| symbol | String | Trading pair |
| riskRateRatio | String | Risk ratio (total assets/total liabilities under isolated mode) |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Tier Configuration

Frequency limit:10 times/1s (UID)

### Description

This interface will determine the user's VIP level based on the User ID sending the request, and then return the tier information based on the VIP level.

### HTTP Request

- GET /api/v2/margin/isolated/tier-data

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/tier-data?symbol=BTCUSDT"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, like BTCUSDT |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1694583297872,
  "data": [
    {
      "tier": "1",
      "symbol": "BTCUSDT",
      "leverage": "5",
      "baseCoin": "BTC",
      "quoteCoin": "USDT",
      "baseMaxBorrowableAmount": "0",
      "quoteMaxBorrowableAmount": "0",
      "maintainMarginRate": "0.1",
      "initRate": "0.25"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| tier | String | Tier |
| symbol | String | Trading pair |
| leverage | String | Effective leverage, global default: 10x |
| baseCoin | String | Base currency |
| quoteCoin | String | Quote currency |
| baseMaxBorrowableAmount | String | Maximum borrow in base currency |
| quoteMaxBorrowableAmount | String | Maximum borrow in quote currency |
| maintainMarginRate | String | Maintenance margin rate |
| initRate | String | Initial margin rate |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Query Isolated Flash Repayment Result

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/account/query-flash-repay-status

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/account/query-flash-repay-status" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '{"idList": ["13423423"]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| idList | List | Yes | Repayment ID list under isolated mode<br>Up to 100 trading pairs in one request |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 163123213132,
  "data": [
    {
      "repayId": "13423423",
      "status": "FINISH"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| repayId | String | Repayment ID |
| status | String | Repayment result status |

---

# Isolated Margin — Trade

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Orders History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/history-orders

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/history-orders?startTime=1695336324000&symbol=ETHUSDT"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderId | String | No | Order ID |
| enterPointSource | String | No | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| clientOid | String | No | Client customized ID |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages<br>No setting is needed when first querying. Set to to smallest orderId returned from the last query when searching for data in the second page and other paged. Data smaller than the orderId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695623642247,
  "data": {
    "orderList": [
      {
        "symbol": "ETHUSDT",
        "orderType": "limit",
        "enterPointSource": "API",
        "orderId": "121211212122",
        "clientOid": "121211212122",
        "loanType": "normal",
        "price": "1410.5",
        "side": "buy",
        "status": "cancelled",
        "baseSize": "0.1",
        "quoteSize": "0",
        "priceAvg": "0",
        "size": "0",
        "amount": "0",
        "force": "gtc",
        "cTime": "1695621699722",
        "uTime": "1695621708242"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderList | List | List |
| \> symbol | String | Trading pair |
| \> orderId | String | Order ID |
| \> clientOid | String | Client customized ID |
| \> size | String | Filled quantity |
| \> priceAvg | String | Filled price |
| \> amount | String | Filled volume |
| \> force | String | Time in force<br>`gtc`: normal limit order, good till canceled<br>`post_only`: Post only<br>`fok`: Fill or kill<br>`ioc`: Immediate or cancel |
| \> price | String | Order price |
| \> enterPointSource | String | Order source<br>WEB: WEB client<br>IOS: IOS client<br>ANDROID: Andriod client<br>API: API Client<br>SYS: system, usually because burst |
| \> status | String | Order status<br>`live`: New order<br>`partially_fill`: Partially filled<br>`filled`: All filled<br>`cancelled`: Cancelled<br>`reject`: Rejected |
| \> side | String | Direction<br>sell: Sell<br>buy: Buy |
| \> baseSize | String | Number in base coins |
| \> quoteSize | String | Number in quote currency |
| \> orderType | String | Order type<br>limit<br>market |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| \> loanType | String | Margin order model<br>normal: Normal order<br>autoLoan: auto-borrow order<br>autoRepay: auto-repay order<br>autoLoanAndRepay: auto-borrow and auto-repay order |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Order Fills

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/fills

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/fills?symbol=ETHUSDT&startTime=1695336324000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderId | String | No | Order ID |
| idLessThan | String | No | Match order ID. A parameter for paging. No setting is needed when querying for the first time. Set to to smallest orderId returned from the last query when searching for data in the second page and other paged. Data smaller than the orderId entered will be returned. This is designed to shorten the query response time. |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695623797791,
  "data": {
    "fills": [
      {
        "orderId": "121211212122",
        "tradeId": "121211212122",
        "orderType": "limit",
        "side": "buy",
        "priceAvg": "1721.5",
        "size": "0.1",
        "amount": "172.15",
        "tradeScope": "taker",
        "feeDetail": {
          "deduction": "no",
          "feeCoin": "ETH",
          "totalDeductionFee": "0",
          "totalFee": "-0.00010000"
        },
        "cTime": "1695621005237",
        "uTime": "1695621006149"
      }
    ],
    "minId": "1",
    "maxId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| fills | List | List |
| \> orderId | String | Order no. |
| \> tradeId | String | Order details ID |
| \> orderType | String | Order type<br>limit<br>market |
| \> size | String | Filled quantity |
| \> priceAvg | String | Filled price |
| \> amount | String | Filled quantity |
| \> side | String | Direction<br>sell: Sell<br>buy: Buy |
| \> tradeScope | String | Trader tag<br>taker<br>maker |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| \> feeDetail | Array | Transaction fee breakdown |
| >\> deduction | String | Discount or not |
| >\> feecoinCode | String | Transaction fee currency |
| >\> totalDeductionFee | String | Total transaction fee discount |
| >\> totalFee | String | Total transaction fee |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Cancel Isolated Orders in Batch

Rate limit: 10 req/sec/UID

### Description

### HTTP Request

- POST /api/v2/margin/isolated/batch-cancel-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/batch-cancel-order" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '   {    "symbol": "BTCUSDT",    "orderIdList": [{        "orderId":"121211212122",        "clientId":"121211212122"    }    ]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderIdList | List | No | Order ID list <br>Either orderId or clientOid is required |
| > orderId | String | No | Order ID <br>Either orderId or clientOid |
| > clientOid | String | No | Client customized ID <br>Either orderId or clientOid |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695622930742,
  "data": {
    "successList": [
      {
        "orderId": "121211212122",
        "clientOid": "121211212122"
      }
    ],
    "failureList": [
    ]
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| successList | Array | Successful order number |
| > orderId | String | Order ID |
| > clientOid | String | Client customized ID |
| failureList | Array | Failed order number |
| > clientOid | String | Client customized ID |
| > errorMsg | String | Error information |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Batch Orders

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/batch-place-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/batch-place-order" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '   {    "symbol":"BTCUSDT",    "orderList": [        {    "side": "buy",        "orderType": "market",    "force": "gtc",        "quoteSize":"110",    "loanType":"normal"}    ]}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderList | List | Yes | Order Entries |
| orderType | String | Yes | Order type<br>limit<br>market |
| price | String | No | Price |
| loanType | String | Yes | Margin order model<br>normal: Normal order<br>autoLoan: auto-borrow order<br>autoRepay: auto-repay order<br>autoLoanAndRepay: auto-borrow and auto-repay order<br>timeInforce |
| force | String | Yes | Time in force (invalid when `orderType` is `market`)<br>gtc: normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| baseSize | String | No | Limit and Market sell are required. Sell orders represent the number of baseCoins (left coin). |
| quoteSize | String | No | market buy is required, the buy order represents the number of quote coins (right coin). |
| clientOid | String | No | Customized ID |
| side | String | Yes | Direction<br>sell: Sell<br>buy: Buy |
| stpMode | String | No | STP Mode, default `none`<br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695621286952,
  "data": {
    "successList": [
      {
        "orderId": "121211212122",
        "clientOid": "121211212122"
      }
    ],
    "failureList": [
    ]
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| successList | Array | Successful order number |
| > orderId | String | Order ID |
| > clientOid | String | Client customized ID |
| failureList | Array | Failed order number |
| > clientOid | String | Client customized ID |
| > errorMsg | String | Error information |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Cancel Order

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- POST /api/v2/margin/isolated/cancel-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/cancel-order" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '   {    "symbol":"ETHUSDT",    "orderId":"121211212122"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderId | String | No | Order ID<br>Either orderId or clientOid is required |
| clientOid | String | No | Client customized ID<br>Either orderId or clientOid is required |

Response Example

```json
{
    "code": "00000",
    "msg": "success",
    "requestTime": 1695621708119,
    "data": {
        "orderId": "121211212122",
        "clientOid": "121211212122"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| clientOid | String | Customized ID |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Current Orders

Rate limit: 10 req/sec/UID

### Description

### HTTP Request

- GET /api/v2/margin/isolated/open-orders

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/open-orders?symbol=ETHUSDT&startTime=1695336324000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderId | String | No | Order ID |
| clientOid | String | No | Client customized ID |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps |
| limit | String | No | Number of queries<br>The default value is 100 entries and the maximum value is 500 entries. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695623147774,
  "data": {
    "orderList": [
      {
        "symbol": "ETHUSDT",
        "orderType": "limit",
        "enterPointSource": "API",
        "orderId": "121211212122",
        "clientOid": "121211212122",
        "loanType": "normal",
        "price": "1410.5",
        "side": "buy",
        "status": "live",
        "baseSize": "0.1",
        "quoteSize": "141.05",
        "priceAvg": "0",
        "size": "0",
        "amount": "0",
        "force": "gtc",
        "cTime": "1695623143333",
        "uTime": "1695623143333"
      }
    ],
    "maxId": "121211212122",
    "minId": "121211212122"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderList | List | Order list |
| \> symbol | String | Trading pair |
| \> orderId | String | Order no. |
| \> clientOid | String | Client customized ID |
| \> size | String | Filled quantity |
| \> priceAvg | String | Filled price |
| \> amount | String | Filled quantity |
| \> force | String | Order strategy<br>gtc: normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| \> price | String | Order price |
| \> enterPointSource | String | Order source<br>WEB: WEB client<br>IOS: IOS client<br>ANDROID: Andriod client<br>API: API Client<br>SYS: system, usually because burst |
| \> status | String | Order status<br>`live`: New order<br>`partial_fill`: Partially filled<br>`filled`: Fully filled<br>`cancelled`: The order is cancelled<br>`reject`: Rejected |
| \> side | String | Direction<br>sell: Sell<br>buy: Buy |
| \> baseSize | String | Number in base coins |
| \> quoteSize | String | Number in quote currency |
| \> orderType | String | Order type<br>limit<br>market |
| \> cTime | String | Creation time |
| \> uTime | String | Updated time |
| \> loanType | String | Margin order model<br>normal: Normal order<br>autoLoan: auto-borrow order<br>autoRepay: auto-repay order<br>autoLoanAndRepay: auto-borrow and auto-repay order |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated Place Order

Rate Limit: 10 req/sec/UID

### Description

### HTTP Request

- POST /api/v2/margin/isolated/place-order

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/margin/isolated/place-order" 
   -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json" 
   -d '   {    "symbol": "ETHUSDT",    "side": "buy",    "price":"1796.5",    "orderType": "limit",    "force": "gtc",    "baseSize":"0.1",    "loanType":"normal"}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| orderType | String | Yes | Order type<br>limit<br>market |
| price | String | No | Price |
| loanType | String | Yes | Margin order model<br>normal: Normal order<br>autoLoan: auto-borrow order<br>autoRepay: auto-repay order<br>autoLoanAndRepay: auto-borrow and auto-repay order |
| force | String | Yes | Time in force (invalid when `orderType` is `market`)<br>gtc: normal limit order, good till canceled<br>post\_only: Post only<br>fok: Fill or kill<br>ioc: Immediate or cancel |
| baseSize | String | No | Limit and Market sell are required. Sell orders represent the number of baseCoins (left coin). |
| quoteSize | String | No | market buy is required, the buy order represents the number of quote coins (right coin). |
| clientOid | String | No | Customized ID |
| side | String | Yes | Direction<br>sell: Sell<br>buy: Buy |
| stpMode | String | No | STP Mode, default `none`<br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695621004679,
  "data": {
    "orderId": "121211212122",
    "clientOid": "121211212122"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| clientOid | String | Customized ID |

---

# Isolated Margin — Records

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Financial History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/financial-records

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/financial-records?symbol=BTCUSDT&startTime=1692690126000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| marginType | String | No | Capital flow type<br>transfer\_in: inbound transfer<br>transfer\_out: outbound transfer<br>borrow: Borrowings<br>repay: Repayment<br>liquidation\_fee: Liquidation fee<br>compensate: Risk fund compensation for collateral shortfall<br>deal\_in: buy<br>deal\_out: sell<br>confiscated: Deducted for collateral shortfall<br>exchange\_in: Exchange income, charged to the system account<br>exchange\_out: Exchange spending, received by the system account<br>sys\_exchange\_in: exchange income of the system account, with exchange spending appearing at the same time<br>sys\_exchange\_out: exchange spending of the system account, with exchange income appearing at the same time |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. No setting is needed when querying for the first time. Set to to smallest marginId returned from the last query when searching for data in the second page and other paged. Data smaller than the marginId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
    "code": "00000",
    "msg": "success",
    "requestTime": 1695625219083,
    "data": {
        "resultList": [
            {
                "coin": "BTC",
                "symbol": "BTCUSDT",
                "marginId": "1",
                "amount": "-1",
                "balance": "199999.0036963",
                "fee": "0",
                "marginType": "repay",
                "cTime": "1695624616058",
                "uTime": "1695624616058"
            }
        ],
        "maxId": "1",
        "minId": "1"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| resultList | List | List |
| \> symbol | String | Trading pair |
| \> coin | String | Coin |
| \> marginId | String | Capital flow ID |
| \> marginType | String | Capital flow type<br>transfer\_in: assets transferred in<br>transfer\_out: assets transferred out<br>borrow: borrow<br>Repay: repay<br>liquidation\_fee: liquidation fee<br>compensate: collateral shortfall compensation from risk fund<br>deal\_in: trade and deposit (buy)<br>deal\_out: trade and withdraw (sell)<br>confiscated: deduction for collateral shortfall<br>exchange\_in: exchange income, charged from the system account<br>exchange\_out: exchange expense, charged to the system account<br>sys\_exchange\_in: exchange income of the system account, with exchange expense appearing at the same time<br>sys\_exchange\_out: exchange expense of the system account, with exchange income appearing at the same time |
| \> amount | String | Capital flow amount |
| \> balance | String | Account balance |
| \> fee | String | Transaction fee details |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Interest History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/interest-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/interest-history?coin=USDT&startTime=1695336324000&symbol=BTCUSDT"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pair |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. No setting is needed when querying for the first time. Set to to smallest interestId returned from the last query when searching for data in the second page and other paged. Data smaller than the interestId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695624945382,
  "data": {
    "resultList": [
      {
        "interestId": "1",
        "interestCoin": "USDT",
        "dailyInterestRate": "0.24",
        "loanCoin": "USDT",
        "interestAmount": "0.01",
        "interstType": "first",
        "symbol": "BTCUSDT",
        "cTime": "1695624061276",
        "uTime": "1695624061276"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| resultList | List | List |
| \> symbol | String | Trading pair |
| \> interestId | String | Interest ID |
| \> interestAmount | String | Amount of interest |
| \> dailyInterestRate | String | Daily interest rate |
| \> interstType | String | Type of interest<br>first: Interest on initial borrowing<br>scheduled: scheduled interest |
| \> interestCoin | String | Interest coin |
| \> loanCoin | String | Borrowing coin |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Liquidation History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/liquidation-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/liquidation-history?symbol=BTCUSDT&startTime=1692690126000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pair |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. No setting is needed when querying for the first time. Set to to smallest liqId returned from the last query when searching for data in the second page and other paged. Data smaller than the liqId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695624945382,
  "data": {
    "resultList": [
      {
        "liqId": "123",
        "symbol": "BTCUSDT",
        "liqStartTime": "1653453245342",
        "liqEndTime": "16312423423432",
        "liqRiskRatio": "1.01",
        "totalAssets": "1242.34",
        "totalDebt": "1100",
        "liqFee": "1.2",
        "cTime": "1653453245342",
        "uTime": "1692690126000"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| resultList | List | List |
| \> symbol | String | Trading pair |
| \> liqId | String | Liquidation ID |
| \> liqStartTime | String | Start time of liquidation |
| \> liqEndTime | String | Liquidation end time |
| \> liqRiskRatio | String | Risk ratio at liquidation |
| \> totalAssets | String | Total assets at liquidation<br>in USDT |
| \> totalDebt | String | Total borrowings at liquidation<br>in USDT |
| \> liqFee | String | Liquidation fees |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Borrow History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/borrow-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/borrow-history?symbol=BTCUSDT&startTime=1695336324000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| loanId | String | No | Borrowing ID (accurate matching of single entry) |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. No setting is needed when querying for the first time. Set to to smallest loanId returned from the last query when searching for data in the second page and other paged. Data smaller than the loanId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695624081817,
  "data": {
    "resultList": [
      {
        "loanId": "1",
        "coin": "USDT",
        "borrowAmount": "1",
        "borrowType": "manual_loan",
        "symbol": "BTCUSDT",
        "cTime": "1695624061276",
        "uTime": "1695624061276"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| resultList | List | List |
| \> symbol | String | Trading pair |
| \> loanId | String | ID |
| \> coin | String | Borrowed coin |
| \> amount | String | Borrowed amount |
| \> borrowType | String | Type of borrowing<br>auto\_loan: Auto-loan<br>manual\_loan: manual\_loan |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Get Isolated Repayment History

Frequency limit: 10 times/1s (UID)

### Description

### HTTP Request

- GET /api/v2/margin/isolated/repay-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/margin/isolated/repay-history?symbol=BTCUSDT&startTime=1695336324000"  -H "ACCESS-KEY:*******" 
   -H "ACCESS-SIGN:*******" 
   -H "ACCESS-PASSPHRASE:*****" 
   -H "ACCESS-TIMESTAMP:1659076670000" 
   -H "locale:en-US" 
   -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| symbol | String | Yes | Trading pairs, BTCUSDT |
| repayId | String | No | Repayment ID |
| coin | String | No | Coin |
| startTime | String | Yes | Start time, Unix millisecond timestamps |
| endTime | String | No | End time, Unix millisecond timestamps<br>Maximum interval between start and end times is 90 days |
| limit | String | No | Number of queries<br>Default: 100, maximum: 500 |
| idLessThan | String | No | For turning pages. No setting is needed when querying for the first time. Set to to smallest repayId returned from the last query when searching for data in the second page and other paged. Data smaller than the repayId entered will be returned. This is designed to shorten the query response time. |

Response Example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1695624627000,
  "data": {
    "resultList": [
      {
        "repayId": "1",
        "coin": "BTC",
        "repayAmount": "1",
        "repayType": "manual_repay",
        "repayInterest": "0.01",
        "repayPrincipal": "0.99",
        "symbol": "BTCUSDT",
        "cTime": "1695624616058",
        "uTime": "1695624616058"
      }
    ],
    "maxId": "1",
    "minId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| resultList | List | List |
| \> symbol | String | Trading pair |
| \> repayId | String | ID |
| \> coin | String | Coin |
| \> repayPrincipal | String | Repayment principal |
| \> repayAmount | String | Total repayment |
| \> repayInterest | String | Repayment interest |
| \> repayType | String | Repayment type<br>auto-repay: Automatic repayment<br>manual-repay: Manual repayment<br>liq-repay: Liquidation repayment<br>force-repay: Forced repayment |
| \> cTime | String | Creation time, millisecond timestamp |
| \> uTime | String | Update time, millisecond timestamp |
| maxId | String | Max ID of current search result |
| minId | String | Min ID of current search result, use: `idLessThan=minId` in the next query |

---

# Isolated Margin — WebSocket Private

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated-Margin Orders Channel

### Description

Request Example

```bash

{
    "args":[
        {
            "channel":"orders-isolated",
            "instId":"BTCUSDT",
            "instType":"MARGIN"
        }
    ],
    "op":"subscribe"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| op | String | Yes | Operation<br>subscribe Subscribe<br>unsubscribe Unsubscribe |
| args | List<Object> | Yes | List of channels to request subscription |
| > instType | String | Yes | Product type |
| > channel | String | Yes | Channel name |
| > instId | String | Yes | Product ID<br>E.g. ETHUSDT |

Response Example

```json
{
    "event":"subscribe",
    "arg":{
        "instType":"MARGIN",
        "channel":"orders-isolated",
        "instId":"BTCUSDT"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| event | String | Event |
| arg | Object | Subscribed channels |
| > instType | String | Product type |
| > channel | String | Channel name |
| > instId | String | Product ID<br>E.g. ETHUSDT |
| code | String | Error code, returned only on error |
| msg | String | Error message |

Push Data

```json
{
    "action":"snapshot",
    "arg":{
        "instType":"MARGIN",
        "channel":"orders-isolated",
        "instId":"BTCUSDT"
    },
    "data":[
        {
            "enterPointSource":"web",
            "force":"gtc",
            "orderType":"market",
            "price":"0.000000000",
            "quoteSize":"0.000000000",
            "side":"sell",
            "feeDetail":[
                {
                    "feeCoin":"USDT",
                    "deduction":"no",
                    "totalDeductionFee":"0",
                    "totalFee":"0.01538693"
                }
            ],
            "status":"partially_filled",
            "baseSize":"0.056100000",
            "cTime":"1697094058377",
            "uTime":"1697094058377",
            "clientOid":"1",
            "fillPrice":"26869.6530837789661319",
            "baseVolume":"0.056100000",
            "fillTotalAmount":"1507.387538000",
            "loanType":"auto-repay",
            "orderId":"1",
            "stpMode":"cancel_taker"
        }
    ],
    "ts":1697094058809
}
```

### Push Data Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| arg | Object | Successful channel subscriptions |
| > instType | String | Product type |
| > channel | String | Channel name |
| > instId | String | Product ID e.g. ETHUSDT |
| action | String | Push data action, `snapshot` or `update` |
| data | List<Object> | Subscribed data |
| > baseSize | String | Number of base coins |
| > cTime | String | Creation time, Unix millisecond timestamp |
| > uTime | String | Update time, Unix millisecond timestamp |
| > clientOid | String | Client Unique Identifier |
| > fillPrice | String | Sale price |
| > baseVolume | String | Base coin quantity |
| > fillTotalAmount | String | sum of money sold |
| > loanType | String | Margin order model<br>normal: place a normal order<br>autoLoan place an order with auto-borrow<br>autoRepay place an order with auto-repay<br>autoLoanAndRepay place an order with auto-borrow and auto-repay |
| > orderId | String | Order ID |
| > orderType | String | Order type<br>limit: limit price<br>market: market price |
| > price | String | Order price |
| > quoteSize | String | Number of denominated coins |
| > side | String | Methods of Sale and Purchase |
| > feeDetail | List<Object> | Transaction fee of the order |
| >> deduction | String | Is discount |
| >> totalDeductionFee | String | Fee of discount |
| >> totalFee | String | Order transaction fee, the transaction fee charged by the platform from the user. |
| >> feeCoin | String | The currency of the transaction fee |
| > enterPointSource | String | Order source<br>WEB: Orders created on the website<br>API: Orders created on API<br>SYS: System managed orders, usually generated by forced liquidation logic<br>ANDROID: Orders created on the Android app<br>IOS: Orders created on the iOS app |
| > status | String | Order status |
| > symbol | String | Trading pair |
| > force | String | Order Strategy |
| > stpMode | String | STP Mode <br>`none` not setting STP <br>`cancel_taker` cancel taker order <br>`cancel_maker` cancel maker order <br>`cancel_both` cancel both of taker and maker orders |
| ts | Long | Timestamp ms |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Isolated-Margin Account Channel

### Description

Request Example

```bash
{
    "args":[
        {
            "channel":"account-isolated",
            "coin":"default",
            "instType":"MARGIN"
        }
    ],
    "op":"subscribe"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| op | String | Yes | Operation<br>subscribe Subscribe<br>unsubscribe Unsubscribe |
| args | List<Object> | Yes | List of channels to request subscription |
| > instType | String | Yes | Product type: MARGIN |
| > channel | String | Yes | Channel name |
| > coin | String | Yes | Coin name，`default` represents all the coins，Only default is supported now |

Response Example

```json
{
    "event":"subscribe",
    "arg":{
        "instType":"MARGIN",
        "channel":"account-isolated",
        "coin":"default"
    }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| event | String | Yes<br>Event |
| arg | Object | Subscribed channels |
| > instType | String | Product type |
| > channel | String | Channel name |
| > coin | String | `default` |
| code | String | Error code, returned only on error |
| msg | String | Error message |

Push Data

```json
{
  "action":"snapshot",
  "arg":{
    "instType":"MARGIN",
    "channel":"account-isolated",
    "coin":"default"
  },
  "data":[
    {
      "uTime":"1695881380876",
      "id":"1",
      "coin":"USDT",
      "symbol":"BTCUSDT",
      "available":"989.4502207",
      "borrow":"0",
      "frozen":"0",
      "interest":"0",
      "coupon":"0"
    },
    {
      "uTime":"1695881380876",
      "id":"1000000002",
      "coin":"BTC",
      "symbol":"BTCUSDT",
      "available":"0.00039814",
      "borrow":"0",
      "frozen":"0",
      "interest":"0",
      "coupon":"0"
    },
    {
      "uTime":"1695881380876",
      "id":"1000000001",
      "coin":"USDT",
      "symbol":"ETHUSDT",
      "available":"9607383.17171658",
      "borrow":"0",
      "frozen":"0",
      "interest":"0",
      "coupon":"0"
    },
    {
      "uTime":"1695881380876",
      "id":"1000000000",
      "coin":"ETH",
      "symbol":"ETHUSDT",
      "available":"2.5574646",
      "borrow":"0",
      "frozen":"0",
      "interest":"0",
      "coupon":"0"
    }
  ],
  "ts":1695881380876
}
```

### Push Data Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| arg | Object | Successful channel subscriptions |
| > instType | String | Product type |
| > channel | String | Channel name |
| > coin | String | `default` |
| action | String | Push data action, `snapshot` or `update` |
| data | List<Object> | Subscribed data |
| > available | String | Available amount |
| > borrow | String | Borrow amount |
| > symbol | String | Trading pair |
| > coin | String | Coin name |
| > frozen | String | Amount frozen |
| > id | String | ID |
| > interest | String | Interest |
| > uTime | String | Updated time, ms |
| > coupon | String | Coupon |

---

# Error Codes

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# Rest API Error Code

| Error message | Error code |
| --- | --- |
| 429 | Too many requests |
| 00001 | startTime and endTime interval cannot be greater than {0} days |
| 00171 | Parameter verification failed {0}{1} |
| 00172 | Parameter verification failed |
| 01001 | {0} must be greater than 0 |
| 01002 | {0} precision must be less than or equal to {1} |
| 01003 | {0} Duplicate data exists |
| 11000 | withdraw address is not in addressBook |
| 12001 | {0} can be used at most |
| 12002 | Current currency {0}, limit net sell value {1} USD |
| 12003 | Current currency {0}, limit net buy value {1} USD |
| 12004 | Quote query failed, try again later |
| 12006 | Minimum limit not reached. |
| 12007 | Maximum limit exceeded. |
| 12008 | Daily individual convert limit reached |
| 12009 | Daily convert limit reached |
| 12014 | Failed to get a quote |
| 12017 | Contact customer service because your Convert permission is abnormal. |
| 12018 | Service disruption, contact customer service for assistance. |
| 12021 | Flash quote expired |
| 12022 | he flash quote information has been tampered with |
| 13001 | Withdraw is too frequent |
| 13002 | Currency does not exist |
| 13004 | Your remaining withdrawal amount{0} |
| 13005 | Failed to generate address |
| 13006 | The actual amount you can withdraw is {0}, and the rest is frozen for copy trade |
| 13007 | The current currency is {0}, and the net purchase value of {1} USD is limited within 24 hours, and the net purchase value of {2} USD is also allowed for {3} |
| 13008 | Traders minimum place orderSize is {0} |
| 13009 | Current currency {0}, 24-hour limit on net selling value {1} USD, you can also net sell {3} worth {2} USD |
| 13010 | The order is review, cannot be cancelled. |
| 13212 | A single withdrawal exceeds the maximum limit |
| 19000 | Spot copy trading operation failed |
| 20001 | startTime should be less than endTime |
| 20002 | {0} Only one is allowed to be passed |
| 22001 | No order to cancel |
| 22002 | No position to close |
| 22003 | modify price and size, please pass in newClientOid |
| 22004 | This symbol {0} not support API trade |
| 22005 | This symbol does not support cross mode |
| 22006 | limit price > risk price |
| 22007 | limit price < risk price |
| 22008 | market price > risk price |
| 22009 | market price < risk price |
| 22010 | Please bind ip whitelist address |
| 22011 | It is not allowed to set auto add margin in cross mode |
| 22012 | There are different business lines, {0} does not belong to {1} product |
| 22013 | Abnormal status of position experience coupon |
| 22014 | This position experience coupon does not exist |
| 22015 | This user has no position experience coupon sub-account |
| 22016 | This user is not a sub-account for position experience coupons and cannot operate experience coupons |
| 22017 | The position experience coupon does not support this tokenId |
| 22018 | The face value of the position experience coupon is a negative number |
| 22019 | The position experience coupon has not expired yet |
| 22020 | The leverage multiple is the leverage multiple of the current position and cannot be adjusted. |
| 22021 | Limit orders are not supported when placing orders with position experience coupons |
| 22022 | The position experience coupon has been used |
| 22023 | The trial coupon for this position has expired |
| 22024 | The experience coupon for this position has been recycled |
| 22025 | Margin cannot be added to the experience coupon account |
| 22026 | The position mode cannot be adjusted for the experience coupon account |
| 22027 | The position experience coupon does not support this currency pair |
| 22028 | The position experience coupon does not support this type of order |
| 22029 | Risk control, you can currently open a maximum position of {0} {1}. The risk control proportion limit for a uid is calculated including all main accounts and sub-accounts. |
| 22030 | {0} Demo trading mode，can not use:{1} |
| 22034 | Less than the minimum order amount |
| 22035 | Demo account open position too frequently |
| 22038 | Please enter the quantity as an integral multiple of {0} |
| 22039 | Limit open position when delivery is approaching. |
| 22040 | Limit open order when delivery is approaching. |
| 22041 | Limit close order when delivery is approaching. |
| 22042 | When a one-way position is held, trigger order cannot only reduce positions. |
| 22043 | ADL processing，{0} is limit close position |
| 22044 | ADL processing，cannot flash close Position |
| 22045 | Insufficient liquidity in the market, please operate later |
| 22067 | ADL processing，forbid operate the symbol:{0} |
| 31001 | The user is not a trader |
| 31002 | Condition {0} is not satisfied |
| 31003 | Parameter {0} must have a value, cannot be empty |
| 31004 | Take profit price must be > current price |
| 31005 | Stop loss price must be < current price |
| 31006 | The order is in the process of being placed, closing of the position is not supported at the moment |
| 31007 | Order does not exist |
| 31008 | There is no position in this position, no take profit or stop loss order can be made |
| 31009 | Tracking order status error |
| 31010 | Clear user prompt |
| 31011 | The order is not completely filled and the order is closed prompting the cancellation of the commission |
| 31012 | Pullback greater than {0} |
| 31013 | Pullback range is less than {0} |
| 31014 | Stop gain yield greater than {0} |
| 31015 | Stop loss yield less than {0} |
| 31016 | Batch execution exception |
| 31017 | Maximum price limit exceeded {0} |
| 31018 | Minimum price change of {0} |
| 31019 | Support trading currency pair does not exist |
| 31020 | Business is restricted |
| 31021 | The currency pair is not available for trading, please select another currency pair |
| 31022 | Minimum order size for this trading area is not met, please select another trading area |
| 31023 | Ending order processing |
| 31024 | The order is not completely filled, please go to \\"Spot trading\\"-\\"Current orders\\" to cancel the order and then sell or close the operation! |
| 31025 | The user is not a trader |
| 31026 | The user is not exist |
| 31028 | Parameter verification failed |
| 31029 | User is not existed |
| 31030 | Chosen trading pair is empty |
| 31031 | You’re log in as trader,can not follow trade |
| 31032 | Can not follow trade with yourself |
| 31033 | Fail to remove |
| 31034 | This trader’s no. of follower has reached limit, please select other trader |
| 31035 | Follow order ratio can not less than{0} |
| 31036 | Follow order ratio can not greater than{0} |
| 31037 | Follow order count can not less than{0} |
| 31038 | Exceeds max. limit |
| 31039 | Can not set reminder as your Elite Trader status has been revoked |
| 31040 | T/P ratio must between {0}%%-{1}%% |
| 31041 | S/L ratio must between {0}%%-{1}%% |
| 31042 | The status of your Elite Trader has been suspended, please contact online customer service to resume. |
| 31043 | Your copy trade follower cap is too high. Please contact customer support to lower it if you want to enable this function! |
| 31044 | You are applying to become a trader now. Copying trade is not allowed |
| 31045 | The max. quantity for TP/SL is {0}. For any quantity exceeding this limit, please operate under “Initiated Copies”. |
| 31046 | No copy trade relationship is allowed between a parent account and its sub-account |
| 31047 | No copying is allowed within {0} minutes after the copier has been removed. Please try again later. |
| 31048 | Only this trader's referrals are allowed to follow this trader at the moment. Please create an account with the trader's referral link! |
| 31049 | The trader's status is abnormal or has been revoked, and cannot be viewed at this time! |
| 31050 | This trader UID is already set for the region. |
| 31051 | traderUserId error |
| 31052 | Cannot set trading symbol that have not been opened by traders. |
| 31053 | executePrice cannot exceed triggerPrice 的{0} |
| 31054 | No order to cancel |
| 31057 | user has not follow order |
| 40000 | Bitget is providing services to many countries and regions around the world and strictly adheres to the rules and regulatory requirements of each country and region. According to the relevant regulations, Bitget is currently unable to provide services to your region (Mainland China) and you do not have access to open positions.Apologies for any inconvenience caused! |
| 40001 | ACCESS\_KEY cannot be empty |
| 40002 | ACCESS\_SIGN cannot be empty |
| 40003 | Signature cannot be empty |
| 40005 | Invalid ACCESS\_TIMESTAMP |
| 40006 | Invalid ACCESS\_KEY |
| 40007 | Invalid Content\_Type |
| 40008 | Request timestamp expired |
| 40009 | sign signature error |
| 40010 | Request timed out |
| 40011 | ACCESS\_PASSPHRASE cannot be empty |
| 40012 | apikey/password is incorrect |
| 40013 | User status is abnormal |
| 40014 | Incorrect permissions, need {0} permissions |
| 40015 | System is abnormal, please try again later |
| 40016 | The user must bind the phone or Google |
| 40017 | Parameter verification failed {0} |
| 40018 | Invalid IP |
| 40019 | Parameter {0} cannot be empty |
| 40020 | Parameter {0} error |
| 40021 | User disable withdraw |
| 40022 | The business of this account has been restricted |
| 40023 | API service has been restricted. For any inquiries, please contact customer service |
| 40024 | Account has been frozen |
| 40025 | The user does not have this permission |
| 40026 | User is disabled |
| 40027 | Withdrawals in this account area must be kyc |
| 40028 | This subUid does not belong to this account |
| 40029 | This account is not a Broker, please apply to become a Broker first |
| 40031 | The account has been cancelled and cannot be used again |
| 40032 | The Max of sub-account created has reached the limit |
| 40033 | This email has been bound |
| 40034 | Parameter {0} does not exist |
| 40035 | Judging from your login information, you are required to complete KYC first for compliance reasons. |
| 40036 | passphrase is error |
| 40037 | Apikey does not exist |
| 40038 | The current ip is not in the apikey ip whitelist |
| 40039 | FD Broker's user signature error |
| 40040 | user api key permission setting error |
| 40041 | User's ApiKey does not exist |
| 40043 | FD Broker does not exist |
| 40045 | The bound user cannot be an FD broker |
| 40047 | FD Broker binding related interface call frequency limit |
| 40048 | The user's ApiKey must be the parent account |
| 40049 | User related fields decrypt error |
| 40051 | This account is not a FD Broker, please apply to become a FD Broker first |
| 40052 | Security settings have been modified for this account. For the safety of your account, withdrawals are prohibited within 24 hours |
| 40053 | Value range verification failed: {0} should be between {1} |
| 40054 | The data fetched by {0} is empty |
| 40055 | subName must be an English letter with a length of 8 |
| 40056 | remark must be length of 1 ~ 20 |
| 40057 | Parameter {0} {1} does not meet specification |
| 40058 | Parameter {0} Only a maximum of {1} is allowed |
| 40059 | Parameter {0} should be less than {1} |
| 40060 | subNames already exists |
| 40061 | sub-account not allow access |
| 40063 | API exceeds the maximum limit added |
| 40064 | Parameter verification failed |
| 40065 | This subApikey does not exist |
| 40066 | This subUid does not belong to the account or is not a virtual sub-account |
| 40067 | Account creation failed |
| 40068 | Disable subaccount access |
| 40069 | The maximum number of sub-accounts created has been reached |
| 40070 | passphrase 8-32 characters with letters and numbers |
| 40071 | subName exist duplication |
| 40072 | symbol {0} is Invalid or not supported mix contract trade |
| 40074 | {0} MatchRunServer not exist |
| 40077 | Transfers from custody sub-accounts are only allowed between spot and contract accounts. |
| 40078 | Timestamp for this request is outside of the ME receiveWindow. |
| 40079 | receiveWindow timestamp must be less than 60s |
| 40081 | Spot DEMO accounts can only access spot orders and spot plan order endpoints. |
| 40086 | An error occurred when accessing the VIP private domain name. Please check whether you have applied for it. |
| 40100 | Due to regulatory requirements, Hong Kong IPs are required to complete identity verification first |
| 40101 | Please complete KYC |
| 40102 | Symbol does not exist |
| 40103 | based on your IP address , it appears that you are located in a country or region where we are currently unable to provide services |
| 40104 | Based on your IP address , it appears that you are located in a country or region where we are currently unable to provide services |
| 40104 | Unable to withdraw to this account Please make sure this is a valid and verified account |
| 40105 | Currently Spot Demo trade does not support this feature |
| 40107 | The subaccountName has been used |
| 40108 | The subaccountName has been used |
| 40109 | The data of the order cannot be found, please confirm the order number |
| 40110 | It is currently unavailable in a demo trading |
| 40111 | If you are located in a country where our services are restricted, please complete the KYC verification. If you have any questions, please contact customer service. |
| 40199 | Traders are prohibited from calling the API |
| 40200 | Server upgrade, please try again later |
| 40301 | Permission has not been obtained yet. If you need to use it, please contact customer service |
| 40303 | Can only query up to 20,000 data |
| 40304 | clientOid or clientOrderId length cannot greater than 50 |
| 40305 | clientOid or clientOrderId length cannot greater than 64, and cannot be Martian characters |
| 40306 | Batch processing orders can only process up to 20 |
| 40309 | The contract has been removed |
| 40402 | orderId or clientOId format error |
| 40404 | Request URL NOT FOUND |
| 40407 | The query direction is not the direction entrusted by the plan |
| 40408 | Range error |
| 40409 | wrong format |
| 40704 | Can only check the data of the last three months |
| 40705 | The start and end time cannot exceed 90 days |
| 40706 | Wrong order price |
| 40707 | Start time is greater than end time |
| 40710 | The Account has been frozen. |
| 40714 | No direct margin call is allowed |
| 40715 | delegate count can not high max of open count |
| 40716 | This trading pair not support Cross Margin mode |
| 40717 | The number of closed positions cannot exceed the number of sheets held |
| 40718 | The entrusted price of Pingduo shall not be lower than the bursting price |
| 40719 | Flat empty entrustment price is not allowed to be higher than explosion price |
| 40720 | swap hand depth does not exist |
| 40721 | Market price list is not allowed at present |
| 40722 | Due to excessive price fluctuations and the insufficient market price entrusted cost, the opening commission is failed. |
| 40723 | The total number of unexecuted orders is too high |
| 40724 | Parameter is empty |
| 40725 | service return an error |
| 40730 | There is currently a commission or a planned commission, and the leverage cannot be adjusted |
| 40732 | Not currently a trader |
| 40734 | Failed to place an order, the minimum number of traders to open a position {0} |
| 40744 | The tracking order status is wrong |
| 40746 | The current maximum number of positions that can be closed is {0}, if you exceed the number, please go to the current order to close the position |
| 40748 | The commission price is higher than the highest bid price |
| 40752 | You are disabled for current business, if you have any questions, please contact customer service |
| 40753 | The contract transaction business is disabled, if you have any questions, please contact customer service |
| 40754 | balance not enough |
| 40755 | Not enough open positions are available. |
| 40756 | The balance lock is insufficient. |
| 40757 | Not enough position is available. |
| 40758 | The position lock is insufficient. |
| 40760 | Account abnormal status |
| 40761 | The total number of unfilled orders is too high |
| 40762 | The order size is greater than the max open size |
| 40764 | The remaining amount of the order is less than the current transaction volume |
| 40765 | The remaining volume of the position is less than the current transaction volume |
| 40766 | The number of open orders is less than this transaction volume |
| 40775 | The market-making account can only be a unilateral position type. |
| 40778 | Coin pair {0} does not support {1} currency as margin |
| 40780 | There are multiple risk handling records for the same symbolId at the same time |
| 40781 | The transfer order was not found |
| 40782 | Internal transfer error |
| 40783 | No gear found |
| 40784 | Need to configure modify depth account |
| 40785 | Need to configure draw line account |
| 40788 | Internal batch transfer error |
| 40789 | The tokenId is duplicated in the configuration item |
| 40790 | Duplicate symbolCode in configuration item |
| 40791 | The baseToken or quoteToken of symbolCode does not exist |
| 40792 | The symbol in the configuration item is duplicated |
| 40793 | The symbolCode of BusinessSymbol does not exist |
| 40794 | The supportMarginToken of BusinessSymbol is not configured |
| 40798 | Insufficient contract account balance |
| 40799 | Cannot be less than the minimum transfer amount |
| 40800 | Insufficient amount of margin |
| 40801 | Cannot exceed the maximum transferable deposit amount |
| 40802 | Position is zero and direct margin call is not allowed |
| 40804 | The number of closed positions cannot exceed the number of positions held |
| 40805 | Unsupported operation |
| 40806 | Unsupported currency |
| 40807 | The account does not exist |
| 40808 | Parameter verification exception {0} |
| 40811 | The parameter {0} should not be null |
| 40812 | The condition {0} is not met |
| 40813 | The parameter {0} must have a value and cannot be empty |
| 40814 | No change in leverage |
| 40815 | The order price is higher than the highest bid price |
| 40816 | The order price is lower than the lowest selling price |
| 40820 | The order price for closing a long position is not allowed to be lower than the liquidation price |
| 40821 | The closing order price cannot be higher than the liquidation price |
| 40822 | The contract configuration does not exist |
| 40823 | The transaction or reasonable marked price does not exist |
| 40824 | Currently, it is not allowed to list market orders |
| 40825 | Contract opponent depth does not exist |
| 40826 | Due to excessive price fluctuations, the market order cost is insufficient, and the position opening order failed. |
| 40827 | The bonus is not allowed to hold two-way positions |
| 40828 | Special market making accounts cannot manually place orders |
| 40829 | The take profit price of a long position should be greater than the average open price |
| 40830 | The take profit price of the long position should be greater than the current price |
| 40831 | The short position take profit price should be less than the average open price |
| 40832 | The take profit price of short positions should be less than the current price |
| 40833 | The stop loss price of a long position should be less than the average opening price |
| 40834 | The stop loss price of the long position should be less than the current price |
| 40835 | The stop loss price of the short position should be greater than the average opening price |
| 40836 | The stop loss price of the short position should be greater than the current price |
| 40837 | There is no position in this position, so stop-profit and stop-loss orders cannot be made |
| 40838 | There is no position in this position, and automatic margin call cannot be set |
| 40839 | The automatic margin call function of this contract has been suspended |
| 40840 | Duplicate shard market making account |
| 40841 | Online environment does not allow execution |
| 40842 | Current configuration does not allow adjustment, please try again later |
| 40843 | no\_datasource\_key\_exists |
| 40844 | This contract is under temporary maintenance |
| 40845 | This contract has been removed |
| 40846 | Status verification abnormal |
| 40847 | The operation cannot be performed |
| 40848 | Cannot open a copy transaction if there is a position |
| 40850 | The copy is in progress, the balance cannot be transferred |
| 40851 | Account status is wrong, cannot end copying |
| 40852 | There are unfilled orders, cannot end the copy |
| 40853 | There is an unexecuted plan order, cannot end the copy |
| 40854 | This product does not support copy trading |
| 40855 | The user has ended copying and cannot end copying again |
| 40856 | Data abnormal |
| 40857 | Document number error |
| 40858 | Error tracking order status |
| 40859 | This order is being closed and cannot be closed again |
| 40860 | The trader does not exist and cannot be set to follow |
| 40861 | The trader has been disabled and cannot be set to follow |
| 40862 | Please cancel the current order |
| 40863 | Please cancel the current plan |
| 40864 | Please close the current position with orders |
| 40865 | This order is being commissioned, and it is not currently supported to close the position |
| 40866 | You are currently a trader, please close the position under the current order |
| 40867 | Currently the maximum number of positions that can be closed is {0}, please go to the current order to close the position if the amount exceeds |
| 40868 | You are currently a trader and currently do not support liquidation through planned orders |
| 40869 | You are currently a trader and currently do not support modification of leverage |
| 40871 | The leverage does not meet the configuration, and you cannot become a trader |
| 40872 | Failed to adjust position, currently holding position or order or plan order |
| 40873 | The account has a margin and needs to be transferred out |
| 40874 | Whole position mode does not support automatic margin call |
| 40875 | Whole position mode does not support margin adjustment |
| 40877 | Too many follow-up orders |
| 40878 | The contract index data is abnormal. In order to avoid causing your loss, please try again later. |
| 40879 | The risk is being processed, and the funds cannot be adjusted. |
| 40880 | The risk is being processed and the leverage cannot be adjusted. |
| 40887 | Failed to place the order, the number of single lightning open positions is at most {0} |
| 40888 | Failed to place the order, the maximum amount of single lightning closing is {0} |
| 40889 | The plan order of this contract has reached the upper limit |
| 40890 | The order of stop-profit and stop-loss for this contract has reached the upper limit |
| 40891 | Insufficient position, can not set take profit or stop loss |
| 40892 | Failed to place the order, the minimum number of positions opened by the trader is {0} |
| 40893 | Unable to update the leverage factor of this position, there is not enough margin! |
| 40894 | The documentary closing has been processed |
| 40895 | The preset price does not match the order/execution price |
| 40896 | The default stop profit and stop loss has been partially fulfilled and cannot be modified |
| 40897 | The system experience gold account does not exist |
| 40898 | The system experience gold account balance is insufficient |
| 40899 | The number of stored users exceeds the limit |
| 40900 | The system experience gold account is inconsistent |
| 40901 | The contract experience fund balance is insufficient |
| 40902 | Future time is not allowed |
| 40903 | Failed to obtain leverage information |
| 40904 | Failed to collect funds |
| 40905 | Failed to collect user funds |
| 40906 | Failed to pay user funds |
| 40907 | The payment cannot be transferred |
| 40908 | Concurrent operation failed |
| 40909 | Transfer processing |
| 40910 | Operation timed out |
| 40912 | single cancel cannot exceed 50 |
| 40913 | {0} must be passed one |
| 40914 | Trader the maximum leverage can use is {0} |
| 40915 | Long position take profit price please > mark price |
| 40916 | Futures services for this account have been restricted. |
| 40917 | Stop price for long positions please < mark price {0} |
| 40918 | Traders open positions with orders too frequently |
| 40919 | This function is not open yet |
| 40920 | Position or order exists, the position mode cannot be switched |
| 40921 | The order size cannot exceed the maximum size of the positionLevel |
| 40922 | Only work order modifications are allowed |
| 40923 | Order size and price have not changed |
| 40924 | orderId and clientOid must have one |
| 40925 | price or size must be passed in together |
| 40927 | The return field type or dest of this order does not meet expectations |
| 40928 | Risk control, currently your max open size is {0} {1}. The size was calculated with all the main-sub accounts |
| 40929 | TraderPro Maximum leverage is {0}X |
| 40930 | The remaining quantity for your normal order is {0}{1}, and the quantity for post only order is {2}{3} |
| 40931 | Trigger the risk control of position closing , prohibiting position closing |
| 40936 | The received red envelope of {1} USDT is restricted from transfer for 24 hours; the {0} USDT purchased via OTC is restricted from transfer for 24 hours; {2} USDT is temporarily frozen as the recharge has not reached the required number of confirmations |
| 40937 | Your available withdrawal amount is {0} |
| 40939 | Reducing positions only will decrease your position. Please cancel or modify the original pending order first before placing a new order |
| 41002 | param error {0} |
| 41100 | error {0} |
| 41101 | param {0} error |
| 41103 | param {0} error |
| 41104 | Unsupported coin: {0} |
| 42013 | transfer fail |
| 42014 | The current currency does not support deposit |
| 42015 | The current currency does not support withdrawal |
| 42016 | symbol {0} is Invalid or not supported spot trade |
| 42017 | The current chain does not support deposit the coin |
| 42072 | Param endTime cannot be earlier than startTime |
| 43001 | The order does not exist |
| 43002 | Pending order failed |
| 43003 | Pending order failed |
| 43004 | There is no order to cancel |
| 43005 | Exceed the maximum number of orders |
| 43006 | The order quantity is less than the minimum transaction quantity |
| 43007 | The order quantity is greater than the maximum transaction quantity |
| 43008 | The current order price cannot be less than {0}{1} |
| 43009 | The current order price exceeds the limit {0}{1} |
| 43010 | The transaction amount cannot be less than {0}{1} |
| 43011 | The parameter does not meet the specification {0} |
| 43012 | Insufficient balance |
| 43013 | Take profit price needs> current price |
| 43014 | Take profit price needs to be <current price |
| 43015 | Stop loss price needs to be <current price |
| 43016 | Stop loss price needs to be> current price |
| 43017 | You are currently a trader and currently do not support liquidation through planned orders |
| 43020 | Stop profit and stop loss order does not exist |
| 43021 | The stop-profit and stop-loss order has been closed |
| 43022 | Failed to trigger the default stop loss |
| 43023 | Insufficient position, can not set profit or stop loss |
| 43024 | Take profit/stop loss in an existing order, please change it after canceling all |
| 43025 | Plan order does not exist |
| 43026 | The planned order has been closed |
| 43027 | The minimum order value {0} is not met |
| 43028 | Please enter an integer multiple of {0} for price |
| 43029 | The size of the current Order > the maximum number of positions that can be closed |
| 43030 | Take profit order already existed |
| 43031 | Stop loss order already existed |
| 43032 | rangeRate is smaller than {0} |
| 43033 | Trailing order does not exist |
| 43034 | The trigger price should be ≤ the current market price |
| 43035 | The trigger price should be ≥ the current market price |
| 43036 | Trader modify tpsl can only be operated once within 300ms |
| 43037 | The minimum order amount allowed for trading is {0} |
| 43038 | The maximum order amount allowed for trading is {0} |
| 43039 | Maximum price limit exceeded {0} |
| 43040 | Minimum price limit exceeded {0} |
| 43041 | Maximum transaction amount {0} |
| 43042 | Minimum transaction amount {0} |
| 43043 | There is no position |
| 43044 | The follow order status error |
| 43045 | The trader is ful |
| 43047 | Followers are not allowed to follow again within xx minutes after being removed, please try again later! |
| 43048 | The symbol is null |
| 43049 | Margin coin is not allowed |
| 43050 | Leverage exceeds the effective range |
| 43051 | Maximum limit exceeded |
| 43052 | Follow order count can not less than {0} |
| 43053 | The copy ratio cannot exceed {0} |
| 43054 | The copy ratio cannot be less than {0} |
| 43055 | The take loss ratio must be between {0}-{1} |
| 43056 | The take profit ratio must be between {0}-{1} |
| 43057 | It is not allowed to bring orders or copy orders between sub-accounts |
| 43058 | Parameter verification failed |
| 43059 | Request failed, please try again |
| 43060 | Sort rule must send |
| 43061 | Sort Flag must send |
| 43062 | not to follow |
| 43063 | Can not follow trade with yourself |
| 43064 | Tracking order status error |
| 43065 | Tracking No does not exist |
| 43066 | The trader failed to remove the follower |
| 43067 | The loaded data has reached the upper limit, and the maximum support for loading {0} data |
| 43068 | The status of the current follower is abnormal and removal is not allowed for now |
| 43069 | A follower account can only be removed when its equity is lower than {0} USDT |
| 43071 | Trigger order limit for a single trading pair is {0} |
| 43075 | Position pattern mismatch |
| 43111 | param error {0} |
| 43112 | The amount of coins withdrawn is less than the handling fee {0} |
| 43113 | The daily limit {0} is exceeded in a single transaction |
| 43114 | Withdrawal is less than the minimum withdrawal count {0} |
| 43116 | This chain requires a tag to withdraw coins |
| 43117 | Exceeds the maximum amount that can be transferred |
| 43118 | clientOrderId duplicate |
| 43121 | Withdrawal address cannot be your own |
| 43122 | The purchase limit of this currency is {0}, and there is still {1} left |
| 43123 | param error {0} |
| 43124 | withdraw step is error |
| 43125 | No more than 8 decimal places |
| 43126 | This currency does not support withdrawals |
| 43127 | Sub transfer not by main account, or main/sub relationship error |
| 43128 | Exceeded the limit of the maximum number of orders for the total transaction pair {0} |
| 43129 | Transfer coin not support or invalid coin |
| 43130 | StartTime params error |
| 43131 | Current currency: {0} does not support convert |
| 43132 | {0}Insufficient funds |
| 43134 | Transfers from custody sub-accounts are only allow transfers from spot accounts |
| 43136 | The transferred account is frozen |
| 43137 | Transfer in progress |
| 45001 | Unknown error |
| 45002 | Insufficient asset |
| 45003 | Insufficient position |
| 45004 | Insufficient lock-in asset |
| 45005 | Insufficient available positions |
| 45006 | Insufficient position |
| 45007 | Insufficient lock position |
| 45008 | No assets |
| 45009 | The account is at risk and cannot perform trades temporarily |
| 45011 | Order remaining volume < Current transaction volume |
| 45012 | Remaining volume of position < Volume of current transaction |
| 45013 | The number of open orders < Current transaction volume |
| 45014 | Position does not exist during opening |
| 45017 | Settlement or the coin for transaction configuration not found |
| 45018 | In the case of a netting, you cannot have a liquidation order |
| 45019 | Account does not exist |
| 45020 | Liquidation can only occur under two-way positions |
| 45021 | When one-way position is held, the order type must also be one-way position type |
| 45023 | Error creating order |
| 45024 | Cancel order error |
| 45025 | The currency pair does not support the currency as a margin |
| 45026 | Please check that the correct delegateType is used |
| 45031 | The order is finalized |
| 45034 | clientOid duplicate |
| 45035 | Price step mismatch |
| 45043 | Due to settlement or maintenance reasons, the trade is suspended |
| 45044 | Leverage is not within the suitable range after adjustment |
| 45045 | Exceeds the maximum possible leverage |
| 45047 | Reduce the leverage and the amount of additional margin is incorrect |
| 45051 | Execution price parameter verification is abnormal |
| 45052 | Trigger price parameter verification anbormal |
| 45054 | No change in leverage |
| 45055 | The current order status cannot be cancelled |
| 45056 | The current order type cannot be cancelled |
| 45057 | The order does not exist! |
| 45060 | TP price of long position > Current price {0} |
| 45061 | TP price of short position < Current price {0} |
| 45062 | SL price of long position < Current price {0} |
| 45064 | TP price of long position > order price {0} |
| 45065 | TP price of short position < order price {0} |
| 45066 | SL price of long position < order price {0} |
| 45067 | SL price of short position > order price {0} |
| 45068 | There is no position temporarily, and the order of TP and SL cannot be carried out |
| 45075 | The user already has an ongoing copy trade |
| 45082 | Copy trade number error |
| 45089 | You are currently copy trading, leverage cannot be changed |
| 45091 | Too many tracking orders |
| 45097 | There is currently an order or a limit order, and the leverage cannot be adjusted |
| 45098 | You are currently a trader and cannot be switched to the full position mode |
| 45099 | When there are different coins, it cannot be adjusted to Isolated Margin mode |
| 45100 | When a one-way position is held, it cannot be adjusted to the Isolated Margin mode |
| 45101 | In Isolated Margin mode, it cannot be adjusted to a one-way position |
| 45102 | In the full position mode, the automatic margin call cannot be adjusted |
| 45103 | Failed to place the order, the maximum amount of single flash opening position is %s |
| 45104 | Failed to place the order, the maximum amount of single flash closing position is %s |
| 45107 | API is restricted to open positions. If you have any questions, please contact our customer service |
| 45108 | API is restricted to close position. If you have any questions, please contact our customer service |
| 45109 | The current account is a two-way position |
| 45110 | less than the minimum amount {0} USDT |
| 45111 | less than the minimum order quantity |
| 45112 | more than the maximum order quantity |
| 45113 | Maximum order value limit triggered |
| 45114 | The minimum order requirement is not met |
| 45115 | The price you enter should be a multiple of {0} |
| 45116 | The count of positions hold by the account exceeds the maximum count {0} |
| 45117 | Currently holding positions or orders, the margin mode cannot be adjusted |
| 45118 | Reached the upper limit of the order of transactions (the current number of order + the current number of orders) {0} |
| 45119 | This symbol does not support position opening operation |
| 45120 | size > max can open order size |
| 45121 | The reasonable mark price deviates too much from the market, and your current leveraged position opening risk is high |
| 45122 | Short position stop loss price please > mark price {0} |
| 45123 | Insufficient availability, currently only market orders can be placed |
| 45124 | Please edit and submit again. |
| 45127 | Position brackets disabled TP SL |
| 45128 | Position brackets disabled modify qty |
| 45129 | Cancel order is too frequent, the same orderId is only allowed to be canceled once in a second |
| 46013 | This symbol limits the selling amount{0}，Remaining{0} |
| 47001 | Currency recharge is not enabled |
| 47002 | Address verification failed |
| 47003 | Withdraw address is not in addressBook |
| 48001 | Parameter validation failed {0} |
| 48002 | Missing request Parameter |
| 49000 | apiKey and userId mismatch |
| 49001 | not custody account, operation deny |
| 49002 | missing http header: ACCESS-BROKER-KEY or ACCESS-BROKER-SIGN |
| 49003 | illegal IP, access deny |
| 49004 | illegal ACCESS-BROKER-KEY |
| 49005 | access deny: sub account |
| 49006 | ACCESS-BROKER-SIGN check sign fail |
| 49007 | account is unbound |
| 49008 | account is bound |
| 49009 | clientUserId check mismatch with the bound user ID |
| 49010 | account: {0} still have assets: {1} |
| 49011 | kyc must be done before bind |
| 49021 | operation accepted |
| 49022 | access deny |
| 49023 | insufficient fund |
| 49024 | {0} decimal precision error |
| 49025 | Parameter mismatch with the initial requestId, request body: {0} |
| 49026 | {0} maximum {1} digits |
| 49030 | custody account, operation deny |
| 49040 | Unknown Error |
| 49050 | unsupported chain |
| 49051 | Missing callback signature request header |
| 49052 | callback signature verification failed |
| 49053 | can not bind other platforms |
| 49060 | The switch of adding money to cobo is not turned on |
| 49061 | The custody currency is not allowed |
| 49062 | fundId is invalid or not exist {0} |
| 49063 | The custody currency already exists |
| 49064 | Insufficient amount of shadow account |
| 49065 | User withdrawal address already exists |
| 49066 | The switch of cobo money reduction is not turned on |
| 49067 | fundSupplementId is invalid {0} |
| 49068 | No currency available for settlement |
| 49069 | There is an unfinished fund process, which cannot be cleared and settled |
| 49070 | Clearing settlement must include all currencies |
| 49071 | fundSettlementId is invalid {0} |
| 49072 | Failed to get user assets |
| 49073 | Confirm that the set of fundIds receivable for clearing and settlement is not all fundIds |
| 49074 | The settlement process has not been completed, and fund operations cannot be performed |
| 49075 | Failed to query the address list of bg clearing and settlement account |
| 49076 | cobo callback params error |
| 49077 | Failed to call the cobo transaction query interface |
| 49078 | cobo withdrawal transaction callback requestId is invalid {0} |
| 49079 | supplement type illegal |
| 49080 | cobo confirms settlement, txId is invalid |
| 49081 | Request amount parameter error |
| 50001 | coin {0} does not support cross |
| 50002 | symbol {0} does not support isolated |
| 50003 | coin {0} does not support isolated |
| 50004 | symbol {0} does not support cross |
| 50011 | Parameter verification error |
| 50012 | The account has been suspended or deleted. Please contact our Customer Support |
| 50013 | The account has been suspended and deleted. Please contact our Customer Support |
| 50014 | The account already exists |
| 50015 | Currently, sub-accounts cannot engage in margin trading |
| 50016 | The number of open orders is smaller than the minimum limit of the trading pair |
| 50017 | The number of open orders is bigger than the maximum limit of the trading pair |
| 50018 | The price must be 0 or higher |
| 50019 | The user is forbidden to trade. |
| 50020 | Insufficient balance |
| 50021 | The margin trading account does not exist |
| 50022 | The account is liquidated |
| 50023 | The account has been suspended due to abnormal behavior. Please contact our Customer Support is you have any questions. |
| 50024 | The trading pair does not exist |
| 50025 | The trading pair is currently unavailable |
| 50026 | The trading pair is currently unavailable |
| 50027 | The trading pair is suspended for maintenance |
| 50028 | The trading pair is removed |
| 50029 | The trading pair has no order price |
| 50030 | The trading pair will soon be available |
| 50031 | System error |
| 50032 | The currency does not exist |
| 50033 | The topic of the websocket query does not exist |
| 50034 | The borrowing amount must be over 0.00000001 |
| 50035 | The maximum borrowing amount is exceeded |
| 50036 | The loan configuration does not exist |
| 50037 | This currency cannot be borrowed |
| 50038 | The system limit is exceeded |
| 50039 | The currency and the trading pair do not match |
| 50040 | The repayment amount must be more than 0 |
| 50041 | The repayment amount must be less than your available balance |
| 50042 | The repayment amount must be more than the interest |
| 50043 | Unknown transaction type |
| 50044 | The system account is not found |
| 50045 | Insufficient locked asset |
| 50046 | The price is too low |
| 50047 | The price is too high |
| 50048 | The maximum number of orders is exceeded |
| 50049 | The request body of the system user is empty |
| 50050 | The system loan collection has been done |
| 50051 | The user in reconciliation is not in the system (cache) |
| 50052 | The asset balance will be less than 0 after transferring |
| 50053 | The amount is less than 0 when making loan repayment |
| 50054 | The amount is less than 0 when making interest repayment |
| 50055 | The amount is less than 0 when paying trading fees |
| 50056 | The amount is less than 0 when paying liquidation fees |
| 50057 | The amount is less than 0 when paying the excessive loss resulted from liquidation |
| 50058 | After the profit is used to cover the excessive loss resulted from liquidation, the balance will be less than 0 |
| 50059 | This currency cannot be transferred |
| 50060 | Duplicated clientOid |
| 50061 | There is a problem with the parameter you requested |
| 50062 | The order status is cancelled or fullFill |
| 50063 | Token precision must less than or equal to eight |
| 50064 | Your account is temporarily frozen. Please contact customer support if you have any questions |
| 50065 | symbol\_off\_shelf |
| 50066 | Position closing, please try again later |
| 50068 | {0} selling restriction: currently, you can sell {2} worth {1} USDT |
| 50077 | Can be convert {1} times every {0} hours |
| 50078 | The amount you can withdraw is {0} |
| 50081 | Exceeds the 24-hour net selling limit of {0}; currently, you can sell {2} worth {1} USDT |
| 59002 | Insufficient product balance |
| 59003 | This product is not available for purchase yet |
| 59005 | KYC verification not performed |
| 59006 | The country where KYC is located cannot apply for subscription |
| 59007 | Minimum limit for single currency subscription |
| 59008 | Maximum single currency subscription limit |
| 59009 | The subscription amount does not meet the step size verification |
| 59010 | The precision of the subscription amount cannot exceed {0} digits |
| 59011 | Insufficient balance |
| 59013 | Parameter exception: {0} |
| 59016 | The total position of a single person is exceeded |
| 59019 | The subscription time range is {0} ~ {1} |
| 59020 | Minimum limit for single subscription |
| 59021 | Redemption of the product has been suspended |
| 59022 | Insufficient balance |
| 59023 | Insufficient product remaining quota, remaining {0} |
| 59024 | Amount cannot be empty when redeeming current financial management |
| 59025 | orderId cannot be empty when redeeming regular financial management |
| 59026 | Parameter error |
| 59027 | This product is a novice product. You are not a novice user. Please choose another product. |
| 59029 | Product cannot be subscribe |
| 59030 | Exceeding the max amount for once subscribe |
| 59031 | Cannot perform redemption operation |
| 59033 | Less than redemption minimum limit |
| 59034 | The redemption amount accuracy cannot exceed {0} digits |
| 59035 | The redemption amount must be greater than the minimum limit |
| 59037 | The current order status does not allow operation |
| 59038 | Redemption is not allowed on the day of expiration |
| 59039 | Cannot perform redemption operation |
| 59040 | The redemption time range is {0}-{1} |
| 59041 | The accuracy of the subscription amount is not met |
| 59043 | Insufficient product remaining quota, remaining {0} {1} |
| 59044 | Operations are frequent, please try again later. |
| 59045 | subscription time range is {0}~{1} |
| 59046 | Transaction failed |
| 59047 | redemption time range is {0}-{1} |
| 59048 | fixed redemption not pass amount |
| 59049 | Product does not exist |
| 60001 | StartTime not empty |
| 60002 | MerchantId not empty |
| 60003 | Not found the p2p order |
| 60004 | Not found the p2p advertisement |
| 60005 | Not found the p2p merchant |
| 60006 | Parameter error |
| 60007 | upload image cannot exceed 5M |
| 60008 | The image format must be \[". jpg", ". jpeg", ". png"\] |
| 60009 | The image format error |
| 60010 | upload error |
| 60011 | Ordinary users can not post ads |
| 60012 | Please change your status from offline to online before posting your ads！ |
| 60013 | Insufficient balance |
| 60014 | Fiat info not found |
| 60015 | Digital currency info not found |
| 60016 | Only supports publish CNY advertisement |
| 60017 | Not support publish CNY advertisement |
| 60018 | Your KYC certification only supports publishing {0} |
| 60019 | Post failed. Unable to obtain preference price |
| 60020 | advertisement type error |
| 60021 | Payment method is empty |
| 60022 | Trading amount incorrect |
| 60023 | Beyond fiat limit ({0}-{1}) |
| 60024 | Abnormality occurred in the P2P merchant fund refund |
| 60025 | The remark length cannot be longer than the configuration length |
| 60026 | Exclusive country error |
| 60027 | Payment time limit error |
| 60028 | Payment method error |
| 60029 | publish advertisement error |
| 60030 | status error |
| 60031 | The advertisement number is too long |
| 60032 | The advertisement not exist |
| 60033 | Posted ad amount incorrect |
| 60034 | Number of images attached in the remark cannot exceed the allocation limit. |
| 60035 | Edit advertisement error |
| 60036 | payTimeLimit cannot be empty |
| 60037 | Post failed. Price is significantly deviated from preference price |
| 60038 | Post failed. Incorrect floating rate |
| 60039 | User does not exist |
| 60040 | Unauthorized access not supported |
| 60041 | Edit advertisement price error |
| 60042 | limitPrice not empty |
| 60043 | The advertisement status update fail |
| 60044 | The advertisement status in editing can be edited |
| 60045 | Exceeding the number of advertisement that can be published |
| 60046 | priceValue not empty |
| 60047 | userPayMethodId not empty |
| 60049 | You are not currently a merchant |
| 70001 | Activity ID not correct |
| 70002 | rankType error |
| 70006 | Parameter value range verification failed: {0} |
| 70008 | Parameter verification failed: {0}, please make sure the time is within 30 days |
| 70020 | Account does not exist |
| 70101 | illegal parameter |
| 70102 | Parameter verification failed-brokerUserId |
| 70103 | Parameter verification failed-startTime |
| 70104 | Parameter verification failed-endTime |
| 70204 | The account has open order, please cancel the open order. |
| 70205 | Today's reset has exceeded the maximum number of resets for the day: {0} and cannot be reset. |
| 70206 | Not main Account |
| 70207 | UID {0} not exist |
| 70208 | {0} can not deposit or withdrawal, please wait. |
| 70209 | Risk control, please contact with CS service. |
| 70210 | Exchange fail, please contact with CS service. |
| 70211 | {0} balance exceeds {1} USDT |
| 70212 | please less than {0} USDT |
| 70213 | The withdrawal amount exceeds the daily limit |
| 70214 | Restricted assets exist |
| 70215 | Frozen assets exist |
| 70216 | locked assets exist |
| 70217 | The whitelist is turned on, withdraw address is not in addressBook |
| 70218 | The transaction quantity of pending orders is higher than the modified quantity |
| 70219 | Withdrawal exceeds monthly limit |
| 70220 | Insufficient liquidity in the market, please operate later |
| 70221 | The current currency is {0}, the net purchase value is limited to {1} USD, and you can also purchase {3} with a net purchase value of {2} USD. |
| 70222 | The current currency is {0}, the net purchase quantity is limited to {1}, and the net purchase quantity is also {2} |
| 70223 | Exceeding the maximum number of orders for total trading |
| 70224 | This currency has a selling limit of {0}, leaving {1} |
| 70225 | This currency has a buying limit of {0}, leaving {1} |
| 70226 | The current maximum amount of coins that can be withdrawn or transferred out is {0}. If you want to withdraw or transfer all coins, please confirm that all spot orders have been ended. |
| 70227 | The user does not allow to place order |
| 70228 | The operation is too frequent, please try again later. |
| 70229 | Place order error |
| 80001 | illegal params |
| 80002 | system error |
| 80003 | Loan coin not exist |
| 80004 | Place coin not exist |
| 80005 | Place single minimum limit |
| 80006 | Place single Maximum limit |
| 80007 | Loan single minimum limit |
| 80008 | Loan single maximum limit |
| 80009 | Loan pool not enough |
| 80010 | place float exceed |
| 80011 | Order not exist |
| 80012 | Pledge not exist |
| 80013 | Extract exceed maximum limit |
| 80014 | Operate limit amount is {0} USDT |
| 80015 | Order count maximum limit |
| 80016 | Order status illegal |
| 80020 | The minimum number of operations is {0} {1} |
| 90001 | The single subscription limit cannot be exceeded {0} |
| 400172 | Parameter verification failed |

---

- [Margin Trading API](https://www.bitget.com/api-doc/margin/intro)
# WebSocket Error Code

| Error Code | Description |
| --- | --- |
| 429 | Too Many Requests |
| 25000 | System error, please try again later |
| 25001 | Operation timed out |
| 25002 | Unsupported operation |
| 25003 | Concurrent operation, please retry |
| 25004 | Operation too frequent, please try again later |
| 25005 | User does not exist |
| 25006 | Abnormal account status |
| 25007 | Account in an irregular status |
| 25008 | Account is in forced liquidation status |
| 25009 | Unsupported account mode switch |
| 25010 | Unsupported position mode switch |
| 25011 | Transfer failed, account is in forced liquidation status |
| 25012 | Account at risk, trading temporarily disabled |
| 25013 | {0} not support API trade |
| 25100 | Trading pair {0} does not exist |
| 25101 | Trading pair not open for trading |
| 25102 | Trading pair temporarily closed for maintenance |
| 25104 | Contract delisted |
| 25105 | This contract does not support opening positions |
| 25107 | There is currently a position, please close the position |
| 25108 | The contract is in the initialization state |
| 25110 | This coin is not supported for deposits into the unified account |
| 25200 | {0} validation error |
| 25201 | Currency does not exist |
| 25202 | Insufficient balance |
| 25203 | Insufficient margin |
| 25204 | Order does not exist |
| 25205 | {0} trading price cannot be below {1}% |
| 25206 | {0} trading price cannot exceed {1}% |
| 25207 | {0} trading quantity cannot be less than {1} units |
| 25208 | {0} trading quantity cannot exceed {1} units |
| 25209 | Insufficient market liquidity, please try again later |
| 25210 | Large price fluctuation, placing the order entails higher risk, please reorder |
| 25211 | Exceeds the limit of {0} maximum orders |
| 25212 | Duplicate clientOid |
| 25213 | Exceeds {0} permanent net buy limit, you can currently buy up to {1} USD worth of {0} |
| 25214 | Exceeds {0} permanent net sell limit, you can currently sell up to {1} USD worth of {0} |
| 25215 | Exceeds {0} 24-hour net buy limit, you can currently buy up to {1} USD worth of {0} |
| 25216 | Exceeds {0} 24-hour net sell limit, you can currently sell up to {1} USD worth of {0} |
| 25217 | Exceeds maximum repayment limit |
| 25218 | High risk, usage may result in forced liquidation |
| 25219 | Increase in IMR exceeds available assets |
| 25220 | 未找到描述 |
| 25221 | Level gradient does not exist |
| 25222 | Trade or fair mark price does not exist |
| 25223 | Exceeds Max. leverage |
| 25224 | Take-profit or stop-loss triggered warning, please proceed with cancellation |
| 25225 | Insufficient available quantity in position |
| 25226 | Insufficient total position quantity |
| 25227 | No position available to close |
| 25228 | Unable to update leverage for this position, insufficient margin! |
| 25229 | Total positions exceed the current limit of {0} positions |
| 25230 | Order quantity exceeds the maximum open quantity |
| 25231 | Close quantity cannot exceed the held position quantity |
| 25232 | Reduce-only orders will only reduce your position; please cancel or modify existing orders before placing a new one |
| 25233 | Order quantity cannot exceed the maximum for this level |
| 25234 | Remaining quantity for regular orders is {0} {1}, and for post-only orders it is {2} {3}. |
| 25235 | Due to risk control, the maximum open position currently allowed is {0} {1}. The risk control limit for a single user includes all primary and sub-accounts |
| 25236 | Incorrect position open type |
| 25237 | Close orders can only occur in bi-directional mode |
| 25238 | {0} do not assign values at the same |
| 25239 | Closing failed, please try again later |
| 25240 | {0} does not support this operation |
| 25241 | Bulk orders cannot exceed the corresponding maximum order value. |
| 25242 | The maximum reducible amount is {0}{1}. |
| 25243 | You have exceeded the currency holding limit and cannot add more of this currency. |
| 25244 | Price should be a multiple of {0} |
| 25245 | The account is not the unified account mode |
| 25559 | Not loan user |
| 25560 | Subaccount the same with RiskUnit ID |
| 25561 | SubUid not in risk unit |
| 25562 | The subacccount is the other's Risk Unit |
| 25563 | The UID you are unbinding has a non-zero balance and has unsettled or pending loan orders in its associated risk unit. |
| 25564 | The number of sub-accounts under the risk unit has exceeded the limit. |
| 25565 | The sub-account UID you entered is already bound to another risk unit. |
| 25566 | The risk unit does not exist. |
| 25567 | Exceeded the maximum quantity of contract orders：{0} {1} |
| 25568 | The order does not meet the modification requirements. |
| 25569 | Exceeded the maximum limit for modifications. |
| 25570 | There are pending orders for contract bidirectional closing or contract unidirectional opening/reducing positions. |
| 25571 | The modification price and qty is the same as the original value. Please adjust and try again. |
| 25572 | Too many pending modification requests. Please try again later |
| 25573 | Skipped due to prior modification failure |
| 25574 | Reduce-only order protection |
| 25575 | Failed to stop the strategy because the corresponding strategy ID could not be found |
| 25576 | Failed to stop the strategy because it is already stopping |
| 25577 | Failed to stop the strategy because the corresponding strategy ID could not be found |
| 25578 | No strategies available to stop |
| 25579 | For long position TP/SL (close long), the take-profit trigger price must be greater than the average entry price |
| 25580 | For long position TP/SL (close long), the stop-loss trigger price must be less than the average entry price |
| 25581 | For short position TP/SL (close short), the take-profit trigger price must be less than the average entry price |
| 25582 | For short position TP/SL (close short), the stop-loss trigger price must be greater than the average entry price |
| 25583 | Take-profit price must be greater than 0 |
| 25584 | Stop-loss price must be greater than 0 |
| 25585 | For long position TP/SL (close long), the take-profit trigger price must be greater than the latest price |
| 25586 | For long position TP/SL (close long), the stop-loss trigger price must be less than the latest price |
| 25587 | For short position TP/SL (close short), the take-profit trigger price must be less than the latest price |
| 25588 | For short position TP/SL (close short), the stop-loss trigger price must be greater than the latest price |
| 25589 | For long position TP/SL (close long), the take-profit trigger price must be greater than the mark price |
| 25590 | For long position TP/SL (close long), the stop-loss trigger price must be less than the mark price |
| 25591 | For short position TP/SL (close short), the take-profit trigger price must be less than the mark price |
| 25592 | For short position TP/SL (close short), the stop-loss trigger price must be greater than the mark price |
| 25593 | The current trading type does not support TP/SL settings |
| 25594 | Only one TP/SL can be set for all positions of the current symbol |
| 25595 | The total TP/SL quantity for partial positions exceeds the position amount |
| 25596 | Cannot modify strategy in triggered or error state |
| 25597 | Strategy does not exist or has already been stopped |
| 25598 | Current strategy status does not support modification |
| 25599 | Maximum number of active TP/SL orders for this symbol is {0}; cannot add more |
| 25600 | No changes made to the TP/SL order; please modify before submitting |
| 25601 | Position does not exist; cannot set TP/SL |
| 25602 | Take-profit trigger price must be greater than 0 |
| 25603 | Stop-loss trigger price must be greater than 0 |
| 25604 | Take-profit order price must be greater than 0 |
| 25605 | Stop-loss order price must be greater than 0 |
| 25606 | Take-profit trigger price does not meet precision requirements |
| 25607 | Stop-loss trigger price does not meet precision requirements |
| 25608 | Take-profit order price does not meet precision requirements |
| 25609 | Stop-loss order price does not meet precision requirements |
| 25610 | Order amount does not meet precision requirements |
| 25611 | Order quantity must be greater than the minimum order amount |
| 25612 | Only opening orders are allowed to have preset take-profit and stop-loss |
| 25620 | permission denied |
| 25622 | The collateral setting for institutional loans sub-accounts must be in all-coins mode |
| 25650 | For long positions, the stop-loss price must be lower than the current price: {0} |
| 25651 | For short positions, the take-profit price must be lower than the current price: {0} |
| 25652 | For long positions, the take-profit price must be greater than the current price: {0} |
| 25653 | The order has been changed. Please try again later |
| 25654 | {0} trading price cannot be greater than the bankruptcy price of {1} {2} |
| 25655 | {0} trading price cannot be lower than the bankruptcy price of {1} {2} |
| 40001 | ACCESS\_KEY cannot be empty |
| 40002 | SECRET\_KEY cannot be empty |
| 40003 | Signature cannot be empty |
| 40006 | Invalid ACCESS\_KEY |
| 40008 | Request timestamp expired |
| 40009 | sign signature error |
| 40017 | Parameter verification failed {0} |
| 40034 | Parameter {0} does not exist |
| 40725 | service return an error |
| 43117 | Exceeded Max. transferable quantity |
| 95001 | The current user is undergoing forced liquidation. |
| 95002 | The sub-account has contract positions and cannot be added. |
| 95003 | he sub-account UID you entered does not exist. |
| 95004 | The UID you unbound has a non-zero asset balance and is in a risk unit with unsettled or pending loan orders. |
| 95005 | The LTV for ins loan has exceeded the limit, and opening contracts is prohibited |
| 95006 | The LTV for ins loan has exceeded the limit, and spot buying is prohibited |
| 95007 | Ins loan is not support this spot trading pair. |
| 95008 | Ins loan is not support futures trading. |
| 95009 | Ins loan is not support this margin trading pair. |
| 95010 | Your entered subaccount UID is already bound to another risk unit. |
| 95011 | Parameter validation failed:{0} |
| 95012 | The leverage {1} of the sub-account contract position for {0} exceeds max leverage {2} for ins loan |
| 95013 | The leverage {1} of the sub-account contract order trading pair {0} exceeds max leverage {2} for ins loan.", |
| 95014 | The sub-account has contract orders and cannot be added |

---

