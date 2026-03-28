# Gate.io Unified Account API Documentation Reference

> Source: https://www.gate.com/docs/developers/apiv4/en/#unified (REST API v4.106.51)
> Base URL: `https://api.gateio.ws/api/v4`

## Overview

The Unified Account system consolidates spot, margin, futures, and options under a single account with shared margin. It supports four modes:

| Mode | Value | Description |
|------|-------|-------------|
| Classic | `classic` | Classic account mode |
| Multi-currency margin | `multi_currency` | Cross-currency margin mode |
| Portfolio margin | `portfolio` | Portfolio margin mode |
| Single-currency margin | `single_currency` | Single-currency margin mode |

## Authentication

All endpoints in this section require API key + secret authentication unless noted otherwise.

Required headers: `KEY`, `SIGN`, `Timestamp` (see main Gate.io API docs for signature generation).

---

## REST API Endpoints

### Unified Account — Account Information

#### Get Unified Account Information
- **Endpoint**: `GET /unified/accounts`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get unified account information. The assets of each currency in the account will be adjusted according to their liquidity, defined by corresponding adjustment coefficients, and then uniformly converted to USD to calculate the total asset value and position value of the account.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | No | Query by specified currency name |
| sub_uid | query | string | No | Sub account user ID |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| mode | string | Unified account mode: `classic`, `multi_currency`, `portfolio`, `single_currency` |
| user_id | integer(int64) | User ID |
| refresh_time | integer(int64) | Last refresh time |
| locked | boolean | Whether the account is locked, valid in cross-currency margin/portfolio margin mode, false in other modes |
| balances | object | Map of currency name to UnifiedBalance object |
| balances.{currency}.available | string | Cross available balance, deducted futures isolated margin occupation and frozen amount |
| balances.{currency}.freeze | string | Frozen amount |
| balances.{currency}.borrowed | string | Borrowed amount, valid in cross-currency margin/portfolio margin mode, 0 in other modes |
| balances.{currency}.negative_liab | string | Negative balance borrowing, valid in cross-currency margin/portfolio margin mode |
| balances.{currency}.futures_pos_liab | string | Contract opening position borrowing currency (deprecated) |
| balances.{currency}.equity | string | Currency equity amount (cross) |
| balances.{currency}.total_freeze | string | Total frozen (deprecated) |
| balances.{currency}.total_liab | string | Total borrowed amount, valid in cross-currency margin/portfolio margin mode |
| balances.{currency}.spot_in_use | string | Amount of spot hedging, valid in portfolio margin mode, 0 in other modes |
| balances.{currency}.funding | string | Uniloan financial management amount, effective when turned on as unified account margin switch |
| balances.{currency}.funding_version | string | Funding version |
| balances.{currency}.cross_balance | string | Full margin balance, valid in single currency margin mode, 0 in other modes |
| balances.{currency}.iso_balance | string | Futures isolated balance, effective in single-currency and multi-currency margin mode, 0 in portfolio margin mode |
| balances.{currency}.im | string | Cross initial margin, only effective for USDT in single-currency margin mode |
| balances.{currency}.mm | string | Cross maintenance margin, only effective for USDT in single-currency margin mode |
| balances.{currency}.imr | string | Cross initial margin rate, only effective for USDT in single-currency margin mode |
| balances.{currency}.mmr | string | Cross maintenance margin rate, only effective for USDT in single-currency margin mode |
| balances.{currency}.margin_balance | string | Cross margin balance, only effective for USDT in single-currency margin mode |
| balances.{currency}.available_margin | string | Cross available margin, only effective for USDT in single-currency margin mode |
| balances.{currency}.enabled_collateral | boolean | Currency enabled as collateral: true - Enabled, false - Disabled |
| balances.{currency}.balance_version | number(int64) | Balance version number |
| total | string | Total account assets converted to USD (deprecated, replaced by unified_account_total) |
| borrowed | string | Total borrowed amount converted to USD, valid in cross-currency margin/portfolio margin mode |
| total_initial_margin | string | Total initial margin (cross), effective in multi-currency margin/portfolio margin mode |
| total_margin_balance | string | Total margin balance (cross), effective in multi-currency margin/portfolio margin mode |
| total_maintenance_margin | string | Total maintenance margin (cross), effective in multi-currency margin/portfolio margin mode |
| total_initial_margin_rate | string | Total initial margin rate (cross), effective in multi-currency margin/portfolio margin mode |
| total_maintenance_margin_rate | string | Total maintenance margin rate (cross), effective in multi-currency margin/portfolio margin mode |
| total_available_margin | string | Available margin amount, valid in cross-currency margin/portfolio margin mode |
| unified_account_total | string | Total unified account assets, includes both cross and isolated total assets |
| unified_account_total_liab | string | Total unified account borrowed (total cross borrowed) |
| unified_account_total_equity | string | Total unified account equity |
| leverage | string | Account leverage multiplier (deprecated). Use GET /unified/leverage/user_currency_setting instead |
| spot_order_loss | string | Spot Pending Order Loss, in USDT, effective in Cross-Currency Margin and Portfolio Margin modes |
| options_order_loss | string | Option Pending Order Loss, in USDT, effective only in Portfolio Margin Mode |
| spot_hedge | boolean | Spot hedging status: true - enabled, false - disabled |
| use_funding | boolean | Whether to use Earn funds as margin |
| is_all_collateral | boolean | Whether all currencies are used as margin |

