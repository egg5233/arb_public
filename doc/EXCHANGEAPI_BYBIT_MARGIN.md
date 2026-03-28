# Bybit Spot Margin / UTA Borrowing API Documentation

> Scraped from https://bybit-exchange.github.io/docs/v5/ on 2026-03-28.
> Covers all spot margin, borrow/repay, collateral, and related endpoints for the Unified Trading Account (UTA).

## Base URLs

- **Mainnet**: `https://api.bybit.com`
- **Testnet**: `https://api-testnet.bybit.com`

## Authentication

All endpoints require HMAC-SHA256 signature unless noted otherwise. Headers:

| Header | Description |
|---|---|
| X-BAPI-API-KEY | API key |
| X-BAPI-TIMESTAMP | UTC timestamp (ms) |
| X-BAPI-SIGN | HMAC-SHA256 signature (lowercase hex) |
| X-BAPI-RECV-WINDOW | Request validity window (ms), default 5000 |

Signature: `HMAC-SHA256(timestamp + api_key + recv_window + payload)` where payload is query string (GET) or JSON body (POST).

## Common Response Wrapper

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {},
    "retExtInfo": {},
    "time": 1671017382656
}
```

`retCode=0` means success. `retMsg` of "OK", "success", "SUCCESS", or "" all indicate success.

---

## REST API Endpoints

### Account -- Borrow & Repay

#### Manual Borrow
- **Endpoint**: `POST /v5/account/borrow`
- **Auth**: Requires signature (Spot permission)
- **Notes**: Borrowing via OpenAPI supports **variable rate borrowing only**.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | string | Yes | Coin name, uppercase only (e.g. BTC, USDT) |
| amount | string | Yes | Borrow amount |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| result | object | |
| > coin | string | Coin name |
| > amount | string | Borrow amount |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "coin": "BTC",
        "amount": "0.01"
    },
    "retExtInfo": {},
    "time": 1756197991955
}
```

---

#### Manual Repay (with asset conversion)
- **Endpoint**: `POST /v5/account/repay`
- **Auth**: Requires signature (Spot permission)
- **Notes**:
  - If neither `coin` nor `amount` is passed, repays **all liabilities**.
  - If only `coin` is passed (no `amount`), that coin is repaid in full.
  - System uses spot available balance first; if insufficient, converts other assets per liquidation order.
  - Repayment prohibited between **:04 and :05:30 each hour** (interest calculated at :05:00).
  - Repays floating-rate liabilities first, then fixed-rate.
  - Per-transaction coin-conversion limit of USD 300,000.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | string | No | Coin name, uppercase only |
| amount | string | No | Repay amount. Cannot pass amount without coin |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| result | object | |
| > resultStatus | string | `P`: Processing, `SU`: Success, `FA`: Failed |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "resultStatus": "P"
    },
    "retExtInfo": {},
    "time": 1756295680801
}
```

---

#### Manual Repay Without Asset Conversion
- **Endpoint**: `POST /v5/account/no-convert-repay`
- **Auth**: Requires signature (Spot permission)
- **Notes**:
  - Uses **only** the spot available balance of the debt currency -- no asset conversion.
  - If `coin` is passed without `amount`, repayment amount = available spot balance of that coin.
  - Check available amount via `GET /v5/spot-margin-trade/repayment-available-amount` first.
  - Repayment prohibited between :04 and :05:30 each hour.
  - Repays floating-rate liabilities first, then fixed-rate.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| coin | string | Yes | Coin name, uppercase only |
| amount | string | No | Repay amount. If omitted, repays up to available spot balance |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| result | object | |
| > resultStatus | string | `P`: Processing, `SU`: Success, `FA`: Failed |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "resultStatus": "P"
    },
    "retExtInfo": {},
    "time": 1756295680801
}
```

---

