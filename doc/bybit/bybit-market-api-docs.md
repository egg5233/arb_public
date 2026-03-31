# Bybit Market Data API

Source: https://bybit-exchange.github.io/docs/v5/market/

---

# Get ADL Alert

Query for [ADL](https://www.bybit.com/en/help-center/article/Auto-Deleveraging-ADL) (auto-deleveraging mechanism) alerts and insurance pool information.

> **Covers: USDT Perpetual / USDT Delivery / USDC Perpetual / USDC Delivery / Inverse Contracts**

tip

Data update frequency: every 1 minute.

info

- **ADL trigger and stop conditions are based on the following three cases:**

1. **Contract PnL drawdown ADL (based on the new grouped insurance pool mechanism, see examples 1 and 2)**

   - **Trigger condition**:

     `balance` (insurance fund balance) > `adlTriggerThreshold` (trigger threshold for contract PnL drawdown ADL)

     and `pnlRatio` < `insurancePnlRatio` (PnL ratio threshold for triggering ADL)

     Where:

     - **pnlRatio**: drawdown ratio of the symbol in the last 8 hours

       Formula: `pnlRatio` = (Symbol's current PnL - Symbol's 8h max PnL) / Insurance pool's 8h max balance (`maxBalance`)

       _Note: the symbol's Current PnL and 8h Max PnL are not provided by the API_.
     - **Insurance pool 8h max balance (`maxBalance`)**: the maximum balance of the grouped insurance pool in the last 8 hours
   - **Stop condition**:
     `pnlRatio` \> `adlStopRatio` (stop ratio threshold for ADL)
2. **Insurance pool equity drawdown ADL (original mechanism, see example 3)**

   - **Trigger condition**:
     `balance` (insurance fund balance) ≤ 0
   - **Stop condition**:
     `balance` (insurance fund balance) > 0
3. **Excessive margin loss of a symbol after removing it from a grouped insurance pool (can be regarded as a special case of pool equity drawdown ADL)**

   - To ensure pool safety, the risk control team may remove a symbol from its grouped pool and temporarily establish it as a new independent insurance pool.
   - **Trigger condition**:
     `balance` (insurance fund balance) ≤ 0
   - **Stop condition**:
     `balance` (insurance fund balance) > 0

ADL examples: Triggered by PnL Drawdown and Insurance Pool Balance

1. **Example 1: Pool has no significant profit in the last 8 hours, then symbol loss exceeds the PnL ratio threshold (`insurancePnlRatio`), ADL will be triggered**

   - Assume symbols A, B, and C share the same pool with an initial 8h `balance` of **1M USDT**
   - A incurs a loss of **350K**
   - Calculation:
     - `pnlRatio` = -35%
     - `balance` = 1M
     - `adlTriggerThreshold` = 1 (a constant set by Bybit)
     - `insurancePnlRatio` = -0.3 (a constant set by Bybit)
   - Condition check:
     - `balance` (1M) > `adlTriggerThreshold` (1)
     - `pnlRatio` (-0.35) < `insurancePnlRatio` (-0.3)
   - → Contract PnL drawdown ADL is triggered
   - The system calculates the bankruptcy price at **-30% drawdown** so ADL closes **50K** worth of user positions to keep A's `pnlRatio` at -30%
   - **Stop condition**: ADL stops if A's `pnlRatio` \> `adlStopRatio` (-0.25, a constant set by Bybit)

**Recovery methods**:

1. Platform injects funds into the pool and adjusts A's PnL
2. Pool continues to take A's positions and earns maintenance margin through liquidation on the market

* * *

2. **Example 2: Pool has significant profit in the last 8 hours, but symbol loss exceeds the PnL ratio threshold (`insurancePnlRatio`), ADL will still be triggered**

   - Assume symbols A, B, C share the same pool, initial `balance` = **1M USDT**
   - A gains profit through liquidation, pool 8h Max Balance = **2M USDT** (A's PnL = +1M)
   - Later A incurs a loss of **600K**
   - Calculation:
     - `pnlRatio` = -30%
     - `balance` = 2M
     - `adlTriggerThreshold` = 1 (a constant set by Bybit)
     - `insurancePnlRatio` = -0.3 (a constant set by Bybit)
   - Condition check:
     - `balance` (2M) > `adlTriggerThreshold` (1)
     - `pnlRatio` (-0.30) ≤ `insurancePnlRatio` (-0.3)
   - → Contract PnL drawdown ADL is triggered
   - The system calculates the bankruptcy price at **-30% drawdown**
   - **Stop condition**: ADL stops if A's `pnlRatio` \> `adlStopRatio` (-0.25, a constant set by Bybit)

**Recovery methods**:

1. Platform injects funds into the pool and adjusts A's PnL
2. Pool continues to take A's positions and earns maintenance margin through liquidation on the market

* * *

3. **Example 3: Pool balance reaches zero which triggers ADL**
   - Assume symbols A, B, C, D share the same pool, initial `balance` = **1M USDT**
   - Although none of the `pnlRatio` values for the symbols reach -30%, the pool `balance` drops to 0
   - Condition check:
     - `balance` (0) ≤ 0
   - → Insurance pool equity ADL is triggered
   - The system redistributes bankruptcy loss across symbols based on their PnL when pool balance = 0
   - **Stop condition**: ADL stops if `balance` \> 0

Subscribe to the [ADL WebSocket topic](https://bybit-exchange.github.io/docs/v5/websocket/public/adl-alert) for faster updates.

### HTTP Request

GET`/v5/market/adlAlert`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| symbol | false | string | Contract name, e.g. `BTCUSDT`. Uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| updateTime | string | Latest data update timestamp (ms) |
| list | array | Object |
| \> coin | string | Token of the insurance pool |
| \> symbol | string | Trading pair name |
| \> balance | string | Balance of the insurance fund. Used to determine if ADL is triggered. For shared insurance pool, the "balance" field will follow a T+1 refresh mechanism and will be updated daily at 00:00 UTC. |
| \> maxBalance | string | Deprecated, always return "". Maximum balance of the insurance pool in the last 8 hours |
| \> insurancePnlRatio | string | PnL ratio threshold for triggering **contract PnL drawdown ADL** <br>- ADL is triggered when the symbol's PnL drawdown ratio in the last 8 hours exceeds this value |
| \> pnlRatio | string | Symbol's PnL drawdown ratio in the last 8 hours. Used to determine whether ADL is triggered or stopped |
| \> adlTriggerThreshold | string | Trigger threshold for **contract PnL drawdown ADL** <br>- This condition is only effective when the insurance pool balance is greater than this value; if so, an 8 hours drawdown exceeding n% may trigger ADL |
| \> adlStopRatio | string | Stop ratio threshold for **contract PnL drawdown ADL** <br>- ADL stops when the symbol's 8 hours drawdown ratio falls below this value |

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/adlAlert&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_adl_alert(
    symbol="BTCUSDT"
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "updatedTime": "1757733960000",
        "list": [
            {
                "coin": "USDT",
                "symbol": "BTCUSDT",
                "balance": "92203504694.99632",
                "maxBalance": "92231510324.75948",
                "insurancePnlRatio": "-0.3",
                "pnlRatio": "-0.560973",
                "adlTriggerThreshold": "10000",
                "adlStopRatio": "-0.25"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1757734022014
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Delivery Price

Get the delivery price.

> **Covers: USDT futures / USDC futures / Inverse futures / Option**

info

- Option: only returns those symbols which are `DELIVERING` (UTC 8 - UTC 12) when `symbol` is not specified.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/delivery-price`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| baseCoin | false | string | Base coin, uppercase only. Default: `BTC`. _Valid for `option` only_ |
| settleCoin | false | string | Settle coin, uppercase only. Default: `USDC`. |
| limit | false | integer | Limit for data size per page. \[`1`, `200`\]. Default: `50` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> deliveryPrice | string | Delivery price |
| \> deliveryTime | string | Delivery timestamp (ms) |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/delivery-price)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/delivery-price?category=option&symbol=ETH-26DEC22-1400-C HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP()
print(session.get_option_delivery_price(
    category="option",
    symbol="ETH-26DEC22-1400-C",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "ETH-26DEC22-1400-C"}
client.NewUtaBybitServiceWithParams(params).GetDeliveryPrice(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var deliveryPriceRequest = MarketDataRequest.builder().category(CategoryType.OPTION).baseCoin("BTC").limit(10).build();
client.getDeliveryPrice(deliveryPriceRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getDeliveryPrice({ category: 'option', symbol: 'ETH-26DEC22-1400-C' })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "category": "option",
        "nextPageCursor": "",
        "list": [
            {
                "symbol": "ETH-26DEC22-1400-C",
                "deliveryPrice": "1220.728594450",
                "deliveryTime": "1672041600000"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672055336993
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Fee Group Structure

Query for the [group fee structure](https://www.bybit.com/en/help-center/article/Group-Fee-Structure-Symbol-Grouping) and fee rates.

note

The new grouped fee structure only applies to Pro-level and Market Maker clients. It does not apply to retail traders.

For more details please refer to the [fee structure update announcement](https://announcements.bybit.com/article/bybit-fee-structure-update-for-pro-and-market-maker-clients-blt06875b6d623e7581/).

> **Covers: USDT Perpetual / USDT Delivery / USDC Perpetual / USDC Delivery / Inverse Contracts**

info

- **Weighted maker volume** = Σ(Maker volume on pair × Group weighting factor (`weightingFactor`))
- **Weighted maker share** = (Your total weighted maker volume ÷ Bybit's total weighted maker volume)
- _Note: Bybit's total weighted maker volume is not provided by the API. Weighted maker share will be provided in the monthly MM report_.

### HTTP Request

GET`/v5/market/fee-group-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| productType | **true** | string | Product type. `contract` only |
| [groupId](https://bybit-exchange.github.io/docs/v5/enum#groupid) | false | string | Group ID. `1`, `2`, `3`, `4`, `5`, `6`, `7`, `8` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | :-- |
| list | array | List of fee group objects |
| \> [groupName](https://bybit-exchange.github.io/docs/v5/enum#groupname) | string | Fee group name |
| \> weightingFactor | integer | Group weighting factor |
| \> symbolsNumbers | integer | Symbols number |
| \> symbols | array | Symbol name |
| \> feeRates | object | Fee rate details for different categories. `pro`, `marketMaker` |
| >\> pro | array | Pro-level fee structures |
| >>\> level | string | Pro level name. `Pro 1`, `Pro 2`, `Pro 3`, `Pro 4`, `Pro 5`, `Pro 6` |
| >>\> takerFeeRate | string | Taker fee rate |
| >>\> makerFeeRate | string | Maker fee rate |
| >>\> makerRebate | string | Maker rebate fee rate |
| >\> marketMaker | array | Market Maker-level fee structures |
| >>\> level | string | Market Maker level name. `MM 1`, `MM 2`, `MM 3` |
| >>\> takerFeeRate | string | Taker fee rate |
| >>\> makerFeeRate | string | Maker fee rate |
| >>\> makerRebate | string | Maker rebate fee rate |
| \> updateTime | string | Latest data update timestamp (ms) |

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/fee-group-info?productType=contract&groupId=1 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_fee_group_info(
    productType="contract",
    groupId="1"
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "groupName": "G1(Major Coins)",
                "weightingFactor": 1,
                "symbolsNumbers": 4,
                "symbols": [
                    "ETHUSDT",
                    "XRPUSDT",
                    "SOLUSDT",
                    "BTCUSDT"
                ],
                "feeRates": {
                    "pro": [
                        {
                            "level": "Pro 1",
                            "takerFeeRate": "0.00028",
                            "makerFeeRate": "0.0001",
                            "makerRebate": ""
                        },
                        {
                            "level": "Pro 2",
                            "takerFeeRate": "0.00025",
                            "makerFeeRate": "0.00005",
                            "makerRebate": ""
                        },
                        {
                            "level": "Pro 3",
                            "takerFeeRate": "0.00022",
                            "makerFeeRate": "0.000025",
                            "makerRebate": ""
                        },
                        {
                            "level": "Pro 4",
                            "takerFeeRate": "0.0002",
                            "makerFeeRate": "0.00001",
                            "makerRebate": ""
                        },
                        {
                            "level": "Pro 5",
                            "takerFeeRate": "0.00018",
                            "makerFeeRate": "0",
                            "makerRebate": ""
                        },
                        {
                            "level": "Pro 6",
                            "takerFeeRate": "0.00015",
                            "makerFeeRate": "0",
                            "makerRebate": ""
                        }
                    ],
                    "marketMaker": [
                        {
                            "level": "MM 1",
                            "takerFeeRate": "",
                            "makerFeeRate": "",
                            "makerRebate": "-0.0000075"
                        },
                        {
                            "level": "MM 2",
                            "takerFeeRate": "",
                            "makerFeeRate": "",
                            "makerRebate": "-0.000015"
                        },
                        {
                            "level": "MM 3",
                            "takerFeeRate": "",
                            "makerFeeRate": "",
                            "makerRebate": "-0.000025"
                        }
                    ]
                },
                "updateTime": "1753240500012"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1758627388542
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Funding Rate History

Query for historical funding rates. Each symbol has a different funding interval. For example, if the interval is 8 hours and the current time is UTC 12, then it returns the last funding rate, which settled at UTC 8.

To query the funding rate interval, please refer to the [instruments-info](https://bybit-exchange.github.io/docs/v5/market/instrument) endpoint.

> **Covers: USDT and USDC perpetual / Inverse perpetual**

info

- Passing only `startTime` returns an error.
- Passing only `endTime` returns 200 records up till `endTime`.
- Passing neither returns 200 records up till the current time.

### HTTP Request

GET`/v5/market/funding/history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`,`inverse` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| startTime | false | integer | The start timestamp (ms) |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `200`\]. Default: `200` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> fundingRate | string | Funding rate |
| \> fundingRateTimestamp | string | Funding rate timestamp (ms) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/history-fund-rate)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/funding/history?category=linear&symbol=ETHPERP&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP()
print(session.get_funding_rate_history(
    category="linear",
    symbol="ETHPERP",
    limit=1,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetFundingRateHistory(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var fundingHistoryRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSD).startTime(1632046800000L).endTime(1632133200000L).limit(150).build();
client.getFundingHistory(fundingHistoryRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getFundingRateHistory({
        category: 'linear',
        symbol: 'ETHPERP',
        limit: 1,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "linear",
        "list": [
            {
                "symbol": "ETHPERP",
                "fundingRate": "0.0001",
                "fundingRateTimestamp": "1672041600000"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672051897447
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Index Price Components

### HTTP Request

GET`/v5/market/index-price-components`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| indexName | **true** | string | Index name, like `BTCUSDT` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| indexName | string | Name of the index (e.g., BTCUSDT) |
| lastPrice | string | Last price of the index |
| updateTime | string | Timestamp of the last update in milliseconds |
| components | array | List of components contributing to the index price |
| \> exchange | string | Name of the exchange |
| \> spotPair | string | Spot trading pair on the exchange (e.g., BTCUSDT) |
| \> equivalentPrice | string | Equivalent price |
| \> multiplier | string | Multiplier used for the component price |
| \> price | string | Actual price |
| \> weight | string | Weight in the index calculation |

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/index-price-components?indexName=1000BTTUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_index_price_components(
    indexName="1000BTTUSDT"
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
  "retCode": 0,
  "retMsg": "",
  "result": {
    "indexName": "1000BTTUSDT",
    "lastPrice": "0.0006496",
    "updateTime": "1758182745072",
    "components": [
      {
        "exchange": "GateIO",
        "spotPair": "BTT_USDT",
        "equivalentPrice": "0.0006485",
        "multiplier": "1000",
        "price": "0.0006485",
        "weight": "0.1383220862762299"
      },
      {
        "exchange": "Bybit",
        "spotPair": "BTTUSDT",
        "equivalentPrice": "0.0006502",
        "multiplier": "1000",
        "price": "0.0006502",
        "weight": "0.0407528429737999"
      },
      {
        "exchange": "Bitget",
        "spotPair": "BTTUSDT",
        "equivalentPrice": "0.000648",
        "multiplier": "1000",
        "price": "0.000648",
        "weight": "0.1629044859431618"
      },
      {
        "exchange": "BitMart",
        "spotPair": "BTT_USDT",
        "equivalentPrice": "0.000649",
        "multiplier": "1000",
        "price": "0.000649",
        "weight": "0.0432327388538453"
      },
      {
        "exchange": "Binance",
        "spotPair": "BTTCUSDT",
        "equivalentPrice": "0.00065",
        "multiplier": "1000",
        "price": "0.00065",
        "weight": "0.5322401401714303"
      },
      {
        "exchange": "Mexc",
        "spotPair": "BTTUSDT",
        "equivalentPrice": "0.0006517",
        "multiplier": "1000",
        "price": "0.0006517",
        "weight": "0.0825477057815328"
      }
    ]
  },
  "retExtInfo": {},
  "time": 1758182745621
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Index Price Kline

Query for historical [index price](https://www.bybit.com/en-US/help-center/s/article/Glossary-Bybit-Trading-Terms) klines. Charts are returned in groups based on the requested interval.

> **Covers: USDT contract / USDC contract / Inverse contract**

### HTTP Request

GET`/v5/market/index-price-kline`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `linear`,`inverse`<br>- When `category` is not passed, use `linear` by default |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [interval](https://bybit-exchange.github.io/docs/v5/enum#interval) | **true** | string | Kline interval. `1`,`3`,`5`,`15`,`30`,`60`,`120`,`240`,`360`,`720`,`D`,`W`,`M` |
| start | false | integer | The start timestamp (ms) |
| end | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `1000`\]. Default: `200` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| symbol | string | Symbol name |
| list | array | - An string array of individual candle<br>- Sort in reverse by `startTime` |
| \> list\[0\]: startTime | string | Start time of the candle (ms) |
| \> list\[1\]: openPrice | string | Open price |
| \> list\[2\]: highPrice | string | Highest price |
| \> list\[3\]: lowPrice | string | Lowest price |
| \> list\[4\]: closePrice | string | Close price. _Is the last traded price when the candle is not closed_ |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/index-kline)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/index-price-kline?category=inverse&symbol=BTCUSDZ22&interval=1&start=1670601600000&end=1670608800000&limit=2 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_index_price_kline(
    category="inverse",
    symbol="BTCUSDZ22",
    interval=1,
    start=1670601600000,
    end=1670608800000,
    limit=2,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT", "interval": "1"}
client.NewUtaBybitServiceWithParams(params).GetIndexPriceKline(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var marketKLineRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").marketInterval(MarketInterval.WEEKLY).build();
client.getIndexPriceLinesData(marketKLineRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getIndexPriceKline({
        category: 'inverse',
        symbol: 'BTCUSDZ22',
        interval: '1',
        start: 1670601600000,
        end: 1670608800000,
        limit: 2,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "symbol": "BTCUSDZ22",
        "category": "inverse",
        "list": [
            [
                "1670608800000",
                "17167.00",
                "17167.00",
                "17161.90",
                "17163.07"
            ],
            [
                "1670608740000",
                "17166.54",
                "17167.69",
                "17165.42",
                "17167.00"
            ]
        ]
    },
    "retExtInfo": {},
    "time": 1672026471128
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Instruments Info

Query for the instrument specification of online trading pairs.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract / Option**

info

- Spot does not support pagination, so `limit`, `cursor` are invalid.
- When querying by `baseCoin`, regardless of if category=`linear` or `inverse`, the result will contain USDT contract, USDC contract and Inverse contract symbols.

caution

- This endpoint returns 500 entries by default. There are now more than 500 `linear` symbols on the platform. As a result, you will need to use `cursor` for pagination or `limit` to get all entries.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery
- The fields `maxLimitOrderQty`, `maxMarketOrderQty`, and `postOnlyMaxLimitOrderSize` are adjusted bi-monthly (3rd and 17th, 08:00 UTC+8). Developers should not assume these values remain constant.

### HTTP Request

GET`/v5/market/instruments-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `spot`,`linear`,`inverse`,`option` |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| [symbolType](https://bybit-exchange.github.io/docs/v5/enum#symboltype) | false | string | SymbolType:The region to which the trading pair belongs,only for`linear`,`inverse`,`spot` |
| [status](https://bybit-exchange.github.io/docs/v5/enum#status) | false | string | Symbol status filter <br>- `linear` & `inverse` & `spot`By default returns only `Trading` symbols<br>- `option` By default returns `PreLaunch`, `Trading`, and `Delivering`<br>- Spot has `Trading` only<br>- `linear` & `inverse`: when status=PreLaunch, it returns [Pre-Market contracts](https://www.bybit.com/help-center/article/Introduction-to-Pre-Market-Perpetual) |
| baseCoin | false | string | Base coin, uppercase only <br>- Applies to `linear`,`inverse`,`option` **only**<br>- `option`: returns BTC by default |
| limit | false | integer | Limit for data size per page. \[`1`, `1000`\]. Default: `500` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

- Linear/Inverse
- Option
- Spot

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| nextPageCursor | string | Cursor. Used to pagination |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> [contractType](https://bybit-exchange.github.io/docs/v5/enum#contracttype) | string | Contract type |
| \> [status](https://bybit-exchange.github.io/docs/v5/enum#status) | string | Instrument status |
| \> baseCoin | string | Base coin |
| \> quoteCoin | string | Quote coin |
| \> [symbolType](https://bybit-exchange.github.io/docs/v5/enum#symboltype) | string | the region to which the trading pair belongs |

| \> launchTime | string | Launch timestamp (ms) |
| \> deliveryTime | string | Delivery timestamp (ms) <br>- Expired futures delivery time<br>- Perpetual delisting time |
| \> deliveryFeeRate | string | Delivery fee rate |
| \> priceScale | string | Price scale |
| \> leverageFilter | Object | Leverage attributes |
| >\> minLeverage | string | Minimum leverage |
| >\> maxLeverage | string | Maximum leverage |
| >\> leverageStep | string | The step to increase/reduce leverage |
| \> priceFilter | Object | Price attributes |
| >\> minPrice | string | Minimum order price |
| >\> maxPrice | string | Maximum order price |
| >\> tickSize | string | The step to increase/reduce order price |
| \> lotSizeFilter | Object | Size attributes |
| >\> minNotionalValue | string | Minimum notional value |
| >\> maxOrderQty | string | Maximum quantity for Limit and PostOnly order |
| >\> maxMktOrderQty | string | Maximum quantity for Market order |
| >\> minOrderQty | string | Minimum order quantity |
| >\> qtyStep | string | The step to increase/reduce order quantity |
| >\> postOnlyMaxOrderQty | string | deprecated, please use `maxOrderQty` |
| \> unifiedMarginTrade | boolean | Whether to support unified margin trade |
| \> fundingInterval | integer | Funding interval (minute) |
| \> settleCoin | string | Settle coin |
| \> [copyTrading](https://bybit-exchange.github.io/docs/v5/enum#copytrading) | string | Copy trade symbol or not |
| \> upperFundingRate | string | Upper limit of funding date |
| \> lowerFundingRate | string | Lower limit of funding date |
| \> displayName | string | The USDC futures & perpetual name displayed in the Web or App |
| \> forbidUplWithdrawal | boolean | Whether to prohibit unrealised profit withdrawal |
| \> riskParameters | object | Risk parameters for limit order price. Note that the [formula changed](https://announcements.bybit.com/en/article/adjustments-to-bybit-s-derivative-trading-limit-order-mechanism-blt469228de1902fff6/) in Jan 2025 |
| >\> priceLimitRatioX | string | Ratio X |
| >\> priceLimitRatioY | string | Ratio Y |
| \> isPreListing | boolean | - Whether the contract is a pre-market contract<br>- When the pre-market contract is converted to official contract, it will be false |
| \> preListingInfo | object | - If isPreListing=false, preListingInfo=null<br>- If isPreListing=true, preListingInfo is an object |
| >\> [curAuctionPhase](https://bybit-exchange.github.io/docs/v5/enum#curauctionphase) | string | The current auction phase |
| >\> phases | array<object> | Each phase time info |
| >>\> [phase](https://bybit-exchange.github.io/docs/v5/enum#curauctionphase) | string | pre-market trading phase |
| >>\> startTime | string | The start time of the phase, timestamp(ms) |
| >>\> endTime | string | The end time of the phase, timestamp(ms) |
| >\> auctionFeeInfo | object | Action fee info |
| >>\> auctionFeeRate | string | The trading fee rate during auction phase <br>- There is no trading fee until entering continues trading phase |
| >>\> takerFeeRate | string | The taker fee rate during continues trading phase |
| >>\> makerFeeRate | string | The maker fee rate during continues trading phase |
| >\> skipCallAuction | boolean | `false`, `true` Whether the pre-market contract skips the call auction phase |

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| nextPageCursor | string | Cursor. Used to pagination |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> optionsType | string | Option type. `Call`, `Put` |
| \> [status](https://bybit-exchange.github.io/docs/v5/enum#status) | string | Instrument status |
| \> baseCoin | string | Base coin |
| \> quoteCoin | string | Quote coin |
| \> settleCoin | string | Settle coin |
| \> launchTime | string | Launch timestamp (ms) |
| \> deliveryTime | string | Delivery timestamp (ms) |
| \> deliveryFeeRate | string | Delivery fee rate |
| \> priceFilter | Object | Price attributes |
| >\> minPrice | string | Minimum order price |
| >\> maxPrice | string | Maximum order price |
| >\> tickSize | string | The step to increase/reduce order price |
| \> lotSizeFilter | Object | Size attributes |
| >\> maxOrderQty | string | Maximum order quantity |
| >\> minOrderQty | string | Minimum order quantity |
| >\> qtyStep | string | The step to increase/reduce order quantity |
| \> displayName | string | The option name displayed in the Web or App |

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> baseCoin | string | Base coin |
| \> quoteCoin | string | Quote coin |
| \> innovation | string | deprecated, please use `symbolType` |
| \> [symbolType](https://bybit-exchange.github.io/docs/v5/enum#symboltype) | string | the region to which the trading pair belongs |
| \> xstockMultiplier | string | Xstock mutiplier It only applies to those "symbolType"=`xstocks` trading pairs<br>relationship: stock\_price = token\_price / mutiplier; stock\_qty = token\_qty \* mutiplier<br>default value: "1" |
| \> [status](https://bybit-exchange.github.io/docs/v5/enum#status) | string | Instrument status |
| \> [marginTrading](https://bybit-exchange.github.io/docs/v5/enum#margintrading) | string | Whether or not this symbol supports margin trading<br>- This is to identify if the symbol supports margin trading under different account modes<br>- You may find some symbols do not support margin buy or margin sell, so you need to go to [Collateral Info (UTA)](https://bybit-exchange.github.io/docs/v5/account/collateral-info) to check if that coin is borrowable<br>- When the lending pool has insufficient balance to lend out funds (can happen during big market movements) then this will switch to `none` until there is sufficient balance to re-enable margin trading |
| \> stTag | string | Whether or not it has an [special treatment label](https://www.bybit.com/en/help-center/article/Bybit-Special-Treatment-ST-Label-Management-Rules). `0`: false, `1`: true |
| \> lotSizeFilter | Object | Size attributes |
| >\> basePrecision | string | The precision of base coin |
| >\> quotePrecision | string | The precision of quote coin |
| >\> minOrderQty | string | Minimum order quantity, deprecated, no longer check `minOrderQty`, check `minOrderAmt` instead |
| >\> maxOrderQty | string | Maximum order quantity, deprecated, please refer to `maxLimitOrderQty`, `maxMarketOrderQty` based on order type |
| >\> minOrderAmt | string | Minimum order amount |
| >\> maxOrderAmt | string | Maximum order amount, deprecated, no longer check `maxOrderAmt`, check `maxLimitOrderQty` and `maxMarketOrderQty` instead |
| >\> maxLimitOrderQty | string | Maximum Limit order quantity <br>- For post-only and retail price improvement (RPI) orders, the maximum limit order quantity is 5x `maxLimitOrderQty` |
| >\> maxMarketOrderQty | string | Maximum Market order quantity |
| >\> postOnlyMaxLimitOrderSize | string | Maximum limit order size for Post-only and RPI orders |
| \> priceFilter | Object | Price attributes |
| >\> tickSize | string | The step to increase/reduce order price |
| \> riskParameters | Object | Risk parameters for limit order price, refer to [announcement](https://announcements.bybit.com/en/article/title-adjustments-to-bybit-s-spot-trading-limit-order-mechanism-blt786c0c5abf865983/) |
| >\> priceLimitRatioX | string | Ratio X |
| >\> priceLimitRatioY | string | Ratio Y |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/instrument)

* * *

### Request Example

- Linear
- Option
- Spot

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/instruments-info?category=linear&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_instruments_info(
    category="linear",
    symbol="BTCUSDT",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetInstrumentInfo(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var instrumentInfoRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").instrumentStatus(InstrumentStatus.TRADING).limit(500).build();
client.getInstrumentsInfo(instrumentInfoRequest,System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getInstrumentsInfo({
        category: 'linear',
        symbol: 'BTCUSDT',
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/instruments-info?category=option&symbol=ETH-3JAN23-1250-P HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_instruments_info(
    category="option",
    symbol="ETH-3JAN23-1250-P",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "option", "symbol": "ETH-3JAN23-1250-P"}
client.NewUtaBybitServiceWithParams(params).GetInstrumentInfo(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var instrumentInfoRequest = MarketDataRequest.builder().category(CategoryType.OPTION).symbol("ETH-3JAN23-1250-P").instrumentStatus(InstrumentStatus.TRADING).limit(500).build();
client.getInstrumentsInfo(instrumentInfoRequest,System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
  .getInstrumentsInfo({
    category: 'option',
    symbol: 'ETH-3JAN23-1250-P',
  })
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/instruments-info?category=spot&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_instruments_info(
    category="spot",
    symbol="BTCUSDT",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetInstrumentInfo(context.Background())
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var instrumentInfoRequest = MarketDataRequest.builder().category(CategoryType.SPOT).symbol("BTCUSDT").instrumentStatus(InstrumentStatus.TRADING).limit(500).build();
client.getInstrumentsInfo(instrumentInfoRequest,System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
  .getInstrumentsInfo({
    category: 'spot',
    symbol: 'BTCUSDT',
  })
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

### Response Example

- Linear
- Option
- Spot

```json
// official USDT Perpetual instrument structure
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "linear",
        "list": [
            {
                "symbol": "BTCUSDT",
                "contractType": "LinearPerpetual",
                "status": "Trading",
                "baseCoin": "BTC",
                "quoteCoin": "USDT",
                "launchTime": "1585526400000",
                "deliveryTime": "0",
                "deliveryFeeRate": "",
                "priceScale": "2",
                "leverageFilter": {
                    "minLeverage": "1",
                    "maxLeverage": "100.00",
                    "leverageStep": "0.01"
                },
                "priceFilter": {
                    "minPrice": "0.10",
                    "maxPrice": "1999999.80",
                    "tickSize": "0.10"
                },
                "lotSizeFilter": {
                    "maxOrderQty": "1190.000",
                    "minOrderQty": "0.001",
                    "qtyStep": "0.001",
                    "postOnlyMaxOrderQty": "1190.000",
                    "maxMktOrderQty": "500.000",
                    "minNotionalValue": "5"
                },
                "unifiedMarginTrade": true,
                "fundingInterval": 480,
                "settleCoin": "USDT",
                "copyTrading": "both",
                "upperFundingRate": "0.00375",
                "lowerFundingRate": "-0.00375",
                "isPreListing": false,
                "preListingInfo": null,
                "riskParameters": {
                    "priceLimitRatioX": "0.01",
                    "priceLimitRatioY": "0.02"
                },
                "symbolType": ""
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1735809771618
}

// Pre-market Perpetual instrument structure
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "linear",
        "list": [
            {
                "symbol": "BIOUSDT",
                "contractType": "LinearPerpetual",
                "status": "PreLaunch",
                "baseCoin": "BIO",
                "quoteCoin": "USDT",
                "launchTime": "1735032510000",
                "deliveryTime": "0",
                "deliveryFeeRate": "",
                "priceScale": "4",
                "leverageFilter": {
                    "minLeverage": "1",
                    "maxLeverage": "5.00",
                    "leverageStep": "0.01"
                },
                "priceFilter": {
                    "minPrice": "0.0001",
                    "maxPrice": "1999.9998",
                    "tickSize": "0.0001"
                },
                "lotSizeFilter": {
                    "maxOrderQty": "70000",
                    "minOrderQty": "1",
                    "qtyStep": "1",
                    "postOnlyMaxOrderQty": "70000",
                    "maxMktOrderQty": "14000",
                    "minNotionalValue": "5"
                },
                "unifiedMarginTrade": true,
                "fundingInterval": 480,
                "settleCoin": "USDT",
                "copyTrading": "none",
                "upperFundingRate": "0.05",
                "lowerFundingRate": "-0.05",
                "isPreListing": true,
                "preListingInfo": {
                    "curAuctionPhase": "ContinuousTrading",
                    "phases": [
                        {
                            "phase": "CallAuction",
                            "startTime": "1735113600000",
                            "endTime": "1735116600000"
                        },
                        {
                            "phase": "CallAuctionNoCancel",
                            "startTime": "1735116600000",
                            "endTime": "1735116900000"
                        },
                        {
                            "phase": "CrossMatching",
                            "startTime": "1735116900000",
                            "endTime": "1735117200000"
                        },
                        {
                            "phase": "ContinuousTrading",
                            "startTime": "1735117200000",
                            "endTime": ""
                        }
                    ],
                    "auctionFeeInfo": {
                        "auctionFeeRate": "0",
                        "takerFeeRate": "0.001",
                        "makerFeeRate": "0.0004"
                    }
                },
                "riskParameters": {
                    "priceLimitRatioX": "0.05",
                    "priceLimitRatioY": "0.1"
                },
                "symbolType": ""
            }
        ],
        "nextPageCursor": "first%3DBIOUSDT%26last%3DBIOUSDT"
    },
    "retExtInfo": {},
    "time": 1735810114435
}
```

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "option",
        "nextPageCursor": "",
        "list": [
            {
                "symbol": "BTC-27MAR26-70000-P-USDT",
                "status": "Trading",
                "baseCoin": "BTC",
                "quoteCoin": "USDT",
                "settleCoin": "USDT",
                "optionsType": "Put",
                "launchTime": "1743669649256",
                "deliveryTime": "1774598400000",
                "deliveryFeeRate": "0.00015",
                "priceFilter": {
                    "minPrice": "5",
                    "maxPrice": "1110000",
                    "tickSize": "5"
                },
                "lotSizeFilter": {
                    "maxOrderQty": "500",
                    "minOrderQty": "0.01",
                    "qtyStep": "0.01"
                },
                "displayName": "BTCUSDT-27MAR26-70000-P"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672712537130
}
```

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "spot",
        "list": [
            {
                "symbol": "BTCUSDT",
                "baseCoin": "BTC",
                "quoteCoin": "USDT",
                "innovation": "0",
                "status": "Trading",
                "marginTrading": "utaOnly",
                "stTag": "0",
                "lotSizeFilter": {
                    "basePrecision": "0.000001",
                    "quotePrecision": "0.0000001",
                    "minOrderQty": "0.000011",
                    "maxOrderQty": "83",
                    "minOrderAmt": "5",
                    "maxOrderAmt": "8000000",
                    "maxLimitOrderQty": "83",
                    "maxMarketOrderQty": "41.5",
                    "postOnlyMaxLimitOrderSize":"60000"

                },
                "priceFilter": {
                    "tickSize": "0.1"
                },
                "riskParameters": {
                    "priceLimitRatioX": "0.005",
                    "priceLimitRatioY": "0.01"
                },
                "symbolType": ""
            }
        ]
    },
    "retExtInfo": {},
    "time": 1760027412300
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Insurance Pool

Query for Bybit [insurance pool](https://www.bybit.com/en/announcement-info/insurance-fund/) data (BTC/USDT/USDC etc)

info

- The isolated insurance pool balance is updated every 1 minute, and shared insurance pool balance is updated every 24 hours
- Please note that you may receive data from the previous minute. This is due to multiple backend containers starting
at different times, which may cause a slight delay. You can always rely on the latest minute data for accuracy.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/insurance`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | false | string | coin, uppercase only. Default: return all insurance coins |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| updatedTime | string | Data updated time (ms) |
| list | array | Object |
| \> coin | string | Coin |
| \> symbols | string | - symbols with `"BTCUSDT,ETHUSDT,SOLUSDT"` mean these contracts are shared with one insurance pool<br>- For an isolated insurance pool, it returns one contract |
| \> balance | string | Balance |
| \> value | string | USD value |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/insurance)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/insurance?coin=USDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_insurance(
    coin="USDT",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetMarketInsurance(context.Background())
```

```java
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var insuranceRequest = MarketDataRequest.builder().coin("BTC").build();
var insuranceData = client.getInsurance(insuranceRequest);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getInsurance({
        coin: 'USDT',
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "updatedTime": "1714003200000",
        "list": [
            {
                "coin": "USDT",
                "symbols": "MERLUSDT,10000000AIDOGEUSDT,ZEUSUSDT",
                "balance": "902178.57602476",
                "value": "901898.0963091522"
            },
            {
                "coin": "USDT",
                "symbols": "SOLUSDT,OMNIUSDT,ALGOUSDT",
                "balance": "14454.51626125",
                "value": "14449.515598975464"
            },
            {
                "coin": "USDT",
                "symbols": "XLMUSDT,WUSDT",
                "balance": "23.45018235",
                "value": "22.992864174376344"
            },
            {
                "coin": "USDT",
                "symbols": "AGIUSDT,WIFUSDT",
                "balance": "10002",
                "value": "9998.896846613574"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1714028451228
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Historical Volatility

Query option historical volatility

> **Covers: Option**

info

- The data is hourly.
- If both `startTime` and `endTime` are not specified, it will return the most recent 1 hours worth of data.
- `startTime` and `endTime` are a pair of params. Either both are passed or they are not passed at all.
- This endpoint can query the last 2 years worth of data, but make sure \[`endTime` \- `startTime`\] <= 30 days.

### HTTP Request

GET`/v5/market/historical-volatility`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| category | **true** | string | Product type. `option` |
| baseCoin | false | string | Base coin, uppercase only. Default: return BTC data |
| quoteCoin | false | string | Quote coin, `USD` or `USDT`. Default: return quoteCoin=USD |
| [period](https://bybit-exchange.github.io/docs/v5/enum#optionperiod) | false | integer | Period. If not specified, it will return data with a 7-day average by default |
| startTime | false | integer | The start timestamp (ms) |
| endTime | false | integer | The end timestamp (ms) |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> period | integer | Period |
| \> value | string | Volatility |
| \> time | string | Timestamp (ms) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/iv)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/market/historical-volatility?category=option&baseCoin=ETH&period=30 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_historical_volatility(
    category="option",
    baseCoin="ETH",
    period=30,
))
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var historicalVolatilityRequest = MarketDataRequest.builder().category(CategoryType.OPTION).optionPeriod(7).build();
client.getHistoricalVolatility(historicalVolatilityRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getHistoricalVolatility({
        category: 'option',
        baseCoin: 'ETH',
        period: 30,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "SUCCESS",
    "category": "option",
    "result": [
        {
            "period": 30,
            "value": "0.45024716",
            "time": "1672052400000"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Kline

Query for historical klines (also known as candles/candlesticks). Charts are returned in groups based on the requested interval.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract**

### HTTP Request

GET`/v5/market/kline`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `spot`,`linear`,`inverse`<br>- When `category` is not passed, use `linear` by default |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [interval](https://bybit-exchange.github.io/docs/v5/enum#interval) | **true** | string | Kline interval. `1`,`3`,`5`,`15`,`30`,`60`,`120`,`240`,`360`,`720`,`D`,`W`,`M` |
| start | false | integer | The start timestamp (ms) |
| end | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `1000`\]. Default: `200` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| symbol | string | Symbol name |
| list | array | - An string array of individual candle<br>- Sort in reverse by `startTime` |
| \> list\[0\]: startTime | string | Start time of the candle (ms) |
| \> list\[1\]: openPrice | string | Open price |
| \> list\[2\]: highPrice | string | Highest price |
| \> list\[3\]: lowPrice | string | Lowest price |
| \> list\[4\]: closePrice | string | Close price. _Is the last traded price when the candle is not closed_ |
| \> list\[5\]: volume | string | Trade volume <br>- USDT or USDC contract: unit is base coin (e.g., BTC)<br>- Inverse contract: unit is quote coin (e.g., USD) |
| \> list\[6\]: turnover | string | Turnover. <br>- USDT or USDC contract: unit is quote coin (e.g., USDT)<br>- Inverse contract: unit is base coin (e.g., BTC) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/kline)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/kline?category=inverse&symbol=BTCUSD&interval=60&start=1670601600000&end=1670608800000 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_kline(
    category="inverse",
    symbol="BTCUSD",
    interval=60,
    start=1670601600000,
    end=1670608800000,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT", "interval": "1"}
client.NewUtaBybitServiceWithParams(params).GetMarketKline(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var marketKLineRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").marketInterval(MarketInterval.WEEKLY).build();
client.getMarketLinesData(marketKLineRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getKline({
        category: 'inverse',
        symbol: 'BTCUSD',
        interval: '60',
        start: 1670601600000,
        end: 1670608800000,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "symbol": "BTCUSD",
        "category": "inverse",
        "list": [
            [
                "1670608800000",
                "17071",
                "17073",
                "17027",
                "17055.5",
                "268611",
                "15.74462667"
            ],
            [
                "1670605200000",
                "17071.5",
                "17071.5",
                "17061",
                "17071",
                "4177",
                "0.24469757"
            ],
            [
                "1670601600000",
                "17086.5",
                "17088",
                "16978",
                "17071.5",
                "6356",
                "0.37288112"
            ]
        ]
    },
    "retExtInfo": {},
    "time": 1672025956592
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Long Short Ratio

This refers to the net long and short positions as percentages of all position holders during the selected time.

Long account ratio = Number of holders with long positions / Total number of holders

Short account ratio = Number of holders with short positions / Total number of holders

Long-short account ratio = Long account ratio / Short account ratio

info

- The earliest query start time is July 20, 2020
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/account-ratio`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`(USDT Contract),`inverse` |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [period](https://bybit-exchange.github.io/docs/v5/enum#datarecordingperiod) | **true** | string | Data recording period. `5min`, `15min`, `30min`, `1h`, `4h`, `1d` |
| startTime | false | string | The start timestamp (ms) |
| endTime | false | string | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `500`\]. Default: `50` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> buyRatio | string | The ratio of the number of long position |
| \> sellRatio | string | The ratio of the number of short position |
| \> timestamp | string | Timestamp (ms) |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/long-short-ratio)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/account-ratio?category=linear&symbol=BTCUSDT&period=1h&limit=2&startTime=1696089600000&endTime=1696262400000 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_long_short_ratio(
    category="linear",
    symbol="BTCUSDT",
    period="1h",
    limit=2,
    startTime="1696089600000",
    endTime="1696262400000"
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT", "period": "5min"}
client.NewUtaBybitServiceWithParams(params).GetLongShortRatio(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var marketAccountRatioRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").dataRecordingPeriod(DataRecordingPeriod.FIFTEEN_MINUTES).limit(10).build();
client.getMarketAccountRatio(marketAccountRatioRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
});

client
  .getLongShortRatio({
    category: 'linear',
    symbol: 'BTCUSDT',
    period: '1h',
    limit: 100,
  })
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "symbol": "BTCUSDT",
                "buyRatio": "0.49",
                "sellRatio": "0.51",
                "timestamp": "1696262400000"
            },
            {
                "symbol": "BTCUSDT",
                "buyRatio": "0.4927",
                "sellRatio": "0.5073",
                "timestamp": "1696258800000"
            }
        ],
        "nextPageCursor": "lastid%3D0%26lasttime%3D1696258800"
    },
    "retExtInfo": {},
    "time": 1731567491688
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Mark Price Kline

Query for historical [mark price](https://www.bybit.com/en-US/help-center/s/article/Glossary-Bybit-Trading-Terms) klines. Charts are returned in groups based on the requested interval.

> **Covers: USDT contract / USDC contract / Inverse contract / Options**

### HTTP Request

GET`/v5/market/mark-price-kline`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `linear`,`inverse`, `option`<br>- When `category` is not passed, use `linear` by default |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [interval](https://bybit-exchange.github.io/docs/v5/enum#interval) | **true** | string | Kline interval. `1`,`3`,`5`,`15`,`30`,`60`,`120`,`240`,`360`,`720`,`D`,`M`,`W` |
| start | false | integer | The start timestamp (ms) |
| end | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. futures: \[`1`, `1000`\], option: \[`1`, `500`\]. Default: `200` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| symbol | string | Symbol name |
| list | array | - An string array of individual candle<br>- Sort in reverse by `startTime` |
| \> list\[0\]: startTime | string | Start time of the candle (ms) |
| \> list\[1\]: openPrice | string | Open price |
| \> list\[2\]: highPrice | string | Highest price |
| \> list\[3\]: lowPrice | string | Lowest price |
| \> list\[4\]: closePrice | string | Close price. _Is the last traded price when the candle is not closed_ |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/mark-kline)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/mark-price-kline?category=linear&symbol=BTCUSDT&interval=15&start=1670601600000&end=1670608800000&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_mark_price_kline(
    category="linear",
    symbol="BTCUSDT",
    interval=15,
    start=1670601600000,
    end=1670608800000,
    limit=1,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT", "interval": "1"}
client.NewUtaBybitServiceWithParams(params).GetMarkPriceKline(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var marketKLineRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").marketInterval(MarketInterval.WEEKLY).build();
client.getMarketPriceLinesData(marketKLineRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getMarkPriceKline({
        category: 'linear',
        symbol: 'BTCUSD',
        interval: '15',
        start: 1670601600000,
        end: 1670608800000,
        limit: 1,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "symbol": "BTCUSDT",
        "category": "linear",
        "list": [
            [
            "1670608800000",
            "17164.16",
            "17164.16",
            "17121.5",
            "17131.64"
            ]
        ]
    },
    "retExtInfo": {},
    "time": 1672026361839
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get New Delivery Price

Get historical option delivery prices.

> **Covers: Option**

info

- It is recommended to query this endpoint 1 minute after settlement is completed, because the data returned by this endpoint may be delayed by 1 minute.
- By default, the most recent 50 records are returned in reverse order of "deliveryTime".

### HTTP Request

GET`/v5/market/new-delivery-price`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. _Valid for `option` only_ |
| baseCoin | **true** | string | Base coin, uppercase only. _Valid for `option` only_ |
| settleCoin | false | string | Settle coin, uppercase only. Default: `USDT`. |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> deliveryPrice | string | Delivery price |
| \> deliveryTime | string | Delivery timestamp (ms) |

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/new-delivery-price?category=option&baseCoin=BTC HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_new_delivery_price(
    category="option",
    baseCoin="BTC"
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "",
    "result": {
        "category": "option",
        "list": [
            {
                "deliveryPrice": "111675.89830854",
                "deliveryTime": "1756080000000"
            },
            {
                "deliveryPrice": "114990.41430239",
                "deliveryTime": "1755993600000"
            },
            {
                "deliveryPrice": "115792.27557281",
                "deliveryTime": "1755907200000"
            },
            {
                "deliveryPrice": "113162.32041387",
                "deliveryTime": "1755820800000"
            },
            {
                "deliveryPrice": "113852.00497157",
                "deliveryTime": "1755734400000"
            },
            {
                "deliveryPrice": "113604.53226162",
                "deliveryTime": "1755648000000"
            },
            {
                "deliveryPrice": "114828.99222851",
                "deliveryTime": "1755561600000"
            },
            {
                "deliveryPrice": "115321.04746356",
                "deliveryTime": "1755475200000"
            },
            {
                "deliveryPrice": "117969.66726839",
                "deliveryTime": "1755388800000"
            },
            {
                "deliveryPrice": "117622.21555318",
                "deliveryTime": "1755302400000"
            },
            {
                "deliveryPrice": "118846.72206411",
                "deliveryTime": "1755216000000"
            },
            {
                "deliveryPrice": "121778.983223",
                "deliveryTime": "1755129600000"
            },
            {
                "deliveryPrice": "119383.31934289",
                "deliveryTime": "1755043200000"
            },
            {
                "deliveryPrice": "119030.19489407",
                "deliveryTime": "1754956800000"
            },
            {
                "deliveryPrice": "121725.4933271",
                "deliveryTime": "1754870400000"
            },
            {
                "deliveryPrice": "117780.91332268",
                "deliveryTime": "1754784000000"
            },
            {
                "deliveryPrice": "116795.39864682",
                "deliveryTime": "1754697600000"
            },
            {
                "deliveryPrice": "116880.31622213",
                "deliveryTime": "1754611200000"
            },
            {
                "deliveryPrice": "114782.09402227",
                "deliveryTime": "1754524800000"
            },
            {
                "deliveryPrice": "114212.80688625",
                "deliveryTime": "1754438400000"
            },
            {
                "deliveryPrice": "114046.80650192",
                "deliveryTime": "1754352000000"
            },
            {
                "deliveryPrice": "114668.76736223",
                "deliveryTime": "1754265600000"
            },
            {
                "deliveryPrice": "113691.29780823",
                "deliveryTime": "1754179200000"
            },
            {
                "deliveryPrice": "113947.55450439",
                "deliveryTime": "1754092800000"
            },
            {
                "deliveryPrice": "114786.86096974",
                "deliveryTime": "1754006400000"
            },
            {
                "deliveryPrice": "118693.64929462",
                "deliveryTime": "1753920000000"
            },
            {
                "deliveryPrice": "118218.22353841",
                "deliveryTime": "1753833600000"
            },
            {
                "deliveryPrice": "118953.66791589",
                "deliveryTime": "1753747200000"
            },
            {
                "deliveryPrice": "118894.70314174",
                "deliveryTime": "1753660800000"
            },
            {
                "deliveryPrice": "118137.86446229",
                "deliveryTime": "1753574400000"
            },
            {
                "deliveryPrice": "117344.01937262",
                "deliveryTime": "1753488000000"
            },
            {
                "deliveryPrice": "115166.35343924",
                "deliveryTime": "1753401600000"
            },
            {
                "deliveryPrice": "118217.70562761",
                "deliveryTime": "1753315200000"
            },
            {
                "deliveryPrice": "118444.57154255",
                "deliveryTime": "1753228800000"
            },
            {
                "deliveryPrice": "118155.53638794",
                "deliveryTime": "1753142400000"
            },
            {
                "deliveryPrice": "119370.88939816",
                "deliveryTime": "1753056000000"
            },
            {
                "deliveryPrice": "118080.35649338",
                "deliveryTime": "1752969600000"
            },
            {
                "deliveryPrice": "118197.36884665",
                "deliveryTime": "1752883200000"
            },
            {
                "deliveryPrice": "119644.49252705",
                "deliveryTime": "1752796800000"
            },
            {
                "deliveryPrice": "118316.40871555",
                "deliveryTime": "1752710400000"
            },
            {
                "deliveryPrice": "118216.19126195",
                "deliveryTime": "1752624000000"
            },
            {
                "deliveryPrice": "116746.02994227",
                "deliveryTime": "1752537600000"
            },
            {
                "deliveryPrice": "122778.73513717",
                "deliveryTime": "1752451200000"
            },
            {
                "deliveryPrice": "117973.83741111",
                "deliveryTime": "1752364800000"
            },
            {
                "deliveryPrice": "117741.30111399",
                "deliveryTime": "1752278400000"
            },
            {
                "deliveryPrice": "117851.19238216",
                "deliveryTime": "1752192000000"
            },
            {
                "deliveryPrice": "111263.21196833",
                "deliveryTime": "1752105600000"
            },
            {
                "deliveryPrice": "108721.62176788",
                "deliveryTime": "1752019200000"
            },
            {
                "deliveryPrice": "108410.57999842",
                "deliveryTime": "1751932800000"
            },
            {
                "deliveryPrice": "108969.06709828",
                "deliveryTime": "1751846400000"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1756110714178
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Open Interest

Get the [open interest](https://www.bybit.com/en-US/help-center/s/article/Glossary-Bybit-Trading-Terms) of each symbol.

> **Covers: USDT contract / USDC contract / Inverse contract**

info

- The upper limit time you can query is the launch time of the symbol.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/open-interest`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`,`inverse` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [intervalTime](https://bybit-exchange.github.io/docs/v5/enum#intervaltime) | **true** | string | Interval time. `5min`,`15min`,`30min`,`1h`,`4h`,`1d` |
| startTime | false | integer | The start timestamp (ms) |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `200`\]. Default: `50` |
| cursor | false | string | Cursor. Used to paginate |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| symbol | string | Symbol name |
| list | array | Object |
| \> openInterest | string | Open interest. The value is the sum of both sides. <br>The unit of value, e.g., BTCUSD(inverse) is USD, BTCUSDT(linear) is BTC |
| \> timestamp | string | The timestamp (ms) |
| nextPageCursor | string | Used to paginate |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/open-interest)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/open-interest?category=inverse&symbol=BTCUSD&intervalTime=5min&startTime=1669571100000&endTime=1669571400000 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_open_interest(
    category="inverse",
    symbol="BTCUSD",
    intervalTime="5min",
    startTime=1669571100000,
    endTime=1669571400000,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetOpenInterests(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var openInterest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").marketInterval(MarketInterval.FIVE_MINUTES).build();
client.getOpenInterest(openInterest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getOpenInterest({
        category: 'inverse',
        symbol: 'BTCUSD',
        intervalTime: '5min',
        startTime: 1669571100000,
        endTime: 1669571400000,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "symbol": "BTCUSD",
        "category": "inverse",
        "list": [
            {
                "openInterest": "461134384.00000000",
                "timestamp": "1669571400000"
            },
            {
                "openInterest": "461134292.00000000",
                "timestamp": "1669571100000"
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1672053548579
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Order Price Limit

For derivative trading order price limit, refer to [announcement](https://announcements.bybit.com/en/article/adjustments-to-bybit-s-derivative-trading-limit-order-mechanism-blt469228de1902fff6/)

For spot trading order price limit, refer to [announcement](https://announcements.bybit.com/en/article/title-adjustments-to-bybit-s-spot-trading-limit-order-mechanism-blt786c0c5abf865983/)

### HTTP Request

GET`/v5/market/price-limit`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `spot`,`linear`,`inverse`<br>- When `category` is not passed, use `linear` by default |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| symbol | string | Symbol name |
| buyLmt | string | Highest Bid Price |
| sellLmt | string | Lowest Ask Price |
| ts | string | timestamp in milliseconds |

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/price-limit?category=linear&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_price_limit(
    category="linear",
    symbol="BTCUSDT",
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "",
    "result": {
        "symbol": "BTCUSDT",
        "buyLmt": "105878.10",
        "sellLmt": "103781.60",
        "ts": "1750302284491"
    },
    "retExtInfo": {},
    "time": 1750302285376
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Orderbook

Query for orderbook depth data.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract / Option**

- Contract: 1000-level of orderbook data
- Spot: 1000-level of orderbook data
- Option: 25-level of orderbook data

info

- The response is in the snapshot format.
- [Retail Price Improvement (RPI)](https://www.bybit.com/en/help-center/article/Retail-Price-Improvement-RPI-Order) orders will not be included in the response message and will not be visible over API.

### HTTP Request

GET`/v5/market/orderbook`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `spot`, `linear`, `inverse`, `option` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| limit | false | integer | Limit size for each bid and ask<br>- `spot`: \[`1`, `200`\]. Default: `1`.<br>- `linear`&`inverse`: \[`1`, `500`\]. Default: `25`.<br>- `option`: \[`1`, `25`\]. Default: `1`. |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| s | string | Symbol name |
| b | array | Bid, buyer. Sorted by price in descending order |
| \> b\[0\] | string | Bid price |
| \> b\[1\] | string | Bid size |
| a | array | Ask, seller. Sorted by price in ascending order |
| \> a\[0\] | string | Ask price |
| \> a\[1\] | string | Ask size |
| ts | integer | The timestamp (ms) that the system generates the data |
| u | integer | Update ID, is always in sequence<br>- For contract, corresponds to `u` in the 1000-level [WebSocket orderbook stream](https://bybit-exchange.github.io/docs/v5/websocket/public/orderbook)<br>- For spot, corresponds to `u` in the 1000-level [WebSocket orderbook stream](https://bybit-exchange.github.io/docs/v5/websocket/public/orderbook) |
| seq | integer | Cross sequence <br>- You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier. |
| cts | integer | The timestamp from the matching engine when this orderbook data is produced. It can be correlated with `T` from [public trade channel](https://bybit-exchange.github.io/docs/v5/websocket/public/trade) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/orderbook)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/orderbook?category=spot&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_orderbook(
    category="linear",
    symbol="BTCUSDT",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetOrderBookInfo(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var orderbookRequest = MarketDataRequest.builder().category(CategoryType.SPOT).symbol("BTCUSDT").build();
client.getMarketOrderBook(orderbookRequest,System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getOrderbook({
        category: 'linear',
        symbol: 'BTCUSDT',
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "s": "BTCUSDT",
        "a": [
            [
                "65557.7",
                "16.606555"
            ]
        ],
        "b": [
            [
                "65485.47",
                "47.081829"
            ]
        ],
        "ts": 1716863719031,
        "u": 230704,
        "seq": 1432604333,
        "cts": 1716863718905
    },
    "retExtInfo": {},
    "time": 1716863719382
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Premium Index Price Kline

Query for historical [premium index](https://www.bybit.com/data/basic/linear/index-price/premium-index?symbol=BTCUSDT) klines. Charts are returned in groups based on the requested interval.

> **Covers: USDT and USDC perpetual**

### HTTP Request

GET`/v5/market/premium-index-price-kline`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `linear` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| [interval](https://bybit-exchange.github.io/docs/v5/enum#interval) | **true** | string | Kline interval. `1`,`3`,`5`,`15`,`30`,`60`,`120`,`240`,`360`,`720`,`D`,`W`,`M` |
| start | false | integer | The start timestamp (ms) |
| end | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `1000`\]. Default: `200` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type |
| symbol | string | Symbol name |
| list | array | - An string array of individual candle<br>- Sort in reverse by `start` |
| \> list\[0\] | string | Start time of the candle (ms) |
| \> list\[1\] | string | Open price |
| \> list\[2\] | string | Highest price |
| \> list\[3\] | string | Lowest price |
| \> list\[4\] | string | Close price. _Is the last traded price when the candle is not closed_ |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/premium-index-kline)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/premium-index-price-kline?category=linear&symbol=BTCUSDT&interval=D&start=1652112000000&end=1652544000000 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP()
print(session.get_premium_index_price_kline(
    category="linear",
    symbol="BTCUSDT",
    inverval="D",
    start=1652112000000,
    end=1652544000000,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT", "interval": "1"}
client.NewUtaBybitServiceWithParams(params).GetPremiumIndexPriceKline(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var marketKLineRequest = MarketDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").marketInterval(MarketInterval.WEEKLY).build();
client.getPremiumIndexPriceLinesData(marketKLineRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getPremiumIndexPriceKline({
        category: 'linear',
        symbol: 'BTCUSDT',
        interval: 'D',
        start: 1652112000000,
        end: 1652544000000,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "symbol": "BTCUSDT",
        "category": "linear",
        "list": [
            [
                "1652486400000",
                "-0.000587",
                "-0.000344",
                "-0.000480",
                "-0.000344"
            ],
            [
                "1652400000000",
                "-0.000989",
                "-0.000561",
                "-0.000587",
                "-0.000587"
            ]
        ]
    },
    "retExtInfo": {},
    "time": 1672765216291
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Recent Public Trades

Query recent public trading history in Bybit.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract / Option**

You can download archived historical trades from the [website](https://www.bybit.com/derivatives/en/history-data)

### HTTP Request

GET`/v5/market/recent-trade`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `spot`,`linear`,`inverse`,`option` |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | false | string | Symbol name, like `BTCUSDT`, uppercase only <br>- **required** for spot/linear/inverse<br>- optional for option |
| baseCoin | false | string | Base coin, uppercase only <br>- Apply to `option` **only**<br>- If the field is not passed, return **BTC** data by default |
| optionType | false | string | Option type. `Call` or `Put`. Apply to `option` **only** |
| limit | false | integer | Limit for data size per page <br>- `spot`: \[1,60\], default: `60`<br>- others: \[1,1000\], default: `500` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Products category |
| list | array | Object |
| \> execId | string | Execution ID |
| \> symbol | string | Symbol name |
| \> price | string | Trade price |
| \> size | string | Trade size |
| \> side | string | Side of taker `Buy`, `Sell` |
| \> time | string | Trade time (ms) |
| \> isBlockTrade | boolean | Whether the trade is block trade |
| \> isRPITrade | boolean | Whether the trade is RPI trade |
| \> mP | string | Mark price, unique field for `option` |
| \> iP | string | Index price, unique field for `option` |
| \> mIv | string | Mark iv, unique field for `option` |
| \> iv | string | iv, unique field for `option` |
| \> seq | string | cross sequence |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/recent-trade)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/recent-trade?category=spot&symbol=BTCUSDT&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_public_trade_history(
    category="spot",
    symbol="BTCUSDT",
    limit=1,
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetPublicRecentTrades(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var recentTrade = MarketDataRequest.builder().category(CategoryType.OPTION).symbol("ETH-30JUN23-2050-C").build();
client.getRecentTradeData(recentTrade, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getPublicTradingHistory({
        category: 'spot',
        symbol: 'BTCUSDT',
        limit: 1,
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "spot",
        "list": [
            {
                "execId": "2100000000007764263",
                "symbol": "BTCUSDT",
                "price": "16618.49",
                "size": "0.00012",
                "side": "Buy",
                "time": "1672052955758",
                "isBlockTrade": false,
                "isRPITrade": true,
                "seq":"123456"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672053054358
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Risk Limit

Query for the [risk limit](https://www.bybit.com/en/help-center/article/Risk-Limit-Perpetual-and-Futures) margin parameters. This information is also displayed on the website [here](https://www.bybit.com/en/announcement-info/margin-parameters/).

> **Covers: USDT contract / USDC contract / Inverse contract**

info

- category=`linear` returns a data set of 15 symbols in each response. Please use the `cursor` param to get the next data set.
- `symbol` support `Trading` status and `PreLaunch` [Pre-Market contracts](https://www.bybit.com/en/help-center/article/Introduction-to-Pre-Market-Perpetual) status trading pairs.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/risk-limit`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`,`inverse` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the data set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> id | integer | Risk ID |
| \> symbol | string | Symbol name |
| \> riskLimitValue | string | Position limit |
| \> maintenanceMargin | number | Maintain margin rate |
| \> initialMargin | number | Initial margin rate |
| \> isLowestRisk | integer | `1`: true, `0`: false |
| \> maxLeverage | string | Allowed max leverage |
| \> mmDeduction | string | The maintenance margin deduction value when risk limit tier changed |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/risk-limit)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/risk-limit?category=inverse&symbol=BTCUSD HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_risk_limit(
    category="inverse",
    symbol="BTCUSD",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "linear", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetMarketRiskLimits(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var riskMimitRequest = MarketDataRequest.builder().category(CategoryType.INVERSE).symbol("ADAUSD").build();
client.getRiskLimit(riskMimitRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getRiskLimit({
        category: 'inverse',
        symbol: 'BTCUSD',
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "inverse",
        "list": [
            {
                "id": 1,
                "symbol": "BTCUSD",
                "riskLimitValue": "150",
                "maintenanceMargin": "0.5",
                "initialMargin": "1",
                "isLowestRisk": 1,
                "maxLeverage": "100.00",
                "mmDeduction": ""
            },
        ....
        ]
    },
    "retExtInfo": {},
    "time": 1672054488010
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get RPI Orderbook

Query for orderbook depth data.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract /**

- Contract: 50-level of RPI orderbook data
- Spot: 50-level of RPI orderbook data

info

- The response is in the snapshot format.

### HTTP Request

GET`/v5/market/rpi_orderbook`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type. `spot`, `linear`, `inverse` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| limit | **true** | integer | Limit size for each bid and ask: \[1, 50\] |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| s | string | Symbol name |
| \> b | array | Bids. For `snapshot` stream. Sorted by price in descending order |
| >\> b\[0\] | string | Bid price |
| >\> b\[1\] | string | None RPI bid size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| >\> b\[2\] | string | RPI bid size <br>- When a bid RPI order crosses with a non-RPI ask price, the quantity of the bid RPI becomes invalid and is hidden |
| \> a | array | Asks. For `snapshot` stream. Sorted by price in ascending order |
| >\> a\[0\] | string | Ask price |
| >\> a\[1\] | string | None RPI ask size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| >\> a\[2\] | string | RPI ask size <br>- When an ask RPI order crosses with a non-RPI bid price, the quantity of the ask RPI becomes invalid and is hidden |
| ts | integer | The timestamp (ms) that the system generates the data |
| u | integer | Update ID, is always in sequence corresponds to `u` in the 50-level [WebSocket RPI orderbook stream](https://bybit-exchange.github.io/docs/v5/websocket/public/orderbook-rpi) |
| seq | integer | Cross sequence <br>- You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier. |
| cts | integer | The timestamp from the matching engine when this orderbook data is produced. It can be correlated with `T` from [public trade channel](https://bybit-exchange.github.io/docs/v5/websocket/public/trade) |

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/rpi_orderbook?category=spot&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_rpi_orderbook(
    category="spot",
    symbol="BTCUSDT",
    limit=50
))
```

```go

```

```java

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "s": "BTCUSDT",
        "a": [
            [
                "116600.00",
                "4.428",
                "0.000"
            ]
        ],
        "b": [
            [
                "116599.90",
                "3.721",
                "0.000"
            ]
        ],
        "ts": 1758078286128,
        "u": 28419362,
        "seq": 454803359210,
        "cts": 1758078286118
    },
    "retExtInfo": {},
    "time": 1758078286162
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Tickers

Query for the latest price snapshot, best bid/ask price, and trading volume in the last 24 hours.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract / Option**

info

If category= _option_, `symbol` or `baseCoin` must be passed.

### HTTP Request

GET`/v5/market/tickers`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `spot`,`linear`,`inverse`,`option` |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| baseCoin | false | string | Base coin, uppercase only. Apply to `option` **only** |
| expDate | false | string | Expiry date. e.g., 25DEC22. Apply to `option` **only** |

### Response Parameters

- Linear/Inverse
- Option
- Spot

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> lastPrice | string | Last price |
| \> indexPrice | string | Index price |
| \> markPrice | string | Mark price |
| \> prevPrice24h | string | Market price 24 hours ago |
| \> price24hPcnt | string | Percentage change of market price relative to 24h |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> prevPrice1h | string | Market price an hour ago |
| \> openInterest | string | Open interest size |
| \> openInterestValue | string | Open interest value |
| \> turnover24h | string | Turnover for 24h |
| \> volume24h | string | Volume for 24h |
| \> fundingRate | string | Funding rate |
| \> nextFundingTime | string | Next funding time (ms) |
| \> predictedDeliveryPrice | string | Predicated delivery price. It has a value 30 mins before delivery |
| \> basisRate | string | Basis rate |
| \> basis | string | Basis |
| \> deliveryFeeRate | string | Delivery fee rate |
| \> deliveryTime | string | Delivery timestamp (ms), applicable to expiry futures only |
| \> ask1Size | string | Best ask size |
| \> bid1Price | string | Best bid price |
| \> ask1Price | string | Best ask price |
| \> bid1Size | string | Best bid size |
| \> preOpenPrice | string | Estimated pre-market contract open price <br>- Meaningless once the market opens |
| \> preQty | string | Estimated pre-market contract open qty <br>- The value is meaningless once the market opens |
| \> [curPreListingPhase](https://bybit-exchange.github.io/docs/v5/enum#curauctionphase) | string | The current pre-market contract phase |
| \> fundingIntervalHour | string | Funding interval hour<br>- This value currently only supports whole hours |
| \> fundingCap | string | Funding rate upper and lower limits |
| \> basisRateYear | string | Annual basis rate<br>- Only for Futures,For Perpetual,it will return "" |

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> bid1Price | string | Best bid price |
| \> bid1Size | string | Best bid size |
| \> bid1Iv | string | Best bid iv |
| \> ask1Price | string | Best ask price |
| \> ask1Size | string | Best ask size |
| \> ask1Iv | string | Best ask iv |
| \> lastPrice | string | Last price |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> markPrice | string | Mark price |
| \> indexPrice | string | Index price |
| \> markIv | string | Mark price iv |
| \> underlyingPrice | string | Underlying price |
| \> openInterest | string | Open interest size |
| \> turnover24h | string | Turnover for 24h |
| \> volume24h | string | Volume for 24h |
| \> totalVolume | string | Total volume |
| \> totalTurnover | string | Total turnover |
| \> delta | string | Delta |
| \> gamma | string | Gamma |
| \> vega | string | Vega |
| \> theta | string | Theta |
| \> predictedDeliveryPrice | string | Predicated delivery price. It has a value 30 mins before delivery |
| \> change24h | string | The change in the last 24 hous |

| Parameter | Type | Comments |
| --- | --- | --- |
| category | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> bid1Price | string | Best bid price |
| \> bid1Size | string | Best bid size |
| \> ask1Price | string | Best ask price |
| \> ask1Size | string | Best ask size |
| \> lastPrice | string | Last price |
| \> prevPrice24h | string | Market price 24 hours ago |
| \> price24hPcnt | string | Percentage change of market price relative to 24h |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> turnover24h | string | Turnover for 24h |
| \> volume24h | string | Volume for 24h |
| \> usdIndexPrice | string | USD index price <br>- used to calculate USD value of the assets in Unified account<br>- non-collateral margin coin returns ""<br>- Only those trading pairs like "XXX/USDT" or "XXX/USDC" have the value |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/tickers)

* * *

### Request Example

- Inverse
- Option
- Spot

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/tickers?category=inverse&symbol=BTCUSD HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_tickers(
    category="inverse",
    symbol="BTCUSD",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "inverse", "symbol": "BTCUSD"}
client.NewUtaBybitServiceWithParams(params).GetMarketTickers(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var tickerReueqt = MarketDataRequest.builder().category(CategoryType.INVERSE).symbol("BTCUSD").build();
client.getMarketTickers(tickerReueqt, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
    .getTickers({
        category: 'inverse',
        symbol: 'BTCUSDT',
    })
    .then((response) => {
        console.log(response);
    })
    .catch((error) => {
        console.error(error);
    });
```

- HTTP
- Python
- Go
- Java
- Node.js

```http
GET /v5/market/tickers?category=option&symbol=BTC-30DEC22-18000-C HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_tickers(
    category="option",
    symbol="BTC-30DEC22-18000-C",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "option", "symbol": "BTC-30DEC22-18000-C"}
client.NewUtaBybitServiceWithParams(params).GetMarketTickers(context.Background())
```

```java
import com.bybit.api.client.domain.CategoryType;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.MarketDataRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var tickerReueqt = MarketDataRequest.builder().category(CategoryType.OPTION).symbol("BTC-30DEC22-18000-C").build();
client.getMarketTickers(tickerReueqt, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
  .getTickers({
    category: 'option',
    symbol: 'BTC-30DEC22-18000-C',
  })
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

- HTTP
- Python
- GO
- Java
- Node.js

```http
GET /v5/market/tickers?category=spot&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_tickers(
    category="spot",
    symbol="BTCUSDT",
))
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "spot", "symbol": "BTCUSDT"}
client.NewUtaBybitServiceWithParams(params).GetMarketTickers(context.Background())
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.market.*;
import com.bybit.api.client.domain.market.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
var tickerReueqt = MarketDataRequest.builder().category(CategoryType.SPOT).symbol("BTCUSDT").build();
client.getMarketTickers(tickerReueqt, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
});

client
  .getTickers({
    category: 'spot',
    symbol: 'BTCUSDT',
  })
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

### Response Example

- Inverse
- Option
- Spot

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "inverse",
        "list": [
            {
                "symbol": "BTCUSD",
                "lastPrice": "120635.50",
                "indexPrice": "114890.92",
                "markPrice": "114898.43",
                "prevPrice24h": "105595.90",
                "price24hPcnt": "0.142425",
                "highPrice24h": "131309.30",
                "lowPrice24h": "102007.60",
                "prevPrice1h": "119806.10",
                "openInterest": "240113967",
                "openInterestValue": "2089.79",
                "turnover24h": "115.6907",
                "volume24h": "13713832.0000",
                "fundingRate": "0.0001",
                "nextFundingTime": "1760371200000",
                "predictedDeliveryPrice": "",
                "basisRate": "",
                "deliveryFeeRate": "",
                "deliveryTime": "0",
                "ask1Size": "9854",
                "bid1Price": "103401.00",
                "ask1Price": "109152.80",
                "bid1Size": "1063",
                "basis": "",
                "preOpenPrice": "",
                "preQty": "",
                "curPreListingPhase": "",
                "fundingIntervalHour": "8",
                "basisRateYear": "",
                "fundingCap": "0.005"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1760352369814
}
```

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "option",
        "list": [
            {
                "symbol": "BTC-30DEC22-18000-C",
                "bid1Price": "0",
                "bid1Size": "0",
                "bid1Iv": "0",
                "ask1Price": "435",
                "ask1Size": "0.66",
                "ask1Iv": "5",
                "lastPrice": "435",
                "highPrice24h": "435",
                "lowPrice24h": "165",
                "markPrice": "0.00000009",
                "indexPrice": "16600.55",
                "markIv": "0.7567",
                "underlyingPrice": "16590.42",
                "openInterest": "6.3",
                "turnover24h": "2482.73",
                "volume24h": "0.15",
                "totalVolume": "99",
                "totalTurnover": "1967653",
                "delta": "0.00000001",
                "gamma": "0.00000001",
                "vega": "0.00000004",
                "theta": "-0.00000152",
                "predictedDeliveryPrice": "0",
                "change24h": "86"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672376592395
}
```

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "category": "spot",
        "list": [
            {
                "symbol": "BTCUSDT",
                "bid1Price": "20517.96",
                "bid1Size": "2",
                "ask1Price": "20527.77",
                "ask1Size": "1.862172",
                "lastPrice": "20533.13",
                "prevPrice24h": "20393.48",
                "price24hPcnt": "0.0068",
                "highPrice24h": "21128.12",
                "lowPrice24h": "20318.89",
                "turnover24h": "243765620.65899866",
                "volume24h": "11801.27771",
                "usdIndexPrice": "20784.12009279"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1673859087947
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Bybit Server Time

info

- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/market/time`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| timeSecond | string | Bybit server timestamp (sec) |
| timeNano | string | Bybit server timestamp (nano) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/market/time)

* * *

### Request Example

- HTTP
- Python
- Java
- Go
- Node.js

```http
GET /v5/market/time HTTP/1.1
Host: api.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(testnet=True)
print(session.get_server_time())
```

```java
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncMarketDataRestClient();
client.getServerTime(System.out::println);
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("", "", bybit.WithBaseURL(bybit.TESTNET))
client.NewUtaBybitServiceNoParams().GetServerTime(context.Background())
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
});

client
  .getServerTime()
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "timeSecond": "1688639403",
        "timeNano": "1688639403423213947"
    },
    "retExtInfo": {},
    "time": 1688639403423
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

