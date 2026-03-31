# Gate.io Isolated Margin API Documentation

Source: https://www.gate.com/docs/developers/apiv4/en/#isolated-margin

---

# Isolated-Margin

Isolated Margin

## Margin account list

`GET /margin/accounts`

_Margin account list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Margin account information for a trading pair. `base` corresponds to base currency account information, `quote` corresponds to quote currency account information\] |
| » _None_ | object | Margin account information for a trading pair. `base` corresponds to base currency account information, `quote` corresponds to quote currency account information |
| »» currency\_pair | string | Currency pair |
| »» account\_type | string | Account Type mmr: maintenance margin rate account;inactive: market not activated |
| »» leverage | string | User's current market leverage multiplier |
| »» locked | boolean | Whether the account is locked |
| »» risk | string | Deprecated |
| »» mmr | string | Current Maintenance Margin Rate of the account |
| »» base | object | Currency account information |
| »»» currency | string | Currency name |
| »»» available | string | Amount available for margin trading, available = margin + borrowed |
| »»» locked | string | Frozen funds, such as amounts already placed in margin market for order trading |
| »»» borrowed | string | Borrowed funds |
| »»» interest | string | Unpaid interest |
| »» quote | object | Currency account information |
| »»» currency | string | Currency name |
| »»» available | string | Amount available for margin trading, available = margin + borrowed |
| »»» locked | string | Frozen funds, such as amounts already placed in margin market for order trading |
| »»» borrowed | string | Borrowed funds |
| »»» interest | string | Unpaid interest |

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

url = '/margin/accounts'
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
url="/margin/accounts"
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

## Query margin account balance change history

`GET /margin/account_book`

_Query margin account balance change history_

Currently only provides transfer history to and from margin accounts. Query time range cannot exceed 30 days

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | query | string | false | Query history for specified currency. If `currency` is specified, `currency_pair` must also be specified. |
| currency\_pair | query | string | false | Specify margin account currency pair. Used in combination with `currency`. Ignored if `currency` is not specified |
| type | query | string | false | Query by specified account change type. If not specified, all change types will be included. |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Balance change record ID |
| » time | string | Account change timestamp |
| » time\_ms | integer(int64) | The timestamp of the change (in milliseconds) |
| » currency | string | Currency changed |
| » currency\_pair | string | Account trading pair |
| » change | string | Amount changed. Positive value means transferring in, while negative out |
| » balance | string | Balance after change |
| » type | string | Account book type. Please refer to [account book type](https://www.gate.com/docs/developers/apiv4/en/#accountbook-type) for more detail |

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

url = '/margin/account_book'
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
url="/margin/account_book"
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
    "id": "123456",
    "time": "1547633726",
    "time_ms": 1547633726123,
    "currency": "BTC",
    "currency_pair": "BTC_USDT",
    "change": "1.03",
    "balance": "4.59316525194"
}
]
```

## Funding account list

`GET /margin/funding_accounts`

_Funding account list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | query | string | false | Query by specified currency name |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » currency | string | Currency name |
| » available | string | Available assets to lend, which is identical to spot account `available` |
| » locked | string | Locked amount. i.e. amount in `open` loans |
| » lent | string | Outstanding loan amount yet to be repaid |
| » total\_lent | string | Amount used for lending. total\_lent = lent + locked |

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

url = '/margin/funding_accounts'
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
url="/margin/funding_accounts"
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
    "currency": "BTC",
    "available": "1.238",
    "locked": "0",
    "lent": "3.32",
    "total_lent": "3.32"
}
]
```

## Update user auto repayment settings

`POST /margin/auto_repay`

_Update user auto repayment settings_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| status | query | string | true | Whether to enable auto repayment: `on` \- enabled, `off` \- disabled |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | User's current auto repayment settings | Inline |

### Response Schema

Status Code **200**

_AutoRepaySetting_

| Name | Type | Description |
| --- | --- | --- |
| » status | string | Auto repayment status: `on` \- enabled, `off` \- disabled |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | on |
| status | off |

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

