# Bybit Spot Margin (UTA) API

Source: https://bybit-exchange.github.io/docs/v5/spot-margin-uta/

---

## Repo Usage Quick Reference

- Primary repo use: spot-margin borrow/repay and spot leg operations for the spot-futures engine
- Repo symbol format: `BTCUSDT`
- Most relevant endpoints for this repo:
  - coin state / state
  - max borrowable
  - interest rate history
  - collateral and margin data
- Important repo note: Bybit UTA can temporarily lock balances around settlement; do not assume `available` always reflects repayability during those windows

# Get Coin State

### HTTP Request

`GET /v5/spot-margin-trade/coinstate`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | arrayList | Object |
| \> currency | string | Coin name, uppercase only |
| \> spotLeverage | string | Spot margin leverage. Returns "" if spot margin mode is turned off |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/coinstate HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1692696840996
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_coin_state(
    currency="BTC"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "list": [
            {
                "spotLeverage": 3,
                "currency": "BTC"
            },
            {
                "spotLeverage": 4,
                "currency": "ETH"
            },
            {
                "spotLeverage": 4,
                "currency": "AVAX"
            },
            {
                "spotLeverage": 4,
                "currency": "EOS"
            },
            {
                "spotLeverage": 4,
                "currency": "XRP"
            },
            {
                "spotLeverage": 4,
                "currency": "USDT"
            },
            {
                "spotLeverage": 4,
                "currency": "GALA"
            },
            {
                "spotLeverage": 4,
                "currency": "DOGE"
            },
            {
                "spotLeverage": 4,
                "currency": "BIT"
            },
            {
                "spotLeverage": 4,
                "currency": "BTC3S"
            },
            {
                "spotLeverage": 4,
                "currency": "BTC3L"
            },
            {
                "spotLeverage": 4,
                "currency": "EUR"
            },
            {
                "spotLeverage": 4,
                "currency": "USDC"
            },
            {
                "spotLeverage": 4,
                "currency": "UNI"
            },
            {
                "spotLeverage": 4,
                "currency": "SOL"
            },
            {
                "spotLeverage": 4,
                "currency": "ADA"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1756273703314
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Currency Data

> Info

If the borrowable switch is disabled (`false`), the related configuration fields will return `""`.

### HTTP Request

`GET /v5/spot-margin-trade/currency-data`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | Coin name |
| \> flexibleManualBorrowable | boolean | Whether flexible manual borrow is enabled. `true`: enabled, `false`: disabled |
| \> minFlexibleManualBorrowQty | string | Min flexible manual borrow qty |
| \> flexibleManualBorrowAccuracy | string | Coin precision for flexible manual borrow |
| \> fixedManualBorrowable | boolean | Whether fixed manual borrow is enabled. `true`: enabled, `false`: disabled |
| \> minFixedManualBorrowQty | string | Min fixed manual borrow qty |
| \> fixedManualBorrowAccuracy | string | Coin precision for fixed manual borrow |
| \> fixedInterestRateAccuracy | string | Coin precision for fixed manual borrow interest rate. |
| \> minFixedInterestRate | string | Min fixed manual borrow interest rate, e.g.: `0.01` |
| \> maxFixedInterestRate | string | Max fixed manual borrow interest rate, e.g.: `0.8` |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/currency-data?currency=BTC HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1773220082000
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_currency_data(
    currency="BTC"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "flexibleManualBorrowable": true,
                "minFlexibleManualBorrowQty": "0.001",
                "flexibleManualBorrowAccuracy": "8",
                "fixedManualBorrowable": false,
                "minFixedManualBorrowQty": "",
                "fixedManualBorrowAccuracy": "",
                "fixedInterestRateAccuracy": "",
                "minFixedInterestRate": "",
                "maxFixedInterestRate": ""
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1773220082091
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Auto Repay Mode

Get spot automatic repayment mode

### HTTP Request

`GET /v5/spot-margin-trade/get-auto-repay-mode`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only. If `currency` is not passed, automatic repay mode for all currencies will be returned. |

* * *

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| data | array | Object |
| \> currency | string | Coin name, uppercase only. |
| \> autoRepayMode | string | - `1`: On<br>- `0`: Off |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/get-auto-repay-mode?currency=ETH HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672299806626
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
print(session.get_auto_repay_mode(
    currency="ETH"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "data": [
            {
                "autoRepayMode": "1",
                "currency": "ETH"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1766977353904
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Historical Interest Rate

You can query up to six months borrowing interest rate of Margin trading.

> Info

- Need authentication, the api key needs "Spot" permission
- Only supports Unified account
- It is public data, i.e., different users get the same historical interest rate for the same VIP/Pro

### HTTP Request

`GET /v5/spot-margin-trade/interest-rate-history`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Coin name, uppercase only |
| [vipLevel](https://bybit-exchange.github.io/docs/v5/enum#viplevel) | false | string | VIP level <br>- Please note that "No VIP" should be passed like "No%20VIP" in the query string<br>- If not passed, it returns your account's VIP level data |
| startTime | false | integer | The start timestamp (ms) <br>- Either both time parameters are passed or neither is passed.<br>- Returns 7 days data when both are not passed<br>- Supports up to 30 days interval when both are passed |
| endTime | false | integer | The end timestamp (ms) |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array<object> |  |
| \> timestamp | long | timestamp |
| \> currency | string | coin name |
| \> hourlyBorrowRate | string | Hourly borrowing rate |
| \> vipLevel | string | VIP/Pro level |

### Request Example

- HTTP
- Python

```http
GET /v5/spot-margin-trade/interest-rate-history?currency=USDC&vipLevel=No%20VIP&startTime=1721458800000&endTime=1721469600000 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1721891663064
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_historical_interest_rate(
    currency="BTC"
))
```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "timestamp": 1721469600000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            },
            {
                "timestamp": 1721466000000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            },
            {
                "timestamp": 1721462400000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            },
            {
                "timestamp": 1721458800000,
                "currency": "USDC",
                "hourlyBorrowRate": "0.000014621596",
                "vipLevel": "No VIP"
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1721899048991
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Max Borrowable Amount

### HTTP Request

`GET /v5/spot-margin-trade/max-borrowable`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| currency | string | Coin name, uppercase only |
| maxLoan | string | Max borrowable amount |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/max-borrowable?currency=BTC HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1692696840996
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_max_borrowable(
    currency="BTC"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "maxLoan": "17.54689892",
        "currency": "BTC"
    },
    "retExtInfo": {},
    "time": 1756261353733
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Position Tiers

> Info

- If `currency` is passed in the input parameter, query by currency; if `currency` is not passed in the input parameter, query all configured currencies

### HTTP Request

`GET /v5/spot-margin-trade/position-tiers`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | Coin name, uppercase only |
| \> positionTiersRatioList | string | Object |
| >\> tier | string | Tiers. Display from small to large |
| >\> borrowLimit | string | Tiers Accumulation Borrow limit |
| >\> positionMMR | string | Loan Maintenance Margin Rate. Precision 8 decimal places |
| >\> positionIMR | string | Loan Initial Margin Rate. Precision 8 decimal places |
| >\> maxLeverage | string | Max Loan Leverage |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/position-tiers?currency=BTC HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1692696840996
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_position_tiers(
    currency="BTC"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "positionTiersRatioList": [
                    {
                        "tier": "1",
                        "borrowLimit": "390",
                        "positionMMR": "0.04",
                        "positionIMR": "0.2",
                        "maxLeverage": "5"
                    },
                    {
                        "tier": "2",
                        "borrowLimit": "391",
                        "positionMMR": "0.04",
                        "positionIMR": "0.25",
                        "maxLeverage": "4"
                    },
                    {
                        "tier": "3",
                        "borrowLimit": "392",
                        "positionMMR": "0.04",
                        "positionIMR": "0.33333333",
                        "maxLeverage": "3"
                    },
                    {
                        "tier": "4",
                        "borrowLimit": "393",
                        "positionMMR": "0.04",
                        "positionIMR": "0.5",
                        "maxLeverage": "2"
                    }
                ]
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1756272543440
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Available Amount to Repay

### HTTP Request

`GET /v5/spot-margin-trade/repayment-available-amount`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| currency | string | Coin name, uppercase only |
| lossLessRepaymentAmount | string | Repayment amount = min(spot coin available balance, coin borrow amount) |

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/repayment-available-amount?currency=BTC HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1692696840996
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_repayment_available_amount(
    currency="BTC"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "Success",
    "result": {
        "lossLessRepaymentAmount": "0.02000000",
        "currency": "BTC"
    },
    "retExtInfo": {},
    "time": 1756273388821
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Auto Repay Mode

Set spot automatic repayment mode

> Info

1. If `currency` is not passed, spot automatic repayment will be enabled for all currencies.
2. If `autoRepayMode` of a currency is set to 1, the system will automatically make repayments without asset conversion to that currency at 0 and 30 minutes every hour.
3. The amount of repayments without asset conversion is the minimum of available spot balance in that currency and liability of that currency.
4. If you missed the automatic repayment batches for 0 and 30 minutes every hour, you can manually make the repayment via the API. Please refer to [Manual Repay Without Asset Conversion](https://bybit-exchange.github.io/docs/v5/account/no-convert-repay)

### HTTP Request

`POST /v5/spot-margin-trade/set-auto-repay-mode`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only. If `currency` is not passed, spot automatic repayment will be enabled for all currencies. |
| autoRepayMode | **true** | string | - `1`: On<br>- `0`: Off |

* * *

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| data | array | Object |
| \> currency | string | Coin name, uppercase only. |
| \> autoRepayMode | string | - `1`: On<br>- `0`: Off |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/spot-margin-trade/set-auto-repay-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672299806626
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "currency": "ETH",
    "autoRepayMode":"1"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.set_auto_repay_mode(
    currency="ETH",
    autoRepayMode="1"
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "success",
    "result": {
        "data": [
            {
                "currency": "ETH",
                "autoRepayMode": "1"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1766976677678
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Set Leverage

Set the user's maximum leverage in spot cross margin

caution

Your account needs to activate spot margin first; i.e., you must have finished the quiz on web / app.

The updated leverage must be less than or equal to the maximum leverage of the currency

### HTTP Request

`POST /v5/spot-margin-trade/set-leverage`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| leverage | **true** | string | Leverage. \[`2`, `10`\]. |
| currency | false | string | Coin name, uppercase only |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/spot-margin-uta/set-leverage)

* * *

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/spot-margin-trade/set-leverage HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672299806626
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "leverage": "4"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_set_leverage(
    leverage="4",
))
```

```javascript
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .setSpotMarginLeverage('4')
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
    "time": 1672710944282
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Status And Leverage

Query the Spot margin status and leverage

### HTTP Request

`GET /v5/spot-margin-trade/state`

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| spotLeverage | string | Spot margin leverage. Returns `""` if the margin trade is turned off |
| spotMarginMode | string | Spot margin status. `1`: on, `0`: off |
| effectiveLeverage | string | actual leverage ratio. Precision retains 2 decimal places, truncate downwards |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/spot-margin-uta/status)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/state HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1692696840996
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_status_and_leverage())
```

```javascript
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .getSpotMarginState()
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
        "spotLeverage": "10",
        "spotMarginMode": "1",
        "effectiveLeverage": "1"
    },
    "retExtInfo": {},
    "time": 1692696841231
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Toggle Margin Trade

Turn on / off spot margin trade

caution

Your account needs to activate spot margin first; i.e., you must have finished the quiz on web / app.

### HTTP Request

`POST /v5/spot-margin-trade/switch-mode`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| spotMarginMode | **true** | string | `1`: on, `0`: off |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| spotMarginMode | string | Spot margin status. `1`: on, `0`: off |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/spot-margin-uta/switch-mode)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/spot-margin-trade/switch-mode HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1672297794480
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json

{
    "spotMarginMode": "0"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_toggle_margin_trade(
    spotMarginMode="0",
))
```

```javascript
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .toggleSpotMarginTrade('0')
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
        "spotMarginMode": "0"
    },
    "retExtInfo": {},
    "time": 1672297795542
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Tiered Collateral Ratio

UTA loan tiered collateral ratio

> Info

Does not need authentication.

### HTTP Request

`GET /v5/spot-margin-trade/collateral`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | Coin name |
| \> collateralRatioList | array | Object |
| >\> maxQty | string | Upper limit(in coin) of the tiered range, `""` means positive infinity |
| >\> minQty | string | lower limit(in coin) of the tiered range |
| >\> collateralRatio | string | Collateral ratio |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/collateral?currency=BTC HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_tiered_collateral_ratio(
    currency="BTC",
))
```


### Response Example

```json
{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
        "list": [
            {
                "currency": "BTC",
                "collateralRatioList": [
                    {
                        "minQty": "0",
                        "maxQty": "1000000",
                        "collateralRatio": "0.85"
                    },
                    {
                        "minQty": "1000000",
                        "maxQty": "",
                        "collateralRatio": "0"
                    }
                ]
            }
        ]
    },
    "retExtInfo": "{}",
    "time": 1739848984945
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get VIP Margin Data

This margin data is for **Unified account** in particular.

> Info

Does not need authentication.

### HTTP Request

`GET /v5/spot-margin-trade/data`

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| [vipLevel](https://bybit-exchange.github.io/docs/v5/enum#viplevel) | false | string | VIP level |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| vipCoinList | array | Object |
| \> list | array | Object |
| >\> borrowable | boolean | Whether it is allowed to be borrowed |
| >\> collateralRatio | string | Due to the new Tiered Collateral value logic, this field will no longer be accurate starting on February 19, 2025. Please refer to [Get Tiered Collateral Ratio](https://bybit-exchange.github.io/docs/v5/spot-margin-uta/tier-collateral-ratio) |
| >\> currency | string | Coin name |
| >\> hourlyBorrowRate | string | Borrow interest rate per hour |
| >\> liquidationOrder | string | Liquidation order |
| >\> marginCollateral | boolean | Whether it can be used as a margin collateral currency |
| >\> maxBorrowingAmount | string | Max borrow amount |
| \> vipLevel | string | VIP level |

[RUN >>](https://bybit-exchange.github.io/docs/api-explorer/v5/spot-margin-uta/vip-margin)

* * *

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/spot-margin-trade/data?vipLevel=No VIP&currency=BTC HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.spot_margin_trade_get_vip_margin_data())
```

```javascript
const { RestClientV5 } = require('bybit-api');

const client = new RestClientV5({
  testnet: true,
  key: 'xxxxxxxxxxxxxxxxxx',
  secret: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
});

client
  .getVIPMarginData({
    vipLevel: 'No VIP',
    currency: 'BTC',
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
    "retMsg": "success",
    "result": {
        "vipCoinList": [
            {
                "list": [
                    {
                        "borrowable": true,
                        "collateralRatio": "0.95",
                        "currency": "BTC",
                        "hourlyBorrowRate": "0.0000015021220000",
                        "liquidationOrder": "11",
                        "marginCollateral": true,
                        "maxBorrowingAmount": "3"
                    }
                ],
                "vipLevel": "No VIP"
            }
        ]
    }
}
```





- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---
