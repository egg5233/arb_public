# OKX Funding Account API

Source: https://www.okx.com/docs-v5/en/

---

# Funding Account

The API endpoints of `Funding Account` require authentication.

## REST API

### Get currencies

Retrieve a list of all currencies available which are related to the current account's KYC entity.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/currencies`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/currencies
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get currencies
result = fundingAPI.get_currencies()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Single currency or multiple currencies separated with comma, e.g. `BTC` or `BTC,ETH`. |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "burningFeeRate": "",
        "canDep": true,
        "canInternal": true,
        "canWd": true,
        "ccy": "BTC",
        "chain": "BTC-Bitcoin",
        "ctAddr": "",
        "depEstOpenTime": "",
        "depQuotaFixed": "",
        "depQuoteDailyLayer2": "",
        "fee": "0.00005",
        "logoLink": "https://static.coinall.ltd/cdn/oksupport/asset/currency/icon/btc20230419112752.png",
        "mainNet": true,
        "maxFee": "0.00005",
        "maxFeeForCtAddr": "",
        "maxWd": "500",
        "minDep": "0.0005",
        "minDepArrivalConfirm": "1",
        "minFee": "0.00005",
        "minFeeForCtAddr": "",
        "minInternal": "0.0001",
        "minWd": "0.0005",
        "minWdUnlockConfirm": "2",
        "name": "Bitcoin",
        "needTag": false,
        "usedDepQuotaFixed": "",
        "usedWdQuota": "0",
        "wdEstOpenTime": "",
        "wdQuota": "10000000",
        "wdTickSz": "8"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `BTC` |
| name | String | Name of currency. There is no related name when it is not shown. |
| logoLink | String | The logo link of currency |
| chain | String | Chain name, e.g. `USDT-ERC20`, `USDT-TRC20` |
| ctAddr | String | Contract address |
| canDep | Boolean | The availability to deposit from chain <br>`false`: not available <br>`true`: available |
| canWd | Boolean | The availability to withdraw to chain <br>`false`: not available <br>`true`: available |
| canInternal | Boolean | The availability to internal transfer <br>`false`: not available <br>`true`: available |
| depEstOpenTime | String | Estimated opening time for deposit, Unix timestamp format in milliseconds, e.g. `1597026383085`<br>if `canDep` is `true`, it returns `""` |
| wdEstOpenTime | String | Estimated opening time for withdraw, Unix timestamp format in milliseconds, e.g. `1597026383085`<br>if `canWd` is `true`, it returns `""` |
| minDep | String | The minimum deposit amount of currency in a single transaction |
| minWd | String | The minimum `on-chain withdrawal` amount of currency in a single transaction |
| minInternal | String | The minimum `internal transfer` amount of currency in a single transaction<br>No maximum `internal transfer` limit in a single transaction, subject to the withdrawal limit in the past 24 hours(`wdQuota`). |
| maxWd | String | The maximum amount of currency `on-chain withdrawal` in a single transaction |
| wdTickSz | String | The withdrawal precision, indicating the number of digits after the decimal point.<br>The withdrawal fee precision kept the same as withdrawal precision.<br>The accuracy of internal transfer withdrawal is 8 decimal places. |
| wdQuota | String | The withdrawal limit in the past 24 hours (including `on-chain withdrawal` and `internal transfer`), unit in `USD` |
| usedWdQuota | String | The amount of currency withdrawal used in the past 24 hours, unit in `USD` |
| fee | String | The fixed withdrawal fee<br>Apply to `on-chain withdrawal` |
| minFee | String | ~~The minimum withdrawal fee for normal address<br>Apply to `on-chain withdrawal`~~<br>(Deprecated) |
| maxFee | String | ~~The maximum withdrawal fee for normal address<br>Apply to `on-chain withdrawal`~~<br>(Deprecated) |
| minFeeForCtAddr | String | ~~The minimum withdrawal fee for contract address<br>Apply to `on-chain withdrawal`~~<br>(Deprecated) |
| maxFeeForCtAddr | String | ~~The maximum withdrawal fee for contract address<br>Apply to `on-chain withdrawal`~~<br>(Deprecated) |
| burningFeeRate | String | Burning fee rate, e.g "0.05" represents "5%".<br>Some currencies may charge combustion fees. The burning fee is deducted based on the withdrawal quantity (excluding gas fee) multiplied by the burning fee rate.<br>Apply to `on-chain withdrawal` |
| mainNet | Boolean | If current chain is main net, then it will return `true`, otherwise it will return `false` |
| needTag | Boolean | Whether tag/memo information is required for withdrawal, e.g. `EOS` will return `true` |
| minDepArrivalConfirm | String | The minimum number of blockchain confirmations to acknowledge fund deposit. The account is credited after that, but the deposit can not be withdrawn |
| minWdUnlockConfirm | String | The minimum number of blockchain confirmations required for withdrawal of a deposit |
| depQuotaFixed | String | The fixed deposit limit, unit in `USD`<br>Return empty string if there is no deposit limit |
| usedDepQuotaFixed | String | The used amount of fixed deposit quota, unit in `USD`<br>Return empty string if there is no deposit limit |
| depQuoteDailyLayer2 | String | The layer2 network daily deposit limit |

### Get balance

Retrieve the funding account balances of all the assets and the amount that is available or on hold.

Only asset information of a currency with a balance greater than 0 will be returned.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/balances`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/balances
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get balane
result = fundingAPI.get_balances()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Single currency or multiple currencies (no more than 20) separated with comma, e.g. `BTC` or `BTC,ETH`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [
        {
            "availBal": "37.11827078",
            "bal": "37.11827078",
            "ccy": "ETH",
            "frozenBal": "0"
        }
    ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| bal | String | Balance |
| frozenBal | String | Frozen balance |
| availBal | String | Available balance |

### Get non-tradable assets

Retrieve the funding account balances of all the assets and the amount that is available or on hold.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/non-tradable-assets`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/non-tradable-assets
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