url = '/margin/auto_repay'
query_param = 'status=on'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('POST', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('POST', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="POST"
url="/margin/auto_repay"
query_param="status=on"
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
{
"status": "on"
}
```

## Query user auto repayment settings

`GET /margin/auto_repay`

_Query user auto repayment settings_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | User's current auto repayment settings | Inline |

### Response Schema

Status Code **200**

_AutoRepaySetting_

| Name | Type | Description |
| --- | --- | --- |
| » status | string | Auto repayment status: `on` \- enabled, `off` \- disabled |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | on |
| status | off |

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

url = '/margin/auto_repay'
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
url="/margin/auto_repay"
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
"status": "on"
}
```

## Get maximum transferable amount for isolated margin

`GET /margin/transferable`

_Get maximum transferable amount for isolated margin_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | query | string | true | Query by specified currency name |
| currency\_pair | query | string | false | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | [MarginTransferable](https://www.gate.com/docs/developers/apiv4/en/#schemamargintransferable) |

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

url = '/margin/transferable'
query_param = 'currency=BTC'
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
url="/margin/transferable"
query_param="currency=BTC"
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
{
"currency": "ETH",
"currency_pair": "ETH_USDT",
"amount": "10000"
}
```

## List lending markets

`GET /margin/uni/currency_pairs`

_List lending markets_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Currency pair of the loan\] |
| » _None_ | object | Currency pair of the loan |
| »» currency\_pair | string | Currency pair |
| »» base\_min\_borrow\_amount | string | Minimum borrow amount of base currency |
| »» quote\_min\_borrow\_amount | string | Minimum borrow amount of quote currency |
| »» leverage | string | Position leverage |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/margin/uni/currency_pairs'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/margin/uni/currency_pairs 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

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

## Get lending market details

`GET /margin/uni/currency_pairs/{currency_pair}`

_Get lending market details_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | path | string | true | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_Currency pair of the loan_

| Name | Type | Description |
| --- | --- | --- |
| » currency\_pair | string | Currency pair |
| » base\_min\_borrow\_amount | string | Minimum borrow amount of base currency |
| » quote\_min\_borrow\_amount | string | Minimum borrow amount of quote currency |
| » leverage | string | Position leverage |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/margin/uni/currency_pairs/AE_USDT'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/margin/uni/currency_pairs/AE_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"currency_pair": "AE_USDT",
"base_min_borrow_amount": "100",
"quote_min_borrow_amount": "100",
"leverage": "3"
}
```

## Estimate interest rate for isolated margin currencies

`GET /margin/uni/estimate_rate`

_Estimate interest rate for isolated margin currencies_

Interest rates change hourly based on lending depth, so completely accurate rates cannot be provided.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currencies | query | array\[string\] | true | Array of currency names to query, maximum 10 |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_Estimate current hourly lending rates, returned by currency_

| Name | Type | Description |
| --- | --- | --- |
| » **additionalProperties** | string | none |

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

url = '/margin/uni/estimate_rate'
query_param = 'currencies=BTC,GT'
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
url="/margin/uni/estimate_rate"
query_param="currencies=BTC,GT"
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
{
"BTC": "0.000002",
"GT": "0.000001"
}
```

## Borrow or repay

`POST /margin/uni/loans`

_Borrow or repay_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » currency | body | string | true | Currency |
| » type | body | string | true | Loan Type margin: margin borrowing |
| » amount | body | string | true | Borrow or repayment amount |
| » repaid\_all | body | boolean | false | Full repayment. For repayment operations only. When `true`, overrides `amount` and repays the full amount |
| » currency\_pair | body | string | true | Currency pair |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » type | borrow |
| » type | repay |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 204 | [No Content(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.5) | Operation successful | None |

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

url = '/margin/uni/loans'
query_param = ''
body='{"currency":"BTC","amount":"0.1","type":"borrow","currency_pair":"BTC_USDT","repaid_all":false}'
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
url="/margin/uni/loans"
query_param=""
body_param='{"currency":"BTC","amount":"0.1","type":"borrow","currency_pair":"BTC_USDT","repaid_all":false}'
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
"currency": "BTC",
"amount": "0.1",
"type": "borrow",
"currency_pair": "BTC_USDT",
"repaid_all": false
}
```

## Query loans

`GET /margin/uni/loans`

_Query loans_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |
| currency | query | string | false | Query by specified currency name |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Borrowing\] |
| » _None_ | object | Borrowing |
| »» currency | string | Currency |
| »» currency\_pair | string | Currency pair |
| »» amount | string | Amount to Repay |
| »» type | string | Loan type: platform borrowing - platform, margin borrowing - margin |
| »» create\_time | integer(int64) | Created time |
| »» update\_time | integer(int64) | Last Update Time |

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

url = '/margin/uni/loans'
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
url="/margin/uni/loans"
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
    "currency": "USDT",
    "currency_pari": "GT_USDT",
    "amount": "1",
    "type": "margin",
    "change_time": 1673247054000,
    "create_time": 1673247054000
}
]
```

## Query loan records

`GET /margin/uni/loan_records`

_Query loan records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| type | query | string | false | Type: `borrow` \- borrow, `repay` \- repay |
| currency | query | string | false | Query by specified currency name |
| currency\_pair | query | string | false | Currency pair |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| type | borrow |
| type | repay |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Borrowing Records |
| »» type | string | Type: `borrow` \- borrow, `repay` \- repay |
| »» currency\_pair | string | Currency pair |
| »» currency | string | Currency |
| »» amount | string | Borrow or repayment amount |
| »» create\_time | integer(int64) | Created time |

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

url = '/margin/uni/loan_records'
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
url="/margin/uni/loan_records"
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
    "type": "borrow",
    "currency_pair": "AE_USDT",
    "currency": "USDT",
    "amount": "1000",
    "create_time": 1673247054000
}
]
```

## Query interest deduction records

`GET /margin/uni/interest_records`

_Query interest deduction records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |
| currency | query | string | false | Query by specified currency name |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Interest Deduction Record\] |
| » _None_ | object | Interest Deduction Record |
| »» currency | string | Currency name |
| »» currency\_pair | string | Currency pair |
| »» actual\_rate | string | Actual Rate |
| »» interest | string | Interest |
| »» status | integer | Status: 0 - fail, 1 - success |
| »» type | string | Loan Type margin: margin borrowing |
| »» create\_time | integer(int64) | Created time |

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

