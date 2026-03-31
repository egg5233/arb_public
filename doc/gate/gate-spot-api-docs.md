# Gate.io Spot Trading API Documentation

Source: https://www.gate.com/docs/developers/apiv4/en/#spot

---

# Spot

Spot trading

## Query all currency information

`GET /spot/currencies`

_Query all currency information_

When a currency corresponds to multiple chains, you can query the information of multiple chains through the `chains` field, such as the charging and recharge status, identification, etc. of the chain

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | none |
| » currency | string | Currency symbol |
| » name | string | Currency name |
| » delisted | boolean | Whether currency is de-listed |
| » withdraw\_disabled | boolean | Whether currency's withdrawal is disabled (deprecated) |
| » withdraw\_delayed | boolean | Whether currency's withdrawal is delayed (deprecated) |
| » deposit\_disabled | boolean | Whether currency's deposit is disabled (deprecated) |
| » trade\_disabled | boolean | Whether currency's trading is disabled |
| » fixed\_rate | string | Fixed fee rate. Only for fixed rate currencies, not valid for normal currencies |
| » chain | string | The main chain corresponding to the coin |
| » chains | array | All links corresponding to coins |
| »» SpotCurrencyChain | object | none |
| »»» name | string | Blockchain name |
| »»» addr | string | token address |
| »»» withdraw\_disabled | boolean | Whether currency's withdrawal is disabled |
| »»» withdraw\_delayed | boolean | Whether currency's withdrawal is delayed |
| »»» deposit\_disabled | boolean | Whether currency's deposit is disabled |
| »» total\_supply | string | Total supply |
| »» market\_cap | string | Market cap |
| »» category | array | Currency categories<br>\- stocks: Stocks<br>\- metals: Metals<br>\- indices: Indices<br>\- forex: Forex<br>\- commodities: Commodities |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/currencies'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/currencies 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "currency": "GT",
    "name": "GateToken",
    "delisted": false,
    "withdraw_disabled": false,
    "withdraw_delayed": false,
    "deposit_disabled": false,
    "trade_disabled": false,
    "chain": "GT",
    "chains": [
      {
        "name": "GT",
        "addr": "",
        "withdraw_disabled": false,
        "withdraw_delayed": false,
        "deposit_disabled": false
      },
      {
        "name": "ETH",
        "withdraw_disabled": false,
        "withdraw_delayed": false,
        "deposit_disabled": false,
        "addr": "0xE66747a101bFF2dBA3697199DCcE5b743b454759"
      },
      {
        "name": "GTEVM",
        "withdraw_disabled": false,
        "withdraw_delayed": false,
        "deposit_disabled": false,
        "addr": ""
      }
    ],
    "total_supply": "2100000",
    "market_cap": "18880000",
    "category": []
}
]
```

## Query single currency information

`GET /spot/currencies/{currency}`

_Query single currency information_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | path | string | true | Currency name |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » currency | string | Currency symbol |
| » name | string | Currency name |
| » delisted | boolean | Whether currency is de-listed |
| » withdraw\_disabled | boolean | Whether currency's withdrawal is disabled (deprecated) |
| » withdraw\_delayed | boolean | Whether currency's withdrawal is delayed (deprecated) |
| » deposit\_disabled | boolean | Whether currency's deposit is disabled (deprecated) |
| » trade\_disabled | boolean | Whether currency's trading is disabled |
| » fixed\_rate | string | Fixed fee rate. Only for fixed rate currencies, not valid for normal currencies |
| » chain | string | The main chain corresponding to the coin |
| » chains | array | All links corresponding to coins |
| »» SpotCurrencyChain | object | none |
| »»» name | string | Blockchain name |
| »»» addr | string | token address |
| »»» withdraw\_disabled | boolean | Whether currency's withdrawal is disabled |
| »»» withdraw\_delayed | boolean | Whether currency's withdrawal is delayed |
| »»» deposit\_disabled | boolean | Whether currency's deposit is disabled |
| »» total\_supply | string | Total supply |
| »» market\_cap | string | Market cap |
| »» category | array | Currency categories<br>\- stocks: Stocks<br>\- metals: Metals<br>\- indices: Indices<br>\- forex: Forex<br>\- commodities: Commodities |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/currencies/GT'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/currencies/GT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"currency": "GT",
"name": "GateToken",
"delisted": false,
"withdraw_disabled": false,
"withdraw_delayed": false,
"deposit_disabled": false,
"trade_disabled": false,
"chain": "GT",
"chains": [
    {
      "name": "GT",
      "addr": "",
      "withdraw_disabled": false,
      "withdraw_delayed": false,
      "deposit_disabled": false
    },
    {
      "name": "ETH",
      "withdraw_disabled": false,
      "withdraw_delayed": false,
      "deposit_disabled": false,
      "addr": "0xE66747a101bFF2dBA3697199DCcE5b743b454759"
    },
    {
      "name": "GTEVM",
      "withdraw_disabled": false,
      "withdraw_delayed": false,
      "deposit_disabled": false,
      "addr": ""
    }
],
"total_supply": "2100000",
"market_cap": "18880000",
"category": []
}
```

## Query all supported currency pairs

`GET /spot/currency_pairs`

_Query all supported currency pairs_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | All currency pairs retrieved | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Spot currency pair\] |
| » _None_ | object | Spot currency pair |
| »» id | string | Currency pair |
| »» base | string | Base currency |
| »» base\_name | string | Base currency name |
| »» quote | string | Quote currency |
| »» quote\_name | string | Quote currency name |
| »» fee | string | Trading fee rate(deprecated) |
| »» min\_base\_amount | string | Minimum amount of base currency to trade, `null` means no limit |
| »» min\_quote\_amount | string | Minimum amount of quote currency to trade, `null` means no limit |
| »» max\_base\_amount | string | Maximum amount of base currency to trade, `null` means no limit |
| »» max\_quote\_amount | string | Maximum amount of quote currency to trade, `null` means no limit |
| »» amount\_precision | integer | Amount scale |
| »» precision | integer | Price scale |
| »» trade\_status | string | Trading status<br>\- untradable: cannot be traded<br>\- buyable: can be bought<br>\- sellable: can be sold<br>\- tradable: can be bought and sold |
| »» sell\_start | integer(int64) | Sell start unix timestamp in seconds |
| »» buy\_start | integer(int64) | Buy start unix timestamp in seconds |
| »» delisting\_time | integer(int64) | Expected time to remove the shelves, Unix timestamp in seconds |
| »» type | string | Trading pair type, normal: normal, premarket: pre-market |
| »» trade\_url | string | Transaction link |
| »» st\_tag | boolean | Whether the trading pair is in ST risk assessment, false - No, true - Yes |
| »» up\_rate | string | Maximum Quote Rise Percentage |
| »» down\_rate | string | Maximum Quote Decline Percentage |
| »» slippage | string | Maximum supported slippage ratio for Spot Market Order Placement, calculated based on the latest market price at the time of order placement as the benchmark (Example: 0.03 means 3%) |
| »» market\_order\_max\_stock | string | Maximum Market Order Quantity |
| »» market\_order\_max\_money | string | Maximum Market Order Amount |

#### Enumerated Values

| Property | Value |
| --- | --- |
| trade\_status | untradable |
| trade\_status | buyable |
| trade\_status | sellable |
| trade\_status | tradable |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/currency_pairs'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/currency_pairs 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "id": "ETH_USDT",
    "base": "ETH",
    "base_name": "Ethereum",
    "quote": "USDT",
    "quote_name": "Tether",
    "fee": "0.2",
    "min_base_amount": "0.001",
    "min_quote_amount": "1.0",
    "max_base_amount": "10000",
    "max_quote_amount": "10000000",
    "amount_precision": 3,
    "precision": 6,
    "trade_status": "tradable",
    "sell_start": 1516378650,
    "buy_start": 1516378650,
    "delisting_time": 0,
    "trade_url": "https://www.gate.io/trade/ETH_USDT",
    "st_tag": false,
    "up_rate": "0.05",
    "down_rate": "0.02",
    "slippage": "0.05",
    "max_market_order_stock": "100000",
    "max_market_order_money": "1000000"
}
]
```

## Query single currency pair details

`GET /spot/currency_pairs/{currency_pair}`

_Query single currency pair details_

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

_Spot currency pair_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Currency pair |
| » base | string | Base currency |
| » base\_name | string | Base currency name |
| » quote | string | Quote currency |
| » quote\_name | string | Quote currency name |
| » fee | string | Trading fee rate(deprecated) |
| » min\_base\_amount | string | Minimum amount of base currency to trade, `null` means no limit |
| » min\_quote\_amount | string | Minimum amount of quote currency to trade, `null` means no limit |
| » max\_base\_amount | string | Maximum amount of base currency to trade, `null` means no limit |
| » max\_quote\_amount | string | Maximum amount of quote currency to trade, `null` means no limit |
| » amount\_precision | integer | Amount scale |
| » precision | integer | Price scale |
| » trade\_status | string | Trading status<br>\- untradable: cannot be traded<br>\- buyable: can be bought<br>\- sellable: can be sold<br>\- tradable: can be bought and sold |
| » sell\_start | integer(int64) | Sell start unix timestamp in seconds |
| » buy\_start | integer(int64) | Buy start unix timestamp in seconds |
| » delisting\_time | integer(int64) | Expected time to remove the shelves, Unix timestamp in seconds |
| » type | string | Trading pair type, normal: normal, premarket: pre-market |
| » trade\_url | string | Transaction link |
| » st\_tag | boolean | Whether the trading pair is in ST risk assessment, false - No, true - Yes |
| » up\_rate | string | Maximum Quote Rise Percentage |
| » down\_rate | string | Maximum Quote Decline Percentage |
| » slippage | string | Maximum supported slippage ratio for Spot Market Order Placement, calculated based on the latest market price at the time of order placement as the benchmark (Example: 0.03 means 3%) |
| » market\_order\_max\_stock | string | Maximum Market Order Quantity |
| » market\_order\_max\_money | string | Maximum Market Order Amount |

#### Enumerated Values

| Property | Value |
| --- | --- |
| trade\_status | untradable |
| trade\_status | buyable |
| trade\_status | sellable |
| trade\_status | tradable |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/currency_pairs/ETH_BTC'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/currency_pairs/ETH_BTC 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"id": "ETH_USDT",
"base": "ETH",
"base_name": "Ethereum",
"quote": "USDT",
"quote_name": "Tether",
"fee": "0.2",
"min_base_amount": "0.001",
"min_quote_amount": "1.0",
"max_base_amount": "10000",
"max_quote_amount": "10000000",
"amount_precision": 3,
"precision": 6,
"trade_status": "tradable",
"sell_start": 1516378650,
"buy_start": 1516378650,
"delisting_time": 0,
"trade_url": "https://www.gate.io/trade/ETH_USDT",
"st_tag": false,
"up_rate": "0.05",
"down_rate": "0.02",
"slippage": "0.05",
"max_market_order_stock": "100000",
"max_market_order_money": "1000000"
}
```

