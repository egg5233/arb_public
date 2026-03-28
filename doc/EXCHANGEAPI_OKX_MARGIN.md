# OKX API — Spot Margin / Borrow-Repay Reference

> Scraped from [OKX V5 API Docs](https://www.okx.com/docs-v5/en/) on 2026-03-28.
> Covers all margin borrow/repay, interest, loan limit, and spot-cross trading endpoints.

## Overview

OKX margin borrowing behavior depends on **account mode** (`acctLv`):

| acctLv | Mode | Margin Borrow Behavior |
|--------|------|----------------------|
| 1 | Spot mode | Manual or auto borrow via `enableSpotBorrow`. Uses `spot-manual-borrow-repay`, `set-auto-repay`. `tdMode=cross` for spot margin orders. |
| 2 | Futures mode | Isolated/cross margin for MARGIN pairs. `tdMode=isolated` or `cross`. |
| 3 | Multi-currency margin | Auto-loan via `set-auto-loan`. `tdMode=cross` for spot. |
| 4 | Portfolio margin | Auto-loan via `set-auto-loan`. `tdMode=cross` for spot. |

**Base URL**: `https://www.okx.com`

**Auth**: All endpoints below require signature. See EXCHANGEAPI_OKX.md for auth details.

**Response wrapper**: All responses use `{"code":"0","msg":"","data":[...]}`. `code` = `"0"` means success.

---

## REST API Endpoints

### Spot Margin — Borrow & Repay

#### Manual Borrow / Repay
- **Endpoint**: `POST /api/v5/account/spot-manual-borrow-repay`
- **Rate Limit**: 1 req/s (Master Account User ID)
- **Auth**: Trade permission required
- **Description**: Manually borrow or repay coins. Only applicable to **Spot mode** (with borrowing enabled via `enableSpotBorrow`).

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | Yes | Currency, e.g. `BTC`, `USDT` |
| side | String | Yes | `borrow` or `repay` |
| amt | String | Yes | Amount to borrow or repay |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ccy | String | Currency, e.g. `BTC` |
| side | String | `borrow` or `repay` |
| amt | String | Actual amount borrowed/repaid |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "side": "borrow",
            "amt": "100"
        }
    ],
    "msg": ""
}
```

---

#### Get Borrow/Repay History
- **Endpoint**: `GET /api/v5/account/spot-borrow-repay-history`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Retrieve borrow/repay history under Spot mode.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | No | Currency, e.g. `BTC` |
| type | String | No | Event type: `auto_borrow`, `auto_repay`, `manual_borrow`, `manual_repay` |
| after | String | No | Pagination — return records earlier than this timestamp (ms), e.g. `1597026383085` |
| before | String | No | Pagination — return records newer than this timestamp (ms), e.g. `1597026383085` |
| limit | String | No | Results per request. Max 100, default 100. |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ccy | String | Currency, e.g. `BTC` |
| type | String | Event type: `auto_borrow`, `auto_repay`, `manual_borrow`, `manual_repay` |
| amt | String | Amount |
| accBorrowed | String | Accumulated borrow amount |
| ts | String | Timestamp (ms Unix), e.g. `1597026383085` |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "accBorrowed": "0",
            "amt": "6764.802661157592",
            "ccy": "USDT",
            "ts": "1725330976644",
            "type": "auto_repay"
        }
    ],
    "msg": ""
}
```

---

#### Set Auto Repay
- **Endpoint**: `POST /api/v5/account/set-auto-repay`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Trade permission required
- **Description**: Enable or disable auto-repay under Spot mode (with borrowing enabled).

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| autoRepay | Boolean | Yes | `true`: Enable auto repay. `false`: Disable auto repay. |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| autoRepay | Boolean | `true`: auto repay enabled. `false`: auto repay disabled. |

- **Response Example**:
```json
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "autoRepay": true
        }
    ]
}
```

---

#### Set Auto Loan
- **Endpoint**: `POST /api/v5/account/set-auto-loan`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Trade permission required
- **Description**: Enable or disable automatic loan. Only applicable to **Multi-currency margin** and **Portfolio margin** modes.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| autoLoan | Boolean | No | `true`: auto-loan enabled. `false`: auto-loan disabled. Default is `true`. |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| autoLoan | Boolean | Whether auto loan is enabled |

- **Response Example**:
```json
{
    "code": "0",
    "msg": "",
    "data": [{
        "autoLoan": true
    }]
}
```

---

### Interest & Loan Limits

