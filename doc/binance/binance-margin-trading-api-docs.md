# Binance Margin Trading API Documentation

Source: https://developers.binance.com/docs/

---

Margin Trading

# Change Log

## 2026-01-21

- Following the announcement from 2025-11-10, the following endpoints/methods will no longer be available starting from **2026-02-20 07:00 UTC**:

**REST API**
  - POST /sapi/v1/userDataStream
  - PUT /sapi/v1/userDataStream
  - DELETE /sapi/v1/userDataStream
  - POST /sapi/v1/userDataStream/isolated
  - PUT /sapi/v1/userDataStream/isolated
  - DELETE /sapi/v1/userDataStream/isolated

## 2026-01-16

- Update on endpoints restrictions
  - `GET /sapi/v1/margin/capital-flow`：Addition of query time range restriction. This restriction will take effect from approximately **2026-02-02 07:00 UTC**.
  - When both `startTime` and `endTime` are specified, the time span cannot exceed 7 days, otherwise, the endpoint is expected to return an error:

    ```text
    {
      "code": -4047,
      "msg": "Time interval must be within 0-7 days"
    }
    ```

  - Please split your query into multiple requests if the time range required exceeds 7 days.

## 2025-12-26

**Time-sensitive Notice**

- **The following change to REST API will occur at approximately 2026-01-15 07:00 UTC:**

