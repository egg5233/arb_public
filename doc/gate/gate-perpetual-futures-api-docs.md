# Gate.io Perpetual Futures API Documentation

Source: https://www.gate.com/docs/developers/apiv4/en/#futures

---

# Futures

Perpetual futures

## Query all futures contracts

`GET /futures/{settle}/contracts`

_Query all futures contracts_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [Contract](https://www.gate.com/docs/developers/apiv4/en/#schemacontract)\] |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/contracts'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/contracts 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "name": "BTC_USDT",
    "type": "direct",
    "quanto_multiplier": "0.0001",
    "ref_discount_rate": "0",
    "order_price_deviate": "0.5",
    "maintenance_rate": "0.005",
    "mark_type": "index",
    "last_price": "38026",
    "mark_price": "37985.6",
    "index_price": "37954.92",
    "funding_rate_indicative": "0.000219",
    "mark_price_round": "0.01",
    "funding_offset": 0,
    "in_delisting": false,
    "risk_limit_base": "1000000",
    "interest_rate": "0.0003",
    "order_price_round": "0.1",
    "order_size_min": "1",
    "enable_decimal": false,
    "ref_rebate_rate": "0.2",
    "funding_interval": 28800,
    "risk_limit_step": "1000000",
    "leverage_min": "1",
    "leverage_max": "100",
    "risk_limit_max": "8000000",
    "maker_fee_rate": "-0.00025",
    "taker_fee_rate": "0.00075",
    "funding_rate": "0.002053",
    "order_size_max": "1000000",
    "funding_next_apply": 1610035200,
    "short_users": 977,
    "config_change_time": 1609899548,
    "trade_size": "28530850594",
    "position_size": "5223816",
    "long_users": 455,
    "funding_impact_value": "60000",
    "orders_limit": 50,
    "trade_id": 10851092,
    "orderbook_id": 2129638396,
    "enable_bonus": true,
    "enable_credit": true,
    "create_time": 1669688556,
    "funding_cap_ratio": "0.75",
    "status": "trading",
    "launch_time": 1609899548,
    "delisting_time": 1609899548,
    "delisted_time": 1609899548,
    "market_order_slip_ratio": "0.05",
    "market_order_size_max": "0",
    "funding_rate_limit": "0.003"
}
]
```

## Query single contract information

`GET /futures/{settle}/contracts/{contract}`

_Query single contract information_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Contract information | [Contract](https://www.gate.com/docs/developers/apiv4/en/#schemacontract) |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/contracts/BTC_USDT'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/contracts/BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"name": "BTC_USDT",
"type": "direct",
"quanto_multiplier": "0.0001",
"ref_discount_rate": "0",
"order_price_deviate": "0.5",
"maintenance_rate": "0.005",
"mark_type": "index",
"last_price": "38026",
"mark_price": "37985.6",
"index_price": "37954.92",
"funding_rate_indicative": "0.000219",
"mark_price_round": "0.01",
"funding_offset": 0,
"in_delisting": false,
"risk_limit_base": "1000000",
"interest_rate": "0.0003",
"order_price_round": "0.1",
"order_size_min": "1",
"enable_decimal": false,
"ref_rebate_rate": "0.2",
"funding_interval": 28800,
"risk_limit_step": "1000000",
"leverage_min": "1",
"leverage_max": "100",
"risk_limit_max": "8000000",
"maker_fee_rate": "-0.00025",
"taker_fee_rate": "0.00075",
"funding_rate": "0.002053",
"order_size_max": "1000000",
"funding_next_apply": 1610035200,
"short_users": 977,
"config_change_time": 1609899548,
"trade_size": "28530850594",
"position_size": "5223816",
"long_users": 455,
"funding_impact_value": "60000",
"orders_limit": 50,
"trade_id": 10851092,
"orderbook_id": 2129638396,
"enable_bonus": true,
"enable_credit": true,
"create_time": 1669688556,
"funding_cap_ratio": "0.75",
"status": "trading",
"launch_time": 1609899548,
"delisting_time": 1609899548,
"delisted_time": 1609899548,
"market_order_slip_ratio": "0.05",
"market_order_size_max": "0",
"funding_rate_limit": "0.003"
}
```

## Query futures market depth information

`GET /futures/{settle}/order_book`

_Query futures market depth information_

Bids will be sorted by price from high to low, while asks sorted reversely

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| interval | query | string | false | Price precision for merged depth. 0 means no merging. If not specified, defaults to 0 |
| limit | query | integer | false | Number of depth levels |
| with\_id | query | boolean | false | Whether to return depth update ID. This ID increments by 1 each time the depth changes |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Depth query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | integer(int64) | Order Book ID. Increases by 1 on every order book change. Set `with_id=true` to include this field in response |
| » current | number(double) | Response data generation timestamp |
| » update | number(double) | Order book changed timestamp |
| » asks | array | Ask Depth |
| »» FuturesOrderBookItem | object | none |
| »»» p | string | Price (quote currency) |
| »»» s | string | Size |
| »» bids | array | Bid Depth |
| »»» FuturesOrderBookItem | object | none |
| »»»» p | string | Price (quote currency) |
| »»»» s | string | Size |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/order_book'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/order_book?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"id": 123456,
"current": 1623898993.123,
"update": 1623898993.121,
"asks": [
    {
      "p": "1.52",
      "s": "100"
    },
    {
      "p": "1.53",
      "s": "40"
    }
],
"bids": [
    {
      "p": "1.17",
      "s": "150"
    },
    {
      "p": "1.16",
      "s": "203"
    }
]
}
```

## Futures market transaction records

`GET /futures/{settle}/trades`

_Futures market transaction records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| last\_id | query | string | false | Specify the starting point for this list based on a previously retrieved id |
| from | query | integer(int64) | false | Specify starting time in Unix seconds. If not specified, `to` and `limit` will be used to limit response items. |
| to | query | integer(int64) | false | Specify end time in Unix seconds, default to current time. |

#### Detailed descriptions

**last\_id**: Specify the starting point for this list based on a previously retrieved id

This parameter is deprecated. Use `from` and `to` instead to limit time range

**from**: Specify starting time in Unix seconds. If not specified, `to` and `limit` will be used to limit response items.
If items between `from` and `to` are more than `limit`, only `limit` number will be returned.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | integer(int64) | Fill ID |
| » create\_time | number(double) | Fill Time |
| » create\_time\_ms | number(double) | Trade time, with millisecond precision to 3 decimal places |
| » contract | string | Futures contract |
| » size | string | Trading size |
| » price | string | Trade price (quote currency) |
| » is\_internal | boolean | Deprecated |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/trades'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/trades?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "id": 121234231,
    "create_time": 1514764800,
    "contract": "BTC_USDT",
    "size": "-100",
    "price": "100.123"
}
]
```

## Futures market K-line chart

`GET /futures/{settle}/candlesticks`

_Futures market K-line chart_

Return specified contract candlesticks.
If prefix `contract` with `mark_`, the contract's mark price candlesticks are returned;
if prefix with `index_`, index price candlesticks will be returned.

Maximum of 2000 points are returned in one query. Be sure not to exceed the limit when specifying `from`, `to` and `interval`

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| from | query | integer(int64) | false | Start time of candlesticks, formatted in Unix timestamp in seconds. Default to`to - 100 * interval` if not specified |
| to | query | integer(int64) | false | Specify the end time of the K-line chart, defaults to current time if not specified, note that the time format is Unix timestamp with second precision |
| limit | query | integer | false | Maximum number of recent data points to return. `limit` conflicts with `from` and `to`. If either `from` or `to` is specified, request will be rejected. |
| interval | query | string | false | Time interval for data points. Note: 1w represents a natural week, 7d is aligned with Unix epoch time, 30d represents a natural month |
| timezone | query | string | false | Time zone: all/utc0/utc8, default utc0 |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |
| interval | 10s |
| interval | 1m |
| interval | 5m |
| interval | 15m |
| interval | 30m |
| interval | 1h |
| interval | 4h |
| interval | 8h |
| interval | 1d |
| interval | 7d |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | data point in every timestamp |
| »» t | number(double) | Unix timestamp in seconds |
| »» v | string | size volume (contract size). Only returned if `contract` is not prefixed |
| »» c | string | Close price (quote currency) |
| »» h | string | Highest price (quote currency) |
| »» l | string | Lowest price (quote currency) |
| »» o | string | Open price (quote currency) |
| »» sum | string | Trading volume (unit: Quote currency) |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/candlesticks'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/candlesticks?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "t": 1539852480,
    "v": "97151",
    "c": "1.032",
    "h": "1.032",
    "l": "1.032",
    "o": "1.032",
    "sum": "3580"
}
]
```

## Premium Index K-line chart

`GET /futures/{settle}/premium_index`

_Premium Index K-line chart_

K-line chart data returns a maximum of 1000 points per request. When specifying from, to, and interval, ensure the number of points is not excessive

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| from | query | integer(int64) | false | Start time of candlesticks, formatted in Unix timestamp in seconds. Default to`to - 100 * interval` if not specified |
| to | query | integer(int64) | false | Specify the end time of the K-line chart, defaults to current time if not specified, note that the time format is Unix timestamp with second precision |
| limit | query | integer | false | Maximum number of recent data points to return. `limit` conflicts with `from` and `to`. If either `from` or `to` is specified, request will be rejected. |
| interval | query | string | false | Time interval between data points |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |
| interval | 1m |
| interval | 5m |
| interval | 15m |
| interval | 30m |
| interval | 1h |
| interval | 4h |
| interval | 8h |
| interval | 1d |
| interval | 7d |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | data point in every timestamp |
| »» t | number(double) | Unix timestamp in seconds |
| »» c | string | Close price |
| »» h | string | Highest price |
| »» l | string | Lowest price |
| »» o | string | Open price |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/premium_index'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/premium_index?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "t": 1539852480,
    "c": "0",
    "h": "0.00023",
    "l": "0",
    "o": "0"
}
]
```

## Get all futures trading statistics

`GET /futures/{settle}/tickers`

_Get all futures trading statistics_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » contract | string | Futures contract |
| » last | string | Last trading price |
| » change\_percentage | string | Price change percentage. Negative values indicate price decrease, e.g. -7.45 |
| » total\_size | string | Contract total size |
| » low\_24h | string | 24-hour lowest price |
| » high\_24h | string | 24-hour highest price |
| » volume\_24h | string | 24-hour trading volume |
| » volume\_24h\_btc | string | 24-hour trading volume in BTC (deprecated, use `volume_24h_base`, `volume_24h_quote`, `volume_24h_settle` instead) |
| » volume\_24h\_usd | string | 24-hour trading volume in USD (deprecated, use `volume_24h_base`, `volume_24h_quote`, `volume_24h_settle` instead) |
| » volume\_24h\_base | string | 24-hour trading volume in base currency |
| » volume\_24h\_quote | string | 24-hour trading volume in quote currency |
| » volume\_24h\_settle | string | 24-hour trading volume in settle currency |
| » mark\_price | string | Recent mark price |
| » funding\_rate | string | Funding rate |
| » funding\_rate\_indicative | string | Indicative Funding rate in next period. (deprecated. use `funding_rate`) |
| » index\_price | string | Index price |
| » quanto\_base\_rate | string | Deprecated |
| » lowest\_ask | string | Recent lowest ask |
| » lowest\_size | string | The latest seller's lowest price order quantity |
| » highest\_bid | string | Recent highest bid |
| » highest\_size | string | The latest buyer's highest price order volume |
| » change\_utc0 | string | Percentage change at utc0. Negative values indicate a drop, e.g., -7.45% |
| » change\_utc8 | string | Percentage change at utc8. Negative values indicate a drop, e.g., -7.45% |
| » change\_price | string | 24h change amount. Negative values indicate a drop, e.g., -7.45 |
| » change\_utc0\_price | string | Change amount at utc0. Negative values indicate a drop, e.g., -7.45 |
| » change\_utc8\_price | string | Change amount at utc8. Negative values indicate a drop, e.g., -7.45 |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/tickers'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/tickers 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "contract": "BTC_USDT",
    "last": "6432",
    "low_24h": "6278",
    "high_24h": "6790",
    "change_percentage": "4.43",
    "total_size": "32323904",
    "volume_24h": "184040233284",
    "volume_24h_btc": "28613220",
    "volume_24h_usd": "184040233284",
    "volume_24h_base": "28613220",
    "volume_24h_quote": "184040233284",
    "volume_24h_settle": "28613220",
    "mark_price": "6534",
    "funding_rate": "0.0001",
    "funding_rate_indicative": "0.0001",
    "index_price": "6531",
    "highest_bid": "34089.7",
    "highest_size": "100",
    "lowest_ask": "34217.9",
    "lowest_size": "1000"
}
]
```

## Futures market historical funding rate

`GET /futures/{settle}/funding_rate`

_Futures market historical funding rate_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| limit | query | integer | false | Maximum number of records returned in a single list |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | History query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » t | integer(int64) | Unix timestamp in seconds |
| » r | string | Funding rate |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/funding_rate'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/funding_rate?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "t": 1543968000,
    "r": "0.000157"
}
]
```

## Batch Query Historical Funding Rate Data for Perpetual Contracts

`POST /futures/{settle}/funding_rates`