#### Get Interest Rate
- **Endpoint**: `GET /api/v5/account/interest-rate`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Get the user's current leveraged currency borrowing market interest rate.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | No | Currency, e.g. `BTC` |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| interestRate | String | **Hourly** borrowing interest rate |
| ccy | String | Currency |

- **Response Example**:
```json
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "ccy": "BTC",
            "interestRate": "0.0001"
        },
        {
            "ccy": "LTC",
            "interestRate": "0.0003"
        }
    ]
}
```

---

#### Get Borrow Interest and Limit (interest-limits)
- **Endpoint**: `GET /api/v5/account/interest-limits`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Get borrow interest rates, loan quotas, and available loan amounts per currency. This is the primary endpoint for checking borrowing capacity.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | String | No | Loan type. `2`: Market loans. Default is `2`. |
| ccy | String | No | Loan currency, e.g. `BTC` |

- **Response Fields** (top level):

| Field | Type | Description |
|-------|------|-------------|
| debt | String | Current total debt in USD |
| interest | String | Current total interest in USD (Market loans only) |
| nextDiscountTime | String | Next deduction time (ms Unix) |
| nextInterestTime | String | Next interest accrual time (ms Unix) |
| loanAlloc | String | VIP Loan allocation %. Range [0,100]. `"0"` if master did not assign. `""` if shared between master/sub. |
| records | Array | Per-currency detail records (see below) |

- **Response Fields** (records[]):

| Field | Type | Description |
|-------|------|-------------|
| ccy | String | Loan currency, e.g. `BTC` |
| rate | String | Current **daily** borrowing rate |
| loanQuota | String | Borrow limit of master account (or current account if loan allocation assigned) |
| surplusLmt | String | Available amount to borrow across all sub-accounts (or current account if allocation assigned) |
| usedLmt | String | Borrowed amount for current account |
| interest | String | Interest pending deduction (Market loans only) |
| interestFreeLiab | String | Interest-free liability for current account |
| potentialBorrowingAmt | String | Potential borrowing amount for current account |
| posLoan | String | Frozen amount within locked quota (VIP loans, deprecated) |
| availLoan | String | Available amount within locked quota (VIP loans, deprecated) |
| usedLoan | String | Borrowed amount (VIP loans, deprecated) |
| avgRate | String | Average hourly interest of borrowed coin (VIP loans, deprecated) |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "debt": "0.85893159114900247077000000000000",
            "interest": "0.00000000000000000000000000000000",
            "loanAlloc": "",
            "nextDiscountTime": "1729490400000",
            "nextInterestTime": "1729490400000",
            "records": [
                {
                    "availLoan": "",
                    "avgRate": "",
                    "ccy": "BTC",
                    "interest": "0",
                    "loanQuota": "175.00000000",
                    "posLoan": "",
                    "rate": "0.0000276",
                    "surplusLmt": "175.00000000",
                    "surplusLmtDetails": {},
                    "usedLmt": "0.00000000",
                    "usedLoan": "",
                    "interestFreeLiab": "",
                    "potentialBorrowingAmt": ""
                }
            ]
        }
    ],
    "msg": ""
}
```

---

#### Get Interest Accrued Data
- **Endpoint**: `GET /api/v5/account/interest-accrued`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Get interest accrued data for the past year.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | String | No | Loan type. `2`: Market loans. Default is `2`. |
| ccy | String | No | Loan currency, e.g. `BTC`. Only for Market loans / MARGIN. |
| instId | String | No | Instrument ID, e.g. `BTC-USDT`. Only for Market loans. |
| mgnMode | String | No | Margin mode: `cross` or `isolated`. Only for Market loans. |
| after | String | No | Pagination — records earlier than timestamp (ms) |
| before | String | No | Pagination — records newer than timestamp (ms) |
| limit | String | No | Results per request. Max 100, default 100. |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| type | String | Loan type. `2`: Market loans |
| ccy | String | Loan currency |
| instId | String | Instrument ID (Market loans only) |
| mgnMode | String | Margin mode: `cross` or `isolated` |
| interest | String | Interest accrued |
| interestRate | String | **Hourly** borrowing interest rate |
| liab | String | Liability |
| totalLiab | String | Total liability for current account |
| interestFreeLiab | String | Interest-free liability for current account |
| ts | String | Timestamp for interest accrual (ms Unix) |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "instId": "",
            "interest": "0.0003960833333334",
            "interestRate": "0.0000040833333333",
            "liab": "97",
            "totalLiab": "",
            "interestFreeLiab": "",
            "mgnMode": "",
            "ts": "1637312400000",
            "type": "1"
        }
    ],
    "msg": ""
}
```

