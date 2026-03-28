# Gate.io API — Margin / Borrow-Repay Reference

> Scraped from [Gate.io API v4 Docs](https://www.gate.com/docs/developers/apiv4/en/) (REST API v4.106.51) on 2026-03-28.
> Covers unified account margin borrow/repay, isolated margin (margin uni) borrow/repay, interest, loan limits, margin transfers, and spot margin orders.

## Overview

Gate.io has **two margin systems** with different API paths:

| System | Path Prefix | Description |
|--------|-------------|-------------|
| **Unified Account** | `/unified/*` | Cross-margin across all trading products. Supports multi-currency margin, portfolio margin, and single-currency margin modes. |
| **Isolated Margin (Margin Uni)** | `/margin/uni/*` | Per-trading-pair isolated margin. Each currency pair has its own margin pool. |

### Account Modes (Unified)

| Mode | Value | Description |
|------|-------|-------------|
| Classic | `classic` | Classic account mode |
| Multi-currency margin | `multi_currency` | Cross-currency margin mode |
| Portfolio margin | `portfolio` | Portfolio margin mode |
| Single-currency margin | `single_currency` | Single-currency margin mode |

### Migration Notice

As of April 2023, Gate.io migrated old `/margin/loans` endpoints to `/margin/uni/*`. The old endpoints are deprecated. Key mappings:

| Old Path | New Path |
|----------|----------|
| `GET /margin/currency_pairs` | `GET /margin/uni/currency_pairs` |
| `POST /margin/loans` | `POST /margin/uni/loans` |
| `GET /margin/loans` | `GET /margin/uni/loans` |
| `GET /margin/loans/{id}/repayment` | `GET /margin/uni/loan_records` |
| `GET /margin/borrowable` | `GET /margin/uni/borrowable` |
| - | `GET /margin/uni/interest_records` |
| - | `GET /margin/uni/estimate_rate` |

**Base URL**: `https://api.gateio.ws/api/v4`

**Auth**: All private endpoints require API key signature. See EXCHANGEAPI_GATEIO.md for auth details (KEY, SIGN, Timestamp headers).

**Rate Limits**: Private endpoints default to 200 req/10s per UID per endpoint unless noted. Spot order placement is 10 req/s total (UID+Market). Wallet transfers are 80 req/10s (UID).

---

## REST API Endpoints

### Unified Account — Account Info

#### Get Unified Account Information
- **Endpoint**: `GET /unified/accounts`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get unified account information. Assets of each currency are adjusted by liquidity coefficients and converted to USD for total asset/position value.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | No | Query by specified currency name |
| sub_uid | string | No | Sub account user ID |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| mode | string | Account mode: `classic`, `multi_currency`, `portfolio`, `single_currency` |
| user_id | integer(int64) | User ID |
| refresh_time | integer(int64) | Last refresh time |
| locked | boolean | Whether account is locked (valid in cross-currency/portfolio mode) |
| balances | object | Map of currency name to balance object |
| balances.{CCY}.available | string | Cross available balance (deducted futures isolated margin and frozen) |
| balances.{CCY}.freeze | string | Frozen amount |
| balances.{CCY}.borrowed | string | Borrowed amount (valid in cross-currency/portfolio mode, 0 in single-currency) |
| balances.{CCY}.negative_liab | string | Negative balance borrowing (valid in cross-currency/portfolio mode) |
| balances.{CCY}.equity | string | Currency equity amount (cross) |
| balances.{CCY}.total_liab | string | Total borrowed amount |
| balances.{CCY}.spot_in_use | string | Spot hedging amount (valid in portfolio mode) |
| balances.{CCY}.funding | string | Uniloan financial management amount |
| balances.{CCY}.cross_balance | string | Full margin balance (valid in single-currency mode) |
| balances.{CCY}.iso_balance | string | Futures isolated balance |
| balances.{CCY}.im | string | Cross initial margin (USDT only in single-currency mode) |
| balances.{CCY}.mm | string | Cross maintenance margin (USDT only in single-currency mode) |
| balances.{CCY}.enabled_collateral | boolean | Currency enabled as margin collateral |
| total | string | Total account assets in USD (deprecated, use unified_account_total) |
| borrowed | string | Total borrowed in USD |
| total_initial_margin | string | Total initial margin (cross) |
| total_margin_balance | string | Total margin balance (cross) |
| total_maintenance_margin | string | Total maintenance margin (cross) |
| total_initial_margin_rate | string | Total initial margin rate (cross) |
| total_maintenance_margin_rate | string | Total maintenance margin rate (cross) |
| total_available_margin | string | Available margin amount |
| unified_account_total | string | Total unified account assets |
| unified_account_total_liab | string | Total unified account borrowed |
| unified_account_total_equity | string | Total unified account equity |
| leverage | string | Account leverage multiplier (deprecated) |
| spot_order_loss | string | Spot pending order loss in USDT |
| spot_hedge | boolean | Spot hedging status |
| use_funding | boolean | Whether to use Earn funds as margin |

- **Response Example**:
```json
{
  "user_id": 10001,
  "locked": false,
  "balances": {
    "USDT": {
      "available": "0.00000062023",
      "freeze": "0",
      "borrowed": "0",
      "negative_liab": "0",
      "equity": "16.1",
      "total_liab": "0",
      "spot_in_use": "12"
    }
  },
  "total": "230.94621713",
  "borrowed": "161.66395521",
  "total_initial_margin": "1025.0524665088",
  "total_margin_balance": "3382495.944473949183",
  "total_maintenance_margin": "205.01049330176",
  "total_initial_margin_rate": "3299.827135672679",
  "total_maintenance_margin_rate": "16499.135678363399",
  "total_available_margin": "3381470.892007440383",
  "unified_account_total": "3381470.892007440383",
  "unified_account_total_liab": "0",
  "unified_account_total_equity": "100016.1",
  "leverage": "2",
  "spot_order_loss": "12",
  "spot_hedge": false
}
```

---

#### Query Mode of the Unified Account
- **Endpoint**: `GET /unified/unified_mode`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query the current unified account mode and settings.

- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| mode | string | `classic`, `multi_currency`, `portfolio`, or `single_currency` |
| settings.usdt_futures | boolean | USDT futures switch |
| settings.spot_hedge | boolean | Spot hedging switch |
| settings.use_funding | boolean | Earn switch (use Earn funds as margin) |
| settings.options | boolean | Options switch |

---

#### Set Unified Account Mode
- **Endpoint**: `PUT /unified/unified_mode`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Switch unified account mode. Each mode switch only requires the corresponding mode parameter. Also supports toggling config switches during the switch.

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| mode | string | Yes | `classic`, `multi_currency`, `portfolio`, or `single_currency` |
| settings.usdt_futures | boolean | No | USDT futures switch |
| settings.spot_hedge | boolean | No | Spot hedging switch |
| settings.use_funding | boolean | No | Earn switch |
| settings.options | boolean | No | Options switch |

- **Response**: `204 No Content` on success

- **Request Example**:
```json
{
  "mode": "portfolio",
  "settings": {
    "spot_hedge": true,
    "usdt_futures": true,
    "options": true
  }
}
```

---

### Unified Account — Borrow & Repay

#### Borrow or Repay (Unified)
- **Endpoint**: `POST /unified/loans`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Borrow or repay in unified account. When borrowing, ensure amount is not below minimum threshold and does not exceed max limit. Interest is auto-deducted at regular intervals. For repayment, use `repaid_all=true` to repay all.

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency, e.g. `BTC`, `USDT` |
| type | string | Yes | `borrow` or `repay` |
| amount | string | Yes | Borrow or repayment amount |
| repaid_all | boolean | No | Full repayment. When `true`, overrides `amount` and repays full amount. Only for repay. |
| text | string | No | User-defined custom ID |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| tran_id | integer(int64) | Transaction ID |

- **Response Example**:
```json
{
  "tran_id": 9527
}
```

- **Request Example**:
```json
{
  "currency": "BTC",
  "amount": "0.1",
  "type": "borrow",
  "repaid_all": false,
  "text": "t-test"
}
```

---

#### Query Loans (Unified)
- **Endpoint**: `GET /unified/loans`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query outstanding loan positions for the unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | No | Filter by currency name |
| page | integer | No | Page number |
| limit | integer | No | Max items returned. Default: 100, min: 1, max: 100 |
| type | string | No | Loan type: `platform` (platform borrowing), `margin` (margin borrowing) |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency |
| currency_pair | string | Currency pair |
| amount | string | Amount to repay |
| type | string | Loan type: `platform` or `margin` |
| create_time | integer(int64) | Created time (ms) |
| update_time | integer(int64) | Last update time (ms) |

- **Response Example**:
```json
[
  {
    "currency": "USDT",
    "currency_pari": "GT_USDT",
    "amount": "1",
    "type": "margin",
    "change_time": 1673247054000,
    "create_time": 1673247054000
  }
]
```

---

#### Query Loan Records (Unified)
- **Endpoint**: `GET /unified/loan_records`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query borrow and repayment records for the unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | string | No | `borrow` or `repay` |
| currency | string | No | Filter by currency name |
| page | integer | No | Page number |
| limit | integer | No | Max items returned. Default: 100, min: 1, max: 100 |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| id | integer(int64) | Record ID |
| type | string | `borrow` or `repay` |
| repayment_type | string | `none`, `manual_repay`, `auto_repay`, `cancel_auto_repay`, `different_currencies_repayment` |
| borrow_type | string | `manual_borrow` or `auto_borrow` (returned when querying loan records) |
| currency_pair | string | Currency pair |
| currency | string | Currency |
| amount | string | Borrow or repayment amount |
| create_time | integer(int64) | Created time (ms) |

- **Response Example**:
```json
[
  {
    "id": 16442,
    "type": "borrow",
    "margin_mode": "cross",
    "currency_pair": "AE_USDT",
    "currency": "USDT",
    "amount": "1000",
    "create_time": 1673247054000,
    "repayment_type": "auto_repay"
  }
]
```

---

#### Query Interest Deduction Records (Unified)
- **Endpoint**: `GET /unified/interest_records`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query interest deduction records for the unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | No | Filter by currency name |
| page | integer | No | Page number |
| limit | integer | No | Max items returned. Default: 100, min: 1, max: 100 |
| from | integer(int64) | No | Start timestamp (Unix seconds) |
| to | integer(int64) | No | End timestamp (defaults to current time) |
| type | string | No | Loan type: `platform` or `margin`. Defaults to `margin`. |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| currency_pair | string | Currency pair |
| actual_rate | string | Actual interest rate |
| interest | string | Interest amount |
| status | integer | Status: `0` = fail, `1` = success |
| type | string | Loan type: `platform` or `margin` |
| create_time | integer(int64) | Created time (ms) |

- **Response Example**:
```json
[
  {
    "status": 1,
    "currency_pair": "BTC_USDT",
    "currency": "USDT",
    "actual_rate": "0.00000236",
    "interest": "0.00006136",
    "type": "platform",
    "create_time": 1673247054000
  }
]
```

---

### Unified Account — Borrowable & Transferable

#### Query Maximum Borrowable Amount (Unified)
- **Endpoint**: `GET /unified/borrowable`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query maximum borrowable amount for a currency in the unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name to query |

- **Response Fields** (UnifiedBorrowable):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| amount | string | Max borrowable amount |

- **Response Example**:
```json
{
  "currency": "ETH",
  "amount": "10000"
}
```

---

#### Batch Query Maximum Borrowable (Unified)
- **Endpoint**: `GET /unified/batch_borrowable`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Batch query unified account maximum borrowable amount.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currencies | array[string] | Yes | Comma-separated currency names, maximum 10 |

- **Response Fields** (array of UnifiedBorrowable):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| amount | string | Max borrowable amount |

---

#### Query Maximum Transferable Amount (Unified)
- **Endpoint**: `GET /unified/transferable`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query maximum transferable amount for a currency in the unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name to query |

- **Response Fields** (UnifiedTransferable):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| amount | string | Maximum transferable amount |

---

#### Batch Query Maximum Transferable (Unified)
- **Endpoint**: `GET /unified/transferables`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Batch query maximum transferable amount. Each currency shows the maximum value. After withdrawal, transferable amounts for all currencies will change.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currencies | string | Yes | Comma-separated currency names, up to 100 |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| amount | string | Maximum transferable amount |

---

### Unified Account — Interest Rate & Currencies

#### Query Estimated Interest Rate (Unified)
- **Endpoint**: `GET /unified/estimate_rate`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query estimated hourly lending rates for the unified account. Rates fluctuate hourly based on lending depth. When a currency is not supported, the rate returned will be an empty string.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currencies | array[string] | Yes | Comma-separated currency names, maximum 10 |

- **Response Fields** (object, key-value):

| Field | Type | Description |
|-------|------|-------------|
| {currency} | string | Estimated hourly lending rate. Empty string if unsupported. |

- **Response Example**:
```json
{
  "BTC": "0.000002",
  "GT": "0.000001",
  "ETH": ""
}
```

---

#### List Loan Currencies (Unified)
- **Endpoint**: `GET /unified/currencies`
- **Rate Limit**: 200/10s (public)
- **Auth**: No authentication required
- **Description**: List all currencies supported for borrowing in unified account.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | No | Filter by specific currency |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| name | string | Currency name |
| prec | string | Currency precision |
| min_borrow_amount | string | Minimum borrowable amount (in currency units) |
| user_max_borrow_amount | string | User's maximum borrowable limit (in USDT) |
| total_max_borrow_amount | string | Platform's maximum borrowable limit (in USDT) |
| loan_status | string | `enable` or `disable` |

- **Response Example**:
```json
[
  {
    "name": "BTC",
    "prec": "0.000001",
    "min_borrow_amount": "0.01",
    "user_max_borrow_amount": "1000000",
    "total_max_borrow_amount": "1000000",
    "loan_status": "enable"
  }
]
```

---

#### Set Loan Currency Leverage (Unified)
- **Endpoint**: `POST /unified/leverage/user_currency_setting`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Set leverage multiplier for a loan currency in the unified account.

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name |
| leverage | string | Yes | Leverage multiplier |

- **Response**: `204 No Content` on success

---

### Unified Account — Tiers & Discount

#### Query Currency Discount Tiers (Unified)
- **Endpoint**: `GET /unified/currency_discount_tiers`
- **Rate Limit**: 200/10s (public)
- **Auth**: No authentication required
- **Description**: Query unified account tiered discount information.

- **Parameters**: None

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| discount_tiers[].tier | string | Tier number |
| discount_tiers[].discount | string | Discount rate |
| discount_tiers[].lower_limit | string | Lower limit |
| discount_tiers[].upper_limit | string | Upper limit (`+` = infinity) |
| discount_tiers[].leverage | string | Position leverage |

---

#### Query Loan Margin Tiers (Unified)
- **Endpoint**: `GET /unified/loan_margin_tiers`
- **Rate Limit**: 200/10s (public)
- **Auth**: No authentication required
- **Description**: Query unified account borrowing margin tier information.

- **Parameters**: None

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| margin_tiers[].tier | string | Tier number |
| margin_tiers[].margin_rate | string | Margin rate |
| margin_tiers[].lower_limit | string | Lower limit |
| margin_tiers[].upper_limit | string | Upper limit |
| margin_tiers[].leverage | string | Position leverage |

---

## Isolated Margin (Margin Uni) Endpoints

### Isolated Margin — Currency Pairs

#### List Lending Markets (All Pairs)
- **Endpoint**: `GET /margin/uni/currency_pairs`
- **Rate Limit**: 200/10s (IP, public)
- **Auth**: No authentication required
- **Description**: List all supported margin currency pairs with borrowing info.

- **Parameters**: None

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency_pair | string | Currency pair, e.g. `BTC_USDT` |
| base_min_borrow_amount | string | Minimum borrow amount of base currency |
| quote_min_borrow_amount | string | Minimum borrow amount of quote currency |
| leverage | string | Position leverage |

- **Response Example**:
```json
[
  {
    "currency_pair": "AE_USDT",
    "base_min_borrow_amount": "100",
    "quote_min_borrow_amount": "100",
    "leverage": "3"
  }
]
```

---

#### Get Lending Market Details (Single Pair)
- **Endpoint**: `GET /margin/uni/currency_pairs/{currency_pair}`
- **Rate Limit**: 200/10s (IP, public)
- **Auth**: No authentication required
- **Description**: Get lending market details for a single margin currency pair.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency_pair | string (path) | Yes | Currency pair, e.g. `BTC_USDT` |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| currency_pair | string | Currency pair |
| base_min_borrow_amount | string | Minimum borrow amount of base currency |
| quote_min_borrow_amount | string | Minimum borrow amount of quote currency |
| leverage | string | Position leverage |

- **Response Example**:
```json
{
  "currency_pair": "AE_USDT",
  "base_min_borrow_amount": "100",
  "quote_min_borrow_amount": "100",
  "leverage": "3"
}
```

---

### Isolated Margin — Borrow & Repay

#### Borrow or Repay (Isolated Margin)
- **Endpoint**: `POST /margin/uni/loans`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Borrow or repay in isolated margin. Unlike unified borrow, this requires specifying a `currency_pair`.

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency, e.g. `BTC`, `USDT` |
| type | string | Yes | `borrow` or `repay` |
| amount | string | Yes | Borrow or repayment amount |
| repaid_all | boolean | No | Full repayment. When `true`, overrides `amount`. Only for repay. |
| currency_pair | string | Yes | Currency pair, e.g. `BTC_USDT` |

- **Response**: `204 No Content` on success

- **Request Example**:
```json
{
  "currency": "BTC",
  "amount": "0.1",
  "type": "borrow",
  "currency_pair": "BTC_USDT",
  "repaid_all": false
}
```

---

#### Query Loans (Isolated Margin)
- **Endpoint**: `GET /margin/uni/loans`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query outstanding loan positions for isolated margin.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency_pair | string | No | Filter by currency pair |
| currency | string | No | Filter by currency name |
| page | integer | No | Page number |
| limit | integer | No | Max records returned per page |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency |
| currency_pair | string | Currency pair |
| amount | string | Amount to repay |
| type | string | Loan type: `platform` or `margin` |
| create_time | integer(int64) | Created time (ms) |
| update_time | integer(int64) | Last update time (ms) |

- **Response Example**:
```json
[
  {
    "currency": "USDT",
    "currency_pari": "GT_USDT",
    "amount": "1",
    "type": "margin",
    "change_time": 1673247054000,
    "create_time": 1673247054000
  }
]
```

---

#### Query Loan Records (Isolated Margin)
- **Endpoint**: `GET /margin/uni/loan_records`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query borrow and repayment records for isolated margin.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | string | No | `borrow` or `repay` |
| currency | string | No | Filter by currency name |
| currency_pair | string | No | Filter by currency pair |
| page | integer | No | Page number |
| limit | integer | No | Max records returned per page |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| type | string | `borrow` or `repay` |
| currency_pair | string | Currency pair |
| currency | string | Currency |
| amount | string | Borrow or repayment amount |
| create_time | integer(int64) | Created time (ms) |

- **Response Example**:
```json
[
  {
    "type": "borrow",
    "currency_pair": "AE_USDT",
    "currency": "USDT",
    "amount": "1000",
    "create_time": 1673247054000
  }
]
```

---

#### Query Interest Deduction Records (Isolated Margin)
- **Endpoint**: `GET /margin/uni/interest_records`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query interest deduction records for isolated margin.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency_pair | string | No | Filter by currency pair |
| currency | string | No | Filter by currency name |
| page | integer | No | Page number |
| limit | integer | No | Max records returned per page |
| from | integer(int64) | No | Start timestamp (Unix seconds) |
| to | integer(int64) | No | End timestamp (defaults to current time) |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| currency_pair | string | Currency pair |
| actual_rate | string | Actual interest rate |
| interest | string | Interest amount |
| status | integer | Status: `0` = fail, `1` = success |
| type | string | Loan type: `margin` |
| create_time | integer(int64) | Created time (ms) |

- **Response Example**:
```json
[
  {
    "status": 1,
    "currency_pair": "BTC_USDT",
    "currency": "USDT",
    "actual_rate": "0.00000236",
    "interest": "0.00006136",
    "type": "platform",
    "create_time": 1673247054000
  }
]
```

---

### Isolated Margin — Borrowable & Interest Rate

#### Query Maximum Borrowable by Currency (Isolated Margin)
- **Endpoint**: `GET /margin/uni/borrowable`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query maximum borrowable amount for a specific currency in a margin pair.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name to query |
| currency_pair | string | Yes | Currency pair, e.g. `BTC_USDT` |

- **Response Fields** (MaxUniBorrowable):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency |
| currency_pair | string | Currency pair |
| borrowable | string | Maximum borrowable amount |

- **Response Example**:
```json
{
  "currency": "AE",
  "borrowable": "1123.344",
  "currency_pair": "AE_USDT"
}
```

---

#### Estimate Interest Rate (Isolated Margin)
- **Endpoint**: `GET /margin/uni/estimate_rate`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Estimate current hourly lending rates for isolated margin currencies. Rates change hourly based on lending depth; exact rates cannot be provided.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currencies | array[string] | Yes | Comma-separated currency names, maximum 10 |

- **Response Fields** (object, key-value):

| Field | Type | Description |
|-------|------|-------------|
| {currency} | string | Estimated hourly lending rate. Empty string if unsupported. |

- **Response Example**:
```json
{
  "BTC": "0.000002",
  "GT": "0.000001"
}
```

---

### Isolated Margin — Account Info

#### Margin Account List (Isolated)
- **Endpoint**: `GET /margin/accounts`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: List isolated margin account balances for all or specific trading pairs.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency_pair | string | No | Filter by currency pair |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| currency_pair | string | Currency pair |
| account_type | string | `mmr` (maintenance margin rate account) or `inactive` (not activated) |
| leverage | string | User's current market leverage multiplier |
| locked | boolean | Whether account is locked |
| risk | string | Deprecated |
| mmr | string | Current maintenance margin rate |
| base.currency | string | Base currency name |
| base.available | string | Available for margin trading (= margin + borrowed) |
| base.locked | string | Frozen funds (in orders) |
| base.borrowed | string | Borrowed funds |
| base.interest | string | Unpaid interest |
| quote.currency | string | Quote currency name |
| quote.available | string | Available for margin trading |
| quote.locked | string | Frozen funds (in orders) |
| quote.borrowed | string | Borrowed funds |
| quote.interest | string | Unpaid interest |

- **Response Example**:
```json
[
  {
    "currency_pair": "BTC_USDT",
    "account_type": "mmr",
    "leverage": "20",
    "locked": false,
    "risk": "1.3318",
    "mmr": "16.5949188975473644",
    "base": {
      "currency": "BTC",
      "available": "0.047060413211",
      "locked": "0",
      "borrowed": "0.047233",
      "interest": "0"
    },
    "quote": {
      "currency": "USDT",
      "available": "1234",
      "locked": "0",
      "borrowed": "0",
      "interest": "0"
    }
  }
]
```

---

#### Query User's Isolated Margin Account List
- **Endpoint**: `GET /margin/user/account`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query user's isolated margin account list. Supports querying risk ratio isolated accounts and margin ratio isolated accounts.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency_pair | string | No | Filter by currency pair |

- **Response Fields** (array): Same as `GET /margin/accounts` above.

---

#### Query Margin Account Balance Change History
- **Endpoint**: `GET /margin/account_book`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query margin account balance change history. Currently only provides transfer history. Query time range cannot exceed 30 days.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | No | Filter by currency. If specified, `currency_pair` must also be specified. |
| currency_pair | string | No | Filter by margin pair. Used with `currency`. |
| type | string | No | Account change type filter. All types if unspecified. |
| from | integer(int64) | No | Start timestamp (Unix seconds) |
| to | integer(int64) | No | End timestamp (defaults to current time) |
| page | integer | No | Page number |
| limit | integer | No | Max records returned |

- **Response Fields** (array):

| Field | Type | Description |
|-------|------|-------------|
| id | string | Balance change record ID |
| time | string | Account change timestamp |
| time_ms | integer(int64) | Timestamp in milliseconds |
| currency | string | Currency changed |
| currency_pair | string | Account trading pair |
| change | string | Amount changed (positive = transfer in, negative = transfer out) |
| balance | string | Balance after change |
| type | string | Account book type |

---

#### Get Maximum Transferable Amount (Isolated Margin)
- **Endpoint**: `GET /margin/transferable`
- **Rate Limit**: 200/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get maximum transferable amount for a specific currency in isolated margin.

- **Parameters**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name |
| currency_pair | string | No | Currency pair |

- **Response Fields** (MarginTransferable):

| Field | Type | Description |
|-------|------|-------------|
| currency | string | Currency name |
| currency_pair | string | Currency pair |
| amount | string | Max transferable amount |

---

## Spot Orders (with Margin Support)

### Create an Order
- **Endpoint**: `POST /spot/orders`
- **Rate Limit**: 10/s total per UID+Market
- **Auth**: API key + secret required
- **Description**: Create a spot order. Supports spot, margin (isolated), and unified account trading via the `account` field. When using margin account, set `auto_borrow=true` to automatically borrow if balance is insufficient. For unified account orders, `auto_repay` controls whether assets from order execution auto-repay borrowing.

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| text | string | No | User-defined info. Must be prefixed with `t-`, max 28 chars after prefix, alphanumeric + `_-.` only. |
| currency_pair | string | Yes | Currency pair, e.g. `BTC_USDT` |
| type | string | No | `limit` (default) or `market` |
| account | string | No | `spot` (default), `margin` (isolated), or `unified` |
| side | string | Yes | `buy` or `sell` |
| amount | string | Yes | Trading quantity. For `limit`: base currency. For `market`: depends on side (buy=quote, sell=base). |
| price | string | Cond. | Required when `type=limit` |
| time_in_force | string | No | `gtc` (default), `ioc`, `poc` (post-only), `fok` |
| iceberg | string | No | Displayed amount for iceberg orders. `0` or null for normal orders. |
| auto_borrow | boolean | No | Auto-borrow if insufficient balance (for `margin` or `unified` account) |
| auto_repay | boolean | No | Auto-repay after order execution (for `unified` account cross-margin orders). Can be combined with `auto_borrow`. |
| stp_act | string | No | Self-trade prevention: `cn` (cancel newest), `co` (cancel oldest), `cb` (cancel both), `-` (none) |
| action_mode | string | No | `ACK` (async, returns key fields only), `RESULT` (no clearing info), `FULL` (default, full response) |
| slippage | string | No | Max slippage ratio for market orders (e.g. `0.03` = 3%) |

- **Key Notes for Margin Trading**:
  - `account=margin`: Uses isolated margin account. `auto_borrow=true` triggers `POST /margin/uni/loans` if balance insufficient.
  - `account=unified`: Uses unified account. `auto_borrow=true` borrows from unified pool. `auto_repay=true` repays after execution.
  - Auto repayment is triggered when order ends (status = `cancelled` or `closed`).

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| id | string | Order ID |
| text | string | User-defined info |
| create_time | string | Creation time (Unix seconds) |
| update_time | string | Last modification time |
| create_time_ms | integer(int64) | Creation time (ms) |
| update_time_ms | integer(int64) | Last modification time (ms) |
| status | string | `open`, `closed`, or `cancelled` |
| currency_pair | string | Currency pair |
| type | string | `limit` or `market` |
| account | string | `spot`, `margin`, or `unified` |
| side | string | `buy` or `sell` |
| amount | string | Trading quantity |
| price | string | Trading price |
| time_in_force | string | `gtc`, `ioc`, `poc`, `fok` |
| auto_borrow | boolean | Auto-borrow flag |
| auto_repay | boolean | Auto-repay flag |
| left | string | Amount left to fill |
| filled_amount | string | Amount filled |
| filled_total | string | Total filled in quote currency |
| avg_deal_price | string | Average fill price |
| fee | string | Fee deducted |
| fee_currency | string | Fee currency |
| point_fee | string | Points used for fee |
| gt_fee | string | GT used for fee |
| gt_discount | boolean | GT fee deduction enabled |
| rebated_fee | string | Rebated fee |
| rebated_fee_currency | string | Rebated fee currency |
| stp_id | integer | STP group ID (0 if not set) |
| stp_act | string | STP action: `cn`, `co`, `cb`, `-` |
| finish_as | string | Finish reason: `open`, `filled`, `cancelled`, `liquidate_cancelled`, `depth_not_enough`, `trader_not_enough`, `small`, `ioc`, `poc`, `fok`, `stp`, `unknown` |

- **Request Example** (margin order with auto_borrow):
```json
{
  "text": "t-abc123",
  "currency_pair": "BTC_USDT",
  "type": "limit",
  "account": "margin",
  "side": "buy",
  "amount": "0.001",
  "price": "65000",
  "time_in_force": "ioc",
  "auto_borrow": true
}
```

- **Response Example** (FULL mode):
```json
{
  "id": "1852454420",
  "text": "t-abc123",
  "amend_text": "-",
  "create_time": "1710488334",
  "update_time": "1710488334",
  "create_time_ms": 1710488334073,
  "update_time_ms": 1710488334074,
  "status": "closed",
  "currency_pair": "BTC_USDT",
  "type": "limit",
  "account": "unified",
  "side": "buy",
  "amount": "0.001",
  "price": "65000",
  "time_in_force": "gtc",
  "iceberg": "0",
  "left": "0",
  "filled_amount": "0.001",
  "fill_price": "63.4693",
  "filled_total": "63.4693",
  "avg_deal_price": "63469.3",
  "fee": "0.00000022",
  "fee_currency": "BTC",
  "point_fee": "0",
  "gt_fee": "0",
  "gt_discount": false,
  "rebated_fee": "0",
  "rebated_fee_currency": "USDT",
  "stp_id": 0,
  "stp_act": "-",
  "finish_as": "filled"
}
```

---

## Wallet Transfers (Margin Support)

### Transfer Between Trading Accounts
- **Endpoint**: `POST /wallet/transfers`
- **Rate Limit**: 80/10s (UID)
- **Auth**: API key + secret required
- **Description**: Transfer funds between personal trading accounts (spot, margin, futures, delivery, options).

- **Parameters** (JSON body):

| Name | Type | Required | Description |
|------|------|----------|-------------|
| currency | string | Yes | Currency name. For contract accounts, can be `POINT` or settlement currencies. |
| from | string | Yes | Source account: `spot`, `margin`, `futures`, `delivery`, `options` |
| to | string | Yes | Destination account: `spot`, `margin`, `futures`, `delivery`, `options` |
| amount | string | Yes | Transfer amount (up to 8 decimals, must be > 0) |
| currency_pair | string | Cond. | **Required** when transferring to/from `margin` account |
| settle | string | Cond. | **Required** when transferring to/from `futures` or `delivery` account |

- **Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| tx_id | integer(int64) | Transaction ID |

- **Request Example** (spot to margin):
```json
{
  "currency": "USDT",
  "from": "spot",
  "to": "margin",
  "amount": "100",
  "currency_pair": "BTC_USDT"
}
```

- **Response Example**:
```json
{
  "tx_id": 59636381286
}
```

---

## Key Differences: Unified vs Isolated Margin

| Feature | Unified (`/unified/*`) | Isolated (`/margin/uni/*`) |
|---------|----------------------|--------------------------|
| Margin scope | Cross-margin across all products | Per-trading-pair isolated pools |
| Borrow endpoint | `POST /unified/loans` | `POST /margin/uni/loans` |
| Requires currency_pair | No (cross-margin) | Yes (pair-specific) |
| Borrow response | Returns `tran_id` (200 OK) | Returns `204 No Content` |
| Account query | `GET /unified/accounts` | `GET /margin/accounts` |
| Borrowable query | `GET /unified/borrowable` | `GET /margin/uni/borrowable` |
| Interest rate | `GET /unified/estimate_rate` | `GET /margin/uni/estimate_rate` |
| Currency pairs | `GET /unified/currencies` | `GET /margin/uni/currency_pairs` |
| Spot order account | `account=unified` | `account=margin` |
| Auto borrow | `auto_borrow=true` on spot orders | `auto_borrow=true` on spot orders |
| Auto repay | `auto_repay=true` on spot orders | Depends on user's auto-repay setting |
| Transfer | Uses unified balance directly | `POST /wallet/transfers` with `currency_pair` |
