# Gate.io Multi-Collateral Loan API Documentation

Source: https://www.gate.com/docs/developers/apiv4/en/#multi-collateral-loan

---

# Multi-collateral-loan

Multi-currency collateral

## Place multi-currency collateral order

`POST /loan/multi_collateral/orders`

_Place multi-currency collateral order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » order\_id | body | string | false | Order ID |
| » order\_type | body | string | false | current - current rate, fixed - fixed rate, defaults to current if not specified |
| » fixed\_type | body | string | false | Fixed interest rate lending period: 7d - 7 days, 30d - 30 days. Required for fixed rate |
| » fixed\_rate | body | string | false | Fixed interest rate, required for fixed rate |
| » auto\_renew | body | boolean | false | Fixed interest rate, auto-renewal |
| » auto\_repay | body | boolean | false | Fixed interest rate, auto-repayment |
| » borrow\_currency | body | string | true | Borrowed currency |
| » borrow\_amount | body | string | true | Borrowed amount |
| » collateral\_currencies | body | array | false | Collateral currency and amount |
| »» CollateralCurrency | body | object | false | none |
| »»» currency | body | string | false | Currency |
| »»» amount | body | string | false | Size |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order placed successfully | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » order\_id | integer(int64) | Order ID |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/orders'
query_param = ''
body='{"order_id":1721387470,"order_type":"fixed","fixed_type":"7d","fixed_rate":0.00001,"auto_renew":true,"auto_repay":true,"borrow_currency":"BTC","borrow_amount":"1","collateral_currencies":[{"currency":"USDT","amount":"1000"}]}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('POST', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('POST', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="POST"
url="/loan/multi_collateral/orders"
query_param=""
body_param='{"order_id":1721387470,"order_type":"fixed","fixed_type":"7d","fixed_rate":0.00001,"auto_renew":true,"auto_repay":true,"borrow_currency":"BTC","borrow_amount":"1","collateral_currencies":[{"currency":"USDT","amount":"1000"}]}'
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url -d "$body_param" -H "Content-Type: application/json" 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Body parameter

```json
{
"order_id": 1721387470,
"order_type": "fixed",
"fixed_type": "7d",
"fixed_rate": 0.00001,
"auto_renew": true,
"auto_repay": true,
"borrow_currency": "BTC",
"borrow_amount": "1",
"collateral_currencies": [
    {
      "currency": "USDT",
      "amount": "1000"
    }
]
}
```

> Example responses

> 200 Response

```json
{
"order_id": 10005578
}
```

## Query multi-currency collateral order list

`GET /loan/multi_collateral/orders`

_Query multi-currency collateral order list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| page | query | integer | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |
| sort | query | string | false | Sort type: time\_desc - Default descending by creation time, ltv\_asc - Ascending by LTV ratio, ltv\_desc - Descending by LTV ratio |
| order\_type | query | string | false | Order type: current - Query current orders, fixed - Query fixed orders, defaults to current orders if not specified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Multi-Collateral Order\] |
| » _None_ | object | Multi-Collateral Order |
| »» order\_id | string | Order ID |
| »» order\_type | string | current - current, fixed - fixed |
| »» fixed\_type | string | Fixed interest rate loan periods: 7d - 7 days, 30d - 30 days |
| »» fixed\_rate | string | Fixed interest rate |
| »» expire\_time | integer(int64) | Expiration time, timestamp, unit in seconds |
| »» auto\_renew | boolean | Fixed interest rate, auto-renewal |
| »» auto\_repay | boolean | Fixed interest rate, auto-repayment |
| »» current\_ltv | string | Current collateralization rate |
| »» status | string | Order status:<br>\- initial: Initial state after placing the order<br>\- collateral\_deducted: Collateral deduction successful<br>\- collateral\_returning: Loan failed - Collateral return pending<br>\- lent: Loan successful<br>\- repaying: Repayment in progress<br>\- liquidating: Liquidation in progress<br>\- finished: Order completed<br>\- closed\_liquidated: Liquidation and repayment completed |
| »» borrow\_time | integer(int64) | Borrowing time, timestamp in seconds |
| »» total\_left\_repay\_usdt | string | Total outstanding value converted to USDT |
| »» total\_left\_collateral\_usdt | string | Total collateral value converted to USDT |
| »» borrow\_currencies | array | Borrowing Currency List |
| »»» BorrowCurrencyInfo | object | none |
| »»»» currency | string | Currency |
| »»»» index\_price | string | Currency Index Price |
| »»»» left\_repay\_principal | string | Outstanding principal |
| »»»» left\_repay\_interest | string | Outstanding interest |
| »»»» left\_repay\_usdt | string | Remaining total outstanding value converted to USDT |
| »»» collateral\_currencies | array | Collateral Currency List |
| »»»» CollateralCurrencyInfo | object | none |
| »»»»» currency | string | Currency |
| »»»»» index\_price | string | Currency Index Price |
| »»»»» left\_collateral | string | Remaining collateral amount |
| »»»»» left\_collateral\_usdt | string | Remaining collateral value converted to USDT |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/orders'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('GET', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="GET"
url="/loan/multi_collateral/orders"
query_param=""
body_param=''
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Example responses

