# OKX Financial Product API (Earn/Lending)

Source: https://www.okx.com/docs-v5/en/

---

## Repo Usage Quick Reference

- Primary repo use: usually not on the hot trading path; useful for operational reference around OKX earn/lending surfaces
- Repo symbol context:
  - repo trading symbols look like `BTCUSDT`
  - financial-product APIs work mostly with currencies and product IDs such as `BTC`, `USDT`, `productId`
- Most relevant sections if this repo ever touches them:
  - available offers
  - purchase / redeem flows
  - position or holdings queries
- Important repo note: this file is mainly reference material; spot-futures and funding-arb logic should not assume these earn/lending APIs are part of the normal execution path unless the adapter code explicitly adds them

# Financial Product

## On-chain earn

Only the assets in the funding account can be used for purchase. [More details](https://www.okx.com/earn/onchain-earn)

### GET / Offers

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/offers`

> Request Example

```
GET /api/v5/finance/staking-defi/offers
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.get_offers(ccy="USDT")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| productId | String | No | Product ID |
| protocolType | String | No | Protocol type<br>`defi`: on-chain earn |
| ccy | String | No | Investment currency, e.g. `BTC` |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "ccy": "DOT",
            "productId": "101",
            "protocol": "Polkadot",
            "protocolType": "defi",
            "term": "0",
            "apy": "0.1767",
            "earlyRedeem": false,
            "state": "purchasable",
            "investData": [
                {
                    "bal": "0",
                    "ccy": "DOT",
                    "maxAmt": "0",
                    "minAmt": "2"
                }
            ],
            "earningData": [
                {
                    "ccy": "DOT",
                    "earningType": "0"
                }
            ],
            "fastRedemptionDailyLimit": "",
            "redeemPeriod": [
                "28D",
                "28D"
            ]
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency type, e.g. `BTC` |
| productId | String | Product ID |
| protocol | String | Protocol |
| protocolType | String | Protocol type<br>`defi`: on-chain earn |
| term | String | Protocol term<br>It will return the days of fixed term and will return `0` for flexible product |
| apy | String | Estimated annualization<br>If the annualization is 7% , this field is 0.07 |
| earlyRedeem | Boolean | Whether the protocol supports early redemption |
| investData | Array of objects | Current target currency information available for investment |
| \> ccy | String | Investment currency, e.g. `BTC` |
| \> bal | String | Available balance to invest |
| \> minAmt | String | Minimum subscription amount |
| \> maxAmt | String | Maximum available subscription amount |
| earningData | Array of objects | Earning data |
| \> ccy | String | Earning currency, e.g. `BTC` |
| \> earningType | String | Earning type<br>`0`: Estimated earning<br>`1`: Cumulative earning |
| state | String | Product state<br>`purchasable`: Purchasable<br>`sold_out`: Sold out<br>`Stop`: Suspension of subscription |
| redeemPeriod | Array of strings | Redemption Period, format in \[min time,max time\]<br>`H`: Hour, `D`: Day<br>e.g. \["1H","24H"\] represents redemption period is between 1 Hour and 24 Hours.<br>\["14D","14D"\] represents redemption period is 14 days. |
| fastRedemptionDailyLimit | String | Fast redemption daily limit<br>If fast redemption is not supported, it will return ''. |

### POST / Purchase

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/purchase`

> Request Example

```
# Invest 100ZIL 30-day staking protocol
POST /api/v5/finance/staking-defi/purchase
body
{
    "productId":"1234",
    "investData":[
      {
        "ccy":"ZIL",
        "amt":"100"
      }
    ],
    "term":"30"
}
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.purchase(
            productId = "4005",
            investData = [{
                "ccy":"USDT",
                "amt":"100"
            }]
        )
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| productId | String | Yes | Product ID |
| investData | Array of objects | Yes | Investment data |
| \> ccy | String | Yes | Investment currency, e.g. `BTC` |
| \> amt | String | Yes | Investment amount |
| term | String | Conditional | Investment term<br>Investment term must be specified for fixed-term product |
| tag | String | No | Order tag<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters. |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "ordId": "754147",
      "tag": ""
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| tag | String | Order tag |

### POST / Redeem

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/redeem`

> Request Example

```
# Early redemption of investment
POST /api/v5/finance/staking-defi/redeem
body
{
    "ordId":"754147",
    "protocolType":"defi",
    "allowEarlyRedeem":true
}
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.redeem(
           ordId = "1234",
           protocolType = "defi"
        )
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |
| protocolType | String | Yes | Protocol type<br>`defi`: on-chain earn |
| allowEarlyRedeem | Boolean | No | Whether allows early redemption<br>Default is `false` |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "ordId": "754147",
      "tag": ""
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| tag | String | Order tag |

### POST / Cancel purchases/redemptions

After cancelling, returning funds will go to the funding account.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/cancel`

> Request Example

```
POST /api/v5/finance/staking-defi/cancel
body
{
    "ordId":"754147",
    "protocolType":"defi"
}
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.cancel(
           ordId = "1234",
           protocolType = "defi"
        )
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |
| protocolType | String | Yes | Protocol type<br>`defi`: on-chain earn |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "ordId": "754147",
      "tag": ""
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| tag | String | Order tag |

### GET / Active orders

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/orders-active`

> Request Example

```
GET /api/v5/finance/staking-defi/orders-active
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.get_activity_orders()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| productId | String | No | Product ID |
| protocolType | String | No | Protocol type<br>`defi`: on-chain earn |
| ccy | String | No | Investment currency, e.g. `BTC` |
| state | String | No | Order state<br>`8`: Pending <br>`13`: Cancelling <br>`9`: Onchain <br>`1`: Earning <br>`2`: Redeeming |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "ordId": "2413499",
            "ccy": "DOT",
            "productId": "101",
            "state": "1",
            "protocol": "Polkadot",
            "protocolType": "defi",
            "term": "0",
            "apy": "0.1014",
            "investData": [
                {
                    "ccy": "DOT",
                    "amt": "2"
                }
            ],
            "earningData": [
                {
                    "ccy": "DOT",
                    "earningType": "0",
                    "earnings": "0.10615025"
                }
            ],
            "purchasedTime": "1729839328000",
            "tag": "",
            "estSettlementTime": "",
            "cancelRedemptionDeadline": "",
            "fastRedemptionData": []
        },
        {
            "ordId": "2213257",
            "ccy": "USDT",
            "productId": "4005",
            "state": "1",
            "protocol": "On-Chain Defi",
            "protocolType": "defi",
            "term": "0",
            "apy": "0.0323",
            "investData": [
                {
                    "ccy": "USDT",
                    "amt": "1"
                }
            ],
            "earningData": [
                {
                    "ccy": "USDT",
                    "earningType": "0",
                    "earnings": "0.02886582"
                },
                {
                    "ccy": "COMP",
                    "earningType": "1",
                    "earnings": "0.0000627"
                }
            ],
            "purchasedTime": "1725345790000",
            "tag": "",
            "estSettlementTime": "",
            "cancelRedemptionDeadline": "",
            "fastRedemptionData": []
        },
        {
            "ordId": "2210943",
            "ccy": "USDT",
            "productId": "4005",
            "state": "1",
            "protocol": "On-Chain Defi",
            "protocolType": "defi",
            "term": "0",
            "apy": "0.0323",
            "investData": [
                {
                    "ccy": "USDT",
                    "amt": "1"
                }
            ],
            "earningData": [
                {
                    "ccy": "USDT",
                    "earningType": "0",
                    "earnings": "0.02891823"
                },
                {
                    "ccy": "COMP",
                    "earningType": "1",
                    "earnings": "0.0000632"
                }
            ],
            "purchasedTime": "1725280801000",
            "tag": "",
            "estSettlementTime": "",
            "cancelRedemptionDeadline": "",
            "fastRedemptionData": []
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| ordId | String | Order ID |
| productId | String | Product ID |
| state | String | Order state<br>`8`: Pending <br>`13`: Cancelling <br>`9`: Onchain <br>`1`: Earning <br>`2`: Redeeming |
| protocol | String | Protocol |
| protocolType | String | Protocol type<br>`defi`: on-chain earn |
| term | String | Protocol term<br>It will return the days of fixed term and will return `0` for flexible product |
| apy | String | Estimated APY<br>If the estimated APY is 7% , this field is 0.07<br>Retain to 4 decimal places (truncated) |
| investData | Array of objects | Investment data |
| \> ccy | String | Investment currency, e.g. `BTC` |
| \> amt | String | Invested amount |
| earningData | Array of objects | Earning data |
| \> ccy | String | Earning currency, e.g. `BTC` |
| \> earningType | String | Earning type<br>`0`: Estimated earning<br>`1`: Cumulative earning |
| \> earnings | String | Earning amount |
| fastRedemptionData | Array of objects | Fast redemption data |
| \> ccy | String | Currency, e.g. `BTC` |
| \> redeemingAmt | String | Redeeming amount |
| purchasedTime | String | Order purchased time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| estSettlementTime | String | Estimated redemption settlement time |
| cancelRedemptionDeadline | String | Deadline for cancellation of redemption application |
| tag | String | Order tag |

### GET / Order history

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/orders-history`

> Request Example

```
GET /api/v5/finance/staking-defi/orders-history
```

```
import okx.Finance.StakingDefi as StakingDefi

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StakingAPI = StakingDefi.StakingDefiAPI(apikey, secretkey, passphrase, False, flag)

result = StakingAPI.get_orders_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| productId | String | No | Product ID |
| protocolType | String | No | Protocol type<br>`defi`: on-chain earn |
| ccy | String | No | Investment currency, e.g. `BTC` |
| after | String | No | Pagination of data to return records earlier than the requested ID. The value passed is the corresponding `ordId` |
| before | String | No | Pagination of data to return records newer than the requested ID. The value passed is the corresponding `ordId` |
| limit | String | No | Number of results per request. The default is `100`. The maximum is `100`. |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
       {
            "ordId": "1579252",
            "ccy": "DOT",
            "productId": "101",
            "state": "3",
            "protocol": "Polkadot",
            "protocolType": "defi",
            "term": "0",
            "apy": "0.1704",
            "investData": [
                {
                    "ccy": "DOT",
                    "amt": "2"
                }
            ],
            "earningData": [
                {
                    "ccy": "DOT",
                    "earningType": "0",
                    "realizedEarnings": "0"
                }
            ],
            "purchasedTime": "1712908001000",
            "redeemedTime": "1712914294000",
            "tag": ""
       }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| ordId | String | Order ID |
| productId | String | Product ID |
| state | String | Order state<br>`3`: Completed (including canceled and redeemed) |
| protocol | String | Protocol |
| protocolType | String | Protocol type<br>`defi`: on-chain earn |
| term | String | Protocol term<br>It will return the days of fixed term and will return `0` for flexible product |
| apy | String | Estimated APY<br>If the estimated APY is 7% , this field is `0.07`<br>Retain to 4 decimal places (truncated) |
| investData | Array of objects | Investment data |
| \> ccy | String | Investment currency, e.g. `BTC` |
| \> amt | String | Invested amount |
| earningData | Array of objects | Earning data |
| \> ccy | String | Earning currency, e.g. `BTC` |
| \> earningType | String | Earning type<br>`0`: Estimated earning<br>`1`: Cumulative earning |
| \> realizedEarnings | String | Cumulative earning of redeemed orders<br>This field is just valid when the order is in redemption state |
| purchasedTime | String | Order purchased time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| redeemedTime | String | Order redeemed time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| tag | String | Order tag |

## ETH staking

ETH Staking, also known as Ethereum Staking, is the process of participating in the Ethereum blockchain's Proof-of-Stake (PoS) consensus mechanism.

Stake to receive BETH for liquidity at 1:1 ratio and earn daily BETH rewards

[Learn more about ETH Staking](https://www.okx.com/earn/ethereum-staking)

### GET / Product info

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/eth/product-info`

> Request Example

```
GET /api/v5/finance/staking-defi/eth/product-info
```

```
import okx.Finance.EthStaking as EthStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.eth_product_info()
print(result)
```

> Response Example

```
{
    "code": "0",
    "data": [
      {
        "fastRedemptionDailyLimit": "100"
      }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fastRedemptionDailyLimit | String | Fast redemption daily limit<br>The master account and sub-accounts share the same limit |

### POST / Purchase

Staking ETH for BETH

Only the assets in the funding account can be used.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/eth/purchase`

> Request Example

```
POST /api/v5/finance/staking-defi/eth/purchase
body
{
    "amt":"100"
}
```

```
import okx.Finance.EthStaking as EthStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.eth_purchase(amt="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| amt | String | Yes | Investment amount |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
  ]
}
```

#### Response Parameters

code = `0` means your request has been successfully handled.

### POST / Redeem

Only the assets in the funding account can be used. If your BETH is in your trading account, you can make funding transfer first.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/eth/redeem`

> Request Example

```
POST /api/v5/finance/staking-defi/eth/redeem
body
{
    "amt": "10"
}
```

```
import okx.Finance.EthStaking as EthStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.eth_redeem(amt="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| amt | String | Yes | Redeeming amount |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
  ]
}
```

#### Response Parameters

code = `0` means your request has been successfully handled.

### POST / Cancel redeem

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/eth/cancel-redeem`

> Request Example

```
POST /api/v5/finance/staking-defi/eth/cancel-redeem
body
{
    "ordId": "1234567890"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "ordId": "1234567890"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |

### GET / Balance

The balance represents the real-time total BETH holdings across the entire account, including assets in the trading account, funding account, and those currently in the redeeming process.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/eth/balance`

> Request Example

```
GET /api/v5/finance/staking-defi/eth/balance
```

```
import okx.Finance.EthStaking as EthStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.eth_balance()
print(result)
```

#### Request Parameters

None

> Response Example

```
{
    "code": "0",
    "data": [
      {
        "amt": "0.63926191",
        "ccy": "BETH",
        "latestInterestAccrual": "0.00006549",
        "totalInterestAccrual": "0.01490596",
        "ts": "1699257600000"
      }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BETH` |
| amt | String | Currency amount |
| latestInterestAccrual | String | Latest interest accrual |
| totalInterestAccrual | String | Total interest accrual |
| ts | String | Query data time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / Purchase&Redeem history

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/eth/purchase-redeem-history`

> Request Example

```
GET /api/v5/finance/staking-defi/eth/purchase-redeem-history
```

```
import okx.Finance.EthStaking as EthStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.eth_purchase_redeem_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| type | String | No | Type<br>`purchase`<br>`redeem` |
| status | String | No | Status<br>`pending`<br>`success`<br>`failed`<br>`cancelled` |
| after | String | No | Pagination of data to return records earlier than the `requestTime`. The value passed is the corresponding `timestamp` |
| before | String | No | Pagination of data to return records newer than the `requestTime`. The value passed is the corresponding `timestamp` |
| limit | String | No | Number of results per request. The default is `100`. The maximum is `100`. |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "amt": "0.62666630",
            "completedTime": "1683413171000",
            "estCompletedTime": "",
            "redeemingAmt": "",
            "requestTime": "1683413171000",
            "status": "success",
            "type": "purchase"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| type | String | Type<br>`purchase`<br>`redeem` |
| amt | String | Purchase/Redeem amount |
| redeemingAmt | String | Redeeming amount |
| status | String | Status<br>`pending`<br>`success`<br>`failed`<br>`cancelled` |
| ordId | String | Order ID |
| requestTime | String | Request time of make purchase/redeem, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| completedTime | String | Completed time of redeem settlement, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| estCompletedTime | String | Estimated completed time of redeem settlement, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / APY history (Public)

Public endpoints don't need authorization.

#### Rate Limit: 6 requests per second

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/finance/staking-defi/eth/apy-history`

> Request Example

```
GET /api/v5/finance/staking-defi/eth/apy-history?days=2
```

```
import okx.Finance.EthStaking as EthStaking

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = EthStaking.EthStakingAPI(flag=flag)

result = StackingAPI.eth_apy_history(days="7")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| days | String | Yes | Get the days of APY(Annual percentage yield) history record in the past<br>No more than 365 days |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "rate": "0.02690000",
            "ts": "1734195600000"
        },
        {
            "rate": "0.02840000",
            "ts": "1734109200000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| rate | String | APY(Annual percentage yield), e.g. `0.01` represents `1%` |
| ts | String | Data time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

## SOL staking

By staking SOL tokens and delegating them to validators on the Solana network, you can receive equivalent OKSOL and earn extra OKSOL rewards.

Stake SOL on Solana to receive OKSOL at a 1:1 ratio for liquidity

[Learn more about OKSOL Staking](https://www.okx.com/earn/solana-staking#from=finance_crypto)

### GET / Product info

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/sol/product-info`

> Request Example

```
GET /api/v5/finance/staking-defi/sol/product-info
```

```
```

> Response Example

```
{
    "code": "0",
    "data": {
        "fastRedemptionAvail": "240",
        "fastRedemptionDailyLimit": "240"
    },
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fastRedemptionDailyLimit | String | Fast redemption daily limit<br>The master account and sub-accounts share the same limit |
| fastRedemptionAvail | String | Currently fast redemption max available amount |

### POST / Purchase

Staking SOL for OKSOL

Only the assets in the funding account can be used.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/sol/purchase`

> Request Example

```
POST /api/v5/finance/staking-defi/sol/purchase
body
{
    "amt":"100"
}
```

```
import okx.Finance.SolStaking as SolStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = SolStaking.SolStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.sol_purchase(amt="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| amt | String | Yes | Investment amount |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
  ]
}
```

#### Response Parameters

code = `0` means your request has been successfully handled.

### POST / Redeem

Only the assets in the funding account can be used. If your OKSOL is in your trading account, you can make funding transfer first.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/staking-defi/sol/redeem`

> Request Example

```
POST /api/v5/finance/staking-defi/sol/redeem
body
{
    "amt": "10"
}
```

```
import okx.Finance.SolStaking as SolStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = SolStaking.SolStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.sol_redeem(amt="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| amt | String | Yes | Redeeming amount |

> Response Example

```
{
  "code": "0",
  "msg": "",
  "data": [
  ]
}
```

#### Response Parameters

code = `0` means your request has been successfully handled.

### GET / Balance

The balance represents the real-time total OKSOL holdings across the entire account, including assets in the trading account, funding account, and those currently in the redeeming process.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/sol/balance`

> Request Example

```
GET /api/v5/finance/staking-defi/sol/balance
```

```
import okx.Finance.SolStaking as SolStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = SolStaking.SolStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.sol_balance()
print(result)
```

#### Request Parameters

None

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "amt": "0.01100012",
            "ccy": "OKSOL",
            "latestInterestAccrual": "0.00000012",
            "totalInterestAccrual": "0.00000012"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `OKSOL` |
| amt | String | Currency amount |
| latestInterestAccrual | String | Latest interest accrual |
| totalInterestAccrual | String | Total interest accrual |

### GET / Purchase&Redeem history

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/staking-defi/sol/purchase-redeem-history`

> Request Example

```
GET /api/v5/finance/staking-defi/sol/purchase-redeem-history
```

```
import okx.Finance.SolStaking as SolStaking

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = SolStaking.SolStakingAPI(apikey, secretkey, passphrase, False, flag)

result = StackingAPI.sol_purchase_redeem_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| type | String | No | Type<br>`purchase`<br>`redeem` |
| status | String | No | Status<br>`pending`<br>`success`<br>`failed` |
| after | String | No | Pagination of data to return records earlier than the `requestTime`. The value passed is the corresponding `timestamp` |
| before | String | No | Pagination of data to return records newer than the `requestTime`. The value passed is the corresponding `timestamp` |
| limit | String | No | Number of results per request. The default is `100`. The maximum is `100`. |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "amt": "0.62666630",
            "completedTime": "1683413171000",
            "estCompletedTime": "",
            "redeemingAmt": "",
            "requestTime": "1683413171000",
            "status": "success",
            "type": "purchase"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| type | String | Type<br>`purchase`<br>`redeem` |
| amt | String | Purchase/Redeem amount |
| redeemingAmt | String | Redeeming amount |
| status | String | Status<br>`pending`<br>`success`<br>`failed` |
| requestTime | String | Request time of make purchase/redeem, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| completedTime | String | Completed time of redeem settlement, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| estCompletedTime | String | Estimated completed time of redeem settlement, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / APY history (Public)

Public endpoints don't need authorization.

#### Rate Limit: 6 requests per second

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/finance/staking-defi/sol/apy-history`

> Request Example

```
GET /api/v5/finance/staking-defi/sol/apy-history?days=2
```

```
import okx.Finance.SolStaking as SolStaking

flag = "0"  # Production trading:0 , demo trading:1

StackingAPI = SolStaking.SolStakingAPI(flag=flag)

result = StackingAPI.sol_apy_history(days="7")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| days | String | Yes | Get the days of APY(Annual percentage yield) history record in the past<br>No more than 365 days |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "rate": "0.11280000",
            "ts": "1734192000000"
        },
        {
            "rate": "0.11270000",
            "ts": "1734105600000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| rate | String | APY(Annual percentage yield), e.g. `0.01` represents `1%` |
| ts | String | Data time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

## Simple earn flexible

Simple earn flexible (saving) is earned by lending to leveraged trading users in the lending market. [learn more](https://www.okx.com/earn/simple-earn)

### GET / Saving balance

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/savings/balance`

> Request Example

```
GET /api/v5/finance/savings/balance?ccy=USDT
```

```
import okx.Finance.Savings as Savings

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(apikey, secretkey, passphrase, False, flag)

result = SavingsAPI.get_saving_balance(ccy="USDT")
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |

> Response Example

```
{
    "code": "0",
    "msg":"",
    "data": [
        {
            "earnings": "0.0010737388791526",
            "redemptAmt": "",
            "rate": "0.0100000000000000",
            "ccy": "USDT",
            "amt": "11.0010737453457821",
            "loanAmt": "11.0010630707982819",
            "pendingAmt": "0.0000106745475002"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| amt | String | Currency amount |
| earnings | String | Currency earnings |
| rate | String | Minimum annual lending rate configured by users |
| loanAmt | String | Lending amount |
| pendingAmt | String | Pending amount |
| redemptAmt | String | ~~Redempting amount~~ (Deprecated) |

### POST / Savings purchase/redemption

Only the assets in the funding account can be used for saving.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/savings/purchase-redempt`

> Request Example

```
POST /api/v5/finance/savings/purchase-redempt
body
{
    "ccy":"BTC",
    "amt":"1",
    "side":"purchase",
    "rate":"0.01"
}
```

```
import okx.Finance.Savings as Savings

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(apikey, secretkey, passphrase, False, flag)

result = SavingsAPI.savings_purchase_redemption(ccy='USDT',amt="0.1",side="purchase",rate="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | Yes | Currency, e.g. `BTC` |
| amt | String | Yes | Purchase/redemption amount |
| side | String | Yes | Action type. <br>`purchase`: purchase saving shares <br>`redempt`: redeem saving shares |
| rate | String | Conditional | Annual purchase rate, e.g. `0.1` represents `10%`<br>Only applicable to purchase saving shares<br>The interest rate of the new subscription will cover the interest rate of the last subscription<br>The rate value range is between 1% and 365% |

> Response Example

```
{
    "code":"0",
    "msg":"",
    "data":[
        {
            "ccy":"BTC",
            "amt":"1",
            "side":"purchase",
            "rate": "0.01"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| amt | String | Purchase/Redemption amount |
| side | String | Action type |
| rate | String | Annual purchase rate, e.g. `0.1` represents `10%` |

### POST / Set lending rate

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/savings/set-lending-rate`

> Request Example

```
POST /api/v5/finance/savings/set-lending-rate
body
{
    "ccy":"BTC",
    "rate":"0.02"
}
```

```
import okx.Finance.Savings as Savings

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(apikey, secretkey, passphrase, False, flag)

result = SavingsAPI.set_lending_rate(ccy='USDT',rate="1")
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | Yes | Currency, e.g. `BTC` |
| rate | String | Yes | Annual lending rate<br>The rate value range is between 1% and 365% |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [{
        "ccy": "BTC",
        "rate": "0.02"
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| rate | String | Annual lending rate |

### GET / Lending history

Return data in the past month.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/savings/lending-history`

> Request Example

```
GET /api/v5/finance/savings/lending-history
```

```
import okx.Finance.Savings as Savings

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(apikey, secretkey, passphrase, False, flag)

result = SavingsAPI.get_lending_history()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |
| after | String | No | Pagination of data to return records earlier than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [{
            "ccy": "BTC",
            "amt": "0.01",
            "earnings": "0.001",
            "rate": "0.01",
            "ts": "1597026383085"
        },
        {
            "ccy": "ETH",
            "amt": "0.2",
            "earnings": "0.001",
            "rate": "0.01",
            "ts": "1597026383085"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| amt | String | Lending amount |
| earnings | String | Currency earnings |
| rate | String | Lending annual interest rate |
| ts | String | Lending time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / Public borrow info (public)

Authentication is not required for this public endpoint.

#### Rate Limit: 6 requests per second

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/finance/savings/lending-rate-summary`

> Request Example

```
GET /api/v5/finance/savings/lending-rate-summary
```

```
import okx.Finance.Savings as Savings

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(flag=flag)

result = SavingsAPI.get_public_borrow_info()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [{
        "ccy": "BTC",
        "avgAmt": "10000",
        "avgAmtUsd": "10000000000",
        "avgRate": "0.03",
        "preRate": "0.02",
        "estRate": "0.01"
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| avgAmt | String | ~~24H average borrowing amount~~(deprecated) |
| avgAmtUsd | String | ~~24H average borrowing amount in `USD` value~~(deprecated) |
| avgRate | String | 24-hours average annual borrowing rate |
| preRate | String | Last annual borrowing interest rate |
| estRate | String | Next estimate annual borrowing interest rate |

### GET / Public borrow history (public)

Authentication is not required for this public endpoint.

Only returned records after December 14, 2021.

#### Rate Limit: 6 requests per second

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/finance/savings/lending-rate-history`

> Request Example

```
GET /api/v5/finance/savings/lending-rate-history
```

```
import okx.Finance.Savings as Savings

flag = "0"  # Production trading:0 , demo trading:1

SavingsAPI = Savings.SavingsAPI(flag=flag)

result = SavingsAPI.get_public_borrow_history()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |
| after | String | No | Pagination of data to return records earlier than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`.<br>If `ccy` is not specified, all data under the same `ts` will be returned, not limited by `limit` |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [{
        "ccy": "BTC",
        "amt": "0.01",
        "rate": "0.001",
        "lendingRate": "0.001",
        "ts": "1597026383085"
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| amt | String | ~~Lending amount~~(deprecated) |
| rate | String | Annual borrowing interest rate |
| lendingRate | String | Annual lending interest rate |
| ts | String | Time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

## Flexible loan

OKX Flexible Loan is a high-end loan product that allows users to increase cash flow without selling off their crypto. [More details](https://www.okx.com/loan)

### GET / Borrowable currencies

Get borrowable currencies

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/borrow-currencies`

> Request Example

```
GET /api/v5/finance/flexible-loan/borrow-currencies
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.borrow_currencies()
print(result)
```

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "borrowCcy": "USDT"
        },
        {
            "borrowCcy": "USDC"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| borrowCcy | String | Borrowable currency, e.g. `BTC` |

### GET / Collateral assets

Get collateral assets in funding account.

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/collateral-assets`

> Request Example

```
GET /api/v5/finance/flexible-loan/collateral-assets
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.collateral_assets()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Collateral currency, e.g. `BTC` |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "assets": [
                {
                    "amt": "1.7921483143067599",
                    "ccy": "BTC",
                    "notionalUsd": "158292.621793314105231"
                },
                {
                    "amt": "1.9400755578876945",
                    "ccy": "ETH",
                    "notionalUsd": "6325.6652712507628946"
                },
                {
                    "amt": "63.9795959720319628",
                    "ccy": "USDT",
                    "notionalUsd": "64.3650372635940345"
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
| assets | Array of objects | Collateral assets data |
| \> ccy | String | Currency, e.g. `BTC` |
| \> amt | String | Available amount |
| \> notionalUsd | String | Notional value in `USD` |

### POST / Maximum loan amount

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`POST /api/v5/finance/flexible-loan/max-loan`

> Request Example

```
POST /api/v5/finance/flexible-loan/max-loan
body
{
    "borrowCcy": "USDT"
}
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.max_loan(borrowCcy="USDT")
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| borrowCcy | String | Yes | Currency to borrow, e.g. `USDT` |
| supCollateral | Array of objects | No | Supplementary collateral assets |
| \> ccy | String | Yes | Currency, e.g. `BTC` |
| \> amt | String | Yes | Amount |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "borrowCcy": "USDT",
            "maxLoan": "0.01113",
            "notionalUsd": "0.01113356",
            "remainingQuota": "3395000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| borrowCcy | String | Currency to borrow, e.g. `USDT` |
| maxLoan | String | Maximum available loan |
| notionalUsd | String | Maximum available loan notional value, unit in `USD` |
| remainingQuota | String | Remaining quota, unit in `borrowCcy` |

### GET / Maximum collateral redeem amount

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/max-collateral-redeem-amount`

> Request Example

```
GET /api/v5/finance/flexible-loan/max-collateral-redeem-amount?ccy=USDT
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.max_collateral_redeem_amount("USDT")
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | Yes | Collateral currency, e.g. `USDT` |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "ccy": "USDT",
            "maxRedeemAmt": "1"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ccy | String | Collateral currency, e.g. `USDT` |
| maxRedeemAmt | String | Maximum collateral redeem amount |

### POST / Adjust collateral

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/flexible-loan/adjust-collateral`

> Request Example

```
POST /api/v5/finance/flexible-loan/adjust-collateral
body
{
    "type":"add",
    "collateralCcy": "BTC",
    "collateralAmt": "0.1"
}
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.adjust_collateral(type="add", collateralCcy="USDT", collateralAmt="1")
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| type | String | Yes | Operation type<br>`add`: Add collateral<br>`reduce`: Reduce collateral |
| collateralCcy | String | Yes | Collateral currency, e.g. `BTC` |
| collateralAmt | String | Yes | Collateral amount |

> Response Example

```
{
    "code": "0",
    "data": [
    ],
    "msg": ""
}
```

#### Response Parameters

code = `0` means your request has been accepted (It doesn't mean the request has been successfully handled.)

### GET / Loan info

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/loan-info`

> Request Example

```
GET /api/v5/finance/flexible-loan/loan-info
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.loan_info()
print(result)
```

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "collateralData": [
                {
                    "amt": "0.0000097",
                    "ccy": "COMP"
                },
                {
                    "amt": "0.78",
                    "ccy": "STX"
                },
                {
                    "amt": "0.001",
                    "ccy": "DOT"
                },
                {
                    "amt": "0.05357864",
                    "ccy": "LUNA"
                }
            ],
            "collateralNotionalUsd": "1.5078763",
            "curLTV": "0.5742",
            "liqLTV": "0.8374",
            "loanData": [
                {
                    "amt": "0.86590608",
                    "ccy": "USDC"
                }
            ],
            "loanNotionalUsd": "0.8661285",
            "marginCallLTV": "0.7374",
            "riskWarningData": {
                "instId": "",
                "liqPx": ""
            }
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| loanNotionalUsd | String | Loan value in `USD` |
| loanData | Array of objects | Loan data |
| \> ccy | String | Loan currency, e.g. `USDT` |
| \> amt | String | Loan amount |
| collateralNotionalUsd | String | Collateral value in `USD` |
| collateralData | Array of objects | Collateral data |
| \> ccy | String | Collateral currency, e.g. `BTC` |
| \> amt | String | Collateral amount |
| riskWarningData | Object | Risk warning data |
| \> instId | String | Liquidation instrument ID, e.g. `BTC-USDT`<br>This field is only valid when there is only one type of collateral and one type of borrowed currency. In other cases, it returns "". |
| \> liqPx | String | Liquidation price<br>The unit of the liquidation price is the quote currency of the instrument, e.g. `USDT` in `BTC-USDT`.<br>This field is only valid when there is only one type of collateral and one type of borrowed currency. In other cases, it returns "". |
| curLTV | String | Current LTV, e.g. `0.1` represents `10%`<br>Note: LTV = Loan to Value |
| marginCallLTV | String | Margin call LTV, e.g. `0.1` represents `10%`<br>If your loan hits the margin call LTV, our system will automatically warn you that your loan is getting close to forced liquidation. |
| liqLTV | String | Liquidation LTV, e.g. `0.1` represents `10%`<br>If your loan reaches liquidation LTV, it'll trigger forced liquidation. When this happens, you'll lose access to your collateral and any repayments made. |

### GET / Loan history

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/loan-history`

> Request Example

```
GET /api/v5/finance/flexible-loan/loan-history
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.loan_history()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| type | String | No | Action type<br>`borrowed`<br>`repaid`<br>`collateral_locked`<br>`collateral_released`<br>`forced_repayment_buy`<br>`forced_repayment_sell`<br>`forced_liquidation`<br>`partial_liquidation`<br>`sell_collateral`<br>`buy_transition_coin`<br>`sell_transition_coin`<br>`buy_borrowed_coin` |
| after | String | No | Pagination of data to return records earlier than the requested `refId`(not include) |
| before | String | No | Pagination of data to return records newer than the requested `refId`(not include) |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |

> Response Example

```
{
    "code": "0",
    "data": [
        {
            "amt": "-0.001",
            "ccy": "DOT",
            "refId": "17316594851045086",
            "ts": "1731659485000",
            "type": "collateral_locked"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| refId | String | Reference ID |
| type | String | Action type |
| ccy | String | Currency, e.g. `BTC` |
| amt | String | Amount |
| ts | String | Timestamp for the action, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### GET / Accrued interest

Retrieves the interest accrual history for flexible loans over the past 30 days.

#### Rate Limit: 5 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/flexible-loan/interest-accrued`

> Request Example

```
GET /api/v5/finance/flexible-loan/interest-accrued
```

```
from okx.Finance import FlexibleLoan

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading:0 , demo trading:1

flexibleLoanAPI = FlexibleLoan.FlexibleLoanAPI(apikey, secretkey, passphrase, False, flag)
result = flexibleLoanAPI.interest_accrued()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Loan currency, e.g. `BTC` |
| after | String | No | Pagination of data to return records earlier than the requested `refId`(not include) |
| before | String | No | Pagination of data to return records newer than the requested `refId`(not include) |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |

> 返回结果

```
{
    "code": "0",
    "data": [
        {
            "ccy": "USDC",
            "interest": "0.00004054",
            "interestRate": "0.41",
            "loan": "0.86599309",
            "refId": "17319133035195744",
            "ts": "1731913200000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| refId | String | Reference ID |
| ccy | String | Loan currency, e.g. `BTC` |
| loan | String | Loan when calculated interest |
| interest | String | Interest |
| interestRate | String | APY, e.g. `0.01` represents `1%` |
| ts | String | Timestamp to calculated interest, Unix timestamp format in milliseconds, e.g. `1597026383085` |

## Dual investment

### GET / Currency pairs

Returns available dual investment currency pairs.

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/sfp/dcd/currency-pair`

> Request Example

```
GET /api/v5/finance/sfp/dcd/currency-pair
```

#### Request Parameters

None

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "baseCcy": "BTC",
            "quoteCcy": "USDT",
            "optType": "C",
            "uly": "BTC-USD"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| baseCcy | String | Base currency |
| quoteCcy | String | Quote currency |
| optType | String | Option type<br>`C`: Call<br>`P`: Put |
| uly | String | Underlying |

### GET / Product info

Return dual investment product list.

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/sfp/dcd/products`

> Request Example

```
GET /api/v5/finance/sfp/dcd/products?baseCcy=BTC&quoteCcy=USDT&optType=C
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| baseCcy | String | Yes | Base currency |
| quoteCcy | String | Yes | Quote currency |
| optType | String | Yes | Option type<br>`C`: Call<br>`P`: Put |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "absYield": "0.00232413",
            "annualizedYield": "0.0541",
            "baseCcy": "BTC",
            "quoteCcy": "USDT",
            "expTime": "1774598400000",
            "interestAccrualTime": "1773244800000",
            "listTime": "1743150759000",
            "maxSize": "6000000",
            "minSize": "10",
            "notionalCcy": "USDT",
            "optType": "P",
            "productId": "BTC-USDT-260327-54500-P",
            "quoteTime": "1773243808703",
            "redeemEndTime": "1774594800000",
            "redeemStartTime": "1773244800000",
            "stepSz": "1",
            "tradeEndTime": "1774584000000",
            "strike": "54500",
            "uly": "BTC-USD"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| absYield | String | Absolute yield |
| annualizedYield | String | Annualized yield |
| baseCcy | String | Base currency |
| quoteCcy | String | Quote currency |
| notionalCcy | String | Investment currency. If `C`, then baseCcy; if `P`, then quoteCcy. |
| expTime | String | Expiry time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| interestAccrualTime | String | Interest accrual start time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| listTime | String | Product launch time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| minSize | String | Minimum trade size in notional currency |
| maxSize | String | Maximum trade size in notional currency |
| optType | String | Option type<br>`C`: Call<br>`P`: Put |
| productId | String | Product ID |
| quoteTime | String | When product was quoted, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| redeemStartTime | String | Earliest time to request early redemption, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| redeemEndTime | String | Latest time to request early redemption, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| stepSz | String | Trade step size in notional currency |
| tradeEndTime | String | Trade end time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uly | String | Underlying |
| strike | String | Strike price |

### POST / Request for quote

Requests a real-time quote for a dual investment product. The quote has a TTL and must be used before expiry.

#### Rate Limit: 10 requests per 60 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/sfp/dcd/quote`

> Request Example

```
POST /api/v5/finance/sfp/dcd/quote
body
{
    "productId": "BTC-USDT-260327-77000-C",
    "notionalSz": "1.5",
    "notionalCcy": "BTC"
}
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| productId | String | Yes | Product ID |
| notionalSz | String | Yes | Investment size |
| notionalCcy | String | Yes | Investment currency |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "absYield": "0.00135182",
            "annualizedYield": "69.65",
            "interestAccrualTime": "1773241200000",
            "notionalSz": "0.001",
            "notionalCcy": "BTC",
            "productId": "BTC-USDT-260312-72000-C",
            "quoteId": "qtbcDCD-QUOTE17732395560537636",
            "validUntil": "1774584000000",
            "idxPx": "69000"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| absYield | String | Absolute yield |
| annualizedYield | String | Annualized yield |
| interestAccrualTime | String | Interest accrual start time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| notionalSz | String | Investment size |
| notionalCcy | String | Investment currency |
| productId | String | Product ID |
| quoteId | String | Quote ID |
| validUntil | String | Quote valid until, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| idxPx | String | Index price |

### POST / Trade

Places a dual investment order using a valid quote.

#### Rate Limit: 2 requests per 60 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/sfp/dcd/trade`

> Request Example

```
POST /api/v5/finance/sfp/dcd/trade
body
{
    "quoteId": "quoterbpDCD-QUOTE17732116652401234"
}
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| quoteId | String | Yes | Quote ID |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "quoteId": "quoterbpDCD-QUOTE17732116652401234",
            "ordId": "987654321",
            "state": "live"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| quoteId | String | Quote ID |
| ordId | String | Order ID |
| state | String | Order state<br>`initial`: request has been received by system, will further process<br>`pending_book`: trade received by liquidity provider, pending further processing<br>`live`: trade is live<br>`rejected`: trade has been rejected |

### POST / Request for redeem quote

Requests an early redemption quote for a live dual investment order. This is step 1 of the two-step early redemption flow; call POST / Redeem to confirm.

#### Rate Limit: 10 requests per 60 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/sfp/dcd/redeem-quote`

> Request Example

```
POST /api/v5/finance/sfp/dcd/redeem-quote
body
{
    "ordId": "987654321"
}
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "ordId": "987654321",
            "quoteId": "quoterbcDCD-REDEEM17732116652401234",
            "redeemCcy": "BTC",
            "redeemSz": "1.4856",
            "termRate": "-0.50",
            "validUntil": "1774598400000"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ordId | String | Order ID |
| quoteId | String | Quote ID |
| redeemSz | String | Redeem size |
| redeemCcy | String | Redeem currency |
| termRate | String | Term rate |
| validUntil | String | Redeem quote valid until, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### POST / Redeem

Confirms early redemption using a valid redeem quote. This is step 2 of the two-step early redemption flow.

#### Rate Limit: 2 requests per 60 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/finance/sfp/dcd/redeem`

> Request Example

```
POST /api/v5/finance/sfp/dcd/redeem
body
{
    "ordId": "987654321",
    "quoteId": "quoterbcDCD-REDEEM17732116652401234"
}
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |
| quoteId | String | Yes | Quote ID |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "ordId": "987654321",
            "state": "pending_redeem_booking"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ordId | String | Order ID |
| state | String | order state<br>`pending_redeem_booking`: redeem received, waiting for liquidity provider further processing<br>`pending_redeem`: liquidity provider booked, waiting for transfer<br>`redeeming`: redemption in progress<br>`redeemed`: redemption completed |

### GET / Order state

Returns the current state of a dual investment order.

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/sfp/dcd/order-status`

> Request Example

```
GET /api/v5/finance/sfp/dcd/order-status?ordId=987654321
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "ordId": "987654321",
            "state": "live"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ordId | String | Order ID |
| state | String | Order state<br>`initial`<br>`live`<br>`pending_settle`<br>`settled`<br>`pending_redeem`<br>`redeemed`<br>`rejected` |

### GET / Order history

Return dual investment history orders

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/finance/sfp/dcd/order-history`

> Request Example

```
GET /api/v5/finance/sfp/dcd/order-history
```

#### Request Parameters

| **Parameter** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | No | Order ID. When provided, returns that specific order directly (ignores other filters) |
| productId | String | No | Product ID, e.g. `BTC-USDT-260327-77000-C` |
| uly | String | No | Underlying index, e.g. `BTC-USD` |
| state | String | No | Order state filter<br>`initial`<br>`live`<br>`pending_settle`<br>`settled`<br>`pending_redeem`<br>`redeemed`<br>`rejected` |
| beginId | String | No | Return records newer than this order ID |
| endId | String | No | Return records earlier than this order ID |
| begin | String | No | Begin timestamp filter, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| end | String | No | End timestamp filter, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request, max 100 |

> Response Example

```
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "ordId": "987654321",
            "quoteId": "quoterbpDCD-QUOTE17732116652401234",
            "state": "settled",
            "productId": "BTC-USDT-260327-77000-C",
            "baseCcy": "BTC",
            "quoteCcy": "USDT",
            "uly": "BTC-USD",
            "strike": "77000",
            "notionalSz": "1.5",
            "notionalCcy": "BTC",
            "absYield": "0.00806038",
            "annualizedYield": "0.1834",
            "yieldSz": "0.01209057",
            "yieldCcy": "BTC",
            "settleSz": "1.51209057",
            "settleCcy": "BTC",
            "settlePx": "76500",
            "settleTime": "1774598400000",
            "expTime": "1774598400000",
            "redeemStartTime" : "1774598400000",
            "redeemEndime": "1774598400000",
            "cTime": "1773212400000",
            "uTime": "1773212400000"
        }
    ]
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ordId | String | Order ID |
| quoteId | String | Quote ID |
| state | String | Order state<br>`initial`<br>`live`<br>`pending_settle`<br>`settled`<br>`pending_redeem`<br>`redeemed`<br>`rejected` |
| productId | String | Product ID, e.g. `BTC-USDT-260327-77000-C` |
| baseCcy | String | Base currency, e.g. `BTC` |
| quoteCcy | String | Quote currency, e.g. `USDT` |
| uly | String | Underlying index, e.g. `BTC-USD` |
| strike | String | Strike price |
| notionalSz | String | Notional size |
| notionalCcy | String | Notional currency |
| absYield | String | Absolute yield rate |
| annualizedYield | String | Annual yield rate |
| yieldSz | String | Yield size |
| yieldCcy | String | Yield currency |
| settleSz | String | Settlement size ("" if not yet settled) |
| settleCcy | String | Settlement currency ("" if not yet settled) |
| settlePx | String | Settlement price ("" if not yet settled) |
| expTime | String | Product expiration time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| settleTime | String | Actual settled time, Unix timestamp format in milliseconds, e.g. `1597026383085` ("" if not yet settled) |
| redeemStartTime | String | Earliest time to request early redemption, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| redeemEndTime | String | Latest time to request early redemption, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| cTime | String | Order creation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Last update time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