## Get currency pair ticker information

`GET /spot/tickers`

_Get currency pair ticker information_

If `currency_pair` is specified, only query that currency pair; otherwise return all information

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |
| timezone | query | string | false | Timezone |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| timezone | utc0 |
| timezone | utc8 |
| timezone | all |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » currency\_pair | string | Currency pair |
| » last | string | Last trading price |
| » lowest\_ask | string | Recent lowest ask |
| » lowest\_size | string | Latest seller's lowest price quantity; not available for batch queries; available for single queries, empty if no data |
| » highest\_bid | string | Recent highest bid |
| » highest\_size | string | Latest buyer's highest price quantity; not available for batch queries; available for single queries, empty if no data |
| » change\_percentage | string | 24h price change percentage (negative for decrease, e.g., -7.45) |
| » change\_utc0 | string | UTC+0 timezone, 24h price change percentage, negative for decline (e.g., -7.45) |
| » change\_utc8 | string | UTC+8 timezone, 24h price change percentage, negative for decline (e.g., -7.45) |
| » base\_volume | string | Base currency trading volume in the last 24h |
| » quote\_volume | string | Quote currency trading volume in the last 24h |
| » high\_24h | string | 24h High |
| » low\_24h | string | 24h Low |
| » etf\_net\_value | string | ETF net value |
| » etf\_pre\_net\_value | string\|null | ETF net value at previous rebalancing point |
| » etf\_pre\_timestamp | integer(int64)\|null | ETF previous rebalancing time |
| » etf\_leverage | string\|null | ETF current leverage |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/tickers'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/tickers 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "currency_pair": "BTC3L_USDT",
    "last": "2.46140352",
    "lowest_ask": "2.477",
    "highest_bid": "2.4606821",
    "change_percentage": "-8.91",
    "change_utc0": "-8.91",
    "change_utc8": "-8.91",
    "base_volume": "656614.0845820589",
    "quote_volume": "1602221.66468375534639404191",
    "high_24h": "2.7431",
    "low_24h": "1.9863",
    "etf_net_value": "2.46316141",
    "etf_pre_net_value": "2.43201848",
    "etf_pre_timestamp": 1611244800,
    "etf_leverage": "2.2803019447281203"
}
]
```

## Get market depth information

`GET /spot/order_book`

_Get market depth information_

Market depth buy orders are sorted by price from high to low, sell orders are reversed

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Currency pair |
| interval | query | string | false | Price precision for merged depth. 0 means no merging. If not specified, defaults to 0 |
| limit | query | integer | false | Number of depth levels |
| with\_id | query | boolean | false | Return order book update ID |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | integer(int64) | Order book ID, which is updated whenever the order book is changed. Valid only when `with_id` is set to `true` |
| » current | integer(int64) | The timestamp of the response data being generated (in milliseconds) |
| » update | integer(int64) | The timestamp of when the orderbook last changed (in milliseconds) |
| » asks | array | Ask Depth |
| »» _None_ | array | Price and Quantity Pair |
| » bids | array | Bid Depth |
| »» _None_ | array | Price and Quantity Pair |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/order_book'
query_param = 'currency_pair=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/order_book?currency_pair=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"id": 123456,
"current": 1623898993123,
"update": 1623898993121,
"asks": [
    [
      "1.52",
      "1.151"
    ],
    [
      "1.53",
      "1.218"
    ]
],
"bids": [
    [
      "1.17",
      "201.863"
    ],
    [
      "1.16",
      "725.464"
    ]
]
}
```

## Query market transaction records

`GET /spot/trades`

_Query market transaction records_

Supports querying by time range using `from` and `to` parameters or pagination based on `last_id`. By default, queries the last 30 days.

Pagination based on `last_id` is no longer recommended. If `last_id` is specified, the time range query parameters will be ignored.

When using limit&page pagination to retrieve data, the maximum number of pages is 100,000, that is, limit \* (page - 1) <= 100,000.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Currency pair |
| limit | query | integer(int32) | false | Maximum number of items returned in list. Default: 100, minimum: 1, maximum: 1000 |
| last\_id | query | string | false | Use the ID of the last record in the previous list as the starting point for the next list |
| reverse | query | boolean | false | Whether to retrieve data less than `last_id`. Default returns records greater than `last_id`. |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |
| page | query | integer(int32) | false | Page number |

#### Detailed descriptions

**last\_id**: Use the ID of the last record in the previous list as the starting point for the next list

Operations based on custom IDs can only be checked when orders are pending. After orders are completed (filled/cancelled), they can be checked within 1 hour after completion. After expiration, only order IDs can be used

**reverse**: Whether to retrieve data less than `last_id`. Default returns records greater than `last_id`.

Set to `true` to trace back market trade records, `false` to get latest trades.

No effect when `last_id` is not set.

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | none |
| » id | string | Fill ID |
| » create\_time | string | Fill Time |
| » create\_time\_ms | string | Trading time, with millisecond precision |
| » currency\_pair | string | Currency pair |
| » side | string | Buy or sell order |
| » role | string | Trade role, not returned in public endpoints |
| » amount | string | Trade amount |
| » price | string | Order price |
| » order\_id | string | Related order ID, not returned in public endpoints |
| » fee | string | Fee deducted, not returned in public endpoints |
| » fee\_currency | string | Fee currency unit, not returned in public endpoints |
| » point\_fee | string | Points used to deduct fee, not returned in public endpoints |
| » gt\_fee | string | GT used to deduct fee, not returned in public endpoints |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » sequence\_id | string | Consecutive trade ID within a single market.<br>Used to track and identify trades in the specific market |
| » text | string | Order's Custom Information. This field is not returned by public interfaces.<br>The scenarios pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent full-account forced liquidation orders.<br>liquidate represents isolated-account forced liquidation orders. |
| » deal | string | Total Executed Value |

#### Enumerated Values

| Property | Value |
| --- | --- |
| side | buy |
| side | sell |
| role | taker |
| role | maker |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/trades'
query_param = 'currency_pair=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/trades?currency_pair=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "id": "1232893232",
    "create_time": "1548000000",
    "create_time_ms": "1548000000123.456",
    "order_id": "4128442423",
    "side": "buy",
    "role": "maker",
    "amount": "0.15",
    "price": "0.03",
    "fee": "0.0005",
    "fee_currency": "ETH",
    "point_fee": "0",
    "gt_fee": "0",
    "sequence_id": "588018",
    "text": "t-test",
    "deal": "0.0045"
}
]
```

## Market K-line chart

`GET /spot/candlesticks`

_Market K-line chart_

K-line chart data returns a maximum of 1000 points per request. When specifying from, to, and interval, ensure the number of points is not excessive

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Currency pair |
| limit | query | integer | false | Maximum number of recent data points to return. `limit` conflicts with `from` and `to`. If either `from` or `to` is specified, request will be rejected. |
| from | query | integer(int64) | false | Start time of candlesticks, formatted in Unix timestamp in seconds. Default to`to - 100 * interval` if not specified |
| to | query | integer(int64) | false | Specify the end time of the K-line chart, defaults to current time if not specified, note that the time format is Unix timestamp with second precision |
| interval | query | string | false | Time interval between data points. Note that `30d` represents a calendar month, not aligned to 30 days |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| interval | 1s |
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
| interval | 30d |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[\[string\]\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | array | Candlestick data for each time granularity, from left to right:<br>\- Unix timestamp with second precision<br>\- Trading volume in quote currency<br>\- Closing price<br>\- Highest price<br>\- Lowest price<br>\- Opening price<br>\- Trading volume in base currency<br>\- Whether window is closed; true means this candlestick data segment is complete, false means not yet complete |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/candlesticks'
query_param = 'currency_pair=BTC_USDT'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/candlesticks?currency_pair=BTC_USDT 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
[
    "1539852480",
    "971519.677",
    "0.0021724",
    "0.0021922",
    "0.0021724",
    "0.0021737",
    "true"
]
]
```

## Query account fee rates

`GET /spot/fee`

_Query account fee rates_

This API is deprecated. The new fee query API is `/wallet/fee`

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Specify currency pair to get more accurate fee settings. |

#### Detailed descriptions

**currency\_pair**: Specify currency pair to get more accurate fee settings.

This field is optional. Usually fee settings are the same for all currency pairs.

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » user\_id | integer(int64) | User ID |
| » taker\_fee | string | taker fee rate |
| » maker\_fee | string | maker fee rate |
| » rpi\_maker\_fee | string | RPI MM maker fee rate |
| » gt\_discount | boolean | Whether GT deduction discount is enabled |
| » gt\_taker\_fee | string | Taker fee rate if using GT deduction. It will be 0 if GT deduction is disabled |
| » gt\_maker\_fee | string | Maker fee rate with GT deduction. Returns 0 if GT deduction is disabled |
| » loan\_fee | string | Loan fee rate of margin lending |
| » point\_type | string | Point card type: 0 - Original version, 1 - New version since 202009 |
| » currency\_pair | string | Currency pair |
| » debit\_fee | integer | Deduction types for rates, 1 - GT deduction, 2 - Point card deduction, 3 - VIP rates |
| » rpi\_mm | integer | RPI MM Level |

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