> 200 Response

```json
[
{
    "order_id": "10005578",
    "order_type": "fixed",
    "fixed_type": "7d",
    "fixed_rate": 0.00001,
    "expire_time": 1703820105,
    "auto_renew": true,
    "auto_repay": true,
    "current_ltv": "0.0001004349664281",
    "status": "lent",
    "borrow_time": 1702615021,
    "total_left_repay_usdt": "106.491212982",
    "total_left_collateral_usdt": "1060300.18",
    "borrow_currencies": [
      {
        "currency": "GT",
        "index_price": "10.6491",
        "left_repay_principal": "10",
        "left_repay_interest": "0.00002",
        "left_repay_usdt": "106.491212982"
      }
    ],
    "collateral_currencies": [
      {
        "currency": "BTC",
        "index_price": "112794.7",
        "left_collateral": "9.4",
        "left_collateral_usdt": "1060270.18"
      },
      {
        "currency": "USDT",
        "index_price": "1",
        "left_collateral": "30",
        "left_collateral_usdt": "30"
      }
    ]
}
]
```

## Query order details

`GET /loan/multi_collateral/orders/{order_id}`

_Query order details_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | Order ID returned when order is successfully created |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order details queried successfully | Inline |

### Response Schema

Status Code **200**

_Multi-Collateral Order_

| Name | Type | Description |
| --- | --- | --- |
| » order\_id | string | Order ID |
| » order\_type | string | current - current, fixed - fixed |
| » fixed\_type | string | Fixed interest rate loan periods: 7d - 7 days, 30d - 30 days |
| » fixed\_rate | string | Fixed interest rate |
| » expire\_time | integer(int64) | Expiration time, timestamp, unit in seconds |
| » auto\_renew | boolean | Fixed interest rate, auto-renewal |
| » auto\_repay | boolean | Fixed interest rate, auto-repayment |
| » current\_ltv | string | Current collateralization rate |
| » status | string | Order status:<br>\- initial: Initial state after placing the order<br>\- collateral\_deducted: Collateral deduction successful<br>\- collateral\_returning: Loan failed - Collateral return pending<br>\- lent: Loan successful<br>\- repaying: Repayment in progress<br>\- liquidating: Liquidation in progress<br>\- finished: Order completed<br>\- closed\_liquidated: Liquidation and repayment completed |
| » borrow\_time | integer(int64) | Borrowing time, timestamp in seconds |
| » total\_left\_repay\_usdt | string | Total outstanding value converted to USDT |
| » total\_left\_collateral\_usdt | string | Total collateral value converted to USDT |
| » borrow\_currencies | array | Borrowing Currency List |
| »» BorrowCurrencyInfo | object | none |
| »»» currency | string | Currency |
| »»» index\_price | string | Currency Index Price |
| »»» left\_repay\_principal | string | Outstanding principal |
| »»» left\_repay\_interest | string | Outstanding interest |
| »»» left\_repay\_usdt | string | Remaining total outstanding value converted to USDT |
| »» collateral\_currencies | array | Collateral Currency List |
| »»» CollateralCurrencyInfo | object | none |
| »»»» currency | string | Currency |
| »»»» index\_price | string | Currency Index Price |
| »»»» left\_collateral | string | Remaining collateral amount |
| »»»» left\_collateral\_usdt | string | Remaining collateral value converted to USDT |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/orders/12345'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('GET', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="GET"
url="/loan/multi_collateral/orders/12345"
query_param=""
body_param=''
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Example responses