#### Get Borrow History
- **Endpoint**: `GET /v5/account/borrow-history`
- **Auth**: Requires signature
- **Notes**: Returns interest records sorted by creation time descending. Data retention: 2 years.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | USDC, USDT, BTC, ETH etc., uppercase only |
| startTime | integer | No | Start timestamp (ms). If both times omitted, returns 30 days. If only startTime, returns startTime to startTime+30d. endTime-startTime <= 30 days |
| endTime | integer | No | End timestamp (ms) |
| limit | integer | No | Page size [1, 50]. Default: 20 |
| cursor | string | No | Cursor for pagination (use nextPageCursor from response) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of borrow records |
| > currency | string | USDC, USDT, BTC, ETH |
| > createdTime | integer | Created timestamp (ms) |
| > borrowCost | string | Interest charged |
| > hourlyBorrowRate | string | Hourly borrow rate |
| > InterestBearingBorrowSize | string | Interest-bearing borrow size |
| > costExemption | string | Cost exemption amount |
| > borrowAmount | string | Total borrow amount |
| > unrealisedLoss | string | Unrealised loss |
| > freeBorrowedAmount | string | Interest-free borrowed amount |
| nextPageCursor | string | Pagination cursor |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "nextPageCursor": "2671153%3A1%2C2671153%3A1",
        "list": [
            {
                "borrowAmount": "1.06333265702840778",
                "costExemption": "0",
                "freeBorrowedAmount": "0",
                "createdTime": 1697439900204,
                "InterestBearingBorrowSize": "1.06333265702840778",
                "currency": "BTC",
                "unrealisedLoss": "0",
                "hourlyBorrowRate": "0.000001216904",
                "borrowCost": "0.00000129"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1697442206478
}
```

---

#### Get Collateral Info
- **Endpoint**: `GET /v5/account/collateral-info`
- **Auth**: Requires signature
- **Notes**: Returns collateral info for current UTA account: interest rate, borrowable amounts, collateral ratios.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Asset currency, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of collateral info objects |
| > currency | string | Currency name |
| > hourlyBorrowRate | string | Hourly borrow rate |
| > maxBorrowingAmount | string | Max borrow amount (shared across main-sub UIDs) |
| > freeBorrowingLimit | string | Max interest-free borrowing limit. Only contracts unrealised loss has interest-free; spot margin always has interest |
| > freeBorrowAmount | string | Amount within total borrowing exempt from interest |
| > borrowAmount | string | Current borrow amount |
| > otherBorrowAmount | string | Sum of borrowing for other accounts under same main account |
| > availableToBorrow | string | Available amount to borrow (shared across main-sub UIDs) |
| > borrowable | boolean | Whether currency can be borrowed |
| > borrowUsageRate | string | Borrow usage rate (main + sub accounts). 0.5 = 50% |
| > marginCollateral | boolean | Whether it can be used as margin collateral (platform-level) |
| > collateralSwitch | boolean | Whether collateral is turned on by user |
| > collateralRatio | string | **Deprecated** -- use Get Tiered Collateral Ratio instead |
| > freeBorrowingAmount | string | **Deprecated** -- always returns "", use freeBorrowingLimit |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "availableToBorrow": "3",
                "freeBorrowingAmount": "",
                "freeBorrowAmount": "0",
                "maxBorrowingAmount": "3",
                "hourlyBorrowRate": "0.00000147",
                "borrowUsageRate": "0",
                "collateralSwitch": true,
                "borrowAmount": "0",
                "borrowable": true,
                "currency": "BTC",
                "otherBorrowAmount": "0",
                "marginCollateral": true,
                "freeBorrowingLimit": "0",
                "collateralRatio": "0.95"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1691565901952
}
```

---

#### Get Wallet Balance
- **Endpoint**: `GET /v5/account/wallet-balance`
- **Auth**: Requires signature
- **Notes**:
  - Returns wallet balance and per-coin asset info. Currencies with 0 assets/liabilities are omitted by default.
  - Under UTA manual borrow: `equity = walletBalance - spotBorrow + unrealisedPnl + optionValue`.
  - Old walletBalance = New walletBalance - spotBorrow.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| accountType | string | Yes | Account type: `UNIFIED` |
| coin | string | No | Coin name, uppercase. Comma-separated for multiple (e.g. `USDT,USDC`). If omitted, returns non-zero assets |

- **Response Fields** (account-level):

| Field | Type | Description |
|---|---|---|
| list | array | Account info array |
| > accountType | string | Account type |
| > accountIMRate | string | Account initial margin rate |
| > accountMMRate | string | Account maintenance margin rate |
| > totalEquity | string | Total equity (USD) |
| > totalWalletBalance | string | Total wallet balance (USD) |
| > totalMarginBalance | string | Margin balance (USD) = totalWalletBalance + totalPerpUPL |
| > totalAvailableBalance | string | Available balance (USD). Cross: totalMarginBalance - Haircut - totalInitialMargin |
| > totalPerpUPL | string | Perps/Futures unrealised PnL (USD) |
| > totalInitialMargin | string | Total initial margin (USD) |
| > totalMaintenanceMargin | string | Total maintenance margin (USD) |
| > accountLTV | string | **Deprecated** |

- **Response Fields** (coin-level, nested under `> coin`):

| Field | Type | Description |
|---|---|---|
| >> coin | string | Coin name (BTC, ETH, USDT, USDC) |
| >> equity | string | Coin equity = walletBalance - spotBorrow + unrealisedPnl + optionValue |
| >> usdValue | string | USD value |
| >> walletBalance | string | Wallet balance |
| >> locked | string | Locked balance (spot open orders) |
| >> spotHedgingQty | string | Spot asset qty for portfolio margin hedging |
| >> borrowAmount | string | Total borrow = spot liabilities + derivatives liabilities |
| >> accruedInterest | string | Accrued interest |
| >> totalOrderIM | string | Pre-occupied margin for orders |
| >> totalPositionIM | string | Sum of position initial margins + liquidation fee |
| >> totalPositionMM | string | Sum of position maintenance margins |
| >> unrealisedPnl | string | Unrealised PnL |
| >> cumRealisedPnl | string | Cumulative realised PnL |
| >> bonus | string | Bonus |
| >> marginCollateral | boolean | Platform-level collateral eligibility |
| >> collateralSwitch | boolean | User-level collateral toggle |
| >> spotBorrow | string | Spot margin + manual borrow amount (excludes active order borrow) |
| >> availableToWithdraw | string | **Deprecated** for UNIFIED from Jan 2025 |
| >> availableToBorrow | string | **Deprecated** -- use Get Collateral Info |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "totalEquity": "3.31216591",
                "accountIMRate": "0",
                "totalMarginBalance": "3.00326056",
                "totalInitialMargin": "0",
                "accountType": "UNIFIED",
                "totalAvailableBalance": "3.00326056",
                "accountMMRate": "0",
                "totalPerpUPL": "0",
                "totalWalletBalance": "3.00326056",
                "accountLTV": "0",
                "totalMaintenanceMargin": "0",
                "coin": [
                    {
                        "availableToBorrow": "3",
                        "bonus": "0",
                        "accruedInterest": "0",
                        "availableToWithdraw": "0",
                        "totalOrderIM": "0",
                        "equity": "0",
                        "totalPositionMM": "0",
                        "usdValue": "0",
                        "spotHedgingQty": "0.01592413",
                        "unrealisedPnl": "0",
                        "collateralSwitch": true,
                        "borrowAmount": "0.0",
                        "totalPositionIM": "0",
                        "walletBalance": "0",
                        "cumRealisedPnl": "0",
                        "locked": "0",
                        "marginCollateral": true,
                        "coin": "BTC",
                        "spotBorrow": "0"
                    }
                ]
            }
        ]
    },
    "retExtInfo": {},
    "time": 1690872862481
}
```

---

### Spot Margin Trade (UTA) -- Configuration

#### Toggle Margin Trade
- **Endpoint**: `POST /v5/spot-margin-trade/switch-mode`
- **Auth**: Requires signature
- **Notes**: Turn on/off spot margin trade. Account must have activated spot margin first (quiz completed on web/app).
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| spotMarginMode | string | Yes | `1`: on, `0`: off |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| spotMarginMode | string | Spot margin status. `1`: on, `0`: off |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "spotMarginMode": "0"
    },
    "retExtInfo": {},
    "time": 1672297795542
}
```