url = '/spot/fee'
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
url="/spot/fee"
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
"user_id": 10001,
"taker_fee": "0.002",
"maker_fee": "0.002",
"gt_discount": false,
"gt_taker_fee": "0",
"gt_maker_fee": "0",
"loan_fee": "0.18",
"point_type": "1",
"currency_pair": "BTC_USDT",
"debit_fee": 3
}
```

## Batch query account fee rates

`GET /spot/batch_fee`

_Batch query account fee rates_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pairs | query | string | true | Maximum 50 currency pairs per request |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | Inline |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » **additionalProperties** | object | none |
| »» user\_id | integer(int64) | User ID |
| »» taker\_fee | string | taker fee rate |
| »» maker\_fee | string | maker fee rate |
| »» rpi\_maker\_fee | string | RPI MM maker fee rate |
| »» gt\_discount | boolean | Whether GT deduction discount is enabled |
| »» gt\_taker\_fee | string | Taker fee rate if using GT deduction. It will be 0 if GT deduction is disabled |
| »» gt\_maker\_fee | string | Maker fee rate with GT deduction. Returns 0 if GT deduction is disabled |
| »» loan\_fee | string | Loan fee rate of margin lending |
| »» point\_type | string | Point card type: 0 - Original version, 1 - New version since 202009 |
| »» currency\_pair | string | Currency pair |
| »» debit\_fee | integer | Deduction types for rates, 1 - GT deduction, 2 - Point card deduction, 3 - VIP rates |
| »» rpi\_mm | integer | RPI MM Level |

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

url = '/spot/batch_fee'
query_param = 'currency_pairs=BTC_USDT,ETH_USDT'
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
url="/spot/batch_fee"
query_param="currency_pairs=BTC_USDT,ETH_USDT"
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
"BTC_USDT": {
    "user_id": 10001,
    "taker_fee": "0.002",
    "maker_fee": "0.002",
    "rpi_maker_fee": "-0.00175",
    "gt_discount": false,
    "gt_taker_fee": "0",
    "gt_maker_fee": "0",
    "loan_fee": "0.18",
    "point_type": "1",
    "currency_pair": "BTC_USDT",
    "debit_fee": 3,
    "rpi_mm": 2
},
"GT_USDT": {
    "user_id": 10001,
    "taker_fee": "0.002",
    "maker_fee": "0.002",
    "rpi_maker_fee": "-0.00175",
    "gt_discount": false,
    "gt_taker_fee": "0",
    "gt_maker_fee": "0",
    "loan_fee": "0.18",
    "point_type": "1",
    "currency_pair": "GT_USDT",
    "debit_fee": 3,
    "rpi_mm": 2
},
"ETH_USDT": {
    "user_id": 10001,
    "taker_fee": "0.002",
    "maker_fee": "0.002",
    "rpi_maker_fee": "-0.00175",
    "gt_discount": false,
    "gt_taker_fee": "0",
    "gt_maker_fee": "0",
    "loan_fee": "0.18",
    "point_type": "1",
    "currency_pair": "ETH_USDT",
    "debit_fee": 3,
    "rpi_mm": 2
}
}
```

## List spot trading accounts

`GET /spot/accounts`

_List spot trading accounts_

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
| » currency | string | Currency detail |
| » available | string | Available amount |
| » locked | string | Locked amount, used in trading |
| » update\_id | integer(int64) | Version number |

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

url = '/spot/accounts'
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
url="/spot/accounts"
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
    "currency": "ETH",
    "available": "968.8",
    "locked": "0",
    "update_id": 98
}
]
```

## Query spot account transaction history

`GET /spot/account_book`

_Query spot account transaction history_

Record query time range cannot exceed 30 days.

When using limit&page pagination to retrieve data, the maximum number of pages is 100,000, that is, limit \* (page - 1) <= 100,000.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency | query | string | false | Query by specified currency name |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in a single list |
| type | query | string | false | Query by specified account change type. If not specified, all change types will be included. |
| code | query | string | false | Specify account change code for query. If not specified, all change types are included. This parameter has higher priority than `type` |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Balance change record ID |
| » time | integer(int64) | The timestamp of the change (in milliseconds) |
| » currency | string | Currency changed |
| » change | string | Amount changed. Positive value means transferring in, while negative out |
| » balance | string | Balance after change |
| » type | string | Account book type. Please refer to [account book type](https://www.gate.com/docs/developers/apiv4/en/#accountbook-type) for more detail |
| » code | string | Account change code, see \[Asset Record Code\] (Asset Record Code) |
| » text | string | Additional information |

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

url = '/spot/account_book'
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
url="/spot/account_book"
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
    "time": 1547633726123,
    "currency": "BTC",
    "change": "1.03",
    "balance": "4.59316525194",
    "type": "margin_in",
    "text": "3815099"
}
]
```

## Batch place orders

`POST /spot/batch_orders`

_Batch place orders_

Batch order requirements:

1. Custom order field `text` must be specified
2. Up to 4 currency pairs per request, with up to 10 orders per currency pair
3. Spot orders and margin orders cannot be mixed; all `account` fields in the same request must be identical

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | array | true | none |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Request execution completed | \[Inline\] |

### Response Schema

Status Code **200**

_Contains multiple order objects; for the specific structure of the order object, refer to the structure of the /spot/orders order placement interface_

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | Contains multiple order objects; for the specific structure of the order object, refer to the structure of the /spot/orders order placement interface |
| » _None_ | object | Batch order details |
| »» order\_id | string | Order ID |
| »» amend\_text | string | The custom data that the user remarked when amending the order |
| »» text | string | Order custom information. Users can set custom ID with this field. Custom fields must meet the following conditions:<br>1\. Must start with `t-`<br>2\. Excluding `t-`, length cannot exceed 28 bytes<br>3\. Can only contain numbers, letters, underscore(\_), hyphen(-) or dot(.) |
| »» succeeded | boolean | Request execution result |
| »» label | string | Error label, if any, otherwise an empty string |
| »» message | string | Detailed error message, if any, otherwise an empty string |
| »» id | string | Order ID |
| »» create\_time | string | Creation time of order |
| »» update\_time | string | Last modification time of order |
| »» create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| »» update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| »» status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| »» currency\_pair | string | Currency pair |
| »» type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| »» account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| »» side | string | Buy or sell order |
| »» amount | string | Trade amount |
| »» price | string | Order price |
| »» time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none |
| »» iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| »» auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| »» left | string | Amount left to fill |
| »» filled\_amount | string | Amount filled |
| »» fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| »» filled\_total | string | Total filled in quote currency |
| »» avg\_deal\_price | string | Average fill price |
| »» fee | string | Fee deducted |
| »» fee\_currency | string | Fee currency unit |
| »» point\_fee | string | Points used to deduct fee |
| »» gt\_fee | string | GT used to deduct fee |
| »» gt\_discount | boolean | Whether GT fee deduction is enabled |
| »» rebated\_fee | string | Rebated fee |
| »» rebated\_fee\_currency | string | Rebated fee currency unit |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevetion strategies<br>1\. After users join the `STP Group`, he can pass `stp_act` to limit the user's self-trade prevetion strategy. If `stp_act` is not passed, the default is `cn` strategy. <br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter. <br>3\. If the user did not use 'stp\_act' when placing the order, 'stp\_act' will return '-'<br>\- cn: Cancel newest, Cancel new orders and keep old ones<br>\- co: Cancel oldest, new ones<br>\- cb: Cancel both, Both old and new orders will be cancelled |
| »» finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| account | spot |
| account | margin |
| account | cross\_margin |
| account | unified |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/batch_orders'
query_param = ''
body='[{"text":"t-abc123","currency_pair":"BTC_USDT","type":"limit","account":"unified","side":"buy","amount":"0.001","price":"65000","time_in_force":"gtc","iceberg":"0","slippage":"0.05"}]'
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
url="/spot/batch_orders"
query_param=""
body_param='[{"text":"t-abc123","currency_pair":"BTC_USDT","type":"limit","account":"unified","side":"buy","amount":"0.001","price":"65000","time_in_force":"gtc","iceberg":"0","slippage":"0.05"}]'
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
    "text": "t-abc123",
    "currency_pair": "BTC_USDT",
    "type": "limit",
    "account": "unified",
    "side": "buy",
    "amount": "0.001",
    "price": "65000",
    "time_in_force": "gtc",
    "iceberg": "0",
    "slippage": "0.05"
}
]
```

> Example responses

> 200 Response

```json
[
{
    "order_id": "12332324",
    "amend_text": "t-123456",
    "text": "t-123456",
    "succeeded": true,
    "label": "",
    "message": "",
    "id": "12332324",
    "create_time": "1548000000",
    "update_time": "1548000100",
    "create_time_ms": 1548000000123,
    "update_time_ms": 1548000100123,
    "currency_pair": "ETC_BTC",
    "status": "cancelled",
    "type": "limit",
    "account": "spot",
    "side": "buy",
    "amount": "1",
    "price": "5.00032",
    "time_in_force": "gtc",
    "iceberg": "0",
    "left": "0.5",
    "filled_amount": "1.242",
    "filled_total": "2.50016",
    "avg_deal_price": "5.00032",
    "fee": "0.005",
    "fee_currency": "ETH",
    "point_fee": "0",
    "gt_fee": "0",
    "gt_discount": false,
    "rebated_fee": "0",
    "rebated_fee_currency": "BTC",
    "stp_act": "cn",
    "finish_as": "stp",
    "stp_id": 10240
}
]
```

## List all open orders

`GET /spot/open_orders`

_List all open orders_

Query the current order list of all trading pairs.
Please note that the paging parameter controls the number of pending orders in each trading pair. There is no paging control trading pairs. All trading pairs with pending orders will be returned.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records returned in one page in each currency pair |
| account | query | string | false | Specify query account |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » currency\_pair | string | Currency pair |
| » total | integer | Total number of open orders for this trading pair on the current page |
| » orders | array | none |
| »» _None_ | object | Spot order details |
| »»» id | string | Order ID |
| »»» text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| »»» amend\_text | string | The custom data that the user remarked when amending the order |
| »»» create\_time | string | Creation time of order |
| »»» update\_time | string | Last modification time of order |
| »»» create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| »»» update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| »»» status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| »»» currency\_pair | string | Currency pair |
| »»» type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| »»» account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| »»» side | string | Buy or sell order |
| »»» amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| »»» price | string | Trading price, required when `type`=`limit` |
| »»» time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| »»» iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| »»» auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| »»» left | string | Amount left to fill |
| »»» filled\_amount | string | Amount filled |
| »»» fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| »»» filled\_total | string | Total filled in quote currency |
| »»» avg\_deal\_price | string | Average fill price |
| »»» fee | string | Fee deducted |
| »»» fee\_currency | string | Fee currency unit |
| »»» point\_fee | string | Points used to deduct fee |
| »»» gt\_fee | string | GT used to deduct fee |
| »»» gt\_maker\_fee | string | GT amount used to deduct maker fee |
| »»» gt\_taker\_fee | string | GT amount used to deduct taker fee |
| »»» gt\_discount | boolean | Whether GT fee deduction is enabled |
| »»» rebated\_fee | string | Rebated fee |
| »»» rebated\_fee\_currency | string | Rebated fee currency unit |
| »»» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »»» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| »»» finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/open_orders'
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
url="/spot/open_orders"
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
    "currency_pair": "ETH_BTC",
    "total": 1,
    "orders": [
      {
        "id": "12332324",
        "text": "t-123456",
        "create_time": "1548000000",
        "update_time": "1548000100",
        "currency_pair": "ETH_BTC",
        "status": "open",
        "type": "limit",
        "account": "spot",
        "side": "buy",
        "amount": "1",
        "price": "5.00032",
        "time_in_force": "gtc",
        "left": "0.5",
        "filled_total": "2.50016",
        "fee": "0.005",
        "fee_currency": "ETH",
        "point_fee": "0",
        "gt_fee": "0",
        "gt_discount": false,
        "rebated_fee": "0",
        "rebated_fee_currency": "BTC"
      }
    ]
}
]
```