result = fundingAPI.get_non_tradable_assets()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Single currency or multiple currencies (no more than 20) separated with comma, e.g. `BTC` or `BTC,ETH`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "bal": "989.84719571",
            "burningFeeRate": "",
            "canWd": true,
            "ccy": "CELT",
            "chain": "CELT-OKTC",
            "ctAddr": "f403fb",
            "fee": "2",
            "feeCcy": "USDT",
            "logoLink": "https://static.coinall.ltd/cdn/assets/imgs/221/460DA8A592400393.png",
            "minWd": "0.1",
            "name": "",
            "needTag": false,
            "wdAll": false,
            "wdTickSz": "8"
        },
        {
            "bal": "0.001",
            "burningFeeRate": "",
            "canWd": true,
            "ccy": "MEME",
            "chain": "MEME-ERC20",
            "ctAddr": "09b760",
            "fee": "5",
            "feeCcy": "USDT",
            "logoLink": "https://static.coinall.ltd/cdn/assets/imgs/207/2E664E470103C613.png",
            "minWd": "0.001",
            "name": "MEME Inu",
            "needTag": false,
            "wdAll": false,
            "wdTickSz": "8"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. `CELT` |
| name | String | Chinese name of currency. There is no related name when it is not shown. |
| logoLink | String | Logo link of currency |
| bal | String | Withdrawable balance |
| canWd | Boolean | Availability to withdraw to chain. <br>`false`: not available `true`: available |
| chain | String | Chain for withdrawal |
| minWd | String | Minimum withdrawal amount of currency in a single transaction |
| wdAll | Boolean | Whether all assets in this currency must be withdrawn at one time |
| fee | String | Fixed withdrawal fee |
| feeCcy | String | Fixed withdrawal fee unit, e.g. `USDT` |
| burningFeeRate | String | Burning fee rate, e.g "0.05" represents "5%".<br>Some currencies may charge combustion fees. The burning fee is deducted based on the withdrawal quantity (excluding gas fee) multiplied by the burning fee rate. |
| ctAddr | String | Last 6 digits of contract address |
| wdTickSz | String | Withdrawal precision, indicating the number of digits after the decimal point |
| needTag | Boolean | Whether tag/memo information is required for withdrawal |

### Get account asset valuation

View account asset valuation

#### Rate Limit: 1 request per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/asset-valuation`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/asset-valuation
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get account asset valuation
result = fundingAPI.get_asset_valuation()
print(result)
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Asset valuation calculation unit <br>BTC, USDT<br>USD, CNY, JP, KRW, RUB, EUR<br>VND, IDR, INR, PHP, THB, TRY <br>AUD, SGD, ARS, SAR, AED, IQD <br>The default is the valuation in BTC. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "details": {
                "classic": "124.6",
                "earn": "1122.73",
                "funding": "0.09",
                "trading": "2544.28"
            },
            "totalBal": "3790.09",
            "ts": "1637566660769"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| totalBal | String | Valuation of total account assets |
| ts | String | Unix timestamp format in milliseconds, e.g.`1597026383085` |
| details | Object | Asset valuation details for each account |
| \> funding | String | Funding account |
| \> trading | String | Trading account |
| \> classic | String | \[Deprecated\] Classic account |
| \> earn | String | Earn account |

### Funds transfer

Only API keys with `Trade` privilege can call this endpoint.

This endpoint supports the transfer of funds between your funding account and trading account, and from the master account to sub-accounts.

Sub-account can transfer out to master account by default. Need to call [Set permission of transfer out](https://www.okx.com/docs-v5/en/#sub-account-rest-api-set-permission-of-transfer-out) to grant privilege first if you want sub-account transferring to another sub-account (sub-accounts need to belong to same master account.)

The success or failure of the request does not necessarily reflect the actual transfer result. Recommend checking the transfer status by calling "Get funds transfer state" to confirm the final result.

#### Rate Limit: 2 requests per second

#### Rate limit rule: User ID + Currency

#### Permission: Trade

#### HTTP Request

`POST /api/v5/asset/transfer`

> Request Example

```
Copy to Clipboard
# Transfer 1.5 USDT from funding account to Trading account when current account is master-account
POST /api/v5/asset/transfer
body
{
    "ccy":"USDT",
    "amt":"1.5",
    "from":"6",
    "to":"18"
}

# Transfer 1.5 USDT from funding account to subAccount when current account is master-account
POST /api/v5/asset/transfer
body
{
    "ccy":"USDT",
    "type":"1",
    "amt":"1.5",
    "from":"6",
    "to":"6",
    "subAcct":"mini"
}

# Transfer 1.5 USDT from funding account to subAccount when current account is sub-account
POST /api/v5/asset/transfer
body
{
    "ccy":"USDT",
    "type":"4",
    "amt":"1.5",
    "from":"6",
    "to":"6",
    "subAcct":"mini"
}
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Funds transfer
result = fundingAPI.funds_transfer(
    ccy="USDT",
    amt="1.5",
    from_="6",
    to="18"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| type | String | No | Transfer type<br>`0`: transfer within account<br>`1`: master account to sub-account (Only applicable to API Key from master account)<br>`2`: sub-account to master account (Only applicable to API Key from master account)<br>`3`: sub-account to master account (Only applicable to APIKey from sub-account)<br>`4`: sub-account to sub-account (Only applicable to APIKey from sub-account, and target account needs to be another sub-account which belongs to same master account. Sub-account directly transfer out permission is disabled by default, set permission please refer to [Set permission of transfer out](https://www.okx.com/docs-v5/en/#sub-account-rest-api-set-permission-of-transfer-out))<br>The default is `0`.<br>If you want to make transfer between sub-accounts by master account API key, refer to [Master accounts manage the transfers between sub-accounts](https://www.okx.com/docs-v5/en/#sub-account-rest-api-master-accounts-manage-the-transfers-between-sub-accounts) |
| ccy | String | Yes | Transfer currency, e.g. `USDT` |
| amt | String | Yes | Amount to be transferred |
| from | String | Yes | The remitting account<br>`6`: Funding account<br>`18`: Trading account |
| to | String | Yes | The beneficiary account<br>`6`: Funding account<br>`18`: Trading account |
| subAcct | String | Conditional | Name of the sub-account<br>When `type` is `1`/`2`/`4`, this parameter is required. |
| loanTrans | Boolean | No | Whether or not borrowed coins can be transferred out under `Spot mode`/`Multi-currency margin`/`Portfolio margin`<br>`true`: borrowed coins can be transferred out<br>`false`: borrowed coins cannot be transferred out<br>the default is `false` |
| omitPosRisk | String | No | Ignore position risk<br>Default is `false`<br>Applicable to `Portfolio margin` |
| clientId | String | No | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "transId": "754147",
      "ccy": "USDT",
      "clientId": "",
      "from": "6",
      "amt": "0.1",
      "to": "18"
    }
  ]
}
```

#### Response Parameters

> Response Example

| Parameter | Type | Description |
| --- | --- | --- |
| transId | String | Transfer ID |
| clientId | String | Client-supplied ID |
| ccy | String | Currency |
| from | String | The remitting account |
| amt | String | Transfer amount |
| to | String | The beneficiary account |

### Get funds transfer state

Retrieve the transfer state data of the last 2 weeks.

#### Rate Limit: 10 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/transfer-state`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/transfer-state?transId=1&type=1
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "1"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get funds transfer state
result = fundingAPI.transfer_state(
    transId="248424899",
    type="0"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| transId | String | Conditional | Transfer ID<br>Either transId or clientId is required. If both are passed, transId will be used. |
| clientId | String | Conditional | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| type | String | No | Transfer type<br>`0`: transfer within account <br>`1`: master account to sub-account (Only applicable to API Key from master account) <br>`2`: sub-account to master account (Only applicable to API Key from master account)<br>`3`: sub-account to master account (Only applicable to APIKey from sub-account)<br>`4`: sub-account to sub-account (Only applicable to APIKey from sub-account, and target account needs to be another sub-account which belongs to same master account)<br>The default is `0`.<br>For Custody accounts, can choose not to pass this parameter or pass `0`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "amt": "1.5",
            "ccy": "USDT",
            "clientId": "",
            "from": "18",
            "instId": "", //deprecated
            "state": "success",
            "subAcct": "test",
            "to": "6",
            "toInstId": "", //deprecated
            "transId": "1",
            "type": "1"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| transId | String | Transfer ID |
| clientId | String | Client-supplied ID |
| ccy | String | Currency, e.g. `USDT` |
| amt | String | Amount to be transferred |
| type | String | Transfer type<br>`0`: transfer within account<br>`1`: master account to sub-account (Only applicable to API Key from master account) <br>`2`: sub-account to master account (Only applicable to APIKey from master account)<br>`3`: sub-account to master account (Only applicable to APIKey from sub-account)<br>`4`: sub-account to sub-account (Only applicable to APIKey from sub-account, and target account needs to be another sub-account which belongs to same master account) |
| from | String | The remitting account<br>`6`: Funding account<br>`18`: Trading account |
| to | String | The beneficiary account<br>`6`: Funding account<br>`18`: Trading account |
| subAcct | String | Name of the sub-account |
| instId | String | deprecated |
| toInstId | String | deprecated |
| state | String | Transfer state<br>`success`<br>`pending`<br>`failed` |

### Asset bills details

Query the billing record in the past month.

#### Rate Limit: 6 Requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/bills`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/bills
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get asset bills details
result = fundingAPI.get_bills()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | No | Currency |
| type | String | No | Bill type<br>`1`: Deposit<br>`2`: Withdrawal<br>`13`: Canceled withdrawal<br>`20`: Transfer to sub account (for master account)<br>`21`: Transfer from sub account (for master account)<br>`22`: Transfer out from sub to master account (for sub-account)<br>`23`: Transfer in from master to sub account (for sub-account)<br>`28`: Manually claimed Airdrop<br>`47`: System reversal<br>`48`: Event Reward<br>`49`: Event Giveaway<br>`68`: Fee rebate (by rebate card)<br>`72`: Token received<br>`73`: Token given away<br>`74`: Token refunded<br>`75`: \[Simple earn flexible\] Subscription<br>`76`: \[Simple earn flexible\] Redemption<br>`77`: Jumpstart distribute<br>`78`: Jumpstart lock up<br>`80`: DEFI/Staking subscription<br>`82`: DEFI/Staking redemption<br>`83`: Staking yield<br>`84`: Violation fee<br>`89`: Deposit yield<br>`116`: \[Fiat\] Place an order<br>`117`: \[Fiat\] Fulfill an order<br>`118`: \[Fiat\] Cancel an order<br>`124`: Jumpstart unlocking<br>`130`: Transferred from Trading account<br>`131`: Transferred to Trading account<br>`132`: \[P2P\] Frozen by customer service<br>`133`: \[P2P\] Unfrozen by customer service<br>`134`: \[P2P\] Transferred by customer service<br>`135`: Cross chain exchange<br>`137`: \[ETH Staking\] Subscription<br>`138`: \[ETH Staking\] Swapping<br>`139`: \[ETH Staking\] Earnings<br>`146`: Customer feedback<br>`150`: Affiliate commission<br>`151`: Referral reward<br>`152`: Broker reward<br>`160`: Dual Investment subscribe<br>`161`: Dual Investment collection<br>`162`: Dual Investment profit<br>`163`: Dual Investment refund<br>`172`: \[Affiliate\] Sub-affiliate commission<br>`173`: \[Affiliate\] Fee rebate (by trading fee)<br>`174`: Jumpstart Pay<br>`175`: Locked collateral<br>`176`: Loan<br>`177`: Added collateral<br>`178`: Returned collateral<br>`179`: Repayment<br>`180`: Unlocked collateral<br>`181`: Airdrop payment<br>`185`: \[Broker\] Convert reward<br>`187`: \[Broker\] Convert transfer<br>`189`: Mystery box bonus<br>`195`: Untradable asset withdrawal<br>`196`: Untradable asset withdrawal revoked<br>`197`: Untradable asset deposit<br>`198`: Untradable asset collection reduce<br>`199`: Untradable asset collection increase<br>`200`: Buy<br>`202`: Price Lock Subscribe<br>`203`: Price Lock Collection<br>`204`: Price Lock Profit<br>`205`: Price Lock Refund<br>`207`: Dual Investment Lite Subscribe<br>`208`: Dual Investment Lite Collection<br>`209`: Dual Investment Lite Profit<br>`210`: Dual Investment Lite Refund<br>`212`: \[Flexible loan\] Multi-collateral loan collateral locked<br>`215`: \[Flexible loan\] Multi-collateral loan collateral released<br>`217`: \[Flexible loan\] Multi-collateral loan borrowed<br>`218`: \[Flexible loan\] Multi-collateral loan repaid<br>`232`: \[Flexible loan\] Subsidized interest received<br>`220`: Delisted crypto<br>`221`: Blockchain's withdrawal fee<br>`222`: Withdrawal fee refund<br>`223`: SWAP lead trading profit share<br>`225`: Shark Fin subscribe<br>`226`: Shark Fin collection<br>`227`: Shark Fin profit<br>`228`: Shark Fin refund<br>`229`: Airdrop<br>`232`: Subsidized interest received<br>`233`: Broker rebate compensation<br>`240`: Snowball subscribe<br>`241`: Snowball refund<br>`242`: Snowball profit<br>`243`: Snowball trading failed<br>`249`: Seagull subscribe<br>`250`: Seagull collection<br>`251`: Seagull profit<br>`252`: Seagull refund<br>`263`: Strategy bots profit share<br>`265`: Signal revenue<br>`266`: SPOT lead trading profit share<br>`270`: DCD broker transfer<br>`271`: DCD broker rebate<br>`272`: \[Convert\] Buy Crypto/Fiat<br>`273`: \[Convert\] Sell Crypto/Fiat<br>`284`: \[Custody\] Transfer out trading sub-account<br>`285`: \[Custody\] Transfer in trading sub-account<br>`286`: \[Custody\] Transfer out custody funding account<br>`287`: \[Custody\] Transfer in custody funding account<br>`288`: \[Custody\] Fund delegation <br>`289`: \[Custody\] Fund undelegation<br>`299`: Affiliate recommendation commission<br>`300`: Fee discount rebate<br>`303`: Snowball market maker transfer<br>~~`304`: \[Simple Earn Fixed\] Order submission~~<br>~~`305`: \[Simple Earn Fixed\] Order redemption~~<br>~~`306`: \[Simple Earn Fixed\] Principal distribution~~<br>~~`307`: \[Simple Earn Fixed\] Interest distribution (early termination compensation)~~<br>~~`308`: \[Simple Earn Fixed\] Interest distribution~~<br>~~`309`: \[Simple Earn Fixed\] Interest distribution (extension compensation)~~<br>`311`: Crypto dust auto-transfer in<br>`313`: Sent by gift<br>`314`: Received from gift<br>`315`: Refunded from gift<br>`328`: \[SOL staking\] Send Liquidity Staking Token reward<br>`329`: \[SOL staking\] Subscribe Liquidity Staking Token staking<br>`330`: \[SOL staking\] Mint Liquidity Staking Token<br>`331`: \[SOL staking\] Redeem Liquidity Staking Token order<br>`332`: \[SOL staking\] Settle Liquidity Staking Token order<br>`333`: Trial fund reward<br>`339`: \[Simple Earn Fixed\] Order submission<br>`340`: \[Simple Earn Fixed\] Order failure refund<br>`341`: \[Simple Earn Fixed\] Redemption<br>`342`: \[Simple Earn Fixed\] Principal<br>`343`: \[Simple Earn Fixed\] Interest<br>`344`: \[Simple Earn Fixed\] Compensatory interest<br>`345`: \[Institutional Loan\] Principal repayment<br>`346`: \[Institutional Loan\] Interest repayment<br>`347`: \[Institutional Loan\] Overdue penalty<br>`348`: \[BTC staking\] Subscription<br>`349`: \[BTC staking\] Redemption<br>`350`: \[BTC staking\] Earnings<br>`351`: \[Institutional Loan\] Loan disbursement<br>`354`: Copy and bot rewards<br>`361`: Deposit from closed sub-account<br>`372`: Asset segregation<br>`373`: Asset release<br>`400`: Auto lend interest <br>`408`: Auto earn USDG interest<br>`476`: Transferred out to Cloud Exchange<br>`477`: Transferred in from Cloud Exchange |
| clientId | String | No | Client-supplied ID for transfer or withdrawal<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| after | String | No | Pagination of data to return records earlier than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [{
        "billId": "12344",
        "ccy": "BTC",
        "clientId": "",
        "balChg": "2",
        "bal": "12",
        "type": "1",
        "ts": "1597026383085",
        "notes": ""
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| billId | String | Bill ID |
| ccy | String | Account balance currency |
| clientId | String | Client-supplied ID for transfer or withdrawal |
| balChg | String | Change in balance at the account level |
| bal | String | Balance at the account level |
| type | String | Bill type |
| notes | String | Notes |
| ts | String | Creation time, Unix timestamp format in milliseconds, e.g.`1597026383085` |

### Asset bills history

Query the billing records of all time since 1 February, 2021.

⚠️ **IMPORTANT**: Data updates occur every 30 seconds. Update frequency may vary based on data volume - please be aware of potential delays during high-traffic periods.

#### Rate Limit: 1 Requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/bills-history`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/bills-history
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get asset bills details
result = fundingAPI.get_bills_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | No | Currency |
| type | String | No | Bill type<br>`1`: Deposit<br>`2`: Withdrawal<br>`13`: Canceled withdrawal<br>`20`: Transfer to sub account (for master account)<br>`21`: Transfer from sub account (for master account)<br>`22`: Transfer out from sub to master account (for sub-account)<br>`23`: Transfer in from master to sub account (for sub-account)<br>`28`: Manually claimed Airdrop<br>`47`: System reversal<br>`48`: Event Reward<br>`49`: Event Giveaway<br>`68`: Fee rebate (by rebate card)<br>`72`: Token received<br>`73`: Token given away<br>`74`: Token refunded<br>`75`: \[Simple earn flexible\] Subscription<br>`76`: \[Simple earn flexible\] Redemption<br>`77`: Jumpstart distribute<br>`78`: Jumpstart lock up<br>`80`: DEFI/Staking subscription<br>`82`: DEFI/Staking redemption<br>`83`: Staking yield<br>`84`: Violation fee<br>`89`: Deposit yield<br>`116`: \[Fiat\] Place an order<br>`117`: \[Fiat\] Fulfill an order<br>`118`: \[Fiat\] Cancel an order<br>`124`: Jumpstart unlocking<br>`130`: Transferred from Trading account<br>`131`: Transferred to Trading account<br>`132`: \[P2P\] Frozen by customer service<br>`133`: \[P2P\] Unfrozen by customer service<br>`134`: \[P2P\] Transferred by customer service<br>`135`: Cross chain exchange<br>`137`: \[ETH Staking\] Subscription<br>`138`: \[ETH Staking\] Swapping<br>`139`: \[ETH Staking\] Earnings<br>`146`: Customer feedback<br>`150`: Affiliate commission<br>`151`: Referral reward<br>`152`: Broker reward<br>`160`: Dual Investment subscribe<br>`161`: Dual Investment collection<br>`162`: Dual Investment profit<br>`163`: Dual Investment refund<br>`172`: \[Affiliate\] Sub-affiliate commission<br>`173`: \[Affiliate\] Fee rebate (by trading fee)<br>`174`: Jumpstart Pay<br>`175`: Locked collateral<br>`176`: Loan<br>`177`: Added collateral<br>`178`: Returned collateral<br>`179`: Repayment<br>`180`: Unlocked collateral<br>`181`: Airdrop payment<br>`185`: \[Broker\] Convert reward<br>`187`: \[Broker\] Convert transfer<br>`189`: Mystery box bonus<br>`195`: Untradable asset withdrawal<br>`196`: Untradable asset withdrawal revoked<br>`197`: Untradable asset deposit<br>`198`: Untradable asset collection reduce<br>`199`: Untradable asset collection increase<br>`200`: Buy<br>`202`: Price Lock Subscribe<br>`203`: Price Lock Collection<br>`204`: Price Lock Profit<br>`205`: Price Lock Refund<br>`207`: Dual Investment Lite Subscribe<br>`208`: Dual Investment Lite Collection<br>`209`: Dual Investment Lite Profit<br>`210`: Dual Investment Lite Refund<br>`212`: \[Flexible loan\] Multi-collateral loan collateral locked<br>`215`: \[Flexible loan\] Multi-collateral loan collateral released<br>`217`: \[Flexible loan\] Multi-collateral loan borrowed<br>`218`: \[Flexible loan\] Multi-collateral loan repaid<br>`232`: \[Flexible loan\] Subsidized interest received<br>`220`: Delisted crypto<br>`221`: Blockchain's withdrawal fee<br>`222`: Withdrawal fee refund<br>`223`: SWAP lead trading profit share<br>`225`: Shark Fin subscribe<br>`226`: Shark Fin collection<br>`227`: Shark Fin profit<br>`228`: Shark Fin refund<br>`229`: Airdrop<br>`232`: Subsidized interest received<br>`233`: Broker rebate compensation<br>`240`: Snowball subscribe<br>`241`: Snowball refund<br>`242`: Snowball profit<br>`243`: Snowball trading failed<br>`249`: Seagull subscribe<br>`250`: Seagull collection<br>`251`: Seagull profit<br>`252`: Seagull refund<br>`263`: Strategy bots profit share<br>`265`: Signal revenue<br>`266`: SPOT lead trading profit share<br>`270`: DCD broker transfer<br>`271`: DCD broker rebate<br>`272`: \[Convert\] Buy Crypto/Fiat<br>`273`: \[Convert\] Sell Crypto/Fiat<br>`284`: \[Custody\] Transfer out trading sub-account<br>`285`: \[Custody\] Transfer in trading sub-account<br>`286`: \[Custody\] Transfer out custody funding account<br>`287`: \[Custody\] Transfer in custody funding account<br>`288`: \[Custody\] Fund delegation <br>`289`: \[Custody\] Fund undelegation<br>`299`: Affiliate recommendation commission<br>`300`: Fee discount rebate<br>`303`: Snowball market maker transfer<br>~~`304`: \[Simple Earn Fixed\] Order submission~~<br>~~`305`: \[Simple Earn Fixed\] Order redemption~~<br>~~`306`: \[Simple Earn Fixed\] Principal distribution~~<br>~~`307`: \[Simple Earn Fixed\] Interest distribution (early termination compensation)~~<br>~~`308`: \[Simple Earn Fixed\] Interest distribution~~<br>~~`309`: \[Simple Earn Fixed\] Interest distribution (extension compensation)~~<br>`311`: Crypto dust auto-transfer in<br>`313`: Sent by gift<br>`314`: Received from gift<br>`315`: Refunded from gift<br>`328`: \[SOL staking\] Send Liquidity Staking Token reward<br>`329`: \[SOL staking\] Subscribe Liquidity Staking Token staking<br>`330`: \[SOL staking\] Mint Liquidity Staking Token<br>`331`: \[SOL staking\] Redeem Liquidity Staking Token order<br>`332`: \[SOL staking\] Settle Liquidity Staking Token order<br>`333`: Trial fund reward<br>`339`: \[Simple Earn Fixed\] Order submission<br>`340`: \[Simple Earn Fixed\] Order failure refund<br>`341`: \[Simple Earn Fixed\] Redemption<br>`342`: \[Simple Earn Fixed\] Principal<br>`343`: \[Simple Earn Fixed\] Interest<br>`344`: \[Simple Earn Fixed\] Compensatory interest<br>`345`: \[Institutional Loan\] Principal repayment<br>`346`: \[Institutional Loan\] Interest repayment<br>`347`: \[Institutional Loan\] Overdue penalty<br>`348`: \[BTC staking\] Subscription<br>`349`: \[BTC staking\] Redemption<br>`350`: \[BTC staking\] Earnings<br>`351`: \[Institutional Loan\] Loan disbursement<br>`354`: Copy and bot rewards<br>`361`: Deposit from closed sub-account<br>`372`: Asset segregation<br>`373`: Asset release<br>`400`: auto lend interest<br>`408`: Auto earn interest (USDG earn)<br>`476`: Transferred out to Cloud Exchange<br>`477`: Transferred in from Cloud Exchange |
| clientId | String | No | Client-supplied ID for transfer or withdrawal<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| after | String | No | Pagination of data to return records earlier than the requested `ts` or `billId`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |
| pagingType | String | No | PagingType<br>`1`: Timestamp of the bill record<br>`2`: Bill ID of the bill record<br>The default is `1` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [{
        "billId": "12344",
        "ccy": "BTC",
        "clientId": "",
        "balChg": "2",
        "bal": "12",
        "type": "1",
        "ts": "1597026383085",
        "notes": ""
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| billId | String | Bill ID |
| ccy | String | Account balance currency |
| clientId | String | Client-supplied ID for transfer or withdrawal |
| balChg | String | Change in balance at the account level |
| bal | String | Balance at the account level |
| type | String | Bill type |
| notes | String | Notes |
| ts | String | Creation time, Unix timestamp format in milliseconds, e.g.`1597026383085` |

### Get deposit address

Retrieve the deposit addresses of currencies, including previously-used addresses.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/deposit-address`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/deposit-address?ccy=BTC
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get deposit address
result = fundingAPI.get_deposit_address(
    ccy="USDT"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | Yes | Currency, e.g. `BTC` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "chain": "BTC-Bitcoin",
            "ctAddr": "",
            "ccy": "BTC",
            "to": "6",
            "addr": "39XNxK1Ryqgg3Bsyn6HzoqV4Xji25pNkv6",
            "verifiedName":"John Corner",
            "selected": true
        },
        {
            "chain": "BTC-OKC",
            "ctAddr": "",
            "ccy": "BTC",
            "to": "6",
            "addr": "0x66d0edc2e63b6b992381ee668fbcb01f20ae0428",
            "verifiedName":"John Corner",
            "selected": true
        },
        {
            "chain": "BTC-ERC20",
            "ctAddr": "5807cf",
            "ccy": "BTC",
            "to": "6",
            "addr": "0x66d0edc2e63b6b992381ee668fbcb01f20ae0428",
            "verifiedName":"John Corner",
            "selected": true
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| addr | String | Deposit address |
| tag | String | Deposit tag (This will not be returned if the currency does not require a tag for deposit) |
| memo | String | Deposit memo (This will not be returned if the currency does not require a memo for deposit) |
| pmtId | String | Deposit payment ID (This will not be returned if the currency does not require a payment\_id for deposit) |
| addrEx | Object | Deposit address attachment (This will not be returned if the currency does not require this)<br>e.g. `TONCOIN` attached tag name is `comment`, the return will be `{'comment':'123456'}` |
| ccy | String | Currency, e.g. `BTC` |
| chain | String | Chain name, e.g. `USDT-ERC20`, `USDT-TRC20` |
| to | String | The beneficiary account<br>`6`: Funding account `18`: Trading account<br>The users under some entity (e.g. Brazil) only support deposit to trading account. |
| verifiedName | String | Verified name (for recipient) |
| selected | Boolean | Return `true` if the current deposit address is selected by the website page |
| ctAddr | String | Last 6 digits of contract address |

### Get deposit history

Retrieve the deposit records according to the currency, deposit status, and time range in reverse chronological order. The 100 most recent records are returned by default.

Websocket API is also available, refer to [Deposit info channel](https://www.okx.com/docs-v5/en/#funding-account-websocket-deposit-info-channel).

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/deposit-history`

> Request Example

```
Copy to Clipboard

GET /api/v5/asset/deposit-history

# Query deposit history from 2022-06-01 to 2022-07-01
GET /api/v5/asset/deposit-history?ccy=BTC&after=1654041600000&before=1656633600000
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Get deposit history
result = fundingAPI.get_deposit_history()
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |
| depId | String | No | Deposit ID |
| fromWdId | String | No | Internal transfer initiator's withdrawal ID<br>If the deposit comes from internal transfer, this field displays the withdrawal ID of the internal transfer initiator |
| txId | String | No | Hash record of the deposit |
| type | String | No | Deposit Type<br>`3`: internal transfer<br>`4`: deposit from chain |
| state | String | No | Status of deposit <br>`0`: waiting for confirmation<br>`1`: deposit credited <br>`2`: deposit successful <br>`8`: pending due to temporary deposit suspension on this crypto currency<br>`11`: match the address blacklist<br>`12`: account or deposit is frozen<br>`13`: sub-account deposit interception<br>`14`: KYC limit<br>`17`: Pending response from Travel Rule vendor |
| after | String | No | Pagination of data to return records earlier than the requested ts, Unix timestamp format in milliseconds, e.g. `1654041600000` |
| before | String | No | Pagination of data to return records newer than the requested ts, Unix timestamp format in milliseconds, e.g. `1656633600000` |
| limit | string | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "actualDepBlkConfirm": "2",
        "amt": "1",
        "areaCodeFrom": "",
        "ccy": "USDT",
        "chain": "USDT-TRC20",
        "depId": "88****33",
        "from": "",
        "fromWdId": "",
        "state": "2",
        "to": "TN4hGjVXMzy*********9b4N1aGizqs",
        "ts": "1674038705000",
        "txId": "fee235b3e812********857d36bb0426917f0df1802"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| chain | String | Chain name |
| amt | String | Deposit amount |
| from | String | Deposit account<br>If the deposit comes from an internal transfer, this field displays the account information of the internal transfer initiator, which can be a mobile phone number or email address (masked), and will return "" in other cases |
| areaCodeFrom | String | If `from` is a phone number, this parameter return area code of the phone number |
| to | String | Deposit address<br>If the deposit comes from the on-chain, this field displays the on-chain address, and will return "" in other cases |
| txId | String | Hash record of the deposit |
| ts | String | The timestamp that the deposit record is created, Unix timestamp format in milliseconds, e.g. `1655251200000` |
| state | String | Status of deposit<br>`0`: Waiting for confirmation<br>`1`: Deposit credited <br>`2`: Deposit successful <br>`8`: Pending due to temporary deposit suspension on this crypto currency<br>`11`: Match the address blacklist<br>`12`: Account or deposit is frozen<br>`13`: Sub-account deposit interception<br>`14`: KYC limit |
| depId | String | Deposit ID |
| fromWdId | String | Internal transfer initiator's withdrawal ID<br>If the deposit comes from internal transfer, this field displays the withdrawal ID of the internal transfer initiator, and will return "" in other cases |
| actualDepBlkConfirm | String | The actual amount of blockchain confirmed in a single deposit |

About deposit state

**Waiting for confirmation** is that the required number of blockchain confirmations has not been reached.

**Deposit credited** is that there is sufficient number of blockchain confirmations for the currency to be credited to the account, but it cannot be withdrawn yet.

**Deposit successful** means the crypto has been credited to the account and it can be withdrawn.

### Withdrawal

Only supported withdrawal of assets from funding account. Common sub-account does not support withdrawal.

The API can only make withdrawal to verified addresses/account, and verified addresses can be set by WEB/APP.

About tag

Some token deposits require a deposit address and a tag (e.g. Memo/Payment ID), which is a string that guarantees the uniqueness of your deposit address. Follow the deposit procedure carefully, or you may risk losing your assets.

For currencies with labels, if it is a withdrawal between OKX users, please use internal transfer instead of online withdrawal

The following content only applies to users residing in the United Arab Emirates

Due to local laws and regulations in your country or region, a certain ratio of user assets must be stored in cold wallets. We will perform cold-to-hot wallet asset transfers from time to time. However, if assets in hot wallets are not sufficient to meet user withdrawal demands, an extra step is needed to transfer cold wallet assets to the hot wallet. This may cause delays of up to 24 hours to receive withdrawals.

Learn more (https://www.okx.com/help/what-is-a-segregated-wallet-and-why-is-my-withdrawal-delayed)

Users under certain entities need to provide additional information for withdrawal

Bahamas entity users refer to https://www.okx.com/docs-v5/log\_en/#2024-08-08-withdrawal-api-adjustment-for-bahama-entity-users

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Withdraw

#### HTTP Request

`POST /api/v5/asset/withdrawal`

> Request Example

```
Copy to Clipboard
# on-chain withdrawal
POST /api/v5/asset/withdrawal
body
{
    "amt":"1",
    "dest":"4",
    "ccy":"BTC",
    "chain":"BTC-Bitcoin",
    "toAddr":"17DKe3kkkkiiiiTvAKKi2vMPbm1Bz3CMKw"
}

# internal withdrawal
POST /api/v5/asset/withdrawal
body
{
    "amt":"10",
    "dest":"3",
    "ccy":"USDT",
    "areaCode":"86",
    "toAddr":"15651000000"
}

# Specific entity users need to provide receiver's info
POST /api/v5/asset/withdrawal
body
{
    "amt":"1",
    "dest":"4",
    "ccy":"BTC",
    "chain":"BTC-Bitcoin",
    "toAddr":"17DKe3kkkkiiiiTvAKKi2vMPbm1Bz3CMKw",
    "rcvrInfo":{
        "walletType":"exchange",
        "exchId":"did:ethr:0xfeb4f99829a9acdf52979abee87e83addf22a7e1",
        "rcvrFirstName":"Bruce",
        "rcvrLastName":"Wayne"
    }
}
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Withdrawal
result = fundingAPI.withdrawal(
    ccy="USDT",
    toAddr="TXtvfb7cdrn6VX9H49mgio8bUxZ3DGfvYF",
    amt="100",
    dest="4",
    chain="USDT-TRC20"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | Yes | Currency, e.g. `USDT` |
| amt | String | Yes | Withdrawal amount<br>Withdrawal fee is not included in withdrawal amount. Please reserve sufficient transaction fees when withdrawing.<br>You can get fee amount by [Get currencies](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-currencies).<br>For `internal transfer`, transaction fee is always `0`. |
| dest | String | Yes | Withdrawal method<br>`3`: internal transfer<br>`4`: on-chain withdrawal |
| toAddr | String | Yes | `toAddr` should be a trusted address/account. <br>If your `dest` is `4`, some crypto currency addresses are formatted as `'address:tag'`, e.g. `'ARDOR-7JF3-8F2E-QUWZ-CAN7F:123456'`<br>If your `dest` is `3`,`toAddr` should be a recipient address which can be UID, email, phone or login account name (account name is only for sub-account). |
| toAddrType | String | No | Address type<br>`1`: wallet address, email, phone, or login account name<br>`2`: UID (applicable only when dest=`3`) |
| chain | String | Conditional | Chain name<br>There are multiple chains under some currencies, such as `USDT` has `USDT-ERC20`, `USDT-TRC20`<br>If the parameter is not filled in, the default will be the main chain.<br>When you withdrawal the non-tradable asset, if the parameter is not filled in, the default will be the unique withdrawal chain.<br>Apply to `on-chain withdrawal`.<br>You can get supported chain name by the endpoint of [Get currencies](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-currencies). |
| areaCode | String | Conditional | Area code for the phone number, e.g. `86`<br>If `toAddr` is a phone number, this parameter is required.<br>Apply to `internal transfer` |
| rcvrInfo | Object | Conditional | Recipient information<br>For the specific entity users to do on-chain withdrawal/lightning withdrawal, this information is required. |
| \> walletType | String | Yes | Wallet Type<br>`exchange`: Withdraw to exchange wallet<br>`private`: Withdraw to private wallet<br>For the wallet belongs to business recipient, `rcvrFirstName` may input the company name, `rcvrLastName` may input "N/A", location info may input the registered address of the company. |
| \> exchId | String | Conditional | Exchange ID<br>You can query supported exchanges through the endpoint of [Get exchange list (public)](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-exchange-list-public)<br>If the exchange is not in the exchange list, fill in '0' in this field. <br>Apply to walletType = `exchange` |
| \> rcvrFirstName | String | Conditional | Receiver's first name, e.g. `Bruce` |
| \> rcvrLastName | String | Conditional | Receiver's last name, e.g. `Wayne` |
| \> rcvrCountry | String | Conditional | The recipient's country, e.g. `United States`<br>You must enter an English country name or a two letter country code (ISO 3166-1). Please refer to the `Country Name` and `Country Code` in the country information table below. |
| \> rcvrCountrySubDivision | String | Conditional | State/Province of the recipient, e.g. `California` |
| \> rcvrTownName | String | Conditional | The town/city where the recipient is located, e.g. `San Jose` |
| \> rcvrStreetName | String | Conditional | Recipient's street address, e.g. `Clementi Avenue 1` |
| clientId | String | No | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "msg": "",
    "data": [{
        "amt": "0.1",
        "wdId": "67485",
        "ccy": "BTC",
        "clientId": "",
        "chain": "BTC-Bitcoin"
    }]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| chain | String | Chain name, e.g. `USDT-ERC20`, `USDT-TRC20` |
| amt | String | Withdrawal amount |
| wdId | String | Withdrawal ID |
| clientId | String | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

#### Country information

| Country name | Country code |
| --- | --- |
| Afghanistan | AF |
| Albania | AL |
| Algeria | DZ |
| Andorra | AD |
| Angola | AO |
| Anguilla | AI |
| Antigua and Barbuda | AG |
| Argentina | AR |
| Armenia | AM |
| Australia | AU |
| Austria | AT |
| Azerbaijan | AZ |
| Bahamas | BS |
| Bahrain | BH |
| Bangladesh | BD |
| Barbados | BB |
| Belarus | BY |
| Belgium | BE |
| Belize | BZ |
| Benin | BJ |
| Bermuda | BM |
| Bhutan | BT |
| Bolivia | BO |
| Bosnia and Herzegovina | BA |
| Botswana | BW |
| Brazil | BR |
| British Virgin Islands | VG |
| Brunei | BN |
| Bulgaria | BG |
| Burkina Faso | BF |
| Burundi | BI |
| Cambodia | KH |
| Cameroon | CM |
| Canada | CA |
| Cape Verde | CV |
| Cayman Islands | KY |
| Central African Republic | CF |
| Chad | TD |
| Chile | CL |
| Colombia | CO |
| Comoros | KM |
| Congo (Republic) | CG |
| Congo (Democratic Republic) | CD |
| Costa Rica | CR |
| Cote d´Ivoire (Ivory Coast) | CI |
| Croatia | HR |
| Cuba | CU |
| Cyprus | CY |
| Czech Republic | CZ |
| Denmark | DK |
| Djibouti | DJ |
| Dominica | DM |
| Dominican Republic | DO |
| Ecuador | EC |
| Egypt | EG |
| El Salvador | SV |
| Equatorial Guinea | GQ |
| Eritrea | ER |
| Estonia | EE |
| Ethiopia | ET |
| Fiji | FJ |
| Finland | FI |
| France | FR |
| Gabon | GA |
| Gambia | GM |
| Georgia | GE |
| Germany | DE |
| Ghana | GH |
| Greece | GR |
| Grenada | GD |
| Guatemala | GT |
| Guinea | GN |
| Guinea-Bissau | GW |
| Guyana | GY |
| Haiti | HT |
| Honduras | HN |
| Hong Kong | HK |
| Hungary | HU |
| Iceland | IS |
| India | IN |
| Indonesia | ID |
| Iran | IR |
| Iraq | IQ |
| Ireland | IE |
| Israel | IL |
| Italy | IT |
| Jamaica | JM |
| Japan | JP |
| Jordan | JO |
| Kazakhstan | KZ |
| Kenya | KE |
| Kiribati | KI |
| North Korea | KP |
| South Korea | KR |
| Kuwait | KW |
| Kyrgyzstan | KG |
| Laos | LA |
| Latvia | LV |
| Lebanon | LB |
| Lesotho | LS |
| Liberia | LR |
| Libya | LY |
| Liechtenstein | LI |
| Lithuania | LT |
| Luxembourg | LU |
| Macau | MO |
| Macedonia | MK |
| Madagascar | MG |
| Malawi | MW |
| Malaysia | MY |
| Maldives | MV |
| Mali | ML |
| Malta | MT |
| Marshall Islands | MH |
| Mauritania | MR |
| Mauritius | MU |
| Mexico | MX |
| Micronesia | FM |
| Moldova | MD |
| Monaco | MC |
| Mongolia | MN |
| Montenegro | ME |
| Morocco | MA |
| Mozambique | MZ |
| Myanmar (Burma) | MM |
| Namibia | NA |
| Nauru | NR |
| Nepal | NP |
| Netherlands | NL |
| New Zealand | NZ |
| Nicaragua | NI |
| Niger | NE |
| Nigeria | NG |
| Norway | NO |
| Oman | OM |
| Pakistan | PK |
| Palau | PW |
| Panama | PA |
| Papua New Guinea | PG |
| Paraguay | PY |
| Peru | PE |
| Philippines | PH |
| Poland | PL |
| Portugal | PT |
| Qatar | QA |
| Romania | RO |
| Russia | RU |
| Rwanda | RW |
| Saint Kitts and Nevis | KN |
| Saint Lucia | LC |
| Saint Vincent and the Grenadines | VC |
| Samoa | WS |
| San Marino | SM |
| Sao Tome and Principe | ST |
| Saudi Arabia | SA |
| Senegal | SN |
| Serbia | RS |
| Seychelles | SC |
| Sierra Leone | SL |
| Singapore | SG |
| Slovakia | SK |
| Slovenia | SI |
| Solomon Islands | SB |
| Somalia | SO |
| South Africa | ZA |
| Spain | ES |
| Sri Lanka | LK |
| Sudan | SD |
| Suriname | SR |
| Swaziland | SZ |
| Sweden | SE |
| Switzerland | CH |
| Syria | SY |
| Taiwan | TW |
| Tajikistan | TJ |
| Tanzania | TZ |
| Thailand | TH |
| Timor-Leste (East Timor) | TL |
| Togo | TG |
| Tonga | TO |
| Trinidad and Tobago | TT |
| Tunisia | TN |
| Turkey | TR |
| Turkmenistan | TM |
| Tuvalu | TV |
| U.S. Virgin Islands | VI |
| Uganda | UG |
| Ukraine | UA |
| United Arab Emirates | AE |
| United Kingdom | GB |
| United States | US |
| Uruguay | UY |
| Uzbekistan | UZ |
| Vanuatu | VU |
| Vatican City | VA |
| Venezuela | VE |
| Vietnam | VN |
| Yemen | YE |
| Zambia | ZM |
| Zimbabwe | ZW |
| Kosovo | XK |
| South Sudan | SS |
| China | CN |
| Palestine | PS |
| Curacao | CW |
| Dominican Republic | DO |
| Dominican Republic | DO |
| Gibraltar | GI |
| New Caledonia | NC |
| Cook Islands | CK |
| Reunion | RE |
| Guernsey | GG |
| Guadeloupe | GP |
| Martinique | MQ |
| French Polynesia | PF |
| Faroe Islands | FO |
| Greenland | GL |
| Jersey | JE |
| Aruba | AW |
| Puerto Rico | PR |
| Isle of Man | IM |
| Guam | GU |
| Sint Maarten | SX |
| Turks and Caicos | TC |
| Åland Islands | AX |
| Caribbean Netherlands | BQ |
| British Indian Ocean Territory | IO |
| Christmas as Island | CX |
| Cocos (Keeling) Islands | CC |
| Falkland Islands (Islas Malvinas) | FK |
| Mayotte | YT |
| Niue | NU |
| Norfolk Island | NF |
| Northern Mariana Islands | MP |
| Pitcairn Islands | PN |
| Saint Helena, Ascension and Tristan da Cunha | SH |
| Collectivity of Saint Martin | MF |
| Saint Pierre and Miquelon | PM |
| Tokelau | TK |
| Wallis and Futuna | WF |
| American Samoa | AS |

### Cancel withdrawal

You can cancel normal withdrawal requests, but you cannot cancel withdrawal requests on Lightning.

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/asset/cancel-withdrawal`

> Request Example

```
Copy to Clipboard
POST /api/v5/asset/cancel-withdrawal
body {
   "wdId":"1123456"
}
```

```
Copy to Clipboard
import okx.Funding as Funding

# API initialization
apikey = "YOUR_API_KEY"
secretkey = "YOUR_SECRET_KEY"
passphrase = "YOUR_PASSPHRASE"

flag = "0"  # Production trading: 0, Demo trading: 1

fundingAPI = Funding.FundingAPI(apikey, secretkey, passphrase, False, flag)

# Cancel withdrawal
result = fundingAPI.cancel_withdrawal(
    wdId="123456"
)
print(result)
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| wdId | String | Yes | Withdrawal ID |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "wdId": "1123456"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| wdId | String | Withdrawal ID |

If the code is equal to 0, it cannot be strictly considered that the withdrawal has been revoked. It only means that your request is accepted by the server. The actual result is subject to the status in the withdrawal history.

### Get withdrawal history

Retrieve the withdrawal records according to the currency, withdrawal status, and time range in reverse chronological order. The 100 most recent records are returned by default.

Websocket API is also available, refer to [Withdrawal info channel](https://www.okx.com/docs-v5/en/#funding-account-websocket-withdrawal-info-channel).

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/withdrawal-history`

> Request Example

```
Copy to Clipboard

GET /api/v5/asset/withdrawal-history

# Query withdrawal history from 2022-06-01 to 2022-07-01
GET /api/v5/asset/withdrawal-history?ccy=BTC&after=1654041600000&before=1656633600000
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| ccy | String | No | Currency, e.g. `BTC` |
| wdId | String | No | Withdrawal ID |
| clientId | String | No | Client-supplied ID<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| txId | String | No | Hash record of the deposit |
| type | String | No | Withdrawal type<br>`3`: Internal transfer<br>`4`: On-chain withdrawal |
| state | String | No | Status of withdrawal<br>Stage 1 : Pending withdrawal<br>`19`: insufficient balance in the hot wallet<br>`17`: Pending response from Travel Rule vendor<br>`10`: Waiting transfer<br>`0`: Waiting withdrawal<br>`4`/`5`/`6`/`8`/`9`/`12`: Waiting manual review<br>`7`: Approved<br>\> `0`, `17`, `19` can be cancelled, other statuses cannot be cancelled<br>Stage 2 : Withdrawal in progress (Applicable to on-chain withdrawals, internal transfers do not have this stage)<br>`1`: Broadcasting your transaction to chain<br>`15`: Pending transaction validation<br>`16`: Due to local laws and regulations, your withdrawal may take up to 24 hours to arrive<br>`-3`: Canceling <br>Final stage<br>`-2`: Canceled <br>`-1`: Failed<br>`2`: Success |
| after | String | No | Pagination of data to return records earlier than the requested ts, Unix timestamp format in milliseconds, e.g. `1654041600000` |
| before | String | No | Pagination of data to return records newer than the requested ts, Unix timestamp format in milliseconds, e.g. `1656633600000` |
| limit | String | No | Number of results per request. The maximum is `100`; The default is `100` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "note": "",
      "chain": "ETH-Ethereum",
      "fee": "0.007",
      "feeCcy": "ETH",
      "ccy": "ETH",
      "clientId": "",
      "toAddrType": "1",
      "amt": "0.029809",
      "txId": "0x35c******b360a174d",
      "from": "156****359",
      "areaCodeFrom": "86",
      "to": "0xa30d1fab********7CF18C7B6C579",
      "areaCodeTo": "",
      "state": "2",
      "ts": "1655251200000",
      "nonTradableAsset": false,
      "wdId": "15447421"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency |
| chain | String | Chain name, e.g. `USDT-ERC20`, `USDT-TRC20` |
| nonTradableAsset | Boolean | Whether it is a non-tradable asset or not<br>`true`: non-tradable asset, `false`: tradable asset |
| amt | String | Withdrawal amount |
| ts | String | Time the withdrawal request was submitted, Unix timestamp format in milliseconds, e.g. `1655251200000`. |
| from | String | Withdrawal account <br>It can be `email`/`phone`/`sub-account name` |
| areaCodeFrom | String | Area code for the phone number<br>If `from` is a phone number, this parameter returns the area code for the phone number |
| to | String | Receiving address |
| areaCodeTo | String | Area code for the phone number<br>If `to` is a phone number, this parameter returns the area code for the phone number |
| toAddrType | String | Address type<br>`1`: wallet address, email, phone, or login account name<br>`2`: UID |
| tag | String | Some currencies require a tag for withdrawals. This is not returned if not required. |
| pmtId | String | Some currencies require a payment ID for withdrawals. This is not returned if not required. |
| memo | String | Some currencies require this parameter for withdrawals. This is not returned if not required. |
| addrEx | Object | Withdrawal address attachment (This will not be returned if the currency does not require this) e.g. TONCOIN attached tag name is comment, the return will be {'comment':'123456'} |
| txId | String | Hash record of the withdrawal<br>This parameter will return "" for internal transfers. |
| fee | String | Withdrawal fee amount |
| feeCcy | String | Withdrawal fee currency, e.g. `USDT` |
| state | String | Status of withdrawal |
| wdId | String | Withdrawal ID |
| clientId | String | Client-supplied ID |
| note | String | Withdrawal note |

### Get deposit withdraw status

Retrieve deposit's and withdrawal's detailed status and estimated complete time.

#### Rate Limit: 1 request per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/deposit-withdraw-status`

> Request Example

```
Copy to Clipboard
# For deposit
GET /api/v5/asset/deposit-withdraw-status?txId=xxxxxx&to=1672734730284&ccy=USDT&chain=USDT-ERC20

# For withdrawal
GET /api/v5/asset/deposit-withdraw-status?wdId=200045249
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| wdId | String | Conditional | Withdrawal ID, use to retrieve withdrawal status <br>Required to input one and only one of `wdId` and `txId` |
| txId | String | Conditional | Hash record of the deposit, use to retrieve deposit status <br>Required to input one and only one of `wdId` and `txId` |
| ccy | String | Conditional | Currency type, e.g. `USDT`<br>Required when retrieving deposit status with `txId` |
| to | String | Conditional | To address, the destination address in deposit <br>Required when retrieving deposit status with `txId` |
| chain | String | Conditional | Currency chain information, e.g. USDT-ERC20 <br>Required when retrieving deposit status with `txId` |

> Response Example

```
Copy to Clipboard
{
    "code":"0",
    "data":[
        {
            "wdId": "200045249",
            "txId": "16f3638329xxxxxx42d988f97",
            "state": "Pending withdrawal: Wallet is under maintenance, please wait.",
            "estCompleteTime": "01/09/2023, 8:10:48 PM"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| estCompleteTime | String | Estimated complete time<br>The timezone is `UTC+8`. The format is MM/dd/yyyy, h:mm:ss AM/PM <br>estCompleteTime is only an approximate estimated time, for reference only. |
| state | String | The detailed stage and status of the deposit/withdrawal <br>The message in front of the colon is the stage; the message after the colon is the ongoing status. |
| txId | String | Hash record on-chain<br>For withdrawal, if the `txId` has already been generated, it will return the value, otherwise, it will return "". |
| wdId | String | Withdrawal ID<br>When retrieving deposit status, wdId returns blank "". |

Stage References

Deposit

Stage 1: On-chain transaction detection

Stage 2: Push deposit data to associated account

Stage 3: Receiving account credit

Final stage: Deposit complete

Withdrawal

Stage 1: Pending withdrawal

Stage 2: Withdrawal in progress

Final stage: Withdrawal complete / cancellation complete

### Get exchange list (public)

Authentication is not required for this public endpoint.

#### Rate Limit: 6 requests per second

#### Rate limit rule: IP

#### HTTP Request

`GET /api/v5/asset/exchange-list`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/exchange-list
```

```
Copy to Clipboard
```

#### Request Parameters

None

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "exchId": "did:ethr:0xfeb4f99829a9acdf52979abee87e83addf22a7e1",
        "exchName": "1xbet"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| exchName | String | Exchange name, e.g. `1xbet` |
| exchId | String | Exchange ID, e.g. `did:ethr:0xfeb4f99829a9acdf52979abee87e83addf22a7e1` |

### Apply for monthly statement (last year)

Apply for monthly statement in the past year.

#### Rate Limit: 20 requests per month

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`POST /api/v5/asset/monthly-statement`

> Request Example

```
Copy to Clipboard
POST /api/v5/asset/monthly-statement
body
{
    "month":"Jan"
}
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| month | String | No | Month,last month by default. Valid value is `Jan`, `Feb`, `Mar`, `Apr`,`May`, `Jun`, `Jul`,`Aug`, `Sep`,`Oct`,`Nov`,`Dec` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ts": "1646892328000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| ts | String | Download link generation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get monthly statement (last year)

Retrieve monthly statement in the past year.

#### Rate Limit: 10 requests per 2 seconds

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/monthly-statement`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/monthly-statement?month=Jan
```

#### Request Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| month | String | Yes | Month, valid value is `Jan`, `Feb`, `Mar`, `Apr`,`May`, `Jun`, `Jul`,`Aug`, `Sep`,`Oct`,`Nov`,`Dec` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "fileHref": "http://xxx",
            "state": "finished",
            "ts": 1646892328000
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| **Parameter** | **Type** | **Description** |
| --- | --- | --- |
| fileHref | String | Download file link |
| ts | Int | Download link generation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| state | String | Download link status <br>"finished" "ongoing" |

### Get convert currencies

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/convert/currencies`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/convert/currencies
```

#### Response parameters

none

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "min": "",  // Deprecated
            "max": "",  // Deprecated
            "ccy": "BTC"
        },
        {
            "min": "",
            "max": "",
            "ccy": "ETH"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Currency, e.g. BTC |
| min | String | Minimum amount to convert ( Deprecated ) |
| max | String | Maximum amount to convert ( Deprecated ) |

### Get convert currency pair

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/asset/convert/currency-pair`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/convert/currency-pair?fromCcy=USDT&toCcy=BTC
```

#### Response parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| fromCcy | String | Yes | Currency to convert from, e.g. `USDT` |
| toCcy | String | Yes | Currency to convert to, e.g. `BTC` |
| convertMode | String | No | `0`: standard convert (default) <br>`1`: large order convert for VIP |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "baseCcy": "BTC",
            "baseCcyMax": "0.5",
            "baseCcyMin": "0.0001",
            "instId": "BTC-USDT",
            "quoteCcy": "USDT",
            "quoteCcyMax": "10000",
            "quoteCcyMin": "1"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| instId | String | Currency pair, e.g. `BTC-USDT` |
| baseCcy | String | Base currency, e.g. `BTC` in `BTC-USDT` |
| baseCcyMax | String | Maximum amount of base currency |
| baseCcyMin | String | Minimum amount of base currency |
| quoteCcy | String | Quote currency, e.g. `USDT` in `BTC-USDT` |
| quoteCcyMax | String | Maximum amount of quote currency |
| quoteCcyMin | String | Minimum amount of quote currency |

### Estimate quote

#### Rate Limit: 10 requests per second

#### Rate limit rule: User ID

#### Rate Limit: 1 request per 5 seconds

#### Rate limit rule: Instrument ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/asset/convert/estimate-quote`

> Request Example

```
Copy to Clipboard
POST /api/v5/asset/convert/estimate-quote
body
{
    "baseCcy": "ETH",
    "quoteCcy": "USDT",
    "side": "buy",
    "rfqSz": "30",
    "rfqSzCcy": "USDT"
}
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| baseCcy | String | Yes | Base currency, e.g. `BTC` in `BTC-USDT` |
| quoteCcy | String | Yes | Quote currency, e.g. `USDT` in `BTC-USDT` |
| side | String | Yes | Trade side based on `baseCcy`<br>`buy``sell` |
| rfqSz | String | Yes | RFQ amount |
| rfqSzCcy | String | Yes | RFQ currency |
| clQReqId | String | No | Client Order ID as assigned by the client<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag<br>Applicable to broker user |
| convertMode | String | No | `0`: standard convert (default) <br>`1`: large order convert for VIP |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "baseCcy": "ETH",
            "baseSz": "0.01023052",
            "clQReqId": "",
            "cnvtPx": "2932.40104429",
            "origRfqSz": "30",
            "quoteCcy": "USDT",
            "quoteId": "quoterETH-USDT16461885104612381",
            "quoteSz": "30",
            "quoteTime": "1646188510461",
            "rfqSz": "30",
            "rfqSzCcy": "USDT",
            "side": "buy",
            "ttlMs": "10000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| quoteTime | String | Quotation generation time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| ttlMs | String | Validity period of quotation in milliseconds |
| clQReqId | String | Client Order ID as assigned by the client |
| quoteId | String | Quote ID |
| baseCcy | String | Base currency, e.g. `BTC` in `BTC-USDT` |
| quoteCcy | String | Quote currency, e.g. `USDT` in `BTC-USDT` |
| side | String | Trade side based on `baseCcy` |
| origRfqSz | String | Original RFQ amount |
| rfqSz | String | Real RFQ amount |
| rfqSzCcy | String | RFQ currency |
| cnvtPx | String | Convert price based on quote currency |
| baseSz | String | Convert amount of base currency |
| quoteSz | String | Convert amount of quote currency |

### Convert trade

You should make [estimate quote](https://www.okx.com/docs-v5/en/#funding-account-rest-api-estimate-quote) before convert trade.

Only assets in the trading account supported convert.

#### Rate Limit: 10 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

For the same side (buy/sell), there's a trading limit of 1 request per 5 seconds.

#### HTTP Request

`POST /api/v5/asset/convert/trade`

> Request Example

```
Copy to Clipboard
POST /api/v5/asset/convert/trade
body
{
    "baseCcy": "ETH",
    "quoteCcy": "USDT",
    "side": "buy",
    "sz": "30",
    "szCcy": "USDT",
    "quoteId": "quoterETH-USDT16461885104612381"
}
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| quoteId | String | Yes | Quote ID |
| baseCcy | String | Yes | Base currency, e.g. `BTC` in `BTC-USDT` |
| quoteCcy | String | Yes | Quote currency, e.g. `USDT` in `BTC-USDT` |
| side | String | Yes | Trade side based on `baseCcy`<br>`buy``sell` |
| sz | String | Yes | Quote amount<br>The quote amount should no more then RFQ amount |
| szCcy | String | Yes | Quote currency |
| clTReqId | String | No | Client Order ID as assigned by the client<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| tag | String | No | Order tag<br>Applicable to broker user |
| convertMode | String | No | `0`: standard convert (default) <br>`1`: large order convert for VIP |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "baseCcy": "ETH",
            "clTReqId": "",
            "fillBaseSz": "0.01023052",
            "fillPx": "2932.40104429",
            "fillQuoteSz": "30",
            "instId": "ETH-USDT",
            "quoteCcy": "USDT",
            "quoteId": "quoterETH-USDT16461885104612381",
            "side": "buy",
            "state": "fullyFilled",
            "tradeId": "trader16461885203381437",
            "ts": "1646188520338"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| tradeId | String | Trade ID |
| quoteId | String | Quote ID |
| clTReqId | String | Client Order ID as assigned by the client |
| state | String | Trade state<br>`fullyFilled`: success<br>`rejected`: failed |
| instId | String | Currency pair, e.g. `BTC-USDT` |
| baseCcy | String | Base currency, e.g. `BTC` in `BTC-USDT` |
| quoteCcy | String | Quote currency, e.g. `USDT` in `BTC-USDT` |
| side | String | Trade side based on `baseCcy`<br>`buy`<br>`sell` |
| fillPx | String | Filled price based on quote currency |
| fillBaseSz | String | Filled amount for base currency |
| fillQuoteSz | String | Filled amount for quote currency |
| ts | String | Convert trade time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get convert history

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### HTTP Request

`GET /api/v5/asset/convert/history`

> Request Example

```
Copy to Clipboard
GET /api/v5/asset/convert/history
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| clTReqId | String | No | Client Order ID as assigned by the client<br>A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| after | String | No | Pagination of data to return records earlier than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| before | String | No | Pagination of data to return records newer than the requested `ts`, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is `100`. The default is `100`. |
| tag | String | No | Order tag<br>Applicable to broker user<br>If the convert trading used `tag`, this parameter is also required. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "clTReqId": "",
            "instId": "ETH-USDT",
            "side": "buy",
            "fillPx": "2932.401044",
            "baseCcy": "ETH",
            "quoteCcy": "USDT",
            "fillBaseSz": "0.01023052",
            "state": "fullyFilled",
            "tradeId": "trader16461885203381437",
            "fillQuoteSz": "30",
            "ts": "1646188520000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| tradeId | String | Trade ID |
| clTReqId | String | Client Order ID as assigned by the client |
| state | String | Trade state<br>`fullyFilled` : success <br>`rejected` : failed |
| instId | String | Currency pair, e.g. `BTC-USDT` |
| baseCcy | String | Base currency, e.g. `BTC` in `BTC-USDT` |
| quoteCcy | String | Quote currency, e.g. `USDT` in `BTC-USDT` |
| side | String | Trade side based on `baseCcy`<br>`buy``sell` |
| fillPx | String | Filled price based on quote currency |
| fillBaseSz | String | Filled amount for base currency |
| fillQuoteSz | String | Filled amount for quote currency |
| ts | String | Convert trade time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get deposit payment methods

To display all the available fiat deposit payment methods

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/deposit-payment-methods`

> Request Example

```
Copy to Clipboard
GET /api/v5/fiat/deposit-payment-methods?ccy=TRY
body
{
  "ccy" : "TRY",
}
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | Yes | Fiat currency, ISO-4217 3 digit currency code, e.g. `TRY` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "ccy": "TRY",
      "paymentMethod": "TR_BANKS",
      "feeRate": "0",
      "minFee": "0",
      "limits": {
        "dailyLimit": "2147483647",
        "dailyLimitRemaining": "2147483647",
        "weeklyLimit": "2147483647",
        "weeklyLimitRemaining": "2147483647",
        "monthlyLimit": "",
        "monthlyLimitRemaining": "",
        "maxAmt": "1000000",
        "minAmt": "1",
        "lifetimeLimit": "2147483647"
      },
      "accounts": [
          {
            "paymentAcctId": "1",
            "acctNum": "TR740001592093703829602611",
            "recipientName": "John Doe",
            "bankName": "VakıfBank",
            "bankCode": "TVBATR2AXXX",
            "state": "active"
          },
          {
            "paymentAcctId": "2",
            "acctNum": "TR740001592093703829602622",
            "recipientName": "John Doe",
            "bankName": "FBHLTRISXXX",
            "bankCode": "",
            "state": "active"
          }
      ]
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Fiat currency |
| paymentMethod | String | The payment method associated with the currency<br>`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| feeRate | String | The fee rate for each deposit, expressed as a percentage<br>e.g. `0.02` represents 2 percent fee for each transaction. |
| minFee | String | The minimum fee for each deposit |
| limits | Object | An object containing limits for various transaction intervals |
| \> dailyLimit | String | The daily transaction limit |
| \> dailyLimitRemaining | String | The remaining daily transaction limit |
| \> weeklyLimit | String | The weekly transaction limit |
| \> weeklyLimitRemaining | String | The remaining weekly transaction limit |
| \> monthlyLimit | String | The monthly transaction limit |
| \> monthlyLimitRemaining | String | The remaining monthly transaction limit |
| \> maxAmt | String | The maximum amount allowed per transaction |
| \> minAmt | String | The minimum amount allowed per transaction |
| \> lifetimeLimit | String | The lifetime transaction limit. Return the configured value, "" if not configured |
| accounts | Array of Object | An array containing information about payment accounts associated with the currency and method. |
| \> paymentAcctId | String | The account ID for withdrawal |
| \> acctNum | String | The account number, which can be an IBAN or other bank account number. |
| \> recipientName | String | The name of the recipient |
| \> bankName | String | The name of the bank associated with the account |
| \> bankCode | String | The SWIFT code / BIC / bank code associated with the account |
| \> state | String | The state of the account<br>`active` |

### Get withdrawal payment methods

To display all the available fiat withdrawal payment methods

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/withdrawal-payment-methods`

> Request Example

```
Copy to Clipboard
 GET /api/v5/fiat/withdrawal-payment-methods?ccy=TRY
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | Yes | Fiat currency, ISO-4217 3 digit currency code. e.g. `TRY` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
      "ccy": "TRY",
      "paymentMethod": "TR_BANKS",
      "feeRate": "0.02",
      "minFee": "1",
      "limits": {
        "dailyLimit": "",
        "dailyLimitRemaining": "",
        "weeklyLimit": "",
        "weeklyLimitRemaining": "",
        "monthlyLimit": "",
        "monthlyLimitRemaining": "",
        "maxAmt": "",
        "minAmt": "",
        "lifetimeLimit": ""
      },
      "accounts": [
          {
            "paymentAcctId": "1",
            "acctNum": "TR740001592093703829602668",
            "recipientName": "John Doe",
            "bankName": "VakıfBank",
            "bankCode": "TVBATR2AXXX",
            "state": "active"
          },
          {
            "paymentAcctId": "2",
            "acctNum": "TR740001592093703829603024",
            "recipientName": "John Doe",
            "bankName": "Şekerbank",
            "bankCode": "SEKETR2AXXX",
            "state": "active"
          }
      ]
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ccy | String | Fiat currency |
| paymentMethod | String | The payment method associated with the currency<br>`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| feeRate | String | The fee rate for each deposit, expressed as a percentage <br> e.g. `0.02` represents 2 percent fee for each transaction. |
| minFee | String | The minimum fee for each deposit |
| limits | Object | An object containing limits for various transaction intervals |
| \> dailyLimit | String | The daily transaction limit |
| \> dailyLimitRemaining | String | The remaining daily transaction limit |
| \> weeklyLimit | String | The weekly transaction limit |
| \> weeklyLimitRemaining | String | The remaining weekly transaction limit |
| \> monthlyLimit | String | The monthly transaction limit |
| \> monthlyLimitRemaining | String | The remaining monthly transaction limit |
| \> minAmt | String | The minimum amount allowed per transaction |
| \> maxAmt | String | The maximum amount allowed per transaction |
| \> lifetimeLimit | String | The lifetime transaction limit. Return the configured value, "" if not configured |
| accounts | Array of Object | An array containing information about payment accounts associated with the currency and method. |
| \> paymentAcctId | String | The account ID for withdrawal |
| \> acctNum | String | The account number, which can be an IBAN or other bank account number. |
| \> recipientName | String | The name of the recipient |
| \> bankName | String | The name of the bank associated with the account |
| \> bankCode | String | The SWIFT code / BIC / bank code associated with the account |
| \> state | String | The state of the account<br>`active` |

### Create withdrawal order

Initiate a fiat withdrawal request (Authenticated endpoint, Only for API keys with "Withdrawal" access)

Only supported withdrawal of assets from funding account.

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Withdraw

#### HTTP Request

`POST /api/v5/fiat/create-withdrawal`

> Request Example

```
Copy to Clipboard
 POST /api/v5/fiat/create-withdrawal
 body
 {
    "paymentAcctId": "412323",
    "ccy": "TRY",
    "amt": "10000",
    "paymentMethod": "TR_BANKS",
    "clientId": "194a6975e98246538faeb0fab0d502df"
 }
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Type** | **Required** | **Description** |
| --- | --- | --- | --- |
| paymentAcctId | String | Yes | Payment account id to withdraw to, retrieved from get withdrawal payment methods API |
| ccy | String | Yes | Currency for withdrawal, must match currency allowed for paymentMethod |
| amt | String | Yes | Requested withdrawal amount before fees. Has to be less than or equal to 2 decimal points double |
| paymentMethod | String | Yes | Payment method to use for withdrawal<br>`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| clientId | String | Yes | Client-supplied ID, A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters <br> e.g. `194a6975e98246538faeb0fab0d502df` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "124041201450544699",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "fee": "0",
        "amt": "100",
        "ccy": "TRY",
        "state": "completed",
        "clientId": "194a6975e98246538faeb0fab0d502df"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | The unique order Id |
| clientId | String | The client ID associated with the transaction |
| amt | String | The requested amount for the transaction |
| ccy | String | The currency of the transaction |
| fee | String | The transaction fee |
| paymentAcctId | String | The Id of the payment account used |
| paymentMethod | String | Payment Method<br>`TR_BANKS`<br>`PIX`<br>`SEPA` |
| state | String | The State of the transaction<br>`processing`<br>`completed` |
| cTime | String | The creation time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | The update time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Cancel withdrawal order

Cancel a pending fiat withdrawal order, currently only applicable to TRY

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/fiat/cancel-withdrawal`

> Request Example

```
Copy to Clipboard
 POST /api/v5/fiat/cancel-withdrawal
 body
 {
    "ordId": "124041201450544699"
 }
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Payment Order Id |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "ordId": "124041201450544699",
        "state": "canceled"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Payment Order ID |
| state | String | The state of the transaction, e.g.`canceled` |

### Get withdrawal order history

Get fiat withdrawal order history

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/withdrawal-order-history`

> Request Example

```
Copy to Clipboard
 GET /api/v5/fiat/withdrawal-order-history
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | Fiat currency, ISO-4217 3 digit currency code, e.g. `TRY` |
| paymentMethod | String | No | Payment Method<br>`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| state | String | No | State of the order<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue`<br>`processing` |
| after | String | No | Filter with a begin timestamp. Unix timestamp format in milliseconds (inclusive), e.g. `1597026383085` |
| before | String | No | Filter with an end timestamp. Unix timestamp format in milliseconds (inclusive), e.g. `1597026383085` |
| limit | String | No | Number of results per request. Maximum and default is `100` |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "124041201450544699",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "10000",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": "194a6975e98246538faeb0fab0d502df"
    },
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "124041201450544690",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "5000",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": "164a6975e48946538faeb0fab0d414fg"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Unique Order Id |
| clientId | String | Client Id of the transaction |
| amt | String | Final amount of the transaction |
| ccy | String | Currency of the transaction |
| fee | String | Transaction fee |
| paymentAcctId | String | ID of the payment account used |
| paymentMethod | String | Payment method type |
| state | String | State of the transaction<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue`<br>`processing` |
| cTime | String | Creation time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Update time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get withdrawal order detail

Get fiat withdraw order detail

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/withdrawal`

> Request Example

```
Copy to Clipboard
 GET /api/v5/fiat/withdrawal?ordId=024041201450544699
 body
 {
    "ordId": "024041201450544699"
 }
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "024041201450544699",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "100",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": "194a6975e98246538faeb0fab0d502df"
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| clientId | String | The original request ID associated with the transaction |
| ccy | String | The currency of the transaction |
| amt | String | Amount of the transaction |
| fee | String | The transaction fee |
| paymentAcctId | String | The ID of the payment account used |
| paymentMethod | String | Payment method, e.g. `TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| state | String | The state of the transaction<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue`<br>`processing` |
| cTime | String | The creation time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | The update time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get deposit order history

Get fiat deposit order history

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/deposit-order-history`

> Request Example

```
Copy to Clipboard
 GET /api/v5/fiat/deposit-order-history
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ccy | String | No | ISO-4217 3 digit currency code |
| paymentMethod | String | No | Payment Method<br>`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| state | String | No | State of the order<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue`<br>`processing` |
| after | String | No | Filter with a begin timestamp. Unix timestamp format in milliseconds (inclusive), e.g. `1597026383085` |
| before | String | No | Filter with an end timestamp. Unix timestamp format in milliseconds (inclusive), e.g. `1597026383085` |
| limit | String | No | Number of results per request. Maximum and default is 100 |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "024041201450544699",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "10000",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": ""
    },
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "024041201450544690",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "50000",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": ""
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Unique Order ID |
| clientId | String | Client Id of the transaction |
| ccy | String | Currency of the transaction |
| amt | String | Final amount of the transaction |
| fee | String | Transaction fee |
| paymentAcctId | String | ID of the payment account used |
| paymentMethod | String | Payment Method, e.g. `TR_BANKS` |
| state | String | State of the transaction<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue` |
| cTime | String | Creation time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Update time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get deposit order detail

Get fiat deposit order detail

#### Rate Limit: 3 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/deposit`

> Request Example

```
Copy to Clipboard
GET /api/v5/fiat/deposit?ordId=024041201450544699
body
{
    "ordId": "024041201450544699",
}
```

```
Copy to Clipboard
```

#### Request Parameters

| **Parameters** | **Types** | **Required** | **Description** |
| --- | --- | --- | --- |
| ordId | String | Yes | Order ID |

> Response Example

```
Copy to Clipboard
{
  "code": "0",
  "msg": "",
  "data": [
    {
        "cTime": "1707429385000",
        "uTime": "1707429385000",
        "ordId": "024041201450544699",
        "paymentMethod": "TR_BANKS",
        "paymentAcctId": "20",
        "amt": "100",
        "fee": "0",
        "ccy": "TRY",
        "state": "completed",
        "clientId": ""
    }
  ]
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| clientId | String | The original request ID associated with the transaction. If it's a deposit, it's most likely an empty string (""). |
| amt | String | Amount of the transaction |
| ccy | String | The currency of the transaction |
| fee | String | The transaction fee |
| paymentAcctId | String | The ID of the payment account used |
| paymentMethod | String | Payment method, e.g.`TR_BANKS`<br>`PIX`<br>`SEPA`<br>`XPULSE`<br>`NPP`<br>`US_WIRE` |
| state | String | The state of the transaction<br>`completed`<br>`failed`<br>`pending`<br>`canceled`<br>`inqueue`<br>`processing` |
| cTime | String | The creation time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | The update time of the transaction, Unix timestamp format in milliseconds, e.g. `1597026383085` |

### Get buy/sell currencies

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/buy-sell/currencies`

> Request Example

```
Copy to Clipboard
GET /api/v5/fiat/buy-sell/currencies
```

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
           "fiatCcyList":[
                {
                    "ccy": "USD"
                },
                {
                    "ccy": "EUR"
                },
                ...
            ],
            "cryptoCcyList":[
                {
                    "ccy": "BTC"
                },
                ...
            ],
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| fiatCcyList | Array of objects | Fiat currency list |
| >ccy | String | Currency, e.g. `BTC` |
| cryptoCcyList | Array of objects | Crypto currency list |
| >ccy | String | Currency, e.g. `USD` |

This feature is only available to Bahamas institutional users at the moment.

### Get buy/sell currency pair

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/buy-sell/currency-pair`

> Request Example

```
Copy to Clipboard
GET /api/v5/fiat/buy-sell/currency-pair?fromCcy=USD&toCcy=BTC
```

#### Request Parameters

| Parameters | Types | Required | Description |
| --- | --- | --- | --- |
| fromCcy | String | Yes | Currency to sell, e.g. `USD` |
| toCcy | String | Yes | Currency to buy, e.g. `BTC` |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "side": "buy",
            "fromCcy": "USD",
            "toCcy": "BTC",
            "singleTradeMax": "1",
            "singleTradeMin": "0.01",
            "fixedPxRemainingDailyQuota": "",
            "fixedPxDailyLimit": "",
            "paymentMethods":["balance"]
        }
    ],
    "msg": ""
}

{
    "code": "0",
    "data": [
        {
            "side": "sell",
            "fromCcy": "BTC",
            "toCcy": "USD",
            "singleTradeMax": "1",
            "singleTradeMin": "0.01",
            "fixedPxRemainingDailyQuota": "",
            "fixedPxDailyLimit": "",
            "paymentMethods":["balance"]
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| side | String | Side<br>`buy`: Fiat to crypto<br>`sell`: Crypto to fiat<br>May support both sides in the future, separated with a comma, e.g. `buy,sell`. |
| fromCcy | String | Currency to sell, e.g. `USD` |
| toCcy | String | Currency to buy, e.g. `BTC` |
| singleTradeMax | String | The maximum amount of currency for a single trade, unit in `fromCcy` |
| singleTradeMin | String | The minimum amount of currency for a single trade, unit in `fromCcy` |
| fixedPxDailyLimit | String | Fixed price daily limit<br>Applicable to Fiat to Fiat trade, else return ''.<br>If `side` = `buy`, unit in `fromCcy`<br>If `side` = `sell`, unit in `toCcy` |
| fixedPxRemainingDailyQuota | String | Fixed price remaining daily quota<br>Applicable to Fiat to Fiat trade, else return ''.<br>If `side` = `buy`, unit in `fromCcy`<br>If `side` = `sell`, unit in `toCcy` |
| paymentMethods | Array of strings | Supported payment methods<br>`balance`<br>e.g. \["balance"\] |

This feature is only available to Bahamas institutional users at the moment.

### Get buy/sell quote

#### Rate Limit: 10 requests per second

#### Rate limit rule: User ID

#### Rate Limit: 1 request per 5 seconds

#### Rate limit rule: Instrument ID

#### Permission: Read

#### HTTP Request

`POST /api/v5/fiat/buy-sell/quote`

> Request Example

```
Copy to Clipboard
# Sell USD to buy 0.1 BTC
POST /api/v5/fiat/buy-sell/quote
body
{
    "side":"buy",
    "fromCcy": "USD",
    "toCcy": "BTC",
    "rfqAmt": "0.1",
    "rfqCcy": "BTC"
}

# Sell 30 USD to buy BTC
POST /api/v5/fiat/buy-sell/quote
body
{
    "side":"buy",
    "fromCcy": "USD",
    "toCcy": "BTC",
    "rfqAmt": "30",
    "rfqCcy": "USD"
}

# Sell BTC to buy 30 USD
POST /api/v5/fiat/buy-sell/quote
body
{
    "side":"sell",
    "fromCcy": "BTC",
    "toCcy": "USD",
    "rfqAmt": "30",
    "rfqCcy": "USD"
}

# Sell 0.1 BTC to buy USD
POST /api/v5/fiat/buy-sell/quote
body
{
    "side":"sell",
    "fromCcy": "BTC",
    "toCcy": "USD",
    "rfqAmt": "0.1",
    "rfqCcy": "BTC"
}
```

#### Request Parameters

| Parameters | Types | Required | Description |
| --- | --- | --- | --- |
| side | String | Yes | Side <br>`buy`: Buy Crypto / Fiat with Fiat <br>`sell`: Sell Crypto to Crypto / Fiat |
| fromCcy | String | Yes | Currency to sell |
| toCcy | String | Yes | Currency to buy |
| rfqAmt | String | Yes | RFQ amount |
| rfqCcy | String | Yes | RFQ currency |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "quoteId": "quoterBTC-USD16461885104612381",
            "fromCcy": "USD",
            "toCcy": "BTC",
            "rfqAmt": "30",
            "rfqCcy": "USD",
            "quotePx": "2932.40104429",
            "quoteCcy": "USD",
            "quoteFromAmt": "30",
            "quoteToAmt": "30",
            "quoteTime": "1646188510461",
            "ttlMs": "10000"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| quoteId | String | Quote ID |
| side | String | Side <br>`buy`: Buy Crypto / Fiat with Fiat <br>`sell`: Sell Crypto to Crypto / Fiat |
| fromCcy | String | Currency to sell, e.g. `USD` |
| toCcy | String | Currency to buy, e.g. `BTC` |
| rfqAmt | String | RFQ amount |
| rfqCcy | String | RFQ currency |
| quotePx | String | Quote price |
| quoteCcy | String | Quote price unit <br> e.g. `USD` |
| quoteFromAmt | String | Quote amount, unit in `fromCcy` |
| quoteToAmt | String | Quote amount, unit in `toCcy` |
| quoteTime | String | Quotation generation time, Unix timestamp format in milliseconds, e.g. 1597026383085 |
| ttlMs | String | The validity period of quotation in milliseconds <br> e.g. `10000` represents the quotation only valid for 10 seconds |

This feature is only available to Bahamas institutional users at the moment.

### Buy/sell trade

#### Rate Limit: 1 request per 5 seconds

#### Rate limit rule: User ID

#### Permission: Trade

#### HTTP Request

`POST /api/v5/fiat/buy-sell/trade`

> Request Example

```
Copy to Clipboard
# Sell 30 USD to buy BTC
POST /api/v5/fiat/buy-sell/trade
body
{
    "clOrdId":"123456",
    "side":"sell",
    "fromCcy": "USD",
    "toCcy": "BTC",
    "rfqAmt": "30",
    "rfqCcy": "USD",
    "paymentMethod":"balance",
    "quoteId": "quoterETH-USDT16461885104612381"
}
```

#### Request Parameters

| Parameters | Types | Required | Description |
| --- | --- | --- | --- |
| quoteId | String | Yes | Quote ID<br>Get from Buy/Sell quote API |
| side | String | Yes | Side <br>`buy`: Buy Crypto / Fiat with Fiat <br>`sell`: Sell Crypto to Crypto / Fiat <br>Should be the same as the Quote request |
| fromCcy | String | Yes | Currency to sell <br>Should be the same as the Quote request |
| toCcy | String | Yes | Currency to buy <br>Should be the same as the Quote request |
| rfqAmt | String | Yes | RFQ amount <br>Should be the same as the Quote request |
| rfqCcy | String | Yes | RFQ currency <br>Should be the same as the Quote request |
| paymentMethod | String | Yes | paymentMethod <br>`balance` |
| clOrdId | String | Yes | Client Order ID as assigned by the client <br> A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ordId": "1234",
            "clOrdId": "",
            "quoteId": "quoterBTC-USD16461885104612381",
            "side":"buy",
            "fromCcy": "USD",
            "toCcy": "BTC",
            "rfqAmt": "30",
            "rfqCcy": "USD",
            "fillPx": "2932.40104429",
            "fillQuoteCcy": "USD",
            "fillFromAmt": "30",
            "fillToAmt": "0.01",
            "cTime": "1646188510461"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| quoteId | String | Quote ID |
| state | String | Trade state <br>`processing`<br>`completed`<br>`failed` |
| side | String | Side <br>`buy`: Buy Crypto / Fiat with Fiat <br>`sell`: Sell Crypto to Crypto / Fiat |
| fromCcy | String | Currency to sell |
| toCcy | String | Currency to buy |
| rfqAmt | String | RFQ amount |
| rfqCcy | String | RFQ currency |
| fillPx | String | Filled price based on quote currency |
| fillQuoteCcy | String | Filled price quote currency <br> e.g. `USD` |
| fillFromAmt | String | Sold amount, unit in `fromCcy` |
| fillToAmt | String | Bought amount, unit in `toCcy` |
| cTime | String | Request time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

This feature is only available to Bahamas institutional users at the moment.

### Get buy/sell trade history

#### Rate Limit: 6 requests per second

#### Rate limit rule: User ID

#### Permission: Read

#### HTTP Request

`GET /api/v5/fiat/buy-sell/history`

> Request Example

```
Copy to Clipboard
GET /api/v5/fiat/buy-sell/history
```

#### Request Parameters

| Parameters | Types | Required | Description |
| --- | --- | --- | --- |
| ordId | String | No | Order ID |
| clOrdId | String | No | Client Order ID as assigned by the client <br> A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters. |
| state | String | No | Trade state <br>`processing`<br>`completed`<br>`failed` |
| begin | String | No | Filter with a begin timestamp. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| end | String | No | Filter with an end timestamp. Unix timestamp format in milliseconds, e.g. `1597026383085` |
| limit | String | No | Number of results per request. The maximum is 100. The default is 100. |

> Response Example

```
Copy to Clipboard
{
    "code": "0",
    "data": [
        {
            "ordId": "1234",
            "clOrdId": "",
            "quoteId": "quoterBTC-USD16461885104612381",
            "state":"completed",
            "side":"buy",
            "fromCcy": "USD",
            "toCcy": "BTC",
            "rfqAmt": "30",
            "rfqCcy": "USD",
            "fillPx": "2932.40104429",
            "fillQuoteCcy": "USD",
            "fillFromAmt": "30",
            "fillToAmt": "0.01",
            "cTime": "1646188510461",
            "uTime": "1646188510461"
        }
    ],
    "msg": ""
}
```

#### Response Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| ordId | String | Order ID |
| clOrdId | String | Client Order ID as assigned by the client |
| quoteId | String | Quote ID |
| state | String | Trade state <br>`processing`<br>`completed`<br>`failed` |
| fromCcy | String | Currency to sell |
| toCcy | String | Currency to buy |
| rfqAmt | String | RFQ amount |
| rfqCcy | String | RFQ currency |
| fillPx | String | Filled price based on quote currency |
| fillQuoteCcy | String | Filled price quote currency <br> e.g. `USD` |
| fillFromAmt | String | Filled amount unit in fromCcy |
| fillToAmt | String | Filled amount unit in toCcy |
| cTime | String | Request time, Unix timestamp format in milliseconds, e.g. `1597026383085` |
| uTime | String | Updated time, Unix timestamp format in milliseconds, e.g. `1597026383085` |

This feature is only available to Bahamas institutional users at the moment.

## WebSocket

### Deposit info channel

A push notification is triggered when a deposit is initiated or the deposit status changes.

Supports subscriptions for accounts

- If it is a master account subscription, you can receive the push of the deposit info of both the master account and the sub-account.
- If it is a sub-account subscription, only the push of sub-account deposit info you can receive.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [
        {
            "channel": "deposit-info"
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
            "channel": "deposit-info"
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
| \> channel | String | Yes | Channel name<br>`deposit-info` |
| \> ccy | String | No | Currency, e.g. `BTC` |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "deposit-info"
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
    "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"deposit-info\""}]}",
    "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Operation<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name<br>`deposit-info` |
| \> ccy | String | No | Currency, e.g. `BTC` |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
    "arg": {
        "channel": "deposit-info",
        "uid": "289320****60975104"
    },
    "data": [{
        "actualDepBlkConfirm": "0",
        "amt": "1",
        "areaCodeFrom": "",
        "ccy": "USDT",
        "chain": "USDT-TRC20",
        "depId": "88165462",
        "from": "",
        "fromWdId": "",
        "pTime": "1674103661147",
        "state": "0",
        "subAcct": "test",
        "to": "TEhFAqpuHa3LY*****8ByNoGnrmexeGMw",
        "ts": "1674103661123",
        "txId": "bc5376817*****************dbb0d729f6b",
        "uid": "289320****60975104"
    }]
}
```

#### Push data parameters

| **Parameters** | **Types** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name<br>`deposit-info` |
| \> uid | String | User Identifier |
| \> ccy | String | Currency, e.g. `BTC` |
| data | Array of objects | Subscribed data |
| \> uid | String | User Identifier of the message producer |
| \> subAcct | String | Sub-account name<br>If the message producer is master account, the parameter will return "" |
| \> pTime | String | Push time, the millisecond format of the Unix timestamp, e.g. `1597026383085` |
| \> ccy | String | Currency |
| \> chain | String | Chain name |
| \> amt | String | Deposit amount |
| \> from | String | Deposit account<br>Only the internal OKX account (masked mobile phone number or email address) is returned, not the address on the blockchain. |
| \> areaCodeFrom | String | If `from` is a phone number, this parameter return area code of the phone number |
| \> to | String | Deposit address |
| \> txId | String | Hash record of the deposit |
| \> ts | String | Time of deposit record is created, Unix timestamp format in milliseconds, e.g. `1655251200000` |
| \> state | String | Status of deposit<br>`0`: waiting for confirmation<br>`1`: deposit credited <br>`2`: deposit successful <br>`8`: pending due to temporary deposit suspension on this crypto currency<br>`11`: match the address blacklist<br>`12`: account or deposit is frozen<br>`13`: sub-account deposit interception<br>`14`: KYC limit |
| \> depId | String | Deposit ID |
| \> fromWdId | String | Internal transfer initiator's withdrawal ID<br>If the deposit comes from internal transfer, this field displays the withdrawal ID of the internal transfer initiator, and will return "" in other cases |
| \> actualDepBlkConfirm | String | The actual amount of blockchain confirmed in a single deposit |

### Withdrawal info channel

A push notification is triggered when a withdrawal is initiated or the withdrawal status changes.

Supports subscriptions for accounts

- If it is a master account subscription, you can receive the push of the withdrawal info of both the master account and the sub-account.
- If it is a sub-account subscription, only the push of sub-account withdrawal info you can receive.

#### URL Path

/ws/v5/business (required login)

> Request Example

```
Copy to Clipboard
{
    "id": "1512",
    "op": "subscribe",
    "args": [
        {
            "channel": "withdrawal-info"
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
            "channel": "withdrawal-info"
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
| \> channel | String | Yes | Channel name<br>`withdrawal-info` |
| \> ccy | String | No | Currency, e.g. `BTC` |

> Successful Response Example

```
Copy to Clipboard
{
    "id": "1512",
    "event": "subscribe",
    "arg": {
        "channel": "withdrawal-info"
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
    "msg": "Invalid request: {\"op\": \"subscribe\", \"argss\":[{ \"channel\" : \"withdrawal-info\"}]}",
    "connId": "a4d3ae55"
}
```

#### Response parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| id | String | No | Unique identifier of the message |
| event | String | Yes | Operation<br>`subscribe`<br>`unsubscribe`<br>`error` |
| arg | Object | No | Subscribed channel |
| \> channel | String | Yes | Channel name<br>`withdrawal-info` |
| \> ccy | String | No | Currency, e.g. `BTC` |
| code | String | No | Error code |
| msg | String | No | Error message |
| connId | String | Yes | WebSocket connection ID |

> Push Data Example

```
Copy to Clipboard
{
    "arg": {
        "channel": "withdrawal-info",
        "uid": "289320*****0975104"
    },
    "data": [{
        "addrEx": null,
        "amt": "2",
        "areaCodeFrom": "",
        "areaCodeTo": "",
        "ccy": "USDT",
        "chain": "USDT-TRC20",
        "clientId": "",
        "fee": "0.8",
        "feeCcy": "USDT",
        "from": "",
        "memo": "",
        "nonTradableAsset": false,
        "note": "",
        "pTime": "1674103268578",
        "pmtId": "",
        "state": "0",
        "subAcct": "test",
        "tag": "",
        "to": "TN8CKTQMnpWfT******8KipbJ24ErguhF",
        "toAddrType": "1",
        "ts": "1674103268472",
        "txId": "",
        "uid": "289333*****1101696",
        "wdId": "63754560"
    }]
}
```

#### Push data parameters

| **Parameters** | **Types** | **Description** |
| --- | --- | --- |
| arg | Object | Successfully subscribed channel |
| \> channel | String | Channel name |
| \> uid | String | User Identifier |
| \> ccy | String | Currency, e.g. `BTC` |
| data | Array of objects | Subscribed data |
| \> uid | String | User Identifier of the message producer |
| \> subAcct | String | Sub-account name<br>If the message producer is master account, the parameter will return "" |
| \> pTime | String | Push time, the millisecond format of the Unix timestamp, e.g. `1597026383085` |
| \> ccy | String | Currency |
| \> chain | String | Chain name, e.g. `USDT-ERC20`, `USDT-TRC20` |
| \> nonTradableAsset | String | Whether it is a non-tradable asset or not<br>`true`: non-tradable asset, `false`: tradable asset |
| \> amt | String | Withdrawal amount |
| \> ts | String | Time the withdrawal request was submitted, Unix timestamp format in milliseconds, e.g. `1655251200000`. |
| \> from | String | Withdrawal account<br>It can be `email`/`phone`/`sub-account name` |
| \> areaCodeFrom | String | Area code for the phone number<br>If `from` is a phone number, this parameter returns the area code for the phone number |
| \> to | String | Receiving address |
| \> areaCodeTo | String | Area code for the phone number<br>If `to` is a phone number, this parameter returns the area code for the phone number |
| \> toAddrType | String | Address type<br>`1`: wallet address, email, phone, or login account name<br>`2`: UID |
| \> tag | String | Some currencies require a tag for withdrawals |
| \> pmtId | String | Some currencies require a payment ID for withdrawals |
| \> memo | String | Some currencies require this parameter for withdrawals |
| \> addrEx | Object | Withdrawal address attachment, e.g. `TONCOIN` attached tag name is comment, the return will be {'comment':'123456'} |
| \> txId | String | Hash record of the withdrawal <br>This parameter will return "" for internal transfers. |
| \> fee | String | Withdrawal fee amount |
| \> feeCcy | String | Withdrawal fee currency, e.g. `USDT` |
| \> state | String | Status of withdrawal<br>Stage 1 : Pending withdrawal<br>`17`: Pending response from Travel Rule vendor<br>`10`: Waiting transfer<br>`0`: Waiting withdrawal<br>`4`/`5`/`6`/`8`/`9`/`12`: Waiting manual review<br>`7`: Approved<br>Stage 2 : Withdrawal in progress (Applicable to on-chain withdrawals, internal transfers do not have this stage)<br>`1`: Broadcasting your transaction to chain<br>`15`: Pending transaction validation<br>`16`: Due to local laws and regulations, your withdrawal may take up to 24 hours to arrive<br>`-3`: Canceling <br>Final stage<br>`-2`: Canceled <br>`-1`: Failed<br>`2`: Success |
| \> wdId | String | Withdrawal ID |
| \> clientId | String | Client-supplied ID |
| \> note | String | Withdrawal note |