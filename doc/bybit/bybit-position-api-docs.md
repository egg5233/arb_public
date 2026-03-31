# Bybit Position API

Source: https://bybit-exchange.github.io/docs/v5/position/

---

# Set Auto Add Margin

Turn on/off auto-add-margin for **isolated** margin position

### HTTP Request

POST`/v5/position/set-auto-add-margin`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear` (USDT Contract, USDC Contract) |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| autoAddMargin | **true** | integer | Turn on/off. `0`: off. `1`: on |
| [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | false | integer | Used to identify positions in different position modes. For hedge mode position, this param is **required** <br>- `0`: one-way mode<br>- `1`: hedge-mode Buy side<br>- `2`: hedge-mode Sell side |

### Response Parameters

None

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/auto-add-margin)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/set-auto-add-margin HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN-TYPE: 2
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675255134857
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "linear",
    "symbol": "BTCUSDT",
    "autoAddmargin": 1,
    "positionIdx": null
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_auto_add_margin(
    category="linear",
    symbol="BTCUSDT",
    autoAddmargin=1,
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var setAutoAddMarginRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").autoAddMargin(AutoAddMargin.ON).build();
client.setAutoAddMargin(setAutoAddMarginRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setAutoAddMargin({
        category: 'linear',
        symbol: 'BTCUSDT',
        autoAddMargin: 1,
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
    "result": {},
    "retExtInfo": {},
    "time": 1675255135069
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Closed PnL

Query user's closed profit and loss records

### HTTP Request

GET`/v5/position/closed-pnl`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`(USDT Contract, USDC Contract) |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| startTime | false | integer | The start timestamp (ms) <br>- startTime and endTime are not passed, return 7 days by default<br>- Only startTime is passed, return range between startTime and startTime+7 days<br>- Only endTime is passed, return range between endTime-7 days and endTime<br>- If both are passed, the rule is endTime - startTime <= 7 days |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `100`\]. Default: `50` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> orderId | string | Order ID |
| \> side | string | `Buy`, `Sell` |
| \> qty | string | Order qty |
| \> orderPrice | string | Order price |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit` |
| \> execType | string | Exec type<br>`Trade`, `BustTrade`<br>`SessionSettlePnL`<br>`Settle`, `MovePosition` |
| \> closedSize | string | Closed size |
| \> cumEntryValue | string | Cumulated Position value |
| \> avgEntryPrice | string | Average entry price |
| \> cumExitValue | string | Cumulated exit position value |
| \> avgExitPrice | string | Average exit price |
| \> closedPnl | string | Closed PnL |
| \> fillCount | string | The number of fills in a single order |
| \> leverage | string | leverage |
| \> openFee | string | Open position trading fee |
| \> closeFee | string | Close position trading fee |
| \> createdTime | string | The created time (ms) |
| \> updatedTime | string | The updated time (ms) |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/close-pnl)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/position/closed-pnl?category=linear&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672284128523
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_closed_pnl(
    category="linear",
    limit=1,
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var closPnlRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).build();
client.getClosePnlList(closPnlRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getClosedPnL({
        category: 'linear',
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
        "nextPageCursor": "5a373bfe-188d-4913-9c81-d57ab5be8068%3A1672214887231423699%2C5a373bfe-188d-4913-9c81-d57ab5be8068%3A1672214887231423699",
        "category": "linear",
        "list": [
            {
                "symbol": "ETHPERP",
                "orderType": "Market",
                "leverage": "3",
                "updatedTime": "1672214887236",
                "side": "Sell",
                "orderId": "5a373bfe-188d-4913-9c81-d57ab5be8068",
                "closedPnl": "-47.4065323",
                "avgEntryPrice": "1194.97516667",
                "qty": "3",
                "cumEntryValue": "3584.9255",
                "createdTime": "1672214887231",
                "orderPrice": "1122.95",
                "closedSize": "3",
                "avgExitPrice": "1180.59833333",
                "execType": "Trade",
                "fillCount": "4",
                "cumExitValue": "3541.795"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672284129153
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Closed Options Positions

Query user's closed options positions, sorted by `closeTime` in descending order.

info

- Only supports users to query closed options positions in the last 6 months.
- Fee and price are displayed with trailing zeroes up to 8 decimal places.

### HTTP Request

GET`/v5/position/get-closed-positions`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | `option` |
| symbol | false | string | Symbol name |
| startTime | false | integer | The start timestamp (ms) <br>- startTime and endTime are not passed, return 1 days by default<br>- Only startTime is passed, return range between startTime and startTime+1 days<br>- Only endTime is passed, return range between endTime-1 days and endTime<br>- If both are passed, the rule is endTime - startTime <= 7 days |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `100`\]. Default: `50` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| nextPageCursor | string | Refer to the `cursor` request parameter |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> side | string | `Buy`, `Sell` |
| \> totalOpenFee | string | Total open fee |
| \> deliveryFee | string | Delivery fee |
| \> totalCloseFee | string | Total close fee |
| \> qty | string | Order qty |
| \> closeTime | integer | The closed time (ms) |
| \> avgExitPrice | string | Average exit price |
| \> deliveryPrice | string | Delivery price |
| \> openTime | integer | The opened time (ms) |
| \> avgEntryPrice | string | Average entry price |
| \> totalPnl | string | Total PnL |

* * *

### Request Example

- HTTP
- Python

```http
GET /v5/position/get-closed-positions?category=option&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672284128523
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_closed_options_positions(
    category="option",
    limit="1",
))
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "nextPageCursor": "1749726002161%3A0%2C1749715220240%3A1",
        "category": "option",
        "list": [
            {
                "symbol": "BTC-12JUN25-104019-C-USDT",
                "side": "Sell",
                "totalOpenFee": "0.94506647",
                "deliveryFee": "0.32184533",
                "totalCloseFee": "0.00000000",
                "qty": "0.02",
                "closeTime": 1749726002161,
                "avgExitPrice": "107281.77405000",
                "deliveryPrice": "107281.77405031",
                "openTime": 1749722990063,
                "avgEntryPrice": "3371.50000000",
                "totalPnl": "0.90760719"
            },
            {
                "symbol": "BTC-12JUN25-104000-C-USDT",
                "side": "Buy",
                "totalOpenFee": "0.86379999",
                "deliveryFee": "0.32287622",
                "totalCloseFee": "0.00000000",
                "qty": "0.02",
                "closeTime": 1749715220240,
                "avgExitPrice": "107625.40470150",
                "deliveryPrice": "107625.40470159",
                "openTime": 1749710568608,
                "avgEntryPrice": "3946.50000000",
                "totalPnl": "-7.60858218"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1749736532193
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Confirm New Risk Limit

It is only applicable when the user is marked as only reducing positions (please see the isReduceOnly field in
the [Get Position Info](https://bybit-exchange.github.io/docs/v5/position) interface). After the user actively adjusts the risk level, this interface
is called to try to calculate the adjusted risk level, and if it passes (retCode=0), the system will remove the position reduceOnly mark.
You are recommended to call [Get Position Info](https://bybit-exchange.github.io/docs/v5/position) to check `isReduceOnly` field.

### HTTP Request

POST`/v5/position/confirm-pending-mmr`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse` |
| symbol | **true** | string | Symbol name |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/confirm-pending-mmr HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1698051123673
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 53

{
    "category": "linear",
    "symbol": "BTCUSDT"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.confirm_new_risk_limit(
    category="linear",
    symbol="BTCUSDT"
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var confirmNewRiskRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").build();
client.confirmPositionRiskLimit(confirmNewRiskRequest, System.out::println);
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {},
    "retExtInfo": {},
    "time": 1698051124588
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Leverage

info

According to the risk limit, leverage affects the maximum position value that can be opened,
that is, the greater the leverage, the smaller the maximum position value that can be opened,
and vice versa. [Learn more](https://www.bybit.com/en/help-center/article/Risk-Limit-Perpetual-and-FuturesBybit_Perpetual_Contract_mechanism)

### HTTP Request

POST`/v5/position/set-leverage`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| buyLeverage | **true** | string | \[`1`, max leverage\]<br>- one-way mode: `buyLeverage` must be the same as `sellLeverage`<br>- Hedge mode: <br>  <br>  isolated margin: `buyLeverage` and `sellLeverage` can be different; <br>  <br>  cross margin: `buyLeverage` must be the same as `sellLeverage` |
| sellLeverage | **true** | string | \[`1`, max leverage\] |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/leverage)

* * *

### Response Parameters

None

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/set-leverage HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672281605082
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "linear",
    "symbol": "BTCUSDT",
    "buyLeverage": "6",
    "sellLeverage": "6"

}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_leverage(
    category="linear",
    symbol="BTCUSDT",
    buyLeverage="6",
    sellLeverage="6",
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var setLeverageRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").buyLeverage("5").sellLeverage("5").build();
client.setPositionLeverage(setLeverageRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setLeverage({
        category: 'linear',
        symbol: 'BTCUSDT',
        buyLeverage: '6',
        sellLeverage: '6',
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
    "result": {},
    "retExtInfo": {},
    "time": 1672281607343
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Add Or Reduce Margin

Manually add or reduce margin for **isolated** margin position

### HTTP Request

POST`/v5/position/add-margin`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| margin | **true** | string | Add or reduce. To add, then `10`; To reduce, then `-10`. Support up to 4 decimal |
| [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | false | integer | Used to identify positions in different position modes. For hedge mode position, this param is **required** <br>- `0`: one-way mode<br>- `1`: hedge-mode Buy side<br>- `2`: hedge-mode Sell side |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type |
| symbol | string | Symbol name |
| [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | integer | Position idx, used to identify positions in different position modes<br>- `0`: One-Way Mode<br>- `1`: Buy side of both side mode<br>- `2`: Sell side of both side mode |
| riskId | integer | Risk limit ID |
| riskLimitValue | string | Risk limit value |
| size | string | Position size |
| avgPrice | string | Average entry price |
| liqPrice | string | Liquidation price |
| bustPrice | string | Bankruptcy price |
| markPrice | string | Last mark price |
| positionValue | string | Position value |
| leverage | string | Position leverage |
| autoAddMargin | integer | Whether to add margin automatically. `0`: false, `1`: true |
| [positionStatus](https://bybit-exchange.github.io/docs/v5/enum#positionstatus) | String | Position status. `Normal`, `Liq`, `Adl` |
| positionIM | string | Initial margin |
| positionMM | string | Maintenance margin |
| takeProfit | string | Take profit price |
| stopLoss | string | Stop loss price |
| trailingStop | string | Trailing stop (The distance from market price) |
| unrealisedPnl | string | Unrealised PnL |
| cumRealisedPnl | string | Cumulative realised pnl |
| createdTime | string | Timestamp of the first time a position was created on this symbol (ms) |
| updatedTime | string | Position updated timestamp (ms) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/manual-add-margin)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/add-margin HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1684234363665
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 97

{
    "category": "inverse",
    "symbol": "ETHUSD",
    "margin": "0.01",
    "positionIdx": 0
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.add_or_reduce_margin(
    category="linear",
    symbol="BTCUSDT",
    margin="10"
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var updateMarginRequest = PositionDataRequest.builder().category(CategoryType.INVERSE).symbol("ETHUSDT").margin("0.0001").build();
client.modifyPositionMargin(updateMarginRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .addOrReduceMargin({
        category: 'linear',
        symbol: 'BTCUSDT',
        margin: '10',
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
        "symbol": "ETHUSD",
        "positionIdx": 0,
        "riskId": 11,
        "riskLimitValue": "500",
        "size": "200",
        "positionValue": "0.11033265",
        "avgPrice": "1812.70004844",
        "liqPrice": "1550.80",
        "bustPrice": "1544.20",
        "markPrice": "1812.90",
        "leverage": "12",
        "autoAddMargin": 0,
        "positionStatus": "Normal",
        "positionIM": "0.01926611",
        "positionMM": "0",
        "unrealisedPnl": "0.00001217",
        "cumRealisedPnl": "-0.04618929",
        "stopLoss": "0.00",
        "takeProfit": "0.00",
        "trailingStop": "0.00",
        "createdTime": "1672737740039",
        "updatedTime": "1684234363788"
    },
    "retExtInfo": {},
    "time": 1684234363789
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Move Position

You can move positions between sub-master, master-sub, or sub-sub UIDs when necessary

info

- The endpoint can only be called by master UID api key
- UIDs must be the same master-sub account relationship
- The trades generated from move-position endpoint will not be displayed in the Recent Trade (Rest API & Websocket)
- There is no trading fee
- `fromUid` and `toUid` both should be Unified trading accounts, and they need to be one-way mode when moving the positions
- Please note that once executed, you will get execType=`MovePosition` entry from [Get Trade History](https://bybit-exchange.github.io/docs/v5/order/execution), [Get Closed Pnl](https://bybit-exchange.github.io/docs/v5/position/close-pnl), and stream from [Execution](https://bybit-exchange.github.io/docs/v5/websocket/private/execution).

### HTTP Request

POST`/v5/position/move-positions`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| fromUid | **true** | string | From UID <br>- Must be UTA<br>- Must be in one-way mode for Futures |
| toUid | **true** | string | To UID <br>- Must be UTA<br>- Must be in one-way mode for Futures |
| list | **true** | array | Object. Up to 25 legs per request |
| \> [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `spot`, `option`,`inverse` |
| \> symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| \> price | **true** | string | Trade price <br>- `linear`&`inverse`: the price needs to be between \[95% of mark price, 105% of mark price\]<br>- `spot`&`option`: the price needs to follow the price rule from [Instruments Info](https://bybit-exchange.github.io/docs/v5/market/instrument) |
| \> side | **true** | string | Trading side of `fromUid`<br>- For example, `fromUid` has a long position, when side=`Sell`, then once executed, the position of `fromUid` will be reduced or open a short position depending on `qty` input |
| \> qty | **true** | string | Executed qty <br>- The value must satisfy the qty rule from [Instruments Info](https://bybit-exchange.github.io/docs/v5/market/instrument), in particular, category=`linear` is able to input `maxOrderQty` \\* 5 |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| retCode | integer | Result code. `0` means request is successfully accepted |
| retMsg | string | Result message |
| result | map | Object |
| \> blockTradeId | string | Block trade ID |
| \> status | string | Status. `Processing`, `Rejected` |
| \> rejectParty | string | - `""` means initial validation is passed, please check the order status via [Get Move Position History](https://bybit-exchange.github.io/docs/v5/position/move-position-history)<br>- `Taker`, `Maker` when status=`Rejected`<br>- `bybit` means error is occurred on the Bybit side |

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/move-positions HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1697447928051
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "fromUid": "100307601",
    "toUid": "592324",
    "list": [
        {
            "category": "spot",
            "symbol": "BTCUSDT",
            "price": "100",
            "side": "Sell",
            "qty": "0.01"
        }
    ]
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.move_position(
    fromUid="100307601",
    toUid="592324",
    list=[
        {
            "category": "spot",
            "symbol": "BTCUSDT",
            "price": "100",
            "side": "Sell",
            "qty": "0.01",
        }
    ]
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var movePositionsRequest = Arrays.asList(MovePositionDetailsRequest.builder().category(CategoryType.SPOT.getCategoryTypeId()).symbol("BTCUSDT").side(Side.SELL.getTransactionSide()).price("100").qty("0.01").build(),
                MovePositionDetailsRequest.builder().category(CategoryType.SPOT.getCategoryTypeId()).symbol("ETHUSDT").side(Side.SELL.getTransactionSide()).price("100").qty("0.01").build());
var batchMovePositionsRequest = BatchMovePositionRequest.builder().fromUid("123456").toUid("456789").list(movePositionsRequest).build();
System.out.println(client.batchMovePositions(batchMovePositionsRequest));
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "blockTradeId": "e9bb926c95f54cf1ba3e315a58b8597b",
        "status": "Processing",
        "rejectParty": ""
    }
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Move Position History

You can query moved position data by master UID api key

### HTTP Request

GET`/v5/position/move-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| startTime | false | number | The order creation start timestamp. The interval is 7 days |
| endTime | false | number | The order creation end timestamp. The interval is 7 days |
| status | false | string | Order status. `Processing`, `Filled`, `Rejected` |
| blockTradeId | false | string | Block trade ID |
| limit | false | string | Limit for data size per page. \[`1`, `200`\]. Default: `20` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> blockTradeId | string | Block trade ID |
| \> [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type. `linear`, `spot`, `option` |
| \> orderId | string | Bybit order ID |
| \> userId | integer | User ID |
| \> symbol | string | Symbol name |
| \> side | string | Order side from taker's perspective. `Buy`, `Sell` |
| \> price | string | Order price |
| \> qty | string | Order quantity |
| \> execFee | string | The fee for taker or maker in the base currency paid to the Exchange executing the block trade |
| \> status | string | Block trade status. `Processing`, `Filled`, `Rejected` |
| \> execId | string | The unique trade ID from the exchange |
| \> resultCode | integer | The result code of the order. `0` means success |
| \> resultMessage | string | The error message. `""` when resultCode=0 |
| \> createdAt | number | The timestamp (ms) when the order is created |
| \> updatedAt | number | The timestamp (ms) when the order is updated |
| \> rejectParty | string | - `""` means the status=`Filled`<br>- `Taker`, `Maker` when status=`Rejected`<br>- `bybit` means error is occurred on the Bybit side |
| nextPageCursor | string | Used to get the next page data |

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/position/move-history?limit=1&status=Filled HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1697523024244
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_move_position_history(
    limit="1",
    status="Filled",
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var movePositionsHistoryRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").status(MovePositionStatus.Processing).build();
System.out.println(client.getMovePositionHistory(movePositionsHistoryRequest));
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
                "blockTradeId": "1a82e5801af74b67b7ad71ba00a7391a",
                "category": "option",
                "orderId": "8e09c5b8-f651-4cec-968d-52764cac11ec",
                "userId": 592324,
                "symbol": "BTC-14OCT23-27000-C",
                "side": "Buy",
                "price": "6",
                "qty": "0.99",
                "execFee": "0",
                "status": "Filled",
                "execId": "677ad344-6bb4-4ace-baca-128fcffcaca7",
                "resultCode": 0,
                "resultMessage": "",
                "createdAt": 1697186522865,
                "updatedAt": 1697186523289,
                "rejectParty": ""
            }
        ],
        "nextPageCursor": "page_token%3D1241742%26"
    },
    "retExtInfo": {},
    "time": 1697523024386
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Switch Position Mode

It supports to switch the position mode for **USDT perpetual** and **Inverse futures**. If you are in one-way Mode, you can only open one position on Buy or Sell side. If you are in hedge mode, you can open both Buy and Sell side positions simultaneously.

tip

- Priority for configuration to take effect: symbol > coin > system default
- System default: one-way mode
- If the request is by coin (settleCoin), then all symbols based on this setteCoin that do not have position and open order will be batch switched, and new listed symbol based on this settleCoin will be the same mode you set.

### Example

|  | System default | coin | symbol |
| --- | --- | --- | --- |
| Initial setting | one-way | never configured | never configured |
| Result | All USDT perpetual trading pairs are one-way mode |
| **Change 1** | - | - | Set BTCUSDT to hedge-mode |
| Result | BTCUSDT becomes hedge-mode, and all other symbols keep one-way mode |
| list new symbol ETHUSDT | ETHUSDT is one-way mode (inherit default rules) |
| **Change 2** | - | Set USDT to hedge-mode | - |
| Result | All current trading pairs with no positions or orders are hedge-mode, and no adjustments will be made for trading pairs with positions and orders |
| list new symbol SOLUSDT | SOLUSDT is hedge-mode (Inherit coin rule) |
| **Change 3** | - | - | Set ASXUSDT to one-mode |
| Take effect result | AXSUSDT is one-way mode, other trading pairs have no change |
| list new symbol BITUSDT | BITUSDT is hedge-mode (Inherit coin rule) |

### The position-switch ability for each contract

|  | UTA2.0 |
| --- | --- |
| USDT perpetual | **Support one-way & hedge-mode** |
| USDT futures | Support one-way **only** |
| USDC perpetual | Support one-way **only** |
| Inverse perpetual | Support one-way **only** |
| Inverse futures | Support one-way **only** |

### HTTP Request

POST`/v5/position/switch-mode`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, USDT Contract |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only. Either `symbol` or `coin` is **required**. `symbol` has a higher priority |
| coin | false | string | Coin, uppercase only |
| mode | **true** | integer | Position mode. `0`: Merged Single. `3`: Both Sides |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/position-mode)

* * *

### Response Parameters

None

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/switch-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675249072041
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 87

{
    "category":"inverse",
    "symbol":"BTCUSDH23",
    "coin": null,
    "mode": 0
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.switch_position_mode(
    category="inverse",
    symbol="BTCUSDH23",
    mode=0,
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newPositionRestClient();
var switchPositionMode = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").positionMode(PositionMode.BOTH_SIDES).build();
System.out.println(client.switchPositionMode(switchPositionMode));
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .switchPositionMode({
        category: 'inverse',
        symbol: 'BTCUSDH23',
        mode: 0,
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
    "result": {},
    "retExtInfo": {},
    "time": 1675249072814
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Trading Stop

Set the take profit, stop loss or trailing stop for the position.

tip

Passing these parameters will create conditional orders by the system internally. The system will cancel these orders if the position is closed, and adjust the qty according to the size of the open position.

info

New version of TP/SL function supports both holding entire position TP/SL orders and holding partial position TP/SL orders.

- Full position TP/SL orders: This API can be used to modify the parameters of existing TP/SL orders.
- Partial position TP/SL orders: This API can only add partial position TP/SL orders.

note

Under the new version of TP/SL function, when calling this API to perform one-sided take profit or stop loss modification
on existing TP/SL orders on the holding position, it will cause the paired tp/sl orders to lose binding relationship.
This means that when calling the cancel API through the tp/sl order ID, it will only cancel the corresponding one-sided
take profit or stop loss order ID.

### HTTP Request

POST`/v5/position/trading-stop`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse`, `option` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| tpslMode | **true** | string | TP/SL mode<br>- `Full`: entire position TP/SL, option supports "tpslMode"=`Full` only<br>- `Partial`: partial position TP/SL |
| [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | true | integer | Used to identify positions in different position modes. <br>- `0`: one-way mode<br>- `1`: hedge-mode Buy side<br>- `2`: hedge-mode Sell side |
| takeProfit | false | string | Cannot be less than 0, 0 means cancel TP |
| stopLoss | false | string | Cannot be less than 0, 0 means cancel SL |
| trailingStop | false | string | Trailing stop by price distance. Cannot be less than 0, 0 means cancel TS |
| [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Take profit trigger price type |
| [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Stop loss trigger price type |
| activePrice | false | string | Trailing stop trigger price. Trailing stop will be triggered when this price is reached **only** |
| tpSize | false | string | Take profit size<br>valid for TP/SL partial mode, note: the value of tpSize and slSize must equal |
| slSize | false | string | Stop loss size<br>valid for TP/SL partial mode, note: the value of tpSize and slSize must equal |
| tpLimitPrice | false | string | The limit order price when take profit price is triggered. Only works when tpslMode=Partial and tpOrderType=Limit |
| slLimitPrice | false | string | The limit order price when stop loss price is triggered. Only works when tpslMode=Partial and slOrderType=Limit |
| tpOrderType | false | string | The order type when take profit is triggered. `Market`(default), `Limit`<br>For tpslMode=Full, it only supports tpOrderType="Market" |
| slOrderType | false | string | The order type when stop loss is triggered. `Market`(default), `Limit`<br>For tpslMode=Full, it only supports slOrderType="Market" |

### Response Parameters

None

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/trading-stop)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST /v5/position/trading-stop HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672283124270
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category":"linear",
    "symbol": "XRPUSDT",
    "takeProfit": "0.6",
    "stopLoss": "0.2",
    "tpTriggerBy": "MarkPrice",
    "slTriggerBy": "IndexPrice",
    "tpslMode": "Partial",
    "tpOrderType": "Limit",
    "slOrderType": "Limit",
    "tpSize": "50",
    "slSize": "50",
    "tpLimitPrice": "0.57",
    "slLimitPrice": "0.21",
    "positionIdx": 0
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_trading_stop(
    category="linear",
    symbol="XRPUSDT",
    takeProfit="0.6",
    stopLoss="0.2",
    tpTriggerBy="MarkPrice",
    slTriggerB="IndexPrice",
    tpslMode="Partial",
    tpOrderType="Limit",
    slOrderType="Limit",
    tpSize="50",
    slSize="50",
    tpLimitPrice="0.57",
    slLimitPrice="0.21",
    positionIdx=0,
))
```

```java
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.position.*;
import com.bybit.api.client.domain.position.request.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance().newAsyncPositionRestClient();
var setTradingStopRequest = PositionDataRequest.builder().category(CategoryType.LINEAR).symbol("XRPUSDT").takeProfit("0.6").stopLoss("0.2").tpTriggerBy(TriggerBy.MARK_PRICE).slTriggerBy(TriggerBy.LAST_PRICE)
                .tpslMode(TpslMode.PARTIAL).tpOrderType(TradeOrderType.LIMIT).slOrderType(TradeOrderType.LIMIT).tpSize("50").slSize("50").tpLimitPrice("0.57").slLimitPrice("0.21").build();
client.setTradingStop(setTradingStopRequest, System.out::println);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setTradingStop({
        category: 'linear',
        symbol: 'XRPUSDT',
        takeProfit: '0.6',
        stopLoss: '0.2',
        tpTriggerBy: 'MarkPrice',
        slTriggerBy: 'IndexPrice',
        tpslMode: 'Partial',
        tpOrderType: 'Limit',
        slOrderType: 'Limit',
        tpSize: '50',
        slSize: '50',
        tpLimitPrice: '0.57',
        slLimitPrice: '0.21',
        positionIdx: 0,
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
    "result": {},
    "retExtInfo": {},
    "time": 1672283125359
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