---

#### Set Leverage
- **Endpoint**: `POST /v5/spot-margin-trade/set-leverage`
- **Auth**: Requires signature
- **Notes**: Set max leverage for spot cross margin. Must have activated spot margin. Updated leverage must be <= max leverage of the currency.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| leverage | string | Yes | Leverage value. Range: [2, 10] |
| currency | string | No | Coin name, uppercase only |

- **Response Fields**: None (empty result object)

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {},
    "retExtInfo": {},
    "time": 1672710944282
}
```

---

#### Get Status And Leverage
- **Endpoint**: `GET /v5/spot-margin-trade/state`
- **Auth**: Requires signature
- **Parameters**: None
- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| spotLeverage | string | Spot margin leverage. Returns "" if margin trade is off |
| spotMarginMode | string | `1`: on, `0`: off |
| effectiveLeverage | string | Actual leverage ratio (2 decimal places, truncated) |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "spotLeverage": "10",
        "spotMarginMode": "1",
        "effectiveLeverage": "1"
    },
    "retExtInfo": {},
    "time": 1692696841231
}
```

---

#### Set Auto Repay Mode
- **Endpoint**: `POST /v5/spot-margin-trade/set-auto-repay-mode`
- **Auth**: Requires signature
- **Notes**:
  - If currency not passed, enables/disables for **all currencies**.
  - When `autoRepayMode=1`, system auto-repays (without conversion) at :00 and :30 each hour.
  - Repayment amount = min(spot available balance, liability).
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase. If omitted, applies to all currencies |
| autoRepayMode | string | Yes | `1`: On, `0`: Off |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| data | array | Array of result objects |
| > currency | string | Coin name |
| > autoRepayMode | string | `1`: On, `0`: Off |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "data": [
            {
                "currency": "ETH",
                "autoRepayMode": "1"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1766976677678
}
```

---

#### Get Auto Repay Mode
- **Endpoint**: `GET /v5/spot-margin-trade/get-auto-repay-mode`
- **Auth**: Requires signature
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase. If omitted, returns all currencies |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| data | array | Array of result objects |
| > currency | string | Coin name |
| > autoRepayMode | string | `1`: On, `0`: Off |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "data": [
            {
                "autoRepayMode": "1",
                "currency": "ETH"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1766977353904
}
```

---

### Spot Margin Trade (UTA) -- Query Endpoints

#### Get Max Borrowable Amount
- **Endpoint**: `GET /v5/spot-margin-trade/max-borrowable`
- **Auth**: Requires signature
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | Yes | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| currency | string | Coin name |
| maxLoan | string | Max borrowable amount |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "maxLoan": "17.54689892",
        "currency": "BTC"
    },
    "retExtInfo": {},
    "time": 1756261353733
}
```

---

#### Get Available Amount to Repay
- **Endpoint**: `GET /v5/spot-margin-trade/repayment-available-amount`
- **Auth**: Requires signature
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | Yes | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| currency | string | Coin name |
| lossLessRepaymentAmount | string | Repayment amount = min(spot coin available balance, coin borrow amount) |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "lossLessRepaymentAmount": "0.02000000",
        "currency": "BTC"
    },
    "retExtInfo": {},
    "time": 1756273388821
}
```