When calling endpoints that require signatures, percent-encode payloads before computing signatures. Requests that do not follow this order will be rejected with [`-1022 INVALID_SIGNATURE`](https://developers.binance.com/docs/margin_trading/error-code#-1022-invalid_signature). Please review and update your signing logic accordingly.

**REST API**

- Updated documentation for REST API regarding [Signed Endpoints examples for placing an order](https://developers.binance.com/docs/margin_trading/general-info#signed-endpoint-examples-for-post-apiv3order).

## 2025-11-10

- **All documentation related with listenKey for use on wss:// [stream.binance.com](http://stream.binance.com/) removed from the Margin Trading SAPI portal on 2025-11-10. The features below will remain available until a future retirement announcement is made..**
  - POST /sapi/v1/userDataStream
  - PUT /sapi/v1/userDataStream
  - DELETE /sapi/v1/userDataStream
  - POST /sapi/v1/userDataStream/isolated
  - PUT /sapi/v1/userDataStream/isolated
  - DELETE /sapi/v1/userDataStream/isolated
- **Users are recommended to move to the new listen token subscription method below, which offers slightly better performance (lower latency):**
  - POST /sapi/v1/userListenToken : Create a new user data stream and return a listenToken.
  - method userDataStream.subscribe.listenToken : Subscribe to the user data stream using listenToken.
- The [User Data Stream](https://developers.binance.com/docs/margin_trading/trade-data-stream) documentation will remain as reference for the payloads users can receive:

  - [Account Update](https://developers.binance.com/docs/margin_trading/trade-data-stream/Event-Account-Update): outboundAccountPosition is sent any time an account balance has changed and contains the assets that were possibly changed by the event that generated the balance change.
  - [Balance Update](https://developers.binance.com/docs/margin_trading/trade-data-stream/Event-Balance-Update): Balance Update occurs when transfer of funds between accounts.
  - [Order Update](https://developers.binance.com/docs/margin_trading/trade-data-stream/Event-Order-Update): Orders are updated with the executionReport event.

## 2025-10-06

- **Receiving user data stream on wss://stream.binance.com:9443 using a listenKey is now deprecated.**
  - This feature will be removed from our system at a later date.
  - The related documents will also be removed together with the feature removal.
  - Users are recommended to move to the new listen token subscription method below.
- New user data stream subscription with [Websocket API](https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/general-api-information) released:

  - POST /sapi/v1/userListenToken : Create a new user data stream and return a listenToken.
  - method userDataStream.subscribe.listenToken : Subscribe to the user data stream using listenToken.

## 2025-09-16

- One endpoint updated :
  - POST `/sapi/v1/margin/apiKey`: Supported produects scope and error code description added into the API description.

## 2025-06-17

- Best Practice section uploaded on the Margin Trading

## 2025-06-16

- List schedule endpoint is now released for Margin:
  - GET `/sapi/v1/margin/list-schedule`: Get the upcoming tokens or symbols listing schedule for Cross Margin and Isolated Margin.

## 2024-09-19

- Binance Margin offers low-latency trading through a special key, available exclusively to users with VIP level 4 or higher.

If you are VIP level 3 or below, please contact your VIP manager for eligibility criterias.

The endpoints below are available :
  - POST /sapi/v1/margin/apiKey
  - DELETE /sapi/v1/margin/apiKey
  - PUT /sapi/v1/margin/apiKey/ip
  - GET /sapi/v1/margin/apiKey
  - GET /sapi/v1/margin/api-key-list
- How to use the special API key
1. Use SAPI to create a special pair of "margin trade only" key/secret via the endpoint above.

2. For cross margin account, do not send the symbol parameter.

3. For isolated margin account of a specific symbol, please send the symbol as the isolated margin pair.

4. Use the key/secret responded in step 1 to do the margin trading and listenKey creating via SPOT REST api ( [https://api.binance.com/api/v3/](https://api.binance.com/api/v3/)) endpoints.

## 2024-09-13

- One-Triggers-the-Other (OTO) orders and One-Triggers-a-One-Cancels-The-Other (OTOCO) orders are now enabled for Margin:
  - POST `/sapi/v1/margin/order/oto`: Post a new OTO order for margin account
  - POST `/sapi/v1/margin/order/otoco`: Post a new OTOCO order for margin account
- New parameters added into response body to replace the parameter of 'transferEnabled' in the endpoint of GET `/sapi/v1/margin/account`:

  - transferInEnabled
  - transferOutEnabled

* * *

## 2024-01-09

- According to the [announcement](https://www.binance.com/en/support/announcement/updates-on-binance-margin-sapi-endpoints-2024-03-31-a1868c686ce7448da8c3061a82a87b0c), Binance Margin will remove the following SAPI interfaces at 4:00 on March 31, 2024 (UTC). Please switch to the corresponding alternative interfaces in time:
  - `POST /sapi/v1/margin/transfer` will be removed, please replace with `POST /sapi/v1/asset/transfer` universal transfer
  - `POST /sapi/v1/margin/isolated/transfer` will be removed, please replace with `POST /sapi/v1/asset/transfer` universal transfer
  - `POST /sapi/v1/margin/loan` will be removed, please replace with the new `POST /sapi/v1/margin/borrow-repay` borrowing and repayment interface
  - `POST /sapi/v1/margin/repay` will be removed, please replace with the new `POST /sapi/v1/margin/borrow-repay` borrowing and repayment interface
  - `GET /sapi/v1/margin/isolated/transfer` will be removed, please replace it with `GET /sapi/v1/margin/transfer` to get total margin transfer history
  - `GET /sapi/v1/margin/asset` will be removed, please replace with `GET /sapi/v1/margin/allAssets`
  - `GET /sapi/v1/margin/pair` will be removed, please replace with `GET /sapi/v1/margin/allPairs`
  - `GET /sapi/v1/margin/isolated/pair` will be removed, please replace with `GET /sapi/v1/margin/isolated/allPairs`
  - `GET /sapi/v1/margin/loan` will be removed, please replace with `GET /sapi/v1/margin/borrow-repay`
  - `GET /sapi/v1/margin/repay` will be removed, please replace with `GET /sapi/v1/margin/borrow-repay`
  - `GET /sapi/v1/margin/dribblet` will be removed, please replace with `GET /sapi/v1/asset/dribblet`
  - `GET /sapi/v1/margin/dust` will be removed, please replace with `POST /sapi/v1/asset/dust-btc`
  - `POST /sapi/v1/margin/dust` will be removed, please replace with `POST /sapi/v1/asset/dust`
- New Endpoints for Margin:
  - `POST /sapi/v1/margin/borrow-repay`: Margin account borrow/repay
  - `GET /sapi/v1/margin/borrow-repay`: Query borrow/repay records in Margin account
- Update Endpoints fpr Margin:
  - `GET /sapi/v1/margin/transfer`: add parameter `isolatedSymbol`, add response body
  - `GET /sapi/v1/margin/allAssets`: add parameter `asset`, add response body
  - `GET /sapi/v1/margin/allPairs`: add parameter `symbol`
  - `GET /sapi/v1/margin/isolated/allPairs`: add parameter `symbol`

* * *

## 2023-12-22

- New Websocket for Margin Trading:
  - New Base url `wss://margin-stream.binance.com` for two events: Liability update event and Margin call event

* * *

## 2023-11-21

- Update endpoints for Margin
  - `POST /sapi/v1/margin/order`：New enumerate value `AUTO_BORROW_REPAY` for the field of `sideEffectType`
  - `POST /sapi/v1/margin/order/oco`：New enumerate value `AUTO_BORROW_REPAY` for the field of `sideEffectType`
  - `GET /sapi/v1/margin/available-inventory`：New response field of `updateTime` which indicates the acquisition time of lending inventory.

* * *

## 2023-11-17

- New endpoint for Margin to support cross margin Pro mode [FAQ](https://www.binance.com/en/support/faq/introduction-to-binance-cross-margin-pro-0b5441a1c1ff431bb2e135dfa8e6ffba):

  - `GET /sapi/v1/margin/leverageBracket`: query Liability coin leverage bracket in cross margin Pro mode
- Update endpoints for Margin:
  - `POST /sapi/v1/margin/max-leverage`: field `maxLeverage` adds value 10 for Cross Margin Pro
  - `GET /sapi/v1/margin/account`: add new response field `accountType`, `MARGIN_2` for Cross Margin Pro

* * *

## 2023-10-16

- New endpoint for Margin:
  - `GET /sapi/v1/margin/available-inventory`: Query margin available inventory
  - `POST /sapi/v1/margin/manual-liquidation`: Margin manual liquidation

* * *

## 2023-08-31

- New endpoint for Margin:
  - `/sapi/v1/margin/capital-flow`: Get cross or isolated margin capital flow

* * *

## 2023-07-07

- New endpoints for Margin:
  - `POST /sapi/v1/margin/max-leverage`: Adjust cross margin max leverage

* * *

## 2023-06-29

- New endpoints for Margin:
  - `GET /sapi/v1/margin/dust`: Get Assets That Can Be Converted Into BNB
  - `POST /sapi/v1/margin/dust`: Convert dust assets to BNB.

* * *

## 2023-06-22

- Update endpoints for Margin:
  - `POST /sapi/v1/margin/order`: add fields `autoRepayAtCancel` and `selfTradePreventionMode`
  - `POST /sapi/v1/margin/order/oco`: add field `selfTradePreventionMode`

* * *

## 2023-06-20

- Update endpoints for Margin:
  - `GET /sapi/v1/margin/delist-schedule`: get tokens or symbols delist schedule for cross margin and isolated margin

* * *

## 2023-02-27

- New endpoints for Margin:
  - `/sapi/v1/margin/next-hourly-interest-rate`: Get user the next hourly estimate interest

* * *

## 2023-02-02

- New endpoints for Margin:
  - `GET /sapi/v1/margin/exchange-small-liability`: Query the coins which can be small liability exchange
  - `POST /sapi/v1/margin/exchange-small-liability`: Cross Margin Small Liability Exchange
  - `GET /sapi/v1/margin/exchange-small-liability-history`: Get Small liability Exchange History

* * *

## 2022-09-16

- New endpoint for Margin：
  - `GET /sapi/v1/margin/tradeCoeff`: Get personal margin level information

* * *

## 2022-07-01

- New endpoint for Margin:
  - `GET /sapi/v1/margin/dribblet` to query the historical information of user's margin account small-value asset conversion BNB.
- Update endpoint for Margin:
  - `GET /sapi/v1/margin/repay`: Add response field rawAsset.

* * *

## 2022-05-26

- Update info for the following margin account endpoints: The max interval between `startTime` and `endTime` is 30 days.:

  - `GET /sapi/v1/margin/transfer`
  - `GET /sapi/v1/margin/loan`
  - `GET /sapi/v1/margin/repay`
  - `GET /sapi/v1/margin/isolated/transfer`
  - `GET /sapi/v1/margin/interestHistory`

* * *

## 2022-04-26

- `GET /sapi/v1/margin/rateLimit/order`added

  - The endpoint will display the user's current margin order count usage for all intervals.

* * *

## 2021-12-30

- Update endpoint for Margin：
  - Removed out `limit` from`GET /sapi/v1/margin/interestRateHistory`; The max interval between startTime and endTime is 30 days.

* * *

## 2021-12-03

- New endpoints for Margin:
  - `GET  /sapi/v1/margin/crossMarginData` to get cross margin fee data collection
  - `GET  /sapi/v1/margin/isolatedMarginData` to get isolated margin fee data collection
  - `GET  /sapi/v1/margin/isolatedMarginTier` to get isolated margin tier data collection

* * *

## 2021-10-14

- Update the time range of the response data for the following margin account endpoints, `startTime` and `endTime` time span will not exceed 30 days, without time parameter sent the system will return the last 7 days of data by default, while the `archived` parameter is `true`, the system will return the last 7 days of data 6 months ago by default:

  - `GET /sapi/v1/margin/transfer`
  - `GET /sapi/v1/margin/loan`
  - `GET /sapi/v1/margin/repay`
  - `GET /sapi/v1/margin/isolated/transfer`
  - `GET /sapi/v1/margin/interestHistory`

* * *

## 2021-09-08

- Add endpoints for enabled isolated margin account limit:
  - `DELETE /sapi/v1/margin/isolated/account` to disable isolated margin account for a specific symbol
  - `POST /sapi/v1/margin/isolated/account` to enable isolated margin account for a specific symbol
  - `GET /sapi/v1/margin/isolated/accountLimit` to query enabled isolated margin account limit
- New field "enabled" in response of `GET /sapi/v1/margin/isolated/account` to check if the isolated margin account is enabled

* * *

## 2021-08-23

- New endpoints for Margin Account OCO:
  - `POST /sapi/v1/margin/order/oco`
  - `DELETE /sapi/v1/margin/orderList`
  - `GET /sapi/v1/margin/orderList`
  - `GET /sapi/v1/margin/allOrderList`
  - `GET /sapi/v1/margin/openOrderList`

Same usage as spot account OCO

* * *

## 2021-04-28

On **May 15, 2021 08:00 UTC** the SAPI Create Margin Account endpoint will be discontinued:

- `POST /sapi/v1/margin/isolated/create`

Isolated Margin account creation and trade preparation can be completed directly through Isolated Margin funds transfer `POST /sapi/v1/margin/isolated/transfer`

* * *

## 2021-03-05

- New endpoints for Margin:
  - `GET /sapi/v1/margin/interestRateHistory` to support margin interest rate history query

* * *

## 2021-01-15

- New endpoint `DELETE /sapi/v1/margin/openOrders` for Margin Trade

  - This will allow a user to cancel all open orders on a single symbol for margin account.
  - This endpoint will cancel all open orders including OCO orders for margin account.

* * *

## 2020-12-01

- Update Margin Trade Endpoint:
  - `POST /sapi/v1/margin/order` new parameter `quoteOrderQty` allow a user to specify the total `quoteOrderQty` spent or received in the `MARKET` order.

* * *

## 2020-11-16

- Updated endpoints for Margin, new parameter `archived` to query data from 6 months ago:

  - `GET /sapi/v1/margin/loan`
  - `GET /sapi/v1/margin/repay`
  - `GET /sapi/v1/margin/interestHistory`

* * *

## 2020-11-10

- New endpoint to toggle BNB Burn:
  - `POST /sapi/v1/bnbBurn` to toggle BNB Burn on spot trade and margin interest.
  - `GET /sapi/v1/bnbBurn` to get BNB Burn status.

* * *

## 2020-09-30

- Update endpoints for Margin Account:
  - `GET /sapi/v1/margin/maxBorrowable` new field `borrowLimit` in response for account borrow limit.

* * *

## 2020-08-26

- New parameter `symbols` added in the endpoint `GET /sapi/v1/margin/isolated/account`.

* * *

## 2020-07-28

ISOLATED MARGIN

- New parameters "isIsolated" and "symbol" added for isolated margin in the following endpoints:
  - `POST /sapi/v1/margin/loan`
  - `POST /sapi/v1/margin/repay`
- New parameter "isIsolated" and new response field "isIsolated" added for isolated margin in the following endpoints:
  - `POST /sapi/v1/margin/order`
  - `DELETE /sapi/v1/margin/order`
  - `GET /sapi/v1/margin/order`
  - `GET /sapi/v1/margin/openOrders`
  - `GET /sapi/v1/margin/allOrders`
  - `GET /sapi/v1/margin/myTrades`
- New parameter "isolatedSymbol" and new response field "isolatedSymbol" added for isolated margin in the following endpoints:
  - `GET /sapi/v1/margin/loan`
  - `GET /sapi/v1/margin/repay`
  - `GET /sapi/v1/margin/interestHistory`
- New parameter "isolatedSymbol" and new response field "isIsolated" added for isolated margin in the following endpoint `GET /sapi/v1/margin/forceLiquidationRec`

- New parameter "isolatedSymbol" added for isolated margin in the following endpoints:
  - `GET /sapi/v1/margin/maxBorrowable`
  - `GET /sapi/v1/margin/maxTransferable`
- New endpoints for isolated margin:
  - `POST /sapi/v1/margin/isolated/create`
  - `POST /sapi/v1/margin/isolated/transfer`
  - `GET /sapi/v1/margin/isolated/transfer`
  - `GET /sapi/v1/margin/isolated/account`
  - `GET /sapi/v1/margin/isolated/pair`
  - `GET /sapi/v1/margin/isolated/allPairs`
- New endpoints for listenKey management of isolated margin account:
  - `POST /sapi/v1/userDataStream/isolated`
  - `PUT /sapi/v1/userDataStream/isolated`
  - `DELETE /sapi/v1/userDataStream/isolated`

* * *

## 2019-12-18

- New endpoint to get daily snapshot of account:

`GET /sapi/v1/accountSnapshot`

* * *

## 2019-11-30

- Added parameter `sideEffectType` in `POST  /sapi/v1/margin/order (HMAC SHA256)` with enums:
  - `NO_SIDE_EFFECT` for normal trade order;
  - `MARGIN_BUY` for margin trade order;
  - `AUTO_REPAY` for making auto repayment after order filled.
- New field `marginBuyBorrowAmount` and `marginBuyBorrowAsset` in `FULL` response to `POST  /sapi/v1/margin/order (HMAC SHA256)`

* * *

## 2019-11-28

- New SAPI endpont to disable fast withdraw switch:

`POST /sapi/v1/account/disableFastWithdrawSwitch (HMAC SHA256)`
- New SAPI endpont to enable fast withdraw switch:

`POST /sapi/v1/account/enableFastWithdrawSwitch (HMAC SHA256)`

Copyright © 2026 Binance.

---

Margin Trading

# Adjust cross margin max leverage (USER\_DATA)

## API Description

Adjust cross margin max leverage

## HTTP Request

POST `/sapi/v1/margin/max-leverage`

## Request Weight(UID)

**3000**

## Request Limit

1 times/min per IP

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| maxLeverage | Integer | YES | Can only adjust 3 , 5 or 10，Example: maxLeverage = 5 or 3 for Cross Margin Classic; maxLeverage=10 for Cross Margin Pro 10x leverage or 20x if compliance allows. |

- The margin level need higher than the initial risk ratio of adjusted leverage, the initial risk ratio of 3x is 1.5 , the initial risk ratio of 5x is 1.25; The detail conditions on how to switch between Cross Margin Classic and Cross Margin Pro can refer to [the FAQ](https://www.binance.com/en/support/faq/how-to-activate-the-cross-margin-pro-mode-on-binance-e27786da05e743a694b8c625b3bc475d).

## Response Example

```javascript
{
    "success": true
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Adjust cross margin max leverage (USER\_DATA)

## API Description

Adjust cross margin max leverage

## HTTP Request

POST `/sapi/v1/margin/max-leverage`

## Request Weight(UID)

**3000**

## Request Limit

1 times/min per IP

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| maxLeverage | Integer | YES | Can only adjust 3 , 5 or 10，Example: maxLeverage = 5 or 3 for Cross Margin Classic; maxLeverage=10 for Cross Margin Pro 10x leverage or 20x if compliance allows. |

- The margin level need higher than the initial risk ratio of adjusted leverage, the initial risk ratio of 3x is 1.5 , the initial risk ratio of 5x is 1.25; The detail conditions on how to switch between Cross Margin Classic and Cross Margin Pro can refer to [the FAQ](https://www.binance.com/en/support/faq/how-to-activate-the-cross-margin-pro-mode-on-binance-e27786da05e743a694b8c625b3bc475d).

## Response Example

```javascript
{
    "success": true
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Disable Isolated Margin Account (TRADE)

## API Description

Disable isolated margin account for a specific symbol. Each trading pair can only be deactivated once every 24
hours.

## HTTP Request

DELETE `/sapi/v1/margin/isolated/account`

## Request Weight

**300(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "success": true,
  "symbol": "BTCUSDT"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Enable Isolated Margin Account (TRADE)

## API Description

Enable isolated margin account for a specific symbol(Only supports activation of previously disabled accounts).

## HTTP Request

POST `/sapi/v1/margin/isolated/account`

## Request Weight

**300(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "success": true,
  "symbol": "BTCUSDT"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get BNB Burn Status (USER\_DATA)

## API Description

Get BNB Burn Status

## HTTP Request

GET `/sapi/v1/bnbBurn`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
   "spotBNBBurn":true,
   "interestBNBBurn": false
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Summary of Margin account (USER\_DATA)

## API Description

Get personal margin level information

## HTTP Request

GET `/sapi/v1/margin/tradeCoeff`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "normalBar": "1.5",
  "marginCallBar": "1.3",
  "forceLiquidationBar": "1.1"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Cross Isolated Margin Capital Flow (USER\_DATA)

## API Description

Query Cross Isolated Margin Capital Flow

## HTTP Request

GET `/sapi/v1/margin/capital-flow`

## Request Weight

**100(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| symbol | STRING | NO | Mandatory for Isolated data |
| type | STRING | NO |  |
| startTime | LONG | NO | Only supports querying data from the past 90 days. |
| endTime | LONG | NO |  |
| fromId | LONG | NO | If `fromId` is set, data with `id` greater than `fromId` will be returned. Otherwise, the latest data will be returned. |
| limit | LONG | NO | Limit on the number of data records returned per request. Default: 500; Maximum: 1000. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Only supports querying the data of the last 90 days
- The time between startTime and endTime cannot be longer than 7 days.
- If fromId is set, the data with id > fromId will be returned. Otherwise the latest data will be returned
- To query isolated data, Symbol needs to be entered.
- Supported types:
  - TRANSFER("Transfer")
  - BORROW("Borrow")
  - REPAY("Repay")
  - BUY\_INCOME("Buy-Trading Income")
  - BUY\_EXPENSE("Buy-Trading Expense")
  - SELL\_INCOME("Sell-Trading Income")
  - SELL\_EXPENSE("Sell-Trading Expense")
  - TRADING\_COMMISSION("Trading Commission")
  - BUY\_LIQUIDATION("Buy by Liquidation")
  - SELL\_LIQUIDATION("Sell by Liquidation")
  - REPAY\_LIQUIDATION("Repay by Liquidation")
  - OTHER\_LIQUIDATION("Other Liquidation")
  - LIQUIDATION\_FEE("Liquidation Fee")
  - SMALL\_BALANCE\_CONVERT("Small Balance Convert")
  - COMMISSION\_RETURN("Commission Return")
  - SMALL\_CONVERT("Small Convert")

## Response Example

```javascript
[
  {
    "id": 123456,
    "tranId": 123123,
    "timestamp": 1691116657000,
    "asset": "USDT",
    "symbol": "BTCUSDT",
    "type": "BORROW",
    "amount": "101"
  },
  {
    "id": 123457,
    "tranId": 123124,
    "timestamp": 1691116658000,
    "asset": "BTC",
    "symbol": "BTCUSDT",
    "type": "REPAY",
    "amount": "10"
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Cross Margin Account Details (USER\_DATA)

## API Description

Query Cross Margin Account Details

## HTTP Request

GET `/sapi/v1/margin/account`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
      "created" : true, // True means margin account created , false means margin account not created.
      "borrowEnabled": true,
      "marginLevel": "11.64405625",
      "collateralMarginLevel" : "3.2",
      "totalAssetOfBtc": "6.82728457",
      "totalLiabilityOfBtc": "0.58633215",
      "totalNetAssetOfBtc": "6.24095242",
      "TotalCollateralValueInUSDT": "5.82728457",
      "totalOpenOrderLossInUSDT": "582.728457",
      "tradeEnabled": true,
      "transferInEnabled": true,
      "transferOutEnabled": true,
      "accountType": "MARGIN_1",  // // MARGIN_1 for Cross Margin Classic, MARGIN_2 for Cross Margin Pro
      "userAssets": [
          {
              "asset": "BTC",
              "borrowed": "0.00000000",
              "free": "0.00499500",
              "interest": "0.00000000",
              "locked": "0.00000000",
              "netAsset": "0.00499500"
          },
          {
              "asset": "BNB",
              "borrowed": "201.66666672",
              "free": "2346.50000000",
              "interest": "0.00000000",
              "locked": "0.00000000",
              "netAsset": "2144.83333328"
          },
          {
              "asset": "ETH",
              "borrowed": "0.00000000",
              "free": "0.00000000",
              "interest": "0.00000000",
              "locked": "0.00000000",
              "netAsset": "0.00000000"
          },
          {
              "asset": "USDT",
              "borrowed": "0.00000000",
              "free": "0.00000000",
              "interest": "0.00000000",
              "locked": "0.00000000",
              "netAsset": "0.00000000"
          }
      ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Cross Margin Fee Data (USER\_DATA)

## API Description

Get cross margin fee data collection with any vip level or user's current specific data as [https://www.binance.com/en/margin-fee](https://www.binance.com/en/margin-fee)

## HTTP Request

GET `/sapi/v1/margin/crossMarginData`

## Request Weight

**1 when coin is specified;(IP)** **5 when the coin parameter is omitted(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| vipLevel | INT | NO | User's current specific margin data will be returned if vipLevel is omitted |
| coin | STRING | NO |  |
| recvWindow | LONG | NO | No more than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "vipLevel": 0,
        "coin": "BTC",
        "transferIn": true,
        "borrowable": true,
        "dailyInterest": "0.00026125",
        "yearlyInterest": "0.0953",
        "borrowLimit": "180",
        "marginablePairs": [
            "BNBBTC",
            "TRXBTC",
            "ETHBTC",
            "BTCUSDT"
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Enabled Isolated Margin Account Limit (USER\_DATA)

## API Description

Query enabled isolated margin account limit.

## HTTP Request

GET `/sapi/v1/margin/isolated/accountLimit`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "enabledAccount": 5,
  "maxAccount": 20
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Isolated Margin Account Info (USER\_DATA)

## API Description

Query Isolated Margin Account Info

## HTTP Request

GET `/sapi/v1/margin/isolated/account`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbols | STRING | NO | Max 5 symbols can be sent; separated by ",". e.g. "BTCUSDT,BNBUSDT,ADAUSDT" |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

- If "symbols" is not sent, all isolated assets will be returned.
- If "symbols" is sent, only the isolated assets of the sent symbols will be returned.

## Response Example

> If "symbols" is not sent

```javascript
{
   "assets":[
      {
        "baseAsset":
        {
          "asset": "BTC",
          "borrowEnabled": true,
          "borrowed": "0.00000000",
          "free": "0.00000000",
          "interest": "0.00000000",
          "locked": "0.00000000",
          "netAsset": "0.00000000",
          "netAssetOfBtc": "0.00000000",
          "repayEnabled": true,
          "totalAsset": "0.00000000"
        },
        "quoteAsset":
        {
          "asset": "USDT",
          "borrowEnabled": true,
          "borrowed": "0.00000000",
          "free": "0.00000000",
          "interest": "0.00000000",
          "locked": "0.00000000",
          "netAsset": "0.00000000",
          "netAssetOfBtc": "0.00000000",
          "repayEnabled": true,
          "totalAsset": "0.00000000"
        },
        "symbol": "BTCUSDT",
        "isolatedCreated": true,
        "enabled": true, // true-enabled, false-disabled
        "marginLevel": "0.00000000",
        "marginLevelStatus": "EXCESSIVE", // "EXCESSIVE", "NORMAL", "MARGIN_CALL", "PRE_LIQUIDATION", "FORCE_LIQUIDATION"
        "marginRatio": "0.00000000",
        "indexPrice": "10000.00000000",
        "liquidatePrice": "1000.00000000",
        "liquidateRate": "1.00000000",
        "tradeEnabled": true
      }
    ],
    "totalAssetOfBtc": "0.00000000",
    "totalLiabilityOfBtc": "0.00000000",
    "totalNetAssetOfBtc": "0.00000000"
}
```

> If "symbols" is sent

```javascript
{
   "assets":[
      {
        "baseAsset":
        {
          "asset": "BTC",
          "borrowEnabled": true,
          "borrowed": "0.00000000",
          "free": "0.00000000",
          "interest": "0.00000000",
          "locked": "0.00000000",
          "netAsset": "0.00000000",
          "netAssetOfBtc": "0.00000000",
          "repayEnabled": true,
          "totalAsset": "0.00000000"
        },
        "quoteAsset":
        {
          "asset": "USDT",
          "borrowEnabled": true,
          "borrowed": "0.00000000",
          "free": "0.00000000",
          "interest": "0.00000000",
          "locked": "0.00000000",
          "netAsset": "0.00000000",
          "netAssetOfBtc": "0.00000000",
          "repayEnabled": true,
          "totalAsset": "0.00000000"
        },
        "symbol": "BTCUSDT",
        "isolatedCreated": true,
        "enabled": true, // true-enabled, false-disabled
        "marginLevel": "0.00000000",
        "marginLevelStatus": "EXCESSIVE", // "EXCESSIVE", "NORMAL", "MARGIN_CALL", "PRE_LIQUIDATION", "FORCE_LIQUIDATION"
        "marginRatio": "0.00000000",
        "indexPrice": "10000.00000000",
        "liquidatePrice": "1000.00000000",
        "liquidateRate": "1.00000000",
        "tradeEnabled": true
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Isolated Margin Fee Data (USER\_DATA)

## API Description

Get isolated margin fee data collection with any vip level or user's current specific data as [https://www.binance.com/en/margin-fee](https://www.binance.com/en/margin-fee)

## HTTP Request

GET `/sapi/v1/margin/isolatedMarginData`

## Request Weight

**1 when a single is specified;(IP)** **10 when the symbol parameter is omitted(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| vipLevel | INT | NO | User's current specific margin data will be returned if vipLevel is omitted |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO | No more than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "vipLevel": 0,
        "symbol": "BTCUSDT",
        "leverage": "10",
        "data": [
            {
                "coin": "BTC",
                "dailyInterest": "0.00026125",
                "borrowLimit": "270"
            },
            {
                "coin": "USDT",
                "dailyInterest": "0.000475",
                "borrowLimit": "2100000"
            }
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Best Practice

### Activating & Enabling Margin Trading via AP

#### Enable Margin on Your Account

Before using the API, please ensure margin trading is enabled on your Binance account. For first time users, you will be required to complete the margin quiz and agree to the margin terms, once completed you will need to transfer supported tokens into the cross margin or isolated margin wallet to activate it. For existing users, you will just need to transfer supported tokens into the cross margin or isolated margin wallet to activate the margin wallet. When creating your API key, check the setting to “Enable Spot & Margin Trading, and Enable Margin Loan, Repay & Transfer”, otherwise margin API calls will be rejected. For your security, please also consider IP whitelisting on your API key.

Please refer to the article [“How to Create API Keys on Binance?”](https://www.binance.com/en/support/faq/detail/360002502072) for more details.

If you are looking for a low-latency connectivity similar to spot trading, VIP 4 and above users are automatically eligible,for further information please refer to [Margin Special Key Api Portal](https://developers.binance.com/docs/margin_trading/trade/Create-Special-Key-of-Low-Latency-Trading)

#### Tips to Avoid Common Mistakes:

- Account activation: Double-check that your margin account is enabled. Otherwise, your API calls will return the following error {"code":-3003, "msg":"Margin account does not exist."}
- Error handling: If you get an error like {"code":-2015, "msg":"Invalid API-key, IP, or permissions for action."}, it usually means either your API key lacks permission. To resolve this, please enable the API key settings via the website.

### Funding the Margin Account

Before trading on margin, you need to fund your margin account by transferring assets into it as collateral. Binance keeps Spot (exchange wallet) and Margin wallets separate. For cross margin, you have to transfer assets to the cross margin account; for isolated margin, you have to transfer to the specific isolated account for the trading pair.

#### Cross Margin Transfer

Users can invoke the following REST API to transfer assets to the cross margin account:

[POST /sapi/v1/asset/transfer](https://developers.binance.com/docs/wallet/asset/user-universal-transfer)

This endpoint uses a parameter type to indicate direction:

- MAIN\_MARGIN: Spot account transfer to Margin (cross) account
- UMFUTURE\_MARGIN: USDⓈ-M Futures account transfer to Margin (cross) account
- CMFUTURE\_MARGIN: COIN-M Futures account transfer to Margin (cross) account
- FUNDING\_MARGIN: Funding account transfer to Margin (cross) account
- OPTION\_MARGIN: Options account transfer to Margin (cross) account

Other required parameters are asset (e.g. "USDT", "BTC"), amount (the quantity to transfer as a string) and timestamp. Please ensure you have that asset available for use in the source account.

- Request Parameter:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | ENUM | YES |  |
| asset | STRING | YES |  |
| amount | DECIMAL | YES |  |
| fromSymbol | STRING | NO |  |
| toSymbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

#### Isolated Margin Transfer

For isolated account, the type of direction is as follows:

- MARGIN\_ISOLATEDMARGIN: Margin(cross) account transfer to Isolated margin account
- ISOLATEDMARGIN\_ISOLATEDMARGIN: Isolated margin account transfer to Isolated margin account

When direction types are ISOLATEDMARGIN\_MARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN, you should specify the source and destination explicitly. Additional parameters, fromSymbol, is required.

On the other hand, when direction types are MARGIN\_ISOLATEDMARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN, toSymbol is required.

You will receive a successful response once the transfer is completed successfully.

{ "tranId": 1234567890 }

This tranId can be used to query transfer status if needed (though usually, a successful response means the transfer is complete).

With funds now in your margin account, you have collateral to borrow against. Next, we’ll borrow funds to perform a leverage trade.

#### Tips to Avoid Common Mistakes:

- Insufficient collateral margin level: Sometimes, your collateral margin level may be too low to allow a transfer out of your account. You will get an error response {"code":-3020,"msg":"Transfer out amount exceeds max amount."}. To resolve it, you can reduce your outstanding debt or add more assets to meet the required margin level for the transfer.

### Borrowing Funds

One key feature of margin trading is the ability to borrow funds to increase your position size. On Binance, you can borrow different assets as long as you have sufficient collateral in your margin account for your chosen leverage. Borrowing is subject to interest (accrued hourly), and each asset has a maximum borrowable amount based on your collateral value and a chosen leverage.

In Binance, we use the same endpoint to execute borrow and repay

[POST /sapi/v1/margin/borrow-repay](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Margin-Account-Borrow-Repay)

#### Borrow

You must specify the asset you want to borrow and the amount. For isolated margin, you will include an extra parameter to indicate the isolated account. Binance uses a boolean isolated flag and a symbol parameter.

- Request Parameter:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| isIsolated | STRING | YES | TRUE for Isolated Margin, FALSE for Cross Margin, Default FALSE |
| symbol | STRING | YES | Only for Isolated margin |
| amount | STRING | YES |  |
| type | STRING | YES | BORROW or REPAY |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

On success, you’ll get a JSON with a transaction ID for the loan. For example:

{ "tranId": 1234567891 }

This tranId is the identifier of the borrowing transaction (you can use it to query the loan status or history later).

After a successful borrow, the borrowed funds will be credited to the corresponding margin account (increasing your “borrowed” balance for that asset). You are now free to use those funds to trade. Keep in mind interest immediately starts accruing on the loan until it’s repaid.

#### Tips to Avoid Common Mistakes:

- Maximum borrowable: If you get an error {"code": -3006, "msg": "Your borrow amount has exceed maximum borrow amount."}, it means the amount you want to borrow exceeds your limit. Binance enforces an initial margin requirement – borrowing too much will cause you to be below the initial margin requirement and hence the additional borrowings will be rejected. You can check your max borrowable amount via [GET /sapi/v1/margin/maxBorrowable](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Query-Max-Borrow) for a given asset (and optionally symbol for isolated). This can be useful to see how much you could borrow against your available collateral.

- Interest: Each borrowed asset accrues interest, which you can query via [GET /sapi/v1/margin/interestHistory](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Get-Interest-History). Interest is usually deducted from your margin account (adding to the “interest” field for that asset). Ensure you account for interest when repaying (please note that repayment covers interest first, then principal).

- Asset availability: Not all assets may be borrowable at all times. Binance may have limits or temporarily suspend borrowing for certain assets if liquidity is low. If you get an error that an asset is not borrowable, you may need to check [GET /sapi/v1/margin/allAssets](https://developers.binance.com/docs/margin_trading/market-data/Get-All-Margin-Assets) or try later.

- Insufficient available assets: If you get an error {"code": -3045, "msg": The system does not have enough asset now."} This can occur to both manual Margin borrow requests and auto-borrow Margin orders that require actual borrowing. This can be due to:
  - The Margin system's assets available for borrowing are less than the requested borrowing amount.
  - The system's inventory is critically low, leading to the rejection of all borrowing requests, irrespective of the amount.

We recommend monitoring the system status and adjusting your borrowing strategies accordingly.

- Cross margin collateral haircuts: If you are trading in Cross Margin/Cross Margin Pro mode, collateral haircuts are factored into the collateral margin level calculation. Meaning assets have tiered collateral ratios that discount their value for margin calculations. For example, higher asset holdings may be valued at lower collateral percentages across tiers, reducing total collateral value accordingly. Please note that this does not affect Isolated Margin mode and the collateral margin level uses collateral asset value, not normal asset value:
  - For Cross Margin Classic, Collateral margin level will be used to calculate maximum borrowing and transfer-out amount, but will NOT be used to trigger Margin Call and Liquidation, where margin level will still be used.
  - For Cross Margin Pro, Collateral margin level will be used to calculate the maximum borrowing and transfer-out amounts, and will be used to trigger Margin Call and Liquidation as well.

You can find out more in [How to Calculate the Margin Level on Cross Margin Pro?](https://www.binance.com/en/support/faq/detail/12a78d8aa813470f96be283b45f75410)

- Portfolio margin collateral haircuts: The USD value of all assets in the Cross Margin, USDⓈ-M Futures, and COIN-M Futures Wallets will be calculated based on the specified collateral rate (Not the same collateral ratio as Cross Margin Classic/Cross Margin Pro). You can find out the collateral ratio in [Tiered Collateral Ratio for PM Pro](https://www.binance.com/en/futures/trading-rules/perpetual/portfolio-margin/tiered-collateral-ratio)

Upon successfully borrowing, the token will be transferred to your margin wallet.Next, we will use the borrowed tokens to place a margin trade (buy or sell).

### Placing a Margin Trade

With your collateral (including funds you may have borrowed), you can create orders (limit, market, etc.) on your margin account. Binance provides a dedicated endpoint for margin orders:

[POST /sapi/v1/margin/order](https://developers.binance.com/docs/margin_trading/trade/Margin-Account-New-Order)

- Request Parameter:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| side | ENUM | YES | BUY SELL |
| type | ENUM | YES |  |
| quantity | DECIMAL | NO |  |
| quoteOrderQty | DECIMAL | NO |  |
| price | DECIMAL | NO |  |
| stopPrice | DECIMAL | NO | Used with STOP\_LOSS, STOP\_LOSS\_LIMIT, TAKE\_PROFIT, and TAKE\_PROFIT\_LIMIT orders. |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. |
| icebergQty | DECIMAL | NO | Used with LIMIT, STOP\_LOSS\_LIMIT, and TAKE\_PROFIT\_LIMIT to create an iceberg order. |
| newOrderRespType | ENUM | NO | Set the response JSON. ACK, RESULT, or FULL; MARKET and LIMIT order types default to FULL, all other orders default to ACK. |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY, AUTO\_REPAY,AUTO\_BORROW\_REPAY; default NO\_SIDE\_EFFECT. More info in [FAQ](https://www.binance.com/en/support/faq/detail/f9fc51cda1984bf08b95e0d96c4570bc) |
| timeInForce | ENUM | NO | GTC,IOC,FOK |
| selfTradePreventionMode | ENUM | NO | The allowed enums is dependent on what is configured on the symbol. The possible supported values are EXPIRE\_TAKER, EXPIRE\_MAKER, EXPIRE\_BOTH, NONE |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY or AUTO\_BORROW\_REPAY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

#### Auto-Borrow

If you have manually borrowed funds, you can use those in your margin trade. If you have not borrowed manually, Binance’s margin order endpoint can auto-borrow or auto-repay for you, depending on a parameter called sideEffectType.

The sideEffectType parameter lets you automate borrowing or repaying as part of the order placement.

- NO\_SIDE\_EFFECT: No automatic borrow or repay. This is the default setting. You should have sufficient balance in your margin account (either from deposits or prior manual borrows) to execute the order. If you do not, the order placement will fail due to insufficient balance. Users essentially self-manage their borrowing and repayment themselves.
- MARGIN\_BUY: Automatic borrowing when needed for a BUY order. If you place a BUY order and do not have enough quote assets to deduct, Binance will automatically borrow the necessary amount of the quote asset up to your leverage limit. For a SELL order, if using MARGIN\_BUY mode, it would auto-borrow the asset to sell (though typically one would use MARGIN\_BUY for buys; see AUTO\_BORROW\_REPAY for a comprehensive mode). The borrow occurs only if needed and only when the order is executed (not at order creation). Essentially this equates to “borrow asset + place order” in one step.
- AUTO\_REPAY: Automatic repayment after the order executes. If your order results in you obtaining the asset that you owe (borrowed), the system will immediately use the proceeds to repay the loan. For example, if you had borrowed BTC and you sold some BTC (thus obtaining the quoted assets), setting AUTO\_REPAY would use the quote assets you have to repay the BTC loan (converting the quote asset to BTC as needed to repay). As an example: if you borrowed USDT to buy BTC, when you sell BTC (for USDT) with AUTO\_REPAY, it will use the USDT you receive to repay the USDT loan. Important: auto-repay will repay as much as possible of that asset’s liability (interest first) and will only work if the trade yields the same asset that was borrowed. This is useful to close out a margin position in one step (“place order + upon fill, repay debt”).
- AUTO\_BORROW\_REPAY: Combines both of the above – automatic borrow and automatic repay in one step. This mode will borrow as needed to execute the order, and then after execution, immediately try to repay with whatever assets were obtained. It’s effectively a margin flip: e.g., if you have nothing and submit a BUY with AUTO\_BORROW\_REPAY, it will borrow the quote asset, buy the base asset, then if that base asset was what you needed to repay (in case of short position) it would repay, etc. In practice, this mode is a bit complex and is not allowed for certain multi-leg orders (like OCO/OTOCO).

For beginners, you may wish to start with NO\_SIDE\_EFFECT (ensuring you manually borrowed what you need) or use MARGIN\_BUY if you want to skip the manual borrow step for a buy. Always verify that these automated steps did what you expected (please check your balances and loans after the order).

#### Tips to Avoid Common Mistakes:

- Order Rules: Margin orders obey the same trading rules as spot (lot size, price step, minimum notional value, etc.). You can check the exchange information( [GET /api/v3/exchangeInfo](https://developers.binance.com/docs/binance-spot-api-docs/rest-api/general-endpoints)) to find min/max order sizes or the margin symbol information ( [GET /sapi/v1/margin/allAssets](https://developers.binance.com/docs/margin_trading/market-data/Get-All-Margin-Assets)) to find the minimum borrow/repay amount. If an order is invalid (e.g., too small), you’ll get an error about lot size or minimum notional.
- Canceling Orders: If you need to cancel an open margin order, use [DELETE /sapi/v1/margin/order](https://developers.binance.com/docs/margin_trading/trade/Margin-Account-Cancel-Order#http-request) with similar parameters (symbol, orderId or newclientOrderId, and if isolated, isIsolated=TRUE). There is also an endpoint to cancel all open margin orders on a symbol ( [DELETE /sapi/v1/margin/openOrders](https://developers.binance.com/docs/margin_trading/trade/Margin-Account-Cancel-All-Open-Orders)). Canceling a margin order that had an auto-borrow does not automatically repay the borrow (unless you set autoRepayAtCancel=true for One-Triggers-a-One-Cancels-the-Other (OTOCO) orders – a parameter autoRepayAtCancel exists for that purpose.)
- Limit price restriction: This error {“code”: -3064, “msg”: “Limit price needs to be within (-15%,15%) of current index price for this margin trading pair.”} often occurs when the limit price is not allowed. For certain low liquidity pairs or stablecoin to stablecoin pairs on Margin (e.g. USDT/DAI), there will be a price bracket of \[-15%, 15%\] (which is subject to changes). Please adjust the limit price accordingly.

At this stage, you have executed margin trades. Now it is important that you monitor your margin account to manage risk, as margin trading carries the risk of liquidation if the market moves against you.

### Monitoring the Margin Account

Properly monitoring your margin account is vital. You need to keep track of your balances, margin level (risk ratio), and any accumulating interest. Binance provides endpoints to fetch account details for both cross and isolated margin accounts.

#### Cross Margin Account Details

The endpoint: [GET /sapi/v1/margin/account](https://developers.binance.com/docs/margin_trading/account/Query-Cross-Margin-Account-Details) returns an overview of your cross margin account. This includes your current margin level, total asset value, total liability (debt) value, and a breakdown of each asset. Key fields in the response:

{

"assets":\[

```text
{

    "baseAsset":

    {

      "asset": "BTC",

      "borrowEnabled": true,

      "borrowed": "0.00000000",

      "free": "0.00000000",

      "interest": "0.00000000",

      "locked": "0.00000000",

      "netAsset": "0.00000000",

      "netAssetOfBtc": "0.00000000",

      "repayEnabled": true,

      "totalAsset": "0.00000000"

    },

    "quoteAsset":

    {

      "asset": "USDT",

      "borrowEnabled": true,

      "borrowed": "0.00000000",

      "free": "0.00000000",

      "interest": "0.00000000",

      "locked": "0.00000000",

      "netAsset": "0.00000000",

      "netAssetOfBtc": "0.00000000",

      "repayEnabled": true,

      "totalAsset": "0.00000000"

    },

    "symbol": "BTCUSDT",

    "isolatedCreated": true,

    "enabled": true, // true-enabled, false-disabled

    "marginLevel": "0.00000000",

    "marginLevelStatus": "EXCESSIVE", // "EXCESSIVE", "NORMAL", "MARGIN\_CALL", "PRE\_LIQUIDATION", "FORCE\_LIQUIDATION"

    "marginRatio": "0.00000000",

    "indexPrice": "10000.00000000",

    "liquidatePrice": "1000.00000000",

    "liquidateRate": "1.00000000",

    "tradeEnabled": true

  }

],

"totalAssetOfBtc": "0.00000000",

"totalLiabilityOfBtc": "0.00000000",

"totalNetAssetOfBtc": "0.00000000"
```

}

#### Isolated Margin Account Details

For isolated accounts, there is a similar endpoint: [GET /sapi/v1/margin/isolated/account](https://developers.binance.com/docs/margin_trading/account/Query-Isolated-Margin-Account-Info). This response includes details for each isolated margin account you have enabled. Each isolated account will have fields including isolatedMarginLevel, totalAsset, totalLiability, etc., specific to that pair, along with an array of assets (two assets per isolated pair, typically).

{

"assets":\[

```text
{

    "baseAsset":

    {

      "asset": "BTC",

      "borrowEnabled": true,

      "borrowed": "0.00000000",

      "free": "0.00000000",

      "interest": "0.00000000",

      "locked": "0.00000000",

      "netAsset": "0.00000000",

      "netAssetOfBtc": "0.00000000",

      "repayEnabled": true,

      "totalAsset": "0.00000000"

    },

    "quoteAsset":

    {

      "asset": "USDT",

      "borrowEnabled": true,

      "borrowed": "0.00000000",

      "free": "0.00000000",

      "interest": "0.00000000",

      "locked": "0.00000000",

      "netAsset": "0.00000000",

      "netAssetOfBtc": "0.00000000",

      "repayEnabled": true,

      "totalAsset": "0.00000000"

    },

    "symbol": "BTCUSDT",

    "isolatedCreated": true,

    "enabled": true, // true-enabled, false-disabled

    "marginLevel": "0.00000000",

    "marginLevelStatus": "EXCESSIVE", // "EXCESSIVE", "NORMAL", "MARGIN\_CALL", "PRE\_LIQUIDATION", "FORCE\_LIQUIDATION"

    "marginRatio": "0.00000000",

    "indexPrice": "10000.00000000",

    "liquidatePrice": "1000.00000000",

    "liquidateRate": "1.00000000",

    "tradeEnabled": true

  }

],

"totalAssetOfBtc": "0.00000000",

"totalLiabilityOfBtc": "0.00000000",

"totalNetAssetOfBtc": "0.00000000"
```

}

#### User Data Stream (Advanced)

Binance provides a WebSocket User Data Stream for margin accounts that can push real-time updates on account balance changes and order updates. If you need real-time monitoring (instead of polling REST endpoints), you can create a listenKey via [POST /sapi/v1/userDataStream](https://developers.binance.com/docs/margin_trading/trade-data-stream/Start-Margin-User-Data-Stream) (for margin) and subscribe to events. The use of data stream is advanced and beyond the scope of this guide, but you may wish to keep it in mind if you want instantaneous alerts on margin calls or fills.

After actively managing your positions, you will eventually want to close them and repay any borrowed funds. Let’s move to repaying the loans.

### Repaying Borrowed Tokens

Once you’ve finished using the borrowed tokens (for example, after closing a leveraged trade), you can repay your margin loans. Repaying returns the borrowed tokens to Binance and frees up your collateral. You can repay partially or in full.

In Binance, we use the same endpoint to execute borrow and repay

[POST /sapi/v1/margin/borrow-repay](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Margin-Account-Borrow-Repay)

- Request Parameter:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| isIsolated | STRING | YES | TRUE for Isolated Margin, FALSE for Cross Margin, Default FALSE |
| symbol | STRING | YES | Only for Isolated margin |
| amount | STRING | YES |  |
| type | STRING | YES | BORROW or REPAY |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

Upon repayment successfully, you will receive a JSON with a transaction ID for the loan. For example:

{ "tranId": 1234567894 }

This indicates the repay transaction was successful. After repayment, your outstanding loan for that asset should decrease by the amount repaid (and interest for that asset may drop to 0 if fully repaid).

With borrowing and repaying covered, you have essentially completed a margin trade lifecycle: fund account -> borrow -> trade -> (optional: trade back) -> repay. The last piece is keeping track of what happened – your trade and transaction history.

### Reviewing Trading and Account History

Iti’s important to review your margin trades and account activities, both for understanding your performance and for record-keeping. It can also be helpful for debugging your trading bot. Binance’s API provides endpoints to query past orders, trades, and account actions (transfers, loans, repayments, etc.).

#### Trade History

To get a list of trades (fills) executed on your margin account, use [GET /sapi/v1/margin/myTrades.](https://developers.binance.com/docs/margin_trading/trade/Query-Margin-Account-Trade-List#http-request)

- Request Parameter:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | For isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | TradeId to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

For example, after an earlier BNB buy, a call to myTrades for BNBBTC may return something like:

\[
{
"commission": "0.00006000",
"commissionAsset": "BTC",
"id": 34,
"isBestMatch": true,
"isBuyer": false,
"isMaker": false,
"orderId": 39324,
"price": "0.02000000",
"qty": "3.00000000",
"symbol": "BNBBTC",
"isIsolated": false,
"time": 1561973357171
}
\]

#### Order History

If you need the order details (including orders that might not have any fills, such as canceled orders), you can use:

- [GET /sapi/v1/margin/allOrders](https://developers.binance.com/docs/margin_trading/trade/Query-Margin-Account-All-Orders) – to fetch all orders (filled, canceled, etc.) on a symbol.
- [GET /sapi/v1/margin/openOrders](https://developers.binance.com/docs/margin_trading/trade/Query-Margin-Account-Open-Orders#http-request) – for current open orders (similar to spot).
- [GET /sapi/v1/margin/order](https://developers.binance.com/docs/margin_trading/trade/Query-Margin-Account-Order) – to query a specific order by ID.

#### Account Activity History

Binance provides endpoints to review other account activities:

- [GET /sapi/v1/margin/borrow-repay](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Query-Borrow-Repay#http-request) – Query borrow/repay history. You can filter by asset, isolated symbol, etc. This returns records of each loan and repay transaction with amounts, interest, status. For example, it can show when you borrowed 100 USDC, with a tranId and timestamp.
- [GET /sapi/v1/margin/transfer](https://developers.binance.com/docs/margin_trading/transfer#http-request) – Query transfer history between spot and margin. You can see deposits and withdrawals from margin accounts, with timestamps and amounts.
- [GET /sapi/v1/margin/interestHistory](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Get-Interest-History#http-request) – List of interest charged over time per asset.
- [GET /sapi/v1/margin/forceLiquidationRec](https://developers.binance.com/docs/margin_trading/trade/Get-Force-Liquidation-Record#http-request) – Record of any forced liquidations (hopefully none!).

Copyright © 2026 Binance.

---

Margin Trading

# Get future hourly interest rate (USER\_DATA)

## API Description

Get future hourly interest rate

## HTTP Request

GET `/sapi/v1/margin/next-hourly-interest-rate`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| assets | String | YES | List of assets, separated by commas, up to 20 |
| isIsolated | Boolean | YES | for isolated margin or not, "TRUE", "FALSE" |

## Response Example

```javascript
[
    {
        "asset": "BTC",
        "nextHourlyInterestRate": "0.00000571"
    },
    {
        "asset": "ETH",
        "nextHourlyInterestRate": "0.00000578"
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Interest History (USER\_DATA)

## API Description

Get Interest History

## HTTP Request

GET `/sapi/v1/margin/interestHistory`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| isolatedSymbol | STRING | NO | isolated symbol |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Response in descending order
- If isolatedSymbol is not sent, crossed margin data will be returned
- The max interval between `startTime` and `endTime` is 30 days. It is a MUST to ensure data correctness.
- If `startTime`and `endTime` not sent, return records of the last 7 days by default.
- If `startTime` is sent and `endTime` is not sent, return records of \[max(`startTime`, now-30d), now\].
- If `startTime` is not sent and `endTime` is sent, return records of \[`endTime`-7, `endTime`\]
- `type`in response has 4 enums:

  - `PERIODIC` interest charged per hour
  - `ON_BORROW` first interest charged on borrow
  - `PERIODIC_CONVERTED` interest charged per hour converted into BNB
  - `ON_BORROW_CONVERTED` first interest charged on borrow converted into BNB
  - `PORTFOLIO` interest charged daily on the portfolio margin negative balance

## Response Example

```javascript
{
  "rows": [
    {
      "txId": 1352286576452864727,
      "interestAccuredTime": 1672160400000,
      "asset": "USDT",
      "rawAsset": “USDT”,  // will not be returned for isolated margin
      "principal": "45.3313",
      "interest": "0.00024995",
      "interestRate": "0.00013233",
      "type": "ON_BORROW",
      "isolatedSymbol": "BNBUSDT"  // isolated symbol, will not be returned for crossed margin
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get future hourly interest rate (USER\_DATA)

## API Description

Get future hourly interest rate

## HTTP Request

GET `/sapi/v1/margin/next-hourly-interest-rate`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| assets | String | YES | List of assets, separated by commas, up to 20 |
| isIsolated | Boolean | YES | for isolated margin or not, "TRUE", "FALSE" |

## Response Example

```javascript
[
    {
        "asset": "BTC",
        "nextHourlyInterestRate": "0.00000571"
    },
    {
        "asset": "ETH",
        "nextHourlyInterestRate": "0.00000578"
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin account borrow/repay(MARGIN)

## API Description

Margin account borrow/repay(MARGIN)

## HTTP Request

POST `/sapi/v1/margin/borrow-repay`

## Request Weight

1500

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| isIsolated | STRING | YES | `TRUE` for Isolated Margin, `FALSE` for Cross Margin, Default `FALSE` |
| symbol | STRING | YES | Only for Isolated margin |
| amount | STRING | YES |  |
| type | STRING | YES | BORROW or REPAY |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  //transaction id
  "tranId": 100000001
}
```

\*\*Error Code Description: \*\*

- **INSUFFICIENT\_INVENTORY**

The error {"code": -3045, "msg": "The system does not have enough asset now."} can occur to both manual Margin borrow requests and auto-borrow Margin orders that require actual borrowing. The error can be due to:

  - The Margin system's available assets are below the requested borrowing amount.
  - The system's inventory is critically low, leading to the rejection of all borrowing requests, irrespective of the amount.

We recommend monitoring the system status and adjusting your borrowing strategies accordingly.

- **EXCEED\_MAX\_BORROWABLE**

The error {"code": -3006, "msg": "Your borrow amount has exceed maximum borrow amount."} occurs when your borrow request exceeds the maximum allowable amount. You can check the maximum borrowable amount using [GET /sapi/v1/margin/maxBorrowable](https://developers.binance.com/docs/margin_trading/borrow-and-repay/Query-Max-Borrow) and adjust your request accordingly.

- **REPAY\_EXCEED\_LIABILITY**

When repaying your debt, ensure that your repayment does not exceed the outstanding borrowed amount. Otherwise, the error {“code”: -3015, “msg”: “Repay amount exceeds borrow amount.”} will occur.

- **ASSET\_ADMIN\_BAN\_BORROW**

This error {“code”: -3012, “msg”: “Borrow is banned for this asset.”} indicates that borrowing is currently prohibited for the specified asset. You can check the availability of borrowing via [GET /sapi/v1/margin/allAssets](https://developers.binance.com/docs/margin_trading/market-data/Get-All-Margin-Assets). You can also check if there are any announcements or updates regarding the asset's borrowing status on Binance's official channels.

- **FEW\_LIABILITY\_LEFT**

If you get an error {"code": -3015, "msg": "The unpaid debt is too small after this repayment."}, this means your repayment would leave a remaining debt below Binance's minimum threshold. You can resolve this by adjusting the repayment to meet the minimum requirement.

- **HAS\_PENDING\_TRANSACTION**

This error {“code”: -3007, “msg”: “You have pending transaction, please try again later.”} indicates that there is an ongoing borrow or repayment process in your account, preventing new borrow or repayment actions. This can occur in both manual and auto-borrow margin orders. Key points to consider:
  - Concurrent Transactions: The system processes borrow and repay requests sequentially, even if they involve different assets. An ongoing transaction can block new requests temporarily.
  - Processing Time: Typically, these borrow/repay complete within 100 milliseconds. To lower the potential of encountering this error, you may wish to set your requests apart with at least 100 milliseconds intervals.
  - Auto Repayment: Auto-repay orders might fail silently due to the same issue, without generating an error message. We suggest you check your outstanding loan once the auto-repay orders are triggered.

 

Copyright © 2026 Binance.

---

Margin Trading

# Query borrow/repay records in Margin account(USER\_DATA)

## API Description

Query borrow/repay records in Margin account

## HTTP Request

GET `/sapi/v1/margin/borrow-repay`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| isolatedSymbol | STRING | NO | Symbol in Isolated Margin |
| txId | LONG | NO | `tranId` in `POST /sapi/v1/margin/loan` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Current querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| type | STRING | YES | `BORROW` or `REPAY` |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - `txId` or `startTime` must be sent. `txId` takes precedence.
>   Response in descending order
> - If an asset is sent, data within 30 days before `endTime`; If an asset is not sent, data within 7 days before `endTime`
> - If neither `startTime` nor `endTime` is sent, the recent 7-day data will be returned.
> - `startTime` set as `endTime` \- 7days by default, `endTime` set as current time by default

## Response Example

```javascript
{
  "rows": [
      {
        "type": "AUTO", // AUTO,MANUAL for Cross Margin Borrow; MANUAL，AUTO，BNB_AUTO_REPAY，POINT_AUTO_REPAY for Cross Margin Repay; AUTO，MANUAL for Isolated Margin Borrow/Repay;
        "isolatedSymbol": "BNBUSDT",     // isolated symbol, will not be returned for crossed margin
        "amount": "14.00000000",   // Total amount borrowed/repaid
        "asset": "BNB",
        "interest": "0.01866667",    // Interest repaid
        "principal": "13.98133333",   // Principal repaid
        "status": "CONFIRMED",   //one of PENDING (pending execution), CONFIRMED (successfully execution), FAILED (execution failed, nothing happened to your account);
        "timestamp": 1563438204000,
        "txId": 2970933056
      }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Interest Rate History (USER\_DATA)

## API Description

Query Margin Interest Rate History

## HTTP Request

GET `/sapi/v1/margin/interestRateHistory`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| vipLevel | INT | NO | Default: user's vip level |
| startTime | LONG | NO | Default: 7 days ago |
| endTime | LONG | NO | Default: present. Maximum range: 1 months. |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "asset": "BTC",
        "dailyInterestRate": "0.00025000",
        "timestamp": 1611544731000,
        "vipLevel": 1
    },
    {
        "asset": "BTC",
        "dailyInterestRate": "0.00035000",
        "timestamp": 1610248118000,
        "vipLevel": 1
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Max Borrow (USER\_DATA)

## API Description

Query Max Borrow

## HTTP Request

GET `/sapi/v1/margin/maxBorrowable`

## Request Weight

**50(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| isolatedSymbol | STRING | NO | isolated symbol |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- If isolatedSymbol is not sent, crossed margin data will be sent.
- `borrowLimit` is also available from [https://www.binance.com/en/margin-fee](https://www.binance.com/en/margin-fee)

## Response Example

```javascript
{
  "amount": "1.69248805", // account's currently max borrowable amount with sufficient system availability
  "borrowLimit": "60" // max borrowable amount limited by the account level
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Public API Definitions

## Terminology

These terms will be used throughout the documentation, so it is recommended especially for new users to read to help their understanding of the API.

- `base asset` refers to the asset that is the `quantity` of a symbol. For the symbol BTCUSDT, BTC would be the `base asset.`
- `quote asset` refers to the asset that is the `price` of a symbol. For the symbol BTCUSDT, USDT would be the `quote asset`.

## ENUM definitions

**Symbol status (status):**

- `PRE_TRADING`
- `TRADING`
- `POST_TRADING`
- `END_OF_DAY`
- `HALT`
- `AUCTION_MATCH`
- `BREAK`

**Account and Symbol Permissions (permissions):**

- `SPOT`
- `MARGIN`
- `LEVERAGED`
- `TRD_GRP_002`
- `TRD_GRP_003`
- `TRD_GRP_004`
- `TRD_GRP_005`
- `TRD_GRP_006`
- `TRD_GRP_007`
- `TRD_GRP_008`
- `TRD_GRP_009`
- `TRD_GRP_010`
- `TRD_GRP_011`
- `TRD_GRP_012`
- `TRD_GRP_013`
- `TRD_GRP_014`

**Order status (status):**

| Status | Description |
| --- | --- |
| `NEW` | The order has been accepted by the engine. |
| `PARTIALLY_FILLED` | A part of the order has been filled. |
| `FILLED` | The order has been completed. |
| `CANCELED` | The order has been canceled by the user. |
| `PENDING_CANCEL` | Currently unused |
| `REJECTED` | The order was not accepted by the engine and not processed. |
| `EXPIRED` | The order was canceled according to the order type's rules (e.g. LIMIT FOK orders with no fill, LIMIT IOC or MARKET orders that partially fill) or by the exchange, (e.g. orders canceled during liquidation, orders canceled during maintenance) |
| `EXPIRED_IN_MATCH` | The order was canceled by the exchange due to STP trigger. (e.g. an order with `EXPIRE_TAKER` will match with existing orders on the book with the same account or same `tradeGroupId`) |

**OCO Status (listStatusType):**

| Status | Description |
| --- | --- |
| `RESPONSE` | This is used when the ListStatus is responding to a failed action. (E.g. Orderlist placement or cancellation) |
| `EXEC_STARTED` | The order list has been placed or there is an update to the order list status. |
| `ALL_DONE` | The order list has finished executing and thus no longer active. |

**OCO Order Status (listOrderStatus):**

| Status | Description |
| --- | --- |
| `EXECUTING` | Either an order list has been placed or there is an update to the status of the list. |
| `ALL_DONE` | An order list has completed execution and thus no longer active. |
| `REJECT` | The List Status is responding to a failed action either during order placement or order canceled.) |

**ContingencyType**

- `OCO`

**AllocationType**

- `SOR`

**WorkingFloor**

- `EXCHANGE`
- `SOR`

**Order types (orderTypes, type):**

- `LIMIT`
- `MARKET`
- `STOP_LOSS`
- `STOP_LOSS_LIMIT`
- `TAKE_PROFIT`
- `TAKE_PROFIT_LIMIT`
- `LIMIT_MAKER`

**Order Response Type (newOrderRespType):**

- `ACK`
- `RESULT`
- `FULL`

**Order side (side):**

- BUY
- SELL

**Time in force (timeInForce):**

This sets how long an order will be active before expiration.

| Status | Description |
| --- | --- |
| `GTC` | Good Til Canceled <br> An order will be on the book unless the order is canceled. |
| `IOC` | Immediate Or Cancel <br> An order will try to fill the order as much as it can before the order expires. |
| `FOK` | Fill or Kill <br> An order will expire if the full order cannot be filled upon execution. |

**Kline/Candlestick chart intervals:**

s-> seconds; m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

- 1s
- 1m
- 3m
- 5m
- 15m
- 30m
- 1h
- 2h
- 4h
- 6h
- 8h
- 12h
- 1d
- 3d
- 1w
- 1M

**Rate limiters (rateLimitType)**

> REQUEST\_WEIGHT

```json
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 6000
    }
```

> ORDERS

```json
    {
      "rateLimitType": "ORDERS",
      "interval": "SECOND",
      "intervalNum": 10,
      "limit": 100
    },
    {
      "rateLimitType": "ORDERS",
      "interval": "DAY",
      "intervalNum": 1,
      "limit": 200000
    }
```

> RAW\_REQUESTS

```json
    {
      "rateLimitType": "RAW_REQUESTS",
      "interval": "MINUTE",
      "intervalNum": 5,
      "limit": 5000
    }
```

- REQUEST\_WEIGHT

- ORDERS

- RAW\_REQUESTS

**Rate limit intervals (interval)**

- SECOND
- MINUTE
- DAY

* * *

# Filters

Filters define trading rules on a symbol or an exchange.
Filters come in two forms: `symbol filters` and `exchange filters`.

## Symbol Filters

### PRICE\_FILTER

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "PRICE_FILTER",
    "minPrice": "0.00000100",
    "maxPrice": "100000.00000000",
    "tickSize": "0.00000100"
  }
```

The `PRICE_FILTER` defines the `price` rules for a symbol. There are 3 parts:

- `minPrice` defines the minimum `price`/`stopPrice` allowed; disabled on `minPrice` == 0.
- `maxPrice` defines the maximum `price`/`stopPrice` allowed; disabled on `maxPrice` == 0.
- `tickSize` defines the intervals that a `price`/`stopPrice` can be increased/decreased by; disabled on `tickSize` == 0.

Any of the above variables can be set to 0, which disables that rule in the `price filter`. In order to pass the `price filter`, the following must be true for `price`/`stopPrice` of the enabled rules:

- `price` >= `minPrice`
- `price` <= `maxPrice`
- `price` % `tickSize` == 0

### PERCENT\_PRICE

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "PERCENT_PRICE",
    "multiplierUp": "1.3000",
    "multiplierDown": "0.7000",
    "avgPriceMins": 5
  }
```

The `PERCENT_PRICE` filter defines the valid range for the price based on the average of the previous trades.
`avgPriceMins` is the number of minutes the average price is calculated over. 0 means the last price is used.

In order to pass the `percent price`, the following must be true for `price`:

- `price` <= `weightedAveragePrice` \\* `multiplierUp`
- `price` >= `weightedAveragePrice` \\* `multiplierDown`

### PERCENT\_PRICE\_BY\_SIDE

> **ExchangeInfo format:**

```javascript
    {
          "filterType": "PERCENT_PRICE_BY_SIDE",
          "bidMultiplierUp": "1.2",
          "bidMultiplierDown": "0.2",
          "askMultiplierUp": "5",
          "askMultiplierDown": "0.8",
          "avgPriceMins": 1
    }
```

The `PERCENT_PRICE_BY_SIDE` filter defines the valid range for the price based on the average of the previous trades.

`avgPriceMins` is the number of minutes the average price is calculated over. 0 means the last price is used.

There is a different range depending on whether the order is placed on the `BUY` side or the `SELL` side.

Buy orders will succeed on this filter if:

- `Order price` <= `weightedAveragePrice` \\* `bidMultiplierUp`
- `Order price` >= `weightedAveragePrice` \\* `bidMultiplierDown`

Sell orders will succeed on this filter if:

- `Order Price` <= `weightedAveragePrice` \\* `askMultiplierUp`
- `Order Price` >= `weightedAveragePrice` \\* `askMultiplierDown`

### LOT\_SIZE

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "LOT_SIZE",
    "minQty": "0.00100000",
    "maxQty": "100000.00000000",
    "stepSize": "0.00100000"
  }
```

The `LOT_SIZE` filter defines the `quantity` (aka "lots" in auction terms) rules for a symbol. There are 3 parts:

- `minQty` defines the minimum `quantity`/`icebergQty` allowed.
- `maxQty` defines the maximum `quantity`/`icebergQty` allowed.
- `stepSize` defines the intervals that a `quantity`/`icebergQty` can be increased/decreased by.

In order to pass the `lot size`, the following must be true for `quantity`/`icebergQty`:

- `quantity` >= `minQty`
- `quantity` <= `maxQty`
- `quantity` % `stepSize` == 0

### MIN\_NOTIONAL

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "MIN_NOTIONAL",
    "minNotional": "0.00100000",
    "applyToMarket": true,
    "avgPriceMins": 5
  }
```

The `MIN_NOTIONAL` filter defines the minimum notional value allowed for an order on a symbol.
An order's notional value is the `price` \\* `quantity`.
If the order is an Algo order (e.g. `STOP_LOSS_LIMIT`), then the notional value of the `stopPrice` \\* `quantity` will also be evaluated.
If the order is an Iceberg Order, then the notional value of the `price` \\* `icebergQty` will also be evaluated.
`applyToMarket` determines whether or not the `MIN_NOTIONAL` filter will also be applied to `MARKET` orders.
Since `MARKET` orders have no price, the average price is used over the last `avgPriceMins` minutes.
`avgPriceMins` is the number of minutes the average price is calculated over. 0 means the last price is used.

### NOTIONAL

> **ExchangeInfo format:**

```javascript
{
   "filterType": "NOTIONAL",
   "minNotional": "10.00000000",
   "applyMinToMarket": false,
   "maxNotional": "10000.00000000",
   "applyMaxToMarket": false,
   "avgPriceMins": 5
}
```

The `NOTIONAL` filter defines the acceptable notional range allowed for an order on a symbol.

`applyMinToMarket` determines whether the `minNotional` will be applied to `MARKET` orders.

`applyMaxToMarket` determines whether the `maxNotional` will be applied to `MARKET` orders.

In order to pass this filter, the notional (`price * quantity`) has to pass the following conditions:

- `price * quantity` <= `maxNotional`
- `price * quantity` >= `minNotional`

For `MARKET` orders, the average price used over the last `avgPriceMins` minutes will be used for calculation.

If the `avgPriceMins` is 0, then the last price will be used.

### ICEBERG\_PARTS

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "ICEBERG_PARTS",
    "limit": 10
  }
```

The `ICEBERG_PARTS` filter defines the maximum parts an iceberg order can have. The number of `ICEBERG_PARTS` is defined as `CEIL(qty / icebergQty)`.

### MARKET\_LOT\_SIZE

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "MARKET_LOT_SIZE",
    "minQty": "0.00100000",
    "maxQty": "100000.00000000",
    "stepSize": "0.00100000"
  }
```

The `MARKET_LOT_SIZE` filter defines the `quantity` (aka "lots" in auction terms) rules for `MARKET` orders on a symbol. There are 3 parts:

- `minQty` defines the minimum `quantity` allowed.
- `maxQty` defines the maximum `quantity` allowed.
- `stepSize` defines the intervals that a `quantity` can be increased/decreased by.

In order to pass the `market lot size`, the following must be true for `quantity`:

- `quantity` >= `minQty`
- `quantity` <= `maxQty`
- `quantity` % `stepSize` == 0

### MAX\_NUM\_ORDERS

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "MAX_NUM_ORDERS",
    "maxNumOrders": 25
  }
```

The `MAX_NUM_ORDERS` filter defines the maximum number of orders an account is allowed to have open on a symbol.
Note that both "algo" orders and normal orders are counted for this filter.

### MAX\_NUM\_ALGO\_ORDERS

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "MAX_NUM_ALGO_ORDERS",
    "maxNumAlgoOrders": 5
  }
```

The `MAX_NUM_ALGO_ORDERS` filter defines the maximum number of "algo" orders an account is allowed to have open on a symbol.
"Algo" orders are `STOP_LOSS`, `STOP_LOSS_LIMIT`, `TAKE_PROFIT`, and `TAKE_PROFIT_LIMIT` orders.

### MAX\_NUM\_ICEBERG\_ORDERS

The `MAX_NUM_ICEBERG_ORDERS` filter defines the maximum number of `ICEBERG` orders an account is allowed to have open on a symbol.
An `ICEBERG` order is any order where the `icebergQty` is > 0.

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "MAX_NUM_ICEBERG_ORDERS",
    "maxNumIcebergOrders": 5
  }
```

### MAX\_POSITION

The `MAX_POSITION` filter defines the allowed maximum position an account can have on the base asset of a symbol.
An account's position defined as the sum of the account's:

1. free balance of the base asset
2. locked balance of the base asset
3. sum of the qty of all open BUY orders

`BUY` orders will be rejected if the account's position is greater than the maximum position allowed.

If an order's `quantity` can cause the position to overflow, this will also fail the `MAX_POSITION` filter.

> **ExchangeInfo format:**

```javascript
{
  "filterType":"MAX_POSITION",
  "maxPosition":"10.00000000"
}
```

### TRAILING\_DELTA

> **ExchangeInfo format:**

```javascript
    {
          "filterType": "TRAILING_DELTA",
          "minTrailingAboveDelta": 10,
          "maxTrailingAboveDelta": 2000,
          "minTrailingBelowDelta": 10,
          "maxTrailingBelowDelta": 2000
   }
```

The `TRAILING_DELTA` filter defines the minimum and maximum value for the parameter `trailingDelta`.

In order for a trailing stop order to pass this filter, the following must be true:

For `STOP_LOSS BUY`, `STOP_LOSS_LIMIT_BUY`,`TAKE_PROFIT SELL` and `TAKE_PROFIT_LIMIT SELL` orders:

- `trailingDelta` >= `minTrailingAboveDelta`
- `trailingDelta` <= `maxTrailingAboveDelta`

For `STOP_LOSS SELL`, `STOP_LOSS_LIMIT SELL`, `TAKE_PROFIT BUY`, and `TAKE_PROFIT_LIMIT BUY` orders:

- `trailingDelta` >= `minTrailingBelowDelta`
- `trailingDelta` <= `maxTrailingBelowDelta`

## Exchange Filters

### EXCHANGE\_MAX\_NUM\_ORDERS

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "EXCHANGE_MAX_NUM_ORDERS",
    "maxNumOrders": 1000
  }
```

The `EXCHANGE_MAX_NUM_ORDERS` filter defines the maximum number of orders an account is allowed to have open on the exchange.
Note that both "algo" orders and normal orders are counted for this filter.

### EXCHANGE\_MAX\_NUM\_ALGO\_ORDERS

> **ExchangeInfo format:**

```javascript
  {
    "filterType": "EXCHANGE_MAX_NUM_ALGO_ORDERS",
    "maxNumAlgoOrders": 200
  }
```

The `EXCHANGE_MAX_NUM_ALGO_ORDERS` filter defines the maximum number of "algo" orders an account is allowed to have open on the exchange.
"Algo" orders are `STOP_LOSS`, `STOP_LOSS_LIMIT`, `TAKE_PROFIT`, and `TAKE_PROFIT_LIMIT` orders.

### EXCHANGE\_MAX\_NUM\_ICEBERG\_ORDERS

The `EXCHANGE_MAX_NUM_ICEBERG_ORDERS` filter defines the maximum number of iceberg orders an account is allowed to have open on the exchange.

> **ExchangeInfo format:**

```javascript
{
  "filterType": "EXCHANGE_MAX_NUM_ICEBERG_ORDERS",
  "maxNumIcebergOrders": 10000
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Cross margin collateral ratio (MARKET\_DATA)

## API Description

Cross margin collateral ratio

## HTTP Request

GET `/sapi/v1/margin/crossMarginCollateralRatio`

## Request Weight

**100(IP)**

## Request Parameters

None

## Response Example

```javascript
[
  {
    "collaterals": [
      {
        "minUsdValue": "0",
        "maxUsdValue": "13000000",
        "discountRate": "1"
      },
      {
        "minUsdValue": "13000000",
        "maxUsdValue": "20000000",
        "discountRate": "0.975"
      },
      {
        "minUsdValue": "20000000",
        "discountRate": "0"
      }
    ],
    "assetNames": [
      "BNX"
    ]
  },
  {
    "collaterals": [
      {
        "minUsdValue": "0",
        "discountRate": "1"
      }
    ],
    "assetNames": [
      "BTC",
      "BUSD",
      "ETH",
      "USDT"
    ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Cross margin collateral ratio (MARKET\_DATA)

## API Description

Cross margin collateral ratio

## HTTP Request

GET `/sapi/v1/margin/crossMarginCollateralRatio`

## Request Weight

**100(IP)**

## Request Parameters

None

## Response Example

```javascript
[
  {
    "collaterals": [
      {
        "minUsdValue": "0",
        "maxUsdValue": "13000000",
        "discountRate": "1"
      },
      {
        "minUsdValue": "13000000",
        "maxUsdValue": "20000000",
        "discountRate": "0.975"
      },
      {
        "minUsdValue": "20000000",
        "discountRate": "0"
      }
    ],
    "assetNames": [
      "BNX"
    ]
  },
  {
    "collaterals": [
      {
        "minUsdValue": "0",
        "discountRate": "1"
      }
    ],
    "assetNames": [
      "BTC",
      "BUSD",
      "ETH",
      "USDT"
    ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get All Cross Margin Pairs (MARKET\_DATA)

## API Description

Get All Cross Margin Pairs

## HTTP Request

GET `/sapi/v1/margin/allPairs`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

## Response Example

```javascript
[
    {
        "base": "BNB",
        "id": 351637150141315861,
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "BNBBTC"
    },
    {
        "base": "TRX",
        "id": 351637923235429141,
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "TRXBTC",
        "delistTime": 1704973040
    },
    {
        "base": "XRP",
        "id": 351638112213990165,
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "XRPBTC"
    },
    {
        "base": "ETH",
        "id": 351638524530850581,
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "ETHBTC"
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get All Isolated Margin Symbol(MARKET\_DATA)

## API Description

Get All Isolated Margin Symbol

## HTTP Request

GET `/sapi/v1/margin/isolated/allPairs`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "base": "BNB",
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "BNBBTC"
    },
    {
        "base": "TRX",
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "TRXBTC"
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get All Margin Assets (MARKET\_DATA)

## API Description

Get All Margin Assets.

## HTTP Request

GET `/sapi/v1/margin/allAssets`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |

## Response Example

```javascript
[
  {
    "assetFullName": "USD coin",
    "assetName": "USDC",
    "isBorrowable": true,
    "isMortgageable": true,
    "userMinBorrow": "0.00000000",
    "userMinRepay": "0.00000000",
    "delistTime": 1704973040
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Delist Schedule (MARKET\_DATA)

## API Description

Get tokens or symbols delist schedule for cross margin and isolated margin

## HTTP Request

GET `/sapi/v1/margin/delist-schedule`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "delistTime": 1686161202000,
    "crossMarginAssets": [
      "BTC",
      "USDT"
    ],
    "isolatedMarginSymbols": [
      "ADAUSDT",
      "BNBUSDT"
    ]
  },
  {
    "delistTime": 1686222232000,
    "crossMarginAssets": [
      "ADA"
    ],
    "isolatedMarginSymbols": []
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Limit Price Pairs(MARKET\_DATA)

## API Description

Query trading pairs with restriction on limit price range.
In margin trading, you can place orders with limit price. Limit price should be within (-15%, 15%) of current index price for a list of margin trading pairs. This rule only impacts limit sell orders with limit price that is lower than current index price and limit buy orders with limit price that is higher than current index price.

- Buy order: Your order will be rejected with an error message notification if the limit price is 15% above the index price.
- Sell order: Your order will be rejected with an error message notification if the limit price is 15% below the index price.
Please review the limit price order placing strategy, backtest and calibrate the planned order size with the trading volume and order book depth to prevent trading loss.

## HTTP Request

GET `/sapi/v1/margin/limit-price-pairs`

## Request Weight(IP)

**1**

## Request Parameters

NA

## Response Example

```javascript
 {  "crossMarginSymbols":
 	[  "BLURUSDC",
  	"SANDBTC",
  	"QKCBTC",
  	"SEIFDUSD",
  	"NEOUSDC",
  	"ARBFDUSD",
  	"ORDIUSDC"
 	]
 }
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get list Schedule (MARKET\_DATA)

## API Description

Get the upcoming tokens or symbols listing schedule for Cross Margin and Isolated Margin.

## HTTP Request

GET `/sapi/v1/margin/list-schedule`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "listTime": 1686161202000,
    "crossMarginAssets": [
      "BTC",
      "USDT"
    ],
    "isolatedMarginSymbols": [
      "ADAUSDT",
      "BNBUSDT"
    ]
  },
  {
    "listTime": 1686222232000,
    "crossMarginAssets": [
      "ADA"
    ],
    "isolatedMarginSymbols": []
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Margin Asset Risk-Based Liquidation Ratio (MARKET\_DATA)

## API Description

Get Margin Asset Risk-Based Liquidation Ratio

## HTTP Request

GET `/sapi/v1/margin/risk-based-liquidation-ratio`

## Request Weight(IP)

1

## Request Parameters

None

## Response Example

```json
[
  { "asset": "USDC", "riskBasedLiquidationRatio": "0.01" },
  { "asset": "BUSD", "riskBasedLiquidationRatio": "0.01" }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Margin Restricted Assets (MARKET\_DATA)

## API Description

Get Margin Restricted Assets

## HTTP Request

GET `/sapi/v1/margin/restricted-asset`

## Request Weight(IP)

1

## Request Parameters

None

## Response Example

```json
{
  "openLongRestrictedAsset": ["ADA", "CHZ", "ETH", "LTC", "XRP", "币安人生"],
  "maxCollateralExceededAsset": ["ACH", "BNB", "BTC", "USDT"]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Isolated Margin Tier Data (USER\_DATA)

## API Description

Get isolated margin tier data collection with any tier as [https://www.binance.com/en/margin-data](https://www.binance.com/en/margin-data)

## HTTP Request

GET `/sapi/v1/margin/isolatedMarginTier`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| tier | INTEGER | NO | All margin tier data will be returned if tier is omitted |
| recvWindow | LONG | NO | No more than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "symbol": "BTCUSDT",
        "tier": 1,
        "effectiveMultiple": "10",
        "initialRiskRatio": "1.111",
        "liquidationRiskRatio": "1.05",
        "baseAssetMaxBorrowable": "9",
        "quoteAssetMaxBorrowable": "70000"
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Liability Coin Leverage Bracket in Cross Margin Pro Mode(MARKET\_DATA)

## API Description

Liability Coin Leverage Bracket in Cross Margin Pro Mode

## HTTP Request

GET `/sapi/v1/margin/leverageBracket`

## Request Weight(IP)

**1**

## Request Parameters

None

## Response Example

```javascript
[
  {
     "assetNames":[
        "SHIB",
        "FDUSD",
        "BTC",
        "ETH",
        "USDC"
     ],
     "rank":1,
     "brackets":[
        {
           "leverage":10,
           "maxDebt":1000000.00000000,
           "maintenanceMarginRate":0.02000000,
           "initialMarginRate":0.1112,
           "fastNum":0
        },
        {
           "leverage":3,
           "maxDebt":4000000.00000000,
           "maintenanceMarginRate":0.07000000,
           "initialMarginRate":0.5000,
           "fastNum":60000.0000000000000000
        }
     ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin PriceIndex (MARKET\_DATA)

## API Description

Query Margin PriceIndex

## HTTP Request

GET `/sapi/v1/margin/priceIndex`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |

## Response Example

```javascript
{
   "calcTime": 1562046418000,
   "price": "0.00333930",
   "symbol": "BNBBTC"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Available Inventory(USER\_DATA)

## API Description

Margin available Inventory query

## HTTP Request

GET `/sapi/v1/margin/available-inventory`

## Request Weight(UID)

**50**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | STRING | YES | MARGIN,ISOLATED |

## Response Example

```javascript
{
    "assets": {
        "MATIC": "100000000",
        "STPT": "100000000",
        "TVK": "100000000",
        "SHIB": "97409653"
    }
   "updateTime": 1699272487
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# User Data Streams Connect

- Margin websocket only support Cross Margin Accounts
- The base API endpoint is: **[https://api.binance.com](https://api.binance.com/)**
- A User Data Stream `listenKey` is valid for 60 minutes after creation.
- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes.
- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.
- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.
- A `listenKey` is a stream.
- Users can listen to multiple streams.
- The base websocket endpoint is: **wss://margin-stream.binance.com**
- User Data Streams are accessed at **/ws/<listenKey>** or **/stream?streams=<listenKey>**
- A single connection to **stream.binance.com** is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Margin Trading

# Close User Data Stream (USER\_STREAM)

## API Description

Close out a user data stream.

## HTTP Request

DELETE `/sapi/v1/margin/listen-key`

## Request Weight(UID)

**3000**

## Request Parameters

None

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# User Data Streams Connect

- Margin websocket only support Cross Margin Accounts
- The base API endpoint is: **[https://api.binance.com](https://api.binance.com/)**
- A User Data Stream `listenKey` is valid for 60 minutes after creation.
- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes.
- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.
- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.
- A `listenKey` is a stream.
- Users can listen to multiple streams.
- The base websocket endpoint is: **wss://margin-stream.binance.com**
- User Data Streams are accessed at **/ws/<listenKey>** or **/stream?streams=<listenKey>**
- A single connection to **stream.binance.com** is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Margin Trading

# Payload: liability update

## Event Description

Liability update during the following :

- borrowing
- Repayment
- Interest Calculation

## Event Name

`USER_LIABILITY_CHANGE`

## Response Example

> **Payload:**

```javascript
{
  "e": "USER_LIABILITY_CHANGE", // Event Type
  "E": 1701949801133, // Event Time
  "a": "BTC", // Asset
  "t": "BORROW", // Liability Update Type
  "p": "0.00000100", // Principle Quantity
  "i": "0.00000000" // Interest Quantity
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Payload: Margin Call

## Event Description

Margin call trigger the event

## Event Name

`MARGIN_LEVEL_STATUS_CHANGE`

## Response Example

```javascript
{
   "e": "MARGIN_LEVEL_STATUS_CHANGE", // Event Type
   "E": 1701949763462, // Event Time
   "l": "1.1", // margin level
   "s": "MARGIN_CALL" // margin call status
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Keepalive User Data Stream (USER\_STREAM)

## API Description

Keepalive a user data stream to prevent a time out.

## HTTP Request

PUT `/sapi/v1/margin/listen-key`

## Request Weight(UID)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| listenKey | STRING | YES |  |

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Start User Data Stream (USER\_STREAM)

## API Description

Start a new user data stream.

## HTTP Request

POST `/sapi/v1/margin/listen-key`

## Request Weight(UID)

**1**

## Request Parameters

None

## Response Example

```javascript
{
  "listenKey": "T3ee22BIYuWqmvne0HNq2A2WsFlEtLhvWCtItw6ffhhd"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Force Liquidation Record (USER\_DATA)

## API Description

Get Force Liquidation Record

## HTTP Request

GET `/sapi/v1/margin/forceLiquidationRec`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| isolatedSymbol | STRING | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Response in descending order

## Response Example

```javascript
  {
      "rows": [
          {
              "avgPrice": "0.00388359",
              "executedQty": "31.39000000",
              "orderId": 180015097,
              "price": "0.00388110",
              "qty": "31.39000000",
              "side": "SELL",
              "symbol": "BNBBTC",
              "timeInForce": "GTC",
              "isIsolated": true,
              "updatedTime": 1558941374745
          }
      ],
      "total": 1
  }
```

 

Copyright © 2026 Binance.

---

Margin Trading

# listenToken Subscription Methods

## Create Margin Account listenToken (USER\_STREAM)

### Description

Create a listenToken that authorizes the user to access the User Data Stream of the current account for a limited amout of time. The stream's validity is specified by the validity parameter (milliseconds), default 24 hours, maximum 24 hours. The response includes the listenToken and the corresponding expirationTime (in milliseconds).

### HTTP Request

**POST**`/sapi/v1/userListenToken`

**Request weight (UID)**: 1

### Request Parameters

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| symbol | STRING | CONDITIONAL | Trading pair symbol; required when isIsolated is true, e.g., BNBUSDT |
| isIsolated | BOOLEAN | NO | Whether it is isolated margin; true means isolated; default is cross margin |
| validity | LONG | NO | Validity in milliseconds; default 24 hours, maximum 24 hours |

### Notes

- The token validity is determined by the validity parameter; default is 24 hours, maximum 24 hours. expirationTime = current time + validity.
- The response returns the token and expirationTime.

### Response Example

```json
{
  "token": "6xXxePXwZRjVSHKhzUCCGnmN3fkvMTXru+pYJS8RwijXk9Vcyr3rkwfVOTcP2OkONqciYA",
  "expirationTime": 1758792204196
}
```

## Subscribe to User Data Stream using listenToken (USER\_STREAM)

### Description

Subscribe to the user data stream using listenToken.

This method must be called on the WebSocket API. For more information about how to use the WebSocket API, see : [WebSocket API documentation](https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/general-api-information)

### method

`userDataStream.subscribe.listenToken`

### Request Example

```json
{
  "id": "f3a8f7a29f2e54df796db582f3d",
  "method": "userDataStream.subscribe.listenToken",
  "params": {
    "listenToken": "5DbylArkmImhyHkpG6s9tbiFy5uAMTFwzx9vwsFjDv9dC3GkKxSuoTCj0HvcJC0WYi8fA"
  }
}
```

### **Request weight**: 2

### Request Parameters

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| listenToken | STRING | YES | The listen token |

### Notes

- Non-authenticated sessions are allowed to use this feature.
- If the listenToken is invalid, an error **-1209** will be returned.
- The subscription is not automatically renewed by the WebSocket API. To extend the validity of your subscription, you must call `/sapi/v1/userListenToken` before the expiration of your current subscription, obtain a new listenToken with an updated expirationTime, and call `userDataStream.subscribe.listenToken` again passing the new listenToken. This will seamlessly extend your subscription to the new expirationDate.
- If the subscription is not extended, it will expire and you will receive a `eventStreamTerminated` event (see example below).
- You can receive the events in SBE instead of JSON if you require better performance. See the [Simple Binary Encoding (SBE) FAQ](https://developers.binance.com/docs/binance-spot-api-docs/faqs/sbe_faq) for more details.

### Response Example

```json
{
  "subscriptionId": 1,
  "expirationTime": 1749094553955907
}
```

### Subscription Expiration Example

```json
{
  "subscriptionId": 0,
  "event": {
    "e": "eventStreamTerminated",
    "E": 1759089357377
  }
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Payload: Account Update

The Margin account uses the same `outboundAccountPosition` event payload as the Spot account.

Please refer to the Spot User Data Streams documentation for the full event description, fields, and response example:

[Spot User Data Streams - Account Update](https://developers.binance.com/docs/binance-spot-api-docs/user-data-stream#account-update)

 

Copyright © 2026 Binance.

---

Margin Trading

# Payload: Balance Update

The Margin account uses the same `balanceUpdate` event payload as the Spot account.

Please refer to the Spot User Data Streams documentation for the full event description, fields, and response example:

[Spot User Data Streams - Balance Update](https://developers.binance.com/docs/binance-spot-api-docs/user-data-stream#balance-update)

 

Copyright © 2026 Binance.

---

Margin Trading

# Payload: Order Update

The Margin account uses the same `executionReport` and `listStatus` event payloads as the Spot account.

Please refer to the Spot User Data Streams documentation for the full event description, fields, and response example:

[Spot User Data Streams - Order Update](https://developers.binance.com/docs/binance-spot-api-docs/user-data-stream#order-update)

 

Copyright © 2026 Binance.

---

Margin Trading

# listenToken Subscription Methods

## Create Margin Account listenToken (USER\_STREAM)

### Description

Create a listenToken that authorizes the user to access the User Data Stream of the current account for a limited amout of time. The stream's validity is specified by the validity parameter (milliseconds), default 24 hours, maximum 24 hours. The response includes the listenToken and the corresponding expirationTime (in milliseconds).

### HTTP Request

**POST**`/sapi/v1/userListenToken`

**Request weight (UID)**: 1

### Request Parameters

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| symbol | STRING | CONDITIONAL | Trading pair symbol; required when isIsolated is true, e.g., BNBUSDT |
| isIsolated | BOOLEAN | NO | Whether it is isolated margin; true means isolated; default is cross margin |
| validity | LONG | NO | Validity in milliseconds; default 24 hours, maximum 24 hours |

### Notes

- The token validity is determined by the validity parameter; default is 24 hours, maximum 24 hours. expirationTime = current time + validity.
- The response returns the token and expirationTime.

### Response Example

```json
{
  "token": "6xXxePXwZRjVSHKhzUCCGnmN3fkvMTXru+pYJS8RwijXk9Vcyr3rkwfVOTcP2OkONqciYA",
  "expirationTime": 1758792204196
}
```

## Subscribe to User Data Stream using listenToken (USER\_STREAM)

### Description

Subscribe to the user data stream using listenToken.

This method must be called on the WebSocket API. For more information about how to use the WebSocket API, see : [WebSocket API documentation](https://developers.binance.com/docs/binance-spot-api-docs/websocket-api/general-api-information)

### method

`userDataStream.subscribe.listenToken`

### Request Example

```json
{
  "id": "f3a8f7a29f2e54df796db582f3d",
  "method": "userDataStream.subscribe.listenToken",
  "params": {
    "listenToken": "5DbylArkmImhyHkpG6s9tbiFy5uAMTFwzx9vwsFjDv9dC3GkKxSuoTCj0HvcJC0WYi8fA"
  }
}
```

### **Request weight**: 2

### Request Parameters

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| listenToken | STRING | YES | The listen token |

### Notes

- Non-authenticated sessions are allowed to use this feature.
- If the listenToken is invalid, an error **-1209** will be returned.
- The subscription is not automatically renewed by the WebSocket API. To extend the validity of your subscription, you must call `/sapi/v1/userListenToken` before the expiration of your current subscription, obtain a new listenToken with an updated expirationTime, and call `userDataStream.subscribe.listenToken` again passing the new listenToken. This will seamlessly extend your subscription to the new expirationDate.
- If the subscription is not extended, it will expire and you will receive a `eventStreamTerminated` event (see example below).
- You can receive the events in SBE instead of JSON if you require better performance. See the [Simple Binary Encoding (SBE) FAQ](https://developers.binance.com/docs/binance-spot-api-docs/faqs/sbe_faq) for more details.

### Response Example

```json
{
  "subscriptionId": 1,
  "expirationTime": 1749094553955907
}
```

### Subscription Expiration Example

```json
{
  "subscriptionId": 0,
  "event": {
    "e": "eventStreamTerminated",
    "E": 1759089357377
  }
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Create Special Key(Low-Latency Trading)(TRADE)

## API Description

- Binance Margin offers low-latency trading through a [special key](https://www.binance.com/en/support/faq/frequently-asked-questions-on-margin-special-api-key-3208663e900d4d2e9fec4140e1832f4e), available exclusively to users with VIP level 7 or higher.
- If you are VIP level 6 or below, please contact your VIP manager for eligibility criterias.

**Supported Products:**

- Cross Margin
- Isolated Margin
- Portfolio Margin Pro

**Unsupported Products:**

- Portfolio Margin

We support several types of API keys:

- Ed25519 (recommended)
- HMAC
- RSA

We recommend to **use Ed25519 API keys** as it should provide the best performance and security out of all supported key types. We accept PKCS#8 (BEGIN PUBLIC KEY). For how to generate an RSA key pair to send API requests on Binance. Please refer to the document below [FAQ](https://www.binance.com/en/support/faq/how-to-generate-an-rsa-key-pair-to-send-api-requests-on-binance-2b79728f331e43079b27440d9d15c5db) .

## How to use the Margin Special Key

- Use the below `sapi` endpoint to create your margin special API Key.
- For accessing the Cross Margin account, do not send the `symbol` parameter.
- For accessing the Isolated Margin account(s), pass the relevant `symbol` parameter in the API Key creation request.
- Use the generated API Key (and Secret key, if applicable) to perform margin trading and listenKey generation via **Spot** REST API (`https://api.binance.com/api/v3/*`) endpoints.

Read [REST API](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#signed-trade-and-user_data-endpoint-security) or [WebSocket API](https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-api.md#request-security) documentation to learn how to use different API keys

You need to enable Permits “Enable Spot & Margin Trading” option for the API Key which requests this endpoint.

## HTTP Request

POST `/sapi/v1/margin/apiKey`

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| apiName | STRING | YES |  |
| symbol | STRING | NO | isolated margin pair |
| ip | STRING | NO | Can be added in batches, separated by commas. Max 30 for an API key |
| publicKey | STRING | NO | 1\. If publicKey is inputted it will create an RSA or Ed25519 key. <br>2\. Need to be encoded to URL-encoded format |
| permissionMode | enum | NO | This parameter is only for the Ed25519 API key, and does not effact for other encryption methods. The value can be TRADE (TRADE for all permissions) or READ (READ for USER\_DATA, FIX\_API\_READ\_ONLY). The default value is TRADE. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "apiKey":"npOzOAeLVgr2TuxWfNo43AaPWpBbJEoKezh1o8mSQb6ryE2odE11A4AoVlJbQoGx",
  "secretKey":"87ssWB7azoy6ACRfyp6OVOL5U3rtZptX31QWw2kWjl1jHEYRbyM1pd6qykRBQw8p" //secretKey will be null when creating an RSA key
  "type": "HMAC_SHA256"   //HMAC_SHA256 or RSA
}
```

Error Code Description

- **UNSUPPORTED\_OPERATION** : Portfolio Margin is an unsupported product, please change the account type to a supported margin product.
- **Forbidden**: Cross Margin Pro accounts require additional agreements, please contact your relationship manager.

 

Copyright © 2026 Binance.

---

Margin Trading

# Delete Special Key(Low-Latency Trading)(TRADE)

## API Description

This only applies to Special Key for Low Latency Trading.

If apiKey is given, apiName will be ignored. If apiName is given with no apiKey, all apikeys with given apiName will be deleted.

You need to enable Permits “Enable Spot & Margin Trading” option for the API Key which requests this endpoint.

## HTTP Request

DELETE `/sapi/v1/margin/apiKey`

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| apiKey | STRING | NO |  |
| apiName | STRING | NO |  |
| symbol | STRING | NO | isolated margin pair |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Edit ip for Special Key(Low-Latency Trading)(TRADE)

## API Description

Edit ip restriction. This only applies to Special Key for Low Latency Trading.

You need to enable Permits “Enable Spot & Margin Trading” option for the API Key which requests this endpoint.

## HTTP Request

PUT `/sapi/v1/margin/apiKey/ip`

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| apiKey | STRING | YES |  |
| symbol | STRING | NO | isolated margin pair |
| ip | STRING | YES | Can be added in batches, separated by commas. Max 30 for an API key |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Force Liquidation Record (USER\_DATA)

## API Description

Get Force Liquidation Record

## HTTP Request

GET `/sapi/v1/margin/forceLiquidationRec`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| isolatedSymbol | STRING | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Response in descending order

## Response Example

```javascript
  {
      "rows": [
          {
              "avgPrice": "0.00388359",
              "executedQty": "31.39000000",
              "orderId": 180015097,
              "price": "0.00388110",
              "qty": "31.39000000",
              "side": "SELL",
              "symbol": "BNBBTC",
              "timeInForce": "GTC",
              "isIsolated": true,
              "updatedTime": 1558941374745
          }
      ],
      "total": 1
  }
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Small Liability Exchange Coin List (USER\_DATA)

## API Description

Query the coins which can be small liability exchange

## HTTP Request

GET `/sapi/v1/margin/exchange-small-liability`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
      "asset": "ETH",
      "interest": "0.00083334",
      "principal": "0.001",
      "liabilityAsset": "USDT",
      "liabilityQty": 0.3552
    }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Small Liability Exchange History (USER\_DATA)

## API Description

Get Small liability Exchange History

## HTTP Request

GET `/sapi/v1/margin/exchange-small-liability-history`

## Request Weight

**100(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| current | INT | YES | Currently querying page. Start from 1. Default:1 |
| size | INT | YES | Default:10, Max:100 |
| startTime | LONG | NO | Default: 30 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "total": 1,
    "rows": [
      {
        "asset": "ETH",
        "amount": "0.00083434",
        "targetAsset": "BUSD",
        "targetAmount": "1.37576819",
        "bizType": "EXCHANGE_SMALL_LIABILITY",
        "timestamp": 1672801339253
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account Cancel all Open Orders on a Symbol (TRADE)

## API Description

Cancels all active orders on a symbol for margin account.

This includes OCO orders.

## HTTP Request

DELETE /sapi/v1/margin/openOrders

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "symbol": "BTCUSDT",
    "isIsolated": true,       // if isolated margin
    "origClientOrderId": "E6APeyTJvkMvLMYMqu1KQ4",
    "orderId": 11,
    "orderListId": -1,
    "clientOrderId": "pXLV6Hz6mprAcVYpVMTGgx",
    "price": "0.089853",
    "origQty": "0.178622",
    "executedQty": "0.000000",
    "cummulativeQuoteQty": "0.000000",
    "status": "CANCELED",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "side": "BUY",
    "selfTradePreventionMode": "NONE"
  },
  {
    "symbol": "BTCUSDT",
    "isIsolated": false,       // if isolated margin
    "origClientOrderId": "A3EF2HCwxgZPFMrfwbgrhv",
    "orderId": 13,
    "orderListId": -1,
    "clientOrderId": "pXLV6Hz6mprAcVYpVMTGgx",
    "price": "0.090430",
    "origQty": "0.178622",
    "executedQty": "0.000000",
    "cummulativeQuoteQty": "0.000000",
    "status": "CANCELED",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "side": "BUY",
    "selfTradePreventionMode": "NONE"
  },
  {
    "orderListId": 1929,
    "contingencyType": "OCO",
    "listStatusType": "ALL_DONE",
    "listOrderStatus": "ALL_DONE",
    "listClientOrderId": "2inzWQdDvZLHbbAmAozX2N",
    "transactionTime": 1585230948299,
    "symbol": "BTCUSDT",
    "isIsolated": true,       // if isolated margin
    "orders": [
      {
        "symbol": "BTCUSDT",
        "orderId": 20,
        "clientOrderId": "CwOOIPHSmYywx6jZX77TdL"
      },
      {
        "symbol": "BTCUSDT",
        "orderId": 21,
        "clientOrderId": "461cPg51vQjV3zIMOXNz39"
      }
    ],
    "orderReports": [
      {
        "symbol": "BTCUSDT",
        "origClientOrderId": "CwOOIPHSmYywx6jZX77TdL",
        "orderId": 20,
        "orderListId": 1929,
        "clientOrderId": "pXLV6Hz6mprAcVYpVMTGgx",
        "price": "0.668611",
        "origQty": "0.690354",
        "executedQty": "0.000000",
        "cummulativeQuoteQty": "0.000000",
        "status": "CANCELED",
        "timeInForce": "GTC",
        "type": "STOP_LOSS_LIMIT",
        "side": "BUY",
        "stopPrice": "0.378131",
        "icebergQty": "0.017083"
      },
      {
        "symbol": "BTCUSDT",
        "origClientOrderId": "461cPg51vQjV3zIMOXNz39",
        "orderId": 21,
        "orderListId": 1929,
        "clientOrderId": "pXLV6Hz6mprAcVYpVMTGgx",
        "price": "0.008791",
        "origQty": "0.690354",
        "executedQty": "0.000000",
        "cummulativeQuoteQty": "0.000000",
        "status": "CANCELED",
        "timeInForce": "GTC",
        "type": "LIMIT_MAKER",
        "side": "BUY",
        "icebergQty": "0.639962"
      }
    ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account Cancel OCO (TRADE)

## API Description

Cancel an entire Order List for a margin account.

## HTTP Request

DELETE /sapi/v1/margin/orderList

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderListId | LONG | NO | Either `orderListId` or `listClientOrderId` must be provided |
| listClientOrderId | STRING | NO | Either `orderListId` or `listClientOrderId` must be provided |
| newClientOrderId | STRING | NO | Used to uniquely identify this cancel. Automatically generated by default |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

Additional notes:

- Canceling an individual leg will cancel the entire OCO

## Response Example

```javascript
{
  "orderListId": 0,
  "contingencyType": "OCO",
  "listStatusType": "ALL_DONE",
  "listOrderStatus": "ALL_DONE",
  "listClientOrderId": "C3wyj4WVEktd7u9aVBRXcN",
  "transactionTime": 1574040868128,
  "symbol": "LTCBTC",
  "isIsolated": false,       // if isolated margin
  "orders": [
    {
      "symbol": "LTCBTC",
      "orderId": 2,
      "clientOrderId": "pO9ufTiFGg3nw2fOdgeOXa"
    },
    {
      "symbol": "LTCBTC",
      "orderId": 3,
      "clientOrderId": "TXOvglzXuaubXAaENpaRCB"
    }
  ],
  "orderReports": [
    {
      "symbol": "LTCBTC",
      "origClientOrderId": "pO9ufTiFGg3nw2fOdgeOXa",
      "orderId": 2,
      "orderListId": 0,
      "clientOrderId": "unfWT8ig8i0uj6lPuYLez6",
      "price": "1.00000000",
      "origQty": "10.00000000",
      "executedQty": "0.00000000",
      "cummulativeQuoteQty": "0.00000000",
      "status": "CANCELED",
      "timeInForce": "GTC",
      "type": "STOP_LOSS_LIMIT",
      "side": "SELL",
      "stopPrice": "1.00000000",
      "selfTradePreventionMode": "NONE"
    },
    {
      "symbol": "LTCBTC",
      "origClientOrderId": "TXOvglzXuaubXAaENpaRCB",
      "orderId": 3,
      "orderListId": 0,
      "clientOrderId": "unfWT8ig8i0uj6lPuYLez6",
      "price": "3.00000000",
      "origQty": "10.00000000",
      "executedQty": "0.00000000",
      "cummulativeQuoteQty": "0.00000000",
      "status": "CANCELED",
      "timeInForce": "GTC",
      "type": "LIMIT_MAKER",
      "side": "SELL",
      "selfTradePreventionMode": "NONE"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account Cancel Order (TRADE)

## API Description

Cancel an active order for margin account.

## HTTP Request

DELETE /sapi/v1/margin/order

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| newClientOrderId | STRING | NO | Used to uniquely identify this cancel. Automatically generated by default. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Either orderId or origClientOrderId must be sent.

## Response Example

```javascript
{
  "symbol": "LTCBTC",
  "isIsolated": true,       // if isolated margin
  "orderId": "28",
  "origClientOrderId": "myOrder1",
  "clientOrderId": "cancelMyOrder1",
  "price": "1.00000000",
  "origQty": "10.00000000",
  "executedQty": "8.00000000",
  "cummulativeQuoteQty": "8.00000000",
  "status": "CANCELED",
  "timeInForce": "GTC",
  "type": "LIMIT",
  "side": "SELL"
}
```

**Error Code Description:**

- **CANCEL\_REJECTED**

This error {“code”: -2011, “msg”: “Unknown order sent.”} occurs when the order (by either orderId, clientOrderId, origClientOrderId) could not be found in the matching engine. It often results from your attempt to cancel an order that has already been processed. You can review all orders( [GET /sapi/v1/margin/allOrders](https://developers.binance.com/docs/margin_trading/trade/Query-Margin-Account-All-Orders)) to confirm the status of the order.

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account New OCO (TRADE)

## API Description

Send in a new OCO for a margin account

## HTTP Request

POST /sapi/v1/margin/order/oco

## Request Weight

**6(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| listClientOrderId | STRING | NO | A unique Id for the entire orderList |
| side | ENUM | YES |  |
| quantity | DECIMAL | YES |  |
| limitClientOrderId | STRING | NO | A unique Id for the limit order |
| price | DECIMAL | YES |  |
| limitIcebergQty | DECIMAL | NO |  |
| stopClientOrderId | STRING | NO | A unique Id for the stop loss/stop loss limit leg |
| stopPrice | DECIMAL | YES |  |
| stopLimitPrice | DECIMAL | NO | If provided, `stopLimitTimeInForce` is required. |
| stopIcebergQty | DECIMAL | NO |  |
| stopLimitTimeInForce | ENUM | NO | Valid values are `GTC`/`FOK`/`IOC` |
| newOrderRespType | ENUM | NO | Set the response JSON. |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY, AUTO\_REPAY,AUTO\_BORROW\_REPAY; default NO\_SIDE\_EFFECT. More info in [FAQ](https://www.binance.com/en/support/faq/how-to-use-the-sideeffecttype-parameter-with-the-margin-order-endpoints-f9fc51cda1984bf08b95e0d96c4570bc) |
| selfTradePreventionMode | ENUM | NO | The allowed enums is dependent on what is configured on the symbol. The possible supported values are EXPIRE\_TAKER, EXPIRE\_MAKER, EXPIRE\_BOTH, NONE |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY or AUTO\_BORROW\_REPAY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- autoRepayAtCancel is suggested to set as “FALSE” to keep liability unrepaid under high frequent new order/cancel order execution

## Response Example

```javascript
{
  "orderListId": 0,
  "contingencyType": "OCO",
  "listStatusType": "EXEC_STARTED",
  "listOrderStatus": "EXECUTING",
  "listClientOrderId": "JYVpp3F0f5CAG15DhtrqLp",
  "transactionTime": 1563417480525,
  "symbol": "LTCBTC",
  "marginBuyBorrowAmount": "5",       // will not return if no margin trade happens
  "marginBuyBorrowAsset": "BTC",    // will not return if no margin trade happens
  "isIsolated": false,       // if isolated margin
  "orders": [
    {
      "symbol": "LTCBTC",
      "orderId": 2,
      "clientOrderId": "Kk7sqHb9J6mJWTMDVW7Vos"
    },
    {
      "symbol": "LTCBTC",
      "orderId": 3,
      "clientOrderId": "xTXKaGYd4bluPVp78IVRvl"
    }
  ],
  "orderReports": [
    {
      "symbol": "LTCBTC",
      "orderId": 2,
      "orderListId": 0,
      "clientOrderId": "Kk7sqHb9J6mJWTMDVW7Vos",
      "transactTime": 1563417480525,
      "price": "0.000000",
      "origQty": "0.624363",
      "executedQty": "0.000000",
      "cummulativeQuoteQty": "0.000000",
      "status": "NEW",
      "timeInForce": "GTC",
      "type": "STOP_LOSS",
      "side": "BUY",
      "stopPrice": "0.960664",
      "selfTradePreventionMode": "NONE"
    },
    {
      "symbol": "LTCBTC",
      "orderId": 3,
      "orderListId": 0,
      "clientOrderId": "xTXKaGYd4bluPVp78IVRvl",
      "transactTime": 1563417480525,
      "price": "0.036435",
      "origQty": "0.624363",
      "executedQty": "0.000000",
      "cummulativeQuoteQty": "0.000000",
      "status": "NEW",
      "timeInForce": "GTC",
      "type": "LIMIT_MAKER",
      "side": "BUY",
      "selfTradePreventionMode": "NONE"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account New OTO (TRADE)

## API Description

Post a new OTO order for margin account:

- An OTO (One-Triggers-the-Other) is an order list comprised of 2 orders.
- The first order is called the **working order** and must be `LIMIT` or `LIMIT_MAKER`. Initially, only the working order goes on the order book.
- The second order is called the **pending order**. It can be any order type except for `MARKET` orders using parameter `quoteOrderQty`. The pending order is only placed on the order book when the working order gets **fully filled**.
- If either the working order or the pending order is cancelled individually, the other order in the order list will also be canceled or expired.
- When the order list is placed, if the working order gets **immediately fully filled**, the placement response will show the working order as `FILLED` but the pending order will still appear as `PENDING_NEW`. You need to query the status of the pending order again to see its updated status.
- OTOs add **2 orders** to the unfilled order count, `EXCHANGE_MAX_NUM_ORDERS` filter and `MAX_NUM_ORDERS` filter.

## HTTP Request

POST `/sapi/v1/margin/order/oto`

## Request Weight

**6(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| listClientOrderId | STRING | NO | Arbitrary unique ID among open order lists. Automatically generated if not sent.<br>A new order list with the same listClientOrderId is accepted only when the previous one is filled or completely expired.<br>`listClientOrderId` is distinct from the `workingClientOrderId` and the `pendingClientOrderId`. |
| newOrderRespType | ENUM | NO | Set the response JSON. ACK, RESULT, or FULL; MARKET and LIMIT order types default to FULL, all other orders default to ACK. |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY. More info in [FAQ](https://www.binance.com/en/support/faq/how-to-use-the-sideeffecttype-parameter-with-the-margin-order-endpoints-f9fc51cda1984bf08b95e0d96c4570bc) |
| selfTradePreventionMode | ENUM | NO | The allowed enums is dependent on what is configured on the symbol. The possible supported values are EXPIRE\_TAKER, EXPIRE\_MAKER, EXPIRE\_BOTH, NONE |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| workingType | ENUM | YES | Supported values: `LIMIT`,`LIMIT_MAKER` |
| workingSide | ENUM | YES | BUY, SELL |
| workingClientOrderId | STRING | NO | Arbitrary unique ID among open orders for the working order. Automatically generated if not sent. |
| workingPrice | DECIMAL | YES |  |
| workingQuantity | DECIMAL | YES | Sets the quantity for the working order. |
| workingIcebergQty | DECIMAL | YES | This can only be used if `workingTimeInForce` is `GTC`. |
| workingTimeInForce | ENUM | NO | GTC,IOC,FOK |
| pendingType | ENUM | YES | Supported values: [Order Types](https://developers.binance.com/docs/binance-spot-api-docs/enums#order-types-ordertypes-type) Note that `MARKET` orders using `quoteOrderQty` are not supported. |
| pendingSide | ENUM | YES | BUY, SELL |
| pendingClientOrderId | STRING | NO | Arbitrary unique ID among open orders for the pending order. Automatically generated if not sent. |
| pendingPrice | DECIMAL | NO |  |
| pendingStopPrice | DECIMAL | NO |  |
| pendingTrailingDelta | DECIMAL | NO |  |
| pendingQuantity | DECIMAL | YES | Sets the quantity for the pending order. |
| pendingIcebergQty | DECIMAL | NO | This can only be used if `pendingTimeInForce` is `GTC`. |
| pendingTimeInForce | ENUM | NO | GTC,IOC,FOK |

- autoRepayAtCancel is suggested to set as “FALSE” to keep liability unrepaid under high frequent new order/cancel order execution

- Depending on the `pendingType` or `workingType`, some optional parameters will become mandatory:

| Type | Additional mandatory parameters | Additional information |
| --- | --- | --- |
| `workingType` = `LIMIT` | `workingTimeInForce` |  |
| `pendingType` = `LIMIT` | `pendingPrice`, `pendingTimeInForce` |  |
| `pendingType` = `STOP_LOSS` or `TAKE_PROFIT` | `pendingStopPrice` and/or `pendingTrailingDelta` |  |
| `pendingType` = `STOP_LOSS_LIMIT` or `TAKE_PROFIT_LIMIT` | `pendingPrice`, `pendingStopPrice` and/or `pendingTrailingDelta`, `pendingTimeInForce` |  |

## Response Example

Response Example:

```javascript
{
    "orderListId": 13551,
    "contingencyType": "OTO",
    "listStatusType": "EXEC_STARTED",
    "listOrderStatus": "EXECUTING",
    "listClientOrderId": "JDuOrsu0Ge8GTyvx8J7VTD",
    "transactionTime": 1725521998054,
    "symbol": "BTCUSDT",
    "isIsolated": false,
    "orders": [
        {
            "symbol": "BTCUSDT",
            "orderId": 29896699,
            "clientOrderId": "y8RB6tQEMuHUXybqbtzTxk"
        },
        {
            "symbol": "BTCUSDT",
            "orderId": 29896700,
            "clientOrderId": "dKQEdh5HhXb7Lpp85jz1dQ"
        }
    ],
    "orderReports": [
        {
            "symbol": "BTCUSDT",
            "orderId": 29896699,
            "orderListId": 13551,
            "clientOrderId": "y8RB6tQEMuHUXybqbtzTxk",
            "transactTime": 1725521998054,
            "price": "80000.00000000",
            "origQty": "0.02000000",
            "executedQty": "0",
            "cummulativeQuoteQty": "0",
            "status": "NEW",
            "timeInForce": "GTC",
            "type": "LIMIT",
            "side": "SELL",
            "selfTradePreventionMode": "NONE"
        },
        {
            "symbol": "BTCUSDT",
            "orderId": 29896700,
            "orderListId": 13551,
            "clientOrderId": "dKQEdh5HhXb7Lpp85jz1dQ",
            "transactTime": 1725521998054,
            "price": "50000.00000000",
            "origQty": "0.02000000",
            "executedQty": "0",
            "cummulativeQuoteQty": "0",
            "status": "PENDING_NEW",
            "timeInForce": "GTC",
            "type": "LIMIT",
            "side": "BUY",
            "selfTradePreventionMode": "NONE"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account New OTOCO (TRADE)

## API Description

Post a new OTOCO order for margin account：

- An OTOCO (One-Triggers-One-Cancels-the-Other) is an order list comprised of 3 orders.
- The first order is called the **working order** and must be `LIMIT` or `LIMIT_MAKER`. Initially, only the working order goes on the order book.

  - The behavior of the working order is the same as the OTO.
- OTOCO has 2 pending orders (pending above and pending below), forming an OCO pair. The pending orders are only placed on the order book when the working order gets **fully filled**.

  - The rules of the pending above and pending below follow the same rules as the [Order List OCO](https://developers.binance.com/docs/margin_trading/trade/Margin-Account-New-OCO).
- OTOCOs add **3 orders** against the unfilled order count, `EXCHANGE_MAX_NUM_ORDERS` filter, and `MAX_NUM_ORDERS` filter.

## HTTP Request

POST `/sapi/v1/margin/order/otoco`

## Request Weight

**6(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY. More info in [FAQ](https://www.binance.com/en/support/faq/how-to-use-the-sideeffecttype-parameter-with-the-margin-order-endpoints-f9fc51cda1984bf08b95e0d96c4570bc) |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| listClientOrderId | STRING | NO | Arbitrary unique ID among open order lists. Automatically generated if not sent. A new order list with the same listClientOrderId is accepted only when the previous one is filled or completely expired. `listClientOrderId` is distinct from the `workingClientOrderId`, `pendingAboveClientOrderId`, and the `pendingBelowClientOrderId`. |
| newOrderRespType | ENUM | NO | Format of the JSON response. Supported values: [Order Response Type](https://developers.binance.com/docs/zh-CN/binance-spot-api-docs/testnet/enums.md#orderresponsetype) |
| selfTradePreventionMode | ENUM | NO | The allowed enums is dependent on what is configured on the symbol. The possible supported values are EXPIRE\_TAKER, EXPIRE\_MAKER, EXPIRE\_BOTH, NONE |
| workingType | ENUM | YES | Supported values: `LIMIT`, `LIMIT_MAKER` |
| workingSide | ENUM | YES | BUY, SELL |
| workingClientOrderId | STRING | NO | Arbitrary unique ID among open orders for the working order. Automatically generated if not sent. |
| workingPrice | DECIMAL | YES |  |
| workingQuantity | DECIMAL | YES |  |
| workingIcebergQty | DECIMAL | NO | This can only be used if `workingTimeInForce` is `GTC`. |
| workingTimeInForce | ENUM | NO | GTC,IOC,FOK |
| pendingSide | ENUM | YES | BUY, SELL |
| pendingQuantity | DECIMAL | YES |  |
| pendingAboveType | ENUM | YES | Supported values: `LIMIT_MAKER`, `STOP_LOSS`, and `STOP_LOSS_LIMIT` |
| pendingAboveClientOrderId | STRING | NO | Arbitrary unique ID among open orders for the pending above order. Automatically generated if not sent. |
| pendingAbovePrice | DECIMAL | NO |  |
| pendingAboveStopPrice | DECIMAL | NO |  |
| pendingAboveTrailingDelta | DECIMAL | NO |  |
| pendingAboveIcebergQty | DECIMAL | NO | This can only be used if `pendingAboveTimeInForce` is `GTC`. |
| pendingAboveTimeInForce | ENUM | NO |  |
| pendingBelowType | ENUM | NO | Supported values: `LIMIT_MAKER`, `STOP_LOSS`, and `STOP_LOSS_LIMIT` |
| pendingBelowClientOrderId | STRING | NO | Arbitrary unique ID among open orders for the pending below order. Automatically generated if not sent. |
| pendingBelowPrice | DECIMAL | NO |  |
| pendingBelowStopPrice | DECIMAL | NO |  |
| pendingBelowTrailingDelta | DECIMAL | NO |  |
| pendingBelowIcebergQty | DECIMAL | NO | This can only be used if `pendingBelowTimeInForce` is `GTC`. |
| pendingBelowTimeInForce | ENUM | NO |  |

- autoRepayAtCancel is suggested to set as “FALSE” to keep liability unrepaid under high frequent new order/cancel order execution

- Depending on the `pendingAboveType`/`pendingBelowType` or `workingType`, some optional parameters will become mandatory:

| Type | Additional mandatory parameters | Additional information |
| --- | --- | --- |
| `workingType` = `LIMIT` | `workingTimeInForce` |  |
| `pendingAboveType`= `LIMIT_MAKER` | `pendingAbovePrice` |  |
| `pendingAboveType`= `STOP_LOSS` | `pendingAboveStopPrice` and/or `pendingAboveTrailingDelta` |  |
| `pendingAboveType`=`STOP_LOSS_LIMIT` | `pendingAbovePrice`, `pendingAboveStopPrice` and/or `pendingAboveTrailingDelta`, `pendingAboveTimeInForce` |  |
| `pendingBelowType`= `LIMIT_MAKER` | `pendingBelowPrice` |  |
| `pendingBelowType`= `STOP_LOSS` | `pendingBelowStopPrice` and/or `pendingBelowTrailingDelta` |  |
| `pendingBelowType`=`STOP_LOSS_LIMIT` | `pendingBelowPrice`, `pendingBelowStopPrice` and/or `pendingBelowTrailingDelta`, `pendingBelowTimeInForce` |  |

## Response Example

```javascript
{
    "orderListId": 13509,
    "contingencyType": "OTO",
    "listStatusType": "EXEC_STARTED",
    "listOrderStatus": "EXECUTING",
    "listClientOrderId": "u2AUo48LLef5qVenRtwJZy",
    "transactionTime": 1725521881300,
    "symbol": "BNBUSDT",
    "isIsolated": false,
    "orders": [
        {
            "symbol": "BNBUSDT",
            "orderId": 28282534,
            "clientOrderId": "IfYDxvrZI4kiyqYpRH13iI"
        },
        {
            "symbol": "BNBUSDT",
            "orderId": 28282535,
            "clientOrderId": "0HCSsPRxVfW8BkTUy9z4np"
        },
        {
            "symbol": "BNBUSDT",
            "orderId": 28282536,
            "clientOrderId": "dypsgdxWnLY75kwT930cbD"
        }
    ],
    "orderReports": [
        {
            "symbol": "BNBUSDT",
            "orderId": 28282534,
            "orderListId": 13509,
            "clientOrderId": "IfYDxvrZI4kiyqYpRH13iI",
            "transactTime": 1725521881300,
            "price": "300.00000000",
            "origQty": "1.00000000",
            "executedQty": "0",
            "cummulativeQuoteQty": "0",
            "status": "NEW",
            "timeInForce": "GTC",
            "type": "LIMIT",
            "side": "BUY",
            "selfTradePreventionMode": "NONE"
        },
        {
            "symbol": "BNBUSDT",
            "orderId": 28282535,
            "orderListId": 13509,
            "clientOrderId": "0HCSsPRxVfW8BkTUy9z4np",
            "transactTime": 1725521881300,
            "price": "0E-8",
            "origQty": "1.00000000",
            "executedQty": "0",
            "cummulativeQuoteQty": "0",
            "status": "PENDING_NEW",
            "timeInForce": "GTC",
            "type": "STOP_LOSS",
            "side": "SELL",
            "stopPrice": "299.00000000",
            "selfTradePreventionMode": "NONE"
        },
        {
            "symbol": "BNBUSDT",
            "orderId": 28282536,
            "orderListId": 13509,
            "clientOrderId": "dypsgdxWnLY75kwT930cbD",
            "transactTime": 1725521881300,
            "price": "301.00000000",
            "origQty": "1.00000000",
            "executedQty": "0",
            "cummulativeQuoteQty": "0",
            "status": "PENDING_NEW",
            "timeInForce": "GTC",
            "type": "LIMIT_MAKER",
            "side": "SELL",
            "selfTradePreventionMode": "NONE"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Account New Order (TRADE)

## API Description

Post a new order for margin account.

## HTTP Request

POST `/sapi/v1/margin/order`

## Request Weight

**6(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| side | ENUM | YES | BUY<br>SELL |
| type | ENUM | YES |  |
| quantity | DECIMAL | NO |  |
| quoteOrderQty | DECIMAL | NO |  |
| price | DECIMAL | NO |  |
| stopPrice | DECIMAL | NO | Used with `STOP_LOSS`, `STOP_LOSS_LIMIT`, `TAKE_PROFIT`, and `TAKE_PROFIT_LIMIT` orders. |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. |
| icebergQty | DECIMAL | NO | Used with `LIMIT`, `STOP_LOSS_LIMIT`, and `TAKE_PROFIT_LIMIT` to create an iceberg order. |
| newOrderRespType | ENUM | NO | Set the response JSON. ACK, RESULT, or FULL; MARKET and LIMIT order types default to FULL, all other orders default to ACK. |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY, AUTO\_REPAY,AUTO\_BORROW\_REPAY; default NO\_SIDE\_EFFECT. More info in [FAQ](https://www.binance.com/en/support/faq/how-to-use-the-sideeffecttype-parameter-with-the-margin-order-endpoints-f9fc51cda1984bf08b95e0d96c4570bc) |
| timeInForce | ENUM | NO | GTC,IOC,FOK |
| selfTradePreventionMode | ENUM | NO | The allowed enums is dependent on what is configured on the symbol. The possible supported values are EXPIRE\_TAKER, EXPIRE\_MAKER, EXPIRE\_BOTH, NONE |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY or AUTO\_BORROW\_REPAY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- autoRepayAtCancel is suggested to set as “FALSE” to keep liability unrepaid under high frequent new order/cancel order execution

## Response Example

> Response ACK:

```javascript
{
  "symbol": "BTCUSDT",
  "orderId": 28,
  "clientOrderId": "6gCrw2kRUAF9CvJDGP16IP",
  "isIsolated": true,       // if isolated margin
  "transactTime": 1507725176595

}
```

> Response RESULT:

```javascript
{
    "symbol": "BTCUSDT",
    "orderId": 26769564559,
    "clientOrderId": "E156O3KP4gOif65bjuUK5V",
    "transactTime": 1713873075893,
    "price": "0",
    "origQty": "0.001",
    "executedQty": "0.001",
    "cummulativeQuoteQty": "65982.53",
    "status": "FILLED",
    "timeInForce": "GTC",
    "type": "MARKET",
    "side": "SELL",
    "isIsolated": false,   // if isolated margin
    "selfTradePreventionMode": "EXPIRE_MAKER"
}
```

> Response FULL:

```javascript
{
  "symbol": "BTCUSDT",
  "orderId": 26769564559,
  "clientOrderId": "E156O3KP4gOif65bjuUK5V",
  "transactTime": 1713873075893,
  "price": "0",
  "origQty": "0.001",
  "executedQty": "0.001",
  "cummulativeQuoteQty": "65.98253",
  "status": "FILLED",
  "timeInForce": "GTC",
  "type": "MARKET",
  "side": "SELL",
  "marginBuyBorrowAmount": 5,       // will not return if no margin trade happens
  "marginBuyBorrowAsset": "BTC",    // will not return if no margin trade happens
  "isIsolated": true,       // if isolated margin
  "selfTradePreventionMode": "NONE",
  "fills": [
        {
            "price": "65982.53",
            "qty": "0.001",
            "commission": "0.06598253",
            "commissionAsset": "USDT",
            "tradeId": 3570680726
        }
    ],
  "isIsolated": false,
  "selfTradePreventionMode": "EXPIRE_MAKER"
}
```

**Error Code Description:**

- **ASSET\_BAN\_TRADE**

This error {“code”: -3067, “msg”: “This asset is currently not a supported margin asset, please try another asset.”} indicates that the asset is currently restricted. This restriction can be due to various reasons, such as the asset may be subject to regulatory restrictions that prevent it from being borrowed, etc.

You can verify if there are any announcements or updates regarding the asset's borrowing status on Binance's official channels.

- **NOT\_VALID\_MARGIN\_ASSET**

This error {“code”: -3027, “msg”: “Not a valid margin asset.”} occurs when a user requests an asset that is either delisted or is not supported on the margin product. Users can check the margin symbol info ( [GET /sapi/v1/margin/allAssets](https://developers.binance.com/docs/margin_trading/market-data/Get-All-Margin-Assets)) to find all supported margin assets before trading.

- **BALANCE\_NOT\_CLEARED**

This error {“code”: -3041, “msg”: “Balance is not enough.”} indicates that your account balance is insufficient to complete the requested transaction.

- **TOO\_MANY\_ORDERS**

This error {“code”: -1015, “msg”: “Too many new orders; current limit is %s orders per %s.”} means that you have reached the limit for the number of orders you can place within a certain timeframe. To address this issue:
  - Review Open Orders: Check your current open orders and consider canceling any unnecessary ones to free up capacity.
  - Space Out Orders: If possible, space out your order placements to prevent hitting the limit.
- **Filter failure: NOTIONAL**

This error {“code”:-20204, “msg”: “Filter failure: NOTIONAL.”} occurs when your request is blocked before reaching the Matching Engine, often due to the order value not meeting the minimum notional value requirement. By carefully reviewing your order request, you can identify and correct the issues causing the request rejection.

- **NOT\_VALID\_MARGIN\_PAIR**

This error {“code”: -3028, “msg”: “Not a valid margin pair.”} occurs when a user requests an asset that is either delisted or is not supported on the margin product. Users can check the margin symbol info (GET /sapi/v1/margin/allAssets) to find all supported margin assets before trading.

- **NEW\_ORDER\_REJECTED**

This error {“code”: -2010, “msg”: “NEW\_ORDER\_REJECTED”} often occurs for two reasons:
  - When a limit order is placed at a price that would immediately execute as a market order. You can adjust your limit order price to ensure it does not match the current market price if you intend to avoid taker fees.
  - Your account does not have enough funds to cover the order. You can resolve this by transferring additional funds if necessary or reduce the order size to fit your available balance.
- **EXCEED\_PRICE\_LIMIT**

This error {“code”: -3064, “msg”: “Limit price needs to be within (-15%,15%) of current index price for this margin trading pair.”} often occurs when the limit price is not allowed. For certain low liquidity pairs or stablecoin to stablecoin pairs on Margin (e.g. USDT/DAI), there will be a price bracket of \[-15%, 15%\] (which is subject to changes).

That is, when a BUY Margin order’s limit price is more than 15% higher than the current index price or a SELL Margin order’s limit price is more than 15% lower than the current index price, it will trigger this error message.

 

Copyright © 2026 Binance.

---

Margin Trading

# Margin Manual Liquidation(MARGIN)

## API Description

Margin Manual Liquidation

## HTTP Request

POST `/sapi/v1/margin/manual-liquidation`

## Request Weight(UID)

**3000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | STRING | YES | `MARGIN`,`ISOLATED` |
| symbol | STRING | NO | When type selects `ISOLATED`, `symbol` must be filled in |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional notes:

- This endpoint can support Cross Margin Classic Mode and Pro Mode.
- And only support Isolated Margin for restricted region.

## Response Example

```javascript
{
  "asset": "ETH",
  "interest": "0.00083334",
  "principal": "0.001",
  "liabilityAsset": "USDT",
  "liabilityQty": 0.3552
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Current Margin Order Count Usage (TRADE)

## API Description

Displays the user's current margin order count usage for all intervals.

## HTTP Request

GET `/sapi/v1/margin/rateLimit/order`

## Request Weight

**20(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| symbol | STRING | NO | isolated symbol, mandatory for isolated margin |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[

  {
    "rateLimitType": "ORDERS",
    "interval": "SECOND",
    "intervalNum": 10,
    "limit": 10000,
    "count": 0
  },
  {
    "rateLimitType": "ORDERS",
    "interval": "DAY",
    "intervalNum": 1,
    "limit": 20000,
    "count": 0
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's all OCO (USER\_DATA)

## API Description

Retrieves all OCO for a specific margin account based on provided optional parameters

## HTTP Request

GET `/sapi/v1/margin/allOrderList`

## Request Weight

**200(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| symbol | STRING | NO | mandatory for isolated margin, not supported for cross margin |
| fromId | LONG | NO | If supplied, neither `startTime` or `endTime` can be provided |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default Value: 500; Max Value: 1000 |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "orderListId": 29,
    "contingencyType": "OCO",
    "listStatusType": "EXEC_STARTED",
    "listOrderStatus": "EXECUTING",
    "listClientOrderId": "amEEAXryFzFwYF1FeRpUoZ",
    "transactionTime": 1565245913483,
    "symbol": "LTCBTC",
    "isIsolated": true,       // if isolated margin
    "orders": [
      {
        "symbol": "LTCBTC",
        "orderId": 4,
        "clientOrderId": "oD7aesZqjEGlZrbtRpy5zB"
      },
      {
        "symbol": "LTCBTC",
        "orderId": 5,
        "clientOrderId": "Jr1h6xirOxgeJOUuYQS7V3"
      }
    ]
  },
  {
    "orderListId": 28,
    "contingencyType": "OCO",
    "listStatusType": "EXEC_STARTED",
    "listOrderStatus": "EXECUTING",
    "listClientOrderId": "hG7hFNxJV6cZy3Ze4AUT4d",
    "transactionTime": 1565245913407,
    "symbol": "LTCBTC",
    "orders": [
      {
        "symbol": "LTCBTC",
        "orderId": 2,
        "clientOrderId": "j6lFOfbmFMRjTYA7rRJ0LP"
      },
      {
        "symbol": "LTCBTC",
        "orderId": 3,
        "clientOrderId": "z0KCjOdditiLS5ekAFtK81"
      }
    ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's All Orders (USER\_DATA)

## API Description

Query Margin Account's All Orders

## HTTP Request

GET `/sapi/v1/margin/allOrders`

## Request Weight

**200(IP)**

## Request Limit

**60times/min per IP**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 500. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- If orderId is set, it will get orders >= that orderId. Otherwise the orders within 24 hours are returned.
- For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.
- Less than 24 hours between startTime and endTime.

## Response Example

```javascript
[
      {
          "clientOrderId": "D2KDy4DIeS56PvkM13f8cP",
          "cummulativeQuoteQty": "0.00000000",
          "executedQty": "0.00000000",
          "icebergQty": "0.00000000",
          "isWorking": false,
          "orderId": 41295,
          "origQty": "5.31000000",
          "price": "0.22500000",
          "side": "SELL",
          "status": "CANCELED",
          "stopPrice": "0.18000000",
          "symbol": "BNBBTC",
          "isIsolated": false,
          "time": 1565769338806,
          "timeInForce": "GTC",
          "type": "TAKE_PROFIT_LIMIT",
          "selfTradePreventionMode": "NONE",
          "updateTime": 1565769342148
      },
      {
          "clientOrderId": "gXYtqhcEAs2Rn9SUD9nRKx",
          "cummulativeQuoteQty": "0.00000000",
          "executedQty": "0.00000000",
          "icebergQty": "1.00000000",
          "isWorking": true,
          "orderId": 41296,
          "origQty": "6.65000000",
          "price": "0.18000000",
          "side": "SELL",
          "status": "CANCELED",
          "stopPrice": "0.00000000",
          "symbol": "BNBBTC",
          "isIsolated": false,
          "time": 1565769348687,
          "timeInForce": "GTC",
          "type": "LIMIT",
          "selfTradePreventionMode": "NONE",
          "updateTime": 1565769352226
      }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's OCO (USER\_DATA)

## API Description

Retrieves a specific OCO based on provided optional parameters

## HTTP Request

GET `/sapi/v1/margin/orderList`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| symbol | STRING | NO | mandatory for isolated margin, not supported for cross margin |
| orderListId | LONG | NO | Either `orderListId` or `origClientOrderId` must be provided |
| origClientOrderId | STRING | NO | Either `orderListId` or `origClientOrderId` must be provided |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "orderListId": 27,
  "contingencyType": "OCO",
  "listStatusType": "EXEC_STARTED",
  "listOrderStatus": "EXECUTING",
  "listClientOrderId": "h2USkA5YQpaXHPIrkd96xE",
  "transactionTime": 1565245656253,
  "symbol": "LTCBTC",
  "isIsolated": false,       // if isolated margin
  "orders": [
    {
      "symbol": "LTCBTC",
      "orderId": 4,
      "clientOrderId": "qD1gy3kc3Gx0rihm9Y3xwS"
    },
    {
      "symbol": "LTCBTC",
      "orderId": 5,
      "clientOrderId": "ARzZ9I00CPM8i3NhmU9Ega"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's Open OCO (USER\_DATA)

## API Description

Query Margin Account's Open OCO

## HTTP Request

GET `/sapi/v1/margin/openOrderList`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| symbol | STRING | NO | mandatory for isolated margin, not supported for cross margin |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "orderListId": 31,
    "contingencyType": "OCO",
    "listStatusType": "EXEC_STARTED",
    "listOrderStatus": "EXECUTING",
    "listClientOrderId": "wuB13fmulKj3YjdqWEcsnp",
    "transactionTime": 1565246080644,
    "symbol": "LTCBTC",
    "isIsolated": false,       // if isolated margin
    "orders": [
      {
        "symbol": "LTCBTC",
        "orderId": 4,
        "clientOrderId": "r3EH2N76dHfLoSZWIUw1bT"
      },
      {
        "symbol": "LTCBTC",
        "orderId": 5,
        "clientOrderId": "Cv1SnyPD3qhqpbjpYEHbd2"
      }
    ]
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's Open Orders (USER\_DATA)

## API Description

Query Margin Account's Open Orders

## HTTP Request

GET `/sapi/v1/margin/openOrders`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- If the symbol is not sent, orders for all symbols will be returned in an array.
- When all symbols are returned, the number of requests counted against the rate limiter is equal to the number of symbols currently trading on the exchange.
- If isIsolated ="TRUE", symbol must be sent.

## Response Example

```javascript
[
   {
       "clientOrderId": "qhcZw71gAkCCTv0t0k8LUK",
       "cummulativeQuoteQty": "0.00000000",
       "executedQty": "0.00000000",
       "icebergQty": "0.00000000",
       "isWorking": true,
       "orderId": 211842552,
       "origQty": "0.30000000",
       "price": "0.00475010",
       "side": "SELL",
       "status": "NEW",
       "stopPrice": "0.00000000",
       "symbol": "BNBBTC",
       "isIsolated": true,
       "time": 1562040170089,
       "timeInForce": "GTC",
       "type": "LIMIT",
       "selfTradePreventionMode": "NONE",
       "updateTime": 1562040170089
	}
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's Order (USER\_DATA)

## API Description

Query Margin Account's Order

## HTTP Request

GET `/sapi/v1/margin/order`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Either orderId or origClientOrderId must be sent.
- For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.

## Response Example

```javascript
{
   "clientOrderId": "ZwfQzuDIGpceVhKW5DvCmO",
   "cummulativeQuoteQty": "0.00000000",
   "executedQty": "0.00000000",
   "icebergQty": "0.00000000",
   "isWorking": true,
   "orderId": 213205622,
   "origQty": "0.30000000",
   "price": "0.00493630",
   "side": "SELL",
   "status": "NEW",
   "stopPrice": "0.00000000",
   "symbol": "BNBBTC",
   "isIsolated": true,
   "time": 1562133008725,
   "timeInForce": "GTC",
   "type": "LIMIT",
   "selfTradePreventionMode": "NONE",
   "updateTime": 1562133008725
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Margin Account's Trade List (USER\_DATA)

## API Description

Query Margin Account's Trade List

## HTTP Request

GET `/sapi/v1/margin/myTrades`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| isIsolated | STRING | NO | for isolated margin or not, "TRUE", "FALSE"，default "FALSE" |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | TradeId to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- If fromId is set, it will get trades >= that fromId. Otherwise the trades within 24 hours are returned.
- Less than 24 hours between startTime and endTime.

## Response Example

```javascript
[
	{
		"commission": "0.00006000",
		"commissionAsset": "BTC",
		"id": 34,
		"isBestMatch": true,
		"isBuyer": false,
		"isMaker": false,
		"orderId": 39324,
		"price": "0.02000000",
		"qty": "3.00000000",
		"symbol": "BNBBTC",
		"isIsolated": false,
		"time": 1561973357171
	}
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Prevented Matches(USER\_DATA)

## Description

Displays the list of orders that were expired due to STP. (Self-Trade Prevention).

## HTTP Request

GET `/sapi/v1/margin/myPreventedMatches`

## Request Weight

**10(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| preventedMatchId | LONG | NO |  |
| orderId | LONG | NO |  |
| fromPreventedMatchId | LONG | NO |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000. Supports up to three decimal places of precision (e.g., 6000.346) so that microseconds may be specified. |
| timestamp | LONG | YES |  |
| isIsolated | STRING | NO | For isolated margin or not, "TRUE", "FALSE", default "FALSE" |

- Supported parameter combinations:
  - `symbol` \+ `preventedMatchId`
  - `symbol` \+ `orderId`
  - `symbol` \+ `orderId` \+ `fromPreventedMatchId`
- If `orderId` is provided, all prevented matches for that order will be returned.
- If `preventedMatchId` is provided, the specific prevented match will be returned.
- A single request returns a maximum of 500 records. If there are more than 500 records, use `symbol` \+ `orderId` \+ `fromPreventedMatchId` combination for pagination.

## Data Source

Database

## Response Example

```javascript
[
    {
        "symbol": "BTCUSDT",
        "preventedMatchId": 1,
        "takerOrderId": 5,
        "makerSymbol": "BTCUSDT",
        "makerOrderId": 3,
        "tradeGroupId": 1,
        "selfTradePreventionMode": "EXPIRE_MAKER",
        "price": "1.100000",
        "makerPreventedQuantity": "1.300000",
        "transactTime": 1669101687094
    }
]
```

## Response Parameters

| Name | Type | Description |
| --- | --- | --- |
| symbol | STRING | The trading pair symbol |
| preventedMatchId | LONG | Unique identifier for the prevented match event |
| takerOrderId | LONG | The order ID of the taker order that triggered STP |
| makerSymbol | STRING | The symbol of the maker order |
| makerOrderId | LONG | The order ID of the maker order involved in STP |
| tradeGroupId | LONG | Identifier grouping related prevented matches |
| selfTradePreventionMode | STRING | The STP mode applied. Possible values: `EXPIRE_TAKER`, `EXPIRE_MAKER`, `EXPIRE_BOTH` |
| price | STRING | The price at which the match would have occurred |
| makerPreventedQuantity | STRING | The quantity that was prevented from being filled on the maker side |
| transactTime | LONG | Unix timestamp (milliseconds) when the prevention occurred |

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Special key List(Low Latency Trading)(TRADE)

## API Description

This only applies to Special Key for Low Latency Trading.

## HTTP Request

GET `/sapi/v1/margin/api-key-list`

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO | isolated margin pair |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "apiName": "testName1",
    "apiKey":"znpOzOAeLVgr2TuxWfNo43AaPWpBbJEoKezh1o8mSQb6ryE2odE11A4AoVlJbQoG",
    "ip": "192.168.0.1,192.168.0.2",
    "type": "RSA",
     "permissionMode": "TRADE"
  },
  {
    "apiName": "testName2",
    "apiKey":"znpOzOAeLVgr2TuxWfNo43AaPWpBbJEoKezh1o8mSQb6ryE2odE11A4AoVlJbQoG",
    "ip": "192.168.0.1,192.168.0.2",
    "type": "Ed25519",
     "permissionMode": "READ"
  }
]
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Special key(Low Latency Trading)(TRADE)

## API Description

Query Special Key Information.

This only applies to Special Key for Low Latency Trading.

## HTTP Request

GET `/sapi/v1/margin/apiKey`

## Request Weight

**1(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| apiKey | STRING | YES |  |
| symbol | STRING | NO | isolated margin pair |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"apiKey":"npOzOAeLVgr2TuxWfNo43AaPWpBbJEoKezh1o8mSQb6ryE2odE11A4AoVlJbQoGx",
  "ip": "0.0.0.0,192.168.0.1,192.168.0.2", // 0.0.0.0 is just an initial statereference (no extra meaning).
  "apiName": "testName",
  "type": "RSA",
  "permissionMode": "TRADE"
}
```

 

Copyright © 2026 Binance.

---

Margin Trading

# Small Liability Exchange (MARGIN)

## API Description

Small Liability Exchange

## HTTP Request

POST `/sapi/v1/margin/exchange-small-liability`

## Request Weight

**3000(UID)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| assetNames | ARRAY | YES | The assets list of small liability exchange， Example: assetNames = BTC,ETH |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- Only convert once within 6 hours
- Only liability valuation less than 10 USDT are supported
- The maximum number of coin is 10

## Response Example

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Cross Margin Transfer History (USER\_DATA)

## API Description

Get Cross Margin Transfer History

## HTTP Request

GET `/sapi/v1/margin/transfer`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| type | STRING | NO | Transfer Type: ROLL\_IN, ROLL\_OUT |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| isolatedSymbol | STRING | NO | Symbol in Isolated Margin |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Response in descending order
- The max interval between `startTime` and `endTime` is 30 days.
- Returns data for last 7 days by default

## Response Example

```javascript
{
  "rows": [
    {
      "amount": "0.10000000",
      "asset": "BNB",
      "status": "CONFIRMED",
      "timestamp": 1566898617,
      "txId": 5240372201,
      "type": "ROLL_IN",
      "transFrom": "SPOT", //SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "transTo": "ISOLATED_MARGIN",//SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
    },
    {
      "amount": "5.00000000",
      "asset": "USDT",
      "status": "CONFIRMED",
      "timestamp": 1566888436,
      "txId": 5239810406,
      "type": "ROLL_OUT",
      "transFrom": "ISOLATED_MARGIN",//SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "transTo": "ISOLATED_MARGIN", //SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "fromSymbol": "BNBUSDT",
      "toSymbol": "BTCUSDT"
    },
    {
      "amount": "1.00000000",
      "asset": "EOS",
      "status": "CONFIRMED",
      "timestamp": 1566888403,
      "txId": 5239808703,
      "type": "ROLL_IN"
    }
  ],
  "total": 3
}
```

**Error Code Description:**

- **EXCEED\_MAX\_ROLLOUT**

Sometimes, your collateral margin level may be too low to allow a transfer out of your account. You will get an error response {"code":-3020,"msg":"Transfer out amount exceeds max amount."}. To resolve it, you can reduce your outstanding debt or add more assets to meet the required margin level for the transfer.

- **PREPAREDELIST\_CANT\_TRANSFER\_IN**

This error {“code”: -3065, “msg”: “%s has been scheduled for delisting. You may only transfer up to %s %s, which is the amount of liabilities less any collateral already available.”} indicates that a specific asset is planned to be delisted. As a result, there are restrictions on how much of this asset you can transfer out of your account. When transferring the asset out of Binance, you will not be able to exceed the allowed amount

- **NET\_ASSET\_MUST\_LTE\_RATIO**

This error {“code”:-21003, “msg”: ”Fail to retrieve margin assets.”} typically occurs when users send requests at a very high frequency. Because asset information updates need processing time, sending requests too frequently can cause failures or delayed responses.

We recommend that users maintain at least a 500 milliseconds (0.5 seconds) interval between each request. This interval allows the system enough time to process and update asset information, reducing errors or delays caused by high-frequency requests.

 

Copyright © 2026 Binance.

---

Margin Trading

# Get Cross Margin Transfer History (USER\_DATA)

## API Description

Get Cross Margin Transfer History

## HTTP Request

GET `/sapi/v1/margin/transfer`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| type | STRING | NO | Transfer Type: ROLL\_IN, ROLL\_OUT |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| isolatedSymbol | STRING | NO | Symbol in Isolated Margin |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- Response in descending order
- The max interval between `startTime` and `endTime` is 30 days.
- Returns data for last 7 days by default

## Response Example

```javascript
{
  "rows": [
    {
      "amount": "0.10000000",
      "asset": "BNB",
      "status": "CONFIRMED",
      "timestamp": 1566898617,
      "txId": 5240372201,
      "type": "ROLL_IN",
      "transFrom": "SPOT", //SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "transTo": "ISOLATED_MARGIN",//SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
    },
    {
      "amount": "5.00000000",
      "asset": "USDT",
      "status": "CONFIRMED",
      "timestamp": 1566888436,
      "txId": 5239810406,
      "type": "ROLL_OUT",
      "transFrom": "ISOLATED_MARGIN",//SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "transTo": "ISOLATED_MARGIN", //SPOT,FUTURES,FIAT,DELIVERY,MINING,ISOLATED_MARGIN,FUNDING,MOTHER_SPOT,OPTION,SUB_SPOT,SUB_MARGIN,CROSS_MARGIN
      "fromSymbol": "BNBUSDT",
      "toSymbol": "BTCUSDT"
    },
    {
      "amount": "1.00000000",
      "asset": "EOS",
      "status": "CONFIRMED",
      "timestamp": 1566888403,
      "txId": 5239808703,
      "type": "ROLL_IN"
    }
  ],
  "total": 3
}
```

**Error Code Description:**

- **EXCEED\_MAX\_ROLLOUT**

Sometimes, your collateral margin level may be too low to allow a transfer out of your account. You will get an error response {"code":-3020,"msg":"Transfer out amount exceeds max amount."}. To resolve it, you can reduce your outstanding debt or add more assets to meet the required margin level for the transfer.

- **PREPAREDELIST\_CANT\_TRANSFER\_IN**

This error {“code”: -3065, “msg”: “%s has been scheduled for delisting. You may only transfer up to %s %s, which is the amount of liabilities less any collateral already available.”} indicates that a specific asset is planned to be delisted. As a result, there are restrictions on how much of this asset you can transfer out of your account. When transferring the asset out of Binance, you will not be able to exceed the allowed amount

- **NET\_ASSET\_MUST\_LTE\_RATIO**

This error {“code”:-21003, “msg”: ”Fail to retrieve margin assets.”} typically occurs when users send requests at a very high frequency. Because asset information updates need processing time, sending requests too frequently can cause failures or delayed responses.

We recommend that users maintain at least a 500 milliseconds (0.5 seconds) interval between each request. This interval allows the system enough time to process and update asset information, reducing errors or delays caused by high-frequency requests.

 

Copyright © 2026 Binance.

---

Margin Trading

# Query Max Transfer-Out Amount (USER\_DATA)

## API Description

Query Max Transfer-Out Amount

## HTTP Request

GET `/sapi/v1/margin/maxTransferable`

## Request Weight

**50(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| isolatedSymbol | STRING | NO | isolated symbol |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

- If isolatedSymbol is not sent, crossed margin data will be sent.

## Response Example

```javascript
 {
      "amount": "3.59498107"
 }
```

 

Copyright © 2026 Binance.

---