> 200 Response

```json
{
"order_id": "10005578",
"order_type": "fixed",
"fixed_type": "7d",
"fixed_rate": 0.00001,
"expire_time": 1703820105,
"auto_renew": true,
"auto_repay": true,
"current_ltv": "0.0001004349664281",
"status": "lent",
"borrow_time": 1702615021,
"total_left_repay_usdt": "106.491212982",
"total_left_collateral_usdt": "1060300.18",
"borrow_currencies": [
    {
      "currency": "GT",
      "index_price": "10.6491",
      "left_repay_principal": "10",
      "left_repay_interest": "0.00002",
      "left_repay_usdt": "106.491212982"
    }
],
"collateral_currencies": [
    {
      "currency": "BTC",
      "index_price": "112794.7",
      "left_collateral": "9.4",
      "left_collateral_usdt": "1060270.18"
    },
    {
      "currency": "USDT",
      "index_price": "1",
      "left_collateral": "30",
      "left_collateral_usdt": "30"
    }
]
}
```

## Multi-currency collateral repayment

`POST /loan/multi_collateral/repay`

_Multi-currency collateral repayment_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » order\_id | body | integer(int64) | true | Order ID |
| » repay\_items | body | array | true | Repay Currency Item |
| »» MultiLoanRepayItem | body | object | false | none |
| »»» currency | body | string | false | Repayment currency |
| »»» amount | body | string | false | Size |
| »»» repaid\_all | body | boolean | true | Repayment method, set to true for full repayment, false for partial repayment |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Operation successful | Inline |

### Response Schema

Status Code **200**

_Multi-currency collateral repayment_

| Name | Type | Description |
| --- | --- | --- |
| » order\_id | integer(int64) | Order ID |
| » repaid\_currencies | array | Repay Currency List |
| »» RepayCurrencyRes | object | none |
| »»» succeeded | boolean | Whether the repayment was successful |
| »»» label | string | Error identifier for failed operations; empty when successful |
| »»» message | string | Error description for failed operations; empty when successful |
| »»» currency | string | Repayment currency |
| »»» repaid\_principal | string | Principal |
| »»» repaid\_interest | string | Principal |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/repay'
query_param = ''
body='{"order_id":10005578,"repay_items":[{"currency":"btc","amount":"1","repaid_all":false}]}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('POST', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('POST', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="POST"
url="/loan/multi_collateral/repay"
query_param=""
body_param='{"order_id":10005578,"repay_items":[{"currency":"btc","amount":"1","repaid_all":false}]}'
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url -d "$body_param" -H "Content-Type: application/json" 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Body parameter

```json
{
"order_id": 10005578,
"repay_items": [
    {
      "currency": "btc",
      "amount": "1",
      "repaid_all": false
    }
]
}
```

> Example responses

> 200 Response

```json
{
"order_id": 10005679,
"repaid_currencies": [
    {
      "succeeded": false,
      "label": "INVALID_PARAM_VALUE",
      "message": "Invalid parameter value",
      "currency": "BTC",
      "repaid_principal": "1",
      "repaid_interest": "0.0001"
    },
    {
      "succeeded": true,
      "currency": "BTC",
      "repaid_principal": "1",
      "repaid_interest": "0.0001"
    }
]
}
```

## Query multi-currency collateral repayment records

`GET /loan/multi_collateral/repay`