---

#### Get VIP Margin Data
- **Endpoint**: `GET /v5/spot-margin-trade/data`
- **Auth**: **No authentication required** (public data)
- **Notes**: Margin data for Unified accounts. Returns borrow rates, limits, and collateral info per VIP level.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| vipLevel | string | No | VIP level (e.g. "No VIP", "VIP-1") |
| currency | string | No | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| vipCoinList | array | Array of VIP-level objects |
| > list | array | Array of coin data |
| >> borrowable | boolean | Whether borrowing is allowed |
| >> collateralRatio | string | **Deprecated** -- use Get Tiered Collateral Ratio |
| >> currency | string | Coin name |
| >> hourlyBorrowRate | string | Borrow interest rate per hour |
| >> liquidationOrder | string | Liquidation order priority |
| >> marginCollateral | boolean | Whether it can be margin collateral |
| >> maxBorrowingAmount | string | Max borrow amount |
| > vipLevel | string | VIP level |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "vipCoinList": [
            {
                "list": [
                    {
                        "borrowable": true,
                        "collateralRatio": "0.95",
                        "currency": "BTC",
                        "hourlyBorrowRate": "0.0000015021220000",
                        "liquidationOrder": "11",
                        "marginCollateral": true,
                        "maxBorrowingAmount": "3"
                    }
                ],
                "vipLevel": "No VIP"
            }
        ]
    }
}
```

---

#### Get Historical Interest Rate
- **Endpoint**: `GET /v5/spot-margin-trade/interest-rate-history`
- **Auth**: Requires signature (Spot permission)
- **Notes**:
  - Query up to 6 months of borrowing interest rate history.
  - Public data (same rates for same VIP/Pro level).
  - Only supports Unified account.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | Yes | Coin name, uppercase only |
| vipLevel | string | No | VIP level. "No VIP" must be URL-encoded as "No%20VIP". If omitted, returns your account's VIP level data |
| startTime | integer | No | Start timestamp (ms). Either both times or neither. Returns 7 days if neither. Max 30 days interval |
| endTime | integer | No | End timestamp (ms) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of rate records |
| > timestamp | long | Timestamp (ms) |
| > currency | string | Coin name |
| > hourlyBorrowRate | string | Hourly borrowing rate |
| > vipLevel | string | VIP/Pro level |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "timestamp": 1721469600000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            },
            {
                "timestamp": 1721466000000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1721899048991
}
```