url = '/margin/uni/interest_records'
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
url="/margin/uni/interest_records"
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

## Query maximum borrowable amount by currency

`GET /margin/uni/borrowable`

_Query maximum borrowable amount by currency_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | query | string | true | Query by specified currency name |
| currency\_pair | query | string | true | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | [MaxUniBorrowable](https://www.gate.com/docs/developers/apiv4/en/#schemamaxuniborrowable) |

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

url = '/margin/uni/borrowable'
query_param = 'currency=BTC&currency_pair=BTC_USDT'
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
url="/margin/uni/borrowable"
query_param="currency=BTC&currency_pair=BTC_USDT"
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
{
"currency": "AE",
"borrowable": "1123.344",
"currency_pair": "AE_USDT"
}
```

## Query user's own leverage lending tiers in current market

`GET /margin/user/loan_margin_tiers`

_Query user's own leverage lending tiers in current market_

Query the borrowing tier margin requirements of a specific spot market.For more details about borrowing tier margin requirements, please refer to Underlying Logic of the New Isolated Margin System(https://www.gate.com/en/help/trade/margin-trading/42357)

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Market gradient information\] |
| » _None_ | object | Market gradient information |
| »» upper\_limit | string | Maximum borrowing limit. Determined by the leverage you set; the lower the leverage, the larger the borrowing limit. |
| »» mmr | string | Maintenance margin rate.Under tiered margin requirements(https://www.gate.com/en/help/trade/margin-trading/42357), the maintenance margin rate is a composite value. |
| »» leverage | string | the maximum permissible leverage given to the current debt level; the higher the debt level, the lower the maximum leverage. |

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

url = '/margin/user/loan_margin_tiers'
query_param = 'currency_pair=BTC_USDT'
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
url="/margin/user/loan_margin_tiers"
query_param="currency_pair=BTC_USDT"
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
    "tier_amount": "100",
    "mmr": "0.9",
    "leverage": "1"
}
]
```

## Query current market leverage lending tiers

`GET /margin/loan_margin_tiers`

_Query current market leverage lending tiers_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Market gradient information\] |
| » _None_ | object | Market gradient information |
| »» upper\_limit | string | Maximum borrowing limit. Determined by the leverage you set; the lower the leverage, the larger the borrowing limit. |
| »» mmr | string | Maintenance margin rate.Under tiered margin requirements(https://www.gate.com/en/help/trade/margin-trading/42357), the maintenance margin rate is a composite value. |
| »» leverage | string | the maximum permissible leverage given to the current debt level; the higher the debt level, the lower the maximum leverage. |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/margin/loan_margin_tiers'
query_param = 'currency_pair=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/margin/loan_margin_tiers?currency_pair=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "tier_amount": "100",
    "mmr": "0.9",
    "leverage": "1"
}
]
```

## Set user market leverage multiplier

`POST /margin/leverage/user_market_setting`

_Set user market leverage multiplier_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » currency\_pair | body | string | false | Market |
| » leverage | body | string | true | Position leverage |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 204 | [No Content(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.5) | Set successfully | None |

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

url = '/margin/leverage/user_market_setting'
query_param = ''
body='{"currency_pair":"BTC_USDT","leverage":"10"}'
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
url="/margin/leverage/user_market_setting"
query_param=""
body_param='{"currency_pair":"BTC_USDT","leverage":"10"}'
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
"currency_pair": "BTC_USDT",
"leverage": "10"
}
```

## Query user's isolated margin account list

`GET /margin/user/account`

_Query user's isolated margin account list_

Supports querying risk ratio isolated accounts and margin ratio isolated accounts

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Margin account information for a trading pair. `base` corresponds to base currency account information, `quote` corresponds to quote currency account information\] |
| » _None_ | object | Margin account information for a trading pair. `base` corresponds to base currency account information, `quote` corresponds to quote currency account information |
| »» currency\_pair | string | Currency pair |
| »» account\_type | string | Account Type mmr: maintenance margin rate account;inactive: market not activated |
| »» leverage | string | User's current market leverage multiplier |
| »» locked | boolean | Whether the account is locked |
| »» risk | string | Deprecated |
| »» mmr | string | Current Maintenance Margin Rate of the account |
| »» base | object | Currency account information |
| »»» currency | string | Currency name |
| »»» available | string | Amount available for margin trading, available = margin + borrowed |
| »»» locked | string | Frozen funds, such as amounts already placed in margin market for order trading |
| »»» borrowed | string | Borrowed funds |
| »»» interest | string | Unpaid interest |
| »» quote | object | Currency account information |
| »»» currency | string | Currency name |
| »»» available | string | Amount available for margin trading, available = margin + borrowed |
| »»» locked | string | Frozen funds, such as amounts already placed in margin market for order trading |
| »»» borrowed | string | Borrowed funds |
| »»» interest | string | Unpaid interest |

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

url = '/margin/user/account'
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
url="/margin/user/account"
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