_Batch Query Historical Funding Rate Data for Perpetual Contracts_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| body | body | [BatchFundingRatesRequest](https://www.gate.com/docs/developers/apiv4/en/#schemabatchfundingratesrequest) | true | none |
| » contracts | body | array | true | Array of Contract Names |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch Query Successful | \[ [BatchFundingRatesResponse](https://www.gate.com/docs/developers/apiv4/en/#schemabatchfundingratesresponse)\] |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/funding_rates'
query_param = ''
body='{"contracts":["BTC_USDT","ETH_USDT"]}'
r = requests.request('POST', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell

curl -X POST https://api.gateio.ws/api/v4/futures/usdt/funding_rates 
  -H 'Content-Type: application/json' 
  -H 'Accept: application/json'
```

> Body parameter

```json
{
"contracts": [
    "BTC_USDT",
    "ETH_USDT"
]
}
```

> Example responses

> 200 Response

```json
[
[
    {
      "contract": "BTC_USDT",
      "data": [
        {
          "t": 1543968000,
          "r": "0.000157"
        },
        {
          "t": 1544054400,
          "r": "0.000145"
        }
      ]
    }
]
]
```

## Futures market insurance fund history

`GET /futures/{settle}/insurance`

_Futures market insurance fund history_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| limit | query | integer | false | Maximum number of records returned in a single list |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | none |
| » t | integer(int64) | Unix timestamp in seconds |
| » b | string | Insurance balance |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/insurance'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/insurance 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "t": 1543968000,
    "b": "83.0031"
}
]
```

## Futures statistics

`GET /futures/{settle}/contract_stats`

_Futures statistics_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| from | query | integer(int64) | false | Start timestamp |
| interval | query | string | false | none |
| limit | query | integer | false | none |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | integer(int64) | Stat timestamp |
| » lsr\_taker | number(double) | Long/short taker ratio |
| » lsr\_account | number(double) | Long/short position user ratio |
| » long\_liq\_size | string | Long liquidation size (contracts) |
| » long\_liq\_amount | number(double) | Long liquidation amount (base currency) |
| » long\_liq\_usd | number(double) | Long liquidation volume (quote currency) |
| » short\_liq\_size | string | Short liquidation size (contracts) |
| » short\_liq\_amount | number(double) | Short liquidation amount (base currency) |
| » short\_liq\_usd | number(double) | Short liquidation volume (quote currency) |
| » open\_interest | string | Total open interest size (contracts) |
| » open\_interest\_usd | number(double) | Total open interest volume (quote currency) |
| » top\_lsr\_account | number(double) | Top trader long/short account ratio |
| » top\_lsr\_size | string | Top trader long/short position ratio |
| » mark\_price | number(double) | Mark price |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/contract_stats'
query_param = 'contract=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/contract_stats?contract=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "time": 1603865400,
    "lsr_taker": 100,
    "lsr_account": 0.5,
    "long_liq_size": "0",
    "short_liq_size": "0",
    "open_interest": "124724",
    "short_liq_usd": 0,
    "mark_price": "8865",
    "top_lsr_size": "1.02",
    "short_liq_amount": 0,
    "long_liq_amount": 0,
    "open_interest_usd": 1511,
    "top_lsr_account": 1.5,
    "long_liq_usd": 0
}
]
```

## Query index constituents

`GET /futures/{settle}/index_constituents/{index}`

_Query index constituents_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| index | path | string | true | Index name |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » index | string | Index name |
| » constituents | array | Constituents |
| »» IndexConstituent | object | none |
| »»» exchange | string | Exchange |
| »»» symbols | array | Symbol list |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/index_constituents/BTC_USDT'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/index_constituents/BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"index": "BTC_USDT",
"constituents": [
    {
      "exchange": "Binance",
      "symbols": [
        "BTC_USDT"
      ]
    },
    {
      "exchange": "Gate.com",
      "symbols": [
        "BTC_USDT"
      ]
    },
    {
      "exchange": "Huobi",
      "symbols": [
        "BTC_USDT"
      ]
    }
]
}
```

## Query liquidation order history

`GET /futures/{settle}/liq_orders`

_Query liquidation order history_

The time interval between from and to is maximum 3600. Some private fields are not returned by public interfaces, refer to field descriptions for details

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| limit | query | integer | false | Maximum number of records returned in a single list |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | integer(int64) | Liquidation time |
| » contract | string | Futures contract |
| » size | string | User position size |
| » order\_size | string | Number of forced liquidation orders |
| » order\_price | string | Liquidation order price |
| » fill\_price | string | Liquidation order average taker price |
| » left | string | System liquidation order maker size |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/liq_orders'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/liq_orders 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "time": 1548654951,
    "contract": "BTC_USDT",
    "size": "600",
    "order_size": "-600",
    "order_price": "3405",
    "fill_price": "3424",
    "left": "0"
}
]
```

## Query risk limit tiers

`GET /futures/{settle}/risk_limit_tiers`

_Query risk limit tiers_

When the 'contract' parameter is not passed, the default is to query the risk limits for the top 100 markets. 'Limit' and 'offset' correspond to pagination queries at the market level, not to the length of the returned array. This only takes effect when the contract parameter is empty.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Retrieve risk limit configurations for different tiers under a specified contract |
| »» tier | integer(int) | Tier |
| »» risk\_limit | string | Position risk limit |
| »» initial\_rate | string | Initial margin rate |
| »» maintenance\_rate | string | The maintenance margin rate of the first tier of risk limit sheet |
| »» leverage\_max | string | Maximum leverage |
| »» contract | string | Market, only visible when market pagination is requested |
| »» deduction | string | Maintenance margin quick calculation deduction amount |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/risk_limit_tiers'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/risk_limit_tiers 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "maintenance_rate": "0.01",
    "tier": 1,
    "initial_rate": "0.02",
    "leverage_max": "50",
    "risk_limit": "20000",
    "contract": "ZTX_USDT",
    "deduction": "0"
},
{
    "maintenance_rate": "0.013",
    "tier": 2,
    "initial_rate": "0.025",
    "leverage_max": "40",
    "risk_limit": "30000",
    "contract": "ZTX_USDT",
    "deduction": "60"
},
{
    "maintenance_rate": "0.015",
    "tier": 3,
    "initial_rate": "0.02857",
    "leverage_max": "35",
    "risk_limit": "50000",
    "contract": "ZTX_USDT",
    "deduction": "120"
},
{
    "maintenance_rate": "0.02",
    "tier": 4,
    "initial_rate": "0.03333",
    "leverage_max": "30",
    "risk_limit": "70000",
    "contract": "ZTX_USDT",
    "deduction": "370"
},
{
    "maintenance_rate": "0.025",
    "tier": 5,
    "initial_rate": "0.04",
    "leverage_max": "25",
    "risk_limit": "100000",
    "contract": "ZTX_USDT",
    "deduction": "720"
}
]
```

## Get futures account

`GET /futures/{settle}/accounts`

_Get futures account_

Query account information for classic future account and unified account

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » total | string | Balance, only applicable to classic contract account.The balance is the sum of all historical fund flows, including historical transfers in and out, closing settlements, and transaction fee expenses, but does not include upl of positions.total = SUM(history\_dnw, history\_pnl, history\_fee, history\_refr, history\_fund) |
| » unrealised\_pnl | string | Unrealized PNL |
| » position\_margin | string | Deprecated |
| » order\_margin | string | initial margin of all open orders |
| » available | string | Refers to the available withdrawal or trading amount in per-position, specifically the per-position available balance under the unified account that includes the credit line (which incorporates trial funds; since trial funds cannot be withdrawn, the actual withdrawal amount needs to deduct the trial fund portion when processing withdrawals) |
| » point | string | Point card amount |
| » currency | string | Settlement currency |
| » in\_dual\_mode | boolean | Whether Hedge Mode is enabled |
| » enable\_credit | boolean | Whether portfolio margin account mode is enabled |
| » position\_initial\_margin | string | Initial margin occupied by positions, applicable to unified account mode |
| » maintenance\_margin | string | Maintenance margin occupied by positions, applicable to new classic account margin mode and unified account mode |
| » bonus | string | Bonus |
| » enable\_evolved\_classic | boolean | Deprecated |
| » cross\_order\_margin | string | Cross margin order margin, applicable to new classic account margin mode |
| » cross\_initial\_margin | string | Cross margin initial margin, applicable to new classic account margin mode |
| » cross\_maintenance\_margin | string | Cross margin maintenance margin, applicable to new classic account margin mode |
| » cross\_unrealised\_pnl | string | Cross margin unrealized P&L, applicable to new classic account margin mode |
| » cross\_available | string | Cross margin available balance, applicable to new classic account margin mode |
| » cross\_margin\_balance | string | Cross margin balance, applicable to new classic account margin mode |
| » cross\_mmr | string | Cross margin maintenance margin rate, applicable to new classic account margin mode |
| » cross\_imr | string | Cross margin initial margin rate, applicable to new classic account margin mode |
| » isolated\_position\_margin | string | Isolated position margin, applicable to new classic account margin mode |
| » enable\_new\_dual\_mode | boolean | Deprecated |
| » margin\_mode | integer | Margin mode of the account<br>0: classic future account or Classic Spot Margin Mode of unified account;<br>1: Multi-Currency Margin Mode;<br>2: Portoforlio Margin Mode;<br>3: Single-Currency Margin Mode |
| » enable\_tiered\_mm | boolean | Whether to enable tiered maintenance margin calculation |
| » enable\_dual\_plus | boolean | Whether to Support Split Position Mode |
| » position\_mode | string | Position Holding Mode single - Single Direction Position, dual - Dual Direction Position, dual\_plus - Split Position |
| » history | object | Statistical data |
| »» dnw | string | total amount of deposit and withdraw |
| »» pnl | string | total amount of trading profit and loss |
| »» fee | string | total amount of fee |
| »» refr | string | total amount of referrer rebates |
| »» fund | string | total amount of funding costs |
| »» point\_dnw | string | total amount of point deposit and withdraw |
| »» point\_fee | string | total amount of point fee |
| »» point\_refr | string | total amount of referrer rebates of point fee |
| »» bonus\_dnw | string | total amount of perpetual contract bonus transfer |
| »» bonus\_offset | string | total amount of perpetual contract bonus deduction |
| »» cross\_settle | string | Represents the value of profit settlement from the futures account to the spot account under Unified Account Mode. Negative values indicate settlement from futures to spot, while positive values indicate settlement from spot to futures. This value is cumulative. |

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

url = '/futures/usdt/accounts'
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
url="/futures/usdt/accounts"
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
"user": 1666,
"currency": "USDT",
"total": "9707.803567115145",
"unrealised_pnl": "3371.248828",
"position_margin": "38.712189181",
"order_margin": "0",
"available": "9669.091377934145",
"point": "0",
"bonus": "0",
"in_dual_mode": false,
"enable_evolved_classic": false,
"cross_initial_margin": "61855.56788525",
"cross_maintenance_margin": "682.04678105",
"cross_order_margin": "0",
"cross_unrealised_pnl": "1501.178222634128",
"cross_available": "27549.406108813951",
"cross_margin_balance": "10371.77306201952",
"cross_mmr": "797.2134",
"cross_imr": "116.6097",
"isolated_position_margin": "0",
"history": {
    "dnw": "10000",
    "pnl": "68.3685",
    "fee": "-1.645812875",
    "refr": "0",
    "fund": "-358.919120009855",
    "point_dnw": "0",
    "point_fee": "0",
    "point_refr": "0",
    "bonus_dnw": "0",
    "bonus_offset": "0"
},
"enable_tiered_mm": true,
"position_mode": "dual_plus",
"enable_dual_plus": true
}
```

## Query futures account change history

`GET /futures/{settle}/account_book`

_Query futures account change history_

If the contract field is passed, only records containing this field after 2023-10-30 can be filtered.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| type | query | string | false | Change types: |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

**type**: Change types:

- dnw: Deposit and withdrawal
- pnl: Profit and loss from position reduction
- fee: Trading fees
- refr: Referrer rebates
- fund: Funding fees
- point\_dnw: Point card deposit and withdrawal
- point\_fee: Point card trading fees
- point\_refr: Point card referrer rebates
- bonus\_offset: Trial fund deduction

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | number(double) | Change time |
| » change | string | Change amount |
| » balance | string | Balance after change |
| » type | string | Change types:<br>\- dnw: Deposit and withdrawal<br>\- pnl: Profit and loss from position reduction<br>\- fee: Trading fees<br>\- refr: Referrer rebates<br>\- fund: Funding fees<br>\- point\_dnw: Point card deposit and withdrawal<br>\- point\_fee: Point card trading fees<br>\- point\_refr: Point card referrer rebates<br>\- bonus\_offset: Trial fund deduction |
| » text | string | Comment |
| » contract | string | Futures contract, the field is only available for data after 2023-10-30 |
| » trade\_id | string | trade id |
| » id | string | Account change record ID |

#### Enumerated Values

| Property | Value |
| --- | --- |
| type | dnw |
| type | pnl |
| type | fee |
| type | refr |
| type | fund |
| type | point\_dnw |
| type | point\_fee |
| type | point\_refr |
| type | bonus\_offset |

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

url = '/futures/usdt/account_book'
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
url="/futures/usdt/account_book"
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
    "time": 1682294400.123456,
    "change": "0.000010152188",
    "balance": "4.59316525194",
    "text": "ETH_USD:6086261",
    "type": "fee",
    "contract": "ETH_USD",
    "trade_id": "1",
    "id": "1"
}
]
```

## Get user position list

`GET /futures/{settle}/positions`

_Get user position list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| holding | query | boolean | false | Return only real positions - true, return all - false |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/positions'
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
url="/futures/usdt/positions"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Get user's historical position information list by time

`GET /futures/{settle}/positions_timerange`

_Get user's historical position information list by time_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | true | Futures contract |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [PositionTimerange](https://www.gate.com/docs/developers/apiv4/en/#schemapositiontimerange)\] |

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

url = '/futures/usdt/positions_timerange'
query_param = 'contract=BTC_USDT'
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
url="/futures/usdt/positions_timerange"
query_param="contract=BTC_USDT"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Get single position information

`GET /futures/{settle}/positions/{contract}`

_Get single position information_

Get single position information from a contract. If you hold two postions in one contract market, please use this API: /futures/{settle}/dual\_comp/positions/{contract}

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/BTC_USDT'
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
url="/futures/usdt/positions/BTC_USDT"
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
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Get Leverage Information for Specified Mode

`GET /futures/{settle}/get_leverage/{contract}`

_Get Leverage Information for Specified Mode_

Get Leverage Information for Specified Mode

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| pos\_margin\_mode | query | string | true | Position Margin Mode, required for split position mode, values: isolated/cross. |
| dual\_side | query | string | true | dual\_long - Long, dual\_short - Short |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | query leverage success | Inline |

### Response Schema

Status Code **200**

_Return result includes Lever field_

| Name | Type | Description |
| --- | --- | --- |
| » Lever | string | leverage |

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

url = '/futures/usdt/get_leverage/BTC_USDT'
query_param = 'pos_margin_mode=isolated&dual_side=dual_long'
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
url="/futures/usdt/get_leverage/BTC_USDT"
query_param="pos_margin_mode=isolated&dual_side=dual_long"
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
"Lever": "10"
}
```

## Update position margin

`POST /futures/{settle}/positions/{contract}/margin`

_Update position margin_

Under the new risk limit rules(https://www.gate.com/en/help/futures/futures-logic/22162), the position limit is related to the leverage you set; a lower leverage will result in a higher position limit. Please use the leverage adjustment api to adjust the position limit.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| change | query | string | true | Margin change amount, positive number increases, negative number decreases |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/BTC_USDT/margin'
query_param = 'change=0.01'
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
url="/futures/usdt/positions/BTC_USDT/margin"
query_param="change=0.01"
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
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Update position leverage

`POST /futures/{settle}/positions/{contract}/leverage`

_Update position leverage_

⚠️ Position Mode Switching Rules:

- leverage ≠ 0: Isolated Margin Mode (Regardless of whether cross\_leverage\_limit is filled, this parameter will be ignored)
- leverage = 0: Cross Margin Mode (Use cross\_leverage\_limit to set the leverage multiple)

Examples:

- Set isolated margin with 10x leverage: leverage=10
- Set cross margin with 10x leverage: leverage=0&cross\_leverage\_limit=10
- leverage=5&cross\_leverage\_limit=10 → Result: Isolated margin with 5x leverage (cross\_leverage\_limit is ignored)

⚠️ Warning: Incorrect settings may cause unexpected position mode switching, affecting risk management.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| leverage | query | string | true | Set the leverage for isolated margin. When setting isolated margin leverage, the `cross_leverage_limit` must be empty. |
| cross\_leverage\_limit | query | string | false | Set the leverage for cross margin. When setting cross margin leverage, the `leverage` must be set to 0. |
| pid | query | integer(int32) | false | Product ID |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/BTC_USDT/leverage'
query_param = 'leverage=10'
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
url="/futures/usdt/positions/BTC_USDT/leverage"
query_param="leverage=10"
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
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Update Leverage for Specified Mode

`POST /futures/{settle}/positions/{contract}/set_leverage`

_Update Leverage for Specified Mode_

To simplify the complex logic of the leverage interface, added a new interface for modifying leverage

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| leverage | query | string | true | Position Leverage Multiple |
| margin\_mode | query | string | true | Margin Mode isolated/cross |
| dual\_side | query | string | false | dual\_long - Long, dual\_short - Short |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/BTC_USDT/set_leverage'
query_param = 'leverage=10&margin_mode=cross'
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
url="/futures/usdt/positions/BTC_USDT/set_leverage"
query_param="leverage=10&margin_mode=cross"
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
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Switch Position Margin Mode

`POST /futures/{settle}/positions/cross_mode`

_Switch Position Margin Mode_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| body | body | [FuturesPositionCrossMode](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespositioncrossmode) | true | none |
| » mode | body | string | true | Cross/isolated margin mode. ISOLATED - isolated margin, CROSS - cross margin |
| » contract | body | string | true | Futures market |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/cross_mode'
query_param = ''
body='{"mode":"ISOLATED","contract":"BTC_USDT"}'
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
url="/futures/usdt/positions/cross_mode"
query_param=""
body_param='{"mode":"ISOLATED","contract":"BTC_USDT"}'
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
"mode": "ISOLATED",
"contract": "BTC_USDT"
}
```

> Example responses

> 200 Response

```json
{
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Switch Between Cross and Isolated Margin Modes Under Hedge Mode

`POST /futures/{settle}/dual_comp/positions/cross_mode`

_Switch Between Cross and Isolated Margin Modes Under Hedge Mode_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| body | body | [UpdateDualCompPositionCrossModeRequest](https://www.gate.com/docs/developers/apiv4/en/#schemaupdatedualcomppositioncrossmoderequest) | true | none |
| » mode | body | string | true | Cross/isolated margin mode. ISOLATED - isolated margin, CROSS - cross margin |
| » contract | body | string | true | Futures market |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/dual_comp/positions/cross_mode'
query_param = ''
body='{"mode":"ISOLATED","contract":"BTC_USDT"}'
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
url="/futures/usdt/dual_comp/positions/cross_mode"
query_param=""
body_param='{"mode":"ISOLATED","contract":"BTC_USDT"}'
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
"mode": "ISOLATED",
"contract": "BTC_USDT"
}
```

> Example responses