---

#### Get Tiered Collateral Ratio
- **Endpoint**: `GET /v5/spot-margin-trade/collateral`
- **Auth**: **No authentication required** (public data)
- **Notes**: UTA loan tiered collateral ratio. Replaces the deprecated `collateralRatio` field.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of objects |
| > currency | string | Coin name |
| > collateralRatioList | array | Tiered ratio list |
| >> maxQty | string | Upper limit (in coin). "" = positive infinity |
| >> minQty | string | Lower limit (in coin) |
| >> collateralRatio | string | Collateral ratio for this tier |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "collateralRatioList": [
                    {
                        "minQty": "0",
                        "maxQty": "1000000",
                        "collateralRatio": "0.85"
                    },
                    {
                        "minQty": "1000000",
                        "maxQty": "",
                        "collateralRatio": "0"
                    }
                ]
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1739848984945
}
```

---

#### Get Currency Data
- **Endpoint**: `GET /v5/spot-margin-trade/currency-data`
- **Auth**: Requires signature
- **Notes**: If borrowable switch is disabled (false), related config fields return "".
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of currency data |
| > currency | string | Coin name |
| > flexibleManualBorrowable | boolean | Whether flexible (variable rate) manual borrow is enabled |
| > minFlexibleManualBorrowQty | string | Min flexible manual borrow qty |
| > flexibleManualBorrowAccuracy | string | Coin precision for flexible manual borrow |
| > fixedManualBorrowable | boolean | Whether fixed-rate manual borrow is enabled |
| > minFixedManualBorrowQty | string | Min fixed manual borrow qty |
| > fixedManualBorrowAccuracy | string | Coin precision for fixed manual borrow |
| > fixedInterestRateAccuracy | string | Precision for fixed borrow interest rate |
| > minFixedInterestRate | string | Min fixed interest rate (e.g. 0.01) |
| > maxFixedInterestRate | string | Max fixed interest rate (e.g. 0.8) |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "flexibleManualBorrowable": true,
                "minFlexibleManualBorrowQty": "0.001",
                "flexibleManualBorrowAccuracy": "8",
                "fixedManualBorrowable": false,
                "minFixedManualBorrowQty": "",
                "fixedManualBorrowAccuracy": "",
                "fixedInterestRateAccuracy": "",
                "minFixedInterestRate": "",
                "maxFixedInterestRate": ""
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1773220082091
}
```

