# Bybit Order API

Source: https://bybit-exchange.github.io/docs/v5/order/

---

# Amend Order

info

You can only modify **unfilled** or **partially filled** orders.

### HTTP Request

POST`/v5/order/amend`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| orderId | false | string | Order ID. Either `orderId` or `orderLinkId` is required |
| orderLinkId | false | string | User customised order ID. Either `orderId` or `orderLinkId` is required |
| orderIv | false | string | Implied volatility. `option` **only**. Pass the real value, e.g for 10%, 0.1 should be passed |
| triggerPrice | false | string | - For Perps & Futures, it is the conditional order trigger price. If you expect the price to rise to trigger your conditional order, make sure:<br>  <br>  _triggerPrice > market price_<br>  <br>  Else, _triggerPrice < market price_<br>- For spot, it is the TP/SL and Conditional order trigger price |
| qty | false | string | Order quantity after modification. Do not pass it if not modify the qty |
| price | false | string | Order price after modification. Do not pass it if not modify the price |
| tpslMode | false | string | TP/SL mode <br>- `Full`: entire position for TP/SL. Then, tpOrderType or slOrderType must be `Market`<br>- `Partial`: partial position tp/sl. Limit TP/SL order are supported. Note: When create limit tp/sl, tpslMode is **required** and it must be `Partial`<br>Valid for `linear` & `inverse` |
| takeProfit | false | string | Take profit price after modification. If pass "0", it means cancel the existing take profit of the order. Do not pass it if you do not want to modify the take profit |
| stopLoss | false | string | Stop loss price after modification. If pass "0", it means cancel the existing stop loss of the order. Do not pass it if you do not want to modify the stop loss |
| [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger take profit. When set a take profit, this param is **required** if no initial value for the order |
| [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger stop loss. When set a take profit, this param is **required** if no initial value for the order |
| [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Trigger price type |
| tpLimitPrice | false | string | Limit order price when take profit is triggered. Only working when original order sets partial limit tp/sl. _Option not supported_ |
| slLimitPrice | false | string | Limit order price when stop loss is triggered. Only working when original order sets partial limit tp/sl. _Option not supported\`_ |

info

The acknowledgement of an amend order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/amend-order)

* * *

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Order ID |
| orderLinkId | string | User customised order ID |

### Request Example

- HTTP
- Python
- Java
- .Net
- Node.js

```http
POST /v5/order/amend HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672217108106
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "linear",
    "symbol": "ETHPERP",
    "orderLinkId": "linear-004",
    "triggerPrice": "1145",
    "qty": "0.15",
    "price": "1050",
    "takeProfit": "0",
    "stopLoss": "0"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.amend_order(
    category="linear",
    symbol="ETHPERP",
    orderLinkId="linear-004",
    triggerPrice="1145",
    qty="0.15",
    price="1050",
    takeProfit="0",
    stopLoss="0",
))
```

```java
import com.bybit.api.client.restApi.BybitApiTradeRestClient;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var amendOrderRequest = TradeOrderRequest.builder().orderId("1523347543495541248").category(ProductType.LINEAR).symbol("XRPUSDT")
                        .price("0.5")  // setting a new price, for example
                        .qty("15")  // and a new quantity
                        .build();
var amendedOrder = client.amendOrder(amendOrderRequest);
System.out.println(amendedOrder);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
BybitTradeService tradeService = new(apiKey: "xxxxxxxxxxxxxx", apiSecret: "xxxxxxxxxxxxxxxxxxxxx");
var orderInfoString = await TradeService.AmendOrder(orderId: "1523347543495541248", category:Category.LINEAR, symbol: "XRPUSDT", price:"0.5", qty:"15");
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .amendOrder({
        category: 'linear',
        symbol: 'ETHPERP',
        orderLinkId: 'linear-004',
        triggerPrice: '1145',
        qty: '0.15',
        price: '1050',
        takeProfit: '0',
        stopLoss: '0',
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
        "orderId": "c6f055d9-7f21-4079-913d-e6523a9cfffa",
        "orderLinkId": "linear-004"
    },
    "retExtInfo": {},
    "time": 1672217093461
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Batch Amend Order

tip

This endpoint allows you to amend more than one open order in a single request.

- You can modify **unfilled** or **partially filled** orders. Conditional orders are not supported.
- A maximum of 20 orders (option), 20 orders (inverse), 20 orders (linear), 10 orders (spot) can be amended per request.

### HTTP Request

POST`/v5/order/amend-batch`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `option`, `spot`, `inverse` |
| request | **true** | array | Object |
| \> symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| \> orderId | false | string | Order ID. Either `orderId` or `orderLinkId` is required |
| \> orderLinkId | false | string | User customised order ID. Either `orderId` or `orderLinkId` is required |
| \> orderIv | false | string | Implied volatility. `option` **only**. Pass the real value, e.g for 10%, 0.1 should be passed |
| \> triggerPrice | false | string | - For Perps & Futures, it is the conditional order trigger price. If you expect the price to rise to trigger your conditional order, make sure:<br>  <br>  _triggerPrice > market price_<br>  <br>  Else, _triggerPrice < market price_<br>- For spot, it is for tpslOrder or stopOrder trigger price |
| \> qty | false | string | Order quantity after modification. Do not pass it if not modify the qty |
| \> price | false | string | Order price after modification. Do not pass it if not modify the price |
| \> tpslMode | false | string | TP/SL mode <br>- `Full`: entire position for TP/SL. Then, tpOrderType or slOrderType must be `Market`<br>- `Partial`: partial position tp/sl. Limit TP/SL order are supported. Note: When create limit tp/sl, tpslMode is **required** and it must be `Partial` |
| \> takeProfit | false | string | Take profit price after modification. If pass "0", it means cancel the existing take profit of the order. Do not pass it if you do not want to modify the take profit |
| \> stopLoss | false | string | Stop loss price after modification. If pass "0", it means cancel the existing stop loss of the order. Do not pass it if you do not want to modify the stop loss |
| \> [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger take profit. When set a take profit, this param is **required** if no initial value for the order |
| \> [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger stop loss. When set a take profit, this param is **required** if no initial value for the order |
| \> [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Trigger price type |
| \> tpLimitPrice | false | string | Limit order price when take profit is triggered. Only working when original order sets partial limit tp/sl |
| \> slLimitPrice | false | string | Limit order price when stop loss is triggered. Only working when original order sets partial limit tp/sl |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | Object |  |
| \> list | array | Object |
| >\> category | string | Product type |
| >\> symbol | string | Symbol name |
| >\> orderId | string | Order ID |
| >\> orderLinkId | string | User customised order ID |
| retExtInfo | Object |  |
| \> list | array | Object |
| >\> code | number | Success/error code |
| >\> msg | string | Success/error message |

info

The acknowledgement of an amend order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/batch-amend)

* * *

### Request Example

- HTTP
- Python
- Java
- .Net
- Node.js

```http
POST /v5/order/amend-batch HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672222935987
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "option",
    "request": [
        {
            "symbol": "ETH-30DEC22-500-C",
            "qty": null,
            "price": null,
            "orderIv": "6.8",
            "orderId": "b551f227-7059-4fb5-a6a6-699c04dbd2f2"
        },
        {
            "symbol": "ETH-30DEC22-700-C",
            "qty": null,
            "price": "650",
            "orderIv": null,
            "orderId": "fa6a595f-1a57-483f-b9d3-30e9c8235a52"
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
print(session.amend_batch_order(
    category="option",
    request=[
        {
            "category": "option",
            "symbol": "ETH-30DEC22-500-C",
            "orderIv": "6.8",
            "orderId": "b551f227-7059-4fb5-a6a6-699c04dbd2f2"
        },
        {
            "category": "option",
            "symbol": "ETH-30DEC22-700-C",
            "price": "650",
            "orderId": "fa6a595f-1a57-483f-b9d3-30e9c8235a52"
        }
    ]
))
```

```java
import com.bybit.api.client.restApi.BybitApiAsyncTradeRestClient;
import com.bybit.api.client.domain.ProductType;
import com.bybit.api.client.domain.TradeOrderType;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
import java.util.Arrays;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var amendOrderRequests = Arrays.asList(TradeOrderRequest.builder().symbol("BTC-10FEB23-24000-C").qty("0.1").price("5").orderLinkId("9b381bb1-401").build(),
                TradeOrderRequest.builder().symbol("BTC-10FEB23-24000-C").qty("0.1").price("5").orderLinkId("82ee86dd-001").build());
var amendBatchOrders = BatchOrderRequest.builder().category(ProductType.OPTION).request(amendOrderRequests).build();
client.createBatchOrder(amendBatchOrders, System.out::println);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
var order1 = new OrderRequest { Symbol = "XRPUSDT", OrderId = "xxxxxxxxxx", Qty = "10", Price = "0.6080" };
var order2 = new OrderRequest { Symbol = "BLZUSDT", OrderId = "xxxxxxxxxx", Qty = "15", Price = "0.6090" };
var orderInfoString = await TradeService.AmendBatchOrder(category:Category.LINEAR, request: new List<OrderRequest> { order1, order2 });
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .batchAmendOrders('option', [
        {
            symbol: 'ETH-30DEC22-500-C',
            orderIv: '6.8',
            orderId: 'b551f227-7059-4fb5-a6a6-699c04dbd2f2',
        },
        {
            symbol: 'ETH-30DEC22-700-C',
            price: '650',
            orderId: 'fa6a595f-1a57-483f-b9d3-30e9c8235a52',
        },
    ])
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
                "category": "option",
                "symbol": "ETH-30DEC22-500-C",
                "orderId": "b551f227-7059-4fb5-a6a6-699c04dbd2f2",
                "orderLinkId": ""
            },
            {
                "category": "option",
                "symbol": "ETH-30DEC22-700-C",
                "orderId": "fa6a595f-1a57-483f-b9d3-30e9c8235a52",
                "orderLinkId": ""
            }
        ]
    },
    "retExtInfo": {
        "list": [
            {
                "code": 0,
                "msg": "OK"
            },
            {
                "code": 0,
                "msg": "OK"
            }
        ]
    },
    "time": 1672222808060
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Batch Cancel Order

This endpoint allows you to cancel more than one open order in a single request.

important

- You must specify `orderId` or `orderLinkId`.
- If `orderId` and `orderLinkId` is not matched, the system will process `orderId` first.
- You can cancel **unfilled** or **partially filled** orders.
- A maximum of 20 orders (option), 20 orders (inverse), 20 orders (linear), 10 orders (spot) can be cancelled per request.

### HTTP Request

POST`/v5/order/cancel-batch`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `option`, `spot`, `inverse` |
| request | **true** | array | Object |
| \> symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| \> orderId | false | string | Order ID. Either `orderId` or `orderLinkId` is required |
| \> orderLinkId | false | string | User customised order ID. Either `orderId` or `orderLinkId` is required |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | Object |  |
| \> list | array | Object |
| >\> category | string | Product type |
| >\> symbol | string | Symbol name |
| >\> orderId | string | Order ID |
| >\> orderLinkId | string | User customised order ID |
| retExtInfo | Object |  |
| \> list | array | Object |
| >\> code | number | Success/error code |
| >\> msg | string | Success/error message |

info

The acknowledgement of an cancel order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/batch-cancel)

* * *

### Request Example

- HTTP
- Python
- Java
- .Net
- Node.js

```http
POST /v5/order/cancel-batch HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672223356634
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "spot",
    "request": [
        {
            "symbol": "BTCUSDT",
            "orderId": "1666800494330512128"
        },
        {
            "symbol": "ATOMUSDT",
            "orderLinkId": "1666800494330512129"
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
print(session.cancel_batch_order(
    category="spot",
    request=[
        {
            "symbol": "BTCUSDT",
            "orderId": "1666800494330512128"
        },
        {
            "symbol": "ATOMUSDT",
            "orderLinkId": "1666800494330512129"
        }
    ]
))
```

```java
import com.bybit.api.client.restApi.BybitApiTradeRestClient;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var cancelOrderRequests = Arrays.asList(TradeOrderRequest.builder().symbol("BTC-10FEB23-24000-C").orderLinkId("9b381bb1-401").build(),
                TradeOrderRequest.builder().symbol("BTC-10FEB23-24000-C").orderLinkId("82ee86dd-001").build());
var cancelBatchOrders = BatchOrderRequest.builder().category(ProductType.OPTION).request(cancelOrderRequests).build();
client.createBatchOrder(cancelBatchOrders, System.out::println);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
var order1 = new OrderRequest { Symbol = "BTC-10FEB23-24000-C", OrderLinkId = "9b381bb1-401" };
var order2 = new OrderRequest { Symbol = "BTC-10FEB23-24000-C", OrderLinkId = "82ee86dd-001" };
var orderInfoString = await TradeService.CancelBatchOrder(category: Category.LINEAR, request: new List<OrderRequest> { order1, order2 });
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .batchCancelOrders('spot', [
        {
            "symbol": "BTCUSDT",
            "orderId": "1666800494330512128"
        },
        {
            "symbol": "ATOMUSDT",
            "orderLinkId": "1666800494330512129"
        },
    ])
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
                "category": "spot",
                "symbol": "BTCUSDT",
                "orderId": "1666800494330512128",
                "orderLinkId": "spot-btc-03"
            },
            {
                "category": "spot",
                "symbol": "ATOMUSDT",
                "orderId": "",
                "orderLinkId": "1666800494330512129"
            }
        ]
    },
    "retExtInfo": {
        "list": [
            {
                "code": 0,
                "msg": "OK"
            },
            {
                "code": 170213,
                "msg": "Order does not exist."
            }
        ]
    },
    "time": 1713434299047
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Batch Place Order

tip

This endpoint allows you to place more than one order in a single request.

- Make sure you have sufficient funds in your account when placing an order. Once an order is placed, according to the
funds required by the order, the funds in your account will be frozen by the corresponding amount during the life cycle
of the order.
- A maximum of 20 orders (option), 20 orders (inverse), 20 orders (linear), 10 orders (spot) can be placed per request. The returned data list is divided into two lists.
The first list indicates whether or not the order creation was successful and the second list details the created order information. The structure of the two lists are completely consistent.

info

- **Option rate limt** instruction: its rate limit is count based on the actual number of request sent, e.g., by default, option trading rate limit is 10 reqs per sec, so you can send up to 20 \* 10 = 200 orders in one second.
- **Perpetual, Futures, Spot rate limit instruction**, please check [here](https://bybit-exchange.github.io/docs/v5/rate-limit#instructions-for-batch-endpoints)
- **Risk control limit notice:**

Bybit will monitor on your API requests. When the total number of orders of a single user (aggregated the number of orders across main account and subaccounts) within a day (UTC 0 - UTC 24) exceeds a certain upper limit, the platform will reserve the right to remind, warn, and impose necessary restrictions.
Customers who use API default to acceptance of these terms and have the obligation to cooperate with adjustments.

### HTTP Request

POST`/v5/order/create-batch`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `option`, `spot`, `inverse` |
| request | **true** | array | Object |
| \> symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| \> isLeverage | false | integer | Whether to borrow, `spot`\\*\\* only. `0`(default): false then spot trading, `1`: true then margin trading |
| \> side | **true** | string | `Buy`, `Sell` |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | **true** | string | `Market`, `Limit` |
| \> qty | **true** | string | Order quantity <br>- Spot: set `marketUnit` for market order qty unit, `quoteCoin` for market buy by default, `baseCoin` for market sell by default<br>- Perps, Futures & Option: always use base coin as unit.<br>- Perps & Futures: If you pass `qty`="0" and specify `reduceOnly`=true&`closeOnTrigger`=true, you can close the position up to `maxMktOrderQty` or `maxOrderQty` shown on [Get Instruments Info](https://bybit-exchange.github.io/docs/v5/market/instrument) of current symbol |
| \> marketUnit | false | string | The unit for `qty` when create **Spot market** orders, `orderFilter`="tpslOrder" and "StopOrder" are supported as well.<br>- `baseCoin`: for example, buy BTCUSDT, then "qty" unit is BTC<br>- `quoteCoin`: for example, sell BTCUSDT, then "qty" unit is USDT |
| \> price | false | string | Order price <br>- Market order will ignore this field<br>- Please check the min price and price precision from [instrument info](https://bybit-exchange.github.io/docs/v5/market/instrument#response-parameters) endpoint<br>- If you have position, price needs to be better than liquidation price |
| \> triggerDirection | false | integer | Conditional order param. Used to identify the expected direction of the conditional order. <br>- `1`: triggered when market price rises to `triggerPrice`<br>- `2`: triggered when market price falls to `triggerPrice`<br>Valid for `linear` |
| \> orderFilter | false | string | If it is not passed, `Order` by default. <br>- `Order`<br>- `tpslOrder`: Spot TP/SL order, the assets are occupied even before the order is triggered<br>- `StopOrder`: Spot conditional order, the assets will not be occupied until the price of the underlying asset reaches the trigger price, and the required assets will be occupied after the Conditional order is triggered<br>Valid for `spot` **only** |
| \> triggerPrice | false | string | - For Perps & Futures, it is the conditional order trigger price. If you expect the price to rise to trigger your conditional order, make sure:<br>  <br>  _triggerPrice > market price_<br>  <br>  Else, _triggerPrice < market price_<br>- For spot, it is the `orderFilter`="tpslOrder", or "StopOrder" trigger price |
| \> [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Conditional order param (Perps & Futures). Trigger price type. `LastPrice`, `IndexPrice`, `MarkPrice` |
| \> orderIv | false | string | Implied volatility. `option` **only**. Pass the real value, e.g for 10%, 0.1 should be passed. `orderIv` has a higher priority when `price` is passed as well |
| \> [timeInForce](https://bybit-exchange.github.io/docs/v5/enum#timeinforce) | false | string | [Time in force](https://www.bybit.com/en/help-center/article/What-Are-Time-In-Force-TIF-GTC-IOC-FOK) <br>- Market order will use `IOC` directly<br>- If not passed, `GTC` is used by default |
| \> [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | false | integer | Used to identify positions in different position modes. Under hedge-mode, this param is **required** <br>- `0`: one-way mode<br>- `1`: hedge-mode Buy side<br>- `2`: hedge-mode Sell side |
| \> orderLinkId | false | string | User customised order ID. A max of 36 characters. Combinations of numbers, letters (upper and lower cases), dashes, and underscores are supported.<br>_Futures, Perps & Spot: orderLinkId rules:_ <br>- optional param<br>- always unique<br> _Options orderLinkId rules:_ <br>- **required** param<br>- always unique |
| \> takeProfit | false | string | Take profit price |
| \> stopLoss | false | string | Stop loss price |
| \> [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger take profit. `MarkPrice`, `IndexPrice`, default: `LastPrice`.<br>Valid for `linear`, `inverse` |
| \> [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger stop loss. `MarkPrice`, `IndexPrice`, default: `LastPrice`<br>Valid for `linear`, `inverse` |
| \> reduceOnly | false | boolean | [What is a reduce-only order?](https://www.bybit.com/en/help-center/article/Reduce-Only-Order)`true` means your position can only reduce in size if this order is triggered. <br>- You **must** specify it as `true` when you are about to close/reduce the position<br>- When reduceOnly is true, take profit/stop loss cannot be set<br>Valid for `linear`, `inverse` & `option` |
| \> closeOnTrigger | false | boolean | [What is a close on trigger order?](https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order) For a closing order. It can only reduce your position, not increase it. If the account has insufficient available balance when the closing order is triggered, then other active orders of similar contracts will be cancelled or reduced. It can be used to ensure your stop loss reduces your position regardless of current available margin.<br>Valid for `linear`, `inverse` |
| \> [smpType](https://bybit-exchange.github.io/docs/v5/enum#smptype) | false | string | Smp execution type. [What is SMP?](https://bybit-exchange.github.io/docs/v5/smp) |
| \> mmp | false | boolean | Market maker protection. `option` **only**. `true` means set the order as a market maker protection order. [What is mmp?](https://bybit-exchange.github.io/docs/v5/account/set-mmp) |
| \> tpslMode | false | string | TP/SL mode <br>- `Full`: entire position for TP/SL. Then, tpOrderType or slOrderType must be `Market`<br>- `Partial`: partial position tp/sl (as there is no size option, so it will create tp/sl orders with the qty you actually fill). Limit TP/SL order are supported. Note: When create limit tp/sl, tpslMode is **required** and it must be `Partial`<br>Valid for `linear`, `inverse` |
| \> tpLimitPrice | false | string | The limit order price when take profit price is triggered <br>- `linear`&`inverse`: only works when tpslMode=Partial and tpOrderType=Limit<br>- Spot: it is required when the order has `takeProfit` and `tpOrderType=Limit` |
| \> slLimitPrice | false | string | The limit order price when stop loss price is triggered<br>- `linear`&`inverse`: only works when tpslMode=Partial and slOrderType=Limit<br>- Spot: it is required when the order has `stopLoss` and `slOrderType=Limit` |
| \> tpOrderType | false | string | The order type when take profit is triggered <br>- `linear`&`inverse`: `Market`(default), `Limit`. For tpslMode=Full, it only supports tpOrderType=Market<br>- Spot: <br>  <br>  `Market`: when you set "takeProfit", <br>  <br>  `Limit`: when you set "takeProfit" and "tpLimitPrice" |
| \> slOrderType | false | string | The order type when stop loss is triggered <br>- `linear`&`inverse`: `Market`(default), `Limit`. For tpslMode=Full, it only supports slOrderType=Market<br>- Spot: <br>  <br>  `Market`: when you set "stopLoss", <br>  <br>  `Limit`: when you set "stopLoss" and "slLimitPrice" |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | Object |  |
| \> list | array | Object |
| >\> category | string | Product type |
| >\> symbol | string | Symbol name |
| >\> orderId | string | Order ID |
| >\> orderLinkId | string | User customised order ID |
| >\> createAt | string | Order created time (ms) |
| retExtInfo | Object |  |
| \> list | array | Object |
| >\> code | number | Success/error code |
| >\> msg | string | Success/error message |

info

The acknowledgement of an place order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/batch-place)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- .Net
- Node.js

```http
POST /v5/order/create-batch HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672222064519
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "category": "spot",
    "request": [
        {
            "symbol": "BTCUSDT",
            "side": "Buy",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "0.05",
            "price": "30000",
            "timeInForce": "GTC",
            "orderLinkId": "spot-btc-03"
        },
        {
            "symbol": "ATOMUSDT",
            "side": "Sell",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "2",
            "price": "12",
            "timeInForce": "GTC",
            "orderLinkId": "spot-atom-03"
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
print(session.place_batch_order(
    category="spot",
    request=[
        {
            "symbol": "BTCUSDT",
            "side": "Buy",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "0.05",
            "price": "30000",
            "timeInForce": "GTC",
            "orderLinkId": "spot-btc-03"
        },
        {
            "symbol": "ATOMUSDT",
            "side": "Sell",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "2",
            "price": "12",
            "timeInForce": "GTC",
            "orderLinkId": "spot-atom-03"
        }
    ]
))
```

```go
import (
    "context"
    "fmt"
    bybit "https://github.com/bybit-exchange/bybit.go.api")
client := bybit.NewBybitHttpClient("YOUR_API_KEY", "YOUR_API_SECRET", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{"category": "option",
    "request": []map[string]interface{}{
        {
            "category":    "option",
            "symbol":      "BTC-10FEB23-24000-C",
            "orderType":   "Limit",
            "side":        "Buy",
            "qty":         "0.1",
            "price":       "5",
            "orderIv":     "0.1",
            "timeInForce": "GTC",
            "orderLinkId": "9b381bb1-401",
            "mmp":         false,
            "reduceOnly":  false,
        },
        {
            "category":    "option",
            "symbol":      "BTC-10FEB23-24000-C",
            "orderType":   "Limit",
            "side":        "Buy",
            "qty":         "0.1",
            "price":       "5",
            "orderIv":     "0.1",
            "timeInForce": "GTC",
            "orderLinkId": "82ee86dd-001",
            "mmp":         false,
            "reduceOnly":  false,
        },
    },
}
client.NewUtaBybitServiceWithParams(params).PlaceBatchOrder(context.Background())
```

```java
import com.bybit.api.client.restApi.BybitApiAsyncTradeRestClient;
import com.bybit.api.client.domain.ProductType;
import com.bybit.api.client.domain.TradeOrderType;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
import java.util.Arrays;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var orderRequests = Arrays.asList(TradeOrderRequest.builder().category(ProductType.OPTION).symbol("BTC-10FEB23-24000-C").side(Side.BUY).orderType(TradeOrderType.LIMIT).qty("0.1")
                        .price("5").orderIv("0.1").timeInForce(TimeInForce.GOOD_TILL_CANCEL).orderLinkId("9b381bb1-401").mmp(false).reduceOnly(false).build(),
                TradeOrderRequest.builder().category(ProductType.OPTION).symbol("BTC-10FEB23-24000-C").side(Side.BUY).orderType(TradeOrderType.LIMIT).qty("0.1")
                        .price("5").orderIv("0.1").timeInForce(TimeInForce.GOOD_TILL_CANCEL).orderLinkId("82ee86dd-001").mmp(false).reduceOnly(false).build());
var createBatchOrders = BatchOrderRequest.builder().category(ProductType.OPTION).request(orderRequests).build();
client.createBatchOrder(createBatchOrders, System.out::println);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
var order1 = new OrderRequest { Symbol = "XRPUSDT", OrderType = "Limit", Side = "Buy", Qty = "10", Price = "0.6080", TimeInForce = "GTC" };
var order2 = new OrderRequest { Symbol = "BLZUSDT", OrderType = "Limit", Side = "Buy", Qty = "10", Price = "0.6080", TimeInForce = "GTC" };
List<OrderRequest> request = new() { order1, order2 };
var orderInfoString = await TradeService.PlaceBatchOrder(category: Category.LINEAR, request: request);
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .batchSubmitOrders('spot', [
        {
            "symbol": "BTCUSDT",
            "side": "Buy",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "0.05",
            "price": "30000",
            "timeInForce": "GTC",
            "orderLinkId": "spot-btc-03"
        },
        {
            "symbol": "ATOMUSDT",
            "side": "Sell",
            "orderType": "Limit",
            "isLeverage": 0,
            "qty": "2",
            "price": "12",
            "timeInForce": "GTC",
            "orderLinkId": "spot-atom-03"
        },
    ])
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
                "category": "spot",
                "symbol": "BTCUSDT",
                "orderId": "1666800494330512128",
                "orderLinkId": "spot-btc-03",
                "createAt": "1713434102752"
            },
            {
                "category": "spot",
                "symbol": "ATOMUSDT",
                "orderId": "1666800494330512129",
                "orderLinkId": "spot-atom-03",
                "createAt": "1713434102752"
            }
        ]
    },
    "retExtInfo": {
        "list": [
            {
                "code": 0,
                "msg": "OK"
            },
            {
                "code": 0,
                "msg": "OK"
            }
        ]
    },
    "time": 1713434102753
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Cancel All Orders

Cancel all open orders

info

- Support cancel orders by `symbol`/`baseCoin`/`settleCoin`. If you pass multiple of these params, the system will process one of param, which priority is `symbol` \> `baseCoin` \> `settleCoin`.
- **NOTE**: category= _option_, you can cancel all option open orders without passing any of those three params. However, for "linear" and "inverse", you must specify one of those three params.
- **NOTE**: category= _spot_, you can cancel all spot open orders (normal order by default) without passing other params.

info

**Spot**: no limit

**Futures**: cancel up to 500 orders (System **picks up 500 orders randomly to cancel** when you have over 500 orders)

**Options**: no limit

### HTTP Request

POST`/v5/order/cancel-all`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only<br>`linear`&`inverse`: **Required** if not passing baseCoin or settleCoin |
| baseCoin | false | string | Base coin, uppercase only. `linear` & `inverse`: If cancel all by baseCoin, it will cancel all of the corresponding category's orders. **Required** if not passing symbol or settleCoin |
| settleCoin | false | string | Settle coin, uppercase only <br>- `linear` & `inverse`: **Required** if not passing symbol or baseCoin<br>- `option`: USDT or USDC<br>- Not support `spot` |
| orderFilter | false | string | - category=`spot`, you can pass `Order`, `tpslOrder`, `StopOrder`, `OcoOrder`, `BidirectionalTpslOrder`<br>  <br>  If not passed, `Order` by default<br>- category=`linear` or `inverse`, you can pass `Order`, `StopOrder`,`OpenOrder`<br>  <br>  If not passed, all kinds of orders will be cancelled, like active order, conditional order, TP/SL order and trailing stop order<br>- category=`option`, you can pass `Order`,`StopOrder`<br>  <br>   If not passed, all kinds of orders will be cancelled, like active order, conditional order, TP/SL order and trailing stop order |
| [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | false | string | Stop order type `Stop`<br>- Only used for category=`linear` or `inverse` and orderFilter=`StopOrder`,you can cancel conditional orders except TP/SL order and Trailing stop orders with this param |

info

The acknowledgement of create/amend/cancel order requests indicates that the request was sucessfully accepted. The request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/cancel-all)

* * *

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customised order ID |
| success | string | "1": success, "0": fail. [UTA1.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-10) (inverse) does not return this field |

### Request Example

- HTTP
- Python
- Java
- .Net
- Node.js

```http
POST /v5/order/cancel-all HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672219779140
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
  "category": "linear",
  "symbol": null,
  "settleCoin": "USDT"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.cancel_all_orders(
    category="linear",
    settleCoin="USDT",
))
```

```java
import com.bybit.api.client.restApi.BybitApiTradeRestClient;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var cancelAllOrdersRequest = TradeOrderRequest.builder().category(ProductType.LINEAR).baseCoin("USDT").build();
client.cancelAllOrder(cancelAllOrdersRequest, System.out::println);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
BybitTradeService tradeService = new(apiKey: "xxxxxxxxxxxxxx", apiSecret: "xxxxxxxxxxxxxxxxxxxxx");
var orderInfoString = await TradeService.CancelAllOrder(category: Category.LINEAR, baseCoin:"USDT");
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .cancelAllOrders({
    category: 'linear',
    settleCoin: 'USDT',
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
                "orderId": "1616024329462743808",
                "orderLinkId": "1616024329462743809"
            },
            {
                "orderId": "1616024287544869632",
                "orderLinkId": "1616024287544869633"
            }
        ],
        "success": "1"
    },
    "retExtInfo": {},
    "time": 1707381118116
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Cancel Order

important

- You must specify `orderId` or `orderLinkId` to cancel the order.
- If `orderId` and `orderLinkId` do not match, the system will process `orderId` first.
- You can only cancel **unfilled** or **partially filled** orders.

### HTTP Request

POST`/v5/order/cancel`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `linear`, `inverse`, `spot`, `option` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| orderId | false | string | Order ID. Either `orderId` or `orderLinkId` is **required** |
| orderLinkId | false | string | User customised order ID. Either `orderId` or `orderLinkId` is **required** |
| orderFilter | false | string | Spot trading **only** <br>- `Order`<br>- `tpslOrder`<br>- `StopOrder`<br>If not passed, `Order` by default |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Order ID |
| orderLinkId | string | User customised order ID |

info

The acknowledgement of an cancel order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/cancel-order)

* * *

### Request Example

- HTTP
- Python
- Java
- .Net
- Node.js

```http
POST /v5/order/cancel HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672217376681
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
  "category": "linear",
  "symbol": "BTCPERP",
  "orderLinkId": null,
  "orderId":"c6f055d9-7f21-4079-913d-e6523a9cfffa"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.cancel_order(
    category="linear",
    symbol="BTCPERP",
    orderId="c6f055d9-7f21-4079-913d-e6523a9cfffa",
))
```

```java
import com.bybit.api.client.restApi.BybitApiTradeRestClient;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
var cancelOrderRequest = TradeOrderRequest.builder().category(ProductType.SPOT).symbol("XRPUSDT").orderId("1523347543495541248").build();
var canceledOrder = client.cancelOrder(cancelOrderRequest);
System.out.println(canceledOrder);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
BybitTradeService tradeService = new(apiKey: "xxxxxxxxxxxxxx", apiSecret: "xxxxxxxxxxxxxxxxxxxxx");
var orderInfoString = await TradeService.CancelOrder(orderId: "1523347543495541248", category: Category.SPOT, symbol: "XRPUSDT");
Console.WriteLine(orderInfoString);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .cancelOrder({
        category: 'linear',
        symbol: 'BTCPERP',
        orderId: 'c6f055d9-7f21-4079-913d-e6523a9cfffa',
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
        "orderId": "c6f055d9-7f21-4079-913d-e6523a9cfffa",
        "orderLinkId": "linear-004"
    },
    "retExtInfo": {},
    "time": 1672217377164
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Place Order

This endpoint supports to create the order for Spot, Margin trading, USDT perpetual, USDT futures, USDC perpetual, USDC futures, Inverse Futures and Options.

info

- **Supported order type (`orderType`):**

Limit order: `orderType`= _Limit_, it is necessary to specify order qty and price.

[Market order](https://www.bybit.com/en/help-center/article/Types-of-Orders-Available-on-Bybit): `orderType`= _Market_, execute at the best price in the Bybit market until the transaction is completed. When selecting a market order, the "price" can be empty. In the trading system, in order to protect traders against the serious slippage of the Market order, Bybit trading engine will convert the market order into an IOC limit order for matching. If there are no orderbook entries within price slippage limit, the order will not be executed. If there is insufficient liquidity, the order will be cancelled. The slippage threshold refers to the percentage that the order price deviates from the mark price. You can learn more here: [Adjustments to Bybit's Derivative Trading Price Limit Mechanism](https://announcements.bybit.com/en/article/adjustments-to-bybit-s-derivative-trading-limit-order-mechanism-blt469228de1902fff6/)
- **Supported timeInForce strategy:**

`GTC`

`IOC`

`FOK`

`PostOnly`: If the order would be filled immediately when submitted, it will be **cancelled**. The purpose of this is to protect your order during the submission process. If the matching system cannot entrust the order to the order book due to price changes on the market, it will be cancelled.

`RPI`: Retail Price Improvement order. Assigned market maker can place this kind of order, and it is a post only order, only match with the order from Web or APP.

- **How to create a conditional order:**

When submitting an order, if `triggerPrice` is set, the order will be automatically converted into a conditional order. In addition, the conditional order does not occupy the margin. If the margin is insufficient after the conditional order is triggered, the order will be cancelled.

- **[Take profit / Stop loss](https://www.bybit.com/en/help-center/article/Introduction-to-Take-Profit-Stop-Loss-Perpetual-Futures-Contracts)**: You can set TP/SL while placing orders. Besides, you could modify the position's TP/SL.

- **Order quantity**: The quantity of perpetual contracts you are going to buy/sell. For the order quantity, Bybit only supports positive number at present.

- **Order price**: Place a limit order, this parameter is **required**. If you have position, the price should be higher than the _liquidation price_.
For the minimum unit of the price change, please refer to the `priceFilter` \> `tickSize` field in the [instruments-info](https://bybit-exchange.github.io/docs/v5/market/instrument) endpoint.

- **orderLinkId**: You can customize the active order ID. We can link this ID to the order ID in the system. Once the
active order is successfully created, we will send the unique order ID in the system to you. Then, you can use this order
ID to cancel active orders, and if both orderId and orderLinkId are entered in the parameter input, Bybit will prioritize the orderId to process the corresponding order. Meanwhile, your customized order ID should be no longer than 36 characters and should be **unique**.

- **Open orders up limit:**

**Perps & Futures:**

a) Each account can hold a maximum of _500_ **active** orders simultaneously **per symbol.**

b) **conditional** orders: each account can hold a maximum of **10 active orders** simultaneously **per symbol**.

**Spot:** 500 orders in total, including a maximum of 30 open TP/SL orders, a maximum of 30 open conditional orders for each symbol per account

**Option:** a maximum of 50 open orders in the coin dimension by default.

- **Rate limit:**

Please refer to [rate limit table](https://bybit-exchange.github.io/docs/v5/rate-limit#trade). If you need to raise the rate limit, please contact your client manager or submit an application via [here](https://www.bybit.com/future-activity/en-US/institutional-services)

- **Risk control limit notice:**

Bybit will monitor on your API requests. When the total number of orders of a single user (aggregated the number of orders across main account and subaccounts) within a day (UTC 0 - UTC 24) exceeds a certain upper limit, the platform will reserve the right to remind, warn, and impose necessary restrictions.
Customers who use API default to acceptance of these terms and have the obligation to cooperate with adjustments.

- **Reduce only orders:**

If reduceOnly=true and order qty > max order qty, the order will automatically be split up into multiple orders.

### HTTP Request

POST`/v5/order/create`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| category | **true** | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | **true** | string | Symbol name, like `BTCUSDT`, uppercase only |
| isLeverage | false | integer | Whether to borrow. <br>- `0`(default): false, spot trading<br>- `1`: true, margin trading, _make sure you turn on margin trading, and set the relevant currency as collateral_ |
| side | **true** | string | `Buy`, `Sell` |
| [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | **true** | string | `Market`, `Limit` |
| qty | **true** | string | Order quantity <br>- Spot: Market Buy order by value by default, you can set `marketUnit` field to choose order by value or qty for market orders<br>- Perps, Futures & Option: always order by qty<br>- Perps & Futures: if you pass `qty`="0" and specify `reduceOnly`=true&`closeOnTrigger`=true, you can close the position up to `maxMktOrderQty` or `maxOrderQty` shown on [Get Instruments Info](https://bybit-exchange.github.io/docs/v5/market/instrument) of current symbol |
| marketUnit | false | string | Select the unit for `qty` when create **Spot market** orders<br>- `baseCoin`: for example, buy BTCUSDT, then "qty" unit is BTC<br>- `quoteCoin`: for example, sell BTCUSDT, then "qty" unit is USDT |
| slippageToleranceType | false | string | Slippage tolerance Type for **market order**, `TickSize`, `Percent`<br>- take profit, stoploss, conditional orders are not supported<br>- **TickSize**: <br>  <br>  the highest price of Buy order = ask1 + `slippageTolerance` x tickSize; <br>  <br>  the lowest price of Sell order = bid1 - `slippageTolerance` x tickSize<br>- **Percent**: <br>  <br>  the highest price of Buy order = ask1 x (1 + `slippageTolerance` x 0.01); <br>  <br>  the lowest price of Sell order = bid1 x (1 - `slippageTolerance` x 0.01)<br>- Learn more about slippage tolerance in the [help centre](https://www.bybit.com/en/help-center/article/Market-Order-with-Slippage-Tolerance) |
| slippageTolerance | false | string | Slippage tolerance value <br>- `TickSize`: range is \[1, 10000\], integer only<br>- `Percent`: range is \[0.01, 10\], up to 2 decimals |
| price | false | string | Order price <br>- Market order will ignore this field<br>- Please check the min price and price precision from [instrument info](https://bybit-exchange.github.io/docs/v5/market/instrument#response-parameters) endpoint<br>- If you have position, price needs to be better than liquidation price |
| triggerDirection | false | integer | Conditional order param. Used to identify the expected direction of the conditional order. <br>- `1`: triggered when market price rises to `triggerPrice`<br>- `2`: triggered when market price falls to `triggerPrice`<br>Valid for `linear` & `inverse` |
| orderFilter | false | string | If it is not passed, `Order` by default. <br>- `Order`<br>- `tpslOrder`: Spot TP/SL order, the assets are occupied even before the order is triggered<br>- `StopOrder`: Spot conditional order, the assets will not be occupied until the price of the underlying asset reaches the trigger price, and the required assets will be occupied after the Conditional order is triggered<br>Valid for `spot` **only** |
| triggerPrice | false | string | - For Perps & Futures, it is the conditional order trigger price. If you expect the price to rise to trigger your conditional order, make sure:<br>  <br>  _triggerPrice > market price_<br>  <br>  Else, _triggerPrice < market price_<br>- For spot, it is the TP/SL and Conditional order trigger price |
| [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | Trigger price type, Conditional order param for Perps & Futures. <br>- `LastPrice`<br>- `IndexPrice`<br>- `MarkPrice`<br>Valid for `linear` & `inverse` |
| orderIv | false | string | Implied volatility. `option` **only**. Pass the real value, e.g for 10%, 0.1 should be passed. `orderIv` has a higher priority when `price` is passed as well |
| [timeInForce](https://bybit-exchange.github.io/docs/v5/enum#timeinforce) | false | string | [Time in force](https://www.bybit.com/en/help-center/article/What-Are-Time-In-Force-TIF-GTC-IOC-FOK) <br>- Market order will always use `IOC`<br>- If not passed, `GTC` is used by default |
| [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | false | integer | Used to identify positions in different position modes. Under hedge-mode, this param is **required** <br>- `0`: one-way mode<br>- `1`: hedge-mode Buy side<br>- `2`: hedge-mode Sell side |
| \> orderLinkId | false | string | User customised order ID. A max of 36 characters. Combinations of numbers, letters (upper and lower cases), dashes, and underscores are supported.<br>_Futures, Perps & Spot: orderLinkId rules:_ <br>- optional param<br>- always unique<br> _Options orderLinkId rules:_ <br>- **required** param<br>- always unique |
| takeProfit | false | string | Take profit price<br>- Spot Limit order supports take profit, stop loss or limit take profit, limit stop loss when creating an order<br>- Option order supports full size market take profit |
| stopLoss | false | string | Stop loss price <br>- Spot Limit order supports take profit, stop loss or limit take profit, limit stop loss when creating an order<br>- Option order supports full size market stop loss |
| [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger take profit. `MarkPrice`, `IndexPrice`, default: `LastPrice`. Valid for `linear` & `inverse` |
| [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | false | string | The price type to trigger stop loss. `MarkPrice`, `IndexPrice`, default: `LastPrice`. Valid for `linear` & `inverse` |
| reduceOnly | false | boolean | [What is a reduce-only order?](https://www.bybit.com/en/help-center/article/Reduce-Only-Order)`true` means your position can only reduce in size if this order is triggered. <br>- You **must** specify it as `true` when you are about to close/reduce the position<br>- When reduceOnly is true, take profit/stop loss cannot be set<br>Valid for `linear`, `inverse` & `option` |
| closeOnTrigger | false | boolean | [What is a close on trigger order?](https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order) For a closing order. It can only reduce your position, not increase it. If the account has insufficient available balance when the closing order is triggered, then other active orders of similar contracts will be cancelled or reduced. It can be used to ensure your stop loss reduces your position regardless of current available margin.<br>Valid for `linear` & `inverse` |
| [smpType](https://bybit-exchange.github.io/docs/v5/enum#smptype) | false | string | Smp execution type. [What is SMP?](https://bybit-exchange.github.io/docs/v5/smp) |
| mmp | false | boolean | Market maker protection. `option` **only**. `true` means set the order as a market maker protection order. [What is mmp?](https://bybit-exchange.github.io/docs/v5/account/set-mmp) |
| tpslMode | false | string | TP/SL mode <br>- `Full`: entire position for TP/SL. Then, tpOrderType or slOrderType must be `Market`<br>- `Partial`: partial position tp/sl (as there is no size option, so it will create tp/sl orders with the qty you actually fill). Limit TP/SL order are supported. Note: When create limit tp/sl, tpslMode is **required** and it must be `Partial`<br>Valid for `linear` & `inverse` |
| tpLimitPrice | false | string | The limit order price when take profit price is triggered <br>- `linear` & `inverse`: only works when tpslMode=Partial and tpOrderType=Limit<br>- Spot: it is required when the order has `takeProfit` and "tpOrderType"=`Limit` |
| slLimitPrice | false | string | The limit order price when stop loss price is triggered<br>- `linear` & `inverse`: only works when tpslMode=Partial and slOrderType=Limit<br>- Spot: it is required when the order has `stopLoss` and "slOrderType"=`Limit` |
| tpOrderType | false | string | The order type when take profit is triggered <br>- `linear` & `inverse`: `Market`(default), `Limit`. For tpslMode=Full, it only supports tpOrderType=Market<br>- Spot: <br>  <br>  `Market`: when you set "takeProfit", <br>  <br>  `Limit`: when you set "takeProfit" and "tpLimitPrice" |
| slOrderType | false | string | The order type when stop loss is triggered <br>- `linear` & `inverse`: `Market`(default), `Limit`. For tpslMode=Full, it only supports slOrderType=Market<br>- Spot: <br>  <br>  `Market`: when you set "stopLoss", <br>  <br>  `Limit`: when you set "stopLoss" and "slLimitPrice" |
| bboSideType | false | string | - `Queue`: use the order price on the orderbook in the same direction as the `side`<br>- `Counterparty`: use the order price on the orderbook in the opposite direction as the `side`<br> Valid for `linear` & `inverse` |
| bboLevel | false | string | `1`,`2`,`3`,`4`,`5` Valid for `linear` & `inverse` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Order ID |
| orderLinkId | string | User customised order ID |

info

The acknowledgement of an place order request indicates that the request was sucessfully accepted. This request is asynchronous so please use the websocket to confirm the order status.

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/create-order)

* * *

### Request Example

- HTTP
- Python
- Go
- Java
- .Net
- Node.js

```http
POST /v5/order/create HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672211928338
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

// Spot Limit order with market tp sl
{"category": "spot","symbol": "BTCUSDT","side": "Buy","orderType": "Limit","qty": "0.01","price": "28000","timeInForce": "PostOnly","takeProfit": "35000","stopLoss": "27000","tpOrderType": "Market","slOrderType": "Market"}

// Spot Limit order with limit tp sl
{"category": "spot","symbol": "BTCUSDT","side": "Buy","orderType": "Limit","qty": "0.01","price": "28000","timeInForce": "PostOnly","takeProfit": "35000","stopLoss": "27000","tpLimitPrice": "36000","slLimitPrice": "27500","tpOrderType": "Limit","slOrderType": "Limit"}

// Spot PostOnly normal order
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","timeInForce":"PostOnly","orderLinkId":"spot-test-01","isLeverage":0,"orderFilter":"Order"}

// Spot TP/SL order
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","triggerPrice": "15000", "timeInForce":"Limit","orderLinkId":"spot-test-02","isLeverage":0,"orderFilter":"tpslOrder"}

// Spot margin normal order (UTA)
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","timeInForce":"GTC","orderLinkId":"spot-test-limit","isLeverage":1,"orderFilter":"Order"}

// Spot Market Buy order, qty is quote currency
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Market","qty":"200","timeInForce":"IOC","orderLinkId":"spot-test-04","isLeverage":0,"orderFilter":"Order"}

// USDT Perp open long position (one-way mode)
{"category":"linear","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"1","price":"25000","timeInForce":"GTC","positionIdx":0,"orderLinkId":"usdt-test-01","reduceOnly":false,"takeProfit":"28000","stopLoss":"20000","tpslMode":"Partial","tpOrderType":"Limit","slOrderType":"Limit","tpLimitPrice":"27500","slLimitPrice":"20500"}

// USDT Perp close long position (one-way mode)
{"category": "linear", "symbol": "BTCUSDT", "side": "Sell", "orderType": "Limit", "qty": "1", "price": "30000", "timeInForce": "GTC", "positionIdx": 0, "orderLinkId": "usdt-test-02", "reduceOnly": true}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.place_order(
    category="spot",
    symbol="BTCUSDT",
    side="Buy",
    orderType="Limit",
    qty="0.1",
    price="15600",
    timeInForce="PostOnly",
    orderLinkId="spot-test-postonly",
    isLeverage=0,
    orderFilter="Order",
))
```

```go
import (
    "context"
    "fmt"
    bybit "https://github.com/bybit-exchange/bybit.go.api")
client := bybit.NewBybitHttpClient("YOUR_API_KEY", "YOUR_API_SECRET", bybit.WithBaseURL(bybit.TESTNET))
params := map[string]interface{}{
        "category":    "linear",
        "symbol":      "BTCUSDT",
        "side":        "Buy",
        "positionIdx": 0,
        "orderType":   "Limit",
        "qty":         "0.001",
        "price":       "10000",
        "timeInForce": "GTC",
    }
client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
```

```java
import com.bybit.api.client.restApi.BybitApiAsyncTradeRestClient;
import com.bybit.api.client.domain.ProductType;
import com.bybit.api.client.domain.TradeOrderType;
import com.bybit.api.client.domain.trade.PositionIdx;
import com.bybit.api.client.domain.trade.Side;
import com.bybit.api.client.domain.trade.TimeInForce;
import com.bybit.api.client.domain.trade.TradeOrderRequest;
import com.bybit.api.client.service.BybitApiClientFactory;
import java.util.Map;
BybitApiClientFactory factory = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET");
BybitApiAsyncTradeRestClient client = factory.newAsyncTradeRestClient();
Map<String, Object> order =Map.of(
                  "category", "option",
                  "symbol", "BTC-29DEC23-10000-P",
                  "side", "Buy",
                  "orderType", "Limit",
                  "orderIv", "0.1",
                  "qty", "0.1",
                  "price", "5",
                  "orderLinkId", "test_orderLinkId_1"
                );
client.createOrder(order, System.out::println);
```

```c#
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models.Trade;
BybitTradeService tradeService = new(apiKey: "xxxxxxxxxxxxxx", apiSecret: "xxxxxxxxxxxxxxxxxxxxx");
var orderInfo = await tradeService.PlaceOrder(category: Category.LINEAR, symbol: "BLZUSDT", side: Side.BUY, orderType: OrderType.MARKET, qty: "15", timeInForce: TimeInForce.GTC);
Console.WriteLine(orderInfo);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

// Submit a market order
client
  .submitOrder({
    category: 'spot',
    symbol: 'BTCUSDT',
    side: 'Buy',
    orderType: 'Market',
    qty: '1',
  })
  .then((response) => {
    console.log('Market order result', response);
  })
  .catch((error) => {
    console.error('Market order error', error);
  });

// Submit a limit order
client
  .submitOrder({
    category: 'spot',
    symbol: 'BTCUSDT',
    side: 'Buy',
    orderType: 'Limit',
    qty: '1',
    price: '55000',
  })
  .then((response) => {
    console.log('Limit order result', response);
  })
  .catch((error) => {
    console.error('Limit order error', error);
  });
```

### Response Example

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

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Disconnect Cancel All

info

## What is Disconnection Protect (DCP)?

Based on the websocket private connection and heartbeat mechanism, Bybit provides disconnection protection function. The
timing starts from the first disconnection. If the Bybit server does not receive the reconnection from the client for
more than 10 (default) seconds and resumes the heartbeat "ping", then the client is in the state of "disconnection protect",
all active **futures / spot / option** orders of the client will be cancelled automatically. If within 10 seconds, the client reconnects
and resumes the heartbeat "ping", the timing will be reset and restarted at the next disconnection.

## How to enable DCP

- If you need to turn it on/off, you can contact your client manager for consultation and application. The default time window is 10 seconds.
- DCP feature is only available for Ins clients. VIP clients cannot access this feature

## Applicable

Effective for **Inverse Perp / Inverse Futures / USDT Perp / USDT Futures / USDC Perp / USDC Futures / Spot / options**

tip

After the request is successfully sent, the system needs a certain time to take effect. It is recommended to query or set again after 10 seconds

- You can use [this endpoint](https://bybit-exchange.github.io/docs/v5/account/dcp-info) to get your current DCP configuration.
- Your private websocket connection **must** subscribe ["dcp" topic](https://bybit-exchange.github.io/docs/v5/websocket/private/dcp) in order to trigger DCP successfully

### HTTP Request

POST`/v5/order/disconnected-cancel-all`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| product | false | string | `OPTIONS`(default), `DERIVATIVES`, `SPOT` |
| timeWindow | **true** | integer | Disconnection timing window time. \[`3`, `300`\], unit: second |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
POST v5/order/disconnected-cancel-all HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675852742375
X-BAPI-RECV-WINDOW: 50000
Content-Type: application/json

{
  "timeWindow": 40
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_dcp(
    timeWindow=40,
))
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.trade.request.TradeOrderRequest;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newTradeRestClient();
var setDcpOptionsRequest = TradeOrderRequest.builder().timeWindow(40).build();
System.out.println(client.setDisconnectCancelAllTime(setDcpOptionsRequest));
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setDisconnectCancelAllWindow('option', 40)
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
    "retMsg": "success"
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Trade History

Query users' execution records, sorted by `execTime` in descending order.

tip

- Response items will have sorting issues when 'execTime' is the same, it is recommended to sort according to `execId+OrderId+leavesQty`.
If you want to receive real-time execution information, Use the [websocket stream](https://bybit-exchange.github.io/docs/v5/websocket/private/execution) (recommended).
- You may have multiple executions in a single order.
- You can query by symbol, baseCoin, orderId and orderLinkId, and if you pass multiple params, the system will process them according to this priority: orderId > orderLinkId > symbol > baseCoin. orderId and orderLinkId have a higher priority and as long as these two parameters are in the input parameters, other input parameters will be ignored.

### HTTP Request

GET`/v5/execution/list`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| orderId | false | string | Order ID |
| orderLinkId | false | string | User customised order ID |
| baseCoin | false | string | Base coin, uppercase only. For type `option`, default value is BTC |
| settleCoin | false | string | Settle coin, uppercase only. Only for `linear`, `inverse`,`option` |
| startTime | false | integer | The start timestamp (ms) <br>- startTime and endTime are not passed, return 7 days by default<br>- Only startTime is passed, return range between startTime and startTime+7 days<br>- Only endTime is passed, return range between endTime-7 days and endTime<br>  <br>  If both are passed, the rule is endTime - startTime <= 7 days |
| endTime | false | integer | The end timestamp (ms) |
| [execType](https://bybit-exchange.github.io/docs/v5/enum#exectype) | false | string | Execution type |
| limit | false | integer | Limit for data size per page. \[`1`, `100`\]. Default: `50` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type |
| list | array | Object |
| \> symbol | string | Symbol name |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customized order ID |
| \> side | string | Side. `Buy`,`Sell` |
| \> orderPrice | string | Order price |
| \> orderQty | string | Order qty |
| \> leavesQty | string | The remaining qty not executed |
| \> [createType](https://bybit-exchange.github.io/docs/v5/enum#createtype) | string | Order create type Spot does not have this key |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit` |
| \> [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | string | Stop order type. If the order is not stop order, it either returns `UNKNOWN` or `""` |
| \> execFee | string | Executed trading fee. You can get spot fee currency instruction [here](https://bybit-exchange.github.io/docs/v5/enum#spot-fee-currency-instruction) |
| \> execFeeV2 | string | Spot leg transaction fee, only works for execType=`FutureSpread` |
| \> execId | string | Execution ID |
| \> execPrice | string | Execution price |
| \> execQty | string | Execution qty |
| \> [execType](https://bybit-exchange.github.io/docs/v5/enum#exectype) | string | Executed type |
| \> execValue | string | Executed order value |
| \> execTime | string | Executed timestamp (ms) |
| \> feeCurrency | string | Trading fee currency |
| \> isMaker | boolean | Is maker order. `true`: maker, `false`: taker |
| \> feeRate | string | Trading fee rate |
| \> tradeIv | string | Implied volatility. _Valid for `option`_ |
| \> markIv | string | Implied volatility of mark price. _Valid for `option`_ |
| \> markPrice | string | The mark price of the symbol when executing |
| \> indexPrice | string | The index price of the symbol when executing. _Valid for `option` only_ |
| \> underlyingPrice | string | The underlying price of the symbol when executing. _Valid for `option`_ |
| \> blockTradeId | string | Paradigm block trade ID |
| \> closedSize | string | Closed position size |
| \> seq | long | Cross sequence, used to associate each fill and each position update<br>- The seq will be the same when conclude multiple transactions at the same time<br>- Different symbols may have the same seq, please use seq + symbol to check unique |
| \> extraFees | string | Trading fee rate information. Currently, this data is returned only for kyc=Indian user or spot orders placed on the Indonesian site or spot fiat currency orders placed on the EU site. In other cases, an empty string is returned. Enum: [feeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeesfeetype), [subFeeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeessubfeetype) |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/position/execution)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/execution/list?category=linear&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672283754132
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_executions(
    category="linear",
    limit=1,
))
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.trade.request.TradeOrderRequest;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newTradeRestClient();
var tradeHistoryRequest = TradeOrderRequest.builder().category(CategoryType.LINEAR).symbol("BTCUSDT").execType(ExecType.Trade).limit(100).build();
System.out.println(client.getTradeHistory(tradeHistoryRequest));
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getExecutionList({
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
        "nextPageCursor": "132766%3A2%2C132766%3A2",
        "category": "linear",
        "list": [
            {
                "symbol": "ETHPERP",
                "orderType": "Market",
                "underlyingPrice": "",
                "orderLinkId": "",
                "side": "Buy",
                "indexPrice": "",
                "orderId": "8c065341-7b52-4ca9-ac2c-37e31ac55c94",
                "stopOrderType": "UNKNOWN",
                "leavesQty": "0",
                "execTime": "1672282722429",
                "feeCurrency": "",
                "isMaker": false,
                "execFee": "0.071409",
                "feeRate": "0.0006",
                "execId": "e0cbe81d-0f18-5866-9415-cf319b5dab3b",
                "tradeIv": "",
                "blockTradeId": "",
                "markPrice": "1183.54",
                "execPrice": "1190.15",
                "markIv": "",
                "orderQty": "0.1",
                "orderPrice": "1236.9",
                "execValue": "119.015",
                "execType": "Trade",
                "execQty": "0.1",
                "closedSize": "",
                "extraFees": "",
                "seq": 4688002127
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672283754510
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Open & Closed Orders

Primarily query unfilled or partially filled orders in **real-time**, but also supports querying recent 500 closed status (Cancelled, Filled) orders. Please see the usage of request param `openOnly`.

And to query older order records, please use the [order history](https://bybit-exchange.github.io/docs/v5/order/order-list) interface.

tip

- You can query filled, cancelled, and rejected orders to the most recent 500 orders for spot, linear, inverse and option categories
- You can query by symbol, baseCoin, orderId and orderLinkId, and if you pass multiple params, the system will process them according to this priority: orderId > orderLinkId > symbol > baseCoin.
- The records are sorted by the `createdTime` from newest to oldest.

info

- After a server release or restart, filled, cancelled, and rejected orders of Unified account should only be queried through [order history](https://bybit-exchange.github.io/docs/v5/order/order-list).
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/order/realtime`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| category | **true** | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only. For **linear**, either `symbol`, `baseCoin`, `settleCoin` is **required** |
| baseCoin | false | string | Base coin, uppercase only <br>- Supports `linear`, `inverse` & `option`<br>- `option`: it returns all option open orders by default |
| settleCoin | false | string | Settle coin, uppercase only <br>- **linear**: either `symbol`, `baseCoin` or `settleCoin` is **required**<br>- `spot`: not supported<br>- `option`: USDT or USDC |
| orderId | false | string | Order ID |
| orderLinkId | false | string | User customised order ID |
| openOnly | false | integer | - `0`(default): query open status orders (e.g., New, PartiallyFilled) **only**<br>- `1`: Query a maximum of recent 500 closed status records are kept under each account each category (e.g., Cancelled, Rejected, Filled orders).<br>  <br>  _If the Bybit service is restarted due to an update, this part of the data will be cleared and accumulated again, but the order records will still be queried in [order history](https://bybit-exchange.github.io/docs/v5/order/order-list)_<br>- `openOnly` param will be ignored when query by _orderId_ or _orderLinkId_ |
| orderFilter | false | string | `Order`: active order, `StopOrder`: conditional order for Futures and Spot, `tpslOrder`: spot TP/SL order, `OcoOrder`: Spot oco order, `BidirectionalTpslOrder`: Spot bidirectional TPSL order<br>- all kinds of orders by default |
| limit | false | integer | Limit for data size per page. \[`1`, `50`\]. Default: `20` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| nextPageCursor | string | Refer to the `cursor` request parameter |
| list | array | Object |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customised order ID |
| \> parentOrderLinkId | string | Indicates the linked parent order for attached take-profit and stop-loss orders. Supported for futures and options.<br>- [Amending](https://bybit-exchange.github.io/docs/v5/order/amend-order) take-profit or stop-loss orders does not change the parentOrderLinkId<br>- **Futures**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order does not change the parentOrderLinkId.<br>- **Options**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order will change the parentOrderLinkId.<br>- **Futures & Options**: if TP/SL is set via [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) for a position that originally has no attached TP/SL, the parentOrderLinkId is meaningless. |
| \> blockTradeId | string | Paradigm block trade ID |
| \> symbol | string | Symbol name |
| \> price | string | Order price |
| \> qty | string | Order qty |
| \> side | string | Side. `Buy`,`Sell` |
| \> isLeverage | string | Whether to borrow `0`: false, `1`: true |
| \> [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | integer | Position index. Used to identify positions in different position modes. |
| \> [orderStatus](https://bybit-exchange.github.io/docs/v5/enum#orderstatus) | string | Order status |
| \> [createType](https://bybit-exchange.github.io/docs/v5/enum#createtype) | string | Order create type <br>- Spot does not have this key |
| \> [cancelType](https://bybit-exchange.github.io/docs/v5/enum#canceltype) | string | Cancel type |
| \> [rejectReason](https://bybit-exchange.github.io/docs/v5/enum#rejectreason) | string | Reject reason |
| \> avgPrice | string | Average filled price, returns `""` for those orders without avg price |
| \> leavesQty | string | The remaining qty not executed |
| \> leavesValue | string | The estimated value not executed |
| \> cumExecQty | string | Cumulative executed order qty |
| \> cumExecValue | string | Cumulative executed order value |
| \> cumExecFee | string | - `inverse`, `option`: Cumulative executed trading fee.<br>- `linear`, `spot`: Deprecated. Use `cumFeeDetail` instead. |
| \> [timeInForce](https://bybit-exchange.github.io/docs/v5/enum#timeinforce) | string | Time in force |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit`. For TP/SL orders, is the order type after the order was triggered |
| \> [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | string | Stop order type |
| \> orderIv | string | Implied volatility |
| \> marketUnit | string | The unit for `qty` when create **Spot market** orders. `baseCoin`, `quoteCoin` |
| \> triggerPrice | string | Trigger price. If `stopOrderType`= _TrailingStop_, it is activate price. Otherwise, it is trigger price |
| \> takeProfit | string | Take profit price |
| \> stopLoss | string | Stop loss price |
| \> tpslMode | string | TP/SL mode, `Full`: entire position for TP/SL. `Partial`: partial position tp/sl. Spot does not have this field, and Option returns always "" |
| \> ocoTriggerBy | string | The trigger type of Spot OCO order.`OcoTriggerByUnknown`, `OcoTriggerByTp`, `OcoTriggerByBySl` |
| \> tpLimitPrice | string | The limit order price when take profit price is triggered |
| \> slLimitPrice | string | The limit order price when stop loss price is triggered |
| \> [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger take profit |
| \> [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger stop loss |
| \> triggerDirection | integer | Trigger direction. `1`: rise, `2`: fall |
| \> [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type of trigger price |
| \> lastPriceOnCreated | string | Last price when place the order, Spot is not applicable |
| \> basePrice | string | Last price when place the order, Spot has this field only |
| \> reduceOnly | boolean | Reduce only. `true` means reduce position size |
| \> closeOnTrigger | boolean | Close on trigger. [What is a close on trigger order?](https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order) |
| \> placeType | string | Place type, `option` used. `iv`, `price` |
| \> [smpType](https://bybit-exchange.github.io/docs/v5/enum#smptype) | string | SMP execution type |
| \> smpGroup | integer | Smp group ID. If the UID has no group, it is `0` by default |
| \> smpOrderId | string | The counterparty's orderID which triggers this SMP execution |
| \> createdTime | string | Order created timestamp (ms) |
| \> updatedTime | string | Order updated timestamp (ms) |
| \> cumFeeDetail | json | - `linear`, `spot`: Cumulative trading fee details instead of `cumExecFee` |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/open-order)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/order/realtime?symbol=ETHUSDT&category=linear&openOnly=0&limit=1  HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672219525810
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_open_orders(
    category="linear",
    symbol="ETHUSDT",
    openOnly=0,
    limit=1,
))
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.trade.request.TradeOrderRequest;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newTradeRestClient();
var openLinearOrdersResult = client.getOpenOrders(openOrderRequest.category(CategoryType.LINEAR).openOnly(1).build());
System.out.println(openLinearOrdersResult);
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getActiveOrders({
        category: 'linear',
        symbol: 'ETHUSDT',
        openOnly: 0,
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
        "list": [
            {
                "orderId": "fd4300ae-7847-404e-b947-b46980a4d140",
                "orderLinkId": "test-000005",
                "blockTradeId": "",
                "symbol": "ETHUSDT",
                "price": "1600.00",
                "qty": "0.10",
                "side": "Buy",
                "isLeverage": "",
                "positionIdx": 1,
                "orderStatus": "New",
                "cancelType": "UNKNOWN",
                "rejectReason": "EC_NoError",
                "avgPrice": "0",
                "leavesQty": "0.10",
                "leavesValue": "160",
                "cumExecQty": "0.00",
                "cumExecValue": "0",
                "cumExecFee": "0",
                "timeInForce": "GTC",
                "orderType": "Limit",
                "stopOrderType": "UNKNOWN",
                "orderIv": "",
                "triggerPrice": "0.00",
                "takeProfit": "2500.00",
                "stopLoss": "1500.00",
                "tpTriggerBy": "LastPrice",
                "slTriggerBy": "LastPrice",
                "triggerDirection": 0,
                "triggerBy": "UNKNOWN",
                "lastPriceOnCreated": "",
                "reduceOnly": false,
                "closeOnTrigger": false,
                "smpType": "None",
                "smpGroup": 0,
                "smpOrderId": "",
                "tpslMode": "Full",
                "tpLimitPrice": "",
                "slLimitPrice": "",
                "placeType": "",
                "createdTime": "1684738540559",
                "updatedTime": "1684738540561",
                "cumFeeDetail": {
                    "MNT": "0.00242968"
                }
            }
        ],
        "nextPageCursor": "page_args%3Dfd4300ae-7847-404e-b947-b46980a4d140%26symbol%3D6%26",
        "category": "linear"
    },
    "retExtInfo": {},
    "time": 1684765770483
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Order History

Query order history. As order creation/cancellation is **asynchronous**, the data returned from this endpoint may delay. If you want to get
real-time order information, you could query this [endpoint](https://bybit-exchange.github.io/docs/v5/order/open-order) or rely on the [websocket stream](https://bybit-exchange.github.io/docs/v5/websocket/private/order) (recommended).

rule

- The orders in the **last 7 days**:

support querying all [closed status](https://bybit-exchange.github.io/docs/v5/enum#orderstatus) except "Cancelled", "Rejected", "Deactivated" status
- The orders in the **last 24 hours**:

the orders with "Cancelled" (fully cancelled order), "Rejected", "Deactivated" can be query
- The orders **beyond 7 days**:

supports querying orders which have fills only, i.e., fully filled, partial filled but cancelled orders

### HTTP Request

GET`/v5/order/history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `linear`, `inverse`, `spot`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| baseCoin | false | string | Base coin, uppercase only |
| settleCoin | false | string | Settle coin, uppercase only |
| orderId | false | string | Order ID |
| orderLinkId | false | string | User customised order ID |
| orderFilter | false | string | `Order`: active order<br>`StopOrder`: conditional order for Futures and Spot<br>`tpslOrder`: spot TP/SL order<br>`OcoOrder`: spot OCO orders<br>`BidirectionalTpslOrder`: Spot bidirectional TPSL order <br>- all kinds of orders are returned by default |
| [orderStatus](https://bybit-exchange.github.io/docs/v5/enum#orderstatus) | false | string | Order status |
| startTime | false | integer | The start timestamp (ms)<br>- startTime and endTime are not passed, return 7 days by default<br>- Only startTime is passed, return range between startTime and startTime+7 days<br>- Only endTime is passed, return range between endTime-7 days and endTime<br>- If both are passed, the rule is endTime - startTime <= 7 days |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `50`\]. Default: `20` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type |
| list | array | Object |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customised order ID |
| \> parentOrderLinkId | string | Indicates the linked parent order for attached take-profit and stop-loss orders. Supported for futures and options.<br>- [Amending](https://bybit-exchange.github.io/docs/v5/order/amend-order) take-profit or stop-loss orders does not change the parentOrderLinkId<br>- **Futures**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order does not change the parentOrderLinkId.<br>- **Options**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order will change the parentOrderLinkId.<br>- **Futures & Options**: if TP/SL is set via [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) for a position that originally has no attached TP/SL, the parentOrderLinkId is meaningless. |
| \> blockTradeId | string | Block trade ID |
| \> symbol | string | Symbol name |
| \> price | string | Order price |
| \> qty | string | Order qty |
| \> side | string | Side. `Buy`,`Sell` |
| \> isLeverage | string | Whether to borrow. `0`: false, `1`: true. |
| \> [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | integer | Position index. Used to identify positions in different position modes |
| \> [orderStatus](https://bybit-exchange.github.io/docs/v5/enum#orderstatus) | string | Order status |
| \> [createType](https://bybit-exchange.github.io/docs/v5/enum#createtype) | string | Order create type. Spot does not have this key |
| \> [cancelType](https://bybit-exchange.github.io/docs/v5/enum#canceltype) | string | Cancel type |
| \> [rejectReason](https://bybit-exchange.github.io/docs/v5/enum#rejectreason) | string | Reject reason |
| \> avgPrice | string | Average filled price, returns `""` for those orders without avg price |
| \> leavesQty | string | The remaining qty not executed |
| \> leavesValue | string | The estimated value not executed |
| \> cumExecQty | string | Cumulative executed order qty |
| \> cumExecValue | string | Cumulative executed order value |
| \> cumExecFee | string | - `inverse`, `option`: Cumulative executed trading fee.<br>- `linear`, `spot`: Deprecated. Use `cumFeeDetail` instead. |
| \> [timeInForce](https://bybit-exchange.github.io/docs/v5/enum#timeinforce) | string | Time in force |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit`. For TP/SL orders, is the order type after the order was triggered <br>- `Block trade Roll Back`, `Block trade-Limit`: Unique enum values for Unified account block trades |
| \> [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | string | Stop order type |
| \> orderIv | string | Implied volatility |
| \> marketUnit | string | The unit for `qty` when create **Spot market** orders. `baseCoin`, `quoteCoin` |
| \> slippageToleranceType | string | Spot and Futures market order slippage tolerance type `TickSize`, `Percent`, `UNKNOWN`(default) |
| \> slippageTolerance | string | Slippage tolerance value |
| \> triggerPrice | string | Trigger price. If `stopOrderType`= _TrailingStop_, it is activate price. Otherwise, it is trigger price |
| \> takeProfit | string | Take profit price |
| \> stopLoss | string | Stop loss price |
| \> tpslMode | string | TP/SL mode, `Full`: entire position for TP/SL. `Partial`: partial position tp/sl. Spot does not have this field, and Option returns always "" |
| \> ocoTriggerBy | string | The trigger type of Spot OCO order.`OcoTriggerByUnknown`, `OcoTriggerByTp`, `OcoTriggerBySl` |
| \> tpLimitPrice | string | The limit order price when take profit price is triggered |
| \> slLimitPrice | string | The limit order price when stop loss price is triggered |
| \> [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger take profit |
| \> [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger stop loss |
| \> triggerDirection | integer | Trigger direction. `1`: rise, `2`: fall |
| \> [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type of trigger price |
| \> lastPriceOnCreated | string | Last price when place the order, Spot is not applicable |
| \> basePrice | string | Last price when place the order, Spot has this field only |
| \> reduceOnly | boolean | Reduce only. `true` means reduce position size |
| \> closeOnTrigger | boolean | Close on trigger. [What is a close on trigger order?](https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order) |
| \> placeType | string | Place type, `option` used. `iv`, `price` |
| \> [smpType](https://bybit-exchange.github.io/docs/v5/enum#smptype) | string | SMP execution type |
| \> smpGroup | integer | Smp group ID. If the UID has no group, it is `0` by default |
| \> smpOrderId | string | The counterparty's orderID which triggers this SMP execution |
| \> createdTime | string | Order created timestamp (ms) |
| \> updatedTime | string | Order updated timestamp (ms) |
| \> extraFees | string | Trading fee rate information. Currently, this data is returned only for spot orders placed on the Indonesian site or spot fiat currency orders placed on the EU site. In other cases, an empty string is returned. Enum: [feeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeesfeetype), [subFeeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeessubfeetype) |
| \> cumFeeDetail | json | - `linear`, `spot`: Cumulative trading fee details instead of `cumExecFee` |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/order-list)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/order/history?category=linear&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672221263407
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_order_history(
    category="linear",
    limit=1,
))
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.trade.request.TradeOrderRequest;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newTradeRestClient();
var orderHistory = TradeOrderRequest.builder().category(CategoryType.LINEAR).limit(10).build();
System.out.println(client.getOrderHistory(orderHistory));
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getHistoricOrders({
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
        "list": [
            {
                "orderId": "14bad3a1-6454-43d8-bcf2-5345896cf74d",
                "orderLinkId": "YLxaWKMiHU",
                "blockTradeId": "",
                "symbol": "BTCUSDT",
                "price": "26864.40",
                "qty": "0.003",
                "side": "Buy",
                "isLeverage": "",
                "positionIdx": 1,
                "orderStatus": "Cancelled",
                "cancelType": "UNKNOWN",
                "rejectReason": "EC_PostOnlyWillTakeLiquidity",
                "avgPrice": "0",
                "leavesQty": "0.000",
                "leavesValue": "0",
                "cumExecQty": "0.000",
                "cumExecValue": "0",
                "cumExecFee": "0",
                "timeInForce": "PostOnly",
                "orderType": "Limit",
                "stopOrderType": "UNKNOWN",
                "orderIv": "",
                "triggerPrice": "0.00",
                "takeProfit": "0.00",
                "stopLoss": "0.00",
                "tpTriggerBy": "UNKNOWN",
                "slTriggerBy": "UNKNOWN",
                "triggerDirection": 0,
                "triggerBy": "UNKNOWN",
                "lastPriceOnCreated": "0.00",
                "reduceOnly": false,
                "closeOnTrigger": false,
                "smpType": "None",
                "smpGroup": 0,
                "smpOrderId": "",
                "tpslMode": "",
                "tpLimitPrice": "",
                "slLimitPrice": "",
                "placeType": "",
                "slippageToleranceType": "UNKNOWN",
                "slippageTolerance": "",
                "createdTime": "1684476068369",
                "updatedTime": "1684476068372",
                "extraFees": "",
                "cumFeeDetail": {
                    "MNT": "0.00242968"
                }
            }
        ],
        "nextPageCursor": "page_token%3D39380%26",
        "category": "linear"
    },
    "retExtInfo": {},
    "time": 1684766282976
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Pre Check Order

This endpoint is used to calculate the changes in IMR and MMR of UTA account before and after placing an order.

info

1. This endpoint supports orders with category = `inverse`,`linear`,`option`.

2. Only Cross Margin mode and Portfolio Margin mode are supported, isolated margin mode is not supported.

3. category = `inverse` is not supported in Cross Margin mode.

4. Conditional order is not supported.

5. If `retCode` is neither 0 nor 110007, `result` will return an empty json. `future_order_id`, `future_order_link_id` will be displayed in the `retExtInfo` json.
6. If `retCode` is 110007, `result` will return an empty json. `future_order_id`, `future_order_link_id`, `post_imr_e4`, and `post_mmr_e4` will be displayed in the `retExtInfo` json.

### HTTP Request

POST`/v5/order/pre-check`Copy

### Request Parameters

refer to [create order request](https://bybit-exchange.github.io/docs/v5/order/create-order#request-parameters)

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Order ID |
| orderLinkId | string | User customised order ID |
| preImrE4 | int | Initial margin rate before checking, keep four decimal places. For examples, 30 means IMR = 30/1e4 = 0.30% |
| preMmrE4 | int | Maintenance margin rate before checking, keep four decimal places. For examples, 30 means MMR = 30/1e4 = 0.30% |
| postImrE4 | int | Initial margin rate calculated after checking, keep four decimal places. For examples, 30 means IMR = 30/1e4 = 0.30% |
| postMmrE4 | int | Maintenance margin rate calculated after checking, keep four decimal places. For examples, 30 means MMR = 30/1e4 = 0.30% |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/order/pre-check HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672211928338
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

// Spot Limit order with market tp sl
{"category": "spot","symbol": "BTCUSDT","side": "Buy","orderType": "Limit","qty": "0.01","price": "28000","timeInForce": "PostOnly","takeProfit": "35000","stopLoss": "27000","tpOrderType": "Market","slOrderType": "Market"}

// Spot Limit order with limit tp sl
{"category": "spot","symbol": "BTCUSDT","side": "Buy","orderType": "Limit","qty": "0.01","price": "28000","timeInForce": "PostOnly","takeProfit": "35000","stopLoss": "27000","tpLimitPrice": "36000","slLimitPrice": "27500","tpOrderType": "Limit","slOrderType": "Limit"}

// Spot PostOnly normal order
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","timeInForce":"PostOnly","orderLinkId":"spot-test-01","isLeverage":0,"orderFilter":"Order"}

// Spot TP/SL order
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","triggerPrice": "15000", "timeInForce":"Limit","orderLinkId":"spot-test-02","isLeverage":0,"orderFilter":"tpslOrder"}

// Spot margin normal order (UTA)
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"0.1","price":"15600","timeInForce":"GTC","orderLinkId":"spot-test-limit","isLeverage":1,"orderFilter":"Order"}

// Spot Market Buy order, qty is quote currency
{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Market","qty":"200","timeInForce":"IOC","orderLinkId":"spot-test-04","isLeverage":0,"orderFilter":"Order"}

// USDT Perp open long position (one-way mode)
{"category":"linear","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","qty":"1","price":"25000","timeInForce":"GTC","positionIdx":0,"orderLinkId":"usdt-test-01","reduceOnly":false,"takeProfit":"28000","stopLoss":"20000","tpslMode":"Partial","tpOrderType":"Limit","slOrderType":"Limit","tpLimitPrice":"27500","slLimitPrice":"20500"}

// USDT Perp close long position (one-way mode)
{"category": "linear", "symbol": "BTCUSDT", "side": "Sell", "orderType": "Limit", "qty": "1", "price": "30000", "timeInForce": "GTC", "positionIdx": 0, "orderLinkId": "usdt-test-02", "reduceOnly": true}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.pre_check_order(
    category="spot",
    symbol="BTCUSDT",
    side="Buy",
    orderType="Limit",
    qty="0.1",
    price="28000",
    timeInForce="PostOnly",
    takeProfit="35000",
    stopLoss="27000",
    tpOrderType="Market",
    slOrderType="Market",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "orderId": "24920bdb-4019-4e37-ad1c-876e3a855ac3",
        "orderLinkId": "test129",
        "preImrE4": 30,
        "preMmrE4": 21,
        "postImrE4": 357,
        "postMmrE4": 294
    },
    "retExtInfo": {},
    "time": 1749541599589
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrow Quota (Spot)

Query the available balance for Spot trading and Margin trading

info

- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/order/spot-borrow-check`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type `spot` |
| symbol | **true** | string | Symbol name |
| side | **true** | string | Transaction side. `Buy`,`Sell` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| symbol | string | Symbol name, like `BTCUSDT`, uppercase only |
| side | string | Side |
| maxTradeQty | string | The maximum base coin qty can be traded<br>- If spot margin trade on and symbol is margin trading pair, it returns available balance + max.borrowable quantity = min(The maximum quantity that a single user can borrow on the platform, The maximum quantity that can be borrowed calculated by IMR MMR of UTA account, The available quantity of the platform's capital pool) <br>- Otherwise, it returns actual available balance<br>- up to 4 decimals |
| maxTradeAmount | string | The maximum quote coin amount can be traded<br>- If spot margin trade on and symbol is margin trading pair, it returns available balance + max.borrowable amount = min(The maximum amount that a single user can borrow on the platform, The maximum amount that can be borrowed calculated by IMR MMR of UTA account, The available amount of the platform's capital pool) <br>- Otherwise, it returns actual available balance<br>- up to 8 decimals |
| spotMaxTradeQty | string | No matter your Spot margin switch on or not, it always returns actual qty of base coin you can trade or you have (borrowable qty is not included), up to 4 decimals |
| spotMaxTradeAmount | string | No matter your Spot margin switch on or not, it always returns actual amount of quote coin you can trade or you have (borrowable amount is not included), up to 8 decimals |
| borrowCoin | string | Borrow coin |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/trade/query-spot-quota)

* * *

### Request Example

- HTTP
- Python
- Java
- Node.js

```http
GET /v5/order/spot-borrow-check?category=spot&symbol=BTCUSDT&side=Buy HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672228522214
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrow_quota(
    category="spot",
    symbol="BTCUSDT",
    side="Buy",
))
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.trade.request.TradeOrderRequest;
import com.bybit.api.client.domain.*;
import com.bybit.api.client.domain.trade.*;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newTradeRestClient();
var getBorrowQuotaRequest = TradeOrderRequest.builder().category(CategoryType.SPOT).symbol("BTCUSDT").side(Side.BUY).build();
System.out.println(client.getBorrowQuota(getBorrowQuotaRequest));
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getSpotBorrowCheck('BTCUSDT', 'Buy')
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
        "maxTradeQty": "6.6065",
        "side": "Buy",
        "spotMaxTradeAmount": "9004.75628594",
        "maxTradeAmount": "218014.01330797",
        "borrowCoin": "USDT",
        "spotMaxTradeQty": "0.2728"
    },
    "retExtInfo": {},
    "time": 1698895841534
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