> 200 Response

```json
[
{
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Update position risk limit

`POST /futures/{settle}/positions/{contract}/risk_limit`

_Update position risk limit_

Under the new risk limit rules(https://www.gate.com/en/help/futures/futures-logic/22162), the position limit is related to the leverage you set; a lower leverage will result in a higher position limit. Please use the leverage adjustment api to adjust the position limit.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| risk\_limit | query | string | true | New risk limit value |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Position information | [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition) |

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

url = '/futures/usdt/positions/BTC_USDT/risk_limit'
query_param = 'risk_limit=1000000'
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
url="/futures/usdt/positions/BTC_USDT/risk_limit"
query_param="risk_limit=1000000"
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
"user": 10000,
"contract": "BTC_USDT",
"size": "-9440",
"leverage": "0",
"risk_limit": "100",
"leverage_max": "100",
"maintenance_rate": "0.005",
"value": "3568.62",
"margin": "4.431548146258",
"entry_price": "3779.55",
"liq_price": "99999999",
"mark_price": "3780.32",
"unrealised_pnl": "-0.000507486844",
"realised_pnl": "0.045543982432",
"pnl_pnl": "0.045543982432",
"pnl_fund": "0",
"pnl_fee": "0",
"history_pnl": "0",
"last_close_pnl": "0",
"realised_point": "0",
"history_point": "0",
"adl_ranking": 5,
"pending_orders": 16,
"close_order": {
    "id": 232323,
    "price": "3779",
    "is_liq": false
},
"mode": "single",
"update_time": 1684994406,
"update_id": 1,
"cross_leverage_limit": "0",
"risk_limit_table": "BIG_HOT_COIN_50X_V2",
"average_maintenance_rate": "0.005",
"pos_margin_mode": "isolated",
"lever": "30"
}
```

## Set position mode

`POST /futures/{settle}/dual_mode`

_Set position mode_

The prerequisite for changing mode is that all positions have no holdings and no pending orders

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| dual\_mode | query | boolean | true | Whether to enable Hedge Mode |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Updated successfully | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » total | string | Balance, only applicable to classic contract account.The balance is the sum of all historical fund flows, including historical transfers in and out, closing settlements, and transaction fee expenses, but does not include upl of positions.total = SUM(history\_dnw, history\_pnl, history\_fee, history\_refr, history\_fund) |
| » unrealised\_pnl | string | Unrealized PNL |
| » position\_margin | string | Deprecated |
| » order\_margin | string | initial margin of all open orders |
| » available | string | Refers to the available withdrawal or trading amount in per-position, specifically the per-position available balance under the unified account that includes the credit line (which incorporates trial funds; since trial funds cannot be withdrawn, the actual withdrawal amount needs to deduct the trial fund portion when processing withdrawals) |
| » point | string | Point card amount |
| » currency | string | Settlement currency |
| » in\_dual\_mode | boolean | Whether Hedge Mode is enabled |
| » enable\_credit | boolean | Whether portfolio margin account mode is enabled |
| » position\_initial\_margin | string | Initial margin occupied by positions, applicable to unified account mode |
| » maintenance\_margin | string | Maintenance margin occupied by positions, applicable to new classic account margin mode and unified account mode |
| » bonus | string | Bonus |
| » enable\_evolved\_classic | boolean | Deprecated |
| » cross\_order\_margin | string | Cross margin order margin, applicable to new classic account margin mode |
| » cross\_initial\_margin | string | Cross margin initial margin, applicable to new classic account margin mode |
| » cross\_maintenance\_margin | string | Cross margin maintenance margin, applicable to new classic account margin mode |
| » cross\_unrealised\_pnl | string | Cross margin unrealized P&L, applicable to new classic account margin mode |
| » cross\_available | string | Cross margin available balance, applicable to new classic account margin mode |
| » cross\_margin\_balance | string | Cross margin balance, applicable to new classic account margin mode |
| » cross\_mmr | string | Cross margin maintenance margin rate, applicable to new classic account margin mode |
| » cross\_imr | string | Cross margin initial margin rate, applicable to new classic account margin mode |
| » isolated\_position\_margin | string | Isolated position margin, applicable to new classic account margin mode |
| » enable\_new\_dual\_mode | boolean | Deprecated |
| » margin\_mode | integer | Margin mode of the account<br>0: classic future account or Classic Spot Margin Mode of unified account;<br>1: Multi-Currency Margin Mode;<br>2: Portoforlio Margin Mode;<br>3: Single-Currency Margin Mode |
| » enable\_tiered\_mm | boolean | Whether to enable tiered maintenance margin calculation |
| » enable\_dual\_plus | boolean | Whether to Support Split Position Mode |
| » position\_mode | string | Position Holding Mode single - Single Direction Position, dual - Dual Direction Position, dual\_plus - Split Position |
| » history | object | Statistical data |
| »» dnw | string | total amount of deposit and withdraw |
| »» pnl | string | total amount of trading profit and loss |
| »» fee | string | total amount of fee |
| »» refr | string | total amount of referrer rebates |
| »» fund | string | total amount of funding costs |
| »» point\_dnw | string | total amount of point deposit and withdraw |
| »» point\_fee | string | total amount of point fee |
| »» point\_refr | string | total amount of referrer rebates of point fee |
| »» bonus\_dnw | string | total amount of perpetual contract bonus transfer |
| »» bonus\_offset | string | total amount of perpetual contract bonus deduction |
| »» cross\_settle | string | Represents the value of profit settlement from the futures account to the spot account under Unified Account Mode. Negative values indicate settlement from futures to spot, while positive values indicate settlement from spot to futures. This value is cumulative. |

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

url = '/futures/usdt/dual_mode'
query_param = 'dual_mode=true'
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
url="/futures/usdt/dual_mode"
query_param="dual_mode=true"
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
"user": 1666,
"currency": "USDT",
"total": "9707.803567115145",
"unrealised_pnl": "3371.248828",
"position_margin": "38.712189181",
"order_margin": "0",
"available": "9669.091377934145",
"point": "0",
"bonus": "0",
"in_dual_mode": false,
"enable_evolved_classic": false,
"cross_initial_margin": "61855.56788525",
"cross_maintenance_margin": "682.04678105",
"cross_order_margin": "0",
"cross_unrealised_pnl": "1501.178222634128",
"cross_available": "27549.406108813951",
"cross_margin_balance": "10371.77306201952",
"cross_mmr": "797.2134",
"cross_imr": "116.6097",
"isolated_position_margin": "0",
"history": {
    "dnw": "10000",
    "pnl": "68.3685",
    "fee": "-1.645812875",
    "refr": "0",
    "fund": "-358.919120009855",
    "point_dnw": "0",
    "point_fee": "0",
    "point_refr": "0",
    "bonus_dnw": "0",
    "bonus_offset": "0"
},
"enable_tiered_mm": true,
"position_mode": "dual_plus",
"enable_dual_plus": true
}
```

## Set Position Holding Mode, replacing the dual\_mode interface

`POST /futures/{settle}/set_position_mode`

_Set Position Holding Mode, replacing the dual\_mode interface_

The prerequisite for changing mode is that all positions have no holdings and no pending orders

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| position\_mode | query | string | true | Optional Values: single, dual, dual\_plus, representing Single Direction, Dual Direction, Split Position respectively |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Updated successfully | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » total | string | Balance, only applicable to classic contract account.The balance is the sum of all historical fund flows, including historical transfers in and out, closing settlements, and transaction fee expenses, but does not include upl of positions.total = SUM(history\_dnw, history\_pnl, history\_fee, history\_refr, history\_fund) |
| » unrealised\_pnl | string | Unrealized PNL |
| » position\_margin | string | Deprecated |
| » order\_margin | string | initial margin of all open orders |
| » available | string | Refers to the available withdrawal or trading amount in per-position, specifically the per-position available balance under the unified account that includes the credit line (which incorporates trial funds; since trial funds cannot be withdrawn, the actual withdrawal amount needs to deduct the trial fund portion when processing withdrawals) |
| » point | string | Point card amount |
| » currency | string | Settlement currency |
| » in\_dual\_mode | boolean | Whether Hedge Mode is enabled |
| » enable\_credit | boolean | Whether portfolio margin account mode is enabled |
| » position\_initial\_margin | string | Initial margin occupied by positions, applicable to unified account mode |
| » maintenance\_margin | string | Maintenance margin occupied by positions, applicable to new classic account margin mode and unified account mode |
| » bonus | string | Bonus |
| » enable\_evolved\_classic | boolean | Deprecated |
| » cross\_order\_margin | string | Cross margin order margin, applicable to new classic account margin mode |
| » cross\_initial\_margin | string | Cross margin initial margin, applicable to new classic account margin mode |
| » cross\_maintenance\_margin | string | Cross margin maintenance margin, applicable to new classic account margin mode |
| » cross\_unrealised\_pnl | string | Cross margin unrealized P&L, applicable to new classic account margin mode |
| » cross\_available | string | Cross margin available balance, applicable to new classic account margin mode |
| » cross\_margin\_balance | string | Cross margin balance, applicable to new classic account margin mode |
| » cross\_mmr | string | Cross margin maintenance margin rate, applicable to new classic account margin mode |
| » cross\_imr | string | Cross margin initial margin rate, applicable to new classic account margin mode |
| » isolated\_position\_margin | string | Isolated position margin, applicable to new classic account margin mode |
| » enable\_new\_dual\_mode | boolean | Deprecated |
| » margin\_mode | integer | Margin mode of the account<br>0: classic future account or Classic Spot Margin Mode of unified account;<br>1: Multi-Currency Margin Mode;<br>2: Portoforlio Margin Mode;<br>3: Single-Currency Margin Mode |
| » enable\_tiered\_mm | boolean | Whether to enable tiered maintenance margin calculation |
| » enable\_dual\_plus | boolean | Whether to Support Split Position Mode |
| » position\_mode | string | Position Holding Mode single - Single Direction Position, dual - Dual Direction Position, dual\_plus - Split Position |
| » history | object | Statistical data |
| »» dnw | string | total amount of deposit and withdraw |
| »» pnl | string | total amount of trading profit and loss |
| »» fee | string | total amount of fee |
| »» refr | string | total amount of referrer rebates |
| »» fund | string | total amount of funding costs |
| »» point\_dnw | string | total amount of point deposit and withdraw |
| »» point\_fee | string | total amount of point fee |
| »» point\_refr | string | total amount of referrer rebates of point fee |
| »» bonus\_dnw | string | total amount of perpetual contract bonus transfer |
| »» bonus\_offset | string | total amount of perpetual contract bonus deduction |
| »» cross\_settle | string | Represents the value of profit settlement from the futures account to the spot account under Unified Account Mode. Negative values indicate settlement from futures to spot, while positive values indicate settlement from spot to futures. This value is cumulative. |

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

url = '/futures/usdt/set_position_mode'
query_param = 'position_mode=dual_plus'
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
url="/futures/usdt/set_position_mode"
query_param="position_mode=dual_plus"
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
"user": 1666,
"currency": "USDT",
"total": "9707.803567115145",
"unrealised_pnl": "3371.248828",
"position_margin": "38.712189181",
"order_margin": "0",
"available": "9669.091377934145",
"point": "0",
"bonus": "0",
"in_dual_mode": false,
"enable_evolved_classic": false,
"cross_initial_margin": "61855.56788525",
"cross_maintenance_margin": "682.04678105",
"cross_order_margin": "0",
"cross_unrealised_pnl": "1501.178222634128",
"cross_available": "27549.406108813951",
"cross_margin_balance": "10371.77306201952",
"cross_mmr": "797.2134",
"cross_imr": "116.6097",
"isolated_position_margin": "0",
"history": {
    "dnw": "10000",
    "pnl": "68.3685",
    "fee": "-1.645812875",
    "refr": "0",
    "fund": "-358.919120009855",
    "point_dnw": "0",
    "point_fee": "0",
    "point_refr": "0",
    "bonus_dnw": "0",
    "bonus_offset": "0"
},
"enable_tiered_mm": true,
"position_mode": "dual_plus",
"enable_dual_plus": true
}
```

## Get position information in Hedge Mode

`GET /futures/{settle}/dual_comp/positions/{contract}`

_Get position information in Hedge Mode_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/dual_comp/positions/BTC_USDT'
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
url="/futures/usdt/dual_comp/positions/BTC_USDT"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Update position margin in Hedge Mode

`POST /futures/{settle}/dual_comp/positions/{contract}/margin`

_Update position margin in Hedge Mode_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| change | query | string | true | Margin change amount, positive number increases, negative number decreases |
| dual\_side | query | string | true | Long or short position |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/dual_comp/positions/BTC_USDT/margin'
query_param = 'change=0.01&dual_side=dual_long'
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
url="/futures/usdt/dual_comp/positions/BTC_USDT/margin"
query_param="change=0.01&dual_side=dual_long"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Update position leverage in Hedge Mode

`POST /futures/{settle}/dual_comp/positions/{contract}/leverage`

_Update position leverage in Hedge Mode_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| leverage | query | string | true | New position leverage |
| cross\_leverage\_limit | query | string | false | Cross margin leverage (valid only when `leverage` is 0) |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/dual_comp/positions/BTC_USDT/leverage'
query_param = 'leverage=10'
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
url="/futures/usdt/dual_comp/positions/BTC_USDT/leverage"
query_param="leverage=10"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Update position risk limit in Hedge Mode

`POST /futures/{settle}/dual_comp/positions/{contract}/risk_limit`

_Update position risk limit in Hedge Mode_