---

#### Get Position Tiers
- **Endpoint**: `GET /v5/spot-margin-trade/position-tiers`
- **Auth**: Requires signature
- **Notes**: If currency passed, query by currency; otherwise returns all configured currencies.
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of objects |
| > currency | string | Coin name |
| > positionTiersRatioList | array | Tier data, small to large |
| >> tier | string | Tier number |
| >> borrowLimit | string | Tier accumulation borrow limit |
| >> positionMMR | string | Loan maintenance margin rate (8 decimals) |
| >> positionIMR | string | Loan initial margin rate (8 decimals) |
| >> maxLeverage | string | Max loan leverage for this tier |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "positionTiersRatioList": [
                    {
                        "tier": "1",
                        "borrowLimit": "390",
                        "positionMMR": "0.04",
                        "positionIMR": "0.2",
                        "maxLeverage": "5"
                    },
                    {
                        "tier": "2",
                        "borrowLimit": "391",
                        "positionMMR": "0.04",
                        "positionIMR": "0.25",
                        "maxLeverage": "4"
                    }
                ]
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1756272543440
}
```

---

#### Get Coin State
- **Endpoint**: `GET /v5/spot-margin-trade/coinstate`
- **Auth**: Requires signature
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| currency | string | No | Coin name, uppercase only |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| list | array | Array of objects |
| > currency | string | Coin name |
| > spotLeverage | string | Spot margin leverage. Returns "" if margin mode is off |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "list": [
            {
                "spotLeverage": 3,
                "currency": "BTC"
            },
            {
                "spotLeverage": 4,
                "currency": "ETH"
            },
            {
                "spotLeverage": 4,
                "currency": "USDT"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1756273703314
}
```

---

### Trade -- Place Order (Spot Margin)

#### Place Order
- **Endpoint**: `POST /v5/order/create`
- **Rate Limit**: 10/s (Spot), 10/s (Linear/Inverse), 20/s (Option)
- **Auth**: Requires signature
- **Notes**:
  - Supports Spot, Margin, USDT Perp, USDC Perp/Futures, Inverse, Options.
  - For **spot margin orders**: set `category="spot"` and `isLeverage=1`.
  - Supported order types: `Limit`, `Market`.
  - Supported timeInForce: `GTC`, `IOC`, `FOK`, `PostOnly`.
  - Spot open order limit: 500 total per account (incl. 30 TP/SL, 30 conditional per symbol).
  - Market orders are converted to IOC limit orders internally for slippage protection.
- **Parameters** (spot-margin relevant subset):