---

#### Get Maximum Loan of Instrument
- **Endpoint**: `GET /api/v5/account/max-loan`
- **Rate Limit**: 20 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Get the maximum loan amount for an instrument or currency. Behavior varies by account mode and margin mode.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| mgnMode | String | Yes | Margin mode: `isolated` or `cross` |
| instId | String | Conditional | Instrument ID (single or up to 5 comma-separated), e.g. `BTC-USDT,ETH-USDT` |
| ccy | String | Conditional | Currency. For manual borrow max loan in Spot mode (enabled borrowing). |
| mgnCcy | String | Conditional | Margin currency. For isolated MARGIN and cross MARGIN in Futures mode. |
| tradeQuoteCcy | String | No | Quote currency for trading. Only applicable to SPOT. Default is quote currency of instId. |

- **Request Examples**:
```
# Max loan of cross MARGIN for trading pair in Spot mode (enabled borrowing)
GET /api/v5/account/max-loan?instId=BTC-USDT&mgnMode=cross

# Max loan for currency in Spot mode (enabled borrowing)
GET /api/v5/account/max-loan?ccy=USDT&mgnMode=cross

# Max loan of isolated MARGIN in Futures mode
GET /api/v5/account/max-loan?instId=BTC-USDT&mgnMode=isolated

# Max loan of cross MARGIN in Futures mode (margin currency BTC)
GET /api/v5/account/max-loan?instId=BTC-USDT&mgnMode=cross&mgnCcy=BTC

# Max loan of cross MARGIN in Multi-currency margin
GET /api/v5/account/max-loan?instId=BTC-USDT&mgnMode=cross
```

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| instId | String | Instrument ID |
| mgnMode | String | Margin mode |
| mgnCcy | String | Margin currency |
| maxLoan | String | Maximum loan amount |
| ccy | String | Currency |
| side | String | Order side: `buy` or `sell` |

- **Response Example**:
```json
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "instId": "BTC-USDT",
            "mgnMode": "isolated",
            "mgnCcy": "",
            "maxLoan": "0.1",
            "ccy": "BTC",
            "side": "sell"
        },
        {
            "instId": "BTC-USDT",
            "mgnMode": "isolated",
            "mgnCcy": "",
            "maxLoan": "0.2",
            "ccy": "USDT",
            "side": "buy"
        }
    ]
}
```

---

### Account Configuration (Margin-Relevant Fields)

#### Get Account Configuration
- **Endpoint**: `GET /api/v5/account/config`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Retrieve current account configuration. Key margin-related fields listed below.

- **Parameters**: None