Under the new risk limit rules(https://www.gate.com/en/help/futures/futures-logic/22162), the position limit is related to the leverage you set; a lower leverage will result in a higher position limit. Please use the leverage adjustment api to adjust the position limit.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | path | string | true | Futures contract |
| risk\_limit | query | string | true | New risk limit value |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[ [Position](https://www.gate.com/docs/developers/apiv4/en/#schemaposition)\] |

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

url = '/futures/usdt/dual_comp/positions/BTC_USDT/risk_limit'
query_param = 'risk_limit=1000000'
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
url="/futures/usdt/dual_comp/positions/BTC_USDT/risk_limit"
query_param="risk_limit=1000000"
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
    "user": 10000,
    "contract": "BTC_USDT",
    "size": "-9440",
    "leverage": "0",
    "risk_limit": "100",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "3568.62",
    "margin": "4.431548146258",
    "entry_price": "3779.55",
    "liq_price": "99999999",
    "mark_price": "3780.32",
    "unrealised_pnl": "-0.000507486844",
    "realised_pnl": "0.045543982432",
    "pnl_pnl": "0.045543982432",
    "pnl_fund": "0",
    "pnl_fee": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 16,
    "close_order": {
      "id": 232323,
      "price": "3779",
      "is_liq": false
    },
    "mode": "single",
    "update_time": 1684994406,
    "update_id": 1,
    "cross_leverage_limit": "0",
    "risk_limit_table": "BIG_HOT_COIN_50X_V2",
    "average_maintenance_rate": "0.005",
    "pos_margin_mode": "isolated",
    "lever": "30"
}
]
```

## Place futures order

`POST /futures/{settle}/orders`

_Place futures order_

- When placing an order, the number of contracts is specified `size`, not the number of coins. The number of coins corresponding to each contract is returned in the contract details interface
`quanto_multiplier`
- 0 The order that was completed cannot be obtained after 10 minutes of withdrawal, and the order will be mentioned that the order does not exist
- Setting `reduce_only` to `true` can prevent the position from being penetrated when reducing the position
- In single-position mode, if you need to close the position, you need to set `size` to 0 and `close` to `true`
- In dual warehouse mode,
- Reduce position: reduce\_only=true, size is a positive number that indicates short position, negative number that indicates long position
- Add number that indicates adding long positions, and negative numbers indicate adding short positions
- Close position: size=0, set the direction of closing position according to auto\_size, and set `reduce_only` to true
at the same time - reduce\_only: Make sure to only perform position reduction operations to prevent increased positions
- Set `stp_act` to determine the use of a strategy that restricts user transactions. For detailed usage, refer to the body parameter `stp_act`

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) | true | none |
| » contract | body | string | true | Futures contract |
| » size | body | string | true | Required. Trading quantity. Positive for buy, negative for sell. Set to 0 for close position orders. |
| » iceberg | body | string | false | Display size for iceberg orders. 0 for non-iceberg orders. Note that hidden portions are charged taker fees. |
| » price | body | string | true | Required. Order Price; a price of 0 with `tif` as `ioc` represents a market order. |
| » close | body | boolean | false | Set as `true` to close the position, with `size` set to 0 |
| » reduce\_only | body | boolean | false | Set as `true` to be reduce-only order |
| » tif | body | string | false | Time in force |
| » text | body | string | false | Custom order information. If not empty, must follow the rules below: |
| » auto\_size | body | string | false | Set side to close dual-mode position. `close_long` closes the long side; while `close_short` the short one. Note `size` also needs to be set to 0 |
| » stp\_act | body | string | false | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies |
| » pid | body | integer(int64) | false | Position ID |
| » market\_order\_slip\_ratio | body | string | false | Custom maximum slippage rate for market orders. If not provided, the default contract settings will be used |
| » pos\_margin\_mode | body | string | false | Position Margin Mode isolated - Isolated Margin, cross - Cross Margin, only passed in simple split position mode |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**» tif**: Time in force

- gtc: GoodTillCancelled
- ioc: ImmediateOrCancelled, taker only
- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee
- fok: FillOrKill, fill either completely or none

**» text**: Custom order information. If not empty, must follow the rules below:

1. Prefixed with `t-`
2. No longer than 28 bytes without `t-` prefix
3. Can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)

In addition to user-defined information, the following are internal reserved fields that identify the order source:

- web: Web
- api: API call
- app: Mobile app
- auto\_deleveraging: Automatic deleveraging
- liquidation: Forced liquidation of positions under the old classic mode
- liq-xxx: a. Forced liquidation of positions under the new classic mode, including isolated margin, one-way cross margin, and non-hedged positions under two-way cross margin. b. Forced liquidation of isolated positions under the unified account single-currency margin mode
- hedge-liq-xxx: Forced liquidation of hedged positions under the new classic mode two-way cross margin, i.e., simultaneously closing long and short positions
- pm\_liquidate: Forced liquidation under unified account multi-currency margin mode
- comb\_margin\_liquidate: Forced liquidation under unified account portfolio margin mode
- scm\_liquidate: Forced liquidation of positions under unified account single-currency margin mode
- insurance: Insurance
- clear: Contract delisting withdrawal

**» stp\_act**: Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies

1. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.
2. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.
3. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'

- cn: Cancel newest, cancel new orders and keep old ones
- co: Cancel oldest, cancel old orders and keep new ones
- cb: Cancel both, both old and new orders will be cancelled

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » tif | gtc |
| » tif | ioc |
| » tif | poc |
| » tif | fok |
| » auto\_size | close\_long |
| » auto\_size | close\_short |
| » stp\_act | co |
| » stp\_act | cn |
| » stp\_act | cb |
| » stp\_act | - |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Order details | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) |

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

url = '/futures/usdt/orders'
query_param = ''
body='{"contract":"BTC_USDT","size":"6024","iceberg":"0","price":"3765","tif":"gtc","text":"t-my-custom-id","stp_act":"-","order_value":"64112.2099000000005","trade_value":"64112.2099000000005","market_order_slip_ratio":"0.03","pos_margin_mode":"isolated"}'
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
url="/futures/usdt/orders"
query_param=""
body_param='{"contract":"BTC_USDT","size":"6024","iceberg":"0","price":"3765","tif":"gtc","text":"t-my-custom-id","stp_act":"-","order_value":"64112.2099000000005","trade_value":"64112.2099000000005","market_order_slip_ratio":"0.03","pos_margin_mode":"isolated"}'
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
"contract": "BTC_USDT",
"size": "6024",
"iceberg": "0",
"price": "3765",
"tif": "gtc",
"text": "t-my-custom-id",
"stp_act": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

> Example responses

> 201 Response

```json
{
"id": 15675394,
"user": 100000,
"contract": "BTC_USDT",
"create_time": 1546569968,
"size": "6024",
"iceberg": "0",
"left": "6024",
"price": "3765",
"fill_price": "0",
"mkfr": "-0.00025",
"tkfr": "0.00075",
"tif": "gtc",
"refu": 0,
"is_reduce_only": false,
"is_close": false,
"is_liq": false,
"text": "t-my-custom-id",
"status": "finished",
"finish_time": 1514764900,
"finish_as": "cancelled",
"stp_id": 0,
"stp_act": "-",
"amend_text": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

## Query futures order list

`GET /futures/{settle}/orders`

_Query futures order list_

- Zero-fill order cannot be retrieved for 10 minutes after cancellation
- Historical orders, by default, only data within the past 6 months is supported.
If you need to query data for a longer period, please use `GET /futures/{settle}/orders_timerange`.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| contract | query | string | false | Futures contract, return related data only if specified |
| status | query | string | true | Query order list based on status |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| last\_id | query | string | false | Use the ID of the last record in the previous list as the starting point for the next list |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**last\_id**: Use the ID of the last record in the previous list as the starting point for the next list

Operations based on custom IDs can only be checked when orders are pending. After orders are completed (filled/cancelled), they can be checked within 1 hour after completion. After expiration, only order IDs can be used

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder)\] |

### Response Headers

| Status | Header | Type | Format | Description |
| --- | --- | --- | --- | --- |
| 200 | X-Pagination-Limit | integer |  | Limit specified for pagination |
| 200 | X-Pagination-Offset | integer |  | Offset specified for pagination |

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

url = '/futures/usdt/orders'
query_param = 'status=open'
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
url="/futures/usdt/orders"
query_param="status=open"
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
    "id": 15675394,
    "user": 100000,
    "contract": "BTC_USDT",
    "create_time": 1546569968,
    "size": "6024",
    "iceberg": "0",
    "left": "6024",
    "price": "3765",
    "fill_price": "0",
    "mkfr": "-0.00025",
    "tkfr": "0.00075",
    "tif": "gtc",
    "refu": 0,
    "is_reduce_only": false,
    "is_close": false,
    "is_liq": false,
    "text": "t-my-custom-id",
    "status": "finished",
    "finish_time": 1514764900,
    "finish_as": "cancelled",
    "stp_id": 0,
    "stp_act": "-",
    "amend_text": "-",
    "order_value": "64112.2099000000005",
    "trade_value": "64112.2099000000005",
    "market_order_slip_ratio": "0.03",
    "pos_margin_mode": "isolated"
}
]
```

## Cancel all orders with 'open' status

`DELETE /futures/{settle}/orders`

_Cancel all orders with 'open' status_

Zero-fill orders cannot be retrieved 10 minutes after order cancellation

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| contract | query | string | false | Contract Identifier; if specified, only cancel pending orders related to this contract |
| side | query | string | false | Specify all buy orders or all sell orders, both are included if not specified. Set to bid to cancel all buy orders, set to ask to cancel all sell orders |
| exclude\_reduce\_only | query | boolean | false | Whether to exclude reduce-only orders |
| text | query | string | false | Remark for order cancellation |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch cancellation successful | \[ [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder)\] |

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

url = '/futures/usdt/orders'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('DELETE', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('DELETE', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="DELETE"
url="/futures/usdt/orders"
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
    "id": 15675394,
    "user": 100000,
    "contract": "BTC_USDT",
    "create_time": 1546569968,
    "size": "6024",
    "iceberg": "0",
    "left": "6024",
    "price": "3765",
    "fill_price": "0",
    "mkfr": "-0.00025",
    "tkfr": "0.00075",
    "tif": "gtc",
    "refu": 0,
    "is_reduce_only": false,
    "is_close": false,
    "is_liq": false,
    "text": "t-my-custom-id",
    "status": "finished",
    "finish_time": 1514764900,
    "finish_as": "cancelled",
    "stp_id": 0,
    "stp_act": "-",
    "amend_text": "-",
    "order_value": "64112.2099000000005",
    "trade_value": "64112.2099000000005",
    "market_order_slip_ratio": "0.03",
    "pos_margin_mode": "isolated"
}
]
```

## Query futures order list by time range

`GET /futures/{settle}/orders_timerange`

_Query futures order list by time range_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [FuturesOrderTimerange](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesordertimerange)\] |

### Response Headers

| Status | Header | Type | Format | Description |
| --- | --- | --- | --- | --- |
| 200 | X-Pagination-Limit | integer |  | Limit specified for pagination |
| 200 | X-Pagination-Offset | integer |  | Offset specified for pagination |

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

url = '/futures/usdt/orders_timerange'
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
url="/futures/usdt/orders_timerange"
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
    "id": 15675394,
    "user": 100000,
    "contract": "BTC_USDT",
    "create_time": 1546569968,
    "size": "6024",
    "iceberg": "0",
    "left": "6024",
    "price": "3765",
    "fill_price": "0",
    "mkfr": "-0.00025",
    "tkfr": "0.00075",
    "tif": "gtc",
    "refu": 0,
    "is_reduce_only": false,
    "is_close": false,
    "is_liq": false,
    "text": "t-my-custom-id",
    "status": "finished",
    "finish_time": 1514764900,
    "finish_as": "cancelled",
    "stp_id": 0,
    "stp_act": "-",
    "amend_text": "-",
    "order_value": "64112.2099000000005",
    "trade_value": "64112.2099000000005",
    "market_order_slip_ratio": "0.03",
    "pos_margin_mode": "isolated"
}
]
```

## Place batch futures orders

`POST /futures/{settle}/batch_orders`

_Place batch futures orders_

- Up to 10 orders per request
- If any of the order's parameters are missing or in the wrong format, all of them will not be executed, and a http status 400 error will be returned directly
- If the parameters are checked and passed, all are executed. Even if there is a business logic error in the middle (such as insufficient funds), it will not affect other execution orders
- The returned result is in array format, and the order corresponds to the orders in the request body
- In the returned result, the `succeeded` field of type bool indicates whether the execution was successful or not
- If the execution is successful, the normal order content is included; if the execution fails, the `label` field is included to indicate the cause of the error
- In the rate limiting, each order is counted individually

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | array\[ [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder)\] | true | none |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Request execution completed | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Futures order details\] |
| » _None_ | object | Futures order details |
| »» succeeded | boolean | Request execution result |
| »» label | string | Error label, only exists if execution fails |
| »» detail | string | Error detail, only present if execution failed and details need to be given |
| »» id | integer(int64) | Futures order ID |
| »» user | integer | User ID |
| »» create\_time | number(double) | Creation time of order |
| »» finish\_time | number(double) | Order finished time. Not returned if order is open |
| »» finish\_as | string | How the order was finished:<br>\- filled: all filled<br>\- cancelled: manually cancelled<br>\- liquidated: cancelled because of liquidation<br>\- ioc: time in force is `IOC`, finish immediately<br>\- auto\_deleveraged: finished by ADL<br>\- reduce\_only: cancelled because of increasing position while `reduce-only` set<br>\- position\_closed: cancelled because the position was closed<br>\- reduce\_out: only reduce positions by excluding hard-to-fill orders<br>\- stp: cancelled because self trade prevention |
| »» status | string | Order status<br>\- `open`: Pending<br>\- `finished`: Completed |
| »» contract | string | Futures contract |
| »» size | string | Required. Trading quantity. Positive for buy, negative for sell. Set to 0 for close position orders. |
| »» iceberg | string | Display size for iceberg orders. 0 for non-iceberg orders. Note that hidden portions are charged taker fees. |
| »» price | string | Order price. Price of 0 with `tif` set to `ioc` represents a market order. |
| »» is\_close | boolean | Is the order to close position |
| »» is\_reduce\_only | boolean | Is the order reduce-only |
| »» is\_liq | boolean | Is the order for liquidation |
| »» tif | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none |
| »» left | string | Unfilled quantity |
| »» fill\_price | string | Fill price |
| »» text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- web: from web<br>\- api: from API<br>\- app: from mobile phones<br>\- auto\_deleveraging: from ADL<br>\- liquidation: from liquidation<br>\- insurance: from insurance |
| »» tkfr | string | Taker fee |
| »» mkfr | string | Maker fee |
| »» refu | integer | Referrer user ID |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» market\_order\_slip\_ratio | string | The maximum slippage ratio |

#### Enumerated Values

| Property | Value |
| --- | --- |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidated |
| finish\_as | ioc |
| finish\_as | auto\_deleveraged |
| finish\_as | reduce\_only |
| finish\_as | position\_closed |
| finish\_as | reduce\_out |
| finish\_as | stp |
| status | open |
| status | finished |
| tif | gtc |
| tif | ioc |
| tif | poc |
| tif | fok |
| stp\_act | co |
| stp\_act | cn |
| stp\_act | cb |
| stp\_act | - |

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

url = '/futures/usdt/batch_orders'
query_param = ''
body='[{"contract":"BTC_USDT","size":"6024","iceberg":"0","price":"3765","tif":"gtc","text":"t-my-custom-id","stp_act":"-","order_value":"64112.2099000000005","trade_value":"64112.2099000000005","market_order_slip_ratio":"0.03","pos_margin_mode":"isolated"}]'
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
url="/futures/usdt/batch_orders"
query_param=""
body_param='[{"contract":"BTC_USDT","size":"6024","iceberg":"0","price":"3765","tif":"gtc","text":"t-my-custom-id","stp_act":"-","order_value":"64112.2099000000005","trade_value":"64112.2099000000005","market_order_slip_ratio":"0.03","pos_margin_mode":"isolated"}]'
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
[
{
    "contract": "BTC_USDT",
    "size": "6024",
    "iceberg": "0",
    "price": "3765",
    "tif": "gtc",
    "text": "t-my-custom-id",
    "stp_act": "-",
    "order_value": "64112.2099000000005",
    "trade_value": "64112.2099000000005",
    "market_order_slip_ratio": "0.03",
    "pos_margin_mode": "isolated"
}
]
```

> Example responses

> 200 Response

```json
[
{
    "succeeded": true,
    "id": 15675394,
    "user": 100000,
    "contract": "BTC_USDT",
    "create_time": 1546569968,
    "size": "6024",
    "iceberg": "0",
    "left": "6024",
    "price": "3765",
    "fill_price": "0",
    "mkfr": "-0.00025",
    "tkfr": "0.00075",
    "tif": "gtc",
    "refu": 0,
    "is_reduce_only": false,
    "is_close": false,
    "is_liq": false,
    "text": "t-my-custom-id",
    "status": "finished",
    "finish_time": 1514764900,
    "finish_as": "cancelled",
    "stp_id": 0,
    "stp_act": "-",
    "amend_text": "-",
    "market_order_slip_ratio": "0.03"
}
]
```

## Query single order details

`GET /futures/{settle}/orders/{order_id}`

_Query single order details_

- Zero-fill order cannot be retrieved for 10 minutes after cancellation
- Historical orders, by default, only data within the past 6 months is supported.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| order\_id | path | string | true | The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field: |

#### Detailed descriptions

**order\_id**: The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field:

1. If the order was not filled and has been cancelled, after 60 seconds you cannot query the order by `text`; continuing to use `text` returns error ORDER\_NOT\_FOUND.
2. If the order was fully or partially filled, you can query the order by `text` indefinitely.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order details | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) |

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

url = '/futures/usdt/orders/12345'
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
url="/futures/usdt/orders/12345"
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
"id": 15675394,
"user": 100000,
"contract": "BTC_USDT",
"create_time": 1546569968,
"size": "6024",
"iceberg": "0",
"left": "6024",
"price": "3765",
"fill_price": "0",
"mkfr": "-0.00025",
"tkfr": "0.00075",
"tif": "gtc",
"refu": 0,
"is_reduce_only": false,
"is_close": false,
"is_liq": false,
"text": "t-my-custom-id",
"status": "finished",
"finish_time": 1514764900,
"finish_as": "cancelled",
"stp_id": 0,
"stp_act": "-",
"amend_text": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

## Cancel single order

`DELETE /futures/{settle}/orders/{order_id}`

_Cancel single order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| settle | path | string | true | Settle currency |
| order\_id | path | string | true | The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field: |

#### Detailed descriptions

**order\_id**: The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field:

1. If the order was not filled and has been cancelled, after 60 seconds you cannot query the order by `text`; continuing to use `text` returns error ORDER\_NOT\_FOUND.
2. If the order was fully or partially filled, you can query the order by `text` indefinitely.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order details | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) |

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