| Name | Type | Required | Description |
|---|---|---|---|
| category | string | Yes | Product type: `spot`, `linear`, `inverse`, `option` |
| symbol | string | Yes | Symbol name (e.g. BTCUSDT), uppercase |
| isLeverage | integer | No | `0` (default): spot trading, `1`: margin trading. Must have margin enabled and collateral set |
| side | string | Yes | `Buy`, `Sell` |
| orderType | string | Yes | `Market`, `Limit` |
| qty | string | Yes | Order quantity. Spot Market Buy: by value (quote currency) by default |
| marketUnit | string | No | Unit for qty in spot market orders: `baseCoin` or `quoteCoin` |
| price | string | No | Order price (required for Limit orders) |
| timeInForce | string | No | `GTC` (default), `IOC`, `FOK`, `PostOnly` |
| orderLinkId | string | No | Custom order ID, max 36 chars, must be unique |
| orderFilter | string | No | `Order` (default), `tpslOrder`, `StopOrder`. Spot only |
| slippageToleranceType | string | No | `TickSize` or `Percent` for market orders |
| slippageTolerance | string | No | Slippage value. TickSize: [1, 10000] integer. Percent: [0.01, 10] |
| triggerPrice | string | No | TP/SL and conditional order trigger price (spot) |
| takeProfit | string | No | Take profit price |
| stopLoss | string | No | Stop loss price |
| tpOrderType | string | No | `Market` (default) or `Limit` |
| slOrderType | string | No | `Market` (default) or `Limit` |
| tpLimitPrice | string | No | Limit price when TP triggers (requires tpOrderType=Limit) |
| slLimitPrice | string | No | Limit price when SL triggers (requires slOrderType=Limit) |
| reduceOnly | boolean | No | Only for linear/inverse/option |
| positionIdx | integer | No | 0: one-way, 1: hedge Buy, 2: hedge Sell. For linear/inverse |
| smpType | string | No | Self-match prevention type |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| orderId | string | Order ID |
| orderLinkId | string | User customised order ID |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "orderId": "1321003749386327552",
        "orderLinkId": "spot-test-postonly"
    },
    "retExtInfo": {},
    "time": 1672211918471
}
```

- **Spot Margin Order Example** (request body):
```json
{
    "category": "spot",
    "symbol": "BTCUSDT",
    "side": "Buy",
    "orderType": "Limit",
    "qty": "0.1",
    "price": "15600",
    "timeInForce": "GTC",
    "orderLinkId": "spot-test-limit",
    "isLeverage": 1,
    "orderFilter": "Order"
}
```

---

### Asset -- Internal Transfer

#### Create Internal Transfer
- **Endpoint**: `POST /v5/asset/transfer/inter-transfer`
- **Auth**: Requires signature
- **Notes**: Transfer between different account types under the same UID (e.g. UNIFIED to FUND, UNIFIED to CONTRACT).
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| transferId | string | Yes | UUID (manually generated) |
| coin | string | Yes | Coin name, uppercase only |
| amount | string | Yes | Transfer amount |
| fromAccountType | string | Yes | Source account type (e.g. `UNIFIED`, `CONTRACT`, `FUND`) |
| toAccountType | string | Yes | Destination account type |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| transferId | string | UUID |
| status | string | `STATUS_UNKNOWN`, `SUCCESS`, `PENDING`, `FAILED` |

- **Response Example**:
```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "transferId": "42c0cfb0-6bca-c242-bc76-4e6df6cbab16",
        "status": "SUCCESS"
    },
    "retExtInfo": {},
    "time": 1670986962783
}
```

---

## Account Types Reference

| Account Type | Description |
|---|---|
| UNIFIED | Unified Trading Account (UTA) -- spot, derivatives, margin |
| CONTRACT | Derivatives account (legacy) |
| FUND | Funding/Spot wallet |

---

## Important Notes for Arb Bot Integration

1. **Interest calculation**: Interest is charged hourly at :05:00. Borrowing costs are based on `hourlyBorrowRate`.
2. **Repayment window**: Repayment is **prohibited** between :04 and :05:30 each hour.
3. **Auto repay**: When enabled, system auto-repays at :00 and :30 each hour using spot balance only (no conversion).
4. **Variable vs Fixed rates**: OpenAPI manual borrow supports **variable rate only**.
5. **Collateral ratio**: The flat `collateralRatio` field is deprecated. Use `GET /v5/spot-margin-trade/collateral` for tiered collateral ratios.
6. **spotBorrow field**: In wallet balance, `spotBorrow` = spot margin trade borrow + manual borrow (excludes active order borrow).
7. **Leverage**: Range [2, 10]. Must be <= max leverage of the currency. Check via Get Coin State or Get Position Tiers.
8. **Spot margin orders**: Use `POST /v5/order/create` with `category="spot"` and `isLeverage=1`.
9. **Interest-free borrowing**: Only available for borrowing caused by derivatives unrealised loss. Spot margin borrowing **always** has interest.
10. **Borrow limits**: `maxBorrowingAmount` and `availableToBorrow` are shared across main and sub UIDs.
