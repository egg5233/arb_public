# Bybit WebSocket API

Source: https://bybit-exchange.github.io/docs/v5/websocket/

---

# Dcp

Subscribe to the dcp stream to trigger DCP function.

For example, connection A subscribes "dcp.xxx", connection B does not and connection C subscribes "dcp.xxx".

1. If A is alive, B is dead, C is alive, then this case will not trigger DCP.
2. If A is alive, B is dead, C is dead, then this case will not trigger DCP.
3. If A is dead, B is alive, C is dead, then DCP is triggered when reach the timeWindow threshold

To sum up, for those private connections subscribing "dcp" topic are all dead, then DCP will be triggered.

**Topic:**`dcp.future`, `dcp.spot`, `dcp.option`

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "dcp.future"
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Execution

Subscribe to the execution stream to see your executions in **real-time**.

tip

You may have multiple executions for one order in a single message.

**All-In-One Topic:**`execution`

**Categorised Topic:**`execution.spot`, `execution.linear`, `execution.inverse`, `execution.option`

info

- All-In-One topic and Categorised topic **cannot** be in the same subscription request
- All-In-One topic: Allow you to listen to all categories (spot, linear, inverse, option) websocket updates
- Categorised Topic: Allow you to listen only to specific category websocket updates

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message ID |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type `spot`, `linear`, `inverse`, `option` |
| \> symbol | string | Symbol name |
| \> isLeverage | string | Whether to borrow. `0`: false, `1`: true |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customized order ID |
| \> side | string | Side. `Buy`,`Sell` |
| \> orderPrice | string | Order price |
| \> orderQty | string | Order qty |
| \> leavesQty | string | The remaining qty not executed |
| \> [createType](https://bybit-exchange.github.io/docs/v5/enum#createtype) | string | Order create type <br>- Spot, Option do not have this key |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit` |
| \> [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | string | Stop order type. If the order is not stop order, any type is not returned |
| \> execFee | string | Executed trading fee. You can get spot fee currency instruction [here](https://bybit-exchange.github.io/docs/v5/enum#spot-fee-currency-instruction) |
| \> execId | string | Execution ID |
| \> execPrice | string | Execution price |
| \> execQty | string | Execution qty |
| \> execPnl | string | Profit and Loss for each close position execution. The value keeps consistent with the field "cashFlow" in the [Get Transaction Log](https://bybit-exchange.github.io/docs/v5/account/transaction-log) |
| \> [execType](https://bybit-exchange.github.io/docs/v5/enum#exectype) | string | Executed type |
| \> execValue | string | Executed order value |
| \> execTime | string | Executed timestamp (ms) |
| \> isMaker | boolean | Is maker order. `true`: maker, `false`: taker |
| \> feeRate | string | Trading fee rate |
| \> tradeIv | string | Implied volatility. valid for `option` |
| \> markIv | string | Implied volatility of mark price. valid for `option` |
| \> markPrice | string | The mark price of the symbol when executing. valid for `option` |
| \> indexPrice | string | The index price of the symbol when executing. valid for `option` |
| \> underlyingPrice | string | The underlying price of the symbol when executing. valid for `option` |
| \> blockTradeId | string | Paradigm block trade ID |
| \> closedSize | string | Closed position size |
| \> extraFees | List | Extra trading fee information. Currently, this data is returned only for kyc=Indian user or spot orders placed on the Indonesian site or spot fiat currency orders placed on the EU site. In other cases, an empty string is returned. Enum: [feeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeesfeetype), [subFeeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeessubfeetype) |
| \> seq | long | Cross sequence, used to associate each fill and each position update<br>- The seq will be the same when conclude multiple transactions at the same time<br>- Different symbols may have the same seq, please use seq + symbol to check unique |
| \> feeCurrency | string | Trading fee currency |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "execution"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="private",
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
def handle_message(message):
    print(message)
ws.execution_stream(callback=handle_message)
while True:
    sleep(1)
```

### Stream Example

```json
{
    "topic": "execution",
    "id": "386825804_BTCUSDT_140612148849382",
    "creationTime": 1746270400355,
    "data": [
        {
            "category": "linear",
            "symbol": "BTCUSDT",
            "closedSize": "0.5",
            "execFee": "26.3725275",
            "execId": "0ab1bdf7-4219-438b-b30a-32ec863018f7",
            "execPrice": "95900.1",
            "execQty": "0.5",
            "execType": "Trade",
            "execValue": "47950.05",
            "feeRate": "0.00055",
            "tradeIv": "",
            "markIv": "",
            "blockTradeId": "",
            "markPrice": "95901.48",
            "indexPrice": "",
            "underlyingPrice": "",
            "leavesQty": "0",
            "orderId": "9aac161b-8ed6-450d-9cab-c5cc67c21784",
            "orderLinkId": "",
            "orderPrice": "94942.5",
            "orderQty": "0.5",
            "orderType": "Market",
            "stopOrderType": "UNKNOWN",
            "side": "Sell",
            "execTime": "1746270400353",
            "isLeverage": "0",
            "isMaker": false,
            "seq": 140612148849382,
            "marketUnit": "",
            "execPnl": "0.05",
            "createType": "CreateByUser",
            "extraFees":[{"feeCoin":"USDT","feeType":"GST","subFeeType":"IND_GST","feeRate":"0.0000675","fee":"0.006403779"}],
            "feeCurrency": "USDT"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Fast Execution

Fast execution stream significantly reduces data latency compared original "execution" stream. However, it pushes limited
execution type of trades, and fewer data fields.

**All-In-One Topic:**`execution.fast`

**Categorised Topic:**`execution.fast.linear`, `execution.fast.inverse`, `execution.fast.spot`, `execution.fast.option`

info

- Supports all Perps, Futures, Spot and Options exceution
- You can only receive [execType](https://bybit-exchange.github.io/docs/v5/enum#exectype) =Trade update

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type `linear`, `inverse`, `spot`, `option` |
| \> symbol | string | Symbol name |
| \> orderId | string | Order ID |
| \> isMaker | boolean | `true`: Maker, `false`: Taker |
| \> orderLinkId | string | User customized order ID <br>- maker trade is always `""`<br>- If a maker order in the orderbook is converted to taker (by price amend), orderLinkId is also `""`<br>- For option: maker trade is always `""`, taker trade is always orderLinkId |
| \> execId | string | Execution ID |
| \> execPrice | string | Execution price |
| \> execQty | string | Execution qty |
| \> side | string | Side. `Buy`,`Sell` |
| \> execTime | string | Executed timestamp (ms) |
| \> seq | long | Cross sequence, used to associate each fill and each position update<br>- The seq will be the same when conclude multiple transactions at the same time<br>- Different symbols may have the same seq, please use seq + symbol to check unique |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "execution.fast"
    ]
}
```

### Stream Example

```json
{
    "topic": "execution.fast",
    "creationTime": 1716800399338,
    "data": [
        {
            "category": "linear",
            "symbol": "ICPUSDT",
            "execId": "3510f361-0add-5c7b-a2e7-9679810944fc",
            "execPrice": "12.015",
            "execQty": "3000",
            "orderId": "443d63fa-b4c3-4297-b7b1-23bca88b04dc",
            "isMaker": false,
            "orderLinkId": "test-00001",
            "side": "Sell",
            "execTime": "1716800399334",
            "seq": 34771365464
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Greek

Subscribe to the greeks stream to see changes to your greeks data in **real-time**. `option` only.

**Topic:**`greeks`

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message ID |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> baseCoin | string | Base coin |
| \> totalDelta | string | Delta value |
| \> totalGamma | string | Gamma value |
| \> totalVega | string | Vega value |
| \> totalTheta | string | Theta value |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "greeks"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="private",
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
def handle_message(message):
    print(message)
ws.greek_stream(callback=handle_message)
while True:
    sleep(1)
```

### Stream Example

```json
{
    "id": "592324fa945a30-2603-49a5-b865-21668c29f2a6",
    "topic": "greeks",
    "creationTime": 1672364262482,
    "data": [
        {
            "baseCoin": "ETH",
            "totalDelta": "0.06999986",
            "totalGamma": "-0.00000001",
            "totalVega": "-0.00000024",
            "totalTheta": "0.00001314"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Order

Subscribe to the order stream to see changes to your orders in **real-time**.

**All-In-One Topic:**`order`

**Categorised Topic:**`order.spot`, `order.linear`, `order.inverse`, `order.option`

info

- All-In-One topic and Categorised topic **cannot** be in the same subscription request
- All-In-One topic: Allow you to listen to all categories (spot, linear, inverse, option) websocket updates
- Categorised Topic: Allow you to listen only to specific category websocket updates

tip

You may receive two orderStatus=`Filled` messages when the cancel request is accepted but the order is executed at the same time. Generally, one
message contains "orderStatus=Filled, rejectReason=EC\_NoError", and another message contains "orderStatus=Filled, cancelType=CancelByUser, rejectReason=EC\_OrigClOrdIDDoesNotExist".
The first message tells you the order is executed, and the second message tells you the followed cancel request is rejected due to order is executed.

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message ID |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> category | string | Product type `spot`, `linear`, `inverse`, `option` |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customised order ID |
| \> parentOrderLinkId | string | Indicates the linked parent order for attached take-profit and stop-loss orders. Supported for futures and options.<br>- [Amending](https://bybit-exchange.github.io/docs/v5/order/amend-order) take-profit or stop-loss orders does not change the parentOrderLinkId<br>- **Futures**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order does not change the parentOrderLinkId.<br>- **Options**: using [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) to update attached TP/SL from the original order will change the parentOrderLinkId.<br>- **Futures & Options**: if TP/SL is set via [set trading stop](https://bybit-exchange.github.io/docs/v5/position/trading-stop) for a position that originally has no attached TP/SL, the parentOrderLinkId is meaningless. |
| \> isLeverage | string | Whether to borrow. `0`: false, `1`: true |
| \> blockTradeId | string | Block trade ID |
| \> symbol | string | Symbol name |
| \> price | string | Order price |
| \> brokerOrderPrice | string | Dedicated field for EU liquidity provider |
| \> qty | string | Order qty |
| \> side | string | Side. `Buy`,`Sell` |
| \> [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | integer | Position index. Used to identify positions in different position modes |
| \> [orderStatus](https://bybit-exchange.github.io/docs/v5/enum#orderstatus) | string | Order status |
| \> [createType](https://bybit-exchange.github.io/docs/v5/enum#createtype) | string | Order create type, Spot, Option do not have this key |
| \> [cancelType](https://bybit-exchange.github.io/docs/v5/enum#canceltype) | string | Cancel type |
| \> [rejectReason](https://bybit-exchange.github.io/docs/v5/enum#rejectreason) | string | Reject reason |
| \> avgPrice | string | Average filled price, returns `""` for those orders without avg price |
| \> leavesQty | string | The remaining qty not executed |
| \> leavesValue | string | The remaining value not executed |
| \> cumExecQty | string | Cumulative executed order qty |
| \> cumExecValue | string | Cumulative executed order value |
| \> cumExecFee | string | - `inverse`, `option`: Cumulative executed trading fee.<br>- `linear`, `spot`: Deprecated. Use `cumFeeDetail` instead.<br>- After upgraded to the Unified account, you can use `execFee` for each fill in [Execution](https://bybit-exchange.github.io/docs/v5/websocket/private/execution) topic |
| \> closedPnl | string | Closed profit and loss for each close position order. The figure is the same as "closedPnl" from [Get Closed PnL](https://bybit-exchange.github.io/docs/v5/position/close-pnl) |
| \> feeCurrency | string | Deprecated. Trading fee currency for Spot only. Please understand Spot trading fee currency [here](https://bybit-exchange.github.io/docs/v5/enum#spot-fee-currency-instruction) |
| \> [timeInForce](https://bybit-exchange.github.io/docs/v5/enum#timeinforce) | string | Time in force |
| \> [orderType](https://bybit-exchange.github.io/docs/v5/enum#ordertype) | string | Order type. `Market`,`Limit`. For TP/SL orders, is the order type after the order was triggered |
| \> [stopOrderType](https://bybit-exchange.github.io/docs/v5/enum#stopordertype) | string | Stop order type |
| \> ocoTriggerBy | string | The trigger type of Spot OCO order.`OcoTriggerByUnknown`, `OcoTriggerByTp`, `OcoTriggerBySl` |
| \> orderIv | string | Implied volatility |
| \> marketUnit | string | The unit for `qty` when create **Spot market** orders. `baseCoin`, `quoteCoin` |
| \> slippageToleranceType | string | Spot and Futures market order slippage tolerance type `TickSize`, `Percent`, `UNKNOWN`(default) |
| \> slippageTolerance | string | Slippage tolerance value |
| \> triggerPrice | string | Trigger price. If `stopOrderType`= _TrailingStop_, it is activate price. Otherwise, it is trigger price |
| \> takeProfit | string | Take profit price |
| \> stopLoss | string | Stop loss price |
| \> tpslMode | string | TP/SL mode, `Full`: entire position for TP/SL. `Partial`: partial position tp/sl. Spot does not have this field, and Option returns always "" |
| \> tpLimitPrice | string | The limit order price when take profit price is triggered |
| \> slLimitPrice | string | The limit order price when stop loss price is triggered |
| \> [tpTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger take profit |
| \> [slTriggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type to trigger stop loss |
| \> triggerDirection | integer | Trigger direction. `1`: rise, `2`: fall |
| \> [triggerBy](https://bybit-exchange.github.io/docs/v5/enum#triggerby) | string | The price type of trigger price |
| \> lastPriceOnCreated | string | Last price when place the order |
| \> reduceOnly | boolean | Reduce only. `true` means reduce position size |
| \> closeOnTrigger | boolean | Close on trigger. [What is a close on trigger order?](https://www.bybit.com/en/help-center/article/Close-On-Trigger-Order) |
| \> placeType | string | Place type, `option` used. `iv`, `price` |
| \> [smpType](https://bybit-exchange.github.io/docs/v5/enum#smptype) | string | SMP execution type |
| \> smpGroup | integer | Smp group ID. If the UID has no group, it is `0` by default |
| \> smpOrderId | string | The counterparty's orderID which triggers this SMP execution |
| \> createdTime | string | Order created timestamp (ms) |
| \> updatedTime | string | Order updated timestamp (ms) |
| \> cumFeeDetail | json | - `linear`, `spot`: Cumulative trading fee details instead of `cumExecFee` |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "order"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="private",
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
def handle_message(message):
    print(message)
ws.order_stream(callback=handle_message)
while True:
    sleep(1)
```

### Stream Example

```json
{
    "id": "5923240c6880ab-c59f-420b-9adb-3639adc9dd90",
    "topic": "order",
    "creationTime": 1672364262474,
    "data": [
        {
            "symbol": "ETH-30DEC22-1400-C",
            "orderId": "5cf98598-39a7-459e-97bf-76ca765ee020",
            "side": "Sell",
            "orderType": "Market",
            "cancelType": "UNKNOWN",
            "price": "72.5",
            "qty": "1",
            "orderIv": "",
            "timeInForce": "IOC",
            "orderStatus": "Filled",
            "orderLinkId": "",
            "lastPriceOnCreated": "",
            "reduceOnly": false,
            "leavesQty": "",
            "leavesValue": "",
            "cumExecQty": "1",
            "cumExecValue": "75",
            "avgPrice": "75",
            "blockTradeId": "",
            "positionIdx": 0,
            "cumExecFee": "0.358635",
            "closedPnl": "0",
            "createdTime": "1672364262444",
            "updatedTime": "1672364262457",
            "rejectReason": "EC_NoError",
            "stopOrderType": "",
            "tpslMode": "",
            "triggerPrice": "",
            "takeProfit": "",
            "stopLoss": "",
            "tpTriggerBy": "",
            "slTriggerBy": "",
            "tpLimitPrice": "",
            "slLimitPrice": "",
            "triggerDirection": 0,
            "triggerBy": "",
            "closeOnTrigger": false,
            "category": "option",
            "placeType": "price",
            "smpType": "None",
            "smpGroup": 0,
            "smpOrderId": "",
            "feeCurrency": "",
            "cumFeeDetail": {
                "MNT": "0.00242968"
            }
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Position

Subscribe to the position stream to see changes to your position data in **real-time**.

**All-In-One Topic:**`position`

**Categorised Topic:**`position.linear`, `position.inverse`, `position.option`

info

- All-In-One topic and Categorised topic **cannot** be in the same subscription request
- All-In-One topic: Allow you to listen to all categories (linear, inverse, option) websocket updates
- Categorised Topic: Allow you to listen only to specific category websocket updates

tip

Every time when you create/amend/cancel an order, the position topic will generate a new message (regardless if there's any actual change)

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message ID |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> [category](https://bybit-exchange.github.io/docs/v5/enum#category) | string | Product type `linear`, `inverse`, `option` |
| \> symbol | string | Symbol name |
| \> side | string | Position side. `Buy`: long, `Sell`: short<br>return an empty string `""` for an empty position |
| \> size | string | Position size |
| \> [positionIdx](https://bybit-exchange.github.io/docs/v5/enum#positionidx) | integer | Used to identify positions in different position modes |
| \> positionValue | string | Position value |
| \> riskId | integer | Risk tier ID<br>_for portfolio margin mode, this field returns 0, which means risk limit rules are invalid_ |
| \> riskLimitValue | string | Risk limit value, become meaningless when auto risk-limit tier is applied<br>_for portfolio margin mode, this field returns 0, which means risk limit rules are invalid_ |
| \> entryPrice | string | Average entry price <br>- For USDC Perp & Futures, it indicates average entry price, and it will not be changed with 8-hour session settlement |
| \> markPrice | string | Mark price |
| \> leverage | string | Position leverage<br>_for portfolio margin mode, this field returns "", which means leverage rules are invalid_ |
| \> breakEvenPrice | string | Break even price, only for `linear`,`inverse`. <br>- breakeven\_price = (entry\_price _qty - realized\_pnl) / (qty - abs(qty)_ max(taker fee rate, 0.00055)) |
| \> autoAddMargin | integer | Whether to add margin automatically when using isolated margin mode <br>- `0`: false<br>- `1`: true |
| \> positionIM | string | Initial margin, the same value as `positionIMByMp`, please note this change [The New Margin Calculation: Adjustments and Implications](https://www.bybit.com/en/help-center/article/Understanding-the-Adjustment-and-Impact-of-the-New-Margin-Calculation) <br>- Portfolio margin mode: returns "" |
| \> positionMM | string | Maintenance margin, the same value as `positionMMByMp`<br>- Portfolio margin mode: returns "" |
| \> liqPrice | string | Position liquidation price <br>- Isolated margin: <br>  <br>  it is the real price for isolated and cross positions, and keeps `""` when liqPrice <= minPrice or liqPrice >= maxPrice<br>- Cross margin:<br>  <br>  it is an **estimated** price for cross positions(because the unified mode controls the risk rate according to the account), and keeps `""` when liqPrice <= minPrice or liqPrice >= maxPrice<br> _this field is empty for Portfolio Margin Mode, and no liquidation price will be provided_ |
| \> takeProfit | string | Take profit price |
| \> stopLoss | string | Stop loss price |
| \> trailingStop | string | Trailing stop |
| \> unrealisedPnl | string | Unrealised profit and loss |
| \> curRealisedPnl | string | The realised PnL for the current holding position |
| \> sessionAvgPrice | string | USDC contract session avg price, it is the same figure as avg entry price shown in the web UI |
| \> delta | string | Delta. It is only pushed when you subscribe to the option position. |
| \> gamma | string | Gamma. It is only pushed when you subscribe to the option position. |
| \> vega | string | Vega. It is only pushed when you subscribe to the option position. |
| \> theta | string | Theta. It is only pushed when you subscribe to the option position. |
| \> cumRealisedPnl | string | Cumulative realised pnl <br>- Futures & Perp: it is the all time cumulative realised P&L<br>- Option: it is the realised P&L when you hold that position |
| \> [positionStatus](https://bybit-exchange.github.io/docs/v5/enum#positionstatus) | string | Position status. `Normal`, `Liq`, `Adl` |
| \> [adlRankIndicator](https://bybit-exchange.github.io/docs/v5/enum#adlrankindicator) | integer | Auto-deleverage rank indicator. [What is Auto-Deleveraging?](https://www.bybit.com/en-US/help-center/s/article/What-is-Auto-Deleveraging-ADL) |
| \> isReduceOnly | boolean | Useful when Bybit lower the risk limit <br>- `true`: Only allowed to reduce the position. You can consider a series of measures, e.g., lower the risk limit, decrease leverage or reduce the position, add margin, or cancel orders, after these operations, you can call [confirm new risk limit](https://bybit-exchange.github.io/docs/v5/position/confirm-mmr) endpoint to check if your position can be removed the reduceOnly mark<br>- `false`: There is no restriction, and it means your position is under the risk when the risk limit is systematically adjusted<br>- Only meaningful for isolated margin & cross margin of USDT Perp, USDC Perp, USDC Futures, Inverse Perp and Inverse Futures, meaningless for others |
| \> createdTime | string | Timestamp of the first time a position was created on this symbol (ms) |
| \> updatedTime | string | Position data updated timestamp (ms) |
| \> seq | long | Cross sequence, used to associate each fill and each position update<br>- Different symbols may have the same seq, please use seq + symbol to check unique<br>- Returns `"-1"` if the symbol has never been traded<br>- Returns the seq updated by the last transaction when there are setting like leverage, risk limit |
| \> mmrSysUpdatedTime | string | Useful when Bybit lower the risk limit <br>- When isReduceOnly=`true`: the timestamp (ms) when the MMR will be forcibly adjusted by the system<br>When isReduceOnly=`false`: the timestamp when the MMR had been adjusted by system<br>  - It returns the timestamp when the system operates, and if you manually operate, there is no timestamp<br>  - Keeps `""` by default, if there was a lower risk limit system adjustment previously, it shows that system operation timestamp<br>  - Only meaningful for isolated margin & cross margin of USDT Perp, USDC Perp, USDC Futures, Inverse Perp and Inverse Futures, meaningless for others |
| \> leverageSysUpdatedTime | string | Useful when Bybit lower the risk limit <br>- When isReduceOnly=`true`: the timestamp (ms) when the leverage will be forcibly adjusted by the system<br>When isReduceOnly=`false`: the timestamp when the leverage had been adjusted by system<br>  - It returns the timestamp when the system operates, and if you manually operate, there is no timestamp<br>  - Keeps `""` by default, if there was a lower risk limit system adjustment previously, it shows that system operation timestamp<br>  - Only meaningful for isolated margin & cross margin of USDT Perp, USDC Perp, USDC Futures, Inverse Perp and Inverse Futures, meaningless for others |
| \> positionIMByMp | string | Initial margin calculated by mark price, the same value as `positionIM`<br>- Portfolio margin mode: returns "" |
| \> positionMMByMp | string | Maintenance margin calculated by mark price, the same value as `positionMM`<br>- Portfolio margin mode: returns "" |
| \> tpslMode | string | **Deprecated**, always "Full" |
| \> bustPrice | string | **Deprecated**, always `""` |
| \> positionBalance | string | **Deprecated**, can refer to `positionIM` or `positionIMByMp` field |
| \> tradeMode | integer | **Deprecated**, always `0`, check [Get Account Info](https://bybit-exchange.github.io/docs/v5/account/account-info) to know the margin mode |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "position"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="private",
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
def handle_message(message):
    print(message)
ws.position_stream(callback=handle_message)
while True:
    sleep(1)
```

### Stream Example

```json
{
    "id": "1003076014fb7eedb-c7e6-45d6-a8c1-270f0169171a",
    "topic": "position",
    "creationTime": 1697682317044,
    "data": [
        {
            "positionIdx": 2,
            "tradeMode": 0,
            "riskId": 1,
            "riskLimitValue": "2000000",
            "symbol": "BTCUSDT",
            "side": "",
            "size": "0",
            "entryPrice": "0",
            "leverage": "10",
            "breakEvenPrice":"93556.73034991",
            "positionValue": "0",
            "positionBalance": "0",
            "markPrice": "28184.5",
            "positionIM": "0",
            "positionIMByMp": "0",
            "positionMM": "0",
            "positionMMByMp": "0",
            "takeProfit": "0",
            "stopLoss": "0",
            "trailingStop": "0",
            "unrealisedPnl": "0",
            "curRealisedPnl": "1.26",
            "cumRealisedPnl": "-25.06579337",
            "sessionAvgPrice": "0",
            "createdTime": "1694402496913",
            "updatedTime": "1697682317038",
            "tpslMode": "Full",
            "liqPrice": "0",
            "bustPrice": "",
            "category": "linear",
            "positionStatus": "Normal",
            "adlRankIndicator": 0,
            "autoAddMargin": 0,
            "leverageSysUpdatedTime": "",
            "mmrSysUpdatedTime": "",
            "seq": 8327597863,
            "isReduceOnly": false
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Wallet

Subscribe to the wallet stream to see changes to your wallet in **real-time**.

info

- There is no snapshot event given at the time when the subscription is successful
- The unrealised PnL change does not trigger an event
- Under the new logic of UTA manual borrow, `spotBorrow` field corresponding to spot liabilities is detailed in the [announcement](https://announcements.bybit.com/en/article/bybit-uta-function-optimization-manual-coin-borrowing-will-be-launched-soon-blt5d858199bd12e849/).

Old `walletBalance` = New `walletBalance` \- `spotBorrow`

**Topic:**`wallet`

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message ID |
| topic | string | Topic name |
| creationTime | number | Data created timestamp (ms) |
| data | array | Object |
| \> accountType | string | Account type `UNIFIED` |
| \> accountIMRate | string | Account IM rate <br>- You can refer to this [Glossary](https://www.bybit.com/en/help-center/article/Glossary-Unified-Trading-Account) to understand the below fields calculation and mearning<br>- All account wide fields are **not** applicable to isolated margin |
| \> accountMMRate | string | Account MM rate |
| \> totalEquity | string | Account total equity (USD): ∑Asset Equity By USD value of each asset |
| \> totalWalletBalance | string | Account wallet balance (USD): ∑Asset Wallet Balance By USD value of each asset |
| \> totalMarginBalance | string | Account margin balance (USD): totalWalletBalance + totalPerpUPL |
| \> totalAvailableBalance | string | Account available balance (USD), <br>- Cross Margin: totalMarginBalance - Haircut - totalInitialMargin.<br>- Porfolio Margin: total Equity - Haircut - totalInitialMargin |
| \> totalPerpUPL | string | Account Perps and Futures unrealised p&l (USD): ∑Each Perp and USDC Futures upl by base coin |
| \> totalInitialMargin | string | Account initial margin (USD): ∑Asset Total Initial Margin Base Coin |
| \> totalMaintenanceMargin | string | Account maintenance margin (USD): ∑ Asset Total Maintenance Margin Base Coin |
| \> accountIMRateByMp | string | You can **ignore** this field, and refer to `accountIMRate`, which has the same calculation |
| \> accountMMRateByMp | string | You can **ignore** this field, and refer to `accountMMRate`, which has the same calculation |
| \> totalInitialMarginByMp | string | You can **ignore** this field, and refer to `totalInitialMargin`, which has the same calculation |
| \> totalMaintenanceMarginByMp | string | You can **ignore** this field, and refer to `totalMaintenanceMargin`, which has the same calculation |
| \> accountLTV | string | **Deprecated** field |
| \> coin | array | Object |
| >\> coin | string | Coin name, such as BTC, ETH, USDT, USDC |
| >\> equity | string | Equity of coin. Asset Equity = Asset Wallet Balance + Asset Perp UPL + Asset Future UPL + Asset Option Value = `walletBalance` \- `spotBorrow` \+ `unrealisedPnl` \+ Asset Option Value |
| >\> usdValue | string | USD value of coin. If this coin cannot be collateral, then it is 0 |
| >\> walletBalance | string | Wallet balance of coin |
| >\> locked | string | Locked balance due to the Spot open order |
| >\> spotHedgingQty | string | The spot asset qty that is used to hedge in the portfolio margin, truncate to 8 decimals and "0" by default |
| >\> borrowAmount | string | Borrow amount of coin = spot liabilities + derivatives liabilities |
| >\> accruedInterest | string | Accrued interest |
| >\> totalOrderIM | string | Pre-occupied margin for order. For portfolio margin mode, it returns "" |
| >\> totalPositionIM | string | Sum of initial margin of all positions + Pre-occupied liquidation fee. For portfolio margin mode, it returns "" |
| >\> totalPositionMM | string | Sum of maintenance margin for all positions. For portfolio margin mode, it returns "" |
| >\> unrealisedPnl | string | Unrealised P&L |
| >\> cumRealisedPnl | string | Cumulative Realised P&L |
| >\> bonus | string | Bonus |
| >\> collateralSwitch | boolean | Whether it can be used as a margin collateral currency (platform) <br>- When marginCollateral=false, then collateralSwitch is meaningless |
| >\> marginCollateral | boolean | Whether the collateral is turned on by user (user) <br>- When marginCollateral=true, then collateralSwitch is meaningful |
| >\> spotBorrow | string | Borrow amount by spot margin trade and manual borrow amount(does not include borrow amount by spot margin active order). `spotBorrow` field corresponding to spot liabilities is detailed in the [announcement](https://announcements.bybit.com/en/article/bybit-uta-function-optimization-manual-coin-borrowing-will-be-launched-soon-blt5d858199bd12e849/). |
| >\> free | string | **Deprecated** since there is no Spot wallet any more |
| >\> availableToBorrow | string | **Deprecated** field, always return `""`. Please refer to `availableToBorrow` in the [Get Collateral Info](https://bybit-exchange.github.io/docs/v5/account/collateral-info) |
| >\> availableToWithdraw | string | **Deprecated** for `accountType=UNIFIED` from 9 Jan, 2025 <br>- Transferable balance: you can use [Get Transferable Amount (Unified)](https://bybit-exchange.github.io/docs/v5/account/unified-trans-amnt) or [Get All Coins Balance](https://bybit-exchange.github.io/docs/v5/asset/balance/all-balance) instead<br>- Derivatives available balance: <br>  <br>  **isolated margin**: walletBalance - totalPositionIM - totalOrderIM - locked - bonus<br>  <br>  **cross & portfolio margin**: look at field `totalAvailableBalance`(USD), which needs to be converted into the available balance of accordingly coin through index price<br>- Spot (margin) available balance: refer to [Get Borrow Quota (Spot)](https://bybit-exchange.github.io/docs/v5/order/spot-borrow-quota) |

### Subscribe Example

```json
{
    "op": "subscribe",
    "args": [
        "wallet"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="private",
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
def handle_message(message):
    print(message)
ws.wallet_stream(callback=handle_message)
while True:
    sleep(1)
```

### Stream Example

```json
{
    "id": "592324d2bce751-ad38-48eb-8f42-4671d1fb4d4e",
    "topic": "wallet",
    "creationTime": 1700034722104,
    "data": [
        {
            "accountIMRate": "0",
            "accountIMRateByMp": "0",
            "accountMMRate": "0",
            "accountMMRateByMp": "0",
            "totalEquity": "10262.91335023",
            "totalWalletBalance": "9684.46297164",
            "totalMarginBalance": "9684.46297164",
            "totalAvailableBalance": "9556.6056555",
            "totalPerpUPL": "0",
            "totalInitialMargin": "0",
            "totalInitialMarginByMp": "0",
            "totalMaintenanceMargin": "0",
            "totalMaintenanceMarginByMp": "0",
            "coin": [
                {
                    "coin": "BTC",
                    "equity": "0.00102964",
                    "usdValue": "36.70759517",
                    "walletBalance": "0.00102964",
                    "availableToWithdraw": "0.00102964",
                    "availableToBorrow": "",
                    "borrowAmount": "0",
                    "accruedInterest": "0",
                    "totalOrderIM": "",
                    "totalPositionIM": "",
                    "totalPositionMM": "",
                    "unrealisedPnl": "0",
                    "cumRealisedPnl": "-0.00000973",
                    "bonus": "0",
                    "collateralSwitch": true,
                    "marginCollateral": true,
                    "locked": "0",
                    "spotHedgingQty": "0.01592413",
                    "spotBorrow": "0"
                }
            ],
            "accountLTV": "0",
            "accountType": "UNIFIED"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# ADL Alert

Subscribe to ADL alerts and insurance pool information.

> **Covers: USDT Perpetual / USDT Delivery / USDC Perpetual / USDC Delivery / Inverse Contracts**

Push frequency: **1s**

**Topic:**

`adlAlert.{coin}`

Available filters:

- `adlAlert.USDT` for USDT Perpetual/Delivery
- `adlAlert.USDC` for USDC Perpetual/Delivery
- `adlAlert.inverse` for Inverse contracts.

For more information on how ADL is triggered, see the [ADL endpoint](https://bybit-exchange.github.io/docs/v5/market/adl-alert).

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> c | string | Token of the insurance pool |
| \> s | string | Trading pair name |
| \> b | string | Balance of the insurance fund. Used to determine if ADL is triggered. For shared insurance pool, the "b" field will follow a T+1 refresh mechanism and will be updated daily at 00:00 UTC. |
| \> mb | string | Deprecated, always return "". Maximum balance of the insurance pool in the last 8 hours |
| \> i\_pr | string | PnL ratio threshold for triggering **contract PnL drawdown ADL** <br>- ADL is triggered when the symbol's PnL drawdown ratio in the last 8 hours exceeds this value |
| \> pr | string | Symbol's PnL drawdown ratio in the last 8 hours. Used to determine whether ADL is triggered or stopped |
| \> adl\_tt | string | Trigger threshold for **contract PnL drawdown ADL** <br>- This condition is only effective when the insurance pool balance is greater than this value; if so, an 8 hours drawdown exceeding n% may trigger ADL |
| \> adl\_sr | string | Stop ratio threshold for **contract PnL drawdown ADL** <br>- ADL stops when the symbol's 8 hours drawdown ratio falls below this value |

### Subscribe Example

```python
{"op": "subscribe", "args": ["adlAlert.USDT"]}
```

### Response Example

```json
{
  "topic": "adlAlert.USDT",
  "type": "snapshot",
  "ts": 1757736794000,
  "data": [
    {
      "c": "USDT",
      "s": "FWOGUSDT",
      "b": -5421.29889888,
      "mb": -5421.29889888,
      "i_pr": -0.3,
      "pr": 0,
      "adl_tt": 10000,
      "adl_sr": -0.25
    },
    {
      "c": "USDT",
      "s": "ZORAUSDT",
      "b": 19873.46255153,
      "mb": 19874.97612833,
      "i_pr": -0.3,
      "pr": 0.000174,
      "adl_tt": 10000,
      "adl_sr": -0.25
    },
    {
      "c": "USDT",
      "s": "BERAUSDT",
      "b": 453.36427074,
      "mb": 453.36427074,
      "i_pr": -0.3,
      "pr": 0.24576,
      "adl_tt": 10000,
      "adl_sr": -0.25
    },
    ...,
  ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# All Liquidation

Subscribe to the liquidation stream, push all liquidations that occur on Bybit.

> **Covers: USDT contract / USDC contract / Inverse contract**

Push frequency: **500ms**

**Topic:**

`allLiquidation.{symbol}` e.g., allLiquidation.BTCUSDT

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | Object |  |
| \> T | number | The updated timestamp (ms) |
| \> s | string | Symbol name |
| \> S | string | Position side. `Buy`,`Sell`. When you receive a `Buy` update, this means that a long position has been liquidated |
| \> v | string | Executed size |
| \> p | string | [Bankruptcy price](https://www.bybit.com/en-US/help-center/s/article/Bankruptcy-Price-USDT-Contract) |

### Subscribe Example

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.all_liquidation_stream("ROSEUSDT", handle_message)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "allLiquidation.ROSEUSDT",
    "type": "snapshot",
    "ts": 1739502303204,
    "data": [
        {
            "T": 1739502302929,
            "s": "ROSEUSDT",
            "S": "Sell",
            "v": "20000",
            "p": "0.04499"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Insurance Pool

Subscribe to get the update of insurance pool balance

Push frequency: **1s**

**Topic:**

USDT contracts: `insurance.USDT`

USDC contracts: `insurance.USDC` ( **note**: all USDC Perpetuals, USDC Futures have their own shared insurance pools)

Inverse contracts: `insurance.inverse`

info

- Shared insurance pool data is **not** pushed, please refer to Rest API [Get Insurance](https://bybit-exchange.github.io/docs/v5/market/insurance) to understand which symbols belong to isolated or shared insurance pools.
- No event will be published if the balances of all insurance pools remain unchanged.

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot`, `delta` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | Object |  |
| \> coin | string | Insurance pool coin |
| \> symbols | string | Symbol name |
| \> balance | string | Balance |
| \> updateTime | string | Data updated timestamp (ms) |

### Subscribe Example

- JSON
- Python

```json
{
    "op": "subscribe",
    "args": [
        "insurance.USDT",
        "insurance.USDC"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.insurance_pool_stream(
    contract_group=["USDT", "USDC"],
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "insurance.USDT",
    "type": "delta",
    "ts": 1747722930000,
    "data": [
        {
            "coin": "USDT",
            "symbols": "GRIFFAINUSDT",
            "balance": "25614.92972633",
            "updateTime": "1747722930000"
        },
        {
            "coin": "USDT",
            "symbols": "CGPTUSDT",
            "balance": "100000.27064825",
            "updateTime": "1747722930000"
        },
        {
            "coin": "USDT",
            "symbols": "GOATUSDT",
            "balance": "20352.32665441",
            "updateTime": "1747722930000"
        },
        {
            "coin": "USDT",
            "symbols": "XTERUSDT",
            "balance": "19998.81533291",
            "updateTime": "1747722930000"
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Kline

Subscribe to the klines stream.

tip

If `confirm`=true, this means that the candle has closed. Otherwise, the candle is still open and updating.

**Available intervals:**

- `1``3``5``15``30` (min)
- `60``120``240``360``720` (min)
- `D` (day)
- `W` (week)
- `M` (month)

**Push frequency:** 1-60s

**Topic:**

`kline.{interval}.{symbol}` e.g., kline.30.BTCUSDT

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object |
| \> start | number | The start timestamp (ms) |
| \> end | number | The end timestamp (ms) |
| \> [interval](https://bybit-exchange.github.io/docs/v5/enum#interval) | string | Kline interval |
| \> open | string | Open price |
| \> close | string | Close price |
| \> high | string | Highest price |
| \> low | string | Lowest price |
| \> volume | string | Trade volume |
| \> turnover | string | Turnover |
| \> confirm | boolean | Whether the tick is ended or not |
| \> timestamp | number | The timestamp (ms) of the last matched order in the candle |

### Subscribe Example

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.kline_stream(
    interval=5,
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "kline.5.BTCUSDT",
    "data": [
        {
            "start": 1672324800000,
            "end": 1672325099999,
            "interval": "5",
            "open": "16649.5",
            "close": "16677",
            "high": "16677",
            "low": "16608",
            "volume": "2.081",
            "turnover": "34666.4005",
            "confirm": false,
            "timestamp": 1672324988882
        }
    ],
    "ts": 1672324988882,
    "type": "snapshot"
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Order Price Limit

Subscribe to Get Order Price Limit.

For derivative trading order price limit, refer to [announcement](https://announcements.bybit.com/en/article/adjustments-to-bybit-s-derivative-trading-limit-order-mechanism-blt469228de1902fff6/)

For spot trading order price limit, refer to [announcement](https://announcements.bybit.com/en/article/title-adjustments-to-bybit-s-spot-trading-limit-order-mechanism-blt786c0c5abf865983/)

Push frequency: **300ms**

**Topic:**

`priceLimit.{symbol}`

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object. |
| \> symbol | string | Symbol name |
| \> buyLmt | string | Highest Bid Price |
| \> sellLmt | string | Lowest Ask Price |

### Subscribe Example

- JSON
- Python

```json
{
    "op": "subscribe",
    "args": [
        "priceLimit.BTCUSDT"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.price_limit_stream(
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "priceLimit.BTCUSDT",
    "data": {
        "symbol": "BTCUSDT",
        "buyLmt": "114450.00",
        "sellLmt": "103550.00"
    },
    "ts": 1750059683782
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Orderbook

Subscribe to the orderbook stream. Supports different depths.

info

[Retail Price Improvement (RPI)](https://www.bybit.com/en/help-center/article/Retail-Price-Improvement-RPI-Order) orders will not be included in the messages.

### Depths

**Linear & inverse:**

Level 1 data, push frequency: **10ms**

Level 50 data, push frequency: **20ms**

Level 200 data, push frequency: **100ms**

Level 1000 data, push frequency: **200ms**

**Spot:**

Level 1 data, push frequency: **10ms**

Level 50 data, push frequency: **20ms**

Level 200 data, push frequency: **100ms**

Level 1000 data, push frequency: **200ms**

**Option:**

Level 25 data, push frequency: **20ms**

Level 100 data, push frequency: **100ms**

**Topic:**

`orderbook.{depth}.{symbol}` e.g., orderbook.1.BTCUSDT

### Process snapshot/delta

To process `snapshot` and `delta` messages, please follow these rules:

Once you have subscribed successfully, you will receive a `snapshot`. The WebSocket will keep pushing `delta` messages every time the orderbook changes. If you receive a new `snapshot` message, you will have to reset your local orderbook. If there is a problem on Bybit's end, a `snapshot` will be re-sent, which is guaranteed to contain the latest data.

To apply `delta` updates:

- If you receive an amount that is `0`, delete the entry
- If you receive an amount that does not exist, insert it
- If the entry exists, you simply update the value

See working code examples of this logic in the [FAQ](https://bybit-exchange.github.io/docs/faq#how-can-i-process-websocket-snapshot-and-delta-messages).

info

- Linear, inverse, spot level 1 data: if 3 seconds have elapsed without a change in the orderbook, a `snapshot` message will be pushed again, and the field `u` will be the
same as that in the previous message.
- **Linear, inverse, spot level 1 data has `snapshot` message only**
- **PreLaunch contracts**: there is no feed until `ContinuousTrading` stage

reminder

- Spot (all levels) & Futures (Level 1): the "ts" field appears **before** the "type" field in the JSON message, e.g., `{"toptic": "orderbook.1.BTCUSDT", "ts": "1772694601512", "type": "snapshot", ...}`
- Futures (all other levels): the "ts" field appears **after** the "type" field in the JSON message, e.g., `{"toptic": "orderbook.50.BTCUSDT", "type": "delta", "ts": "1772694601512", ...}`

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot`,`delta` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | map | Object |
| \> s | string | Symbol name |
| \> b | array | Bids. For `snapshot` stream. Sorted by price in descending order |
| >\> b\[0\] | string | Bid price |
| >\> b\[1\] | string | Bid size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| \> a | array | Asks. For `snapshot` stream. Sorted by price in ascending order |
| >\> a\[0\] | string | Ask price |
| >\> a\[1\] | string | Ask size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| \> u | integer | Update ID<br>- Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of the service. So please overwrite your local orderbook<br>- For level 1 of linear, inverse Perps and Futures, the snapshot data will be pushed again when there is no change in 3 seconds, and the "u" will be the same as that in the previous message |
| \> seq | integer | Cross sequence <br>- You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier. |
| cts | number | The timestamp from the matching engine when this orderbook data is produced. It can be correlated with `T` from [public trade channel](https://bybit-exchange.github.io/docs/v5/websocket/public/trade) |

### Subscribe Example

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.orderbook_stream(
    depth=50,
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

- Snapshot
- Delta

```json
{
    "topic": "orderbook.50.BTCUSDT",
    "type": "snapshot",
    "ts": 1672304484978,
    "data": {
        "s": "BTCUSDT",
        "b": [
            ...,
            [
                "16493.50",
                "0.006"
            ],
            [
                "16493.00",
                "0.100"
            ]
        ],
        "a": [
            [
                "16611.00",
                "0.029"
            ],
            [
                "16612.00",
                "0.213"
            ],
            ...,
        ],
    "u": 18521288,
    "seq": 7961638724
    },
    "cts": 1672304484976
}
```

```json
{
    "topic": "orderbook.50.BTCUSDT",
    "type": "delta",
    "ts": 1687940967466,
    "data": {
        "s": "BTCUSDT",
        "b": [
            [
                "30247.20",
                "30.028"
            ],
            [
                "30245.40",
                "0.224"
            ],
            [
                "30242.10",
                "1.593"
            ],
            [
                "30240.30",
                "1.305"
            ],
            [
                "30240.00",
                "0"
            ]
        ],
        "a": [
            [
                "30248.70",
                "0"
            ],
            [
                "30249.30",
                "0.892"
            ],
            [
                "30249.50",
                "1.778"
            ],
            [
                "30249.60",
                "0"
            ],
            [
                "30251.90",
                "2.947"
            ],
            [
                "30252.20",
                "0.659"
            ],
            [
                "30252.50",
                "4.591"
            ]
        ],
        "u": 177400507,
        "seq": 66544703342
    },
    "cts": 1687940967464
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# RPI Orderbook

Subscribe to the orderbook stream including RPI quote

### Depths

**Spot, Perpetual & Futures:**

Level 50 data, push frequency: **100ms**

**Topic:**

`orderbook.rpi.{symbol}` e.g., orderbook.rpi.BTCUSDT

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot`,`delta` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | map | Object |
| \> s | string | Symbol name |
| \> b | array | Bids. For `snapshot` stream. Sorted by price in descending order |
| >\> b\[0\] | string | Bid price |
| >\> b\[1\] | string | None RPI bid size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| >\> b\[2\] | string | RPI bid size <br>- When a bid RPI order crosses with a non-RPI ask price, the quantity of the bid RPI becomes invalid and is hidden |
| \> a | array | Asks. For `snapshot` stream. Sorted by price in ascending order |
| >\> a\[0\] | string | Ask price |
| >\> a\[1\] | string | None RPI ask size <br>- The delta data has size=0, which means that all quotations for this price have been filled or cancelled |
| >\> a\[2\] | string | RPI ask size <br>- When an ask RPI order crosses with a non-RPI bid price, the quantity of the ask RPI becomes invalid and is hidden |
| \> u | integer | Update ID<br>- Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of the service. So please overwrite your local orderbook |
| \> seq | integer | Cross sequence <br>- You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier. |
| cts | number | The timestamp from the matching engine when this orderbook data is produced. It can be correlated with `T` from [public trade channel](https://bybit-exchange.github.io/docs/v5/websocket/public/trade) |

### Subscribe Example

- JSON
- Python

```json
{
    "op": "subscribe",
    "args": [
        "orderbook.rpi.BTCUSDT"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.rpi_orderbook_stream(
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Subscribe Success Response

```json
{
    "success": true,
    "ret_msg": "subscribe",
    "conn_id": "f6b17b77-48b6-4c5c-b5ec-4a1c733f5763",
    "op": "subscribe"
}
```

### Response Example

```json
{
    "topic": "orderbook.rpi.BTCUSDT",
    "ts": 1752472188075,
    "type": "delta",
    "data": {
        "s": "BTCUSDT",
        "b": [
            [
                "121975.1",
                "0.114259",
                "0"
            ],
            [
                "121969.9",
                "0",
                "0"
            ],
            [
                "121960.5",
                "0",
                "0.163986"
            ]
        ],
        "a": [
            [
                "121990.8",
                "0.441585",
                "0.78821"
            ],
            [
                "121996.1",
                "0.016393",
                "0"
            ],
            [
                "122018.5",
                "0",
                "0"
            ]
        ],
        "u": 2258980,
        "seq": 79683241099
    },
    "cts": 1752472188067
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Ticker

Subscribe to the ticker stream.

note

- This topic utilises the snapshot field and delta field. If a response param is not found in the message, then its value has not changed.
- Spot & Option tickers message are `snapshot` **only**

Push frequency: Derivatives & Options - **100ms**, Spot - **50ms**

**Topic:**

`tickers.{symbol}`

### Response Parameters

- Linear/Inverse
- Option
- Spot

| Parameter | Type | Comments |
| --- | --- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot`,`delta` |
| cs | integer | Cross sequence |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object |
| \> symbol | string | Symbol name |
| \> [tickDirection](https://bybit-exchange.github.io/docs/v5/enum#tickdirection) | string | Tick direction |
| \> price24hPcnt | string | Percentage change of market price in the last 24 hours |
| \> lastPrice | string | Last price |
| \> prevPrice24h | string | Market price 24 hours ago |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> prevPrice1h | string | Market price an hour ago |
| \> markPrice | string | Mark price |
| \> indexPrice | string | Index price |
| \> openInterest | string | Open interest size |
| \> openInterestValue | string | Open interest value |
| \> turnover24h | string | Turnover for 24h |
| \> volume24h | string | Volume for 24h |
| \> nextFundingTime | string | Next funding timestamp (ms) |
| \> fundingRate | string | Funding rate |
| \> bid1Price | string | Best bid price |
| \> bid1Size | string | Best bid size |
| \> ask1Price | string | Best ask price |
| \> ask1Size | string | Best ask size |
| \> deliveryTime | datetime | Delivery date time (UTC+0), applicable to expired futures only |
| \> basisRate | string | Basis rate. _Unique field for inverse futures & USDT/USDC futures_ |
| \> deliveryFeeRate | string | Delivery fee rate. _Unique field for inverse futures & USDT/USDC futures_ |
| \> predictedDeliveryPrice | string | Predicated delivery price. _Unique field for inverse futures & USDT/USDC futures_ |
| \> preOpenPrice | string | Estimated pre-market contract open price <br>- The value is meaningless when entering continuous trading phase<br>- USDC Futures and Inverse Futures do not have this field |
| \> preQty | string | Estimated pre-market contract open qty <br>- The value is meaningless when entering continuous trading phase<br>- USDC Futures and Inverse Futures do not have this field |
| \> [curPreListingPhase](https://bybit-exchange.github.io/docs/v5/enum#curauctionphase) | string | The current pre-market contract phase <br>- USDC Futures and Inverse Futures do not have this field |
| \> fundingIntervalHour | string | Funding interval hour<br>- This value currently only supports whole hours<br>- Only for Perpetual,For Futures,this field will not return |
| \> fundingCap | string | Funding rate upper and lower limits<br>- Only for Perpetual,For Futures,this field will not return |
| \> basisRateYear | string | Annual basis rate<br>- Only for Futures,For Perpetual,this field will not return |

| Parameter | Type | Comments |
| --- | --- | --- |
| topic | string | Topic name |
| type | string | Data type. `snapshot` |
| id | string | message ID |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object |
| \> symbol | string | Symbol name |
| \> bidPrice | string | Best bid price |
| \> bidSize | string | Best bid size |
| \> bidIv | string | Best bid iv |
| \> askPrice | string | Best ask price |
| \> askSize | string | Best ask size |
| \> askIv | string | Best ask iv |
| \> lastPrice | string | Last price |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> markPrice | string | Mark price |
| \> indexPrice | string | Index price |
| \> markPriceIv | string | Mark price iv |
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
| \> predictedDeliveryPrice | string | Predicated delivery price. It has value when 30 min before delivery |
| \> change24h | string | The change in the last 24 hous |

| Parameter | Type | Comments |
| --- | --- | --- |
| topic | string | Topic name |
| ts | number | The timestamp (ms) that the system generates the data |
| type | string | Data type. `snapshot` |
| cs | integer | Cross sequence |
| data | array | Object |
| \> symbol | string | Symbol name |
| \> lastPrice | string | Last price |
| \> highPrice24h | string | The highest price in the last 24 hours |
| \> lowPrice24h | string | The lowest price in the last 24 hours |
| \> prevPrice24h | string | Percentage change of market price relative to 24h |
| \> volume24h | string | Volume for 24h |
| \> turnover24h | string | Turnover for 24h |
| \> price24hPcnt | string | Percentage change of market price relative to 24h |
| \> usdIndexPrice | string | USD index price <br>- used to calculate USD value of the assets in Unified account<br>- non-collateral margin coin returns "" |

### Subscribe Example

- Linear
- Option
- Spot

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.ticker_stream(
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="option",
)
def handle_message(message):
    print(message)
ws.ticker_stream(
    symbol="tickers.BTC-22JAN23-17500-C",
    callback=handle_message
)
while True:
    sleep(1)
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="spot",
)
def handle_message(message):
    print(message)
ws.ticker_stream(
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

- Linear
- Option
- Spot

```json
LinearPerpetual
{
  "topic": "tickers.BTCUSDT",
  "type": "snapshot",
  "data": {
    "symbol": "BTCUSDT",
    "tickDirection": "MinusTick",
    "price24hPcnt": "-0.158315",
    "lastPrice": "66666.60",
    "prevPrice24h": "79206.20",
    "highPrice24h": "79266.30",
    "lowPrice24h": "65076.90",
    "prevPrice1h": "66666.60",
    "markPrice": "66666.60",
    "indexPrice": "115418.19",
    "openInterest": "492373.72",
    "openInterestValue": "32824881841.75",
    "turnover24h": "4936790807.6521",
    "volume24h": "73191.3870",
    "fundingIntervalHour": "8",
    "fundingCap": "0.005",
    "nextFundingTime": "1760342400000",
    "fundingRate": "-0.005",
    "bid1Price": "66666.60",
    "bid1Size": "23789.165",
    "ask1Price": "66666.70",
    "ask1Size": "23775.469",
    "preOpenPrice": "",
    "preQty": "",
    "curPreListingPhase": ""
  },
  "cs": 9532239429,
  "ts": 1760325052630
}
LinearFutures
{
  "topic": "tickers.BTC-26DEC25",
  "type": "snapshot",
  "data": {
    "symbol": "BTC-26DEC25",
    "tickDirection": "ZeroMinusTick",
    "price24hPcnt": "0",
    "lastPrice": "109401.50",
    "prevPrice24h": "109401.50",
    "highPrice24h": "109401.50",
    "lowPrice24h": "109401.50",
    "prevPrice1h": "109401.50",
    "markPrice": "121144.63",
    "indexPrice": "114132.51",
    "openInterest": "6.622",
    "openInterestValue": "802219.74",
    "turnover24h": "0.0000",
    "volume24h": "0.0000",
    "deliveryTime": "2025-12-26T08:00:00Z",
    "basisRate": "0.06129209",
    "deliveryFeeRate": "0",
    "predictedDeliveryPrice": "0.00",
    "basis": "-4730.84",
    "basisRateYear": "0.30655351",
    "nextFundingTime": "",
    "fundingRate": "",
    "bid1Price": "111254.50",
    "bid1Size": "0.176",
    "ask1Price": "131001.00",
    "ask1Size": "0.580"
  },
  "cs": 31337927919,
  "ts": 1760409119857
}
```

```json
{
    "id": "tickers.BTC-6JAN23-17500-C-2480334983-1672917511074",
    "topic": "tickers.BTC-6JAN23-17500-C",
    "ts": 1672917511074,
    "data": {
        "symbol": "BTC-6JAN23-17500-C",
        "bidPrice": "0",
        "bidSize": "0",
        "bidIv": "0",
        "askPrice": "10",
        "askSize": "5.1",
        "askIv": "0.514",
        "lastPrice": "10",
        "highPrice24h": "25",
        "lowPrice24h": "5",
        "markPrice": "7.86976724",
        "indexPrice": "16823.73",
        "markPriceIv": "0.4896",
        "underlyingPrice": "16815.1",
        "openInterest": "49.85",
        "turnover24h": "446802.8473",
        "volume24h": "26.55",
        "totalVolume": "86",
        "totalTurnover": "1437431",
        "delta": "0.047831",
        "gamma": "0.00021453",
        "vega": "0.81351067",
        "theta": "-19.9115368",
        "predictedDeliveryPrice": "0",
        "change24h": "-0.33333334"
    },
    "type": "snapshot"
}
```

```json
{
    "topic": "tickers.BTCUSDT",
    "ts": 1673853746003,
    "type": "snapshot",
    "cs": 2588407389,
    "data": {
        "symbol": "BTCUSDT",
        "lastPrice": "21109.77",
        "highPrice24h": "21426.99",
        "lowPrice24h": "20575",
        "prevPrice24h": "20704.93",
        "volume24h": "6780.866843",
        "turnover24h": "141946527.22907118",
        "price24hPcnt": "0.0196",
        "usdIndexPrice": "21120.2400136"
    }
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Trade

Subscribe to the recent trades stream.

After subscription, you will be pushed trade messages in real-time.

Push frequency: **real-time**

**Topic:**

`publicTrade.{symbol}`

**Note**: option uses baseCoin, e.g., publicTrade.BTC

note

- For Futures and Spot, a single message may have up to 1024 trades. As such, multiple messages may be sent for the same `seq`.
- **PreLaunch contracts**: there is no feed until `ContinuousTrading` stage

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| id | string | Message id. _Unique field for option_ |
| topic | string | Topic name |
| type | string | Data type. `snapshot` |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object. Sorted by the time the trade was matched in ascending order |
| \> T | number | The timestamp (ms) that the order is filled |
| \> s | string | Symbol name |
| \> S | string | Side of taker. `Buy`,`Sell` |
| \> v | string | Trade size |
| \> p | string | Trade price |
| \> [L](https://bybit-exchange.github.io/docs/v5/enum#tickdirection) | string | Direction of price change. _Unique field for Perps & futures_ |
| \> i | string | Trade ID |
| \> BT | boolean | Whether it is a block trade order or not |
| \> RPI | boolean | Whether it is a RPI trade or not |
| \> seq | integer | cross sequence |
| \> mP | string | Mark price, unique field for `option` |
| \> iP | string | Index price, unique field for `option` |
| \> mIv | string | Mark iv, unique field for `option` |
| \> iv | string | iv, unique field for `option` |

### Subscribe Example

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="linear",
)
def handle_message(message):
    print(message)
ws.trade_stream(
    symbol="BTCUSDT",
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "publicTrade.BTCUSDT",
    "type": "snapshot",
    "ts": 1672304486868,
    "data": [
        {
            "T": 1672304486865,
            "s": "BTCUSDT",
            "S": "Buy",
            "v": "0.001",
            "p": "16578.50",
            "L": "PlusTick",
            "i": "20f43950-d8dd-5b31-9112-a178eb6023af",
            "BT": false,
            "seq": 1783284617
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# System Status

Listen to the system status when there is a platform maintenance or service incident.

info

Please note currently system maintenance that may result in short interruption (lasting less than 10 seconds) or websocket disconnection (users can immediately reconnect) will not be announced.

## URL

- **Mainnet:**

`wss://stream.bybit.com/v5/public/misc/status`

info

- EU users registered from " [www.bybit.eu"](http://www.bybit.eu"/), please use `wss://stream.bybit.eu/v5/public/misc/status`

**Topic:**

`system.status`

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| topic | string | Topic name |
| ts | number | The timestamp (ms) that the system generates the data |
| data | array | Object |
| \> id | string | Id. Unique identifier |
| \> title | string | Title of system maintenance |
| \> [state](https://bybit-exchange.github.io/docs/v5/enum#state) | string | System state |
| \> begin | string | Start time of system maintenance, timestamp in milliseconds |
| \> end | string | End time of system maintenance, timestamp in milliseconds. Before maintenance is completed, it is the expected end time; After maintenance is completed, it will be changed to the actual end time. |
| \> href | string | Hyperlink to system maintenance details. Default value is empty string |
| \> [serviceTypes](https://bybit-exchange.github.io/docs/v5/enum#servicetypes) | array<int> | Service Type |
| \> [product](https://bybit-exchange.github.io/docs/v5/enum#product) | array<int> | Product |
| \> uidSuffix | array<int> | Affected UID tail number |
| \> [maintainType](https://bybit-exchange.github.io/docs/v5/enum#maintaintype) | string | Maintenance type |
| \> [env](https://bybit-exchange.github.io/docs/v5/enum#env) | string | Environment |

### Subscribe Example

- JSON
- Python

```json
{
    "op": "subscribe",
    "args": [
        "system.status"
    ]
}
```

```python
from pybit.unified_trading import WebSocket
from time import sleep
ws = WebSocket(
    testnet=True,
    channel_type="misc/status",
)
def handle_message(message):
    print(message)
ws.system_status_stream(
    callback=handle_message
)
while True:
    sleep(1)
```

### Response Example

```json
{
    "topic": "system.status",
    "ts": 1751858399649,
    "data": [
        {
            "id": "4d95b2a0-587f-11f0-bcc9-56f28c94d6ea",
            "title": "t06",
            "state": "completed",
            "begin": "1751596902000",
            "end": "1751597011000",
            "href": "",
            "serviceTypes": [
                2,
                3,
                4,
                5
            ],
            "product": [
                1,
                2
            ],
            "uidSuffix": [],
            "maintainType": 1,
            "env": 1
        }
    ]
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Websocket Trade Guideline

## URL

- **Mainnet:**

`wss://stream.bybit.com/v5/trade`

info

- Turkey users registered from " [www.bybit-tr.com"](http://www.bybit-tr.com"/), please use `wss://stream.bybit-tr.com/v5/trade`
- Kazakhstan users registered from " [www.bybit.kz"](http://www.bybit.kz"/), please use `wss://stream.bybit.kz/v5/trade`

- **Testnet:**

`wss://stream-testnet.bybit.com/v5/trade`

## Scope

- **Support**: USDT Contract, USDC Contract, Spot, Options, Inverse contract
- **Not support**: demo trading, spread trading

## Authentication

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| reqId | false | string | Optional field, used to match the response <br>- If not passed, this field will not be returned in response |
| op | **true** | string | Op type. `auth` |
| args | **true** | string | \["api key", expiry timestamp, "signature"\]. Please click [here](https://bybit-exchange.github.io/docs/v5/ws/connect#authentication) to generate signature |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| reqId | string | - If it is passed on the request, then it is returned in the response<br>- If it is not passed, then it is not returned in the response |
| retCode | integer | - `0`: auth success<br>- `20001`: repeat auth<br>- `10004`: invalid sign<br>- `10001`: param error |
| retMsg | string | - `OK`<br>- Error message |
| op | string | Op type |
| connId | string | Connection id, the unique id for the connection |

### Request Example

```json
{
    "op": "auth",
    "args": [
        "XXXXXX",
        1711010121452,
        "ec71040eff72b163a36153d770b69d6637bcb29348fbfbb16c269a76595ececf"
    ]
}
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "op": "auth",
    "connId": "cnt5leec0hvan15eukcg-2t"
}
```

## Create/Amend/Cancel Order

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| reqId | false | string | Used to identify the uniqueness of the request, the response will return it when passed. The length cannot exceed 36 characters. <br>- If passed, it can't be duplicated, otherwise you will get "20006" |
| header | **true** | object | Request headers |
| \> X-BAPI-TIMESTAMP | **true** | string | Current timestamp |
| \> X-BAPI-RECV-WINDOW | false | string | 5000(ms) by default. Request will be rejected when not satisfy this rule: _Bybit\_server\_time - X-BAPI-RECV-WINDOW <= X-BAPI-TIMESTAMP < Bybit\_server\_time + 1000_ |
| \> Referer | false | string | The referer identifier for API broker user |
| op | **true** | string | Op type <br>- `order.create`: create an order<br>- `order.amend`: amend an order<br>- `order.cancel`: cancel an order |
| args | **true** | array<object> | Args array, support one item only for now <br>- `order.create`: refer to [create order request](https://bybit-exchange.github.io/docs/v5/order/create-order#request-parameters)<br>- `order.amend`: refer to [amend order request](https://bybit-exchange.github.io/docs/v5/order/amend-order#request-parameters)<br>- `order.cancel`: refer to [cancel order request](https://bybit-exchange.github.io/docs/v5/order/cancel-order#request-parameters) |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| reqId | string | - If it is passed on the request, then it is returned in the response<br>- If it is not passed, then it is not returned in the response |
| retCode | integer | - `0`: success<br>- `10403`: exceed IP rate limit. 3000 requests per second per IP<br>- `10404`: 1\. op type is not found; 2. `category` is not correct/supported<br>- `10429`: System level frequency protection<br>- `20006`: reqId is duplicated<br>- `10016`: 1\. internal server error; 2. Service is restarting<br>- `10019`: ws trade service is restarting, do not accept new request, but the request in the process is not affected. You can build new connection to be routed to normal service |
| retMsg | string | - `OK`<br>- `""`<br>- Error message |
| op | string | Op type |
| data | object | Business data, keep the same as `result` on rest api response <br>- `order.create`: refer to [create order response](https://bybit-exchange.github.io/docs/v5/order/create-order#response-parameters)<br>- `order.amend`: refer to [amend order response](https://bybit-exchange.github.io/docs/v5/order/amend-order#response-parameters)<br>- `order.cancel`: refer to [cancel order response](https://bybit-exchange.github.io/docs/v5/order/cancel-order#response-parameters) |
| retExtInfo | object | Always empty object |
| header | object | Header info |
| \> TraceId | string | Trace ID, used to track the trip of request |
| \> Timenow | string | Current timestamp |
| \> X-Bapi-Limit | string | The total rate limit of the current account for this op type |
| \> X-Bapi-Limit-Status | string | The remaining rate limit of the current account for this op type |
| \> X-Bapi-Limit-Reset-Timestamp | string | The timestamp indicates when your request limit resets if you have exceeded your rate limit. Otherwise, this is just the current timestamp (it may not exactly match `timeNow`) |
| connId | string | Connection id, the unique id for the connection |

info

The ack of create/amend/cancel order request indicates that the request is successfully accepted. Please use websocket order stream to confirm the order status

### Request Example

```json
{
    "reqId": "test-005",
    "header": {
        "X-BAPI-TIMESTAMP": "1711001595207",
        "X-BAPI-RECV-WINDOW": "8000",
        "Referer": "bot-001" // for api broker
    },
    "op": "order.create",
    "args": [
        {
            "symbol": "ETHUSDT",
            "side": "Buy",
            "orderType": "Limit",
            "qty": "0.2",
            "price": "2800",
            "category": "linear",
            "timeInForce": "PostOnly"
        }
    ]
}
```

### Response Example

```json
{
    "reqId": "test-005",
    "retCode": 0,
    "retMsg": "OK",
    "op": "order.create",
    "data": {
        "orderId": "a4c1718e-fe53-4659-a118-1f6ecce04ad9",
        "orderLinkId": ""
    },
    "retExtInfo": {},
    "header": {
        "X-Bapi-Limit": "10",
        "X-Bapi-Limit-Status": "9",
        "X-Bapi-Limit-Reset-Timestamp": "1711001595208",
        "Traceid": "38b7977b430f9bd228f4b19724794dfd",
        "Timenow": "1711001595209"
    },
    "connId": "cnt5leec0hvan15eukcg-2v"
}
```

## Batch Create/Amend/Cancel Order

info

- A maximum of 20 orders (option), 20 orders (inverse), 20 orders (linear), 10 orders (spot) can be placed per request. The returned data list is divided into two lists. The first list indicates whether or not the order creation was successful and the second list details the created order information. The structure of the two lists are completely consistent.

- **Option rate limt** instruction: its rate limit is count based on the actual number of request sent, e.g., by default, option trading rate limit is 10 reqs per sec, so you can send up to 20 \* 10 = 200 orders in one second.
- **Perpetual, Futures, Spot rate limit instruction**, please check [here](https://bybit-exchange.github.io/docs/v5/rate-limit#api-rate-limit-rules-for-vips)

- The account rate limit is shared between websocket and http batch orders
- The acknowledgement of batch create/amend/cancel order requests indicates that the request was sucessfully accepted. The request is asynchronous so please use the websocket to confirm the order status.

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| reqId | false | string | Used to identify the uniqueness of the request, the response will return it when passed. The length cannot exceed 36 characters. <br>- If passed, it can't be duplicated, otherwise you will get "20006" |
| header | **true** | object | Request headers |
| \> X-BAPI-TIMESTAMP | **true** | string | Current timestamp |
| \> X-BAPI-RECV-WINDOW | false | string | 5000(ms) by default. Request will be rejected when not satisfy this rule: _Bybit\_server\_time - X-BAPI-RECV-WINDOW <= X-BAPI-TIMESTAMP < Bybit\_server\_time + 1000_ |
| \> Referer | false | string | The referer identifier for API broker user |
| op | **true** | string | Op type <br>- `order.create-batch`: batch create orders<br>- `order.amend-batch`: batch amend orders<br>- `order.cancel-batch`: batch cancel orders |
| args | **true** | array<object> | Args array <br>- `order.create-batch`: refer to [Batch Place Order request](https://bybit-exchange.github.io/docs/v5/order/batch-place#request-parameters)<br>- `order.amend-batch`: refer to [Batch Amend Order request](https://bybit-exchange.github.io/docs/v5/order/batch-amend#request-parameters)<br>- `order.cancel-batch`: refer to [Batch Cancel Order request](https://bybit-exchange.github.io/docs/v5/order/batch-cancel#request-parameters) |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| reqId | string | - If it is passed on the request, then it is returned in the response<br>- If it is not passed, then it is not returned in the response |
| retCode | integer | - `0`: success<br>- `10403`: exceed IP rate limit. 3000 requests per second per IP<br>- `10404`: 1\. op type is not found; 2. `category` is not correct/supported<br>- `10429`: System level frequency protection<br>- `20006`: reqId is duplicated<br>- `10016`: 1\. internal server error; 2. Service is restarting<br>- `10019`: ws trade service is restarting, do not accept new request, but the request in the process is not affected. You can build new connection to be routed to normal service |
| retMsg | string | - `OK`<br>- `""`<br>- Error message |
| op | string | Op type |
| data | object | Business data, keep the same as `result` on rest api response <br>- `order.create-batch`: refer to [Batch Place Order response](https://bybit-exchange.github.io/docs/v5/order/batch-place#response-parameters)<br>- `order.amend-batch`: refer to [Batch Amend Order response](https://bybit-exchange.github.io/docs/v5/order/batch-amend#response-parameters)<br>- `order.cancel-batch`: refer to [Batch Cancel Order response](https://bybit-exchange.github.io/docs/v5/order/batch-cancel#response-parameters) |
| retExtInfo | object |  |
| \> list | array<object> |  |
| >\> code | number | Success/error code |
| >\> msg | string | Success/error message |
| header | object | Header info |
| \> TraceId | string | Trace ID, used to track the trip of request |
| \> Timenow | string | Current timestamp |
| \> X-Bapi-Limit | string | The total rate limit of the current account for this op type |
| \> X-Bapi-Limit-Status | string | The remaining rate limit of the current account for this op type |
| \> X-Bapi-Limit-Reset-Timestamp | string | The timestamp indicates when your request limit resets if you have exceeded your rate limit. Otherwise, this is just the current timestamp (it may not exactly match `timeNow`) |
| connId | string | Connection id, the unique id for the connection |

### Request Example

```json

{
    "op": "order.create-batch",
    "header": {
        "X-BAPI-TIMESTAMP": "1740453381256",
        "X-BAPI-RECV-WINDOW": "1000"
    },
    "args": [
        {
            "category": "linear",
            "request": [
                {
                    "symbol": "SOLUSDT",
                    "qty": "10",
                    "price": "500",
                    "orderType": "Limit",
                    "timeInForce": "GTC",
                    "orderLinkId": "-batch-000",
                    "side": "Buy"
                },
                {
                    "symbol": "SOLUSDT",
                    "qty": "20",
                    "price": "1000",
                    "orderType": "Limit",
                    "timeInForce": "GTC",
                    "orderLinkId": "batch-001",
                    "side": "Buy"
                },
                {
                    "symbol": "SOLUSDT",
                    "qty": "30",
                    "price": "1500",
                    "orderType": "Limit",
                    "timeInForce": "GTC",
                    "orderLinkId": "batch-002",
                    "side": "Buy"
                }
            ]
        }
    ]
}
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "op": "order.create-batch",
    "data": {
        "list": [
            {
                "category": "linear",
                "symbol": "SOLUSDT",
                "orderId": "",
                "orderLinkId": "batch-000",
                "createAt": ""
            },
            {
                "category": "linear",
                "symbol": "SOLUSDT",
                "orderId": "",
                "orderLinkId": "batch-001",
                "createAt": ""
            },
            {
                "category": "linear",
                "symbol": "SOLUSDT",
                "orderId": "",
                "orderLinkId": "batch-002",
                "createAt": ""
            }
        ]
    },
    "retExtInfo": {
        "list": [
            {
                "code": 10001,
                "msg": "position idx not match position mode"
            },
            {
                "code": 10001,
                "msg": "position idx not match position mode"
            },
            {
                "code": 10001,
                "msg": "position idx not match position mode"
            }
        ]
    },
    "header": {
        "Timenow": "1740453408556",
        "X-Bapi-Limit": "150",
        "X-Bapi-Limit-Status": "147",
        "X-Bapi-Limit-Reset-Timestamp": "1740453408555",
        "Traceid": "0e32b551b3e17aae77651aadf6a5be80"
    },
    "connId": "cupviqn88smf24t2kpb0-536o"
}
```

## Ping

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| op | **true** | string | Op type. `ping` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| retCode | integer | Result code |
| retMsg | string | Result message |
| op | string | Op type `pong` |
| data | array | One item in the array, current timestamp (string) |
| connId | string | Connection id, the unique id for the connection |

### Request Example

```json
{
    "op": "ping"
}
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "op": "pong",
    "data": [
        "1711002002529"
    ],
    "connId": "cnt5leec0hvan15eukcg-2v"
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