url = '/futures/usdt/orders/12345'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('DELETE', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('DELETE', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="DELETE"
url="/futures/usdt/orders/12345"
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
"id": 15675394,
"user": 100000,
"contract": "BTC_USDT",
"create_time": 1546569968,
"size": "6024",
"iceberg": "0",
"left": "6024",
"price": "3765",
"fill_price": "0",
"mkfr": "-0.00025",
"tkfr": "0.00075",
"tif": "gtc",
"refu": 0,
"is_reduce_only": false,
"is_close": false,
"is_liq": false,
"text": "t-my-custom-id",
"status": "finished",
"finish_time": 1514764900,
"finish_as": "cancelled",
"stp_id": 0,
"stp_act": "-",
"amend_text": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

## Amend single order

`PUT /futures/{settle}/orders/{order_id}`

_Amend single order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | [FuturesOrderAmendment](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorderamendment) | true | none |
| » size | body | string | false | New order size, including filled part. |
| » price | body | string | false | New order price |
| » amend\_text | body | string | false | Custom info during order amendment |
| » text | body | string | false | Internal users can modify information in the text field. |
| settle | path | string | true | Settle currency |
| order\_id | path | string | true | The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field: |

#### Detailed descriptions

**» size**: New order size, including filled part.

- If new size is less than or equal to filled size, the order will be cancelled.
- Order side must be identical to the original one.
- Close order size cannot be changed.
- For reduce only orders, increasing size may leads to other reduce only orders being cancelled.
- If price is not changed, decreasing size will not change its precedence in order book, while increasing will move it to the last at current price.

**order\_id**: The order ID returned when the order is created successfully, or the custom ID specified by the user when creating the order (i.e. the `text` field). When using the custom `text` field:

1. If the order was not filled and has been cancelled, after 60 seconds you cannot query the order by `text`; continuing to use `text` returns error ORDER\_NOT\_FOUND.
2. If the order was fully or partially filled, you can query the order by `text` indefinitely.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order details | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) |

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

url = '/futures/usdt/orders/12345'
query_param = ''
body='{"size":"100","price":"54321"}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('PUT', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('PUT', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="PUT"
url="/futures/usdt/orders/12345"
query_param=""
body_param='{"size":"100","price":"54321"}'
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
"size": "100",
"price": "54321"
}
```

> Example responses

> 200 Response

```json
{
"id": 15675394,
"user": 100000,
"contract": "BTC_USDT",
"create_time": 1546569968,
"size": "6024",
"iceberg": "0",
"left": "6024",
"price": "3765",
"fill_price": "0",
"mkfr": "-0.00025",
"tkfr": "0.00075",
"tif": "gtc",
"refu": 0,
"is_reduce_only": false,
"is_close": false,
"is_liq": false,
"text": "t-my-custom-id",
"status": "finished",
"finish_time": 1514764900,
"finish_as": "cancelled",
"stp_id": 0,
"stp_act": "-",
"amend_text": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

## Query personal trading records

`GET /futures/{settle}/my_trades`

_Query personal trading records_

By default, only supports querying data within 6 months. For older data, use `GET /futures/{settle}/my_trades_timerange`

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| order | query | integer(int64) | false | Futures order ID, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| last\_id | query | string | false | Specify the starting point for this list based on a previously retrieved id |

#### Detailed descriptions

**last\_id**: Specify the starting point for this list based on a previously retrieved id

This parameter is deprecated. If you need to iterate through and retrieve more records, we recommend using 'GET /futures/{settle}/my\_trades\_timerange'.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | integer(int64) | Fill ID |
| » create\_time | number(double) | Fill Time |
| » contract | string | Futures contract |
| » order\_id | string | Related order ID |
| » size | string | Trading size |
| » close\_size | string | Number of closed positions:<br>close\_size=0 && size＞0 Open long position<br>close\_size=0 && size＜0 Open short position<br>close\_size>0 && size>0 && size <= close\_size Close short position<br>close\_size>0 && size>0 && size > close\_size Close short position and open long position<br>close\_size<0 && size<0 && size >= close\_size Close long position<br>close\_size<0 && size<0 && size < close\_size Close long position and open short position |
| » price | string | Fill Price |
| » role | string | Trade role. taker - taker, maker - maker |
| » text | string | Order custom information |
| » fee | string | Trade fee |
| » point\_fee | string | Points used to deduct trade fee |
| » trade\_value | string | trade value |

#### Enumerated Values

| Property | Value |
| --- | --- |
| role | taker |
| role | maker |

### Response Headers

| Status | Header | Type | Format | Description |
| --- | --- | --- | --- | --- |
| 200 | X-Pagination-Limit | integer |  | Limit specified for pagination |
| 200 | X-Pagination-Offset | integer |  | Offset specified for pagination |

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

url = '/futures/usdt/my_trades'
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
url="/futures/usdt/my_trades"
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
    "id": 121234231,
    "create_time": 1514764800.123,
    "contract": "BTC_USDT",
    "order_id": "21893289839",
    "size": "100",
    "price": "100.123",
    "text": "t-123456",
    "fee": "0.01",
    "point_fee": "0",
    "role": "taker",
    "close_size": "0",
    "trade_value": "28601.83"
}
]
```

## Query personal trading records by time range

`GET /futures/{settle}/my_trades_timerange`

_Query personal trading records by time range_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| role | query | string | false | Query role, maker or taker |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » trade\_id | string | Fill ID |
| » create\_time | number(double) | Fill Time |
| » contract | string | Futures contract |
| » order\_id | string | Related order ID |
| » size | string | Trading size |
| » close\_size | string | Number of closed positions:<br>close\_size=0 && size＞0 Open long position<br>close\_size=0 && size＜0 Open short position<br>close\_size>0 && size>0 && size <= close\_size Close short position<br>close\_size>0 && size>0 && size > close\_size Close short position and open long position<br>close\_size<0 && size<0 && size >= close\_size Close long position<br>close\_size<0 && size<0 && size < close\_size Close long position and open short position |
| » price | string | Fill Price |
| » role | string | Trade role. taker - taker, maker - maker |
| » text | string | Order custom information |
| » fee | string | Trade fee |
| » point\_fee | string | Points used to deduct trade fee |

#### Enumerated Values

| Property | Value |
| --- | --- |
| role | taker |
| role | maker |

### Response Headers

| Status | Header | Type | Format | Description |
| --- | --- | --- | --- | --- |
| 200 | X-Pagination-Limit | integer |  | Limit specified for pagination |
| 200 | X-Pagination-Offset | integer |  | Offset specified for pagination |

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

url = '/futures/usdt/my_trades_timerange'
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
url="/futures/usdt/my_trades_timerange"
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
    "trade_id": "121234231",
    "create_time": 1514764800.123,
    "contract": "BTC_USDT",
    "order_id": "21893289839",
    "size": "100",
    "price": "100.123",
    "text": "t-123456",
    "fee": "0.01",
    "point_fee": "0",
    "role": "taker",
    "close_size": "0"
}
]
```

## Query position close history

`GET /futures/{settle}/position_close`

_Query position close history_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| side | query | string | false | Query side. long or shot |
| pnl | query | string | false | Query profit or loss |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | number(double) | Position close time |
| » contract | string | Futures contract |
| » side | string | Position side<br>\- `long`: Long position<br>\- `short`: Short position |
| » pnl | string | PnL |
| » pnl\_pnl | string | PNL - Position P/L |
| » pnl\_fund | string | PNL - Funding Fees |
| » pnl\_fee | string | PNL - Transaction Fees |
| » text | string | Source of close order. See `order.text` field for specific values |
| » max\_size | string | Max Trade Size |
| » accum\_size | string | Cumulative closed position volume |
| » first\_open\_time | integer(int64) | First Open Time |
| » long\_price | string | When side is 'long', it indicates the opening average price; when side is 'short', it indicates the closing average price |
| » short\_price | string | When side is 'long', it indicates the closing average price; when side is 'short', it indicates the opening average price |

#### Enumerated Values

| Property | Value |
| --- | --- |
| side | long |
| side | short |

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

url = '/futures/usdt/position_close'
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
url="/futures/usdt/position_close"
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
    "time": 1546487347,
    "pnl": "0.00013",
    "pnl_pnl": "0.00011",
    "pnl_fund": "0.00001",
    "pnl_fee": "0.00001",
    "side": "long",
    "contract": "BTC_USDT",
    "text": "web",
    "max_size": "100",
    "accum_size": "100",
    "first_open_time": 1546487347,
    "long_price": "2026.87",
    "short_price": "2544.4"
}
]
```

## Query liquidation history

`GET /futures/{settle}/liquidates`

_Query liquidation history_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| at | query | integer | false | Specify liquidation timestamp |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | integer(int64) | Liquidation time |
| » contract | string | Futures contract |
| » leverage | string | Position leverage. Not returned in public endpoints |
| » size | string | Position size |
| » margin | string | Position margin. Not returned in public endpoints |
| » entry\_price | string | Average entry price. Not returned in public endpoints |
| » liq\_price | string | Liquidation price. Not returned in public endpoints |
| » mark\_price | string | Mark price. Not returned in public endpoints |
| » order\_id | integer(int64) | Liquidation order ID. Not returned in public endpoints |
| » order\_price | string | Liquidation order price |
| » fill\_price | string | Liquidation order average taker price |
| » left | string | Liquidation order maker size |

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

url = '/futures/usdt/liquidates'
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
url="/futures/usdt/liquidates"
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
    "time": 1548654951,
    "contract": "BTC_USDT",
    "size": "600",
    "leverage": "25",
    "margin": "0.006705256878",
    "entry_price": "3536.123",
    "liq_price": "3421.54",
    "mark_price": "3420.27",
    "order_id": 317393847,
    "order_price": "3405",
    "fill_price": "3424",
    "left": "0"
}
]
```

## Query ADL auto-deleveraging order information

`GET /futures/{settle}/auto_deleverages`

_Query ADL auto-deleveraging order information_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| from | query | integer(int64) | false | Start timestamp |
| to | query | integer(int64) | false | Termination Timestamp |
| at | query | integer | false | Specify auto-deleveraging timestamp |

#### Detailed descriptions

**from**: Start timestamp

Specify start time, time format is Unix timestamp. If not specified, it defaults to (the data start time of the time range actually returned by to and limit)

**to**: Termination Timestamp

Specify the end time. If not specified, it defaults to the current time, and the time format is a Unix timestamp

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » time | integer(int64) | Automatic deleveraging time |
| » user | integer(int64) | User ID |
| » order\_id | integer(int64) | Order ID. Order IDs before 2023-02-20 are null |
| » contract | string | Futures contract |
| » leverage | string | leverage for isolated margin. 0 means cross margin. For leverage of cross margin, please refer to `cross_leverage_limit`. |
| » cross\_leverage\_limit | string | leverage for cross margin |
| » entry\_price | string | Average entry price |
| » fill\_price | string | Average fill price |
| » trade\_size | string | Trading size |
| » position\_size | string | Positions after auto-deleveraging |

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

url = '/futures/usdt/auto_deleverages'
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
url="/futures/usdt/auto_deleverages"
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
    "time": 1675841679,
    "contract": "ACH_USDT",
    "order_id": 73873128,
    "user": 1666,
    "cross_leverage_limit": "0",
    "leverage": "0",
    "entry_price": "2649.648633636364",
    "fill_price": "2790.8082",
    "position_size": "1",
    "trade_size": "-10"
}
]
```

## Countdown cancel orders

`POST /futures/{settle}/countdown_cancel_all`

_Countdown cancel orders_

Heartbeat detection for contract orders: When the user-set `timeout` time is reached, if neither the existing countdown is canceled nor a new countdown is set, the relevant contract orders will be automatically canceled.
This API can be called repeatedly to or cancel the countdown.
Usage example: Repeatedly call this API at 30-second intervals, setting the `timeout` to 30 (seconds) each time.
If this API is not called again within 30 seconds, all open orders on your specified `market` will be automatically canceled.
If the `timeout` is set to 0 within 30 seconds, the countdown timer will terminate, and the automatic order cancellation function will be disabled.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [CountdownCancelAllFuturesTask](https://www.gate.com/docs/developers/apiv4/en/#schemacountdowncancelallfuturestask) | true | none |
| » timeout | body | integer(int32) | true | Countdown time in seconds |
| » contract | body | string | false | Futures contract |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**» timeout**: Countdown time in seconds
At least 5 seconds, 0 means cancel countdown

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Countdown set successfully | [TriggerTime](https://www.gate.com/docs/developers/apiv4/en/#schematriggertime) |

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

url = '/futures/usdt/countdown_cancel_all'
query_param = ''
body='{"timeout":30,"contract":"BTC_USDT"}'
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
url="/futures/usdt/countdown_cancel_all"
query_param=""
body_param='{"timeout":30,"contract":"BTC_USDT"}'
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
"timeout": 30,
"contract": "BTC_USDT"
}
```

> Example responses

> 200 Response

```json
{
"triggerTime": "1660039145000"
}
```

## Query futures market trading fee rates

`GET /futures/{settle}/fee`

_Query futures market trading fee rates_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Futures contract, return related data only if specified |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_FuturesFee_

| Name | Type | Description |
| --- | --- | --- |
| » **additionalProperties** | object | The returned result is a map type, where the key represents the market and taker and maker fee rates |
| »» taker\_fee | string | Taker fee |
| »» maker\_fee | string | maker fee |

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

url = '/futures/usdt/fee'
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
url="/futures/usdt/fee"
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
"1INCH_USDT": {
    "taker_fee": "0.00025",
    "maker_fee": "-0.00010"
},
"AAVE_USDT": {
    "taker_fee": "0.00025",
    "maker_fee": "-0.00010"
}
}
```

## Cancel batch orders by specified ID list

`POST /futures/{settle}/batch_cancel_orders`

_Cancel batch orders by specified ID list_

Multiple different order IDs can be specified. A maximum of 20 records can be cancelled in one request

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | [CancelBatchFutureOrdersRequest](https://www.gate.com/docs/developers/apiv4/en/#schemacancelbatchfutureordersrequest) | true | none |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order cancellation operation completed | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » FutureCancelOrderResult | object | Order cancellation result |
| »» id | string | Order ID |
| »» user\_id | integer(int64) | User ID |
| »» succeeded | boolean | Whether cancellation succeeded |
| »» message | string | Error description when cancellation fails, empty if successful |

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

url = '/futures/usdt/batch_cancel_orders'
query_param = ''
body='["1","2","3"]'
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
url="/futures/usdt/batch_cancel_orders"
query_param=""
body_param='["1","2","3"]'
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
[
"1",
"2",
"3"
]
```

> Example responses

> 200 Response

```json
[
{
    "user_id": 111,
    "id": "123456",
    "succeeded": true,
    "message": ""
}
]
```

## Batch modify orders by specified IDs

`POST /futures/{settle}/batch_amend_orders`

_Batch modify orders by specified IDs_

