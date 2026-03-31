# Binance Wallet API Documentation

Source: https://developers.binance.com/docs/

---

Wallet

# Change Log

## 2026-02-27

- Added a new field `identifier` to the response of `GET /sapi/v1/localentity/vasp`.
- Updated the Travel Rule deposit and withdrawal questionnaire:
  - The input parameter `vasp` should now use the `identifier` field from the `GET /sapi/v1/localentity/vasp` response instead of the previously expected `vaspCode`.
  - Both `vaspCode` and `identifier` will be accepted for the `vasp` field in the deposit and withdrawal questionnaires during the transition period until **28 May 2026**.

* * *

## 2025-12-26

### Time-sensitive Notice

- **The following change to REST API will occur at approximately 2026-01-15 07:00 UTC:**

When calling endpoints that require signatures, percent-encode payloads before computing signatures. Requests that do not follow this order will be rejected with [`-1022 INVALID_SIGNATURE`](https://developers.binance.com/docs/wallet/error-code#-1022-invalid_signature). Please review and update your signing logic accordingly.

### REST API

- Updated documentation for REST API regarding [Signed Endpoints examples for placing an order](https://developers.binance.com/docs/wallet/general-info#signed-endpoint-examples-for-post-apiv3order).

* * *

## 2025-12-19

- Add new API for travel rule:
  - `PUT /sapi/v2/localentity/deposit/provide-info` \- V2 version that uses `depositId` parameter instead of `tranId`.

* * *

## 2025-09-18

- Change menu `Onboarded VASP List` to `VASP List`.

* * *

## 2025-09-12

- Add 1 response field `travelRuleStatus` to `GET /sapi/v1/capital/deposit/hisrec`. travelRuleStatus: 0: travel rule not required OR info already provided and funds ready to use, 1: travel rule required to provide deposit info.

* * *

## 2025-09-08

- Add 1 response field `withdrawTag` to `GET /sapi/v1/capital/config/getall`. To replace `sameAddress` use before. We provide same value for these two fields for now, recommend user to switch to `withdrawTag`.

* * *

## 2025-08-25

- Add new deposit history api.
- Update description of the address verify list api.
- Update weight description of following pages:
  - /travel-rule/withdraw-history
  - /travel-rule/withdraw-history-v2
  - /travel-rule/questionnaire-requirements
  - /travel-rule/onboarded-vasp-list

* * *

## 2025-08-05

- Update footnote of `POST /sapi/v1/capital/withdraw/apply` related to travel rule.

* * *

## 2025-07-11

- Add appendix:
  - Name restriction rules.
  - Country code for travelrule.
- Add new API for travel rule questionnaire requirements.
  - `GET /sapi/v1/localentity/questionnaire-requirements`

* * *

## 2025-06-25

- Update travel rule Japan documentation:
  - Modify withdrawal questionnaire's `txnPurpose`:

    - 1: Purchase of goods within Japan
    - 2: Inheritance, gift or living expenses
    - 3: Investment
    - 5: Use of services provided by the beneficiary VASP
    - 6: Loan repayment
    - 7: Gifts & Donations
  - Remove `txnPurposeOthers`

* * *

## 2025-06-12

- Enable SAPI for France.
- Fix issues in Chinese version.

* * *

## 2025-06-09

- Explained withdrawOrderId in POST `/sapi/v1/capital/withdraw/apply` and GET `/sapi/v1/capital/withdraw/history` in detail.

* * *

## 2025-05-29

- Update withdraw questionnaire of New Zealand to support travel rule requirements.

* * *

## 2025-05-19

- Update withdraw/deposit questionnaire of Bahrain
  - Withdraw:`bnfName` change to `bnfFirstName` and `bnfLastName`
  - Deposit: `orgName` change to `orgFirstName` and `orgLastName`
  - Remove residency field for withdraw/deposit.
- Update withdraw/deposit questionnaire of Poland
  - Withdraw:`bnfName` change to `bnfFirstName` and `bnfLastName`
  - Deposit: `orgName` change to `orgFirstName` and `orgLastName`

* * *

## 2025-05-12

- Questionnaire update for entity KZ and IN.

* * *

## 2025-03-27

- Add new API `GET sapi/v1/capital/withdraw/quota`，Gets the user's withdrawal quota

* * *

## 2025-02-27

- Add 1 response field to `GET /sapi/v1/capital/config/getall`. To fetch "denomination of the coin", default 1

* * *

## 2025-01-15

- Changed Request Weight description from IP to UID for `GET /sapi/v2/localentity/withdraw/history`
- Changed UID rate limit description from 600 to 900 for `GET /sapi/v1/capital/withdraw/apply`.

* * *

## 2025-01-08

- Add new API `GET /sapi/v1/localentity/vasp` to fetch onboarded VASP list for the local entity.
- Add new API `GET /sapi/v2/localentity/withdraw/history` to improve the performance of the query.
- Support all the Binance entities combination.

* * *

## 2024-11-21

- Add 1 response fields to `GET /sapi/v1/capital/config/getall`. To fetch "Minimum internal withdraw amount".
- The following APIs will no longer be supported from 2024-11-21:
  - `POST /sapi/v1/asset/convert-transfer` BUSD asset conversion function is offline. For compatible calls, it now returns: "{"tranId":null,"status":"F","response":"No longer supported"}".
  - `GET /sapi/v1/capital/contract/convertible-coins` BUSD asset convertible stablecoin query function is offline. For compatible calls, it now returns: "{"convertEnabled":false,"coins":\[\],"exchangeRates":{}}".
  - `POST /sapi/v1/capital/contract/convertible-coins` BUSD asset convertible stablecoin editing function is offline. For compatible calls, there will be no changes in the backend.
- Changed maximum idList in `GET /sapi/v1/capital/withdraw/history` to 45.

* * *

## 2024-11-08

- Add 2 new response fields to `GET /sapi/v1/account/info`. To fetch the "European Options account enable status" and the "Portfolio Margin enable status".
- Add 2 new response fields to `GET /sapi/v1/account/apiRestrictions`. To fetch "FIX API trading permission" and "FIX API reading permission".

* * *

## 2024-10-28

- The Withdraw Query History API now supports `withdrawOrderId` as a query parameter.
- The Withdraw Apply API has been updated to include logic for handling cases where the network parameter is empty.

* * *

## 2024-10-18

- Add the onboarded VASP list for each entity.

* * *

## 2024-10-16

- Add the onboarded VASP list of Travel Rule.

* * *

## 2024-10-09

- Update travel rule questionnaire content:
  - Add withdrawal/deposit questionnaire for India: India users can now use sAPI to withdraw/deposit funds

* * *

## 2025-06-25

- Update travel rule Japan documentation:
  - Modify withdrawal questionnaire's `txnPurpose`:

    - 1: Purchase of goods within Japan
    - 2: Inheritance, gift or living expenses
    - 3: Investment
    - 5: Use of services provided by the beneficiary VASP
    - 6: Loan repayment
    - 7: Gifts & Donations
  - Remove `txnPurposeOthers`

* * *

## 2024-08-14

- Fix travel rule api documentation:
  - For NZ travel rule content: `isAddressOwner` should be `1`: Yes, `2`: No
  - Add comments to withdrawal/deposit API regarding url parameters

* * *

## 2024-07-09

- Update travel rule questionnaire content:
  - Add withdrawal/deposit questionnaire for Bahrain: Bahrain users can now use sAPI to withdraw/deposit funds
  - Update deposit questionnaire for Japan: Adding new required filled `isAttested` and fix some text issue

* * *

## 2024-06-21

- Adding local entity withdrawal/deposit APIs to support travel rule requirements:
  - `POST /sapi/v1/localentity/withdraw/apply`
  - `GET /sapi/v1/localentity/withdraw/history`
  - `PUT /sapi/v1/localentity/deposit/provide-info`
  - `GET /sapi/v1/localentity/deposit/history`

* * *

## 2024-06-04

- Wallet Endpoints adjustment: for internal transfers, the txid prefix has been replaced to “Off-chain transfer”on 28 May 2024. “internal transfer” flag is no longer available in the TXID field, including historical transactions, the following endpoints are impacted:
  - `GET /sapi/v1/capital/deposit/hisrec`
  - `GET /sapi/v1/capital/withdraw/history`
  - `GET /sapi/v1/capital/deposit/subHisrec`

* * *

## 2024-05-22

- Update Sub Account Endpoint:
  - `GET /sapi/v1/sub-account/transfer/subUserHistory`: update response field `fromAccountType` and `toAccountType`. Return USDT\_FUTURE/COIN\_FUTURE in order to differentiate 2 futures wallets.
- New Wallet Endpoint:
  - `GET /sapi/v1/account/info`: To fetch the “VIP Level”, “whether Margin account is enabled” and “whether Futures account is enabled”

* * *

## 2024-04-08

- Update Wallet Endpoint:
  - `GET /sapi/v1/capital/config/getall`: delete response field `resetAddressStatus`

* * *

## 2024-01-15

- New Endpoints for Wallet:
  - `GET /sapi/v1/spot/delist-schedule`: Query spot delist schedule
- Update Endpoints for Wallet:
  - `GET /sapi/v1/asset/dribblet`：add parameter `accountType`
  - `POST /sapi/v1/asset/dust-btc`：add parameter `accountType`
  - `POST /sapi/v1/asset/dust`：add parameter `accountType`

* * *

## 2023-11-21

- New endpoint for Wallet:
  - `GET /sapi/v1/capital/deposit/address/list`: Fetch deposit address list with network.

* * *

## 2023-11-02

- Changes to Wallet Endpoint:
  - `GET /sapi/v1/account/apiRestrictions`: add new response field `enablePortfolioMarginTrading`

* * *

## 2023-09-22

- New endpoints for Wallet:
  - `GET /sapi/v1/asset/wallet/balance`: query user wallet balance
  - `GET /sapi/v1/asset/custody/transfer-history`: query user delegation history(For Master Account)

* * *

## 2023-09-04

- Rate limit adjustment for Wallet Endpoint:
  - `GET /sapi/v1/capital/withdraw/history`: UID rate limit is adjusted to 18000, maxmium 10 requests per second. Please refer to the endpoint description for detail

* * *

## 2023-05-18

- New endpoints for Wallet：
  - `POST /sapi/v1/capital/deposit/credit-apply`: apply deposit credit for expired address

* * *

## 2023-05-09

- Update endpoints for Wallet:
  - `POST /sapi/v1/asset/transfer`: add enum `MAIN_PORTFOLIO_MARGIN` and `PORTFOLIO_MARGIN_MAIN`

* * *

## 2023-02-02

- Update endpoints for Wallet:
  - Universal Transfer `POST /sapi/v1/asset/transfer` support option transfer

* * *

## 2022-12-26

- New endpoints for wallet:
  - `GET /sapi/v1/capital/contract/convertible-coins`: Get a user's auto-conversion settings in deposit/withdrawal
  - `POST /sapi/v1/capital/contract/convertible-coins`: User can use it to turn on or turn off the BUSD auto-conversion from/to a specific stable coin.

* * *

## 2022-11-18

- New endpoint for Wallet:
  - `GET /sapi/v1/asset/ledger-transfer/cloud-mining/queryByPage`: The query of Cloud-Mining payment and refund history

* * *

## 2022-11-02

- Update endpoints for Wallet:
  - `POST /sapi/v1/capital/withdraw/apply`: Weight changed to Weight(UID): 600

* * *

## 2022-10-28

- Update endpoints for Wallet:
  - `POST /sapi/v1/asset/convert-transfer`: New parameter `accountType`
  - `POST /sapi/v1/asset/convert-transfer/queryByPage`: request method is changed to `GET`, new parameter `clientTranId`

* * *

## 2022-09-29

- New endpoints for Wallet:
  - `POST /sapi/v1/asset/convert-transfer`: Convert transfer, convert between BUSD and stablecoins.
  - `POST /sapi/v1/asset/convert-transfer/queryByPage`: Query convert transfer

* * *

## 2022-07-01

- New endpoint for Wallet:
  - `POST /sapi/v3/asset/getUserAsset` to get user assets.

* * *

## 2022-2-17

The following updates will take effect on **February 24, 2022 08:00 AM UTC**

- Update endpoint for Wallet：
  - `GET /sapi/v1/accountSnapshot`

The time limit of this endpoint is shortened to only support querying the data of the latest month

* * *

## 2022-02-09

- New endpoint for Wallet:
  - `POST /sapi/v1/asset/dust-btc` to get assets that can be converted into BNB

* * *

## 2021-12-30

- Update endpoint for Wallet：
  - As the Mining account is merged into Funding account, transfer types MAIN\_MINING, MINING\_MAIN, MINING\_UMFUTURE, MARGIN\_MINING, and MINING\_MARGIN will be discontinued in Universal Transfer endpoint `POST /sapi/v1/asset/transfer` on **January 05, 2022 08:00 AM UTC**

* * *

## 2021-11-19

- Update endpoint for Wallet:
  - New field`info`added in`GET /sapi/v1/capital/withdraw/history`to show the reason for withdrawal failure

* * *

## 2021-11-18

The following updates will take effect on **November 25, 2021 08:00 AM UTC**

- Update endpoint for Wallet：
  - `GET /sapi/v1/accountSnapshot`

The query time range of both endpoints are shortened to support data query within the last 6 months only, where startTime does not support selecting a timestamp beyond 6 months.
If you do not specify startTime and endTime, the data of the last 7 days will be returned by default.

* * *

## 2021-11-17

- The following endpoints will be discontinued on **November 17, 2021 13:00 PM UTC**:

  - `POST /sapi/v1/account/apiRestrictions/ipRestriction` to support user enable and disable IP restriction for an API Key
  - `POST /sapi/v1/account/apiRestrictions/ipRestriction/ipList` to support user add IP list for an API Key
  - `GET /sapi/v1/account/apiRestrictions/ipRestriction` to support user query IP restriction for an API Key
  - `DELETE /sapi/v1/account/apiRestrictions/ipRestriction/ipList` to support user delete IP list for an API Key

* * *

## 2021-11-16

- New endpoints for Sub-Account:
  - `POST /sapi/v1/sub-account/subAccountApi/ipRestriction` to support master account enable and disable IP restriction for a sub-account API Key
  - `POST /sapi/v1/sub-account/subAccountApi/ipRestriction/ipList` to support master account add IP list for a sub-account API Key
  - `GET /sapi/v1/sub-account/subAccountApi/ipRestriction` to support master account query IP restriction for a sub-account API Key
  - `DELETE /sapi/v1/sub-account/subAccountApi/ipRestriction/ipList` to support master account delete IP list for a sub-account API Key

* * *

## 2021-11-05

- Update endpoint for Wallet:
  - New parameter `walletType`added in `POST /sapi/v1/capital/withdraw/apply` to support user choose wallet type `spot wallet` and `funding wallet` when withdraw crypto.

* * *

## 2021-11-04

The following updates will take effect on **November 11, 2021 08:00 AM UTC**

- Update endpoints for Wallet and Futures：
  - `GET /sapi/v1/asset/transfer`
  - `GET /sapi/v1/futures/transfer`

The query time range of both endpoints are shortened to support data query within the last 6 months only, where startTime does not support selecting a timestamp beyond 6 months.
If you do not specify startTime and endTime, the data of the last 7 days will be returned by default.

* * *

## 2021-10-22

- Update endpoint for Wallet:
  - New transfer types `MAIN_FUNDING`,`FUNDING_MAIN`,`FUNDING_UMFUTURE`,`UMFUTURE_FUNDING`,`MARGIN_FUNDING`,`FUNDING_MARGIN`,`FUNDING_CMFUTURE`and `CMFUTURE_FUNDING` added in Universal Transfer endpoint `POST /sapi/v1/asset/transfer` and `GET /sapi/v1/asset/transfer` to support transfer assets among funding account and other accounts
  - As the C2C account, Binance Payment, Binance Card and other business account are merged into a Funding account, transfer types `MAIN_C2C`,`C2C_MAIN`,`C2C_UMFUTURE`,`C2C_MINING`,`UMFUTURE_C2C`,`MINING_C2C`,`MARGIN_C2C`,`C2C_MARGIN`,`MAIN_PAY`and `PAY_MAIN` will be discontinued in Universal Transfer endpoint `POST /sapi/v1/asset/transfer` and `GET /sapi/v1/asset/transfer` on **November 04, 2021 08:00 AM UTC**

* * *

## 2021-09-03

- Update endpoint for Wallet:
\_ New fields `sameAddress`,`depositDust` and `specialWithdrawTips`added in `GET /sapi/v1/capital/config/getall``sameAddress` means if the coin needs to provide memo to withdraw
`depositDust` means minimum creditable amount
`specialWithdrawTips` means special tips for withdraw
\_ New field `confirmNo`added in `GET /sapi/v1/capital/withdraw/history` to support query confirm times for withdraw history

* * *

## 2021-08-27

- Update endpoint for Wallet:
  - New parameter `withdrawOrderId`added in `GET  /sapi/v1/capital/withdraw/history` to support user query withdraw history by withdrawOrderId
  - New field `unlockConfirm`added in `GET /sapi/v1/capital/deposit/hisrec` to support query network confirm times for unlocking

* * *

## 2021-08-20

- Update endpoint for Wallet:
  - New parameters`fromSymbol`,`toSymbol`and new transfer types `ISOLATEDMARGIN_MARGIN`, `MARGIN_ISOLATEDMARGIN`and `ISOLATEDMARGIN_ISOLATEDMARGIN` added in `POST /sapi/v1/asset/transfer` and `GET /sapi/v1/asset/transfer` to support user transfer assets between Margin(cross) account and Margin(isolated) account

* * *

## 2021-07-16

- New endpoint for Wallet:
  - `GET /sapi/v1/account/apiRestrictions` to query user API Key permission

* * *

## 2021-07-09

- New endpoint for Wallet:
  - `POST /sapi/v1/asset/get-funding-asset` to query funding wallet, includes Binance Pay, Binance Card, Binance Gift Card, Stock Token

* * *

## 2021-06-24

- Update endpoints for Wallet:
  - `GET /sapi/v1/capital/withdraw/history` added default value 1000, max value 1000 for the parameter`limit`
  - `GET /sapi/v1/capital/deposit/hisrec` added default value 1000, max value 1000 for the parameter`limit`

* * *

## 2021-05-26

- Update endpoint for Wallet:
  - New transfer types `MAIN_PAY` ,`PAY_MAIN` added in Universal Transfer endpoint `POST /sapi/v1/asset/transfer` and `GET /sapi/v1/asset/transfer` to support trasnfer assets between spot account and pay account

* * *

## 2020-12-30

- New endpoint for Wallet:
  - `POST /sapi/v1/asset/transfer` to support user universal transfer among Spot, Margin, Futures, C2C, MINING accounts.
  - `GET /sapi/v1/asset/transfer` to get user universal transfer history.

* * *

## 2020-04-02

- New fields in response to endpoint`GET /sapi/v1/capital/config/getall`：

  - `minConfirm` for min number for balance confirmation
  - `unLockConfirm` for confirmation number for balance unlock

* * *

## 2020-03-13

- New parameter `transactionFeeFlag` is available in endpoint:

  - `POST /sapi/v1/capital/withdraw/apply` and
  - `POST /wapi/v3/withdraw.html`

* * *

## 2020-01-15

- New parameter `withdrawOrderId` for client customized withdraw id for endpoint `POST /wapi/v3/withdraw.html`.
- New field `withdrawOrderId` in response to `GET /wapi/v3/withdrawHistory.html`

* * *

## 2019-12-25

- Added time interval limit in

`GET  /sapi/v1/capital/withdraw/history`,

`GET /wapi/v3/withdrawHistory.html`,

`GET /sapi/v1/capital/deposit/hisrec` and

`GET /wapi/v3/depositHistory.html`:
\_ The default `startTime` is 90 days from current time, and the default `endTime` is current time.
\_ Please notice the default `startTime` and `endTime` to make sure that time interval is within 0-90 days. \* If both `startTime` and `endTime` are sent, time between `startTime` and `endTime` must be less than 90 days.

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

## 2019-10-29

- New sapi endpoints for wallet.
  - `POST /sapi/v1/capital/withdraw/apply (HMAC SHA256)`: withdraw.
  - `Get /sapi/v1/capital/withdraw/history (HMAC SHA256)`: fetch withdraw history with network.

* * *

## 2019-10-14

- New sapi endpoints for wallet.
  - `GET /sapi/v1/capital/config/getall (HMAC SHA256)`: get all coins' information for user.
  - `GET /sapi/v1/capital/deposit/hisrec (HMAC SHA256)`: fetch deposit history with network.
  - `GET /sapi/v1/capital/deposit/address (HMAC SHA256)`: fetch deposit address with network.

Copyright © 2026 Binance.

---

Wallet

# Account info (USER\_DATA)

## API Description

Fetch account info detail.

## HTTP Request

GET `/sapi/v1/account/info`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "vipLevel": 0,
    "isMarginEnabled": true,                   // true or false for margin.
    "isFutureEnabled": true,                   // true or false for futures.
    "isOptionsEnabled": true,                  // true or false for options.
    "isPortfolioMarginRetailEnabled": true     // true or false for portfolio margin retail.
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Account API Trading Status (USER\_DATA)

## API Description

Fetch account api trading status detail.

## HTTP Request

GET `/sapi/v1/account/apiTradingStatus`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "data": {
        // API trading status detail
        "isLocked": false,              // API trading function is locked or not
        "plannedRecoverTime": 0,        // If API trading function is locked, this is the planned recover time
        "triggerCondition": {
            "GCR": 150,                 // Number of GTC orders
            "IFER": 150,                // Number of FOK/IOC orders
            "UFR": 300                  // Number of orders
        },
        "updateTime": 1547630471725
    }
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Account info (USER\_DATA)

## API Description

Fetch account info detail.

## HTTP Request

GET `/sapi/v1/account/info`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "vipLevel": 0,
    "isMarginEnabled": true,                   // true or false for margin.
    "isFutureEnabled": true,                   // true or false for futures.
    "isOptionsEnabled": true,                  // true or false for options.
    "isPortfolioMarginRetailEnabled": true     // true or false for portfolio margin retail.
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Account Status (USER\_DATA)

## API Description

Fetch account status detail.

## HTTP Request

GET `/sapi/v1/account/status`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "data": "Normal"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Get API Key Permission (USER\_DATA)

## API Description

Get API Key Permission

## HTTP Request

GET `/sapi/v1/account/apiRestrictions`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "ipRestrict": false,
    "createTime": 1698645219000,
    "enableReading": true,
    "enableWithdrawals": false,              // This option allows you to withdraw via API. You must apply the IP Access Restriction filter in order to enable withdrawals
    "enableInternalTransfer": false,         // This option authorizes this key to transfer funds between your master account and your sub account instantly
    "enableMargin": false,                   // This option can be adjusted after the Cross Margin account transfer is completed
    "enableFutures": false,                  // The Futures API cannot be used if the API key was created before the Futures account was opened, or if you have enabled portfolio margin.
    "permitsUniversalTransfer": false,       // Authorizes this key to be used for a dedicated universal transfer API to transfer multiple supported currencies. Each business's own transfer API rights are not affected by this authorization
    "enableVanillaOptions": false,           // Authorizes this key to Vanilla options trading
    "enableFixApiTrade": false,              //
    "enableFixReadOnly": true,
    "enableSpotAndMarginTrading": false,     // Spot and margin trading
    "enablePortfolioMarginTrading": true     // API Key created before your activate portfolio margin does not support portfolio margin API service
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Daily Account Snapshot (USER\_DATA)

## API Description

Daily account snapshot

## HTTP Request

GET `/sapi/v1/accountSnapshot`

## Request Weight(IP)

2400

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | STRING | YES | "SPOT", "MARGIN", "FUTURES" |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | min 7, max 30, default 7 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - The query time period must be less then 30 days
> - Support query within the last one month only
> - If startTimeand endTime not sent, return records of the last 7 days by default

## Response Example

```javascript
{
    "code": 200,                                    // 200 for success; others are error codes
    "msg": "",                                      // error message
    "snapshotVos": [
        {
            "data": {
                "balances": [
                    {
                        "asset": "BTC",
                        "free": "0.09905021",
                        "locked": "0.00000000"
                    },
                    {
                        "asset": "USDT",
                        "free": "1.89109409",
                        "locked": "0.00000000"
                    }
                ],
                "totalAssetOfBtc": "0.09942700"
            },
            "type": "spot",
            "updateTime": 1576281599000
        }
    ]
}
```

> OR

```javascript
{
    "code": 200,                                         // 200 for success; others are error codes
    "msg": "",                                           // error message
    "snapshotVos": [
        {
            "data": {
                "marginLevel": "2748.02909813",
                "totalAssetOfBtc": "0.00274803",
                "totalLiabilityOfBtc": "0.00000100",
                "totalNetAssetOfBtc": "0.00274750",
                "userAssets": [
                    {
                        "asset": "XRP",
                        "borrowed": "0.00000000",
                        "free": "1.00000000",
                        "interest": "0.00000000",
                        "locked": "0.00000000",
                        "netAsset": "1.00000000"
                    }
                ]
            },
            "type": "margin",
            "updateTime": 1576281599000
        }
    ]
}
```

> OR

```javascript
{
    "code": 200,                                             // 200 for success; others are error codes
    "msg": "",                                               // error message
    "snapshotVos": [
        {
            "data": {
                "assets": [
                    {
                        "asset": "USDT",
                        "marginBalance": "118.99782335",     // Not real-time data, can ignore
                        "walletBalance": "120.23811389"
                    }
                ],
                "position": [
                    {
                        "entryPrice": "7130.41000000",
                        "markPrice": "7257.66239673",
                        "positionAmt": "0.01000000",
                        "symbol": "BTCUSDT",
                        "unRealizedProfit": "1.24029054"     // Only show the value at the time of opening the position
                    }
                ]
            },
            "type": "futures",
            "updateTime": 1576281599000
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Disable Fast Withdraw Switch (USER\_DATA)

## HTTP Request

POST `/sapi/v1/account/disableFastWithdrawSwitch`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- **Caution:**

This request will disable fastwithdraw switch under your account.

You need to enable "trade" option for the api key which requests this endpoint.

## Response Example

```text
{}
```

 

Copyright © 2026 Binance.

---

Wallet

# Enable Fast Withdraw Switch (USER\_DATA)

## API Description

Enable Fast Withdraw Switch (USER\_DATA)

## HTTP Request

POST `/sapi/v1/account/enableFastWithdrawSwitch`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - This request will enable fastwithdraw switch under your account.
>
>
>
>
>   You need to enable "trade" option for the api key which requests this endpoint.
> - When Fast Withdraw Switch is on, transferring funds to a Binance account will be done instantly. There is no on-chain transaction, no transaction ID and no withdrawal fee.

## Response Example

```text
{}
```

 

Copyright © 2026 Binance.

---

Wallet

# Asset Detail (USER\_DATA)

## API Description

Fetch details of assets supported on Binance.

## HTTP Request

GET `/sapi/v1/asset/assetDetail`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- Please get network and other deposit or withdraw details from `GET /sapi/v1/capital/config/getall`.

## Response Example

```javascript
{
    "CTR": {
        "minWithdrawAmount": "70.00000000",             // min withdraw amount
        "depositStatus": false,                         // deposit status (false if ALL of networks' are false)
        "withdrawFee": 35,                              // withdraw fee
        "withdrawStatus": true,                         // withdraw status (false if ALL of networks' are false)
        "depositTip": "Delisted, Deposit Suspended"     // reason
    },
    "SKY": {
        "minWithdrawAmount": "0.02000000",
        "depositStatus": true,
        "withdrawFee": 0.01,
        "withdrawStatus": true
    }
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Toggle BNB Burn On Spot Trade And Margin Interest (USER\_DATA)

## API Description

Toggle BNB Burn On Spot Trade And Margin Interest

## HTTP Request

POST `/sapi/v1/bnbBurn`

## Request Weight

**1(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| spotBNBBurn | STRING | NO | "true" or "false"; Determines whether to use BNB to pay for trading fees on SPOT |
| interestBNBBurn | STRING | NO | "true" or "false"; Determines whether to use BNB to pay for margin loan's interest |
| recvWindow | LONG | NO | No more than 60000 |
| timestamp | LONG | YES |  |

- "spotBNBBurn" and "interestBNBBurn" should be sent at least one.

## Response Example

```javascript
{
    "spotBNBBurn": true,
    "interestBNBBurn": false
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Asset Detail (USER\_DATA)

## API Description

Fetch details of assets supported on Binance.

## HTTP Request

GET `/sapi/v1/asset/assetDetail`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- Please get network and other deposit or withdraw details from `GET /sapi/v1/capital/config/getall`.

## Response Example

```javascript
{
    "CTR": {
        "minWithdrawAmount": "70.00000000",             // min withdraw amount
        "depositStatus": false,                         // deposit status (false if ALL of networks' are false)
        "withdrawFee": 35,                              // withdraw fee
        "withdrawStatus": true,                         // withdraw status (false if ALL of networks' are false)
        "depositTip": "Delisted, Deposit Suspended"     // reason
    },
    "SKY": {
        "minWithdrawAmount": "0.02000000",
        "depositStatus": true,
        "withdrawFee": 0.01,
        "withdrawStatus": true
    }
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Get Assets That Can Be Converted Into BNB (USER\_DATA)

## API Description

Get Assets That Can Be Converted Into BNB

## HTTP Request

POST `/sapi/v1/asset/dust-btc`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| accountType | STRING | NO | `SPOT` or `MARGIN`,default `SPOT` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "details": [
        {
            "asset": "ADA",
            "assetFullName": "ADA",
            "amountFree": "6.21",                 // Convertible amount
            "toBTC": "0.00016848",                // BTC amount
            "toBNB": "0.01777302",                // BNB amount（Not deducted commission fee）
            "toBNBOffExchange": "0.01741756",     // BNB amount（Deducted commission fee）
            "exchange": "0.00035546"              // Commission fee
        }
    ],
    "totalTransferBtc": "0.00016848",
    "totalTransferBNB": "0.01777302",
    "dribbletPercentage": "0.02"                  // Commission fee
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Asset Dividend Record (USER\_DATA)

## API Description

Query asset dividend record.

## HTTP Request

GET `/sapi/v1/asset/assetDividend`

## Request Weight(IP)

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 20, max 500 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - There cannot be more than 180 days between parameter `startTime` and `endTime`.

## Response Example

```javascript
{
    "rows": [
        {
            "id": 1637366104,
            "amount": "10.00000000",
            "asset": "BHFT",
            "divTime": 1563189166000,
            "enInfo": "BHFT distribution",
            "tranId": 2968885920,
            "direction": 1 //direction: 1 for Asset credited (inflow), -1 for Asset debited (outflow)
        },
        {
            "id": 1631750237,
            "amount": "10.00000000",
            "asset": "BHFT",
            "divTime": 1563189165000,
            "enInfo": "BHFT distribution",
            "tranId": 2968885920,
            "direction": 1
        }
    ],
    "total": 2
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Get Cloud-Mining payment and refund history (USER\_DATA)

## API Description

The query of Cloud-Mining payment and refund history

## HTTP Request

GET `/sapi/v1/asset/ledger-transfer/cloud-mining/queryByPage`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| tranId | LONG | NO | The transaction id |
| clientTranId | STRING | NO | The unique flag |
| asset | STRING | NO | If it is blank, we will query all assets |
| startTime | LONG | YES | inclusive, unit: ms |
| endTime | LONG | YES | exclusive, unit: ms |
| current | INTEGER | NO | current page, default 1, the min value is 1 |
| size | INTEGER | NO | page size, default 10, the max value is 100 |

> - Just return the SUCCESS records of payment and refund.
> - For response, type = 248 means payment, type = 249 means refund, status =S means SUCCESS.

## Response Example

```javascript
{
    "total": 5,
    "rows": [
        {
            "createTime": 1667880112000,
            "tranId": 121230610120,
            "type": 248,
            "asset": "USDT",
            "amount": "25.0068",
            "status": "S"
        },
        {
            "createTime": 1666776366000,
            "tranId": 119991507468,
            "type": 249,
            "asset": "USDT",
            "amount": "0.027",
            "status": "S"
        },
        {
            "createTime": 1666764505000,
            "tranId": 119977966327,
            "type": 248,
            "asset": "USDT",
            "amount": "0.027",
            "status": "S"
        },
        {
            "createTime": 1666758189000,
            "tranId": 119973601721,
            "type": 248,
            "asset": "USDT",
            "amount": "0.018",
            "status": "S"
        },
        {
            "createTime": 1666757278000,
            "tranId": 119973028551,
            "type": 248,
            "asset": "USDT",
            "amount": "0.018",
            "status": "S"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Dust Convert (USER\_DATA)

## API Description

Convert dust assets

## HTTP Request

POST `/sapi/v1/asset/dust-convert/convert`

## Request Weight(UID)

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | ARRAY | YES |  |
| clientId | STRING | NO | A unique id for the request |
| targetAsset | STRING | NO |  |
| thirdPartyClientId | STRING | NO |  |
| dustQuotaAssetToTargetAssetPrice | BIGDECIMAL | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "totalTransfered": "3.5971223",
    "totalServiceCharge": "0.0794964",
    "transferResult": [
        {
            "tranId": 2987331510,
            "fromAsset": "USDT",
            "amount": "1",
            "transferedAmount": "3.5971223",
            "serviceChargeAmount": "0.0794964",
            "operateTime": 1765212029749
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Dust Convertible Assets (USER\_DATA)

## API Description

Query dust convertible assets

## HTTP Request

POST `/sapi/v1/asset/dust-convert/query-convertible-assets`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| targetAsset | STRING | YES |  |
| dustQuotaAssetToTargetAssetPrice | BIGDECIMAL | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "dribbletPercentage": "0.02",
    "totalTransferQuotaAssetAmount": "0.7899968",
    "totalTransferTargetAssetAmount": "0.7899968",
    "dribbletBase": "10",
    "details": [
        {
            "asset": "AR",
            "assetFullName": "AR",
            "amountFree": "0.00856",
            "exchange": "0.00073616",
            "toQuotaAssetAmount": "0.036808",
            "toTargetAssetAmount": "0.036808",
            "toTargetAssetOffExchange": "0.03607184"
        },
        {
            "asset": "BNB",
            "assetFullName": "BNB",
            "amountFree": "0.00082768",
            "exchange": "0.01506378",
            "toQuotaAssetAmount": "0.7531888",
            "toTargetAssetAmount": "0.7531888",
            "toTargetAssetOffExchange": "0.73812502"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# DustLog(USER\_DATA)

## API Description

Dustlog

## HTTP Request

GET `/sapi/v1/asset/dribblet`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| accountType | STRING | NO | `SPOT`or`MARGIN`,default`SPOT` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Only return last 100 records
> - Only return records after 2020/12/01

## Response Example

```javascript
{
    "total": 8,                                              // Total counts of exchange
    "userAssetDribblets": [
        {
            "operateTime": 1615985535000,
            "totalTransferedAmount": "0.00132256",           // Total transfered BNB amount for this exchange.
            "totalServiceChargeAmount": "0.00002699",        // Total service charge amount for this exchange.
            "transId": 45178372831,
            "userAssetDribbletDetails": [
                //Details of  this exchange.
                {
                    "transId": 4359321,
                    "serviceChargeAmount": "0.000009",
                    "amount": "0.0009",
                    "operateTime": 1615985535000,
                    "transferedAmount": "0.000441",
                    "fromAsset": "USDT"
                },
                {
                    "transId": 4359321,
                    "serviceChargeAmount": "0.00001799",
                    "amount": "0.0009",
                    "operateTime": 1615985535000,
                    "transferedAmount": "0.00088156",
                    "fromAsset": "ETH"
                }
            ]
        },
        {
            "operateTime": 1616203180000,
            "totalTransferedAmount": "0.00058795",
            "totalServiceChargeAmount": "0.000012",
            "transId": 4357015,
            "userAssetDribbletDetails": [
                {
                    "transId": 4357015,
                    "serviceChargeAmount": "0.00001",
                    "amount": "0.001",
                    "operateTime": 1616203180000,
                    "transferedAmount": "0.00049",
                    "fromAsset": "USDT"
                },
                {
                    "transId": 4357015,
                    "serviceChargeAmount": "0.000002",
                    "amount": "0.0001",
                    "operateTime": 1616203180000,
                    "transferedAmount": "0.00009795",
                    "fromAsset": "ETH"
                }
            ]
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Dust Transfer (USER\_DATA)

## API Description

Convert dust assets to BNB.

## HTTP Request

POST `/sapi/v1/asset/dust`

## Request Weight(UID)

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | ARRAY | YES | The asset being converted. For example: asset=BTC,USDT |
| accountType | STRING | NO | `SPOT` or `MARGIN`,default `SPOT` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - You need to open`Enable Spot & Margin Trading` permission for the API Key which requests this endpoint.

## Response Example

```javascript
{
    "totalServiceCharge": "0.02102542",
    "totalTransfered": "1.05127099",
    "transferResult": [
        {
            "amount": "0.03000000",
            "fromAsset": "ETH",
            "operateTime": 1563368549307,
            "serviceChargeAmount": "0.00500000",
            "tranId": 2970932918,
            "transferedAmount": "0.25000000"
        },
        {
            "amount": "0.09000000",
            "fromAsset": "LTC",
            "operateTime": 1563368549404,
            "serviceChargeAmount": "0.01548000",
            "tranId": 2970932918,
            "transferedAmount": "0.77400000"
        },
        {
            "amount": "248.61878453",
            "fromAsset": "TRX",
            "operateTime": 1563368549489,
            "serviceChargeAmount": "0.00054542",
            "tranId": 2970932918,
            "transferedAmount": "0.02727099"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Funding Wallet (USER\_DATA)

## API Description

Query Funding Wallet

## HTTP Request

POST `/sapi/v1/asset/get-funding-asset`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| needBtcValuation | STRING | NO | true or false |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Currently supports querying the following business assets：Binance Pay, Binance Card, Binance Gift Card, Stock Token

## Response Example

```javascript
[
    {
        "asset": "USDT",
        "free": "1",                     // avalible balance
        "locked": "0",                   // locked asset
        "freeze": "0",                   // freeze asset
        "withdrawing": "0",
        "btcValuation": "0.00000091"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Get Open Symbol List (MARKET\_DATA)

## API Description

Get the list of symbols that are scheduled to be opened for trading in the market.

## HTTP Request

GET `/sapi/v1/spot/open-symbol-list`

## Request Weight(IP)

**100**

## Request Parameters

No parameters required.

## Response Example

```javascript
[
    {
        "openTime": 1686161202000,
        "symbols": ["BNBBTC", "BNBETH"]
    },
    {
        "openTime": 1686222232000,
        "symbols": ["BTCUSDT"]
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Query User Delegation History(For Master Account)(USER\_DATA)

## API Description

Query User Delegation History

## HTTP Request

GET `/sapi/v1/asset/custody/transfer-history`

## Request Weight(IP)

**60**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| email | STRING | YES |  |
| startTime | LONG | YES |  |
| endTime | LONG | YES |  |
| type | ENUM | NO | Delegate/Undelegate |
| asset | STRING | NO |  |
| current | INTEGER | NO | default 1 |
| size | INTEGER | NO | default 10, max 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "total": 3316,
    "rows": [
        {
            "clientTranId": "293915932290879488",
            "transferType": "Undelegate",
            "asset": "ETH",
            "amount": "1",
            "time": 1695205406000
        },
        {
            "clientTranId": "293915892281413632",
            "transferType": "Delegate",
            "asset": "ETH",
            "amount": "1",
            "time": 1695205396000
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Query User Universal Transfer History(USER\_DATA)

## API Description

Query User Universal Transfer History

## HTTP Request

GET `/sapi/v1/asset/transfer`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | INT | NO | Default 1 |
| size | INT | NO | Default 10, Max 100 |
| fromSymbol | STRING | NO |  |
| toSymbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - `fromSymbol` must be sent when type are ISOLATEDMARGIN\_MARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN
> - `toSymbol` must be sent when type are MARGIN\_ISOLATEDMARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN
> - Support query within the last 6 months only
> - If `startTime`and `endTime` not sent, return records of the last 7 days by default

## Response Example

```javascript
{
    "total": 2,
    "rows": [
        {
            "asset": "USDT",
            "amount": "1",
            "type": "MAIN_UMFUTURE",
            "status": "CONFIRMED", // status: CONFIRMED / FAILED / PENDING
            "tranId": 11415955596,
            "timestamp": 1544433328000
        },
        {
            "asset": "USDT",
            "amount": "2",
            "type": "MAIN_UMFUTURE",
            "status": "CONFIRMED",
            "tranId": 11366865406,
            "timestamp": 1544433328000
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Query User Wallet Balance (USER\_DATA)

## API Description

Query User Wallet Balance

## HTTP Request

GET `/sapi/v1/asset/wallet/balance`

## Request Weight(IP)

**60**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| quoteAsset | STRING | NO | `USDT`, `ETH`, `USDC`, `BNB`, etc. default `BTC` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "activate": true,
        "balance": "0",
        "walletName": "Spot"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Funding"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Cross Margin"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Isolated Margin"
    },
    {
        "activate": true,
        "balance": "0.71842752",
        "walletName": "USDⓈ-M Futures"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "COIN-M Futures"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Earn"
    },
    {
        "activate": false,
        "balance": "0",
        "walletName": "Options"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Trading Bots"
    },
    {
        "activate": true,
        "balance": "0",
        "walletName": "Copy Trading"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Get Spot Delist Schedule (MARKET\_DATA)

## API Description

Get symbols delist schedule for spot

## HTTP Request

GET `/sapi/v1/spot/delist-schedule`

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
        "symbols": ["BTCUSDT", "ETHUSDT"]
    },
    {
        "delistTime": 1686222232000,
        "symbols": ["ADAUSDT"]
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Trade Fee (USER\_DATA)

## API Description

Fetch trade fee

## HTTP Request

GET `/sapi/v1/asset/tradeFee`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "symbol": "ADABNB",
        "makerCommission": "0.001",
        "takerCommission": "0.001"
    },
    {
        "symbol": "BNBBTC",
        "makerCommission": "0.001",
        "takerCommission": "0.001"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# User Asset (USER\_DATA)

## API Description

Get user assets, just for positive data.

## HTTP Request

POST `/sapi/v3/asset/getUserAsset`

## Request Weight(IP)

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO | If asset is blank, then query all positive assets user have. |
| needBtcValuation | BOOLEAN | NO | Whether need btc valuation or not. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If asset is set, then return this asset, otherwise return all assets positive.
> - If needBtcValuation is set, then return btcValudation.

## Response Example

```javascript
[
    {
        "asset": "AVAX",
        "free": "1",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "BCH",
        "free": "0.9",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "BNB",
        "free": "887.47061626",
        "locked": "0",
        "freeze": "10.52",
        "withdrawing": "0.1",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "BUSD",
        "free": "9999.7",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "SHIB",
        "free": "532.32",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "USDT",
        "free": "50300000001.44911105",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    },
    {
        "asset": "WRZ",
        "free": "1",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoable": "0",
        "btcValuation": "0"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# User Universal Transfer (USER\_DATA)

## API Description

user universal transfer

## HTTP Request

POST `/sapi/v1/asset/transfer`

You need to enable `Permits Universal Transfer` option for the API Key which requests this endpoint.

## Request Weight(UID)

**900**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| type | ENUM | YES |  |
| asset | STRING | YES |  |
| amount | DECIMAL | YES |  |
| fromSymbol | STRING | NO |  |
| toSymbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- `fromSymbol` must be sent when type are ISOLATEDMARGIN\_MARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN

- `toSymbol` must be sent when type are MARGIN\_ISOLATEDMARGIN and ISOLATEDMARGIN\_ISOLATEDMARGIN

- ENUM of transfer types:
  - MAIN\_UMFUTURE Spot account transfer to USDⓈ-M Futures account
  - MAIN\_CMFUTURE Spot account transfer to COIN-M Futures account
  - MAIN\_MARGIN Spot account transfer to Margin（cross）account
  - UMFUTURE\_MAIN USDⓈ-M Futures account transfer to Spot account
  - UMFUTURE\_MARGIN USDⓈ-M Futures account transfer to Margin（cross）account
  - CMFUTURE\_MAIN COIN-M Futures account transfer to Spot account
  - CMFUTURE\_MARGIN COIN-M Futures account transfer to Margin(cross) account
  - MARGIN\_MAIN Margin（cross）account transfer to Spot account
  - MARGIN\_UMFUTURE Margin（cross）account transfer to USDⓈ-M Futures
  - MARGIN\_CMFUTURE Margin（cross）account transfer to COIN-M Futures
  - ISOLATEDMARGIN\_MARGIN Isolated margin account transfer to Margin(cross) account
  - MARGIN\_ISOLATEDMARGIN Margin(cross) account transfer to Isolated margin account
  - ISOLATEDMARGIN\_ISOLATEDMARGIN Isolated margin account transfer to Isolated margin account
  - MAIN\_FUNDING Spot account transfer to Funding account
  - FUNDING\_MAIN Funding account transfer to Spot account
  - FUNDING\_UMFUTURE Funding account transfer to UMFUTURE account
  - UMFUTURE\_FUNDING UMFUTURE account transfer to Funding account
  - MARGIN\_FUNDING MARGIN account transfer to Funding account
  - FUNDING\_MARGIN Funding account transfer to Margin account
  - FUNDING\_CMFUTURE Funding account transfer to CMFUTURE account
  - CMFUTURE\_FUNDING CMFUTURE account transfer to Funding account
  - MAIN\_OPTION Spot account transfer to Options account
  - OPTION\_MAIN Options account transfer to Spot account
  - UMFUTURE\_OPTION USDⓈ-M Futures account transfer to Options account
  - OPTION\_UMFUTURE Options account transfer to USDⓈ-M Futures account
  - MARGIN\_OPTION Margin（cross）account transfer to Options account
  - OPTION\_MARGIN Options account transfer to Margin（cross）account
  - FUNDING\_OPTION Funding account transfer to Options account
  - OPTION\_FUNDING Options account transfer to Funding account
  - MAIN\_PORTFOLIO\_MARGIN Spot account transfer to Portfolio Margin account
  - PORTFOLIO\_MARGIN\_MAIN Portfolio Margin account transfer to Spot account

## Response Example

```javascript
{
    "tranId": 13526853623
}
```

 

Copyright © 2026 Binance.

---

Wallet

# All Coins' Information (USER\_DATA)

## API Description

Get information of coins (available for deposit and withdraw) for user.

## HTTP Request

GET `/sapi/v1/capital/config/getall`

## Request Weight(IP)

10

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "coin": "1MBABYDOGE",
        "depositAllEnable": true,
        "withdrawAllEnable": true,
        "name": "1M x BABYDOGE",
        "free": "34941.1",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoing": "0",
        "ipoable": "0",
        "storage": "0",
        "isLegalMoney": false,
        "trading": true,
        "networkList": [
            {
                "network": "BSC",
                "coin": "1MBABYDOGE",
                "withdrawIntegerMultiple": "0.01",
                "isDefault": false,
                "depositEnable": true,
                "withdrawEnable": true,
                "depositDesc": "",                                                   // shown only when "depositEnable" is false.
                "withdrawDesc": "",                                                  // shown only when "withdrawEnable" is false.
                "specialTips": "",
                "specialWithdrawTips": "",
                "name": "BNB Smart Chain (BEP20)",
                "resetAddressStatus": false,
                "addressRegex": "^(0x)[0-9A-Fa-f]{40}$",
                "memoRegex": "",
                "withdrawFee": "10",
                "withdrawMin": "20",
                "withdrawMax": "9999999999",
                "withdrawInternalMin": "0.01",                                       // Minimum internal transfer amount
                "depositDust": "0.01",
                "minConfirm": 5,                                                     // min number for balance confirmation
                "unLockConfirm": 0,                                                  // confirmation number for balance unlock
                "sameAddress": false,                                                // Obsoleted, recomment to use withdrawTag
                "withdrawTag": false,                                                // If the coin needs to provide memo to withdraw
                "estimatedArrivalTime": 1,
                "busy": false,
                "contractAddressUrl": "https://bscscan.com/token/",
                "contractAddress": "0xc748673057861a797275cd8a068abb95a902e8de",
                "denomination": 1000000                                              // 1 1MBABYDOGE = 1000000 BABYDOGE
            },
            {
                "network": "ETH",
                "coin": "1MBABYDOGE",
                "withdrawIntegerMultiple": "0.01",
                "isDefault": true,
                "depositEnable": true,
                "withdrawEnable": true,
                "depositDesc": "",
                "withdrawDesc": "",
                "specialTips": "",
                "specialWithdrawTips": "",
                "name": "Ethereum (ERC20)",
                "resetAddressStatus": false,
                "addressRegex": "^(0x)[0-9A-Fa-f]{40}$",
                "memoRegex": "",
                "withdrawFee": "1511",
                "withdrawMin": "3022",
                "withdrawMax": "9999999999",
                "withdrawInternalMin": "0.01",
                "depositDust": "0.01",
                "minConfirm": 6,
                "unLockConfirm": 64,
                "sameAddress": false,
                "withdrawTag": false,
                "estimatedArrivalTime": 2,
                "busy": false,
                "contractAddressUrl": "https://etherscan.io/address/",
                "contractAddress": "0xac57de9c1a09fec648e93eb98875b212db0d460b",
                "denomination": 1000000
            }
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# All Coins' Information (USER\_DATA)

## API Description

Get information of coins (available for deposit and withdraw) for user.

## HTTP Request

GET `/sapi/v1/capital/config/getall`

## Request Weight(IP)

10

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "coin": "1MBABYDOGE",
        "depositAllEnable": true,
        "withdrawAllEnable": true,
        "name": "1M x BABYDOGE",
        "free": "34941.1",
        "locked": "0",
        "freeze": "0",
        "withdrawing": "0",
        "ipoing": "0",
        "ipoable": "0",
        "storage": "0",
        "isLegalMoney": false,
        "trading": true,
        "networkList": [
            {
                "network": "BSC",
                "coin": "1MBABYDOGE",
                "withdrawIntegerMultiple": "0.01",
                "isDefault": false,
                "depositEnable": true,
                "withdrawEnable": true,
                "depositDesc": "",                                                   // shown only when "depositEnable" is false.
                "withdrawDesc": "",                                                  // shown only when "withdrawEnable" is false.
                "specialTips": "",
                "specialWithdrawTips": "",
                "name": "BNB Smart Chain (BEP20)",
                "resetAddressStatus": false,
                "addressRegex": "^(0x)[0-9A-Fa-f]{40}$",
                "memoRegex": "",
                "withdrawFee": "10",
                "withdrawMin": "20",
                "withdrawMax": "9999999999",
                "withdrawInternalMin": "0.01",                                       // Minimum internal transfer amount
                "depositDust": "0.01",
                "minConfirm": 5,                                                     // min number for balance confirmation
                "unLockConfirm": 0,                                                  // confirmation number for balance unlock
                "sameAddress": false,                                                // Obsoleted, recomment to use withdrawTag
                "withdrawTag": false,                                                // If the coin needs to provide memo to withdraw
                "estimatedArrivalTime": 1,
                "busy": false,
                "contractAddressUrl": "https://bscscan.com/token/",
                "contractAddress": "0xc748673057861a797275cd8a068abb95a902e8de",
                "denomination": 1000000                                              // 1 1MBABYDOGE = 1000000 BABYDOGE
            },
            {
                "network": "ETH",
                "coin": "1MBABYDOGE",
                "withdrawIntegerMultiple": "0.01",
                "isDefault": true,
                "depositEnable": true,
                "withdrawEnable": true,
                "depositDesc": "",
                "withdrawDesc": "",
                "specialTips": "",
                "specialWithdrawTips": "",
                "name": "Ethereum (ERC20)",
                "resetAddressStatus": false,
                "addressRegex": "^(0x)[0-9A-Fa-f]{40}$",
                "memoRegex": "",
                "withdrawFee": "1511",
                "withdrawMin": "3022",
                "withdrawMax": "9999999999",
                "withdrawInternalMin": "0.01",
                "depositDust": "0.01",
                "minConfirm": 6,
                "unLockConfirm": 64,
                "sameAddress": false,
                "withdrawTag": false,
                "estimatedArrivalTime": 2,
                "busy": false,
                "contractAddressUrl": "https://etherscan.io/address/",
                "contractAddress": "0xac57de9c1a09fec648e93eb98875b212db0d460b",
                "denomination": 1000000
            }
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Deposit Address(supporting network) (USER\_DATA)

## API Description

Fetch deposit address with network.

## HTTP Request

GET `/sapi/v1/capital/deposit/address`

## Request Weight(IP)

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | YES |  |
| network | STRING | NO |  |
| amount | DECIMAL | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If `network` is not send, return with default network of the coin.
> - You can get `network` and `isDefault` in `networkList` in the response of `Get /sapi/v1/capital/config/getall (HMAC SHA256)`.
> - `amount` needs to be sent if using LIGHTNING network

## Response Example

```javascript
{
    "address": "1HPn8Rx2y6nNSfagQBKy27GB99Vbzg89wv",
    "coin": "BTC",
    "tag": "",
    "url": "https://btc.com/1HPn8Rx2y6nNSfagQBKy27GB99Vbzg89wv"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Deposit History (supporting network) (USER\_DATA)

## API Description

Fetch deposit history.

## HTTP Request

GET `/sapi/v1/capital/deposit/hisrec`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| includeSource | Boolean | NO | Default: `false`, return `sourceAddress`field when set to `true` |
| coin | STRING | NO |  |
| status | INT | NO | 0(0:pending, 6:credited but cannot withdraw, 7:Wrong Deposit, 8:Waiting User confirm, 1:success, 2:rejected) |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| offset | INT | NO | Default:0 |
| limit | INT | NO | Default:1000, Max:1000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |
| txId | STRING | NO |  |

> - Please notice the default `startTime` and `endTime` to make sure that time interval is within 0-90 days.
> - If both `startTime` and `endTime` are sent, time between `startTime` and `endTime` must be less than 90 days.

## Response Example

```javascript
[
    {
        "id": "769800519366885376",
        "amount": "0.001",
        "coin": "BNB",
        "network": "BNB",
        "status": 1,
        "address": "bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23",
        "addressTag": "101764890",
        "txId": "98A3EA560C6B3336D348B6C83F0F95ECE4F1F5919E94BD006E5BF3BF264FACFC",
        "insertTime": 1661493146000,
        "completeTime": 1661493146000,
        "transferType": 0,
        "confirmTimes": "1/1",
        "unlockConfirm": 0,
        "walletType": 0,
        "travelRuleStatus": 0                                                                                        // 0: travel rule not required OR info already provided and funds ready to use, 1: travel rule required to provide deposit info
    },
    {
        "id": "769754833590042625",
        "amount": "0.50000000",
        "coin": "IOTA",
        "network": "IOTA",
        "status": 1,
        "address": "SIZ9VLMHWATXKV99LH99CIGFJFUMLEHGWVZVNNZXRJJVWBPHYWPPBOSDORZ9EQSHCZAMPVAPGFYQAUUV9DROOXJLNW",
        "addressTag": "",
        "txId": "ESBFVQUTPIWQNJSPXFNHNYHSQNTGKRVKPRABQWTAXCDWOAKDKYWPTVG9BGXNVNKTLEJGESAVXIKIZ9999",
        "insertTime": 1599620082000,
        "completeTime": 1661493146000,                                                                               // represents deposit completion datetime, available for deposits after 6-Mar-2025.
        "transferType": 0,
        "confirmTimes": "1/1",
        "unlockConfirm": 0,
        "walletType": 0,
        "travelRuleStatus": 1                                                                                        // 0: travel rule not required OR info already provided and funds ready to use, 1: travel rule required to provide deposit info
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Fetch deposit address list with network(USER\_DATA)

## API Description

Fetch deposit address list with network.

## HTTP Request

GET `/sapi/v1/capital/deposit/address/list`

## Request Weight(IP)

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | YES | `coin` refers to the parent network address format that the address is using |
| network | STRING | NO |  |
| timestamp | LONG | YES |  |

> - If network is not send, return with default network of the coin.
> - You can get network and isDefault in networkList in the response of `Get /sapi/v1/capital/config/getall`.

## Response Example

```javascript
[
    {
        "coin": "ETH",                                               // coin here means network address space, ETH for all EVM-like network
        "address": "0xD316E95Fd9E8E237Cb11f8200Babbc5D8D177BA4",
        "tag": "",
        "isDefault": 0
    },
    {
        "coin": "ETH",
        "address": "0xD316E95Fd9E8E237Cb11f8200Babbc5D8D177BA4",
        "tag": "",
        "isDefault": 0
    },
    {
        "coin": "ETH",
        "address": "0x00003ada75e7da97ba0db2fcde72131f712455e2",
        "tag": "",
        "isDefault": 1                                               // 'isDefault' is 1 means the address is default, same as shown in the app.
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Fetch withdraw address list (USER\_DATA)

## API Description

Fetch withdraw address list

## HTTP Request

GET `/sapi/v1/capital/withdraw/address/list`

## Request Weight(IP)

**10**

## Request Parameters

NONE

## Response Example

```javascript
[
    {
        "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
        "addressTag": "",
        "coin": "BTC",
        "name": "Satoshi",                                   // is a user-defined name
        "network": "BTC",
        "origin": "bla",                                     // if originType != 'others', this value is blank, otherwise the origin is manually filled in by the user
        "originType": "others",                              // Address source type, including but not limited to: type Exchange Address: Binance, CoinBase, HTX, Bitfinex, OKX, Bithumb, Kraken, Kucoin, Gemini, Bitget, Bybit, Upbit, Gate.io;  type Wallet Address: Binance Web3 Wallet, Trust Wallet, MetaMask, Rabby Wallet, Phantom, OKX Web 3 Wallet, Coinbase Wallet, Bitget Wallet; type Others: others(multilanguage support)
        "whiteStatus": true                                  // Is it whitelisted
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Fetch withdraw quota (USER\_DATA)

## API Description

Fetch withdraw quota

## HTTP Request

GET `/sapi/v1/capital/withdraw/quota`

## Request Weight(IP)

**10**

## Request Parameters

NONE

## Response Example

```javascript
{
    "wdQuota": "10000",       // User's total withdrawal quota in the past 24 hours (including on-chain withdrawal and internal transfer), unit in USD
    "usedWdQuota": "1000"     // User withdrawal quota usage in the past 24 hours, unit in USD
}
```

 

Copyright © 2026 Binance.

---

Wallet

# One click arrival deposit apply (for expired address deposit) (USER\_DATA)

## API Description

Apply deposit credit for expired address (One click arrival)

## HTTP Request

POST `/sapi/v1/capital/deposit/credit-apply`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositId | LONG | NO | Deposit record Id, priority use |
| txId | STRING | NO | Deposit txId, used when depositId is not specified |
| subAccountId | LONG | NO | Sub-accountId of Cloud user |
| subUserId | LONG | NO | Sub-userId of parent user |

> - Params need to be in the POST body

## Response Example

```javascript
{
    "code": "000000",
    "message": "success",
    "data": true,
    "success": true
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw(USER\_DATA)

## API Description

Submit a withdraw request.

## HTTP Request

POST `/sapi/v1/capital/withdraw/apply`

## Request Weight(UID)

**900**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | YES |  |
| withdrawOrderId | STRING | NO | client side id for withdrawal, if provide here, can be used in GET `/sapi/v1/capital/withdraw/history` for query. |
| network | STRING | NO |  |
| address | STRING | YES |  |
| addressTag | STRING | NO | Secondary address identifier for coins like XRP,XMR etc. |
| amount | DECIMAL | YES |  |
| transactionFeeFlag | BOOLEAN | NO | When making internal transfer, `true` for returning the fee to the destination account; `false` for returning the fee back to the departure account. Default `false`. |
| name | STRING | NO | Description of the address. Address book cap is 200, space in name should be encoded into `%20` |
| walletType | INTEGER | NO | The wallet type for withdraw，0-spot wallet ，1-funding wallet. Default walletType is the current "selected wallet" under wallet->Fiat and Spot/Funding->Deposit |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If `network` not send, return with default network of the coin.
> - You can get `network` and `isDefault` in `networkList` of a coin in the response of `Get /sapi/v1/capital/config/getall (HMAC SHA256)`.
> - To check if travel rule is required, by using `GET /sapi/v1/localentity/questionnaire-requirements` and if it returns anything other than `NIL` you will need update SAPI to `POST /sapi/v1/localentity/withdraw/apply` else you can continue `POST /sapi/v1/capital/withdraw/apply`. Please note that if you are required to comply to travel rule please refer to the Travel Rule SAPI.

## Response Example

```javascript
{
    "id": "7213fea8e94b4a5593d507237e5a555b"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw History (supporting network) (USER\_DATA)

## API Description

Fetch withdraw history.

## HTTP Request

GET `/sapi/v1/capital/withdraw/history`

## Request Weight(UID)

**18000**
Request limit: 10 requests per second

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | NO |  |
| withdrawOrderId | STRING | NO | client side id for withdrawal, if provided in POST `/sapi/v1/capital/withdraw/apply`, can be used here for query. |
| status | INT | NO | 0(0:Email Sent, 2:Awaiting Approval 3:Rejected 4:Processing 6:Completed) |
| offset | INT | NO |  |
| limit | INT | NO | Default: 1000, Max: 1000 |
| idList | STRING | NO | id list returned in the response of POST `/sapi/v1/capital/withdraw/apply`, separated by `,` |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - `network` may not be in the response for old withdraw.
> - Please notice the default `startTime` and `endTime` to make sure that time interval is within 0-90 days.
> - If both `startTime` and `endTime`are sent, time between `startTime`and `endTime`must be less than 90 days.
> - If `withdrawOrderId` is sent, time between `startTime` and `endTime` must be less than 7 days.
> - If `withdrawOrderId` is sent, `startTime` and `endTime` are not sent, will return last 7 days records by default.
> - Maximum support `idList` number is 45.

## Response Example

```javascript
[
    {
        "id": "b6ae22b3aa844210a7041aee7589627c",                                         // Withdrawal id in Binance
        "amount": "8.91000000",                                                           // withdrawal amount
        "transactionFee": "0.004",                                                        // transaction fee
        "coin": "USDT",
        "status": 6,
        "address": "0x94df8b352de7f46f64b01d3666bf6e936e44ce60",
        "txId": "0xb5ef8c13b968a406cc62a93a8bd80f9e9a906ef1b3fcf20a2e48573c17659268",     // withdrawal transaction id
        "applyTime": "2019-10-12 11:12:02",                                               // UTC time
        "network": "ETH",
        "transferType": 0,                                                                // 1 for internal transfer, 0 for external transfer
        "withdrawOrderId": "WITHDRAWtest123",                                             // will not be returned if there's no withdrawOrderId for this withdraw.
        "info": "The address is not valid. Please confirm with the recipient",            // reason for withdrawal failure
        "confirmNo": 3,                                                                   // confirm times for withdraw
        "walletType": 1,                                                                  // 1: Funding Wallet 0:Spot Wallet
        "txKey": "",
        "completeTime": "2023-03-23 16:52:41"                                             // complete UTC time when user's asset is deduct from withdrawing, only if status =  6(success)
    },
    {
        "id": "156ec387f49b41df8724fa744fa82719",
        "amount": "0.00150000",
        "transactionFee": "0.004",
        "coin": "BTC",
        "status": 6,
        "address": "1FZdVHtiBqMrWdjPyRPULCUceZPJ2WLCsB",
        "txId": "60fd9007ebfddc753455f95fafa808c4302c836e4d1eebc5a132c36c1d8ac354",
        "applyTime": "2019-09-24 12:43:45",
        "network": "BTC",
        "transferType": 0,
        "info": "",
        "confirmNo": 2,
        "walletType": 1,
        "txKey": "",
        "completeTime": "2023-03-23 16:52:41"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# System Status (System)

## API Description

Fetch system status.

## HTTP Request

GET `/sapi/v1/system/status`

## Request Weight(IP)

**1**

## Response Example

```javascript
{
    "status": 0,        // 0: normal，1：system maintenance
    "msg": "normal"     // "normal", "system_maintenance"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Get symbols delist schedule for spot (MARKET\_DATA)

## API Description

Get symbols delist schedule for spot

## HTTP Request

GET `/sapi/v1/spot/delist-schedule`

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
        "symbols": ["ADAUSDT", "BNBUSDT"]
    },
    {
        "delistTime": 1686222232000,
        "symbols": ["ETHUSDT"]
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# System Status (System)

## API Description

Fetch system status.

## HTTP Request

GET `/sapi/v1/system/status`

## Request Weight(IP)

**1**

## Response Example

```javascript
{
    "status": 0,        // 0: normal，1：system maintenance
    "msg": "normal"     // "normal", "system_maintenance"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw (for local entities that require travel rule) (USER\_DATA)

## API Description

Submit a withdrawal request for local entities that required travel rule.

## HTTP Request

POST `/sapi/v1/localentity/withdraw/apply`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | YES |  |
| withdrawOrderId | STRING | NO | withdrawID defined by the client (i.e. client's internal withdrawID) |
| network | STRING | NO |  |
| address | STRING | YES |  |
| addressTag | STRING | NO | Secondary address identifier for coins like XRP,XMR etc. |
| amount | DECIMAL | YES |  |
| transactionFeeFlag | BOOLEAN | NO | When making internal transfer, `true` for returning the fee to the destination account; `false` for returning the fee back to the departure account. Default `false`. |
| name | STRING | NO | Description of the address. Address book cap is 200, space in name should be encoded into `%20` |
| walletType | INTEGER | NO | The wallet type for withdraw，0-spot wallet ，1-funding wallet. Default walletType is the current "selected wallet" under wallet->Fiat and Spot/Funding->Deposit |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |
| questionnaire | STRING | YES | JSON format questionnaire answers. |

> - If `network` not send, return with default network of the coin, but if the address could not match default network, the withdraw will be rejected.
> - You can get `network` and `isDefault` in `networkList` of a coin in the response
>   of `Get /sapi/v1/capital/config/getall (HMAC SHA256)`.
> - Questionnaire is different for each local entity, please refer to
>   the `Withdraw Questionnaire Contents` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## Response Example

```javascript
{
    "trId": 123456,                         // The travel rule record Id
    "accpted": true,                        // Whether the withdraw request is accepted
    "info": "Withdraw request accepted"     // The detailed infomation of the withdrawal result.
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Fetch address verification list (USER\_DATA)

## API Description

Fetch address verification list for user to check on status and other details for the addresses stored in Address Book.

## HTTP Request

GET `/sapi/v1/addressVerify/list`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "status": "PENDING",
        "token": "AVAX",
        "network": "AVAXC",
        "walletAddress": "0xc03a6aa728a8dde7464c33828424ede7553a0021",
        "addressQuestionnaire": {
            "sendTo": 1,
            "satoshiToken": "AVAX",
            "isAddressOwner": 1,
            "verifyMethod": 1
        }
    }
]
```

1. `status`: Refers to the status of the address verification. Response would return either of the following - Verified, Unverified, Pending.
2. `token` & `network`: Address is verified for this particular token/network withdrawals.
3. `walletAddress`: Wallet address that was added into the address book.
4. `addressQuestionaire`: Details of what you answered for the verification questionnaire.

 

Copyright © 2026 Binance.

---

Wallet

# Appendix

## Name restrictions

Strings that match the following regular expression rules are accepted.

```regexp
REGEXP : ^(?=.{1,})(?!.{100,})(?!.*([a-zA-Z])\1{2,})(?!.*[0-9!@#$%^&*()_=.~`<>/:;?€£¥₹₩¢₿+=÷]).*$
```

- The string length must be between 1 and 99 characters, inclusive.
- The string must not contain any digits or a specified set of special characters.
- The string must not contain any letter repeated consecutively 3 or more times.
- The string may contain letters (both uppercase and lowercase) and other characters that are not excluded (such as spaces, punctuation marks not listed in the excluded symbols, etc.).

## Country (Regions) ISO code

> You will not be able to use an ISO code that is not listed in the table

| Country (Regions) | Code |
| --- | --- |
| Afghanistan | af |
| Albania | al |
| Algeria | dz |
| American Samoa | as |
| Andorra | ad |
| Angola | ao |
| Anguilla | ai |
| Antigua and Barbuda | ag |
| Argentina | ar |
| Armenia | am |
| Aruba | aw |
| Australia | au |
| Austria | at |
| Azerbaijan | az |
| Bahamas (the) | bs |
| Bahrain | bh |
| Bangladesh | bd |
| Barbados | bb |
| Belarus | by |
| Belgium | be |
| Belize | bz |
| Benin | bj |
| Bermuda | bm |
| Bhutan | bt |
| Bolivia (Plurinational State of) | bo |
| Bonaire, Sint Eustatius and Saba | bq |
| Bosnia and Herzegovina | ba |
| Botswana | bw |
| Bouvet Island | bv |
| Brazil | br |
| British Indian Ocean Territory (the) | io |
| Brunei Darussalam | bn |
| Bulgaria | bg |
| Burkina Faso | bf |
| Burundi | bi |
| Cabo Verde | cv |
| Cambodia | kh |
| Cameroon | cm |
| Canada | ca |
| Cayman Islands (the) | ky |
| Central African Republic (the) | cf |
| Chad | td |
| Chile | cl |
| China | cn |
| Christmas Island | cx |
| Cocos (Keeling) Islands (the) | cc |
| Colombia | co |
| Comoros (the) | km |
| Congo (the Democratic Republic of the) | cd |
| Congo (the) | cg |
| Cook Islands (the) | ck |
| Costa Rica | cr |
| Croatia | hr |
| Cuba | cu |
| Curaçao | cw |
| Cyprus | cy |
| Czechia | cz |
| Côte d'Ivoire | ci |
| Denmark | dk |
| Djibouti | dj |
| Dominica | dm |
| Dominican Republic (the) | do |
| Ecuador | ec |
| Egypt | eg |
| El Salvador | sv |
| Equatorial Guinea | gq |
| Eritrea | er |
| Estonia | ee |
| Eswatini | sz |
| Ethiopia | et |
| Falkland Islands (the) \[Malvinas\] | fk |
| Faroe Islands (the) | fo |
| Fiji | fj |
| Finland | fi |
| France | fr |
| French Guiana | gf |
| French Polynesia | pf |
| Gabon | ga |
| Gambia (the) | gm |
| Georgia | ge |
| Germany | de |
| Ghana | gh |
| Gibraltar | gi |
| Greece | gr |
| Greenland | gl |
| Grenada | gd |
| Guadeloupe | gp |
| Guam | gu |
| Guatemala | gt |
| Guinea | gn |
| Guinea-Bissau | gw |
| Guernsey | gg |
| Guyana | gy |
| Haiti | ht |
| Heard Island and McDonald Islands | hm |
| Holy See (the) | va |
| Honduras | hn |
| Hungary | hu |
| Iceland | is |
| India | in |
| Indonesia | id |
| Iran (Islamic Republic of) | ir |
| Iraq | iq |
| Ireland | ie |
| Israel | il |
| Italy | it |
| Jamaica | jm |
| Japan | jp |
| Jersey | je |
| Jordan | jo |
| Kazakhstan | kz |
| Kenya | ke |
| Kiribati | ki |
| Korea (the Democratic People's Republic of) | kp |
| Korea (the Republic of) | kr |
| Kosovo | xk |
| Kuwait | kw |
| Kyrgyzstan | kg |
| Lao People's Democratic Republic (the) | la |
| Latvia | lv |
| Lebanon | lb |
| Lesotho | ls |
| Liberia | lr |
| Libya | ly |
| Liechtenstein | li |
| Lithuania | lt |
| Luxembourg | lu |
| Madagascar | mg |
| Malawi | mw |
| Malaysia | my |
| Maldives | mv |
| Mali | ml |
| Malta | mt |
| Marshall Islands (the) | mh |
| Martinique | mq |
| Mauritania | mr |
| Mauritius | mu |
| Mayotte | yt |
| Mexico | mx |
| Micronesia (Federated States of) | fm |
| Moldova (the Republic of) | md |
| Monaco | mc |
| Mongolia | mn |
| Montenegro | me |
| Montserrat | ms |
| Morocco | ma |
| Mozambique | mz |
| Myanmar | mm |
| Namibia | na |
| Nauru | nr |
| Nepal | np |
| Netherlands (the) | nl |
| New Caledonia | nc |
| New Zealand | nz |
| Nicaragua | ni |
| Niger (the) | ne |
| Nigeria | ng |
| Niue | nu |
| Northern Cyprus | cy-2 |
| Norfolk Island | nf |
| Northern Mariana Islands (the) | mp |
| Norway | no |
| Oman | om |
| Pakistan | pk |
| Palau | pw |
| Palestine, State of | ps |
| Panama | pa |
| Papua New Guinea | pg |
| Paraguay | py |
| Peru | pe |
| Philippines (the) | ph |
| Pitcairn | pn |
| Poland | pl |
| Portugal | pt |
| Pridnestrovian Moldavian Republic | pmr |
| Puerto Rico | pr |
| Qatar | qa |
| Republic of North Macedonia | mk |
| Romania | ro |
| Russian Federation (the) | ru |
| Rwanda | rw |
| Réunion | re |
| Saint Barthélemy | bl |
| Saint Helena, Ascension and Tristan da Cunha | sh |
| Saint Kitts and Nevis | kn |
| Saint Lucia | lc |
| Saint Martin (French part) | mf |
| Saint Pierre and Miquelon | pm |
| Saint Vincent and the Grenadines | vc |
| Samoa | ws |
| San Marino | sm |
| Sao Tome and Principe | st |
| Saudi Arabia | sa |
| Senegal | sn |
| Serbia | rs |
| Seychelles | sc |
| Sierra Leone | sl |
| Singapore | sg |
| Sint Maarten (Dutch part) | sx |
| Slovakia | sk |
| Slovenia | si |
| Solomon Islands | sb |
| Somalia | so |
| Somaliland, Republic of | so-2 |
| South Africa | za |
| South Georgia and the South Sandwich Islands | gs |
| South Ossetia | so-3 |
| South Sudan | ss |
| Spain | es |
| Sri Lanka | lk |
| Sudan (the) | sd |
| Suriname | sr |
| Svalbard and Jan Mayen | sj |
| Sweden | se |
| Switzerland | ch |
| Syrian Arab Republic | sy |
| Tajikistan | tj |
| Tanzania, United Republic of | tz |
| Thailand | th |
| Timor-Leste | tl |
| Togo | tg |
| Tokelau | tk |
| Tonga | to |
| Trinidad and Tobago | tt |
| Tunisia | tn |
| Turkey | tr |
| Turkmenistan | tm |
| Turks and Caicos Islands (the) | tc |
| Tuvalu | tv |
| Uganda | ug |
| Ukraine | ua |
| United Arab Emirates (the) | ae |
| United Kingdom of Great Britain and Northern Ireland (the) | gb |
| United States Minor Outlying Islands (the) | um |
| United States of America (the) | us |
| Uruguay | uy |
| Uzbekistan | uz |
| Vanuatu | vu |
| Venezuela (Bolivarian Republic of) | ve |
| Viet Nam | vn |
| Virgin Islands (British) | vg |
| Virgin Islands (U.S.) | vi |
| Wallis and Futuna | wf |
| Western Sahara | eh |
| Yemen | ye |
| Zambia | zm |
| Zimbabwe | zw |
| Åland Islands | ax |

 

Copyright © 2026 Binance.

---

Wallet

# Submit Deposit Questionnaire (For local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Submit questionnaire for brokers of local entities that require travel rule.
The questionnaire is only applies to transactions from un-hosted wallets or VASPs that are not
yet onboarded with GTR.

## HTTP Request

PUT `/sapi/v1/localentity/broker/deposit/provide-info`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| subAccountId | STRING | YES | External user ID. |
| depositId | STRING | YES | Wallet deposit ID. |
| questionnaire | STRING | YES | JSON format questionnaire answers. |
| beneficiaryPii | STRING | YES | JSON format beneficiary Pii. |
| network | STRING | NO |  |
| coin | STRING | NO |  |
| amount | BigDecimal | NO |  |
| address | STRING | NO |  |
| addressTag | STRING | NO |  |
| timestamp | LONG | YES | Epoch Sec |
| signature | STRING | YES | Must be the last parameter |

> - Questionnaire is different for each local entity, please refer
>   to `Deposit Questionnaire Content` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## StandardPii

**For Natural Person**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| piiType | INTEGER | YES | Fix to 0: Natural Person |
| latinNames | List | YES | In case a person have complicated names or multiple names, this parameter is a list |
| localNames | List | NO | In case a person have complicated names or multiple names, this parameter is a list |
| nationality | STRING | NO |  |
| residenceCountry | STRING | YES |  |
| nationalIdentifier | STRING | NO |  |
| nationalIdentifierType | STRING | NO |  |
| nationalIdentifierIssueCountry | STRING | NO |  |
| dateOfBirth | STRING | NO | yyyy-mm-dd. Not required but strongly recommended. Providing DOB could greatly reduce false positive rate during risk checking process. |
| placeOfBirth | STRING | NO |  |
| address | STRING | NO |  |

**For Legal Person**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| piiType | INTEGER | YES | Fix to 1: Legal Person |
| latinName | STRING | YES | It's company name for Legal Person |
| localName | STRING | NO |  |
| registrationCountry | STRING | YES |  |
| nationalIdentifier | STRING | NO |  |
| nationalIdentifierType | STRING | NO |  |
| nationalIdentifierIssueCountry | STRING | NO |  |
| registrationDate | STRING | NO | yyyy-mm-dd. Not required but strongly recommended. |
| address | STRING | NO |  |
| walletAddress | STRING | NO |  |
| walletTag | STRING | NO |  |

**PiiName**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| firstName | STRING | YES | Mandatory for Natural person |
| middleName | STRING | NO |  |
| lastName | STRING | NO |  |

## Response Example

```javascript
{
    "trId": 765127651,
    "accepted": true,
    "info": "Deposit questionnaire accepted."
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Broker Withdraw (for brokers of local entities that require travel rule) (USER\_DATA)

## API Description

Submit a withdrawal request for brokers of local entities that required travel rule.

## HTTP Request

POST `/sapi/v1/localentity/broker/withdraw/apply`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| address | STRING | YES |  |
| addressTag | STRING | NO | Secondary address identifier for coins like XRP,XMR etc. |
| network | STRING | NO |  |
| coin | STRING | YES |  |
| addressName | STRING | NO | Description of the address. Address book cap is 200, space in name should be encoded into `%20` |
| amount | BigDECIMAL | YES |  |
| withdrawOrderId | STRING | YES | withdrawID defined by the client (i.e. client's internal withdrawID) |
| transactionFeeFlag | BOOLEAN | NO | When making internal transfer, `true` for returning the fee to the destination account; `false` for returning the fee back to the departure account. Default `false`. |
| walletType | INTEGER | NO | The wallet type for withdraw，0-spot wallet ，1-funding wallet. Default walletType is the current "selected wallet" under wallet->Fiat and Spot/Funding->Deposit |
| questionnaire | STRING | YES | JSON format questionnaire answers. |
| originatorPii | STRING | YES | JSON format originator Pii, see StandardPii section below |
| timestamp | LONG | YES |  |
| signature | STRING | YES | Must be the last parameter. |

> - If `network` not send, return with default network of the coin, but if the address could not match default network, the withdraw will be rejected.
> - You can get `network` in `networkList` of a coin in the response
>   of `Get /sapi/v1/capital/config/getall (HMAC SHA256)`.
> - Questionnaire is different for each local entity, please refer to
>   the `Withdraw Questionnaire Contents` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## StandardPii

**For Natural Person**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| piiType | INTEGER | YES | Fix to 0: Natural Person |
| latinNames | List | YES | In case a person have complicated names or multiple names, this parameter is a list |
| localNames | List | NO | In case a person have complicated names or multiple names, this parameter is a list |
| nationality | STRING | NO |  |
| residenceCountry | STRING | YES |  |
| nationalIdentifier | STRING | NO |  |
| nationalIdentifierType | STRING | NO |  |
| nationalIdentifierIssueCountry | STRING | NO |  |
| dateOfBirth | STRING | NO | yyyy-mm-dd. Not required but strongly recommended. Providing DOB could greatly reduce false positive rate during risk checking process. |
| placeOfBirth | STRING | NO |  |
| address | STRING | NO |  |

**For Legal Person**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| piiType | INTEGER | YES | Fix to 1: Legal Person |
| latinName | STRING | YES | It's company name for Legal Person |
| localName | STRING | NO |  |
| registrationCountry | STRING | YES |  |
| nationalIdentifier | STRING | NO |  |
| nationalIdentifierType | STRING | NO |  |
| nationalIdentifierIssueCountry | STRING | NO |  |
| registrationDate | STRING | NO | yyyy-mm-dd. Not required but strongly recommended. |
| address | STRING | NO |  |
| walletAddress | STRING | NO |  |
| walletTag | STRING | NO |  |

**PiiName**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| firstName | STRING | YES | Mandatory for Natural person |
| middleName | STRING | NO |  |
| lastName | STRING | NO |  |

## Response Example

```javascript
{
    "trId": 123456,                         // The travel rule record Id
    "accpted": true,                        // Whether the withdraw request is accepted
    "info": "Withdraw request accepted"     // The detailed infomation of the withdrawal result.
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Deposit History (for local entities that required travel rule) (supporting network) (USER\_DATA)

## API Description

Fetch deposit history for local entities that required travel rule.

## HTTP Request

GET `/sapi/v1/localentity/deposit/history`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| trId | STRING | NO | Comma(,) separated list of travel rule record Ids. |
| txId | STRING | NO | Comma(,) separated list of transaction Ids. |
| tranId | STRING | NO | Comma(,) separated list of wallet tran Ids. |
| network | STRING | NO |  |
| coin | STRING | NO |  |
| travelRuleStatus | INTEGER | NO | 0:Completed,1:Pending,2:Failed |
| pendingQuestionnaire | BOOLEAN | NO | true: Only return records that pending deposit questionnaire. false/not provided: return all records. |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| offset | INT | NO | Default:0 |
| limit | INT | NO | Default:1000, Max:1000 |
| timestamp | LONG | YES |  |

> - Please notice the default `startTime` and `endTime` to make sure that time interval is within
>   0-90 days.
> - If both `startTime` and `endTime` are sent, time between `startTime` and `endTime` must
>   be less than 90 days.
> - Please, note that due to network-specific characteristics, the returned source address may be inaccurate. If multiple source addresses are found, only the first one will be returned.

## Response Example

```javascript
[
    {
        "trId": 123451123,
        "tranId": 17644346245865,
        "amount": "0.001",
        "coin": "BNB",
        "network": "BNB",
        "depositStatus": 0,
        "travelRuleStatus": 1,
        "address": "bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23",
        "addressTag": "101764890",
        "txId": "98A3EA560C6B3336D348B6C83F0F95ECE4F1F5919E94BD006E5BF3BF264FACFC",
        "insertTime": 1661493146000,
        "transferType": 0,
        "confirmTimes": "1/1",
        "unlockConfirm": 0,
        "walletType": 0,
        "requireQuestionnaire": false,                                                                               // true: This deposit require user to answer questionnaire to get it credited
                                                                                                                     // false: This deposit doesn't require user to answer questionnaire as it's already completed or information has been verified
        "questionnaire": null
    },
    {
        "trId": 2451123,
        "tranId": 4544346245865,
        "amount": "0.50000000",
        "coin": "IOTA",
        "network": "IOTA",
        "depositStatus": 0,
        "travelRuleStatus": 0,
        "address": "SIZ9VLMHWATXKV99LH99CIGFJFUMLEHGWVZVNNZXRJJVWBPHYWPPBOSDORZ9EQSHCZAMPVAPGFYQAUUV9DROOXJLNW",
        "addressTag": "",
        "txId": "ESBFVQUTPIWQNJSPXFNHNYHSQNTGKRVKPRABQWTAXCDWOAKDKYWPTVG9BGXNVNKTLEJGESAVXIKIZ9999",
        "insertTime": 1599620082000,
        "transferType": 0,
        "confirmTimes": "1/1",
        "unlockConfirm": 0,
        "walletType": 0,
        "requireQuestionnaire": false,
        "questionnaire": "{\'question1\':\'answer1\',\'question2\':\'answer2\'}"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Deposit History V2 (for local entities that required travel rule) (supporting network) (USER\_DATA)

## API Description

Fetch deposit history for local entities that with required travel rule information.

## HTTP Request

GET `/sapi/v2/localentity/deposit/history`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositId | STRING | NO | Comma(,) separated list of wallet tran Ids. |
| txId | STRING | NO | Comma(,) separated list of transaction Ids. |
| network | STRING | NO |  |
| coin | STRING | NO |  |
| retrieveQuestionnaire | BOOLEAN | NO | true: return `questionnaire` within response. |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| offset | INT | NO | Default:0 |
| limit | INT | NO | Default:1000, Max:1000 |
| timestamp | LONG | YES |  |

> - Please notice the default `startTime` and `endTime` to make sure that time interval is within
>   0-90 days.
> - If both `startTime` and `endTime` are sent, time between `startTime` and `endTime` must
>   be less than 90 days.
> - Please, note that due to network-specific characteristics, the returned source address may be inaccurate. If multiple source addresses are found, only the first one will be returned.

## Response Example

```javascript
[
    {
        "depositId": "4615328107052018945",
        "amount": "0.01",
        "network": "AVAXC",
        "coin": "AVAX",
        "depositStatus": 1,
        "travelRuleReqStatus": 0,                                                         // 0:PASS,2:REJECTED,3:PENDING,-1:FAILED
        "address": "0x0010627ab66d69232f4080d54e0f838b4dc3894a",
        "addressTag": "",
        "txId": "0xdde578983015741eed764e7ca10defb5a2caafdca3db5f92872d24a96beb1879",
        "transferType": 0,
        "confirmTimes": "12/12",
        "requireQuestionnaire": false,                                                    // true: This deposit require user to answer questionnaire to get it credited
                                                                                          // false: This deposit doesn't require user to answer questionnaire as it's already completed or information has been verified
        "questionnaire": {
            "vaspName": "BINANCE",
            "depositOriginator": 0
        },
        "insertTime": 1753053392000
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Submit Deposit Questionnaire (For local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Submit questionnaire for local entities that require travel rule.
The questionnaire is only applies to transactions from unhosted wallets or VASPs that are not
yet onboarded with GTR.

## HTTP Request

PUT `/sapi/v1/localentity/deposit/provide-info`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| tranId | LONG | YES | Wallet tran ID |
| questionnaire | STRING | YES | JSON format questionnaire answers. |
| timestamp | LONG | YES |  |

> - Questionnaire is different for each local entity, please refer
>   to `Deposit Questionnaire Content` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## Response Example

```javascript
{
    "trId": 765127651,
    "accepted": true,
    "info": "Deposit questionnaire accepted."
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Submit Deposit Questionnaire V2 (For local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Submit questionnaire for local entities that require travel rule.
The questionnaire is only applies to transactions from unhosted wallets or VASPs that are not
yet onboarded with GTR.

## HTTP Request

PUT `/sapi/v2/localentity/deposit/provide-info`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositId | LONG | YES | Wallet deposit ID |
| questionnaire | STRING | YES | JSON format questionnaire answers. |
| timestamp | LONG | YES |  |

> - Questionnaire is different for each local entity, please refer
>   to `Deposit Questionnaire Content` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## Response Example

```javascript
{
    "trId": 765127651,
    "accepted": true,
    "info": "Deposit questionnaire accepted."
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Deposit Questionnaire Contents (for existing local entities)

## Local Entities

## Japan

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 0:Myself, 1:Not Myself |
| bnfType | INTEGER | YES | 0:Individual, 1:Corporate/Entity |
| country | STRING | YES \*1 | Originator country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| region | STRING | YES \*2 | Originator region |
| city | STRING | YES \*1 | Originator’s city/village/town. |
| kanjiName | STRING | YES \*1 |  |
| kanaName | STRING | YES \*1 |  |
| latinName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| vaspName | STRING | YES |  |
| isAttested | BOOLEAN | YES |  |

> 1. Required when `depositOriginator` is `1`.
> 2. Required when `country` is `cn`(China) or `ua`(Ukraine).
>    \> 1\. If `country` is `cn`(China), `region` must be `notNortheasternProvinces` (Jilin, Liaoning and Heilongjiang) or `other`.
>    2\. If `country` is `ua`(Ukraine), `region` should not be `crimea`, `donetsk` or `luhansk`，should be `other`.

## Kazakhstan

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| originatorName | STRING | YES | Name of originator. |
| country | STRING | YES | Originator country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES | Originator’s city/village/town. |
| txnPurpose | STRING | YES | Value: service, goods, p2p, charity, others |
| txnPurposeOthers | STRING | YES \*1 |  |

> 1. Required when `txnPurpose` is `others`.

## Bahrain

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 1:Myself, 2:Not myself |
| orgType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| orgFirstName | STRING | YES \*1 | Originator's First Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| orgLastName | STRING | YES \*1 | Originator's Last Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*1 | Originator's residence country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES \*1 |  |
| receiveFrom | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*2 | Vasp identifier of the originator. |
| vaspName | STRING | YES \*3 |  |

> 1. Required when `depositOriginator` is `2`.
> 2. Required when `receiveFrom` is `2`.
> 3. Required when `vasp` is `others`.
> 4. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp list` and the name of the exchange within `vaspName` field.

## United Arab Emirates

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 1:Myself, 2:Not myself |
| orgType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| orgName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*1 | Originator's nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES \*1 |  |
| receiveFrom | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*2 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*3 |  |

> 1. Required when `depositOriginator` is `2`.
> 2. Required when `receiveFrom` is `2`.
> 3. Required when `vasp` is `others`.
> 4. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp list` and the name of the exchange within `vaspName` field.

## India

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 1:Myself, 2:Not myself |
| orgType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| orgName | STRING | YES \*1 | Originator's Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| pan | STRING | YES \*1 | Permanent Account Number (PAN) or National ID Number |
| country | STRING | YES \*1 | Originator's nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| state | STRING | YES \*1 | Originator's State |
| city | STRING | YES \*1 | Originator’s City/Village/Town |
| pinCode | STRING | YES \*1 |  |
| address | STRING | YES \*1 |  |
| receiveFrom | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*2 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*3 |  |

> 1. Required when `depositOriginator` is `2`.
> 2. Required when `receiveFrom` is `2`.
> 3. Required when `vasp` is `others`.
> 4. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp list` and the name of the exchange within `vaspName` field.

## EU(Poland,France)

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 1:Myself, 2:Not myself |
| orgType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| orgFirstName | STRING | YES \*2 | Originator's First Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| orgLastName | STRING | YES \*2 | Originator's Last Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*2 | Originator's nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| corpName | STRING | YES \*3 | Originator's (corporation) Name |
| corpCountry | STRING | YES \*3 | Originator's (corporation) nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| receiveFrom | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |
| declaration | BOOLEAN | YES | Declaration confirmation |

> 1. Required when `depositOriginator` is `2`.
> 2. Required when `orgType` is `0`.
> 3. Required when `orgType` is `1`.
> 4. Required when `receiveFrom` is `2`.
> 5. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp list` and the name of the exchange within `vaspName` field.

## South Africa

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| depositOriginator | INTEGER | YES | 1:Myself, 2:Not myself |
| orgType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| orgName | STRING | YES \*2 | Originator's Name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*2 | Originator's nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| corpName | STRING | YES \*3 | Originator's (corporation) Name |
| corpCountry | STRING | YES \*3 | Originator's (corporation) nationality code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| receiveFrom | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |
| declaration | BOOLEAN | YES | Declaration confirmation |

> 1. Required when `depositOriginator` is `2`.
> 2. Required when `orgType` is `0`.
> 3. Required when `orgType` is `1`.
> 4. Required when `receiveFrom` is `2`.
> 5. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp list` and the name of the exchange within `vaspName` field.

 

Copyright © 2026 Binance.

---

Wallet

# VASP list (for local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Fetch the VASP list for local entities.

## HTTP Request

GET `/sapi/v1/localentity/vasp`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "vaspCode": "BINANCE",
        "vaspName": "Binance",
        "identifier": "I1QNLP" // For populating the `vasp` field in the deposit/withdrawal questionnaire
    },
    {
        "vaspCode": "NVBH3Z_nNEHjvqbUfkaL",
        "vaspName": "HashKeyGlobal",
        "identifier": "ABC123"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Check Questionnaire Requirements (for local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

This API will return user-specific Travel Rule questionnaire requirement information in reference to the current API key.

## HTTP Request

GET `/sapi/v1/localentity/questionnaire-requirements`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "questionnaireCountryCode": "AE"
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw (for local entities that require travel rule) (USER\_DATA)

## API Description

Submit a withdrawal request for local entities that required travel rule.

## HTTP Request

POST `/sapi/v1/localentity/withdraw/apply`

## Request Weight(UID)

**600**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| coin | STRING | YES |  |
| withdrawOrderId | STRING | NO | withdrawID defined by the client (i.e. client's internal withdrawID) |
| network | STRING | NO |  |
| address | STRING | YES |  |
| addressTag | STRING | NO | Secondary address identifier for coins like XRP,XMR etc. |
| amount | DECIMAL | YES |  |
| transactionFeeFlag | BOOLEAN | NO | When making internal transfer, `true` for returning the fee to the destination account; `false` for returning the fee back to the departure account. Default `false`. |
| name | STRING | NO | Description of the address. Address book cap is 200, space in name should be encoded into `%20` |
| walletType | INTEGER | NO | The wallet type for withdraw，0-spot wallet ，1-funding wallet. Default walletType is the current "selected wallet" under wallet->Fiat and Spot/Funding->Deposit |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |
| questionnaire | STRING | YES | JSON format questionnaire answers. |

> - If `network` not send, return with default network of the coin, but if the address could not match default network, the withdraw will be rejected.
> - You can get `network` and `isDefault` in `networkList` of a coin in the response
>   of `Get /sapi/v1/capital/config/getall (HMAC SHA256)`.
> - Questionnaire is different for each local entity, please refer to
>   the `Withdraw Questionnaire Contents` page.
> - If getting error like `Questionnaire format not valid.` or `Questionnaire must not be blank`,
>   please try to verify the format of the questionnaire and use URL-encoded format.

## Response Example

```javascript
{
    "trId": 123456,                         // The travel rule record Id
    "accpted": true,                        // Whether the withdraw request is accepted
    "info": "Withdraw request accepted"     // The detailed infomation of the withdrawal result.
}
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw History (for local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Fetch withdraw history for local entities that required travel rule.

## HTTP Request

GET `/sapi/v1/localentity/withdraw/history`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| trId | STRING | NO | Comma(,) separated list of travel rule record Ids. |
| txId | STRING | NO | Comma(,) separated list of transaction Ids. |
| withdrawOrderId | STRING | NO | Comma(,) separated list of withdrawID defined by the client (i.e. client's internal withdrawID). |
| network | STRING | NO |  |
| coin | STRING | NO |  |
| travelRuleStatus | INTEGER | NO | 0:Completed,1:Pending,2:Failed |
| offset | INT | NO | Default: 0 |
| limit | INT | NO | Default: 1000, Max: 1000 |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - `network` may not be in the response for old withdraw.
> - Please notice the default `startTime` and `endTime` to make sure that time interval is within
>   0-90 days.
> - If both `startTime` and `endTime`are sent, time between `startTime`and `endTime`must be less
>   than 90 days.

## Response Example

```javascript
[
    {
        "id": "b6ae22b3aa844210a7041aee7589627c",                                         // Withdrawal id in Binance
        "trId": 1234456,                                                                  // Travel rule record id
        "amount": "8.91000000",                                                           // withdrawal amount
        "transactionFee": "0.004",                                                        // only available for sAPI requests
        "coin": "USDT",
        "withdrawalStatus": 6,                                                            // Capital withdrawal status, only available for sAPI requests
        "travelRuleStatus": 0,                                                            // Travel rule status.
        "address": "0x94df8b352de7f46f64b01d3666bf6e936e44ce60",
        "txId": "0xb5ef8c13b968a406cc62a93a8bd80f9e9a906ef1b3fcf20a2e48573c17659268",     // withdrawal transaction id
        "applyTime": "2019-10-12 11:12:02",                                               // UTC time
        "network": "ETH",
        "transferType": 0,                                                                // 1 for internal transfer, 0 for external transfer, only available for sAPI requests
        "withdrawOrderId": "WITHDRAWtest123",                                             // will not be returned if there's no withdrawOrderId for this withdraw, only available for sAPI requests
        "info": "The address is not valid. Please confirm with the recipient",            // reason for withdrawal failure, only available for sAPI requests
        "confirmNo": 3,                                                                   // confirm times for withdraw, only available for sAPI requests
        "walletType": 1,                                                                  // 1: Funding Wallet 0:Spot Wallet, only available for sAPI requests
        "txKey": "",                                                                      // only available for sAPI requests
        "questionnaire": "{\'question1\':\'answer1\',\'question2\':\'answer2\'}",         // The answers of the questionnaire
        "completeTime": "2023-03-23 16:52:41"                                             // complete UTC time when user's asset is deduct from withdrawing, only if status =  6(success)
    },
    {
        "id": "156ec387f49b41df8724fa744fa82719",
        "trId": 2231556234,
        "amount": "0.00150000",
        "transactionFee": "0.004",
        "coin": "BTC",
        "withdrawalStatus": 6,
        "travelRuleStatus": 0,
        "address": "1FZdVHtiBqMrWdjPyRPULCUceZPJ2WLCsB",
        "txId": "60fd9007ebfddc753455f95fafa808c4302c836e4d1eebc5a132c36c1d8ac354",
        "applyTime": "2019-09-24 12:43:45",
        "network": "BTC",
        "transferType": 0,
        "info": "",
        "confirmNo": 2,
        "walletType": 1,
        "txKey": "",
        "questionnaire": "{\'question1\':\'answer1\',\'question2\':\'answer2\'}",
        "completeTime": "2023-03-23 16:52:41"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw History V2 (for local entities that require travel rule) (supporting network) (USER\_DATA)

## API Description

Fetch withdraw history for local entities that required travel rule.

## HTTP Request

GET `/sapi/v2/localentity/withdraw/history`

## Request Weight(IP)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| trId | STRING | NO | Comma(,) separated list of travel rule record Ids. |
| txId | STRING | NO | Comma(,) separated list of transaction Ids. |
| withdrawOrderId | STRING | NO | Withdraw ID defined by the client (i.e. client's internal withdrawID). |
| network | STRING | NO |  |
| coin | STRING | NO |  |
| travelRuleStatus | INTEGER | NO | 0:Completed,1:Pending,2:Failed |
| offset | INT | NO | Default: 0 |
| limit | INT | NO | Default: 1000, Max: 1000 |
| startTime | LONG | NO | Default: 90 days from current timestamp |
| endTime | LONG | NO | Default: present timestamp |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - `network` may not be in the response for old withdraw.
> - Withdrawal made through /sapi/v1/capital/withdraw/apply may not be in the response.
> - Please notice the default `startTime` and `endTime` to make sure that time interval is within
>   0-90 days.
> - If both `startTime` and `endTime`are sent, time between `startTime`and `endTime`must be less
>   than 90 days.
> - If withdrawOrderId is sent, time between startTime and endTime must be less than 7 days.
> - If withdrawOrderId is sent, startTime and endTime are not sent, will return last 7 days records by default.
> - Maximum support trId,txId number is 45.
> - WithdrawOrderId only support 1.
> - If responsible does not include withdrawalStatus, please input trId or txId retrieve the data.

## Response Example

```javascript
[
    {
        "id": "b6ae22b3aa844210a7041aee7589627c",                                         // Withdrawal id in Binance
        "trId": 1234456,                                                                  // Travel rule record id
        "amount": "8.91000000",                                                           // withdrawal amount
        "transactionFee": "0.004",                                                        // only available for sAPI requests
        "coin": "USDT",
        "withdrawalStatus": 6,                                                            // Capital withdrawal status, only available for sAPI requests
        "travelRuleStatus": 0,                                                            // Travel rule status.
        "address": "0x94df8b352de7f46f64b01d3666bf6e936e44ce60",
        "txId": "0xb5ef8c13b968a406cc62a93a8bd80f9e9a906ef1b3fcf20a2e48573c17659268",     // withdrawal transaction id
        "applyTime": "2019-10-12 11:12:02",                                               // UTC time
        "network": "ETH",
        "transferType": 0,                                                                // 1 for internal transfer, 0 for external transfer, only available for sAPI requests
        "withdrawOrderId": "WITHDRAWtest123",                                             // will not be returned if there's no withdrawOrderId for this withdraw, only available for sAPI requests
        "info": "The address is not valid. Please confirm with the recipient",            // reason for withdrawal failure, only available for sAPI requests
        "confirmNo": 3,                                                                   // confirm times for withdraw, only available for sAPI requests
        "walletType": 1,                                                                  // 1: Funding Wallet 0:Spot Wallet, only available for sAPI requests
        "txKey": "",                                                                      // only available for sAPI requests
        "questionnaire": "{\'question1\':\'answer1\',\'question2\':\'answer2\'}",         // The answers of the questionnaire
        "completeTime": "2023-03-23 16:52:41"                                             // complete UTC time when user's asset is deduct from withdrawing, only if status =  6(success)
    },
    {
        "id": "156ec387f49b41df8724fa744fa82719",
        "trId": 2231556234,
        "amount": "0.00150000",
        "transactionFee": "0.004",
        "coin": "BTC",
        "withdrawalStatus": 6,
        "travelRuleStatus": 0,
        "address": "1FZdVHtiBqMrWdjPyRPULCUceZPJ2WLCsB",
        "txId": "60fd9007ebfddc753455f95fafa808c4302c836e4d1eebc5a132c36c1d8ac354",
        "applyTime": "2019-09-24 12:43:45",
        "network": "BTC",
        "transferType": 0,
        "info": "",
        "confirmNo": 2,
        "walletType": 1,
        "txKey": "",
        "questionnaire": "{\'question1\':\'answer1\',\'question2\':\'answer2\'}",
        "completeTime": "2023-03-23 16:52:41"
    }
]
```

 

Copyright © 2026 Binance.

---

Wallet

# Withdraw Questionnaire Contents (for existing local entities)

## Local Entities

> Please refer to `Check Questionnaire Requirements` if you are unsure of which questionnaire content to be used.

## Japan

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary. |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| kanjiName | STRING | YES \*1 |  |
| kanaName | STRING | YES \*1 |  |
| latinName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES |  |
| sendTo | INTEGER | YES | 1:Crypto-asset services provider, 2:Unhosted Wallet |
| vasp | STRING | YES \*2 | Vasp identifier of the beneficiary |
| vaspCountry | STRING | YES \*2 | VASP country code, ISO 2 digit, lower case. |
| vaspRegion | STRING | YES \*3 |  |
| txnPurpose | INTEGER | YES \*4 | 1:Purchase of goods within Japan, 2:Inheritance, gift or living expenses, 3:Cross border trade, 4:Investment, 5:Use of services provided by the beneficiary VASP, 6:Loan repayment, 7:Gifts & Donations |
| isAttested | BOOLEAN | YES |  |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `sendTo` is `1`.
> 3. Required when `vaspCountry` is `cn`(China) or `ua`(Ukraine).
>
>    1. If `vaspCountry` is `cn`(China), `vaspRegion` must be `notNortheasternProvinces`
>       (Jilin, Liaoning and Heilongjiang) or `other`.
>    2. If `vaspCountry` is `ua`(Ukraine), `vaspRegion` should not be `crimea`, `donetsk`
>       or `luhansk`, should be `other`.
> 4. Required when `txnPurpose` is `others`.
> 5. If `txnPurpose` is `3`, withdrawals will be rejected as transactions for payment for import and/or intermediate trade are prohibited for Binance Japan.
> 6. The `Vasp List` API provides the VASP and identifier information.

## Kazakhstan

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | BOOLEAN | YES | Whether the address is owned by the user. |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| beneficiaryName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| beneficiaryCountry | STRING | YES | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| beneficiaryCity | STRING | YES |  |
| txnPurpose | STRING | YES | Value: service, goods, p2p, charity, others |
| txnPurposeOthers | STRING | YES \*4 |  |
| sendTo | INTEGER | YES | 2:Exchange, 1:Unhosted Wallet |
| vasp | STRING | YES \*2 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*3 | VASP Name |
| isAttested | BOOLEAN | YES |  |

> 1. Required when `isAddressOwner` is `false`.
> 2. Required when `sendTo` is `2`.
> 3. Required when `vasp` is `others`.
> 4. Required when `txnPurpose` is `others`.
> 5. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## New Zealand

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfName | STRING | YES \*2 | Individual beneficiary name, for more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*2 | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| bnfCorpName | STRING | YES \*3 | Beneficiary corporation name. |
| bnfCorpCountry | STRING | YES \*3 | Beneficiary corporation country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | VASP identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |
| declaration | BOOLEAN | YES | Declaration confirmation |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `bnfType` is `0`(Individual).
> 3. Required when `bnfType` is `1`(Corporate/Entity).
> 4. Required when `sendTo` is `2`.
> 5. Required when `vasp` is `others`.
> 6. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## Bahrain

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfFirstName | STRING | YES \*2 | Individual beneficiary first name. For more information please refer to the `Name restrictions` section in the `appendix`. |
| bnfLastName | STRING | YES \*2 | Individual beneficiary last name. For more information please refer to the `Name restrictions` section in the `appendix`. |
| bnfName | STRING | YES \*3 | Beneficiary corporation/entity name. |
| country | STRING | YES \*1 | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES \*1 |  |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `bnfType` is `0`(Individual).
> 3. Required when `bnfType` is `1`(Corporate/Entity).
> 4. Required when `sendTo` is `2`.
> 5. Required when `vasp` is `others`.
> 6. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## UAE

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*1 | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | YES \*1 |  |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*2 | Vasp identifier of the beneficiary |
| vaspName | STRING | YES \*3 | VASP Name |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `sendTo` is `2`.
> 3. Required when `vasp` is `others`.
> 4. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## India

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfName | STRING | YES \*1 | For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*1 | Beneficiary country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| city | STRING | NO |  |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*2 | VASP identifier of the beneficiary |
| vaspName | STRING | YES \*3 | VASP Name |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `sendTo` is `2`.
> 3. Required when `vasp` is `others`.
> 4. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## EU(Poland,France)

For all the EU countries, please following the same questionnaire.

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfFirstName | STRING | YES \*2 | Individual beneficiary first name. For more information please refer to the `Name restrictions` section in the `appendix`. |
| bnfLastName | STRING | YES \*2 | Individual beneficiary last name. For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*2 | Beneficiary country code, ISO 2 digit, lower case. |
| bnfCorpName | STRING | YES \*3 | Beneficiary corporation name. |
| bnfCorpCountry | STRING | YES \*3 | Beneficiary corporation country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | VASP identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |
| declaration | BOOLEAN | YES | Declaration confirmation |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `bnfType` is `0`.
> 3. Required when `bnfType` is `1`.
> 4. Required when `sendTo` is `2`.
> 5. Required when `vasp` is `others`.
> 6. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

## South Africa

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| isAddressOwner | INTEGER | YES | 1:Send to myself, 2:Send to another beneficiary |
| bnfType | INTEGER | YES \*1 | 0:Individual, 1:Corporate/Entity |
| bnfName | STRING | YES \*2 | Individual beneficiary name. For more information please refer to the `Name restrictions` section in the `appendix`. |
| country | STRING | YES \*2 | Beneficiary country code, ISO 2 digit, lower case. |
| bnfCorpName | STRING | YES \*3 | Beneficiary corporation name. |
| bnfCorpCountry | STRING | YES \*3 | Beneficiary corporation country code, ISO 2 digit, lower case. For more information please refer to the `Countries and Regions` section in the `appendix`. |
| sendTo | INTEGER | YES | 1:Private Wallet, 2:Another VASP |
| vasp | STRING | YES \*4 | VASP identifier of the beneficiary |
| vaspName | STRING | YES \*5 | VASP Name |
| declaration | BOOLEAN | YES | Declaration confirmation |

> 1. Required when `isAddressOwner` is `2`.
> 2. Required when `bnfType` is `0`(Individual).
> 3. Required when `bnfType` is `1`(Corporate/Entity).
> 4. Required when `sendTo` is `2`.
> 5. Required when `vasp` is `others`.
> 6. The `Vasp List` API provides the VASP and identifier information. If the VASP cannot be found, please input `others` within `vasp` and the name of the exchange within `vaspName` field.

 

Copyright © 2026 Binance.

---