## Close position when cross-currency is disabled

`POST /spot/cross_liquidate_orders`

_Close position when cross-currency is disabled_

Currently, only cross-margin accounts are supported to place buy orders for disabled currencies.
Maximum buy quantity = (unpaid principal and interest - currency balance - the amount of the currency in pending orders) / 0.998

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | object | true | none |
| » text | body | string | false | Order custom information. Users can set custom ID with this field. Custom fields must meet the following conditions: |
| » currency\_pair | body | string | true | Currency pair |
| » amount | body | string | true | Trade amount |
| » price | body | string | true | Order price |
| » action\_mode | body | string | false | Processing mode: |

#### Detailed descriptions

**» text**: Order custom information. Users can set custom ID with this field. Custom fields must meet the following conditions:

1. Must start with `t-`
2. Excluding `t-`, length cannot exceed 28 bytes
3. Can only contain numbers, letters, underscore(\_), hyphen(-) or dot(.)

**» action\_mode**: Processing mode:

Different fields are returned when placing an order based on action\_mode. This field is only valid during the request and is not included in the response
`ACK`: Asynchronous mode, only returns key order fields
`RESULT`: No liquidation information
`FULL`: Full mode (default)

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Order created successfully | Inline |

### Response Schema

Status Code **201**

_Spot order details_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Order ID |
| » text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » create\_time | string | Creation time of order |
| » update\_time | string | Last modification time of order |
| » create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| » update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| » status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| » currency\_pair | string | Currency pair |
| » type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| » account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | string | Buy or sell order |
| » amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| » price | string | Trading price, required when `type`=`limit` |
| » time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| » iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| » left | string | Amount left to fill |
| » filled\_amount | string | Amount filled |
| » fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| » filled\_total | string | Total filled in quote currency |
| » avg\_deal\_price | string | Average fill price |
| » fee | string | Fee deducted |
| » fee\_currency | string | Fee currency unit |
| » point\_fee | string | Points used to deduct fee |
| » gt\_fee | string | GT used to deduct fee |
| » gt\_maker\_fee | string | GT amount used to deduct maker fee |
| » gt\_taker\_fee | string | GT amount used to deduct taker fee |
| » gt\_discount | boolean | Whether GT fee deduction is enabled |
| » rebated\_fee | string | Rebated fee |
| » rebated\_fee\_currency | string | Rebated fee currency unit |
| » stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| » stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| » finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/cross_liquidate_orders'
query_param = ''
body='{"currency_pair":"GT_USDT","amount":"12","price":"10.15","text":"t-34535"}'
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
url="/spot/cross_liquidate_orders"
query_param=""
body_param='{"currency_pair":"GT_USDT","amount":"12","price":"10.15","text":"t-34535"}'
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
"currency_pair": "GT_USDT",
"amount": "12",
"price": "10.15",
"text": "t-34535"
}
```

> Example responses

> 201 Response

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
"gt_maker_fee": "0",
"gt_taker_fee": "0",
"gt_discount": false,
"rebated_fee": "0",
"rebated_fee_currency": "USDT",
"finish_as": "filled"
}
```

## Create an order

`POST /spot/orders`

_Create an order_

Supports spot, margin, leverage, and cross-margin leverage orders. Use different accounts through the `account` field. Default is `spot`, which means using the spot account to place orders. If the user has a `unified` account, the default is to place orders with the unified account.

When using leveraged account trading (i.e., when `account` is set to `margin`), you can set `auto_borrow` to `true`.
In case of insufficient account balance, the system will automatically execute `POST /margin/uni/loans` to borrow the insufficient amount.
Whether assets obtained after leveraged order execution are automatically used to repay borrowing orders of the isolated margin account depends on the automatic repayment settings of the user's isolated margin account.
Account automatic repayment settings can be queried and set through `/margin/auto_repay`.

When using unified account trading (i.e., when `account` is set to `unified`), `auto_borrow` can also be enabled to realize automatic borrowing of insufficient amounts. However, unlike the isolated margin account, whether unified account orders are automatically repaid depends on the `auto_repay` setting when placing the order. This setting only applies to the current order, meaning only assets obtained after order execution will be used to repay borrowing orders of the cross-margin account.
Unified account ordering currently supports enabling both `auto_borrow` and `auto_repay` simultaneously.

Auto repayment will be triggered when the order ends, i.e., when `status` is `cancelled` or `closed`.

**Order Status**

The order status in pending orders is `open`, which remains `open` until all quantity is filled. If fully filled, the order ends and status becomes `closed`.
If the order is cancelled before all transactions are completed, regardless of partial fills, the status will become `cancelled`.

**Iceberg Orders**

`iceberg` is used to set the displayed quantity of iceberg orders and does not support complete hiding. Note that hidden portions are charged according to the taker's fee rate.

**Self-Trade Prevention**

Set `stp_act` to determine the self-trade prevention strategy to use

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | object | true | none |
| » text | body | string | false | User defined information. If not empty, must follow the rules below: |
| » currency\_pair | body | string | true | Currency pair |
| » type | body | string | false | Order Type |
| » account | body | string | false | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | body | string | true | Buy or sell order |
| » amount | body | string | true | Trading quantity |
| » price | body | string | false | Trading price, required when `type`=`limit` |
| » time\_in\_force | body | string | false | Time in force |
| » iceberg | body | string | false | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_borrow | body | boolean | false | Used in margin or cross margin trading to allow automatic loan of insufficient amount if balance is not enough |
| » auto\_repay | body | boolean | false | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that: |
| » stp\_act | body | string | false | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies |
| » action\_mode | body | string | false | Processing Mode: |
| » slippage | body | string | false | Maximum supported slippage ratio for Spot Market Order Placement, calculated based on the latest market price at the time of order placement as the benchmark (Example: 0.03 means 3%) |

#### Detailed descriptions

**» text**: User defined information. If not empty, must follow the rules below:

1. prefixed with `t-`
2. no longer than 28 bytes without `t-` prefix
3. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)

Besides user defined information, reserved contents are listed below, denoting how the order is created:

- 101: from android
- 102: from IOS
- 103: from IPAD
- 104: from webapp
- 3: from web
- 2: from apiv2
- apiv4: from apiv4
pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders
liquidate represents isolated-margin liquidation orders

**» type**: Order Type

- limit : Limit Order
- market : Market Order

**» amount**: Trading quantity
When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`
When `type` is `market`, it refers to different currencies based on the side:

- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`
- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC`

**» time\_in\_force**: Time in force

- gtc: GoodTillCancelled
- ioc: ImmediateOrCancelled, taker only
- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee
- fok: FillOrKill, fill either completely or none
Only `ioc` and `fok` are supported when `type`=`market`

**» auto\_repay**: Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:

1. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.
2. `auto_borrow` and `auto_repay` can be both set to true in one order

**» stp\_act**: Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies

1. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.
2. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.
3. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'

- cn: Cancel newest, cancel new orders and keep old ones
- co: Cancel oldest, cancel old orders and keep new ones
- cb: Cancel both, both old and new orders will be cancelled

**» action\_mode**: Processing Mode:
When placing an order, different fields are returned based on action\_mode. This field is only valid during the request and is not included in the response result
ACK: Asynchronous mode, only returns key order fields
RESULT: No clearing information
FULL: Full mode (default)

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| » type | limit |
| » type | market |
| » side | buy |
| » side | sell |
| » time\_in\_force | gtc |
| » time\_in\_force | ioc |
| » time\_in\_force | poc |
| » time\_in\_force | fok |
| » stp\_act | cn |
| » stp\_act | co |
| » stp\_act | cb |
| » stp\_act | - |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 201 | [Created(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Order created | Inline |

### Response Schema

Status Code **201**

_Spot order details_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Order ID |
| » text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » create\_time | string | Creation time of order |
| » update\_time | string | Last modification time of order |
| » create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| » update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| » status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| » currency\_pair | string | Currency pair |
| » type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| » account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | string | Buy or sell order |
| » amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| » price | string | Trading price, required when `type`=`limit` |
| » time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| » iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_borrow | boolean | Used in margin or cross margin trading to allow automatic loan of insufficient amount if balance is not enough |
| » auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| » left | string | Amount left to fill |
| » filled\_amount | string | Amount filled |
| » fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| » filled\_total | string | Total filled in quote currency |
| » avg\_deal\_price | string | Average fill price |
| » fee | string | Fee deducted |
| » fee\_currency | string | Fee currency unit |
| » point\_fee | string | Points used to deduct fee |
| » gt\_fee | string | GT used to deduct fee |
| » gt\_maker\_fee | string | GT amount used to deduct maker fee |
| » gt\_taker\_fee | string | GT amount used to deduct taker fee |
| » gt\_discount | boolean | Whether GT fee deduction is enabled |
| » rebated\_fee | string | Rebated fee |
| » rebated\_fee\_currency | string | Rebated fee currency unit |
| » stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| » stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| » finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |
| » action\_mode | string | Processing Mode:<br>When placing an order, different fields are returned based on action\_mode. This field is only valid during the request and is not included in the response result<br>ACK: Asynchronous mode, only returns key order fields<br>RESULT: No clearing information<br>FULL: Full mode (default) |
| » slippage | string | Maximum supported slippage ratio for Spot Market Order Placement, calculated based on the latest market price at the time of order placement as the benchmark (Example: 0.03 means 3%) |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/orders'
query_param = ''
body='{"text":"t-abc123","currency_pair":"BTC_USDT","type":"limit","account":"unified","side":"buy","amount":"0.001","price":"65000","time_in_force":"gtc","iceberg":"0","slippage":"0.05"}'
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
url="/spot/orders"
query_param=""
body_param='{"text":"t-abc123","currency_pair":"BTC_USDT","type":"limit","account":"unified","side":"buy","amount":"0.001","price":"65000","time_in_force":"gtc","iceberg":"0","slippage":"0.05"}'
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
"text": "t-abc123",
"currency_pair": "BTC_USDT",
"type": "limit",
"account": "unified",
"side": "buy",
"amount": "0.001",
"price": "65000",
"time_in_force": "gtc",
"iceberg": "0",
"slippage": "0.05"
}
```

