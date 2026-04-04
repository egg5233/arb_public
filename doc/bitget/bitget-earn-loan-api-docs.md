# Bitget Earn — Loan API Documentation

Crawled from https://www.bitget.com/api-doc/earn/loan/

---

## Repo Usage Quick Reference

- Primary repo use: usually reference-only; not part of the normal funding-arb execution path unless adapter code explicitly adds it
- Repo symbol context:
  - repo trading symbols look like `BTCUSDT`
  - earn-loan APIs operate mainly on currencies such as `BTC`, `ETH`, `USDT`
- Most relevant sections if this repo ever uses them:
  - borrow
  - currency list / borrowable assets
  - loan order state and repayment flows
- Important repo note: do not confuse Bitget earn-loan borrowing with the cross-margin borrow path used by the spot-futures engine; they are operationally different products

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Borrow

Frequency limit: 10c/1s (UID)

### Description

Borrow coin

### HTTP Request

- POST /api/v2/earn/loan/borrow

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/earn/loan/borrow" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json" 
 -d '{
    "loanCoin":"ETH",
    "pledgeCoin":"USDT",
    "daily":"SEVEN",
    "loanAmount":"0.01"
}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| loanCoin | String | Yes | Coin to loan, `ETH` |
| pledgeCoin | String | Yes | Pledge coin (Collateral), `USDT` |
| daily | String | Yes | Mortgage term<br>`SEVEN`: 7 days<br>`THIRTY`: 30 days <br>`FLEXIBLE`: Flexible |
| pledgeAmount | String | No | Pledge (Collateral) amount, `pledgeAmount` and `loanAmount` must send one |
| loanAmount | String | No | Loan amount, `pledgeAmount` and `loanAmount` must send one |

Response example