- **Key Margin-Related Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| acctLv | String | Account mode: `1`=Spot, `2`=Futures, `3`=Multi-currency margin, `4`=Portfolio margin |
| posMode | String | Position mode: `long_short_mode` or `net_mode` |
| autoLoan | Boolean | Whether coins are borrowed automatically. `true`/`false` |
| enableSpotBorrow | Boolean | Whether borrowing is enabled in Spot mode. `true`/`false` |
| spotBorrowAutoRepay | Boolean | Whether auto-repay is enabled in Spot mode. `true`/`false` |
| ctIsoMode | String | Contract isolated margin: `automatic` (auto transfers) or `autonomy` (manual transfers) |
| mgnIsoMode | String | Margin isolated mode: `auto_transfers_ccy` (new auto transfers, both base+quote as margin), `automatic` (auto transfers), `quick_margin` (Quick Margin Mode) |
| uid | String | Account ID |
| mainUid | String | Main account ID (same as uid if master account) |
| level | String | User level, e.g. `Lv1` |
| perm | String | API key permissions: `read_only`, `trade`, `withdraw` (comma-separated) |
| type | String | Account type: `0`=Main, `1`=Standard sub, `2`=Managed trading sub |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "acctLv": "2",
            "acctStpMode": "cancel_maker",
            "autoLoan": false,
            "ctIsoMode": "automatic",
            "enableSpotBorrow": false,
            "greeksType": "PA",
            "label": "v5 test",
            "level": "Lv1",
            "mainUid": "44705892343619584",
            "mgnIsoMode": "automatic",
            "perm": "read_only,withdraw,trade",
            "posMode": "long_short_mode",
            "spotBorrowAutoRepay": false,
            "uid": "44705892343619584",
            "type": "0"
        }
    ],
    "msg": ""
}
```

---

### Account Balance (Liability Fields)

#### Get Balance
- **Endpoint**: `GET /api/v5/account/balance`
- **Rate Limit**: 10 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Retrieve assets with non-zero balance, remaining balance, and available amount. Contains critical margin/liability fields.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| ccy | String | No | Single or multiple currencies (max 20, comma-separated), e.g. `BTC,ETH` |

- **Account-Level Response Fields** (margin-relevant):

| Field | Type | Description |
|-------|------|-------------|
| totalEq | String | Total account equity in USD |
| adjEq | String | Adjusted/effective equity in USD (Spot/Multi-ccy/PM) |
| availEq | String | Account-level available equity, excluding collateral-restricted currencies (Multi-ccy/PM) |
| imr | String | Initial margin requirement in USD — sum of all cross positions + pending orders (Spot/Multi-ccy/PM) |
| mmr | String | Maintenance margin requirement in USD (Spot/Multi-ccy/PM) |
| mgnRatio | String | Maintenance margin ratio in USD (Spot/Multi-ccy/PM) |
| borrowFroz | String | Potential borrowing IMR of the account in USD (Spot/Multi-ccy/PM) |
| notionalUsd | String | Notional value of positions in USD |
| notionalUsdForBorrow | String | Notional value for borrows in USD (Spot/Multi-ccy/PM) |

- **Currency Detail Response Fields** (details[] — margin/liability relevant):

| Field | Type | Description |
|-------|------|-------------|
| ccy | String | Currency |
| eq | String | Equity of currency |
| cashBal | String | Cash balance |
| availBal | String | Available balance |
| availEq | String | Available equity (Futures/Multi-ccy/PM) |
| frozenBal | String | Frozen balance |
| liab | String | **Liabilities** of currency. Positive value, e.g. `21625.64`. (Spot/Multi-ccy/PM) |
| crossLiab | String | **Cross liabilities** of currency (Spot/Multi-ccy/PM) |
| isoLiab | String | **Isolated liabilities** of currency (Multi-ccy/PM) |
| interest | String | Accrued interest of currency. Positive value. (Spot/Multi-ccy/PM) |
| maxLoan | String | Max borrowable amount under current conditions. Affects margin borrowing & transfers. (cross Spot/Multi-ccy/PM) |
| twap | String | Risk indicator of forced repayment (0-5, higher = more likely FRP triggered). (Spot/Multi-ccy/PM) |
| frpType | String | Forced repayment type: `0`=none, `1`=user-based, `2`=platform-based. Returned when twap >= 1. |
| uplLiab | String | Liabilities due to unrealized loss (Multi-ccy/PM) |
| borrowFroz | String | Potential borrowing IMR in USD (Multi-ccy/PM) |
| eqUsd | String | Equity in USD |
| upl | String | Sum of unrealized PnL of all margin/derivatives positions (Futures/Multi-ccy/PM) |
| ordFrozen | String | Margin frozen for open orders |
| collateralEnabled | Boolean | `true`: collateral enabled. `false`: collateral disabled. (Multi-ccy) |
| colRes | String | Platform collateral restriction: `0`=none, `1`=near limit, `2`=restricted (can't use as margin for new orders) |
| colBorrAutoConversion | String | Auto-conversion risk (1-5, higher = more likely). `0` = no risk. `5` = undergoing conversion now. |
| disEq | String | Discount equity in USD (Spot borrow/Multi-ccy/PM) |

- **Applicability by Account Mode** (key liability fields):

| Field | Spot | Futures | Multi-ccy | PM |
|-------|------|---------|-----------|-----|
| liab | Yes | - | Yes | Yes |
| crossLiab | Yes | - | Yes | Yes |
| isoLiab | - | - | Yes | Yes |
| interest | Yes | - | Yes | Yes |
| twap | Yes | - | Yes | Yes |
| maxLoan | Yes | - | Yes | Yes |
| borrowFroz | Yes | - | Yes | Yes |

- **Response Example** (abbreviated):
```json
{
    "code": "0",
    "data": [
        {
            "totalEq": "55837.43556134779",
            "adjEq": "55415.624719833286",
            "borrowFroz": "0",
            "imr": "0",
            "mmr": "0",
            "mgnRatio": "",
            "notionalUsdForBorrow": "0",
            "details": [
                {
                    "ccy": "USDT",
                    "eq": "4992.890093622894",
                    "cashBal": "4850.435693622894",
                    "availBal": "4834.317093622894",
                    "frozenBal": "158.573",
                    "liab": "0",
                    "crossLiab": "0",
                    "interest": "0",
                    "maxLoan": "0",
                    "twap": "0",
                    "frpType": "0",
                    "eqUsd": "4991.542013297616",
                    "borrowFroz": "0",
                    "collateralEnabled": false,
                    "colRes": "0"
                }
            ],
            "uTime": "1705474164160"
        }
    ],
    "msg": ""
}
```

---

### Isolated Margin Settings

#### Isolated Margin Trading Settings
- **Endpoint**: `POST /api/v5/account/set-isolated-mode`
- **Rate Limit**: 5 req/2s (User ID)
- **Auth**: Trade permission required
- **Description**: Set the isolated margin trading mode for MARGIN or CONTRACTS. Cannot be adjusted when there are open positions or pending orders.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| isoMode | String | Yes | `auto_transfers_ccy`: New auto transfers (both base+quote as margin, MARGIN only). `automatic`: Auto transfers. |
| type | String | Yes | Instrument type: `MARGIN` or `CONTRACTS` |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| isoMode | String | `automatic` or `auto_transfers_ccy` |

- **Notes**:
  - **CONTRACTS** `automatic`: Auto-occupy and release margin on open/close.
  - **MARGIN** `automatic`: Auto-borrow and return coins on open/close.
  - **MARGIN** `auto_transfers_ccy`: Both base and quote currency can serve as isolated margin.

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "isoMode": "automatic"
        }
    ],
    "msg": ""
}
```