> Example responses

> ACK response body example

```json
{
"id": "12332324",
"text": "t-123456",
"amend_text": "test2"
}
```

> RESULT response body example

```json
{
"id": "12332324",
"text": "t-123456",
"create_time": "1548000000",
"update_time": "1548000100",
"create_time_ms": 1548000000123,
"update_time_ms": 1548000100123,
"currency_pair": "ETH_BTC",
"status": "cancelled",
"type": "limit",
"account": "spot",
"side": "buy",
"iceberg": "0",
"amount": "1",
"price": "5.00032",
"time_in_force": "gtc",
"auto_borrow": false,
"left": "0.5",
"filled_total": "2.50016",
"avg_deal_price": "5.00032",
"stp_act": "cn",
"finish_as": "stp",
"stp_id": 10240
}
```

> FULL response body example

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
"gt_maker_fee": "0",
"gt_taker_fee": "0",
"gt_discount": false,
"rebated_fee": "0",
"rebated_fee_currency": "USDT",
"finish_as": "filled",
"slippage": "0.05"
}
```

## List orders

`GET /spot/orders`

_List orders_

Note that query results default to spot order lists for spot, unified account, and isolated margin accounts.

When `status` is set to `open` (i.e., when querying pending order lists), only `page` and `limit` pagination controls are supported. `limit` can only be set to a maximum of 100.
The `side` parameter and time range query parameters `from` and `to` are not supported.

When `status` is set to `finished` (i.e., when querying historical orders), in addition to pagination queries, `from` and `to` time range queries are also supported.
Additionally, the `side` parameter can be set to filter one-sided history.

Time range filter parameters are processed according to the order end time.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | true | Query by specified currency pair. Required for open orders, optional for filled orders |
| status | query | string | true | List orders based on status |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | Maximum number of records to be returned. If `status` is `open`, maximum of `limit` is 100 |
| account | query | string | false | Specify query account |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |
| side | query | string | false | Specify all bids or all asks, both included if not specified |

#### Detailed descriptions

**status**: List orders based on status

`open` \- order is waiting to be filled
`finished` \- order has been filled or cancelled

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Spot order details\] |
| » _None_ | object | Spot order details |
| »» id | string | Order ID |
| »» text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| »» amend\_text | string | The custom data that the user remarked when amending the order |
| »» create\_time | string | Creation time of order |
| »» update\_time | string | Last modification time of order |
| »» create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| »» update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| »» status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| »» currency\_pair | string | Currency pair |
| »» type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| »» account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| »» side | string | Buy or sell order |
| »» amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| »» price | string | Trading price, required when `type`=`limit` |
| »» time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| »» iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| »» auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| »» left | string | Amount left to fill |
| »» filled\_amount | string | Amount filled |
| »» fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| »» filled\_total | string | Total filled in quote currency |
| »» avg\_deal\_price | string | Average fill price |
| »» fee | string | Fee deducted |
| »» fee\_currency | string | Fee currency unit |
| »» point\_fee | string | Points used to deduct fee |
| »» gt\_fee | string | GT used to deduct fee |
| »» gt\_maker\_fee | string | GT amount used to deduct maker fee |
| »» gt\_taker\_fee | string | GT amount used to deduct taker fee |
| »» gt\_discount | boolean | Whether GT fee deduction is enabled |
| »» rebated\_fee | string | Rebated fee |
| »» rebated\_fee\_currency | string | Rebated fee currency unit |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| »» finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/orders'
query_param = 'currency_pair=BTC_USDT&status=open'
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
url="/spot/orders"
query_param="currency_pair=BTC_USDT&status=open"
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
    "gt_maker_fee": "0",
    "gt_taker_fee": "0",
    "gt_discount": false,
    "rebated_fee": "0",
    "rebated_fee_currency": "USDT",
    "finish_as": "filled"
}
]
```

## Cancel all `open` orders in specified currency pair

`DELETE /spot/orders`

_Cancel all `open` orders in specified currency pair_

When the `account` parameter is not specified, all pending orders including spot, unified account, and isolated margin will be cancelled.
When `currency_pair` is not specified, all trading pair pending orders will be cancelled.
You can specify a particular account to cancel all pending orders under that account

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Currency pair |
| side | query | string | false | Specify all bids or all asks, both included if not specified |
| account | query | string | false | Specify account type |
| action\_mode | query | string | false | Processing Mode |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |

#### Detailed descriptions

**account**: Specify account type

Classic account: All are included if not specified
Unified account: Specify `unified`

**action\_mode**: Processing Mode

When placing an order, different fields are returned based on the action\_mode

- `ACK`: Asynchronous mode, returns only key order fields
- `RESULT`: No clearing information
- `FULL`: Full mode (default)

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch cancel request is received and processed. Success is determined based on the order list | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » _None_ | object | Spot order details |
| »» id | string | Order ID |
| »» text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4 |
| »» amend\_text | string | The custom data that the user remarked when amending the order |
| »» succeeded | boolean | Request execution result |
| »» label | string | Error label, if any, otherwise an empty string |
| »» message | string | Detailed error message, if any, otherwise an empty string |
| »» create\_time | string | Creation time of order |
| »» update\_time | string | Last modification time of order |
| »» create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| »» update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| »» status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| »» currency\_pair | string | Currency pair |
| »» type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| »» account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| »» side | string | Buy or sell order |
| »» amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| »» price | string | Trading price, required when `type`=`limit` |
| »» time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| »» iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| »» auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| »» left | string | Amount left to fill |
| »» filled\_amount | string | Amount filled |
| »» fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| »» filled\_total | string | Total filled in quote currency |
| »» avg\_deal\_price | string | Average fill price |
| »» fee | string | Fee deducted |
| »» fee\_currency | string | Fee currency unit |
| »» point\_fee | string | Points used to deduct fee |
| »» gt\_fee | string | GT used to deduct fee |
| »» gt\_maker\_fee | string | GT amount used to deduct maker fee |
| »» gt\_taker\_fee | string | GT amount used to deduct taker fee |
| »» gt\_discount | boolean | Whether GT fee deduction is enabled |
| »» rebated\_fee | string | Rebated fee |
| »» rebated\_fee\_currency | string | Rebated fee currency unit |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| »» finish\_as | string | How the order was finished.<br>\- open: processing<br>\- filled: filled totally<br>\- cancelled: manually cancelled<br>\- ioc: time in force is `IOC`, finish immediately<br>\- stp: cancelled because self trade prevention |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | ioc |
| finish\_as | stp |

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

url = '/spot/orders'
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
url="/spot/orders"
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
    "id": "1852454420",
    "text": "t-abc123",
    "amend_text": "-",
    "succeeded": true,
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
    "gt_maker_fee": "0",
    "gt_taker_fee": "0",
    "gt_discount": false,
    "rebated_fee": "0",
    "rebated_fee_currency": "USDT",
    "finish_as": "filled"
}
]
```

## Cancel batch orders by specified ID list

`POST /spot/cancel_batch_orders`

_Cancel batch orders by specified ID list_

Multiple currency pairs can be specified, but maximum 20 orders are allowed per request

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | array\[object\] | true | none |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch cancellation completed | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » CancelOrderResult | object | Order cancellation result |
| »» currency\_pair | string | Order currency pair |
| »» id | string | Order ID |
| »» text | string | Custom order information |
| »» succeeded | boolean | Whether cancellation succeeded |
| »» label | string | Error label when failed to cancel the order; emtpy if succeeded |
| »» message | string | Error description when cancellation fails, empty if successful |
| »» account | string | Default is empty (deprecated) |

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

url = '/spot/cancel_batch_orders'
query_param = ''
body='[{"currency_pair":"BTC_USDT","id":"123456"}]'
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
url="/spot/cancel_batch_orders"
query_param=""
body_param='[{"currency_pair":"BTC_USDT","id":"123456"}]'
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
    "currency_pair": "BTC_USDT",
    "id": "123456"
}
]
```

> Example responses

> 200 Response

```json
[
{
    "currency_pair": "BTC_USDT",
    "id": "123456",
    "text": "123456",
    "succeeded": true,
    "label": null,
    "message": null
}
]
```

## Query single order details

`GET /spot/orders/{order_id}`

_Query single order details_

By default, queries orders for spot, unified account, and isolated margin accounts.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field). |
| currency\_pair | query | string | true | Specify the trading pair to query. This field is required when querying pending order records. This field can be omitted when querying filled order records. |
| account | query | string | false | Specify query account |

#### Detailed descriptions

**order\_id**: The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field).
Operations based on custom IDs can only be checked in pending orders. Only order ID can be used after the order is finished (transaction/cancel)

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Detail retrieved | Inline |

### Response Schema

Status Code **200**