Multiple different order IDs can be specified. A maximum of 10 orders can be modified in one request

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | array\[ [BatchAmendOrderReq](https://www.gate.com/docs/developers/apiv4/en/#schemabatchamendorderreq)\] | true | none |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Request execution completed | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Futures order details\] |
| » _None_ | object | Futures order details |
| »» succeeded | boolean | Request execution result |
| »» label | string | Error label, only exists if execution fails |
| »» detail | string | Error detail, only present if execution failed and details need to be given |
| »» id | integer(int64) | Futures order ID |
| »» user | integer | User ID |
| »» create\_time | number(double) | Creation time of order |
| »» finish\_time | number(double) | Order finished time. Not returned if order is open |
| »» finish\_as | string | How the order was finished:<br>\- filled: all filled<br>\- cancelled: manually cancelled<br>\- liquidated: cancelled because of liquidation<br>\- ioc: time in force is `IOC`, finish immediately<br>\- auto\_deleveraged: finished by ADL<br>\- reduce\_only: cancelled because of increasing position while `reduce-only` set<br>\- position\_closed: cancelled because the position was closed<br>\- reduce\_out: only reduce positions by excluding hard-to-fill orders<br>\- stp: cancelled because self trade prevention |
| »» status | string | Order status<br>\- `open`: Pending<br>\- `finished`: Completed |
| »» contract | string | Futures contract |
| »» size | string | Required. Trading quantity. Positive for buy, negative for sell. Set to 0 for close position orders. |
| »» iceberg | string | Display size for iceberg orders. 0 for non-iceberg orders. Note that hidden portions are charged taker fees. |
| »» price | string | Order price. Price of 0 with `tif` set to `ioc` represents a market order. |
| »» is\_close | boolean | Is the order to close position |
| »» is\_reduce\_only | boolean | Is the order reduce-only |
| »» is\_liq | boolean | Is the order for liquidation |
| »» tif | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none |
| »» left | string | Unfilled quantity |
| »» fill\_price | string | Fill price |
| »» text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- web: from web<br>\- api: from API<br>\- app: from mobile phones<br>\- auto\_deleveraging: from ADL<br>\- liquidation: from liquidation<br>\- insurance: from insurance |
| »» tkfr | string | Taker fee |
| »» mkfr | string | Maker fee |
| »» refu | integer | Referrer user ID |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» market\_order\_slip\_ratio | string | The maximum slippage ratio |

#### Enumerated Values

| Property | Value |
| --- | --- |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidated |
| finish\_as | ioc |
| finish\_as | auto\_deleveraged |
| finish\_as | reduce\_only |
| finish\_as | position\_closed |
| finish\_as | reduce\_out |
| finish\_as | stp |
| status | open |
| status | finished |
| tif | gtc |
| tif | ioc |
| tif | poc |
| tif | fok |
| stp\_act | co |
| stp\_act | cn |
| stp\_act | cb |
| stp\_act | - |

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

url = '/futures/usdt/batch_amend_orders'
query_param = ''
body='[{"order_id":121212,"amend_text":"batch amend text","size":"100","price":"54321"}]'
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
url="/futures/usdt/batch_amend_orders"
query_param=""
body_param='[{"order_id":121212,"amend_text":"batch amend text","size":"100","price":"54321"}]'
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
[
{
    "order_id": 121212,
    "amend_text": "batch amend text",
    "size": "100",
    "price": "54321"
}
]
```

> Example responses

> 200 Response

```json
[
{
    "succeeded": true,
    "id": 15675394,
    "user": 100000,
    "contract": "BTC_USDT",
    "create_time": 1546569968,
    "size": "6024",
    "iceberg": "0",
    "left": "6024",
    "price": "3765",
    "fill_price": "0",
    "mkfr": "-0.00025",
    "tkfr": "0.00075",
    "tif": "gtc",
    "refu": 0,
    "is_reduce_only": false,
    "is_close": false,
    "is_liq": false,
    "text": "t-my-custom-id",
    "status": "finished",
    "finish_time": 1514764900,
    "finish_as": "cancelled",
    "stp_id": 0,
    "stp_act": "-",
    "amend_text": "-",
    "market_order_slip_ratio": "0.03"
}
]
```

## Query risk limit table by table\_id

`GET /futures/{settle}/risk_limit_table`

_Query risk limit table by table\_id_

Just pass table\_id

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| table\_id | query | string | true | Risk limit table ID |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Information for each tier of the gradient risk limit table |
| »» tier | integer(int) | Tier |
| »» risk\_limit | string | Position risk limit |
| »» initial\_rate | string | Initial margin rate |
| »» maintenance\_rate | string | The maintenance margin rate of the first tier of risk limit sheet |
| »» leverage\_max | string | Maximum leverage |
| »» deduction | string | Maintenance margin quick calculation deduction amount |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/futures/usdt/risk_limit_table'
query_param = 'table_id=CYBER_USDT_20241122'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/futures/usdt/risk_limit_table?table_id=CYBER_USDT_20241122 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "tier": 1,
    "risk_limit": "10000",
    "initial_rate": "0.025",
    "maintenance_rate": "0.015",
    "leverage_max": "40",
    "deduction": "0"
},
{
    "tier": 2,
    "risk_limit": "30000",
    "initial_rate": "0.03333",
    "maintenance_rate": "0.02",
    "leverage_max": "30",
    "deduction": "50"
},
{
    "tier": 3,
    "risk_limit": "50000",
    "initial_rate": "0.04545",
    "maintenance_rate": "0.03",
    "leverage_max": "22",
    "deduction": "350"
},
{
    "tier": 4,
    "risk_limit": "70000",
    "initial_rate": "0.05555",
    "maintenance_rate": "0.04",
    "leverage_max": "18",
    "deduction": "850"
},
{
    "tier": 5,
    "risk_limit": "100000",
    "initial_rate": "0.1",
    "maintenance_rate": "0.085",
    "leverage_max": "10",
    "deduction": "4000"
},
{
    "tier": 6,
    "risk_limit": "150000",
    "initial_rate": "0.333",
    "maintenance_rate": "0.3",
    "leverage_max": "3",
    "deduction": "25500"
},
{
    "tier": 7,
    "risk_limit": "200000",
    "initial_rate": "0.5",
    "maintenance_rate": "0.45",
    "leverage_max": "2",
    "deduction": "48000"
},
{
    "tier": 8,
    "risk_limit": "300000",
    "initial_rate": "1",
    "maintenance_rate": "0.95",
    "leverage_max": "1",
    "deduction": "148000"
}
]
```

## Level-based BBO Contract Order Placement

`POST /futures/{settle}/bbo_orders`

_Level-based BBO Contract Order Placement_

Compared to the futures trading order placement interface (futures/{settle}/orders), it adds the `level` and `direction` parameters.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | [FuturesBBOOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesbboorder) | true | none |
| » contract | body | string | true | Futures contract |
| » size | body | integer(int64) | true | Required. Trading quantity. Positive for buy, negative for sell. Set to 0 for close position orders. |
| » direction | body | string | true | Direction: 'sell' fetches the bid side, 'buy' fetches the ask side. |
| » iceberg | body | integer(int64) | false | Display size for iceberg orders. 0 for non-iceberg orders. Note that hidden portions are charged taker fees. |
| » level | body | integer(int64) | true | Level: maximum 20 levels |
| » close | body | boolean | false | Set as `true` to close the position, with `size` set to 0 |
| » reduce\_only | body | boolean | false | Set as `true` to be reduce-only order |
| » tif | body | string | false | Time in force |
| » text | body | string | false | Order Custom Information: Users can set custom IDs via this field. Custom fields must meet the following conditions: |
| » auto\_size | body | string | false | Set side to close dual-mode position. `close_long` closes the long side; while `close_short` the short one. Note `size` also needs to be set to 0 |
| » stp\_act | body | string | false | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies |
| » pid | body | integer(int64) | false | Position ID |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**» tif**: Time in force

- gtc: GoodTillCancelled
- ioc: ImmediateOrCancelled, taker only
- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee
- fok: FillOrKill, fill either completely or none

**» text**: Order Custom Information: Users can set custom IDs via this field. Custom fields must meet the following conditions:

1. Must start with `t-`
2. Excluding `t-`, length cannot exceed 28 bytes
3. Content can only contain numbers, letters, underscores (\_), hyphens (-), or dots (.)

In addition to user custom information, the following are internal reserved fields identifying order sources:

- web: Web
- api: API Call
- app: Mobile App
- auto\_deleveraging: Auto-Deleveraging
- liquidation: Forced Liquidation of Legacy Classic Mode Positions
- liq-xxx: a. Forced liquidation of New Classic Mode positions, including isolated margin, single-direction cross margin, and non-hedged dual-direction cross margin positions. b. Forced liquidation of isolated margin positions in Unified Account Single-Currency Margin Mode
- hedge-liq-xxx: Forced liquidation of hedged portions in New Classic Mode dual-direction cross margin (simultaneous closing of long and short positions)
- pm\_liquidate: Forced liquidation in Unified Account Cross-Currency Margin Mode
- comb\_margin\_liquidate: Forced liquidation in Unified Account Portfolio Margin Mode
- scm\_liquidate: Forced liquidation of positions in Unified Account Single-Currency Margin Mode
- insurance: Insurance

**» stp\_act**: Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies

1. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.
2. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.
3. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'

- cn: Cancel newest, cancel new orders and keep old ones
- co: Cancel oldest, cancel old orders and keep new ones
- cb: Cancel both, both old and new orders will be cancelled

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » tif | gtc |
| » tif | ioc |
| » tif | poc |
| » tif | fok |
| » auto\_size | close\_long |
| » auto\_size | close\_short |
| » stp\_act | co |
| » stp\_act | cn |
| » stp\_act | cb |
| » stp\_act | - |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Order details | [FuturesOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesorder) |

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

url = '/futures/usdt/bbo_orders'
query_param = ''
body='{"contract":"PI_USDT","level":8,"direction":"sell","size":1}'
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
url="/futures/usdt/bbo_orders"
query_param=""
body_param='{"contract":"PI_USDT","level":8,"direction":"sell","size":1}'
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
"contract": "PI_USDT",
"level": 8,
"direction": "sell",
"size": 1
}
```

> Example responses

> 201 Response

```json
{
"id": 15675394,
"user": 100000,
"contract": "BTC_USDT",
"create_time": 1546569968,
"size": "6024",
"iceberg": "0",
"left": "6024",
"price": "3765",
"fill_price": "0",
"mkfr": "-0.00025",
"tkfr": "0.00075",
"tif": "gtc",
"refu": 0,
"is_reduce_only": false,
"is_close": false,
"is_liq": false,
"text": "t-my-custom-id",
"status": "finished",
"finish_time": 1514764900,
"finish_as": "cancelled",
"stp_id": 0,
"stp_act": "-",
"amend_text": "-",
"order_value": "64112.2099000000005",
"trade_value": "64112.2099000000005",
"market_order_slip_ratio": "0.03",
"pos_margin_mode": "isolated"
}
```

## Create trail order

`POST /futures/{settle}/autoorder/v1/trail/create`

_Create trail order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [CreateTrailOrder](https://www.gate.com/docs/developers/apiv4/en/#schemacreatetrailorder) | true | none |
| » contract | body | string | true | Contract name |
| » amount | body | string | true | Trading quantity in contracts, positive for buy, negative for sell |
| » activation\_price | body | string | false | Activation price, 0 means trigger immediately |
| » is\_gte | body | boolean | false | true: activate when market price >= activation price, false: <= activation price |
| » price\_type | body | integer(int32) | false | Activation price type: 1-latest price, 2-index price, 3-mark price |
| » price\_offset | body | string | false | Callback ratio or price distance, e.g., `0.1` or `0.1%` |
| » reduce\_only | body | boolean | false | Whether reduce only |
| » position\_related | body | boolean | false | Whether bound to a position (if position\_related = true (position-related), then reduce\_only must also be true) |
| » text | body | string | false | Order custom information, optional field. Used to identify the order source or set a user-defined ID. |
| » pos\_margin\_mode | body | string | false | Position margin mode: isolated/cross |
| » position\_mode | body | string | false | Position mode: single, dual, and dual\_plus |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**» text**: Order custom information, optional field. Used to identify the order source or set a user-defined ID.

If non-empty, it must meet one of the following rules:

1. Internal Reserved Fields (identifying order source):

- `apiv4`: API call

2. User-defined Fields (setting custom ID):

- Must start with `t-`
- The content after `t-` must not exceed 28 bytes in length
- Can only contain: numbers, letters, underscores (\_), hyphens (-), or dots (.)
- Examples: `t-my-order-001`, `t-trail_2024.01`

Note: User-defined fields must not conflict with internal reserved fields.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » price\_type | 1 |
| » price\_type | 2 |
| » price\_type | 3 |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Created successfully | [CreateTrailOrderResponse](https://www.gate.com/docs/developers/apiv4/en/#schemacreatetrailorderresponse) |

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

url = '/futures/usdt/autoorder/v1/trail/create'
query_param = ''
body='{"contract":"BTC_USDT","amount":"10","activation_price":"50000","is_gte":true,"price_type":1,"price_offset":"0.1%","reduce_only":false,"text":"apiv4"}'
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
url="/futures/usdt/autoorder/v1/trail/create"
query_param=""
body_param='{"contract":"BTC_USDT","amount":"10","activation_price":"50000","is_gte":true,"price_type":1,"price_offset":"0.1%","reduce_only":false,"text":"apiv4"}'
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
"contract": "BTC_USDT",
"amount": "10",
"activation_price": "50000",
"is_gte": true,
"price_type": 1,
"price_offset": "0.1%",
"reduce_only": false,
"text": "apiv4"
}
```

> Example responses

> 201 Response

```json
{
"code": 0,
"message": "ok",
"data": {
    "id": "63648"
},
"timestamp": 1769583885680
}
```

## Terminate trail order

`POST /futures/{settle}/autoorder/v1/trail/stop`

_Terminate trail order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [StopTrailOrder](https://www.gate.com/docs/developers/apiv4/en/#schemastoptrailorder) | true | none |
| » id | body | integer(int64) | false | Order ID, if ID is specified, text is not needed |
| » text | body | string | false | Custom text, if ID is not specified, terminate based on user\_id and text |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Termination successful | Inline |

### Response Schema

Status Code **200**

_TrailOrderResponse_

| Name | Type | Description |
| --- | --- | --- |
| » order | object | Trail order details |
| »» id | integer(int64) | Order ID |
| »» user\_id | integer(int64) | User ID |
| »» user | integer(int64) | User ID |
| »» contract | string | Contract name |
| »» settle | string | Settle currency |
| »» amount | string | Trading quantity in contracts, positive for buy, negative for sell |
| »» is\_gte | boolean | true: activate when market price >= activation price, false: <= activation price |
| »» activation\_price | string | Activation price, 0 means trigger immediately |
| »» price\_type | integer(int32) | Activation price type: 0-unknown, 1-latest price, 2-index price, 3-mark price |
| »» price\_offset | string | Callback ratio or price distance, e.g., `0.1` or `0.1%` |
| »» text | string | Custom field |
| »» reduce\_only | boolean | Reduce Position Only |
| »» position\_related | boolean | Whether bound to position |
| »» created\_at | integer(int64) | Created time |
| »» activated\_at | integer(int64) | Activation time |
| »» finished\_at | integer(int64) | End time |
| »» create\_time | integer(int64) | Created time |
| »» active\_time | integer(int64) | Activation time |
| »» finish\_time | integer(int64) | End time |
| »» reason | string | End reason |
| »» suborder\_text | string | Sub-order text field |
| »» is\_dual\_mode | boolean | Whether dual position mode when creating order |
| »» trigger\_price | string | Trigger price |
| »» suborder\_id | integer(int64) | Sub-order ID |
| »» side\_label | string | Order direction label: long/short/open long/open short/close long/close short |
| »» original\_status | integer(int32) | Order status |
| »» status | string | Simplified order status: open/finished |
| »» position\_side\_output | string | Same as side\_label, client requires consistency with other order types |
| »» updated\_at | integer(int64) | Update time |
| »» extremum\_price | string | Extremum price |
| »» status\_code | string | Status code value |
| »» created\_at\_precise | string | Creation time (high precision, seconds.microseconds format) |
| »» finished\_at\_precise | string | End time (high precision, seconds.microseconds format) |
| »» activated\_at\_precise | string | Activation time (high precision, seconds.microseconds format) |
| »» status\_label | string | Status internationalization label (translated status text) |
| »» pos\_margin\_mode | string | Position margin mode: isolated/cross |
| »» position\_mode | string | Position mode: single, dual, and dual\_plus |
| »» error\_label | string | Error label |
| »» leverage | string | leverage |

#### Enumerated Values

| Property | Value |
| --- | --- |
| price\_type | 0 |
| price\_type | 1 |
| price\_type | 2 |
| price\_type | 3 |
| status | open |
| status | finished |

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

url = '/futures/usdt/autoorder/v1/trail/stop'
query_param = ''
body='{"id":123456789}'
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
url="/futures/usdt/autoorder/v1/trail/stop"
query_param=""
body_param='{"id":123456789}'
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
"id": 123456789
}
```

> Example responses

> 200 Response

```json
{
"id": "63585",
"user_id": "2124438083",
"contract": "BTC_USDT",
"settle": "usdt",
"amount": "10",
"is_gte": false,
"activation_price": "0",
"price_type": 1,
"price_offset": "5%",
"text": "apiv4",
"reduce_only": false,
"position_related": false,
"created_at": "1769569837",
"activated_at": "1769569837",
"finished_at": "1769578529",
"reason": "",
"suborder_text": "apiv4-auto-trail-o-1d29",
"is_dual_mode": false,
"trigger_price": "91047.4",
"suborder_id": "94294117233225616",
"side_label": "买入",
"original_status": 4,
"status": "finished",
"user": "2124438083",
"create_time": "1769569837",
"active_time": "1769569837",
"finish_time": "1769578529",
"position_side_output": "买入",
"updated_at": "1769578529",
"extremum_price": "86711.9",
"status_code": "success",
"created_at_precise": "1769569837778000",
"finished_at_precise": "1769578529853294",
"activated_at_precise": "1769569837976010",
"status_label": "已完成",
"pos_margin_mode": "",
"position_mode": "single",
"error_label": "",
"leverage": ""
}
```

## Batch terminate trail orders

`POST /futures/{settle}/autoorder/v1/trail/stop_all`

_Batch terminate trail orders_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [StopAllTrailOrders](https://www.gate.com/docs/developers/apiv4/en/#schemastopalltrailorders) | true | none |
| » contract | body | string | false | Contract name |
| » related\_position | body | integer(int32) | false | Associated position, if provided, only cancel orders associated with this position, 1-long, 2-short |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » related\_position | 1 |
| » related\_position | 2 |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Termination successful | Inline |

### Response Schema

Status Code **200**

_TrailOrderListResponse_

| Name | Type | Description |
| --- | --- | --- |
| » orders | array | none |
| »» TrailOrder | object | Trail order details |
| »»» id | integer(int64) | Order ID |
| »»» user\_id | integer(int64) | User ID |
| »»» user | integer(int64) | User ID |
| »»» contract | string | Contract name |
| »»» settle | string | Settle currency |
| »»» amount | string | Trading quantity in contracts, positive for buy, negative for sell |
| »»» is\_gte | boolean | true: activate when market price >= activation price, false: <= activation price |
| »»» activation\_price | string | Activation price, 0 means trigger immediately |
| »»» price\_type | integer(int32) | Activation price type: 0-unknown, 1-latest price, 2-index price, 3-mark price |
| »»» price\_offset | string | Callback ratio or price distance, e.g., `0.1` or `0.1%` |
| »»» text | string | Custom field |
| »»» reduce\_only | boolean | Reduce Position Only |
| »»» position\_related | boolean | Whether bound to position |
| »»» created\_at | integer(int64) | Created time |
| »»» activated\_at | integer(int64) | Activation time |
| »»» finished\_at | integer(int64) | End time |
| »»» create\_time | integer(int64) | Created time |
| »»» active\_time | integer(int64) | Activation time |
| »»» finish\_time | integer(int64) | End time |
| »»» reason | string | End reason |
| »»» suborder\_text | string | Sub-order text field |
| »»» is\_dual\_mode | boolean | Whether dual position mode when creating order |
| »»» trigger\_price | string | Trigger price |
| »»» suborder\_id | integer(int64) | Sub-order ID |
| »»» side\_label | string | Order direction label: long/short/open long/open short/close long/close short |
| »»» original\_status | integer(int32) | Order status |
| »»» status | string | Simplified order status: open/finished |
| »»» position\_side\_output | string | Same as side\_label, client requires consistency with other order types |
| »»» updated\_at | integer(int64) | Update time |
| »»» extremum\_price | string | Extremum price |
| »»» status\_code | string | Status code value |
| »»» created\_at\_precise | string | Creation time (high precision, seconds.microseconds format) |
| »»» finished\_at\_precise | string | End time (high precision, seconds.microseconds format) |
| »»» activated\_at\_precise | string | Activation time (high precision, seconds.microseconds format) |
| »»» status\_label | string | Status internationalization label (translated status text) |
| »»» pos\_margin\_mode | string | Position margin mode: isolated/cross |
| »»» position\_mode | string | Position mode: single, dual, and dual\_plus |
| »»» error\_label | string | Error label |
| »»» leverage | string | leverage |

#### Enumerated Values

| Property | Value |
| --- | --- |
| price\_type | 0 |
| price\_type | 1 |
| price\_type | 2 |
| price\_type | 3 |
| status | open |
| status | finished |

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

url = '/futures/usdt/autoorder/v1/trail/stop_all'
query_param = ''
body='{"contract":"BTC_USDT"}'
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
url="/futures/usdt/autoorder/v1/trail/stop_all"
query_param=""
body_param='{"contract":"BTC_USDT"}'
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
"contract": "BTC_USDT"
}
```

> Example responses

> 200 Response

```json
{
"orders": [
    {
      "id": "123456789",
      "user_id": "100000",
      "user": "100000",
      "contract": "BTC_USDT",
      "settle": "usdt",
      "amount": "10",
      "is_gte": true,
      "activation_price": "50000",
      "price_type": 1,
      "price_offset": "0.1%",
      "text": "t-my-trail-order-1",
      "reduce_only": false,
      "position_related": false,
      "created_at": "1546569968",
      "create_time": "1546569968",
      "activated_at": "1546570000",
      "active_time": "1546570000",
      "finished_at": "0",
      "finish_time": "0",
      "reason": "",
      "suborder_text": "",
      "is_dual_mode": false,
      "trigger_price": "50000",
      "suborder_id": "0",
      "side_label": "开多",
      "position_side_output": "开多",
      "original_status": 1,
      "status": "open",
      "updated_at": "1546569968",
      "extremum_price": "50100",
      "status_code": "pending",
      "created_at_precise": "1546569968.123456",
      "finished_at_precise": "",
      "activated_at_precise": "",
      "status_label": "待激活",
      "pos_margin_mode": "isolated",
      "position_mode": "single",
      "error_label": "",
      "leverage": "10"
    },
    {
      "id": "123456790",
      "user_id": "100000",
      "user": "100000",
      "contract": "ETH_USDT",
      "settle": "usdt",
      "amount": "-5",
      "is_gte": false,
      "activation_price": "3000",
      "price_type": 2,
      "price_offset": "0.2%",
      "text": "t-my-trail-order-2",
      "reduce_only": true,
      "position_related": true,
      "created_at": "1546569970",
      "create_time": "1546569970",
      "activated_at": "1546570100",
      "active_time": "1546570100",
      "finished_at": "1546571000",
      "finish_time": "1546571000",
      "reason": "success",
      "suborder_text": "t-suborder-1",
      "is_dual_mode": true,
      "trigger_price": "3000",
      "suborder_id": "987654321",
      "side_label": "平空",
      "position_side_output": "平空",
      "original_status": 4,
      "status": "finished",
      "updated_at": "1546571000",
      "extremum_price": "2990",
      "status_code": "success",
      "created_at_precise": "1546569970.654321",
      "finished_at_precise": "1546571000.123456",
      "activated_at_precise": "1546570100.789012",
      "status_label": "完成全部委托量",
      "pos_margin_mode": "cross",
      "position_mode": "dual",
      "error_label": "",
      "leverage": "20"
    }
]
}
```

## Get trail order list

`GET /futures/{settle}/autoorder/v1/trail/list`

_Get trail order list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| contract | query | string | false | Contract name |
| is\_finished | query | boolean | false | Whether historical order |
| start\_at | query | integer(int64) | false | Start time of time range |
| end\_at | query | integer(int64) | false | End time of time range |
| page\_num | query | integer(int32) | false | Page number, starting from 1 |
| page\_size | query | integer(int32) | false | Number of items per page |
| sort\_by | query | integer(int32) | false | Common sort field, 1-creation time, 2-end time |
| hide\_cancel | query | boolean | false | Hide cancelled orders |
| related\_position | query | integer(int32) | false | Associated position, if provided, only return orders associated with this position, 1-long, 2-short |
| sort\_by\_trigger | query | boolean | false | Sort by trigger price and activation price, easy to trigger or activate first, only for current orders associated with positions |
| reduce\_only | query | integer(int32) | false | Whether reduce only, 1-yes, 2-no |
| side | query | integer(int32) | false | Direction, 1-long position, 2-short position |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |
| sort\_by | 1 |
| sort\_by | 2 |
| related\_position | 1 |
| related\_position | 2 |
| reduce\_only | 1 |
| reduce\_only | 2 |
| side | 1 |
| side | 2 |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

_TrailOrderListResponse_

| Name | Type | Description |
| --- | --- | --- |
| » orders | array | none |
| »» TrailOrder | object | Trail order details |
| »»» id | integer(int64) | Order ID |
| »»» user\_id | integer(int64) | User ID |
| »»» user | integer(int64) | User ID |
| »»» contract | string | Contract name |
| »»» settle | string | Settle currency |
| »»» amount | string | Trading quantity in contracts, positive for buy, negative for sell |
| »»» is\_gte | boolean | true: activate when market price >= activation price, false: <= activation price |
| »»» activation\_price | string | Activation price, 0 means trigger immediately |
| »»» price\_type | integer(int32) | Activation price type: 0-unknown, 1-latest price, 2-index price, 3-mark price |
| »»» price\_offset | string | Callback ratio or price distance, e.g., `0.1` or `0.1%` |
| »»» text | string | Custom field |
| »»» reduce\_only | boolean | Reduce Position Only |
| »»» position\_related | boolean | Whether bound to position |
| »»» created\_at | integer(int64) | Created time |
| »»» activated\_at | integer(int64) | Activation time |
| »»» finished\_at | integer(int64) | End time |
| »»» create\_time | integer(int64) | Created time |
| »»» active\_time | integer(int64) | Activation time |
| »»» finish\_time | integer(int64) | End time |
| »»» reason | string | End reason |
| »»» suborder\_text | string | Sub-order text field |
| »»» is\_dual\_mode | boolean | Whether dual position mode when creating order |
| »»» trigger\_price | string | Trigger price |
| »»» suborder\_id | integer(int64) | Sub-order ID |
| »»» side\_label | string | Order direction label: long/short/open long/open short/close long/close short |
| »»» original\_status | integer(int32) | Order status |
| »»» status | string | Simplified order status: open/finished |
| »»» position\_side\_output | string | Same as side\_label, client requires consistency with other order types |
| »»» updated\_at | integer(int64) | Update time |
| »»» extremum\_price | string | Extremum price |
| »»» status\_code | string | Status code value |
| »»» created\_at\_precise | string | Creation time (high precision, seconds.microseconds format) |
| »»» finished\_at\_precise | string | End time (high precision, seconds.microseconds format) |
| »»» activated\_at\_precise | string | Activation time (high precision, seconds.microseconds format) |
| »»» status\_label | string | Status internationalization label (translated status text) |
| »»» pos\_margin\_mode | string | Position margin mode: isolated/cross |
| »»» position\_mode | string | Position mode: single, dual, and dual\_plus |
| »»» error\_label | string | Error label |
| »»» leverage | string | leverage |

#### Enumerated Values

| Property | Value |
| --- | --- |
| price\_type | 0 |
| price\_type | 1 |
| price\_type | 2 |
| price\_type | 3 |
| status | open |
| status | finished |

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

url = '/futures/usdt/autoorder/v1/trail/list'
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
url="/futures/usdt/autoorder/v1/trail/list"
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
"orders": [
    {
      "id": "123456789",
      "user_id": "100000",
      "user": "100000",
      "contract": "BTC_USDT",
      "settle": "usdt",
      "amount": "10",
      "is_gte": true,
      "activation_price": "50000",
      "price_type": 1,
      "price_offset": "0.1%",
      "text": "t-my-trail-order-1",
      "reduce_only": false,
      "position_related": false,
      "created_at": "1546569968",
      "create_time": "1546569968",
      "activated_at": "1546570000",
      "active_time": "1546570000",
      "finished_at": "0",
      "finish_time": "0",
      "reason": "",
      "suborder_text": "",
      "is_dual_mode": false,
      "trigger_price": "50000",
      "suborder_id": "0",
      "side_label": "开多",
      "position_side_output": "开多",
      "original_status": 1,
      "status": "open",
      "updated_at": "1546569968",
      "extremum_price": "50100",
      "status_code": "pending",
      "created_at_precise": "1546569968.123456",
      "finished_at_precise": "",
      "activated_at_precise": "",
      "status_label": "待激活",
      "pos_margin_mode": "isolated",
      "position_mode": "single",
      "error_label": "",
      "leverage": "10"
    },
    {
      "id": "123456790",
      "user_id": "100000",
      "user": "100000",
      "contract": "ETH_USDT",
      "settle": "usdt",
      "amount": "-5",
      "is_gte": false,
      "activation_price": "3000",
      "price_type": 2,
      "price_offset": "0.2%",
      "text": "t-my-trail-order-2",
      "reduce_only": true,
      "position_related": true,
      "created_at": "1546569970",
      "create_time": "1546569970",
      "activated_at": "1546570100",
      "active_time": "1546570100",
      "finished_at": "1546571000",
      "finish_time": "1546571000",
      "reason": "success",
      "suborder_text": "t-suborder-1",
      "is_dual_mode": true,
      "trigger_price": "3000",
      "suborder_id": "987654321",
      "side_label": "平空",
      "position_side_output": "平空",
      "original_status": 4,
      "status": "finished",
      "updated_at": "1546571000",
      "extremum_price": "2990",
      "status_code": "success",
      "created_at_precise": "1546569970.654321",
      "finished_at_precise": "1546571000.123456",
      "activated_at_precise": "1546570100.789012",
      "status_label": "完成全部委托量",
      "pos_margin_mode": "cross",
      "position_mode": "dual",
      "error_label": "",
      "leverage": "20"
    }
]
}
```

## Get trail order details

`GET /futures/{settle}/autoorder/v1/trail/detail`

_Get trail order details_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| id | query | integer(int64) | true | Order ID |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | [TrailOrderDetailResponse](https://www.gate.com/docs/developers/apiv4/en/#schematrailorderdetailresponse) |

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

url = '/futures/usdt/autoorder/v1/trail/detail'
query_param = 'id=0'
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
url="/futures/usdt/autoorder/v1/trail/detail"
query_param="id=0"
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
"code": 0,
"message": "ok",
"data": {
    "order": {
      "id": "63585",
      "user_id": "2124438083",
      "contract": "BTC_USDT",
      "settle": "usdt",
      "amount": "10",
      "is_gte": false,
      "activation_price": "0",
      "price_type": 1,
      "price_offset": "5%",
      "text": "apiv4",
      "reduce_only": false,
      "position_related": false,
      "created_at": "1769569837",
      "activated_at": "1769569837",
      "finished_at": "1769578529",
      "reason": "",
      "suborder_text": "apiv4-auto-trail-o-1d29",
      "is_dual_mode": false,
      "trigger_price": "91047.4",
      "suborder_id": "94294117233225616",
      "side_label": "买入",
      "original_status": 4,
      "status": "finished",
      "user": "2124438083",
      "create_time": "1769569837",
      "active_time": "1769569837",
      "finish_time": "1769578529",
      "position_side_output": "买入",
      "updated_at": "1769578529",
      "extremum_price": "86711.9",
      "status_code": "success",
      "created_at_precise": "1769569837778000",
      "finished_at_precise": "1769578529853294",
      "activated_at_precise": "1769569837976010",
      "status_label": "已完成",
      "pos_margin_mode": "",
      "position_mode": "single",
      "error_label": "",
      "leverage": ""
    }
},
"timestamp": 1769584936814
}
```