- **Response Example**:
```json
{
  "user_id": 10001,
  "locked": false,
  "balances": {
    "ETH": {
      "available": "0",
      "freeze": "0",
      "borrowed": "0.075393666654",
      "negative_liab": "0",
      "futures_pos_liab": "0",
      "equity": "1016.1",
      "total_freeze": "0",
      "total_liab": "0",
      "spot_in_use": "1.111"
    },
    "USDT": {
      "available": "0.00000062023",
      "freeze": "0",
      "borrowed": "0",
      "negative_liab": "0",
      "futures_pos_liab": "0",
      "equity": "16.1",
      "total_freeze": "0",
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

### Unified Account — Account Mode

#### Query Mode of the Unified Account
- **Endpoint**: `GET /unified/unified_mode`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query mode of the unified account.
- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| mode | string | Unified account mode: `classic`, `multi_currency`, `portfolio`, `single_currency` |
| settings | object | Mode settings |
| settings.usdt_futures | boolean | USDT futures switch. In cross-currency margin mode, can only be enabled and cannot be disabled |
| settings.spot_hedge | boolean | Spot hedging switch |
| settings.use_funding | boolean | Earn switch, when mode is cross-currency margin mode, whether to use Earn funds as margin |
| settings.options | boolean | Options switch. In cross-currency margin mode, can only be enabled and cannot be disabled |

- **Response Example**:
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

#### Set Unified Account Mode
- **Endpoint**: `PUT /unified/unified_mode`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Set unified account mode. Each account mode switch only requires passing the corresponding account mode parameter, and also supports turning on or off the configuration switches under the corresponding account mode during the switch.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| body | body | object | Yes | Request body |
| mode | body | string | Yes | Unified account mode: `classic`, `multi_currency`, `portfolio`, `single_currency` |
| settings | body | object | No | Mode settings |
| settings.usdt_futures | body | boolean | No | USDT futures switch. In cross-currency margin mode, can only be enabled and cannot be disabled |
| settings.spot_hedge | body | boolean | No | Spot hedging switch |
| settings.use_funding | body | boolean | No | Earn switch, when mode is cross-currency margin mode, whether to use Earn funds as margin |
| settings.options | body | boolean | No | Options switch. In cross-currency margin mode, can only be enabled and cannot be disabled |

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

### Unified Account — Borrowing & Repayment

#### Borrow or Repay
- **Endpoint**: `POST /unified/loans`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Borrow or repay. When borrowing, ensure the borrowed amount is not below the minimum borrowing threshold for the specific cryptocurrency and does not exceed the maximum borrowing limit set by the platform and user. Loan interest will be automatically deducted from the account at regular intervals. For repayment, use `repaid_all=true` to repay all available amounts.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| body | body | object | Yes | Request body |
| currency | body | string | Yes | Currency |
| type | body | string | Yes | Type: `borrow` - borrow, `repay` - repay |
| amount | body | string | Yes | Borrow or repayment amount |
| repaid_all | body | boolean | No | Full repayment, only used for repayment operations. When set to true, overrides amount and directly repays the full amount |
| text | body | string | No | User defined custom ID |

- **Enumerated Values**:

| Parameter | Value |
|---|---|
| type | `borrow` |
| type | `repay` |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| tran_id | integer(int64) | Transaction ID |

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

- **Response Example**:
```json
{
  "tran_id": 9527
}
```

---

#### Query Loans
- **Endpoint**: `GET /unified/loans`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query current loans.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | No | Query by specified currency name |
| page | query | integer(int32) | No | Page number |
| limit | query | integer(int32) | No | Maximum number of items returned. Default: 100, minimum: 1, maximum: 100 |
| type | query | string | No | Loan type: `platform` - platform borrowing, `margin` - margin borrowing |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency |
| currency_pair | string | Currency pair |
| amount | string | Amount to Repay |
| type | string | Loan type: `platform` - platform borrowing, `margin` - margin borrowing |
| create_time | integer(int64) | Created time |
| update_time | integer(int64) | Last update time |

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

#### Query Loan Records
- **Endpoint**: `GET /unified/loan_records`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query loan records (borrow and repay history).
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| type | query | string | No | Loan record type: `borrow` - borrowing, `repay` - repayment |
| currency | query | string | No | Query by specified currency name |
| page | query | integer(int32) | No | Page number |
| limit | query | integer(int32) | No | Maximum number of items returned. Default: 100, minimum: 1, maximum: 100 |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| id | integer(int64) | Record ID |
| type | string | Type: `borrow` - borrow, `repay` - repay |
| repayment_type | string | Repayment type: `none`, `manual_repay`, `auto_repay`, `cancel_auto_repay` (auto repayment after order cancellation), `different_currencies_repayment` (cross-currency repayment) |
| borrow_type | string | Borrowing type (returned when querying loan records): `manual_borrow` - Manual borrowing, `auto_borrow` - Automatic borrowing |
| currency_pair | string | Currency pair |
| currency | string | Currency |
| amount | string | Borrow or repayment amount |
| create_time | integer(int64) | Created time |

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

#### Query Interest Deduction Records
- **Endpoint**: `GET /unified/interest_records`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query interest deduction records.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | No | Query by specified currency name |
| page | query | integer(int32) | No | Page number |
| limit | query | integer(int32) | No | Maximum number of items returned. Default: 100, minimum: 1, maximum: 100 |
| from | query | integer(int64) | No | Start timestamp for the query |
| to | query | integer(int64) | No | End timestamp for the query, defaults to current time if not specified |
| type | query | string | No | Loan type: `platform` - platform borrowing, `margin` - margin borrowing. Defaults to margin if not specified |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| currency_pair | string | Currency pair |
| actual_rate | string | Actual interest rate |
| interest | string | Interest amount |
| status | integer | Status: 0 - fail, 1 - success |
| type | string | Loan type: `margin` - margin borrowing, `platform` - platform borrowing |
| create_time | integer(int64) | Created time |

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

### Unified Account — Borrowable & Transferable Amounts

#### Query Maximum Borrowable Amount
- **Endpoint**: `GET /unified/borrowable`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query maximum borrowable amount for unified account.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | Yes | Query by specified currency name |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| amount | string | Maximum borrowable amount |

- **Response Example**:
```json
{
  "currency": "ETH",
  "amount": "10000"
}
```

---

#### Batch Query Maximum Borrowable Amount
- **Endpoint**: `GET /unified/batch_borrowable`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Batch query unified account maximum borrowable amount.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currencies | query | array[string] | Yes | Specify currency names for querying in an array, separated by commas, maximum 10 currencies |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| amount | string | Maximum borrowable amount |

- **Response Example**:
```json
[
  {
    "currency": "BTC",
    "amount": "123456"
  }
]
```

---

#### Query Maximum Transferable Amount
- **Endpoint**: `GET /unified/transferable`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query maximum transferable amount for unified account.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | Yes | Query by specified currency name |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| amount | string | Maximum transferable amount |

- **Response Example**:
```json
{
  "currency": "ETH",
  "amount": "10000"
}
```

---

#### Batch Query Maximum Transferable Amount
- **Endpoint**: `GET /unified/transferables`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Batch query maximum transferable amount for unified accounts. Each currency shows the maximum value. After user withdrawal, the transferable amount for all currencies will change.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currencies | query | string | Yes | Specify the currency name to query in batches, supports up to 100 pass parameters at a time |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency detail |
| amount | string | Maximum transferable amount |

- **Response Example**:
```json
[
  {
    "currency": "BTC",
    "amount": "123456"
  }
]
```

---

### Unified Account — Interest Rate & Estimation

#### Query Estimated Interest Rate
- **Endpoint**: `GET /unified/estimate_rate`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Query unified account estimated interest rate. Interest rates fluctuate hourly based on lending depth, so exact rates cannot be provided. When a currency is not supported, the interest rate returned will be an empty string.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currencies | query | array[string] | Yes | Specify currency names for querying in an array, separated by commas, maximum 10 currencies |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| {currency} | string | Estimated current hourly lending rate for the currency (map of currency name to rate string). Empty string if currency is not supported. |

- **Response Example**:
```json
{
  "BTC": "0.000002",
  "GT": "0.000001",
  "ETH": ""
}
```

---

#### Get Historical Lending Rates
- **Endpoint**: `GET /unified/history_loan_rate`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: No authentication required (public)
- **Description**: Get historical lending rates.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| tier | query | string | No | VIP level for the floating rate to be queried |
| currency | query | string | Yes | Currency |
| page | query | integer(int32) | No | Page number |
| limit | query | integer(int32) | No | Maximum number of items returned. Default: 100, minimum: 1, maximum: 100 |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| tier | string | VIP level for the floating rate |
| tier_up_rate | string | Floating rate corresponding to VIP level |
| rates | array | Historical interest rate information, one data point per hour, sorted by time from recent to distant |
| rates[].time | integer(int64) | Hourly timestamp corresponding to this interest rate, in milliseconds |
| rates[].rate | string | Historical interest rate for this hour |

- **Response Example**:
```json
{
  "currency": "USDT",
  "tier": "1",
  "tier_up_rate": "1.18",
  "rates": [
    {
      "time": 1729047616000,
      "rate": "0.00010287"
    }
  ]
}
```

---

### Unified Account — Currencies & Tiers

#### List Loan Currencies Supported by Unified Account
- **Endpoint**: `GET /unified/currencies`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: No authentication required (public)
- **Description**: List of loan currencies supported by unified account.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | No | Currency (filter by specific currency) |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| name | string | Currency name |
| prec | string | Currency precision |
| min_borrow_amount | string | Minimum borrowable limit, in currency units |
| user_max_borrow_amount | string | User's maximum borrowable limit, in USDT |
| total_max_borrow_amount | string | Platform's maximum borrowable limit, in USDT |
| loan_status | string | Lending status: `disable` - Lending prohibited, `enable` - Lending supported |

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

#### Query Unified Account Currency Discount Tiers
- **Endpoint**: `GET /unified/currency_discount_tiers`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: No authentication required (public)
- **Description**: Query unified account tiered currency discount information.
- **Parameters**: None

- **Response Fields** (array of arrays of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| discount_tiers | array | Tiered discount array |
| discount_tiers[].tier | string | Tier number |
| discount_tiers[].discount | string | Discount rate |
| discount_tiers[].lower_limit | string | Lower limit |
| discount_tiers[].upper_limit | string | Upper limit (`+` indicates positive infinity) |
| discount_tiers[].leverage | string | Position leverage |

- **Response Example**:
```json
[
  [
    {
      "currency": "USDT",
      "discount_tiers": [
        {
          "tier": "1",
          "discount": "1",
          "lower_limit": "0",
          "leverage": "10",
          "upper_limit": "+"
        }
      ]
    },
    {
      "currency": "BTC",
      "discount_tiers": [
        {
          "tier": "1",
          "discount": "0.98",
          "lower_limit": "0",
          "leverage": "10",
          "upper_limit": "1000"
        },
        {
          "tier": "2",
          "discount": "0.95",
          "lower_limit": "1000",
          "leverage": "10",
          "upper_limit": "10000"
        }
      ]
    }
  ]
]
```

---

#### Query Unified Account Tiered Loan Margin
- **Endpoint**: `GET /unified/loan_margin_tiers`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: No authentication required (public)
- **Description**: Query unified account tiered loan margin.
- **Parameters**: None

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| margin_tiers | array | Tiered margin array |
| margin_tiers[].tier | string | Tier number |
| margin_tiers[].margin_rate | string | Margin rate (discount) |
| margin_tiers[].lower_limit | string | Lower limit |
| margin_tiers[].upper_limit | string | Upper limit (empty string indicates greater than for last tier) |
| margin_tiers[].leverage | string | Position leverage |

- **Response Example**:
```json
[
  {
    "currency": "USDT",
    "margin_tiers": [
      {
        "tier": "1",
        "margin_rate": "0.02",
        "lower_limit": "200000",
        "upper_limit": "400000",
        "leverage": "3"
      }
    ]
  }
]
```

---

### Unified Account — Leverage Settings

#### Get Maximum and Minimum Currency Leverage Config
- **Endpoint**: `GET /unified/leverage/user_currency_config`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get the maximum and minimum currency leverage that can be set.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | Yes | Currency |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| current_leverage | string | Current leverage ratio |
| min_leverage | string | Minimum adjustable leverage ratio |
| max_leverage | string | Maximum adjustable leverage ratio |
| debit | string | Current liabilities |
| available_margin | string | Available margin |
| borrowable | string | Maximum borrowable amount at current leverage |
| except_leverage_borrowable | string | Maximum borrowable from margin and maximum borrowable from Earn, whichever is smaller |

- **Response Example**:
```json
{
  "current_leverage": "2",
  "min_leverage": "0",
  "max_leverage": "0",
  "debit": "0",
  "available_margin": "0",
  "borrowable": "0",
  "except_leverage_borrowable": "0"
}
```

---

#### Get User Currency Leverage
- **Endpoint**: `GET /unified/leverage/user_currency_setting`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get user currency leverage. If currency is not specified, query all currencies.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| currency | query | string | No | Currency (omit to query all currencies) |

- **Response Fields** (array of objects):

| Field | Type | Description |
|---|---|---|
| currency | string | Currency name |
| leverage | string | Leverage multiplier |

- **Response Example**:
```json
[
  {
    "currency": "BTC",
    "leverage": "3"
  }
]
```

---

#### Set Loan Currency Leverage
- **Endpoint**: `POST /unified/leverage/user_currency_setting`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Set loan currency leverage.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| body | body | object | Yes | Request body |
| currency | body | string | Yes | Currency name |
| leverage | body | string | Yes | Leverage multiplier |

- **Response**: `204 No Content` on success

- **Request Example**:
```json
{
  "currency": "BTC",
  "leverage": "3"
}
```

---

### Unified Account — Collateral Settings

#### Set Collateral Currency
- **Endpoint**: `POST /unified/collateral_currencies`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Set collateral currency configuration.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| body | body | object | Yes | Request body |
| collateral_type | body | integer | No | User-set collateral mode: `0` (all) - All currencies as collateral, `1` (custom) - Custom currencies as collateral. When collateral_type is 0, enable_list and disable_list parameters are invalid |
| enable_list | body | array[string] | No | Currency list. When collateral_type=1, indicates addition logic |
| disable_list | body | array[string] | No | Disable list, indicating the disable logic |

- **Enumerated Values**:

| Parameter | Value |
|---|---|
| collateral_type | `0` (all currencies) |
| collateral_type | `1` (custom) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| is_success | boolean | Whether the setting was successful |

- **Request Example**:
```json
{
  "collateral_type": 1,
  "enable_list": ["BTC", "ETH"],
  "disable_list": ["SOL", "GT"]
}
```

- **Response Example**:
```json
{
  "is_success": true
}
```

---

### Unified Account — Risk & Portfolio

#### Get User Risk Unit Details
- **Endpoint**: `GET /unified/risk_units`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: API key + secret required
- **Description**: Get user risk unit details, only valid in portfolio margin mode.
- **Parameters**: None

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| user_id | integer(int64) | User ID |
| spot_hedge | boolean | Spot hedging status: true - enabled, false - disabled |
| risk_units | array | Risk unit array |
| risk_units[].symbol | string | Risk unit flag |
| risk_units[].spot_in_use | string | Spot hedging occupied amount |
| risk_units[].maintain_margin | string | Maintenance margin for risk unit |
| risk_units[].initial_margin | string | Initial margin for risk unit |
| risk_units[].delta | string | Total Delta of risk unit |
| risk_units[].gamma | string | Total Gamma of risk unit |
| risk_units[].theta | string | Total Theta of risk unit |
| risk_units[].vega | string | Total Vega of risk unit |

- **Response Example**:
```json
{
  "user_id": 0,
  "spot_hedge": true,
  "risk_units": [
    {
      "symbol": "BTC",
      "spot_in_use": "-13500.000001223",
      "maintain_margin": "2334.002",
      "initial_margin": "2334.002",
      "delta": "0.22",
      "gamma": "0.42",
      "theta": "0.29",
      "vega": "0.22"
    }
  ]
}
```

---

#### Portfolio Margin Calculator
- **Endpoint**: `POST /unified/portfolio_calculator`
- **Rate Limit**: 200r/10s (UID)
- **Auth**: No authentication required (public)
- **Description**: Portfolio Margin Calculator. Calculates maintenance and initial margin requirements under the portfolio margin model for custom simulated position and order portfolios. Supports all underlying currencies with active options trading.
- **Parameters**:

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| body | body | object | Yes | Request body |
| spot_balances | body | array | No | Spot positions |
| spot_balances[].currency | body | string | Yes | Currency name |
| spot_balances[].equity | body | string | Yes | Currency equity (balance - borrowed), represents net delta exposure |
| spot_orders | body | array | No | Spot orders |
| spot_orders[].currency_pairs | body | string | Yes | Market |
| spot_orders[].order_price | body | string | Yes | Price |
| spot_orders[].count | body | string | No | Initial order quantity (not involved in actual calculation) |
| spot_orders[].left | body | string | Yes | Unfilled quantity, involved in actual calculation |
| spot_orders[].type | body | string | Yes | Order type: `sell` - sell order, `buy` - buy order |
| futures_positions | body | array | No | Futures positions |
| futures_positions[].contract | body | string | Yes | Perpetual contract name (only USDT perpetual contracts for underlying currencies with active options trading) |
| futures_positions[].size | body | string | Yes | Position size, measured in contract quantity |
| futures_orders | body | array | No | Futures orders |
| futures_orders[].contract | body | string | Yes | Perpetual contract name |
| futures_orders[].size | body | string | Yes | Contract quantity (initial order quantity, not involved in actual settlement) |
| futures_orders[].left | body | string | Yes | Unfilled contract quantity, involved in actual calculation |
| options_positions | body | array | No | Options positions |
| options_positions[].options_name | body | string | Yes | Options contract name |
| options_positions[].size | body | string | Yes | Position size, measured in contract quantity |
| options_orders | body | array | No | Option orders |
| options_orders[].options_name | body | string | Yes | Options contract name |
| options_orders[].size | body | string | Yes | Initial order quantity (not involved in actual calculation) |
| options_orders[].left | body | string | Yes | Unfilled contract quantity, involved in actual calculation |
| spot_hedge | body | boolean | No | Whether to enable spot hedging |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| maintain_margin_total | string | Total maintenance margin (portfolio margin calculation, excludes borrowing margin) |
| initial_margin_total | string | Total initial margin (max of: position, position + positive delta orders, position + negative delta orders) |
| calculate_time | integer(int64) | Calculation time |
| risk_unit | array | Risk unit array |
| risk_unit[].symbol | string | Risk unit name |
| risk_unit[].spot_in_use | string | Spot hedge usage |
| risk_unit[].maintain_margin | string | Maintenance margin |
| risk_unit[].initial_margin | string | Initial margin |
| risk_unit[].margin_result | array | Margin result |
| risk_unit[].margin_result[].type | string | Position combination type: `original_position`, `long_delta_original_position`, `short_delta_original_position` |
| risk_unit[].margin_result[].profit_loss_ranges | array | Results of 33 stress scenarios for MR1 |
| risk_unit[].margin_result[].profit_loss_ranges[].price_percentage | string | Percentage change in price |
| risk_unit[].margin_result[].profit_loss_ranges[].implied_volatility_percentage | string | Percentage change in implied volatility |
| risk_unit[].margin_result[].profit_loss_ranges[].profit_loss | string | PnL |
| risk_unit[].margin_result[].max_loss | object | Maximum loss scenario |
| risk_unit[].margin_result[].max_loss.price_percentage | string | Percentage change in price |
| risk_unit[].margin_result[].max_loss.implied_volatility_percentage | string | Percentage change in implied volatility |
| risk_unit[].margin_result[].max_loss.profit_loss | string | PnL |
| risk_unit[].margin_result[].mr1 | string | Stress testing value |
| risk_unit[].margin_result[].mr2 | string | Basis spread risk |
| risk_unit[].margin_result[].mr3 | string | Volatility spread risk |
| risk_unit[].margin_result[].mr4 | string | Option short risk |
| risk_unit[].delta | string | Total Delta of risk unit |
| risk_unit[].gamma | string | Total Gamma of risk unit |
| risk_unit[].theta | string | Total Theta of risk unit |
| risk_unit[].vega | string | Total Vega of risk unit |

- **Request Example**:
```json
{
  "spot_balances": [
    { "currency": "BTC", "equity": "-1" }
  ],
  "spot_orders": [
    { "currency_pairs": "BTC_USDT", "order_price": "344", "size": "100", "left": "100", "type": "sell" }
  ],
  "futures_positions": [
    { "contract": "BTC_USDT", "size": "100" }
  ],
  "futures_orders": [
    { "contract": "BTC_USDT", "size": "10", "left": "8" }
  ],
  "options_positions": [
    { "options_name": "BTC_USDT-20240329-32000-C", "size": "10" }
  ],
  "options_orders": [
    { "options_name": "BTC_USDT-20240329-32000-C", "size": "100", "left": "80" }
  ],
  "spot_hedge": false
}
```

- **Response Example**:
```json
{
  "maintain_margin_total": "0.000000000000",
  "initial_margin_total": "0.000000000000",
  "calculate_time": "1709014486",
  "risk_unit": [
    {
      "symbol": "BTC",
      "margin_result": [
        {
          "type": "original_position",
          "profit_loss_ranges": [
            {
              "price_percentage": "-0.200000000000",
              "implied_volatility_percentage": "-0.300000000000",
              "profit_loss": "0.000000000000"
            }
          ],
          "max_loss": {
            "price_percentage": "-0.200000000000",
            "implied_volatility_percentage": "-0.300000000000",
            "profit_loss": "0.000000000000"
          },
          "mr1": "0.000000000000",
          "mr2": "0.000000000000",
          "mr3": "0.000000000000",
          "mr4": "0.000000000000"
        }
      ],
      "maintain_margin": "0.000000000000",
      "initial_margin": "0.000000000000"
    }
  ]
}
```

---

## Endpoint Summary

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| `GET` | `/unified/accounts` | Get unified account information | Yes |
| `GET` | `/unified/unified_mode` | Query account mode | Yes |
| `PUT` | `/unified/unified_mode` | Set account mode | Yes |
| `POST` | `/unified/loans` | Borrow or repay | Yes |
| `GET` | `/unified/loans` | Query current loans | Yes |
| `GET` | `/unified/loan_records` | Query loan records (borrow/repay history) | Yes |
| `GET` | `/unified/interest_records` | Query interest deduction records | Yes |
| `GET` | `/unified/borrowable` | Query max borrowable amount (single currency) | Yes |
| `GET` | `/unified/batch_borrowable` | Batch query max borrowable (up to 10 currencies) | Yes |
| `GET` | `/unified/transferable` | Query max transferable amount (single currency) | Yes |
| `GET` | `/unified/transferables` | Batch query max transferable (up to 100 currencies) | Yes |
| `GET` | `/unified/estimate_rate` | Query estimated interest rate | Yes |
| `GET` | `/unified/history_loan_rate` | Get historical lending rates | No |
| `GET` | `/unified/currencies` | List loan currencies supported | No |
| `GET` | `/unified/currency_discount_tiers` | Query currency discount tiers | No |
| `GET` | `/unified/loan_margin_tiers` | Query loan margin tiers | No |
| `GET` | `/unified/leverage/user_currency_config` | Get max/min leverage config | Yes |
| `GET` | `/unified/leverage/user_currency_setting` | Get user currency leverage | Yes |
| `POST` | `/unified/leverage/user_currency_setting` | Set loan currency leverage | Yes |
| `POST` | `/unified/collateral_currencies` | Set collateral currency | Yes |
| `GET` | `/unified/risk_units` | Get risk unit details (portfolio margin only) | Yes |
| `POST` | `/unified/portfolio_calculator` | Portfolio margin calculator | No |