_Query multi-currency collateral repayment records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| type | query | string | true | Operation type: repay - Regular repayment, liquidate - Liquidation |
| borrow\_currency | query | string | false | Borrowed currency |
| page | query | integer | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Multi-Collateral Repayment Record |
| »» order\_id | integer(int64) | Order ID |
| »» record\_id | integer(int64) | Repayment record ID |
| »» init\_ltv | string | Initial collateralization rate |
| »» before\_ltv | string | Ltv before the operation |
| »» after\_ltv | string | Ltv after the operation |
| »» borrow\_time | integer(int64) | Borrowing time, timestamp in seconds |
| »» repay\_time | integer(int64) | Repayment time, timestamp in seconds |
| »» borrow\_currencies | array | List of borrowing information |
| »»» currency | string | Currency |
| »»» index\_price | string | Currency Index Price |
| »»» before\_amount | string | Amount before the operation |
| »»» before\_amount\_usdt | string | USDT Amount before the operation |
| »»» after\_amount | string | Amount after the operation |
| »»» after\_amount\_usdt | string | USDT Amount after the operation |
| »» collateral\_currencies | array | List of collateral information |
| »»» currency | string | Currency |
| »»» index\_price | string | Currency Index Price |
| »»» before\_amount | string | Amount before the operation |
| »»» before\_amount\_usdt | string | USDT Amount before the operation |
| »»» after\_amount | string | Amount after the operation |
| »»» after\_amount\_usdt | string | USDT Amount after the operation |
| »» repaid\_currencies | array | Repay Currency List |
| »»» RepayRecordRepaidCurrency | object | none |
| »»»» currency | string | Repayment currency |
| »»»» index\_price | string | Currency Index Price |
| »»»» repaid\_amount | string | Repayment amount |
| »»»» repaid\_principal | string | Principal |
| »»»» repaid\_interest | string | Interest |
| »»»» repaid\_amount\_usdt | string | Repayment amount converted to USDT |
| »»» total\_interest\_list | array | Total Interest List |
| »»»» RepayRecordTotalInterest | object | none |
| »»»»» currency | string | Currency |
| »»»»» index\_price | string | Currency Index Price |
| »»»»» amount | string | Interest Amount |
| »»»»» amount\_usdt | string | Interest amount converted to USDT |
| »»»» left\_repay\_interest\_list | array | List of remaining interest to be repaid |
| »»»»» RepayRecordLeftInterest | object | none |
| »»»»»» currency | string | Currency |
| »»»»»» index\_price | string | Currency Index Price |
| »»»»»» before\_amount | string | Interest amount before repayment |
| »»»»»» before\_amount\_usdt | string | Converted value of interest before repayment in USDT |
| »»»»»» after\_amount | string | Interest amount after repayment |
| »»»»»» after\_amount\_usdt | string | Converted value of interest after repayment in USDT |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/repay'
query_param = 'type=repay'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('GET', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="GET"
url="/loan/multi_collateral/repay"
query_param="type=repay"
body_param=''
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url?$query_param"
curl -X $method $full_url 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Example responses

> 200 Response

```json
[
{
    "order_id": 10005679,
    "record_id": 1348,
    "init_ltv": "0.2141",
    "before_ltv": "0.215",
    "after_ltv": "0.312",
    "borrow_time": 1702995889,
    "repay_time": 1703053927,
    "borrow_currencies": [
      {
        "currency": "BAT",
        "index_price": "103.02",
        "before_amount": "1",
        "before_amount_usdt": "103.02",
        "after_amount": "0.999017",
        "after_amount_usdt": "102.91873134"
      }
    ],
    "collateral_currencies": [
      {
        "currency": "ETC",
        "index_price": "0.6014228107",
        "before_amount": "1000",
        "before_amount_usdt": "601.4228107",
        "after_amount": "1000",
        "after_amount_usdt": "601.4228107"
      }
    ],
    "repaid_currencies": [
      {
        "currency": "BAT",
        "index_price": "103.02",
        "repaid_amount": "0.001",
        "repaid_principal": "0.000983",
        "repaid_interest": "0.000017",
        "repaid_amount_usdt": "0.10302"
      }
    ],
    "total_interest_list": [
      {
        "currency": "BAT",
        "index_price": "103.02",
        "amount": "0.000017",
        "amount_usdt": "0.00175134"
      }
    ],
    "left_repay_interest_list": [
      {
        "currency": "BAT",
        "index_price": "103.02",
        "before_amount": "0.000017",
        "before_amount_usdt": "0.00175134",
        "after_amount": "0",
        "after_amount_usdt": "0"
      }
    ]
}
]
```

## Add or withdraw collateral

`POST /loan/multi_collateral/mortgage`

_Add or withdraw collateral_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » order\_id | body | integer(int64) | true | Order ID |
| » type | body | string | true | Operation type: append - add collateral, redeem - withdraw collateral |
| » collaterals | body | array | false | Collateral currency list |
| »» currency | body | string | false | Currency |
| »» amount | body | string | false | Size |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Operation successful | Inline |

### Response Schema

Status Code **200**

_Multi-collateral adjustment result_

| Name | Type | Description |
| --- | --- | --- |
| » order\_id | integer(int64) | Order ID |
| » collateral\_currencies | array | Collateral currency information |
| »» CollateralCurrencyRes | object | none |
| »»» succeeded | boolean | Update success status |
| »»» label | string | Error identifier for failed operations; empty when successful |
| »»» message | string | Error description for failed operations; empty when successful |
| »»» currency | string | Currency |
| »»» amount | string | Successfully operated collateral quantity; 0 if operation fails |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/mortgage'
query_param = ''
body='{"order_id":10005578,"type":"append","collaterals":[{"currency":"btc","amount":"0.5"}]}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('POST', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('POST', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="POST"
url="/loan/multi_collateral/mortgage"
query_param=""
body_param='{"order_id":10005578,"type":"append","collaterals":[{"currency":"btc","amount":"0.5"}]}'
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url -d "$body_param" -H "Content-Type: application/json" 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Body parameter

```json
{
"order_id": 10005578,
"type": "append",
"collaterals": [
    {
      "currency": "btc",
      "amount": "0.5"
    }
]
}
```

> Example responses

> 200 Response

```json
{
"order_id": 10005679,
"collateral_currencies": [
    {
      "succeeded": true,
      "currency": "btc",
      "amount": "0.5"
    }
]
}
```

## Query collateral adjustment records

`GET /loan/multi_collateral/mortgage`

_Query collateral adjustment records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| page | query | integer | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |
| collateral\_currency | query | string | false | Collateral currency |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Multi-Collateral adjustment record |
| »» order\_id | integer(int64) | Order ID |
| »» record\_id | integer(int64) | Collateral record ID |
| »» before\_ltv | string | Collateral ratio before adjustment |
| »» after\_ltv | string | Collateral ratio before adjustment |
| »» operate\_time | integer(int64) | Operation time, timestamp in seconds |
| »» borrow\_currencies | array | Borrowing Currency List |
| »»» currency | string | Currency |
| »»» index\_price | string | Currency Index Price |
| »»» before\_amount | string | Amount before the operation |
| »»» before\_amount\_usdt | string | USDT Amount before the operation |
| »»» after\_amount | string | Amount after the operation |
| »»» after\_amount\_usdt | string | USDT Amount after the operation |
| »» collateral\_currencies | array | Collateral Currency List |
| »»» currency | string | Currency |
| »»» index\_price | string | Currency Index Price |
| »»» before\_amount | string | Amount before the operation |
| »»» before\_amount\_usdt | string | USDT Amount before the operation |
| »»» after\_amount | string | Amount after the operation |
| »»» after\_amount\_usdt | string | USDT Amount after the operation |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/mortgage'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('GET', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="GET"
url="/loan/multi_collateral/mortgage"
query_param=""
body_param=''
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url"
curl -X $method $full_url 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Example responses

> 200 Response

```json
[
{
    "order_id": 10000417,
    "record_id": 10000452,
    "before_ltv": "0.00039345555621480000",
    "after_ltv": "0.00019672777810740000",
    "operate_time": 1688461924,
    "borrow_currencies": [
      {
        "currency": "BTC",
        "index_price": "30000",
        "before_amount": "0.1",
        "before_amount_usdt": "1000",
        "after_amount": "0.6",
        "after_amount_usdt": "1006"
      }
    ],
    "collateral_currencies": [
      {
        "currency": "BTC",
        "index_price": "30000",
        "before_amount": "0.1",
        "before_amount_usdt": "1000",
        "after_amount": "0.6",
        "after_amount_usdt": "1006"
      }
    ]
}
]
```

## Query user's collateral and borrowing currency quota information

`GET /loan/multi_collateral/currency_quota`

_Query user's collateral and borrowing currency quota information_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| type | query | string | true | Currency type: collateral - Collateral currency, borrow - Borrowing currency |
| currency | query | string | true | When it is a collateral currency, multiple currencies can be passed separated by commas; when it is a borrowing currency, only one currency can be passed |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Currency Quota |
| »» currency | string | Currency |
| »» index\_price | string | Currency Index Price |
| »» min\_quota | string | Minimum borrowing/collateral limit for the currency |
| »» left\_quota | string | Remaining currency limit for `borrow/collateral` (when input parameter `type` is `borrow`, represents current currency) |
| »» left\_quote\_usdt | string | Remaining currency limit converted to USDT (when input parameter `type` is `borrow`, represents current currency) |
| »» left\_quota\_fixed | string | Remaining `borrow/collateral` limit for fixed-term currency |
| »» left\_quote\_usdt\_fixed | string | Remaining currency limit for fixed-term currency converted to USDT |

WARNING

To perform this operation, you must be authenticated by API key and secret

> Code samples

```python
# coding: utf-8
import requests
import time
import hashlib
import hmac

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/currency_quota'
query_param = 'type=collateral&currency=BTC'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('GET', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="GET"
url="/loan/multi_collateral/currency_quota"
query_param="type=collateral&currency=BTC"
body_param=''
timestamp=$(date +%s)
body_hash=$(printf "$body_param" | openssl sha512 | awk '{print $NF}')
sign_string="$method\n$prefix$url\n$query_param\n$body_hash\n$timestamp"
sign=$(printf "$sign_string" | openssl sha512 -hmac "$secret" | awk '{print $NF}')

full_url="$host$prefix$url?$query_param"
curl -X $method $full_url 
    -H "Timestamp: $timestamp" -H "KEY: $key" -H "SIGN: $sign"
```

> Example responses

> 200 Response

```json
[
{
    "currency": "BTC",
    "index_price": "35306.1",
    "min_quota": "0",
    "left_quota": "2768152.4958445218723677",
    "left_quote_usdt": "97732668833.536273678"
}
]
```

## Query borrow currencies and collateral currencies supported by multi-currency collateral

`GET /loan/multi_collateral/currencies`

_Query borrow currencies and collateral currencies supported by multi-currency collateral_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_Borrowing and collateral currencies supported for Multi-Collateral_

| Name | Type | Description |
| --- | --- | --- |
| » loan\_currencies | array | List of supported borrowing currencies |
| »» MultiLoanItem | object | none |
| »»» currency | string | Currency |
| »»» price | string | Latest price of the currency |
| »» collateral\_currencies | array | List of supported collateral currencies |
| »»» MultiCollateralItem | object | none |
| »»»» currency | string | Currency |
| »»»» index\_price | string | Currency Index Price |
| »»»» discount | string | Discount |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/currencies'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/loan/multi_collateral/currencies 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"loan_currencies": [
    {
      "currency": "BTC",
      "price": "1212"
    },
    {
      "currency": "GT",
      "price": "12"
    }
],
"collateral_currencies": [
    {
      "currency": "BTC",
      "index_price": "1212",
      "discount": "0.7"
    }
]
}
```

## Query collateralization ratio information

`GET /loan/multi_collateral/ltv`

_Query collateralization ratio information_

Multi-currency collateral ratio is fixed, independent of currency

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_Multi-collateral ratio_

| Name | Type | Description |
| --- | --- | --- |
| » init\_ltv | string | Initial collateralization rate |
| » alert\_ltv | string | Warning collateralization rate |
| » liquidate\_ltv | string | Liquidation collateralization rate |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/ltv'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/loan/multi_collateral/ltv 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"init_ltv": "0.7",
"alert_ltv": "0.8",
"liquidate_ltv": "0.9"
}
```