_Spot order details_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Order ID |
| » text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » create\_time | string | Creation time of order |
| » update\_time | string | Last modification time of order |
| » create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| » update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| » status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| » currency\_pair | string | Currency pair |
| » type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| » account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | string | Buy or sell order |
| » amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| » price | string | Trading price, required when `type`=`limit` |
| » time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| » iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| » left | string | Amount left to fill |
| » filled\_amount | string | Amount filled |
| » fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| » filled\_total | string | Total filled in quote currency |
| » avg\_deal\_price | string | Average fill price |
| » fee | string | Fee deducted |
| » fee\_currency | string | Fee currency unit |
| » point\_fee | string | Points used to deduct fee |
| » gt\_fee | string | GT used to deduct fee |
| » gt\_maker\_fee | string | GT amount used to deduct maker fee |
| » gt\_taker\_fee | string | GT amount used to deduct taker fee |
| » gt\_discount | boolean | Whether GT fee deduction is enabled |
| » rebated\_fee | string | Rebated fee |
| » rebated\_fee\_currency | string | Rebated fee currency unit |
| » stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| » stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| » finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/orders/12345'
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
url="/spot/orders/12345"
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
"gt_maker_fee": "0",
"gt_taker_fee": "0",
"gt_discount": false,
"rebated_fee": "0",
"rebated_fee_currency": "USDT",
"finish_as": "filled"
}
```

## Amend single order

`PATCH /spot/orders/{order_id}`

_Amend single order_

Modify orders in spot, unified account and isolated margin account by default.

Currently both request body and query support currency\_pair and account parameters, but request body has higher priority.

currency\_pair must be filled in one of the request body or query parameters.

About rate limit: Order modification and order creation share the same rate limit rules.

About matching priority: Only reducing the quantity does not affect the matching priority. Modifying the price or increasing the quantity will adjust the priority to the end of the new price level.

Note: Modifying the quantity to be less than the filled quantity will trigger a cancellation and isolated margin account by default.

Currently both request body and query support currency\_pair and account parameters, but request body has higher priority.

currency\_pair must be filled in one of the request body or query parameters.

About rate limit: Order modification and order creation share the same rate limit rules.

About matching priority: Only reducing the quantity does not affect the matching priority. Modifying the price or increasing the quantity will adjust the priority to the end of the new price level.

Note: Modifying the quantity to be less than the filled quantity will trigger a cancellation operation.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field). |
| currency\_pair | query | string | false | Currency pair |
| account | query | string | false | Specify query account |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | object | true | none |
| » currency\_pair | body | string | false | Currency pair |
| » account | body | string | false | Specify query account |
| » amount | body | string | false | Trading quantity. Either `amount` or `price` must be specified |
| » price | body | string | false | Trading price. Either `amount` or `price` must be specified |
| » amend\_text | body | string | false | Custom info during order amendment |
| » action\_mode | body | string | false | Processing Mode: |

#### Detailed descriptions

**order\_id**: The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field).
Operations based on custom IDs can only be checked in pending orders. Only order ID can be used after the order is finished (transaction/cancel)

**» action\_mode**: Processing Mode:
When placing an order, different fields are returned based on action\_mode. This field is only valid during the request and is not included in the response result
ACK: Asynchronous mode, only returns key order fields
RESULT: No clearing information
FULL: Full mode (default)

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Updated successfully | Inline |

### Response Schema

Status Code **200**

_Spot order details_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Order ID |
| » text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » create\_time | string | Creation time of order |
| » update\_time | string | Last modification time of order |
| » create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| » update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| » status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| » currency\_pair | string | Currency pair |
| » type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| » account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | string | Buy or sell order |
| » amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| » price | string | Trading price, required when `type`=`limit` |
| » time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| » iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| » left | string | Amount left to fill |
| » filled\_amount | string | Amount filled |
| » fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| » filled\_total | string | Total filled in quote currency |
| » avg\_deal\_price | string | Average fill price |
| » fee | string | Fee deducted |
| » fee\_currency | string | Fee currency unit |
| » point\_fee | string | Points used to deduct fee |
| » gt\_fee | string | GT used to deduct fee |
| » gt\_maker\_fee | string | GT amount used to deduct maker fee |
| » gt\_taker\_fee | string | GT amount used to deduct taker fee |
| » gt\_discount | boolean | Whether GT fee deduction is enabled |
| » rebated\_fee | string | Rebated fee |
| » rebated\_fee\_currency | string | Rebated fee currency unit |
| » stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| » stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| » finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/orders/12345'
query_param = ''
body='{"currency_pair":"BTC_USDT","account":"spot","amount":"1"}'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('PATCH', prefix + url, query_param, body)
headers.update(sign_headers)
r = requests.request('PATCH', host + prefix + url, headers=headers, data=body)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="PATCH"
url="/spot/orders/12345"
query_param=""
body_param='{"currency_pair":"BTC_USDT","account":"spot","amount":"1"}'
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
"account": "spot",
"amount": "1"
}
```

> Example responses

> 200 Response

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
"gt_maker_fee": "0",
"gt_taker_fee": "0",
"gt_discount": false,
"rebated_fee": "0",
"rebated_fee_currency": "USDT",
"finish_as": "filled"
}
```

## Cancel single order

`DELETE /spot/orders/{order_id}`

_Cancel single order_

By default, orders for spot, unified accounts and leveraged accounts are revoked.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field). |
| currency\_pair | query | string | true | Currency pair |
| account | query | string | false | Specify query account |
| action\_mode | query | string | false | Processing Mode |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |

#### Detailed descriptions

**order\_id**: The order ID returned when the order was successfully created or the custom ID specified by the user's creation (i.e. the `text` field).
Operations based on custom IDs can only be checked in pending orders. Only order ID can be used after the order is finished (transaction/cancel)

**action\_mode**: Processing Mode

When placing an order, different fields are returned based on the action\_mode

- `ACK`: Asynchronous mode, returns only key order fields
- `RESULT`: No clearing information
- `FULL`: Full mode (default)

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order cancelled | Inline |

### Response Schema

Status Code **200**

_Spot order details_

| Name | Type | Description |
| --- | --- | --- |
| » id | string | Order ID |
| » text | string | User defined information. If not empty, must follow the rules below:<br>1\. prefixed with `t-`<br>2\. no longer than 28 bytes without `t-` prefix<br>3\. can only include 0-9, A-Z, a-z, underscore(\_), hyphen(-) or dot(.)<br>Besides user defined information, reserved contents are listed below, denoting how the order is created:<br>\- 101: from android<br>\- 102: from IOS<br>\- 103: from IPAD<br>\- 104: from webapp<br>\- 3: from web<br>\- 2: from apiv2<br>\- apiv4: from apiv4<br>pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent cross-margin liquidation orders<br>liquidate represents isolated-margin liquidation orders |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » create\_time | string | Creation time of order |
| » update\_time | string | Last modification time of order |
| » create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| » update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| » status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| » currency\_pair | string | Currency pair |
| » type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| » account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| » side | string | Buy or sell order |
| » amount | string | Trading quantity<br>When `type` is `limit`, it refers to the base currency (the currency being traded), such as `BTC` in `BTC_USDT`<br>When `type` is `market`, it refers to different currencies based on the side:<br>\- `side`: `buy` refers to quote currency, `BTC_USDT` means `USDT`<br>\- `side`: `sell` refers to base currency, `BTC_USDT` means `BTC` |
| » price | string | Trading price, required when `type`=`limit` |
| » time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none<br>Only `ioc` and `fok` are supported when `type`=`market` |
| » iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| » auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| » left | string | Amount left to fill |
| » filled\_amount | string | Amount filled |
| » fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| » filled\_total | string | Total filled in quote currency |
| » avg\_deal\_price | string | Average fill price |
| » fee | string | Fee deducted |
| » fee\_currency | string | Fee currency unit |
| » point\_fee | string | Points used to deduct fee |
| » gt\_fee | string | GT used to deduct fee |
| » gt\_maker\_fee | string | GT amount used to deduct maker fee |
| » gt\_taker\_fee | string | GT amount used to deduct taker fee |
| » gt\_discount | boolean | Whether GT fee deduction is enabled |
| » rebated\_fee | string | Rebated fee |
| » rebated\_fee\_currency | string | Rebated fee currency unit |
| » stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| » stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevention strategies<br>1\. After users join the `STP Group`, they can pass `stp_act` to limit the user's self-trade prevention strategy. If `stp_act` is not passed, the default is `cn` strategy.<br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter.<br>3\. If the user did not use `stp_act` when placing the order, `stp_act` will return '-'<br>\- cn: Cancel newest, cancel new orders and keep old ones<br>\- co: Cancel oldest, cancel old orders and keep new ones<br>\- cb: Cancel both, both old and new orders will be cancelled |
| » finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/orders/12345'
query_param = 'currency_pair=BTC_USDT'
# for gen_sign implementation, refer to Authentication section
sign_headers = gen_sign('DELETE', prefix + url, query_param)
headers.update(sign_headers)
r = requests.request('DELETE', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell
key="YOUR_API_KEY"
secret="YOUR_API_SECRET"
host="https://api.gateio.ws"
prefix="/api/v4"
method="DELETE"
url="/spot/orders/12345"
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
"gt_maker_fee": "0",
"gt_taker_fee": "0",
"gt_discount": false,
"rebated_fee": "0",
"rebated_fee_currency": "USDT",
"finish_as": "filled"
}
```

## Query personal trading records

`GET /spot/my_trades`

_Query personal trading records_

By default query of transaction records for spot, unified account and warehouse-by-site leverage accounts.

The history within a specified time range can be queried by specifying `from` or (and) `to`.

- If no time parameters are specified, only data for the last 7 days can be obtained.
- If only any parameter of `from` or `to` is specified, only 7-day data from the start (or end) of the specified time is returned.
- The range not allowed to exceed 30 days.

The parameters of the time range filter are processed according to the order end time.