---

### Spot Margin Trading (Place Order)

#### Place Order (Spot with tdMode=cross)
- **Endpoint**: `POST /api/v5/trade/order`
- **Rate Limit**: 60 req/2s (User ID + Instrument ID)
- **Auth**: Trade permission required
- **Description**: Place a spot margin order using `tdMode=cross`. This enables borrowing when placing spot orders in margin-enabled modes.

- **Key Margin-Relevant Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| tdMode | String | Yes | Trade mode. For spot margin: `cross`. Others: `isolated`, `cash`, `spot_isolated` (lead trading). Note: `isolated` not available in Multi-ccy/PM. |
| ccy | String | No | Margin currency. Applicable to all isolated MARGIN orders and cross MARGIN orders in Futures mode. |
| side | String | Yes | `buy` or `sell` |
| ordType | String | Yes | `market`, `limit`, `post_only`, `fok`, `ioc`, `optimal_limit_ioc` |
| sz | String | Yes | Quantity. For SPOT/MARGIN limit: base currency qty. For MARGIN buy market: quote currency qty. For MARGIN sell market: base currency qty. |
| px | String | Conditional | Price. Required for `limit`, `post_only`, `fok`, `ioc`. |
| tgtCcy | String | No | Target currency for SPOT market orders: `base_ccy` or `quote_ccy`. Default: `quote_ccy` for buy, `base_ccy` for sell. |
| reduceOnly | Boolean | No | Reduce-only. For MARGIN: coin qty of all reverse pending orders + new `reduceOnly` sz cannot exceed position assets. After debt paid off, remaining does not open reverse position but trades in SPOT. Only Futures/Multi-ccy margin. |
| clOrdId | String | No | Client order ID, up to 32 chars. |
| tag | String | No | Order tag, up to 16 chars. |
| posSide | String | Conditional | Position side. `net` default. `long`/`short` required in long/short mode (FUTURES/SWAP only). |
| banAmend | Boolean | No | Disallow system size amendment for SPOT market orders. Default `false`. |

- **tdMode Rules by Account Mode**:

| Account Mode | Spot | MARGIN | FUTURES/SWAP |
|-------------|------|--------|--------------|
| Spot mode (acctLv=1) | `cash` | isolated=`isolated`, cross=`cross` | N/A |
| Futures mode (acctLv=2) | `cash` | isolated=`isolated`, cross=`cross` | cross=`cross`, isolated=`isolated` |
| Multi-currency margin (acctLv=3) | `cross` | N/A | `cross` |
| Portfolio margin (acctLv=4) | `cross` | N/A | `cross` |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ordId | String | Order ID |
| clOrdId | String | Client order ID |
| tag | String | Order tag |
| ts | String | Timestamp when processing finished (ms Unix) |
| sCode | String | Execution result code. `0` = success. |
| sMsg | String | Rejection or success message. |

