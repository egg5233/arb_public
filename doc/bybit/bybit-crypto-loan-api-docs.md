# Bybit Crypto Loan API

Source: https://bybit-exchange.github.io/docs/v5/crypto-loan/ + https://bybit-exchange.github.io/docs/v5/new-crypto-loan/

---

# Get Account Borrowable/Collateralizable Limit

Query for the minimum and maximum amounts your account can borrow and how much collateral you can put up.

> Permission: "Spot trade"

### HTTP Request

GET`/v5/crypto-loan/borrowable-collateralisable-number`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanCurrency | **true** | string | Loan coin name |
| collateralCurrency | **true** | string | Collateral coin name |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| collateralCurrency | string | Collateral coin name |
| loanCurrency | string | Loan coin name |
| maxCollateralAmount | string | Max. limit to mortgage |
| maxLoanAmount | string | Max. limit to borrow |
| minCollateralAmount | string | Min. limit to mortgage |
| minLoanAmount | string | Min. limit to borrow |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/borrowable-collateralisable-number?loanCurrency=USDT&collateralCurrency=BTC HTTP/1.1
Host: api.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728627083198
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_account_borrowable_or_collateralizable_limit(
    loanCurrency="USDT",
    collateralCurrency="BTC",
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
  .getAccountBorrowCollateralLimit({
    loanCurrency: 'USDT',
    collateralCurrency: 'BTC',
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
    "retMsg": "request.success",
    "result": {
        "collateralCurrency": "BTC",
        "loanCurrency": "USDT",
        "maxCollateralAmount": "164.957732055526752104",
        "maxLoanAmount": "8000000",
        "minCollateralAmount": "0.000412394330138818",
        "minLoanAmount": "20"
    },
    "retExtInfo": {},
    "time": 1728627084863
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Adjust Collateral Amount

You can increase or reduce your collateral amount. When you reduce, please obey the [max. allowed reduction amount](https://bybit-exchange.github.io/docs/v5/crypto-loan/reduce-max-collateral-amt).

> Permission: "Spot trade"

info

- The adjusted collateral amount will be returned to or deducted from the Funding wallet.

### HTTP Request

POST`/v5/crypto-loan/adjust-ltv`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | **true** | string | Loan order ID |
| amount | **true** | string | Adjustment amount |
| direction | **true** | string | `0`: add collateral; `1`: reduce collateral |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| adjustId | string | Collateral adjustment transaction ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan/adjust-ltv HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728635421137
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 85

{
    "orderId": "1794267532472646144",
    "amount": "0.001",
    "direction": "1"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.adjust_collateral_amount(
    orderId="1794267532472646144",
    amount="0.001",
    direction="1",
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
  .adjustCollateralAmount({
    orderId: '1794267532472646144',
    amount: '0.001',
    direction: '1',
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
    "retMsg": "request.success",
    "result": {
        "adjustId": "1794318409405331968"
    },
    "retExtInfo": {},
    "time": 1728635422833
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Collateral Coins

info

Does not need authentication.

### HTTP Request

GET`/v5/crypto-loan/collateral-data`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| vipLevel | false | string | VIP level <br>- `VIP0`, `VIP1`, `VIP2`, `VIP3`, `VIP4`, `VIP5`, `VIP99`(supreme VIP)<br>- `PRO1`, `PRO2`, `PRO3`, `PRO4`, `PRO5`, `PRO6` |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| vipCoinList | array | Object |
| \> list | array | Object |
| >\> collateralAccuracy | integer | Valid collateral coin precision |
| >\> initialLTV | string | The Initial LTV ratio determines the initial amount of coins that can be borrowed. The initial LTV ratio may vary for different collateral |
| >\> marginCallLTV | string | If the LTV ratio (Loan Amount/Collateral Amount) reaches the threshold, you will be required to add more collateral to your loan |
| >\> liquidationLTV | string | If the LTV ratio (Loan Amount/Collateral Amount) reaches the threshold, Bybit will liquidate your collateral assets to repay your loan and interest in full |
| >\> maxLimit | string | Collateral limit |
| \> vipLevel | string | VIP level |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/collateral-data?currency=ETH&vipLevel=PRO1 HTTP/1.1
Host: api.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_collateral_coins(
    currency="ETH",
    vipLevel="PRO1",
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
  .getCollateralCoins({
    currency: 'ETH',
    vipLevel: 'PRO1',
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
    "retMsg": "request.success",
    "result": {
        "vipCoinList": [
            {
                "list": [
                    {
                        "collateralAccuracy": 8,
                        "currency": "ETH",
                        "initialLTV": "0.8",
                        "liquidationLTV": "0.95",
                        "marginCallLTV": "0.87",
                        "maxLimit": "32000"
                    }
                ],
                "vipLevel": "PRO1"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1728618590498
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Completed Loan History

Query for the last 6 months worth of your completed (fully paid off) loans.

> Permission: "Spot trade"

### HTTP Request

GET`/v5/crypto-loan/borrow-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| loanCurrency | false | string | Loan coin name |
| collateralCurrency | false | string | Collateral coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> borrowTime | string | The timestamp to borrow |
| \> collateralCurrency | string | Collateral coin |
| \> expirationTime | string | Loan maturity time, keeps `""` for flexible loan |
| \> hourlyInterestRate | string | Hourly interest rate <br>- Flexible loan, it is real-time interest rate<br>- Fixed term loan: it is fixed term interest rate |
| \> initialCollateralAmount | string | Initial amount to mortgage |
| \> initialLoanAmount | string | Initial loan amount |
| \> loanCurrency | string | Loan coin |
| \> loanTerm | string | Loan term, `7`, `14`, `30`, `90`, `180` days, keep `""` for flexible loan |
| \> orderId | string | Loan order ID |
| \> repaidInterest | string | Total interest repaid |
| \> repaidPenaltyInterest | string | Total penalty interest repaid |
| \> status | integer | Loan order status `1`: fully repaid manually; `2`: fully repaid by liquidation |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/borrow-history?orderId=1793683005081680384 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728630979731
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_completed_loan_history(
        orderId="1793683005081680384",
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
  .getCompletedLoanOrderHistory({ orderId: '1794267532472646144' })
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
    "retMsg": "request.success",
    "result": {
        "list": [
            {
                "borrowTime": "1728546174028",
                "collateralCurrency": "BTC",
                "expirationTime": "1729148399000",
                "hourlyInterestRate": "0.0000010241",
                "initialCollateralAmount": "0.0494727",
                "initialLoanAmount": "1",
                "loanCurrency": "ETH",
                "loanTerm": "7",
                "orderId": "1793569729874260992",
                "repaidInterest": "0.00000515",
                "repaidPenaltyInterest": "0",
                "status": 1
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1728632014857
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrowable Coins

info

Does not need authentication.

danger

Borrowed coins can be returned at any time before the due date. You'll be charged 3 times the hourly interest during the overdue period. Your collateral will be liquidated to repay a loan and the interest if you fail to make the repayment 48 hours after the due time.

### HTTP Request

GET`/v5/crypto-loan/loanable-data`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| vipLevel | false | string | VIP level <br>- `VIP0`, `VIP1`, `VIP2`, `VIP3`, `VIP4`, `VIP5`, `VIP99`(supreme VIP)<br>- `PRO1`, `PRO2`, `PRO3`, `PRO4`, `PRO5`, `PRO6` |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| vipCoinList | array | Object |
| \> list | array | Object |
| >\> borrowingAccuracy | integer | The number of decimal places (precision) of this coin |
| >\> currency | string | Coin name |
| >\> flexibleHourlyInterestRate | string | Flexible hourly variable interest rate <br>- Flexible Crypto Loans offer an hourly variable interest rate, calculated based on the actual borrowing time per hour, with the option for early repayment<br>- Is `""` if the coin does not support flexible loan |
| >\> hourlyInterestRate7D | string | Hourly interest rate for 7 days loan. Is `""` if the coin does not support 7 days loan |
| >\> hourlyInterestRate14D | string | Hourly interest rate for 14 days loan. Is `""` if the coin does not support 14 days loan |
| >\> hourlyInterestRate30D | string | Hourly interest rate for 30 days loan. Is `""` if the coin does not support 30 days loan |
| >\> hourlyInterestRate90D | string | Hourly interest rate for 90 days loan. Is `""` if the coin does not support 90 days loan |
| >\> hourlyInterestRate180D | string | Hourly interest rate for 180 days loan. Is `""` if the coin does not support 180 days loan |
| >\> maxBorrowingAmount | string | Max. amount to borrow |
| >\> minBorrowingAmount | string | Min. amount to borrow |
| \> vipLevel | string | VIP level |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/loanable-data?currency=USDT&vipLevel=VIP0 HTTP/1.1
Host: api.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_borrowable_coins(
    currency="USDT",
    vipLevel="VIP0",
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
  .getBorrowableCoins({
    currency: 'USDT',
    vipLevel: 'VIP0',
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
    "retMsg": "request.success",
    "result": {
        "vipCoinList": [
            {
                "list": [
                    {
                        "borrowingAccuracy": 4,
                        "currency": "USDT",
                        "flexibleHourlyInterestRate": "0.0000090346",
                        "hourlyInterestRate14D": "0.0000207796",
                        "hourlyInterestRate180D": "",
                        "hourlyInterestRate30D": "0.00002349",
                        "hourlyInterestRate7D": "0.0000180692",
                        "hourlyInterestRate90D": "",
                        "maxBorrowingAmount": "8000000",
                        "minBorrowingAmount": "20"
                    }
                ],
                "vipLevel": "VIP0"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1728619315868
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Loan LTV Adjustment History

Query for your LTV adjustment history.

> Permission: "Spot trade"

info

- Support querying last 6 months adjustment transactions
- Only the ltv adjustment transactions launched by the user can be queried

### HTTP Request

GET`/v5/crypto-loan/adjustment-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| adjustId | false | string | Collateral adjustment transaction ID |
| collateralCurrency | false | string | Collateral coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> collateralCurrency | string | Collateral coin |
| \> orderId | string | Loan order ID |
| \> adjustId | string | Collateral adjustment transaction ID |
| \> adjustTime | string | Adjust timestamp |
| \> preLTV | string | LTV before the adjustment |
| \> afterLTV | string | LTV after the adjustment |
| \> direction | integer | The direction of adjustment, `0`: add collateral; `1`: reduce collateral |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/adjustment-history?adjustId=1794318409405331968 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728635871668
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_crypto_loan_ltv_adjustment_history(
    adjustId="1794318409405331968",
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
  .getLoanLTVAdjustmentHistory({ adjustId: '1794271131730737664' })
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
    "retMsg": "request.success",
    "result": {
        "list": [
            {
                "adjustId": "1794318409405331968",
                "adjustTime": "1728635422814",
                "afterLTV": "0.7164",
                "amount": "0.001",
                "collateralCurrency": "BTC",
                "direction": 1,
                "orderId": "1794267532472646144",
                "preLTV": "0.6546"
            }
        ],
        "nextPageCursor": "1844656778923966466"
    },
    "retExtInfo": {},
    "time": 1728635873329
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Max. Allowed Collateral Reduction Amount

Query for the maximum amount by which collateral may be reduced by.

> Permission: "Spot trade"

### HTTP Request

GET`/v5/crypto-loan/max-collateral-amount`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | **true** | string | Loan coin ID |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| maxCollateralAmount | string | Max. reduction collateral amount |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/max-collateral-amount?orderId=1794267532472646144 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728634289933
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_max_allowed_collateral_reduction_amount(
        orderId="1794267532472646144",
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
  .getMaxAllowedReductionCollateralAmount({ orderId: '1794267532472646144' })
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
    "retMsg": "request.success",
    "result": {
        "maxCollateralAmount": "0.00210611"
    },
    "retExtInfo": {},
    "time": 1728634291554
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Repay

Fully or partially repay a loan. If interest is due, that is paid off first, with the loaned amount being paid off only after due interest.

> Permission: "Spot trade"

info

- The repaid amount will be deducted from the Funding wallet.
- The collateral amount will not be auto returned when you don't fully repay the debt, but you can also adjust collateral amount

### HTTP Request

POST`/v5/crypto-loan/repay`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | **true** | string | Loan order ID |
| amount | **true** | string | Repay amount |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| repayId | string | Repayment transaction ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan/repay HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728629785224
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 61

{
    "orderId": "1794267532472646144",
    "amount": "100"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay_crypto_loan(
        orderId="1794267532472646144",
        amount="100",
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
  .repayCryptoLoan({
    orderId: '1794267532472646144',
    amount: '100',
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
    "retMsg": "request.success",
    "result": {
        "repayId": "1794271131730737664"
    },
    "retExtInfo": {},
    "time": 1728629786884
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Loan Repayment History

Query for loan repayment transactions. A loan may be repaid in multiple repayments.

> Permission: "Spot trade"

info

- Supports querying for the last 6 months worth of completed loan orders.
- Only successful repayments can be queried for.

### HTTP Request

GET`/v5/crypto-loan/repayment-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| repayId | false | string | Repayment tranaction ID |
| loanCurrency | false | string | Loan coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> collateralCurrency | string | Collateral coin |
| \> collateralReturn | string | Amount of collateral returned as a result of this repayment. `"0"` if this isn't the final loan repayment |
| \> loanCurrency | string | Loan coin |
| \> loanTerm | string | Loan term, `7`, `14`, `30`, `90`, `180` days, keep `""` for flexible loan |
| \> orderId | string | Loan order ID |
| \> repayAmount | string | Repayment amount |
| \> repayId | string | Repayment transaction ID |
| \> repayStatus | integer | Repayment status, `1`: success; `2`: processing |
| \> repayTime | string | Repay timestamp |
| \> repayType | string | Repayment type, `1`: repay by user; `2`: repay by liquidation |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/repayment-history?repayId=1794271131730737664 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728633716794
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_loan_repayment_history(
        repayId="1794271131730737664",
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
  .getRepaymentHistory({ repayId: '1794271131730737664' })
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
    "retMsg": "request.success",
    "result": {
        "list": [
            {
                "collateralCurrency": "BTC",
                "collateralReturn": "0",
                "loanCurrency": "USDT",
                "loanTerm": "",
                "orderId": "1794267532472646144",
                "repayAmount": "100",
                "repayId": "1794271131730737664",
                "repayStatus": 1,
                "repayTime": "1728629786875",
                "repayType": "1"
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1728633717935
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Unpaid Loans

Query for your ongoing loans.

> Permission: "Spot trade"

### HTTP Request

GET`/v5/crypto-loan/ongoing-orders`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| loanCurrency | false | string | Loan coin name |
| collateralCurrency | false | string | Collateral coin name |
| loanTermType | false | string | - `1`: fixed term, when query this type, `loanTerm` must be filled<br>- `2`: flexible term<br>By default, query all types |
| loanTerm | false | string | `7`, `14`, `30`, `90`, `180` days, working when `loanTermType`=1 |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> collateralAmount | string | Collateral amount |
| \> collateralCurrency | string | Collateral coin |
| \> currentLTV | string | Current LTV |
| \> expirationTime | string | Loan maturity time, keeps `""` for flexible loan |
| \> hourlyInterestRate | string | Hourly interest rate <br>- Flexible loan, it is real-time interest rate<br>- Fixed term loan: it is fixed term interest rate |
| \> loanCurrency | string | Loan coin |
| \> loanTerm | string | Loan term, `7`, `14`, `30`, `90`, `180` days, keep `""` for flexible loan |
| \> orderId | string | Loan order ID |
| \> residualInterest | string | Unpaid interest |
| \> residualPenaltyInterest | string | Unpaid penalty interest |
| \> totalDebt | string | Unpaid principal |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan/ongoing-orders?orderId=1793683005081680384 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: xxxxxxxxxxxxxxxxxx
X-BAPI-TIMESTAMP: 1728630979731
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay_crypto_loan(
        orderId="1794267532472646144",
        amount="100",
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
  .getUnpaidLoanOrders({ orderId: '1793683005081680384' })
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
    "retMsg": "request.success",
    "result": {
        "list": [
            {
                "collateralAmount": "0.0964687",
                "collateralCurrency": "BTC",
                "currentLTV": "0.4161",
                "expirationTime": "1731149999000",
                "hourlyInterestRate": "0.0000010633",
                "loanCurrency": "USDT",
                "loanTerm": "30",
                "orderId": "1793683005081680384",
                "residualInterest": "0.04016",
                "residualPenaltyInterest": "0",
                "totalDebt": "1888.005198"
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1728630980861
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Adjust Collateral Amount

You can increase or reduce your collateral amount. When you reduce, please obey the [Get Max. Allowed Collateral Reduction Amount](https://bybit-exchange.github.io/docs/v5/new-crypto-loan/reduce-max-collateral-amt)

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- The adjusted collateral amount will be returned to or deducted from the Funding wallet.

### HTTP Request

POST`/v5/crypto-loan-common/adjust-ltv`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Collateral coin |
| amount | **true** | string | Adjustment amount |
| direction | **true** | string | `0`: add collateral; `1`: reduce collateral |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| adjustId | long | Collateral adjustment transaction ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-common/adjust-ltv HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752627997649
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 69

{
    "currency": "BTC",
    "amount": "0.08",
    "direction": "1"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.adjust_collateral_amount_new_crypto_loan(
    currency="BTC",
    amount="0.08",
    direction="1",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "adjustId": 27511
    },
    "retExtInfo": {},
    "time": 1752627997915
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Collateral Coins

info

Does not need authentication.

### HTTP Request

GET`/v5/crypto-loan-common/collateral-data`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| collateralRatioConfigList | array | Object |
| \> collateralRatioList | array | Object |
| >\> collateralRatio | string | Collateral ratio |
| >\> maxValue | string | Max qty |
| >\> minValue | string | Min qty |
| \> currencies | string | Currenies with the same collateral ratio, e.g., `BTC,ETH,XRP` |
| currencyLiquidationList | array | Object |
| \> currency | string | Coin name |
| \> liquidationOrder | integer | Liquidation order |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-common/collateral-data?currency=BTC HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_collateral_coins_new_crypto_loan(
    currency="BTC",
    amount="0.08",
    direction="1",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "collateralRatioConfigList": [
            {
                "collateralRatioList": [
                    {
                        "collateralRatio": "0.8",
                        "maxValue": "10000",
                        "minValue": "0"
                    },
                    {
                        "collateralRatio": "0.7",
                        "maxValue": "20000",
                        "minValue": "10000"
                    },
                    {
                        "collateralRatio": "0.5",
                        "maxValue": "30000",
                        "minValue": "20000"
                    },
                    {
                        "collateralRatio": "0.4",
                        "maxValue": "99999999999",
                        "minValue": "30000"
                    }
                ],
                "currencies": "ATOM,AAVE,BTC,BOB"
            }
        ],
        "currencyLiquidationList": [
            {
                "currency": "BTC",
                "liquidationOrder": 1
            }
        ]
    },
    "retExtInfo": {},
    "time": 1752627381571
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Crypto Loan Position

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-common/position`Copy

### Request Parameters

None

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| borrowList | array | Object |
| \> fixedTotalDebt | string | Total debt of fixed loan (coin) |
| \> fixedTotalDebtUSD | string | Total debt of fixed loan (USD) |
| \> flexibleHourlyInterestRate | string | Flebible loan hourly interest rate |
| \> flexibleTotalDebt | string | Total debt of flexible loan (coin) |
| \> flexibleTotalDebtUSD | string | Total debt of flexible loan (USD) |
| \> loanCurrency | string | Loan coin |
| collateralList | array | Object |
| \> amount | string | Collateral amount in coin |
| \> amountUSD | string | Collateral amount in USD (after tierd collateral ratio calculation) |
| \> currency | string | Collateral coin |
| ltv | string | LTV |
| supplyList | array | Object |
| \> amount | string | Supply amount in coin |
| \> amountUSD | string | Supply amount in USD |
| \> currency | string | Supply coin |
| totalCollateral | string | Total collateral amount (USD) |
| totalDebt | string | Total debt (fixed + flexible, in USD) |
| totalSupply | string | Total supply amount (USD) |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-common/position HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752628288472
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_position_new_crypto_loan())
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "borrowList": [
            {
                "fixedTotalDebt": "0",
                "fixedTotalDebtUSD": "0",
                "flexibleHourlyInterestRate": "0.0000001361462",
                "flexibleTotalDebt": "0.08800022",
                "flexibleTotalDebtUSD": "9355.37",
                "loanCurrency": "BTC"
            },
            {
                "fixedTotalDebt": "0.1",
                "fixedTotalDebtUSD": "282.8",
                "flexibleHourlyInterestRate": "0.00000188498892",
                "flexibleTotalDebt": "0",
                "flexibleTotalDebtUSD": "0",
                "loanCurrency": "ETH"
            }
        ],
        "collateralList": [
            {
                "amount": "0.12",
                "amountUSD": "9930.11",
                "currency": "BTC"
            },
            {
                "amount": "2",
                "amountUSD": "4524.81",
                "currency": "ETH"
            },
            {
                "amount": "4002.12",
                "amountUSD": "3201.69",
                "currency": "USDT"
            },
            {
                "amount": "1000",
                "amountUSD": "724.8",
                "currency": "USDC"
            }
        ],
        "ltv": "0.524344",
        "supplyList": [
            {
                "amount": "800.13041095890410959",
                "amountUSD": "800.13",
                "currency": "USDT"
            }
        ],
        "totalCollateral": "18381.41",
        "totalDebt": "9638.17",
        "totalSupply": "800.13"
    },
    "retExtInfo": {},
    "time": 1752627962000
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Create Borrow Order

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- The loan funds are released to the Funding wallet.
- The collateral funds are deducted from the Funding wallet, so make sure you have enough collateral amount in the Funding wallet.

### HTTP Request

POST`/v5/crypto-loan-fixed/borrow`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderCurrency | **true** | string | Currency to borrow |
| orderAmount | **true** | string | Amount to borrow |
| annualRate | **true** | string | Customizable annual interest rate, e.g., `0.02` means 2% |
| term | **true** | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| autoRepay | false | string | Deprecated. Enable Auto-Repay to have assets in your Funding Account automatically repay your loan upon Borrowing order expiration, preventing overdue penalties. Ensure your Funding Account maintains sufficient amount for repayment to avoid automatic repayment failures.<br>`"true"`: enable, default; `"false"`: disable |
| repayType | false | string | `1`:Auto Repayment (default); Enable "Auto Repayment" to automatically repay your loan using assets in your funding account when it dues, avoiding overdue penalties. `2`:Transfer to flexible loan |
| collateralList | false | array<object> | Collateral coin list, supports putting up to 100 currency in the array |
| \> currency | false | string | Currency used to mortgage |
| \> amount | false | string | Amount to mortgage |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Loan order ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/borrow HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752633649752
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 208

{
    "orderCurrency": "ETH",
    "orderAmount": "1.5",
    "annualRate": "0.022",
    "term": "30",
    "autoRepay": "true",
    "collateralList": {
        "currency": "BTC",
        "amount": "0.1"
    }
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.borrow_fixed_crypto_loan(
    loanCurrency="ETH",
    loanAmount="1.5",
    annualRate="0.022",
    term="30",
    autoRepay="true",
    collateralList={
        "currency": "BTC",
        "amount": "0.1",
    },
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "orderId": "13007"
    },
    "retExtInfo": {},
    "time": 1752633650147
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrow Contract Info

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-fixed/borrow-contract-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| loanId | false | string | Loan ID |
| orderCurrency | false | string | Loan coin name |
| term | false | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> annualRate | string | Annual rate for the borrowing |
| \> autoRepay | string | Deprecated. `"true"`: enable auto repay, default; `"false"`: disable auto repay |
| \> borrowCurrency | string | Loan coin |
| \> borrowTime | string | Loan order timestamp |
| \> interestPaid | string | Paid interest |
| \> loanId | string | Loan contract ID |
| \> orderId | string | Loan order ID |
| \> repaymentTime | string | Time to repay |
| \> residualPenaltyInterest | string | Unpaid interest |
| \> residualPrincipal | string | Unpaid principal |
| \> status | integer | Loan order status `1`: unrepaid; `2`: fully repaid; `3`: overdue |
| \> term | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| \> repayType | string | `1`:Auto Repayment; `2`:Transfer to flexible loan; `0`: No Automatic Repayment. Compatible with existing orders; |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/borrow-contract-info?orderCurrency=ETH HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752652691909
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrowing_contract_info_fixed_crypto_loan(
    collateralCurrency="ETH",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "annualRate": "0.022",
                "autoRepay": "true",
                "borrowCurrency": "ETH",
                "borrowTime": "1752633756068",
                "interestPaid": "0.002531506849315069",
                "loanId": "571",
                "orderId": "13007",
                "repayType": "1",
                "repaymentTime": "1755225756068",
                "residualPenaltyInterest": "0",
                "residualPrincipal": "1.4",
                "status": 1,
                "term": "30"
            },
            {
                "annualRate": "0.022",
                "autoRepay": "true",
                "borrowCurrency": "ETH",
                "borrowTime": "1752633696068",
                "interestPaid": "0.00018082191780822",
                "loanId": "570",
                "orderId": "13007",
                "repayType": "1",
                "repaymentTime": "1755225696068",
                "residualPenaltyInterest": "0",
                "residualPrincipal": "0.1",
                "status": 1,
                "term": "30"
            }
        ],
        "nextPageCursor": "568"
    },
    "retExtInfo": {},
    "time": 1752652692603
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrowing Market

info

Does not need authentication.

If you want to borrow, you can use this endpoint to check whether there are any suitable counterparty supply orders available.

### HTTP Request

GET`/v5/crypto-loan-fixed/borrow-order-quote`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderCurrency | **true** | string | Coin name |
| orderBy | **true** | string | Order by, `apy`: annual rate; `term`; `quantity` |
| term | false | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| sort | false | integer | `0`: ascend, default; `1`: descend |
| limit | false | integer | Limit for data size per page. \[`1`, `100`\]. Default: `10` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> orderCurrency | string | Coin name |
| \> term | integer | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| \> annualRate | string | Annual rate |
| \> qty | string | Quantity |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/borrow-order-quote?orderCurrency=USDT&orderBy=apy HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrowing_market_fixed_crypto_loan(
    orderCurrency="USDT",
    orderBy="apy",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "annualRate": "0.04",
                "orderCurrency": "USDT",
                "qty": "988.78",
                "term": 14
            }
        ]
    },
    "retExtInfo": {},
    "time": 1752719158890
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrow Order Info

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-fixed/borrow-order-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| orderCurrency | false | string | Loan coin name |
| state | false | string | Borrow order status, `1`: matching; `2`: partially filled and cancelled; `3`: Fully filled; `4`: Cancelled |
| term | false | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> annualRate | string | Annual rate for the borrowing |
| \> orderId | long | Loan order ID |
| \> orderTime | string | Order created time |
| \> filledQty | string | Filled qty |
| \> orderQty | string | Order qty |
| \> orderCurrency | string | Coin name |
| \> state | integer | Borrow order status, `1`: matching; `2`: partially filled and cancelled; `3`: Fully filled; `4`: Cancelled; `5`: fail |
| \> term | integer | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| \> repayType | string | `1`:Auto Repayment; `2`:Transfer to flexible loan; `0`: No Automatic Repayment. Compatible with existing orders; |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/borrow-order-info?orderId=13010 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752655239825
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrowing_orders_fixed_crypto_loan(
    orderId="13010"
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "annualRate": "0.01",
                "filledQty": "0",
                "orderCurrency": "MANA",
                "orderId": 13010,
                "orderQty": "2000",
                "orderTime": "1752654035179",
                "repayType": "2",
                "state": 1,
                "term": 30
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1752655241090
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Cancel Borrow Order

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

### HTTP Request

POST`/v5/crypto-loan-fixed/borrow-order-cancel`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | **true** | string | Order ID of fixed borrow order |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/borrow-order-cancel HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752652457987
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 26

{
    "orderId": "13009"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.create_lending_order_fixed_crypto_loan(
    orderId="13009",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {},
    "retExtInfo": {},
    "time": 1752652458684
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Cancel Supply Order

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

### HTTP Request

POST`/v5/crypto-loan-fixed/supply-order-cancel`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | **true** | string | Order ID of fixed supply order |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/supply-order-cancel HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752652612736
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 26

{
    "orderId": "13577"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.cancel_lending_order_fixed_crypto_loan(
    orderId="13577",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {},
    "retExtInfo": {},
    "time": 1752652613638
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Renew Borrow Order

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- The loan funds are released to the Funding wallet.
- The collateral funds are deducted from the Funding wallet, so make sure you have enough collateral amount in the Funding wallet.
- This endpoint allows you to re-borrow the principal that was previously repaid. The renewal amount is the same as the amount previously repaid on this loan.

### HTTP Request

POST`/v5/crypto-loan-fixed/renew`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanId | **true** | string | Loan ID |
| collateralList | false | array<object> | Collateral coin list, supports putting up to 100 currency in the array |
| \> currency | false | string | Currency used to mortgage |
| \> amount | false | string | Amount to mortgage |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Loan order ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/renew HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752633649752
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 208

{
    "loanId": "2364",
    "collateralList": {"currency": "ETH","amount": "1"}
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.renew_fixed_crypto_loan(
    loanId="2364",
    collateralList={
        "currency": "ETH",
        "amount": "1",
    },
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "orderId": 49
    },
    "retExtInfo": {},
    "time": 1764142142931
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Renew Order Info

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-fixed/renew-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| orderCurrency | false | string | Loan coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> borrowCurrency | string | Borrow currency |
| \> amount | string | loan amount |
| \> autoRepay | integer | `1`: Auto Repayment; `2`: Transfer to flexible loan; `0`: No Automatic Repayment. Compatible with existing orders; |
| \> contractNo | string | Contract number |
| \> dueTime | string | Due time |
| \> orderId | integer | Order Id |
| \> loanId | string | Loan Id |
| \> renewLoanNo | string | Renew Loan number |
| \> time | string | timestamps |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/renew-info HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752655239825
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_renewal_orders_fixed_crypto_loan())
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "amount": "11",
                "autoRepay": 2,
                "borrowCurrency": "USDT",
                "contractNo": "2092164378648656896",
                "dueTime": "1766750400000",
                "loanId": "2364",
                "orderId": 49,
                "renewLoanNo": "2092170365690461952",
                "time": "1764142142913"
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1764208336537
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Repay

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

### HTTP Request

POST`/v5/crypto-loan-fixed/fully-repay`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanId | false | string | Loan contract ID. Either `loanId` or `loanCurrency` needs to be passed |
| loanCurrency | false | string | Loan coin. Either `loanId` or `loanCurrency` needs to be passed |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| repayId | string | Repayment transaction ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/fully-repay HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752656296791
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 50

{
    "loanId": "570",
    "loanCurrency": "ETH"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay_fixed_crypto_loan(
    loanId="570",
    loanCurrency="ETH",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "repayId": "1771"
    },
    "retExtInfo": {},
    "time": 1752569614549
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Collateral Repayment

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

There are limits on the repayment amount in a single transaction.
Please read this [announcement](https://announcements.bybit.com/article/crypto-loan-manual-repayment-update-bltde33509ddde5e8fd/) before repaying with collateral.

When repaying with collateral, Bybit will charge a repayment fee. The applicable fee rate is the higher of the repayment fee rates for the collateral asset and the debt asset.
You can call this endpoint: [View fee rates by asset](https://www.bybit.com/x-api/spot/api/fixed-loan/v1/coin-config) to get "reapyFee" where "pledgeEnable" = 1 for coins' repayment fee rates.

info

**fixed currency offset logic**

1. From Currency Perspective
   - Orders with the closest maturity date will be sorted in descending order.
   - If the maturity date is the same, the order with the higher interest rate will be prioritized.
   - If the interest rates are the same, the order will be processed randomly.Orders will be processed sequentially. Within an order, interest will be repaid first, followed by principal.
2. From Order Perspective
   - Interest will be repaid first, followed by principal.

### HTTP Request

POST`/v5/crypto-loan-fixed/repay-collateral`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanId | false | string | Loan contract ID. If not passed, the fixed currency offset logic will apply. |
| loanCurrency | **true** | string | Loan coin name |
| collateralCoin | **true** | string | Collateral currencies: Use commas to separate multiple collateral currencies |
| amount | **true** | string | Repay amount |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/repay-collateral HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752656296791
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 50
{
  "loanCurrency": "ETH",
  "amount": "0.1",
  "collateralCoin":"USDT"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.collateral_repayment_fixed_crypto_loan(
    loanCurrency="ETH",
    amount="0.1",
    collateralCoin="USDT",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {},
    "retExtInfo": {},
    "time": 1756973819393
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Repayment History

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-fixed/repayment-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| repayId | false | string | Repayment order ID |
| loanCurrency | false | string | Loan coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> details | array | Object |
| >\> loanCurrency | string | Loan coin name |
| >\> repayAmount | long | Repay amount |
| >\> loanId | string | Loan ID. One repayment may involve multiple loan contracts. |
| \> loanCurrency | string | Loan coin name |
| \> repayAmount | long | Repay amount |
| \> repayId | string | Repay order ID |
| \> repayStatus | integer | Status, `1`: success, `2`: processing, `3`: fail |
| \> repayTime | long | Repay time |
| \> repayType | integer | Repay type, `1`: repay by user; `2`: repay by liquidation; `3`: auto repay; `4`: overdue repay; `5`: repay by delisting; `6`: repay by delay liquidation; `7`: repay by currency; `8`: transfer to flexible loan |
| nextPageCursor | string | Refer to the `cursor` request parameter |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/repayment-history?repayId=1780 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXXX
X-BAPI-API-KEY: XXXXXXX
X-BAPI-TIMESTAMP: 1752714738425
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_repayment_history_fixed_crypto_loan(
    repayId="1780",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "details": [
                    {
                        "loanCurrency": "ETH",
                        "loanId": "568",
                        "repayAmount": "0.1"
                    },
                    {
                        "loanCurrency": "ETH",
                        "loanId": "571",
                        "repayAmount": "1.4"
                    }
                ],
                "loanCurrency": "ETH",
                "repayAmount": "1.5",
                "repayId": "1782",
                "repayStatus": 1,
                "repayTime": 1752717174353,
                "repayType": 1
            }
        ],
        "nextPageCursor": "1674"
    },
    "retExtInfo": {},
    "time": 1752717183557
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Create Supply Order

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

### HTTP Request

POST`/v5/crypto-loan-fixed/supply`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderCurrency | **true** | string | Currency to supply |
| orderAmount | **true** | string | Amount to supply |
| annualRate | **true** | string | Customizable annual interest rate, e.g., `0.02` means 2% |
| term | **true** | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Supply order ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-fixed/supply HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752652261840
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 104

{
    "orderCurrency": "USDT",
    "orderAmount": "2002.21",
    "annualRate": "0.35",
    "term": "7"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.create_lending_order_fixed_crypto_loan(
    orderCurrency="USDT",
    orderAmount="2002.21",
    annualRate="0.35",
    term="7",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "orderId": "13007"
    },
    "retExtInfo": {},
    "time": 1752633650147
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Lending Market

info

Does not need authentication.

If you want to supply, you can use this endpoint to check whether there are any suitable counterparty borrow orders available.

### HTTP Request

GET`/v5/crypto-loan-fixed/supply-order-quote`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderCurrency | **true** | string | Coin name |
| term | false | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| orderBy | **true** | string | Order by, `apy`: annual rate; `term`; `quantity` |
| sort | false | integer | `0`: ascend, default; `1`: descend |
| limit | false | integer | Limit for data size per page. \[`1`, `100`\]. Default: `10` |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> orderCurrency | string | Coin name |
| \> term | integer | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| \> annualRate | string | Annual rate |
| \> qty | string | Quantity |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/supply-order-quote?orderCurrency=USDT&orderBy=apy HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_lending_market_fixed_crypto_loan(
    orderCurrency="USDT",
    orderBy="apy",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "annualRate": "0.02",
                "orderCurrency": "USDT",
                "qty": "1000.1234",
                "term": 60
            },
            {
                "annualRate": "0.022",
                "orderCurrency": "USDT",
                "qty": "212.1234",
                "term": 7
            }
        ]
    },
    "retExtInfo": {},
    "time": 1752652136224
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Supply Order Info

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-fixed/supply-order-info`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Supply order ID |
| orderCurrency | false | string | Supply coin name |
| state | false | string | Supply order status, `1`: matching; `2`: partially filled and cancelled; `3`: Fully filled; `4`: Cancelled |
| term | false | string | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> annualRate | string | Annual rate for the supply |
| \> orderId | long | Supply order ID |
| \> orderTime | string | Order created time |
| \> filledQty | string | Filled qty |
| \> orderQty | string | Order qty |
| \> orderCurrency | string | Coin name |
| \> state | integer | Supply order status, `1`: matching; `2`: partially filled and cancelled; `3`: Fully filled; `4`: Cancelled; `5`: fail |
| \> term | integer | Fixed term `7`: 7 days; `14`: 14 days; `30`: 30 days; `90`: 90 days; `180`: 180 days |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-fixed/supply-order-info?orderId=13564 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752655992606
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_lending_orders_fixed_crypto_loan(
    orderId="13564",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "annualRate": "0.01",
                "filledQty": "800",
                "orderCurrency": "USDT",
                "orderId": 13564,
                "orderQty": "1020",
                "orderTime": "1752482751043",
                "state": 2,
                "term": 7
            }
        ],
        "nextPageCursor": ""
    },
    "retExtInfo": {},
    "time": 1752655993869
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Borrow

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- The loan funds are released to the Funding wallet.
- The collateral funds are deducted from the Funding wallet, so make sure you have enough collateral amount in the Funding wallet.

### HTTP Request

POST`/v5/crypto-loan-flexible/borrow`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanCurrency | **true** | string | Loan coin name |
| loanAmount | **true** | string | Amount to borrow |
| collateralList | false | array<object> | Collateral coin list, supports putting up to 100 currency in the array |
| \> currency | false | string | Currency used to mortgage |
| \> amount | false | string | Amount to mortgage |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| orderId | string | Loan order ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-flexible/borrow HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752569210041
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 244

{
    "loanCurrency": "BTC",
    "loanAmount": "0.1",
    "collateralList": [
        {
            "currency": "USDT",
            "amount": "1000"
        },
        {
            "currency": "ETH",
            "amount": "1"
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
print(session.borrow_flexible_crypto_loan(
    loanCurrency="BTC",
    loanAmount="0.1",
    collateralList=[
        {
            "currency": "USDT",
            "amount": "1000"
        },
        {
            "currency": "ETH",
            "amount": "1"
        }
    ]
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "orderId": "1363"
    },
    "retExtInfo": {},
    "time": 1752569209682
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrowing History

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-flexible/borrow-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| orderId | false | string | Loan order ID |
| loanCurrency | false | string | Loan coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> borrowTime | long | The timestamp to borrow |
| \> initialLoanAmount | string | Loan amount |
| \> loanCurrency | string | Loan coin |
| \> orderId | string | Loan order ID |
| \> status | integer | Loan order status `1`: success; `2`: processing; `3`: fail |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-flexible/borrow-history?limit=2 HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752570519918
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_borrowing_history_flexible_crypto_loan(
    limit="2",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "borrowTime": 1752569950643,
                "initialLoanAmount": "0.006",
                "loanCurrency": "BTC",
                "orderId": "1364",
                "status": 1
            },
            {
                "borrowTime": 1752569209643,
                "initialLoanAmount": "0.1",
                "loanCurrency": "BTC",
                "orderId": "1363",
                "status": 1
            }
        ],
        "nextPageCursor": "1363"
    },
    "retExtInfo": {},
    "time": 1752570519414
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Repay

Fully or partially repay a loan. If interest is due, that is paid off first, with the loaned amount being paid off only after due interest.

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- The repaid amount will be deducted from the Funding wallet.
- The collateral amount will not be auto returned when you don't fully repay the debt, but you can also adjust collateral amount

### HTTP Request

POST`/v5/crypto-loan-flexible/repay`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanCurrency | **true** | string | Loan coin name |
| amount | **true** | string | Amount to repay |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| repayId | string | Repayment transaction ID |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-flexible/repay HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752569628364
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 52

{
    "loanCurrency": "BTC",
    "amount": "0.005"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.repay_flexible_crypto_loan(
    loanCurrency="BTC",
    loanAmount="0.005",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "repayId": "1771"
    },
    "retExtInfo": {},
    "time": 1752569614549
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Collateral Repayment

> Permission: "Spot trade"
>
> UID rate limit: 1 req / second

info

- Pay interest first, then repay the principal.
- There are limits on the repayment amount in a single transaction. Please read this [announcement](https://announcements.bybit.com/article/crypto-loan-manual-repayment-update-bltde33509ddde5e8fd/) before repaying with collateral
- When repaying with collateral, Bybit will charge a repayment fee. The applicable fee rate is the higher of the repayment fee rates for the collateral asset and the debt asset.
You can call this endpoint: [View fee rates by asset](https://www.bybit.com/x-api/spot/api/fixed-loan/v1/coin-config) to get "reapyFee" where "pledgeEnable" = 1 for coins' repayment fee rates

### HTTP Request

POST`/v5/crypto-loan-flexible/repay-collateral`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanCurrency | **true** | string | Loan coin name |
| collateralCoin | **true** | string | Collateral currencies: Use commas to separate multiple collateral currencies |
| amount | **true** | string | Repay amount |

### Response Parameters

None

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-flexible/repay-collateral HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752569628364
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 52

{
  "loanCurrency": "USDT",
  "amount": "500",
  "collateralCoin":"BTC"
}
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.collateral_repayment_flexible_crypto_loan(
    loanCurrency="USDT",
    amount="500",
    collateralCoin="BTC",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {},
    "retExtInfo": {},
    "time": 1756971550401
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Repayment History

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-flexible/repayment-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| repayId | false | string | Repayment tranaction ID |
| loanCurrency | false | string | Loan coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> loanCurrency | string | Loan coin |
| \> repayAmount | string | Repayment amount |
| \> repayId | string | Repayment transaction ID |
| \> repayStatus | integer | Repayment status, `1`: success; `2`: processing; `3`: fail |
| \> repayTime | long | Repay timestamp |
| \> repayType | integer | Repayment type, `1`: repay by user; `2`: repay by liquidation; `5`: repay by delisting; `6`: repay by delay liquidation; `7`: repay by currency |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-flexible/repayment-history?loanCurrency=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752570746227
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_repayment_history_flexible_crypto_loan(
    loanCurrency="BTC",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "loanCurrency": "BTC",
                "repayAmount": "0.007",
                "repayId": "1773",
                "repayStatus": 1,
                "repayTime": 1752570731274,
                "repayType": 1
            },
            {
                "loanCurrency": "BTC",
                "repayAmount": "0.006",
                "repayId": "1772",
                "repayStatus": 1,
                "repayTime": 1752570726038,
                "repayType": 1
            },
            {
                "loanCurrency": "BTC",
                "repayAmount": "0.005",
                "repayId": "1771",
                "repayStatus": 1,
                "repayTime": 1752569614528,
                "repayType": 1
            }
        ],
        "nextPageCursor": "1769"
    },
    "retExtInfo": {},
    "time": 1752570745493
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Flexible Loans

Query for your ongoing loans

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-flexible/ongoing-coin`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| loanCurrency | false | string | Loan coin name |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> hourlyInterestRate | string | Latest hourly flexible interest rate |
| \> loanCurrency | string | Loan coin |
| \> totalDebt | string | Unpaid principal and interest |
| \> unpaidAmount | string | Unpaid principal |
| \> unpaidInterest | string | Unpaid interest |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-flexible/ongoing-coin?loanCurrency=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752570124973
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_flexible_loans_flexible_crypto_loan(
    loanCurrency="BTC",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "hourlyInterestRate": "0.0000018847396",
                "loanCurrency": "ETH",
                "totalDebt": "0.10000019",
                "unpaidAmount": "0.1",
                "unpaidInterest": "0.00000019"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1760452029499
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Borrowable Coins

info

Does not need authentication.

### HTTP Request

GET`/v5/crypto-loan-common/loanable-data`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| vipLevel | false | string | VIP level <br>- `VIP0`, `VIP1`, `VIP2`, `VIP3`, `VIP4`, `VIP5`, `VIP99`(supreme VIP)<br>- `PRO1`, `PRO2`, `PRO3`, `PRO4`, `PRO5`, `PRO6` |
| currency | false | string | Coin name, uppercase only |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> currency | string | Coin name |
| \> fixedBorrowable | boolean | Whether support fixed loan |
| \> fixedBorrowingAccuracy | integer | Coin precision for fixed loan |
| \> flexibleBorrowable | boolean | Whether support flexible loan |
| \> flexibleBorrowingAccuracy | integer | Coin precision for flexible loan |
| \> maxBorrowingAmount | string | Max borrow limit |
| \> minFixedBorrowingAmount | string | Minimum amount for each fixed loan order |
| \> minFlexibleBorrowingAmount | string | Minimum amount for each flexible loan order |
| \> vipLevel | string | VIP level |
| \> flexibleAnnualizedInterestRate | integer | The annualized interest rate for flexible borrowing. If the loan currency does not support flexible borrowing, it will always be """" |
| \> annualizedInterestRate7D | string | The lowest annualized interest rate for fixed borrowing for 7 days that the market can currently provide. If there is no lending in the current market, then it is empty string |
| \> annualizedInterestRate14D | string | The lowest annualized interest rate for fixed borrowing for 14 days that the market can currently provide. If there is no lending in the current market, then it is empty string |
| \> annualizedInterestRate30D | string | The lowest annualized interest rate for fixed borrowing for 30 days that the market can currently provide. If there is no lending in the current market, then it is empty string |
| \> annualizedInterestRate60D | string | The lowest annualized interest rate for fixed borrowing for 60 days that the market can currently provide. If there is no lending in the current market, then it is empty string |
| \> annualizedInterestRate90D | string | The lowest annualized interest rate for fixed borrowing for 90 days that the market can currently provide. If there is no lending in the current market, then it is empty string |
| \> annualizedInterestRate180D | string | The lowest annualized interest rate for fixed borrowing for 180 days that the market can currently provide. If there is no lending in the current market, then it is empty string |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-common/loanable-data?currency=ETH&vipLevel=VIP5 HTTP/1.1
Host: api-testnet.bybit.com
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
)
print(session.get_borrowable_coins_new_crypto_loan())
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "currency": "ETH",
                "fixedBorrowable": true,
                "fixedBorrowingAccuracy": 6,
                "flexibleBorrowable": true,
                "flexibleBorrowingAccuracy": 4,
                "maxBorrowingAmount": "1100",
                "minFixedBorrowingAmount": "0.1",
                "minFlexibleBorrowingAmount": "0.001",
                "vipLevel": "VIP5",
                "annualizedInterestRate14D": "0.08",
                "annualizedInterestRate180D": "",
                "annualizedInterestRate30D": "",
                "annualizedInterestRate60D": "",
                "annualizedInterestRate7D": "",
                "annualizedInterestRate90D": "",
                "flexibleAnnualizedInterestRate": "0.001429799316"
            }
        ]
    },
    "retExtInfo": {},
    "time": 1752573126653
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Collateral Adjustment History

Query for your LTV adjustment history.

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-common/adjustment-history`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| adjustId | false | string | Collateral adjustment transaction ID |
| collateralCurrency | false | string | Collateral coin name |
| limit | false | string | Limit for data size per page. \[`1`, `100`\]. Default: `10` |
| cursor | false | string | Cursor. Use the `nextPageCursor` token from the response to retrieve the next page of the result set |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| list | array | Object |
| \> collateralCurrency | string | Collateral coin |
| \> amount | string | amount |
| \> adjustId | long | Collateral adjustment transaction ID |
| \> adjustTime | long | Adjust timestamp |
| \> preLTV | string | LTV before the adjustment |
| \> afterLTV | string | LTV after the adjustment |
| \> direction | integer | The direction of adjustment, `0`: add collateral; `1`: reduce collateral |
| \> status | integer | The status of adjustment, `1`: success; `2`: processing; `3`: fail |
| nextPageCursor | string | Refer to the `cursor` request parameter |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-common/adjustment-history?limit=2&collateralCurrency=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752628288472
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_ltv_adjustment_history_new_crypto_loan(
    limit="2",
    collateralCurrency="BTC",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "list": [
            {
                "adjustId": 27511,
                "adjustTime": 1752627997907,
                "afterLTV": "0.813743",
                "amount": "0.08",
                "collateralCurrency": "BTC",
                "direction": 1,
                "preLTV": "0.524602",
                "status": 1
            },
            {
                "adjustId": 27491,
                "adjustTime": 1752218558913,
                "afterLTV": "0.41983",
                "amount": "0.03",
                "collateralCurrency": "BTC",
                "direction": 1,
                "preLTV": "0.372314",
                "status": 1
            }
        ],
        "nextPageCursor": "27491"
    },
    "retExtInfo": {},
    "time": 1752628288732
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Obtain Max Loan Amount

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

POST`/v5/crypto-loan-common/max-loan`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Coin to borrow |
| collateralList | false | array<object> |  |
| \> amount | **true** | string | Collateral amount. Only check funding account balance |
| \> ccy | **true** | string | Collateral coin. Both `amount` & `ccy` are required, when you pass "collateralList" |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| currency | string | Coin to borrow |
| maxLoan | string | Based on your current collateral, and with the option to add more collateral, you can borrow up to `maxLoan` |
| notionalUsd | string | Nontional USD value |
| remainingQuota | string | The **remaining** individual platform borrowing limit (shared between main and sub accounts) |

### Request Example

- HTTP
- Python
- Node.js

```http
POST /v5/crypto-loan-common/max-loan HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1768532512103
X-BAPI-RECV-WINDOW: 5000
Content-Type: application/json
Content-Length: 208

{
    "currency": "BTC",
    "collateralList": [
        {
            "ccy": "XRP",
            "amount": "1000"
        },
        {
            "ccy": "USDT",
            "amount": "1000"
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
print(session.get_max_loan_amount_new_crypto_loan(
    currency="BTC",
    collateralList=[
        {
            "ccy": "XRP",
            "amount": "1000"
        },
        {
            "ccy": "USDT",
            "amount": "1000"
        }
    ]
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "currency": "BTC",
        "maxLoan": "0.1722",
        "notionalUsd": "16456.06",
        "remainingQuota": "9999999.9421"
    },
    "retExtInfo": {},
    "time": 1768533990031
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

# Get Max. Allowed Collateral Reduction Amount

Retrieve the maximum redeemable amount of your collateral asset based on LTV.

> Permission: "Spot trade"
>
> UID rate limit: 5 req / second

### HTTP Request

GET`/v5/crypto-loan-common/max-collateral-amount`Copy

### Request Parameters

| Parameter | Required | Type | Comments |
| :-- | :-- | :-- | --- |
| currency | **true** | string | Collateral coin |

### Response Parameters

| Parameter | Type | Comments |
| :-- | :-- | --- |
| maxCollateralAmount | string | Maximum reduction amount |

### Request Example

- HTTP
- Python
- Node.js

```http
GET /v5/crypto-loan-common/max-collateral-amount?currency=BTC HTTP/1.1
Host: api-testnet.bybit.com
X-BAPI-SIGN: XXXXXX
X-BAPI-API-KEY: XXXXXX
X-BAPI-TIMESTAMP: 1752627687351
X-BAPI-RECV-WINDOW: 5000
```

```python
from pybit.unified_trading import HTTP
session = HTTP(
    testnet=True,
    api_key="xxxxxxxxxxxxxxxxxx",
    api_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
)
print(session.get_max_allowed_collateral_reduction_amount_new_crypto_loan(
    collateralCurrency="BTC",
))
```

```n4js

```

### Response Example

```json
{
    "retCode": 0,
    "retMsg": "ok",
    "result": {
        "maxCollateralAmount": "0.08585184"
    },
    "retExtInfo": {},
    "time": 1752627687596
}
```

 

Community

GitHub

- [Official .Net SDK – bybit.net.api](https://github.com/bybit-exchange/bybit.net.api)

---