The maximum number of pages when searching data using limit&page paging function is 100,0, that is, limit \* (page - 1) <= 100,0.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| currency\_pair | query | string | false | Retrieve results with specified currency pair |
| limit | query | integer | false | Maximum number of items returned in list. Default: 100, minimum: 1, maximum: 1000 |
| page | query | integer(int32) | false | Page number |
| order\_id | query | string | false | Filter trades with specified order ID. `currency_pair` is also required if this field is present |
| account | query | string | false | The accountparameter has been deprecated. The interface supports querying all transaction records of the account. |
| from | query | integer(int64) | false | Start timestamp for the query |
| to | query | integer(int64) | false | End timestamp for the query, defaults to current time if not specified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | none |
| » id | string | Fill ID |
| » create\_time | string | Fill Time |
| » create\_time\_ms | string | Trading time, with millisecond precision |
| » currency\_pair | string | Currency pair |
| » side | string | Buy or sell order |
| » role | string | Trade role, not returned in public endpoints |
| » amount | string | Trade amount |
| » price | string | Order price |
| » order\_id | string | Related order ID, not returned in public endpoints |
| » fee | string | Fee deducted, not returned in public endpoints |
| » fee\_currency | string | Fee currency unit, not returned in public endpoints |
| » point\_fee | string | Points used to deduct fee, not returned in public endpoints |
| » gt\_fee | string | GT used to deduct fee, not returned in public endpoints |
| » amend\_text | string | The custom data that the user remarked when amending the order |
| » sequence\_id | string | Consecutive trade ID within a single market.<br>Used to track and identify trades in the specific market |
| » text | string | Order's Custom Information. This field is not returned by public interfaces.<br>The scenarios pm\_liquidate, comb\_margin\_liquidate, and scm\_liquidate represent full-account forced liquidation orders.<br>liquidate represents isolated-account forced liquidation orders. |
| » deal | string | Total Executed Value |

#### Enumerated Values

| Property | Value |
| --- | --- |
| side | buy |
| side | sell |
| role | taker |
| role | maker |

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

url = '/spot/my_trades'
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
url="/spot/my_trades"
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
    "id": "1232893232",
    "create_time": "1548000000",
    "create_time_ms": "1548000000123.456",
    "order_id": "4128442423",
    "side": "buy",
    "role": "maker",
    "amount": "0.15",
    "price": "0.03",
    "fee": "0.0005",
    "fee_currency": "ETH",
    "point_fee": "0",
    "gt_fee": "0",
    "sequence_id": "588018",
    "text": "t-test",
    "deal": "0.0045"
}
]
```

## Get server current time

`GET /spot/time`

_Get server current time_

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | [SystemTime](https://www.gate.com/docs/developers/apiv4/en/#schemasystemtime) |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/time'
query_param = ''
r = requests.request('GET', host + prefix + url, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/time 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
{
"server_time": 1597026383085
}
```

## Countdown cancel orders

`POST /spot/countdown_cancel_all`

_Countdown cancel orders_

Spot order heartbeat detection. If there is no "cancel existing countdown" or "set new countdown" when the user-set `timeout` time is reached, the related `spot pending orders` will be automatically cancelled.
This interface can be called repeatedly to set a new countdown or cancel the countdown.
Usage example: Repeat this interface at 30s intervals, setting the countdown `timeout` to `30 (seconds)` each time.
If this interface is not called again within 30 seconds, all pending orders on the `market` you specified will be automatically cancelled. If no `market` is specified, all market cancelled.
If the `timeout` is set to 0 within 30 seconds, the countdown timer will be terminated and the automatic order cancellation function will be cancelled.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [CountdownCancelAllSpotTask](https://www.gate.com/docs/developers/apiv4/en/#schemacountdowncancelallspottask) | true | none |
| » timeout | body | integer(int32) | true | Countdown time in seconds |
| » currency\_pair | body | string | false | Currency pair |

#### Detailed descriptions

**» timeout**: Countdown time in seconds
At least 5 seconds, 0 means cancel countdown

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

url = '/spot/countdown_cancel_all'
query_param = ''
body='{"timeout":30,"currency_pair":"BTC_USDT"}'
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
url="/spot/countdown_cancel_all"
query_param=""
body_param='{"timeout":30,"currency_pair":"BTC_USDT"}'
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
"currency_pair": "BTC_USDT"
}
```

> Example responses

> 200 Response

```json
{
"triggerTime": "1660039145000"
}
```

## Batch modification of orders

`POST /spot/amend_batch_orders`

_Batch modification of orders_

Modify orders in spot, unified account and isolated margin account by default.
Modify uncompleted orders, up to 5 orders can be modified at a time. Request parameters should be passed in array format.
If there are order modification failures during the batch modification process, the modification of the next order will continue to be executed, and the execution will return with the corresponding order failure information.
The call order of batch modification orders is consistent with the order list order.
The return content order of batch modification orders is consistent with the order list order.

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| x-gate-exptime | header | string | false | Specify the expiration time (milliseconds); if the GATE receives the request time greater than the expiration time, the request will be rejected |
| body | body | array\[object\] | true | none |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Order modification executed successfully | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| _None_ | array | \[Batch order details\] |
| » _None_ | object | Batch order details |
| »» order\_id | string | Order ID |
| »» amend\_text | string | The custom data that the user remarked when amending the order |
| »» text | string | Order custom information. Users can set custom ID with this field. Custom fields must meet the following conditions:<br>1\. Must start with `t-`<br>2\. Excluding `t-`, length cannot exceed 28 bytes<br>3\. Can only contain numbers, letters, underscore(\_), hyphen(-) or dot(.) |
| »» succeeded | boolean | Request execution result |
| »» label | string | Error label, if any, otherwise an empty string |
| »» message | string | Detailed error message, if any, otherwise an empty string |
| »» id | string | Order ID |
| »» create\_time | string | Creation time of order |
| »» update\_time | string | Last modification time of order |
| »» create\_time\_ms | integer(int64) | Creation time of order (in milliseconds) |
| »» update\_time\_ms | integer(int64) | Last modification time of order (in milliseconds) |
| »» status | string | Order status<br>\- `open`: to be filled<br>\- `closed`: closed order<br>\- `cancelled`: cancelled |
| »» currency\_pair | string | Currency pair |
| »» type | string | Order Type <br>\- limit : Limit Order<br>\- market : Market Order |
| »» account | string | Account type, spot - spot account, margin - leveraged account, unified - unified account |
| »» side | string | Buy or sell order |
| »» amount | string | Trade amount |
| »» price | string | Order price |
| »» time\_in\_force | string | Time in force<br>\- gtc: GoodTillCancelled<br>\- ioc: ImmediateOrCancelled, taker only<br>\- poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee<br>\- fok: FillOrKill, fill either completely or none |
| »» iceberg | string | Amount to display for the iceberg order. Null or 0 for normal orders. Hiding all amount is not supported |
| »» auto\_repay | boolean | Enable or disable automatic repayment for automatic borrow loan generated by cross margin order. Default is disabled. Note that:<br>1\. This field is only effective for cross margin orders. Margin account does not support setting auto repayment for orders.<br>2\. `auto_borrow` and `auto_repay` can be both set to true in one order |
| »» left | string | Amount left to fill |
| »» filled\_amount | string | Amount filled |
| »» fill\_price | string | Total filled in quote currency. Deprecated in favor of `filled_total` |
| »» filled\_total | string | Total filled in quote currency |
| »» avg\_deal\_price | string | Average fill price |
| »» fee | string | Fee deducted |
| »» fee\_currency | string | Fee currency unit |
| »» point\_fee | string | Points used to deduct fee |
| »» gt\_fee | string | GT used to deduct fee |
| »» gt\_discount | boolean | Whether GT fee deduction is enabled |
| »» rebated\_fee | string | Rebated fee |
| »» rebated\_fee\_currency | string | Rebated fee currency unit |
| »» stp\_id | integer | Orders between users in the same `stp_id` group are not allowed to be self-traded<br>1\. If the `stp_id` of two orders being matched is non-zero and equal, they will not be executed. Instead, the corresponding strategy will be executed based on the `stp_act` of the taker.<br>2\. `stp_id` returns `0` by default for orders that have not been set for `STP group` |
| »» stp\_act | string | Self-Trading Prevention Action. Users can use this field to set self-trade prevetion strategies<br>1\. After users join the `STP Group`, he can pass `stp_act` to limit the user's self-trade prevetion strategy. If `stp_act` is not passed, the default is `cn` strategy. <br>2\. When the user does not join the `STP group`, an error will be returned when passing the `stp_act` parameter. <br>3\. If the user did not use 'stp\_act' when placing the order, 'stp\_act' will return '-'<br>\- cn: Cancel newest, Cancel new orders and keep old ones<br>\- co: Cancel oldest, new ones<br>\- cb: Cancel both, Both old and new orders will be cancelled |
| »» finish\_as | string | Order finish status, including:<br>\- open: Pending<br>\- filled: Fully executed<br>\- cancelled: Cancelled by user<br>\- liquidate\_cancelled: Cancelled due to liquidation<br>\- small: Order size too small<br>\- depth\_not\_enough: Cancelled due to insufficient market depth<br>\- trader\_not\_enough: Cancelled due to insufficient counterparty<br>\- ioc: Not immediately filled, due to time-in-force set to IOC<br>\- poc: Maker-only policy not met, due to time-in-force set to POC/RVT/RAT/RPI (post-only orders rejected when would take liquidity)<br>\- fok: Not immediately fully executed, due to time-in-force set to FOK<br>\- stp: Cancelled due to self-trade prevention trigger<br>\- price\_protect\_cancelled: Cancelled due to price protection mechanism<br>\- unknown: Unknown status |

#### Enumerated Values

| Property | Value |
| --- | --- |
| status | open |
| status | closed |
| status | cancelled |
| type | limit |
| type | market |
| account | spot |
| account | margin |
| account | cross\_margin |
| account | unified |
| side | buy |
| side | sell |
| time\_in\_force | gtc |
| time\_in\_force | ioc |
| time\_in\_force | poc |
| time\_in\_force | fok |
| stp\_act | cn |
| stp\_act | co |
| stp\_act | cb |
| stp\_act | - |
| finish\_as | open |
| finish\_as | filled |
| finish\_as | cancelled |
| finish\_as | liquidate\_cancelled |
| finish\_as | depth\_not\_enough |
| finish\_as | trader\_not\_enough |
| finish\_as | small |
| finish\_as | ioc |
| finish\_as | poc |
| finish\_as | fok |
| finish\_as | stp |
| finish\_as | price\_protect\_cancelled |
| finish\_as | unknown |

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

url = '/spot/amend_batch_orders'
query_param = ''
body='[{"order_id":"121212","currency_pair":"BTC_USDT","account":"spot","amount":"1","amend_text":"test"}]'
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
url="/spot/amend_batch_orders"
query_param=""
body_param='[{"order_id":"121212","currency_pair":"BTC_USDT","account":"spot","amount":"1","amend_text":"test"}]'
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
    "order_id": "121212",
    "currency_pair": "BTC_USDT",
    "account": "spot",
    "amount": "1",
    "amend_text": "test"
}
]
```

