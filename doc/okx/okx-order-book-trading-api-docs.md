# OKX Order Book Trading API (Trade, Algo, Grid, Market Data)

Source: https://www.okx.com/docs-v5/en/

---

# Order Book Trading

## Trade

All `Trade` API endpoints require authentication.

### POST / Place order

You can place an order only if you have sufficient funds.

#### Rate Limit: 60 requests per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

#### HTTP Request

`POST /api/v5/trade/order`

> Request Example

```
Copy to Clipboard
 place order for SPOT
 POST /api/v5/trade/order
 body
 {
    "instId":"BTC-USDT",
    "tdMode":"cash",
    "clOrdId":"b15",
    "side":"buy",
    "ordType":"limit",
    "px":"2.15",
    "sz":"2"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Spot mode, limit order
result = tradeAPI.place_order(
    instId="BTC-USDT",
    tdMode="cash",
    clOrdId="b15",
    side="buy",
    ordType="limit",
    px="2.15",
    sz="2"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| tdMode | String | Yes | Trade mode<br>Margin mode `cross``isolated`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading, `tdMode` should be `spot_isolated` for `SPOT` lead trading.)<br>Note: `isolated` is not available in multi-currency margin mode and portfolio margin mode. |
| ccy | String | No | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`. |
| clOrdId | String | No | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>Only applicable to general order. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| tag | String | No | Order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| side | String | Yes | Order side, `buy``sell` |
| posSide | String | Conditional | Position side <br>The default is `net` in the `net` mode <br>It is required in the `long/short` mode, and can only be `long` or `short`. <br>Only applicable to `FUTURES`/`SWAP`. |
| ordType | String | Yes | Order type <br>`market`: Market order, only applicable to `SPOT/MARGIN/FUTURES/SWAP`<br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order (applicable only to Expiry Futures and Perpetual Futures).<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode) <br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode)<br>`elp`: Enhanced Liquidity Program order |
| sz | String | Yes | Quantity to buy or sell |
| px | String | Conditional | Order price. Only applicable to `limit`,`post_only`,`fok`,`ioc`,`mmp`,`mmp_and_post_only` order.<br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| pxUsd | String | Conditional | Place options orders in `USD`<br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| pxVol | String | Conditional | Place options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| reduceOnly | Boolean | No | Whether orders can only reduce in position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>Only applicable to `MARGIN` orders, and `FUTURES`/`SWAP` orders in `net` mode <br>Only applicable to `Futures mode` and `Multi-currency margin` |
| tgtCcy | String | No | Whether the target currency uses the quote or base currency.<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| banAmend | Boolean | No | Whether to disallow the system from amending the size of the SPOT Market Order.<br>Valid options: `true` or `false`. The default value is `false`.<br>If `true`, system will not amend and reject the market order if user does not have sufficient funds. <br>Only applicable to SPOT Market Orders |
| pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `px` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `px` exceeds the price limit<br> The default value is `0` |
| tradeQuoteCcy | String | No | The quote currency used for trading. Only applicable to `SPOT`. <br> The default value is the quote currency of the `instId`, for example: for `BTC-USD`, the default is `USD`. |
| stpMode | String | No | Self trade prevention mode. <br>`cancel_maker`,`cancel_taker`, `cancel_both`<br>Cancel both does not support FOK <br>The account-level acctStpMode will be used to place orders by default. The default value of this field is `cancel_maker`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order. |
| isElpTakerAccess | Boolean | No | ELP taker access<br>`true`: the request can trade with ELP orders but a speed bump will be applied<br>`false`: the request cannot trade with ELP orders and no speed bump<br>The default value is `false` while `true` is only applicable to ioc orders. |
| attachAlgoOrds | Array of objects | No | TP/SL information attached when placing order |
| \> attachAlgoClOrdId | String | No | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Conditional | Take-profit trigger price<br>For condition TP order, if you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | Conditional | Take profit trigger ratio, 0.3 represents 30% <br> Only one of `tpTriggerPx` and `tpTriggerRatio` can be passed <br> Only applicable to FUTURES and SWAP. <br>If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. |
| \> tpOrdPx | String | Conditional | Take-profit order price <br>For condition TP order, if you fill in this parameter, you should fill in the take-profit trigger price as well. <br>For limit TP order, you need to fill in this parameter, but the take-profit trigger price doesn’t need to be filled. <br>If the price is -1, take-profit will be executed at the market price. |
| \> tpOrdKind | String | No | TP order kind<br>`condition`<br>`limit`<br> The default is `condition` |
| \> slTriggerPx | String | Conditional | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | Conditional | Stop profit trigger ratio, 0.3 represents 30% <br> Only one of `slTriggerPx` and `slTriggerRatio` can be passed <br> Only applicable to FUTURES and SWAP. <br>If the main order is a buy order, it should be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. |
| \> slOrdPx | String | Conditional | Stop-loss order price<br>If you fill in this parameter, you should fill in the stop-loss trigger price.<br>If the price is -1, stop-loss will be executed at the market price. |
| \> tpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> sz | String | Conditional | Size. Only applicable to TP order of split TPs, and it is required for TP order of split TPs |
| \> amendPxOnTriggerType | String | No | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. Whether `slTriggerPx` will move to `avgPx` when the first TP order is triggered<br>`0`: disable, the default value <br>`1`: Enable |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "312269865356374016",
      "tag": "",
      "ts":"1695190491421",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    }
  ],
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tag | String | Order tag |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection or success message of event execution. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

tdMode

Trade Mode, when placing an order, you need to specify the trade mode.

**Spot mode:**

\- SPOT and OPTION buyer: cash

**Futures mode:**

\- Isolated MARGIN: isolated

\- Cross MARGIN: cross

\- SPOT: cash

\- Cross FUTURES/SWAP/OPTION: cross

\- Isolated FUTURES/SWAP/OPTION: isolated

**Multi-currency margin mode:**

\- Cross SPOT: cross

\- Cross FUTURES/SWAP/OPTION: cross

**Portfolio margin:**

\- Cross SPOT: cross

\- Cross FUTURES/SWAP/OPTION: cross

clOrdId

clOrdId is a user-defined unique ID used to identify the order. It will be included in the response parameters if you have specified during order submission, and can be used as a request parameter to the endpoints to query, cancel and amend orders.

clOrdId must be unique among the clOrdIds of all pending orders.

posSide

Position side, this parameter is not mandatory in **net** mode. If you pass it through, the only valid value is **net**.

In **long/short** mode, it is mandatory. Valid values are **long** or **short**.

In **long/short** mode, **side** and **posSide** need to be specified in the combinations below:

Open long: buy and open long (side: fill in buy; posSide: fill in long)

Open short: sell and open short (side: fill in sell; posSide: fill in short)

Close long: sell and close long (side: fill in sell; posSide: fill in long)

Close short: buy and close short (side: fill in buy; posSide: fill in short)

Portfolio margin mode: Expiry Futures and Perpetual Futures only support net mode

ordType

Order type. When creating a new order, you must specify the order type. The order type you specify will affect: 1) what order parameters are required, and 2) how the matching system executes your order. The following are valid order types:

limit: Limit order, which requires specified sz and px.

market: Market order. For SPOT and MARGIN, market order will be filled with market price (by swiping opposite order book). For Expiry Futures and Perpetual Futures, market order will be placed to order book with most aggressive price allowed by Price Limit Mechanism. For OPTION, market order is not supported yet. As the filled price for market orders cannot be determined in advance, OKX reserves/freezes your quote currency by an additional 5% for risk check.

post\_only: Post-only order, which the order can only provide liquidity to the market and be a maker. If the order would have executed on placement, it will be canceled instead.

fok: Fill or kill order. If the order cannot be fully filled, the order will be canceled. The order would not be partially filled.

ioc: Immediate or cancel order. Immediately execute the transaction at the order price, cancel the remaining unfilled quantity of the order, and the order quantity will not be displayed in the order book.

optimal\_limit\_ioc: Market order with ioc (immediate or cancel). Immediately execute the transaction of this market order, cancel the remaining unfilled quantity of the order, and the order quantity will not be displayed in the order book. Only applicable to Expiry Futures and Perpetual Futures.

sz

Quantity to buy or sell.

For SPOT/MARGIN Buy and Sell Limit Orders, it refers to the quantity in base currency.

For MARGIN Buy Market Orders, it refers to the quantity in quote currency.

For MARGIN Sell Market Orders, it refers to the quantity in base currency.

For SPOT Market Orders, it is set by tgtCcy.

For FUTURES/SWAP/OPTION orders, it refers to the number of contracts.

reduceOnly

When placing an order with this parameter set to true, it means that the order will reduce the size of the position only

For the same MARGIN instrument, the coin quantity of all reverse direction pending orders adds \`sz\` of new \`reduceOnly\` order cannot exceed the position assets. After the debt is paid off, if there is a remaining size of orders, the position will not be opened in reverse, but will be traded in SPOT.

For the same FUTURES/SWAP instrument, the sum of the current order size and all reverse direction reduce-only pending orders which’s price-time priority is higher than the current order, cannot exceed the contract quantity of position.

Only applicable to \`Futures mode\` and \`Multi-currency margin\`

Only applicable to \`MARGIN\` orders, and \`FUTURES\`/\`SWAP\` orders in \`net\` mode

Notice: Under long/short mode of Expiry Futures and Perpetual Futures, all closing orders apply the reduce-only feature which is not affected by this parameter.

tgtCcy

This parameter is used to specify the order quantity in the order request is denominated in the quantity of base or quote currency. This is applicable to SPOT Market Orders only.

Base currency: base\_ccy

Quote currency: quote\_ccy

If you use the Base Currency quantity for buy market orders or the Quote Currency for sell market orders, please note:

1\. If the quantity you enter is greater than what you can buy or sell, the system will execute the order according to your maximum buyable or sellable quantity. If you want to trade according to the specified quantity, you should use Limit orders.

2\. When the market price is too volatile, the locked balance may not be sufficient to buy the Base Currency quantity or sell to receive the Quote Currency that you specified. We will change the quantity of the order to execute the order based on best effort principle based on your account balance. In addition, we will try to over lock a fraction of your balance to avoid changing the order quantity.

2.1 Example of base currency buy market order:

Taking the market order to buy 10 LTCs as an example, and the user can buy 11 LTC. At this time, if 10 < 11, the order is accepted. When the LTC-USDT market price is 200, and the locked balance of the user is 3,000 USDT, as 200\*10 < 3,000, the market order of 10 LTC is fully executed;
If the market is too volatile and the LTC-USDT market price becomes 400, 400\*10 > 3,000, the user's locked balance is not sufficient to buy using the specified amount of base currency, the user's maximum locked balance of 3,000 USDT will be used to settle the trade. Final transaction quantity becomes 3,000/400 = 7.5 LTC.

2.2 Example of quote currency sell market order:

Taking the market order to sell 1,000 USDT as an example, and the user can sell 1,200 USDT, 1,000 < 1,200, the order is accepted. When the LTC-USDT market price is 200, and the locked balance of the user is 6 LTC, as 1,000/200 < 6, the market order of 1,000 USDT is fully executed;
If the market is too volatile and the LTC-USDT market price becomes 100, 100\*6 < 1,000, the user's locked balance is not sufficient to sell using the specified amount of quote currency, the user's maximum locked balance of 6 LTC will be used to settle the trade. Final transaction quantity becomes 6 \* 100 = 600 USDT.

px

The value for px must be a multiple of tickSz for OPTION orders.

If not, the system will apply the rounding rules below. Using tickSz 0.0005 as an example:

The px will be rounded up to the nearest 0.0005 when the remainder of px to 0.0005 is more than 0.00025 or \`px\` is less than 0.0005.

The px will be rounded down to the nearest 0.0005 when the remainder of px to 0.0005 is less than 0.00025 and \`px\` is more than 0.0005.

For placing order with TP/Sl:

1\. TP/SL algo order will be generated only when this order is filled fully, or there is no TP/SL algo order generated.

2\. Attaching TP/SL is neither supported for market buy with tgtCcy is base\_ccy or market sell with tgtCcy is quote\_ccy

3\. If tpOrdKind is limit, and there is only one conditional TP order, attachAlgoClOrdId can be used as clOrdId for retrieving on "GET / Order details" endpoint.

4\. For “split TPs”, including condition TP order and limit TP order.

\\* TP/SL orders in Split TPs only support one-way TP/SL. You can't use slTriggerPx&slOrdPx and tpTriggerPx&tpOrdPx at the same time, or error code 51076 will be thrown.

\\* Take-profit trigger price types (tpTriggerPxType) must be the same in an order with Split TPs attached, or error code 51080 will be thrown.

\\* Take-profit trigger prices (tpTriggerPx) cannot be the same in an order with Split TPs attached, or error code 51081 will be thrown.

\\* The size of the TP order among split TPs attached cannot be empty, or error code 51089 will be thrown.

\\* The total size of TP orders with Split TPs attached in a same order should equal the size of this order, or error code 51083 will be thrown.

\\* The number of TP orders with Split TPs attached in a same order cannot exceed 10, or error code 51079 will be thrown.

\\* Setting multiple TP and cost-price SL orders isn’t supported for spot and margin trading, or error code 51077 will be thrown.

\\* The number of SL orders with Split TPs attached in a same order cannot exceed 1, or error code 51084 will be thrown.

\\* The number of TP orders cannot be less than 2 when cost-price SL is enabled (amendPxOnTriggerType set as 1) for Split TPs, or error code 51085 will be thrown.

\\* All TP orders in one order must be of the same type, or error code 51091 will be thrown.

\\* TP order prices (tpOrdPx) in one order must be different, or error code 51092 will be thrown.

\\* TP limit order prices (tpOrdPx) in one order can't be –1 (market price), or error code 51093 will be thrown.

\\* You can't place TP limit orders in spot, margin, or options trading. Otherwise, error code 51094 will be thrown.

Mandatory self trade prevention (STP)

The trading platform imposes mandatory self trade prevention at master account level, which means the accounts under the same master account, including master account itself and all its affiliated sub-accounts, will be prevented from self trade. The account-level acctStpMode will be used to place orders by default. The default value of this field is \`cancel\_maker\`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order.

Mandatory self trade prevention will not lead to latency.

There are three STP modes. The STP mode is always taken based on the configuration in the taker order.

1\. Cancel Maker: This is the default STP mode, which cancels the maker order to prevent self-trading. Then, the taker order continues to match with the next order based on the order book priority.

2\. Cancel Taker: The taker order is canceled to prevent self-trading. If the user's own maker order is lower in the order book priority, the taker order is partially filled and then canceled. FOK orders are always honored and canceled if they would result in self-trading.

3\. Cancel Both: Both taker and maker orders are canceled to prevent self-trading. If the user's own maker order is lower in the order book priority, the taker order is partially filled. Then, the remaining quantity of the taker order and the first maker order are canceled. FOK orders are not supported in this mode.

tradeQuoteCcy

For users in specific countries and regions, this parameter must be filled out for a successful order. Otherwise, the system will use the quote currency of instId as the default value, then error code 51000 will occur.

The value provided must be one of the enumerated values from tradeQuoteCcyList, which can be obtained from the endpoint Get instruments (GET /api/v5/account/instruments).

Rate limit of orders tagged as isElpTakerAccess:true

\- 50 orders per 2 seconds per User ID per instrument ID.

\- This rate limit is shared in Place order/Place multiple orders endpoints in REST/WebSocket

### POST / Place multiple orders

Place orders in batches. Maximum 20 orders can be placed per request.

Request parameters should be passed in the form of an array. Orders will be placed in turn

#### Rate Limit: 300 orders per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Place order\`.

#### HTTP Request

`POST /api/v5/trade/batch-orders`

> Request Example

```
Copy to Clipboard
 batch place order for SPOT
 POST /api/v5/trade/batch-orders
 body
 [
    {
        "instId":"BTC-USDT",
        "tdMode":"cash",
        "clOrdId":"b15",
        "side":"buy",
        "ordType":"limit",
        "px":"2.15",
        "sz":"2"
    },
    {
        "instId":"BTC-USDT",
        "tdMode":"cash",
        "clOrdId":"b16",
        "side":"buy",
        "ordType":"limit",
        "px":"2.15",
        "sz":"2"
    }
]
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Place multiple orders
place_orders_without_clOrdId = [
    {"instId": "BTC-USDT", "tdMode": "cash", "clOrdId": "b15", "side": "buy", "ordType": "limit", "px": "2.15", "sz": "2"},
    {"instId": "BTC-USDT", "tdMode": "cash", "clOrdId": "b16", "side": "buy", "ordType": "limit", "px": "2.15", "sz": "2"}
]

result = tradeAPI.place_multiple_orders(place_orders_without_clOrdId)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| tdMode | String | Yes | Trade mode<br>Margin mode `cross``isolated`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading, `tdMode` should be `spot_isolated` for `SPOT` lead trading.)<br>Note: `isolated` is not available in multi-currency margin mode and portfolio margin mode. |
| ccy | String | No | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`. |
| clOrdId | String | No | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| side | String | Yes | Order side `buy``sell` |
| posSide | String | Conditional | Position side <br>The default is `net` in the `net` mode <br>It is required in the `long/short` mode, and can only be `long` or `short`. <br>Only applicable to `FUTURES`/`SWAP`. |
| ordType | String | Yes | Order type <br>`market`: Market order, only applicable to `SPOT/MARGIN/FUTURES/SWAP`<br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order (applicable only to Expiry Futures and Perpetual Futures).<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`elp`: Enhanced Liquidity Program order |
| sz | String | Yes | Quantity to buy or sell |
| px | String | Conditional | Order price. Only applicable to `limit`,`post_only`,`fok`,`ioc`,`mmp`,`mmp_and_post_only` order.<br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| pxUsd | String | Conditional | Place options orders in `USD`<br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| pxVol | String | Conditional | Place options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| reduceOnly | Boolean | No | Whether the order can only reduce position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>Only applicable to `MARGIN` orders, and `FUTURES`/`SWAP` orders in `net` mode <br>Only applicable to `Futures mode` and `Multi-currency margin` |
| tgtCcy | String | No | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| banAmend | Boolean | No | Whether to disallow the system from amending the size of the SPOT Market Order.<br>Valid options: `true` or `false`. The default value is `false`.<br>If `true`, system will not amend and reject the market order if user does not have sufficient funds. <br>Only applicable to SPOT Market Orders |
| pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `px` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `px` exceeds the price limit<br> The default value is `0` |
| tradeQuoteCcy | String | No | The quote currency used for trading. Only applicable to `SPOT`. <br> The default value is the quote currency of the `instId`, for example: for `BTC-USD`, the default is `USD`. |
| stpMode | String | No | Self trade prevention mode. <br>`cancel_maker`,`cancel_taker`, `cancel_both`<br>Cancel both does not support FOK. <br>The account-level acctStpMode will be used to place orders by default. The default value of this field is `cancel_maker`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order. |
| isElpTakerAccess | Boolean | No | ELP taker access<br>`true`: the request can trade with ELP orders but a speed bump will be applied<br>`false`: the request cannot trade with ELP orders and no speed bump<br>The default value is `false` while `true` is only applicable to ioc orders. |
| attachAlgoOrds | Array of objects | No | TP/SL information attached when placing order |
| \> attachAlgoClOrdId | String | No | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Conditional | Take-profit trigger price<br>For condition TP order, if you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | Conditional | Take profit trigger ratio, 0.3 represents 30% <br> Only one of `tpTriggerPx` and `tpTriggerRatio` can be passed <br> Only applicable to FUTURES and SWAP.<br>If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. |
| \> tpOrdPx | String | Conditional | Take-profit order price <br>For condition TP order, if you fill in this parameter, you should fill in the take-profit trigger price as well. <br>For limit TP order, you need to fill in this parameter, take-profit trigger needn't to be filled.<br>If the price is -1, take-profit will be executed at the market price. |
| \> tpOrdKind | String | No | TP order kind<br>`condition`<br>`limit`<br> The default is `condition` |
| \> slTriggerPx | String | Conditional | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | Conditional | Stop profit trigger ratio, 0.3 represents 30% <br> Only one of `slTriggerPx` and `slTriggerRatio` can be passed <br> Only applicable to FUTURES and SWAP.<br>If the main order is a buy order, it should be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. |
| \> slOrdPx | String | Conditional | Stop-loss order price<br>If you fill in this parameter, you should fill in the stop-loss trigger price.<br>If the price is -1, stop-loss will be executed at the market price. |
| \> tpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> sz | String | Conditional | Size. Only applicable to TP order of split TPs, and it is required for TP order of split TPs |
| \> amendPxOnTriggerType | String | No | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. Whether `slTriggerPx` will move to `avgPx` when the first TP order is triggered<br>`0`: disable, the default value <br>`1`: Enable |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "clOrdId":"oktswap6",
            "ordId":"12345689",
            "tag":"",
            "ts":"1695190491421",
            "sCode":"0",
            "sMsg":"",
            "subCode": ""
        },
        {
            "clOrdId":"oktswap7",
            "ordId":"12344",
            "tag":"",
            "ts":"1695190491421",
            "sCode":"0",
            "sMsg":"",
            "subCode": ""
        }
    ],
    "inTime": "1695190491421339",
    "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tag | String | Order tag |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection or success message of event execution. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

In the \`Portfolio Margin\` account mode, either all orders are accepted by the system successfully, or all orders are rejected by the system.

clOrdId

clOrdId is a user-defined unique ID used to identify the order. It will be included in the response parameters if you have specified during order submission, and can be used as a request parameter to the endpoints to query, cancel and amend orders.

clOrdId must be unique among all pending orders and the current request.

Rate limit of orders tagged as isElpTakerAccess:true

\- 50 orders per 2 seconds per User ID per instrument ID.

\- This rate limit is shared in Place order/Place multiple orders endpoints in REST/WebSocket

### POST / Cancel order

Cancel an incomplete order.

#### Rate Limit: 60 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/cancel-order`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/cancel-order
body
{
    "ordId":"590908157585625111",
    "instId":"BTC-USD-190927"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Cancel order
result = tradeAPI.cancel_order(instId="BTC-USDT", ordId="590908157585625111")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required. If both are passed, ordId will be used. |
| clOrdId | String | Conditional | Client Order ID as assigned by the client |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "clOrdId":"oktswap6",
            "ordId":"12345689",
            "ts":"1695190491421",
            "sCode":"0",
            "sMsg":""
        }
    ],
    "inTime": "1695190491421339",
    "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection message if the request is unsuccessful. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

Cancel order returns with sCode equal to 0. It is not strictly considered that the order has been canceled. It only means that your cancellation request has been accepted by the system server. The result of the cancellation is subject to the state pushed by the order channel or the get order state.

### POST / Cancel multiple orders

Cancel incomplete orders in batches. Maximum 20 orders can be canceled per request. Request parameters should be passed in the form of an array.

#### Rate Limit: 300 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Cancel order\`.

#### HTTP Request

`POST /api/v5/trade/cancel-batch-orders`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/cancel-batch-orders
body
[
    {
        "instId":"BTC-USDT",
        "ordId":"590908157585625111"
    },
    {
        "instId":"BTC-USDT",
        "ordId":"590908544950571222"
    }
]
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Cancel multiple orders by ordId
cancel_orders_with_orderId = [
    {"instId": "BTC-USDT", "ordId": "590908157585625111"},
    {"instId": "BTC-USDT", "ordId": "590908544950571222"}
]

result = tradeAPI.cancel_multiple_orders(cancel_orders_with_orderId)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| ordId | String | Conditional | Order ID<br>Either `ordId` or `clOrdId` is required. If both are passed, `ordId` will be used. |
| clOrdId | String | Conditional | Client Order ID as assigned by the client |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "clOrdId":"oktswap6",
            "ordId":"12345689",
            "ts":"1695190491421",
            "sCode":"0",
            "sMsg":""
        },
        {
            "clOrdId":"oktswap7",
            "ordId":"12344",
            "ts":"1695190491421",
            "sCode":"0",
            "sMsg":""
        }
    ],
    "inTime": "1695190491421339",
    "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection message if the request is unsuccessful. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

### POST / Amend order

Amend an incomplete order.

#### Rate Limit: 60 requests per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

#### HTTP Request

`POST /api/v5/trade/amend-order`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/amend-order
body
{
    "ordId":"590909145319051111",
    "newSz":"2",
    "instId":"BTC-USDT"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Amend order
result = tradeAPI.amend_order(
    instId="BTC-USDT",
    ordId="590909145319051111",
    newSz="2"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID |
| cxlOnFail | Boolean | No | Whether the order needs to be automatically canceled when the order amendment fails <br>Valid options: `false` or `true`, the default is `false`. |
| ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required. If both are passed, `ordId` will be used. |
| clOrdId | String | Conditional | Client Order ID as assigned by the client |
| reqId | String | No | Client Request ID as assigned by the client for order amendment <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. <br>The response will include the corresponding `reqId` to help you identify the request if you provide it in the request. |
| newSz | String | Conditional | New quantity after amendment and it has to be larger than 0. When amending a partially-filled order, the `newSz` should include the amount that has been filled. |
| newPx | String | Conditional | New price after amendment. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. It must be consistent with parameters when placing orders. For example, if users placed the order using px, they should use newPx when modifying the order. |
| newPxUsd | String | Conditional | Modify options orders using USD prices <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| newPxVol | String | Conditional | Modify options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `newPx` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `newPx` exceeds the price limit<br> The default value is `0` |
| attachAlgoOrds | Array of objects | No | TP/SL information attached when placing order |
| \> attachAlgoId | String | Conditional | The order ID of attached TP/SL order. It is required to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Conditional | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> newTpTriggerPx | String | Conditional | Take-profit trigger price. <br>Either the take profit trigger price or order price is 0, it means that the take profit is deleted. |
| \> newTpTriggerRatio | String | Conditional | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `newTpTriggerPx` and `newTpTriggerRatio` can be passed. <br>If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. 0 means to delete the take-profit. |
| \> newTpOrdPx | String | Conditional | Take-profit order price<br>If the price is -1, take-profit will be executed at the market price. |
| \> newTpOrdKind | String | No | TP order kind<br>`condition`<br>`limit` |
| \> newSlTriggerPx | String | Conditional | Stop-loss trigger price<br>Either the stop loss trigger price or order price is 0, it means that the stop loss is deleted. |
| \> newSlTriggerRatio | String | Conditional | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP.<br> Only one of `newSlTriggerPx` and `newSlTriggerRatio` can be passed. <br>If the main order is a buy order, it should be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. <br> Only one of `newSlTriggerPx` and `newSlTriggerRatio` can be passed, 0 means to delete the stop-loss. |
| \> newSlOrdPx | String | Conditional | Stop-loss order price<br>If the price is -1, stop-loss will be executed at the market price. |
| \> newTpTriggerPxType | String | Conditional | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price<br>Only applicable to `FUTURES`/`SWAP`<br>If you want to add the take-profit, this parameter is required |
| \> newSlTriggerPxType | String | Conditional | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price<br>Only applicable to `FUTURES`/`SWAP`<br>If you want to add the stop-loss, this parameter is required |
| \> sz | String | Conditional | New size. Only applicable to TP order of split TPs, and it is required for TP order of split TPs |
| \> amendPxOnTriggerType | String | No | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
         "clOrdId":"",
         "ordId":"12344",
         "ts":"1695190491421",
         "reqId":"b12344",
         "sCode":"0",
         "sMsg":""
         "subCode": ""
        }
    ],
    "inTime": "1695190491421339",
    "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reqId | String | Client Request ID as assigned by the client for order amendment. |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection message if the request is unsuccessful. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

newSz

If the new quantity of the order is less than or equal to the filled quantity when you are amending a partially-filled order, the order status will be changed to filled.

The amend order returns sCode equal to 0. It is not strictly considered that the order has been amended. It only means that your amend order request has been accepted by the system server. The result of the amend is subject to the status pushed by the order channel or the order status query

### POST / Amend multiple orders

Amend incomplete orders in batches. Maximum 20 orders can be amended per request. Request parameters should be passed in the form of an array.

#### Rate Limit: 300 orders per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Amend order\`.

#### HTTP Request

`POST /api/v5/trade/amend-batch-orders`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/amend-batch-orders
body
[
    {
        "ordId":"590909308792049444",
        "newSz":"2",
        "instId":"BTC-USDT"
    },
    {
        "ordId":"590909308792049555",
        "newSz":"2",
        "instId":"BTC-USDT"
    }
]
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Amend incomplete orders in batches by ordId
amend_orders_with_orderId = [
    {"instId": "BTC-USDT", "ordId": "590909308792049444","newSz":"2"},
    {"instId": "BTC-USDT", "ordId": "590909308792049555","newSz":"2"}
]

result = tradeAPI.amend_multiple_orders(amend_orders_with_orderId)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID |
| cxlOnFail | Boolean | No | Whether the order needs to be automatically canceled when the order amendment fails <br>`false``true`, the default is `false`. |
| ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId`is required, if both are passed, `ordId` will be used. |
| clOrdId | String | Conditional | Client Order ID as assigned by the client |
| reqId | String | No | Client Request ID as assigned by the client for order amendment <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. <br>The response will include the corresponding `reqId` to help you identify the request if you provide it in the request. |
| newSz | String | Conditional | New quantity after amendment and it has to be larger than 0. When amending a partially-filled order, the `newSz` should include the amount that has been filled. |
| newPx | String | Conditional | New price after amendment. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. It must be consistent with parameters when placing orders. For example, if users placed the order using px, they should use newPx when modifying the order. |
| newPxUsd | String | Conditional | Modify options orders using USD prices <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| newPxVol | String | Conditional | Modify options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `newPx` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `newPx` exceeds the price limit<br> The default value is `0` |
| attachAlgoOrds | Array of objects | No | TP/SL information attached when placing order |
| \> attachAlgoId | String | Conditional | The order ID of attached TP/SL order. It is required to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Conditional | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> newTpTriggerPx | String | Conditional | Take-profit trigger price. <br>Either the take profit trigger price or order price is 0, it means that the take profit is deleted. |
| \> newTpTriggerRatio | String | Conditional | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `newTpTriggerPx` and `newTpTriggerRatio` can be passed <br> If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. <br> 0 means to delete the take-profit. |
| \> newTpOrdPx | String | Conditional | Take-profit order price<br>If the price is -1, take-profit will be executed at the market price. |
| \> newTpOrdKind | String | No | TP order kind<br>`condition`<br>`limit` |
| \> newSlTriggerPx | String | Conditional | Stop-loss trigger price<br>Either the stop loss trigger price or order price is 0, it means that the stop loss is deleted. |
| \> newSlTriggerRatio | String | Conditional | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `newSlTriggerPx` and `newSlTriggerRatio` can be passed <br> If the main order is a buy order, it must be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. <br> 0 means to delete the stop-loss. |
| \> newSlOrdPx | String | Conditional | Stop-loss order price<br>If the price is -1, stop-loss will be executed at the market price. |
| \> newTpTriggerPxType | String | Conditional | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price<br>Only applicable to `FUTURES`/`SWAP`<br>If you want to add the take-profit, this parameter is required |
| \> newSlTriggerPxType | String | Conditional | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price<br>Only applicable to `FUTURES`/`SWAP`<br>If you want to add the stop-loss, this parameter is required |
| \> sz | String | Conditional | New size. Only applicable to TP order of split TPs, and it is required for TP order of split TPs |
| \> amendPxOnTriggerType | String | No | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "clOrdId":"oktswap6",
            "ordId":"12345689",
            "ts":"1695190491421",
            "reqId":"b12344",
            "sCode":"0",
            "sMsg":"",
            "subCode": ""
        },
        {
            "clOrdId":"oktswap7",
            "ordId":"12344",
            "ts":"1695190491421",
            "reqId":"b12344",
            "sCode":"0",
            "sMsg":"",
            "subCode": ""
        }
    ],
    "inTime": "1695190491421339",
    "outTime": "1695190491423240"
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reqId | String | Client Request ID as assigned by the client for order amendment. |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection message if the request is unsuccessful. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at REST gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123`<br>The time is recorded after authentication. |
| outTime | String | Timestamp at REST gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

newSz

If the new quantity of the order is less than or equal to the filled quantity when you are amending a partially-filled order, the order status will be changed to filled.

### POST / Close positions

Close the position of an instrument via a market order.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/close-position`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/close-position
body
{
    "instId":"BTC-USDT-SWAP",
    "mgnMode":"cross"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Close the position of an instrument via a market order
result = tradeAPI.close_positions(
    instId="BTC-USDT-SWAP",
    mgnMode="cross"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID |
| posSide | String | Conditional | Position side <br>This parameter can be omitted in `net` mode, and the default value is `net`. You can only fill with `net`.<br>This parameter must be filled in under the `long/short` mode. Fill in `long` for close-long and `short` for close-short. |
| mgnMode | String | Yes | Margin mode<br>`cross``isolated` |
| ccy | String | Conditional | Margin currency, required in the case of closing `cross``MARGIN` position for `Futures mode`. |
| autoCxl | Boolean | No | Whether any pending orders for closing out needs to be automatically canceled when close position via a market order.<br>`false` or `true`, the default is `false`. |
| clOrdId | String | No | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "clOrdId": "",
            "instId": "BTC-USDT-SWAP",
            "posSide": "long",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID |
| posSide | String | Position side |
| clOrdId | String | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

if there are any pending orders for closing out and the orders do not need to be automatically canceled, it will return an error code and message to prompt users to cancel pending orders before closing the positions.

### GET / Order details

Retrieve order details.

#### Rate Limit: 60 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/order`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/order?ordId=1753197687182819328&instId=BTC-USDT
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve order details by ordId
result = tradeAPI.get_order(
    instId="BTC-USDT",
    ordId="680800019749904384"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT`<br>Only applicable to live instruments |
| ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required, if both are passed, `ordId` will be used |
| clOrdId | String | Conditional | Client Order ID as assigned by the client<br>If the `clOrdId` is associated with multiple orders, only the latest one will be returned. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0.00192834",
            "algoClOrdId": "",
            "algoId": "",
            "attachAlgoClOrdId": "",
            "attachAlgoOrds": [],
            "avgPx": "51858",
            "cTime": "1708587373361",
            "cancelSource": "",
            "cancelSourceReason": "",
            "category": "normal",
            "ccy": "",
            "clOrdId": "",
            "fee": "-0.00000192834",
            "feeCcy": "BTC",
            "fillPx": "51858",
            "fillSz": "0.00192834",
            "fillTime": "1708587373361",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "isTpLimit": "false",
            "lever": "",
            "linkedAlgoOrd": {
                "algoId": ""
            },
            "ordId": "680800019749904384",
            "ordType": "market",
            "pnl": "0",
            "posSide": "net",
            "px": "",
            "pxType": "",
            "pxUsd": "",
            "pxVol": "",
            "quickMgnType": "",
            "rebate": "0",
            "rebateCcy": "USDT",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "source": "",
            "state": "filled",
            "stpId": "",
            "stpMode": "",
            "sz": "100",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "quote_ccy",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "tpTriggerPxType": "",
            "tradeId": "744876980",
            "tradeQuoteCcy": "USDT",
            "uTime": "1708587373362"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instId | String | Instrument ID |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| tag | String | Order tag |
| px | String | Price<br>For options, use coin as unit (e.g. BTC, ETH) |
| pxUsd | String | Options price in USDOnly applicable to options; return "" for other instrument types |
| pxVol | String | Implied volatility of the options orderOnly applicable to options; return "" for other instrument types |
| pxType | String | Price type of options <br>`px`: Place an order based on price, in the unit of coin (the unit for the request parameter px is BTC or ETH) <br>`pxVol`: Place an order based on pxVol <br>`pxUsd`: Place an order based on pxUsd, in the unit of USD (the unit for the request parameter px is USD) |
| sz | String | Quantity to buy or sell |
| pnl | String | Profit and loss (excluding the fee).<br> Applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| ordType | String | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok)<br>`elp`: Enhanced Liquidity Program order |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| accFillSz | String | Accumulated fill quantity<br>The unit is `base_ccy` for SPOT and MARGIN, e.g. BTC-USDT, the unit is BTC; For market orders, the unit both is `base_ccy` when the tgtCcy is `base_ccy` or `quote_ccy`;<br>The unit is contract for `FUTURES`/`SWAP`/`OPTION` |
| fillPx | String | Last filled price. If none is filled, it will return "". |
| tradeId | String | Last traded ID |
| fillSz | String | Last filled quantity<br>The unit is `base_ccy` for SPOT and MARGIN, e.g. BTC-USDT, the unit is BTC; For market orders, the unit both is `base_ccy` when the tgtCcy is `base_ccy` or `quote_ccy`;<br>The unit is contract for `FUTURES`/`SWAP`/`OPTION` |
| fillTime | String | Last filled time |
| avgPx | String | Average filled price. If none is filled, it will return "". |
| state | String | State <br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`mmp_canceled` |
| stpId | String | ~~Self trade prevention ID<br>Return "" if self trade prevention is not applicable~~ (deprecated) |
| stpMode | String | Self trade prevention mode |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL. |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| attachAlgoOrds | Array of objects | TP/SL information attached when placing order |
| \> attachAlgoId | String | The order ID of attached TP/SL order. It can be used to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpOrdKind | String | TP order kind<br>`condition`<br>`limit` |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slTriggerRatio | String | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price. |
| \> sz | String | Size. Only applicable to TP order of split TPs |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> failCode | String | The error code when failing to place TP/SL order, e.g. 51020 <br>The default is "" |
| \> failReason | String | The error reason when failing to place TP/SL order. <br>The default is "" |
| linkedAlgoOrd | Object | Linked SL order detail, only applicable to the order that is placed by one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> algoId | String | Algo ID |
| feeCcy | String | Fee currency<br>For maker sell orders of Spot and Margin, this represents the quote currency. For all other cases, it represents the currency in which fees are charged. |
| fee | String | Fee amount<br>For Spot and Margin (excluding maker sell orders): accumulated fee charged by the platform, always negative<br>For maker sell orders in Spot and Margin, Expiry Futures, Perpetual Futures and Options: accumulated fee and rebate (always in quote currency for maker sell orders in Spot and Margin) |
| rebateCcy | String | Rebate currency<br>For maker sell orders of Spot and Margin, this represents the base currency. For all other cases, it represents the currency in which rebates are paid. |
| rebate | String | Rebate amount, only applicable to Spot and Margin<br>For maker sell orders: ~~Accumulated fee and~~ rebate amount in the unit of base currency.<br>For all other cases, it represents the maker rebate amount, always positive, return "" if no rebate. |
| source | String | Order source<br>`6`: The normal order triggered by the `trigger order`<br>`7`:The normal order triggered by the `TP/SL order`<br>`13`: The normal order triggered by the algo order<br>`25`:The normal order triggered by the `trailing stop order`<br>`34`: The normal order triggered by the chase order |
| category | String | Category<br>`normal`<br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge<br>`auto_conversion` |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| isTpLimit | String | Whether it is TP limit order. true or false |
| cancelSource | String | Code of the cancellation source. |
| cancelSourceReason | String | Reason for the cancellation. |
| quickMgnType | String | ~~Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay`~~ (Deprecated) |
| algoClOrdId | String | Client-supplied Algo ID. There will be a value when algo order attaching `algoClOrdId` is triggered, or it will be "". |
| algoId | String | Algo ID. There will be a value when algo order is triggered, or it will be "". |
| uTime | String | Update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| tradeQuoteCcy | String | The quote currency used for trading. |

### GET / Order List

Retrieve all incomplete orders under the current account.

#### Rate Limit: 60 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/orders-pending`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/orders-pending?ordType=post_only,fok,ioc&instType=SPOT
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve all incomplete orders
result = tradeAPI.get_order_list(
    instType="SPOT",
    ordType="post_only,fok,ioc"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USD-200927` |
| ordType | String | No | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order <br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok)<br>`elp`: Enhanced Liquidity Program order |
| state | String | No | State<br>`live`<br>`partially_filled` |
| after | String | No | Pagination of data to return records earlier than the requested `ordId` |
| before | String | No | Pagination of data to return records newer than the requested `ordId` |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0",
            "algoClOrdId": "",
            "algoId": "",
            "attachAlgoClOrdId": "",
            "attachAlgoOrds": [],
            "avgPx": "",
            "cTime": "1724733617998",
            "cancelSource": "",
            "cancelSourceReason": "",
            "category": "normal",
            "ccy": "",
            "clOrdId": "",
            "fee": "0",
            "feeCcy": "BTC",
            "fillPx": "",
            "fillSz": "0",
            "fillTime": "",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "isTpLimit": "false",
            "lever": "",
            "linkedAlgoOrd": {
                "algoId": ""
            },
            "ordId": "1752588852617379840",
            "ordType": "post_only",
            "pnl": "0",
            "posSide": "net",
            "px": "13013.5",
            "pxType": "",
            "pxUsd": "",
            "pxVol": "",
            "quickMgnType": "",
            "rebate": "0",
            "rebateCcy": "USDT",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "source": "",
            "state": "live",
            "stpId": "",
            "stpMode": "cancel_maker",
            "sz": "0.001",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "tpTriggerPxType": "",
            "tradeId": "",
            "tradeQuoteCcy": "USDT",
            "uTime": "1724733617998"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| tag | String | Order tag |
| px | String | Price <br>For options, use coin as unit (e.g. BTC, ETH) |
| pxUsd | String | Options price in USD<br>Only applicable to options; return "" for other instrument types |
| pxVol | String | Implied volatility of the options order<br>Only applicable to options; return "" for other instrument types |
| pxType | String | Price type of options <br>`px`: Place an order based on price, in the unit of coin (the unit for the request parameter px is BTC or ETH) <br>`pxVol`: Place an order based on pxVol <br>`pxUsd`: Place an order based on pxUsd, in the unit of USD (the unit for the request parameter px is USD) |
| sz | String | Quantity to buy or sell |
| pnl | String | Profit and loss (excluding the fee).<br> Applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| ordType | String | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok)<br>`elp`: Enhanced Liquidity Program order |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| accFillSz | String | Accumulated fill quantity |
| fillPx | String | Last filled price |
| tradeId | String | Last trade ID |
| fillSz | String | Last filled quantity |
| fillTime | String | Last filled time |
| avgPx | String | Average filled price. If none is filled, it will return "". |
| state | String | State<br>`live`<br>`partially_filled` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL. |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| attachAlgoOrds | Array of objects | TP/SL information attached when placing order |
| \> attachAlgoId | String | The order ID of attached TP/SL order. It can be used to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpOrdKind | String | TP order kind<br>`condition`<br>`limit` |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slTriggerRatio | String | Stop-loss trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price. |
| \> sz | String | Size. Only applicable to TP order of split TPs |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> failCode | String | The error code when failing to place TP/SL order, e.g. 51020 <br>The default is "" |
| \> failReason | String | The error reason when failing to place TP/SL order. <br>The default is "" |
| linkedAlgoOrd | Object | Linked SL order detail, only applicable to the order that is placed by one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> algoId | String | Algo ID |
| stpId | String | ~~Self trade prevention ID<br>Return "" if self trade prevention is not applicable~~ (deprecated) |
| stpMode | String | Self trade prevention mode |
| feeCcy | String | Fee currency<br>For maker sell orders of Spot and Margin, this represents the quote currency. For all other cases, it represents the currency in which fees are charged. |
| fee | String | Fee amount<br>For Spot and Margin (excluding maker sell orders): accumulated fee charged by the platform, always negative<br>For maker sell orders in Spot and Margin, Expiry Futures, Perpetual Futures and Options: accumulated fee and rebate (always in quote currency for maker sell orders in Spot and Margin) |
| rebateCcy | String | Rebate currency<br>For maker sell orders of Spot and Margin, this represents the base currency. For all other cases, it represents the currency in which rebates are paid. |
| rebate | String | Rebate amount, only applicable to Spot and Margin<br>For maker sell orders: ~~Accumulated fee and~~ rebate amount in the unit of base currency.<br>For all other cases, it represents the maker rebate amount, always positive, return "" if no rebate. |
| source | String | Order source<br>`6`: The normal order triggered by the `trigger order`<br>`7`: The normal order triggered by the `TP/SL order`<br>`13`: The normal order triggered by the algo order<br>`25`: The normal order triggered by the `trailing stop order`<br>`34`: The normal order triggered by the chase order |
| category | String | Category <br>`normal` |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| quickMgnType | String | ~~Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay`~~ (Deprecated) |
| algoClOrdId | String | Client-supplied Algo ID. There will be a value when algo order attaching `algoClOrdId` is triggered, or it will be "". |
| algoId | String | Algo ID. There will be a value when algo order is triggered, or it will be "". |
| isTpLimit | String | Whether it is TP limit order. true or false |
| uTime | String | Update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cancelSource | String | Code of the cancellation source. |
| cancelSourceReason | String | Reason for the cancellation. |
| tradeQuoteCcy | String | The quote currency used for trading. |

### GET / Order history (last 7 days)

Get completed orders which are placed in the last 7 days, including those placed 7 days ago but completed in the last 7 days.

The incomplete orders that have been canceled are only reserved for 2 hours.

#### Rate Limit: 40 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/orders-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/orders-history?ordType=post_only,fok,ioc&instType=SPOT
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get completed SPOT orders which are placed in the last 7 days
# The incomplete orders that have been canceled are only reserved for 2 hours
result = tradeAPI.get_orders_history(
    instType="SPOT",
    ordType="post_only,fok,ioc"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | yes | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| ordType | String | No | Order type<br>`market`: market order <br>`limit`: limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok) <br>`elp`: Enhanced Liquidity Program order |
| state | String | No | State<br>`canceled`<br>`filled`<br>`mmp_canceled`: Order canceled automatically due to Market Maker Protection |
| category | String | No | Category <br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge |
| after | String | No | Pagination of data to return records earlier than the requested `ordId` |
| before | String | No | Pagination of data to return records newer than the requested `ordId` |
| begin | String | No | Filter with a begin timestamp `cTime`. Unix timestamp format in milliseconds, e.g. 1597026383085 |
| end | String | No | Filter with an end timestamp `cTime`. Unix timestamp format in milliseconds, e.g. 1597026383085 |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0.00192834",
            "algoClOrdId": "",
            "algoId": "",
            "attachAlgoClOrdId": "",
            "attachAlgoOrds": [],
            "avgPx": "51858",
            "cTime": "1708587373361",
            "cancelSource": "",
            "cancelSourceReason": "",
            "category": "normal",
            "ccy": "",
            "clOrdId": "",
            "fee": "-0.00000192834",
            "feeCcy": "BTC",
            "fillPx": "51858",
            "fillSz": "0.00192834",
            "fillTime": "1708587373361",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "lever": "",
            "linkedAlgoOrd": {
                "algoId": ""
            },
            "ordId": "680800019749904384",
            "ordType": "market",
            "pnl": "0",
            "posSide": "",
            "px": "",
            "pxType": "",
            "pxUsd": "",
            "pxVol": "",
            "quickMgnType": "",
            "rebate": "0",
            "rebateCcy": "USDT",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "source": "",
            "state": "filled",
            "stpId": "",
            "stpMode": "",
            "sz": "100",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "quote_ccy",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "tpTriggerPxType": "",
            "tradeId": "744876980",
            "tradeQuoteCcy": "USDT",
            "uTime": "1708587373362",
            "isTpLimit": "false"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| tag | String | Order tag |
| px | String | Price <br>For options, use coin as unit (e.g. BTC, ETH) |
| pxUsd | String | Options price in USD<br>Only applicable to options; return "" for other instrument types |
| pxVol | String | Implied volatility of the options order<br>Only applicable to options; return "" for other instrument types |
| pxType | String | Price type of options <br>`px`: Place an order based on price, in the unit of coin (the unit for the request parameter px is BTC or ETH) <br>`pxVol`: Place an order based on pxVol <br>`pxUsd`: Place an order based on pxUsd, in the unit of USD (the unit for the request parameter px is USD) |
| sz | String | Quantity to buy or sell |
| ordType | String | Order type <br>`market`: market order <br>`limit`: limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok)<br>`elp`: Enhanced Liquidity Program order |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| accFillSz | String | Accumulated fill quantity |
| fillPx | String | Last filled price. If none is filled, it will return "". |
| tradeId | String | Last trade ID |
| fillSz | String | Last filled quantity |
| fillTime | String | Last filled time |
| avgPx | String | Average filled price. If none is filled, it will return "". |
| state | String | State <br>`canceled`<br>`filled`<br>`mmp_canceled` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL. |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| attachAlgoOrds | Array of objects | TP/SL information attached when placing order |
| \> attachAlgoId | String | The order ID of attached TP/SL order. It can be used to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpOrdKind | String | TP order kind<br>`condition`<br>`limit` |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slTriggerRatio | String | Stop-loss trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price. |
| \> sz | String | Size. Only applicable to TP order of split TPs |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> failCode | String | The error code when failing to place TP/SL order, e.g. 51020 <br>The default is "" |
| \> failReason | String | The error reason when failing to place TP/SL order. <br>The default is "" |
| linkedAlgoOrd | Object | Linked SL order detail, only applicable to the order that is placed by one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> algoId | String | Algo ID |
| stpId | String | ~~Self trade prevention ID<br>Return "" if self trade prevention is not applicable~~ (deprecated) |
| stpMode | String | Self trade prevention mode |
| feeCcy | String | Fee currency<br>For maker sell orders of Spot and Margin, this represents the quote currency. For all other cases, it represents the currency in which fees are charged. |
| fee | String | Fee amount<br>For Spot and Margin (excluding maker sell orders): accumulated fee charged by the platform, always negative<br>For maker sell orders in Spot and Margin, Expiry Futures, Perpetual Futures and Options: accumulated fee and rebate (always in quote currency for maker sell orders in Spot and Margin) |
| rebateCcy | String | Rebate currency<br>For maker sell orders of Spot and Margin, this represents the base currency. For all other cases, it represents the currency in which rebates are paid. |
| rebate | String | Rebate amount, only applicable to Spot and Margin<br>For maker sell orders: ~~Accumulated fee and~~ rebate amount in the unit of base currency.<br>For all other cases, it represents the maker rebate amount, always positive, return "" if no rebate. |
| source | String | Order source<br>`6`: The normal order triggered by the `trigger order`<br>`7`: The normal order triggered by the `TP/SL order`<br>`13`: The normal order triggered by the algo order<br>`25`: The normal order triggered by the `trailing stop order`<br>`34`: The normal order triggered by the chase order |
| pnl | String | Profit and loss (excluding the fee).<br> Applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| category | String | Category <br>`normal`<br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge<br>`auto_conversion` |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| cancelSource | String | Code of the cancellation source. |
| cancelSourceReason | String | Reason for the cancellation. |
| algoClOrdId | String | Client-supplied Algo ID. There will be a value when algo order attaching `algoClOrdId` is triggered, or it will be "". |
| algoId | String | Algo ID. There will be a value when algo order is triggered, or it will be "". |
| isTpLimit | String | Whether it is TP limit order. true or false |
| uTime | String | Update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| quickMgnType | String | ~~Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay`~~ (Deprecated) |
| tradeQuoteCcy | String | The quote currency used for trading. |

### GET / Order history (last 3 months)

Get completed orders which are placed in the last 3 months, including those placed 3 months ago but completed in the last 3 months.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/orders-history-archive`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/orders-history-archive?ordType=post_only,fok,ioc&instType=SPOT
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get completed SPOT orders which are placed in the last 3 months
result = tradeAPI.get_orders_history_archive(
    instType="SPOT",
    ordType="post_only,fok,ioc"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | yes | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USD-200927` |
| ordType | String | No | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok) <br>`elp`: Enhanced Liquidity Program order |
| state | String | No | State<br>`canceled`<br>`filled`<br>`mmp_canceled`: Order canceled automatically due to Market Maker Protection |
| category | String | No | Category <br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge |
| after | String | No | Pagination of data to return records earlier than the requested `ordId` |
| before | String | No | Pagination of data to return records newer than the requested `ordId` |
| begin | String | No | Filter with a begin timestamp `cTime`. Unix timestamp format in milliseconds, e.g. 1597026383085 |
| end | String | No | Filter with an end timestamp `cTime`. Unix timestamp format in milliseconds, e.g. 1597026383085 |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0.00192834",
            "algoClOrdId": "",
            "algoId": "",
            "attachAlgoClOrdId": "",
            "attachAlgoOrds": [],
            "avgPx": "51858",
            "cTime": "1708587373361",
            "cancelSource": "",
            "cancelSourceReason": "",
            "category": "normal",
            "ccy": "",
            "clOrdId": "",
            "fee": "-0.00000192834",
            "feeCcy": "BTC",
            "fillPx": "51858",
            "fillSz": "0.00192834",
            "fillTime": "1708587373361",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "lever": "",
            "ordId": "680800019749904384",
            "ordType": "market",
            "pnl": "0",
            "posSide": "",
            "px": "",
            "pxType": "",
            "pxUsd": "",
            "pxVol": "",
            "quickMgnType": "",
            "rebate": "0",
            "rebateCcy": "USDT",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "source": "",
            "state": "filled",
            "stpId": "",
            "stpMode": "",
            "sz": "100",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "quote_ccy",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "tpTriggerPxType": "",
            "tradeId": "744876980",
            "tradeQuoteCcy": "USDT",
            "uTime": "1708587373362",
            "isTpLimit": "false",
            "linkedAlgoOrd": {
                "algoId": ""
            }
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| tag | String | Order tag |
| px | String | Price <br>For options, use coin as unit (e.g. BTC, ETH) |
| pxUsd | String | Options price in USDOnly applicable to options; return "" for other instrument types |
| pxVol | String | Implied volatility of the options orderOnly applicable to options; return "" for other instrument types |
| pxType | String | Price type of options <br>`px`: Place an order based on price, in the unit of coin (the unit for the request parameter px is BTC or ETH) <br>`pxVol`: Place an order based on pxVol <br>`pxUsd`: Place an order based on pxUsd, in the unit of USD (the unit for the request parameter px is USD) |
| sz | String | Quantity to buy or sell |
| ordType | String | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode) <br>`op_fok`: Simple options (fok) <br>`elp`: Enhanced Liquidity Program order |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| accFillSz | String | Accumulated fill quantity |
| fillPx | String | Last filled price. If none is filled, it will return "". |
| tradeId | String | Last trade ID |
| fillSz | String | Last filled quantity |
| fillTime | String | Last filled time |
| avgPx | String | Average filled price. If none is filled, it will return "". |
| state | String | State <br>`canceled`<br>`filled`<br>`mmp_canceled` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL. |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| attachAlgoOrds | Array of objects | TP/SL information attached when placing order |
| \> attachAlgoId | String | The order ID of attached TP/SL order. It can be used to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpOrdKind | String | TP order kind<br>`condition`<br>`limit` |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slTriggerRatio | String | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price. |
| \> sz | String | Size. Only applicable to TP order of split TPs |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> failCode | String | The error code when failing to place TP/SL order, e.g. 51020 <br>The default is "" |
| \> failReason | String | The error reason when failing to place TP/SL order. <br>The default is "" |
| linkedAlgoOrd | Object | Linked SL order detail, only applicable to the order that is placed by one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> algoId | String | Algo ID |
| stpId | String | ~~Self trade prevention ID<br>Return "" if self trade prevention is not applicable~~ (deprecated) |
| stpMode | String | Self trade prevention mode |
| feeCcy | String | Fee currency<br>For maker sell orders of Spot and Margin, this represents the quote currency. For all other cases, it represents the currency in which fees are charged. |
| fee | String | Fee amount<br>For Spot and Margin (excluding maker sell orders): accumulated fee charged by the platform, always negative<br>For maker sell orders in Spot and Margin, Expiry Futures, Perpetual Futures and Options: accumulated fee and rebate (always in quote currency for maker sell orders in Spot and Margin) |
| rebateCcy | String | Rebate currency<br>For maker sell orders of Spot and Margin, this represents the base currency. For all other cases, it represents the currency in which rebates are paid. |
| rebate | String | Rebate amount, only applicable to Spot and Margin<br>For maker sell orders: ~~Accumulated fee and~~ rebate amount in the unit of base currency.<br>For all other cases, it represents the maker rebate amount, always positive, return "" if no rebate. |
| source | String | Order source<br>`6`: The normal order triggered by the `trigger order`<br>`7`:The normal order triggered by the `TP/SL order`<br>`13`: The normal order triggered by the algo order<br>`25`:The normal order triggered by the `trailing stop order`<br>`34`: The normal order triggered by the `chase order` |
| pnl | String | Profit and loss (excluding the fee).<br> Applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| category | String | Category <br>`normal`<br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge<br>`auto_conversion` |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| cancelSource | String | Code of the cancellation source. |
| cancelSourceReason | String | Reason for the cancellation. |
| algoClOrdId | String | Client-supplied Algo ID. There will be a value when algo order attaching `algoClOrdId` is triggered, or it will be "". |
| algoId | String | Algo ID. There will be a value when algo order is triggered, or it will be "". |
| isTpLimit | String | Whether it is TP limit order. true or false |
| uTime | String | Update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| quickMgnType | String | ~~Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay`~~ (Deprecated) |
| tradeQuoteCcy | String | The quote currency used for trading. |

This interface does not contain the order data of the \`Canceled orders without any fills\` type, which can be obtained through the \`Get Order History (last 7 days)\` interface.

As far as OPTION orders that are complete, pxVol and pxUsd will update in time for px order, pxVol will update in time for pxUsd order, pxUsd will update in time for pxVol order.

### GET / Transaction details (last 3 days)

Retrieve recently-filled transaction details in the last 3 day.

#### Rate Limit: 60 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/fills`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/fills
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve recently-filled transaction details
result = tradeAPI.get_fills()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| ordId | String | No | Order ID |
| subType | String | No | Transaction type <br>`1`: Buy<br>`2`: Sell<br>`3`: Open long<br>`4`: Open short<br>`5`: Close long<br>`6`: Close short <br>`100`: Partial liquidation close long<br>`101`: Partial liquidation close short<br>`102`: Partial liquidation buy<br>`103`: Partial liquidation sell<br>`104`: Liquidation long<br>`105`: Liquidation short<br>`106`: Liquidation buy <br>`107`: Liquidation sell <br>`110`: Liquidation transfer in<br>`111`: Liquidation transfer out <br>`118`: System token conversion transfer in<br>`119`: System token conversion transfer out<br>`112`: Delivery long<br>`113`: Delivery short <br>`125`: ADL close long<br>`126`: ADL close short<br>`127`: ADL buy<br>`128`: ADL sell <br>`212`: Auto borrow of quick margin<br>`213`: Auto repay of quick margin <br>`204`: block trade buy<br>`205`: block trade sell<br>`206`: block trade open long<br>`207`: block trade open short<br>`208`: block trade close long<br>`209`: block trade close short<br>`236`: Easy convert in<br>`237`: Easy convert out<br>`270`: Spread trading buy<br>`271`: Spread trading sell<br>`272`: Spread trading open long<br>`273`: Spread trading open short<br>`274`: Spread trading close long<br>`275`: Spread trading close short<br>`324`: Move position buy<br>`325`: Move position sell<br>`326`: Move position open long<br>`327`: Move position open short<br>`328`: Move position close long<br>`329`: Move position close short <br>`376`: Collateralized borrowing auto conversion buy<br>`377`: Collateralized borrowing auto conversion sell |
| after | String | No | Pagination of data to return records earlier than the requested `billId` |
| before | String | No | Pagination of data to return records newer than the requested `billId` |
| begin | String | No | Filter with a begin timestamp `ts`. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| end | String | No | Filter with an end timestamp `ts`. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "side": "buy",
            "fillSz": "0.00192834",
            "fillPx": "51858",
            "fillPxVol": "",
            "fillFwdPx": "",
            "fee": "-0.00000192834",
            "fillPnl": "0",
            "ordId": "680800019749904384",
            "feeRate": "-0.001",
            "instType": "SPOT",
            "fillPxUsd": "",
            "instId": "BTC-USDT",
            "clOrdId": "",
            "posSide": "net",
            "billId": "680800019754098688",
            "subType": "1",
            "fillMarkVol": "",
            "tag": "",
            "fillTime": "1708587373361",
            "execType": "T",
            "fillIdxPx": "",
            "tradeId": "744876980",
            "fillMarkPx": "",
            "feeCcy": "BTC",
            "ts": "1708587373362",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tradeId | String | Last trade ID |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| billId | String | Bill ID |
| subType | String | Transaction type |
| tag | String | Order tag |
| fillPx | String | Last filled price. It is the same as the px from "Get bills details". |
| fillSz | String | Last filled quantity |
| fillIdxPx | String | Index price at the moment of trade execution <br>For cross currency spot pairs, it returns baseCcy-USDT index price. For example, for LTC-ETH, this field returns the index price of LTC-USDT. |
| fillPnl | String | Last filled profit and loss, applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| fillPxVol | String | Implied volatility when filled <br>Only applicable to options; return "" for other instrument types |
| fillPxUsd | String | Options price when filled, in the unit of USD <br>Only applicable to options; return "" for other instrument types |
| fillMarkVol | String | Mark volatility when filled <br>Only applicable to options; return "" for other instrument types |
| fillFwdPx | String | Forward price when filled <br>Only applicable to options; return "" for other instrument types |
| fillMarkPx | String | Mark price when filled <br>Applicable to `FUTURES`, `SWAP`, `OPTION` |
| side | String | Order side, `buy``sell` |
| posSide | String | Position side <br>`long``short`<br>it returns `net` in`net` mode. |
| execType | String | Liquidity taker or maker<br>`T`: taker<br>`M`: maker<br>Not applicable to system orders such as ADL and liquidation |
| feeCcy | String | Trading fee or rebate currency |
| fee | String | The amount of trading fee or rebate. The trading fee deduction is negative, such as '-0.01'; the rebate is positive, such as '0.01'. |
| ts | String | Data generation time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |
| fillTime | String | Trade time which is the same as `fillTime` for the order channel. |
| feeRate | String | Fee rate. This field is returned for `SPOT` and `MARGIN` only |
| tradeQuoteCcy | String | The quote currency for trading. |

tradeId

For partial\_liquidation, full\_liquidation, or adl, when it comes to fill information, this field will be assigned a negative value to distinguish it from other matching transaction scenarios, when it comes to order information, this field will be 0.

ordId

Order ID, always "" for block trading.

clOrdId

Client-supplied order ID, always "" for block trading.

### GET / Transaction details (last 3 months)

This endpoint can retrieve data from the last 3 months.

#### Rate Limit: 10 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/fills-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/fills-history?instType=SPOT
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve SPOT transaction details in the last 3 months.
result = tradeAPI.get_fills_history(
    instType="SPOT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | YES | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| ordId | String | No | Order ID |
| subType | String | No | Transaction type <br>`1`: Buy<br>`2`: Sell<br>`3`: Open long<br>`4`: Open short<br>`5`: Close long<br>`6`: Close short <br>`100`: Partial liquidation close long<br>`101`: Partial liquidation close short<br>`102`: Partial liquidation buy<br>`103`: Partial liquidation sell<br>`104`: Liquidation long<br>`105`: Liquidation short<br>`106`: Liquidation buy <br>`107`: Liquidation sell <br>`110`: Liquidation transfer in<br>`111`: Liquidation transfer out <br>`118`: System token conversion transfer in<br>`119`: System token conversion transfer out<br>`112`: Delivery long<br>`113`: Delivery short <br>`125`: ADL close long<br>`126`: ADL close short<br>`127`: ADL buy<br>`128`: ADL sell <br>`212`: Auto borrow of quick margin<br>`213`: Auto repay of quick margin <br>`204`: block trade buy<br>`205`: block trade sell<br>`206`: block trade open long<br>`207`: block trade open short<br>`208`: block trade close long<br>`209`: block trade close short<br>`236`: Easy convert in<br>`237`: Easy convert out<br>`270`: Spread trading buy<br>`271`: Spread trading sell<br>`272`: Spread trading open long<br>`273`: Spread trading open short<br>`274`: Spread trading close long<br>`275`: Spread trading close short<br>`324`: Move position buy<br>`325`: Move position sell<br>`326`: Move position open long<br>`327`: Move position open short<br>`328`: Move position close long<br>`329`: Move position close short <br>`376`: Collateralized borrowing auto conversion buy<br>`377`: Collateralized borrowing auto conversion sell |
| after | String | No | Pagination of data to return records earlier than the requested `billId` |
| before | String | No | Pagination of data to return records newer than the requested `billId` |
| begin | String | No | Filter with a begin timestamp `ts`. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| end | String | No | Filter with an end timestamp `ts`. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "side": "buy",
            "fillSz": "0.00192834",
            "fillPx": "51858",
            "fillPxVol": "",
            "fillFwdPx": "",
            "fee": "-0.00000192834",
            "fillPnl": "0",
            "ordId": "680800019749904384",
            "feeRate": "-0.001",
            "instType": "SPOT",
            "fillPxUsd": "",
            "instId": "BTC-USDT",
            "clOrdId": "",
            "posSide": "net",
            "billId": "680800019754098688",
            "subType": "1",
            "fillMarkVol": "",
            "tag": "",
            "fillTime": "1708587373361",
            "execType": "T",
            "fillIdxPx": "",
            "tradeId": "744876980",
            "fillMarkPx": "",
            "feeCcy": "BTC",
            "ts": "1708587373362",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| tradeId | String | Last trade ID |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| billId | String | Bill ID |
| subType | String | Transaction type |
| tag | String | Order tag |
| fillPx | String | Last filled price |
| fillSz | String | Last filled quantity |
| fillIdxPx | String | Index price at the moment of trade execution <br>For cross currency spot pairs, it returns baseCcy-USDT index price. For example, for LTC-ETH, this field returns the index price of LTC-USDT. |
| fillPnl | String | Last filled profit and loss, applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| fillPxVol | String | Implied volatility when filled <br>Only applicable to options; return "" for other instrument types |
| fillPxUsd | String | Options price when filled, in the unit of USD <br>Only applicable to options; return "" for other instrument types |
| fillMarkVol | String | Mark volatility when filled <br>Only applicable to options; return "" for other instrument types |
| fillFwdPx | String | Forward price when filled <br>Only applicable to options; return "" for other instrument types |
| fillMarkPx | String | Mark price when filled <br>Applicable to `FUTURES`, `SWAP`, `OPTION` |
| side | String | Order side<br>`buy`<br>`sell` |
| posSide | String | Position side<br>`long`<br>`short`<br>it returns `net` in`net` mode. |
| execType | String | Liquidity taker or maker<br>`T`: taker<br>`M`: maker<br>Not applicable to system orders such as ADL and liquidation |
| feeCcy | String | Trading fee or rebate currency |
| fee | String | The amount of trading fee or rebate. The trading fee deduction is negative, such as '-0.01'; the rebate is positive, such as '0.01'. |
| ts | String | Data generation time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |
| fillTime | String | Trade time which is the same as `fillTime` for the order channel. |
| feeRate | String | Fee rate. This field is returned for `SPOT` and `MARGIN` only |
| tradeQuoteCcy | String | The quote currency for trading. |

tradeId

When the order category to which the transaction details belong is partial\_liquidation, full\_liquidation, or adl, this field will be assigned a negative value to distinguish it from other matching transaction scenarios.

ordId

Order ID, always "" for block trading.

clOrdId

Client-supplied order ID, always "" for block trading.

We advise you to use Get Transaction details (last 3 days)when you request data for recent 3 days.

### GET / Easy convert currency list

Get list of small convertibles and mainstream currencies. Only applicable to the crypto balance less than $10.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/easy-convert-currency-list`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/easy-convert-currency-list
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get list of small convertibles and mainstream currencies
result = tradeAPI.get_easy_convert_currency_list()
print(result)
```

#### Request Parameters

| Parameters | Type | Required | Description |
| --- | --- | --- | --- |
| source | String | No | Funding source<br>`1`: Trading account<br>`2`: Funding account<br>The default is `1`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "fromData": [
                {
                    "fromAmt": "6.580712708344864",
                    "fromCcy": "ADA"
                },
                {
                    "fromAmt": "2.9970000013055097",
                    "fromCcy": "USDC"
                }
            ],
            "toCcy": [
                "USDT",
                "BTC",
                "ETH",
                "OKB"
            ]
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fromData | Array of objects | Currently owned and convertible small currency list |
| \> fromCcy | String | Type of small payment currency convert from, e.g. `BTC` |
| \> fromAmt | String | Amount of small payment currency convert from |
| toCcy | Array of strings | Type of mainstream currency convert to, e.g. `USDT` |

### POST / Place easy convert

Convert small currencies to mainstream currencies.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/easy-convert`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/easy-convert
body
{
    "fromCcy": ["ADA","USDC"], //Seperated by commas
    "toCcy": "OKB"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Convert small currencies to mainstream currencies
result = tradeAPI.easy_convert(
    fromCcy=["ADA", "USDC"],
    toCcy="OKB"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| fromCcy | Array of strings | Yes | Type of small payment currency convert from <br>Maximum 5 currencies can be selected in one order. If there are multiple currencies, separate them with commas. |
| toCcy | String | Yes | Type of mainstream currency convert to <br>Only one receiving currency type can be selected in one order and cannot be the same as the small payment currencies. |
| source | String | No | Funding source<br>`1`: Trading account<br>`2`: Funding account<br>The default is `1`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "fillFromSz": "6.5807127",
            "fillToSz": "0.17171580105126",
            "fromCcy": "ADA",
            "status": "running",
            "toCcy": "OKB",
            "uTime": "1661419684687"
        },
        {
            "fillFromSz": "2.997",
            "fillToSz": "0.1683755161661844",
            "fromCcy": "USDC",
            "status": "running",
            "toCcy": "OKB",
            "uTime": "1661419684687"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| status | String | Current status of easy convert <br>`running`: Running <br>`filled`: Filled <br>`failed`: Failed |
| fromCcy | String | Type of small payment currency convert from |
| toCcy | String | Type of mainstream currency convert to |
| fillFromSz | String | Filled amount of small payment currency convert from |
| fillToSz | String | Filled amount of mainstream currency convert to |
| uTime | String | Trade time, Unix timestamp format in milliseconds, e.g. 1597026383085 |

### GET / Easy convert history

Get the history and status of easy convert trades in the past 7 days.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/easy-convert-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/easy-convert-history
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get the history of easy convert trades
result = tradeAPI.get_easy_convert_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| after | String | No | Pagination of data to return records earlier than the requested time (exclude), Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested time (exclude), Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "fillFromSz": "0.1761712511667539",
            "fillToSz": "6.7342205900000000",
            "fromCcy": "OKB",
            "status": "filled",
            "toCcy": "ADA",
            "acct": "18",
            "uTime": "1661313307979"
        },
        {
            "fillFromSz": "0.1722106121112177",
            "fillToSz": "2.9971018300000000",
            "fromCcy": "OKB",
            "status": "filled",
            "toCcy": "USDC",
            "acct": "18",
            "uTime": "1661313307979"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fromCcy | String | Type of small payment currency convert from |
| fillFromSz | String | Amount of small payment currency convert from |
| toCcy | String | Type of mainstream currency convert to |
| fillToSz | String | Amount of mainstream currency convert to |
| acct | String | The account where the mainstream currency is located<br>`6`: Funding account <br>`18`: Trading account |
| status | String | Current status of easy convert <br>`running`: Running <br>`filled`: Filled <br>`failed`: Failed |
| uTime | String | Trade time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / One-click repay currency list

Get list of debt currency data and repay currencies. Debt currencies include both cross and isolated debts. Only applicable to `Multi-currency margin`/`Portfolio margin`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/one-click-repay-currency-list`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/one-click-repay-currency-list
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get list of debt currency data and repay currencies
result = tradeAPI.get_oneclick_repay_list()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| debtType | String | No | Debt type <br>`cross`: cross <br>`isolated`: isolated |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtData": [
                {
                    "debtAmt": "29.653478",
                    "debtCcy": "LTC"
                },
                {
                    "debtAmt": "237803.6828295906051002",
                    "debtCcy": "USDT"
                }
            ],
            "debtType": "cross",
            "repayData": [
                {
                    "repayAmt": "0.4978335419825104",
                    "repayCcy": "ETH"
                }
            ]
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| debtData | Array of objects | Debt currency data list |
| \> debtCcy | String | Debt currency |
| \> debtAmt | String | Debt currency amount <br>Including principal and interest |
| debtType | String | Debt type <br>`cross`: cross <br>`isolated`: isolated |
| repayData | Array of objects | Repay currency data list |
| \> repayCcy | String | Repay currency |
| \> repayAmt | String | Repay currency's available balance amount |

### POST / Trade one-click repay

Trade one-click repay to repay cross debts. Isolated debts are not applicable.
The maximum repayment amount is based on the remaining available balance of funding and trading accounts.
Only applicable to `Multi-currency margin`/`Portfolio margin`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/one-click-repay`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/one-click-repay
body
{
    "debtCcy": ["ETH","BTC"],
    "repayCcy": "USDT"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Trade one-click repay to repay cross debts
result = tradeAPI.oneclick_repay(
    debtCcy=["ETH", "BTC"],
    repayCcy="USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| debtCcy | Array of strings | Yes | Debt currency type <br>Maximum 5 currencies can be selected in one order. If there are multiple currencies, separate them with commas. |
| repayCcy | String | Yes | Repay currency type <br>Only one receiving currency type can be selected in one order and cannot be the same as the small payment currencies. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtCcy": "ETH",
            "fillDebtSz": "0.01023052",
            "fillRepaySz": "30",
            "repayCcy": "USDT",
            "status": "filled",
            "uTime": "1646188520338"
        },
        {
            "debtCcy": "BTC",
            "fillFromSz": "3",
            "fillToSz": "60,221.15910001",
            "repayCcy": "USDT",
            "status": "filled",
            "uTime": "1646188520338"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| status | String | Current status of one-click repay <br>`running`: Running <br>`filled`: Filled <br>`failed`: Failed |
| debtCcy | String | Debt currency type |
| repayCcy | String | Repay currency type |
| fillDebtSz | String | Filled amount of debt currency |
| fillRepaySz | String | Filled amount of repay currency |
| uTime | String | Trade time, Unix timestamp format in milliseconds, e.g. 1597026383085 |

### GET / One-click repay history

Get the history and status of one-click repay trades in the past 7 days. Only applicable to `Multi-currency margin`/`Portfolio margin`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/one-click-repay-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/one-click-repay-history
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Get the history of one-click repay trades
result = tradeAPI.oneclick_repay_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| after | String | No | Pagination of data to return records earlier than the requested time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| before | String | No | Pagination of data to return records newer than the requested time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtCcy": "USDC",
            "fillDebtSz": "6950.4865447900000000",
            "fillRepaySz": "4.3067975995094930",
            "repayCcy": "ETH",
            "status": "filled",
            "uTime": "1661256148746"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| debtCcy | String | Debt currency type |
| fillDebtSz | String | Amount of debt currency transacted |
| repayCcy | String | Repay currency type |
| fillRepaySz | String | Amount of repay currency transacted |
| status | String | Current status of one-click repay <br>`running`: Running <br>`filled`: Filled <br>`failed`: Failed |
| uTime | String | Trade time, Unix timestamp format in milliseconds, e.g. 1597026383085 |

### GET / One-click repay currency list (New)

Get list of debt currency data and repay currencies. Only applicable to `SPOT mode`/`Multi-currency margin mode`/`Portfolio margin mode`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/one-click-repay-currency-list-v2`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/one-click-repay-currency-list-v2
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"
flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag,debug=True)
result = tradeAPI.get_oneclick_repay_list_v2()
print(result)
```

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtData": [
                {
                    "debtAmt": "100",
                    "debtCcy": "USDC"
                }
            ],
            "repayData": [
                {
                    "repayAmt": "1.000022977",
                    "repayCcy": "BTC"
                },
                {
                    "repayAmt": "4998.0002397",
                    "repayCcy": "USDT"
                },
                {
                    "repayAmt": "100",
                    "repayCcy": "OKB"
                },
                {
                    "repayAmt": "1",
                    "repayCcy": "ETH"
                },
                {
                    "repayAmt": "100",
                    "repayCcy": "USDC"
                }
            ]
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| debtData | Array of objects | Debt currency data list |
| \> debtCcy | String | Debt currency |
| \> debtAmt | String | Debt currency amount<br>Including principal and interest |
| repayData | Array of objects | Repay currency data list |
| \> repayCcy | String | Repay currency |
| \> repayAmt | String | Repay currency's available balance amount |

### POST / Trade one-click repay (New)

Trade one-click repay to repay debts. Only applicable to `SPOT mode`/`Multi-currency margin mode`/`Portfolio margin mode`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/one-click-repay-v2`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/one-click-repay-v2
body
{
    "debtCcy": "USDC",
    "repayCcyList": ["USDC","BTC"]
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"
flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag,debug=True)
result = tradeAPI.oneclick_repay_v2("USDC",["USDC","BTC"])
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| debtCcy | String | Yes | Debt currency |
| repayCcyList | Array of strings | Yes | Repay currency list, e.g. \["USDC","BTC"\]<br>The priority of currency to repay is consistent with the order in the array. (The first item has the highest priority) |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtCcy": "USDC",
            "repayCcyList": [
                "USDC",
                "BTC"
            ],
            "ts": "1742192217514"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| debtCcy | String | Debt currency |
| repayCcyList | Array of strings | Repay currency list, e.g. \["USDC","BTC"\] |
| ts | String | Request time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / One-click repay history (New)

Get the history and status of one-click repay trades in the past 7 days. Only applicable to `SPOT mode`.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/one-click-repay-history-v2`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/one-click-repay-history-v2
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"
flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)
result = tradeAPI.oneclick_repay_history_v2()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| after | String | No | Pagination of data to return records earlier than (included) the requested time `ts` , Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than (included) the requested time `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "debtCcy": "USDC",
            "fillDebtSz": "9.079631989",
            "ordIdInfo": [
                {
                    "cTime": "1742194485439",
                    "fillPx": "1",
                    "fillSz": "9.088651",
                    "instId": "USDC-USDT",
                    "ordId": "2338478342062235648",
                    "ordType": "ioc",
                    "px": "1.0049",
                    "side": "buy",
                    "state": "filled",
                    "sz": "9.0886514537313433"
                },
                {
                    "cTime": "1742194482326",
                    "fillPx": "83271.9",
                    "fillSz": "0.00010969",
                    "instId": "BTC-USDT",
                    "ordId": "2338478237607288832",
                    "ordType": "ioc",
                    "px": "82856.7",
                    "side": "sell",
                    "state": "filled",
                    "sz": "0.000109696512171"
                }
            ],
            "repayCcyList": [
                "USDC",
                "BTC"
            ],
            "status": "filled",
            "ts": "1742194481852"
        },
        {
            "debtCcy": "USDC",
            "fillDebtSz": "100",
            "ordIdInfo": [],
            "repayCcyList": [
                "USDC",
                "BTC"
            ],
            "status": "filled",
            "ts": "1742192217511"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| debtCcy | String | Debt currency |
| repayCcyList | Array of strings | Repay currency list, e.g. \["USDC","BTC"\] |
| fillDebtSz | String | Amount of debt currency transacted |
| status | String | Current status of one-click repay <br>`running`: Running <br>`filled`: Filled <br>`failed`: Failed |
| ordIdInfo | Array of objects | Order info |
| \> ordId | String | Order ID |
| \> instId | String | Instrument ID, e.g. `BTC-USDT` |
| \> ordType | String | Order type<br>`ioc`: Immediate-or-cancel order |
| \> side | String | Side<br>`buy`<br>`sell` |
| \> px | String | Price |
| \> sz | String | Quantity to buy or sell |
| \> fillPx | String | Last filled price.<br>If none is filled, it will return "". |
| \> fillSz | String | Last filled quantity |
| \> state | String | State<br>`filled`<br>`canceled` |
| \> cTime | String | Creation time for order, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| ts | String | Request time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### POST / Mass cancel order

Cancel all the MMP pending orders of an instrument family.

Only applicable to Option in Portfolio Margin mode, and MMP privilege is required.

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/mass-cancel`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/mass-cancel
body
{
    "instType":"OPTION",
    "instFamily":"BTC-USD"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | Yes | Instrument type<br>`OPTION` |
| instFamily | String | Yes | Instrument family |
| lockInterval | String | No | Lock interval(ms)<br> The range should be \[0, 10 000\]<br> The default is 0. You can set it as "0" if you want to unlock it immediately.<br> Error 54008 will be returned when placing order during lock interval, it is different from 51034 which is thrown when MMP is triggered |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "result":true
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| result | Boolean | Result of the request `true`, `false` |

### POST / Cancel All After

Cancel all pending orders after the countdown timeout. Applicable to all trading symbols through order book (except Spread trading)

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID + tag

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/cancel-all-after`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/cancel-all-after
{
   "timeOut":"60"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Set cancel all after
result = tradeAPI.cancel_all_after(
    timeOut="10"
)

print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| timeOut | String | Yes | The countdown for order cancellation, with second as the unit.<br>Range of value can be 0, \[10, 120\]. <br>Setting timeOut to 0 disables Cancel All After. |
| tag | String | No | CAA order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "triggerTime":"1587971460",
            "tag":"",
            "ts":"1587971400"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| triggerTime | String | The time the cancellation is triggered.<br>triggerTime=0 means Cancel All After is disabled. |
| tag | String | CAA order tag |
| ts | String | The time the request is received. |

Users are recommended to send heartbeat to the exchange every second. When the cancel all after is triggered, the trading engine will cancel orders on behalf of the client one by one and this operation may take up to a few seconds. This feature is intended as a protection mechanism for clients only and clients should not use this feature as part of their trading strategies.

To use tag level CAA, first, users need to set tags for their orders using the \`tag\` request parameter in the placing orders endpoint. When calling the CAA endpoint, if the \`tag\` request parameter is not provided, the default will be to set CAA at the account level. In this case, all pending orders for all order book trading symbols under that sub-account will be cancelled when CAA triggers, consistent with the existing logic. If the \`tag\` request parameter is provided, CAA will be set at the order tag level. When triggered, only pending orders of order book trading symbols with the specified tag will be canceled, while orders with other tags or no tags will remain unaffected.

Users can run a maximum of 20 tag level CAAs simultaneously under the same sub-account. The system will only count live tag level CAAs. CAAs that have been triggered or revoked by the user will not be counted. The user will receive error code 51071 when exceeding the limit.

### GET / Account rate limit

Get account rate limit related information.

Only new order requests and amendment order requests will be counted towards this limit. For batch order requests consisting of multiple orders, each order will be counted individually.

For details, please refer to [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit)

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID

#### HTTP Request

`GET /api/v5/trade/account-rate-limit`

> Request Example

```
Copy to Clipboard
# Get the account rate limit
GET /api/v5/trade/account-rate-limit
```

#### Request Parameters

None

> Response Example

```
Copy to Clipboard
{
   "code":"0",
   "data":[
      {
         "accRateLimit":"2000",
         "fillRatio":"0.1234",
         "mainFillRatio":"0.1234",
         "nextAccRateLimit":"2000",
         "ts":"123456789000"
      }
   ],
   "msg":""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fillRatio | String | Sub account fill ratio during the monitoring period. <br>Applicable to users with trading fee tier >= VIP 5; other users will receive `""`.<br>If there has been no trading activity on the account in the past 7 days, `""` will be returned.<br>If there is no executed volume during the monitoring period, `"0"` will be returned.<br>If there is executed volume but no order operation count during the monitoring period, `"9999"` will be returned. |
| mainFillRatio | String | Master account aggregated fill ratio during the monitoring period. <br>Applicable to users with trading fee tier >= VIP 5; other users will receive `""`.<br>If there has been no trading activity on the account in the past 7 days, `""` will be returned.<br>If there is no executed volume during the monitoring period, `"0"` will be returned. |
| accRateLimit | String | Current sub-account rate limit per 2 seconds |
| nextAccRateLimit | String | Expected sub-account rate limit (per 2 seconds) in the next monitoring period. <br>Applicable to users with trading fee tier >= VIP 5; other users will receive `""`. |
| ts | String | Data update timestamp <br>For users with trading fee tier >= VIP 5, the data will be generated daily at 08:00 am (UTC) <br>For users with trading fee tier < VIP 5, the current timestamp will be returned. |

### POST / Order precheck

This endpoint is used to precheck the account information before and after placing the order.

Only applicable to `Multi-currency margin mode`, and `Portfolio margin mode`.

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/order-precheck`

> Request Example

```
Copy to Clipboard
# place order for SPOT
POST /api/v5/trade/order-precheck
 body
 {
    "instId":"BTC-USDT",
    "tdMode":"cash",
    "clOrdId":"b15",
    "side":"buy",
    "ordType":"limit",
    "px":"2.15",
    "sz":"2"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| tdMode | String | Yes | Trade mode<br>Margin mode `cross``isolated`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading, `tdMode` should be `spot_isolated` for `SPOT` lead trading.) |
| side | String | Yes | Order side, `buy``sell` |
| posSide | String | Conditional | Position side <br>The default is `net` in the `net` mode <br>It is required in the `long/short` mode, and can only be `long` or `short`. <br>Only applicable to `FUTURES`/`SWAP`. |
| ordType | String | Yes | Order type <br>`market`: Market order <br>`limit`: Limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order (applicable only to Expiry Futures and Perpetual Futures). <br>`elp`: Enhanced Liquidity Program order |
| sz | String | Yes | Quantity to buy or sell |
| px | String | Conditional | Order price. Only applicable to `limit`,`post_only`,`fok`,`ioc`,`mmp`,`mmp_and_post_only` order. |
| reduceOnly | Boolean | No | Whether orders can only reduce in position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>Only applicable to `MARGIN` orders, and `FUTURES`/`SWAP` orders in `net` mode <br>Only applicable to `Futures mode` and `Multi-currency margin` |
| tgtCcy | String | No | Whether the target currency uses the quote or base currency.<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| attachAlgoOrds | Array of objects | No | TP/SL information attached when placing order |
| \> attachAlgoClOrdId | String | No | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Conditional | Take-profit trigger price<br>For condition TP order, if you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpOrdPx | String | Conditional | Take-profit order price <br>For condition TP order, if you fill in this parameter, you should fill in the take-profit trigger price as well. <br>For limit TP order, you need to fill in this parameter, take-profit trigger needn‘t to be filled. <br>If the price is -1, take-profit will be executed at the market price. |
| \> tpOrdKind | String | No | TP order kind<br>`condition`<br>`limit`<br> The default is `condition` |
| \> slTriggerPx | String | Conditional | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slOrdPx | String | Conditional | Stop-loss order price<br>If you fill in this parameter, you should fill in the stop-loss trigger price.<br>If the price is -1, stop-loss will be executed at the market price. |
| \> tpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>The default is last |
| \> sz | String | Conditional | Size. Only applicable to TP order of split TPs, and it is required for TP order of split TPs |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "adjEq": "41.94347460746277",
            "adjEqChg": "-226.05616481626",
            "availBal": "0",
            "availBalChg": "0",
            "imr": "0",
            "imrChg": "57.74709688430927",
            "liab": "0",
            "liabChg": "0",
            "liabChgCcy": "",
            "liqPx": "6764.8556232031115",
            "liqPxDiff": "-57693.044376796888536773622035980224609375",
            "liqPxDiffRatio": "-0.8950500152315991",
            "mgnRatio": "0",
            "mgnRatioChg": "0",
            "mmr": "0",
            "mmrChg": "0",
            "posBal": "",
            "posBalChg": "",
            "type": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| adjEq | String | Current adjusted / Effective equity in `USD` |
| adjEqChg | String | After placing order, changed quantity of adjusted / Effective equity in `USD` |
| imr | String | Current initial margin requirement in `USD` |
| imrChg | String | After placing order, changed quantity of initial margin requirement in `USD` |
| mmr | String | Current Maintenance margin requirement in `USD` |
| mmrChg | String | After placing order, changed quantity of maintenance margin requirement in `USD` |
| mgnRatio | String | Current Maintenance margin ratio in `USD` |
| mgnRatioChg | String | After placing order, changed quantity of Maintenance margin ratio in `USD` |
| availBal | String | Current available balance in margin coin currency, only applicable to turn auto borrow off |
| availBalChg | String | After placing order, changed quantity of available balance after placing order, only applicable to turn auto borrow off |
| liqPx | String | Current estimated liquidation price |
| liqPxDiff | String | After placing order, the distance between estimated liquidation price and mark price |
| liqPxDiffRatio | String | After placing order, the distance rate between estimated liquidation price and mark price |
| posBal | String | Current positive asset, only applicable to margin isolated position |
| posBalChg | String | After placing order, positive asset of margin isolated, only applicable to margin isolated position |
| liab | String | Current liabilities of currency<br> For cross, it is cross liabilities<br>For isolated position, it is isolated liabilities |
| liabChg | String | After placing order, changed quantity of liabilities<br> For cross, it is cross liabilities<br>For isolated position, it is isolated liabilities |
| liabChgCcy | String | After placing order, the unit of changed liabilities quantity<br> only applicable cross and in auto borrow |
| type | String | Unit type of positive asset, only applicable to margin isolated position<br>`1`: it is both base currency before and after placing order <br>`2`: before plaing order, it is base currency. after placing order, it is quota currency.<br>`3`: before plaing order, it is quota currency. after placing order, it is base currency<br>`4`: it is both quota currency before and after placing order |

### WS / Order channel

Retrieve order information. Data will not be pushed when first subscribed. Data will only be pushed when there are new orders or order updates.

Concurrent connection to this channel will be restricted by the following rules: [WebSocket connection count limit](https://www.okx.com/docs-v5/en/#overview-websocket-connection-count-limit).

#### URL Path

/ws/v5/private (required login)

> Request Example : single

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "orders",
      "instType": "FUTURES",
      "instId": "BTC-USD-200329"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/private",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
          "channel": "orders",
          "instType": "FUTURES",
          "instId": "BTC-USD-200329"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "orders",
      "instType": "FUTURES",
      "instFamily": "BTC-USD"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/private",
        useServerTime=False
    )
    await ws.start()
    args =[
        {
          "channel": "orders",
          "instType": "FUTURES",
          "instFamily": "BTC-USD"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`orders` |
| \> instType | String | Yes | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION`<br>`ANY` |
| \> instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| \> instId | String | No | Instrument ID |

> Successful Response Example : single

```
Copy to Clipboard
{
  "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "orders",
        "instType": "FUTURES",
        "instId": "BTC-USD-200329"
    },
    "connId": "a4d3ae55"
}
```

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "orders",
    "instType": "FUTURES",
    "instFamily": "BTC-USD"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"orders\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type<br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`OPTION`<br>`ANY` |
| \> instFamily | String | No | Instrument family |
| \> instId | String | No | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
    "arg": {
        "channel": "orders",
        "instType": "SPOT",
        "instId": "BTC-USDT",
        "uid": "614488474791936"
    },
    "data": [
        {
            "accFillSz": "0.001",
            "algoClOrdId": "",
            "algoId": "",
            "amendResult": "",
            "amendSource": "",
            "avgPx": "31527.1",
            "cancelSource": "",
            "category": "normal",
            "ccy": "",
            "clOrdId": "",
            "code": "0",
            "cTime": "1654084334977",
            "execType": "M",
            "fee": "-0.02522168",
            "feeCcy": "USDT",
            "fillFee": "-0.02522168",
            "fillFeeCcy": "USDT",
            "fillNotionalUsd": "31.50818374",
            "fillPx": "31527.1",
            "fillSz": "0.001",
            "fillPnl": "0.01",
            "fillTime": "1654084353263",
            "fillPxVol": "",
            "fillPxUsd": "",
            "fillMarkVol": "",
            "fillFwdPx": "",
            "fillMarkPx": "",
            "fillIdxPx": "",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "lever": "0",
            "msg": "",
            "notionalUsd": "31.50818374",
            "ordId": "452197707845865472",
            "ordType": "limit",
            "pnl": "0",
            "posSide": "",
            "px": "31527.1",
            "pxUsd":"",
            "pxVol":"",
            "pxType":"",
            "quickMgnType": "",
            "rebate": "0",
            "rebateCcy": "BTC",
            "reduceOnly": "false",
            "reqId": "",
            "side": "sell",
            "attachAlgoClOrdId": "",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "last",
            "source": "",
            "state": "filled",
            "stpId": "",
            "stpMode": "",
            "sz": "0.001",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "tpTriggerPxType": "last",
            "attachAlgoOrds": [],
            "tradeId": "242589207",
            "tradeQuoteCcy": "USDT",
            "lastPx": "38892.2",
            "uTime": "1654084353264",
            "isTpLimit": "false",
            "linkedAlgoOrd": {
                "algoId": ""
            }
        }
    ]
}
```

#### Push data parameters

| Parameter | Type | Description |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> instType | String | Instrument type |
| \> instFamily | String | Instrument family |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market orders. <br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| \> ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tag | String | Order tag |
| \> px | String | Price <br>For options, use coin as unit (e.g. BTC, ETH) |
| \> pxUsd | String | Options price in USDOnly applicable to options; return "" for other instrument types |
| \> pxVol | String | Implied volatility of the options orderOnly applicable to options; return "" for other instrument types |
| \> pxType | String | Price type of options <br>`px`: Place an order based on price, in the unit of coin (the unit for the request parameter px is BTC or ETH) <br>`pxVol`: Place an order based on pxVol <br>`pxUsd`: Place an order based on pxUsd, in the unit of USD (the unit for the request parameter px is USD) |
| \> sz | String | The original order quantity, `SPOT`/`MARGIN`, in the unit of currency; `FUTURES`/`SWAP`/`OPTION`, in the unit of contract |
| \> notionalUsd | String | Estimated national value in `USD` of order |
| \> ordType | String | Order type <br>`market`: market order <br>`limit`: limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order (applicable only to Expiry Futures and Perpetual Futures)<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode). <br>`op_fok`: Simple options (fok) <br>`elp`: Enhanced Liquidity Program order |
| \> side | String | Order side, `buy``sell` |
| \> posSide | String | Position side <br>`net`<br>`long` or `short` Only applicable to `FUTURES`/`SWAP` |
| \> tdMode | String | Trade mode, `cross`: cross `isolated`: isolated `cash`: cash |
| \> fillPx | String | Filled price for the current update. |
| \> tradeId | String | Trade ID for the current update. |
| \> fillSz | String | Filled quantity for the current udpate. <br>The unit is `base_ccy` for SPOT and MARGIN, e.g. BTC-USDT, the unit is BTC; For market orders, the unit both is `base_ccy` when the tgtCcy is `base_ccy` or `quote_ccy`;<br>The unit is contract for `FUTURES`/`SWAP`/`OPTION` |
| \> fillPnl | String | Filled profit and loss for the current udpate, applicable to orders which have a trade and aim to close position. It always is 0 in other conditions |
| \> fillTime | String | Filled time for the current udpate. |
| \> fillFee | String | Filled fee amount or rebate amount for the current udpate. : <br>Negative number represents the user transaction fee charged by the platform; <br>Positive number represents rebate |
| \> fillFeeCcy | String | Filled fee currency or rebate currency for the current udpate..<br>It is fee currency when fillFee is less than 0; It is rebate currency when fillFee>=0. |
| \> fillPxVol | String | Implied volatility when filled <br>Only applicable to options; return "" for other instrument types |
| \> fillPxUsd | String | Options price when filled, in the unit of USD <br>Only applicable to options; return "" for other instrument types |
| \> fillMarkVol | String | Mark volatility when filled <br>Only applicable to options; return "" for other instrument types |
| \> fillFwdPx | String | Forward price when filled <br>Only applicable to options; return "" for other instrument types |
| \> fillMarkPx | String | Mark price when filled <br>Applicable to `FUTURES`, `SWAP`, `OPTION` |
| \> fillIdxPx | String | Index price at the moment of trade execution <br>For cross currency spot pairs, it returns baseCcy-USDT index price. For example, for LTC-ETH, this field returns the index price of LTC-USDT. |
| \> execType | String | Liquidity taker or maker for the current update, T: taker M: maker |
| \> accFillSz | String | Accumulated fill quantity<br>The unit is `base_ccy` for SPOT and MARGIN, e.g. BTC-USDT, the unit is BTC; For market orders, the unit both is `base_ccy` when the tgtCcy is `base_ccy` or `quote_ccy`;<br>The unit is contract for `FUTURES`/`SWAP`/`OPTION` |
| \> fillNotionalUsd | String | Filled notional value in `USD` of order |
| \> avgPx | String | Average filled price. If none is filled, it will return `0`. |
| \> state | String | Order state <br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`mmp_canceled` |
| \> lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL. |
| \> tpTriggerPx | String | Take-profit trigger price, it |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price, it |
| \> slTriggerPx | String | Stop-loss trigger price, it |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price, it |
| \> attachAlgoOrds | Array of objects | TP/SL information attached when placing order |
| >\> attachAlgoId | String | The order ID of attached TP/SL order. It can be used to identity the TP/SL order when amending. It will not be posted to algoId when placing TP/SL order after the general order is filled completely. |
| >\> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to `algoClOrdId` when placing TP/SL order once the general order is filled completely. |
| >\> tpOrdKind | String | TP order kind<br>`condition`<br>`limit` |
| >\> tpTriggerPx | String | Take-profit trigger price. |
| >\> tpTriggerRatio | String | Take-profit trigger ratio, 0.3 represents 30%. Only applicable to `FUTURES`/`SWAP` contracts. |
| >\> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| >\> tpOrdPx | String | Take-profit order price. |
| >\> slTriggerPx | String | Stop-loss trigger price. |
| >\> slTriggerRatio | String | Stop-loss trigger ratio, 0.3 represents 30%. Only applicable to `FUTURES`/`SWAP` contracts. |
| >\> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| >\> slOrdPx | String | Stop-loss order price. |
| >\> sz | String | Size. Only applicable to TP order of split TPs |
| >\> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> linkedAlgoOrd | Object | Linked SL order detail, only applicable to TP limit order of one-cancels-the-other order(oco) |
| >\> algoId | Object | Algo ID |
| \> stpId | String | ~~Self trade prevention ID<br>Return "" if self trade prevention is not applicable~~ (deprecated) |
| \> stpMode | String | Self trade prevention mode |
| \> feeCcy | String | Fee currency<br>For maker sell orders of Spot and Margin, this represents the quote currency. For all other cases, it represents the currency in which fees are charged. |
| \> fee | String | Fee amount<br>For Spot and Margin (excluding maker sell orders): accumulated fee charged by the platform, always negative<br>For maker sell orders in Spot and Margin, Expiry Futures, Perpetual Futures and Options: accumulated fee and rebate (always in quote currency for maker sell orders in Spot and Margin) |
| \> rebateCcy | String | Rebate currency<br>For maker sell orders of Spot and Margin, this represents the base currency. For all other cases, it represents the currency in which rebates are paid. |
| \> rebate | String | Rebate amount, only applicable to Spot and Margin<br>For maker sell orders: ~~Accumulated fee and~~ rebate amount in the unit of base currency.<br>For all other cases, it represents the maker rebate amount, always positive, return "" if no rebate. |
| \> pnl | String | Profit and loss (excluding the fee).<br> applicable to orders which have a trade and aim to close position. It always is 0 in other conditions. <br>For liquidation under cross margin mode, it will include liquidation penalties. |
| \> source | String | Order source<br>`6`: The normal order triggered by the `trigger order`<br>`7`:The normal order triggered by the `TP/SL order`<br>`13`: The normal order triggered by the algo order<br>`25`:The normal order triggered by the `trailing stop order`<br>`34`: The normal order triggered by the chase order |
| \> cancelSource | String | Source of the order cancellation.<br>Valid values and the corresponding meanings are:<br>`0`: Order canceled by system<br>`1`: Order canceled by user<br>`2`: Order canceled: Pre reduce-only order canceled, due to insufficient margin in user position<br>`3`: Order canceled: Risk cancellation was triggered. Pending order was canceled due to insufficient maintenance margin ratio and forced-liquidation risk.<br>`4`: Order canceled: Borrowings of crypto reached hard cap, order was canceled by system.<br>`6`: Order canceled: ADL order cancellation was triggered. Pending order was canceled due to a low margin ratio and forced-liquidation risk. <br>`7`: Order canceled: Futures contract delivery. <br>`9`: Order canceled: Insufficient balance after funding fees deducted. <br>`10`: Order canceled: Option contract expiration.<br>`13`: Order canceled: FOK order was canceled due to incompletely filled.<br>`14`: Order canceled: IOC order was partially canceled due to incompletely filled.<br>`15`: Order canceled: The order price is beyond the limit<br>`17`: Order canceled: Close order was canceled, due to the position was already closed at market price.<br>`20`: Cancel all after triggered<br>`21`: Order canceled: The TP/SL order was canceled because the position had been closed<br>`22` Order canceled: Due to a better price was available for the order in the same direction, the current operation reduce-only order was automatically canceled<br>`23` Order canceled: Due to a better price was available for the order in the same direction, the existing reduce-only order was automatically canceled<br>`27`: Order canceled: Price limit verification failed because the price difference between counterparties exceeds 5% <br>`31`: The post-only order will take liquidity in taker orders <br>`32`: Self trade prevention <br>`33`: The order exceeds the maximum number of order matches per taker order<br>`36`: Your TP limit order was canceled because the corresponding SL order was triggered. <br>`37`: Your TP limit order was canceled because the corresponding SL order was canceled.<br>`38`: You have canceled market maker protection (MMP) orders.<br>`39`: Your order was canceled because market maker protection (MMP) was triggered. <br>`42`: Your order was canceled because the difference between the initial and current best bid or ask prices reached the maximum chase difference.<br>`43`: Order cancelled because the buy order price is higher than the index price or the sell order price is lower than the index price.<br>`44`: Your order was canceled because your available balance of this crypto was insufficient for auto conversion. Auto conversion was triggered when the total collateralized liabilities for this crypto reached the platform’s risk control limit. <br>`45`: Order cancelled because ELP order price verification failed<br>`46`: delta reducing cancel orders |
| \> amendSource | String | Source of the order amendation. <br>`1`: Order amended by user<br>`2`: Order amended by user, but the order quantity is overriden by system due to reduce-only<br>`3`: New order placed by user, but the order quantity is overriden by system due to reduce-only<br>`4`: Order amended by system due to other pending orders<br>`5`: Order modification due to changes in options px, pxVol, or pxUsd as a result of following variations. For example, when iv = 60, USD and px are anchored at iv = 60, the changes in USD or px lead to modification. |
| \> category | String | Category <br>`normal`<br>`twap`<br>`adl`<br>`full_liquidation`<br>`partial_liquidation`<br>`delivery`<br>`ddh`: Delta dynamic hedge<br>`auto_conversion` |
| \> isTpLimit | String | Whether it is TP limit order. true or false |
| \> uTime | String | Update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reqId | String | Client Request ID as assigned by the client for order amendment. "" will be returned if there is no order amendment. |
| \> amendResult | String | The result of amending the order <br>`-1`: failure <br>`0`: success <br>`1`: Automatic cancel (amendment request returned success but amendment subsequently failed then automatically canceled by the system) <br>`2`: Automatic amendation successfully, only applicable to pxVol and pxUsd orders of Option.<br>When amending the order through API and `cxlOnFail` is set to `true` in the order amendment request but the amendment is rejected, "" is returned. <br> When amending the order through API, the order amendment acknowledgement returns success and the amendment subsequently failed, `-1` will be returned if `cxlOnFail` is set to `false`, `1` will be returned if `cxlOnFail` is set to `true`. <br>When amending the order through Web/APP and the amendment failed, `-1` will be returned. |
| \> reduceOnly | String | Whether the order can only reduce the position size. Valid options: `true` or `false`. |
| \> quickMgnType | String | Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay` |
| \> algoClOrdId | String | Client-supplied Algo ID. There will be a value when algo order attaching `algoClOrdId` is triggered, or it will be "". |
| \> algoId | String | Algo ID. There will be a value when algo order is triggered, or it will be "". |
| \> lastPx | String | Last price |
| \> code | String | Error Code, the default is 0 |
| \> msg | String | Error Message, The default is "" |
| \> tradeQuoteCcy | String | The quote currency used for trading. |

For market orders, it's likely the orders channel will show order state as "filled" while showing the "last filled quantity (fillSz)" as 0.

In exceptional cases, the same message may be sent multiple times (perhaps with the different uTime) . The following guidelines are advised:

1\. If a \`tradeId\` is present, it means a fill. Each \`tradeId\` should only be returned once per instrument ID, and the later messages that have the same \`tradeId\` should be discarded.

2\. If \`tradeId\` is absent and the \`state\` is "filled," it means that the \`SPOT\`/\`MARGIN\` market order is fully filled. For messages with the same \`ordId\`, process only the first filled message and discard any subsequent messages. State = filled is the terminal state of an order.

3\. If the state is \`canceled\` or \`mmp\_canceled\`, it indicates that the order has been canceled. For cancellation messages with the same \`ordId\`, process the first one and discard later messages. State = canceled / mmp\_canceled is the terminal state of an order.

4\. If \`reqId\` is present, it indicates a response to a user-requested order modification. It is recommended to use a unique \`reqId\` for each modification request. For modification messages with the same \`reqId\`, process only the first message received and discard subsequent messages.

The definitions for fillPx, tradeId, fillSz, fillPnl, fillTime, fillFee, fillFeeCcy, and execType differ between the REST order information endpoints and the orders channel.

The definitions for fillPx, tradeId, fillSz, fillPnl, fillTime, fillFee, fillFeeCcy, and execType differ between the REST order information endpoints and the orders channel.

Unlike futures contracts, option positions are automatically exercised or expire at maturity. The rights then terminate and no closing orders are generated. Therefore, this channel will not push any closing-order updates for expired options.

### WS / Fills channel

Retrieve transaction information. Data will not be pushed when first subscribed. Data will only be pushed when there are order book fill events, where tradeId > 0.

The channel is exclusively available to users with trading fee tier VIP6 or above. For other users, please use [WS / Order channel](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-order-channel).

#### URL Path

/ws/v5/private (required login)

> Request Example: single

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [
        {
            "channel": "fills",
            "instId": "BTC-USDT-SWAP"
        }
    ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/private",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
            "channel": "fills",
            "instId": "BTC-USDT-SWAP"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [
        {
            "channel": "fills"
        }
    ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/private",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
            "channel": "fills"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe``unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name `fills` |
| \> instId | String | No | Instrument ID |

> Successful Response Example: single

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "fills",
    "instId": "BTC-USDT-SWAP"
  },
  "connId": "a4d3ae55"
}
```

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "fills"
  },
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe``unsubscribe``error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instId | String | No | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example: single

```
Copy to Clipboard
{
    "arg": {
        "channel": "fills",
        "instId": "BTC-USDT-SWAP",
        "uid": "614488474791111"
    },
    "data":[
        {
            "instId": "BTC-USDT-SWAP",
            "fillSz": "100",
            "fillPx": "70000",
            "side": "buy",
            "ts": "1705449605015",
            "ordId": "680800019749904384",
            "clOrdId": "1234567890",
            "tradeId": "12345",
            "execType": "T",
            "count": "10"
        }
    ]
}
```

#### Push data parameters

| Parameter | Type | Description |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instId | String | Instrument ID |
| \> fillSz | String | Filled quantity. If the trade is aggregated, the filled quantity will also be aggregated. |
| \> fillPx | String | Last filled price |
| \> side | String | Trade direction<br>`buy``sell` |
| \> ts | String | Filled time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tradeId | String | The last trade ID in the trades aggregation |
| \> execType | String | Liquidity taker or maker, `T`: taker `M`: maker |
| \> count | String | The count of trades aggregated |

\- The channel is exclusively available to users with trading fee tier VIP6 or above. Others will receive error code 60029 when subscribing to it.

\- The channel only pushes partial information of the orders channel. Fill events of block trading, nitro spread, liquidation, ADL, and some other non order book events will not be pushed through this channel. Users should also subscribe to the orders channel for order confirmation.

\- When a fill event is received by this channel, the account balance, margin, and position information might not have changed yet.

\- Taker orders will be aggregated based on different fill prices. When aggregation occurs, the count field indicates the number of orders matched, and the tradeId represents the tradeId of the last trade in the aggregation. Maker orders will not be aggregated.

\- The channel returns clOrdId. The field will be returned upon trade execution. Note that the fills channel will only return this field if the user-provided clOrdId conforms to the signed int64 positive integer format (1-9223372036854775807, 2^63-1); if the user does not provide this field or if clOrdId does not meet the format requirements, the field will return "0". The order endpoints and channel will continue to return the user-provided clOrdId as usual. All request and response parameters are of string type.

\- In the future, connection limits will be imposed on this channel. The maximum number of connections subscribing to this channel per subaccount will be 20. We recommend users always use this channel within this limit to avoid any impact on their strategies when the limit is enforced.

### WS / Place order

You can place an order only if you have sufficient funds.

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 60 requests per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Rate limit is shared with the \`Place order\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "order",
  "args": [
    {
      "side": "buy",
      "instId": "BTC-USDT",
      "tdMode": "isolated",
      "ordType": "market",
      "sz": "100"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`order` |
| args | Array of objects | Yes | Request parameters |
| \> instIdCode | Integer | 是 | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> tdMode | String | Yes | Trade mode <br>Margin mode `isolated``cross`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading, `tdMode` should be `spot_isolated` for `SPOT` lead trading.) |
| \> ccy | String | No | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`. |
| \> clOrdId | String | No | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| \> tag | String | No | Order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| \> side | String | Yes | Order side, `buy``sell` |
| \> posSide | String | Conditional | Position side <br>The default is `net` in the `net` mode <br>It is required in the `long/short` mode, and can only be `long` or `short`. <br>Only applicable to `FUTURES`/`SWAP`. |
| \> ordType | String | Yes | Order type <br>`market`: Market order, only applicable to `SPOT/MARGIN/FUTURES/SWAP`<br>`limit`: limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode)<br>`elp`: Enhanced Liquidity Program order |
| \> sz | String | Yes | Quantity to buy or sell. |
| \> px | String | Conditional | Order price. Only applicable to `limit`,`post_only`,`fok`,`ioc`,`mmp`,`mmp_and_post_only` order.<br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> pxUsd | String | Conditional | Place options orders in `USD`<br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> pxVol | String | Conditional | Place options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> reduceOnly | Boolean | No | Whether the order can only reduce the position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>Only applicable to `MARGIN` orders, and `FUTURES`/`SWAP` orders in `net` mode <br>Only applicable to `Futures mode` and `Multi-currency margin` |
| \> tgtCcy | String | No | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| \> banAmend | Boolean | No | Whether to disallow the system from amending the size of the SPOT Market Order.<br>Valid options: `true` or `false`. The default value is `false`.<br>If `true`, system will not amend and reject the market order if user does not have sufficient funds. <br>Only applicable to SPOT Market Orders |
| \> pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `px` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `px` exceeds the price limit<br> The default value is `0` |
| \> tradeQuoteCcy | String | No | The quote currency used for trading. Only applicable to `SPOT`. <br> The default value is the quote currency of the `instId`, for example: for `BTC-USD`, the default is `USD`. |
| \> stpMode | String | No | Self trade prevention mode. <br>`cancel_maker`,`cancel_taker`, `cancel_both`.<br>Cancel both does not support FOK <br>The account-level acctStpMode will be used to place orders. The default value of this field is `cancel_maker`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order. |
| \> isElpTakerAccess | Boolean | No | ELP taker access<br>`true`: the request can trade with ELP orders but a speed bump will be applied<br>`false`: the request cannot trade with ELP orders and no speed bump<br>The default value is `false` while `true` is only applicable to ioc orders. |
| expTime | String | No | Request effective deadline. Unix timestamp format in milliseconds, e.g. `1597026383085` |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "12345689",
      "tag": "",
      "ts":"1695190491421",
      "sCode": "0",
      "sMsg": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "",
      "tag": "",
      "ts":"1695190491421",
      "sCode": "5XXXX",
      "sMsg": "not exist"
    }
  ],
  "code": "1",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1512",
  "op": "order",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tag | String | Order tag |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Rejection or success message of event execution. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

tdMode

Trade Mode, when placing an order, you need to specify the trade mode.

**Spot mode:**

\- SPOT and OPTION buyer: cash

**Futures mode:**

\- Isolated MARGIN: isolated

\- Cross MARGIN: cross

\- SPOT: cash

\- Cross FUTURES/SWAP/OPTION: cross

\- Isolated FUTURES/SWAP/OPTION: isolated

**Multi-currency margin:**

\- Isolated MARGIN: isolated

\- Cross SPOT: cross

\- Cross FUTURES/SWAP/OPTION: cross

\- Isolated FUTURES/SWAP/OPTION: isolated

**Portfolio margin:**

\- Isolated MARGIN: isolated

\- Cross SPOT: cross

\- Cross FUTURES/SWAP/OPTION: cross

\- Isolated FUTURES/SWAP/OPTION: isolated

clOrdId

clOrdId is a user-defined unique ID used to identify the order. It will be included in the response parameters if you have specified during order submission, and can be used as a request parameter to the endpoints to query, cancel and amend orders.

clOrdId must be unique among the clOrdIds of all pending orders.

posSide

Position side, this parameter is not mandatory in **net** mode. If you pass it through, the only valid value is **net**.

In **long/short** mode, it is mandatory. Valid values are **long** or **short**.

In **long/short** mode, **side** and **posSide** need to be specified in the combinations below:

Open long: buy and open long (side: fill in buy; posSide: fill in long)

Open short: sell and open short (side: fill in sell; posSide: fill in short)

Close long: sell and close long (side: fill in sell; posSide: fill in long)

Close short: buy and close short (side: fill in buy; posSide: fill in short)

Portfolio margin mode: Expiry Futures and Perpetual Futures only support net mode

ordType

Order type. When creating a new order, you must specify the order type. The order type you specify will affect: 1) what order parameters are required, and 2) how the matching system executes your order. The following are valid order types:

limit: Limit order, which requires specified sz and px.

market: Market order. For SPOT and MARGIN, market order will be filled with market price (by swiping opposite order book). For Expiry Futures and Perpetual Futures, market order will be placed to order book with most aggressive price allowed by Price Limit Mechanism. For OPTION, market order is not supported yet. As the filled price for market orders cannot be determined in advance, OKX reserves/freezes your quote currency by an additional 5% for risk check.

post\_only: Post-only order, which the order can only provide liquidity to the market and be a maker. If the order would have executed on placement, it will be canceled instead.

fok: Fill or kill order. If the order cannot be fully filled, the order will be canceled. The order would not be partially filled.

ioc: Immediate or cancel order. Immediately execute the transaction at the order price, cancel the remaining unfilled quantity of the order, and the order quantity will not be displayed in the order book.

optimal\_limit\_ioc: Market order with ioc (immediate or cancel). Immediately execute the transaction of this market order, cancel the remaining unfilled quantity of the order, and the order quantity will not be displayed in the order book. Only applicable to Expiry Futures and Perpetual Futures.

sz

Quantity to buy or sell.

For SPOT/MARGIN Buy and Sell Limit Orders, it refers to the quantity in base currency.

For MARGIN Buy Market Orders, it refers to the quantity in quote currency.

For MARGIN Sell Market Orders, it refers to the quantity in base currency.

For SPOT Market Orders, it is set by tgtCcy.

For FUTURES/SWAP/OPTION orders, it refers to the number of contracts.

reduceOnly

When placing an order with this parameter set to true, it means that the order will reduce the size of the position only

For the same MARGIN instrument, the coin quantity of all reverse direction pending orders adds \`sz\` of new \`reduceOnly\` order cannot exceed the position assets. After the debt is paid off, if there is a remaining size of orders, the position will not be opened in reverse, but will be traded in SPOT.

For the same FUTURES/SWAP instrument, the sum of the current order size and all reverse direction reduce-only pending orders which's price-time priority is higher than the current order, cannot exceed the contract quantity of position.

Only applicable to \`Futures mode\` and \`Multi-currency margin\`

Only applicable to \`MARGIN\` orders, and \`FUTURES\`/\`SWAP\` orders in \`net\` mode

Notice: Under long/short mode of Expiry Futures and Perpetual Futures, all closing orders apply the reduce-only feature which is not affected by this parameter.

tgtCcy

This parameter is used to specify the order quantity in the order request is denominated in the quantity of base or quote currency. This is applicable to SPOT Market Orders only.

Base currency: base\_ccy

Quote currency: quote\_ccy

If you use the Base Currency quantity for buy market orders or the Quote Currency for sell market orders, please note:

1\. If the quantity you enter is greater than what you can buy or sell, the system will execute the order according to your maximum buyable or sellable quantity. If you want to trade according to the specified quantity, you should use Limit orders.

2\. When the market price is too volatile, the locked balance may not be sufficient to buy the Base Currency quantity or sell to receive the Quote Currency that you specified. We will change the quantity of the order to execute the order based on best effort principle based on your account balance. In addition, we will try to over lock a fraction of your balance to avoid changing the order quantity.

2.1 Example of base currency buy market order:

Taking the market order to buy 10 LTCs as an example, and the user can buy 11 LTC. At this time, if 10 < 11, the order is accepted. When the LTC-USDT market price is 200, and the locked balance of the user is 3,000 USDT, as 200\*10 < 3,000, the market order of 10 LTC is fully executed;
If the market is too volatile and the LTC-USDT market price becomes 400, 400\*10 > 3,000, the user's locked balance is not sufficient to buy using the specified amount of base currency, the user's maximum locked balance of 3,000 USDT will be used to settle the trade. Final transaction quantity becomes 3,000/400 = 7.5 LTC.

2.2 Example of quote currency sell market order:

Taking the market order to sell 1,000 USDT as an example, and the user can sell 1,200 USDT, 1,000 < 1,200, the order is accepted. When the LTC-USDT market price is 200, and the locked balance of the user is 6 LTC, as 1,000/200 < 6, the market order of 1,000 USDT is fully executed;
If the market is too volatile and the LTC-USDT market price becomes 100, 100\*6 < 1,000, the user's locked balance is not sufficient to sell using the specified amount of quote currency, the user's maximum locked balance of 6 LTC will be used to settle the trade. Final transaction quantity becomes 6 \* 100 = 600 USDT.

px

The value for px must be a multiple of tickSz for OPTION orders.

If not, the system will apply the rounding rules below. Using tickSz 0.0005 as an example:

The px will be rounded up to the nearest 0.0005 when the remainder of px to 0.0005 is more than 0.00025 or \`px\` is less than 0.0005.

The px will be rounded down to the nearest 0.0005 when the remainder of px to 0.0005 is less than 0.00025 and \`px\` is more than 0.0005.

Mandatory self trade prevention (STP)

The trading platform imposes mandatory self trade prevention at master account level, which means the accounts under the same master account, including master account itself and all its affiliated sub-accounts, will be prevented from self trade. The account-level acctStpMode will be used to place orders by default. The default value of this field is \`cancel\_maker\`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order.

Mandatory self trade prevention will not lead to latency.

There are three STP modes. The STP mode is always taken based on the configuration in the taker order.

1\. Cancel Maker: This is the default STP mode, which cancels the maker order to prevent self-trading. Then, the taker order continues to match with the next order based on the order book priority.

2\. Cancel Taker: The taker order is canceled to prevent self-trading. If the user's own maker order is lower in the order book priority, the taker order is partially filled and then canceled. FOK orders are always honored and canceled if they would result in self-trading.

3\. Cancel Both: Both taker and maker orders are canceled to prevent self-trading. If the user's own maker order is lower in the order book priority, the taker order is partially filled. Then, the remaining quantity of the taker order and the first maker order are canceled. FOK orders are not supported in this mode.

Rate limit of orders tagged as isElpTakerAccess:true

\- 50 orders per 2 seconds per User ID per instrument ID.

\- This rate limit is shared in Place order/Place multiple orders endpoints in REST/WebSocket

### WS / Place multiple orders

Place orders in a batch. Maximum 20 orders can be placed per request

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 300 orders per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Place order\`.

Rate limit is shared with the \`Place multiple orders\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-orders",
  "args": [
    {
      "side": "buy",
      "instId": "BTC-USDT",
      "tdMode": "cash",
      "ordType": "market",
      "sz": "100"
    },
    {
      "side": "buy",
      "instId": "LTC-USDT",
      "tdMode": "cash",
      "ordType": "market",
      "sz": "1"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`batch-orders` |
| args | Array of objects | Yes | Request Parameters |
| \> instIdCode | Integer | Yes | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> tdMode | String | Yes | Trade mode <br>Margin mode `isolated``cross`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading, `tdMode` should be `spot_isolated` for `SPOT` lead trading.)<br>Note: `isolated` is not available in multi-currency margin mode and portfolio margin mode. |
| \> ccy | String | No | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`. |
| \> clOrdId | String | No | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| \> tag | String | No | Order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| \> side | String | Yes | Order side, `buy``sell` |
| \> posSide | String | Conditional | Position side <br>The default `net` in the `net` mode <br>It is required in the `long/short` mode, and only be `long` or `short`. <br>Only applicable to `FUTURES`/`SWAP`. |
| \> ordType | String | Yes | Order type <br>`market`: Market order, only applicable to `SPOT/MARGIN/FUTURES/SWAP`<br>`limit`: limit order <br>`post_only`: Post-only order <br>`fok`: Fill-or-kill order <br>`ioc`: Immediate-or-cancel order <br>`optimal_limit_ioc`: Market order with immediate-or-cancel order (applicable only to Expiry Futures and Perpetual Futures)<br>`mmp`: Market Maker Protection (only applicable to Option in Portfolio Margin mode)<br>`mmp_and_post_only`: Market Maker Protection and Post-only order(only applicable to Option in Portfolio Margin mode). <br>`elp`: Enhanced Liquidity Program order |
| \> sz | String | Yes | Quantity to buy or sell. |
| \> px | String | Conditional | Order price. Only applicable to `limit`,`post_only`,`fok`,`ioc`,`mmp`,`mmp_and_post_only` order.<br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> pxUsd | String | Conditional | Place options orders in `USD`<br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> pxVol | String | Conditional | Place options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options <br>When placing an option order, one of px/pxUsd/pxVol must be filled in, and only one can be filled in |
| \> reduceOnly | Boolean | No | Whether the order can only reduce the position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>Only applicable to `MARGIN` orders, and `FUTURES`/`SWAP` orders in `net` mode <br>Only applicable to `Futures mode` and `Multi-currency margin` |
| \> tgtCcy | String | No | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| \> banAmend | Boolean | No | Whether to disallow the system from amending the size of the SPOT Market Order.<br>Valid options: `true` or `false`. The default value is `false`.<br>If `true`, system will not amend and reject the market order if user does not have sufficient funds. <br>Only applicable to SPOT Market Orders |
| \> pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `px` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `px` exceeds the price limit<br> The default value is `0` |
| \> tradeQuoteCcy | String | No | The quote currency used for trading. Only applicable to `SPOT`. <br> The default value is the quote currency of the `instId`, for example: for `BTC-USD`, the default is `USD`. |
| \> isElpTakerAccess | Boolean | No | ELP taker access<br>`true`: the request can trade with ELP orders but a speed bump will be applied<br>`false`: the request cannot trade with ELP orders and no speed bump<br>The default value is `false` while `true` is only applicable to ioc orders. |
| \> stpMode | String | No | Self trade prevention mode. <br>`cancel_maker`,`cancel_taker`, `cancel_both`<br>Cancel both does not support FOK. <br>The account-level acctStpMode will be used to place orders by default. The default value of this field is `cancel_maker`. Users can log in to the webpage through the master account to modify this configuration. Users can also utilize the stpMode request parameter of the placing order endpoint to determine the stpMode of a certain order. |
| expTime | String | No | Request effective deadline. Unix timestamp format in milliseconds, e.g. `1597026383085` |

> Response Example When All Succeed

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-orders",
  "data": [
    {
      "clOrdId": "",
      "ordId": "12345689",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    },
    {
      "clOrdId": "",
      "ordId": "12344",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Partially Successful

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-orders",
  "data": [
    {
      "clOrdId": "",
      "ordId": "12345689",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    },
    {
      "clOrdId": "",
      "ordId": "",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "51008",
      "sMsg": "Order failed. Insufficient USDT balance in account",
      "subCode": "1000"
    }
  ],
  "code": "2",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When All Failed

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-orders",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "51008",
      "sMsg": "Order failed. Insufficient USDT balance in account",
      "subCode": "1000"
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "",
      "tag": "",
      "ts": "1695190491421",
      "sCode": "51008",
      "sMsg": "Order failed. Insufficient USDT balance in account",
      "subCode": "1000"
    }
  ],
  "code": "1",
  "msg": "",
  "subCode": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-orders",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> tag | String | Order tag |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Rejection or success message of event execution. |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

In the \`Portfolio Margin\` account mode, either all orders are accepted by the system successfully, or all orders are rejected by the system.

clOrdId

clOrdId is a user-defined unique ID used to identify the order. It will be included in the response parameters if you have specified during order submission, and can be used as a request parameter to the endpoints to query, cancel and amend orders.

clOrdId must be unique among all pending orders and the current request.

Rate limit of orders tagged as isElpTakerAccess:true

\- 50 orders per 2 seconds per User ID per instrument ID.

\- This rate limit is shared in Place order/Place multiple orders endpoints in REST/WebSocket

### WS / Cancel order

Cancel an incomplete order

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 60 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Rate limit is shared with the \`Cancel order\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1514",
  "op": "cancel-order",
  "args": [
    {
      "instId": "BTC-USDT",
      "ordId": "2510789768709120"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`cancel-order` |
| args | Array of objects | Yes | Request Parameters |
| \> instIdCode | Integer | Conditional | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> instId | String | Conditional | Instrument ID <br> Will be deprecated on March 2026. |
| \> ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required, if both are passed, ordId will be used |
| \> clOrdId | String | Conditional | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1514",
  "op": "cancel-order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "2510789768709120",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1514",
  "op": "cancel-order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "2510789768709120",
      "ts": "1695190491421",
      "sCode": "5XXXX",
      "sMsg": "Order not exist"
    }
  ],
  "code": "1",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1514",
  "op": "cancel-order",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Order status message |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

Cancel order returns with sCode equal to 0. It is not strictly considered that the order has been canceled. It only means that your cancellation request has been accepted by the system server. The result of the cancellation is subject to the state pushed by the order channel or the get order state.

### WS / Cancel multiple orders

Cancel incomplete orders in batches. Maximum 20 orders can be canceled per request.

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 300 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Cancel order\`.

Rate limit is shared with the \`Cancel multiple orders\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1515",
  "op": "batch-cancel-orders",
  "args": [
    {
      "instId": "BTC-USDT",
      "ordId": "2517748157541376"
    },
    {
      "instId": "LTC-USDT",
      "ordId": "2517748155771904"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`batch-cancel-orders` |
| args | Array of objects | Yes | Request Parameters |
| \> instIdCode | Integer | Conditional | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> instId | String | Conditional | Instrument ID <br> Will be deprecated on March 2026. |
| \> ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required, if both are passed, ordId will be used |
| \> clOrdId | String | Conditional | Client Order ID as assigned by the client <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Response Example When All Succeed

```
Copy to Clipboard
{
  "id": "1515",
  "op": "batch-cancel-orders",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "2517748157541376",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": ""
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "2517748155771904",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When partially successfully

```
Copy to Clipboard
{
  "id": "1515",
  "op": "batch-cancel-orders",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "2517748157541376",
      "ts": "1695190491421",
      "sCode": "0",
      "sMsg": ""
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "2517748155771904",
      "ts": "1695190491421",
      "sCode": "5XXXX",
      "sMsg": "order not exist"
    }
  ],
  "code": "2",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When All Failed

```
Copy to Clipboard
{
  "id": "1515",
  "op": "batch-cancel-orders",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "2517748157541376",
      "ts": "1695190491421",
      "sCode": "5XXXX",
      "sMsg": "order not exist"
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "2517748155771904",
      "ts": "1695190491421",
      "sCode": "5XXXX",
      "sMsg": "order not exist"
    }
  ],
  "code": "1",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1515",
  "op": "batch-cancel-orders",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Order status message |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

### WS / Amend order

Amend an incomplete order.

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 60 requests per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 requests per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Rate limit is shared with the \`Amend order\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "amend-order",
  "args": [
    {
      "instId": "BTC-USDT",
      "ordId": "2510789768709120",
      "newSz": "2"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`amend-order` |
| args | Array of objects | Yes | Request Parameters |
| \> instIdCode | Integer | Conditional | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> instId | String | Conditional | Instrument ID <br> Will be deprecated on March 2026. |
| \> cxlOnFail | Boolean | No | Whether the order needs to be automatically canceled when the order amendment fails <br>Valid options: `false` or `true`, the default is `false`. |
| \> ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required, if both are passed, `ordId` will be used. |
| \> clOrdId | String | Conditional | Client Order ID as assigned by the client |
| \> reqId | String | No | Client Request ID as assigned by the client for order amendment <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| \> newSz | String | Conditional | New quantity after amendment and it has to be larger than 0. Either `newSz` or `newPx` is required. When amending a partially-filled order, the `newSz` should include the amount that has been filled. |
| \> newPx | String | Conditional | New price after amendment. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. It must be consistent with parameters when placing orders. For example, if users placed the order using px, they should use newPx when modifying the order. |
| \> newPxUsd | String | Conditional | Modify options orders using USD prices <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| \> newPxVol | String | Conditional | Modify options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| \> pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `newPx` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `newPx` exceeds the price limit<br> The default value is `0` |
| expTime | String | No | Request effective deadline. Unix timestamp format in milliseconds, e.g. `1597026383085` |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "amend-order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "2510789768709120",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "amend-order",
  "data": [
    {
      "clOrdId": "",
      "ordId": "2510789768709120",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "51008",
      "sMsg": "Order failed. Insufficient USDT balance in account",
      "subCode": "10000"
    }
  ],
  "code": "1",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1512",
  "op": "amend-order",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reqId | String | Client Request ID as assigned by the client for order amendment |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Order status message |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

newSz

If the new quantity of the order is less than or equal to the filled quantity when you are amending a partially-filled order, the order status will be changed to filled.

The amend order returns sCode equal to 0. It is not strictly considered that the order has been amended. It only means that your amend order request has been accepted by the system server. The result of the amend is subject to the status pushed by the order channel or the order status query

### WS / Amend multiple orders

Amend incomplete orders in batches. Maximum 20 orders can be amended per request.

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 300 orders per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 4 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

Rate limit of this endpoint will also be affected by the rules [Sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-sub-account-rate-limit) and [Fill ratio based sub-account rate limit](https://www.okx.com/docs-v5/en/#overview-rate-limits-fill-ratio-based-sub-account-rate-limit).

Unlike other endpoints, the rate limit of this endpoint is determined by the number of orders. If there is only one order in the request, it will consume the rate limit of \`Amend order\`.

Rate limit is shared with the \`Amend multiple orders\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-amend-orders",
  "args": [
    {
      "instId": "BTC-USDT",
      "ordId": "12345689",
      "newSz": "2"
    },
    {
      "instId": "BTC-USDT",
      "ordId": "12344",
      "newSz": "2"
    }
  ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`batch-amend-orders` |
| args | Array of objects | Yes | Request Parameters |
| \> instIdCode | Integer | Conditional | Instrument ID code. <br> If both `instId` and `instIdCode` are provided, `instIdCode` takes precedence. |
| \> instId | String | Conditional | Instrument ID <br> Will be deprecated on March 2026. |
| \> cxlOnFail | Boolean | No | Whether the order needs to be automatically canceled when the order amendment fails <br>Valid options: `false` or `true`, the default is `false`. |
| \> ordId | String | Conditional | Order ID <br>Either `ordId` or `clOrdId` is required, if both are passed, `ordId` will be used. |
| \> clOrdId | String | Conditional | Client Order ID as assigned by the client |
| \> reqId | String | No | Client Request ID as assigned by the client for order amendment <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| \> newSz | String | Conditional | New quantity after amendment and it has to be larger than 0. Either `newSz` or `newPx` is required. When amending a partially-filled order, the `newSz` should include the amount that has been filled. |
| \> newPx | String | Conditional | New price after amendment. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. It must be consistent with parameters when placing orders. For example, if users placed the order using px, they should use newPx when modifying the order. |
| \> newPxUsd | String | Conditional | Modify options orders using USD prices <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| \> newPxVol | String | Conditional | Modify options orders based on implied volatility, where 1 represents 100% <br>Only applicable to options. <br>When modifying options orders, users can only fill in one of the following: newPx, newPxUsd, or newPxVol. |
| \> pxAmendType | String | No | The price amendment type for orders<br>`0`: Do not allow the system to amend to order price if `newPx` exceeds the price limit <br>`1`: Allow the system to amend the price to the best available value within the price limit if `newPx` exceeds the price limit<br> The default value is `0` |
| expTime | String | No | Request effective deadline. Unix timestamp format in milliseconds, e.g. `1597026383085` |

> Response Example When All Succeed

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-amend-orders",
  "data": [
    {
      "clOrdId": "oktswap6",
      "ordId": "12345689",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "12344",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "0",
      "sMsg": "",
      "subCode": ""
    }
  ],
  "code": "0",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When All Failed

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-amend-orders",
  "data": [
    {
      "clOrdId": "",
      "ordId": "12345689",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "5XXXX",
      "sMsg": "order not exist"
    },
    {
      "clOrdId": "oktswap7",
      "ordId": "",
      "ts": "1695190491421",
      "reqId": "b12344",
          "sCode": "51008",
      "sMsg": "Order failed. Insufficient USDT balance in account",
      "subCode": "1000"
    }
  ],
  "code": "1",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Partially Successful

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-amend-orders",
  "data": [
    {
      "clOrdId": "",
      "ordId": "12345689",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "0",
      "sMsg": ""
    },
    {
      "clOrdId": "",
      "ordId": "oktswap7",
      "ts": "1695190491421",
      "reqId": "b12344",
      "sCode": "51063",
      "sMsg": "OrdId does not exist"
      "subCode": ""
    }
  ],
  "code": "2",
  "msg": "",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1513",
  "op": "batch-amend-orders",
  "data": [],
  "code": "60013",
  "msg": "Invalid args",
  "inTime": "1695190491421339",
  "outTime": "1695190491423240"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> ordId | String | Order ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> ts | String | Timestamp when the order request processing is finished by our system, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reqId | String | Client Request ID as assigned by the client for order amendment <br>If the user provides reqId in the request, the corresponding reqId will be returned |
| \> sCode | String | Order status code, `0` means success |
| \> sMsg | String | Order status message |
| \> subCode | String | Sub-code of sCode.<br> Returns `""` when sCode is 0 (request successful).<br> When sCode is not 0 (request failed), returns the sub-code if available; otherwise returns `""`. |
| inTime | String | Timestamp at Websocket gateway when the request is received, Unix timestamp format in microseconds, e.g. `1597026383085123` |
| outTime | String | Timestamp at Websocket gateway when the response is sent, Unix timestamp format in microseconds, e.g. `1597026383085123` |

newSz

If the new quantity of the order is less than or equal to the filled quantity when you are amending a partially-filled order, the order status will be changed to filled.

### WS / Mass cancel order

Cancel all the MMP pending orders of an instrument family.

Only applicable to Option in Portfolio Margin mode, and MMP privilege is required.

#### URL Path

/ws/v5/private (required login)

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

Rate limit is shared with the \`Mass Cancel Order\` REST API endpoints

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "mass-cancel",
    "args": [{
        "instType":"OPTION",
        "instFamily":"BTC-USD"
    }]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | Yes | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`mass-cancel` |
| args | Array of objects | Yes | Request parameters |
| \> instType | String | Yes | Instrument type<br>`OPTION` |
| \> instFamily | String | Yes | Instrument family |
| \> lockInterval | String | No | Lock interval(ms)<br> The range should be \[0, 10 000\]<br> The default is 0. You can set it as "0" if you want to unlock it immediately.<br> Error 54008 will be returned when placing order during lock interval, it is different from 51034 which is thrown when MMP is triggered |

> ##### Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "mass-cancel",
    "data": [
        {
            "result": true
        }
    ],
    "code": "0",
    "msg": ""
}
```

> Response Example When Format Error

```
Copy to Clipboard
{
  "id": "1512",
  "op": "mass-cancel",
  "data": [],
  "code": "60013",
  "msg": "Invalid args"
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| id | String | Unique identifier of the message |
| op | String | Operation |
| code | String | Error Code |
| msg | String | Error message |
| data | Array of objects | Data |
| \> result | Boolean | Result of the request `true`, `false` |

## Algo Trading

### POST / Place algo order

The algo order includes `trigger` order, `oco` order, `chase` order, `conditional` order, `twap` order and trailing order.

#### Rate Limit: 20 requests per 2 seconds

#### Rate Limit of lead trader lead instruments for Copy Trading: 1 request per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/order-algo`

> Request Example

```
Copy to Clipboard
# Place Take Profit / Stop Loss Order
POST /api/v5/trade/order-algo
body
{
    "instId":"BTC-USDT",
    "tdMode":"cross",
    "side":"buy",
    "ordType":"conditional",
    "sz":"2",
    "tpTriggerPx":"15",
    "tpOrdPx":"18"
}

# Place Trigger Order
POST /api/v5/trade/order-algo
body
{
    "instId": "BTC-USDT-SWAP",
    "side": "buy",
    "tdMode": "cross",
    "posSide": "net",
    "sz": "1",
    "ordType": "trigger",
    "triggerPx": "25920",
    "triggerPxType": "last",
    "orderPx": "-1",
    "attachAlgoOrds": [{
        "attachAlgoClOrdId": "",
        "slTriggerPx": "100",
        "slOrdPx": "600",
        "tpTriggerPx": "25921",
        "tpOrdPx": "2001"
    }]
}

# Place Trailing Stop Order
POST /api/v5/trade/order-algo
body
{
    "instId": "BTC-USDT-SWAP",
    "tdMode": "cross",
    "side": "buy",
    "ordType": "move_order_stop",
    "sz": "10",
    "posSide": "net",
    "callbackRatio": "0.05",
    "reduceOnly": true
}

# Place TWAP Order
POST /api/v5/trade/order-algo
body
{
    "instId": "BTC-USDT-SWAP",
    "tdMode": "cross",
    "side": "buy",
    "ordType": "twap",
    "sz": "10",
    "posSide": "net",
    "szLimit": "10",
    "pxLimit": "100",
    "timeInterval": "10",
    "pxSpread": "10"
}
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# One-way stop order
result = tradeAPI.place_algo_order(
    instId="BTC-USDT",
    tdMode="cross",
    side="buy",
    ordType="conditional",
    sz="2",
    tpTriggerPx="15",
    tpOrdPx="18"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| tdMode | String | Yes | Trade mode<br>Margin mode `cross``isolated`<br>Non-Margin mode `cash`<br>`spot_isolated` (only applicable to SPOT lead trading)<br>Note: `isolated` is not available in multi-currency margin mode and portfolio margin mode. |
| ccy | String | No | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`. |
| side | String | Yes | Order side, `buy``sell` |
| posSide | String | Conditional | Position side <br>Required in `long/short` mode and only be `long` or `short` |
| ordType | String | Yes | Order type <br>`conditional`: One-way stop order<br>`oco`: One-cancels-the-other order<br>`chase`: chase order, only applicable to FUTURES and SWAP<br>`trigger`: Trigger order<br>`move_order_stop`: Trailing order<br>`twap`: TWAP order |
| sz | String | Conditional | Quantity to buy or sell<br>Either `sz` or `closeFraction` is required. |
| tag | String | No | Order tag <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| tgtCcy | String | No | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` traded with Market buy `conditional` order<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| algoClOrdId | String | No | Client-supplied Algo ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| closeFraction | String | Conditional | Fraction of position to be closed when the algo order is triggered. <br>Currently the system supports fully closing the position only so the only accepted value is `1`. For the same position, only one TPSL pending order for fully closing the position is supported. <br>This is only applicable to `FUTURES` or `SWAP` instruments.<br>If `posSide` is `net`, `reduceOnly` must be `true`.<br>This is only applicable if `ordType` is `conditional` or `oco`.<br>This is only applicable if the stop loss and take profit order is executed as market order.<br>This is not supported in Portfolio Margin mode.<br>Either `sz` or `closeFraction` is required. |
| tradeQuoteCcy | String | No | The quote currency used for trading. Only applicable to `SPOT`. <br> The default value is the quote currency of the `instId`, for example: for `BTC-USD`, the default is `USD`. |

**Take Profit / Stop Loss Order**

Predefine the price you want the order to trigger a market order to execute immediately or it will place a limit order.

This type of order will not freeze your free margin in advance.

learn more about [Take Profit / Stop Loss Order](https://www.okx.com/help/11015447687437)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| tpTriggerPx | String | No | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| tpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| tpOrdPx | String | No | Take-profit order price <br>For condition TP order, if you fill in this parameter, you should fill in the take-profit trigger price as well.<br>For limit TP order, you need to fill in this parameter, but the take-profit trigger price doesn’t need to be filled. <br>If the price is `-1`, take-profit will be executed at the market price. |
| tpOrdKind | String | No | TP order kind<br>`condition`<br>`limit`<br> The default is `condition` |
| slTriggerPx | String | No | Stop-loss trigger price <br>If you fill in this parameter, you should fill in the stop-loss order price. |
| slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| slOrdPx | String | No | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |
| cxlOnClosePos | Boolean | No | Whether the TP/SL order placed by the user is associated with the corresponding position of the instrument. If it is associated, the TP/SL order will be canceled when the position is fully closed; if it is not, the TP/SL order will not be affected when the position is fully closed. <br>Valid values: <br>`true`: Place a TP/SL order associated with the position <br>`false`: Place a TP/SL order that is not associated with the position <br>The default value is `false`. If `true` is passed in, users must pass reduceOnly = true as well, indicating that when placing a TP/SL order associated with a position, it must be a reduceOnly order. <br>Only applicable to `Futures mode` and `Multi-currency margin`. |
| reduceOnly | Boolean | No | Whether the order can only reduce the position size. <br>Valid options: `true` or `false`. The default value is `false`. |

Take Profit / Stop Loss Order

When placing net TP/SL order (ordType=conditional) and both take-profit and stop-loss parameters are sent, only stop-loss logic will be performed and take-profit logic will be ignored.

**Chase order**

It will place a Post Only order immediately and amend it continuously

Chase order and corresponding Post Only order can't be amended.

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| chaseType | String | No | Chase type.<br>`distance`: distance from best bid/ask price, the default value.<br>`ratio`: ratio. |
| chaseVal | String | No | Chase value.<br>It represents distance from best bid/ask price when `chaseType` is distance. <br>For USDT-margined contract, the unit is USDT. <br>For USDC-margined contract, the unit is USDC. <br>For Crypto-margined contract, the unit is USD. <br>It represents ratio when `chaseType` is ratio. 0.1 represents 10%.<br> The default value is 0. |
| maxChaseType | String | Conditional | Maximum chase type.<br>`distance`: maximum distance from best bid/ask price<br>`ratio`: the ratio. <br> maxChaseTyep and maxChaseVal need to be used together or none of them. |
| maxChaseVal | String | Conditional | Maximum chase value.<br>It represents maximum distance when `maxChaseType` is distance.<br>It represents ratio when `maxChaseType` is ratio. 0.1 represents 10%. |
| reduceOnly | Boolean | No | Whether the order can only reduce the position size. <br>Valid options: `true` or `false`. The default value is `false`. |

**Trigger Order**

Use a trigger order to place a market or limit order when a specific price level is crossed.

When a Trigger Order is triggered, if your account balance is lower than the order amount, the system will automatically place the order based on your current balance.

Trigger orders do not freeze assets when placed.

Only applicable to SPOT/FUTURES/SWAP

learn more about [Trigger Order](https://www.okx.com/help/11015447687437)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| triggerPx | String | Yes | Trigger price |
| orderPx | String | Yes | Order Price <br>If the price is `-1`, the order will be executed at the market price. |
| advanceOrdType | String | No | Trigger order type<br>`fok`: Fill-or-kill order<br>`ioc`: Immediate-or-cancel order<br>Default is "", limit or market (controlled by orderPx) |
| triggerPxType | String | No | Trigger price type <br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| attachAlgoOrds | Array of objects | No | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| \> attachAlgoClOrdId | String | No | Client-supplied Algo ID when placing order attaching TP/SL.<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to algoClOrdId when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | No | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | No | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `tpTriggerPx` and `tpTriggerRatio` can be passed <br> If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. |
| \> tpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price<br>The default is `last` |
| \> tpOrdPx | String | No | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| \> slTriggerPx | String | No | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | No | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `slTriggerPx` and `slTriggerRatio` can be passed <br> If the main order is a buy order, it must be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. |
| \> slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| \> slOrdPx | String | No | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |

**Trailing Stop Order**

A trailing stop order is a stop order that tracks the market price. Its trigger price changes with the market price. Once the trigger price is reached, a market order is placed.

Actual trigger price for sell orders and short positions = Highest price after order placement – Trail variance (Var.), or Highest price after placement × (1 – Trail variance) (Ratio).

Actual trigger price for buy orders and long positions = Lowest price after order placement + Trail variance, or Lowest price after order placement × (1 + Trail variance).

You can use the activation price to set the activation condition for a trailing stop order.

learn more about [Trailing Stop Order](https://www.okx.com/help/11015447687437)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| callbackRatio | String | Conditional | Callback price ratio, e.g. `0.01` represents `1%`<br>Either `callbackRatio` or `callbackSpread` is allowed to be passed. |
| callbackSpread | String | Conditional | Callback price variance |
| activePx | String | No | Active price<br>The system will only start tracking the market and calculating your trigger price after the activation price is reached. If you don’t set a price, your order will be activated as soon as it’s placed. |
| reduceOnly | Boolean | No | Whether the order can only reduce the position size. <br>Valid options: `true` or `false`. The default value is `false`.<br>This parameter is only valid in the `FUTRUES`/`SWAP` net mode, and is ignored in the long/short mode. |

**TWAP Order**

Time-weighted average price (TWAP) strategy splits your order and places smaller orders at regular time intervals.

It is a strategy that will attempt to execute an order which trades in slices of order quantity at regular intervals of time as specified by users.

learn more about [TWAP Order](https://www.okx.com/help/xiii-time-weighted-average-price-twap)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| pxVar | String | Conditional | Price variance by percentage, range between \[0.0001 ~ 0.01\], e.g. `0.01` represents `1%`<br>Take buy orders as an example. When the market price is lower than the limit price, small buy orders will be placed above the best bid price within a certain range. This parameter determines the range by percentage.<br>Either `pxVar` or `pxSpread` is allowed to be passed. |
| pxSpread | String | Conditional | Price variance by constant, should be no less then 0 (no upper limit)<br>Take buy orders as an example. When the market price is lower than the limit price, small buy orders will be placed above the best bid price within a certain range. This parameter determines the range by constant. |
| szLimit | String | Yes | Average amount<br>Take buy orders as an example. When the market price is lower than the limit price, a certain amount of buy orders will be placed above the best bid price within a certain range. This parameter determines the amount. |
| pxLimit | String | Yes | Price Limit, should be no less then 0 (no upper limit)<br>Take buy orders as an example. When the market price is lower than the limit price, small buy orders will be placed above the best bid price within a certain range. This parameter represents the limit price. |
| timeInterval | String | Yes | Time interval in unit of `second`<br>ake buy orders as an example. When the market price is lower than the limit price, small buy orders will be placed above the best bid price within a certain range based on the time cycle. This parameter represents the time cycle. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "order1234",
            "algoId": "1836487817828872192",
            "clOrdId": "",
            "sCode": "0",
            "sMsg": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| clOrdId | String | ~~Client Order ID as assigned by the client~~(Deprecated) |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |
| tag | String | Order tag |

### POST / Cancel algo order

Cancel unfilled algo orders. A maximum of 10 orders can be canceled per request. Request parameters should be passed in the form of an array.

#### Rate Limit: 20 orders per 2 seconds

#### Rate limit rule (except Options): User ID + Instrument ID

#### Rate limit rule (Options only): User ID + Instrument Family

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/cancel-algos`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/cancel-algos
body
[
    {
        "algoId":"590919993110396111",
        "instId":"BTC-USDT"
    },
    {
        "algoId":"590920138287841222",
        "instId":"BTC-USDT"
    }
]
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Cancel unfilled algo orders (not including Iceberg order, TWAP order, Trailing Stop order)
algo_orders = [
    {"instId": "BTC-USDT", "algoId": "590919993110396111"},
    {"instId": "BTC-USDT", "algoId": "590920138287841222"}
]

result = tradeAPI.cancel_algo_order(algo_orders)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| algoId | String | Conditional | Algo ID<br>Either `algoId` or `algoClOrdId` is required. If both are passed, `algoId` will be used. |
| algoClOrdId | String | Conditional | Client-supplied Algo ID<br>Either `algoId` or `algoClOrdId` is required. If both are passed, `algoId` will be used. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "1836489397437468672",
            "clOrdId": "",
            "sCode": "0",
            "sMsg": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |
| clOrdId | String | ~~Client Order ID as assigned by the client~~(Deprecated) |
| algoClOrdId | String | ~~Client-supplied Algo ID~~(Deprecated) |
| tag | String | ~~Order tag~~(Deprecated) |

### POST / Amend algo order

Amend unfilled algo orders (Support Stop order and Trigger order only, not including Move\_order\_stop order, Iceberg order, TWAP order, Trailing Stop order).

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID + Instrument ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/trade/amend-algos`

> Request Example

```
Copy to Clipboard
POST /api/v5/trade/amend-algos
body
{
    "algoId":"2510789768709120",
    "newSz":"2",
    "instId":"BTC-USDT"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID |
| algoId | String | Conditional | Algo ID<br>Either `algoId` or `algoClOrdId` is required. If both are passed, `algoId` will be used. |
| algoClOrdId | String | Conditional | Client-supplied Algo ID<br>Either `algoId` or `algoClOrdId` is required. If both are passed, `algoId` will be used. |
| cxlOnFail | Boolean | No | Whether the order needs to be automatically canceled when the order amendment fails <br>Valid options: `false` or `true`, the default is `false`. |
| reqId | String | Conditional | Client Request ID as assigned by the client for order amendment <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. <br>The response will include the corresponding `reqId` to help you identify the request if you provide it in the request. |
| newSz | String | Conditional | New quantity after amendment and it has to be larger than 0. |

**Take Profit / Stop Loss Order**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| newTpTriggerPx | String | Conditional | Take-profit trigger price. <br>Either the take-profit trigger price or order price is 0, it means that the take-profit is deleted |
| newTpOrdPx | String | Conditional | Take-profit order price <br>If the price is -1, take-profit will be executed at the market price. |
| newSlTriggerPx | String | Conditional | Stop-loss trigger price.<br>Either the stop-loss trigger price or order price is 0, it means that the stop-loss is deleted |
| newSlOrdPx | String | Conditional | Stop-loss order price <br>If the price is -1, stop-loss will be executed at the market price. |
| newTpTriggerPxType | String | Conditional | Take-profit trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price |
| newSlTriggerPxType | String | Conditional | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price |

**Trigger Order**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| newTriggerPx | String | Yes | New trigger price after amendment |
| newOrdPx | String | Yes | New order price after amendment<br>If the price is `-1`, the order will be executed at the market price. |
| newTriggerPxType | String | No | New trigger price type after amendment <br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| attachAlgoOrds | Array of objects | No | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| \> newTpTriggerPx | String | No | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| \> newTpTriggerRatio | String | No | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `newTpTriggerPx` and `newTpTriggerRatio` can be passed <br> If the main order is a buy order, it must be greater than 0, and if the main order is a sell order, it must be bewteen -1 and 0. <br> 0 means to delete the take-profit. |
| \> newTpTriggerPxType | String | No | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price<br>The default is `last` |
| \> newTpOrdPx | String | No | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| \> newSlTriggerPx | String | No | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> newSlTriggerRatio | String | No | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. <br> Only one of `newSlTriggerPx` and `newSlTriggerRatio` can be passed <br> If the main order is a buy order, it must be bewteen 0 and 1, and if the main order is a sell order, it must be greater than 0. <br> 0 means to delete the stop-loss. |
| \> newSlTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>The default is `last` |
| \> newSlOrdPx | String | No | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoClOrdId":"algo_01",
            "algoId":"2510789768709120",
            "reqId":"po103ux",
            "sCode":"0",
            "sMsg":""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| reqId | String | Client Request ID as assigned by the client for order amendment. |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |

### GET / Algo order details

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/order-algo`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/order-algo?algoId=1753184812254216192
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Conditional | Algo ID<br>Either `algoId` or `algoClOrdId` is required.If both are passed, `algoId` will be used. |
| algoClOrdId | String | Conditional | Client-supplied Algo ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "activePx": "",
            "actualPx": "",
            "actualSide": "",
            "actualSz": "0",
            "algoClOrdId": "",
            "algoId": "1753184812254216192",
            "amendPxOnTriggerType": "0",
            "attachAlgoOrds": [],
            "cTime": "1724751378980",
            "callbackRatio": "",
            "callbackSpread": "",
            "ccy": "",
            "chaseType": "",
            "chaseVal": "",
            "clOrdId": "",
            "closeFraction": "",
            "failCode": "0",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "isTradeBorrowMode": "",
            "last": "62916.5",
            "lever": "",
            "linkedOrd": {
                "ordId": ""
            },
            "maxChaseType": "",
            "maxChaseVal": "",
            "moveTriggerPx": "",
            "ordId": "",
            "ordIdList": [],
            "ordPx": "",
            "ordType": "conditional",
            "posSide": "net",
            "pxLimit": "",
            "pxSpread": "",
            "pxVar": "",
            "quickMgnType": "",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "state": "live",
            "sz": "10",
            "szLimit": "",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "quote_ccy",
            "timeInterval": "",
            "tpOrdPx": "-1",
            "tpTriggerPx": "10000",
            "tpTriggerPxType": "last",
            "triggerPx": "",
            "triggerPxType": "",
            "triggerTime": "",
            "tradeQuoteCcy": "USDT",
            "uTime": "1724751378980"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Latest order ID. It will be deprecated soon |
| ordIdList | Array of strings | Order ID list. There will be multiple order IDs when there is TP/SL splitting order. |
| algoId | String | Algo ID |
| clOrdId | String | Client Order ID as assigned by the client |
| sz | String | Quantity to buy or sell |
| closeFraction | String | Fraction of position to be closed when the algo order is triggered |
| ordType | String | Order type |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| state | String | State <br>`live`<br>`pause`<br>`partially_effective`<br>`effective`<br>`canceled`<br>`order_failed`<br>`partially_failed` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| triggerPx | String | trigger price. |
| triggerPxType | String | trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| ordPx | String | Order price for the trigger order |
| advanceOrdType | String | Trigger order type<br>`fok`: Fill-or-kill order<br>`ioc`: Immediate-or-cancel order<br>Default is "", limit or market (controlled by orderPx) |
| actualSz | String | Actual order quantity |
| actualPx | String | Actual order price |
| tag | String | Order tag |
| actualSide | String | Actual trigger side, `tp`: take profit `sl`: stop loss<br>Only applicable to oco order and conditional order |
| triggerTime | String | Trigger time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| pxVar | String | Price ratio <br>Only applicable to `iceberg` order or `twap` order |
| pxSpread | String | Price variance <br>Only applicable to `iceberg` order or `twap` order |
| szLimit | String | Average amount <br>Only applicable to `iceberg` order or `twap` order |
| pxLimit | String | Price Limit <br>Only applicable to `iceberg` order or `twap` order |
| timeInterval | String | Time interval <br>Only applicable to `twap` order |
| callbackRatio | String | Callback price ratio<br>Only applicable to `move_order_stop` order |
| callbackSpread | String | Callback price variance<br>Only applicable to `move_order_stop` order |
| activePx | String | Active price<br>Only applicable to `move_order_stop` order |
| moveTriggerPx | String | Trigger price<br>Only applicable to `move_order_stop` order |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| quickMgnType | String | Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay` |
| last | String | Last filled price while placing |
| failCode | String | It represents that the reason that algo order fails to trigger. It is "" when the state is `effective`/`canceled`. There will be value when the state is `order_failed`, e.g. 51008;<br>Only applicable to Stop Order, Trailing Stop Order, Trigger order. |
| algoClOrdId | String | Client-supplied Algo ID |
| amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| attachAlgoOrds | Array of objects | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL.<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to algoClOrdId when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| \> slTriggerPx | String | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |
| linkedOrd | Object | Linked TP order detail, only applicable to SL order that comes from the one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> ordId | String | Order ID |
| cTime | String | Creation time Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Order updated time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| isTradeBorrowMode | String | Whether borrowing currency automatically<br> true<br> false<br>Only applicable to `trigger order`, `trailing order` and `twap order` |
| chaseType | String | Chase type. Only applicable to `chase` order. |
| chaseVal | String | Chase value. Only applicable to `chase` order. |
| maxChaseType | String | Maximum chase type. Only applicable to `chase` order. |
| maxChaseVal | String | Maximum chase value. Only applicable to `chase` order. |
| tradeQuoteCcy | String | The quote currency used for trading. |

### GET / Algo order list

Retrieve a list of untriggered Algo orders under the current account.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/orders-algo-pending`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/orders-algo-pending?ordType=conditional
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve a list of untriggered one-way stop orders
result = tradeAPI.order_algos_list(
    ordType="conditional"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ordType | String | Yes | Order type<br>`conditional`: One-way stop order <br>`oco`: One-cancels-the-other order <br>`chase`: chase order, only applicable to FUTURES and SWAP<br>`trigger`: Trigger order <br>`move_order_stop`: Trailing order <br>`iceberg`: Iceberg order <br>`twap`: TWAP order<br>For every request, unlike other ordType which only can use one type, `conditional` and `oco` both can be used and separated with comma. |
| algoId | String | No | Algo ID |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>`FUTURES`<br>`MARGIN` |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "activePx": "",
            "actualPx": "",
            "actualSide": "",
            "actualSz": "0",
            "algoClOrdId": "",
            "algoId": "1753184812254216192",
            "amendPxOnTriggerType": "0",
            "attachAlgoOrds": [],
            "cTime": "1724751378980",
            "callbackRatio": "",
            "callbackSpread": "",
            "ccy": "",
            "chaseType": "",
            "chaseVal": "",
            "clOrdId": "",
            "closeFraction": "",
            "failCode": "0",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "isTradeBorrowMode": "",
            "last": "62916.5",
            "lever": "",
            "linkedOrd": {
                "ordId": ""
            },
            "maxChaseType": "",
            "maxChaseVal": "",
            "moveTriggerPx": "",
            "ordId": "",
            "ordIdList": [],
            "ordPx": "",
            "ordType": "conditional",
            "posSide": "net",
            "pxLimit": "",
            "pxSpread": "",
            "pxVar": "",
            "quickMgnType": "",
            "reduceOnly": "false",
            "side": "buy",
            "slOrdPx": "",
            "slTriggerPx": "",
            "slTriggerPxType": "",
            "state": "live",
            "sz": "10",
            "szLimit": "",
            "tag": "",
            "tdMode": "cash",
            "tgtCcy": "quote_ccy",
            "timeInterval": "",
            "tpOrdPx": "-1",
            "tpTriggerPx": "10000",
            "tpTriggerPxType": "last",
            "triggerPx": "",
            "triggerPxType": "",
            "triggerTime": "",
            "tradeQuoteCcy": "USDT",
            "uTime": "1724751378980"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Latest order ID. It will be deprecated soon |
| ordIdList | Array of strings | Order ID list. There will be multiple order IDs when there is TP/SL splitting order. |
| algoId | String | Algo ID |
| clOrdId | String | Client Order ID as assigned by the client |
| sz | String | Quantity to buy or sell |
| closeFraction | String | Fraction of position to be closed when the algo order is triggered |
| ordType | String | Order type |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` traded with Market order |
| state | String | State<br>`live`<br>`pause` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| tpTriggerPx | String | Take-profit trigger price |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price |
| slTriggerPx | String | Stop-loss trigger price |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price |
| triggerPx | String | Trigger price |
| triggerPxType | String | Trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| ordPx | String | Order price for the trigger order |
| advanceOrdType | String | Trigger order type |
| actualSz | String | Actual order quantity |
| tag | String | Order tag |
| actualPx | String | Actual order price |
| actualSide | String | Actual trigger side<br>`tp`: take profit `sl`: stop loss<br>Only applicable to oco order and conditional order |
| triggerTime | String | Trigger time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| pxVar | String | Price ratio <br>Only applicable to `iceberg` order or `twap` order |
| pxSpread | String | Price variance <br>Only applicable to `iceberg` order or `twap` order |
| szLimit | String | Average amount <br>Only applicable to `iceberg` order or `twap` order |
| pxLimit | String | Price Limit <br>Only applicable to `iceberg` order or `twap` order |
| timeInterval | String | Time interval <br>Only applicable to `twap` order |
| callbackRatio | String | Callback price ratio<br>Only applicable to `move_order_stop` order |
| callbackSpread | String | Callback price variance<br>Only applicable to `move_order_stop` order |
| activePx | String | Active price<br>Only applicable to `move_order_stop` order |
| moveTriggerPx | String | Trigger price<br>Only applicable to `move_order_stop` order |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| quickMgnType | String | Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay` |
| last | String | Last filled price while placing |
| failCode | String | It represents that the reason that algo order fails to trigger. There will be value when the state is `order_failed`, e.g. 51008;<br>For this endpoint, it always is "". |
| algoClOrdId | String | Client-supplied Algo ID |
| amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| attachAlgoOrds | Array of objects | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL.<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to algoClOrdId when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| \> slTriggerPx | String | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |
| linkedOrd | Object | Linked TP order detail, only applicable to SL order that comes from the one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> ordId | String | Order ID |
| cTime | String | Creation time Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Order updated time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| isTradeBorrowMode | String | Whether borrowing currency automatically<br> true<br> false<br>Only applicable to `trigger order`, `trailing order` and `twap order` |
| chaseType | String | Chase type. Only applicable to `chase` order. |
| chaseVal | String | Chase value. Only applicable to `chase` order. |
| maxChaseType | String | Maximum chase type. Only applicable to `chase` order. |
| maxChaseVal | String | Maximum chase value. Only applicable to `chase` order. |
| tradeQuoteCcy | String | The quote currency used for trading. |

### GET / Algo order history

Retrieve a list of all algo orders under the current account in the last 3 months.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/trade/orders-algo-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/trade/orders-algo-history?ordType=conditional&state=effective
```

```
Copy to Clipboard
import okx.Trade as Trade

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

tradeAPI = Trade.TradeAPI(apikey, secretkey, passphrase, False, flag)

# Retrieve a list of all one-way stop algo orders
result = tradeAPI.order_algos_history(
    state="effective",
    ordType="conditional"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ordType | String | Yes | Order type <br>`conditional`: One-way stop order <br>`oco`: One-cancels-the-other order <br>`chase`: chase order, only applicable to FUTURES and SWAP<br>`trigger`: Trigger order <br>`move_order_stop`: Trailing order <br>`iceberg`: Iceberg order <br>`twap`: TWAP order<br> For every request, unlike other ordType which only can use one type, `conditional` and `oco` both can be used and separated with comma. |
| state | String | Conditional | State<br>`effective`<br>`canceled`<br>`order_failed`<br>Either `state` or `algoId` is required |
| algoId | String | Conditional | Algo ID <br>Either `state` or `algoId` is required. |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>`FUTURES`<br>`MARGIN` |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| after | String | No | Pagination of data to return records earlier than the requested `algoId` |
| before | String | No | Pagination of data to return records new than the requested `algoId` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "activePx": "",
            "actualPx": "",
            "actualSide": "tp",
            "actualSz": "100",
            "algoClOrdId": "",
            "algoId": "1880721064716505088",
            "amendPxOnTriggerType": "0",
            "attachAlgoOrds": [],
            "cTime": "1728552255493",
            "callbackRatio": "",
            "callbackSpread": "",
            "ccy": "",
            "chaseType": "",
            "chaseVal": "",
            "clOrdId": "",
            "closeFraction": "1",
            "failCode": "1",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "isTradeBorrowMode": "",
            "last": "60777.5",
            "lever": "10",
            "linkedOrd": {
                "ordId": ""
            },
            "maxChaseType": "",
            "maxChaseVal": "",
            "moveTriggerPx": "",
            "ordId": "1884789786215137280",
            "ordIdList": [
                "1884789786215137280"
            ],
            "ordPx": "",
            "ordType": "oco",
            "posSide": "long",
            "pxLimit": "",
            "pxSpread": "",
            "pxVar": "",
            "quickMgnType": "",
            "reduceOnly": "true",
            "side": "sell",
            "slOrdPx": "-1",
            "slTriggerPx": "57000",
            "slTriggerPxType": "mark",
            "state": "effective",
            "sz": "100",
            "szLimit": "",
            "tag": "",
            "tdMode": "isolated",
            "tgtCcy": "",
            "timeInterval": "",
            "tpOrdPx": "-1",
            "tpTriggerPx": "63000",
            "tpTriggerPxType": "last",
            "triggerPx": "",
            "triggerPxType": "",
            "triggerTime": "1728673513447",
            "tradeQuoteCcy": "USDT",
            "uTime": "1728673513447"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| ordId | String | Latest order ID. It will be deprecated soon |
| ordIdList | Array of strings | Order ID list. There will be multiple order IDs when there is TP/SL splitting order. |
| algoId | String | Algo ID |
| clOrdId | String | Client Order ID as assigned by the client |
| sz | String | Quantity to buy or sell |
| closeFraction | String | Fraction of position to be closed when the algo order is triggered |
| ordType | String | Order type |
| side | String | Order side |
| posSide | String | Position side |
| tdMode | String | Trade mode |
| tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| state | String | State <br>`effective`<br>`canceled`<br>`order_failed`<br>`partially_failed` |
| lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| tpTriggerPx | String | Take-profit trigger price. |
| tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| tpOrdPx | String | Take-profit order price. |
| slTriggerPx | String | Stop-loss trigger price. |
| slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| slOrdPx | String | Stop-loss order price. |
| triggerPx | String | trigger price. |
| triggerPxType | String | trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| ordPx | String | Order price for the trigger order |
| advanceOrdType | String | Trigger order type |
| actualSz | String | Actual order quantity |
| actualPx | String | Actual order price |
| tag | String | Order tag |
| actualSide | String | Actual trigger side, `tp`: take profit `sl`: stop loss<br>Only applicable to oco order and conditional order |
| triggerTime | String | Trigger time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| pxVar | String | Price ratio <br>Only applicable to `iceberg` order or `twap` order |
| pxSpread | String | Price variance <br>Only applicable to `iceberg` order or `twap` order |
| szLimit | String | Average amount <br>Only applicable to `iceberg` order or `twap` order |
| pxLimit | String | Price Limit <br>Only applicable to `iceberg` order or `twap` order |
| timeInterval | String | Time interval <br>Only applicable to `twap` order |
| callbackRatio | String | Callback price ratio<br>Only applicable to `move_order_stop` order |
| callbackSpread | String | Callback price variance<br>Only applicable to `move_order_stop` order |
| activePx | String | Active price<br>Only applicable to `move_order_stop` order |
| moveTriggerPx | String | Trigger price<br>Only applicable to `move_order_stop` order |
| reduceOnly | String | Whether the order can only reduce the position size. Valid options: true or false. |
| quickMgnType | String | Quick Margin type, Only applicable to Quick Margin Mode of isolated margin<br>`manual`, `auto_borrow`, `auto_repay` |
| last | String | Last filled price while placing |
| failCode | String | It represents that the reason that algo order fails to trigger. It is "" when the state is `effective`/`canceled`. There will be value when the state is `order_failed`, e.g. 51008;<br>Only applicable to Stop Order, Trailing Stop Order, Trigger order. |
| algoClOrdId | String | Client Algo Order ID as assigned by the client. |
| amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| attachAlgoOrds | Array of objects | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| \> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL.<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to algoClOrdId when placing TP/SL order once the general order is filled completely. |
| \> tpTriggerPx | String | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| \> tpTriggerRatio | String | Take profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> tpTriggerPxType | String | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| \> slTriggerPx | String | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| \> slTriggerRatio | String | Stop profit trigger ratio, 0.3 represents 30% <br> Only applicable to FUTURES and SWAP. |
| \> slTriggerPxType | String | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |
| linkedOrd | Object | Linked TP order detail, only applicable to SL order that comes from the one-cancels-the-other (OCO) order that contains the TP limit order. |
| \> ordId | String | Order ID |
| cTime | String | Creation time Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Order updated time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| isTradeBorrowMode | String | Whether borrowing currency automatically<br> true<br> false<br>Only applicable to `trigger order`, `trailing order` and `twap order` |
| chaseType | String | Chase type. Only applicable to `chase` order. |
| chaseVal | String | Chase value. Only applicable to `chase` order. |
| maxChaseType | String | Maximum chase type. Only applicable to `chase` order. |
| maxChaseVal | String | Maximum chase value. Only applicable to `chase` order. |
| tradeQuoteCcy | String | The quote currency used for trading. |

### WS / Algo orders channel

Retrieve algo orders (includes `trigger` order, `oco` order, `conditional` order). Data will not be pushed when first subscribed. Data will only be pushed when there are order updates.

#### URL Path

/ws/v5/business (required login)

> Request Example : single

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "orders-algo",
      "instType": "FUTURES",
      "instFamily": "BTC-USD",
      "instId": "BTC-USD-200329"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
          "channel": "orders-algo",
          "instType": "FUTURES",
          "instFamily": "BTC-USD",
          "instId": "BTC-USD-200329"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "orders-algo",
      "instType": "FUTURES",
      "instFamily": "BTC-USD"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
          "channel": "orders-algo",
          "instType": "FUTURES",
          "instFamily": "BTC-USD"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`orders-algo` |
| \> instType | String | Yes | Instrument type <br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`ANY` |
| \> instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| \> instId | String | No | Instrument ID |

> Successful Response Example : single

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "orders-algo",
    "instType": "FUTURES",
    "instFamily": "BTC-USD",
    "instId": "BTC-USD-200329"
  },
  "connId": "a4d3ae55"
}
```

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "orders-algo",
    "instType": "FUTURES",
    "instFamily": "BTC-USD"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"orders-algo\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type <br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`ANY` |
| \> instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |
| \> instId | String | No | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example: single

```
Copy to Clipboard
{
    "arg": {
        "channel": "orders-algo",
        "uid": "77982378738415879",
        "instType": "FUTURES",
        "instId": "BTC-USD-200329"
    },
    "data": [{
        "actualPx": "0",
        "actualSide": "",
        "actualSz": "0",
        "algoClOrdId": "",
        "algoId": "581878926302093312",
        "attachAlgoOrds": [],
        "amendResult": "",
        "cTime": "1685002746818",
        "uTime": "1708679675245",
        "ccy": "",
        "clOrdId": "",
        "closeFraction": "",
        "failCode": "",
        "instId": "BTC-USDC",
        "instType": "SPOT",
        "last": "26174.8",
        "lever": "0",
        "notionalUsd": "11.0",
        "ordId": "",
        "ordIdList": [],
        "ordPx": "",
        "ordType": "conditional",
        "posSide": "",
        "quickMgnType": "",
        "reduceOnly": "false",
        "reqId": "",
        "side": "buy",
        "slOrdPx": "",
        "slTriggerPx": "",
        "slTriggerPxType": "",
        "state": "live",
        "sz": "11",
        "tag": "",
        "tdMode": "cross",
        "tgtCcy": "quote_ccy",
        "tpOrdPx": "-1",
        "tpTriggerPx": "1",
        "tpTriggerPxType": "last",
        "triggerPx": "",
        "triggerTime": "",
        "tradeQuoteCcy": "USDT",
        "amendPxOnTriggerType": "0",
        "linkedOrd":{
                "ordId":"98192973880283"
        },
        "isTradeBorrowMode": ""
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> instType | String | Instrument type |
| \> instFamily | String | Instrument family |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| \> ordId | String | Latest order ID, the order ID associated with the algo order. It will be deprecated soon |
| \> ordIdList | Array of strings | Order ID list. There will be multiple order IDs when there is TP/SL splitting order. |
| \> algoId | String | Algo ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> sz | String | Quantity to buy or sell.<br>`SPOT`/`MARGIN`: in the unit of currency.<br>`FUTURES`/`SWAP`/`OPTION`: in the unit of contract. |
| \> ordType | String | Order type<br>`conditional`: One-way stop order <br>`oco`: One-cancels-the-other order <br>`trigger`: Trigger order <br>`chase`: Chase order |
| \> side | String | Order side<br>`buy`<br>`sell` |
| \> posSide | String | Position side <br>`net`<br>`long` or `short`<br>Only applicable to `FUTURES`/`SWAP` |
| \> tdMode | String | Trade mode<br>`cross`: cross<br>`isolated`: isolated<br>`cash`: cash |
| \> tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency<br>`quote_ccy`: Quote currency<br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| \> lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| \> state | String | Order status <br>`live`: to be effective <br>`effective`: effective <br>`canceled`: canceled <br>`order_failed`: order failed<br>`partially_failed`: partially failed<br>`partially_effective`: partially effective |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpTriggerPxType | String | Take-profit trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slTriggerPxType | String | Stop-loss trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> slOrdPx | String | Stop-loss order price. |
| \> triggerPx | String | Trigger price |
| \> triggerPxType | String | Trigger price type. <br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| \> ordPx | String | Order price for the trigger order |
| \> advanceOrdType | String | Trigger order type |
| \> last | String | Last filled price while placing |
| \> actualSz | String | Actual order quantity |
| \> actualPx | String | Actual order price |
| \> notionalUsd | String | Estimated national value in `USD` of order |
| \> tag | String | Order tag |
| \> actualSide | String | Actual trigger side<br>Only applicable to oco order and conditional order |
| \> triggerTime | String | Trigger time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> reduceOnly | String | Whether the order can only reduce the position size. Valid options: `true` or `false`. |
| \> failCode | String | It represents that the reason that algo order fails to trigger. It is "" when the state is `effective`/`canceled`. There will be value when the state is `order_failed`, e.g. 51008;<br>Only applicable to Stop Order, Trailing Stop Order, Trigger order. |
| \> algoClOrdId | String | Client Algo Order ID as assigned by the client. |
| \> reqId | String | Client Request ID as assigned by the client for order amendment. "" will be returned if there is no order amendment. |
| \> amendResult | String | The result of amending the order<br>`-1`: failure <br>`0`: success |
| \> amendPxOnTriggerType | String | Whether to enable Cost-price SL. Only applicable to SL order of split TPs. <br>`0`: disable, the default value <br>`1`: Enable |
| \> attachAlgoOrds | Array of objects | Attached SL/TP orders info<br>Applicable to `Futures mode/Multi-currency margin/Portfolio margin` |
| >\> attachAlgoClOrdId | String | Client-supplied Algo ID when placing order attaching TP/SL.<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.<br>It will be posted to algoClOrdId when placing TP/SL order once the general order is filled completely. |
| >\> tpTriggerPx | String | Take-profit trigger price<br>If you fill in this parameter, you should fill in the take-profit order price as well. |
| >\> tpTriggerRatio | String | Take-profit trigger ratio, 0.3 represents 30%. Only applicable to `FUTURES`/`SWAP` contracts. |
| >\> tpTriggerPxType | String | Take-profit trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| >\> tpOrdPx | String | Take-profit order price<br>If you fill in this parameter, you should fill in the take-profit trigger price as well. <br>If the price is `-1`, take-profit will be executed at the market price. |
| >\> slTriggerPx | String | Stop-loss trigger price<br>If you fill in this parameter, you should fill in the stop-loss order price. |
| >\> slTriggerRatio | String | Stop-loss trigger ratio, 0.3 represents 30%. Only applicable to `FUTURES`/`SWAP` contracts. |
| >\> slTriggerPxType | String | Stop-loss trigger price type<br>`last`: last price<br>`index`: index price<br>`mark`: mark price |
| >\> slOrdPx | String | Stop-loss order price <br>If you fill in this parameter, you should fill in the stop-loss trigger price. <br>If the price is `-1`, stop-loss will be executed at the market price. |
| \> linkedOrd | Object | Linked TP order detail, only applicable to SL order that comes from the one-cancels-the-other (OCO) order that contains the TP limit order. |
| >\> ordId | String | Order ID |
| \> cTime | String | Creation time Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Order updated time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| \> isTradeBorrowMode | String | Whether borrowing currency automatically<br> true<br> false<br>Only applicable to `trigger order`, `trailing order` and `twap order` |
| \> chaseType | String | Chase type. Only applicable to `chase` order. |
| \> chaseVal | String | Chase value. Only applicable to `chase` order. |
| \> maxChaseType | String | Maximum chase type. Only applicable to `chase` order. |
| \> maxChaseVal | String | Maximum chase value. Only applicable to `chase` order. |
| \> tradeQuoteCcy | String | The quote currency used for trading. |

### WS / Advance algo orders channel

Retrieve advance algo orders (including Iceberg order, TWAP order, Trailing order). Data will be pushed when first subscribed. Data will be pushed when triggered by events such as placing/canceling order.

#### URL Path

/ws/v5/business (required login)

> Request Example : single

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "algo-advance",
      "instType": "SPOT",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
          "channel": "algo-advance",
          "instType": "SPOT",
          "instId": "BTC-USDT"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "algo-advance",
      "instType": "SPOT"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [
        {
          "channel": "algo-advance",
          "instType": "SPOT"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`algo-advance` |
| \> instType | String | Yes | Instrument type <br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`ANY` |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |

> Successful Response Example : single

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "algo-advance",
    "instType": "SPOT",
    "instId": "BTC-USDT"
  },
  "connId": "a4d3ae55"
}
```

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "algo-advance",
    "instType": "SPOT"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"algo-advance\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type <br>`SPOT`<br>`MARGIN`<br>`SWAP`<br>`FUTURES`<br>`ANY` |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example: single

```
Copy to Clipboard
{
    "arg":{
        "channel":"algo-advance",
        "uid": "77982378738415879",
        "instType":"SPOT",
        "instId":"BTC-USDT"
    },
    "data":[
        {
            "actualPx":"",
            "actualSide":"",
            "actualSz":"0",
            "algoId":"355056228680335360",
            "cTime":"1630924001545",
            "ccy":"",
            "clOrdId": "",
            "count":"1",
            "instId":"BTC-USDT",
            "instType":"SPOT",
            "lever":"0",
            "notionalUsd":"",
            "ordPx":"",
            "ordType":"iceberg",
            "pTime":"1630924295204",
            "posSide":"net",
            "pxLimit":"10",
            "pxSpread":"1",
            "pxVar":"",
            "side":"buy",
            "slOrdPx":"",
            "slTriggerPx":"",
            "state":"pause",
            "sz":"0.1",
            "szLimit":"0.1",
            "tdMode":"cash",
            "timeInterval":"",
            "tpOrdPx":"",
            "tpTriggerPx":"",
            "tag": "adadadadad",
            "triggerPx":"",
            "triggerTime":"",
            "tradeQuoteCcy": "USDT",
            "callbackRatio":"",
            "callbackSpread":"",
            "activePx":"",
            "moveTriggerPx":"",
            "failCode": "",
                "algoClOrdId": "",
            "reduceOnly": "",
            "isTradeBorrowMode": true
        }
    ]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> algoId | String | Algo Order ID |
| data | Array of objects | Subscribed data |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> ccy | String | Margin currency <br>Applicable to all `isolated``MARGIN` orders and `cross``MARGIN` orders in `Futures mode`, `FUTURES` and `SWAP` contracts. |
| \> ordId | String | Order ID, the order ID associated with the algo order. |
| \> algoId | String | Algo ID |
| \> clOrdId | String | Client Order ID as assigned by the client |
| \> sz | String | Quantity to buy or sell. `SPOT`/`MARGIN`: in the unit of currency. `FUTURES`/`SWAP`/`OPTION`: in the unit of contract. |
| \> ordType | String | Order type <br>`iceberg`: Iceberg order <br>`twap`: TWAP order <br>`move_order_stop`: Trailing order |
| \> side | String | Order side, `buy``sell` |
| \> posSide | String | Position side <br>`net`<br>`long` or `short` Only applicable to `FUTURES`/`SWAP` |
| \> tdMode | String | Trade mode, `cross`: cross `isolated`: isolated `cash`: cash |
| \> tgtCcy | String | Order quantity unit setting for `sz`<br>`base_ccy`: Base currency ,`quote_ccy`: Quote currency <br>Only applicable to `SPOT` Market Orders<br>Default is `quote_ccy` for buy, `base_ccy` for sell |
| \> lever | String | Leverage, from `0.01` to `125`. <br>Only applicable to `MARGIN/FUTURES/SWAP` |
| \> state | String | Order status <br>`live`: to be effective <br>`effective`: effective<br>`partially_effective`: partially effective<br>`canceled`: canceled <br>`order_failed`: order failed <br>`pause`: pause |
| \> tpTriggerPx | String | Take-profit trigger price. |
| \> tpOrdPx | String | Take-profit order price. |
| \> slTriggerPx | String | Stop-loss trigger price. |
| \> slOrdPx | String | Stop-loss order price. |
| \> triggerPx | String | Trigger price |
| \> ordPx | String | Order price |
| \> actualSz | String | Actual order quantity |
| \> actualPx | String | Actual order price |
| \> notionalUsd | String | Estimated national value in `USD` of order |
| \> tag | String | Order tag |
| \> actualSide | String | Actual trigger side |
| \> triggerTime | String | Trigger time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> cTime | String | Creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> pxVar | String | Price ratio <br>Only applicable to `iceberg` order or `twap` order |
| \> pxSpread | String | Price variance <br>Only applicable to `iceberg` order or `twap` order |
| \> szLimit | String | Average amount <br>Only applicable to `iceberg` order or `twap` order |
| \> pxLimit | String | Price limit <br>Only applicable to `iceberg` order or `twap` order |
| \> timeInterval | String | Time interval <br>Only applicable to `twap` order |
| \> count | String | Algo Order count <br>Only applicable to `iceberg` order or `twap` order |
| \> callbackRatio | String | Callback price ratio<br>Only applicable to `move_order_stop` order |
| \> callbackSpread | String | Callback price variance<br>Only applicable to `move_order_stop` order |
| \> activePx | String | Active price<br>Only applicable to `move_order_stop` order |
| \> moveTriggerPx | String | Trigger price<br>Only applicable to `move_order_stop` order |
| \> failCode | String | It represents that the reason that algo order fails to trigger. It is "" when the state is `effective`/`canceled`. There will be value when the state is `order_failed`, e.g. 51008;<br>Only applicable to Stop Order, Trailing Stop Order, Trigger order. |
| \> algoClOrdId | String | Client Algo Order ID as assigned by the client. |
| \> reduceOnly | String | Whether the order can only reduce the position size. Valid options: `true` or `false`. |
| \> pTime | String | Push time of algo order information, millisecond format of Unix timestamp, e.g. `1597026383085` |
| \> isTradeBorrowMode | Boolean | Whether borrowing currency automatically<br> true<br> false<br>Only applicable to `trigger order`, `trailing order` and `twap order` |
| \> tradeQuoteCcy | String | The quote currency used for trading. |

## Grid Trading

Grid trading works by the simple strategy of buy low and sell high.
After you set the parameters, the system automatically places orders at incrementally increasing or decreasing prices. Overall, the grid bot seeks to capitalize on normal price volatility by placing buy and sell orders at certain regular intervals above and below a predefined base price.

The API endpoints of `Grid Trading` require authentication.

### POST / Place grid algo order

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID + Instrument ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/order-algo`

> Request Example

```
Copy to Clipboard
# Place spot grid algo order
POST /api/v5/tradingBot/grid/order-algo
body
{
    "instId": "BTC-USDT",
    "algoOrdType": "grid",
    "maxPx": "5000",
    "minPx": "400",
    "gridNum": "10",
    "runType": "1",
    "quoteSz": "25",
    "triggerParams":[
      {
         "triggerAction":"stop",
         "triggerStrategy":"price",
         "triggerPx":"1000"
      }
    ]
}

# Place contract grid algo order
POST /api/v5/tradingBot/grid/order-algo
body
{
    "instId": "BTC-USDT-SWAP",
    "algoOrdType": "contract_grid",
    "maxPx": "5000",
    "minPx": "400",
    "gridNum": "10",
    "runType": "1",
    "sz": "200",
    "direction": "long",
    "lever": "2",
    "triggerParams":[
      {
         "triggerAction":"start",
         "triggerStrategy":"rsi",
         "timeframe":"30m",
         "thold":"10",
         "triggerCond":"cross",
         "timePeriod":"14"
      },
      {
         "triggerAction":"stop",
         "triggerStrategy":"price",
         "triggerPx":"1000",
         "stopType":"2"
      }
   ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| maxPx | String | Yes | Upper price of price range |
| minPx | String | Yes | Lower price of price range |
| gridNum | String | Yes | Grid quantity |
| runType | String | No | Grid type<br>`1`: Arithmetic, `2`: Geometric<br>Default is Arithmetic |
| tpTriggerPx | String | No | TP tigger price<br>Applicable to `Spot grid`/`Contract grid` |
| slTriggerPx | String | No | SL tigger price<br>Applicable to `Spot grid`/`Contract grid` |
| algoClOrdId | String | No | Client-supplied Algo ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag |
| profitSharingRatio | String | No | Profit sharing ratio, it only supports these values<br>`0`,`0.1`,`0.2`,`0.3`<br> 0.1 represents 10% |
| triggerParams | Array of objects | No | Trigger Parameters<br>Applicable to `Spot grid`/`Contract grid` |
| \> triggerAction | String | Yes | Trigger action<br>`start`<br>`stop` |
| \> triggerStrategy | String | Yes | Trigger strategy<br>`instant`<br>`price`<br>`rsi`<br>Default is `instant` |
| \> delaySeconds | String | No | Delay seconds after action triggered |
| \> timeframe | String | No | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> thold | String | No | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerCond | String | No | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> timePeriod | String | No | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerPx | String | No | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| \> stopType | String | No | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |

Spot Grid Order

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| quoteSz | String | Conditional | Invest amount for quote currency<br>Either `quoteSz` or `baseSz` is required |
| baseSz | String | Conditional | Invest amount for base currency<br>Either `quoteSz` or `baseSz` is required |
| tradeQuoteCcy | String | No | The quote currency for trading. Only applicable to SPOT.<br>The default value is the quote currency of instId, e.g. USD for BTC-USD. |

Contract Grid Order

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| sz | String | Yes | Used margin based on `USDT` |
| direction | String | Yes | Contract grid type<br>`long`,`short`,`neutral` |
| lever | String | Yes | Leverage |
| basePos | Boolean | No | Whether or not open a position when the strategy activates <br>Default is `false`<br>Neutral contract grid should omit the parameter |
| tpRatio | String | No | Take profit ratio, 0.1 represents 10% |
| slRatio | String | No | Stop loss ratio, 0.1 represents 10% |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "447053782921515008",
            "sCode": "0",
            "sMsg": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |
| tag | String | Order tag |

### POST / Amend grid algo order basic param

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/amend-algo-basic-param`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/amend-algo-basic-param
body
    {
      "algoId": "448965992920907776",
      "maxPx": "100",
      "minPx": "10",
      "gridNum": "5"
      "topupAmount": "123.45"
    }
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| minPx | String | Yes | Minimum price range |
| maxPx | String | Yes | Maximum price range |
| gridNum | String | Yes | Grid quantity |
| topupAmount | String | No | Contract grid only. Optional client-supplied top up investment amount. If this is not supplied or explicitly supplied as "0", the required top up investment amount to edit grid parameters is topped up by default |

> Response Example

```
Copy to Clipboard
{
    "code": "55186",
    "msg": "Due to market fluctuations, your investment amount is too large to apply these modifications.",
    "data": [
        {
            "algoId": "4283223775520665600",
            "maxTopupAmount": "12456.78",
            "requiredTopupAmount": "12.34"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| requiredTopupAmount | String | Required top up investment amount to edit grid parameters. |
| maxTopupAmount | String | Contract grid only. Max top up investment amount to edit grid parameters |

#### Error Code

| **Error Code** | **HTTP Status code** | **Error Message** |
| --- | --- | --- |
| 51000 | 400 | Parameter {param} error. This would also be returned when a grid bot does not support the parameter |
| 51346 | 400 | Upper limit must be greater than the lower limit price. |
| 55123 | 400 | There is insufficient balance for transfer. |
| 55124 | 200 | Due to market fluctuations, your investment amount is insufficient to apply these modifications. |
| 55186 | 200 | Due to market fluctuations, your investment amount is too large to apply these modifications. |

### POST / Amend grid algo order

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/amend-order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/amend-order-algo
body
{
    "algoId":"448965992920907776",
    "instId":"BTC-USDT-SWAP",
    "slTriggerPx":"1200",
    "tpTriggerPx":""
}

POST /api/v5/tradingBot/grid/amend-order-algo
body
{
   "algoId":"578963447615062016",
   "instId":"BTC-USDT",
   "triggerParams":[
       {
           "triggerAction":"stop",
           "triggerStrategy":"price",
           "triggerPx":"1000"
       }
   ]
}

POST /api/v5/tradingBot/grid/amend-order-algo
body
{
   "algoId":"578963447615062016",
   "instId":"BTC-USDT-SWAP",
   "triggerParams":[
       {
           "triggerAction":"stop",
           "triggerStrategy":"instant",
           "stopType":"1"
       }
   ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| slTriggerPx | String | No | New stop-loss trigger price<br>if slTriggerPx is set "" means stop-loss trigger price is canceled.<br>Either `slTriggerPx` or `tpTriggerPx` is required. |
| tpTriggerPx | String | No | New take-profit trigger price<br>if tpTriggerPx is set "" means take-profit trigger price is canceled. |
| tpRatio | String | No | Take profit ratio, 0.1 represents 10%, only applicable to contract grid<br>if it is set "" means take-profit ratio is canceled. |
| slRatio | String | No | Stop loss ratio, 0.1 represents 10%, only applicable to contract grid\`<br>if it is set "" means stop-loss ratio is canceled. |
| topUpAmt | String | No | Top up amount, only applicable to spot grid |
| triggerParams | Array of objects | No | Trigger Parameters |
| \> triggerAction | String | Yes | Trigger action<br>`start`<br>`stop` |
| \> triggerStrategy | String | Yes | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| \> triggerPx | String | No | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| \> stopType | String | No | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoClOrdId": "",
            "algoId":"448965992920907776",
            "sCode":"0",
            "sMsg":"",
            "tag": ""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |
| tag | String | Order tag |

### POST / Stop grid algo order

A maximum of 10 orders can be stopped per request.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/stop-order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/stop-order-algo
body
[
    {
        "algoId":"448965992920907776",
        "instId":"BTC-USDT",
        "stopType":"1",
        "algoOrdType":"grid"
    }
]
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| stopType | String | Yes | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "448965992920907776",
            "sCode": "0",
            "sMsg": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection message if the request is unsuccessful. |
| tag | String | Order tag |

### POST / Close position for contract grid

Close position when the contract grid stop type is 'keep position'.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/close-position`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/close-position
body
{
    "algoId":"448965992920907776",
    "mktClose":true
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| mktClose | Boolean | Yes | Market close all the positions or not<br>`true`: Market close all position, `false`: Close part of position |
| sz | String | Conditional | Close position amount, with unit of `contract`<br>If `mktClose` is `false`, the parameter is required. |
| px | String | Conditional | Close position price<br>If `mktClose` is `false`, the parameter is required. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "448965992920907776",
            "ordId": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| ordId | String | Close position order ID<br>If `mktClose` is `true`, the parameter will return "". |
| algoClOrdId | String | Client-supplied Algo ID |
| tag | String | Order tag |

### POST / Cancel close position order for contract grid

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/cancel-close-order`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/cancel-close-order
body
{
    "algoId":"448965992920907776",
    "ordId":"570627699870375936"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| ordId | String | Yes | Close position order ID |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoClOrdId": "",
            "algoId": "448965992920907776",
            "ordId": "570627699870375936",
            "tag": ""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| ordId | String | Close position order ID |
| algoClOrdId | String | Client-supplied Algo ID |
| tag | String | Order tag |

### POST / Instant trigger grid algo order

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID + Instrument ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/order-instant-trigger`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/order-instant-trigger
body
{
    "algoId":"561564133246894080"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| topUpAmt | String | No | Top up amount, only applicable to spot grid |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "561564133246894080"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |

### GET / Grid algo order list

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/orders-algo-pending`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/orders-algo-pending?algoOrdType=grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| algoId | String | No | Algo ID |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| instType | String | No | Instrument type<br>`SPOT`<br>`MARGIN`<br>`FUTURES`<br>`SWAP` |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100 |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "actualLever": "",
            "algoClOrdId": "",
            "algoId": "56802********64032",
            "algoOrdType": "grid",
            "arbitrageNum": "0",
            "availEq": "",
            "basePos": false,
            "baseSz": "0",
            "cTime": "1681700496249",
            "cancelType": "0",
            "direction": "",
            "floatProfit": "0",
            "gridNum": "10",
            "gridProfit": "0",
            "instFamily": "",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "investment": "25",
            "lever": "",
            "liqPx": "",
            "maxPx": "5000",
            "minPx": "400",
            "ordFrozen": "",
            "pnlRatio": "0",
            "quoteSz": "25",
            "rebateTrans": [
                {
                    "rebate": "0",
                    "rebateCcy": "BTC"
                },
                {
                    "rebate": "0",
                    "rebateCcy": "USDT"
                }
            ],
            "runType": "1",
            "slTriggerPx": "",
            "state": "running",
            "stopType": "",
            "sz": "",
            "tag": "",
            "totalPnl": "0",
            "tpTriggerPx": "",
            "triggerParams": [
                {
                    "triggerAction": "start",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "triggerType": "auto",
                    "triggerTime": ""
                },
                {
                    "triggerAction": "stop",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "stopType": "1",
                    "triggerPx": "1000",
                    "triggerType": "manual",
                    "triggerTime": ""
                }
            ],
            "uTime": "1682062564350",
            "uly": "BTC-USDT",
            "profitSharingRatio": "",
            "copyType": "0",
            "fee": "",
            "fundingFee": "",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| state | String | Algo order state<br>`starting`<br>`running`<br>`stopping`<br>`pending_signal`<br>`no_close_position`: stopped algo order but have not closed position yet |
| rebateTrans | Array of objects | Rebate transfer info |
| \> rebate | String | Rebate amount |
| \> rebateCcy | String | Rebate currency |
| triggerParams | Array of objects | Trigger Parameters |
| \> triggerAction | String | Trigger action<br>`start`<br>`stop` |
| \> triggerStrategy | String | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| \> delaySeconds | String | Delay seconds after action triggered |
| \> triggerTime | String | Actual action triggered time, unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> triggerType | String | Actual action triggered type<br>`manual`<br>`auto` |
| \> timeframe | String | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> thold | String | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerCond | String | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> timePeriod | String | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerPx | String | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| \> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |
| maxPx | String | Upper price of price range |
| minPx | String | Lower price of price range |
| gridNum | String | Grid quantity |
| runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| tpTriggerPx | String | Take-profit trigger price |
| slTriggerPx | String | Stop-loss trigger price |
| arbitrageNum | String | The number of arbitrages executed |
| totalPnl | String | Total P&L |
| pnlRatio | String | P&L ratio |
| investment | String | Accumulated investment amount<br>Spot grid investment amount calculated on quote currency |
| gridProfit | String | Grid profit |
| floatProfit | String | Variable P&L |
| cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop<br>`2`: Take profit<br>`3`: Stop loss<br>`4`: Risk control<br>`5`: Delivery<br>`6`: Signal |
| stopType | String | Actual Stop type<br>Spot `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions |
| quoteSz | String | Quote currency investment amount<br>Only applicable to `Spot grid` |
| baseSz | String | Base currency investment amount<br>Only applicable to `Spot grid` |
| direction | String | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to `contract grid` |
| basePos | Boolean | Whether or not to open a position when the strategy is activated<br>Only applicable to `contract grid` |
| sz | String | Used margin based on `USDT`<br>Only applicable to `contract grid` |
| lever | String | Leverage<br>Only applicable to `contract grid` |
| actualLever | String | Actual Leverage<br>Only applicable to `contract grid` |
| liqPx | String | Estimated liquidation price<br>Only applicable to `contract grid` |
| uly | String | Underlying<br>Only applicable to `contract grid` |
| instFamily | String | Instrument family<br>Only applicable to `FUTURES`/`SWAP`/`OPTION`<br>Only applicable to `contract grid` |
| ordFrozen | String | Margin used by pending orders<br>Only applicable to `contract grid` |
| availEq | String | Available margin<br>Only applicable to `contract grid` |
| tag | String | Order tag |
| profitSharingRatio | String | Profit sharing ratio<br>Value range \[0, 0.3\]<br>If it is a normal order (neither copy order nor lead order), this field returns "" |
| copyType | String | Profit sharing order type<br>`0`: Normal order<br>`1`: Copy order without profit sharing<br>`2`: Copy order with profit sharing<br>`3`: Lead order |
| fee | String | Accumulated fee. Only applicable to contract grid, or it will be "" |
| fundingFee | String | Accumulated funding fee. Only applicable to contract grid, or it will be "" |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Grid algo order history

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/orders-algo-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/orders-algo-history?algoOrdType=grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| algoId | String | No | Algo ID |
| instId | String | No | Instrument ID, e.g. `BTC-USDT` |
| instType | String | No | Instrument type<br>`SPOT`<br>`MARGIN`<br>`FUTURES`<br>`SWAP` |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "actualLever": "",
            "algoClOrdId": "",
            "algoId": "565849588675117056",
            "algoOrdType": "grid",
            "arbitrageNum": "0",
            "availEq": "",
            "basePos": false,
            "baseSz": "0",
            "cTime": "1681181054927",
            "cancelType": "1",
            "direction": "",
            "floatProfit": "0",
            "gridNum": "10",
            "gridProfit": "0",
            "instFamily": "",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "investment": "25",
            "lever": "0",
            "liqPx": "",
            "maxPx": "5000",
            "minPx": "400",
            "ordFrozen": "",
            "pnlRatio": "0",
            "quoteSz": "25",
            "rebateTrans": [
                {
                    "rebate": "0",
                    "rebateCcy": "BTC"
                },
                {
                    "rebate": "0",
                    "rebateCcy": "USDT"
                }
            ],
            "runType": "1",
            "slTriggerPx": "0",
            "state": "stopped",
            "stopResult": "0",
            "stopType": "1",
            "sz": "",
            "tag": "",
            "totalPnl": "0",
            "tpTriggerPx": "0",
            "triggerParams": [
                {
                    "triggerAction": "start",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "triggerType": "auto",
                    "triggerTime": ""
                },
                {
                    "triggerAction": "stop",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "stopType": "1",
                    "triggerPx": "1000",
                    "triggerType": "manual",
                    "triggerTime": "1681181186484"
                }
            ],
            "uTime": "1681181186496",
            "uly": "BTC-USDT",
            "profitSharingRatio": "",
            "copyType": "0",
            "fee": "",
            "fundingFee": "",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| state | String | Algo order state<br>`stopped` |
| rebateTrans | Array of objects | Rebate transfer info |
| \> rebate | String | Rebate amount |
| \> rebateCcy | String | Rebate currency |
| triggerParams | Array of objects | Trigger Parameters |
| \> triggerAction | String | Trigger action<br>`start`<br>`stop` |
| \> triggerStrategy | String | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| \> delaySeconds | String | Delay seconds after action triggered |
| \> triggerTime | String | Actual action triggered time, unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> triggerType | String | Actual action triggered type<br>`manual`<br>`auto` |
| \> timeframe | String | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> thold | String | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerCond | String | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> timePeriod | String | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerPx | String | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| \> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |
| maxPx | String | Upper price of price range |
| minPx | String | Lower price of price range |
| gridNum | String | Grid quantity |
| runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| tpTriggerPx | String | Take-profit trigger price |
| slTriggerPx | String | Stop-loss trigger price |
| arbitrageNum | String | The number of arbitrages executed |
| totalPnl | String | Total P&L |
| pnlRatio | String | P&L ratio |
| investment | String | Accumulated investment amount<br>Spot grid investment amount calculated on quote currency |
| gridProfit | String | Grid profit |
| floatProfit | String | Variable P&L |
| cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop<br>`2`: Take profit<br>`3`: Stop loss<br>`4`: Risk control<br>`5`: Delivery<br>`6`: Signal |
| stopType | String | Actual Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions |
| quoteSz | String | Quote currency investment amount<br>Only applicable to `Spot grid` |
| baseSz | String | Base currency investment amount<br>Only applicable to `Spot grid` |
| direction | String | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to `contract grid` |
| basePos | Boolean | Whether or not to open a position when the strategy is activated<br>Only applicable to `contract grid` |
| sz | String | Used margin based on `USDT`<br>Only applicable to `contract grid` |
| lever | String | Leverage<br>Only applicable to `contract grid` |
| actualLever | String | Actual Leverage<br>Only applicable to `contract grid` |
| liqPx | String | Estimated liquidation price<br>Only applicable to `contract grid` |
| uly | String | Underlying<br>Only applicable to `contract grid` |
| instFamily | String | Instrument family<br>Only applicable to `FUTURES`/`SWAP`/`OPTION`<br>Only applicable to `contract grid` |
| ordFrozen | String | Margin used by pending orders<br>Only applicable to `contract grid` |
| availEq | String | Available margin<br>Only applicable to `contract grid` |
| tag | String | Order tag |
| profitSharingRatio | String | Profit sharing ratio<br>Value range \[0, 0.3\]<br>If it is a normal order (neither copy order nor lead order), this field returns "" |
| copyType | String | Profit sharing order type<br>`0`: Normal order<br>`1`: Copy order without profit sharing<br>`2`: Copy order with profit sharing<br>`3`: Lead order |
| fee | String | Accumulated fee. Only applicable to contract grid, or it will be "" |
| fundingFee | String | Accumulated funding fee. Only applicable to contract grid, or it will be "" |
| stopResult | String | Stop result<br>`0`: default, `1`: Successful selling of currency at market price, `-1`: Failed to sell currency at market price<br>Only applicable to `Spot grid` |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Grid algo order details

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/orders-algo-details`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/orders-algo-details?algoId=448965992920907776&algoOrdType=grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "actualLever": "",
            "activeOrdNum": "0",
            "algoClOrdId": "",
            "algoId": "448965992920907776",
            "algoOrdType": "grid",
            "annualizedRate": "0",
            "arbitrageNum": "0",
            "availEq": "",
            "basePos": false,
            "baseSz": "0",
            "cTime": "1681181054927",
            "cancelType": "1",
            "curBaseSz": "0",
            "curQuoteSz": "0",
            "direction": "",
            "eq": "",
            "floatProfit": "0",
            "gridNum": "10",
            "gridProfit": "0",
            "instFamily": "",
            "instId": "BTC-USDT",
            "instType": "SPOT",
            "investment": "25",
            "lever": "0",
            "liqPx": "",
            "maxPx": "5000",
            "minPx": "400",
            "ordFrozen": "",
            "perMaxProfitRate": "1.14570215",
            "perMinProfitRate": "0.0991200440528634356837",
            "pnlRatio": "0",
            "profit": "0.00000000",
            "quoteSz": "25",
            "rebateTrans": [
                {
                    "rebate": "0",
                    "rebateCcy": "BTC"
                },
                {
                    "rebate": "0",
                    "rebateCcy": "USDT"
                }
            ],
            "runType": "1",
            "runPx": "30089.7",
            "singleAmt": "0.00101214",
            "slTriggerPx": "0",
            "state": "stopped",
            "stopResult": "0",
            "stopType": "1",
            "sz": "",
            "tag": "",
            "totalAnnualizedRate": "0",
            "totalPnl": "0",
            "tpTriggerPx": "0",
            "tradeNum": "0",
            "triggerParams": [
                {
                    "triggerAction": "start",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "triggerType": "auto",
                    "triggerTime": ""
                },
                {
                    "triggerAction": "stop",
                    "delaySeconds": "0",
                    "triggerStrategy": "instant",
                    "stopType": "1",
                    "triggerType": "manual",
                    "triggerTime": "1681181186484"
                }
            ],
            "uTime": "1681181186496",
            "uly": "",
            "profitSharingRatio": "",
            "copyType": "0",
            "tpRatio": "",
            "slRatio": "",
            "fee": "",
            "fundingFee": "",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| state | String | Algo order state<br>`starting`<br>`running`<br>`stopping`<br>`no_close_position`: stopped algo order but have not closed position yet<br>`stopped` |
| rebateTrans | Array of objects | Rebate transfer info |
| \> rebate | String | Rebate amount |
| \> rebateCcy | String | Rebate currency |
| triggerParams | Array of objects | Trigger Parameters |
| \> triggerAction | String | Trigger action<br>`start`<br>`stop` |
| \> triggerStrategy | String | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| \> delaySeconds | String | Delay seconds after action triggered |
| \> triggerTime | String | Actual action triggered time, unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> triggerType | String | Actual action triggered type<br>`manual`<br>`auto` |
| \> timeframe | String | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> thold | String | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerCond | String | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> timePeriod | String | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| \> triggerPx | String | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| \> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |
| maxPx | String | Upper price of price range |
| minPx | String | Lower price of price range |
| gridNum | String | Grid quantity |
| runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| tpTriggerPx | String | Take-profit trigger price |
| slTriggerPx | String | Stop-loss trigger price |
| tradeNum | String | The number of trades executed |
| arbitrageNum | String | The number of arbitrages executed |
| singleAmt | String | Amount per grid |
| perMinProfitRate | String | Estimated minimum Profit margin per grid |
| perMaxProfitRate | String | Estimated maximum Profit margin per grid |
| runPx | String | Price at launch |
| totalPnl | String | Total P&L |
| pnlRatio | String | P&L ratio |
| investment | String | Accumulated investment amount<br>Spot grid investment amount calculated on quote currency |
| gridProfit | String | Grid profit |
| floatProfit | String | Variable P&L |
| totalAnnualizedRate | String | Total annualized rate |
| annualizedRate | String | Grid annualized rate |
| cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop<br>`2`: Take profit<br>`3`: Stop loss<br>`4`: Risk control<br>`5`: Delivery<br>`6`: Signal |
| stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions |
| activeOrdNum | String | Total count of pending sub orders |
| quoteSz | String | Quote currency investment amount<br>Only applicable to `Spot grid` |
| baseSz | String | Base currency investment amount<br>Only applicable to `Spot grid` |
| curQuoteSz | String | Assets of quote currency currently held<br>Only applicable to `Spot grid` |
| curBaseSz | String | Assets of base currency currently held<br>Only applicable to `Spot grid` |
| profit | String | Current available profit based on quote currency<br>Only applicable to `Spot grid` |
| stopResult | String | Stop result<br>`0`: default, `1`: Successful selling of currency at market price, `-1`: Failed to sell currency at market price<br>Only applicable to `Spot grid` |
| direction | String | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to `contract grid` |
| basePos | Boolean | Whether or not to open a position when the strategy is activated<br>Only applicable to `contract grid` |
| sz | String | Used margin based on `USDT`<br>Only applicable to `contract grid` |
| lever | String | Leverage<br>Only applicable to `contract grid` |
| actualLever | String | Actual Leverage<br>Only applicable to `contract grid` |
| liqPx | String | Estimated liquidation price<br>Only applicable to `contract grid` |
| uly | String | Underlying<br>Only applicable to `contract grid` |
| instFamily | String | Instrument family<br>Only applicable to `FUTURES`/`SWAP`/`OPTION`<br>Only applicable to `contract grid` |
| ordFrozen | String | Margin used by pending orders<br>Only applicable to `contract grid` |
| availEq | String | Available margin<br>Only applicable to `contract grid` |
| eq | String | Total equity of strategy account<br>Only applicable to `contract grid` |
| tag | String | Order tag |
| profitSharingRatio | String | Profit sharing ratio<br>Value range \[0, 0.3\]<br>If it is a normal order (neither copy order nor lead order), this field returns "" |
| copyType | String | Profit sharing order type<br>`0`: Normal order<br>`1`: Copy order without profit sharing<br>`2`: Copy order with profit sharing<br>`3`: Lead order |
| tpRatio | String | Take profit ratio, 0.1 represents 10% |
| slRatio | String | Stop loss ratio, 0.1 represents 10% |
| fee | String | Accumulated fee. Only applicable to contract grid, or it will be "" |
| fundingFee | String | Accumulated funding fee. Only applicable to contract grid, or it will be "" |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Grid algo sub orders

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/sub-orders`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/sub-orders?algoId=123456&type=live&algoOrdType=grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| algoId | String | Yes | Algo ID |
| type | String | Yes | Sub order state<br>`live`<br>`filled` |
| groupId | String | No | Group ID |
| after | String | No | Pagination of data to return records earlier than the requested `ordId`. |
| before | String | No | Pagination of data to return records newer than the requested `ordId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100 |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0",
            "algoClOrdId": "",
            "algoId": "448965992920907776",
            "algoOrdType": "grid",
            "avgPx": "0",
            "cTime": "1653347949771",
            "ccy": "",
            "ctVal": "",
            "fee": "0",
            "feeCcy": "USDC",
            "groupId": "3",
            "instId": "BTC-USDC",
            "instType": "SPOT",
            "lever": "0",
            "ordId": "449109084439187456",
            "ordType": "limit",
            "pnl": "0",
            "posSide": "net",
            "px": "30404.3",
            "rebate": "0",
            "rebateCcy": "USDT",
            "side": "sell",
            "state": "live",
            "sz": "0.00059213",
            "tag": "",
            "tdMode": "cash",
            "uTime": "1653347949831"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| groupId | String | Group ID |
| ordId | String | Sub order ID |
| cTime | String | Sub order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Sub order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| tdMode | String | Sub order trade mode<br>Margin mode: `cross`/`isolated`<br>Non-Margin mode: `cash` |
| ccy | String | Margin currency<br>Only applicable to cross MARGIN orders in `Futures mode`. |
| ordType | String | Sub order type<br>`market`: Market order<br>`limit`: Limit order<br>`ioc`: Immediate-or-cancel order |
| sz | String | Sub order quantity to buy or sell |
| state | String | Sub order state<br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`cancelling` |
| side | String | Sub order side<br>`buy``sell` |
| px | String | Sub order price |
| fee | String | Sub order fee amount |
| feeCcy | String | Sub order fee currency |
| rebate | String | Sub order rebate amount |
| rebateCcy | String | Sub order rebate currency |
| avgPx | String | Sub order average filled price |
| accFillSz | String | Sub order accumulated fill quantity |
| posSide | String | Sub order position side<br>`net` |
| pnl | String | Sub order profit and loss |
| ctVal | String | Contract value<br>Only applicable to `FUTURES`/`SWAP` |
| lever | String | Leverage |
| tag | String | Order tag |

### GET / Grid algo order positions

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/positions`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/positions?algoId=448965992920907776&algoOrdType=contract_grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`contract_grid`: Contract grid |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "adl": "1",
            "algoClOrdId": "",
            "algoId": "449327675342323712",
            "avgPx": "29215.0142857142857149",
            "cTime": "1653400065917",
            "ccy": "USDT",
            "imr": "2045.386",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "last": "29206.7",
            "lever": "5",
            "liqPx": "661.1684795867162",
            "markPx": "29213.9",
            "mgnMode": "cross",
            "mgnRatio": "217.19370606167573",
            "mmr": "40.907720000000005",
            "notionalUsd": "10216.70307",
            "pos": "35",
            "posSide": "net",
            "uTime": "1653400066938",
            "upl": "1.674999999999818",
            "uplRatio": "0.0008190504784478"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID, e.g. `BTC-USDT-SWAP` |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| avgPx | String | Average open price |
| ccy | String | Margin currency |
| lever | String | Leverage |
| liqPx | String | Estimated liquidation price |
| posSide | String | Position side<br>`net` |
| pos | String | Quantity of positions |
| mgnMode | String | Margin mode<br>`cross`<br>`isolated` |
| mgnRatio | String | Maintenance margin ratio |
| imr | String | Initial margin requirement |
| mmr | String | Maintenance margin requirement |
| upl | String | Unrealized profit and loss |
| uplRatio | String | Unrealized profit and loss ratio |
| last | String | Latest traded price |
| notionalUsd | String | Notional value of positions in `USD` |
| adl | String | Automatic-Deleveraging, signal area<br>Divided into 5 levels, from 1 to 5, the smaller the number, the weaker the adl intensity. |
| markPx | String | Mark price |

### POST / Spot grid withdraw income

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/withdraw-income`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/withdraw-income
body
{
    "algoId":"448965992920907776"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoClOrdId": "",
            "algoId":"448965992920907776",
            "profit":"100"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| profit | String | Withdraw profit |

### POST / Compute margin balance

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/compute-margin-balance`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/compute-margin-balance
body {
   "algoId":"123456",
   "type":"add",
   "amt":"10"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| type | String | Yes | Adjust margin balance type<br>`add``reduce` |
| amt | String | No | Adjust margin balance amount<br>Default is zero. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "lever": "0.3877200981166066",
            "maxAmt": "1.8309562403342999"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| maxAmt | String | Maximum adjustable margin balance amount |
| lever | String | Leverage after adjustment of margin balance |

### POST / Adjust margin balance

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/margin-balance`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/margin-balance
body {
   "algoId":"123456",
   "type":"add",
   "amt":"10"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| type | String | Yes | Adjust margin balance type<br>`add``reduce` |
| amt | String | Conditional | Adjust margin balance amount<br>Either `amt` or `percent` is required. |
| percent | String | Conditional | Adjust margin balance percentage |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "123456"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |

### POST / Add investment

It is used to add investment and only applicable to contract gird.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/grid/adjust-investment`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/adjust-investment
body
{
    "algoId":"448965992920907776",
    "amt":"12"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| amt | String | Yes | The amount is going to be added |
| allowReinvestProfit | String | No | Whether reinvesting profits, only applicable to spot grid.<br>`true` or `false`. The default is true. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "448965992920907776"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |

### GET / Grid AI parameter (public)

Authentication is not required for this public endpoint.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/ai-param`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/ai-param?instId=BTC-USDT&algoOrdType=grid
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| direction | String | Conditional | Contract grid type<br>`long`,`short`,`neutral`<br>Required in the case of `contract_grid` |
| duration | String | No | Back testing duration<br>`7D`: 7 Days, `30D`: 30 Days, `180D`: 180 Days<br>The default is `7D` for `Spot grid`<br>Only `7D` is available for `Contract grid` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoOrdType": "grid",
            "annualizedRate": "1.5849",
            "ccy": "USDT",
            "direction": "",
            "duration": "7D",
            "gridNum": "5",
            "instId": "BTC-USDT",
            "lever": "0",
            "maxPx": "21373.3",
            "minInvestment": "0.89557758",
            "minPx": "15544.2",
            "perGridProfitRatio": "4.566226200302574",
            "perMaxProfitRate": "0.0733865364573281",
            "perMinProfitRate": "0.0561101403446263",
            "runType": "1",
            "sourceCcy": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| duration | String | Back testing duration<br>`7D`: 7 Days, `30D`: 30 Days, `180D`: 180 Days |
| gridNum | String | Grid quantity |
| maxPx | String | Upper price of price range |
| minPx | String | Lower price of price range |
| perMaxProfitRate | String | Estimated maximum Profit margin per grid |
| perMinProfitRate | String | Estimated minimum Profit margin per grid |
| perGridProfitRatio | String | Per grid profit ratio |
| annualizedRate | String | Grid annualized rate |
| minInvestment | String | The minimum invest amount |
| ccy | String | The invest currency |
| runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| direction | String | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to contract grid |
| lever | String | Leverage<br>Only applicable to contract grid |
| sourceCcy | String | Source currency |

### POST / Compute min investment (public)

Authentication is not required for this public endpoint.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP Request

`POST /api/v5/tradingBot/grid/min-investment`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/grid/min-investment
body
{
    "instId": "ETH-USDT",
    "algoOrdType":"grid",
    "gridNum": "50",
    "maxPx":"5000",
    "minPx":"3000",
    "runType":"1",
    "investmentData":[
        {
            "amt":"0.01",
            "ccy":"ETH"
        },
        {
            "amt":"100",
            "ccy":"USDT"
        }
    ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| maxPx | String | Yes | Upper price of price range |
| minPx | String | Yes | Lower price of price range |
| gridNum | String | Yes | Grid quantity |
| runType | String | Yes | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| direction | String | Conditional | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to `contract grid` |
| lever | String | Conditional | Leverage<br>Only applicable to `contract grid` |
| basePos | Boolean | No | Whether or not open a position when the strategy activates<br>Default is `false`<br>Neutral contract grid should omit the parameter<br>Only applicable to `contract grid` |
| investmentType | String | No | Investment type, only applicable to `grid`<br>`quote`<br>`base`<br>`dual` |
| triggerStrategy | String | No | Trigger stragety, <br>`instant`<br>`price`<br>`rsi` |
| topUpAmt | String | No | Top up amount, only applicable to spot grid |
| investmentData | Array of objects | No | Invest Data |
| \> amt | String | Yes | Invest amount |
| \> ccy | String | Yes | Invest currency |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
           "minInvestmentData": [
               {
                   "amt":"0.1",
                   "ccy":"ETH"
               },
               {
                   "amt":"100",
                   "ccy":"USDT"
               }
           ],
           "singleAmt":"10"
       }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| minInvestmentData | Array of objects | Minimum invest Data |
| \> amt | String | Minimum invest amount |
| \> ccy | String | Minimum Invest currency |
| singleAmt | String | Single grid trading amount<br>In terms of `spot grid`, the unit is `quote currency`<br>In terms of `contract grid`, the unit is `contract` |

### GET / RSI back testing (public)

Authentication is not required for this public endpoint.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/public/rsi-back-testing`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/public/rsi-back-testing?instId=BTC-USDT&thold=30&timeframe=3m&timePeriod=14
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT`<br>Only applicable to `SPOT` |
| timeframe | String | Yes | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day) |
| thold | String | Yes | Threshold<br>The value should be an integer between 1 to 100 |
| timePeriod | String | Yes | Time Period<br>`14` |
| triggerCond | String | No | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>Default is `cross_down` |
| duration | String | No | Back testing duration<br>`1M` (`M`: month)<br>Default is `1M` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "triggerNum": "164"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| triggerNum | String | Trigger number |

### GET / Max grid quantity (public)

Authentication is not required for this public endpoint.

Maximum grid quantity can be retrieved from this endpoint. Minimum grid quantity always is 2.

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/grid/grid-quantity`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/grid/grid-quantity?instId=BTC-USDT-SWAP&runType=1&algoOrdType=contract_grid&maxPx=70000&minPx=50000&lever=5
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| runType | String | Yes | Grid type<br>`1`: Arithmetic<br>`2`: Geometric |
| algoOrdType | String | Yes | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| maxPx | String | Yes | Upper price of price range |
| minPx | String | Yes | Lower price of price range |
| lever | String | Conditional | Leverage, it is required for contract grid |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "maxGridQty": "285"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| maxGridQty | String | Maximum grid quantity |

### WS / Spot grid algo orders channel

Retrieve spot grid algo orders. Data will be pushed when triggered by events such as placing/canceling order. It will also be pushed in regular interval according to subscription granularity.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "grid-orders-spot",
        "instType": "SPOT"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "grid-orders-spot",
        "instType": "SPOT"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`grid-orders-spot` |
| \> instType | String | Yes | Instrument type<br>`SPOT`<br>`ANY` |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "grid-orders-spot",
        "instType": "ANY"
    },
    "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"grid-orders-spot\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "grid-orders-spot",
        "instType": "ANY",
        "uid": "44705892343619584"
    },
    "data": [{
        "algoClOrdId": "",
        "algoId": "568028283477164032",
        "activeOrdNum" : "10",
        "algoOrdType": "grid",
        "annualizedRate": "0",
        "arbitrageNum": "0",
        "baseSz": "0",
        "cTime": "1681700496249",
        "cancelType": "0",
        "curBaseSz": "0",
        "curQuoteSz": "25",
        "floatProfit": "0",
        "gridNum": "10",
        "gridProfit": "0",
        "instId": "BTC-USDT",
        "instType": "SPOT",
        "investment": "25",
        "maxPx": "5000",
        "minPx": "400",
        "pTime": "1682416738467",
        "perMaxProfitRate": "1.14570215",
        "perMinProfitRate": "0.0991200440528634356837",
        "pnlRatio": "0",
        "profit": "0",
        "quoteSz": "25",
        "rebateTrans": [{
            "rebate": "0",
            "rebateCcy": "BTC"
        }, {
            "rebate": "0",
            "rebateCcy": "USDT"
        }],
        "runPx": "30031.7",
        "runType": "1",
        "triggerParams": [{
            "triggerAction": "start",
            "triggerStrategy": "instant",
            "delaySeconds": "0",
            "triggerType": "auto",
            "triggerTime": ""
        }, {
            "triggerAction": "stop",
            "triggerStrategy": "instant",
            "delaySeconds": "0",
            "stopType": "1",
            "triggerType": "manual",
            "triggerTime": ""
        }],
        "singleAmt": "0.00101214",
        "slTriggerPx": "",
        "state": "running",
        "stopResult": "0",
        "stopType": "2",
        "tag": "",
        "totalAnnualizedRate": "0",
        "totalPnl": "0",
        "tpTriggerPx": "",
        "tradeNum": "0",
        "uTime": "1682406665527",
        "profitSharingRatio": "",
        "copyType": "0",
        "tradeQuoteCcy": "USDT"
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instType | String | Instrument type |
| \> uid | String | User ID |
| data | Array of objects | Subscribed data |
| \> algoId | String | Algo ID |
| \> algoClOrdId | String | Client-supplied Algo ID |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> algoOrdType | String | Algo order type<br>`grid`: Spot grid |
| \> state | String | Algo order state<br>`starting`<br>`running`<br>`stopping`<br>`stopped` |
| \> rebateTrans | Array of objects | Rebate transfer info |
| >\> rebate | String | Rebate amount |
| >\> rebateCcy | String | Rebate currency |
| \> triggerParams | Array of objects | Trigger Parameters |
| >\> triggerAction | String | Trigger action<br>`start`<br>`stop` |
| >\> triggerStrategy | String | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| >\> delaySeconds | String | Delay seconds after action triggered |
| >\> triggerTime | String | Actual action triggered time, unix timestamp format in milliseconds, e.g. `1597026383085` |
| >\> triggerType | String | Actual action triggered type<br>`manual`<br>`auto` |
| >\> timeframe | String | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> thold | String | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> triggerCond | String | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> timePeriod | String | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> triggerPx | String | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| >\> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |
| \> maxPx | String | Upper price of price range |
| \> minPx | String | Lower price of price range |
| \> gridNum | String | Grid quantity |
| \> runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| \> tpTriggerPx | String | Take-profit trigger price |
| \> slTriggerPx | String | Stop-loss trigger price |
| \> tradeNum | String | The number of trades executed |
| \> arbitrageNum | String | The number of arbitrages executed |
| \> singleAmt | String | Amount per grid |
| \> perMinProfitRate | String | Estimated minimum Profit margin per grid |
| \> perMaxProfitRate | String | Estimated maximum Profit margin per grid |
| \> runPx | String | Price at launch |
| \> totalPnl | String | Total P&L |
| \> pnlRatio | String | P&L ratio |
| \> investment | String | Investment amount<br>Spot grid investment amount calculated on quote currency |
| \> gridProfit | String | Grid profit |
| \> floatProfit | String | Variable P&L |
| \> totalAnnualizedRate | String | Total annualized rate |
| \> annualizedRate | String | Grid annualized rate |
| \> cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop<br>`2`: Take profit<br>`3`: Stop loss<br>`4`: Risk control<br>`5`: Delivery<br>`6`: Signal |
| \> stopType | String | Stop type<br>`1`: Sell base currency `2`: Keep base currency |
| \> quoteSz | String | Quote currency investment amount<br>Only applicable to `Spot grid` |
| \> baseSz | String | Base currency investment amount<br>Only applicable to `Spot grid` |
| \> curQuoteSz | String | Assets of quote currency currently held<br>Only applicable to `Spot grid` |
| \> curBaseSz | String | Assets of base currency currently held<br>Only applicable to `Spot grid` |
| \> profit | String | Current available profit based on quote currency<br>Only applicable to `Spot grid` |
| \> stopResult | String | Stop result<br>`0`: default, `1`: Successful selling of currency at market price, `-1`: Failed to sell currency at market price<br>Only applicable to `Spot grid` |
| \> activeOrdNum | String | Total count of pending sub orders |
| \> tag | String | Order tag |
| \> profitSharingRatio | String | Profit sharing ratio<br>Value range \[0, 0.3\]<br>If it is a normal order (neither copy order nor lead order), this field returns "" |
| \> copyType | String | Profit sharing order type<br>`0`: Normal order<br>`1`: Copy order without profit sharing<br>`2`: Copy order with profit sharing<br>`3`: Lead order |
| \> pTime | String | Push time of algo grid information, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> tradeQuoteCcy | String | The quote currency for trading. |

### WS / Contract grid algo orders channel

Retrieve contract grid algo orders. Data will be pushed when triggered by events such as placing/canceling order. It will also be pushed in regular interval according to subscription granularity.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "grid-orders-contract",
        "instType": "SWAP"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "grid-orders-contract",
        "instType": "SWAP"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`grid-orders-contract` |
| \> instType | String | Yes | Instrument type<br>`SWAP`<br>`FUTURES`<br>`ANY` |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "grid-orders-contract",
        "instType": "ANY"
    },
    "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"grid-orders-contract\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type |
| \> instId | String | No | Instrument ID |
| \> algoId | String | No | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "grid-orders-contract",
        "instType": "ANY",
        "uid": "4470****9584"
    },
    "data": [{
        "actualLever": "2.3481494635276649",
        "activeOrdNum": "10",
        "algoClOrdId": "",
        "algoId": "571039869070475264",
        "algoOrdType": "contract_grid",
        "annualizedRate": "0",
        "arbitrageNum": "0",
        "availEq": "52.3015392887089673",
        "basePos": true,
        "cTime": "1682418514204",
        "cancelType": "0",
        "direction": "long",
        "eq": "108.7945652387089673",
        "floatProfit": "8.7945652387089673",
        "gridNum": "10",
        "gridProfit": "0",
        "instId": "BTC-USDT-SWAP",
        "instType": "SWAP",
        "investment": "100",
        "lever": "5",
        "liqPx": "16370.482143120824",
        "maxPx": "36437.3",
        "minPx": "26931.9",
        "ordFrozen": "5.38638",
        "pTime": "1682492574068",
        "perMaxProfitRate": "0.1687494513302446",
        "perMinProfitRate": "0.1263869357706788",
        "pnlRatio": "0.0879456523870897",
        "rebateTrans": [{
            "rebate": "0",
            "rebateCcy": "USDT"
        }],
        "runPx": "27306.9",
        "runType": "1",
        "singleAmt": "1",
        "slTriggerPx": "",
        "state": "running",
        "stopType": "0",
        "sz": "100",
        "tag": "",
        "totalAnnualizedRate": "38.52019574554529",
        "totalPnl": "8.7945652387089673",
        "tpTriggerPx": "",
        "tradeNum": "9",
        "triggerParams": [{
            "triggerAction": "start",
            "delaySeconds": "0",
            "triggerStrategy": "price",
            "triggerPx": "1",
            "triggerType": "manual",
            "triggerTime": "1682418561497"
        }, {
            "triggerAction": "stop",
            "delaySeconds": "0",
            "triggerStrategy": "instant",
            "stopType": "1",
            "triggerType": "manual",
            "triggerTime": "0"
        }],
        "uTime": "1682492552257",
        "profitSharingRatio": "",
        "copyType": "0",
        "tpRatio": "",
        "slRatio": "",
        "fee": "",
        "fundingFee": ""
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instType | String | Instrument type |
| \> uid | String | User ID |
| data | Array of objects | Subscribed data |
| \> algoId | String | Algo ID |
| \> algoClOrdId | String | Client-supplied Algo ID |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> algoOrdType | String | Algo order type<br>`contract_grid`: Contract grid |
| \> state | String | Algo order state<br>`starting`<br>`running`<br>`stopping`<br>`no_close_position`: stopped algo order but hadn't close position yet<br>`stopped` |
| \> rebateTrans | Array of objects | Rebate transfer info |
| >\> rebate | String | Rebate amount |
| >\> rebateCcy | String | Rebate currency |
| \> triggerParams | Array of objects | Trigger Parameters |
| >\> triggerAction | String | Trigger action<br>`start`<br>`stop` |
| >\> triggerStrategy | String | Trigger strategy<br>`instant`<br>`price`<br>`rsi` |
| >\> delaySeconds | String | Delay seconds after action triggered |
| >\> triggerTime | String | Actual action triggered time, unix timestamp format in milliseconds, e.g. `1597026383085` |
| >\> triggerType | String | Actual action triggered type<br>`manual`<br>`auto` |
| >\> timeframe | String | K-line type<br>`3m`, `5m`, `15m`, `30m` (`m`: minute)<br>`1H`, `4H` (`H`: hour)<br>`1D` (`D`: day)<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> thold | String | Threshold<br>The value should be an integer between 1 to 100<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> triggerCond | String | Trigger condition<br>`cross_up`<br>`cross_down`<br>`above`<br>`below`<br>`cross`<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> timePeriod | String | Time Period<br>`14`<br>This field is only valid when `triggerStrategy` is `rsi` |
| >\> triggerPx | String | Trigger Price<br>This field is only valid when `triggerStrategy` is `price` |
| >\> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions<br>This field is only valid when `triggerAction` is `stop` |
| \> maxPx | String | Upper price of price range |
| \> minPx | String | Lower price of price range |
| \> gridNum | String | Grid quantity |
| \> runType | String | Grid type<br>`1`: Arithmetic, `2`: Geometric |
| \> tpTriggerPx | String | Take-profit trigger price |
| \> slTriggerPx | String | Stop-loss trigger price |
| \> tradeNum | String | The number of trades executed |
| \> arbitrageNum | String | The number of arbitrages executed |
| \> singleAmt | String | Amount per grid |
| \> perMinProfitRate | String | Estimated minimum Profit margin per grid |
| \> perMaxProfitRate | String | Estimated maximum Profit margin per grid |
| \> runPx | String | Price at launch |
| \> totalPnl | String | Total P&L |
| \> pnlRatio | String | P&L ratio |
| \> investment | String | Accumulated investment amount<br>Spot grid investment amount calculated on quote currency |
| \> gridProfit | String | Grid profit |
| \> floatProfit | String | Variable P&L |
| \> totalAnnualizedRate | String | Total annualized rate |
| \> annualizedRate | String | Grid annualized rate |
| \> cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop<br>`2`: Take profit<br>`3`: Stop loss<br>`4`: Risk control<br>`5`: Delivery<br>`6`: Signal |
| \> stopType | String | Stop type<br>Spot grid `1`: Sell base currency `2`: Keep base currency<br>Contract grid `1`: Market Close All positions `2`: Keep positions |
| \> direction | String | Contract grid type<br>`long`,`short`,`neutral`<br>Only applicable to `contract grid` |
| \> basePos | Boolean | Whether or not to open a position when the strategy is activated<br>Only applicable to `contract grid` |
| \> sz | String | Used margin based on `USDT`<br>Only applicable to `contract grid` |
| \> lever | String | Leverage<br>Only applicable to `contract grid` |
| \> actualLever | String | Actual Leverage<br>Only applicable to `contract grid` |
| \> liqPx | String | Estimated liquidation price<br>Only applicable to `contract grid` |
| \> ordFrozen | String | Margin used by pending orders<br>Only applicable to `contract grid` |
| \> availEq | String | Available margin<br>Only applicable to `contract grid` |
| \> eq | String | Total equity of strategy account<br>Only applicable to `contract grid` |
| \> activeOrdNum | String | Total count of pending sub orders |
| \> tag | String | Order tag |
| \> profitSharingRatio | String | Profit sharing ratio<br>Value range \[0, 0.3\]<br>If it is a normal order (neither copy order nor lead order), this field returns "" |
| \> copyType | String | Profit sharing order type<br>`0`: Normal order<br>`1`: Copy order without profit sharing<br>`2`: Copy order with profit sharing<br>`3`: Lead order |
| \> tpRatio | String | Take profit ratio, 0.1 represents 10% |
| \> slRatio | String | Stop loss ratio, 0.1 represents 10% |
| \> fee | String | Accumulated fee. Only applicable to contract grid, or it will be "" |
| \> fundingFee | String | Accumulated funding fee. Only applicable to contract grid, or it will be "" |
| \> pTime | String | Push time of algo grid information, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### WS / Grid positions channel

Retrieve contract grid positions. Data will be pushed when triggered by events such as placing/canceling order.

Please ignore the empty data.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "grid-positions",
        "algoId": "449327675342323712"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "grid-positions",
        "algoId": "449327675342323712"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`grid-positions` |
| \> algoId | String | Yes | Algo Order ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "grid-positions",
        "algoId": "449327675342323712"
    },
    "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"grid-positions\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> algoId | String | Yes | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "grid-positions",
        "uid": "4470****9584",
        "algoId": "449327675342323712"
    },
    "data": [{
        "adl": "1",
        "algoClOrdId": "",
        "algoId": "449327675342323712",
        "avgPx": "29181.4638888888888895",
        "cTime": "1653400065917",
        "ccy": "USDT",
        "imr": "2089.2690000000002",
        "instId": "BTC-USDT-SWAP",
        "instType": "SWAP",
        "last": "29852.7",
        "lever": "5",
        "liqPx": "604.7617536513744",
        "markPx": "29849.7",
        "mgnMode": "cross",
        "mgnRatio": "217.71740878394456",
        "mmr": "41.78538",
        "notionalUsd": "10435.794191550001",
        "pTime": "1653536068723",
        "pos": "35",
        "posSide": "net",
        "uTime": "1653445498682",
        "upl": "232.83263888888962",
        "uplRatio": "0.1139826489932205"
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> algoId | String | Algo Order ID |
| data | Array of objects | Subscribed data |
| \> algoId | String | Algo ID |
| \> algoClOrdId | String | Client-supplied Algo ID |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> avgPx | String | Average open price |
| \> ccy | String | Margin currency |
| \> lever | String | Leverage |
| \> liqPx | String | Estimated liquidation price |
| \> posSide | String | Position side<br>`net` |
| \> pos | String | Quantity of positions |
| \> mgnMode | String | Margin mode<br>`cross`<br>`isolated` |
| \> mgnRatio | String | Maintenance margin ratio |
| \> imr | String | Initial margin requirement |
| \> mmr | String | Maintenance margin requirement |
| \> upl | String | Unrealized profit and loss |
| \> uplRatio | String | Unrealized profit and loss ratio |
| \> last | String | Latest traded price |
| \> notionalUsd | String | Notional value of positions in `USD` |
| \> adl | String | Automatic-Deleveraging, signal area<br>Divided into 5 levels, from 1 to 5, the smaller the number, the weaker the adl intensity. |
| \> markPx | String | Mark price |
| \> pTime | String | Push time of positions information, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### WS / Grid sub orders channel

Retrieve grid sub orders. Data will be pushed when triggered by events such as placing order.

Please ignore the empty data.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "grid-sub-orders",
        "algoId": "449327675342323712"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "grid-sub-orders",
        "algoId": "449327675342323712"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`grid-sub-orders` |
| \> algoId | String | Yes | Algo Order ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "grid-sub-orders",
        "algoId": "449327675342323712"
    },
    "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"grid-sub-orders\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> algoId | String | Yes | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "grid-sub-orders",
        "uid": "44705892343619584",
        "algoId": "449327675342323712"
    },
    "data": [{
        "accFillSz": "0",
        "algoClOrdId": "",
        "algoId": "449327675342323712",
        "algoOrdType": "contract_grid",
        "avgPx": "0",
        "cTime": "1653445498664",
        "ctVal": "0.01",
        "fee": "0",
        "feeCcy": "USDT",
        "groupId": "-1",
        "instId": "BTC-USDT-SWAP",
        "instType": "SWAP",
        "lever": "5",
        "ordId": "449518234142904321",
        "ordType": "limit",
        "pTime": "1653486524502",
        "pnl": "",
        "posSide": "net",
        "px": "28007.2",
        "rebate": "0",
        "rebateCcy": "USDT",
        "side": "buy",
        "state": "live",
        "sz": "1",
        "tag":"",
        "tdMode": "cross",
        "uTime": "1653445498674"
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> algoId | String | Algo Order ID |
| data | Array of objects | Subscribed data |
| \> algoId | String | Algo ID |
| \> algoClOrdId | String | Client-supplied Algo ID |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> algoOrdType | String | Algo order type<br>`grid`: Spot grid<br>`contract_grid`: Contract grid |
| \> groupId | String | Group ID |
| \> ordId | String | Sub order ID |
| \> cTime | String | Sub order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Sub order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> tdMode | String | Sub order trade mode<br>Margin mode `cross``isolated`<br>Non-Margin mode `cash` |
| \> tag | String | Order tag |
| \> ordType | String | Sub order type<br>`market`: Market order<br>`limit`: Limit order<br>`ioc`: Immediate-or-cancel order |
| \> sz | String | Sub order quantity to buy or sell |
| \> state | String | Sub order state<br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`cancelling` |
| \> side | String | Sub order side<br>`buy``sell` |
| \> px | String | Sub order price |
| \> fee | String | Sub order fee amount |
| \> feeCcy | String | Sub order fee currency |
| \> rebate | String | Sub order rebate amount |
| \> rebateCcy | String | Sub order rebate currency |
| \> avgPx | String | Sub order average filled price |
| \> accFillSz | String | Sub order accumulated fill quantity |
| \> posSide | String | Sub order position side<br>`net` |
| \> pnl | String | Sub order profit and loss |
| \> ctVal | String | Contract value<br>Only applicable to `FUTURES`/`SWAP`/`OPTION` |
| \> lever | String | Leverage |
| \> pTime | String | Push time of orders information, Unix timestamp format in milliseconds, e.g. `1597026383085` |

## Signal bot trading

Create and customize your own signals while gaining access to a diverse selection of signals from top providers. Empower your trading strategies and stay ahead of the game with our comprehensive signal trading platform. [Learn more](https://www.okx.com/learn/signal-trading)

### POST / Create signal

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/create-signal`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/create-signal
body
{
  "signalChanName": "long short",
  "signalDesc": "this is the first version"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| signalChanName | String | Yes | Signal channel name |
| signalChanDesc | String | No | Signal channel description |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
       {
           "signalChanId" :"572112109",
           "signalChanToken":"dojuckew331lkx"
       }

    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| signalChanId | String | Signal channel Id |
| signalChanToken | String | User identify when placing orders via signal |

### GET / Signals

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/signals`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/signals?signalSourceType=1
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| signalSourceType | String | Yes | Signal source type<br>`1`: Created by yourself<br>`2`: Subscribe<br>`3`: Free signal |
| signalChanId | String | No | Signal channel id |
| after | String | No | Pagination of data to return records `signalChanId` earlier than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records `signalChanId` newer than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "signalChanId": "623833708424069120",
            "signalChanName": "test",
            "signalChanDesc": "test",
            "signalChanToken": "test",
            "signalSourceType": "1"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| signalChanId | String | Signal channel id |
| signalChanName | String | Signal channel name |
| signalChanDesc | String | Signal channel description |
| signalChanToken | String | User identify when placing orders via signal |
| signalSourceType | String | Signal source type<br>`1`: Created by yourself<br>`2`: Subscribe<br>`3`: Free signal |

### POST / Create signal bot

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/order-algo`

> Request Example

```
Copy to Clipboard
# Create signal bot
POST /api/v5/tradingBot/signal/order-algo
body
{
  "signalChanId": "627921182788161536",
  "instIds": [
    "BTC-USDT-SWAP",
    "ETH-USDT-SWAP",
    "LTC-USDT-SWAP"
  ],
  "lever": "10",
  "investAmt": "100",
  "subOrdType": "9",
  "entrySettingParam": {
    "allowMultipleEntry": true,
    "entryType": "1",
    "amt": "",
    "ratio": ""
  },
  "exitSettingParam": {
    "tpSlType": "2",
    "tpPct": "",
    "slPct": ""
  }
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| signalChanId | String | Yes | Signal channel Id |
| lever | String | Yes | Leverage<br>Only applicable to `contract signal` |
| investAmt | String | Yes | Investment amount |
| subOrdType | String | Yes | Sub order type `1`：limit order `2`：market order `9`：tradingView signal |
| includeAll | Boolean | No | Whether to include all USDT-margined contract.The default value is `false`. `true`: include `false` : exclude |
| instIds | String | No | Instrument IDs. Single currency or multiple currencies separated with comma. When `includeAll` is `true`, it is ignored |
| ratio | String | No | Price offset ratio, calculate the limit price as a percentage offset from the best bid/ask price.<br>Only applicable to `subOrdType` is `limit` order |
| entrySettingParam | String | No | Entry setting |
| \> allowMultipleEntry | String | No | Whether or not allow multiple entries in the same direction for the same trading pairs.The default value is `true`。 `true`：Allow `false`：Prohibit |
| \> entryType | String | No | Entry type<br>`1`: TradingView signal<br>`2`: Fixed margin<br>`3`: Contracts<br>`4`: Percentage of free margin<br>`5`: Percentage of the initial invested margin |
| \> amt | String | No | Amount per order <br>Only applicable to entryType in `2`/`3` |
| \> ratio | Array of objects | No | Amount ratio per order<br>Only applicable to entryType in `4`/`5` |
| exitSettingParam | String | No | Exit setting |
| \> tpSlType | String | 是 | Type of set the take-profit and stop-loss trigger price <br>`pnl`: Based on the estimated profit and loss percentage from the entry point <br>`price`: Based on price increase or decrease from the crypto’s entry price |
| \> tpPct | String | No | Take-profit percentage |
| \> slPct | String | No | Stop-loss percentage |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "447053782921515008",
            "sCode": "0",
            "sMsg": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, 0 means success. |
| sMsg | String | The code of the event execution result, 0 means success. |

### POST / Cancel signal bots

A maximum of 10 orders can be stopped per request.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/stop-order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/stop-order-algo
body
[
    {
        "algoId":"448965992920907776"
    }
]
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "448965992920907776",
            "sCode": "0",
            "sMsg": "",
            "algoClOrdId": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| sCode | String | The code of the event execution result, `0` means success. |
| sMsg | String | Rejection or success message of event execution. |
| algoClOrdId | String | Client-supplied Algo ID |

### POST / Adjust margin balance

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/margin-balance`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/margin-balance
body
{
   "algoId":"123456",
   "type":"add",
   "amt":"10"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| type | String | Yes | Adjust margin balance type<br>`add``reduce` |
| amt | String | Yes | Adjust margin balance amount<br>Either `amt` or `percent` is required. |
| allowReinvest | Boolean | No | Whether to reinvest with newly added margin. The default value is `false`. <br>`false`:it will be used as passive margin to prevent liquidation and will not be used as active investment<br>`true`:the margin added here will furthermore be accounted for in calculations of your total investment amount, and furthermore your order size。<br>Only applicable to your signal comes in with an “investmentType” of “percentage\_investment” |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "123456"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |

### POST / Amend TPSL

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/amendTPSL`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/amendTPSL
body
{
    "algoId": "637039348240277504",
    "exitSettingParam": {
        "tpSlType": "pnl",
        "tpPct": "0.01",
        "slPct": "0.01"
    }
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| exitSettingParam | String | Yes | Exit setting |
| \> tpSlType | String | Yes | Type of set the take-profit and stop-loss trigger price<br>`pnl`: Based on the estimated profit and loss percentage from the entry point<br>`price`: Based on price increase or decrease from the crypto’s entry price |
| \> tpPct | String | No | Take-profit percentage |
| \> slPct | String | No | Stop-loss percentage |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "637039348240277504"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |

### POST / Set instruments

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/set-instruments`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/set-instruments
body
{
    "algoId": "637039348240277504",
    "instIds": [
        "SHIB-USDT-SWAP",
        "ETH-USDT-SWAP"
    ]
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instIds | Array of strings | Yes | Instrument IDs. When `includeAll` is `true`, it is ignored |
| includeAll | Boolean | Yes | Whether to include all USDT-margined contract.The default value is `false`. `true`: include `false` : exclude |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "637039348240277504"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |

### GET / Signal bot order details

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/orders-algo-details`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/orders-algo-details?algoId=623833708424069120&algoOrdType=contract
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`contract`: Contract signal |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "623833708424069120",
            "algoClOrdId": "",
            "algoOrdType": "contract",
            "availBal": "1.6561369013122267",
            "cTime": "1695005546360",
            "cancelType": "0",
            "entrySettingParam": {
                "allowMultipleEntry": true,
                "amt": "0",
                "entryType": "1",
                "ratio": ""
            },
            "exitSettingParam": {
                "slPct": "",
                "tpPct": "",
                "tpSlType": "price"
            },
            "floatPnl": "0.1279999999999927",
            "frozenBal": "25.16816",
            "instIds": [
                "BTC-USDT-SWAP",
                "ETH-USDT-SWAP"
            ],
            "instType": "SWAP",
            "investAmt": "100",
            "lever": "10",
            "ratio": "",
            "realizedPnl": "-73.303703098687766",
            "signalChanId": "623827579484770304",
            "signalChanName": "testing",
            "signalSourceType": "1",
            "state": "running",
            "subOrdType": "9",
            "totalEq": "26.824296901312227",
            "totalPnl": "-73.1757030986877733",
            "totalPnlRatio": "-0.7317570309868777",
            "uTime": "1697029422313"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instIds | Array of strings | Instrument IDs |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`contract`: Contract signal |
| state | String | Algo order state<br>`starting`<br>`running`<br>`stopping`<br>`stopped` |
| cancelType | String | Algo order stop reason<br>`0`: None<br>`1`: Manual stop |
| totalPnl | String | Total P&L |
| totalPnlRatio | String | Total P&L ratio |
| totalEq | String | Total equity of strategy account |
| floatPnl | String | Float P&L |
| realizedPnl | String | Realized P&L |
| frozenBal | String | Frozen balance |
| availBal | String | Avail balance |
| lever | String | Leverage<br>Only applicable to `contract signal` |
| investAmt | String | Investment amount |
| subOrdType | String | Sub order type<br>`1`：limit order<br>`2`：market order<br>`9`：tradingView signal |
| ratio | String | Price offset ratio, calculate the limit price as a percentage offset from the best bid/ask price<br>Only applicable to `subOrdType` is `limit order` |
| entrySettingParam | Object | Entry setting |
| \> allowMultipleEntry | Boolean | Whether or not allow multiple entries in the same direction for the same trading pairs |
| \> entryType | String | Entry type<br>`1`: TradingView signal<br>`2`: Fixed margin<br>`3`: Contracts<br>`4`: Percentage of free margin<br>`5`: Percentage of the initial invested margin |
| \> amt | String | Amount per order<br>Only applicable to `entryType` in `2`/`3` |
| \> ratio | String | Amount ratio per order<br>Only applicable to `entryType` in `4`/`5` |
| exitSettingParam | Object | Exit setting |
| \> tpSlType | String | Type of set the take-profit and stop-loss trigger price<br>`pnl`: Based on the estimated profit and loss percentage from the entry point<br>`price`: Based on price increase or decrease from the crypto’s entry price |
| \> tpPct | String | Take-profit percentage |
| \> slPct | String | Stop-loss percentage |
| signalChanId | String | Signal channel Id |
| signalChanName | String | Signal channel name |
| signalSourceType | String | Signal source type<br>`1`: Created by yourself<br>`2`: Subscribe<br>`3`: Free signal |

### GET / Active signal bot

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/orders-algo-pending`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/orders-algo-pending?algoOrdType=contract
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`contract`: Contract signal |
| algoId | String | No | Algo ID |
| after | String | Yes | Pagination of data to return records `algoId` earlier than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records `algoId` newer than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "623833708424069120",
            "algoClOrdId": "",
            "algoOrdType": "contract",
            "availBal": "1.6561369013122267",
            "cTime": "1695005546360",
            "cancelType": "0",
            "entrySettingParam": {
                "allowMultipleEntry": true,
                "amt": "0",
                "entryType": "1",
                "ratio": ""
            },
            "exitSettingParam": {
                "slPct": "",
                "tpPct": "",
                "tpSlType": "price"
            },
            "floatPnl": "0.1279999999999927",
            "frozenBal": "25.16816",
            "instIds": [
                "BTC-USDT-SWAP",
                "ETH-USDT-SWAP"
            ],
            "instType": "SWAP",
            "investAmt": "100",
            "lever": "10",
            "ratio": "",
            "realizedPnl": "-73.303703098687766",
            "signalChanId": "623827579484770304",
            "signalChanName": "my signal",
            "signalSourceType": "1",
            "state": "running",
            "subOrdType": "9",
            "totalEq": "26.824296901312227",
            "totalPnl": "-73.1757030986877733",
            "totalPnlRatio": "-0.7317570309868777",
            "uTime": "1697029422313"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instIds | Array of strings | Instrument IDs |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`contract`: Contract signal |
| state | String | Algo order state<br>`starting`<br>`running`<br>`stopping` |
| cancelType | String | Algo order stop reason<br>`0`: None |
| totalPnl | String | Total P&L |
| totalPnlRatio | String | Total P&L ratio |
| totalEq | String | Total equity of strategy account |
| floatPnl | String | Float P&L |
| realizedPnl | String | Realized P&L |
| frozenBal | String | Frozen balance |
| availBal | String | Avail balance |
| lever | String | Leverage<br>Only applicable to `contract signal` |
| investAmt | String | Investment amount |
| subOrdType | String | Sub order type<br>`1`：limit order<br>`2`：market order<br>`9`：tradingView signal |
| ratio | String | Price offset ratio, calculate the limit price as a percentage offset from the best bid/ask price<br>Only applicable to `subOrdType` is `limit order` |
| entrySettingParam | Object | Entry setting |
| \> allowMultipleEntry | Boolean | Whether or not allow multiple entries in the same direction for the same trading pairs |
| \> entryType | String | Entry type<br>`1`: TradingView signal<br>`2`: Fixed margin<br>`3`: Contracts<br>`4`: Percentage of free margin<br>`5`: Percentage of the initial invested margin |
| \> amt | String | Amount per order<br>Only applicable to `entryType` in `2`/`3` |
| \> ratio | String | Amount ratio per order<br>Only applicable to `entryType` in `4`/`5` |
| exitSettingParam | Object | Exit setting |
| \> tpSlType | String | Type of set the take-profit and stop-loss trigger price<br>`pnl`: Based on the estimated profit and loss percentage from the entry point<br>`price`: Based on price increase or decrease from the crypto’s entry price |
| \> tpPct | String | Take-profit percentage |
| \> slPct | String | Stop-loss percentage |
| signalChanId | String | Signal channel Id |
| signalChanName | String | Signal channel name |
| signalSourceType | String | Signal source type<br>`1`: Created by yourself<br>`2`: Subscribe<br>`3`: Free signal |

### GET / Signal bot history

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/orders-algo-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/orders-algo-history?algoId=623833708424069120&algoOrdType=contract
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`contract`: Contract signal |
| algoId | String | Yes | Algo ID |
| after | String | Yes | Pagination of data to return records `algoId` earlier than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records `algoId` newer than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "623833708424069120",
            "algoClOrdId": "",
            "algoOrdType": "contract",
            "availBal": "1.6561369013122267",
            "cTime": "1695005546360",
            "cancelType": "1",
            "entrySettingParam": {
                "allowMultipleEntry": true,
                "amt": "0",
                "entryType": "1",
                "ratio": ""
            },
            "exitSettingParam": {
                "slPct": "",
                "tpPct": "",
                "tpSlType": "price"
            },
            "floatPnl": "0.1279999999999927",
            "frozenBal": "25.16816",
            "instIds": [
                "BTC-USDT-SWAP",
                "ETH-USDT-SWAP"
            ],
            "instType": "SWAP",
            "investAmt": "100",
            "lever": "10",
            "ratio": "",
            "realizedPnl": "-73.303703098687766",
            "signalChanId": "623827579484770304",
            "signalChanName": "my signal",
            "signalSourceType": "1",
            "state": "stopped",
            "subOrdType": "9",
            "totalEq": "26.824296901312227",
            "totalPnl": "-73.1757030986877733",
            "totalPnlRatio": "-0.7317570309868777",
            "uTime": "1697029422313"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| instIds | Array of strings | Instrument IDs |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`contract`: Contract signal |
| state | String | Algo order state<br>`stopped` |
| cancelType | String | Algo order stop reason<br>`1`: Manual stop |
| totalPnl | String | Total P&L |
| totalPnlRatio | String | Total P&L ratio |
| totalEq | String | Total equity of strategy account |
| floatPnl | String | Float P&L |
| realizedPnl | String | Realized P&L |
| frozenBal | String | Frozen balance |
| availBal | String | Avail balance |
| lever | String | Leverage<br>Only applicable to `contract signal` |
| investAmt | String | Investment amount |
| subOrdType | String | Sub order type<br>`1`：limit order<br>`2`：market order<br>`9`：tradingView signal |
| ratio | String | Price offset ratio, calculate the limit price as a percentage offset from the best bid/ask price<br>Only applicable to `subOrdType` is `limit order` |
| entrySettingParam | Object | Entry setting |
| \> allowMultipleEntry | Boolean | Whether or not allow multiple entries in the same direction for the same trading pairs |
| \> entryType | String | Entry type<br>`1`: TradingView signal<br>`2`: Fixed margin<br>`3`: Contracts<br>`4`: Percentage of free margin<br>`5`: Percentage of the initial invested margin |
| \> amt | String | Amount per order<br>Only applicable to `entryType` in `2`/`3` |
| \> ratio | String | Amount ratio per order<br>Only applicable to `entryType` in `4`/`5` |
| exitSettingParam | Object | Exit setting |
| \> tpSlType | String | Type of set the take-profit and stop-loss trigger price<br>`pnl`: Based on the estimated profit and loss percentage from the entry point<br>`price`: Based on price increase or decrease from the crypto’s entry price |
| \> tpPct | String | Take-profit percentage |
| \> slPct | String | Stop-loss percentage |
| signalChanId | String | Signal channel Id |
| signalChanName | String | Signal channel name |
| signalSourceType | String | Signal source type<br>`1`: Created by yourself<br>`2`: Subscribe<br>`3`: Free signal |

### GET / Signal bot order positions

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/positions`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/positions?algoId=623833708424069120&algoOrdType=contract
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoOrdType | String | Yes | Algo order type<br>`contract`: Contract signal |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "adl": "1",
            "algoClOrdId": "",
            "algoId": "623833708424069120",
            "avgPx": "1597.74",
            "cTime": "1697502301460",
            "ccy": "USDT",
            "imr": "23.76495",
            "instId": "ETH-USDT-SWAP",
            "instType": "SWAP",
            "last": "1584.34",
            "lever": "10",
            "liqPx": "1438.7380360728976",
            "markPx": "1584.33",
            "mgnMode": "cross",
            "mgnRatio": "11.719278420807477",
            "mmr": "1.9011959999999997",
            "notionalUsd": "237.75168928499997",
            "pos": "15",
            "posSide": "net",
            "uTime": "1697502301460",
            "upl": "-2.0115000000000123",
            "uplRatio": "-0.0839310526118142"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID. Used to be extended in the future. |
| instType | String | Instrument type |
| instId | String | Instrument ID, e.g. `BTC-USDT-SWAP` |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| avgPx | String | Average open price |
| ccy | String | Margin currency |
| lever | String | Leverage |
| liqPx | String | Estimated liquidation price |
| posSide | String | Position side<br>`net` |
| pos | String | Quantity of positions |
| mgnMode | String | Margin mode<br>`cross`<br>`isolated` |
| mgnRatio | String | Maintenance margin ratio |
| imr | String | Initial margin requirement |
| mmr | String | Maintenance margin requirement |
| upl | String | Unrealized profit and loss |
| uplRatio | String | Unrealized profit and loss ratio |
| last | String | Latest traded price |
| notionalUsd | String | Notional value of positions in `USD` |
| adl | String | Automatic-Deleveraging, signal area<br>Divided into 5 levels, from 1 to 5, the smaller the number, the weaker the adl intensity. |
| markPx | String | Mark price |

### GET / Position history

Retrieve the updated position data for the last 3 months. Return in reverse chronological order using utime.

#### Rate Limit: 10 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/positions-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/positions-history?algoId=1234
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instId | String | No | Instrument ID, e.g.：`BTC-USD-SWAP` |
| after | String | No | Pagination of data to return records earlier than the requested `uTime`, Unix timestamp format in milliseconds, e.g.`1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `uTime`, Unix timestamp format in milliseconds, e.g `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "data": [
    {
      "cTime": "1704724451471",
      "closeAvgPx": "200",
      "direction": "net",
      "instId": "ETH-USDT-SWAP",
      "lever": "5.0",
      "mgnMode": "cross",
      "openAvgPx": "220",
      "pnl": "-2.021",
      "pnlRatio": "-0.4593181818181818",
      "uTime": "1704724456322",
      "uly": "ETH-USDT"
    }
  ],
  "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID |
| mgnMode | String | Margin mode `cross``isolated` |
| cTime | String | Created time of position |
| uTime | String | Updated time of position |
| openAvgPx | String | Average price of opening position |
| closeAvgPx | String | Average price of closing position |
| pnl | String | Profit and loss |
| pnlRatio | String | P&L ratio |
| lever | String | Leverage |
| direction | String | Direction: `long``short` |
| uly | String | Underlying |

### POST / Close position

Close the position of an instrument via a market order.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/close-position`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/close-position
body
{
    "instId":"BTC-USDT-SWAP",
    "algoId":"448965992920907776"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instId | String | Yes | Instrument ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "448965992920907776"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |

### POST / Place sub order

You can place an order only if you have sufficient funds.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/sub-order`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/sub-order
body
{
    "algoId":"1222",
    "instId":"BTC-USDT-SWAP",
    "side":"buy",
    "ordType":"limit",
    "px":"2.15",
    "sz":"2"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT-SWAP` |
| algoId | String | Yes | Algo ID |
| side | String | Yes | Order side, `buy``sell` |
| ordType | String | Yes | Order type <br>`market`: Market order <br>`limit`: Limit order |
| sz | String | Yes | Quantity to buy or sell |
| px | String | Conditional | Order price. Only applicable to `limit` order. |
| reduceOnly | Boolean | No | Whether orders can only reduce in position size. <br>Valid options: `true` or `false`. The default value is `false`. <br>Only applicable to `Futures mode`/`Multi-currency margin` |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |

ordType

Order type. When creating a new order, you must specify the order type. The order type you specify will affect: 1) what order parameters are required, and 2) how the matching system executes your order. The following are valid order types:

\`limit\`: Limit order, which requires specified sz and px.

\`market\`: Market order. It will be filled with market price (by swiping opposite order book). Market order will be placed to order book with most aggressive price allowed by Price Limit Mechanism.

sz refers to the number of contracts。

reduceOnly

When placing an order with this parameter set to true, it means that the order will reduce the size of the position only
The sum of the current order size and all reverse direction reduce-only pending orders which's price-time priority is higher than the current order, cannot exceed the contract quantity of position.
Only applicable to \`Futures mode\` and \`Multi-currency margin\`

### POST / Cancel sub order

Cancel an incomplete order.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/signal/cancel-sub-order`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/signal/cancel-sub-order
body
{
    "algoId":"91664",
    "signalOrdId":"590908157585625111",
    "instId":"BTC-USDT-SWAP"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| instId | String | Yes | Instrument ID, e.g. BTC-USDT-SWAP |
| signalOrdId | String | Yes | Order ID |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "signalOrdId":"590908157585625111",
            "sCode":"0",
            "sMsg":""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| code | String | The result code, `0` means success |
| msg | String | The error message, empty if the code is 0 |
| data | Array of objects | Array of objects contains the response results |
| \> signalOrdId | String | Order ID |
| \> sCode | String | The code of the event execution result, `0` means success. |
| \> sMsg | String | Rejection or success message of event execution. |

Cancel order returns with sCode equal to 0. It is not strictly considered that the order has been canceled. It only means that your cancellation request has been accepted by the system server. The result of the cancellation is subject to the state by get sub orders endpoint.

### GET / Signal bot sub orders

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/sub-orders`

> Request Example

```
Copy to Clipboard
# Get historical filled sub orders
GET /api/v5/tradingBot/signal/sub-orders?algoId=623833708424069120&algoOrdType=contract&state=filled

# Get designated sub order
GET /api/v5/tradingBot/signal/sub-orders?algoId=623833708424069120&algoOrdType=contract&signalOrdId=O632302662327996418
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| algoOrdType | String | Yes | Algo order type<br>`contract`: Contract signal |
| state | String | Conditional | Sub order state<br>`live`<br>`partially_filled`<br>`filled`<br>`cancelled`<br>Either `state` or `signalOrdId` is required, if both are passed in, only `state` is valid. |
| signalOrdId | String | Conditional | Sub order ID |
| after | String | No | Pagination of data to return records earlier than the requested `ordId` |
| before | String | No | Pagination of data to return records newer than the requested `ordId`. |
| begin | String | No | Return records of `ctime` after than the requested timestamp (include), Unix timestamp format in milliseconds, e.g. `1597026383085` |
| end | String | No | Return records of `ctime` before than the requested timestamp (include), Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |
| type | String | No | Sub order type <br>`live`<br>`filled`<br>Either `type` or `clOrdId` is required, if both are passed in, only `clOrdId` is valid. |
| clOrdId | String | No | Sub order client-supplied ID. <br>`It will be deprecated soon` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "18",
            "algoClOrdId": "",
            "algoId": "623833708424069120",
            "algoOrdType": "contract",
            "avgPx": "1572.81",
            "cTime": "1697024702320",
            "ccy": "",
            "clOrdId": "O632302662327996418",
            "ctVal": "0.01",
            "fee": "-0.1415529",
            "feeCcy": "USDT",
            "instId": "ETH-USDT-SWAP",
            "instType": "SWAP",
            "lever": "10",
            "ordId": "632302662351958016",
            "ordType": "market",
            "pnl": "-2.6784",
            "posSide": "net",
            "px": "",
            "side": "buy",
            "state": "filled",
            "sz": "18",
            "tag": "",
            "tdMode": "cross",
            "uTime": "1697024702322"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID. Used to be extended in the future |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| algoOrdType | String | Algo order type<br>`contract`: Contract signal |
| ordId | String | Sub order ID |
| clOrdId | String | Sub order client-supplied ID. <br> It is equal to `signalOrdId` |
| cTime | String | Sub order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Sub order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| tdMode | String | Sub order trade mode<br>Margin mode: `cross`/`isolated`<br>Non-Margin mode: `cash` |
| ccy | String | Margin currency<br>Only applicable to cross MARGIN orders in `Futures mode`. |
| ordType | String | Sub order type<br>`market`: Market order<br>`limit`: Limit order<br>`ioc`: Immediate-or-cancel order |
| sz | String | Sub order quantity to buy or sell |
| state | String | Sub order state<br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`cancelling` |
| side | String | Sub order side<br>`buy`,`sell` |
| px | String | Sub order price |
| fee | String | Sub order fee amount |
| feeCcy | String | Sub order fee currency |
| avgPx | String | Sub order average filled price |
| accFillSz | String | Sub order accumulated fill quantity |
| posSide | String | Sub order position side<br>`net` |
| pnl | String | Sub order profit and loss |
| ctVal | String | Contract value<br>Only applicable to `FUTURES`/`SWAP` |
| lever | String | Leverage |
| tag | String | Order tag |

### GET / Signal bot event history

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/signal/event-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/signal/event-history?algoId=623833708424069120
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| after | String | No | Pagination of data to return records `eventCtime` earlier than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records `eventCtime` newer than the requested timestamp, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "alertMsg": "{\"marketPosition\":\"short\",\"prevMarketPosition\":\"long\",\"action\":\"sell\",\"instrument\":\"ETHUSDT.P\",\"timestamp\":\"2023-10-16T10:50:00.000Z\",\"maxLag\":\"60\",\"investmentType\":\"base\",\"amount\":\"2\"}",
            "algoId": "623833708424069120",
            "eventCtime": "1697453400959",
            "eventProcessMsg": "Processed reverse entry signal and placed ETH-USDT-SWAP order with all available balance",
            "eventStatus": "success",
            "eventType": "signal_processing",
            "eventUtime": "",
            "triggeredOrdData": [
                {
                    "clOrdId": "O634100754731765763"
                },
                {
                    "clOrdId": "O634100754752737282"
                }
            ]
        }
     ],
     "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| alertMsg | String | Alert message |
| algoId | String | Algo ID |
| eventType | String | Event type<br>`system_action`<br>`user_action`<br>`signal_processing` |
| eventCtime | String | Event timestamp of creation. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| eventUtime | String | Event timestamp of update. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| eventProcessMsg | String | Event process message |
| eventStatus | String | Event status<br>`success`<br>`failure` |
| triggeredOrdData | Array of objects | Triggered sub order data |
| \> clOrdId | String | Sub order client-supplied id |

## Recurring Buy

Recurring buy is a strategy for investing a fixed amount in crypto at fixed intervals.
An appropriate recurring approach in volatile markets allows you to buy crypto at lower costs. [Learn more](https://www.okx.com/help/vii-recurring-buy)

The API endpoints of `Recurring buy` require authentication.

### POST / Place recurring buy order

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/recurring/order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/recurring/order-algo
body
{
  "stgyName": "BTC|ETH recurring buy monthly",
  "amt":"100",
  "recurringList":[
    {
         "ccy":"BTC",
         "ratio":"0.2"
    },
    {
         "ccy":"ETH",
         "ratio":"0.8"
    }
  ],
  "period":"monthly",
  "recurringDay":"1",
  "recurringTime":"0",
  "timeZone":"8",   // UTC +8
  "tdMode":"cross",
  "investmentCcy":"USDT"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| stgyName | String | Yes | Custom name for trading bot, no more than 40 characters |
| recurringList | Array of objects | Yes | Recurring buy info |
| \> ccy | String | Yes | Recurring currency, e.g. `BTC` |
| \> ratio | String | Yes | Proportion of recurring currency assets, e.g. "0.2" representing 20% |
| period | String | Yes | Period<br>`monthly`<br>`weekly`<br>`daily`<br>`hourly` |
| recurringDay | String | Conditional | Recurring buy date<br>When the period is `monthly`, the value range is an integer of \[1,28\]<br>When the period is `weekly`, the value range is an integer of \[1,7\]<br>When the period is `daily`/`hourly`, the parameter is not required. |
| recurringHour | String | Conditional | Recurring buy by hourly<br>`1`/`4`/`8`/`12`, e.g. `4` represents "recurring buy every 4 hour"<br>When the period is `hourly`, the parameter is required. |
| recurringTime | String | Yes | Recurring buy time, the value range is an integer of \[0,23\]<br>When the period is `hourly`, the parameter is the time of the first investment occurs. |
| timeZone | String | Yes | UTC time zone, the value range is an integer of \[-12,14\]<br>e.g. "8" representing UTC+8 (East 8 District), Beijing Time |
| amt | String | Yes | Quantity invested per cycle |
| investmentCcy | String | Yes | The invested quantity unit, can only be `USDT`/`USDC` |
| tdMode | String | Yes | Trading mode<br>Margin mode: `cross`<br>Non-Margin mode: `cash` |
| algoClOrdId | String | No | Client-supplied Algo ID<br>There will be a value when algo order attaching algoClOrdId is triggered, or it will be "".<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |
| tradeQuoteCcy | String | No | The quote currency for trading. |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoId":"560472804207104000",
            "algoClOrdId":"",
            "sCode":"0",
            "sMsg":"",
            "tag":""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, 0 means success |
| sMsg | String | Rejection message if the request is unsuccessful |
| tag | String | Order tag |

### POST / Amend recurring buy order

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/recurring/amend-order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/recurring/amend-order-algo
body
{
    "algoId":"448965992920907776",
    "stgyName":"stg1"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| stgyName | String | Yes | New custom name for trading bot after adjustment, no more than 40 characters |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "algoId":"448965992920907776",
            "algoClOrdId":"",
            "sCode":"0",
            "sMsg":""
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, 0 means success |
| sMsg | String | Rejection message if the request is unsuccessful |

### POST / Stop recurring buy order

A maximum of 10 orders can be stopped per request.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/tradingBot/recurring/stop-order-algo`

> Request Example

```
Copy to Clipboard
POST /api/v5/tradingBot/recurring/stop-order-algo
body
[
    {
        "algoId":"560472804207104000"
    }
]
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "1839309556514557952",
            "sCode": "0",
            "sMsg": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| sCode | String | The code of the event execution result, 0 means success |
| sMsg | String | Rejection message if the request is unsuccessful |
| tag | String | ~~Order tag~~(Deprecated) |

### GET / Recurring buy order list

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/recurring/orders-algo-pending`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/recurring/orders-algo-pending
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | No | Algo ID |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100 |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "644497312047435776",
            "algoOrdType": "recurring",
            "amt": "100",
            "cTime": "1699932133373",
            "cycles": "6",
            "instType": "SPOT",
            "investmentAmt": "0",
            "investmentCcy": "USDC",
            "mktCap": "0",
            "period": "hourly",
            "pnlRatio": "0",
            "recurringDay": "",
            "recurringHour": "1",
            "recurringList": [
                {
                    "ccy": "BTC",
                    "ratio": "0.2"
                },
                {
                    "ccy": "ETH",
                    "ratio": "0.8"
                }
            ],
            "recurringTime": "12",
            "state": "running",
            "stgyName": "stg1",
            "tag": "",
            "timeZone": "8",
            "totalAnnRate": "0",
            "totalPnl": "0",
            "uTime": "1699952473152",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`recurring`: recurring buy |
| state | String | Algo order state<br>`running`<br>`stopping`<br>`pause` |
| stgyName | String | Custom name for trading bot, no more than 40 characters |
| recurringList | Array of objects | Recurring buy info |
| \> ccy | String | Recurring currency, e.g. `BTC` |
| \> ratio | String | Proportion of recurring currency assets, e.g. "0.2" representing 20% |
| period | String | Period<br>`monthly`<br>`weekly`<br>`daily`<br>`hourly` |
| recurringDay | String | Recurring buy date<br>When the period is `monthly`, the value range is an integer of \[1,28\]<br>When the period is `weekly`, the value range is an integer of \[1,7\] |
| recurringHour | String | Recurring buy by hourly<br>`1`/`4`/`8`/`12`, e.g. `4` represents "recurring buy every 4 hour" |
| recurringTime | String | Recurring buy time, the value range is an integer of \[0,23\] |
| timeZone | String | UTC time zone, the value range is an integer of \[-12,14\]<br>e.g. "8" representing UTC+8 (East 8 District), Beijing Time |
| amt | String | Quantity invested per cycle |
| investmentAmt | String | Accumulate quantity invested |
| investmentCcy | String | The invested quantity unit, can only be `USDT`/`USDC` |
| totalPnl | String | Total P&L |
| totalAnnRate | String | Total annualized rate of yield |
| pnlRatio | String | Rate of yield |
| mktCap | String | Market value in unit of `USDT` |
| cycles | String | Accumulate recurring buy cycles |
| tag | String | Order tag |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Recurring buy order history

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/recurring/orders-algo-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/recurring/orders-algo-history
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | No | Algo ID |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100 |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "644496098429767680",
            "algoOrdType": "recurring",
            "amt": "100",
            "cTime": "1699931844050",
            "cycles": "0",
            "instType": "SPOT",
            "investmentAmt": "0",
            "investmentCcy": "USDC",
            "mktCap": "0",
            "period": "hourly",
            "pnlRatio": "0",
            "recurringDay": "",
            "recurringHour": "1",
            "recurringList": [
                {
                    "ccy": "BTC",
                    "ratio": "0.2"
                },
                {
                    "ccy": "ETH",
                    "ratio": "0.8"
                }
            ],
            "recurringTime": "0",
            "state": "stopped",
            "stgyName": "stg1",
            "tag": "",
            "timeZone": "8",
            "totalAnnRate": "0",
            "totalPnl": "0",
            "uTime": "1699932177659",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`recurring`: recurring buy |
| state | String | Algo order state<br>`stopped` |
| stgyName | String | Custom name for trading bot, no more than 40 characters |
| recurringList | Array of objects | Recurring buy info |
| \> ccy | String | Recurring currency, e.g. `BTC` |
| \> ratio | String | Proportion of recurring currency assets, e.g. "0.2" representing 20% |
| period | String | Period<br>`monthly`<br>`weekly`<br>`daily`<br>`hourly` |
| recurringDay | String | Recurring buy date<br>When the period is `monthly`, the value range is an integer of \[1,28\]<br>When the period is `weekly`, the value range is an integer of \[1,7\] |
| recurringHour | String | Recurring buy by hourly<br>`1`/`4`/`8`/`12`, e.g. `4` represents "recurring buy every 4 hour" |
| recurringTime | String | Recurring buy time, the value range is an integer of \[0,23\] |
| timeZone | String | UTC time zone, the value range is an integer of \[-12,14\]<br>e.g. "8" representing UTC+8 (East 8 District), Beijing Time |
| amt | String | Quantity invested per cycle |
| investmentAmt | String | Accumulate quantity invested |
| investmentCcy | String | The invested quantity unit, can only be `USDT`/`USDC` |
| totalPnl | String | Total P&L |
| totalAnnRate | String | Total annualized rate of yield |
| pnlRatio | String | Rate of yield |
| mktCap | String | Market value in unit of `USDT` |
| cycles | String | Accumulate recurring buy cycles |
| tag | String | Order tag |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Recurring buy order details

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/recurring/orders-algo-details`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/recurring/orders-algo-details?algoId=644497312047435776
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoClOrdId": "",
            "algoId": "644497312047435776",
            "algoOrdType": "recurring",
            "amt": "100",
            "cTime": "1699932133373",
            "cycles": "6",
            "instType": "SPOT",
            "investmentAmt": "0",
            "investmentCcy": "USDC",
            "mktCap": "0",
            "nextInvestTime": "1699956005500",
            "period": "hourly",
            "pnlRatio": "0",
            "recurringDay": "",
            "recurringHour": "1",
            "recurringList": [
                {
                    "avgPx": "0",
                    "ccy": "BTC",
                    "profit": "0",
                    "px": "36683.2",
                    "ratio": "0.2",
                    "totalAmt": "0"
                },
                {
                    "avgPx": "0",
                    "ccy": "ETH",
                    "profit": "0",
                    "px": "2058.36",
                    "ratio": "0.8",
                    "totalAmt": "0"
                }
            ],
            "recurringTime": "12",
            "state": "running",
            "stgyName": "stg1",
            "tag": "",
            "timeZone": "8",
            "totalAnnRate": "0",
            "totalPnl": "0",
            "uTime": "1699952485451",
            "tradeQuoteCcy": "USDT"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| algoClOrdId | String | Client-supplied Algo ID |
| instType | String | Instrument type |
| cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| algoOrdType | String | Algo order type<br>`recurring`: recurring buy |
| state | String | Algo order state<br>`running`<br>`stopping`<br>`stopped`<br>`pause` |
| stgyName | String | Custom name for trading bot, no more than 40 characters |
| recurringList | Array of objects | Recurring buy info |
| \> ccy | String | Recurring buy currency, e.g. `BTC` |
| \> ratio | String | Proportion of recurring currency assets, e.g. "0.2" representing 20% |
| \> totalAmt | String | Accumulated quantity in unit of recurring buy currency |
| \> profit | String | Profit in unit of `investmentCcy` |
| \> avgPx | String | Average price of recurring buy, quote currency is `investmentCcy` |
| \> px | String | Current market price, quote currency is `investmentCcy` |
| period | String | Period<br>`monthly`<br>`weekly`<br>`daily`<br>`hourly` |
| recurringDay | String | Recurring buy date<br>When the period is `monthly`, the value range is an integer of \[1,28\]<br>When the period is `weekly`, the value range is an integer of \[1,7\] |
| recurringHour | String | Recurring buy by hourly<br>`1`/`4`/`8`/`12`, e.g. `4` represents "recurring buy every 4 hour" |
| recurringTime | String | Recurring buy time, the value range is an integer of \[0,23\] |
| timeZone | String | UTC time zone, the value range is an integer of \[-12,14\]<br>e.g. "8" representing UTC+8 (East 8 District), Beijing Time |
| amt | String | Quantity invested per cycle |
| investmentAmt | String | Accumulate quantity invested |
| investmentCcy | String | The invested quantity unit, can only be `USDT`/`USDC` |
| nextInvestTime | String | Next invest time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| totalPnl | String | Total P&L |
| totalAnnRate | String | Total annualized rate of yield |
| pnlRatio | String | Rate of yield |
| mktCap | String | Market value in unit of `USDT` |
| cycles | String | Accumulate recurring buy cycles |
| tag | String | Order tag |
| tradeQuoteCcy | String | The quote currency for trading. |

### GET / Recurring buy sub orders

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/tradingBot/recurring/sub-orders`

> Request Example

```
Copy to Clipboard
GET /api/v5/tradingBot/recurring/sub-orders?algoId=560516615079727104
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| algoId | String | Yes | Algo ID |
| ordId | String | No | Sub order ID |
| after | String | No | Pagination of data to return records earlier than the requested `algoId`. |
| before | String | No | Pagination of data to return records newer than the requested `algoId`. |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100 |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "accFillSz": "0.045315",
            "algoClOrdId": "",
            "algoId": "560516615079727104",
            "algoOrdType": "recurring",
            "avgPx": "1765.4",
            "cTime": "1679911222200",
            "fee": "-0.0000317205",
            "feeCcy": "ETH",
            "instId": "ETH-USDC",
            "instType": "SPOT",
            "ordId": "560523524230717440",
            "ordType": "market",
            "px": "-1",
            "side": "buy",
            "state": "filled",
            "sz": "80",
            "tag": "",
            "tdMode": "",
            "uTime": "1679911222207"
        },
        {
            "accFillSz": "0.00071526",
            "algoClOrdId": "",
            "algoId": "560516615079727104",
            "algoOrdType": "recurring",
            "avgPx": "27961.6",
            "cTime": "1679911222189",
            "fee": "-0.000000500682",
            "feeCcy": "BTC",
            "instId": "BTC-USDC",
            "instType": "SPOT",
            "ordId": "560523524184580096",
            "ordType": "market",
            "px": "-1",
            "side": "buy",
            "state": "filled",
            "sz": "20",
            "tag": "",
            "tdMode": "",
            "uTime": "1679911222194"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| algoId | String | Algo ID |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| algoOrdType | String | Algo order type<br>`recurring`: recurring buy |
| ordId | String | Sub order ID |
| cTime | String | Sub order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Sub order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| tdMode | String | Sub order trade mode<br>Margin mode : `cross`<br>Non-Margin mode : `cash` |
| ordType | String | Sub order type<br>`market`: Market order |
| sz | String | Sub order quantity to buy or sell |
| state | String | Sub order state<br>`canceled`<br>`live`<br>`partially_filled`<br>`filled`<br>`cancelling` |
| side | String | Sub order side<br>`buy``sell` |
| px | String | Sub order limit price<br>If it's a market order, "-1" will be return |
| fee | String | Sub order fee |
| feeCcy | String | Sub order fee currency |
| avgPx | String | Sub order average filled price |
| accFillSz | String | Sub order accumulated fill quantity |
| tag | String | Order tag |
| algoClOrdId | String | Client-supplied Algo ID |

### WS / Recurring buy orders channel

Retrieve recurring buy orders. Data will be pushed when triggered by events. It will also be pushed in regular interval according to subscription granularity.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "algo-recurring-buy",
        "instType": "SPOT"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "algo-recurring-buy",
        "instType": "SPOT"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`algo-recurring-buy` |
| \> instType | String | Yes | Instrument type<br>`SPOT`<br>`ANY` |
| \> algoId | String | No | Algo Order ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "algo-recurring-buy",
        "instType": "SPOT"
    },
        "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"algo-recurring-buy\", \"instType\" : \"FUTURES\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type |
| \> algoId | String | No | Algo Order ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "algo-recurring-buy",
        "instType": "SPOT",
        "uid": "447*******584"
    },
    "data": [{
        "algoClOrdId": "",
        "algoId": "644497312047435776",
        "algoOrdType": "recurring",
        "amt": "100",
        "cTime": "1699932133373",
        "cycles": "0",
        "instType": "SPOT",
        "investmentAmt": "0",
        "investmentCcy": "USDC",
        "mktCap": "0",
        "nextInvestTime": "1699934415300",
        "pTime": "1699933314691",
        "period": "hourly",
        "pnlRatio": "0",
        "recurringDay": "",
        "recurringHour": "1",
        "recurringList": [{
            "avgPx": "0",
            "ccy": "BTC",
            "profit": "0",
            "px": "36482",
            "ratio": "0.2",
            "totalAmt": "0"
        }, {
            "avgPx": "0",
            "ccy": "ETH",
            "profit": "0",
            "px": "2057.54",
            "ratio": "0.8",
            "totalAmt": "0"
        }],
        "recurringTime": "12",
        "state": "running",
        "stgyName": "stg1",
        "tag": "",
        "timeZone": "8",
        "totalAnnRate": "0",
        "totalPnl": "0",
        "uTime": "1699932136249",
        "tradeQuoteCcy": "USDT"
    }]
}
```

#### Response parameters when data is pushed.

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instType | String | Instrument type |
| \> algoId | String | Algo Order ID |
| \> uid | String | User ID |
| data | Array of objects | Subscribed data |
| \> algoId | String | Algo ID |
| \> algoClOrdId | String | Client-supplied Algo ID |
| \> instType | String | Instrument type |
| \> cTime | String | Algo order created time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> uTime | String | Algo order updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> algoOrdType | String | Algo order type<br>`recurring`: recurring buy |
| \> state | String | Algo order state<br>`running`<br>`stopping`<br>`stopped`<br>`pause` |
| \> stgyName | String | Custom name for trading bot, no more than 40 characters |
| \> recurringList | Array of objects | Recurring buy info |
| >\> ccy | String | Recurring buy currency, e.g. `BTC` |
| >\> ratio | String | Proportion of recurring currency assets, e.g. "0.2" representing 20% |
| >\> totalAmt | String | Accumulated quantity in unit of recurring buy currency |
| >\> profit | String | Profit in unit of `investmentCcy` |
| >\> avgPx | String | Average price of recurring buy, quote currency is `investmentCcy` |
| >\> px | String | Current market price, quote currency is `investmentCcy` |
| \> period | String | Period<br>`monthly`<br>`weekly`<br>`daily`<br>`hourly` |
| \> recurringDay | String | Recurring buy date<br>When the period is `monthly`, the value range is an integer of \[1,28\]<br>When the period is `weekly`, the value range is an integer of \[1,7\] |
| \> recurringHour | String | Recurring buy by hourly<br>`1`/`4`/`8`/`12`, e.g. `4` represents "recurring buy every 4 hour" |
| \> recurringTime | String | Recurring buy time, the value range is an integer of \[0,23\] |
| \> timeZone | String | UTC time zone, the value range is an integer of \[-12,14\]<br>e.g. "8" representing UTC+8 (East 8 District), Beijing Time |
| \> amt | String | Quantity invested per cycle |
| \> investmentAmt | String | Accumulate quantity invested |
| \> investmentCcy | String | The invested quantity unit, can only be `USDT`/`USDC` |
| \> nextInvestTime | String | Next invest time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> totalPnl | String | Total P&L |
| \> totalAnnRate | String | Total annualized rate of yield |
| \> pnlRatio | String | Rate of yield |
| \> mktCap | String | Market value in unit of `USDT` |
| \> cycles | String | Accumulate recurring buy cycles |
| \> tag | String | Order tag |
| \> pTime | String | Push time of algo order information, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> tradeQuoteCcy | String | The quote currency for trading. |

## Copy Trading

Lead trading API Workflow as follows:

**1\. Apply to become a leading trader.**

- The procedure can refer to [How to become a lead trader](https://www.okx.com/help/11639154398221);

- You can know whether you are a lead trader by checking whether `roleType` or `spotRoleType` from [Get account configuration](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-account-configuration) is 1.

**2\. Leading instruments:**

- [GET / Leading instruments](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-get-leading-instruments) can get instruments that are supported to have leading trades and the instruments that you enable leading trade. For instruments that are disenabled copy trading, you can still trade normally, but copy trading will not be triggered;

- [Amend leading instruments](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-amend-leading-instruments) can amend your leading instruments. You need to set initial leading instruments while applying to become a leading trader. All non-leading contracts can't have position or pending orders for the current request when setting non-leading contracts as leading contracts.

**3\. Open position:**

- You can open the position by placing order endpoints and channels including [Place order](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-place-order) endpoint, [Place multiple orders](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-place-multiple-orders) endpoint, [Place order channel](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-place-order), [Place multiple orders channel](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-place-multiple-orders), `tdMode` should be `spot_isolated` for `SPOT` lead trading.

- For buy/sell mode, the orders must be in the same direction as your existing positions and open orders. You can select the direction you want if the instrument does not have position and pending orders.
- For long/short mode, you can open long or open short as you want.

**4\. Close position**

- You can close the position with customized price or size by placing order endpoints and channels including [Place order](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-place-order) endpoint, [Place multiple orders](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-place-multiple-orders) endpoint, [Place order channel](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-place-order), [Place multiple orders channel](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-place-multiple-orders), or close the position by [Close positions](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-close-positions) / [Close lead position](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-post-close-lead-position);

- [Close positions](https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-close-positions) can close certain position under the current instrument(e.g. the long or short position under long/shor mode ), which can contain multiple leading positions;

- [Close lead position](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-post-close-lead-position) can only close a leading position once a time. It is required to pass subPosId which can get from [Get existing leading positions](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-get-existing-lead-positions).

**5\. TP/SL**

- TP/SL can be set by [Place algo order](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-mass-cancel-order) or [Place lead stop order](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-post-place-lead-stop-order);

- [Place algo order](https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-mass-cancel-order) can set TP/SL for certain position under the current instrument(e.g. the long or short position under long/shor mode ), which can contain multiple leading positions;

- [Place lead stop order](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-post-place-lead-stop-order) set set TP/SL for only a leading position once a time. It is required to pass subPosId which can get from [Get existing leading positions](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-get-existing-lead-positions).

### GET / Existing lead positions

Retrieve lead positions that are not closed.

Returns reverse chronological order with `openTime`

#### Rate limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/current-subpositions`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/current-subpositions?instId=BTC-USDT-SWAP
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>It returns all types by default. |
| instId | String | No | Instrument ID, e.g. BTC-USDT-SWAP |
| after | String | No | Pagination of data to return records earlier than the requested `subPosId`. |
| before | String | No | Pagination of data to return records newer than the requested `subPosId`. |
| limit | String | No | Number of results per request. Maximum is 500. Default is 500. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "algoId": "",
            "ccy": "USDT",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "lever": "3",
            "margin": "12.6417",
            "markPx": "38205.8",
            "mgnMode": "isolated",
            "openAvgPx": "37925.1",
            "openOrdId": "",
            "openTime": "1701231120479",
            "posSide": "net",
            "slOrdPx": "",
            "slTriggerPx": "",
            "subPos": "1",
            "subPosId": "649945658862370816",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "uniqueCode": "25CD5A80241D6FE6",
            "upl": "0.2807",
            "uplRatio": "0.0222042921442527",
            "availSubPos": "1"
        },
        {
            "algoId": "",
            "ccy": "USDT",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "lever": "3",
            "margin": "12.6263333333333333",
            "markPx": "38205.8",
            "mgnMode": "isolated",
            "openAvgPx": "37879",
            "openOrdId": "",
            "openTime": "1701225074786",
            "posSide": "net",
            "slOrdPx": "",
            "slTriggerPx": "",
            "subPos": "1",
            "subPosId": "649920301388038144",
            "tpOrdPx": "",
            "tpTriggerPx": "",
            "uniqueCode": "25CD5A80241D6FE6",
            "upl": "0.3268",
            "uplRatio": "0.0258824150584758",
            "availSubPos": "1"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| subPosId | String | Lead position ID |
| posSide | String | Position side<br>`long`<br>`short`<br>`net`<br>(Long positions have positive subPos; short positions have negative subPos) |
| mgnMode | String | Margin mode. `cross``isolated` |
| lever | String | Leverage |
| openOrdId | String | Order ID for opening position, only applicable to lead position |
| openAvgPx | String | Average open price |
| openTime | String | Open time |
| subPos | String | Quantity of positions |
| tpTriggerPx | String | Take-profit trigger price. |
| slTriggerPx | String | Stop-loss trigger price. |
| algoId | String | Stop order ID |
| instType | String | Instrument type |
| tpOrdPx | String | Take-profit order price, it is -1 for market price |
| slOrdPx | String | Stop-loss order price, it is -1 for market price |
| margin | String | Margin |
| upl | String | Unrealized profit and loss |
| uplRatio | String | Unrealized profit and loss ratio |
| markPx | String | Latest mark price, only applicable to contract |
| uniqueCode | String | Lead trader unique code |
| ccy | String | Margin currency |
| availSubPos | String | Quantity of positions that can be closed |

### GET / Lead position history

Retrieve the completed lead position of the last 3 months.

Returns reverse chronological order with `subPosId`.

#### Rate limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/subpositions-history`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/subpositions-history?instId=BTC-USDT-SWAP
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>It returns all types by default. |
| instId | String | No | Instrument ID, e.g. BTC-USDT-SWAP |
| after | String | No | Pagination of data to return records earlier than the requested `subPosId`. |
| before | String | No | Pagination of data to return records newer than the requested `subPosId`. |
| limit | String | No | Number of results per request. Maximum is 100. Default is 100. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "closeAvgPx": "37617.5",
            "closeTime": "1701188587950",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "lever": "3",
            "margin": "37.41",
            "markPx": "38203.4",
            "mgnMode": "isolated",
            "openAvgPx": "37410",
            "openOrdId": "",
            "openTime": "1701184638702",
            "pnl": "0.6225",
            "pnlRatio": "0.0166399358460306",
            "posSide": "net",
            "profitSharingAmt": "0.0407967",
            "subPos": "3",
            "closeSubPos": "2",
            "type": "1",
            "subPosId": "649750700213698561",
            "uniqueCode": "25CD5A80241D6FE6"
        },
        {
            "ccy": "USDT",
            "closeAvgPx": "37617.5",
            "closeTime": "1701188587950",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "lever": "3",
            "margin": "24.94",
            "markPx": "38203.4",
            "mgnMode": "isolated",
            "openAvgPx": "37410",
            "openOrdId": "",
            "openTime": "1701184635381",
            "pnl": "0.415",
            "pnlRatio": "0.0166399358460306",
            "posSide": "net",
            "profitSharingAmt": "0.0271978",
            "subPos": "2",
            "closeSubPos": "2",
            "type": "2",
            "subPosId": "649750686292803585",
            "uniqueCode": "25CD5A80241D6FE6"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| subPosId | String | Lead position ID |
| posSide | String | Position side<br>`long`<br>`short`<br>`net`<br>(long position has positive subPos; short position has negative subPos) |
| mgnMode | String | Margin mode. `cross``isolated` |
| lever | String | Leverage |
| openOrdId | String | Order ID for opening position, only applicable to lead position |
| openAvgPx | String | Average open price |
| openTime | String | Time of opening |
| subPos | String | Quantity of positions |
| closeTime | String | Time of closing position |
| closeAvgPx | String | Average price of closing position |
| pnl | String | Profit and loss |
| pnlRatio | String | P&L ratio |
| instType | String | Instrument type |
| margin | String | Margin |
| ccy | String | Currency |
| markPx | String | Latest mark price, only applicable to contract |
| uniqueCode | String | Lead trader unique code |
| profitSharingAmt | String | Profit sharing amount, only applicable to copy trading. Note: this parameter is already deprecated. |
| closeSubPos | String | Quantity of positions that is already closed |
| type | String | The type of closing position<br>`1`：Close position partially;<br>`2`：Close all |

### POST / Place lead stop order

Set TP/SL for the current lead position that are not closed.

#### Rate limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/copytrading/algo-order`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/algo-order
body
{
    "subPosId": "518541406042591232",
    "tpTriggerPx": "10000"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`, the default value |
| subPosId | String | Yes | Lead position ID |
| tpTriggerPx | String | Conditional | Take-profit trigger price. Take-profit order price will be the market price after triggering. At least one of tpTriggerPx and slTriggerPx must be filled<br>The take profit order will be deleted if it is 0 |
| slTriggerPx | String | Conditional | Stop-loss trigger price. Stop-loss order price will be the market price after triggering. The stop loss order will be deleted if it is 0 |
| tpOrdPx | String | No | Take-profit order price<br>If the price is -1, take-profit will be executed at the market price, the default is `-1`<br>Only applicable to `SPOT` lead trader |
| slOrdPx | String | No | Stop-loss order price<br>If the price is -1, stop-loss will be executed at the market price, the default is `-1`<br>Only applicable to `SPOT` lead trader |
| tpTriggerPxType | String | No | Take-profit trigger price type <br>`last`: last price<br>`index`: index price<br>`mark`: mark price <br>Default is `last` |
| slTriggerPxType | String | No | Stop-loss trigger price type<br>`last`: last price <br>`index`: index price <br>`mark`: mark price <br>Default is last |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "subPosId": "518560559046594560",
            "tag":""
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| subPosId | String | Lead position ID |
| tag | String | Order tag |

### POST / Close lead position

You can only close a lead position once a time.

It is required to pass subPosId which can get from [Get existing leading positions](https://www.okx.com/docs-v5/en/#order-book-trading-copy-trading-get-existing-lead-positions).

#### Rate limit: 20 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/close-subposition`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/close-subposition
body
{
    "subPosId": "518541406042591232",
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`, the default value |
| subPosId | String | Yes | Lead position ID |
| ordType | String | No | Order type<br>`market`：Market order, the default value<br>`limit`：Limit order |
| px | String | No | Order price. Only applicable to `limit` order and `SPOT` lead trader <br>If the price is 0, the pending order will be canceled. <br>It is modifying order if you set `px` after placing limit order. |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "subPosId": "518560559046594560",
            "tag":""
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| subPosId | String | Lead position ID |
| tag | String | Order tag |

### GET / Leading instruments

Retrieve instruments that are supported to lead by the platform.
Retrieve instruments that the lead trader has set.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/instruments`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/instruments
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`, the default value |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "enabled": true,
            "instId": "BTC-USDT-SWAP"
        },
        {
            "enabled": true,
            "instId": "ETH-USDT-SWAP"
        },
        {
            "enabled": false,
            "instId": "ADA-USDT-SWAP"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| enabled | Boolean | Whether instrument is a lead instrument. `true` or `false` |

### POST / Amend leading instruments

The leading trader can amend current leading instruments, need to set initial leading instruments while applying to become a leading trader.

All non-leading instruments can't have position or pending orders for the current request when setting non-leading instruments as leading instruments.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/set-instruments`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/set-instruments
body
{
    "instId": "BTC-USDT-SWAP,ETH-USDT-SWAP"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`, the default value |
| instId | String | Yes | Instrument ID, e.g. BTC-USDT-SWAP. If there are multiple instruments, separate them with commas. |

The value of \`instId\` must include all instruments that you are going to have the lead trading with because the previous settings will be overwritten after the current request is set successfully

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "enabled": true,
            "instId": "BTC-USDT-SWAP"
        },
        {
            "enabled": true,
            "instId": "ETH-USDT-SWAP"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| enabled | Boolean | Whether you set it successfully<br>`true` or `false` |

### GET / Profit sharing details

The leading trader gets profits shared details for the last 3 months.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/profit-sharing-details`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/profit-sharing-details?limit=2
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>It returns all types by default. |
| after | String | No | Pagination of data to return records earlier than the requested `profitSharingId` |
| before | String | No | Pagination of data to return records newer than the requested `profitSharingId` |
| limit | String | No | Number of results per request. Maximum is 100. Default is 100. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "nickName": "Potato",
            "profitSharingAmt": "0.00536",
            "profitSharingId": "148",
            "portLink": "",
            "ts": "1723392000000",
            "instType": "SWAP"
        },
        {
            "ccy": "USDT",
            "nickName": "Apple",
            "profitSharingAmt": "0.00336",
            "profitSharingId": "20",
            "portLink": "",
            "ts": "1723392000000",
            "instType": "SWAP"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ccy | String | The currency of profit sharing. |
| profitSharingAmt | String | Profit sharing amount. It would be 0 if there is no any profit sharing. |
| nickName | String | Nickname of copy trader. |
| profitSharingId | String | Profit sharing ID. |
| instType | String | Instrument type |
| portLink | String | Portrait link |
| ts | String | Profit sharing time. |

### GET / Total profit sharing

The leading trader gets the total amount of profit shared since joining the platform.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/total-profit-sharing`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/total-profit-sharing
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>It returns all types by default. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "totalProfitSharingAmt": "0.6584928",
            "instType": "SWAP"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ccy | String | The currency of profit sharing. |
| totalProfitSharingAmt | String | Total profit sharing amount. |
| instType | String | Instrument type |

### GET / Unrealized profit sharing details

The leading trader gets the profit sharing details that are expected to be shared in the next settlement cycle.

The unrealized profit sharing details will update once there copy position is closed.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/unrealized-profit-sharing-details`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/unrealized-profit-sharing-details
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SPOT`<br>`SWAP`<br>It returns all types by default. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "nickName": "Potato",
            "portLink": "",
            "ts": "1669901824779",
            "unrealizedProfitSharingAmt": "0.455472",
            "instType": "SWAP"
        },
        {
            "ccy": "USDT",
            "nickName": "Apple",
            "portLink": "",
            "ts": "1669460210113",
            "unrealizedProfitSharingAmt": "0.033608",
            "instType": "SWAP"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ccy | String | The currency of profit sharing. e.g. USDT |
| unrealizedProfitSharingAmt | String | Unrealized profit sharing amount. |
| nickName | String | Nickname of copy trader. |
| instType | String | Instrument type |
| portLink | String | Portrait link |
| ts | String | Update time. |

### GET / Total unrealized profit sharing

The leading trader gets the total unrealized amount of profit shared.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/total-unrealized-profit-sharing`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/total-unrealized-profit-sharing
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "profitSharingTs": "1705852800000",
            "totalUnrealizedProfitSharingAmt": "0.114402985553185"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| profitSharingTs | String | The settlement time for the total unrealized profit sharing amount. Unix timestamp format in milliseconds, e.g.1597026383085 |
| totalUnrealizedProfitSharingAmt | String | Total unrealized profit sharing amount |

### POST / Amend profit sharing ratio

It is used to amend profit sharing ratio.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/amend-profit-sharing-ratio`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/amend-profit-sharing-ratio
body
{
    "instType": "SWAP",
    "profitSharingRatio": "0.1"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP` |
| profitSharingRatio | String | Yes | Profit sharing ratio. <br>0.1 represents 10% |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "result": true
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| result | Boolean | The result of setting <br>`true` |

### GET / Account configuration

Retrieve current account configuration related to copy/lead trading.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/config`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/config
```

#### Request Parameters

None

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "details": [
                {
                    "copyTraderNum": "1",
                    "instType": "SWAP",
                    "maxCopyTraderNum": "100",
                    "profitSharingRatio": "0",
                    "roleType": "1"
                },
                {
                    "copyTraderNum": "",
                    "instType": "SPOT",
                    "maxCopyTraderNum": "",
                    "profitSharingRatio": "",
                    "roleType": "0"
                }
            ],
            "nickName": "155***9957",
            "portLink": "",
            "uniqueCode": "5506D3681454A304"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| uniqueCode | String | User unique code |
| nickName | String | Nickname |
| portLink | String | Portrait link |
| details | Array of objects | Details |
| \> instType | String | Instrument type<br>`SPOT`<br>`SWAP` |
| \> roleType | String | Role type<br>`0`: General user<br>`1`: Leading trader<br>`2`: Copy trader |
| \> profitSharingRatio | String | Profit sharing ratio. <br>Only applicable to lead trader, or it will be "". 0.1 represents 10% |
| \> maxCopyTraderNum | String | Maximum number of copy traders |
| \> copyTraderNum | String | Current number of copy traders |

### POST / First copy settings

The first copy settings for the certain lead trader. You need to first copy settings after stopping copying.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/first-copy-settings`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/first-copy-settings
body
{
    "instType": "SWAP",
    "uniqueCode": "25CD5A80241D6FE6",
    "copyMgnMode": "cross",
    "copyInstIdType": "copy",
    "copyMode": "ratio_copy",
    "copyRatio": "1",
    "copyTotalAmt": "500",
    "subPosCloseType": "copy_close"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| copyMgnMode | String | Yes | Copy margin mode<br>`cross`: cross<br>`isolated`: isolated<br>`copy`: Use the same margin mode as lead trader when opening positions |
| copyInstIdType | String | Yes | Copy contract type setted<br>`custom`: custom by `instId` which is required；<br>`copy`: Keep your contracts consistent with this trader by automatically adding or removing contracts when they do |
| instId | String | Conditional | Instrument ID. <br>If there are multiple instruments, separate them with commas. |
| copyMode | String | No | Copy mode<br>`fixed_amount`: set the same fixed amount for each order, and `copyAmt` is required；<br>`ratio_copy`: set amount as a multiple of the lead trader’s order value, and `copyRatio` is required <br>The default is `fixed_amount` |
| copyTotalAmt | String | Yes | Maximum total amount in USDT. <br>The maximum total amount you'll invest at any given time across all orders in this copy trade<br>You won’t copy new orders if you exceed this amount |
| copyAmt | String | Conditional | Copy amount per order in USDT. |
| copyRatio | String | Conditional | Copy ratio per order. |
| tpRatio | String | No | Take profit per order. 0.1 represents 10% |
| slRatio | String | No | Stop loss per order. 0.1 represents 10% |
| slTotalAmt | String | No | Total stop loss in USDT for trader. <br>If your net loss (total profit - total loss) reaches this amount, you'll stop copying this trader |
| subPosCloseType | String | Yes | Action type for open positions<br>`market_close`: immediately close at market price<br>`copy_close`：close when trader closes<br>`manual_close`: close manually<br>The default is `copy_close` |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "result": true
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| result | Boolean | The result of setting <br>`true` |

### POST / Amend copy settings

You need to use this endpoint to amend copy settings

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/amend-copy-settings`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/amend-copy-settings
body
{
    "instType": "SWAP",
    "uniqueCode": "25CD5A80241D6FE6",
    "copyMgnMode": "cross",
    "copyInstIdType": "copy",
    "copyMode": "ratio_copy",
    "copyRatio": "1",
    "copyTotalAmt": "500",
    "subPosCloseType": "copy_close"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP` |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| copyMgnMode | String | Yes | Copy margin mode<br>`cross`: cross<br>`isolated`: isolated<br>`copy`: Use the same margin mode as lead trader when opening positions |
| copyInstIdType | String | Yes | Copy contract type setted<br>`custom`: custom by `instId` which is required；<br>`copy`: Keep your contracts consistent with this trader by automatically adding or removing contracts when they do |
| instId | String | Conditional | Instrument ID. <br>If there are multiple instruments, separate them with commas. |
| copyMode | String | No | Copy mode<br>`fixed_amount`: set the same fixed amount for each order, and `copyAmt` is required；<br>`ratio_copy`: set amount as a multiple of the lead trader’s order value, and `copyRatio` is required <br>The default is `fixed_amount` |
| copyTotalAmt | String | Yes | Maximum total amount in USDT. <br>The maximum total amount you'll invest at any given time across all orders in this copy trade<br>You won’t copy new orders if you exceed this amount |
| copyAmt | String | Conditional | Copy amount per order in USDT |
| copyRatio | String | Conditional | Copy ratio per order. |
| tpRatio | String | No | Take profit per order. 0.1 represents 10% |
| slRatio | String | No | Stop loss per order. 0.1 represents 10% |
| slTotalAmt | String | No | Total stop loss in USDT for trader.<br>If your net loss (total profit - total loss) reaches this amount, you'll stop copying this trader |
| subPosCloseType | String | Yes | Action type for open positions<br>`market_close`: immediately close at market price<br>`copy_close`：close when trader closes<br>`manual_close`: close manually<br>The default is `copy_close` |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "result": true
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| result | Boolean | The result of setting <br>`true` |

### POST / Stop copying

You need to use this endpoint to stop copy trading

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP request

`POST /api/v5/copytrading/stop-copy-trading`

> Request example

```
Copy to Clipboard
POST /api/v5/copytrading/stop-copy-trading
body
{
    "instType": "SWAP",
    "uniqueCode": "25CD5A80241D6FE6",
    "subPosCloseType": "manual_close"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP` |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| subPosCloseType | String | Yes | Action type for open positions, it is required if you have related copy position<br>`market_close`: immediately close at market price<br>`copy_close`：close when trader closes<br>`manual_close`: close manually |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "result": true
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| result | Boolean | The result of setting <br>`true` |

### GET / Copy settings

Retrieve the copy settings about certain lead trader.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/copy-settings`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/copy-settings?instType=SWAP&uniqueCode=25CD5A80241D6FE6
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP` |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "copyAmt": "",
            "copyInstIdType": "copy",
            "copyMgnMode": "isolated",
            "copyMode": "ratio_copy",
            "copyRatio": "1",
            "copyState": "1",
            "copyTotalAmt": "500",
            "instIds": [
                {
                    "enabled": "1",
                    "instId": "ADA-USDT-SWAP"
                },
                {
                    "enabled": "1",
                    "instId": "YFII-USDT-SWAP"
                }
            ],
            "slRatio": "",
            "slTotalAmt": "",
            "subPosCloseType": "copy_close",
            "tpRatio": "",
            "tag": ""
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| copyMode | String | Copy mode<br>`fixed_amount``ratio_copy` |
| copyAmt | String | Copy amount in USDT per order. |
| copyRatio | String | Copy ratio per order. |
| copyTotalAmt | String | Maximum total amount in USDT. <br>The maximum total amount you'll invest at any given time across all orders in this copy trade |
| tpRatio | String | Take profit per order. 0.1 represents 10% |
| slRatio | String | Stop loss per order. 0.1 represents 10% |
| copyInstIdType | String | Copy contract type setted<br>`custom`: custom by `instId` which is required；<br>`copy`: Keep your contracts consistent with this trader by automatically adding or removing contracts when they do |
| instIds | Array of objects | Instrument list. It will return all lead contracts of the lead trader |
| \> instId | String | Instrument ID |
| \> enabled | String | Whether copying this `instId`<br>`0``1` |
| slTotalAmt | String | Total stop loss in USDT for trader. |
| subPosCloseType | String | Action type for open positions<br>`market_close`: immediately close at market price<br>`copy_close`：close when trader closes<br>`manual_close`: close manually |
| copyMgnMode | String | Copy margin mode<br>`cross`: cross<br>`isolated`: isolated<br>`copy`: Use the same margin mode as lead trader when opening positions |
| ccy | String | Margin currency |
| copyState | String | Current copy state <br>`0`: non-copy, `1`: copy |
| tag | String | Order tag |

### GET / My lead traders

Retrieve my lead traders.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/current-lead-traders`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/current-lead-traders?instType=SWAP
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "beginCopyTime": "1701224821936",
            "ccy": "USDT",
            "copyTotalAmt": "500",
            "copyTotalPnl": "0",
            "leadMode": "public",
            "margin": "1.89395",
            "nickName": "Trader9527",
            "portLink": "",
            "profitSharingRatio": "0.08",
            "todayPnl": "0",
            "uniqueCode": "25CD5A80241D6FE6",
            "upl": "0"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| portLink | String | Portrait link |
| nickName | String | Nick name |
| margin | String | Margin for copy trading |
| copyTotalAmt | String | Copy total amount |
| copyTotalPnl | String | Copy total pnl |
| uniqueCode | String | Lead trader unique code |
| ccy | String | margin currency |
| profitSharingRatio | String | Profit sharing ratio. 0.1 represents 10% |
| beginCopyTime | String | Begin copying time. Unix timestamp format in milliseconds, e.g.1597026383085 |
| upl | String | Unrealized profit & loss |
| todayPnl | String | Today pnl |
| leadMode | String | Lead mode `public``private` |

### GET / Copy trading configuration

Public endpoint. Retrieve copy trading parameter configuration information of copy settings

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-config`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-config?instType=SWAP
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "maxCopyAmt": "1000",
            "maxCopyRatio": "100",
            "maxCopyTotalAmt": "30000",
            "maxSlRatio": "0.75",
            "maxTpRatio": "1.5",
            "minCopyAmt": "20",
            "minCopyRatio": "0.01"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| maxCopyAmt | String | Maximum copy amount per order in USDT when you are using copy mode `fixed_amount` |
| minCopyAmt | String | Minimum copy amount per order in USDT when you are using copy mode `fixed_amount` |
| maxCopyTotalAmt | String | Maximum copy total amount under the certain lead trader, the minimum is the same with `minCopyAmt` |
| minCopyRatio | String | Minimum ratio per order when you are using copy mode `ratio_copy` |
| maxCopyRatio | String | Maximum ratio per order when you are using copy mode `ratio_copy` |
| maxTpRatio | String | Maximum ratio of taking profit per order, the minimum is 0 |
| maxSlRatio | String | Maximum ratio of stopping loss per order, the minimum is 0 |

### GET / Lead trader ranks

Public endpoint. Retrieve lead trader ranks.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-lead-traders`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-lead-traders?instType=SWAP
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| sortType | String | No | Sort type<br>`overview`: overview, the default value<br>`pnl`: profit and loss<br>`aum`: assets under management<br>`win_ratio`: win ratio<br>`pnl_ratio`: pnl ratio<br>`current_copy_trader_pnl`: current copy trader pnl |
| state | String | No | Lead trader state<br>`0`: All lead traders, the default, including vacancy and non-vacancy <br>`1`: lead traders who have vacancy |
| minLeadDays | String | No | Minimum lead days<br>`1`: 7 days<br>`2`: 30 days<br>`3`: 90 days<br>`4`: 180 days |
| minAssets | String | No | Minimum assets in USDT |
| maxAssets | String | No | Maximum assets in USDT |
| minAum | String | No | Minimum assets in USDT under management. |
| maxAum | String | No | Maximum assets in USDT under management. |
| dataVer | String | No | Data version. It is 14 numbers. e.g. 20231010182400. Generally, it is used for pagination <br>A new version will be generated every 10 minutes. Only last 5 versions are stored<br>The default is latest version. If it is not exist, error will not be throwed and the latest version will be used. |
| page | String | No | Page for pagination |
| limit | String | No | Number of results per request. The maximum is 20; the default is 10 |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "dataVer": "20231129213200",
            "ranks": [
                {
                    "accCopyTraderNum": "3536",
                    "aum": "1509265.3238761567721365",
                    "ccy": "USDT",
                    "copyState": "0",
                    "copyTraderNum": "999",
                    "leadDays": "156",
                    "maxCopyTraderNum": "1000",
                    "nickName": "Crypto to the moon",
                    "pnl": "48805.1105999999972258",
                    "pnlRatio": "1.6898",
                    "pnlRatios": [
                        {
                            "beginTs": "1701187200000",
                            "pnlRatio": "1.6744"
                        },
                        {
                            "beginTs": "1700755200000",
                            "pnlRatio": "1.649"
                        }
                    ],
                    "portLink": "https://static.okx.com/cdn/okex/users/headimages/20230624/f49a683aaf5949ea88b01bbc771fb9fc",
                    "traderInsts": [
                        "ICP-USDT-SWAP",
                        "MINA-USDT-SWAP"

                    ],
                    "uniqueCode": "540D011FDACCB47A",
                    "winRatio": "0.6957"
                }
            ],
            "totalPage": "1"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| dataVer | String | Data version |
| totalPage | String | Total number of pages |
| ranks | Array of objects | The rank information of lead traders |
| \> aum | String | assets under management |
| \> copyState | String | Current copy state <br>`0`: non-copy, `1`: copy |
| \> maxCopyTraderNum | String | Maximum number of copy traders |
| \> copyTraderNum | String | Current number of copy traders |
| \> accCopyTraderNum | String | Accumulated number of copy traders |
| \> portLink | String | Portrait link |
| \> nickName | String | Nick name |
| \> ccy | String | Margin currency |
| \> uniqueCode | String | Lead trader unique code |
| \> winRatio | String | Win ratio, 0.1 represents 10% |
| \> leadDays | String | Lead days |
| \> traderInsts | Array of strings | Contract list which lead trader is leading |
| \> pnl | String | Pnl (in USDT) of last 90 days. |
| \> pnlRatio | String | Pnl ratio of last 90 days. 0.1 represents 10% |
| \> pnlRatios | Array of objects | Pnl ratios |
| >\> beginTs | String | Begin time of pnl ratio on that day |
| >\> pnlRatio | String | Pnl ratio on that day |

### GET / Lead trader weekly pnl

Public endpoint. Retrieve lead trader weekly pnl. Results are returned in counter chronological order.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-weekly-pnl`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-weekly-pnl?instType=SWAP&uniqueCode=D9ADEAB33AE9EABD
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "beginTs": "1701014400000",
            "pnl": "-2.8428",
            "pnlRatio": "-0.0106"
        },
        {
            "beginTs": "1700409600000",
            "pnl": "81.8446",
            "pnlRatio": "0.3036"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| beginTs | String | Begin time of pnl ratio on that week |
| pnl | String | Pnl on that week |
| pnlRatio | String | Pnl ratio on that week |

### GET / Lead trader daily pnl

Public endpoint. Retrieve lead trader daily pnl. Results are returned in counter chronological order.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-pnl`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-pnl?instType=SWAP&uniqueCode=D9ADEAB33AE9EABD&lastDays=1
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| lastDays | String | Yes | Last days<br>`1`: last 7 days <br>`2`: last 30 days<br>`3`: last 90 days <br>`4`: last 365 days |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "beginTs": "1701100800000",
            "pnl": "97.3309",
            "pnlRatio": "0.3672"
        },
        {
            "beginTs": "1701014400000",
            "pnl": "96.7755",
            "pnlRatio": "0.3651"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| beginTs | String | Begin time on that day |
| pnl | String | Accumulated pnl |
| pnlRatio | String | Accumulated pnl ratio |

### GET / Lead trader stats

Public endpoint. Key data related to lead trader performance.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-stats`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-stats?instType=SWAP&uniqueCode=D9ADEAB33AE9EABD&lastDays=1
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| lastDays | String | Yes | Last days<br>`1`: last 7 days <br>`2`: last 30 days<br>`3`: last 90 days <br>`4`: last 365 days |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "avgSubPosNotional": "213.1038",
            "ccy": "USDT",
            "curCopyTraderPnl": "96.8071",
            "investAmt": "265.095252476476294",
            "lossDays": "1",
            "profitDays": "2",
            "winRatio": "0.6667"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| winRatio | String | Win ratio |
| profitDays | String | Profit days |
| lossDays | String | Loss days |
| curCopyTraderPnl | String | Current copy trader pnl (USDT) |
| avgSubPosNotional | String | Average lead position notional (USDT) |
| investAmt | String | Investment amount (USDT) |
| ccy | String | Margin currency |

### GET / Lead trader currency preferences

Public endpoint. The most frequently traded crypto of this lead trader. Results are sorted by ratio from large to small.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-preference-currency`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-preference-currency?instType=SWAP&uniqueCode=CB4594A3BB5D3538
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "ETH",
            "ratio": "0.8881"
        },
        {
            "ccy": "BTC",
            "ratio": "0.0666"
        },
        {
            "ccy": "YFII",
            "ratio": "0.0453"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ccy | String | Currency |
| ratio | String | Ratio. 0.1 represents 10% |

### GET / Lead trader current lead positions

Public endpoint. Get current leading positions of lead trader

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-current-subpositions`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-current-subpositions?instType=SWAP&uniqueCode=D9ADEAB33AE9EABD
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value. |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| after | String | No | Pagination of data to return records earlier than the requested `subPosId`. |
| before | String | No | Pagination of data to return records newer than the requested `subPosId`. |
| limit | String | No | Number of results per request. Maximum is 100. Default is 100. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "instId": "ETH-USDT-SWAP",
            "instType": "SWAP",
            "lever": "5",
            "margin": "16.23304",
            "markPx": "2027.31",
            "mgnMode": "isolated",
            "openAvgPx": "2029.13",
            "openTime": "1701144639417",
            "posSide": "short",
            "subPos": "4",
            "subPosId": "649582930998104064",
            "uniqueCode": "D9ADEAB33AE9EABD",
            "upl": "0.0728",
            "uplRatio": "0.0044846806266725"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| subPosId | String | Lead position ID |
| posSide | String | Position side<br>`long`<br>`short`<br>`net`<br>(Long positions have positive subPos; short positions have negative subPos) |
| mgnMode | String | Margin mode. `cross``isolated` |
| lever | String | Leverage |
| openAvgPx | String | Average open price |
| openTime | String | Open time |
| subPos | String | Quantity of positions |
| instType | String | Instrument type |
| margin | String | Margin |
| upl | String | Unrealized profit and loss |
| uplRatio | String | Unrealized profit and loss ratio |
| markPx | String | Latest mark price, only applicable to contract |
| uniqueCode | String | Lead trader unique code |
| ccy | String | Currency |

### GET / Lead trader lead position history

Public endpoint. Retrieve the lead trader completed leading position of the last 3 months.

Returns reverse chronological order with `subPosId`.

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-subpositions-history`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-subpositions-history?instType=SWAP&uniqueCode=9A8534AB09862774
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value. |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| after | String | No | Pagination of data to return records earlier than the requested `subPosId`. |
| before | String | No | Pagination of data to return records newer than the requested `subPosId`. |
| limit | String | No | Number of results per request. Maximum is 100. Default is 100. |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "closeAvgPx": "28385.9",
            "closeTime": "1697709137162",
            "instId": "BTC-USDT-SWAP",
            "instType": "SWAP",
            "lever": "20",
            "margin": "4.245285",
            "mgnMode": "isolated",
            "openAvgPx": "28301.9",
            "openTime": "1697698048031",
            "pnl": "0.252",
            "pnlRatio": "0.05935997229868",
            "posSide": "long",
            "subPos": "3",
            "subPosId": "635126416883355648",
            "uniqueCode": "9A8534AB09862774"
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID, e.g. BTC-USDT-SWAP |
| subPosId | String | Lead position ID |
| posSide | String | Position side<br>`long`<br>`short`<br>`net`<br>(long position has positive subPos; short position has negative subPos) |
| mgnMode | String | Margin mode. `cross``isolated` |
| lever | String | Leverage |
| openAvgPx | String | Average open price |
| openTime | String | Time of opening |
| subPos | String | Quantity of positions |
| closeTime | String | Time of closing position |
| closeAvgPx | String | Average price of closing position |
| pnl | String | Profit and loss |
| pnlRatio | String | P&L ratio |
| instType | String | Instrument type |
| margin | String | Margin |
| ccy | String | Currency |
| uniqueCode | String | Lead trader unique code |

### GET / Copy traders

Public endpoint. Retrieve copy trader coming from certain lead trader. Return according to `pnl` from high to low

#### Rate limit: 5 requests per 2 seconds

#### Rate limit rule: IP

#### Permission: Read

#### HTTP request

`GET /api/v5/copytrading/public-copy-traders`

> Request example

```
Copy to Clipboard
GET /api/v5/copytrading/public-copy-traders?instType=SWAP&uniqueCode=D9ADEAB33AE9EABD
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | No | Instrument type<br>`SWAP`, the default value |
| uniqueCode | String | Yes | Lead trader unique code<br>A combination of case-sensitive alphanumerics, all numbers and the length is 16 characters, e.g. 213E8C92DC61EFAC |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "copyTotalPnl": "2060.12242",
            "copyTraderNumChg": "1",
            "copyTraderNumChgRatio": "0.5",
            "copyTraders": [
                {
                    "beginCopyTime": "1686125051000",
                    "nickName": "bre***@gmail.com",
                    "pnl": "1076.77388",
                    "portLink": ""
                },
                {
                    "beginCopyTime": "1698133811000",
                    "nickName": "MrYanDao505",
                    "pnl": "983.34854",
                    "portLink": "https://static.okx.com/cdn/okex/users/headimages/20231010/fd31f45e99fe41f7bb219c0b53ae0ada"
                }
            ]
        }
    ],
    "msg": ""
}
```

#### Response parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| copyTotalPnl | String | Total copy trader profit and loss |
| ccy | String | The currency name of profit and loss |
| copyTraderNumChg | String | Number change in last 7 days |
| copyTraderNumChgRatio | String | Ratio change in last 7 days |
| copyTraders | Array of objects | Copy trader information |
| \> beginCopyTime | String | Begin copying time. Unix timestamp format in milliseconds, e.g.1597026383085 |
| \> nickName | String | Nick name |
| \> portLink | String | Copy trader portrait link |
| \> pnl | String | Copy trading profit and loss |

### WS / Lead trading notification channel

The notification when failing to lead trade.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "copytrading-lead-notification",
        "instType": "SWAP"
    }]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPrivateAsync import WsPrivateAsync

def callbackFunc(message):
    print(message)

async def main():

    ws = WsPrivateAsync(
        apiKey = "YOUR_API_KEY",
        passphrase = "YOUR_PASSPHRASE",
        secretKey = "YOUR_SECRET_KEY",
        url = "wss://ws.okx.com:8443/ws/v5/business",
        useServerTime=False
    )
    await ws.start()
    args = [{
        "channel": "copytrading-lead-notification",
        "instType": "SWAP"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`copytrading-lead-notification` |
| \> instType | String | Yes | Instrument type<br>`SWAP` |
| \> instId | String | No | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "copytrading-lead-notification",
        "instType": "SWAP"
    },
    "connId": "aa993428"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"copytrading-lead-notification\", \"instType\" : \"FUTURES\"}]}",
  "connId":"a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instType | String | Yes | Instrument type<br>`SWAP` |
| \> instId | String | No | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example:

```
Copy to Clipboard
{
    "arg": {
        "channel": "copytrading-lead-notification",
        "instType": "SWAP",
        "uid": "525627088439549953"
    },
    "data": [
        {
            "infoType": "2",
            "instId": "",
            "instType": "SWAP",
            "maxLeadTraderNum": "3",
            "minLeadEq": "",
            "posSide": "",
            "side": "",
            "subPosId": "667695035433385984",
            "uniqueCode": "3AF72F63E3EAD701"
        }
    ]
}
```

#### Push data parameters

| Parameter | Type | Description |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> instType | String | Instrument type |
| data | Array of objects | Subscribed data |
| \> instType | String | Instrument type |
| \> infoType | String | Information type<br>`1`: lead trading failed due to touch max position limitation <br>`2`: lead trading failed due to touch the maximum daily number of lead trading <br>`3`: lead trading failed due to your USDT equity less than the minimum USDT equity of lead trading |
| \> subPosId | String | Lead position ID |
| \> uniqueCode | String | Lead trader unique code |
| \> instId | String | Instrument ID |
| \> side | String | Side `buy``sell` |
| \> posSide | String | Position side <br>`long`<br>`short`<br>`net` |
| \> maxLeadTraderNum | String | Maximum daily number of lead trading. |
| \> minLeadEq | String | Minimum USDT equity of lead trading. |

## Market Data

The API endpoints of `Market Data` do not require authentication.

There are multiple services for market data, and each service has an independent cache. A random service will be requested for every request. So for two requests, it’s expected that the data obtained in the second request is earlier than the first request.

### GET / Tickers

Retrieve the latest price snapshot, best bid/ask price, and trading volume in the last 24 hours. Best ask price may be lower than the best bid price during the pre-open period.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/tickers`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/tickers?instType=SWAP
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve the latest price snapshot, best bid/ask price, and trading volume in the last 24 hours
result = marketDataAPI.get_tickers(
    instType="SWAP"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instType | String | Yes | Instrument type<br>`SPOT`<br>`SWAP`<br>`FUTURES`<br>`OPTION` |
| instFamily | String | No | Instrument family<br>Applicable to `FUTURES`/`SWAP`/`OPTION` |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
     {
        "instType":"SWAP",
        "instId":"LTC-USD-SWAP",
        "last":"9999.99",
        "lastSz":"1",
        "askPx":"9999.99",
        "askSz":"11",
        "bidPx":"8888.88",
        "bidSz":"5",
        "open24h":"9000",
        "high24h":"10000",
        "low24h":"8888.88",
        "volCcy24h":"2222",
        "vol24h":"2222",
        "sodUtc0":"0.1",
        "sodUtc8":"0.1",
        "ts":"1597026383085"
     },
     {
        "instType":"SWAP",
        "instId":"BTC-USD-SWAP",
        "last":"9999.99",
        "lastSz":"1",
        "askPx":"9999.99",
        "askSz":"11",
        "bidPx":"8888.88",
        "bidSz":"5",
        "open24h":"9000",
        "high24h":"10000",
        "low24h":"8888.88",
        "volCcy24h":"2222",
        "vol24h":"2222",
        "sodUtc0":"0.1",
        "sodUtc8":"0.1",
        "ts":"1597026383085"
    }
  ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| last | String | Last traded price |
| lastSz | String | Last traded size. 0 represents there is no trading volume |
| askPx | String | Best ask price |
| askSz | String | Best ask size |
| bidPx | String | Best bid price |
| bidSz | String | Best bid size |
| open24h | String | Open price in the past 24 hours |
| high24h | String | Highest price in the past 24 hours |
| low24h | String | Lowest price in the past 24 hours |
| volCcy24h | String | 24h trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. e.g. the unit is BTC for BTC-USD-SWAP and BTC-USDT-SWAP <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| vol24h | String | 24h trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| sodUtc0 | String | Open price in the UTC 0 |
| sodUtc8 | String | Open price in the UTC 8 |
| ts | String | Ticker data generation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / Ticker

Retrieve the latest price snapshot, best bid/ask price, and trading volume in the last 24 hours. Best ask price may be lower than the best bid price during the pre-open period.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/ticker`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/ticker?instId=BTC-USD-SWAP
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve the latest price snapshot, best bid/ask price, and trading volume in the last 24 hours
result = marketDataAPI.get_ticker(
    instId="BTC-USD-SWAP"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USD-SWAP` |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
     {
        "instType":"SWAP",
        "instId":"BTC-USD-SWAP",
        "last":"9999.99",
        "lastSz":"0.1",
        "askPx":"9999.99",
        "askSz":"11",
        "bidPx":"8888.88",
        "bidSz":"5",
        "open24h":"9000",
        "high24h":"10000",
        "low24h":"8888.88",
        "volCcy24h":"2222",
        "vol24h":"2222",
        "sodUtc0":"2222",
        "sodUtc8":"2222",
        "ts":"1597026383085"
    }
  ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instType | String | Instrument type |
| instId | String | Instrument ID |
| last | String | Last traded price |
| lastSz | String | Last traded size. 0 represents there is no trading volume |
| askPx | String | Best ask price |
| askSz | String | Best ask size |
| bidPx | String | Best bid price |
| bidSz | String | Best bid size |
| open24h | String | Open price in the past 24 hours |
| high24h | String | Highest price in the past 24 hours |
| low24h | String | Lowest price in the past 24 hours |
| volCcy24h | String | 24h trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| vol24h | String | 24h trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| sodUtc0 | String | Open price in the UTC 0 |
| sodUtc8 | String | Open price in the UTC 8 |
| ts | String | Ticker data generation time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |

### GET / Order book

Retrieve order book of the instrument. The data will be updated once every 50 milliseconds. Best ask price may be lower than the best bid price during the pre-open period.

This endpoint does not return data immediately. Instead, it returns the latest data once the server-side cache has been updated.

#### Rate Limit: 40 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/books`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/books?instId=BTC-USDT
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve order book of the instrument
result = marketDataAPI.get_orderbook(
    instId="BTC-USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| sz | String | No | Order book depth per side. Maximum 400, e.g. 400 bids + 400 asks <br>Default returns to `1` depth data |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "asks": [
                [
                    "41006.8",
                    "0.60038921",
                    "0",
                    "1"
                ]
            ],
            "bids": [
                [
                    "41006.3",
                    "0.30178218",
                    "0",
                    "2"
                ]
            ],
            "ts": "1629966436396",
            "seqId": 3235851742
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| asks | Array of Arrays | Order book on sell side |
| bids | Array of Arrays | Order book on buy side |
| ts | String | Order book generation time |
| seqId | Integer | Sequence ID of the current message |

An example of the array of asks and bids values: \["411.8", "10", "0", "4"\]

\- "411.8" is the depth price

\- "10" is the quantity at the price (number of contracts for derivatives, quantity in base currency for Spot and Spot Margin)

\- "0" is part of a deprecated feature and it is always "0"

\- "4" is the number of orders at the price.

The order book data will be updated around once a second during the call auction.

### GET / Full order book

Retrieve order book of the instrument. The data will be updated once a second. Best ask price may be lower than the best bid price during the pre-open period.

This endpoint does not return data immediately. Instead, it returns the latest data once the server-side cache has been updated.

#### Rate Limit: 10 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/books-full`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/books-full?instId=BTC-USDT&sz=1
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| sz | String | No | Order book depth per side. Maximum 5000, e.g. 5000 bids + 5000 asks <br> Default returns to `1` depth data. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "asks": [
                [
                    "41006.8",
                    "0.60038921",
                    "1"
                ]
            ],
            "bids": [
                [
                    "41006.3",
                    "0.30178218",
                    "2"
                ]
            ],
            "ts": "1629966436396"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| asks | Array of Arrays | Order book on sell side |
| bids | Array of Arrays | Order book on buy side |
| ts | String | Order book generation time |

An example of the array of asks and bids values: \["411.8", "10", "4"\]

\- "411.8" is the depth price

\- "10" is the quantity at the price (number of contracts for derivatives, quantity in base currency for Spot and Spot Margin)

\- "4" is the number of orders at the price.

The order book data will be updated around once a second during the call auction.

### GET / Candlesticks

Retrieve the candlestick charts. This endpoint can retrieve the latest 1,440 data entries. Charts are returned in groups based on the requested bar.

#### Rate Limit: 40 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/candles`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/candles?instId=BTC-USDT
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve the candlestick charts
result = marketDataAPI.get_candlesticks(
    instId="BTC-USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| bar | String | No | Bar size, the default is `1m`<br>e.g. \[1m/3m/5m/15m/30m/1H/2H/4H\] <br>UTC+8 opening price k-line: \[6H/12H/1D/2D/3D/1W/1M/3M\]<br>UTC+0 opening price k-line: \[6Hutc/12Hutc/1Dutc/2Dutc/3Dutc/1Wutc/1Mutc/3Mutc\] |
| after | String | No | Pagination of data to return records earlier than the requested `ts` |
| before | String | No | Pagination of data to return records newer than the requested `ts`. The latest data will be returned when using `before` individually |
| limit | String | No | Number of results per request. The maximum is `300`. The default is `100`. |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
     [
        "1597026383085",
        "3.721",
        "3.743",
        "3.677",
        "3.708",
        "8422410",
        "22698348.04828491",
        "12698348.04828491",
        "0"
    ],
    [
        "1597026383085",
        "3.731",
        "3.799",
        "3.494",
        "3.72",
        "24912403",
        "67632347.24399722",
        "37632347.24399722",
        "1"
    ]
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ts | String | Opening time of the candlestick, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| o | String | Open price |
| h | String | highest price |
| l | String | Lowest price |
| c | String | Close price |
| vol | String | Trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| volCcy | String | Trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| volCcyQuote | String | Trading volume, the value is the quantity in quote currency <br>e.g. The unit is USDT for BTC-USDT and BTC-USDT-SWAP;<br>The unit is USD for BTC-USD-SWAP |
| confirm | String | The state of candlesticks.<br>`0`: K line is uncompleted<br>`1`: K line is completed |

The first candlestick data may be incomplete, and should not be polled repeatedly.

The data returned will be arranged in an array like this: \[ts,o,h,l,c,vol,volCcy,volCcyQuote,confirm\].

For the current cycle of k-line data, when there is no transaction, the opening high and closing low default take the closing price of the previous cycle.

### GET / Candlesticks history

Retrieve history candlestick charts from recent years(It is last 3 months supported for 1s candlestick).

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/history-candles`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/history-candles?instId=BTC-USDT
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve history candlestick charts from recent years
result = marketDataAPI.get_history_candlesticks(
    instId="BTC-USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| after | String | No | Pagination of data to return records earlier than the requested `ts` |
| before | String | No | Pagination of data to return records newer than the requested `ts`. The latest data will be returned when using `before` individually |
| bar | String | No | Bar size, the default is `1m`<br>e.g. \[1s/1m/3m/5m/15m/30m/1H/2H/4H\] <br>UTC+8 opening price k-line: \[6H/12H/1D/2D/3D/1W/1M/3M\]<br>UTC+0 opening price k-line: \[6Hutc/12Hutc/1Dutc/2Dutc/3Dutc/1Wutc/1Mutc/3Mutc\] |
| limit | String | No | Number of results per request. The maximum is `300`. The default is `100`. |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
     [
        "1597026383085",
        "3.721",
        "3.743",
        "3.677",
        "3.708",
        "8422410",
        "22698348.04828491",
        "12698348.04828491",
        "1"
    ],
    [
        "1597026383085",
        "3.731",
        "3.799",
        "3.494",
        "3.72",
        "24912403",
        "67632347.24399722",
        "37632347.24399722",
        "1"
    ]
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ts | String | Opening time of the candlestick, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| o | String | Open price |
| h | String | Highest price |
| l | String | Lowest price |
| c | String | Close price |
| vol | String | Trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| volCcy | String | Trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| volCcyQuote | String | Trading volume, the value is the quantity in quote currency<br>e.g. The unit is USDT for BTC-USDT and BTC-USDT-SWAP;<br>The unit is USD for BTC-USD-SWAP |
| confirm | String | The state of candlesticks<br>`0`: K line is uncompleted<br>`1`: K line is completed |

The data returned will be arranged in an array like this: \[ts,o,h,l,c,vol,volCcy,volCcyQuote,confirm\]

1s candle is not supported by OPTION, but it is supported by other business lines (SPOT, MARGIN, FUTURES and SWAP)

### GET / Trades

Retrieve the recent transactions of an instrument.

#### Rate Limit: 100 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/trades`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/trades?instId=BTC-USDT
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve the recent transactions of an instrument
result = marketDataAPI.get_trades(
    instId="BTC-USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| limit | String | No | Number of results per request. The maximum is `500`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "instId": "BTC-USDT",
            "side": "sell",
            "sz": "0.00001",
            "source": "0",
            "px": "29963.2",
            "tradeId": "242720720",
            "ts": "1654161646974"
        },
        {
            "instId": "BTC-USDT",
            "side": "sell",
            "sz": "0.00001",
            "source": "0",
            "px": "29964.1",
            "tradeId": "242720719",
            "ts": "1654161641568"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID |
| tradeId | String | Trade ID |
| px | String | Trade price |
| sz | String | Trade quantity <br>For spot trading, the unit is base currency<br>For `FUTURES`/`SWAP`/`OPTION`, the unit is contract. |
| side | String | Trade side of taker <br>`buy`<br>`sell` |
| source | String | Order source<br>`0`: normal order<br>`1`: Enhanced Liquidity Program order |
| ts | String | Trade time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |

Up to 500 most recent historical public transaction data can be retrieved.

### GET / Trades history

Retrieve the recent transactions of an instrument from the last 3 months with pagination.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/history-trades`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/history-trades?instId=BTC-USDT
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve the recent transactions of an instrument from the last 3 months with pagination
result = marketDataAPI.get_history_trades(
    instId="BTC-USD-SWAP"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |
| type | String | No | Pagination Type <br>`1`: tradeId `2`: timestamp<br>The default is `1` |
| after | String | No | Pagination of data to return records earlier than the requested tradeId or ts. |
| before | String | No | Pagination of data to return records newer than the requested tradeId. <br>Do not support timestamp for pagination. The latest data will be returned when using `before` individually |
| limit | String | No | Number of results per request. The maximum and default both are `100` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "instId": "BTC-USDT",
            "side": "sell",
            "sz": "0.00001",
            "source": "0",
            "px": "29963.2",
            "tradeId": "242720720",
            "ts": "1654161646974"
        },
        {
            "instId": "BTC-USDT",
            "side": "sell",
            "sz": "0.00001",
            "source": "0",
            "px": "29964.1",
            "tradeId": "242720719",
            "ts": "1654161641568"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID |
| tradeId | String | Trade ID |
| px | String | Trade price |
| sz | String | Trade quantity <br>For spot trading, the unit is base currency<br>For `FUTURES`/`SWAP`/`OPTION`, the unit is contract. |
| side | String | Trade side of taker <br>`buy`<br>`sell` |
| source | String | Order source<br>`0`: normal order<br>`1`: Enhanced Liquidity Program order |
| ts | String | Trade time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |

### GET / Option trades by instrument family

Retrieve the recent transactions of an instrument under same instFamily. The maximum is 100.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/option/instrument-family-trades`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/option/instrument-family-trades?instFamily=BTC-USD
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instFamily | String | Yes | Instrument family, e.g. BTC-USD<br>Applicable to `OPTION` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "vol24h": "103381",
            "tradeInfo": [
                {
                    "instId": "BTC-USD-221111-17750-C",
                    "side": "sell",
                    "sz": "1",
                    "px": "0.0075",
                    "tradeId": "20",
                    "ts": "1668090715058"
                },
                {
                    "instId": "BTC-USD-221111-17750-C",
                    "side": "sell",
                    "sz": "91",
                    "px": "0.01",
                    "tradeId": "19",
                    "ts": "1668090421062"
                }
            ],
            "optType": "C"
        },
        {
            "vol24h": "144499",
            "tradeInfo": [
                {
                    "instId": "BTC-USD-230127-10000-P",
                    "side": "sell",
                    "sz": "82",
                    "px": "0.019",
                    "tradeId": "23",
                    "ts": "1668090967057"
                },
                {
                    "instId": "BTC-USD-221111-16250-P",
                    "side": "sell",
                    "sz": "102",
                    "px": "0.0045",
                    "tradeId": "24",
                    "ts": "1668090885050"
                }
            ],
            "optType": "P"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| vol24h | String | 24h trading volume, with a unit of contract. |
| optType | String | Option type, C: Call P: Put |
| tradeInfo | Array of objects | The list trade data |
| \> instId | String | The Instrument ID |
| \> tradeId | String | Trade ID |
| \> px | String | Trade price |
| \> sz | String | Trade quantity. The unit is contract. |
| \> side | String | Trade side<br>`buy`<br>`sell` |
| \> ts | String | Trade time, Unix timestamp format in milliseconds, e.g. 1597026383085. |

### GET / Option trades

The maximum is 100.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/public/option-trades`

> Request Example

```
Copy to Clipboard
GET /api/v5/public/option-trades?instFamily=BTC-USD
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Conditional | Instrument ID, e.g. BTC-USD-221230-4000-C, Either `instId` or `instFamily` is required. If both are passed, `instId` will be used. |
| instFamily | String | Conditional | Instrument family, e.g. BTC-USD |
| optType | String | No | Option type, `C`: Call `P`: put |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "fillVol": "0.24415013671875",
            "fwdPx": "16676.907614127158",
            "idxPx": "16667",
            "instFamily": "BTC-USD",
            "instId": "BTC-USD-221230-16600-P",
            "markPx": "0.006308943261227884",
            "optType": "P",
            "px": "0.005",
            "side": "sell",
            "sz": "30",
            "tradeId": "65",
            "ts": "1672225112048"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| instId | String | Instrument ID |
| instFamily | String | Instrument family |
| tradeId | String | Trade ID |
| px | String | Trade price |
| sz | String | Trade quantity. The unit is contract. |
| side | String | Trade side <br>`buy`<br>`sell` |
| optType | String | Option type, C: Call P: Put |
| fillVol | String | Implied volatility while trading (Correspond to trade price) |
| fwdPx | String | Forward price while trading |
| idxPx | String | Index price while trading |
| markPx | String | Mark price while trading |
| ts | String | Trade time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |

### GET / 24H total volume

The 24-hour trading volume is calculated on a rolling basis.

#### Rate Limit: 2 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/platform-24-volume`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/platform-24-volume
```

```
Copy to Clipboard
import okx.MarketData as MarketData

flag = "0"  # Production trading:0 , demo trading:1

marketDataAPI =  MarketData.MarketAPI(flag=flag)

# Retrieve 24 total volume
result = marketDataAPI.get_volume()
print(result)
```

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "msg":"",
    "data":[
     {
         "volCny": "230900886396766",
         "volUsd": "34462818865189",
         "ts": "1657856040389"
     }
  ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| volUsd | String | 24-hour total trading volume from the order book trading in "USD" |
| volCny | String | 24-hour total trading volume from the order book trading in "CNY" |
| ts | String | Data return time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / Call auction details

Retrieve call auction details.

#### Rate Limit: 20 requests per 2 seconds

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/market/call-auction-details`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/call-auction-details?instId=ONDO-USDC
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instId | String | Yes | Instrument ID, e.g. `BTC-USDT` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "instId": "ONDO-USDC",
            "unmatchedSz": "9988764",
            "eqPx": "0.6",
            "matchedSz": "44978",
            "state": "continuous_trading",
            "auctionEndTime": "1726542000000",
            "ts": "1726542000007"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| instId | String | Instrument ID |
| eqPx | String | Equilibrium price |
| matchedSz | String | Matched size for both buy and sell<br>The unit is in base currency |
| unmatchedSz | String | Unmatched size |
| auctionEndTime | String | Call auction end time. Unix timestamp in milliseconds. |
| state | String | Trading state of the symbol<br>`call_auction`<br>`continuous_trading` |
| ts | String | Data generation time. Unix timestamp in millieseconds. |

During call auction, users can get the updates of equilibrium price, matched size, unmatched size, and auction end time. The data will be updated around once a second. The endpoint returns the actual open price, matched size, and unmatched size when the call auction ends.

For symbols that never go through call auction, the endpoint will also return results but with state always as \`continuous\_trading\` and other fields as 0 or empty.

### WS / Tickers channel

Retrieve the last traded price, bid price, ask price and 24-hour trading volume of instruments. Best ask price may be lower than the best bid price during the pre-open period.

The fastest rate is 1 update/100ms. There will be no update if the event is not triggered. The events which can trigger update: trade, the change on best ask/bid.

#### URL Path

/ws/v5/public

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "tickers",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard
import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/public")
    await ws.start()
    args = [{
        "channel": "tickers",
        "instId": "BTC-USDT"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`tickers` |
| \> instId | String | Yes | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "tickers",
    "instId": "BTC-USDT"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"tickers\", \"instId\" : \"LTC-USD-200327\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instId | String | Yes | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
  "arg": {
    "channel": "tickers",
    "instId": "BTC-USDT"
  },
  "data": [
    {
      "instType": "SPOT",
      "instId": "BTC-USDT",
      "last": "9999.99",
      "lastSz": "0.1",
      "askPx": "9999.99",
      "askSz": "11",
      "bidPx": "8888.88",
      "bidSz": "5",
      "open24h": "9000",
      "high24h": "10000",
      "low24h": "8888.88",
      "volCcy24h": "2222",
      "vol24h": "2222",
      "sodUtc0": "2222",
      "sodUtc8": "2222",
      "ts": "1597026383085"
    }
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instType | String | Instrument type |
| \> instId | String | Instrument ID |
| \> last | String | Last traded price |
| \> lastSz | String | Last traded size. 0 represents there is no trading volume |
| \> askPx | String | Best ask price |
| \> askSz | String | Best ask size |
| \> bidPx | String | Best bid price |
| \> bidSz | String | Best bid size |
| \> open24h | String | Open price in the past 24 hours |
| \> high24h | String | Highest price in the past 24 hours |
| \> low24h | String | Lowest price in the past 24 hours |
| \> volCcy24h | String | 24h trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| \> vol24h | String | 24h trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| \> sodUtc0 | String | Open price in the UTC 0 |
| \> sodUtc8 | String | Open price in the UTC 8 |
| \> ts | String | Ticker data generation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### WS / Candlesticks channel

Retrieve the candlesticks data of an instrument. the push frequency is the fastest interval 1 second push the data.

#### URL Path

/ws/v5/business

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "candle1D",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/business")
    await ws.start()
    args = [
        {
          "channel": "candle1D",
          "instId": "BTC-USDT"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name <br>`candle3M`<br>`candle1M`<br>`candle1W`<br>`candle1D`<br>`candle2D`<br>`candle3D`<br>`candle5D`<br>`candle12H`<br>`candle6H`<br>`candle4H`<br>`candle2H`<br>`candle1H`<br>`candle30m`<br>`candle15m`<br>`candle5m`<br>`candle3m`<br>`candle1m`<br>`candle1s`<br>`candle3Mutc`<br>`candle1Mutc`<br>`candle1Wutc`<br>`candle1Dutc`<br>`candle2Dutc`<br>`candle3Dutc`<br>`candle5Dutc`<br>`candle12Hutc`<br>`candle6Hutc` |
| \> instId | String | Yes | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"candle1D\", \"instId\" : \"BTC-USD-191227\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | yes | channel name |
| \> instId | String | Yes | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
  "arg": {
    "channel": "candle1D",
    "instId": "BTC-USDT"
  },
  "data": [
    [
      "1597026383085",
      "8533.02",
      "8553.74",
      "8527.17",
      "8548.26",
      "45247",
      "529.5858061",
      "5529.5858061",
      "0"
    ]
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| data | Array of Arrays | Subscribed data |
| \> ts | String | Opening time of the candlestick, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> o | String | Open price |
| \> h | String | highest price |
| \> l | String | Lowest price |
| \> c | String | Close price |
| \> vol | String | Trading volume, with a unit of `contract`. <br>If it is a `derivatives` contract, the value is the number of contracts. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in base currency. |
| \> volCcy | String | Trading volume, with a unit of `currency`. <br>If it is a `derivatives` contract, the value is the number of base currency. <br>If it is `SPOT`/`MARGIN`, the value is the quantity in quote currency. |
| \> volCcyQuote | String | Trading volume, the value is the quantity in quote currency <br>e.g. The unit is `USDT` for `BTC-USDT` and `BTC-USDT-SWAP`<br>The unit is `USD` for `BTC-USD-SWAP` |
| \> confirm | String | The state of candlesticks<br>`0`: K line is uncompleted<br>`1`: K line is completed |

### WS / Trades channel

Retrieve the recent trades data. Data will be pushed whenever there is a trade. Every update may aggregate multiple trades.

The message is sent only once per taker order, filled price, source. The count field is used to represent the number of aggregated matches.

#### URL Path

/ws/v5/public

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "trades",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/public")
    await ws.start()
    args = [
        {
          "channel": "trades",
          "instId": "BTC-USDT"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`trades` |
| \> instId | String | Yes | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
      "channel": "trades",
      "instId": "BTC-USDT"
  },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"trades\", \"instId\" : \"BTC-USD-191227\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instId | String | Yes | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
  "arg": {
    "channel": "trades",
    "instId": "BTC-USDT"
  },
  "data": [
    {
      "instId": "BTC-USDT",
      "tradeId": "130639474",
      "px": "42219.9",
      "sz": "0.12060306",
      "side": "buy",
      "ts": "1630048897897",
      "count": "3",
      "source": "0",
      "seqId": 1234
    }
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instId | String | Instrument ID, e.g. `BTC-USDT` |
| \> tradeId | String | The last trade ID in the trades aggregation |
| \> px | String | Trade price |
| \> sz | String | Trade quantity <br>For spot trading, the unit is base currency<br>For `FUTURES`/`SWAP`/`OPTION`, the unit is contract. |
| \> side | String | Trade side of taker<br>`buy`<br>`sell` |
| \> ts | String | Filled time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| \> count | String | The count of trades aggregated |
| \> source | String | Order source<br>`0`: normal orders<br>`1`: Enhanced Liquidity Program order |
| \> seqId | Integer | Sequence ID of the current message. |

Aggregation function description:

1\. The system will send only one message per taker order, filled price, source. The \`count\` field will be used to represent the number of aggregated matches.

2\. The \`tradeId\` field in the message becomes the last trade ID in the aggregation.

3\. When the \`count\` = 1, it means the taker order matches only one maker order with the specific price.

4\. When the \`count\` > 1, it means the taker order matches multiple maker orders with the same price. For example, if \`tradeId\` = 123 and \`count\` = 3, it means the message aggregates the trades of \`tradeId\` = 123, 122, and 121. Maker side has filled multiple orders.

5\. Users can use this information to compare with data from the \`trades-all\` channel.

6\. Order book and the aggregated trades data are still published sequentially.

The seqId may be the same for different trade updates that occur at the same time.

### WS / All trades channel

Retrieve the recent trades data. Data will be pushed whenever there is a trade. Every update contain only one trade.

#### URL Path

/ws/v5/business

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "trades-all",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/business")
    await ws.start()
    args = [
        {
          "channel": "trades-all",
          "instId": "BTC-USDT"
        }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`trades-all` |
| \> instId | String | Yes | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
      "channel": "trades-all",
      "instId": "BTC-USDT"
    },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"trades-all\", \"instId\" : \"BTC-USD-191227\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instId | String | Yes | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
  "arg": {
    "channel": "trades-all",
    "instId": "BTC-USDT"
  },
  "data": [
    {
      "instId": "BTC-USDT",
      "tradeId": "130639474",
      "px": "42219.9",
      "sz": "0.12060306",
      "side": "buy",
      "source": "0",
      "ts": "1630048897897"
    }
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instId | String | Instrument ID, e.g. `BTC-USDT` |
| \> tradeId | String | Trade ID |
| \> px | String | Trade price |
| \> sz | String | Trade quantity <br>For spot trading, the unit is base currency<br>For `FUTURES`/`SWAP`/`OPTION`, the unit is contract. |
| \> side | String | Trade direction<br>`buy`<br>`sell` |
| \> source | String | Order source<br>`0`: normal<br>`1`: Enhanced Liquidity Program order |
| \> ts | String | Filled time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### WS / Order book channel

Retrieve order book data. Best ask price may be lower than the best bid price during the pre-open period.

Use `books` for 400 depth levels, `books5` for 5 depth levels, `bbo-tbt` tick-by-tick 1 depth level, `books50-l2-tbt` tick-by-tick 50 depth levels, and `books-l2-tbt` for tick-by-tick 400 depth levels.

- `books`: 400 depth levels will be pushed in the initial full snapshot. Incremental data will be pushed every 100 ms for the changes in the order book during that period of time.

- `books-elp`: only push ELP orders. 400 depth levels will be pushed in the initial full snapshot. Incremental data will be pushed every 100 ms for the changes in the order book during that period of time.

- `books5`: 5 depth levels snapshot will be pushed in the initial push. Snapshot data will be pushed every 100 ms when there are changes in the 5 depth levels snapshot.

- `bbo-tbt`: 1 depth level snapshot will be pushed in the initial push. Snapshot data will be pushed every 10 ms when there are changes in the 1 depth level snapshot.

- `books-l2-tbt`: 400 depth levels will be pushed in the initial full snapshot. Incremental data will be pushed every 10 ms for the changes in the order book during that period of time.

- `books50-l2-tbt`: 50 depth levels will be pushed in the initial full snapshot. Incremental data will be pushed every 10 ms for the changes in the order book during that period of time.
- The push sequence for order book channels within the same connection and trading symbols is fixed as: bbo-tbt -> books-l2-tbt -> books50-l2-tbt -> books -> books-elp -> books5.
- Users can not simultaneously subscribe to `books-l2-tbt` and `books50-l2-tbt/books` channels for the same trading symbol.

  - For more details, please refer to the changelog [2024-07-17](https://www.okx.com/docs-v5/log_en/#2024-07-17)

Only API users who are VIP6 and above in trading fee tier are allowed to subscribe to "books-l2-tbt" 400 depth channels

Only API users who are VIP5 and above in trading fee tier are allowed to subscribe to "books50-l2-tbt" 50 depth channels

Identity verification refers to [Login](https://www.okx.com/docs-v5/en/#overview-websocket-login)

#### URL Path

/ws/v5/public

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
  "op": "subscribe",
  "args": [
    {
      "channel": "books",
      "instId": "BTC-USDT"
    }
  ]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/public")
    await ws.start()
    args = [
      {
        "channel": "books",
        "instId": "BTC-USDT"
      }
    ]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`books`<br>`books5`<br>`bbo-tbt`<br>`books50-l2-tbt`<br>`books-l2-tbt` |
| \> instId | String | Yes | Instrument ID |

> Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "connId": "a4d3ae55"
}
```

> Failure example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"books\", \"instId\" : \"BTC-USD-191227\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name |
| \> instId | String | Yes | Instrument ID |
| msg | String | No | Error message |
| code | String | No | Error code |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example: Full Snapshot

```
Copy to Clipboard
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "snapshot",
  "data": [
    {
      "asks": [
        ["8476.98", "415", "0", "13"],
        ["8477", "7", "0", "2"],
        ["8477.34", "85", "0", "1"],
        ["8477.56", "1", "0", "1"],
        ["8505.84", "8", "0", "1"],
        ["8506.37", "85", "0", "1"],
        ["8506.49", "2", "0", "1"],
        ["8506.96", "100", "0", "2"]
      ],
      "bids": [
        ["8476.97", "256", "0", "12"],
        ["8475.55", "101", "0", "1"],
        ["8475.54", "100", "0", "1"],
        ["8475.3", "1", "0", "1"],
        ["8447.32", "6", "0", "1"],
        ["8447.02", "246", "0", "1"],
        ["8446.83", "24", "0", "1"],
        ["8446", "95", "0", "3"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": -1,
      "seqId": 123456
    }
  ]
}
```

> Push Data Example: Incremental Data

```
Copy to Clipboard
{
  "arg": {
    "channel": "books",
    "instId": "BTC-USDT"
  },
  "action": "update",
  "data": [
    {
      "asks": [
        ["8476.98", "415", "0", "13"],
        ["8477", "7", "0", "2"],
        ["8477.34", "85", "0", "1"],
        ["8477.56", "1", "0", "1"],
        ["8505.84", "8", "0", "1"],
        ["8506.37", "85", "0", "1"],
        ["8506.49", "2", "0", "1"],
        ["8506.96", "100", "0", "2"]
      ],
      "bids": [
        ["8476.97", "256", "0", "12"],
        ["8475.55", "101", "0", "1"],
        ["8475.54", "100", "0", "1"],
        ["8475.3", "1", "0", "1"],
        ["8447.32", "6", "0", "1"],
        ["8447.02", "246", "0", "1"],
        ["8446.83", "24", "0", "1"],
        ["8446", "95", "0", "3"]
      ],
      "ts": "1597026383085",
      "checksum": -855196043,
      "prevSeqId": 123456,
      "seqId": 123457
    }
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| action | String | Push data action, incremental data or full snapshot. <br>`snapshot`: full <br>`update`: incremental |
| data | Array of objects | Subscribed data |
| \> asks | Array of Arrays | Order book on sell side |
| \> bids | Array of Arrays | Order book on buy side |
| \> ts | String | Order book generation time, Unix timestamp format in milliseconds, e.g. `1597026383085`<br>Exception: For the `bbo-tbt` channel, `ts` is the timestamp when the book is generated by matching engine. |
| \> checksum | Integer | Checksum, implementation details below |
| \> prevSeqId | Integer | Sequence ID of the last sent message. Only applicable to `books`, `books-l2-tbt`, `books50-l2-tbt` |
| \> seqId | Integer | Sequence ID of the current message, implementation details below |

An example of the array of asks and bids values: \["411.8", "10", "0", "4"\]

\- "411.8" is the depth price

\- "10" is the quantity at the price (number of contracts for derivatives, quantity in base currency for Spot and Spot Margin)

\- "0" is part of a deprecated feature and it is always "0"

\- "4" is the number of orders at the price.

If you need to subscribe to many 50 or 400 depth level channels, it is recommended to subscribe through multiple websocket connections, with each of less than 30 channels.

The order book data will be updated around once a second during the call auction.

\`books/books5/bbo-tbt/books-l2-tbt/books50-l2-tbt\` don't return ELP orders

\`books-elp\` only return ELP orders, including both valid and invalid parts (invalid parts means ELP buy orders with a price higher than best bid of non-ELP orders; or ELP sell orders with a price lower than best ask of non-ELP orders). Users should distinguish valid and invalid parts using the best bid/ask price of non-ELP orders.

#### Sequence ID

`seqId` is the sequence ID of the market data published. The set of sequence ID received by users is the same if users are connecting to the same channel through multiple websocket connections. Each `instId` has an unique set of sequence ID. Users can use `prevSeqId` and `seqId` to build the message sequencing for incremental order book updates. Generally the value of seqId is larger than prevSeqId. The `prevSeqId` in the new message matches with `seqId` of the previous message. The smallest possible sequence ID value is 0, except in snapshot messages where the prevSeqId is always -1.

Exceptions:

1\. If there are no updates to the depth for an extended period(Around 60 seconds), for the channel that always updates snapshot data, OKX will send the latest snapshot, for the channel that has incremental data, OKX will send a message with `'asks': [], 'bids': []` to inform users that the connection is still active. `seqId` is the same as the last sent message and `prevSeqId` equals to `seqId`.
2\. The sequence number may be reset due to maintenance, and in this case, users will receive an incremental message with `seqId` smaller than `prevSeqId`. However, subsequent messages will follow the regular sequencing rule.

##### Example

1. Snapshot message: prevSeqId = -1, seqId = 10
2. Incremental message 1 (normal update): prevSeqId = 10, seqId = 15
3. Incremental message 2 (no update): prevSeqId = 15, seqId = 15
4. Incremental message 3 (sequence reset): prevSeqId = 15, seqId = 3
5. Incremental message 4 (normal update): prevSeqId = 3, seqId = 5

#### Checksum

This mechanism can assist users in checking the accuracy of depth data.

##### Merging incremental data into full data

After subscribing to the incremental load push (such as `books` 400 levels) of Order Book Channel, users first receive the initial full load of market depth. After the incremental load is subsequently received, update the local full load.

1. If there is the same price, compare the size. If the size is 0, delete this depth data. If the size changes, replace the original data.
2. If there is no same price, sort by price (bid in descending order, ask in ascending order), and insert the depth information into the full load.

##### Calculate Checksum

Use the first 25 bids and asks in the full load to form a string (where a colon connects the price and size in an ask or a bid), and then calculate the CRC32 value (32-bit signed integer).

> Calculate Checksum

```
Copy to Clipboard
1. More than 25 levels of bid and ask
A full load of market depth (only 2 levels of data are shown here, while 25 levels of data should actually be intercepted):
```

```
Copy to Clipboard
{
    "bids": [
        ["3366.1", "7", "0", "3"],
        ["3366", "6", "3", "4"]
    ],
    "asks": [
        ["3366.8", "9", "10", "3"],
        ["3368", "8", "3", "4"]
    ]
}
```

```
Copy to Clipboard
Check string:
"3366.1:7:3366.8:9:3366:6:3368:8"

2. Less than 25 levels of bid or ask
A full load of market depth:
```

```
Copy to Clipboard
{
    "bids": [
        ["3366.1", "7", "0", "3"]
    ],
    "asks": [
        ["3366.8", "9", "10", "3"],
        ["3368", "8", "3", "4"],
        ["3372", "8", "3", "4"]
    ]
}
```

```
Copy to Clipboard
Check string:
"3366.1:7:3366.8:9:3368:8:3372:8"
```

1. When the bid and ask depth data exceeds 25 levels, each of them will intercept 25 levels of data, and the string to be checked is queued in a way that the bid and ask depth data are alternately arranged.

Such as: `bid[price:size]`:`ask[price:size]`:`bid[price:size]`:`ask[price:size]`...
2. When the bid or ask depth data is less than 25 levels, the missing depth data will be ignored.

Such as: `bid[price:size]`:`ask[price:size]`:`asks[price:size]`:`asks[price:size]`...

> Push Data Example of bbo-tbt channel

```
Copy to Clipboard
{
  "arg": {
    "channel": "bbo-tbt",
    "instId": "BCH-USDT-SWAP"
  },
  "data": [
    {
      "asks": [
        [
          "111.06","55154","0","2"
        ]
      ],
      "bids": [
        [
          "111.05","57745","0","2"
        ]
      ],
      "ts": "1670324386802",
      "seqId": 363996337
    }
  ]
}
```

> Push Data Example of books5 channel

```
Copy to Clipboard
{
  "arg": {
    "channel": "books5",
    "instId": "BCH-USDT-SWAP"
  },
  "data": [
    {
      "asks": [
        ["111.06","55154","0","2"],
        ["111.07","53276","0","2"],
        ["111.08","72435","0","2"],
        ["111.09","70312","0","2"],
        ["111.1","67272","0","2"]],
      "bids": [
        ["111.05","57745","0","2"],
        ["111.04","57109","0","2"],
        ["111.03","69563","0","2"],
        ["111.02","71248","0","2"],
        ["111.01","65090","0","2"]],
      "instId": "BCH-USDT-SWAP",
      "ts": "1670324386802",
      "seqId": 363996337
    }
  ]
}
```

### WS / Option trades channel

Retrieve the recent trades data. Data will be pushed whenever there is a trade. Every update contain only one trade.

#### URL Path

/ws/v5/public

> Request Example

```
Copy to Clipboard
{
  "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "option-trades",
        "instType": "OPTION",
        "instFamily": "BTC-USD"
    }]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/public")
    await ws.start()
    args = [{
        "channel": "option-trades",
        "instType": "OPTION",
        "instFamily": "BTC-USD"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | `subscribe``unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name<br>`option-trades` |
| \> instType | String | Yes | Instrument type, `OPTION` |
| \> instId | String | Conditional | Instrument ID, e.g. BTC-USD-221230-4000-C, Either `instId` or `instFamily` is required. If both are passed, `instId` will be used. |
| \> instFamily | String | Conditional | Instrument family, e.g. BTC-USD |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "option-trades",
        "instType": "OPTION",
        "instFamily": "BTC-USD"
    },
    "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"option-trades\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | `subscribe``unsubscribe``error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name<br>`status` |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
    "arg": {
        "channel": "option-trades",
        "instType": "OPTION",
        "instFamily": "BTC-USD"
    },
    "data": [
        {
            "fillVol": "0.5066007836914062",
            "fwdPx": "16469.69928595038",
            "idxPx": "16537.2",
            "instFamily": "BTC-USD",
            "instId": "BTC-USD-230224-18000-C",
            "markPx": "0.04690107010619562",
            "optType": "C",
            "px": "0.045",
            "side": "sell",
            "sz": "2",
            "tradeId": "38",
            "ts": "1672286551080"
        }
    ]
}
```

#### Push data parameters

| Parameter | Type | Description |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| data | Array of objects | Subscribed data |
| \> instId | String | Instrument ID |
| \> instFamily | String | Instrument family |
| \> tradeId | String | Trade ID |
| \> px | String | Trade price |
| \> sz | String | Trade quantity. The unit is contract. |
| \> side | String | Trade side <br>`buy`<br>`sell` |
| \> optType | String | Option type, C: Call P: Put |
| \> fillVol | String | Implied volatility while trading (Correspond to trade price) |
| \> fwdPx | String | Forward price while trading |
| \> idxPx | String | Index price while trading |
| \> markPx | String | Mark price while trading |
| \> ts | String | Trade time, Unix timestamp format in milliseconds, e.g. `1597026383085`. |

The first data you receive after subscribing may be cached from the previous trade, so please ignore it.

### WS / Call auction details channel

Retrieve call auction details.

#### URL Path

/ws/v5/public

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [{
        "channel": "call-auction-details",
        "instId": "ONDO-USDC"
    }]
}
```

```
Copy to Clipboard

import asyncio

from okx.websocket.WsPublicAsync import WsPublicAsync

def callbackFunc(message):
    print(message)

async def main():
    ws = WsPublicAsync(url="wss://wspap.okx.com:8443/ws/v5/public")
    await ws.start()
    args = [{
        "channel": "call-auction-details",
        "instId": "ONDO-USDC"
    }]

    await ws.subscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

    await ws.unsubscribe(args, callback=callbackFunc)
    await asyncio.sleep(10)

asyncio.run(main())
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message <br>Provided by client. It will be returned in response message for identifying the corresponding request. <br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| op | String | Yes | Operation<br>`subscribe`<br>`unsubscribe` |
| args | Array of objects | Yes | List of subscribed channels |
| \> channel | String | Yes | Channel name <br>`call-auction-details` |
| \> instId | String | Yes | Instrument ID |

> Successful Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "subscribe",
  "arg": {
      "channel": "call-auction-details",
      "instId": "ONDO-USDC"
    },
  "connId": "a4d3ae55"
}
```

> Failure Response Example

```
Copy to Clipboard
{
  "id": "1512",
  "event": "error",
  "code": "60012",
  "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"call-auction-details\", \"instId\" : \"BTC-USD-191227\"}]}",
  "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Event<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | yes | channel name |
| \> instId | String | Yes | Instrument ID |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
  "arg": {
    "channel": "call-auction-details",
    "instId": "ONDO-USDC"
  },
  "data": [
        {
            "instId": "ONDO-USDC",
            "unmatchedSz": "9988764",
            "eqPx": "0.6",
            "matchedSz": "44978",
            "state": "continuous_trading",
            "auctionEndTime": "1726542000000",
            "ts": "1726542000007"
        }
  ]
}
```

#### Push data parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> instId | String | Instrument ID |
| data | Array of objects | Subscribed data |
| \> instId | String | Instrument ID |
| \> eqPx | String | Equilibrium price |
| \> matchedSz | String | Matched size for both buy and sell<br>The unit is in base currency |
| \> unmatchedSz | String | Unmatched size |
| \> auctionEndTime | String | Call auction end time. Unix timestamp in milliseconds. |
| \> state | String | Trading state of the symbol<br>`call_auction`<br>`continuous_trading` |
| \> ts | String | Data generation time. Unix timestamp in millieseconds. |

During call auction, users can get the updates of equilibrium price, matched size, unmatched size, and auction end time. The data will be updated around once a second. When call auction ends, this channel will push the last message, returning the actual open price, matched size, and unmatched size, with trading state as \`continuous\_trading\`.

## SBE Market Data

### Overview

OKX supports Simple Binary Encoding (SBE) for data returned from the following WebSocket channels:

- [WS / Trades channel](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-ws-trades-channel): `trades`
- [WS / Order book channel](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-ws-order-book-channel): `bbo-tbt` and `books-l2-tbt`

### XML Schema

The SBE XML schema is now available for download:

[Download XML Schema](https://www.okx.com/docs-v5/log_en/xml/okx_sbe_1_0.xml)

### General Information

- The `bbo-tbt` channel is **available to users of any trading fee tier** but requires login. The `trades` and `books-l2-tbt` channels are restricted to users with a trading fee tier of **VIP6** or above in the live trading environment, and **VIP1** or above in the demo trading environment.

- SBE channels will use a new WebSocket URL.

Live trading: `wss://ws.okx.com:8443/ws/v5/public-sbe`

Demo trading: `wss://wspap.okx.com:8443/ws/v5/public-sbe`

- Both JSON and SBE format data will be available on the same connection, distinguishable by WebSocket frame type. opcode `1` indicates JSON, while opcode `2` indicates SBE.

- Prices and quantities will be encoded as exponential decimals, using a signed integer mantissa and signed exponent. For example, a mantissa of 123456 and a exponent of -4 represents 12.3456 (actual value = mantissa \* 10 ^ exponent).

- The SBE protocol will use `instIdCode` , an integer will be provided by [Get instruments](https://www.okx.com/docs-v5/en/#public-data-rest-api-get-instruments) to represent trading instruments. Users must map `instIdCode` to `instId`, noting that `instIdCode` will change if a trading symbol is relisted, but `instIdCode` will remains unchanged when `instId` is renamed.

- `tsUs` and `outTime` come from different servers, so their relative order is not guaranteed.
- `tsUs` is in microseconds format but only accurate to milliseconds. The microseconds-format timestamp is obtained by appending 000 to the millisecond timestamp. For example, if the millisecond timestamp is 1726233600001, the related microseconds-format timestamp (tsUs) will be 1726233600001000.

### Integration Information

- To log in, transmit your API key and signature in the WebSocket connection header.

  - The connection requests must contain the following headers:

    - `OK-ACCESS-KEY` The API key as a String.

    - `OK-ACCESS-SIGN` The Base64-encoded signature.

    - `OK-ACCESS-TIMESTAMP` Unix Epoch time in seconds. e.g : `1751335333`

    - `OK-ACCESS-PASSPHRASE` The passphrase you specified when creating the API key.
  - The `OK-ACCESS-SIGN` header is generated as follows:

    - Create a pre-hash string of timestamp + method + requestPath

    - Prepare the SecretKey.

    - Sign the pre-hash string with the SecretKey using the HMAC SHA256.

    - Encode the signature in the Base64 format. Example: sign=CryptoJS.enc.Base64.stringify(CryptoJS.HmacSHA256(timestamp + 'GET' + '/users/self/verify', SecretKey))

    - Example of timestamp: const timestamp = '' + Date.now() / 1,000, e.g. 1704876947

    - Method: always 'GET'.

    - RequestPath : always '/users/self/verify'
  - The response HTTP code of `101` indicates the successful login.

  - The response HTTP code `401`, along with an error message in the response body, indicates a failed login. The error message will be in JSON fromat.

```
Copy to Clipboard
Login error message example
{
    "msg": "Invalid apiKey",
    "code": "60005",
    "connId":"24a2aea3"
}
```

- Subscription request must be sent in JSON format. The response will also be in JSON format, and can be identified by opcode `1`.

  - The protocol is similar to existing JSON-formatted subscription requests/response.

  - The difference is that `instIdCode` should be used instead of instId.

```
Copy to Clipboard
Subscription request example
{
    "op": "subscribe",
    "args": [
        {
            "channel": "trades",
            "instIdCode": 211874
        }
    ]
}

Subscription response example
{
    "event": "subscribe",
    "arg": {
        "channel": "trades",
        "instIdCode": 211874
    },
    "connId": "accb8e21"
}
```

- The notice event is supported in JSON format:

```
Copy to Clipboard
Notice event example
{
    "event": "notice",
    "code": "64008",
    "msg": "The connection will soon be closed for a service upgrade. Please reconnect.",
    "connId": "a4d3ae55"
}
```

- The WebSocket server will send a ping frame with opcode `9` every 20 seconds after receiving a pong frame.

  - If the WebSocket server does not receive a pong frame back from the connection within 60 secondes, the connection will be disconnected.

  - Upon receiving a ping, you must respond with a pong frame using opcode `10`, along with a copy of the ping‘s payload as soon as possible (payload will be a random numerical text like 11446744073709551615).

  - Unsolicited pong frames are permitted but will not prevent disconnection. It is advisable that the payload for these pong frames be empty.
- For `trades``bbo-tbt` and `books-l2-tbt` channels, data will be returned in binary format and can be identified by opcode `2`, distinguishable by template ID. Key differences compared to existing JSON-formatted connections include:

  - For the `trades` channel, the `seqId` will be returned.

  - For the `bbo-tbt` channel, it usually provides real-time data, but under system overload, data loss can occur, varying by different connection.

  - For the `books-l2-tbt`:

    - When prices and quantities decimals change, a exponent update (template ID: 1002) will occur with previous sequence ID and sequence ID, identifiable by template ID. This must be processed to maintain the sequence ID consistence.

    - The checksum will no longer be included.

    - There will be no initial order book snapshot after subscription. Instead, OKX will provide a REST API endpoint that returns SBE binary data for the initial 400 levels snapshot. This endpoint will buffer requests and return data only when a new snapshot is generated, approximately every 500 ms.
- The relationship between the channel and event is not one-to-one. The books-l2-tbt contains two types of events. The mapping is outlined below.

| Channel | XML Template ID and message name |
| --- | --- |
| bbo-tbt | 1000: BboTbtChannelEvent |
| books-l2-tbt | 1001: BooksL2TbtChannelEvent<br>1002: BooksL2TbtExponentUpdateEvent |
| books-l2-tbt-elp <br>(It is not enabled) | 1003: BooksL2TbtElpChannelEvent<br>1004: BooksL2TbtElpExponentUpdateEvent |
| trades | 1005: TradesChannelEvent |

- How to manage a local order book correctly

1. Open a SBE WebSocket connection and subscribe to `books-l2-tbt`.
2. Buffer the events received from the stream. Record the prevSeqId of the first event you received.

     Note: For template ID 1002, the event is an exponent update, containing only exponent update information without ask or bid data. For template ID 1001, the data includes both asks and bids.
3. Get a depth snapshot from `/books-sbe`, e.g. `https://www.okx.com/api/v5/market/books-sbe?instIdCode=12345&source=0`
4. If the `seqId` from the snapshot is strictly less than the `prevSeqId` from step 2, go back to step 3.
5. In the buffered events, discard any event where stream `seqId` is <= snapshot `seqId` of the snapshot.
6. The first buffered event should satisfy the condition: stream `prevSeqId` <= snapshot `seqId` < stream `seqId`.
7. Set your local order book to the snapshot. Its sequence ID is snapshot `seqId`.
8. Apply the update procedure below to all buffered events, and then to all subsequent events received.

     - If the template ID is 1002 (BooksL2TbtExponentUpdateEvent), only update the exponents without bid and ask data. If the template ID is 1001 (BooksL2TbtChannelEvent), follow the process outlined below.
     - For each price level in bids and asks, set the new quantity in the order book:

       - If the price level does not exist in the order book, insert it with new quantity.
       - If the quantity is zero, remove the price level from the order book.
     - Set the order book sequence ID to the latest sequence ID (`seqId`) in the processed event.

       Note: Not all snapshot `seqId` will appear in the `books-l2-tbt` channels.
- Sequence ID

`seqId` is the sequence ID of the market data published. The set of sequence ID received by users is the same if users are connecting to the same channel through multiple websocket connections. Each `instIdCode` has an unique set of sequence ID. Users can use `prevSeqId` and `seqId` to build the message sequencing for incremental order book updates. Generally the value of seqId is larger than prevSeqId. The `prevSeqId` in the new message matches with `seqId` of the previous message. The smallest possible sequence ID value is 0, except in snapshot messages where the prevSeqId is always -1.

Exceptions:

1\. If there are no updates to the depth for an extended period(Around 60 seconds), for the channel that always updates snapshot data, OKX will send the latest snapshot, for the channel that has incremental data, OKX will send a message with numInGroup: 0 to inform users that the connection is still active. `seqId` is the same as the last sent message and `prevSeqId` equals to `seqId`.

2\. The sequence number may be reset due to maintenance, and in this case, users will receive an incremental message with `seqId` smaller than `prevSeqId`. However, subsequent messages will follow the regular sequencing rule.

##### Example

1. Incremental message 1 (normal update): prevSeqId = 10, seqId = 15
2. Incremental message 2 (no update): prevSeqId = 15, seqId = 15
3. Incremental message 3 (sequence reset): prevSeqId = 15, seqId = 3
4. Incremental message 4 (normal update): prevSeqId = 3, seqId = 5

### SBE Order book

It is a public endpoint, returning SBE binary data for the initial 400 levels snapshot. This endpoint will buffer requests and return data only when a new snapshot is generated, approximately every 500 ms.

Note: If the request fails, the error message will be provided in JSON format.

For the HTTP request header, it doesn't need to be set to `application/sbe`; however, the response header will be `Content-Type`: `application/sbe` if the request is successful, and `Content-Type`: `application/json` if the request fails.

#### Rate Limit: 10 requests per 10 seconds

#### Rate limit rule: IP + instIdCode

#### HTTP Request

`GET /api/v5/market/books-sbe`

> Request Example

```
Copy to Clipboard
GET /api/v5/market/books-sbe?instIdCode=12345&source=0
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| instIdCode | Integer | Yes | Instruement ID code |
| source | Integer | Yes | The source of order book.<br>`0`: normal |

> Response Example

```
Copy to Clipboard
Error message example

Response header:
Content-Type: application/json

Response body:
{
    "code": "51000",
    "msg": "Parameter instIdCode error",
    "data": []
}
```

#### Response Parameters

Please refer to the `SnapshotDepthResponseEvent` with ID `1006` in the XML schema.

### New error code

| Error Code | HTTP Status | Error Message |
| --- | --- | --- |
| 60034 | 401 | Only users who are {0} and above in trading fee tier are allowed to use this URL. |

### Upgrade

- In general, only compatible upgrades are made, such as adding a new field. In these cases, the XML schema ID remains unchanged, while the schema version is incremented.
- If a breaking change is needed, a new XML schema with a new schema ID will be released at least 1–2 months in advance. Before the end of the transition period, you’ll need to support both the old and new schemas, based on their schema ID and version.