```json
{
  "code":"00000",
  "msg":"success",
  "requestTime":163123213132,
  "data": {
    "orderId": "1"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Currency List

Frequency limit: 10c/1s (IP)

### Description

Get loan-able currency list

### HTTP Request

- GET /api/v2/earn/loan/public/coinInfos

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/public/coinInfos" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| coin | String | No | Coin, `BTC` |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1692433281223,
  "data": {
    "loanInfos": [
      {
        "coin": "USDT",
        "hourRate7D": "0.00000617",
        "rate7D": "0.054",
        "hourRate30D": "0.00000879",
        "rate30D": "0.077",
        "minUsdt": "200",
        "maxUsdt": "1000000",
        "min": "200",
        "max": "1000000"
      }
  ],
  "pledgeInfos": [
    {
      "coin": "MATIC",
      "initRate": "0.6",
      "supRate": "0.75",
      "forceRate": "0.83",
      "minUsdt": "0",
      "maxUsdt": "200000"
    }
  ]
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanInfos | String | Loan infos |
| >coin | String | Loan coin |
| >hourRate7D | String | 7 days fixed rate per hour percentage |
| >rate7D | String | 7-day fixed rate annualized percentage |
| >hourRate30D | String | 30-day fixed rate hourly percentage |
| >rate30D | String | 30-day fixed rate annualized percentage |
| >minUsdt | String | Minimum Borrowable limit usdt |
| >maxUsdt | String | Maximum borrowing limit usdt |
| >min | String | Minimum borrowing limit |
| >max | String | Maximum borrowing limit |
| pledgeInfos | String | Pledge infos |
| >coin | String | Pledge coin |
| >initRate | String | Initial pledge rate percentage |
| >supRate | String | Percentage of supplementary guarantee pledge rate |
| >forceRate | String | Forced Liquidation Pledge Rate Percentage |
| >minUsdt | String | Minimum pledge limit usdt |
| >maxUsdt | String | Maximum pledge limit usdt |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Loan History

Frequency limit: 10c/1s (UID)

### Description

Get the list of loan history

### HTTP Request

- GET /api/v2/earn/loan/borrow-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/borrow-history?startTime=1685957902000&endTime=1691228302423" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | No | Order ID |
| loanCoin | String | No | Loan coin |
| pledgeCoin | String | No | Pledge (Collateral) coin |
| status | String | No | Status<br>`ROLLBACK`: failure<br>`FORCE`: force liquidation<br>`REPAY`: already repaid |
| startTime | String | Yes | Start time, ms, only supports querying the data of the past three months |
| endTime | String | Yes | End time, ms |
| pageNo | String | No | Page No default 1 |
| pageSize | String | No | Page size default 10，max 100 |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1684747525424,
  "data": [{
    "orderId": "12121212121",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "initPledgeAmount": "0.757",
    "initLoanAmount": "4321321.23820848",
    "hourRate": "59.1",
    "daily": "7",
    "borrowTime": "1684747528424",
    "status": "REPAY"
  }, {
    "orderId": "12121212121",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "initPledgeAmount": "0.757",
    "initLoanAmount": "4321321.23820848",
    "hourRate": "59.1",
    "daily": "7",
    "borrowTime": "1684747528424",
    "status": "REPAY"
  }]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanCoin | String | Loan coin |
| pledgeCoin | String | Pledge (Collateral) coin |
| orderId | String | Order ID |
| initPledgeAmount | String | Initial pledge amount |
| initLoanAmount | String | Initial loan amount |
| hourRate | String | hourly rate percentage |
| daily | String | Pledge days |
| borrowTime | String | Borrowed time |
| status | String | Status<br>`ROLLBACK`: failure<br>`FORCE`: force liquidation<br>`REPAY`: already repaid |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Liquidation Records

Frequency limit: 10c/1s (UID)

### Description

Get the list of repay history

### HTTP Request

- GET /api/v2/earn/loan/reduces

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/reduces?startTime=1685957902000&endTime=1691228302423" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | No | Order ID |
| loanCoin | String | No | Loan coin |
| pledgeCoin | String | No | Pledge (Collateral) coin |
| status | String | No | Status<br>`COMPLETE` completed liquidation<br>`WAIT` liquidating |
| startTime | String | Yes | Start time, ms, only supports querying the data of the past three months |
| endTime | String | Yes | End time, ms |
| pageNo | String | No | Page No, default 1 |
| pageSize | String | No | Page size, default 10, max 100 |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1684747525424,
  "data": [{
    "orderId": "1",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "reduceTime": "0.757",
    "pledgeRate": "98.2",
    "pledgePrice": "111.4",
    "status": "COMPLETE",
    "pledgeAmount": "1213.5",
    "reduceFee": "REPAY",
    "residueAmount": "3234.2",
    "runlockAmount": "23",
    "repayLoanAmount": "53.2"
  }, {
    "orderId": "12121212121",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "reduceTime": "0.757",
    "pledgeRate": "98.2",
    "pledgePrice": "111.4",
    "status": "COMPLETE",
    "pledgeAmount": "1213.5",
    "reduceFee": "REPAY",
    "residueAmount": "3234.2",
    "runlockAmount": "23",
    "repayLoanAmount": "53.2"
  }]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| loanCoin | String | Loan coin |
| pledgeCoin | String | Pledge (Collateral) coin |
| reduceTime | String | Liquidated time |
| pledgeRate | String | Pledge percentage during liquidation |
| pledgePrice | String | Pledge price when Liquidated |
| status | String | Status<br>`COMPLETE` liquidation completed<br>`WAIT` liquidating |
| pledgeAmount | String | Liquidation amount of pledged coin |
| reduceFee | String | Liquidation fee |
| residueAmount | String | Remaining pledged amount |
| runlockAmount | String | Released amount of pledged coin |
| repayLoanAmount | String | Repayment amount of the loan |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Debts

Frequency limit: 10c/1s (UID)

### Description

Get the list of repay history

### HTTP Request

- GET /api/v2/earn/loan/debts

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/debts?startTime=1685957902000&endTime=1691228302423" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

N/A

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1692436610750,
  "data": {
    "pledgeInfos": [
      {
        "coin": "USDT",
        "amount": "28826.61539642",
        "amountUsdt": "28826.61"
      }
    ],
    "loanInfos": [
      {
        "coin": "ETH",
        "amount": "11.00002748",
        "amountUsdt": "18730.85"
      }
    ]
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanInfos | String | Loan info |
| >coin | String | Coin |
| >amount | String | Amount |
| >amountUsdt | String | Amount usdt |
| pledgeInfos | String | Pledge Info |
| >coin | String | Coin |
| >amount | String | Amount |
| >amountUsdt | String | Amount usdt |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Est. Interest and Borrowable

Frequency limit: 10c/1s (IP)

### Description

Get Est. hourly interest rate and Borrowable amount

### HTTP Request

- GET /api/v2/earn/loan/public/hour-interest

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/public/hour-interest?loanCoin=USDT&pledgeCoin=ETH&pledgeAmount=0.2&daily=SEVEN" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| loanCoin | String | Yes | Coin to loan, `BTC` |
| pledgeCoin | String | Yes | Collateral coin, `ETH` |
| daily | String | Yes | Mortgage term<br>`SEVEN`: 7 days<br>`THIRTY`: 30 days<br>`FLEXIBLE`: Flexible |
| pledgeAmount | String | Yes | Pledge amount |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1692433739845,
  "data": {
    "hourInterest": "0.00133436",
    "loanAmount": "216.2654"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| hourInterest | String | Estimated interest amount per hour |
| loanAmount | String | Borrowable amount |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Loan Orders

Frequency limit: 10c/1s (UID)

### Description

Get on-going loan orders

### HTTP Request

- GET /api/v2/earn/loan/ongoing-orders

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/ongoing-orders?orderId=1" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | No | Order ID |
| loanCoin | String | No | Coin to loan |
| pledgeCoin | String | No | Pledge (Collateral) coin |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1692434611622,
  "data": [
    {
      "orderId": "1",
      "loanCoin": "ETH",
      "loanAmount": "1",
      "interestAmount": "0.00000229",
      "hourInterestRate": "0.000229",
      "pledgeCoin": "USDT",
      "pledgeAmount": "2619.69231032",
      "pledgeRate": "65",
      "supRate": "75",
      "forceRate": "83",
      "borrowTime": "1692434472156",
      "expireTime": "1693036799999"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| loanCoin | String | Coin to loan |
| loanAmount | String | Loan amount |
| interestAmount | String | Interest amount |
| hourInterestRate | String | Hour interest rate |
| pledgeCoin | String | Pledge (Collateral) coin |
| pledgeAmount | String | Pledge (Collateral) amount |
| pledgeRate | String | Pledge (Collateral) rate |
| supRate | String | Supplementary rate |
| forceRate | String | Forced Liquidation Pledge Rate Percentage |
| borrowTime | String | Borrow time millseconds |
| expireTime | String | Expire time millseconds |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Pledge Rate History

Frequency limit: 10c/1s (UID)

### Description

Get pledge rate history

### HTTP Request

- GET /api/v2/earn/loan/revise-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/revise-history?startTime=1685957902000&endTime=1691228302423" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | No | Order ID |
| reviseSide | String | No | Revise side<br>`down`: supplement collateral to turn down<br>`up`: withdraw collateral to turn up |
| pledgeCoin | String | No | Pledge (Collateral) coin |
| startTime | String | Yes | Start time, ms, only supports querying the data of the past three months |
| endTime | String | Yes | End time, ms |
| pageNo | String | No | pageNo default 1 |
| pageSize | String | No | pageSize default 10，max 100 |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1692436125845,
  "data": [
    {
      "loanCoin": "ETH",
      "pledgeCoin": "USDT",
      "orderId": "1",
      "reviseTime": "1692436102448",
      "reviseSide": "down",
      "reviseAmount": "10",
      "afterPledgeRate": "64.75",
      "beforePledgeRate": "65"
    }
  ]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanCoin | String | Loan coin |
| pledgeCoin | String | Pledge (Collateral) coin |
| orderId | String | Order ID |
| reviseTime | String | Adjust time |
| reviseSide | String | Revise side<br>`down`: supplement collateral to turn down<br>`up`: withdraw collateral to turn up |
| reviseAmount | String | Adjustment quantity |
| afterPledgeRate | String | Pledge Rate Percentage after adjustment |
| beforePledgeRate | String | Pledge rate percentage before adjustment |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Get Repay History

Frequency limit: 10c/1s (UID)

### Description

Get the list of repay history

### HTTP Request

- GET /api/v2/earn/loan/repay-history

Request Example

```bash
curl "https://api.bitget.com/api/v2/earn/loan/repay-history?startTime=1685957902000&endTime=1691228302423" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json"
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | No | Order ID |
| loanCoin | String | No | Loan coin |
| pledgeCoin | String | No | Pledge (Collateral) coin |
| startTime | String | Yes | Start time, ms, only supports querying the data of the past three months |
| endTime | String | Yes | End time, ms |
| pageNo | String | No | Page No default 1 |
| pageSize | String | No | Page size default 10，max 100 |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1684747525424,
  "data": [{
    "orderId": "12121212121",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "repayAmount": "1566.23820848",
    "payInterest": "0.1185634",
    "repayLoanAmount": "1566.22635214",
    "repayUnlockAmount": "195",
    "repayTime": "1684747525424"
  }, {
    "orderId": "12121212121",
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "repayAmount": "1566.23820848",
    "payInterest": "0.1185634",
    "repayLoanAmount": "1566.22635214",
    "repayUnlockAmount": "195",
    "repayTime": "1684747525424"
  }]
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| orderId | String | Order ID |
| loanCoin | String | Coin to loan |
| loanAmount | String | Pledge (Collateral) coin |
| repayAmount | String | Repay amount |
| payInterest | String | Paid interest |
| repayLoanAmount | String | Loan Currency Repayment of Principal Amount |
| repayUnlockAmount | String | Pledge (Collateral) release amount |
| repayTime | String | Repayment time |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Modify Pledge Rate

