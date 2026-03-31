# Bybit Account API

Source: https://bybit-exchange.github.io/docs/v5/account/

---

# Get Account Info

Query the account information, like margin mode, account mode, etc.

### HTTP Request

GET`/v5/account/info`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| [unifiedMarginStatus](https://bybit-exchange.github.io/docs/v5/enum#unifiedmarginstatus) | integer | Account status |
| marginMode | string | `ISOLATED_MARGIN`, `REGULAR_MARGIN`, `PORTFOLIO_MARGIN` |
| isMasterTrader | boolean | Whether this account is a leader (copytrading). `true`, `false` |
| spotHedgingStatus | string | Whether the unified account enables Spot hedging. `ON`, `OFF` |
| updatedTime | string | Account data updated timestamp (ms) |
| dcpStatus | string | deprecated, always `OFF`. Please use [Get DCP Info](https://bybit-exchange.github.io/docs/v5/account/dcp-info) |
| timeWindow | integer | deprecated, always `0`. Please use [Get DCP Info](https://bybit-exchange.github.io/docs/v5/account/dcp-info) |
| smpGroup | integer | deprecated, always `0`. Please query [Get SMP Group ID](https://bybit-exchange.github.io/docs/v5/account/smp-group) endpoint |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/account-info)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/info HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672129307221
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_account_info())
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getAccountInfo()
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
        "marginMode": "REGULAR_MARGIN",
        "updatedTime": "1697078946000",
        "unifiedMarginStatus": 4,
        "dcpStatus": "OFF",
        "timeWindow": 10,
        "smpGroup": 0,
        "isMasterTrader": false,
        "spotHedgingStatus": "OFF"
    }
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Batch Set Collateral Coin

### HTTP Request

POST`/v5/account/set-collateral-switch-batch`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| request | **true** | array | Object |
| \> coin | **true** | string | Coin name, uppercase only <br>- You can get collateral coin from [here](https://bybit-exchange.github.io/docs/v5/account/collateral-info)<br>- USDT, USDC cannot be set |
| \> collateralSwitch | **true** | string | `ON`: switch on collateral, `OFF`: switch off collateral |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | Object |  |
| \> list | array | Object |
| >\> coin | string | Coin name |
| >\> collateralSwitch | string | `ON`: switch on collateral, `OFF`: switch off collateral |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-collateral-switch-batch HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1704782042755
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 371

{
    "request": [
        {
            "coin": "MATIC",
            "collateralSwitch": "OFF"
        },
        {
            "coin": "BTC",
            "collateralSwitch": "OFF"
        },
        {
            "coin": "ETH",
            "collateralSwitch": "OFF"
        },
        {
            "coin": "SOL",
            "collateralSwitch": "OFF"
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
print(session.batch_set_collateral_coin(
  request=[
    {
      "coin": "BTC",
      "collateralSwitch": "ON",
    },
    {
      "coin": "ETH",
      "collateralSwitch": "ON",
    }
  ]
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .batchSetCollateralCoin({
    request: [
      {
        coin: 'BTC',
        collateralSwitch: 'ON',
      },
      {
        coin: 'ETH',
        collateralSwitch: 'OFF',
      },
    ],
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
    "result": {
        "list": [
            {
                "coin": "MATIC",
                "collateralSwitch": "OFF"
            },
            {
                "coin": "BTC",
                "collateralSwitch": "OFF"
            },
            {
                "coin": "ETH",
                "collateralSwitch": "OFF"
            },
            {
                "coin": "SOL",
                "collateralSwitch": "OFF"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1704782042913
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Manual Borrow

info

Borrowing via OpenAPI endpoint supports variable rate borrowing only.

### HTTP Request

POST`/v5/account/borrow`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | **true** | string | coin name, uppercase only |
| amount | **true** | string | Borrow amount |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array | Object |
| \> coin | string | coin name, uppercase only |
| \> amount | string | Borrow amount |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/borrow HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675842997277
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "coin":"BTC",
    "amount":"0.01"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.borrow(
    coin="BTC",
    amount="0.01"
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "coin": "BTC",
        "amount": "0.01"
    },
    "retExtInfo": {},
    "time": 1756197991955
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrow History

Get interest records, sorted in reverse order of creation time.

### HTTP Request

GET`/v5/account/borrow-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | `USDC`,`USDT`,`BTC`,`ETH` etc, uppercase only |
| startTime | false | integer | The start timestamp (ms) <br>- startTime and endTime are not passed, return 30 days by default<br>- Only startTime is passed, return range between startTime and startTime + 30 days <br>- Only endTime is passed, return range between endTime-30 days and endTime<br>- If both are passed, the rule is endTime - startTime <= 30 days |
| endTime | false | integer | The end time. timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `50`\]. Default: `20` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | `USDC`,`USDT`,`BTC`,`ETH` |
| \> createdTime | integer | Created timestamp (ms) |
| \> borrowCost | string | Interest |
| \> hourlyBorrowRate | string | Hourly Borrow Rate |
| \> InterestBearingBorrowSize | string | Interest Bearing Borrow Size |
| \> costExemption | string | Cost exemption |
| \> borrowAmount | string | Total borrow amount |
| \> unrealisedLoss | string | Unrealised loss |
| \> freeBorrowedAmount | string | The borrowed amount for interest free |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/borrow-history)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/borrow-history?currency=BTC&limit=1 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672277745427
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrow_history(
    currency="BTC",
    limit=1,
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .getBorrowHistory({
    currency: 'USDT',
    startTime: 1670601600000,
    endTime: 1673203200000,
    limit: 30,
    cursor: 'nextPageCursorToken',
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
        "nextPageCursor": "2671153%3A1%2C2671153%3A1",
        "list": [
            {
                "borrowAmount": "1.06333265702840778",
                "costExemption": "0",
                "freeBorrowedAmount": "0",
                "createdTime": 1697439900204,
                "InterestBearingBorrowSize": "1.06333265702840778",
                "currency": "BTC",
                "unrealisedLoss": "0",
                "hourlyBorrowRate": "0.000001216904",
                "borrowCost": "0.00000129"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1697442206478
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Coin Greeks

Get current account Greeks information

info

- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/asset/coin-greeks`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| baseCoin | false | string | Base coin, uppercase only. If not passed, all supported base coin greeks will be returned by default |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> baseCoin | string | Base coin. e.g.,`BTC`,`ETH`,`SOL` |
| \> totalDelta | string | Delta value |
| \> totalGamma | string | Gamma value |
| \> totalVega | string | Vega value |
| \> totalTheta | string | Theta value |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/coin-greeks)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/asset/coin-greeks?baseCoin=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672287887610
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_coin_greeks(
    baseCoin="BTC",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getCoinGreeks('BTC')
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
                "baseCoin": "BTC",
                "totalDelta": "0.00004001",
                "totalGamma": "-0.00000009",
                "totalVega": "-0.00039689",
                "totalTheta": "0.01243824"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672287887942
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Collateral Info

Get the collateral information of the current unified margin account, including loan interest rate, loanable amount,
collateral conversion rate, whether it can be mortgaged as margin, etc.

### HTTP Request

GET`/v5/account/collateral-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Asset currency of all current collateral, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | Currency of all current collateral |
| \> hourlyBorrowRate | string | Hourly borrow rate |
| \> maxBorrowingAmount | string | Max borrow amount. This value is shared across main-sub UIDs |
| \> freeBorrowingLimit | string | The maximum limit for interest-free borrowing <br>- Only the borrowing caused by contracts unrealised loss has interest-free amount<br>- Spot margin borrowing always has interest |
| \> freeBorrowAmount | string | The amount of borrowing within your total borrowing amount that is exempt from interest charges |
| \> borrowAmount | string | Borrow amount |
| \> otherBorrowAmount | string | The sum of borrowing amount for other accounts under the same main account |
| \> availableToBorrow | string | Available amount to borrow. This value is shared across main-sub UIDs |
| \> borrowable | boolean | Whether currency can be borrowed |
| \> borrowUsageRate | string | Borrow usage rate: sum of main & sub accounts borrowAmount/maxBorrowingAmount, it is an actual value, 0.5 means 50% |
| \> marginCollateral | boolean | Whether it can be used as a margin collateral currency (platform), `true`: YES, `false`: NO <br>- When marginCollateral=false, then collateralSwitch is meaningless |
| \> collateralSwitch | boolean | Whether the collateral is turned on by user (user), `true`: ON, `false`: OFF <br>- When marginCollateral=true, then collateralSwitch is meaningful |
| \> collateralRatio | string | **Deprecated** field. Due to the new Tiered Collateral value logic, this field will no longer be accurate starting on February 19, 2025. Please refer to [Get Tiered Collateral Ratio](https://bybit-exchange.github.io/docs/v5/spot-margin-uta/tier-collateral-ratio) |
| \> freeBorrowingAmount | string | **Deprecated** field, always return `""`, please refer to `freeBorrowingLimit` |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/collateral-info)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/collateral-info?currency=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672127952719
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_collateral_info(
    currency="BTC",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getCollateralInfo('BTC')
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
                "availableToBorrow": "3",
                "freeBorrowingAmount": "",
                "freeBorrowAmount": "0",
                "maxBorrowingAmount": "3",
                "hourlyBorrowRate": "0.00000147",
                "borrowUsageRate": "0",
                "collateralSwitch": true,
                "borrowAmount": "0",
                "borrowable": true,
                "currency": "BTC",
                "otherBorrowAmount": "0",
                "marginCollateral": true,
                "freeBorrowingLimit": "0",
                "collateralRatio": "0.95"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1691565901952
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get DCP Info

Query the DCP configuration of the account. Before calling the interface, please make sure you have applied for the UTA account DCP configuration with your account manager

- Only the configured main / sub account can query information from this API. Calling this API by an account always returns empty.

- If you only request to activate Spot trading for DCP, the contract and options data will not be returned.

info

- Support USDT Perpetuals, USDT Futures, USDC Perpetuals, USDC Futures, Inverse Perpetuals, Inverse Futures \[DERIVATIVES\]

Spot \[SPOT\]

Options \[OPTIONS\]

### HTTP Request

GET`/v5/account/query-dcp-info`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| dcpInfos | array<object> | DCP config for each product |
| \> product | string | `SPOT`, `DERIVATIVES`, `OPTIONS` |
| \> dcpStatus | string | [Disconnected-CancelAll-Prevention](https://bybit-exchange.github.io/docs/v5/order/dcp) status: `ON` |
| \> timeWindow | string | DCP trigger time window which user pre-set. Between \[3, 300\] seconds, default: 10 sec |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/query-dcp-info HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1717065530867
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.query_dcp_info())
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .getDCPInfo()
  .then((response) => {
    console.log(response);
  })
  .catch((error) => {
    console.error(error);
  });
```

### Response Example

```json
// it means my account enables Spot and Deriviatvies on the backend
// Options is not enabled with DCP
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "dcpInfos": [
            {
                "product": "SPOT",
                "dcpStatus": "ON",
                "timeWindow": "10"
            },
            {
                "product": "DERIVATIVES",
                "dcpStatus": "ON",
                "timeWindow": "10"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1717065531697
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Fee Rate

Get the trading fee rate.

### HTTP Request

GET`/v5/account/fee-rate`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| category | **true** | string | Product type. `spot`, `linear`, `inverse`, `option` |
| symbol | false | string | Symbol name, like `BTCUSDT`, uppercase only. Valid for `linear`, `inverse`, `spot` |
| baseCoin | false | string | Base coin, uppercase only. `SOL`, `BTC`, `ETH`. Valid for `option` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| category | string | Product type. `spot`, `option`. _Derivatives does not have this field_ |
| list | array | Object |
| \> symbol | string | Symbol name. Keeps `""` for Options |
| \> baseCoin | string | Base coin. `SOL`, `BTC`, `ETH`<br>- Spot and Derivatives does not have this field |
| \> takerFeeRate | string | Taker fee rate |
| \> makerFeeRate | string | Maker fee rate |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/fee-rate)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/fee-rate?symbol=ETHUSDT HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1676360412362
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_fee_rates(
    symbol="ETHUSDT",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getFeeRate({
        category: 'linear',
        symbol: 'ETHUSDT',
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
                "symbol": "ETHUSDT",
                "takerFeeRate": "0.0006",
                "makerFeeRate": "0.0001"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1676360412576
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get MMP State

### HTTP Request

GET`/v5/account/mmp-state`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| baseCoin | **true** | string | Base coin, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array | Object |
| \> baseCoin | string | Base coin |
| \> mmpEnabled | boolean | Whether the account is enabled mmp |
| \> window | string | Time window (ms) |
| \> frozenPeriod | string | Frozen period (ms) |
| \> qtyLimit | string | Trade qty limit |
| \> deltaLimit | string | Delta limit |
| \> mmpFrozenUntil | string | Unfreeze timestamp (ms) |
| \> mmpFrozen | boolean | Whether the mmp is triggered. <br>- `true`: mmpFrozenUntil is meaningful<br>- `false`: please ignore the value of mmpFrozenUntil |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/mmp-reset HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675842997277
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "baseCoin": "ETH"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_mmp_state(
    baseCoin="ETH",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getMMPState('ETH')
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
        "result": [
            {
                "baseCoin": "BTC",
                "mmpEnabled": true,
                "window": "5000",
                "frozenPeriod": "100000",
                "qtyLimit": "0.01",
                "deltaLimit": "0.01",
                "mmpFrozenUntil": "1675760625519",
                "mmpFrozen": false
            }
        ]
    },
    "retExtInfo": {},
    "time": 1675843188984
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Trade Behaviour Config

You can get configuration how the system behaves when your limit order price exceeds the highest bid or lowest ask price. The response also includes whether [Delta Neutral mode](https://bybit-exchange.github.io/docs/v5/account/set-delta-mode) is enabled.

* * *

Where x% is [priceLimitRatioX](https://bybit-exchange.github.io/docs/v5/market/instrument); and y% is the [priceLimitRatioY](https://bybit-exchange.github.io/docs/v5/market/instrument):

Spot

- **Maximum buy price**: _Min\[Max(Index Price, Index Price × (1 + x%) + 2-Minute Average Premium), Index Price × (1 + y%)\]_
- **Minimum sell price**: _Max\[Min(Index Price, Index Price × (1 – x%) + 2-Minute Average Premium), Index Price × (1 – y%)\]_

Futures

- **Maximum buy price**: _Min (Max (Index Price, Mark Price × (1 + x%)), Mark Price × (1 + y%))_
- **Minimum sell price**: _Max (Min (Index Price, Mark Price × (1 - x%)), Mark Price × (1 - y%))_

Default Setting

- Spot:
**lpaSpot = false.** If the order price exceeds the limit, the system rejects the request.

- Futures:
**lpaPerp = false.** If the order price exceeds the limit, the system will automatically adjust the price to the nearest allowed price (i.e., highest bid or lowest ask).

### HTTP Request

GET`/v5/account/user-setting-config`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array | Object |
| \> lpaSpot | boolean | - `true`: If the order price exceeds the limit, the system will automatically adjust the price to the nearest allowed price<br>- `false`: If the order price exceeds the limit, the system rejects the request. |
| \> lpaPerp | boolean | - `true`: If the order price exceeds the limit, the system rejects the request.<br>- `false`: If the order price exceeds the limit, the system will automatically adjust the price to the nearest allowed price. |
| \> deltaEnable | boolean | Whether [Delta Neutral mode](https://bybit-exchange.github.io/docs/v5/account/set-delta-mode) is enabled. `true`: enabled; `false`: disabled |
| \> smsef | boolean | Whether Spot MNT fee deduction is enabled. `true`: enabled; `false`: disabled |
| \> fmsef | boolean | Whether Futures MNT fee deduction is enabled. `true`: enabled; `false`: disabled |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/user-setting-config HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1753255927950
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 52
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_user_setting_config())
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "lpaSpot": false,
        "lpaPerp": false,
        "smsef": false,
        "fmsef": false
    },
    "retExtInfo": {},
    "time": 1773234932707
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Account Instruments Info

Query for the instrument specification of online trading pairs that available to users.

> **Covers: Spot / USDT contract / USDC contract / Inverse contract**

caution

- Spot does not support pagination, so `limit`, `cursor` are invalid.
- This endpoint returns 200 entries by default. There are now more than 200 `linear` symbols on the platform. As a result, you will need to use `cursor` for pagination or `limit` to get all entries.
- Custodial sub-accounts do not support queries.
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery
- The fields `maxLimitOrderQty`, `maxMarketOrderQty`, and `postOnlyMaxLimitOrderSize` are adjusted bi-monthly (3rd and 17th, 08:00 UTC+8). Developers should not assume these values remain constant.

### HTTP Request

GET`/v5/account/instruments-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | **true** | string | Product type. `spot`,`linear`,`inverse` |
| [symbol](https://bybit-exchange.github.io/docs/v5/enum#symbol) | false | string | Symbol name, like `BTCUSDT`, uppercase only |
| limit | false | integer | Limit for data size per page. \[`1`, `200`\]. Default: `200` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

- Linear/Inverse
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
| \> isPublicRpi | boolean | Whether RPI Is Openly Provided to Market Makers or not.<br>- true: RPI Is Openly Provided to Market Makers<br>- false: RPI Is Not Openly Provided to Market Makers |
| \> myRpiPermission | boolean | Whether the Current User Has RPI Permissions or not<br>- true: Has RPI Permissions<br>- false: Does Not Have RPI Permissions |

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
| \> [marginTrading](https://bybit-exchange.github.io/docs/v5/enum#margintrading) | string | Margin trade symbol or not <br>- This is to identify if the symbol support margin trading under different account modes<br>- You may find some symbols not supporting margin buy or margin sell, so you need to go to [Collateral Info (UTA)](https://bybit-exchange.github.io/docs/v5/account/collateral-info) to check if that coin is borrowable |
| \> stTag | string | Whether or not it has an [special treatment label](https://www.bybit.com/en/help-center/article/Bybit-Special-Treatment-ST-Label-Management-Rules). `0`: false, `1`: true |
| \> lotSizeFilter | Object | Size attributes |
| >\> basePrecision | string | The precision of base coin |
| >\> quotePrecision | string | The precision of quote coin |
| >\> minOrderQty | string | Minimum order quantity, deprecated, no longer check `minOrderQty`, check `minOrderAmt` instead |
| >\> maxOrderQty | string | Maximum order quantity, deprecated, please refer to `maxLimitOrderQty`, `maxMarketOrderQty` based on order type |
| >\> minOrderAmt | string | Minimum order amount |
| >\> maxOrderAmt | string | Maximum order amount, deprecated, no longer check `maxOrderAmt`, check `maxLimitOrderQty` and `maxMarketOrderQty` instead |
| >\> maxLimitOrderQty | string | Maximum Limit order quantity |
| >\> maxMarketOrderQty | string | Maximum Market order quantity |
| >\> postOnlyMaxLimitOrderSize | string | Maximum limit order size for Post-only and RPI orders |
| \> priceFilter | Object | Price attributes |
| >\> tickSize | string | The step to increase/reduce order price |
| \> riskParameters | Object | Risk parameters for limit order price, refer to [announcement](https://announcements.bybit.com/en/article/title-adjustments-to-bybit-s-spot-trading-limit-order-mechanism-blt786c0c5abf865983/) |
| >\> priceLimitRatioX | string | Ratio X |
| >\> priceLimitRatioY | string | Ratio Y |
| \> isPublicRpi | boolean | Whether RPI Is Openly Provided to Market Makers or not.<br>- true: RPI Is Openly Provided to Market Makers<br>- false: RPI Is Not Openly Provided to Market Makers |
| \> myRpiPermission | boolean | Whether the Current User Has RPI Permissions or not<br>- true: Has RPI Permissions<br>- false: Does Not Have RPI Permissions |

* * *

### Request Example

- Linear
- Spot

- HTTP
- Python

```http
GET /v5/account/instruments-info?category=linear&symbol=1000000BABYDOGEUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_account_instruments_info(
    category="linear",
    symbol="BTCUSDT"
))
```

- HTTP
- Python

```http
GET /v5/account/instruments-info?category=spot&symbol=BTCUSDT HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_account_instruments_info(
    category="spot",
    symbol="MNTUSDT"
))
```

### Response Example

- Linear
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
                "symbol": "1000000BABYDOGEUSDT",
                "contractType": "LinearPerpetual",
                "status": "Trading",
                "baseCoin": "1000000BABYDOGE",
                "quoteCoin": "USDT",
                "launchTime": "1718098044000",
                "deliveryTime": "0",
                "deliveryFeeRate": "",
                "priceScale": "7",
                "leverageFilter": {
                    "minLeverage": "1",
                    "maxLeverage": "25.00",
                    "leverageStep": "0.01"
                },
                "priceFilter": {
                    "minPrice": "0.0000001",
                    "maxPrice": "1.9999998",
                    "tickSize": "0.0000001"
                },
                "lotSizeFilter": {
                    "maxOrderQty": "60000000",
                    "minOrderQty": "100",
                    "qtyStep": "100",
                    "postOnlyMaxOrderQty": "60000000",
                    "maxMktOrderQty": "12000000",
                    "minNotionalValue": "5"
                },
                "unifiedMarginTrade": true,
                "fundingInterval": 240,
                "settleCoin": "USDT",
                "copyTrading": "none",
                "upperFundingRate": "0.02",
                "lowerFundingRate": "-0.02",
                "isPreListing": false,
                "preListingInfo": null,
                "riskParameters": {
                    "priceLimitRatioX": "0.15",
                    "priceLimitRatioY": "0.3"
                },
                "displayName": "",
                "symbolType": "innovation",
                "myRpiPermission": true,
                "isPublicRpi": true
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1760510800094
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
                    "quotePrecision": "0.00000001",
                    "minOrderQty": "0.000001",
                    "maxOrderQty": "17000",
                    "minOrderAmt": "5",
                    "maxOrderAmt": "1999999999",
                    "maxLimitOrderQty": "17000",
                    "maxMarketOrderQty": "8500",
                    "postOnlyMaxLimitOrderSize":"60000"
                },
                "priceFilter": {
                    "tickSize": "0.01"
                },
                "riskParameters": {
                    "priceLimitRatioX": "0.05",
                    "priceLimitRatioY": "0.05"
                },
                "symbolType": "",
                "isPublicRpi": true,
                "myRpiPermission": true
            }
        ]
    },
    "retExtInfo": {},
    "time": 1760682563907
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Manual Repay Without Asset Conversion

info

- If `coin` is passed in input parameter and `amount` is not, the repayment amount will be the available spot balance of that coin.

important

1. When repaying, system will only use the spot available balance of the debt currency. Users can perform a manual repay without converting their other assets.
2. To check the spot available amount to repay, you can call this API: [Get Available Amount to Repay](https://bybit-exchange.github.io/docs/v5/spot-margin-uta/repayment-available-amount)
3. Repayment is prohibited between 04:00 and 05:30 per hour. Interest is calculated based on the BorrowAmount at 05:00 per hour.
4. System repays floating-rate liabilities first, followed by fixed-rate
5. Starting Mar 17, 2026 (gradual rollout, fully released on Mar 24, 2026), BYUSDT can be used for repayment.

### HTTP Request

POST`/v5/account/no-convert-repay`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | **true** | string | coin name, uppercase only |
| amount | false | string | Repay amount. If `coin` is not passed in input parameter, `amount` can not be passed in input parameter |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array | Object |
| \> resultStatus | string | - `P`: Processing<br>- `SU`: Success<br>- `FA`: Failed |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/no-convert-repay HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675842997277
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "coin":"BTC",
    "amount":"0.01"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_no_convert_repay(
    coin="BTC",
    amount="0.01"
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "resultStatus": "P"
    },
    "retExtInfo": {},
    "time": 1756295680801
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Option Asset Info

Query the option asset profit and loss information for each coin under the account.

### HTTP Request

GET`/v5/account/option-asset-info`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array<object> | Asset P&L info list |
| \> coin | string | Coin name |
| \> totalDelta | string | Total delta. Only includes delta from option **positions** |
| \> totalRPL | string | Total realised P&L |
| \> totalUPL | string | Total unrealised P&L |
| \> assetIM | string | Asset initial margin. Includes IM occupied by option **open orders** |
| \> assetMM | string | Asset maintenance margin. Includes MM occupied by option **open orders** |
| \> sendTime | number | Snapshot timestamp (ms) |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/option-asset-info HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1773230920000
X-BAPI-RECV-WINDOW: 5000
```

```python

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "result": [
            {
                "totalDelta": "0.0118",
                "assetIM": "0.0000",
                "totalUPL": "-47.6318",
                "totalRPL": "-0.2790",
                "assetMM": "0.0000",
                "coin": "BTC",
                "sendTime": 1773230923530
            }
        ]
    },
    "retExtInfo": {},
    "time": 1773230923533
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Pay Info

Query repayment collateral information for the account before repayment. This data is typically used prior to calling the [Repay](https://bybit-exchange.github.io/docs/v5/account/repay) or [Repay Liability](https://bybit-exchange.github.io/docs/v5/account/repay-liability) endpoints.

### HTTP Request

GET`/v5/account/pay-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | false | string | Coin name, e.g. `USDT`, `BTC`. Must be a coin with outstanding liabilities, otherwise an error will be returned. If not passed, returns the aggregated total across all liabilities. |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| collateralInfo | object | Collateral info |
| \> collateralList | array<object> | Collateral list |
| >\> coin | string | Coin name |
| >\> availableSize | string | Available size |
| >\> availableValue | string | Available value (in USD) |
| >\> coinScale | integer | Coin precision |
| >\> borrowSize | string | Borrow size |
| >\> spotHedgeAmount | string | Spot hedge amount |
| >\> assetFrozen | string | Frozen asset |
| borrowInfo | object | Borrow info for the queried coin. |
| \> coin | string | Coin name. Only returned when `coin` is passed in the request |
| \> borrowSize | string | Borrow size |
| \> borrowValue | string | Borrow value (in USD) |
| \> assetFrozen | string | Frozen asset |
| \> availableBalance | string | Available balance |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/pay-info?coin=SOL HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1773230920000
X-BAPI-RECV-WINDOW: 5000
```

```python

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "collateralInfo": {
            "collateralList": [
                {
                    "availableValue": "13038.369918809215134556464993",
                    "coinScale": 4,
                    "borrowSize": "0",
                    "spotHedgeAmount": "0",
                    "assetFrozen": "0",
                    "availableSize": "13041.904274867704282417",
                    "coin": "USDT"
                },
                {
                    "availableValue": "4997.9120975285472554850",
                    "coinScale": 6,
                    "borrowSize": "0",
                    "spotHedgeAmount": "0",
                    "assetFrozen": "0",
                    "availableSize": "4997.912097528547255485",
                    "coin": "USDC"
                },
                {
                    "availableValue": "0",
                    "coinScale": 8,
                    "borrowSize": "0.10006839",
                    "spotHedgeAmount": "0",
                    "assetFrozen": "0",
                    "availableSize": "0",
                    "coin": "SOL"
                },
                {
                    "availableValue": "0",
                    "coinScale": 9,
                    "borrowSize": "0.001000068",
                    "spotHedgeAmount": "-0.00100006728685180109293673123005419256514869630336761474609375",
                    "assetFrozen": "0",
                    "availableSize": "0",
                    "coin": "BTC"
                },
                {
                    "availableValue": "38488.319604591478793130841305",
                    "coinScale": 8,
                    "borrowSize": "0",
                    "spotHedgeAmount": "0",
                    "assetFrozen": "0",
                    "availableSize": "17.867158854367695555",
                    "coin": "ETH"
                }
            ]
        },
        "borrowInfo": {
            "borrowSize": "0.100068384926503532",
            "assetFrozen": "0",
            "borrowValue": "9.172497619169950215718876",
            "coin": "SOL",
            "availableBalance": "0"
        }
    },
    "retExtInfo": {},
    "time": 1774344990873
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Manual Repay

info

- If neither `coin` nor `amount` is passed in input parameter, then repay all the liabilities.
- If `coin` is passed in input parameter and `amount` is not, the coin will be repaid in full.

important

1. When repaying, the system will first use the spot available balance of the debt currency. If that’s not enough, the remaining amount will be repaid by converting other assets according to the [liquidation order](https://www.bybit.com/en/announcement-info/fullstock-leverage-uta/).
2. If you only want to repay using your spot balance and don't want to trigger currency convert repayment, please refer to [Manual Repay Without Asset Conversion](https://bybit-exchange.github.io/docs/v5/account/no-convert-repay)
3. Repayment is prohibited between 04:00 and 05:30 per hour. Interest is calculated based on the BorrowAmount at 05:00 per hour.
4. System repays floating-rate liabilities first, followed by fixed-rate
5. Starting Mar 17, 2026 (gradual rollout, fully released on Mar 24, 2026), BYUSDT can be used for repayment.
6. MNT will temporarily not be used for repayment, and repaying MNT liabilities through convert-repay is not supported. However, you may still use [Manual Repay Without Asset Conversion](https://bybit-exchange.github.io/docs/v5/account/no-convert-repay) to repay MNT using your existing balance.
7. Starting Feb 10, 2026 at 08:00 UTC, UTA Loan manual repayments will be updated to calculate coin-conversion repayment fees using the higher of the collateral or debt asset fee rate and introduce a per-transaction coin-conversion limit of USD 300,000 (Total coin-conversion amount must less than 300,000 USD equivalent) to strengthen stability and risk controls. Please refer to [UTA Loan manual repayment update](https://announcements.bybit.com/article/uta-loan-manual-repayment-update-bltbef3f1ad72a8295d/)

### HTTP Request

POST`/v5/account/repay`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | false | string | coin name, uppercase only |
| amount | false | string | Repay amount. If `coin` is not passed in input parameter, `amount` can not be passed in input parameter |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| result | array | Object |
| \> resultStatus | string | - `P`: Processing<br>- `SU`: Success<br>- `FA`: Failed |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/repay HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675842997277
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "coin":"BTC",
    "amount":"0.01"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay(
    coin="BTC",
    amount="0.01"
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "resultStatus": "P"
    },
    "retExtInfo": {},
    "time": 1756295680801
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Repay Liability

You can manually repay the liabilities of Unified account

> **Permission**: USDC Contracts

info

1. Starting Mar 17, 2026 (gradual rollout, fully released on Mar 24, 2026), BYUSDT can be used for repayment.
2. MNT will temporarily not be used for repayment, and repaying MNT liabilities through convert-repay is not supported. However, you may still use [Manual Repay Without Asset Conversion](https://bybit-exchange.github.io/docs/v5/account/no-convert-repay) to repay MNT using your existing balance.
3. Starting Feb 10, 2026 at 08:00 UTC, UTA Loan manual repayments will be updated to calculate coin-conversion repayment fees using the higher of the collateral or debt asset fee rate and introduce a per-transaction coin-conversion limit of USD 300,000 (Total coin-conversion amount must less than 300,000 USD equivalent) to strengthen stability and risk controls. Please refer to [UTA Loan manual repayment update](https://announcements.bybit.com/article/uta-loan-manual-repayment-update-bltbef3f1ad72a8295d/)

### HTTP Request

POST`/v5/account/quick-repayment`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | false | string | The coin with liability, uppercase only <br>- Input the specific coin: repay the liability of this coin in particular<br>- No coin specified: repay the liability of all coins |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> coin | string | Coin used for repayment <br>- The order of currencies used to repay liability is based on `liquidationOrder` from [this endpoint](https://bybit-exchange.github.io/docs/v5/spot-margin-uta/vip-margin) |
| \> repaymentQty | string | Repayment qty |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/quick-repayment HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1701848610019
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 22

{
    "coin": "USDT"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay_liability(
    coin="USDT"
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .repayLiability({
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
    "retMsg": "SUCCESS",
    "result": {
        "list": [
            {
                "coin": "BTC",
                "repaymentQty": "0.10549670"
            },
            {
                "coin": "ETH",
                "repaymentQty": "2.27768114"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1701848610941
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Reset MMP

info

- Once the mmp triggered, you can unfreeze the account by this endpoint, then `qtyLimit` and `deltaLimit` will be reset to 0.
- If the account is not frozen, reset action can also remove previous accumulation, i.e., `qtyLimit` and `deltaLimit` will be reset to 0.

### HTTP Request

POST`/v5/account/mmp-reset`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| baseCoin | **true** | string | Base coin, uppercase only |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/mmp-reset HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675842997277
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "baseCoin": "ETH"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.reset_mmp(
    baseCoin="ETH",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .resetMMP('ETH')
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

# Set Collateral Coin

You can decide whether the assets in the Unified account needs to be collateral coins.

### HTTP Request

POST`/v5/account/set-collateral-switch`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coin | **true** | string | Coin name, uppercase only <br>- You can get collateral coin from [here](https://bybit-exchange.github.io/docs/v5/account/collateral-info)<br>- USDT, USDC cannot be set |
| collateralSwitch | **true** | string | `ON`: switch on collateral, `OFF`: switch off collateral |

### Response Parameters

None

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/set-collateral)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-collateral-switch HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1690513916181
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 55

{
    "coin": "BTC",
    "collateralSwitch": "ON"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_collateral_coin(
    coin="BTC",
    collateralSwitch="ON"
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .setCollateralCoin({
    coin: 'BTC',
    collateralSwitch: 'ON',
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
    "result": {},
    "retExtInfo": {},
    "time": 1690515818656
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Delta Neutral Mode

Delta Neutral Mode is designed to enhance the trading experience for users running delta-neutral strategies. When enabled, positions that meet the Delta Neutral criteria are ranked lower in the ADL (Auto-Deleveraging) queue, reducing the risk of being auto-deleveraged during extreme market conditions.
For more details, refer to the [Delta Neutral Mode](https://www.bybit.com/en/help-center/article?id=1772092051700) help article.

You can turn on/off the Delta Neutral mode. To query the current status, use the [Get Trade Behaviour Config](https://bybit-exchange.github.io/docs/v5/account/get-user-setting-config) endpoint and check the `deltaEnable` field in the response.

### HTTP Request

POST`/v5/account/set-delta-mode`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| deltaEnable | **true** | string | `1`: Enable; `0`: Disable |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| resultStatus | integer | `success`;`failed` |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-delta-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1773113846000
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 20

{
    "deltaEnable": "1"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_delta_mode(
    deltaEnable="1"
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {},
    "retExtInfo": {},
    "time": 1773113846355
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Margin Mode

Default is regular margin mode

### HTTP Request

POST`/v5/account/set-margin-mode`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| setMarginMode | **true** | string | `ISOLATED_MARGIN`, `REGULAR_MARGIN`(i.e. Cross margin), `PORTFOLIO_MARGIN` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| reasons | array | Object. If requested successfully, it is an empty array |
| \> reasonCode | string | Fail reason code |
| \> reasonMsg | string | Fail reason msg |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/set-margin-mode)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-margin-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672134396332
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "setMarginMode": "PORTFOLIO_MARGIN"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_margin_mode(
    setMarginMode="PORTFOLIO_MARGIN",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setMarginMode('PORTFOLIO_MARGIN')
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
    "retCode": 3400045,
    "retMsg": "Set margin mode failed",
    "result": {
        "reasons": [
            {
                "reasonCode": "3400000",
                "reasonMsg": "Equity needs to be equal to or greater than 1000 USDC"
            }
        ]
    }
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set MMP

info

## What is MMP?

_Market Maker Protection_ (MMP) is an automated mechanism designed to protect market makers (MM) against liquidity risks
and over-exposure in the market. It prevents simultaneous trade executions on quotes provided by the MM within a short time span.
The MM can automatically pull their quotes if the number of contracts traded for an underlying asset exceeds the configured
threshold within a certain time frame. Once MMP is triggered, any pre-existing MMP orders will be **automatically cancelled**,
and new orders tagged as MMP will be **rejected** for a specific duration — known as the frozen period — so that MM can
reassess the market and modify the quotes.

## How to enable MMP

Send an email to Bybit ( [financial.inst@bybit.com](mailto:financial.inst@bybit.com)) or contact your business development (BD) manager to apply for MMP.
After processed, the default settings are as below table:

| Parameter | Type | Comments | Default value |
| :-- | :-- | :-- | --- |
| baseCoin | string | Base coin | BTC |
| window | string | Time window (millisecond) | 5000 |
| frozenPeriod | string | Frozen period (millisecond) | 100 |
| qtyLimit | string | Quantity limit | 100 |
| deltaLimit | string | Delta limit | 100 |

## Applicable

Effective for **options** only. When you place an `option` order, set `mmp`=true, which means you mark this order as a mmp order.

## Some points to note

1. Only maker order qty and delta will be counted into `qtyLimit` and `deltaLimit`.
2. `qty_limit` is the sum of absolute value of qty of each trade executions. `delta_limit` is the absolute value of the sum of qty\*delta. If any of these reaches or exceeds the limit amount, the account's market maker protection will be triggered.

### HTTP Request

POST`/v5/account/mmp-modify`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| baseCoin | **true** | string | Base coin, uppercase only |
| window | **true** | string | Time window (ms) |
| frozenPeriod | **true** | string | Frozen period (ms). "0" means the trade will remain frozen until manually reset |
| qtyLimit | **true** | string | Trade qty limit (positive and up to 2 decimal places) |
| deltaLimit | **true** | string | Delta limit (positive and up to 2 decimal places) |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/mmp-modify HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1675833524616
X-BAPI-RECV-WINDOW: 50000
Content-Type: application/json

{
    "baseCoin": "ETH",
    "window": "5000",
    "frozenPeriod": "100000",
    "qtyLimit": "50",
    "deltaLimit": "20"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_mmp(
    baseCoin="ETH",
    window="5000",
    frozenPeriod="100000",
    qtyLimit="50",
    deltaLimit="20"
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .setMMP({
        baseCoin: 'ETH',
        window: '5000',
        frozenPeriod: '100000',
        qtyLimit: '50',
        deltaLimit: '20',
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
    "retMsg": "success"
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Price Limit Behaviour

You can configure how the system behaves when your limit order price exceeds the highest bid or lowest ask price.
You can query your current configuration with [Get Trade Behaviour Setting](https://bybit-exchange.github.io/docs/v5/account/get-user-setting-config).
Learn more about the price limit for [spot](https://www.bybit.com/en/help-center/article/Bybit-Spot-Trading-Rules#A) and [futures](https://www.bybit.com/en/help-center/article?id=000002177#D) in the help centre.

* * *

Where x% is [priceLimitRatioX](https://bybit-exchange.github.io/docs/v5/market/instrument); and y% is the [priceLimitRatioY](https://bybit-exchange.github.io/docs/v5/market/instrument):

Spot

- **Maximum buy price**: _Min\[Max(Index Price, Index Price × (1 + x%) + 2-Minute Average Premium), Index Price × (1 + y%)\]_
- **Minimum sell price**: _Max\[Min(Index Price, Index Price × (1 – x%) + 2-Minute Average Premium), Index Price × (1 – y%)\]_

Futures

- **Maximum buy price**: _Min (Max (Index Price, Mark Price × (1 + x%)), Mark Price × (1 + y%))_
- **Minimum sell price**: _Max (Min (Index Price, Mark Price × (1 - x%)), Mark Price × (1 - y%))_

Default Setting

- Spot:
**modifyEnable = false.** If the order price exceeds the limit, the system rejects the request.

Corresponds to [Get Limit Price Behaviour](https://bybit-exchange.github.io/docs/v5/account/get-user-setting-config), where **lpaSpot = false, lpaPerp = true**

- Futures:
**modifyEnable = true.** If the order price exceeds the limit, the system will automatically adjust the price to the nearest allowed price (i.e., highest bid or lowest ask).

Corresponds to [Get Limit Price Behaviour](https://bybit-exchange.github.io/docs/v5/account/get-user-setting-config), where **lpaSpot = true, lpaPerp = false**

- Setting either `linear` or `inverse` will set the behaviour for **all futures**.

### HTTP Request

POST`/v5/account/set-limit-px-action`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| category | **true** | string | `linear`, `inverse`, `spot` |
| modifyEnable | **true** | boolean | `true`: allow the system to modify the order price<br>`false`: reject your order request |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-limit-px-action HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1753255927950
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 52

{
    "category": "spot",
    "modifyEnable": true
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_limit_price_action(
    category="spot",
    modifyEnable=True,
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {},
    "retExtInfo": {},
    "time": 1753255927952
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Spot Hedging

You can turn on/off Spot hedging feature in Portfolio margin

### HTTP Request

POST`/v5/account/set-hedging-mode`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| setHedgingMode | **true** | string | `ON`, `OFF` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| retCode | integer | Result code |
| retMsg | string | Result message |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/set-spot-hedge)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/account/set-hedging-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1700117968580
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 31

{
    "setHedgingMode": "OFF"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_hedging_mode(
    setHedgingMode="OFF"
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .setSpotHedging({
    setHedgingMode: 'ON' | 'OFF',
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
    "retMsg": "SUCCESS"
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get SMP Group ID

Query the SMP group ID of self match prevention

### HTTP Request

GET`/v5/account/smp-group`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| smpGroup | integer | Smp group ID. If the UID has no group, it is `0` by default |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/smp-group HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1702363848192
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_smp_group())
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .getSMPGroup()
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
        "smpGroup": 0
    },
    "retExtInfo": {},
    "time": 1702363848539
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Trade Info For Analysis

Query aggregated **spot trade** analysis data for a symbol, including execution values, quantities, fees, and daily breakdown.

### HTTP Request

GET`/v5/account/trade-info-for-analysis`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| symbol | **true** | string | Symbol name, e.g. `BTCUSDT`, `ETHUSDT` |
| startTime | false | long | Query start time (ms) |
| endTime | false | long | Query end time (ms) |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| symbolRnl | string | Symbol realised P&L |
| netExecQty | string | Net execution quantity |
| sumExecValue | string | Total execution value |
| sumExecQty | string | Total execution quantity |
| avgBuyExecPrice | string | Average buy execution price |
| sumBuyExecValue | string | Total buy execution value |
| sumBuyExecQty | string | Total buy execution quantity |
| sumBuyExecFee | string | Total buy execution fee |
| sumBuyOrderQty | string | Total buy order quantity |
| avgSellExecPrice | string | Average sell execution price |
| sumSellExecValue | string | Total sell execution value |
| sumSellExecQty | string | Total sell execution quantity |
| sumSellExecFee | string | Total sell execution fee |
| sumSellOrderQty | string | Total sell order quantity |
| maxMarginVersion | integer | Max margin version number |
| baseCoin | string | Base coin |
| settleCoin | string | Settle coin |
| sumPriceList | array<object> | Daily aggregated price list |
| \> day | string | Date |
| \> sumBuyExecValue | string | Daily total buy execution value |
| \> sumSellExecValue | string | Daily total sell execution value |
| \> sumExecValue | string | Daily total execution value |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/trade-info-for-analysis?symbol=ETHUSDT HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1773230920000
X-BAPI-RECV-WINDOW: 5000
```

```python

```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "symbolRnl": "0",
        "sumBuyExecValue": "0",
        "sumBuyExecQty": "0",
        "sumSellExecFee": "0",
        "netExecQty": "0",
        "sumExecQty": "0",
        "settleCoin": "USDT",
        "sumExecValue": "0",
        "sumSellExecValue": "0",
        "sumBuyOrderQty": "0",
        "sumSellOrderQty": "0",
        "maxMarginVersion": 0,
        "avgSellExecPrice": "0",
        "avgBuyExecPrice": "0",
        "sumBuyExecFee": "0",
        "sumSellExecQty": "0",
        "baseCoin": "ETH"
    },
    "retExtInfo": {},
    "time": 1773230927308
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Transaction Log

Query for transaction logs in your Unified account. It supports up to 2 years worth of data.

info

- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/account/transaction-log`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [accountType](https://bybit-exchange.github.io/docs/v5/enum#accounttype) | false | string | Account Type. `UNIFIED` |
| [category](https://bybit-exchange.github.io/docs/v5/enum#category) | false | string | Product type `spot`,`linear`,`option`,`inverse` |
| currency | false | string | Currency, uppercase only |
| baseCoin | false | string | BaseCoin, uppercase only. e.g., BTC of BTCPERP |
| [type](https://bybit-exchange.github.io/docs/v5/enum#typeuta-translog) | false | string | Types of transaction logs |
| transSubType | false | string | `movePosition`, used to filter trans logs of Move Position only |
| startTime | false | integer | The start timestamp (ms) <br>- startTime and endTime are not passed, return 24 hours by default<br>- Only startTime is passed, return range between startTime and startTime+24 hours<br>- Only endTime is passed, return range between endTime-24 hours and endTime<br>- If both are passed, the rule is endTime - startTime <= 7 days |
| endTime | false | integer | The end timestamp (ms) |
| limit | false | integer | Limit for data size per page. \[`1`, `50`\]. Default: `20` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> id | string | Unique id |
| \> symbol | string | Symbol name |
| \> category | string | Product type |
| \> side | string | Side. `Buy`,`Sell`,`None` |
| \> transactionTime | string | Transaction timestamp (ms) |
| \> [type](https://bybit-exchange.github.io/docs/v5/enum#typeuta-translog) | string | Type |
| \> transSubType | string | Transaction sub type, `movePosition`, used for the logs generated by move position. `""` by default |
| \> qty | string | Quantity <br>- Spot: the negative means the qty of this currency is decreased, the positive means the qty of this currency is increased<br>- Perps & Futures: it is the quantity for each trade entry and it does not have direction |
| \> size | string | Size. The rest position size after the trade is executed, and it has direction, i.e., short with "-" |
| \> currency | string | e.g., USDC, USDT, BTC, ETH |
| \> tradePrice | string | Trade price |
| \> funding | string | Funding fee <br>- Positive fee value means receive funding; negative fee value means pay funding. This is opposite to the `execFee` from [Get Trade History](https://bybit-exchange.github.io/docs/v5/order/execution).<br>- For USDC Perp, as funding settlement and session settlement occur at the same time, they are represented in a single record at settlement. Please refer to `funding` to understand funding fee, and `cashFlow` to understand 8-hour P&L. |
| \> fee | string | Trading fee <br>- Positive fee value means expense<br>- Negative fee value means rebates |
| \> cashFlow | string | Cash flow, e.g., (1) close the position, and unRPL converts to RPL, (2) 8-hour session settlement for USDC Perp and Futures, (3) transfer in or transfer out. This does not include trading fee, funding fee |
| \> change | string | Change = cashFlow + funding - fee |
| \> cashBalance | string | Cash balance. This is the wallet balance after a cash change |
| \> feeRate | string | - When type=`TRADE`, then it is trading fee rate<br>- When type=`SETTLEMENT`, it means funding fee rate. For side=Buy, feeRate=market fee rate; For side=Sell, feeRate= - market fee rate |
| \> bonusChange | string | The change of bonus |
| \> tradeId | string | Trade ID |
| \> orderId | string | Order ID |
| \> orderLinkId | string | User customised order ID |
| \> extraFees | string | Trading fee rate information. Currently, this data is returned only for spot orders placed on the Indonesian site or spot fiat currency orders placed on the EU site. In other cases, an empty string is returned. Enum: [feeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeesfeetype), [subFeeType](https://bybit-exchange.github.io/docs/v5/enum#extrafeessubfeetype) |
| nextPageCursor | string | Refer to the `cursor` request parameter |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/transaction-log)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/transaction-log?accountType=UNIFIED&category=linear&currency=USDT HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672132480085
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_transaction_log(
    accountType="UNIFIED",
    category="linear",
    currency="USDT",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getTransactionLog({
        accountType: 'UNIFIED',
        category: 'linear',
        currency: 'USDT',
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
        "nextPageCursor": "21963%3A1%2C14954%3A1",
        "list": [
            {
                "transSubType": "",
                "id": "592324_XRPUSDT_161440249321",
                "symbol": "XRPUSDT",
                "side": "Buy",
                "funding": "-0.003676",
                "orderLinkId": "",
                "orderId": "1672128000-8-592324-1-2",
                "fee": "0.00000000",
                "change": "-0.003676",
                "cashFlow": "0",
                "transactionTime": "1672128000000",
                "type": "SETTLEMENT",
                "feeRate": "0.0001",
                "bonusChange": "",
                "size": "100",
                "qty": "100",
                "cashBalance": "5086.55825002",
                "currency": "USDT",
                "category": "linear",
                "tradePrice": "0.3676",
                "tradeId": "534c0003-4bf7-486f-aa02-78cee36825e4",
                "extraFees": ""
            },
            {
                "transSubType": "",
                "id": "592324_XRPUSDT_161440249321",
                "symbol": "XRPUSDT",
                "side": "Buy",
                "funding": "",
                "orderLinkId": "linear-order",
                "orderId": "592b7e41-78fd-42e2-9aa3-91e1835ef3e1",
                "fee": "0.01908720",
                "change": "-0.0190872",
                "cashFlow": "0",
                "transactionTime": "1672121182224",
                "type": "TRADE",
                "feeRate": "0.0006",
                "bonusChange": "-0.1430544",
                "size": "100",
                "qty": "88",
                "cashBalance": "5086.56192602",
                "currency": "USDT",
                "category": "linear",
                "tradePrice": "0.3615",
                "tradeId": "5184f079-88ec-54c7-8774-5173cafd2b4e",
                "extraFees": ""
            },
            {
                "transSubType": "",
                "id": "592324_XRPUSDT_161407743011",
                "symbol": "XRPUSDT",
                "side": "Buy",
                "funding": "",
                "orderLinkId": "linear-order",
                "orderId": "592b7e41-78fd-42e2-9aa3-91e1835ef3e1",
                "fee": "0.00260280",
                "change": "-0.0026028",
                "cashFlow": "0",
                "transactionTime": "1672121182224",
                "type": "TRADE",
                "feeRate": "0.0006",
                "bonusChange": "",
                "size": "12",
                "qty": "12",
                "cashBalance": "5086.58101322",
                "currency": "USDT",
                "category": "linear",
                "tradePrice": "0.3615",
                "tradeId": "8569c10f-5061-5891-81c4-a54929847eb3",
                "extraFees": ""
            }
        ]
    },
    "retExtInfo": {},
    "time": 1672132481405
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Transferable Amount (Unified)

Query the available amount to transfer of a specific coin in the Unified wallet.

info

Formula of Asset Available Balance for withdraw:

1. Reverse calculate Asset Available Amount = X, using `totalAvailableBalance` in [Get Wallet Balance](https://bybit-exchange.github.io/docs/v5/account/wallet-balance) and the asset's tiered collateral ratio

2. Asset Available Balance for withdraw = min(X, asset withdraw Available balance)

- under Cross marin mode: asset withdraw Available balance = asset wallet balance + min(unrealised pnl,0) + asset reservation - frozen + negative option value - bonus - Positive Option OrderIM + orderloss

- under Portfolio margin mode: asset withdraw Available balance = asset wallet balance + min(unrealised pnl,0) + asset reservation - frozen - max(bonus, Pm spot Hedged Balance) + orderloss + min(optionValue,0)

3. During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/account/withdrawal`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| coinName | **true** | string | Coin name, uppercase only. Supports up to 20 coins per request, use comma to separate. `BTC,USDC,USDT,SOL` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| availableWithdrawal | string | Transferable amount for the 1st coin in the request |
| availableWithdrawalMap | Object | Transferable amount map for each requested coin. In the map, key is the requested coin, and value is the accordingly amount(string)<br>e.g., "availableWithdrawalMap":{"BTC":"4.54549050","SOL":"33.16713007","XRP":"10805.54548970","ETH":"17.76451865"} |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/withdrawal?coinName=BTC,SOL,ETH,XRP HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1739861239242
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
print(session.get_transferable_amount(
    coinName="BTC,SOL,ETH,XRP"
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
        "availableWithdrawal": "4.54549050",
        "availableWithdrawalMap": {
            "BTC": "4.54549050",
            "SOL": "33.16713007",
            "XRP": "10805.54548970",
            "ETH": "17.76451865"
        }
    },
    "retExtInfo": {},
    "time": 1739858984601
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Upgrade to Unified Account Pro

Upgrade Guidance

Check your current account status by calling this [Get Account Info](https://bybit-exchange.github.io/docs/v5/account/account-info)

- if unifiedMarginStatus=5, then it is [UTA2.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-20), you can call below upgrade endpoint to [UTA2.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-20) Pro. Check
[Get Account Info](https://bybit-exchange.github.io/docs/v5/account/account-info) after a while and if unifiedMarginStatus=6, then the account has successfully upgraded to [UTA2.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-20) Pro.
- When the user is a master account, the current user is allowed to upgrade to UTA PRO if they are a VIP or PRO level user.
- When the user is a sub-account, only parent accounts with VIP or PRO level are allowed to upgrade to UTA PRO.

info

please note belows:

1. Please avoid upgrading during these period:

|  |  |
| :-- | :-- |
| every hour | 50th minute to 5th minute of next hour |

2. Please ensure: there is no open orders when upgrade from [UTA2.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-20) to [UTA2.0](https://bybit-exchange.github.io/docs/v5/acct-mode#uta-20) Pro

3. During the account upgrade process, the data of **Rest API/Websocket stream** may be inaccurate due to the fact that the account-related
asset data is in the processing state. It is recommended to query and use it after the upgrade is completed.

### HTTP Request

POST`/v5/account/upgrade-to-uta`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| unifiedUpdateStatus | string | Upgrade status. `FAIL`,`PROCESS`,`SUCCESS` |
| unifiedUpdateMsg | Object | If `PROCESS`,`SUCCESS`, it returns `null` |
| \> msg | array | Error message array. Only `FAIL` will have this field |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/upgrade-unified-account)

* * *

### Request Example

- HTTP
- Python
- GO
- Java
- .Net
- Node.js

```http
POST /v5/account/upgrade-to-uta HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672125123533
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.upgrade_to_unified_trading_account())
```

```go
import (
    "context"
    "fmt"
    bybit "github.com/bybit-exchange/bybit.go.api"
)
client := bybit.NewBybitHttpClient("YOUR_API_KEY", "YOUR_API_SECRET")
client.NewUtaBybitServiceNoParams().UpgradeToUTA(context.Background())
```

```java
import com.bybit.api.client.config.BybitApiConfig;
import com.bybit.api.client.domain.account.request.AccountDataRequest;
import com.bybit.api.client.domain.account.AccountType;
import com.bybit.api.client.service.BybitApiClientFactory;
var client = BybitApiClientFactory.newInstance("YOUR_API_KEY", "YOUR_API_SECRET", BybitApiConfig.TESTNET_DOMAIN).newAccountRestClient();
System.out.println(client.upgradeAccountToUTA());
```

```c#
using bybit.net.api;
using bybit.net.api.ApiServiceImp;
using bybit.net.api.Models;
BybitAccountService accountService = new(apiKey: "xxxxxx", apiSecret: "xxxxx");
Console.WriteLine(await accountService.UpgradeAccount());
```

```n4js
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .upgradeToUnifiedAccount()
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
    "retMsg": "",
    "result": {
        "unifiedUpdateStatus": "FAIL",
        "unifiedUpdateMsg": {
            "msg": [
                "Update account failed. You have outstanding liabilities in your Spot account.",
                "Update account failed. Please close the usdc perpetual positions in USDC Account.",
                "unable to upgrade, please cancel the usdt perpetual open orders in USDT account.",
                "unable to upgrade, please close the usdt perpetual positions in USDT account."
            ]
    }
},
    "retExtInfo": {},
    "time": 1672125124195
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Wallet Balance

Obtain wallet balance, query asset information of each currency. By default, currency
information with assets or liabilities of 0 is not returned.

info

- Under the new logic of UTA manual borrow, `spotBorrow` field corresponding to spot liabilities is detailed in the [announcement](https://announcements.bybit.com/en/article/bybit-uta-function-optimization-manual-coin-borrowing-will-be-launched-soon-blt5d858199bd12e849/).

- Old `walletBalance` = New `walletBalance` \- `spotBorrow`
- During periods of extreme market volatility, this interface may experience increased latency or temporary delays in data delivery

### HTTP Request

GET`/v5/account/wallet-balance`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [accountType](https://bybit-exchange.github.io/docs/v5/enum#accounttype) | **true** | string | Account type `UNIFIED`. To get Funding wallet balance, please go to this [endpoint](https://bybit-exchange.github.io/docs/v5/asset/balance/all-balance) |
| coin | false | string | Coin name, uppercase only <br>- If not passed, it returns non-zero asset info<br>- You can pass multiple coins to query, separated by comma. `USDT,USDC` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> accountType | string | Account type |
| \> accountIMRate | string | Account IM rate <br>- You can refer to this [Glossary](https://www.bybit.com/en/help-center/article/Glossary-Unified-Trading-Account) to understand the below fields calculation and mearning<br>- All account wide fields are **not** applicable to isolated margin |
| \> accountMMRate | string | Account MM rate |
| \> totalEquity | string | Account total equity (USD): ∑Asset Equity By USD value of each asset |
| \> totalWalletBalance | string | Account wallet balance (USD): ∑Asset Wallet Balance By USD value of each asset |
| \> totalMarginBalance | string | Account margin balance (USD): totalWalletBalance + totalPerpUPL |
| \> totalAvailableBalance | string | Account available balance (USD), <br>- Cross Margin: totalMarginBalance - Haircut - totalInitialMargin.<br>- Porfolio Margin: total Equity - Haircut - totalInitialMargin. |
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
| >\> usdValue | string | USD value of coin |
| >\> walletBalance | string | Wallet balance of coin |
| >\> locked | string | Locked balance due to the Spot open order |
| >\> spotHedgingQty | string | The spot asset qty that is used to hedge in the portfolio margin, truncate to 8 decimals and "0" by default |
| >\> borrowAmount | string | Borrow amount of current coin = spot liabilities + derivatives liabilities |
| >\> accruedInterest | string | Accrued interest |
| >\> totalOrderIM | string | Pre-occupied margin for order. For portfolio margin mode, it returns "" |
| >\> totalPositionIM | string | Sum of initial margin of all positions + Pre-occupied liquidation fee. For portfolio margin mode, it returns "" |
| >\> totalPositionMM | string | Sum of maintenance margin for all positions. For portfolio margin mode, it returns "" |
| >\> unrealisedPnl | string | Unrealised P&L |
| >\> cumRealisedPnl | string | Cumulative Realised P&L |
| >\> bonus | string | Bonus |
| >\> marginCollateral | boolean | Whether it can be used as a margin collateral currency (platform), `true`: YES, `false`: NO <br>- When marginCollateral=false, then collateralSwitch is meaningless |
| >\> collateralSwitch | boolean | Whether the collateral is turned on by user (user), `true`: ON, `false`: OFF <br>- When marginCollateral=true, then collateralSwitch is meaningful |
| >\> spotBorrow | string | Borrow amount by spot margin trade and manual borrow amount (does not include borrow amount by spot margin active order). `spotBorrow` field corresponding to spot liabilities is detailed in the [announcement](https://announcements.bybit.com/en/article/bybit-uta-function-optimization-manual-coin-borrowing-will-be-launched-soon-blt5d858199bd12e849/). |
| >\> free | string | **Deprecated** since there is no Spot wallet any more |
| >\> availableToWithdraw | string | **Deprecated** for `accountType=UNIFIED` from 9 Jan, 2025 <br>- Transferable balance: you can use [Get Transferable Amount (Unified)](https://bybit-exchange.github.io/docs/v5/account/unified-trans-amnt) or [Get All Coins Balance](https://bybit-exchange.github.io/docs/v5/asset/balance/all-balance) instead<br>- Derivatives available balance: <br>  <br>  **isolated margin**: walletBalance - totalPositionIM - totalOrderIM - locked - bonus<br>  <br>  **cross & portfolio margin**: look at field `totalAvailableBalance`(USD), which needs to be converted into the available balance of accordingly coin through index price<br>- Spot (margin) available balance: refer to [Get Borrow Quota (Spot)](https://bybit-exchange.github.io/docs/v5/order/spot-borrow-quota) |
| >\> availableToBorrow | string | **Deprecated** field, always return `""`. Please refer to `availableToBorrow` in the [Get Collateral Info](https://bybit-exchange.github.io/docs/v5/account/collateral-info) |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/account/wallet)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/account/wallet-balance?accountType=UNIFIED&coin=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672125440406
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_wallet_balance(
    accountType="UNIFIED",
    coin="BTC",
))
```

```n4js
const { RestClientV5 } = require('bybit-api');

    const client = new RestClientV5({
    testnet: true,
    key: 'xxxxxxxxxxxxxxxxxx',
    secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
    .getWalletBalance({
        accountType: 'UNIFIED',
        coin: 'BTC',
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
                "totalEquity": "3.31216591",
                "accountIMRate": "0",
                "accountIMRateByMp": "0",
                "totalMarginBalance": "3.00326056",
                "totalInitialMargin": "0",
                "totalInitialMarginByMp": "0",
                "accountType": "UNIFIED",
                "totalAvailableBalance": "3.00326056",
                "accountMMRate": "0",
                "accountMMRateByMp": "0",
                "totalPerpUPL": "0",
                "totalWalletBalance": "3.00326056",
                "accountLTV": "0",
                "totalMaintenanceMargin": "0",
                "totalMaintenanceMarginByMp": "0",
                "coin": [
                    {
                        "availableToBorrow": "3",
                        "bonus": "0",
                        "accruedInterest": "0",
                        "availableToWithdraw": "0",
                        "totalOrderIM": "0",
                        "equity": "0",
                        "totalPositionMM": "0",
                        "usdValue": "0",
                        "spotHedgingQty": "0.01592413",
                        "unrealisedPnl": "0",
                        "collateralSwitch": true,
                        "borrowAmount": "0.0",
                        "totalPositionIM": "0",
                        "walletBalance": "0",
                        "cumRealisedPnl": "0",
                        "locked": "0",
                        "marginCollateral": true,
                        "coin": "BTC",
                        "spotBorrow": "0"
                    }
                ]
            }
        ]
    },
    "retExtInfo": {},
    "time": 1690872862481
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