- **Response Example**:
```json
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "clOrdId": "oktswap6",
            "ordId": "312269865356374016",
            "tag": "",
            "ts": "1695190491421",
            "sCode": "0",
            "sMsg": ""
        }
    ]
}
```

- **Spot Margin Order Example** (buy BTC-USDT on cross margin):
```json
POST /api/v5/trade/order
{
    "instId": "BTC-USDT",
    "tdMode": "cross",
    "side": "buy",
    "ordType": "limit",
    "px": "25000",
    "sz": "0.01"
}
```

---

### Account Risk State

#### Get Account Risk State
- **Endpoint**: `GET /api/v5/account/risk-state`
- **Rate Limit**: 10 req/2s (User ID)
- **Auth**: Read permission required
- **Description**: Get risk status. Only applicable to Portfolio margin accounts.

- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| atRisk | Boolean | `true`: account in specific risk state. `false`: not at risk. |
| atRiskIdx | Array of strings | Derivatives risk unit list |
| atRiskMgn | Array of strings | Margin risk unit list |
| ts | String | Timestamp (ms Unix) |

- **Response Example**:
```json
{
    "code": "0",
    "data": [
        {
            "atRisk": false,
            "atRiskIdx": [],
            "atRiskMgn": [],
            "ts": "1635745078794"
        }
    ],
    "msg": ""
}
```

---

## Quick Reference: Endpoint Summary

| Method | Path | Description | Rate Limit | Auth |
|--------|------|-------------|------------|------|
| POST | `/api/v5/account/spot-manual-borrow-repay` | Manual borrow/repay (Spot mode) | 1/s | Trade |
| GET | `/api/v5/account/spot-borrow-repay-history` | Borrow/repay history (Spot mode) | 5/2s | Read |
| POST | `/api/v5/account/set-auto-repay` | Enable/disable auto-repay (Spot mode) | 5/2s | Trade |
| POST | `/api/v5/account/set-auto-loan` | Enable/disable auto-loan (Multi-ccy/PM) | 5/2s | Trade |
| GET | `/api/v5/account/interest-rate` | Current hourly borrowing interest rate | 5/2s | Read |
| GET | `/api/v5/account/interest-limits` | Borrow limits, quotas, daily rates | 5/2s | Read |
| GET | `/api/v5/account/interest-accrued` | Interest accrued history (past year) | 5/2s | Read |
| GET | `/api/v5/account/max-loan` | Max loan for instrument/currency | 20/2s | Read |
| GET | `/api/v5/account/config` | Account config (acctLv, enableSpotBorrow, etc.) | 5/2s | Read |
| GET | `/api/v5/account/balance` | Balance with liab, crossLiab, interest, twap | 10/2s | Read |
| POST | `/api/v5/account/set-isolated-mode` | Set isolated margin mode | 5/2s | Trade |
| POST | `/api/v5/trade/order` | Place order (tdMode=cross for spot margin) | 60/2s | Trade |
| GET | `/api/v5/account/risk-state` | Account risk state (PM only) | 10/2s | Read |

---

## Key Notes for Implementation

### Borrow Flow in Spot Mode (acctLv=1)
1. Check `config.enableSpotBorrow` is `true`.
2. Check `config.spotBorrowAutoRepay` — if `false`, must repay manually.
3. Use `interest-limits` to check available quota (`surplusLmt`) and rate (`rate` is **daily**).
4. Use `interest-rate` for **hourly** rate.
5. Use `max-loan` to check max borrowable for specific instrument.
6. Borrow via `spot-manual-borrow-repay` with `side=borrow`.
7. Place spot order with `tdMode=cross`.
8. Repay via `spot-manual-borrow-repay` with `side=repay`.
9. Monitor liabilities via `balance` → `details[].liab` and `details[].crossLiab`.
10. Monitor risk via `balance` → `details[].twap` (0-5 scale, higher = more dangerous).

### Borrow Flow in Multi-Currency/Portfolio Margin (acctLv=3/4)
1. Set `autoLoan=true` via `set-auto-loan`.
2. Place orders with `tdMode=cross` — system auto-borrows as needed.
3. Monitor via `balance` → `details[].liab`, `details[].interest`.
4. Check `interest-limits` for quota.
5. Repayments happen automatically when positions are closed.

### Rate Conversions
- `interest-rate` returns **hourly** rate.
- `interest-limits` → `records[].rate` returns **daily** rate.
- To convert daily to hourly: `daily_rate / 24`.
- To convert hourly to annual: `hourly_rate * 24 * 365`.