Frequency limit: 10c/1s (UID)

### Description

Withdraw or supplement collateral

### HTTP Request

- POST /api/v2/earn/loan/revise-pledge

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/earn/loan/revise-pledge" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json" 
 -d '{
    "orderId":"1",
    "pledgeCoin":"USDT",
    "reviseType":"OUT",
    "amount":"1"
}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | Yes | Order ID |
| amount | String | Yes | Amount to withdraw or supplement |
| pledgeCoin | String | Yes | Pledge (Collateral) coin |
| reviseType | String | Yes | Repay Type<br>`OUT`: Withdraw collateral<br>`IN` supplement collateral |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1684747525424,
  "data": {
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "afterPledgeRate": "60.5"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanCoin | Loan coin |  |
| pledgeCoin | Pledge (Collateral) coin |  |
| afterPledgeRate | Pledge Rate Percentage after adjusted |  |

---

- [Earn API](https://www.bitget.com/api-doc/earn/intro)
# Repay

Frequency limit: 10c/1s (UID)

### Description

Repay

### HTTP Request

- POST /api/v2/earn/loan/repay

Request Example

```bash
curl -X POST "https://api.bitget.com/api/v2/earn/loan/repay" 
  -H "ACCESS-KEY:your apiKey" 
  -H "ACCESS-SIGN:*******" 
  -H "ACCESS-PASSPHRASE:*****" 
  -H "ACCESS-TIMESTAMP:1659076670000" 
  -H "locale:en-US" 
  -H "Content-Type: application/json" 
 -d '{
    "orderId":"ETH",
    "repayAll":"yes",
    "repayUnlock":"yes"
}'
```

### Request Parameters

| Parameter | Type | Required | Description |
| :-- | :-- | :-: | :-: |
| orderId | String | Yes | Order ID |
| amount | String | No | When `repayAll`=`no`: Repay amount |
| repayUnlock | String | No | Whether redeem after repay, default: `yes`<br>`yes`<br>`no` |
| repayAll | String | Yes | Repay all<br>`yes`<br>`no` |

Response example

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1684747525424,
  "data": {
    "loanCoin": "TRX",
    "pledgeCoin": "USDT",
    "repayAmount": "1566.23820848",
    "payInterest": "0.1185634",
    "repayLoanAmount": "1566.22635214",
    "repayUnlockAmount": "195"
  }
}
```

### Response Parameters

| Parameter | Type | Description |
| :-- | :-- | :-- |
| loanCoin | String | Coin to loan |
| pledgeCoin | String | Pledge (Collateral) coin |
| repayAmount | String | Repay amount |
| payInterest | String | Paid interest |
| repayLoanAmount | String | Repay loan amount |
| repayUnlockAmount | String | Pledge (Collateral) redeemed amount |

---