## Update trail order

`POST /futures/{settle}/autoorder/v1/trail/update`

_Update trail order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [UpdateTrailOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaupdatetrailorder) | true | none |
| » id | body | integer(int64) | true | Order ID |
| » amount | body | string | false | Total trading quantity in contracts, positive for buy, negative for sell, 0 means no modification |
| » activation\_price | body | string | false | Activation price, 0 means trigger immediately, empty means no modification |
| » is\_gte\_str | body | string | false | true: activate when market price >= activation price, false: <= activation price, empty means no modification |
| » price\_type | body | integer(int32) | false | Activation price type, not provided or 0 means no modification, 1-latest price, 2-index price, 3-mark price |
| » price\_offset | body | string | false | Callback ratio or price distance, e.g., `0.1` or `0.1%`; empty means no modification |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » price\_type | 0 |
| » price\_type | 1 |
| » price\_type | 2 |
| » price\_type | 3 |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Updated successfully | Inline |

### Response Schema

Status Code **200**

_TrailOrderResponse_

| Name | Type | Description |
| --- | --- | --- |
| » order | object | Trail order details |
| »» id | integer(int64) | Order ID |
| »» user\_id | integer(int64) | User ID |
| »» user | integer(int64) | User ID |
| »» contract | string | Contract name |
| »» settle | string | Settle currency |
| »» amount | string | Trading quantity in contracts, positive for buy, negative for sell |
| »» is\_gte | boolean | true: activate when market price >= activation price, false: <= activation price |
| »» activation\_price | string | Activation price, 0 means trigger immediately |
| »» price\_type | integer(int32) | Activation price type: 0-unknown, 1-latest price, 2-index price, 3-mark price |
| »» price\_offset | string | Callback ratio or price distance, e.g., `0.1` or `0.1%` |
| »» text | string | Custom field |
| »» reduce\_only | boolean | Reduce Position Only |
| »» position\_related | boolean | Whether bound to position |
| »» created\_at | integer(int64) | Created time |
| »» activated\_at | integer(int64) | Activation time |
| »» finished\_at | integer(int64) | End time |
| »» create\_time | integer(int64) | Created time |
| »» active\_time | integer(int64) | Activation time |
| »» finish\_time | integer(int64) | End time |
| »» reason | string | End reason |
| »» suborder\_text | string | Sub-order text field |
| »» is\_dual\_mode | boolean | Whether dual position mode when creating order |
| »» trigger\_price | string | Trigger price |
| »» suborder\_id | integer(int64) | Sub-order ID |
| »» side\_label | string | Order direction label: long/short/open long/open short/close long/close short |
| »» original\_status | integer(int32) | Order status |
| »» status | string | Simplified order status: open/finished |
| »» position\_side\_output | string | Same as side\_label, client requires consistency with other order types |
| »» updated\_at | integer(int64) | Update time |
| »» extremum\_price | string | Extremum price |
| »» status\_code | string | Status code value |
| »» created\_at\_precise | string | Creation time (high precision, seconds.microseconds format) |
| »» finished\_at\_precise | string | End time (high precision, seconds.microseconds format) |
| »» activated\_at\_precise | string | Activation time (high precision, seconds.microseconds format) |
| »» status\_label | string | Status internationalization label (translated status text) |
| »» pos\_margin\_mode | string | Position margin mode: isolated/cross |
| »» position\_mode | string | Position mode: single, dual, and dual\_plus |
| »» error\_label | string | Error label |
| »» leverage | string | leverage |

