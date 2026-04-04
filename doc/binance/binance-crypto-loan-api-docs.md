# Binance Crypto Loan API Documentation

Source: https://developers.binance.com/docs/

---

## Repo Usage Quick Reference

- Primary repo use: usually reference-only; not part of the normal funding-arb execution path unless adapter code explicitly adds it
- Base URL context: signed Binance SAPI surface
- Repo asset context:
  - trading symbols look like `BTCUSDT`
  - crypto-loan APIs operate mainly on currencies and loan products, not exchange-traded symbols
- Most relevant sections if this repo ever uses them:
  - borrow / repay
  - loanable and collateral asset metadata
  - LTV adjustment and order history
- Important repo note: do not confuse crypto-loan borrowing with the cross-margin borrow path used by the spot-futures engine; they are separate operational products

Crypto Loan

# Change Log

## 2025-12-26

### Time-sensitive Notice

- **The following change to REST API will occur at approximately 2026-01-15 07:00 UTC:**

When calling endpoints that require signatures, percent-encode payloads before computing signatures. Requests that do not follow this order will be rejected with [`-1022 INVALID_SIGNATURE`](https://developers.binance.com/docs/crypto_loan/error-code#-1022-invalid_signature). Please review and update your signing logic accordingly.

### REST API

- Updated documentation for REST API regarding [Signed Endpoints examples for placing an order](https://developers.binance.com/docs/crypto_loan/general-info#signed-endpoint-examples-for-post-apiv3order).

## 2024-12-20

- New endpoints for Flexible Loan: Collateral repayment is now available for flexible loan

- `POST/sapi/v2/loan/flexible/repay/collateral`
- `GET /sapi/v2/loan/flexible/repay/rate`
- `GET /sapi/v2/loan/flexible/liquidation/history`

## 2024-11-05

- Following the latest upgrade on Binance Loans （Stable Rate Loan）, Binance Loans has retired its SAPI endpoints for Stable Rate Loan as below(Stable Rate orders on VIP Loan are not impacted by this service update):
  - `GET /sapi/v1/loan/loanable/data`
  - `GET /sapi/v1/loan/collateral/data`
  - `POST /sapi/v1/loan/borrow`
  - `POST /sapi/v1/loan/repay`
  - `POST /sapi/v1/loan/adjust/ltv`
  - `POST /sapi/v1/loan/customize/margin_call`
  - `GET /sapi/v1/loan/ongoing/orders`
- In the meantime, Binance Loans will continue to maintain the following SAPI endpoints for stable loan history query:
  - `GET /sapi/v1/loan/income`
  - `GET /sapi/v1/loan/borrow/history`
  - `GET /sapi/v1/loan/ltv/adjustment/history`
  - `GET /sapi/v1/loan/repay/history`

## 2024-02-27

- Following the latest [upgrade](https://www.binance.com/en/support/announcement/binance-upgrades-binance-loans-flexible-rate-2024-02-27-d35942110b53480581773fc62a3e6eae) on Binance Loans
  - Binance Loans has added the following /v2 SAPI endpoints at 2024-02-27 08:00 (UTC). Users may utilize v2 SAPI endpoints to place, repay, and manage new Binance Loans (Flexible Rate) orders created after 2024-02-27 08:00 (UTC).
    - `POST /sapi/v2/loan/flexible/borrow`
    - `GET /sapi/v2/loan/flexible/ongoing/orders`
    - `GET /sapi/v2/loan/flexible/borrow/history`
    - `POST /sapi/v2/loan/flexible/repay`
    - `GET /sapi/v2/loan/flexible/repay/history`
    - `POST /sapi/v2/loan/flexible/adjust/ltv`
    - `GET /sapi/v2/loan/flexible/ltv/adjustment/history`
    - `GET /sapi/v2/loan/flexible/loanable/data`
    - `GET /sapi/v2/loan/flexible/collateral/data`
  - In addition, Binance Loans will be retiring its /v1 SAPI endpoints at the following timings:
    - At 2024-02-27 08:00 (UTC):
      - `POST /sapi/v1/loan/flexible/borrow`
      - `GET /sapi/v1/loan/flexible/loanable/data`
      - `GET /sapi/v1/loan/flexible/collateral/data`
    - At 2024-04-24 03:00 (UTC):
      - `GET /sapi/v1/loan/flexible/ongoing/orders`
      - `POST /sapi/v1/loan/flexible/repay`
      - `POST /sapi/v1/loan/flexible/adjust/ltv`
  - Binance Loans will also continue to maintain the following /v1 SAPI endpoints for users to check their Binance Loans (Flexible Rate) order history before 2024-02-27 08:00 (UTC).
    - `GET /sapi/v1/loan/flexible/borrow/history`
    - `GET /sapi/v1/loan/flexible/repay/history`
    - `GET /sapi/v1/loan/flexible/ltv/adjustment/history`

* * *

## 2023-08-26

- New endpoint for Crypto Loans:
  - `POST /sapi/v1/loan/flexible/borrow`: flexible Loan borrow
  - `GET /sapi/v1/loan/flexible/ongoing/orders`: get flexible loan ongoing orders
  - `GET /sapi/v1/loan/flexible/borrow/history`: Get flexible loan borrow history
  - `POST /sapi/v1/loan/flexible/repay`: flexible loan repay
  - `POST /sapi/v1/loan/flexible/repay/history`: Get flexible loan repayment history
  - `POST /sapi/v1/loan/flexible/adjust/ltv`: adjust flexible Loan adjust LTV
  - `GET /sapi/v1/loan/flexible/ltv/adjustment/history`: Get Flexible loan LTV adjustment history
  - `GET /sapi/v1/loan/flexible/loanable/data`:Get flexible loan assets data
  - `GET /sapi/v1/loan/flexible/collateral/data`: Get flexible loan collateral assets data

* * *

## 2023-02-21

- Adjusted endpoints for Crypto Loan:
  - `POST /sapi/v1/loan/borrow`: paramater `loanTerm` is restricted to 7 or 30

* * *

## 2022-11-01

- New endpoints for Crypto Loan:
  - `GET /sapi/v1/loan/loanable/data`: Get interest rate and borrow limit of loanable assets. The borrow limit is shown in USD value.
  - `GET /sapi/v1/loan/collateral/data`: Get LTV information and collateral limit of collateral assets. The collateral limit is shown in USD value.
  - `GET /sapi/v1/loan/repay/collateral/rate`: Get the the rate of collateral coin / loan coin when using collateral repay, the rate will be valid within 8 second.
  - `POST /sapi/v1/loan/customize/margin_call`: Customize margin call for ongoing orders only.

* * *

## 2022-09-15

- New endpoints for Crypto Loan
  - `POST /sapi/v1/loan/borrow`: Borrow - Crypto Loan Borrow
  - `GET /sapi/v1/loan/borrow/history`: Borrow - Get Loan Borrow History
  - `GET/sapi/v1/loan/ongoing/orders`: Borrow - Get Loan Ongoing Orders
  - `POST/sapi/v1/loan/repay`: Repay - Crypto Loan Repay
  - `GET/sapi/v1/loan/repay/history`: Repay - Get Loan Repayment History
  - `POST/sapi/v1/loan/adjust/ltv`: Adjust LTV - Crypto Loan Adjust LTV
  - `GET/sapi/v1/loan/ltv/adjustment/history`: Adjust LTV - Get Loan LTV Adjustment History

* * *

## 2021-11-08

- New endpoint for Crypto Loans:
  - New endpoint`GET /sapi/v1/loan/income`to support user query crypto loans income history

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Collateral Assets Data(USER\_DATA)

## API Description

Get LTV information and collateral limit of flexible loan's collateral assets. The collateral limit is shown in USD value.

## HTTP Request

GET `/sapi/v2/loan/flexible/collateral/data`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| collateralCoin | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "rows": [
    {
      "collateralCoin": "BNB",
      "initialLTV": "0.65",
      "marginCallLTV": "0.75",
      "liquidationLTV": "0.83",
      "maxLimit": "1000000"
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Assets Data(USER\_DATA)

## API Description

Get interest rate and borrow limit of flexible loanable assets. The borrow limit is shown in USD value.

## HTTP Request

GET `/sapi/v2/loan/flexible/loanable/data`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "flexibleInterestRate": "0.00000491",
      "flexibleMinLimit": "100",
      "flexibleMaxLimit": "1000000"
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Collateral Assets Data(USER\_DATA)

## API Description

Get LTV information and collateral limit of flexible loan's collateral assets. The collateral limit is shown in USD value.

## HTTP Request

GET `/sapi/v2/loan/flexible/collateral/data`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| collateralCoin | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "rows": [
    {
      "collateralCoin": "BNB",
      "initialLTV": "0.65",
      "marginCallLTV": "0.75",
      "liquidationLTV": "0.83",
      "maxLimit": "1000000"
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Interest Rate History (USER\_DATA)

## API Description

Check Flexible Loan interest rate history

## HTTP Request

GET `/sapi/v2/loan/interestRateHistory`

## Request Weight(IP)

**400**

## Request Parameters

| **Name** | **Type** | **Mandatory** | **Description** |
| --- | --- | --- | --- |
| coin | STRING | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Check current querying page, start from 1. Default：1；Max：1000. |
| limit | LONG | NO | Default：10; Max：100. |
| recvWindow | LONG | YES |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned
> - The max interval between startTime and endTime is 90 days.
> - Time based on UTC+0.

## Response Example

```javascript
{
    "rows": [
        {
            "coin": "USDT",
            "annualizedInterestRate": "0.0647",
            "time": 1575018510000
        },
        {
            "coin": "USDT",
            "annualizedInterestRate": "0.0647",
            "time": 1575018510000
        }
    ],
    "total": 2
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Flexible Loan Borrow(TRADE)

## API Description

Borrow Flexible Loan

## HTTP Request

POST `/sapi/v2/loan/flexible/borrow`

## Request Weight

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | YES |  |
| loanAmount | DECIMAL | NO | Mandatory when collateralAmount is empty |
| collateralCoin | STRING | YES |  |
| collateralAmount | DECIMAL | NO | Mandatory when loanAmount is empty |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- This API endpoint is available for both the master account and the sub-account.
- You can customize LTV by entering loanAmount and collateralAmount.

## Response Example

```javascript
{
  "loanCoin": "BUSD",
  "loanAmount": "100.5",
  "collateralCoin": "BNB",
  "collateralAmount": "50.5",
  "status": "Succeeds" //Succeeds, Failed, Processing
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Flexible Loan Adjust LTV(TRADE)

## API Description

Flexible Loan Adjust LTV

## HTTP Request

POST `/sapi/v2/loan/flexible/adjust/ltv`

## Request Weight(UID)

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | YES |  |
| collateralCoin | STRING | YES |  |
| adjustmentAmount | DECIMAL | YES |  |
| direction | ENUM | YES | "ADDITIONAL", "REDUCED" |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - API Key needs Spot & Margin Trading permission for this endpoint

## Response Example

```javascript
{
  "loanCoin": "BUSD",
  "collateralCoin": "BNB",
  "direction": "ADDITIONAL",
  "adjustmentAmount": "5.235",
  "currentLTV": "0.52",
  "status": "Succeeds" // "Succeeds", "Failed", "Processing"
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Flexible Loan Borrow(TRADE)

## API Description

Borrow Flexible Loan

## HTTP Request

POST `/sapi/v2/loan/flexible/borrow`

## Request Weight

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | YES |  |
| loanAmount | DECIMAL | NO | Mandatory when collateralAmount is empty |
| collateralCoin | STRING | YES |  |
| collateralAmount | DECIMAL | NO | Mandatory when loanAmount is empty |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- This API endpoint is available for both the master account and the sub-account.
- You can customize LTV by entering loanAmount and collateralAmount.

## Response Example

```javascript
{
  "loanCoin": "BUSD",
  "loanAmount": "100.5",
  "collateralCoin": "BNB",
  "collateralAmount": "50.5",
  "status": "Succeeds" //Succeeds, Failed, Processing
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Flexible Loan Repay(TRADE)

## API Description

Flexible Loan Repay

## HTTP Request

POST `/sapi/v2/loan/flexible/repay`

## Request Weight

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | YES |  |
| collateralCoin | STRING | YES |  |
| repayAmount | DECIMAL | YES |  |
| collateralReturn | BOOLEAN | NO | Default: TRUE. TRUE: Return extra collateral to spot account; FALSE: Keep extra collateral in the order, and lower LTV. |
| fullRepayment | BOOLEAN | NO | Default: FALSE. TRUE: Full repayment; FALSE: Partial repayment, based on loanAmount |
| repaymentType | INT | NO | Default: 1. 1: Repayment with loan asset; 2: Repayment with collateral |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - repayAmount is mandatory even fullRepayment = FALSE

## Response Example

```javascript
{
  "loanCoin": "BUSD",
  "collateralCoin": "BNB",
  "remainingDebt": "100.5",
  "remainingCollateral": "5.253",
  "fullRepayment": false,
  "currentLTV": "0.25",
  "repayStatus": "REPAID" // REPAID, REPAYING, FAILED
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan LTV Adjustment History(USER\_DATA)

## API Description

Get Flexible Loan LTV Adjustment History

## HTTP Request

GET `/sapi/v2/loan/flexible/ltv/adjustment/history`

GET `/sapi/v1/loan/flexible/ltv/adjustment/history` can be used to check history before 2024-02-27 08:00

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "collateralCoin": "BNB",
      "direction": "ADDITIONAL",
      "collateralAmount": "5.235",
      "preLTV": "0.78",
      "afterLTV": "0.56",
      "adjustTime": 1575018510000
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Check Collateral Repay Rate (USER\_DATA)

## HTTP Request

`GET /sapi/v2/loan/flexible/repay/rate`

## Description

Get the latest rate of collateral coin/loan coin when using collateral repay.

## Request Weight (IP)

**6000**

## Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | YES |  |
| collateralCoin | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "loanCoin": "BUSD",
  "collateralCoin": "BNB",
  "rate": "300.36781234" // rate of collateral coin/loan coin
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Borrow History(USER\_DATA)

## API Description

Get Flexible Loan Borrow History

## HTTP Request

GET `/sapi/v2/loan/flexible/borrow/history`

GET `/sapi/v1/loan/flexible/borrow/history` can be used to check history before 2024-02-27 08:00

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "initialLoanAmount": "10000",
      "collateralCoin": "BNB",
      "initialCollateralAmount": "49.27565492",
      "borrowTime": 1575018510000,
      "status": "SUCCESS" //SUCCESS, FAILED, PENDING
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan LTV Adjustment History(USER\_DATA)

## API Description

Get Flexible Loan LTV Adjustment History

## HTTP Request

GET `/sapi/v2/loan/flexible/ltv/adjustment/history`

GET `/sapi/v1/loan/flexible/ltv/adjustment/history` can be used to check history before 2024-02-27 08:00

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "collateralCoin": "BNB",
      "direction": "ADDITIONAL",
      "collateralAmount": "5.235",
      "preLTV": "0.78",
      "afterLTV": "0.56",
      "adjustTime": 1575018510000
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Liquidation History (USER\_DATA)

## HTTP Request

`GET /sapi/v2/loan/flexible/liquidation/history`

## Request Weight (IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- If startTime and endTime are not sent, the recent 90-day data will be returned.
- The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "liquidationDebt": "10000",
      "collateralCoin": "BNB",
      "liquidationCollateralAmount": "123",
      "returnCollateralAmount": "0.2",
      "liquidationFee": "1.2",
      "liquidationStartingPrice": "49.27565492",
      "liquidationStartingTime": 1575018510000,
      "status": "Liquidated" // Liquidating
    }
  ],
  "total": 1
}
```

liquidationStartingPrice refers to the Collateral/borrowed token exchange rate at the start of the liquidation time. The final exchange rate will vary depending on token liquidity and market conditions.

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Ongoing Orders(USER\_DATA)

## API Description

Get Flexible Loan Ongoing Orders

## HTTP Request

GET `/sapi/v2/loan/flexible/ongoing/orders`

## Request Weight(IP)

**300**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "totalDebt": "10000",
      "collateralCoin": "BNB",
      "collateralAmount": "49.27565492",
      "currentLTV": "0.57"
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Flexible Loan Repayment History(USER\_DATA)

## API Description

Get Flexible Loan Repayment History

## HTTP Request

GET `/sapi/v2/loan/flexible/repay/history`

GET `/sapi/v1/loan/flexible/repay/history` can be used to check history before 2024-02-27 08:00

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
      "loanCoin": "BUSD",
      "repayAmount": "10000",
      "collateralCoin": "BNB",
      "collateralReturn": "49.27565492",
      "repayStatus": "REPAID" // REPAID, REPAYING, FAILED
      "repayTime": 1575018510000
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Crypto Loans Income History(USER\_DATA)

## API Description

Get Crypto Loans Income History

## HTTP Request

`GET /sapi/v1/loan/income`

## Request Weight(UID)

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| type | STRING | NO | All types will be returned by default. Enum：`borrowIn` ,`collateralSpent`, `repayAmount`, `collateralReturn`(Collateral return after repayment), `addCollateral`, `removeCollateral`, `collateralReturnAfterLiquidation` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | default 20, max 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 7-day data will be returned.
> - The max interval between startTime and endTime is 30 days.

## Response Example

```javascript
[
  {
    asset: "BUSD",
    type: "borrowIn",
    amount: "100",
    timestamp: 1633771139847,
    tranId: "80423589583",
  },
  {
    asset: "BUSD",
    type: "borrowIn",
    amount: "100",
    timestamp: 1634638371496,
    tranId: "81685123491",
  },
]
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Crypto Loans Income History(USER\_DATA)

## API Description

Get Crypto Loans Income History

## HTTP Request

`GET /sapi/v1/loan/income`

## Request Weight(UID)

**6000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| type | STRING | NO | All types will be returned by default. Enum：`borrowIn` ,`collateralSpent`, `repayAmount`, `collateralReturn`(Collateral return after repayment), `addCollateral`, `removeCollateral`, `collateralReturnAfterLiquidation` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | default 20, max 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 7-day data will be returned.
> - The max interval between startTime and endTime is 30 days.

## Response Example

```javascript
[
  {
    asset: "BUSD",
    type: "borrowIn",
    amount: "100",
    timestamp: 1633771139847,
    tranId: "80423589583",
  },
  {
    asset: "BUSD",
    type: "borrowIn",
    amount: "100",
    timestamp: 1634638371496,
    tranId: "81685123491",
  },
]
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Loan Borrow History(USER\_DATA)

## API Description

Get Loan Borrow History

## HTTP Request

GET `/sapi/v1/loan/borrow/history`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO | orderId in `POST /sapi/v1/loan/borrow` |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000. |
| limit | LONG | NO | Default: 10; max: 100. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
    "orderId": 100000001,
    "loanCoin": "BUSD",
    "initialLoanAmount": "10000",
    "hourlyInterestRate": "0.000057"
    "loanTerm": "7"
    "collateralCoin": "BNB",
    "initialCollateralAmount": "49.27565492"
    "borrowTime": 1575018510000
    "status": "Repaid" // Accruing_Interest, Overdue, Liquidating, Repaying, Repaid, Liquidated, Pending, Failed
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Loan Borrow History(USER\_DATA)

## API Description

Get Loan Borrow History

## HTTP Request

GET `/sapi/v1/loan/borrow/history`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO | orderId in `POST /sapi/v1/loan/borrow` |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000. |
| limit | LONG | NO | Default: 10; max: 100. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
    "orderId": 100000001,
    "loanCoin": "BUSD",
    "initialLoanAmount": "10000",
    "hourlyInterestRate": "0.000057"
    "loanTerm": "7"
    "collateralCoin": "BNB",
    "initialCollateralAmount": "49.27565492"
    "borrowTime": 1575018510000
    "status": "Repaid" // Accruing_Interest, Overdue, Liquidating, Repaying, Repaid, Liquidated, Pending, Failed
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Loan LTV Adjustment History(USER\_DATA)

## API Description

Get Loan LTV Adjustment History

## HTTP Request

GET `/sapi/v1/loan/ltv/adjustment/history`

## Request Weight

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
    "loanCoin": "BUSD",
    "collateralCoin": "BNB",
    "direction": "ADDITIONAL",
    "amount": "5.235",
    "preLTV": "0.78",
    "afterLTV": "0.56",
    "adjustTime": 1575018510000,
    "orderId": 756783308056935434
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Crypto Loan

# Get Loan Repayment History(USER\_DATA)

## API Description

Get Loan Repayment History

## HTTP Request

GET `/sapi/v1/loan/repay/history`

## Request Weight(IP)

**400**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| loanCoin | STRING | NO |  |
| collateralCoin | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1; default: 1; max: 1000 |
| limit | LONG | NO | Default: 10; max: 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If startTime and endTime are not sent, the recent 90-day data will be returned.
> - The max interval between startTime and endTime is 180 days.

## Response Example

```javascript
{
  "rows": [
    {
    "loanCoin": "BUSD",
    "repayAmount": "10000",
    "collateralCoin": "BNB",
    "collateralUsed": "0"
    "collateralReturn": "49.27565492"
    "repayType": "1" // 1 for "repay with borrowed coin", 2 for "repay with collateral"
    "repayStatus": "Repaid" // Repaid, Repaying, Failed
    "repayTime": 1575018510000
    "orderId": 756783308056935434
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---