> Example responses

> 200 Response

```json
[
{
    "order_id": "12332324",
    "amend_text": "t-123456",
    "text": "t-123456",
    "succeeded": true,
    "label": "",
    "message": "",
    "id": "12332324",
    "create_time": "1548000000",
    "update_time": "1548000100",
    "create_time_ms": 1548000000123,
    "update_time_ms": 1548000100123,
    "currency_pair": "ETC_BTC",
    "status": "cancelled",
    "type": "limit",
    "account": "spot",
    "side": "buy",
    "amount": "1",
    "price": "5.00032",
    "time_in_force": "gtc",
    "iceberg": "0",
    "left": "0.5",
    "filled_amount": "1.242",
    "filled_total": "2.50016",
    "avg_deal_price": "5.00032",
    "fee": "0.005",
    "fee_currency": "ETH",
    "point_fee": "0",
    "gt_fee": "0",
    "gt_discount": false,
    "rebated_fee": "0",
    "rebated_fee_currency": "BTC",
    "stp_act": "cn",
    "finish_as": "stp",
    "stp_id": 10240
}
]
```

## Query spot insurance fund historical data

`GET /spot/insurance_history`

_Query spot insurance fund historical data_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| business | query | string | true | Leverage business, margin - position by position; unified - unified account |
| currency | query | string | true | Currency |
| page | query | integer(int32) | false | Page number |
| limit | query | integer | false | The maximum number of items returned in the list, the default value is 30 |
| from | query | integer(int64) | true | Start timestamp in seconds |
| to | query | integer(int64) | true | End timestamp in seconds |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Query successful | \[Inline\] |

### Response Schema

Status Code **200**

| Name | Type | Description |
| --- | --- | --- |
| » currency | string | Currency |
| » balance | string | Balance |
| » time | integer(int64) | Creation time, timestamp, milliseconds |

This operation does not require authentication

> Code samples

```python
# coding: utf-8
import requests

host = "https://api.gateio.ws"
prefix = "/api/v4"
headers = {'Accept': 'application/json', 'Content-Type': 'application/json'}

url = '/spot/insurance_history'
query_param = 'business=margin&currency=BTC&from=1547706332&to=1547706332'
r = requests.request('GET', host + prefix + url + "?" + query_param, headers=headers)
print(r.json())
```

```shell

curl -X GET https://api.gateio.ws/api/v4/spot/insurance_history?business=margin&currency=BTC&from=1547706332&to=1547706332 
  -H 'Accept: application/json'
```

> Example responses

> 200 Response

```json
[
{
    "currency": "BTC",
    "balance": "1021.21",
    "time": 1727054547
}
]
```

## Create price-triggered order

`POST /spot/price_orders`

_Create price-triggered order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| body | body | [SpotPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaspotpricetriggeredorder) | true | none |
| » trigger | body | object | true | none |
| »» price | body | string | true | Trigger price |
| »» rule | body | string | true | Price trigger condition |
| »» expiration | body | integer | false | Maximum wait time for trigger condition (in seconds). Order will be cancelled if timeout |
| » put | body | object | true | none |
| »» type | body | string | false | Order type, default to `limit` |
| »» side | body | string | true | Order side |
| »» price | body | string | true | Order price |
| »» amount | body | string | true | Trading quantity, refers to the trading quantity of the trading currency, i.e., the currency that needs to be traded, for example, the quantity of BTC in BTC\_USDT. |
| »» account | body | string | true | Trading account type. Unified account must be set to `unified` |
| »» time\_in\_force | body | string | false | time\_in\_force |
| »» auto\_borrow | body | boolean | false | Whether to borrow coins automatically |
| »» auto\_repay | body | boolean | false | Whether to repay the loan automatically |
| »» text | body | string | false | The source of the order, including: |
| » market | body | string | true | Market |

#### Detailed descriptions

**»» rule**: Price trigger condition

- `>=`: triggered when market price is greater than or equal to `price`
- `<=`: triggered when market price is less than or equal to `price`

**»» type**: Order type, default to `limit`

- limit : Limit Order
- market : Market Order

**»» side**: Order side

- buy: buy side
- sell: sell side

**»» account**: Trading account type. Unified account must be set to `unified`

- normal: spot trading
- margin: margin trading
- unified: unified account

**»» time\_in\_force**: time\_in\_force

- gtc: GoodTillCancelled
- ioc: ImmediateOrCancelled, taker only

**»» text**: The source of the order, including:

- web: Web
- api: API call
- app: Mobile app

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| »» rule | >= |
| »» rule | <= |
| »» type | limit |
| »» type | market |
| »» side | buy |
| »» side | sell |
| »» account | normal |
| »» account | margin |
| »» account | unified |
| »» time\_in\_force | gtc |
| »» time\_in\_force | ioc |

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

url = '/spot/price_orders'
query_param = ''
body='{"trigger":{"price":"100","rule":">=","expiration":3600},"put":{"type":"limit","side":"buy","price":"2.15","amount":"2.00000000","account":"normal","time_in_force":"gtc","text":"api"},"market":"GT_USDT"}'
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
url="/spot/price_orders"
query_param=""
body_param='{"trigger":{"price":"100","rule":">=","expiration":3600},"put":{"type":"limit","side":"buy","price":"2.15","amount":"2.00000000","account":"normal","time_in_force":"gtc","text":"api"},"market":"GT_USDT"}'
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
"trigger": {
    "price": "100",
    "rule": ">=",
    "expiration": 3600
},
"put": {
    "type": "limit",
    "side": "buy",
    "price": "2.15",
    "amount": "2.00000000",
    "account": "normal",
    "time_in_force": "gtc",
    "text": "api"
},
"market": "GT_USDT"
}
```

> Example responses

> 201 Response

```json
{
"id": 1432329
}
```

## Query running auto order list

`GET /spot/price_orders`

_Query running auto order list_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| status | query | string | true | Query order list based on status |
| market | query | string | false | Trading market |
| account | query | string | false | Trading account type. Unified account must be set to `unified` |
| limit | query | integer | false | Maximum number of records returned in a single list |
| offset | query | integer | false | List offset, starting from 0 |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| status | open |
| status | finished |
| account | normal |
| account | margin |
| account | unified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | List retrieved successfully | \[ [SpotPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaspotpricetriggeredorder)\] |

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

url = '/spot/price_orders'
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
url="/spot/price_orders"
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
    "trigger": {
      "price": "100",
      "rule": ">=",
      "expiration": 3600
    },
    "put": {
      "type": "limit",
      "side": "buy",
      "price": "2.15",
      "amount": "2.00000000",
      "account": "normal",
      "time_in_force": "gtc",
      "text": "api"
    },
    "id": 1283293,
    "user": 1234,
    "market": "GT_USDT",
    "ctime": 1616397800,
    "ftime": 1616397801,
    "fired_order_id": 0,
    "status": "",
    "reason": ""
}
]
```

## Cancel all auto orders

`DELETE /spot/price_orders`

_Cancel all auto orders_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| market | query | string | false | Trading market |
| account | query | string | false | Trading account type. Unified account must be set to `unified` |

#### Enumerated Values

| Parameter | Value |
| --- | --- |
| account | normal |
| account | margin |
| account | unified |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Batch cancel request is received and processed. Success is determined based on the order list | \[ [SpotPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaspotpricetriggeredorder)\] |

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

url = '/spot/price_orders'
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
url="/spot/price_orders"
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
    "trigger": {
      "price": "100",
      "rule": ">=",
      "expiration": 3600
    },
    "put": {
      "type": "limit",
      "side": "buy",
      "price": "2.15",
      "amount": "2.00000000",
      "account": "normal",
      "time_in_force": "gtc",
      "text": "api"
    },
    "id": 1283293,
    "user": 1234,
    "market": "GT_USDT",
    "ctime": 1616397800,
    "ftime": 1616397801,
    "fired_order_id": 0,
    "status": "",
    "reason": ""
}
]
```

## Query single auto order details

`GET /spot/price_orders/{order_id}`

_Query single auto order details_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | ID returned when order is successfully created |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Auto order details | [SpotPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaspotpricetriggeredorder) |

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

url = '/spot/price_orders/string'
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
url="/spot/price_orders/string"
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
"trigger": {
    "price": "100",
    "rule": ">=",
    "expiration": 3600
},
"put": {
    "type": "limit",
    "side": "buy",
    "price": "2.15",
    "amount": "2.00000000",
    "account": "normal",
    "time_in_force": "gtc",
    "text": "api"
},
"id": 1283293,
"user": 1234,
"market": "GT_USDT",
"ctime": 1616397800,
"ftime": 1616397801,
"fired_order_id": 0,
"status": "",
"reason": ""
}
```

## Cancel single auto order

`DELETE /spot/price_orders/{order_id}`

_Cancel single auto order_

### Parameters

| Name | In | Type | Required | Description |
| --- | --- | --- | --- | --- |
| order\_id | path | string | true | ID returned when order is successfully created |

### Responses

| Status | Meaning | Description | Schema |
| --- | --- | --- | --- |
| 200 | [OK(opens new window)](https://tools.ietf.org/html/rfc7231#section-6.3.1) | Auto order details | [SpotPriceTriggeredOrder](https://www.gate.com/docs/developers/apiv4/en/#schemaspotpricetriggeredorder) |

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

url = '/spot/price_orders/string'
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
url="/spot/price_orders/string"
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
"trigger": {
    "price": "100",
    "rule": ">=",
    "expiration": 3600
},
"put": {
    "type": "limit",
    "side": "buy",
    "price": "2.15",
    "amount": "2.00000000",
    "account": "normal",
    "time_in_force": "gtc",
    "text": "api"
},
"id": 1283293,
"user": 1234,
"market": "GT_USDT",
"ctime": 1616397800,
"ftime": 1616397801,
"fired_order_id": 0,
"status": "",
"reason": ""
}
```