#### Enumerated Values

| Property | Value |
| --- | --- |
| price\_type | 0 |
| price\_type | 1 |
| price\_type | 2 |
| price\_type | 3 |
| status | open |
| status | finished |

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

url = '/futures/usdt/autoorder/v1/trail/update'
query_param = ''
body='{"id":123456789,"amount":"20","activation_price":"51000","price_offset":"0.2%"}'
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
url="/futures/usdt/autoorder/v1/trail/update"
query_param=""
body_param='{"id":123456789,"amount":"20","activation_price":"51000","price_offset":"0.2%"}'
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
"id": 123456789,
"amount": "20",
"activation_price": "51000",
"price_offset": "0.2%"
}
```

> Example responses

> 200 Response

```json
{
"id": "63585",
"user_id": "2124438083",
"contract": "BTC_USDT",
"settle": "usdt",
"amount": "10",
"is_gte": false,
"activation_price": "0",
"price_type": 1,
"price_offset": "5%",
"text": "apiv4",
"reduce_only": false,
"position_related": false,
"created_at": "1769569837",
"activated_at": "1769569837",
"finished_at": "1769578529",
"reason": "",
"suborder_text": "apiv4-auto-trail-o-1d29",
"is_dual_mode": false,
"trigger_price": "91047.4",
"suborder_id": "94294117233225616",
"side_label": "买入",
"original_status": 4,
"status": "finished",
"user": "2124438083",
"create_time": "1769569837",
"active_time": "1769569837",
"finish_time": "1769578529",
"position_side_output": "买入",
"updated_at": "1769578529",
"extremum_price": "86711.9",
"status_code": "success",
"created_at_precise": "1769569837778000",
"finished_at_precise": "1769578529853294",
"activated_at_precise": "1769569837976010",
"status_label": "已完成",
"pos_margin_mode": "",
"position_mode": "single",
"error_label": "",
"leverage": ""
}
```

## Get trail order user modification records

`GET /futures/{settle}/autoorder/v1/trail/change_log`

_Get trail order user modification records_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| id | query | integer(int64) | true | Order ID |
| page\_num | query | integer(int32) | false | Page number, starting from 1 |
| page\_size | query | integer(int32) | false | Number of items per page |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | [TrailOrderChangeLogResponse](https://www.gate.com/docs/developers/apiv4/en/#schematrailorderchangelogresponse) |

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

url = '/futures/usdt/autoorder/v1/trail/change_log'
query_param = 'id=0'
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
url="/futures/usdt/autoorder/v1/trail/change_log"
query_param="id=0"
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
"change_log": [
    {
      "updated_at": 1546569968,
      "amount": "10",
      "is_gte": true,
      "activation_price": "50000",
      "price_type": 1,
      "price_offset": "0.1%",
      "is_create": true
    },
    {
      "updated_at": 1546570000,
      "amount": "20",
      "is_gte": true,
      "activation_price": "51000",
      "price_type": 1,
      "price_offset": "0.2%",
      "is_create": false
    },
    {
      "updated_at": 1546570100,
      "amount": "20",
      "is_gte": true,
      "activation_price": "51000",
      "price_type": 2,
      "price_offset": "0.2%",
      "is_create": false
    }
]
}
```

## Create price-triggered order

`POST /futures/{settle}/price_orders`

_Create price-triggered order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [FuturesPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespricetriggeredorder) | true | none |
| » initial | body | object | true | none |
| »» contract | body | string | true | Futures contract |
| »» size | body | integer(int64) | false | Represents the number of contracts that need to be closed, full closing: size=0 |
| »» price | body | string | true | Order price. Set to 0 to use market price |
| »» close | body | boolean | false | When fully closing a position in single-position mode, close must be set to true to execute the close operation. |
| »» tif | body | string | false | Time in force strategy, default is gtc, market orders currently only support ioc mode |
| »» text | body | string | false | The source of the order, including: |
| »» reduce\_only | body | boolean | false | When set to true, perform automatic position reduction operation. Set to true to ensure that the order will not open a new position, and is only used to close or reduce positions |
| »» auto\_size | body | string | false | One-way Mode: auto\_size is not required |
| » trigger | body | object | true | none |
| »» strategy\_type | body | integer(int32) | false | Trigger Strategy |
| »» price\_type | body | integer(int32) | false | Reference price type. 0 - Latest trade price, 1 - Mark price, 2 - Index price |
| »» price | body | string | true | Price value for price trigger, or spread value for spread trigger |
| »» rule | body | integer(int32) | true | Price Condition Type |
| »» expiration | body | integer | false | Maximum wait time for trigger condition (in seconds). Order will be cancelled if timeout |
| » order\_type | body | string | false | Types of take-profit and stop-loss orders, including: |
| settle | path | string | true | Settle currency |

#### Detailed descriptions

**»» size**: Represents the number of contracts that need to be closed, full closing: size=0
Partial closing: plan-close-short-position size>0
Partial closing: plan-close-long-position size<0

**»» close**: When fully closing a position in single-position mode, close must be set to true to execute the close operation.
When partially closing a position in single-position mode or in dual-position mode, close can be left unset or set to false.

**»» tif**: Time in force strategy, default is gtc, market orders currently only support ioc mode

- gtc: GoodTillCancelled
- ioc: ImmediateOrCancelled

**»» text**: The source of the order, including:

- web: Web
- api: API call
- app: Mobile app

**»» auto\_size**: One-way Mode: auto\_size is not required
Hedge Mode full closing (size=0): auto\_size must be set, close\_long for closing long positions, close\_short for closing short positions
Hedge Mode partial closing (size≠0): auto\_size is not required

**»» strategy\_type**: Trigger Strategy

- 0: Price trigger, triggered when price meets conditions
- 1: Price spread trigger, i.e. the difference between the latest price specified in `price_type` and the second-last price
Currently only supports 0 (latest transaction price)

**»» rule**: Price Condition Type

- 1: Trigger when the price calculated based on `strategy_type` and `price_type` is greater than or equal to `Trigger.Price`, while Trigger.Price must > last\_price
- 2: Trigger when the price calculated based on `strategy_type` and `price_type` is less than or equal to `Trigger.Price`, and Trigger.Price must < last\_price

**» order\_type**: Types of take-profit and stop-loss orders, including:

- `close-long-order`: Order take-profit/stop-loss, close long position
- `close-short-order`: Order take-profit/stop-loss, close short position
- `close-long-position`: Position take-profit/stop-loss, used to close all long positions
- `close-short-position`: Position take-profit/stop-loss, used to close all short positions
- `plan-close-long-position`: Position plan take-profit/stop-loss, used to close all or partial long positions
- `plan-close-short-position`: Position plan take-profit/stop-loss, used to close all or partial short positions

The two types of order take-profit/stop-loss are read-only and cannot be passed in requests

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| »» tif | gtc |
| »» tif | ioc |
| »» strategy\_type | 0 |
| »» strategy\_type | 1 |
| »» price\_type | 0 |
| »» price\_type | 1 |
| »» price\_type | 2 |
| »» rule | 1 |
| »» rule | 2 |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Order created successfully | [TriggerOrderResponse](https://www.gate.com/docs/developers/apiv4/en/#schematriggerorderresponse) |

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

url = '/futures/usdt/price_orders'
query_param = ''
body='{"initial":{"contract":"BTC_USDT","size":100,"price":"5.03"},"trigger":{"strategy_type":0,"price_type":0,"price":"3000","rule":1,"expiration":86400},"order_type":"close-long-order"}'
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
url="/futures/usdt/price_orders"
query_param=""
body_param='{"initial":{"contract":"BTC_USDT","size":100,"price":"5.03"},"trigger":{"strategy_type":0,"price_type":0,"price":"3000","rule":1,"expiration":86400},"order_type":"close-long-order"}'
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
"initial": {
    "contract": "BTC_USDT",
    "size": 100,
    "price": "5.03"
},
"trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "3000",
    "rule": 1,
    "expiration": 86400
},
"order_type": "close-long-order"
}
```

> Example responses

> 201 Response

```json
{
"id": 1432329
}
```

## Query auto order list

`GET /futures/{settle}/price_orders`

_Query auto order list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| status | query | string | true | Query order list based on status |
| contract | query | string | false | Futures contract, return related data only if specified |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| status | open |
| status | finished |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [FuturesPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespricetriggeredorder)\] |

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

url = '/futures/usdt/price_orders'
query_param = 'status=open'
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
url="/futures/usdt/price_orders"
query_param="status=open"
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
    "initial": {
      "contract": "BTC_USDT",
      "size": 100,
      "price": "5.03"
    },
    "trigger": {
      "strategy_type": 0,
      "price_type": 0,
      "price": "3000",
      "rule": 1,
      "expiration": 86400
    },
    "id": 1283293,
    "user": 1234,
    "create_time": 1514764800,
    "finish_time": 1514764900,
    "trade_id": 13566,
    "status": "finished",
    "finish_as": "cancelled",
    "reason": "",
    "order_type": "close-long-order"
}
]
```

## Cancel all auto orders

`DELETE /futures/{settle}/price_orders`

_Cancel all auto orders_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| contract | query | string | false | Futures contract, return related data only if specified |
| settle | path | string | true | Settle currency |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch cancel request is received and processed. Success is determined based on the order list | \[ [FuturesPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespricetriggeredorder)\] |

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

url = '/futures/usdt/price_orders'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('DELETE', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('DELETE', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="DELETE"
url="/futures/usdt/price_orders"
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
    "initial": {
      "contract": "BTC_USDT",
      "size": 100,
      "price": "5.03"
    },
    "trigger": {
      "strategy_type": 0,
      "price_type": 0,
      "price": "3000",
      "rule": 1,
      "expiration": 86400
    },
    "id": 1283293,
    "user": 1234,
    "create_time": 1514764800,
    "finish_time": 1514764900,
    "trade_id": 13566,
    "status": "finished",
    "finish_as": "cancelled",
    "reason": "",
    "order_type": "close-long-order"
}
]
```

## Query single auto order details

`GET /futures/{settle}/price_orders/{order_id}`

_Query single auto order details_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| order\_id | path | integer(int64) | true | ID returned when order is successfully created |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Auto order details | [FuturesPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespricetriggeredorder) |

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

url = '/futures/usdt/price_orders/0'
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
url="/futures/usdt/price_orders/0"
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
"initial": {
    "contract": "BTC_USDT",
    "size": 100,
    "price": "5.03"
},
"trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "3000",
    "rule": 1,
    "expiration": 86400
},
"id": 1283293,
"user": 1234,
"create_time": 1514764800,
"finish_time": 1514764900,
"trade_id": 13566,
"status": "finished",
"finish_as": "cancelled",
"reason": "",
"order_type": "close-long-order"
}
```

## Cancel single auto order

`DELETE /futures/{settle}/price_orders/{order_id}`

_Cancel single auto order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| settle | path | string | true | Settle currency |
| order\_id | path | integer(int64) | true | ID returned when order is successfully created |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Auto order details | [FuturesPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturespricetriggeredorder) |

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

url = '/futures/usdt/price_orders/0'
query_param = ''
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('DELETE', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('DELETE', host + prefix + url, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="DELETE"
url="/futures/usdt/price_orders/0"
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
"initial": {
    "contract": "BTC_USDT",
    "size": 100,
    "price": "5.03"
},
"trigger": {
    "strategy_type": 0,
    "price_type": 0,
    "price": "3000",
    "rule": 1,
    "expiration": 86400
},
"id": 1283293,
"user": 1234,
"create_time": 1514764800,
"finish_time": 1514764900,
"trade_id": 13566,
"status": "finished",
"finish_as": "cancelled",
"reason": "",
"order_type": "close-long-order"
}
```

## Modify a Single Auto Order

`PUT /futures/{settle}/price_orders/amend/{order_id}`

_Modify a Single Auto Order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [FuturesUpdatePriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemafuturesupdatepricetriggeredorder) | true | none |
| » settle | body | string | false | Settlement Currency (e.g., USDT, BTC) |
| » order\_id | body | integer(int64) | true | ID of the Pending Take-Profit/Stop-Loss Trigger Order |
| » size | body | integer(int64) | false | Modified Contract Quantity. Full Close: 0; Partial Close: Positive/Negative values indicate direction (consistent with the creation interface logic). |
| » price | body | string | false | Represents the modified trading price. A value of 0 indicates a market order. |
| » trigger\_price | body | string | false | Modified Trigger Price |
| » price\_type | body | integer(int32) | false | Reference price type. 0 - Latest trade price, 1 - Mark price, 2 - Index price |
| » auto\_size | body | string | false | One-way Mode: auto\_size is not required |
| » close | body | boolean | false | When fully closing a position in single-position mode, close must be set to true to execute the close operation. |
| settle | path | string | true | Settle currency |
| order\_id | path | integer(int64) | true | ID returned when order is successfully created |

#### Detailed descriptions

**» auto\_size**: One-way Mode: auto\_size is not required
Hedge Mode partial closing (size≠0): auto\_size is not required
Hedge Mode full closing (size=0): auto\_size must be set, close\_long for closing long positions, close\_short for closing short positions

**» close**: When fully closing a position in single-position mode, close must be set to true to execute the close operation.
When partially closing a position in single-position mode or in dual-position mode, close can be left unset or set to false.

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » price\_type | 0 |
| » price\_type | 1 |
| » price\_type | 2 |
| settle | btc |
| settle | usdt |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order created successfully | [TriggerOrderResponse](https://www.gate.com/docs/developers/apiv4/en/#schematriggerorderresponse) |

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

url = '/futures/usdt/price_orders/amend/0'
query_param = ''
body='{"order_id":123456789,"size":0,"price":"0","trigger_price":"988888","price_type":0,"auto_size":"close_long"}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('PUT', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('PUT', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="PUT"
url="/futures/usdt/price_orders/amend/0"
query_param=""
body_param='{"order_id":123456789,"size":0,"price":"0","trigger_price":"988888","price_type":0,"auto_size":"close_long"}'
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
"order_id": 123456789,
"size": 0,
"price": "0",
"trigger_price": "988888",
"price_type": 0,
"auto_size": "close_long"
}
```

> Example responses

> 200 Response

```json
{
"id": 1432329
}
```