## Query currency's 7-day and 30-day fixed interest rates

`GET /loan/multi_collateral/fixed_rate`

_Query currency's 7-day and 30-day fixed interest rates_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Multi-collateral fixed interest rate |
| »» currency | string | Currency |
| »» rate\_7d | string | Fixed interest rate for 7-day lending period |
| »» rate\_30d | string | Fixed interest rate for 30-day lending period |
| »» update\_time | integer(int64) | Update time, timestamp in seconds |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/fixed_rate'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/loan/multi_collateral/fixed_rate 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "currency": "BTC",
    "rate_7d": "0.000023",
    "rate_30d": "0.1",
    "update_time": 1703820105
}
]
```

## Query currency's current interest rate

`GET /loan/multi_collateral/current_rate`

_Query currency's current interest rate_

Query the current interest rate of the currency in the previous hour, the current interest rate is updated every hour

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currencies | query | array\[string\] | true | Specify currency name query array, separated by commas, maximum 100 items |
| vip\_level | query | string | false | VIP level, defaults to 0 if not specified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Multi-collateral current interest rate |
| »» currency | string | Currency |
| »» current\_rate | string | Currency current interest rate |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/loan/multi_collateral/current_rate'
query_param = 'currencies=BTC,GT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/loan/multi_collateral/current_rate?currencies=BTC,GT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "currency": "BTC",
    "current_rate": "0.000023"
}
]
```