# Binance USDT-M Futures API Documentation

Source: https://developers.binance.com/docs/

---

## Repo Usage Quick Reference

- Primary repo use: perp-perp trading and the futures leg of spot-futures
- Base URL in this repo: `https://fapi.binance.com`
- Repo symbol format: `BTCUSDT`
- Most relevant endpoints for this repo:
  - exchange info and contract metadata
  - order placement / cancel / query
  - funding rate and funding info
  - balances, positions, leverage, margin mode
- Important repo note: this repo currently uses classic `fapi` endpoints, not portfolio margin routing

Derivatives Trading

# General Info

## General API Information

- Some endpoints will require an API Key. Please refer to [this page](https://www.binance.com/en/support/articles/360002502072)
- The base endpoint is: **[https://fapi.binance.com](https://fapi.binance.com/)**
- All endpoints return either a JSON object or array.
- Data is returned in **ascending** order. Oldest first, newest last.
- All time and timestamp related fields are in milliseconds.
- All data types adopt definition in JAVA.

### Testnet API Information

- Most of the endpoints can be used in the testnet platform.
- The REST base url for **testnet** is " [https://demo-fapi.binance.com](https://demo-fapi.binance.com/)"
- The Websocket base url for **testnet** is "wss://fstream.binancefuture.com"

* * *

## General Information on Endpoints

- For `GET` endpoints, parameters must be sent as a `query string`.
- For `POST`, `PUT`, and `DELETE` endpoints, the parameters may be sent as a
`query string` or in the `request body` with content type
`application/x-www-form-urlencoded`. You may mix parameters between both the
`query string` and `request body` if you wish to do so.
- Parameters may be sent in any order.
- If a parameter sent in both the `query string` and `request body`, the
`query string` parameter will be used.

### HTTP Return Codes

- HTTP `4XX` return codes are used for for malformed requests;
the issue is on the sender's side.
- HTTP `403` return code is used when the WAF Limit (Web Application Firewall) has been violated.
- HTTP `408` return code is used when a timeout has occurred while waiting for a response from the backend server.
- HTTP `429` return code is used when breaking a request rate limit.
- HTTP `418` return code is used when an IP has been auto-banned for continuing to send requests after receiving `429` codes.
- HTTP `5XX` return codes are used for internal errors; the issue is on
Binance's side.

1. If there is an error message **"Request occur unknown error."**, please retry later.
- HTTP `503` return code is used when:

1. If there is an error message **"Unknown error, please check your request or try again later."** returned in the response, the API successfully sent the request but not get a response within the timeout period.

     It is important to **NOT** treat this as a failure operation; the execution status is **UNKNOWN** and could have been a success;
2. If there is an error message **"Service Unavailable."** returned in the response, it means this is a failure API operation and the service might be unavailable at the moment, you need to retry later.
3. If there is an error message **"Internal error; unable to process your request. Please try again."** returned in the response, it means this is a failure API operation and you can resend your request if you need.
4. If the response contains the error message **"Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again." (-1008)**, This indicates the node has exceeded its maximum concurrency and is temporarily throttled. Close-position, reduce-only, and cancel orders are exempt and will not receive this error.

### HTTP 503 Status: Message Variants & Handling

#### A. “Unknown error, please check your request or try again later.” (Execution status **unknown**)

- **Meaning**: Request accepted but no response before timeout; **execution may have succeeded**.
- **Handling**:

  - **Do not treat as immediate failure**; first verify via **WebSocket updates** or **orderId queries** to avoid duplicates.
  - During peaks, prefer **single orders** over batch to reduce uncertainty.
- **Rate-limit counting**: **May or may not** count, check header to verify rate limit info

#### B. “Service Unavailable.” (Failure)

- **Meaning**: Service temporarily unavailable; **100% failure**.
- **Handling**: **Retry with exponential backoff** (e.g., 200ms → 400ms → 800ms, max 3–5 attempts).
- **Rate-limit counting**: **not counted**

#### C. “Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again.” ( **-1008**, Failure)

- **Meaning**: System overload; **100% failure**.
- **Handling**: **Retry with backoff** and **reduce concurrency**;
- **Applicable endpoints**:

  - `POST /fapi/v1/order`
  - `POST /fapi/v1/batchOrders`
  - `POST /fapi/v1/order/test`
- **Rate-limit counting**: **Not counted** (overload protection).
- **Exception integrated here**: When a request **reduces exposure** (Reduce-only / Close-position: `closePosition = true`, or `positionSide = BOTH` with `reduceOnly = true`, or `LONG+SELL`, or `SHORT+BUY`), it is **not affected or prioritized under -1008** to ensure risk reduction.

  - Covered endpoints: `POST /fapi/v1/order`、`POST /fapi/v1/batchOrders` (when parameters satisfy the condition)

### Error Codes and Messages

- Any endpoint can return an ERROR

> **_The error payload is as follows:_**

```javascript
{
  "code": -1121,
  "msg": "Invalid symbol."
}
```

- Specific error codes and messages defined in [Error Codes](https://developers.binance.com/docs/derivatives/usds-margined-futures#error-codes).

* * *

## SDK and Code Demonstration

**Disclaimer:**

- The following SDKs are provided by partners and users, and are **not officially** produced. They are only used to help users become familiar with the API endpoint. Please use it with caution and expand R&D according to your own situation.
- Binance does not make any commitment to the safety and performance of the SDKs, nor will be liable for the risks or even losses caused by using the SDKs.

### Python3

**SDK:**
To get the provided SDK for Binance Futures Connector,
please visit [https://github.com/binance/binance-connector-python](https://github.com/binance/binance-connector-python),
or use the command below:
`pip install binance-sdk-derivatives-trading-usds-futures`

### Java

To get the provided SDK for Binance Futures,
please visit [https://github.com/binance/binance-connector-java](https://github.com/binance/binance-connector-java),
or use the command below:
`git clone https://github.com/binance/binance-connector-java.git`

* * *

## LIMITS

- The `/fapi/v1/exchangeInfo``rateLimits` array contains objects related to the exchange's `RAW_REQUEST`, `REQUEST_WEIGHT`, and `ORDER` rate limits. These are further defined in the `ENUM definitions` section under `Rate limiters (rateLimitType)`.
- A `429` will be returned when either rate limit is violated.

Binance has the right to further tighten the rate limits on users with intent to attack.

### IP Limits

- Every request will contain `X-MBX-USED-WEIGHT-(intervalNum)(intervalLetter)` in the response headers which has the current used weight for the IP for all request rate limiters defined.
- Each route has a `weight` which determines for the number of requests each endpoint counts for. Heavier endpoints and endpoints that do operations on multiple symbols will have a heavier `weight`.
- When a 429 is received, it's your obligation as an API to back off and not spam the API.
- **Repeatedly violating rate limits and/or failing to back off after receiving 429s will result in an automated IP ban (HTTP status 418).**
- IP bans are tracked and **scale in duration** for repeat offenders, **from 2 minutes to 3 days**.
- **The limits on the API are based on the IPs, not the API keys.**

It is strongly recommended to use websocket stream for getting data as much as possible, which can not only ensure the timeliness of the message, but also reduce the access restriction pressure caused by the request.

### Order Rate Limits

- Every order response will contain a `X-MBX-ORDER-COUNT-(intervalNum)(intervalLetter)` header which has the current order count for the account for all order rate limiters defined.
- Rejected/unsuccessful orders are not guaranteed to have `X-MBX-ORDER-COUNT-**` headers in the response.
- **The order rate limit is counted against each account**.

* * *

## Endpoint Security Type

- Each endpoint has a security type that determines the how you will
interact with it.
- API-keys are passed into the Rest API via the `X-MBX-APIKEY`
header.
- API-keys and secret-keys **are case sensitive**.
- API-keys can be configured to only access certain types of secure endpoints.
For example, one API-key could be used for TRADE only, while another API-key
can access everything except for TRADE routes.
- By default, API-keys can access all secure routes.

| Security Type | Description |
| --- | --- |
| NONE | Endpoint can be accessed freely. |
| TRADE | Endpoint requires sending a valid API-Key and signature. |
| USER\_DATA | Endpoint requires sending a valid API-Key and signature. |
| USER\_STREAM | Endpoint requires sending a valid API-Key. |
| MARKET\_DATA | Endpoint requires sending a valid API-Key. |

- `TRADE` and `USER_DATA` endpoints are `SIGNED` endpoints.

### SIGNED (TRADE and USER\_DATA) Endpoint Security

- `SIGNED` endpoints require an additional parameter, `signature`, to be
sent in the `query string` or `request body`.
- Endpoints use `HMAC SHA256` signatures. The `HMAC SHA256 signature` is a keyed `HMAC SHA256` operation.
Use your `secretKey` as the key and `totalParams` as the value for the HMAC operation.
- The `signature` is **not case sensitive**.
- Please make sure the `signature` is the end part of your `query string` or `request body`.
- `totalParams` is defined as the `query string` concatenated with the
`request body`.

### Timing Security

- A `SIGNED` endpoint also requires a parameter, `timestamp`, to be sent which
should be the millisecond timestamp of when the request was created and sent.
- An additional parameter, `recvWindow`, may be sent to specify the number of
milliseconds after `timestamp` the request is valid for. If `recvWindow`
is not sent, **it defaults to 5000**.

> The logic is as follows:

```javascript
if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow) {
  // process request
} else {
  // reject request
}
```

**Serious trading is about timing.** Networks can be unstable and unreliable,
which can lead to requests taking varying amounts of time to reach the
servers. With `recvWindow`, you can specify that the request must be
processed within a certain number of milliseconds or be rejected by the
server.

It is recommended to use a small recvWindow of 5000 or less!

### SIGNED Endpoint Examples for POST /fapi/v1/order - HMAC Keys

Here is a step-by-step example of how to send a vaild signed payload from the
Linux command line using `echo`, `openssl`, and `curl`.

| Key | Value |
| --- | --- |
| apiKey | dbefbc809e3e83c283a984c3a1459732ea7db1360ca80c5c2c8867408d28cc83 |
| secretKey | 2b5eb11e18796d12d88f13dc27dbbd02c2cc51ff7059765ed9821957d82bb4d9 |

| Parameter | Value |
| --- | --- |
| symbol | BTCUSDT |
| side | BUY |
| type | LIMIT |
| timeInForce | GTC |
| quantity | 1 |
| price | 9000 |
| recvWindow | 5000 |
| timestamp | 1591702613943 |

#### Example 1: As a query string

> **Example 1**

> **HMAC SHA256 signature:**

```shell
    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&quantity=1&price=9000&timeInForce=GTC&recvWindow=5000&timestamp=1591702613943" | openssl dgst -sha256 -hmac "2b5eb11e18796d12d88f13dc27dbbd02c2cc51ff7059765ed9821957d82bb4d9"
    (stdin)= 3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9
```

> **curl command:**

```shell
    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: dbefbc809e3e83c283a984c3a1459732ea7db1360ca80c5c2c8867408d28cc83" -X POST 'https://fapi/binance.com/fapi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&quantity=1&price=9000&timeInForce=GTC&recvWindow=5000&timestamp=1591702613943&signature= 3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9'
```

- **queryString:**

symbol=BTCUSDT

&side=BUY

&type=LIMIT

&timeInForce=GTC

&quantity=1

&price=9000

&recvWindow=5000

&timestamp=1591702613943

#### Example 2: As a request body

> **Example 2**

> **HMAC SHA256 signature:**

```shell
    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&quantity=1&price=9000&timeInForce=GTC&recvWindow=5000&timestamp=1591702613943" | openssl dgst -sha256 -hmac "2b5eb11e18796d12d88f13dc27dbbd02c2cc51ff7059765ed9821957d82bb4d9"
    (stdin)= 3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9
```

> **curl command:**

```shell
    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: dbefbc809e3e83c283a984c3a1459732ea7db1360ca80c5c2c8867408d28cc83" -X POST 'https://fapi/binance.com/fapi/v1/order' -d 'symbol=BTCUSDT&side=BUY&type=LIMIT&quantity=1&price=9000&timeInForce=GTC&recvWindow=5000&timestamp=1591702613943&signature= 3c661234138461fcc7a7d8746c6558c9842d4e10870d2ecbedf7777cad694af9'
```

- **requestBody:**

symbol=BTCUSDT

&side=BUY

&type=LIMIT

&timeInForce=GTC

&quantity=1

&price=9000

&recvWindow=5000

&timestamp=1591702613943

#### Example 3: Mixed query string and request body

> **Example 3**

> **HMAC SHA256 signature:**

```shell
    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTCquantity=1&price=9000&recvWindow=5000&timestamp= 1591702613943" | openssl dgst -sha256 -hmac "2b5eb11e18796d12d88f13dc27dbbd02c2cc51ff7059765ed9821957d82bb4d9"
    (stdin)= f9d0ae5e813ef6ccf15c2b5a434047a0181cb5a342b903b367ca6d27a66e36f2
```

> **curl command:**

```shell
    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: dbefbc809e3e83c283a984c3a1459732ea7db1360ca80c5c2c8867408d28cc83" -X POST 'https://fapi.binance.com/fapi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC' -d 'quantity=1&price=9000&recvWindow=5000&timestamp=1591702613943&signature=f9d0ae5e813ef6ccf15c2b5a434047a0181cb5a342b903b367ca6d27a66e36f2'
```

- **queryString:** symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC
- **requestBody:** quantity=1&price=9000&recvWindow=5000&timestamp= 1591702613943

Note that the signature is different in example 3.

There is no & between "GTC" and "quantity=1".

### SIGNED Endpoint Examples for POST /fapi/v1/order - RSA Keys

- This will be a step by step process how to create the signature payload to send a valid signed payload.
- We support `PKCS#8` currently.
- To get your API key, you need to upload your RSA Public Key to your account and a corresponding API key will be provided for you.

For this example, the private key will be referenced as `test-prv-key.pem`

| Key | Value |
| --- | --- |
| apiKey | vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2 |

| Parameter | Value |
| --- | --- |
| symbol | BTCUSDT |
| side | SELL |
| type | MARKET |
| quantity | 1.23 |
| recvWindow | 9999999 |
| timestamp | 1671090801999 |

> **Signature payload (with the listed parameters):**

```console
timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23
```

**Step 1: Construct the payload**

Arrange the list of parameters into a string. Separate each parameter with a `&`.

**Step 2: Compute the signature:**

2.1 - Encode signature payload as ASCII data.

> **Step 2.2**

```console
 $ echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem
```

2.2 - Sign payload using RSASSA-PKCS1-v1\_5 algorithm with SHA-256 hash function.

> **Step 2.3**

```console
$ echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem | openssl enc -base64
aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D
```

2.3 - Encode output as base64 string.

> **Step 2.4**

```console
$  echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem | openssl enc -base64 | tr -d '\n'
aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D
```

2.4 - Delete any newlines in the signature.

> **Step 2.5**

```console
aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D
```

2.5 - Since the signature may contain `/` and `=`, this could cause issues with sending the request. So the signature has to be URL encoded.

> **Step 2.6**

```console
 curl -H "X-MBX-APIKEY: vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2" -X POST 'https://fapi.binance.com/fapi/v1/order?timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23&signature=aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D'
```

2.6 - curl command

> **Bash script**

```bash
#!/usr/bin/env bash

# Set up authentication:
apiKey="vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2"   ### REPLACE THIS WITH YOUR API KEY

# Set up the request:
apiMethod="POST"
apiCall="v1/order"
apiParams="timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23"
function rawurlencode {
    local value="$1"
    local len=${#value}
    local encoded=""
    local pos c o
    for (( pos=0 ; pos<len ; pos++ ))
    do
        c=${value:$pos:1}
        case "$c" in
            [-_.~a-zA-Z0-9] ) o="${c}" ;;
            * )   printf -v o '%%%02x' "'$c"
        esac
        encoded+="$o"
    done
    echo "$encoded"
}
ts=$(date +%s000)
paramsWithTs="$apiParams&timestamp=$ts"
rawSignature=$(echo -n "$paramsWithTs" 
               | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem \  ### THIS IS YOUR PRIVATE KEY. DO NOT SHARE THIS FILE WITH ANYONE.
               | openssl enc -base64 
               | tr -d '\n')
signature=$(rawurlencode "$rawSignature")
curl -H "X-MBX-APIKEY: $apiKey" -X $apiMethod 
    "https://fapi.binance.com/fapi/$apiCall?$paramsWithTs&signature=$signature"
```

A sample Bash script containing similar steps is available in the right side.

* * *

## Postman Collections

There is now a Postman collection containing the API endpoints for quick and easy use.

For more information please refer to this page: [Binance API Postman](https://github.com/binance-exchange/binance-api-postman)

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# New Future Account Transfer

Please find details from [here](https://developers.binance.com/docs/wallet/asset/user-universal-transfer).

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Account Configuration(USER\_DATA)

## API Description

Query account configuration

## HTTP Request

GET `/fapi/v1/accountConfig`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "feeTier": 0,               // account commission tier
    "canTrade": true,           // if can trade
    "canDeposit": true,         // if can transfer in asset
    "canWithdraw": true,        // if can transfer out asset
    "dualSidePosition": true,
    "updateTime": 0,            // reserved property, please ignore
    "multiAssetsMargin": false,
    "tradeGroupId": -1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Account Information V2(USER\_DATA)

## API Description

Get current account information. User in single-asset/ multi-assets mode will see different value, see comments in response section for detail.

## HTTP Request

GET `/fapi/v2/account`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> single-asset mode

```javascript
{
	"feeTier": 0,  		// account commission tier
	"feeBurn": true,  	// "true": Fee Discount On; "false": Fee Discount Off	"canTrade": true,  	// if can trade
	"canDeposit": true,  	// if can transfer in asset
	"canWithdraw": true, 	// if can transfer out asset
	"updateTime": 0,        // reserved property, please ignore
	"multiAssetsMargin": false,
	"tradeGroupId": -1,
	"totalInitialMargin": "0.00000000",    // total initial margin required with current mark price (useless with isolated positions), only for USDT asset
	"totalMaintMargin": "0.00000000",  	  // total maintenance margin required, only for USDT asset
	"totalWalletBalance": "23.72469206",     // total wallet balance, only for USDT asset
	"totalUnrealizedProfit": "0.00000000",   // total unrealized profit, only for USDT asset
	"totalMarginBalance": "23.72469206",     // total margin balance, only for USDT asset
	"totalPositionInitialMargin": "0.00000000",    // initial margin required for positions with current mark price, only for USDT asset
	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price, only for USDT asset
	"totalCrossWalletBalance": "23.72469206",      // crossed wallet balance, only for USDT asset
	"totalCrossUnPnl": "0.00000000",	  // unrealized profit of crossed positions, only for USDT asset
	"availableBalance": "23.72469206",       // available balance, only for USDT asset
	"maxWithdrawAmount": "23.72469206"     // maximum amount for transfer out, only for USDT asset
	"assets": [
		{
			"asset": "USDT",			// asset name
			"walletBalance": "23.72469206",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "23.72469206",      // margin balance
			"maintMargin": "0.00000000",	    // maintenance margin required
			"initialMargin": "0.00000000",    // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "23.72469206",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "23.72469206",       // available balance
			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
			"updateTime": 1625474304765 // last update time
		},
		{
			"asset": "BUSD",			// asset name
			"walletBalance": "103.12345678",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "103.12345678",      // margin balance
			"maintMargin": "0.00000000",	    // maintenance margin required
			"initialMargin": "0.00000000",    // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "103.12345678",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "103.12345678",       // available balance
			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
			"updateTime": 1625474304765 // last update time
		}
	],
	"positions": [  // positions of all symbols in the market are returned
		// only "BOTH" positions will be returned with One-way mode
		// only "LONG" and "SHORT" positions will be returned with Hedge mode
		{
			"symbol": "BTCUSDT",  	// symbol name
			"initialMargin": "0",	// initial margin required with current mark price
			"maintMargin": "0",		// maintenance margin required
			"unrealizedProfit": "0.00000000",  // unrealized profit
			"positionInitialMargin": "0",      // initial margin required for positions with current mark price
			"openOrderInitialMargin": "0",     // initial margin required for open orders with current mark price
			"leverage": "100",		// current initial leverage
			"isolated": true,  		// if the position is isolated
			"entryPrice": "0.00000",  	// average entry price
			"maxNotional": "250000",  	// maximum available notional with current leverage
			"bidNotional": "0",  // bids notional, ignore
			"askNotional": "0",  // ask notional, ignore
			"positionSide": "BOTH",  	// position side
			"positionAmt": "0",			// position amount
			"updateTime": 0           // last update time
		}
	]
}
```

> OR multi-assets mode

```javascript
{
	"feeTier": 0,  		// account commission tier
	"feeBurn": true,  	// "true": Fee Discount On; "false": Fee Discount Off	"canTrade": true,  	// if can trade
	"canTrade": true,  	// if can trade
	"canDeposit": true,  	// if can transfer in asset
	"canWithdraw": true, 	// if can transfer out asset
	"updateTime": 0,        // reserved property, please ignore
	"multiAssetsMargin": true,
	"tradeGroupId": -1,
	"totalInitialMargin": "0.00000000",    // the sum of USD value of all cross positions/open order initial margin
	"totalMaintMargin": "0.00000000",  	  // the sum of USD value of all cross positions maintenance margin
	"totalWalletBalance": "126.72469206",     // total wallet balance in USD
	"totalUnrealizedProfit": "0.00000000",   // total unrealized profit in USD
	"totalMarginBalance": "126.72469206",     // total margin balance in USD
	"totalPositionInitialMargin": "0.00000000",    // the sum of USD value of all cross positions initial margin
	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price in USD
	"totalCrossWalletBalance": "126.72469206",      // crossed wallet balance in USD
	"totalCrossUnPnl": "0.00000000",	  // unrealized profit of crossed positions in USD
	"availableBalance": "126.72469206",       // available balance in USD
	"maxWithdrawAmount": "126.72469206"     // maximum virtual amount for transfer out in USD
	"assets": [
		{
			"asset": "USDT",			// asset name
			"walletBalance": "23.72469206",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "23.72469206",      // margin balance
			"maintMargin": "0.00000000",	    // maintenance margin required
			"initialMargin": "0.00000000",    // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "23.72469206",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "126.72469206",       // available balance
			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
			"updateTime": 1625474304765 // last update time
		},
		{
			"asset": "BUSD",			// asset name
			"walletBalance": "103.12345678",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "103.12345678",      // margin balance
			"maintMargin": "0.00000000",	    // maintenance margin required
			"initialMargin": "0.00000000",    // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "103.12345678",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "126.72469206",       // available balance
			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
			"updateTime": 1625474304765 // last update time
		}
	],
	"positions": [  // positions of all symbols in the market are returned
		// only "BOTH" positions will be returned with One-way mode
		// only "LONG" and "SHORT" positions will be returned with Hedge mode
		{
			"symbol": "BTCUSDT",  	// symbol name
			"initialMargin": "0",	// initial margin required with current mark price
			"maintMargin": "0",		// maintenance margin required
			"unrealizedProfit": "0.00000000",  // unrealized profit
			"positionInitialMargin": "0",      // initial margin required for positions with current mark price
			"openOrderInitialMargin": "0",     // initial margin required for open orders with current mark price
			"leverage": "100",		// current initial leverage
			"isolated": true,  		// if the position is isolated
			"entryPrice": "0.00000",  	// average entry price
			"maxNotional": "250000",  	// maximum available notional with current leverage
			"bidNotional": "0",  // bids notional, ignore
			"askNotional": "0",  // ask notional, ignore
			"positionSide": "BOTH",  	// position side
			"positionAmt": "0",			// position amount
			"updateTime": 0           // last update time
		}
	]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Account Information V3(USER\_DATA)

## API Description

Get current account information. User in single-asset/ multi-assets mode will see different value, see comments in response section for detail.

## HTTP Request

GET `/fapi/v3/account`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> single-asset mode

```javascript
{
	"totalInitialMargin": "0.00000000",            // total initial margin required with current mark price (useless with isolated positions), only for USDT asset
	"totalMaintMargin": "0.00000000",  	           // total maintenance margin required, only for USDT asset
	"totalWalletBalance": "103.12345678",           // total wallet balance, only for USDT asset
	"totalUnrealizedProfit": "0.00000000",         // total unrealized profit, only for USDT asset
	"totalMarginBalance": "103.12345678",           // total margin balance, only for USDT asset
	"totalPositionInitialMargin": "0.00000000",    // initial margin required for positions with current mark price, only for USDT asset
	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price, only for USDT asset
	"totalCrossWalletBalance": "103.12345678",      // crossed wallet balance, only for USDT asset
	"totalCrossUnPnl": "0.00000000",	           // unrealized profit of crossed positions, only for USDT asset
	"availableBalance": "103.12345678",             // available balance, only for USDT asset
	"maxWithdrawAmount": "103.12345678"             // maximum amount for transfer out, only for USDT asset
	"assets": [ // For assets that are quote assets, USDT/USDC/BTC
		{
			"asset": "USDT",			            // asset name
			"walletBalance": "23.72469206",         // wallet balance
			"unrealizedProfit": "0.00000000",       // unrealized profit
			"marginBalance": "23.72469206",         // margin balance
			"maintMargin": "0.00000000",	        // maintenance margin required
			"initialMargin": "0.00000000",          // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",  // initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000", // initial margin required for open orders with current mark price
			"crossWalletBalance": "23.72469206",    // crossed wallet balance
			"crossUnPnl": "0.00000000"              // unrealized profit of crossed positions
			"availableBalance": "23.72469206",      // available balance
			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
			"updateTime": 1625474304765             // last update time
		},
 		{
			"asset": "USDC",			            // asset name
			"walletBalance": "103.12345678",         // wallet balance
			"unrealizedProfit": "0.00000000",       // unrealized profit
			"marginBalance": "103.12345678",         // margin balance
			"maintMargin": "0.00000000",	        // maintenance margin required
			"initialMargin": "0.00000000",          // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",  // initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000", // initial margin required for open orders with current mark price
			"crossWalletBalance": "103.12345678",    // crossed wallet balance
			"crossUnPnl": "0.00000000"              // unrealized profit of crossed positions
			"availableBalance": "126.72469206",      // available balance
			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
			"updateTime": 1625474304765             // last update time
		},
    ],
	"positions": [  // positions of all symbols user had position/ open orders are returned
		            // only "BOTH" positions will be returned with One-way mode
		            // only "LONG" and "SHORT" positions will be returned with Hedge mode
   	  {
           "symbol": "BTCUSDT",
           "positionSide": "BOTH",            // position side
           "positionAmt": "1.000",
           "unrealizedProfit": "0.00000000",  // unrealized profit
           "isolatedMargin": "0.00000000",
           "notional": "0",
           "isolatedWallet": "0",
           "initialMargin": "0",              // initial margin required with current mark price
           "maintMargin": "0",                // maintenance margin required
           "updateTime": 0
  	  }
	]
}
```

> OR multi-assets mode

```javascript
{
	"totalInitialMargin": "0.00000000",            // the sum of USD value of all cross positions/open order initial margin
	"totalMaintMargin": "0.00000000",  	           // the sum of USD value of all cross positions maintenance margin
	"totalWalletBalance": "126.72469206",          // total wallet balance in USD
	"totalUnrealizedProfit": "0.00000000",         // total unrealized profit in USD
	"totalMarginBalance": "126.72469206",          // total margin balance in USD
	"totalPositionInitialMargin": "0.00000000",    // the sum of USD value of all cross positions initial margin
	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price in USD
	"totalCrossWalletBalance": "126.72469206",     // crossed wallet balance in USD
	"totalCrossUnPnl": "0.00000000",	           // unrealized profit of crossed positions in USD
	"availableBalance": "126.72469206",            // available balance in USD
	"maxWithdrawAmount": "126.72469206"            // maximum virtual amount for transfer out in USD
	"assets": [
		{
			"asset": "USDT",			         // asset name
			"walletBalance": "23.72469206",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "23.72469206",      // margin balance
			"maintMargin": "0.00000000",	     // maintenance margin required
			"initialMargin": "0.00000000",       // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "23.72469206",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "126.72469206",       // available balance
			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
			"updateTime": 1625474304765 // last update time
		},
		{
			"asset": "BUSD",			// asset name
			"walletBalance": "103.12345678",      // wallet balance
			"unrealizedProfit": "0.00000000",    // unrealized profit
			"marginBalance": "103.12345678",      // margin balance
			"maintMargin": "0.00000000",	    // maintenance margin required
			"initialMargin": "0.00000000",    // total initial margin required with current mark price
			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
			"crossWalletBalance": "103.12345678",      // crossed wallet balance
			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
			"availableBalance": "126.72469206",       // available balance
			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
			"updateTime": 1625474304765 // last update time
		}
	],
 	"positions": [  // positions of all symbols user had position are returned
                    // only "BOTH" positions will be returned with One-way mode
		            // only "LONG" and "SHORT" positions will be returned with Hedge mode
   	  {
           "symbol": "BTCUSDT",
           "positionSide": "BOTH",            // position side
           "positionAmt": "1.000",
           "unrealizedProfit": "0.00000000",  // unrealized profit
           "isolatedMargin": "0.00000000",
           "notional": "0",
           "isolatedWallet": "0",
           "initialMargin": "0",              // initial margin required with current mark price
           "maintMargin": "0",                // maintenance margin required
           "updateTime": 0
  	  }
	]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Account Balance V2 (USER\_DATA)

## API Description

Query account balance info

## HTTP Request

GET `/fapi/v2/balance`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
 	{
 		"accountAlias": "SgsR",    // unique account code
 		"asset": "USDT",  	// asset name
 		"balance": "122607.35137903", // wallet balance
 		"crossWalletBalance": "23.72469206", // crossed wallet balance
  		"crossUnPnl": "0.00000000",  // unrealized profit of crossed positions
  		"availableBalance": "23.72469206",       // available balance
  		"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
  		"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
  		"updateTime": 1617939110373
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Account Balance V3 (USER\_DATA)

## API Description

Query account balance info

## HTTP Request

GET `/fapi/v3/balance`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
 {
   "accountAlias": "SgsR",              // unique account code
   "asset": "USDT",  	                // asset name
   "balance": "122607.35137903",        // wallet balance
   "crossWalletBalance": "23.72469206", // crossed wallet balance
   "crossUnPnl": "0.00000000",           // unrealized profit of crossed positions
   "availableBalance": "23.72469206",   // available balance
   "maxWithdrawAmount": "23.72469206",  // maximum amount for transfer out
   "marginAvailable": true,             // whether the asset can be used as margin in Multi-Assets mode
   "updateTime": 1617939110373
 }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Trading Quantitative Rules Indicators (USER\_DATA)

## API Description

Futures trading quantitative rules indicators, for more information on this, please refer to the [Futures Trading Quantitative Rules](https://www.binance.com/en/support/faq/4f462ebe6ff445d4a170be7d9e897272)

## HTTP Request

GET `/fapi/v1/apiTradingStatus`

## Request Weight

- **1** for a single symbol
- **10** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> **Response:**

```javascript
{
    "indicators": { // indicator: quantitative rules indicators, value: user's indicators value, triggerValue: trigger indicator value threshold of quantitative rules.
        "BTCUSDT": [
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "UFR",  // Unfilled Ratio (UFR)
                "value": 0.05,  // Current value
                "triggerValue": 0.995  // Trigger value
            },
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "IFER",  // IOC/FOK Expiration Ratio (IFER)
                "value": 0.99,  // Current value
                "triggerValue": 0.99  // Trigger value
            },
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "GCR",  // GTC Cancellation Ratio (GCR)
                "value": 0.99,  // Current value
                "triggerValue": 0.99  // Trigger value
            },
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "DR",  // Dust Ratio (DR)
                "value": 0.99,  // Current value
                "triggerValue": 0.99  // Trigger value
            }
        ],
        "ETHUSDT": [
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "UFR",
                "value": 0.05,
                "triggerValue": 0.995
            },
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "IFER",
                "value": 0.99,
                "triggerValue": 0.99
            },
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "GCR",
                "value": 0.99,
                "triggerValue": 0.99
            }
            {
				"isLocked": true,
			    "plannedRecoverTime": 1545741270000,
                "indicator": "DR",
                "value": 0.99,
                "triggerValue": 0.99
            }
        ]
    },
    "updateTime": 1545741270000
}
```

> Or (account violation triggered)

```javascript
{
    "indicators":{
        "ACCOUNT":[
            {
                "indicator":"TMV",  //  Too many violations under multiple symbols trigger account violation
                "value":10,
                "triggerValue":1,
                "plannedRecoverTime":1644919865000,
                "isLocked":true
            }
        ]
    },
    "updateTime":1644913304748
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get BNB Burn Status (USER\_DATA)

## API Description

Get user's BNB Fee Discount (Fee Discount On or Fee Discount Off )

## HTTP Request

GET `/fapi/v1/feeBurn`

## Request Weight

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"feeBurn": true // "true": Fee Discount On; "false": Fee Discount Off
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Current Multi-Assets Mode (USER\_DATA)

## API Description

Get user's Multi-Assets mode (Multi-Assets Mode or Single-Asset Mode) on _**Every symbol**_

## HTTP Request

GET `/fapi/v1/multiAssetsMargin`

## Request Weight

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"multiAssetsMargin": true // "true": Multi-Assets Mode; "false": Single-Asset Mode
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Current Position Mode(USER\_DATA)

## API Description

Get user's position mode (Hedge Mode or One-way Mode ) on _**EVERY symbol**_

## HTTP Request

GET `/fapi/v1/positionSide/dual`

## Request Weight

30

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"dualSidePosition": true // "true": Hedge Mode; "false": One-way Mode
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Download Id For Futures Order History (USER\_DATA)

## API Description

Get Download Id For Futures Order History

## HTTP Request

GET `/fapi/v1/order/asyn`

## Request Weight

**1000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | YES | Timestamp in ms |
| endTime | LONG | YES | Timestamp in ms |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Request Limitation is 10 times per month, shared by front end download page and rest api
> - The time between `startTime` and `endTime` can not be longer than 1 year

## Response Example

```javascript
{
	"avgCostTimestampOfLast30d":7241837, // Average time taken for data download in the past 30 days
  	"downloadId":"546975389218332672",
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Download Id For Futures Trade History (USER\_DATA)

## API Description

Get download id for futures trade history

## HTTP Request

GET `/fapi/v1/trade/asyn`

## Request Weight

**1000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | YES | Timestamp in ms |
| endTime | LONG | YES | Timestamp in ms |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Request Limitation is 5 times per month, shared by front end download page and rest api
> - The time between `startTime` and `endTime` can not be longer than 1 year

## Response Example

```javascript
{
	"avgCostTimestampOfLast30d":7241837, // Average time taken for data download in the past 30 days
  	"downloadId":"546975389218332672",
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Download Id For Futures Transaction History(USER\_DATA)

## API Description

Get download id for futures transaction history

## HTTP Request

GET `/fapi/v1/income/asyn`

## Request Weight

**1000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | YES | Timestamp in ms |
| endTime | LONG | YES | Timestamp in ms |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Request Limitation is 5 times per month, shared by front end download page and rest api
> - The time between `startTime` and `endTime` can not be longer than 1 year

## Response Example

```javascript
{
	"avgCostTimestampOfLast30d":7241837, // Average time taken for data download in the past 30 days
  	"downloadId":"546975389218332672",
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Future Account Transaction History List(USER\_DATA)

Please find details from [here](https://developers.binance.com/docs/wallet/asset/query-user-universal-transfer).

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Futures Order History Download Link by Id (USER\_DATA)

## API Description

Get futures order history download link by Id

## HTTP Request

GET `/fapi/v1/order/asyn/id`

## Request Weight

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| downloadId | STRING | YES | get by download id api |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Download link expiration: 24h

## Response Example

> **Response:**

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"completed",     // Enum：completed，processing
  	"url":"www.binance.com",  // The link is mapped to download id
  	"notified":true,          // ignore
  	"expirationTimestamp":1645009771000,  // The link would expire after this timestamp
  	"isExpired":null,
}
```

> **OR** (Response when server is processing)

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"processing",
  	"url":"",
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Futures Trade Download Link by Id(USER\_DATA)

## API Description

Get futures trade download link by Id

## HTTP Request

GET `/fapi/v1/trade/asyn/id`

## Request Weight

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| downloadId | STRING | YES | get by download id api |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Download link expiration: 24h

## Response Example

> **Response:**

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"completed",     // Enum：completed，processing
  	"url":"www.binance.com",  // The link is mapped to download id
  	"notified":true,          // ignore
  	"expirationTimestamp":1645009771000,  // The link would expire after this timestamp
  	"isExpired":null,
}
```

> **OR** (Response when server is processing)

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"processing",
  	"url":"",
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Futures Transaction History Download Link by Id (USER\_DATA)

## API Description

Get futures transaction history download link by Id

## HTTP Request

GET `/fapi/v1/income/asyn/id`

## Request Weight

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| downloadId | STRING | YES | get by download id api |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Download link expiration: 24h

## Response Example

> **Response:**

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"completed",     // Enum：completed，processing
  	"url":"www.binance.com",  // The link is mapped to download id
  	"notified":true,          // ignore
  	"expirationTimestamp":1645009771000,  // The link would expire after this timestamp
  	"isExpired":null,
}
```

> **OR** (Response when server is processing)

```javascript
{
	"downloadId":"545923594199212032",
  	"status":"processing",
  	"url":"",
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Income History (USER\_DATA)

## API Description

Query income history

## HTTP Request

GET `/fapi/v1/income`

## Request Weight

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| incomeType | STRING | NO | TRANSFER, WELCOME\_BONUS, REALIZED\_PNL, FUNDING\_FEE, COMMISSION, INSURANCE\_CLEAR, REFERRAL\_KICKBACK, COMMISSION\_REBATE, API\_REBATE, CONTEST\_REWARD, CROSS\_COLLATERAL\_TRANSFER, OPTIONS\_PREMIUM\_FEE, OPTIONS\_SETTLE\_PROFIT, INTERNAL\_TRANSFER, AUTO\_EXCHANGE, DELIVERED\_SETTELMENT, COIN\_SWAP\_DEPOSIT, COIN\_SWAP\_WITHDRAW, POSITION\_LIMIT\_INCREASE\_FEE, STRATEGY\_UMFUTURES\_TRANSFER，FEE\_RETURN，BFUSD\_REWARD |
| startTime | LONG | NO | Timestamp in ms to get funding from INCLUSIVE. |
| endTime | LONG | NO | Timestamp in ms to get funding until INCLUSIVE. |
| page | INT | NO |  |
| limit | INT | NO | Default 100; max 1000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If neither `startTime` nor `endTime` is sent, the recent 7-day data will be returned.
> - If `incomeType` is not sent, all kinds of flow will be returned
> - "trandId" is unique in the same incomeType for a user
> - Income history only contains data for the last three months

## Response Example

```javascript
[
	{
    	"symbol": "",					// trade symbol, if existing
    	"incomeType": "TRANSFER",	// income type
    	"income": "-0.37500000",  // income amount
    	"asset": "USDT",				// income asset
    	"info":"TRANSFER",			// extra information
    	"time": 1570608000000,
    	"tranId":9689322392,		// transaction id
    	"tradeId":""					// trade id, if existing
	},
	{
   		"symbol": "BTCUSDT",
    	"incomeType": "COMMISSION",
    	"income": "-0.01000000",
    	"asset": "USDT",
    	"info":"COMMISSION",
    	"time": 1570636800000,
    	"tranId":9689322392,
    	"tradeId":"2059192"
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New Future Account Transfer

Please find details from [here](https://developers.binance.com/docs/wallet/asset/user-universal-transfer).

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Notional and Leverage Brackets (USER\_DATA)

## API Description

Query user notional and leverage bracket on speicfic symbol

## HTTP Request

GET `/fapi/v1/leverageBracket`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> **Response:**

```javascript
[
    {
        "symbol": "ETHUSDT",
	    "notionalCoef": 1.50,  //user symbol bracket multiplier, only appears when user's symbol bracket is adjusted
        "brackets": [
            {
                "bracket": 1,   // Notional bracket
                "initialLeverage": 75,  // Max initial leverage for this bracket
                "notionalCap": 10000,  // Cap notional of this bracket
                "notionalFloor": 0,  // Notional threshold of this bracket
                "maintMarginRatio": 0.0065, // Maintenance ratio for this bracket
                "cum": 0.0 // Auxiliary number for quick calculation

            },
        ]
    }
]
```

> **OR** (if symbol sent)

```javascript

{
    "symbol": "ETHUSDT",
    "notionalCoef": 1.50,
    "brackets": [
        {
            "bracket": 1,
            "initialLeverage": 75,
            "notionalCap": 10000,
            "notionalFloor": 0,
            "maintMarginRatio": 0.0065,
            "cum":0
        },
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query User Rate Limit (USER\_DATA)

## API Description

Query User Rate Limit

## HTTP Request

GET `/fapi/v1/rateLimit/order`

## Request Weight

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
    "rateLimitType": "ORDERS",
    "interval": "SECOND",
    "intervalNum": 10,
    "limit": 10000,
  },
  {
    "rateLimitType": "ORDERS",
    "interval": "MINUTE",
    "intervalNum": 1,
    "limit": 20000,
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Configuration(USER\_DATA)

## API Description

Get current account symbol configuration.

## HTTP Request

GET `/fapi/v1/symbolConfig`

## Request Weight

**5**

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
  "symbol": "BTCUSDT",
  "marginType": "CROSSED",
  "isAutoAddMargin": false,
  "leverage": 21,
  "maxNotionalValue": "1000000",
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Toggle BNB Burn On Futures Trade (TRADE)

## API Description

Change user's BNB Fee Discount (Fee Discount On or Fee Discount Off ) on _**EVERY symbol**_

## HTTP Request

POST `/fapi/v1/feeBurn`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| feeBurn | STRING | YES | "true": Fee Discount On; "false": Fee Discount Off |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# User Commission Rate (USER\_DATA)

## API Description

Get User Commission Rate

## HTTP Request

GET `/fapi/v1/commissionRate`

## Request Weight

**20**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"symbol": "BTCUSDT",
  	"makerCommissionRate": "0.0002",  // 0.02%
  	"takerCommissionRate": "0.0004",  // 0.04%
    "rpiCommissionRate": "0.00005"   // 0.005%
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Account Balance V2(USER\_DATA)

## API Description

Query account balance info

## Method

`v2/account.balance`

## Request

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "v2/account.balance",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "timestamp": 1702561978458,
        "signature": "208bb94a26f99aa122b1319490ca9cb2798fccc81d9b6449521a26268d53217a"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "status": 200,
    "result": [
      {
        "accountAlias": "SgsR",              // unique account code
        "asset": "USDT",  	                // asset name
        "balance": "122607.35137903",        // wallet balance
        "crossWalletBalance": "23.72469206", // crossed wallet balance
        "crossUnPnl": "0.00000000"           // unrealized profit of crossed positions
        "availableBalance": "23.72469206",   // available balance
        "maxWithdrawAmount": "23.72469206",  // maximum amount for transfer out
        "marginAvailable": true,             // whether the asset can be used as margin in Multi-Assets mode
        "updateTime": 1617939110373
      }
    ],
    "rateLimits": [
      {
        "rateLimitType": "REQUEST_WEIGHT",
        "interval": "MINUTE",
        "intervalNum": 1,
        "limit": 2400,
        "count": 20
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Account Information(USER\_DATA)

## API Description

Get current account information. User in single-asset/ multi-assets mode will see different value, see comments in response section for detail.

## Method

`account.status`

## Request

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "account.status",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "timestamp": 1702620814781,
        "signature": "6bb98ef84170c70ba3d01f44261bfdf50fef374e551e590de22b5c3b729b1d8c"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> Single Asset Mode

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": {
    "feeTier": 0,       // account commission tier
    "canTrade": true,   // if can trade
    "canDeposit": true,     // if can transfer in asset
    "canWithdraw": true,    // if can transfer out asset
    "updateTime": 0,        // reserved property, please ignore
    "multiAssetsMargin": false,
    "tradeGroupId": -1,
    "totalInitialMargin": "0.00000000",    // total initial margin required with current mark price (useless with isolated positions), only for USDT asset
    "totalMaintMargin": "0.00000000",     // total maintenance margin required, only for USDT asset
    "totalWalletBalance": "23.72469206",     // total wallet balance, only for USDT asset
    "totalUnrealizedProfit": "0.00000000",   // total unrealized profit, only for USDT asset
    "totalMarginBalance": "23.72469206",     // total margin balance, only for USDT asset
    "totalPositionInitialMargin": "0.00000000",    // initial margin required for positions with current mark price, only for USDT asset
    "totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price, only for USDT asset
    "totalCrossWalletBalance": "23.72469206",      // crossed wallet balance, only for USDT asset
    "totalCrossUnPnl": "0.00000000",      // unrealized profit of crossed positions, only for USDT asset
    "availableBalance": "23.72469206",       // available balance, only for USDT asset
    "maxWithdrawAmount": "23.72469206"     // maximum amount for transfer out, only for USDT asset
    "assets": [
        {
            "asset": "USDT",            // asset name
            "walletBalance": "23.72469206",      // wallet balance
            "unrealizedProfit": "0.00000000",    // unrealized profit
            "marginBalance": "23.72469206",      // margin balance
            "maintMargin": "0.00000000",        // maintenance margin required
            "initialMargin": "0.00000000",    // total initial margin required with current mark price
            "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
            "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
            "crossWalletBalance": "23.72469206",      // crossed wallet balance
            "crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
            "availableBalance": "23.72469206",       // available balance
            "maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
            "marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
            "updateTime": 1625474304765 // last update time
        },
        {
            "asset": "BUSD",            // asset name
            "walletBalance": "103.12345678",      // wallet balance
            "unrealizedProfit": "0.00000000",    // unrealized profit
            "marginBalance": "103.12345678",      // margin balance
            "maintMargin": "0.00000000",        // maintenance margin required
            "initialMargin": "0.00000000",    // total initial margin required with current mark price
            "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
            "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
            "crossWalletBalance": "103.12345678",      // crossed wallet balance
            "crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
            "availableBalance": "103.12345678",       // available balance
            "maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
            "marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
            "updateTime": 1625474304765 // last update time
        }
    ],
    "positions": [  // positions of all symbols in the market are returned
        // only "BOTH" positions will be returned with One-way mode
        // only "LONG" and "SHORT" positions will be returned with Hedge mode
        {
            "symbol": "BTCUSDT",    // symbol name
            "initialMargin": "0",   // initial margin required with current mark price
            "maintMargin": "0",     // maintenance margin required
            "unrealizedProfit": "0.00000000",  // unrealized profit
            "positionInitialMargin": "0",      // initial margin required for positions with current mark price
            "openOrderInitialMargin": "0",     // initial margin required for open orders with current mark price
            "leverage": "100",      // current initial leverage
            "isolated": true,       // if the position is isolated
            "entryPrice": "0.00000",    // average entry price
            "maxNotional": "250000",    // maximum available notional with current leverage
            "bidNotional": "0",  // bids notional, ignore
            "askNotional": "0",  // ask notional, ignore
            "positionSide": "BOTH",     // position side
            "positionAmt": "0",         // position amount
            "updateTime": 0           // last update time
        }
    ]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

> Multi-Asset Mode

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": {
      "feeTier": 0,       // account commission tier
      "canTrade": true,   // if can trade
      "canDeposit": true,     // if can transfer in asset
      "canWithdraw": true,    // if can transfer out asset
      "updateTime": 0,        // reserved property, please ignore
      "multiAssetsMargin": true,
      "tradeGroupId": -1,
      "totalInitialMargin": "0.00000000",    // the sum of USD value of all cross positions/open order initial margin
      "totalMaintMargin": "0.00000000",     // the sum of USD value of all cross positions maintenance margin
      "totalWalletBalance": "126.72469206",     // total wallet balance in USD
      "totalUnrealizedProfit": "0.00000000",   // total unrealized profit in USD
      "totalMarginBalance": "126.72469206",     // total margin balance in USD
      "totalPositionInitialMargin": "0.00000000",    // the sum of USD value of all cross positions initial margin
      "totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price in USD
      "totalCrossWalletBalance": "126.72469206",      // crossed wallet balance in USD
      "totalCrossUnPnl": "0.00000000",      // unrealized profit of crossed positions in USD
      "availableBalance": "126.72469206",       // available balance in USD
      "maxWithdrawAmount": "126.72469206"     // maximum virtual amount for transfer out in USD
      "assets": [
          {
              "asset": "USDT",            // asset name
              "walletBalance": "23.72469206",      // wallet balance
              "unrealizedProfit": "0.00000000",    // unrealized profit
              "marginBalance": "23.72469206",      // margin balance
              "maintMargin": "0.00000000",        // maintenance margin required
              "initialMargin": "0.00000000",    // total initial margin required with current mark price
              "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
              "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
              "crossWalletBalance": "23.72469206",      // crossed wallet balance
              "crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
              "availableBalance": "126.72469206",       // available balance
              "maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
              "marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
              "updateTime": 1625474304765 // last update time
          },
          {
              "asset": "BUSD",            // asset name
              "walletBalance": "103.12345678",      // wallet balance
              "unrealizedProfit": "0.00000000",    // unrealized profit
              "marginBalance": "103.12345678",      // margin balance
              "maintMargin": "0.00000000",        // maintenance margin required
              "initialMargin": "0.00000000",    // total initial margin required with current mark price
              "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
              "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
              "crossWalletBalance": "103.12345678",      // crossed wallet balance
              "crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
              "availableBalance": "126.72469206",       // available balance
              "maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
              "marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
              "updateTime": 1625474304765 // last update time
          }
      ],
      "positions": [  // positions of all symbols in the market are returned
          // only "BOTH" positions will be returned with One-way mode
          // only "LONG" and "SHORT" positions will be returned with Hedge mode
          {
              "symbol": "BTCUSDT",    // symbol name
              "initialMargin": "0",   // initial margin required with current mark price
              "maintMargin": "0",     // maintenance margin required
              "unrealizedProfit": "0.00000000",  // unrealized profit
              "positionInitialMargin": "0",      // initial margin required for positions with current mark price
              "openOrderInitialMargin": "0",     // initial margin required for open orders with current mark price
              "leverage": "100",      // current initial leverage
              "isolated": true,       // if the position is isolated
              "entryPrice": "0.00000",    // average entry price
              "breakEvenPrice": "0.0",    // average entry price
              "maxNotional": "250000",    // maximum available notional with current leverage
              "bidNotional": "0",  // bids notional, ignore
              "askNotional": "0",  // ask notional, ignore
              "positionSide": "BOTH",     // position side
              "positionAmt": "0",         // position amount
              "updateTime": 0           // last update time
          }
      ]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Account Information V2(USER\_DATA)

## API Description

Get current account information. User in single-asset/ multi-assets mode will see different value, see comments in response section for detail.

## Method

`v2/account.status`

## Request

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "v2/account.status",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "timestamp": 1702620814781,
        "signature": "6bb98ef84170c70ba3d01f44261bfdf50fef374e551e590de22b5c3b729b1d8c"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

> Single Asset Mode

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": {
  	"totalInitialMargin": "0.00000000",            // total initial margin required with current mark price (useless with isolated positions), only for USDT asset
  	"totalMaintMargin": "0.00000000",  	           // total maintenance margin required, only for USDT asset
  	"totalWalletBalance": "103.12345678",           // total wallet balance, only for USDT asset
  	"totalUnrealizedProfit": "0.00000000",         // total unrealized profit, only for USDT asset
  	"totalMarginBalance": "103.12345678",           // total margin balance, only for USDT asset
  	"totalPositionInitialMargin": "0.00000000",    // initial margin required for positions with current mark price, only for USDT asset
  	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price, only for USDT asset
  	"totalCrossWalletBalance": "103.12345678",      // crossed wallet balance, only for USDT asset
  	"totalCrossUnPnl": "0.00000000",	           // unrealized profit of crossed positions, only for USDT asset
  	"availableBalance": "103.12345678",             // available balance, only for USDT asset
  	"maxWithdrawAmount": "103.12345678"             // maximum amount for transfer out, only for USDT asset
  	"assets": [ // For assets that are quote assets, USDT/USDC/BTC
  		{
  			"asset": "USDT",			            // asset name
  			"walletBalance": "23.72469206",         // wallet balance
  			"unrealizedProfit": "0.00000000",       // unrealized profit
  			"marginBalance": "23.72469206",         // margin balance
  			"maintMargin": "0.00000000",	        // maintenance margin required
  			"initialMargin": "0.00000000",          // total initial margin required with current mark price
  			"positionInitialMargin": "0.00000000",  // initial margin required for positions with current mark price
  			"openOrderInitialMargin": "0.00000000", // initial margin required for open orders with current mark price
  			"crossWalletBalance": "23.72469206",    // crossed wallet balance
  			"crossUnPnl": "0.00000000"              // unrealized profit of crossed positions
  			"availableBalance": "23.72469206",      // available balance
  			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
  			"updateTime": 1625474304765             // last update time
  		},
   		{
  			"asset": "USDC",			            // asset name
  			"walletBalance": "103.12345678",         // wallet balance
  			"unrealizedProfit": "0.00000000",       // unrealized profit
  			"marginBalance": "103.12345678",         // margin balance
  			"maintMargin": "0.00000000",	        // maintenance margin required
  			"initialMargin": "0.00000000",          // total initial margin required with current mark price
  			"positionInitialMargin": "0.00000000",  // initial margin required for positions with current mark price
  			"openOrderInitialMargin": "0.00000000", // initial margin required for open orders with current mark price
  			"crossWalletBalance": "103.12345678",    // crossed wallet balance
  			"crossUnPnl": "0.00000000"              // unrealized profit of crossed positions
  			"availableBalance": "126.72469206",      // available balance
  			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
  			"updateTime": 1625474304765             // last update time
  		},
      ],
  	"positions": [  // positions of all symbols user had position/ open orders are returned
  		            // only "BOTH" positions will be returned with One-way mode
  		            // only "LONG" and "SHORT" positions will be returned with Hedge mode
     	  {
             "symbol": "BTCUSDT",
             "positionSide": "BOTH",            // position side
             "positionAmt": "1.000",
             "unrealizedProfit": "0.00000000",  // unrealized profit
             "isolatedMargin": "0.00000000",
             "notional": "0",
             "isolatedWallet": "0",
             "initialMargin": "0",              // initial margin required with current mark price
             "maintMargin": "0",                // maintenance margin required
             "updateTime": 0
    	  }
  	]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

> Multi-Asset Mode

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": {
  	"totalInitialMargin": "0.00000000",            // the sum of USD value of all cross positions/open order initial margin
  	"totalMaintMargin": "0.00000000",  	           // the sum of USD value of all cross positions maintenance margin
  	"totalWalletBalance": "126.72469206",          // total wallet balance in USD
  	"totalUnrealizedProfit": "0.00000000",         // total unrealized profit in USD
  	"totalMarginBalance": "126.72469206",          // total margin balance in USD
  	"totalPositionInitialMargin": "0.00000000",    // the sum of USD value of all cross positions initial margin
  	"totalOpenOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price in USD
  	"totalCrossWalletBalance": "126.72469206",     // crossed wallet balance in USD
  	"totalCrossUnPnl": "0.00000000",	           // unrealized profit of crossed positions in USD
  	"availableBalance": "126.72469206",            // available balance in USD
  	"maxWithdrawAmount": "126.72469206"            // maximum virtual amount for transfer out in USD
  	"assets": [
  		{
  			"asset": "USDT",			         // asset name
  			"walletBalance": "23.72469206",      // wallet balance
  			"unrealizedProfit": "0.00000000",    // unrealized profit
  			"marginBalance": "23.72469206",      // margin balance
  			"maintMargin": "0.00000000",	     // maintenance margin required
  			"initialMargin": "0.00000000",       // total initial margin required with current mark price
  			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
  			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
  			"crossWalletBalance": "23.72469206",      // crossed wallet balance
  			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
  			"availableBalance": "126.72469206",       // available balance
  			"maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
  			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
  			"updateTime": 1625474304765 // last update time
  		},
  		{
  			"asset": "BUSD",			// asset name
  			"walletBalance": "103.12345678",      // wallet balance
  			"unrealizedProfit": "0.00000000",    // unrealized profit
  			"marginBalance": "103.12345678",      // margin balance
  			"maintMargin": "0.00000000",	    // maintenance margin required
  			"initialMargin": "0.00000000",    // total initial margin required with current mark price
  			"positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
  			"openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
  			"crossWalletBalance": "103.12345678",      // crossed wallet balance
  			"crossUnPnl": "0.00000000"       // unrealized profit of crossed positions
  			"availableBalance": "126.72469206",       // available balance
  			"maxWithdrawAmount": "103.12345678",     // maximum amount for transfer out
  			"marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
  			"updateTime": 1625474304765 // last update time
  		}
  	],
   	"positions": [  // positions of all symbols user had position are returned
                      // only "BOTH" positions will be returned with One-way mode
  		            // only "LONG" and "SHORT" positions will be returned with Hedge mode
     	  {
             "symbol": "BTCUSDT",
             "positionSide": "BOTH",            // position side
             "positionAmt": "1.000",
             "unrealizedProfit": "0.00000000",  // unrealized profit
             "isolatedMargin": "0.00000000",
             "notional": "0",
             "isolatedWallet": "0",
             "initialMargin": "0",              // initial margin required with current mark price
             "maintMargin": "0",                // maintenance margin required
             "updateTime": 0
    	  }
  	]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures Account Balance(USER\_DATA)

## API Description

Query account balance info

## Method

`account.balance`

## Request

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "account.balance",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "timestamp": 1702561978458,
        "signature": "208bb94a26f99aa122b1319490ca9cb2798fccc81d9b6449521a26268d53217a"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "status": 200,
    "result": [
        {
            "accountAlias": "SgsR",    // unique account code
            "asset": "USDT",    // asset name
            "balance": "122607.35137903", // wallet balance
            "crossWalletBalance": "23.72469206", // crossed wallet balance
            "crossUnPnl": "0.00000000"  // unrealized profit of crossed positions
            "availableBalance": "23.72469206",       // available balance
            "maxWithdrawAmount": "23.72469206",     // maximum amount for transfer out
            "marginAvailable": true,    // whether the asset can be used as margin in Multi-Assets mode
            "updateTime": 1617939110373
        }
    ],
    "rateLimits": [
      {
        "rateLimitType": "REQUEST_WEIGHT",
        "interval": "MINUTE",
        "intervalNum": 1,
        "limit": 2400,
        "count": 20
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Futures Account Balance V2(USER\_DATA)

## API Description

Query account balance info

## Method

`v2/account.balance`

## Request

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "v2/account.balance",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "timestamp": 1702561978458,
        "signature": "208bb94a26f99aa122b1319490ca9cb2798fccc81d9b6449521a26268d53217a"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "status": 200,
    "result": [
      {
        "accountAlias": "SgsR",              // unique account code
        "asset": "USDT",  	                // asset name
        "balance": "122607.35137903",        // wallet balance
        "crossWalletBalance": "23.72469206", // crossed wallet balance
        "crossUnPnl": "0.00000000"           // unrealized profit of crossed positions
        "availableBalance": "23.72469206",   // available balance
        "maxWithdrawAmount": "23.72469206",  // maximum amount for transfer out
        "marginAvailable": true,             // whether the asset can be used as margin in Multi-Assets mode
        "updateTime": 1617939110373
      }
    ],
    "rateLimits": [
      {
        "rateLimitType": "REQUEST_WEIGHT",
        "interval": "MINUTE",
        "intervalNum": 1,
        "limit": 2400,
        "count": 20
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Public Endpoints Info

## Terminology

- `base asset` refers to the asset that is the `quantity` of a symbol.
- `quote asset` refers to the asset that is the `price` of a symbol.

## ENUM definitions

**Symbol type:**

- FUTURE

**Contract type (contractType):**

- PERPETUAL
- CURRENT\_MONTH
- NEXT\_MONTH
- CURRENT\_QUARTER
- NEXT\_QUARTER
- PERPETUAL\_DELIVERING

**Contract status (contractStatus, status):**

- PENDING\_TRADING
- TRADING
- PRE\_DELIVERING
- DELIVERING
- DELIVERED
- PRE\_SETTLE
- SETTLING
- CLOSE

**Order status (status):**

- NEW
- PARTIALLY\_FILLED
- FILLED
- CANCELED
- REJECTED
- EXPIRED
- EXPIRED\_IN\_MATCH

**Order types (orderTypes, type):**

- LIMIT
- MARKET
- STOP
- STOP\_MARKET
- TAKE\_PROFIT
- TAKE\_PROFIT\_MARKET
- TRAILING\_STOP\_MARKET

**Order side (side):**

- BUY
- SELL

**Position side (positionSide):**

- BOTH
- LONG
- SHORT

**Time in force (timeInForce):**

- GTC - Good Till Cancel(GTC order valitidy is 1 year from placement)
- IOC - Immediate or Cancel
- FOK - Fill or Kill
- GTX - Good Till Crossing (Post Only)
- GTD - Good Till Date
- RPI - Retail Price Improvement(RPI order is post only and only be matched with the order from APP or Web)

**Working Type (workingType)**

- MARK\_PRICE
- CONTRACT\_PRICE

**Response Type (newOrderRespType)**

- ACK
- RESULT

**Kline/Candlestick chart intervals:**

m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

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

**STP MODE (selfTradePreventionMode):**

- EXPIRE\_TAKER
- EXPIRE\_BOTH
- EXPIRE\_MAKER

**Price Match (priceMatch)**

- NONE (No price match)
- OPPONENT (counterparty best price)
- OPPONENT\_5 (the 5th best price from the counterparty)
- OPPONENT\_10 (the 10th best price from the counterparty)
- OPPONENT\_20 (the 20th best price from the counterparty)
- QUEUE (the best price on the same side of the order book)
- QUEUE\_5 (the 5th best price on the same side of the order book)
- QUEUE\_10 (the 10th best price on the same side of the order book)
- QUEUE\_20 (the 20th best price on the same side of the order book)

**Rate limiters (rateLimitType)**

> REQUEST\_WEIGHT

```javascript
  {
  	"rateLimitType": "REQUEST_WEIGHT",
  	"interval": "MINUTE",
  	"intervalNum": 1,
  	"limit": 2400
  }
```

> ORDERS

```javascript
  {
  	"rateLimitType": "ORDERS",
  	"interval": "MINUTE",
  	"intervalNum": 1,
  	"limit": 1200
   }
```

- REQUEST\_WEIGHT

- ORDERS

**Rate limit intervals (interval)**

# Filters

Filters define trading rules on a symbol or an exchange.

## Symbol filters

### PRICE\_FILTER

> **/exchangeInfo format:**

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
- (`price`-`minPrice`) % `tickSize` == 0

### LOT\_SIZE

> **/exchangeInfo format:**

```javascript
  {
    "filterType": "LOT_SIZE",
    "minQty": "0.00100000",
    "maxQty": "100000.00000000",
    "stepSize": "0.00100000"
  }
```

The `LOT_SIZE` filter defines the `quantity` (aka "lots" in auction terms) rules for a symbol. There are 3 parts:

- `minQty` defines the minimum `quantity` allowed.
- `maxQty` defines the maximum `quantity` allowed.
- `stepSize` defines the intervals that a `quantity` can be increased/decreased by.

In order to pass the `lot size`, the following must be true for `quantity`:

- `quantity` >= `minQty`
- `quantity` <= `maxQty`
- (`quantity`-`minQty`) % `stepSize` == 0

### MARKET\_LOT\_SIZE

> **/exchangeInfo format:**

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
- (`quantity`-`minQty`) % `stepSize` == 0

### MAX\_NUM\_ORDERS

> **/exchangeInfo format:**

```javascript
  {
    "filterType": "MAX_NUM_ORDERS",
    "limit": 200
  }
```

The `MAX_NUM_ORDERS` filter defines the maximum number of orders an account is allowed to have open on a symbol.

Note that both "algo" orders and normal orders are counted for this filter.

### MAX\_NUM\_ALGO\_ORDERS

> **/exchangeInfo format:**

```javascript
  {
    "filterType": "MAX_NUM_ALGO_ORDERS",
    "limit": 100
  }
```

The `MAX_NUM_ALGO_ORDERS` filter defines the maximum number of all kinds of algo orders an account is allowed to have open on a symbol.

The algo orders include `STOP`, `STOP_MARKET`, `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`, and `TRAILING_STOP_MARKET` orders.

### PERCENT\_PRICE

> **/exchangeInfo format:**

```javascript
  {
    "filterType": "PERCENT_PRICE",
    "multiplierUp": "1.1500",
    "multiplierDown": "0.8500",
    "multiplierDecimal": 4
  }
```

The `PERCENT_PRICE` filter defines valid range for a price based on the mark price.

In order to pass the `percent price`, the following must be true for `price`:

- BUY: `price` <= `markPrice` \\* `multiplierUp`
- SELL: `price` >= `markPrice` \\* `multiplierDown`

### MIN\_NOTIONAL

> **/exchangeInfo format:**

```javascript
  {
    "filterType": "MIN_NOTIONAL",
    "notional": "5.0"
  }
```

The `MIN_NOTIONAL` filter defines the minimum notional value allowed for an order on a symbol.
An order's notional value is the `price` \\* `quantity`.
Since `MARKET` orders have no price, the mark price is used.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# List All Convert Pairs

## API Description

Query for all convertible token pairs and the tokens’ respective upper/lower limits

## HTTP Request

GET `/fapi/v1/convert/exchangeInfo`

## Request Weight

**20(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| fromAsset | STRING | EITHER OR BOTH | User spends coin |
| toAsset | STRING | EITHER OR BOTH | User receives coin |

> - User needs to supply either or both of the input parameter
> - If not defined for both fromAsset and toAsset, only partial token pairs will be returned
> - Asset BNFCR is only available to convert for MICA region users.

## Response Example

```javascript
[
  {
    "fromAsset":"BTC",
    "toAsset":"USDT",
    "fromAssetMinAmount":"0.0004",
    "fromAssetMaxAmount":"50",
    "toAssetMinAmount":"20",
    "toAssetMaxAmount":"2500000"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Accept the offered quote (USER\_DATA)

## API Description

Accept the offered quote by quote ID.

## HTTP Request

POST `/fapi/v1/convert/acceptQuote`

## Request Weight

**200(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| quoteId | STRING | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "orderId":"933256278426274426",
  "createTime":1623381330472,
  "orderStatus":"PROCESS" //PROCESS/ACCEPT_SUCCESS/SUCCESS/FAIL
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# List All Convert Pairs

## API Description

Query for all convertible token pairs and the tokens’ respective upper/lower limits

## HTTP Request

GET `/fapi/v1/convert/exchangeInfo`

## Request Weight

**20(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| fromAsset | STRING | EITHER OR BOTH | User spends coin |
| toAsset | STRING | EITHER OR BOTH | User receives coin |

> - User needs to supply either or both of the input parameter
> - If not defined for both fromAsset and toAsset, only partial token pairs will be returned
> - Asset BNFCR is only available to convert for MICA region users.

## Response Example

```javascript
[
  {
    "fromAsset":"BTC",
    "toAsset":"USDT",
    "fromAssetMinAmount":"0.0004",
    "fromAssetMaxAmount":"50",
    "toAssetMinAmount":"20",
    "toAssetMaxAmount":"2500000"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Order status(USER\_DATA)

## API Description

Query order status by order ID.

## HTTP Request

GET `/fapi/v1/convert/orderStatus`

## Request Weight

**50(IP)**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | STRING | NO | Either orderId or quoteId is required |
| quoteId | STRING | NO | Either orderId or quoteId is required |

## Response Example

```javascript
{
  "orderId":933256278426274426,
  "orderStatus":"SUCCESS",
  "fromAsset":"BTC",
  "fromAmount":"0.00054414",
  "toAsset":"USDT",
  "toAmount":"20",
  "ratio":"36755",
  "inverseRatio":"0.00002721",
  "createTime":1623381330472
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Send Quote Request(USER\_DATA)

## API Description

Request a quote for the requested token pairs

## HTTP Request

POST `/fapi/v1/convert/getQuote`

## Request Weight

**50(IP)**

**360/hour，500/day**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| fromAsset | STRING | YES |  |
| toAsset | STRING | YES |  |
| fromAmount | DECIMAL | EITHER | When specified, it is the amount you will be debited after the conversion |
| toAmount | DECIMAL | EITHER | When specified, it is the amount you will be credited after the conversion |
| validTime | ENUM | NO | 10s, default 10s |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

- Either fromAmount or toAmount should be sent
- `quoteId` will be returned only if you have enough funds to convert

## Response Example

```javascript
{
   "quoteId":"12415572564",
   "ratio":"38163.7",
   "inverseRatio":"0.0000262",
   "validTimestamp":1623319461670,
   "toAmount":"3816.37",
   "fromAmount":"0.1"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Self Trade Prevention (STP) FAQ

## What is Self Trade Prevention?

Self Trade Prevention (or STP) prevents orders of users, or the user's `tradeGroupId` to match against their own.

## What defines a self-trade?

A self-trade can occur in either scenario:

- The order traded against the same account.
- The order traded against an account with the same `tradeGroupId`.

## What happens when STP is triggered?

There are three possible modes for what the system will do if an order could create a self-trade.

`EXPIRE_TAKER` \- This mode prevents a trade by immediately expiring the taker order's remaining quantity.

`EXPIRE_MAKER` \- This mode prevents a trade by immediately expiring the potential maker order's remaining quantity.

`EXPIRE_BOTH` \- This mode prevents a trade by immediately expiring both the taker and the potential maker orders' remaining quantities.

The STP event will occur depending on the STP mode of the **taker order**.

Thus, the STP mode of an order that goes on the book is no longer relevant and will be ignored for all future order processing.

## Where do I set STP mode for an order?

STP can only be set using field `selfTradePreventionMode` through API endpoints below:

- POST `/fapi/v1/order`
- POST `/fapi/v1/batchOrders`

## What is a Trade Group Id?

Different accounts with the same `tradeGroupId` are considered part of the same "trade group". Orders submitted by members of a trade group are eligible for STP according to the taker-order's STP mode.

A user can confirm if their accounts are under the same `tradeGroupId` from the API either from `GET /fapi/v1/accountConfig` (REST API).

If the value is `-1`, then the `tradeGroupId` has not been set for that account, so the STP may only take place between orders of the same account.

We will release feature for user to group subaccounts to same `tradeGroupId` on website in future updates.

## How do I know which symbol uses STP?

Placing orders on all symbols in `GET fapi/v1/exchangeInfo` can set `selfTradePreventionMode`.

## What order types support STP?

`LIMIT`/`MARKET`/`STOP`/`TAKE_PROFIT`/`STOP_MARKET`/`TAKE_PROFIT_MARKET`/`TRAILING_STOP_MARKET` all supports STP when Time in force(timeInForce) set to `GTC`/ `IOC`/ `GTD`.
STP won't take effect for Time in force(timeInForce) `FOK` or `GTX`

## Does Modify order support STP?

No. Modify order that has reset `selfTradePreventionMode` to `NONE`

## How do I know if an order expired due to STP?

The order will have the status `EXPIRED_IN_MATCH`.

In user data stream event `ORDER_TRADE_UPDATE`, field `X` would be `EXPIRED_IN_MATCH` if order is expired due to STP

```javascript
{
  "e":"ORDER_TRADE_UPDATE",      // Event Type
  "E":1568879465651,             // Event Time
  "T":1568879465650,             // Transaction Time
  "o":{
    "s":"BTCUSDT",               // Symbol
    "c":"TEST",                  // Client Order Id
      // special client order id:
      // starts with "autoclose-": liquidation order
      // "adl_autoclose": ADL auto close order
      // "settlement_autoclose-": settlement order for delisting or delivery
    "S":"SELL",                  // Side
    "o":"TRAILING_STOP_MARKET",  // Order Type
    "f":"GTC",                   // Time in Force
    "q":"0.001",                 // Original Quantity
    "p":"0",                     // Original Price
    "ap":"0",                    // Average Price
    "sp":"7103.04",              // Stop Price. Please ignore with TRAILING_STOP_MARKET order
    "x":"EXPIRED",               // Execution Type
    "X":"EXPIRED_IN_MATCH",      // Order Status
    "i":8886774,                 // Order Id
    "l":"0",                     // Order Last Filled Quantity
    "z":"0",                     // Order Filled Accumulated Quantity
    "L":"0",                     // Last Filled Price
    "N":"USDT",                  // Commission Asset, will not push if no commission
    "n":"0",                     // Commission, will not push if no commission
    "T":1568879465650,           // Order Trade Time
    "t":0,                       // Trade Id
    "b":"0",                     // Bids Notional
    "a":"9.91",                  // Ask Notional
    "m":false,                   // Is this trade the maker side?
    "R":false,                   // Is this reduce only
    "wt":"CONTRACT_PRICE",       // Stop Price Working Type
    "ot":"TRAILING_STOP_MARKET", // Original Order Type
    "ps":"LONG",                 // Position Side
    "cp":false,                  // If Close-All, pushed with conditional order
    "AP":"7476.89",              // Activation Price, only puhed with TRAILING_STOP_MARKET order
    "cr":"5.0",                  // Callback Rate, only puhed with TRAILING_STOP_MARKET order
    "pP": false,                 // ignore
    "si": 0,                     // ignore
    "ss": 0,                     // ignore
    "rp":"0",                    // Realized Profit of the trade
    "V": "EXPIRE_MAKER",         // selfTradePreventionMode
    "pm":"QUEUE",                // price match type
    "gtd":1768879465650          // good till date
   }
}
```

## STP Examples:

For all these cases, assume that all orders for these examples are made on the same account.

**Scenario A- A user sends an order with `EXPIRE_MAKER` that would match with their orders that are already on the book.**

```text
Maker Order 1: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002 selfTradePreventionMode=EXPIRE_MAKER
Maker Order 2: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20001 selfTradePreventionMode=EXPIRE_MAKER
Taker Order 1: symbol=BTCUSDT side=SELL type=LIMIT quantity=1 price=20000 selfTradePreventionMode=EXPIRE_MAKER
```

**Result**: The orders that were on the book will expire due to STP, and the taker order will go on the book.

Maker Order 1

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "FILLED",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "20002",
    "origQty": "1",
    "executedQty": "1",
    "cumQuote": "20002",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Maker Order 2

```json
{
    "orderId": 292864711,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker2",
    "price": "20001",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Output of the Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "PARTIALLY_FILLED",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "20002",
    "origQty": "2",
    "executedQty": "1",
    "cumQuote": "20002",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario B - A user sends an order with `EXPIRE_TAKER` that would match with their orders already on the book.**

```text
Maker Order 1: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002  selfTradePreventionMode=EXPIRE_MAKER
Maker Order 2: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20001  selfTradePreventionMode=EXPIRE_MAKER
Taker Order 1: symbol=BTCUSDT side=SELL type=LIMIT quantity=2 price=3      selfTradePreventionMode=EXPIRE_TAKER
```

**Result**: The orders already on the book will remain, while the taker order will expire.

Maker Order 1

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "FILLED",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Maker Order 2

```json
{
    "orderId": 292864711,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker2",
    "price": "20001",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Output of the Taker order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_TAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario C- A user has an order on the book, and then sends an order with `EXPIRE_BOTH` that would match with the existing order.**

```text
Maker Order: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002 selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=BTCUSDT side=SELL type=LIMIT quantity=3 price=20000 selfTradePreventionMode=EXPIRE_BOTH
```

**Result:** Both orders will expire.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_BOTH",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario D - A user has an order on the book with `EXPIRE_MAKER`, and then sends a new order with `EXPIRE_TAKER` which would match with the existing order.**

```text
Maker Order: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=1 selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=BTCUSDT side=SELL type=LIMIT quantity=1 price=1 selfTradePreventionMode=EXPIRE_TAKER
```

**Result**: The taker order's STP mode will be used, so the taker order will be expired.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "NEW",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_TAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario E - A user sends a market order with `EXPIRE_MAKER` which would match with an existing order.**

```text
Maker Order: symbol=ABCDEF side=BUY  type=LIMIT  quantity=1 price=1  selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=ABCDEF side=SELL type=MARKET quantity=3          selfTradePreventionMode=EXPIRE_MAKER
```

**Result**: The existing order expires with the status `EXPIRED_IN_MATCH`, due to STP.
The new order also expires but with status `EXPIRED`, due to low liquidity on the order book.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Self Trade Prevention (STP) FAQ

## What is Self Trade Prevention?

Self Trade Prevention (or STP) prevents orders of users, or the user's `tradeGroupId` to match against their own.

## What defines a self-trade?

A self-trade can occur in either scenario:

- The order traded against the same account.
- The order traded against an account with the same `tradeGroupId`.

## What happens when STP is triggered?

There are three possible modes for what the system will do if an order could create a self-trade.

`EXPIRE_TAKER` \- This mode prevents a trade by immediately expiring the taker order's remaining quantity.

`EXPIRE_MAKER` \- This mode prevents a trade by immediately expiring the potential maker order's remaining quantity.

`EXPIRE_BOTH` \- This mode prevents a trade by immediately expiring both the taker and the potential maker orders' remaining quantities.

The STP event will occur depending on the STP mode of the **taker order**.

Thus, the STP mode of an order that goes on the book is no longer relevant and will be ignored for all future order processing.

## Where do I set STP mode for an order?

STP can only be set using field `selfTradePreventionMode` through API endpoints below:

- POST `/fapi/v1/order`
- POST `/fapi/v1/batchOrders`

## What is a Trade Group Id?

Different accounts with the same `tradeGroupId` are considered part of the same "trade group". Orders submitted by members of a trade group are eligible for STP according to the taker-order's STP mode.

A user can confirm if their accounts are under the same `tradeGroupId` from the API either from `GET /fapi/v1/accountConfig` (REST API).

If the value is `-1`, then the `tradeGroupId` has not been set for that account, so the STP may only take place between orders of the same account.

We will release feature for user to group subaccounts to same `tradeGroupId` on website in future updates.

## How do I know which symbol uses STP?

Placing orders on all symbols in `GET fapi/v1/exchangeInfo` can set `selfTradePreventionMode`.

## What order types support STP?

`LIMIT`/`MARKET`/`STOP`/`TAKE_PROFIT`/`STOP_MARKET`/`TAKE_PROFIT_MARKET`/`TRAILING_STOP_MARKET` all supports STP when Time in force(timeInForce) set to `GTC`/ `IOC`/ `GTD`.
STP won't take effect for Time in force(timeInForce) `FOK` or `GTX`

## Does Modify order support STP?

No. Modify order that has reset `selfTradePreventionMode` to `NONE`

## How do I know if an order expired due to STP?

The order will have the status `EXPIRED_IN_MATCH`.

In user data stream event `ORDER_TRADE_UPDATE`, field `X` would be `EXPIRED_IN_MATCH` if order is expired due to STP

```javascript
{
  "e":"ORDER_TRADE_UPDATE",      // Event Type
  "E":1568879465651,             // Event Time
  "T":1568879465650,             // Transaction Time
  "o":{
    "s":"BTCUSDT",               // Symbol
    "c":"TEST",                  // Client Order Id
      // special client order id:
      // starts with "autoclose-": liquidation order
      // "adl_autoclose": ADL auto close order
      // "settlement_autoclose-": settlement order for delisting or delivery
    "S":"SELL",                  // Side
    "o":"TRAILING_STOP_MARKET",  // Order Type
    "f":"GTC",                   // Time in Force
    "q":"0.001",                 // Original Quantity
    "p":"0",                     // Original Price
    "ap":"0",                    // Average Price
    "sp":"7103.04",              // Stop Price. Please ignore with TRAILING_STOP_MARKET order
    "x":"EXPIRED",               // Execution Type
    "X":"EXPIRED_IN_MATCH",      // Order Status
    "i":8886774,                 // Order Id
    "l":"0",                     // Order Last Filled Quantity
    "z":"0",                     // Order Filled Accumulated Quantity
    "L":"0",                     // Last Filled Price
    "N":"USDT",                  // Commission Asset, will not push if no commission
    "n":"0",                     // Commission, will not push if no commission
    "T":1568879465650,           // Order Trade Time
    "t":0,                       // Trade Id
    "b":"0",                     // Bids Notional
    "a":"9.91",                  // Ask Notional
    "m":false,                   // Is this trade the maker side?
    "R":false,                   // Is this reduce only
    "wt":"CONTRACT_PRICE",       // Stop Price Working Type
    "ot":"TRAILING_STOP_MARKET", // Original Order Type
    "ps":"LONG",                 // Position Side
    "cp":false,                  // If Close-All, pushed with conditional order
    "AP":"7476.89",              // Activation Price, only puhed with TRAILING_STOP_MARKET order
    "cr":"5.0",                  // Callback Rate, only puhed with TRAILING_STOP_MARKET order
    "pP": false,                 // ignore
    "si": 0,                     // ignore
    "ss": 0,                     // ignore
    "rp":"0",                    // Realized Profit of the trade
    "V": "EXPIRE_MAKER",         // selfTradePreventionMode
    "pm":"QUEUE",                // price match type
    "gtd":1768879465650          // good till date
   }
}
```

## STP Examples:

For all these cases, assume that all orders for these examples are made on the same account.

**Scenario A- A user sends an order with `EXPIRE_MAKER` that would match with their orders that are already on the book.**

```text
Maker Order 1: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002 selfTradePreventionMode=EXPIRE_MAKER
Maker Order 2: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20001 selfTradePreventionMode=EXPIRE_MAKER
Taker Order 1: symbol=BTCUSDT side=SELL type=LIMIT quantity=1 price=20000 selfTradePreventionMode=EXPIRE_MAKER
```

**Result**: The orders that were on the book will expire due to STP, and the taker order will go on the book.

Maker Order 1

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "FILLED",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "20002",
    "origQty": "1",
    "executedQty": "1",
    "cumQuote": "20002",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Maker Order 2

```json
{
    "orderId": 292864711,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker2",
    "price": "20001",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Output of the Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "PARTIALLY_FILLED",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "20002",
    "origQty": "2",
    "executedQty": "1",
    "cumQuote": "20002",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario B - A user sends an order with `EXPIRE_TAKER` that would match with their orders already on the book.**

```text
Maker Order 1: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002  selfTradePreventionMode=EXPIRE_MAKER
Maker Order 2: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20001  selfTradePreventionMode=EXPIRE_MAKER
Taker Order 1: symbol=BTCUSDT side=SELL type=LIMIT quantity=2 price=3      selfTradePreventionMode=EXPIRE_TAKER
```

**Result**: The orders already on the book will remain, while the taker order will expire.

Maker Order 1

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "FILLED",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Maker Order 2

```json
{
    "orderId": 292864711,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker2",
    "price": "20001",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Output of the Taker order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_TAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario C- A user has an order on the book, and then sends an order with `EXPIRE_BOTH` that would match with the existing order.**

```text
Maker Order: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=20002 selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=BTCUSDT side=SELL type=LIMIT quantity=3 price=20000 selfTradePreventionMode=EXPIRE_BOTH
```

**Result:** Both orders will expire.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_BOTH",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario D - A user has an order on the book with `EXPIRE_MAKER`, and then sends a new order with `EXPIRE_TAKER` which would match with the existing order.**

```text
Maker Order: symbol=BTCUSDT side=BUY  type=LIMIT quantity=1 price=1 selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=BTCUSDT side=SELL type=LIMIT quantity=1 price=1 selfTradePreventionMode=EXPIRE_TAKER
```

**Result**: The taker order's STP mode will be used, so the taker order will be expired.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "NEW",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_TAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

**Scenario E - A user sends a market order with `EXPIRE_MAKER` which would match with an existing order.**

```text
Maker Order: symbol=ABCDEF side=BUY  type=LIMIT  quantity=1 price=1  selfTradePreventionMode=EXPIRE_MAKER
Taker Order: symbol=ABCDEF side=SELL type=MARKET quantity=3          selfTradePreventionMode=EXPIRE_MAKER
```

**Result**: The existing order expires with the status `EXPIRED_IN_MATCH`, due to STP.
The new order also expires but with status `EXPIRED`, due to low liquidity on the order book.

Maker Order

```json
{
    "orderId": 292864710,
    "symbol": "BTCUSDT",
    "status": "EXPIRED_IN_MATCH",
    "clientOrderId": "testMaker1",
    "price": "20002",
    "avgPrice": "0.0000",
    "origQty": "1",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "BUY",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

Taker Order

```json
{
    "orderId": 292864712,
    "symbol": "BTCUSDT",
    "status": "EXPIRED",
    "clientOrderId": "testTaker1",
    "price": "20000",
    "avgPrice": "0.0000",
    "origQty": "3",
    "executedQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "closePosition": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "stopPrice": "0",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "goodTillDate": "null",
    "priceProtect": false,
    "origType": "LIMIT",
    "time": 1692849639460,
    "updateTime": 1692849639460
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Test Connectivity

## API Description

Test connectivity to the Rest API.

## HTTP Request

GET `/fapi/v1/ping`

## Request Weight

1

## Request Parameters

NONE

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# 24hr Ticker Price Change Statistics

## API Description

24 hour rolling window price change statistics.

**Careful** when accessing this with no symbol.

## HTTP Request

GET `/fapi/v1/ticker/24hr`

## Request Weight

**1** for a single symbol;

**40** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, tickers for all symbols will be returned in an array.

## Response Example

> **Response:**

```javascript
{
  "symbol": "BTCUSDT",
  "priceChange": "-94.99999800",
  "priceChangePercent": "-95.960",
  "weightedAvgPrice": "0.29628482",
  "lastPrice": "4.00000200",
  "lastQty": "200.00000000",
  "openPrice": "99.00000000",
  "highPrice": "100.00000000",
  "lowPrice": "0.10000000",
  "volume": "8913.30000000",
  "quoteVolume": "15.30000000",
  "openTime": 1499783499040,
  "closeTime": 1499869899040,
  "firstId": 28385,   // First tradeId
  "lastId": 28460,    // Last tradeId
  "count": 76         // Trade count
}
```

> OR

```javascript
[
	{
  		"symbol": "BTCUSDT",
  		"priceChange": "-94.99999800",
  		"priceChangePercent": "-95.960",
  		"weightedAvgPrice": "0.29628482",
  		"lastPrice": "4.00000200",
  		"lastQty": "200.00000000",
  		"openPrice": "99.00000000",
  		"highPrice": "100.00000000",
  		"lowPrice": "0.10000000",
  		"volume": "8913.30000000",
  		"quoteVolume": "15.30000000",
  		"openTime": 1499783499040,
  		"closeTime": 1499869899040,
  		"firstId": 28385,   // First tradeId
  		"lastId": 28460,    // Last tradeId
  		"count": 76         // Trade count
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# ADL Risk

## API Description

Query the symbol-level ADL risk rating.
The ADL risk rating measures the likelihood of ADL during liquidation, and the rating takes into account the insurance fund balance, position concentration on the symbol, order book depth, price volatility, average leverage, unrealized PnL, and margin utilization at the symbol level.
The rating can be high, medium and low, and is updated every 30 minutes.

## HTTP Request

GET `/fapi/v1/symbolAdlRisk`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

## Response Example

> **Response:**

```javascript
{
	"symbol": "BTCUSDT",
	"adlRisk": "low",  // ADL Risk rating
	"updateTime": 1597370495002
}
```

> **OR (when symbol not sent)**

```javascript
[
	{
	    "symbol": "BTCUSDT",
	    "adlRisk": "low",  // ADL Risk rating
	    "updateTime": 1597370495002
	},
	{
	    "symbol": "ETHUSDT",
	    "adlRisk": "high", // ADL Risk rating
	    "updateTime": 1597370495004
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Basis

## API Description

Query future basis

## HTTP Request

GET `/futures/data/basis`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| pair | STRING | YES | BTCUSDT |
| contractType | ENUM | YES | CURRENT\_QUARTER, NEXT\_QUARTER, PERPETUAL |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | YES | Default 30,Max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 30 days is available.

## Response Example

```javascript
[
    {
        "indexPrice": "34400.15945055",
        "contractType": "PERPETUAL",
        "basisRate": "0.0004",
        "futuresPrice": "34414.10",
        "annualizedBasisRate": "",
        "basis": "13.94054945",
        "pair": "BTCUSDT",
        "timestamp": 1698742800000
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Check Server Time

## API Description

Test connectivity to the Rest API and get the current server time.

## HTTP Request

GET `/fapi/v1/time`

## Request Weight

1

## Request Parameters

NONE

## Response Example

```javascript
{
  "serverTime": 1499827319559
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Composite Index Symbol Information

## API Description

Query composite index symbol information

## HTTP Request

GET `/fapi/v1/indexInfo`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - Only for composite index symbols

## Response Example

```javascript
[
	{
		"symbol": "DEFIUSDT",
		"time": 1589437530011,    // Current time
		"component": "baseAsset", //Component asset
		"baseAssetList":[
			{
				"baseAsset":"BAL",
				"quoteAsset": "USDT",
				"weightInQuantity":"1.04406228",
				"weightInPercentage":"0.02783900"
			},
			{
				"baseAsset":"BAND",
				"quoteAsset": "USDT",
				"weightInQuantity":"3.53782729",
				"weightInPercentage":"0.03935200"
			}
		]
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Compressed/Aggregate Trades List

## API Description

Get compressed, aggregate market trades. Market trades that fill in 100ms with the same price and the same taking side will have the quantity aggregated.

## HTTP Request

GET `/fapi/v1/aggTrades`

**Note**:

> Retail Price Improvement(RPI) orders are aggregated and without special tags to be distinguished.

## Request Weight

20

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| fromId | LONG | NO | ID to get aggregate trades from INCLUSIVE. |
| startTime | LONG | NO | Timestamp in ms to get aggregate trades from INCLUSIVE. |
| endTime | LONG | NO | Timestamp in ms to get aggregate trades until INCLUSIVE. |
| limit | INT | NO | Default 500; max 1000. |

> - support querying futures trade histories that are not older than one year
> - If both `startTime` and `endTime` are sent, time between `startTime` and `endTime` must be less than 1 hour.
> - If `fromId`, `startTime`, and `endTime` are not sent, the most recent aggregate trades will be returned.
> - Only market trades will be aggregated and returned, which means the insurance fund trades and ADL trades won't be aggregated.
> - Sending both `startTime`/`endTime` and `fromId` might cause response timeout, please send either `fromId` or `startTime`/`endTime`

## Response Example

```javascript
[
  {
    "a": 26129,         // Aggregate tradeId
    "p": "0.01633102",  // Price
    "q": "4.70443515",  // Quantity
    "f": 27781,         // First tradeId
    "l": 27781,         // Last tradeId
    "T": 1498793709153, // Timestamp
    "m": true,          // Was the buyer the maker?
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Continuous Contract Kline/Candlestick Data

## API Description

Kline/candlestick bars for a specific contract type.
Klines are uniquely identified by their open time.

## HTTP Request

GET `/fapi/v1/continuousKlines`

## Request Weight

based on parameter `LIMIT`

| LIMIT | weight |
| --- | --- |
| \[1,100) | 1 |
| \[100, 500) | 2 |
| \[500, 1000\] | 5 |
| \> 1000 | 10 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| pair | STRING | YES |  |
| contractType | ENUM | YES |  |
| interval | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1500. |

> - If startTime and endTime are not sent, the most recent klines are returned.

> - Contract type:
>   - PERPETUAL
>   - CURRENT\_QUARTER
>   - NEXT\_QUARTER
>   - TRADIFI\_PERPETUAL

## Response Example

```javascript
[
  [
    1607444700000,      	// Open time
    "18879.99",       	 	// Open
    "18900.00",       	 	// High
    "18878.98",       	 	// Low
    "18896.13",      	 	// Close (or latest price)
    "492.363", 			 	// Volume
    1607444759999,       	// Close time
    "9302145.66080",    	// Quote asset volume
    1874,             		// Number of trades
    "385.983",    			// Taker buy volume
    "7292402.33267",      	// Taker buy quote asset volume
    "0" 					// Ignore.
  ]
]
```

 

- [API Description](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Continuous-Contract-Kline-Candlestick-Data#api-description)
- [HTTP Request](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Continuous-Contract-Kline-Candlestick-Data#http-request)
- [Request Weight](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Continuous-Contract-Kline-Candlestick-Data#request-weight)
- [Request Parameters](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Continuous-Contract-Kline-Candlestick-Data#request-parameters)
- [Response Example](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Continuous-Contract-Kline-Candlestick-Data#response-example)

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Delist Schedule

## API Description

The Futures team will update the `deliveryDate` in the `Get /fapi/v1/exchangeInfo` endpoint to the delisting time after the delisting announcement is published. Please refer to [Exchange Info](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Exchange-Information) to check the delisting information of contract trading pairs in advance.

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Quarterly Contract Settlement Price

## API Description

Latest price for a symbol or symbols.

## HTTP Request

GET `/futures/data/delivery-price`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| pair | STRING | YES | e.g BTCUSDT |

## Response Example

```javascript
[
    {
        "deliveryTime": 1695945600000,
        "deliveryPrice": 27103.00000000
    },
    {
        "deliveryTime": 1688083200000,
        "deliveryPrice": 30733.60000000
    },
    {
        "deliveryTime": 1680220800000,
        "deliveryPrice": 27814.20000000
    },
    {
        "deliveryTime": 1648166400000,
        "deliveryPrice": 44066.30000000
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Exchange Information

## API Description

Current exchange trading rules and symbol information

## HTTP Request

GET `/fapi/v1/exchangeInfo`

## Request Weight

**1**

## Request Parameters

NONE

## Response Example

```javascript
{
	"exchangeFilters": [],
 	"rateLimits": [
 		{
 			"interval": "MINUTE",
   			"intervalNum": 1,
   			"limit": 2400,
   			"rateLimitType": "REQUEST_WEIGHT"
   		},
  		{
  			"interval": "MINUTE",
   			"intervalNum": 1,
   			"limit": 1200,
   			"rateLimitType": "ORDERS"
   		}
   	],
 	"serverTime": 1565613908500,    // Ignore please. If you want to check current server time, please check via "GET /fapi/v1/time"
 	"assets": [ // assets information
 		{
 			"asset": "BTC",
   			"marginAvailable": true, // whether the asset can be used as margin in Multi-Assets mode
   			"autoAssetExchange": "-0.10" // auto-exchange threshold in Multi-Assets margin mode
   		},
 		{
 			"asset": "USDT",
   			"marginAvailable": true,
   			"autoAssetExchange": "0"
   		},
 		{
 			"asset": "BNB",
   			"marginAvailable": false,
   			"autoAssetExchange": null
   		}
   	],
 	"symbols": [
 		{
 			"symbol": "BLZUSDT",
 			"pair": "BLZUSDT",
 			"contractType": "PERPETUAL",
 			"deliveryDate": 4133404800000,
 			"onboardDate": 1598252400000,
 			"status": "TRADING",
 			"maintMarginPercent": "2.5000",   // ignore
 			"requiredMarginPercent": "5.0000",  // ignore
 			"baseAsset": "BLZ",
 			"quoteAsset": "USDT",
 			"marginAsset": "USDT",
 			"pricePrecision": 5,	// please do not use it as tickSize
 			"quantityPrecision": 0, // please do not use it as stepSize
 			"baseAssetPrecision": 8,
 			"quotePrecision": 8,
 			"underlyingType": "COIN",
 			"underlyingSubType": ["STORAGE"],
 			"settlePlan": 0,
 			"triggerProtect": "0.15", // threshold for algo order with "priceProtect"
 			"filters": [
 				{
 					"filterType": "PRICE_FILTER",
     				"maxPrice": "300",
     				"minPrice": "0.0001",
     				"tickSize": "0.0001"
     			},
    			{
    				"filterType": "LOT_SIZE",
     				"maxQty": "10000000",
     				"minQty": "1",
     				"stepSize": "1"
     			},
    			{
    				"filterType": "MARKET_LOT_SIZE",
     				"maxQty": "590119",
     				"minQty": "1",
     				"stepSize": "1"
     			},
     			{
    				"filterType": "MAX_NUM_ORDERS",
    				"limit": 200
  				},
  				{
  					"filterType": "MIN_NOTIONAL",
  					"notional": "5.0",
  				},
  				{
    				"filterType": "PERCENT_PRICE",
    				"multiplierUp": "1.1500",
    				"multiplierDown": "0.8500",
    				"multiplierDecimal": "4"
    			}
   			],
 			"orderTypes": [
   				"LIMIT",
   				"MARKET",
   				"STOP",
   				"STOP_MARKET",
   				"TAKE_PROFIT",
   				"TAKE_PROFIT_MARKET",
   				"TRAILING_STOP_MARKET"
   			],
   			"timeInForce": [
   				"GTC",
   				"IOC",
   				"FOK",
   				"GTX"
 			],
 			"liquidationFee": "0.010000",	// liquidation fee rate
   			"marketTakeBound": "0.30",	// the max price difference rate( from mark price) a market order can make
 		}
   	],
	"timezone": "UTC"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Funding Rate History

## API Description

Get Funding Rate History

## HTTP Request

GET `/fapi/v1/fundingRate`

## Request Weight

share 500/5min/IP rate limit with GET /fapi/v1/fundingInfo

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| startTime | LONG | NO | Timestamp in ms to get funding rate from INCLUSIVE. |
| endTime | LONG | NO | Timestamp in ms to get funding rate until INCLUSIVE. |
| limit | INT | NO | Default 100; max 1000 |

> - If `startTime` and `endTime` are not sent, the most recent 200 records are returned.
> - If the number of data between `startTime` and `endTime` is larger than `limit`, return as `startTime` \+ `limit`.
> - In ascending order.

## Response Example

```javascript
[
	{
    	"symbol": "BTCUSDT",
    	"fundingRate": "-0.03750000",
    	"fundingTime": 1570608000000,
		"markPrice": "34287.54619963"   // mark price associated with a particular funding fee charge
	},
	{
   		"symbol": "BTCUSDT",
    	"fundingRate": "0.00010000",
    	"fundingTime": 1570636800000,
		"markPrice": "34287.54619963"
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Funding Rate Info

## API Description

Query funding rate info for symbols that had FundingRateCap/ FundingRateFloor / fundingIntervalHours adjustment

## HTTP Request

GET `/fapi/v1/fundingInfo`

## Request Weight

**0**

share 500/5min/IP rate limit with `GET /fapi/v1/fundingRate`

## Request Parameters

## Response Example

```javascript
[
    {
        "symbol": "BLZUSDT",
        "adjustedFundingRateCap": "0.02500000",
        "adjustedFundingRateFloor": "-0.02500000",
        "fundingIntervalHours": 8,
        "disclaimer": false   // ingore
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Index Price Constituents

## API Description

Query index price constituents

**Note**:

> Prices from constituents of TradFi perps will be hiden and displayed as -1.

## HTTP Request

GET `/fapi/v1/constituents`

## Request Weight

**2**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |

## Response Example

```javascript
{
    "symbol": "BTCUSDT",
    "time": 1745401553408,
    "constituents": [
        {
            "exchange": "binance",
            "symbol": "BTCUSDT",
            "price": "94057.03000000",
            "weight": "0.51282051"
        },
        {
            "exchange": "coinbase",
            "symbol": "BTC-USDT",
            "price": "94140.58000000",
            "weight": "0.15384615"
        },
        {
            "exchange": "gateio",
            "symbol": "BTC_USDT",
            "price": "94060.10000000",
            "weight": "0.02564103"
        },
        {
            "exchange": "kucoin",
            "symbol": "BTC-USDT",
            "price": "94096.70000000",
            "weight": "0.07692308"
        },
        {
            "exchange": "mxc",
            "symbol": "BTCUSDT",
            "price": "94057.02000000",
            "weight": "0.07692308"
        },
        {
            "exchange": "bitget",
            "symbol": "BTCUSDT",
            "price": "94064.03000000",
            "weight": "0.07692308"
        },
        {
            "exchange": "bybit",
            "symbol": "BTCUSDT",
            "price": "94067.90000000",
            "weight": "0.07692308"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Index Price Kline/Candlestick Data

## API Description

Kline/candlestick bars for the index price of a pair.
Klines are uniquely identified by their open time.

## HTTP Request

GET `/fapi/v1/indexPriceKlines`

## Request Weight

based on parameter `LIMIT`

| LIMIT | weight |
| --- | --- |
| \[1,100) | 1 |
| \[100, 500) | 2 |
| \[500, 1000\] | 5 |
| \> 1000 | 10 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| pair | STRING | YES |  |
| interval | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1500. |

- If startTime and endTime are not sent, the most recent klines are returned.

## Response Example

```javascript
[
  [
    1591256400000,      	// Open time
    "9653.69440000",    	// Open
    "9653.69640000",     	// High
    "9651.38600000",     	// Low
    "9651.55200000",     	// Close (or latest price)
    "0	", 					// Ignore
    1591256459999,      	// Close time
    "0",    				// Ignore
    60,                		// Ignore
    "0",    				// Ignore
    "0",      				// Ignore
    "0" 					// Ignore
  ]
]
```

 

- [API Description](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Index-Price-Kline-Candlestick-Data#api-description)
- [HTTP Request](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Index-Price-Kline-Candlestick-Data#http-request)
- [Request Weight](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Index-Price-Kline-Candlestick-Data#request-weight)
- [Request Parameters](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Index-Price-Kline-Candlestick-Data#request-parameters)
- [Response Example](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Index-Price-Kline-Candlestick-Data#response-example)

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Insurance Fund Balance Snapshot

## API Description

Query Insurance Fund Balance Snapshot

## HTTP Request

GET `/fapi/v1/insuranceBalance`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

## Response Example

pass symbol

```javascript
{
   "symbols":[
      "BNBUSDT",
      "BTCUSDT",
      "BTCUSDT_250627",
      "BTCUSDT_250926",
      "ETHBTC",
      "ETHUSDT",
      "ETHUSDT_250627",
      "ETHUSDT_250926"
   ],
   "assets":[
      {
         "asset":"USDC",
         "marginBalance":"299999998.6497832",
         "updateTime":1745366402000
      },
      {
         "asset":"USDT",
         "marginBalance":"793930579.315848",
         "updateTime":1745366402000
      },
      {
         "asset":"BTC",
         "marginBalance":"61.73143554",
         "updateTime":1745366402000
      },
      {
         "asset":"BNFCR",
         "marginBalance":"633223.99396922",
         "updateTime":1745366402000
      }
   ]
}
```

> or not pass symbol

```javascript
[
   {
      "symbols":[
         "ADAUSDT",
         "BCHUSDT",
         "DOTUSDT",
         "EOSUSDT",
         "ETCUSDT",
         "LINKUSDT",
         "LTCUSDT",
         "TRXUSDT",
         "XLMUSDT",
         "XMRUSDT",
         "XRPUSDT"
      ],
      "assets":[
         {
            "asset":"USDT",
            "marginBalance":"314151411.06482935",
            "updateTime":1745366402000
         }
      ]
   },
   {
      "symbols":[
         "ACTUSDT",
         "MUBARAKUSDT",
         "OMUSDT",
         "TSTUSDT"
      ],
      "assets":[
         {
            "asset":"USDT",
            "marginBalance":"5166686.84431694",
            "updateTime":1745366402000
         }
      ]
   }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Kline/Candlestick Data

## API Description

Kline/candlestick bars for a symbol.
Klines are uniquely identified by their open time.

## HTTP Request

GET `/fapi/v1/klines`

## Request Weight

based on parameter `LIMIT`

| LIMIT | weight |
| --- | --- |
| \[1,100) | 1 |
| \[100, 500) | 2 |
| \[500, 1000\] | 5 |
| \> 1000 | 10 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| interval | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1500. |

> - If startTime and endTime are not sent, the most recent klines are returned.

## Response Example

```javascript
[
  [
    1499040000000,      // Open time
    "0.01634790",       // Open
    "0.80000000",       // High
    "0.01575800",       // Low
    "0.01577100",       // Close
    "148976.11427815",  // Volume
    1499644799999,      // Close time
    "2434.19055334",    // Quote asset volume
    308,                // Number of trades
    "1756.87402397",    // Taker buy base asset volume
    "28.46694368",      // Taker buy quote asset volume
    "17928899.62484339" // Ignore.
  ]
]
```

 

- [API Description](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Kline-Candlestick-Data#api-description)
- [HTTP Request](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Kline-Candlestick-Data#http-request)
- [Request Weight](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Kline-Candlestick-Data#request-weight)
- [Request Parameters](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Kline-Candlestick-Data#request-parameters)
- [Response Example](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Kline-Candlestick-Data#response-example)

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Long/Short Ratio

## API Description

Query symbol Long/Short Ratio

## HTTP Request

GET `/futures/data/globalLongShortAccountRatio`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | NO | default 30, max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 30 days is available.
> - IP rate limit 1000 requests/5min

## Response Example

```javascript
[
    {
         "symbol":"BTCUSDT",  // long/short account num ratio of all traders
	      "longShortRatio":"0.1960",  //long account num ratio of all traders
	      "longAccount": "0.6622",   // short account num ratio of all traders
	      "shortAccount":"0.3378",
	      "timestamp":"1583139600000"

     },

     {

         "symbol":"BTCUSDT",
	      "longShortRatio":"1.9559",
	      "longAccount": "0.6617",
	      "shortAccount":"0.3382",
	      "timestamp":"1583139900000"

        },

]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Mark Price

## API Description

Mark Price and Funding Rate

## HTTP Request

GET `/fapi/v1/premiumIndex`

## Request Weight

**1** with symbol, **10** without symbol

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

## Response Example

> **Response:**

```javascript
{
	"symbol": "BTCUSDT",
	"markPrice": "11793.63104562",	// mark price
	"indexPrice": "11781.80495970",	// index price
	"estimatedSettlePrice": "11781.16138815", // Estimated Settle Price, only useful in the last hour before the settlement starts.
	"lastFundingRate": "0.00038246",  // This is the Latest funding rate
	"interestRate": "0.00010000",
	"nextFundingTime": 1597392000000,
	"time": 1597370495002
}
```

> **OR (when symbol not sent)**

```javascript
[
	{
	    "symbol": "BTCUSDT",
	    "markPrice": "11793.63104562",	// mark price
	    "indexPrice": "11781.80495970",	// index price
	    "estimatedSettlePrice": "11781.16138815", // Estimated Settle Price, only useful in the last hour before the settlement starts.
	    "lastFundingRate": "0.00038246",  // This is the Latest funding rate
	    "interestRate": "0.00010000",
	    "nextFundingTime": 1597392000000,
	    "time": 1597370495002
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Mark Price Kline/Candlestick Data

## API Description

Kline/candlestick bars for the mark price of a symbol.
Klines are uniquely identified by their open time.

## HTTP Request

GET `/fapi/v1/markPriceKlines`

## Request Weight

based on parameter `LIMIT`

| LIMIT | weight |
| --- | --- |
| \[1,100) | 1 |
| \[100, 500) | 2 |
| \[500, 1000\] | 5 |
| \> 1000 | 10 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| interval | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1500. |

> - If startTime and endTime are not sent, the most recent klines are returned.

## Response Example

```javascript
[
  [
    1591256460000,     		// Open time
    "9653.29201333",    	// Open
    "9654.56401333",     	// High
    "9653.07367333",     	// Low
    "9653.07367333",     	// Close (or latest price)
    "0	", 					// Ignore
    1591256519999,      	// Close time
    "0",    				// Ignore
    60,                	 	// Ignore
    "0",    				// Ignore
    "0",      			 	// Ignore
    "0" 					// Ignore
  ]
]
```

 

- [API Description](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Mark-Price-Kline-Candlestick-Data#api-description)
- [HTTP Request](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Mark-Price-Kline-Candlestick-Data#http-request)
- [Request Weight](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Mark-Price-Kline-Candlestick-Data#request-weight)
- [Request Parameters](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Mark-Price-Kline-Candlestick-Data#request-parameters)
- [Response Example](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Mark-Price-Kline-Candlestick-Data#response-example)

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Multi-Assets Mode Asset Index

## API Description

asset index for Multi-Assets mode

## HTTP Request

GET `/fapi/v1/assetIndex`

## Request Weight

**1** for a single symbol; **10** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO | Asset pair |

## Response Example

> **Response:**

```javascript
{
	"symbol": "ADAUSD",
	"time": 1635740268004,
	"index": "1.92957370",
	"bidBuffer": "0.10000000",
	"askBuffer": "0.10000000",
	"bidRate": "1.73661633",
	"askRate": "2.12253107",
	"autoExchangeBidBuffer": "0.05000000",
	"autoExchangeAskBuffer": "0.05000000",
	"autoExchangeBidRate": "1.83309501",
	"autoExchangeAskRate": "2.02605238"
}
```

> Or(without symbol)

```javascript
[
	{
		"symbol": "ADAUSD",
		"time": 1635740268004,
		"index": "1.92957370",
		"bidBuffer": "0.10000000",
		"askBuffer": "0.10000000",
		"bidRate": "1.73661633",
		"askRate": "2.12253107",
		"autoExchangeBidBuffer": "0.05000000",
		"autoExchangeAskBuffer": "0.05000000",
		"autoExchangeBidRate": "1.83309501",
		"autoExchangeAskRate": "2.02605238"
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Old Trades Lookup (MARKET\_DATA)

## API Description

Get older market historical trades.

## HTTP Request

GET `/fapi/v1/historicalTrades`

## Request Weight

**20**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 100; max 500. |
| fromId | LONG | NO | TradeId to fetch from. Default gets most recent trades. |

> - Market trades means trades filled in the order book. Only market trades will be returned, which means the insurance fund trades and ADL trades won't be returned.
> - Only supports data from within the last one month

## Response Example

```javascript
[
  {
    "id": 28457,
    "price": "4.00000100",
    "qty": "12.00000000",
    "quoteQty": "8000.00",
    "time": 1499865549590,
    "isBuyerMaker": true,
    "isRPITrade": true,
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Open Interest

## API Description

Get present open interest of a specific symbol.

## HTTP Request

GET `/fapi/v1/openInterest`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |

## Response Example

```javascript
{
	"openInterest": "10659.509",
	"symbol": "BTCUSDT",
	"time": 1589437530011   // Transaction time
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Open Interest Statistics

## API Description

Open Interest Statistics

## HTTP Request

GET `/futures/data/openInterestHist`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | NO | default 30, max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 1 month is available.
> - IP rate limit 1000 requests/5min

## Response Example

```javascript
[
    {
         "symbol":"BTCUSDT",
	      "sumOpenInterest":"20403.63700000",  // total open interest
	      "sumOpenInterestValue": "150570784.07809979",   // total open interest value
          "CMCCirculatingSupply": "165880.538", // circulating supply provided by CMC
	      "timestamp":"1583127900000"
    },
    {
         "symbol":"BTCUSDT",
         "sumOpenInterest":"20401.36700000",
         "sumOpenInterestValue":"149940752.14464448",
         "CMCCirculatingSupply": "165900.14853",
         "timestamp":"1583128200000"
    },
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Order Book

## API Description

Query symbol orderbook

## HTTP Request

GET `/fapi/v1/depth`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Request Weight

Adjusted based on the limit:

| Limit | Weight |
| --- | --- |
| 5, 10, 20, 50 | 2 |
| 100 | 5 |
| 500 | 10 |
| 1000 | 20 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 500; Valid limits:\[5, 10, 20, 50, 100, 500, 1000\] |

## Response Example

```javascript
{
  "lastUpdateId": 1027024,
  "E": 1589436922972,   // Message output time
  "T": 1589436922959,   // Transaction time
  "bids": [
    [
      "4.00000000",     // PRICE
      "431.00000000"    // QTY
    ]
  ],
  "asks": [
    [
      "4.00000200",
      "12.00000000"
    ]
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# RPI Order Book

## API Description

Query symbol orderbook with RPI orders

## HTTP Request

GET `/fapi/v1/rpiDepth`

**Note**:

> RPI(Retail Price Improvement) orders are included and aggreated in the response message. Crossed price levels are hidden and invisible.

## Request Weight

Adjusted based on the limit:

| Limit | Weight |
| --- | --- |
| 1000 | 20 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 1000; Valid limits:\[1000\] |

## Response Example

```javascript
{
  "lastUpdateId": 1027024,
  "E": 1589436922972,   // Message output time
  "T": 1589436922959,   // Transaction time
  "bids": [
    [
      "4.00000000",     // PRICE
      "431.00000000"    // QTY
    ]
  ],
  "asks": [
    [
      "4.00000200",
      "12.00000000"
    ]
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Premium index Kline Data

## API Description

Premium index kline bars of a symbol. Klines are uniquely identified by their open time.

## HTTP Request

GET `/fapi/v1/premiumIndexKlines`

## Request Weight

based on parameter `LIMIT`

| LIMIT | weight |
| --- | --- |
| \[1,100) | 1 |
| \[100, 500) | 2 |
| \[500, 1000\] | 5 |
| \> 1000 | 10 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| interval | ENUM | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1500. |

> - If startTime and endTime are not sent, the most recent klines are returned.

## Response Example

```javascript
[
  [
    1691603820000,          // Open time
    "-0.00042931",          // Open
    "-0.00023641",          // High
    "-0.00059406",          // Low
    "-0.00043659",          // Close
    "0",                    // Ignore
    1691603879999,          // Close time
    "0",                    // Ignore
    12,                     // Ignore
    "0",                    // Ignore
    "0",                    // Ignore
    "0"                     // Ignore
  ]
]
```

 

- [API Description](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Premium-Index-Kline-Data#api-description)
- [HTTP Request](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Premium-Index-Kline-Data#http-request)
- [Request Weight](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Premium-Index-Kline-Data#request-weight)
- [Request Parameters](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Premium-Index-Kline-Data#request-parameters)
- [Response Example](https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Premium-Index-Kline-Data#response-example)

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Recent Trades List

## API Description

Get recent market trades

## HTTP Request

GET `/fapi/v1/trades`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 500; max 1000. |

> - Market trades means trades filled in the order book. Only market trades will be returned, which means the insurance fund trades and ADL trades won't be returned.

## Response Example

```javascript
[
  {
    "id": 28457,
    "price": "4.00000100",
    "qty": "12.00000000",
    "quoteQty": "48.00",
    "time": 1499865549590,
    "isBuyerMaker": true,
    "isRPITrade": true,
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Order Book Ticker

## API Description

Best price/qty on the order book for a symbol or symbols.

## HTTP Request

GET `/fapi/v1/ticker/bookTicker`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Request Weight

**2** for a single symbol;

**5** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, bookTickers for all symbols will be returned in an array.
> - The field `X-MBX-USED-WEIGHT-1M` in response header is not accurate from this endpoint, please ignore.

## Response Example

```javascript
{
  "symbol": "BTCUSDT",
  "bidPrice": "4.00000000",
  "bidQty": "431.00000000",
  "askPrice": "4.00000200",
  "askQty": "9.00000000",
  "time": 1589437530011   // Transaction time
}
```

> OR

```javascript
[
	{
  		"symbol": "BTCUSDT",
  		"bidPrice": "4.00000000",
  		"bidQty": "431.00000000",
  		"askPrice": "4.00000200",
  		"askQty": "9.00000000",
  		"time": 1589437530011
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Price Ticker(Deprecated)

## API Description

Latest price for a symbol or symbols.

## HTTP Request

GET `/fapi/v1/ticker/price`

**Weight:**

**1** for a single symbol;

**2** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, prices for all symbols will be returned in an array.

## Response Example

```javascript
{
  "symbol": "BTCUSDT",
  "price": "6000.01",
  "time": 1589437530011   // Transaction time
}
```

> OR

```javascript
[
	{
  		"symbol": "BTCUSDT",
  		"price": "6000.01",
  		"time": 1589437530011
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Price Ticker V2

## API Description

Latest price for a symbol or symbols.

## HTTP Request

GET `/fapi/v2/ticker/price`

**Weight:**

**1** for a single symbol;

**2** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, prices for all symbols will be returned in an array.
> - The field `X-MBX-USED-WEIGHT-1M` in response header is not accurate from this endpoint, please ignore.

## Response Example

```javascript
{
  "symbol": "BTCUSDT",
  "price": "6000.01",
  "time": 1589437530011   // Transaction time
}
```

> OR

```javascript
[
	{
  		"symbol": "BTCUSDT",
  		"price": "6000.01",
  		"time": 1589437530011
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Taker Buy/Sell Volume

## API Description

Taker Buy/Sell Volume

## HTTP Request

GET `/futures/data/takerlongshortRatio`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | NO | default 30, max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 30 days is available.
> - IP rate limit 1000 requests/5min

## Response Example

```javascript
[
    {
	    "buySellRatio":"1.5586",
	    "buyVol": "387.3300",
	    "sellVol":"248.5030",
	    "timestamp":"1585614900000"
    },
    {
	    "buySellRatio":"1.3104",
	    "buyVol": "343.9290",
	    "sellVol":"248.5030",
	    "timestamp":"1583139900000"
    },
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Test Connectivity

## API Description

Test connectivity to the Rest API.

## HTTP Request

GET `/fapi/v1/ping`

## Request Weight

1

## Request Parameters

NONE

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Top Trader Long/Short Ratio (Accounts)

## API Description

The proportion of net long and net short accounts to total accounts of the top 20% users with the highest margin balance. Each account is counted once only.
Long Account % = Accounts of top traders with net long positions / Total accounts of top traders with open positions
Short Account % = Accounts of top traders with net short positions / Total accounts of top traders with open positions
Long/Short Ratio (Accounts) = Long Account % / Short Account %

## HTTP Request

GET `/futures/data/topLongShortAccountRatio`

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | NO | default 30, max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 30 days is available.
> - IP rate limit 1000 requests/5min

## Response Example

```javascript
[
    {
         "symbol":"BTCUSDT",
	      "longShortRatio":"1.8105",  // long/short account num ratio of top traders
	      "longAccount": "0.6442",   // long account num ratio of top traders
	      "shortAccount":"0.3558",   // long account num ratio of top traders
	      "timestamp":"1583139600000"
    },
    {
         "symbol":"BTCUSDT",
	      "longShortRatio":"0.5576",
	      "longAccount": "0.3580",
	      "shortAccount":"0.6420",
	      "timestamp":"1583139900000"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Top Trader Long/Short Ratio (Positions)

## API Description

The proportion of net long and net short positions to total open positions of the top 20% users with the highest margin balance.
Long Position % = Long positions of top traders / Total open positions of top traders
Short Position % = Short positions of top traders / Total open positions of top traders
Long/Short Ratio (Positions) = Long Position % / Short Position %

## HTTP Request

GET `/futures/data/topLongShortPositionRatio`

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| period | ENUM | YES | "5m","15m","30m","1h","2h","4h","6h","12h","1d" |
| limit | LONG | NO | default 30, max 500 |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |

> - If startTime and endTime are not sent, the most recent data is returned.
> - Only the data of the latest 30 days is available.
> - IP rate limit 1000 requests/5min

## Response Example

```javascript
[
    {
         "symbol":"BTCUSDT",
	      "longShortRatio":"1.4342",// long/short position ratio of top traders
	      "longAccount": "0.5891", // long positions ratio of top traders
	      "shortAccount":"0.4108", // short positions ratio of top traders
	      "timestamp":"1583139600000"

     },

     {

         "symbol":"BTCUSDT",
	      "longShortRatio":"1.4337",
	      "longAccount": "0.3583",
	      "shortAccount":"0.6417",
	      "timestamp":"1583139900000"

        },

]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Trading Schedule

## API Description

Trading session schedules for the underlying assets of TradFi Perps are provided for a one-week period starting from the day prior to the query time, covering both the U.S. equity and commodity markets. Equity market session types include "PRE\_MARKET", "REGULAR", "AFTER\_MARKET", "OVERNIGHT", and "NO\_TRADING", while commodity market session types include "REGULAR" and "NO\_TRADING".

## HTTP Request

GET `/fapi/v1/tradingSchedule`

## Request Weight

**5**

## Request Parameters

NONE

## Response Example

```javascript
{
  "updateTime": 1761286643918,
  "marketSchedules": {
    "EQUITY": {
      "sessions": [
        {
          "startTime": 1761177600000,
          "endTime": 1761206400000,
          "type": "OVERNIGHT"
        },
        {
          "startTime": 1761206400000,
          "endTime": 1761226200000,
          "type": "PRE_MARKET"
        }
      ]
    },
    "COMMODITY": {
      "sessions": [
        {
          "startTime": 1761724800000,
          "endTime": 1761744600000,
          "type": "NO_TRADING"
        },
        {
          "startTime": 1761744600000,
          "endTime": 1761768000000,
          "type": "REGULAR"
        }
      ]
    }
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Order Book

## API Description

Get current order book. Note that this request returns limited market depth.
If you need to continuously monitor order book updates, please consider using Websocket Market Streams:

- `<symbol>@depth<levels>`
- `<symbol>@depth`

You can use `depth` request together with `<symbol>@depth` streams to maintain a local order book.

## Method

`depth`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Request

```javascript
{
    "id": "51e2affb-0aba-4821-ba75-f2625006eb43",
    "method": "depth",
    "params": {
      "symbol": "BTCUSDT"
    }
}
```

## Request Weight

Adjusted based on the limit:

| Limit | Weight |
| --- | --- |
| 5, 10, 20, 50 | 2 |
| 100 | 5 |
| 500 | 10 |
| 1000 | 20 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 500; Valid limits:\[5, 10, 20, 50, 100, 500, 1000\] |

## Response Example

```javascript
{
  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",
  "status": 200,
  "result": {
    "lastUpdateId": 1027024,
    "E": 1589436922972,   // Message output time
    "T": 1589436922959,   // Transaction time
    "bids": [
      [
        "4.00000000",     // PRICE
        "431.00000000"    // QTY
      ]
    ],
    "asks": [
      [
        "4.00000200",
        "12.00000000"
      ]
    ]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 5
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Order Book

## API Description

Get current order book. Note that this request returns limited market depth.
If you need to continuously monitor order book updates, please consider using Websocket Market Streams:

- `<symbol>@depth<levels>`
- `<symbol>@depth`

You can use `depth` request together with `<symbol>@depth` streams to maintain a local order book.

## Method

`depth`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Request

```javascript
{
    "id": "51e2affb-0aba-4821-ba75-f2625006eb43",
    "method": "depth",
    "params": {
      "symbol": "BTCUSDT"
    }
}
```

## Request Weight

Adjusted based on the limit:

| Limit | Weight |
| --- | --- |
| 5, 10, 20, 50 | 2 |
| 100 | 5 |
| 500 | 10 |
| 1000 | 20 |

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| limit | INT | NO | Default 500; Valid limits:\[5, 10, 20, 50, 100, 500, 1000\] |

## Response Example

```javascript
{
  "id": "51e2affb-0aba-4821-ba75-f2625006eb43",
  "status": 200,
  "result": {
    "lastUpdateId": 1027024,
    "E": 1589436922972,   // Message output time
    "T": 1589436922959,   // Transaction time
    "bids": [
      [
        "4.00000000",     // PRICE
        "431.00000000"    // QTY
      ]
    ],
    "asks": [
      [
        "4.00000200",
        "12.00000000"
      ]
    ]
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 5
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Order Book Ticker

## API Description

Best price/qty on the order book for a symbol or symbols.

## Method

`ticker.book`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Request

```javascript
{
    "id": "9d32157c-a556-4d27-9866-66760a174b57",
    "method": "ticker.book",
    "params": {
        "symbol": "BTCUSDT"
    }
}
```

## Request Weight

**2** for a single symbol;

**5** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, bookTickers for all symbols will be returned in an array.
> - The field `X-MBX-USED-WEIGHT-1M` in response header is not accurate from this endpoint, please ignore.

## Response Example

```javascript
{
  "id": "9d32157c-a556-4d27-9866-66760a174b57",
  "status": 200,
  "result": {
    "lastUpdateId": 1027024,
    "symbol": "BTCUSDT",
    "bidPrice": "4.00000000",
    "bidQty": "431.00000000",
    "askPrice": "4.00000200",
    "askQty": "9.00000000",
    "time": 1589437530011   // Transaction time
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

> OR

```javascript
{
  "id": "9d32157c-a556-4d27-9866-66760a174b57",
  "status": 200,
  "result": [
    {
      "lastUpdateId": 1027024,
      "symbol": "BTCUSDT",
      "bidPrice": "4.00000000",
      "bidQty": "431.00000000",
      "askPrice": "4.00000200",
      "askQty": "9.00000000",
      "time": 1589437530011
    }
  ],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Symbol Price Ticker

## API Description

Latest price for a symbol or symbols.

## Method

`ticker.price`

## Request

```javascript
{
   	"id": "9d32157c-a556-4d27-9866-66760a174b57",
    "method": "ticker.price",
    "params": {
        "symbol": "BTCUSDT"
    }
}
```

**Weight:**

**1** for a single symbol;

**2** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |

> - If the symbol is not sent, prices for all symbols will be returned in an array.

## Response Example

```javascript
{
  "id": "9d32157c-a556-4d27-9866-66760a174b57",
  "status": 200,
  "result": {
	"symbol": "BTCUSDT",
	"price": "6000.01",
	"time": 1589437530011   // Transaction time
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

> OR

```javascript
{
  "id": "9d32157c-a556-4d27-9866-66760a174b57",
  "status": 200,
  "result": [
	{
    	"symbol": "BTCUSDT",
      	"price": "6000.01",
      	"time": 1589437530011
  	}
  ],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Classic Portfolio Margin Account Information (USER\_DATA)

## API Description

Get Classic Portfolio Margin current account information.

## HTTP Request

GET `/fapi/v1/pmAccountInfo`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - maxWithdrawAmount is for asset transfer out to the spot wallet.

## Response Example

```javascript
{
	"maxWithdrawAmountUSD": "1627523.32459208",   // Classic Portfolio margin maximum virtual amount for transfer out in USD
	"asset": "BTC",            // asset name
	"maxWithdrawAmount": "27.43689636",        // maximum amount for transfer out
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Classic Portfolio Margin Account Information (USER\_DATA)

## API Description

Get Classic Portfolio Margin current account information.

## HTTP Request

GET `/fapi/v1/pmAccountInfo`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - maxWithdrawAmount is for asset transfer out to the spot wallet.

## Response Example

```javascript
{
	"maxWithdrawAmountUSD": "1627523.32459208",   // Classic Portfolio margin maximum virtual amount for transfer out in USD
	"asset": "BTC",            // asset name
	"maxWithdrawAmount": "27.43689636",        // maximum amount for transfer out
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# New Order(TRADE)

## API Description

Send in a new order.

## HTTP Request

POST `/fapi/v1/order`

## Request Weight

1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
0 on IP rate limit(x-mbx-used-weight-1m)

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `EXPIRE_MAKER` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` |
| `MARKET` | `quantity` |

> - If `newOrderRespType` is sent as `RESULT` :
>
>   - `MARKET` order: the final FILLED result of the order will be return directly.
>   - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.
> - `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.
> - In extreme market conditions, timeInForce `GTD` order auto cancel time might be delayed comparing to `goodTillDate`

## Response Example

```javascript
{
 	"clientOrderId": "testOrder",
 	"cumQty": "0",
 	"cumQuote": "0",
 	"executedQty": "0",
 	"orderId": 22542179,
 	"avgPrice": "0.00000",
 	"origQty": "10",
 	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",		// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"timeInForce": "GTD",
  	"type": "TRAILING_STOP_MARKET",
  	"origType": "TRAILING_STOP_MARKET",
 	"updateTime": 1566818724722,
 	"workingType": "CONTRACT_PRICE",
 	"priceProtect": false,      // if conditional order trigger is protected
 	"priceMatch": "NONE",              //price match mode
 	"selfTradePreventionMode": "NONE", //self trading preventation mode
 	"goodTillDate": 1693207680000      //order pre-set auot cancel time for TIF GTD order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Account Trade List (USER\_DATA)

## API Description

Get trades for a specific account and symbol.

## HTTP Request

GET `/fapi/v1/userTrades`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO | This can only be used in combination with `symbol` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | Trade id to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If `startTime` and `endTime` are both not sent, then the last 7 days' data will be returned.
> - The time between `startTime` and `endTime` cannot be longer than 7 days.
> - The parameter `fromId` cannot be sent with `startTime` or `endTime`.
> - Only support querying trade in the past 6 months

## Response Example

```javascript
[
  {
  	"buyer": false,
  	"commission": "-0.07819010",
  	"commissionAsset": "USDT",
  	"id": 698759,
  	"maker": false,
  	"orderId": 25851813,
  	"price": "7819.01",
  	"qty": "0.002",
  	"quoteQty": "15.63802",
  	"realizedPnl": "-0.91539999",
  	"side": "SELL",
  	"positionSide": "SHORT",
  	"symbol": "BTCUSDT",
  	"time": 1569514978020
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# All Orders (USER\_DATA)

## API Description

Get all account orders; active, canceled, or filled.

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time
  - order create time + 90 days < current time

## HTTP Request

GET `/fapi/v1/allOrders`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Notes:**

> - If `orderId` is set, it will get orders >= that `orderId`. Otherwise most recent orders are returned.
> - The query time period must be less then 7 days( default as the recent 7 days).

## Response Example

```javascript
[
  {
   	"avgPrice": "0.00000",
  	"clientOrderId": "abc",
  	"cumQuote": "0",
  	"executedQty": "0",
  	"orderId": 1917641,
  	"origQty": "0.40",
  	"origType": "TRAILING_STOP_MARKET",
  	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",				// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"time": 1579276756075,				// order time
  	"timeInForce": "GTC",
  	"type": "TRAILING_STOP_MARKET",
  	"activatePrice": "9020",			// activation price, only return with TRAILING_STOP_MARKET order
  	"priceRate": "0.3",					// callback rate, only return with TRAILING_STOP_MARKET order
  	"updateTime": 1579276756075,		// update time
  	"workingType": "CONTRACT_PRICE",
  	"priceProtect": false,              // if conditional order trigger is protected
  	"priceMatch": "NONE",              //price match mode
  	"selfTradePreventionMode": "NONE", //self trading preventation mode
  	"goodTillDate": 0      //order pre-set auot cancel time for TIF GTD order
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Auto-Cancel All Open Orders (TRADE)

## API Description

Cancel all open orders of the specified symbol at the end of the specified countdown.
The endpoint should be called repeatedly as heartbeats so that the existing countdown time can be canceled and replaced by a new one.

> - Example usage:
>
>
>   Call this endpoint at 30s intervals with an countdownTime of 120000 (120s).
>
>
>   If this endpoint is not called within 120 seconds, all your orders of the specified symbol will be automatically canceled.
>
>
>   If this endpoint is called with an countdownTime of 0, the countdown timer will be stopped.

The system will check all countdowns **approximately every 10 milliseconds**, so please note that sufficient redundancy should be considered when using this function. We do not recommend setting the countdown time to be too precise or too small.

## HTTP Request

POST `/fapi/v1/countdownCancelAll`

**Weight:** **10**

**Parameters:**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| countdownTime | LONG | YES | countdown time, 1000 for 1 second. 0 to cancel the timer |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"symbol": "BTCUSDT",
	"countdownTime": "100000"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel Algo Order (TRADE)

## API Description

Cancel an active algo order.

## HTTP Request

DELETE `/fapi/v1/algoOrder`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoId | LONG | NO |  |
| clientAlgoId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `algoId` or `clientAlgoId` must be sent.

## Response Example

```javascript
{
   "algoId": 2146760,
   "clientAlgoId": "6B2I9XVcJpCjqPAJ4YoFX7",
   "code": "200",
   "msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel All Algo Open Orders (TRADE)

## API Description

Cancel All Algo Open Orders

## HTTP Request

DELETE `/fapi/v1/algoOpenOrders`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "The operation of cancel all open order is done."
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel All Open Orders (TRADE)

## API Description

Cancel All Open Orders

## HTTP Request

DELETE `/fapi/v1/allOpenOrders`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "The operation of cancel all open order is done."
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel Multiple Orders (TRADE)

## API Description

Cancel Multiple Orders

## HTTP Request

DELETE `/fapi/v1/batchOrders`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderIdList | LIST<LONG> | NO | max length 10 <br> e.g. \[1234567,2345678\] |
| origClientOrderIdList | LIST<STRING> | NO | max length 10<br> e.g. \["my\_id\_1","my\_id\_2"\], encode the double quotes. No space after comma. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderIdList` or `origClientOrderIdList` must be sent.

## Response Example

```javascript
[
	{
	 	"clientOrderId": "myOrder1",
	 	"cumQty": "0",
	 	"cumQuote": "0",
	 	"executedQty": "0",
	 	"orderId": 283194212,
	 	"origQty": "11",
	 	"origType": "TRAILING_STOP_MARKET",
  		"price": "0",
  		"reduceOnly": false,
  		"side": "BUY",
  		"positionSide": "SHORT",
  		"status": "CANCELED",
  		"stopPrice": "9300",				// please ignore when order type is TRAILING_STOP_MARKET
  		"closePosition": false,   // if Close-All
  		"symbol": "BTCUSDT",
  		"timeInForce": "GTC",
  		"type": "TRAILING_STOP_MARKET",
  		"activatePrice": "9020",			// activation price, only return with TRAILING_STOP_MARKET order
  		"priceRate": "0.3",					// callback rate, only return with TRAILING_STOP_MARKET order
	 	"updateTime": 1571110484038,
	 	"workingType": "CONTRACT_PRICE",
	 	"priceProtect": false,            // if conditional order trigger is protected
	 	"priceMatch": "NONE",              //price match mode
	 	"selfTradePreventionMode": "NONE", //self trading preventation mode
	 	"goodTillDate": 1693207680000      //order pre-set auot cancel time for TIF GTD order
	},
	{
		"code": -2011,
		"msg": "Unknown order sent."
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel Order (TRADE)

## API Description

Cancel an active order.

## HTTP Request

DELETE `/fapi/v1/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent.

## Response Example

```javascript
{
 	"clientOrderId": "myOrder1",
 	"cumQty": "0",
 	"cumQuote": "0",
 	"executedQty": "0",
 	"orderId": 283194212,
 	"origQty": "11",
 	"origType": "TRAILING_STOP_MARKET",
  	"price": "0",
  	"avgPrice": "0.00",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "CANCELED",
  	"stopPrice": "9300",				// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"timeInForce": "GTC",
  	"type": "TRAILING_STOP_MARKET",
  	"activatePrice": "9020",			// activation price, only return with TRAILING_STOP_MARKET order
  	"priceRate": "0.3",					// callback rate, only return with TRAILING_STOP_MARKET order
 	"updateTime": 1571110484038,
 	"workingType": "CONTRACT_PRICE",
 	"priceProtect": false,            // if conditional order trigger is protected
	"priceMatch": "NONE",              //price match mode
	"selfTradePreventionMode": "NONE", //self trading preventation mode
	"goodTillDate": 1693207680000      //order pre-set auot cancel time for TIF GTD order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Change Initial Leverage(TRADE)

## API Description

Change user's initial leverage of specific symbol market.

## HTTP Request

POST `/fapi/v1/leverage`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| leverage | INT | YES | target initial leverage: int from 1 to 125 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
 	"leverage": 21,
 	"maxNotionalValue": "1000000",
 	"symbol": "BTCUSDT"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Change Margin Type(TRADE)

## API Description

Change symbol level margin type

## HTTP Request

POST `/fapi/v1/marginType`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| marginType | ENUM | YES | ISOLATED, CROSSED |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Change Multi-Assets Mode (TRADE)

## API Description

Change user's Multi-Assets mode (Multi-Assets Mode or Single-Asset Mode) on _**Every symbol**_

## HTTP Request

POST `/fapi/v1/multiAssetsMargin`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| multiAssetsMargin | STRING | YES | "true": Multi-Assets Mode; "false": Single-Asset Mode |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Change Position Mode(TRADE)

## API Description

Change user's position mode (Hedge Mode or One-way Mode ) on _**EVERY symbol**_

## HTTP Request

POST `/fapi/v1/positionSide/dual`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| dualSidePosition | STRING | YES | "true": Hedge Mode; "false": One-way Mode |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
	"code": 200,
	"msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Current All Algo Open Orders (USER\_DATA)

## API Description

Get all algo open orders on a symbol.

## HTTP Request

GET `/fapi/v1/openAlgoOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

**Careful** when accessing this with no symbol.

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoType | STRING | NO |  |
| symbol | STRING | NO |  |
| algoId | LONG | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If the symbol is not sent, orders for all symbols will be returned in an array.

## Response Example

```javascript
[
   {
       "algoId": 2148627,
       "clientAlgoId": "MRumok0dkhrP4kCm12AHaB",
       "algoType": "CONDITIONAL",
       "orderType": "TAKE_PROFIT",
       "symbol": "BNBUSDT",
       "side": "SELL",
       "positionSide": "BOTH",
       "timeInForce": "GTC",
       "quantity": "0.01",
       "algoStatus": "NEW",
       "actualOrderId": "",
       "actualPrice": "0.00000",
       "triggerPrice": "750.000",
       "price": "750.000",
       "icebergQuantity": null,
       "tpTriggerPrice": "0.000",
       "tpPrice": "0.000",
	   "slTriggerPrice": "0.000",
	   "slPrice": "0.000",
       "tpOrderType": "",
       "selfTradePreventionMode": "EXPIRE_MAKER",
       "workingType": "CONTRACT_PRICE",
       "priceMatch": "NONE",
       "closePosition": false,
       "priceProtect": false,
       "reduceOnly": false,
       "createTime": 1750514941540,
       "updateTime": 1750514941540,
       "triggerTime": 0,
       "goodTillDate": 0
   }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Current All Open Orders (USER\_DATA)

## API Description

Get all open orders on a symbol.

## HTTP Request

GET `/fapi/v1/openOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

**Careful** when accessing this with no symbol.

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If the symbol is not sent, orders for all symbols will be returned in an array.

## Response Example

```javascript
[
  {
  	"avgPrice": "0.00000",
  	"clientOrderId": "abc",
  	"cumQuote": "0",
  	"executedQty": "0",
  	"orderId": 1917641,
  	"origQty": "0.40",
  	"origType": "TRAILING_STOP_MARKET",
  	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",				// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"time": 1579276756075,				// order time
  	"timeInForce": "GTC",
  	"type": "TRAILING_STOP_MARKET",
  	"activatePrice": "9020",			// activation price, only return with TRAILING_STOP_MARKET order
  	"priceRate": "0.3",					// callback rate, only return with TRAILING_STOP_MARKET order
  	"updateTime": 1579276756075,		// update time
  	"workingType": "CONTRACT_PRICE",
  	"priceProtect": false,            // if conditional order trigger is protected
	"priceMatch": "NONE",              //price match mode
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0      //order pre-set auot cancel time for TIF GTD order
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Order Modify History (USER\_DATA)

## API Description

Get order modification history

## HTTP Request

GET `/fapi/v1/orderAmendment`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| startTime | LONG | NO | Timestamp in ms to get modification history from INCLUSIVE |
| endTime | LONG | NO | Timestamp in ms to get modification history until INCLUSIVE |
| limit | INT | NO | Default 50; max 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.
> - Order modify history longer than 3 month is not avaliable

## Response Example

```javascript
[
    {
        "amendmentId": 5363,	// Order modification ID
        "symbol": "BTCUSDT",
        "pair": "BTCUSDT",
        "orderId": 20072994037,
        "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
        "time": 1629184560899,	// Order modification time
        "amendment": {
            "price": {
                "before": "30004",
                "after": "30003.2"
            },
            "origQty": {
                "before": "1",
                "after": "1"
            },
            "count": 3	// Order modification count, representing the number of times the order has been modified
        }
    },
    {
        "amendmentId": 5361,
        "symbol": "BTCUSDT",
        "pair": "BTCUSDT",
        "orderId": 20072994037,
        "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
        "time": 1629184533946,
        "amendment": {
            "price": {
                "before": "30005",
                "after": "30004"
            },
            "origQty": {
                "before": "1",
                "after": "1"
            },
            "count": 2
        }
    },
    {
        "amendmentId": 5325,
        "symbol": "BTCUSDT",
        "pair": "BTCUSDT",
        "orderId": 20072994037,
        "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
        "time": 1629182711787,
        "amendment": {
            "price": {
                "before": "30002",
                "after": "30005"
            },
            "origQty": {
                "before": "1",
                "after": "1"
            },
            "count": 1
        }
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Get Position Margin Change History (TRADE)

## API Description

Get Position Margin Change History

## HTTP Request

GET `/fapi/v1/positionMargin/history`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| type | INT | NO | 1: Add position margin，2: Reduce position margin |
| startTime | LONG | NO |  |
| endTime | LONG | NO | Default current time if not pass |
| limit | INT | NO | Default: 500 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Support querying future histories that are not older than 30 days
> - The time between `startTime` and `endTime`can't be more than 30 days

## Response Example

```javascript
[
	{
	  	"symbol": "BTCUSDT",
	  	"type": 1,
		"deltaType": "USER_ADJUST",
		"amount": "23.36332311",
	  	"asset": "USDT",
	  	"time": 1578047897183,
	  	"positionSide": "BOTH"
	},
	{
		"symbol": "BTCUSDT",
	  	"type": 1,
		"deltaType": "USER_ADJUST",
		"amount": "100",
	  	"asset": "USDT",
	  	"time": 1578047900425,
	  	"positionSide": "LONG"
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Modify Isolated Position Margin(TRADE)

## API Description

Modify Isolated Position Margin

## HTTP Request

POST `/fapi/v1/positionMargin`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent with Hedge Mode. |
| amount | DECIMAL | YES |  |
| type | INT | YES | 1: Add position margin，2: Reduce position margin |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Only for isolated symbol

## Response Example

```javascript
{
	"amount": 100.0,
  	"code": 200,
  	"msg": "Successfully modify position margin.",
  	"type": 1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Modify Multiple Orders(TRADE)

## API Description

Modify Multiple Orders (TRADE)

## HTTP Request

PUT `/fapi/v1/batchOrders`

## Request Weight

5 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
5 on IP rate limit(x-mbx-used-weight-1m);

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| batchOrders | list<JSON> | YES | order list. Max 5 orders |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Where `batchOrders` is the list of order parameters in JSON**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| symbol | STRING | YES |  |
| side | ENUM | YES | `SELL`, `BUY` |
| quantity | DECIMAL | YES | Order quantity, cannot be sent with `closePosition=true` |
| price | DECIMAL | YES |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| stopPrice | DECIMAL | NO | stop price, only `STOP`, `STOP_MARKET`, `TAKE_PROFIT`, `TAKE_PROFIT_MARKET` need |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Parameter rules are same with `Modify Order`
> - Batch modify orders are processed concurrently, and the order of matching is not guaranteed.
> - The order of returned contents for batch modify orders is the same as the order of the order list.
> - One order can only be modfied for less than 10000 times

## Response Example

```javascript
[
	{
		"orderId": 20072994037,
		"symbol": "BTCUSDT",
		"pair": "BTCUSDT",
		"status": "NEW",
		"clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
		"price": "30005",
		"avgPrice": "0.0",
		"origQty": "1",
		"executedQty": "0",
		"cumQty": "0",
		"cumBase": "0",
		"timeInForce": "GTC",
		"type": "LIMIT",
		"reduceOnly": false,
		"closePosition": false,
		"side": "BUY",
		"positionSide": "LONG",
		"stopPrice": "0",
		"workingType": "CONTRACT_PRICE",
		"priceProtect": false,
		"origType": "LIMIT",
        "priceMatch": "NONE",              //price match mode
        "selfTradePreventionMode": "NONE", //self trading preventation mode
        "goodTillDate": 0,                 //order pre-set auot cancel time for TIF GTD order
		"updateTime": 1629182711600
	},
	{
		"code": -2022,
		"msg": "ReduceOnly Order is rejected."
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Modify Order (TRADE)

## API Description

Order modify function, currently only LIMIT order modification is supported, modified orders will be reordered in the match queue

## HTTP Request

PUT `/fapi/v1/order`

## Request Weight

1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
0 on IP rate limit(x-mbx-used-weight-1m)

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| symbol | STRING | YES |  |
| side | ENUM | YES | `SELL`, `BUY` |
| quantity | DECIMAL | YES | Order quantity, cannot be sent with `closePosition=true` |
| price | DECIMAL | YES |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.
> - Both `quantity` and `price` must be sent, which is different from dapi modify order endpoint.
> - When the new `quantity` or `price` doesn't satisfy PRICE\_FILTER / PERCENT\_FILTER / LOT\_SIZE, amendment will be rejected and the order will stay as it is.
> - However the order will be cancelled by the amendment in the following situations:
>   - when the order is in partially filled status and the new `quantity` <= `executedQty`
>   - When the order is `GTX` and the new price will cause it to be executed immediately
> - One order can only be modfied for less than 10000 times

## Response Example

```javascript
{
 	"orderId": 20072994037,
 	"symbol": "BTCUSDT",
 	"pair": "BTCUSDT",
 	"status": "NEW",
 	"clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
 	"price": "30005",
 	"avgPrice": "0.0",
 	"origQty": "1",
 	"executedQty": "0",
 	"cumQty": "0",
 	"cumBase": "0",
 	"timeInForce": "GTC",
 	"type": "LIMIT",
 	"reduceOnly": false,
 	"closePosition": false,
 	"side": "BUY",
 	"positionSide": "LONG",
 	"stopPrice": "0",
 	"workingType": "CONTRACT_PRICE",
 	"priceProtect": false,
 	"origType": "LIMIT",
    "priceMatch": "NONE",              //price match mode
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,                 //order pre-set auot cancel time for TIF GTD order
 	"updateTime": 1629182711600
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# New Algo Order(TRADE)

## API Description

Send in a new Algo order.

## HTTP Request

POST `/fapi/v1/algoOrder`

## Request Weight

0 on IP rate limit(x-mbx-used-weight-1m)

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoType | ENUM | YES | Only support `CONDITIONAL` |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES | For `CONDITIONAL` algoType, `STOP_MARKET`/`TAKE_PROFIT_MARKET`/`STOP`/`TAKE_PROFIT`/`TRAILING_STOP_MARKET` as order type |
| timeInForce | ENUM | NO | `IOC` or `GTC` or `FOK` or `GTX` , default `GTC` |
| quantity | DECIMAL | NO | Cannot be sent with `closePosition`=`true`(Close-All) |
| price | DECIMAL | NO |  |
| triggerPrice | DECIMAL | NO |  |
| workingType | ENUM | NO | triggerPrice triggered by: `MARK_PRICE`, `CONTRACT_PRICE`. Default `CONTRACT_PRICE` |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| closePosition | STRING | NO | true, false；Close-All，used with `STOP_MARKET` or `TAKE_PROFIT_MARKET`. |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP_MARKET` or `TAKE_PROFIT_MARKET` order. when price reaches the triggerPrice ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the Price Protection Threshold of the symbol. |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode; cannot be sent with `closePosition`=`true` |
| activatePrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the latest price(supporting different `workingType`) |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 10 where 1 for 1% |
| clientAlgoId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| selfTradePreventionMode | ENUM | NO | `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Algo order with type `STOP`, parameter `timeInForce` can be sent ( default `GTC`).
> - Algo order with type `TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).

> - Condition orders will be triggered when:
>   - If parameter`priceProtect`is sent as true:
>
>     - when price reaches the `triggerPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
>     - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
>   - `STOP`, `STOP_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `triggerPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `triggerPrice`
>   - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `triggerPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `triggerPrice`
>   - `TRAILING_STOP_MARKET`:
>
>     - BUY: the lowest price after order placed <= `activatePrice`, and the latest price >= the lowest price \* (1 + `callbackRate`)
>     - SELL: the highest price after order placed >= `activatePrice`, and the latest price <= the highest price \* (1 - `callbackRate`)
> - For `TRAILING_STOP_MARKET`, if you got such error code.
>
>   `{"code": -2021, "msg": "Order would immediately trigger."}`
>
>
>   means that the parameters you send do not meet the following requirements:
>   - BUY: `activatePrice` should be smaller than latest price.
>   - SELL: `activatePrice` should be larger than latest price.
> - `STOP_MARKET`, `TAKE_PROFIT_MARKET` with `closePosition`=`true`:
>   - Follow the same rules for condition orders.
>   - If triggered， **close all** current long position( if `SELL`) or current short position( if `BUY`).
>   - Cannot be used with `quantity` paremeter
>   - Cannot be used with `reduceOnly` parameter
>   - In Hedge Mode,cannot be used with `BUY` orders in `LONG` position side. and cannot be used with `SELL` orders in `SHORT` position side
> - `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.

## Response Example

```javascript
{
   "algoId": 2146760,
   "clientAlgoId": "6B2I9XVcJpCjqPAJ4YoFX7",
   "algoType": "CONDITIONAL",
   "orderType": "TAKE_PROFIT",
   "symbol": "BNBUSDT",
   "side": "SELL",
   "positionSide": "BOTH",
   "timeInForce": "GTC",
   "quantity": "0.01",
   "algoStatus": "NEW",
   "triggerPrice": "750.000",
   "price": "750.000",
   "icebergQuantity": null,
   "selfTradePreventionMode": "EXPIRE_MAKER",
   "workingType": "CONTRACT_PRICE",
   "priceMatch": "NONE",
   "closePosition": false,
   "priceProtect": false,
   "reduceOnly": false,
   "activatePrice": "", //TRAILING_STOP_MARKET order
   "callbackRate": "",  //TRAILING_STOP_MARKET order
   "createTime": 1750485492076,
   "updateTime": 1750485492076,
   "triggerTime": 0,
   "goodTillDate": 0
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New Order(TRADE)

## API Description

Send in a new order.

## HTTP Request

POST `/fapi/v1/order`

## Request Weight

1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
0 on IP rate limit(x-mbx-used-weight-1m)

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `EXPIRE_MAKER` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` |
| `MARKET` | `quantity` |

> - If `newOrderRespType` is sent as `RESULT` :
>
>   - `MARKET` order: the final FILLED result of the order will be return directly.
>   - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.
> - `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.
> - In extreme market conditions, timeInForce `GTD` order auto cancel time might be delayed comparing to `goodTillDate`

## Response Example

```javascript
{
 	"clientOrderId": "testOrder",
 	"cumQty": "0",
 	"cumQuote": "0",
 	"executedQty": "0",
 	"orderId": 22542179,
 	"avgPrice": "0.00000",
 	"origQty": "10",
 	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",		// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"timeInForce": "GTD",
  	"type": "TRAILING_STOP_MARKET",
  	"origType": "TRAILING_STOP_MARKET",
 	"updateTime": 1566818724722,
 	"workingType": "CONTRACT_PRICE",
 	"priceProtect": false,      // if conditional order trigger is protected
 	"priceMatch": "NONE",              //price match mode
 	"selfTradePreventionMode": "NONE", //self trading preventation mode
 	"goodTillDate": 1693207680000      //order pre-set auot cancel time for TIF GTD order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Test Order(TRADE)

## API Description

Testing order request, this order will not be submitted to matching engine

## HTTP Request

POST `/fapi/v1/order/test`

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO | Cannot be sent with `closePosition`=`true`(Close-All) |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode; cannot be sent with `closePosition`=`true` |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| stopPrice | DECIMAL | NO | Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| closePosition | STRING | NO | `true`, `false`；Close-All，used with `STOP_MARKET` or `TAKE_PROFIT_MARKET`. |
| activationPrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the latest price(supporting different `workingType`) |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 5 where 1 for 1% |
| workingType | ENUM | NO | stopPrice triggered by: "MARK\_PRICE", "CONTRACT\_PRICE". Default "CONTRACT\_PRICE" |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` |
| `MARKET` | `quantity` |
| `STOP/TAKE_PROFIT` | `quantity`, `price`, `stopPrice` |
| `STOP_MARKET/TAKE_PROFIT_MARKET` | `stopPrice` |
| `TRAILING_STOP_MARKET` | `callbackRate` |

> - Order with type `STOP`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Order with type `TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Condition orders will be triggered when:
>   - If parameter`priceProtect`is sent as true:
>
>     - when price reaches the `stopPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
>     - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
>   - `STOP`, `STOP_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>   - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>   - `TRAILING_STOP_MARKET`:
>
>     - BUY: the lowest price after order placed `<=`activationPrice`, and the latest price >`= the lowest price \* (1 + `callbackRate`)
>     - SELL: the highest price after order placed >= `activationPrice`, and the latest price <= the highest price \* (1 - `callbackRate`)
> - For `TRAILING_STOP_MARKET`, if you got such error code.
>
>   `{"code": -2021, "msg": "Order would immediately trigger."}`
>
>
>   means that the parameters you send do not meet the following requirements:
>   - BUY: `activationPrice` should be smaller than latest price.
>   - SELL: `activationPrice` should be larger than latest price.
> - If `newOrderRespType` is sent as `RESULT` :
>   - `MARKET` order: the final FILLED result of the order will be return directly.
>   - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.
> - `STOP_MARKET`, `TAKE_PROFIT_MARKET` with `closePosition`=`true`:
>   - Follow the same rules for condition orders.
>   - If triggered， **close all** current long position( if `SELL`) or current short position( if `BUY`).
>   - Cannot be used with `quantity` paremeter
>   - Cannot be used with `reduceOnly` parameter
>   - In Hedge Mode,cannot be used with `BUY` orders in `LONG` position side. and cannot be used with `SELL` orders in `SHORT` position side
> - `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.
>
> - In extreme market conditions, timeInForce `GTD` order auto cancel time might be delayed comparing to `goodTillDate`

## Response Example

```javascript
{
 	"clientOrderId": "testOrder",
 	"cumQty": "0",
 	"cumQuote": "0",
 	"executedQty": "0",
 	"orderId": 22542179,
 	"avgPrice": "0.00000",
 	"origQty": "10",
 	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",		// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   // if Close-All
  	"symbol": "BTCUSDT",
  	"timeInForce": "GTD",
  	"type": "TRAILING_STOP_MARKET",
  	"origType": "TRAILING_STOP_MARKET",
  	"activatePrice": "9020",	// activation price, only return with TRAILING_STOP_MARKET order
  	"priceRate": "0.3",			// callback rate, only return with TRAILING_STOP_MARKET order
 	"updateTime": 1566818724722,
 	"workingType": "CONTRACT_PRICE",
 	"priceProtect": false,      // if conditional order trigger is protected
 	"priceMatch": "NONE",              //price match mode
 	"selfTradePreventionMode": "NONE", //self trading preventation mode
 	"goodTillDate": 1693207680000      //order pre-set auot cancel time for TIF GTD order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Place Multiple Orders(TRADE)

## API Description

Place Multiple Orders

## HTTP Request

POST `/fapi/v1/batchOrders`

## Request Weight

5 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
5 on IP rate limit(x-mbx-used-weight-1m);

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| batchOrders | LIST<JSON> | YES | order list. Max 5 orders |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Where `batchOrders` is the list of order parameters in JSON**

- **Example:** /fapi/v1/batchOrders?batchOrders=\[{"type":"LIMIT","timeInForce":"GTC",

"symbol":"BTCUSDT","side":"BUY","price":"10001","quantity":"0.001"}\]

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent with Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | YES |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |

> - Paremeter rules are same with `New Order`
> - Batch orders are processed concurrently, and the order of matching is not guaranteed.
> - The order of returned contents for batch orders is the same as the order of the order list.

## Response Example

```javascript
[
	{
	 	"clientOrderId": "testOrder",
	 	"cumQty": "0",
	 	"cumQuote": "0",
	 	"executedQty": "0",
	 	"orderId": 22542179,
	 	"avgPrice": "0.00000",
	 	"origQty": "10",
	 	"price": "0",
	  	"reduceOnly": false,
	  	"side": "BUY",
	  	"positionSide": "SHORT",
	  	"status": "NEW",
	  	"stopPrice": "0",
	 	"closePosition": false,
	  	"symbol": "BTCUSDT",
	  	"timeInForce": "GTC",
	  	"type": "TRAILING_STOP_MARKET",
	  	"origType": "TRAILING_STOP_MARKET",
	  	"updateTime": 1566818724722,
	 	"workingType": "CONTRACT_PRICE",
	 	"priceProtect": false,      // if conditional order trigger is protected
		"priceMatch": "NONE",              //price match mode
		"selfTradePreventionMode": "NONE", //self trading preventation mode
		"goodTillDate": 1693207680000      //order pre-set auto cancel time for TIF GTD order
	},
	{
		"code": -2022,
		"msg": "ReduceOnly Order is rejected."
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Position ADL Quantile Estimation(USER\_DATA)

## API Description

Position ADL Quantile Estimation

> - Values update every 30s.
> - Values 0, 1, 2, 3, 4 shows the queue position and possibility of ADL from low to high.
> - For positions of the symbol are in One-way Mode or isolated margined in Hedge Mode, "LONG", "SHORT", and "BOTH" will be returned to show the positions' adl quantiles of different position sides.
> - If the positions of the symbol are crossed margined in Hedge Mode:
>   - "HEDGE" as a sign will be returned instead of "BOTH";
>   - A same value caculated on unrealized pnls on long and short sides' positions will be shown for "LONG" and "SHORT" when there are positions in both of long and short sides.

## HTTP Request

GET `/fapi/v1/adlQuantile`

## Request Weight

**5**

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
		"symbol": "ETHUSDT",
		"adlQuantile":
			{
				// if the positions of the symbol are crossed margined in Hedge Mode, "LONG" and "SHORT" will be returned a same quantile value, and "HEDGE" will be returned instead of "BOTH".
				"LONG": 3,
				"SHORT": 3,
				"HEDGE": 0   // only a sign, ignore the value
			}
		},
 	{
 		"symbol": "BTCUSDT",
 		"adlQuantile":
 			{
 				// for positions of the symbol are in One-way Mode or isolated margined in Hedge Mode
 				"LONG": 1, 	// adl quantile for "LONG" position in hedge mode
 				"SHORT": 2, 	// adl qauntile for "SHORT" position in hedge mode
 				"BOTH": 0		// adl qunatile for position in one-way mode
 			}
 	}
 ]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Position Information V2 (USER\_DATA)

## API Description

Get current position information.

## HTTP Request

GET `/fapi/v2/positionRisk`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Note**

> Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.

## Response Example

> For One-way position mode:

```javascript
[
  	{
  		"entryPrice": "0.00000",
        "breakEvenPrice": "0.0",
  		"marginType": "isolated",
  		"isAutoAddMargin": "false",
  		"isolatedMargin": "0.00000000",
  		"leverage": "10",
  		"liquidationPrice": "0",
  		"markPrice": "6679.50671178",
  		"maxNotionalValue": "20000000",
  		"positionAmt": "0.000",
  		"notional": "0",,
  		"isolatedWallet": "0",
  		"symbol": "BTCUSDT",
  		"unRealizedProfit": "0.00000000",
  		"positionSide": "BOTH",
  		"updateTime": 0
  	}
]
```

> For Hedge position mode:

```javascript
[
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.001",
        "entryPrice": "22185.2",
        "breakEvenPrice": "0.0",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "-1.06214947",
        "liquidationPrice": "19731.45529116",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "marginType": "cross",
        "isolatedMargin": "0.00000000",
        "isAutoAddMargin": "false",
        "positionSide": "LONG",
        "notional": "21.12305052",
        "isolatedWallet": "0",
        "updateTime": 1655217461579
    },
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.000",
        "entryPrice": "0.0",
        "breakEvenPrice": "0.0",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "0.00000000",
        "liquidationPrice": "0",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "marginType": "cross",
        "isolatedMargin": "0.00000000",
        "isAutoAddMargin": "false",
        "positionSide": "SHORT",
        "notional": "0",
        "isolatedWallet": "0",
        "updateTime": 0
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Position Information V3 (USER\_DATA)

## API Description

Get current position information(only symbol that has position or open orders will be returned).

## HTTP Request

GET `/fapi/v3/positionRisk`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Note**

> Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.

## Response Example

> For One-way position mode:

```javascript
[
  {
        "symbol": "ADAUSDT",
        "positionSide": "BOTH",               // position side
        "positionAmt": "30",
        "entryPrice": "0.385",
        "breakEvenPrice": "0.385077",
        "markPrice": "0.41047590",
        "unRealizedProfit": "0.76427700",     // unrealized profit
        "liquidationPrice": "0",
        "isolatedMargin": "0",
        "notional": "12.31427700",
        "marginAsset": "USDT",
        "isolatedWallet": "0",
        "initialMargin": "0.61571385",        // initial margin required with current mark price
        "maintMargin": "0.08004280",          // maintenance margin required
        "positionInitialMargin": "0.61571385",// initial margin required for positions with current mark price
        "openOrderInitialMargin": "0",        // initial margin required for open orders with current mark price
        "adl": 2,
        "bidNotional": "0",                   // bids notional, ignore
        "askNotional": "0",                   // ask notional, ignore
        "updateTime": 1720736417660
  }
]
```

> For Hedge position mode:

```javascript
[
  {
        "symbol": "ADAUSDT",
        "positionSide": "LONG",               // position side
        "positionAmt": "30",
        "entryPrice": "0.385",
        "breakEvenPrice": "0.385077",
        "markPrice": "0.41047590",
        "unRealizedProfit": "0.76427700",     // unrealized profit
        "liquidationPrice": "0",
        "isolatedMargin": "0",
        "notional": "12.31427700",
        "marginAsset": "USDT",
        "isolatedWallet": "0",
        "initialMargin": "0.61571385",        // initial margin required with current mark price
        "maintMargin": "0.08004280",          // maintenance margin required
        "positionInitialMargin": "0.61571385",// initial margin required for positions with current mark price
        "openOrderInitialMargin": "0",        // initial margin required for open orders with current mark price
        "adl": 2,
        "bidNotional": "0",                   // bids notional, ignore
        "askNotional": "0",                   // ask notional, ignore
        "updateTime": 1720736417660
  },
  {
        "symbol": "COMPUSDT",
        "positionSide": "SHORT",
        "positionAmt": "-1.000",
        "entryPrice": "70.92841",
        "breakEvenPrice": "70.900038636",
        "markPrice": "49.72023376",
        "unRealizedProfit": "21.20817624",
        "liquidationPrice": "2260.56757210",
        "isolatedMargin": "0",
        "notional": "-49.72023376",
        "marginAsset": "USDT",
        "isolatedWallet": "0",
        "initialMargin": "2.48601168",
        "maintMargin": "0.49720233",
        "positionInitialMargin": "2.48601168",
        "openOrderInitialMargin": "0",
        "adl": 2,
        "bidNotional": "0",
        "askNotional": "0",
        "updateTime": 1708943511656
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Algo Order (USER\_DATA)

## API Description

Check an algo order's status.

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time
  - order create time + 90 days < current time

## HTTP Request

GET `/fapi/v1/algoOrder`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoId | LONG | NO |  |
| clientAlgoId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Notes:

> - Either `algoId` or `clientAlgoId` must be sent.
> - `algoId` is self-increment for each specific `symbol`

## Response Example

```javascript
{
   "algoId": 2146760,
   "clientAlgoId": "6B2I9XVcJpCjqPAJ4YoFX7",
   "algoType": "CONDITIONAL",
   "orderType": "TAKE_PROFIT",
   "symbol": "BNBUSDT",
   "side": "SELL",
   "positionSide": "BOTH",
   "timeInForce": "GTC",
   "quantity": "0.01",
   "algoStatus": "CANCELED",
   "actualOrderId": "",
   "actualPrice": "0.00000",
   "triggerPrice": "750.000",
   "price": "750.000",
   "icebergQuantity": null,
   "tpTriggerPrice": "0.000",
   "tpPrice": "0.000",
   "slTriggerPrice": "0.000",
   "slPrice": "0.000",
   "tpOrderType": "",
   "selfTradePreventionMode": "EXPIRE_MAKER",
   "workingType": "CONTRACT_PRICE",
   "priceMatch": "NONE",
   "closePosition": false,
   "priceProtect": false,
   "reduceOnly": false,
   "createTime": 1750485492076,
   "updateTime": 1750514545091,
   "triggerTime": 0,
   "goodTillDate": 0
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query All Algo Orders (USER\_DATA)

## API Description

Get all algo orders; active, CANCELED, TRIGGERED or FINISHED .

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time
  - order create time + 90 days < current time

## HTTP Request

GET `/fapi/v1/allAlgoOrders`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| algoId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| page | INT | NO |  |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Notes:**

> - If `algoId` is set, it will get orders >= that `algoId`. Otherwise most recent orders are returned.
> - The query time period must be less then 7 days( default as the recent 7 days).

## Response Example

```javascript
[
   {
       "algoId": 2146760,
       "clientAlgoId": "6B2I9XVcJpCjqPAJ4YoFX7",
       "algoType": "CONDITIONAL",
       "orderType": "TAKE_PROFIT",
       "symbol": "BNBUSDT",
       "side": "SELL",
       "positionSide": "BOTH",
       "timeInForce": "GTC",
       "quantity": "0.01",
       "algoStatus": "CANCELED",
       "actualOrderId": "",
       "actualPrice": "0.00000",
       "triggerPrice": "750.000",
       "price": "750.000",
       "icebergQuantity": null,
       "tpTriggerPrice": "0.000",
       "tpPrice": "0.000",
       "slTriggerPrice": "0.000",
       "slPrice": "0.000",
       "tpOrderType": "",
       "selfTradePreventionMode": "EXPIRE_MAKER",
       "workingType": "CONTRACT_PRICE",
       "priceMatch": "NONE",
       "closePosition": false,
       "priceProtect": false,
       "reduceOnly": false,
       "createTime": 1750485492076,
       "updateTime": 1750514545091,
       "triggerTime": 0,
       "goodTillDate": 0
   }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Current Open Order (USER\_DATA)

## API Description

Query open order

## HTTP Request

GET `/fapi/v1/openOrder`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either`orderId` or `origClientOrderId` must be sent
> - If the queried order has been filled or cancelled, the error message "Order does not exist" will be returned.

## Response Example

```javascript
{
  	"avgPrice": "0.00000",
  	"clientOrderId": "abc",
  	"cumQuote": "0",
  	"executedQty": "0",
  	"orderId": 1917641,
  	"origQty": "0.40",
  	"origType": "TRAILING_STOP_MARKET",
  	"price": "0",
  	"reduceOnly": false,
  	"side": "BUY",
  	"positionSide": "SHORT",
  	"status": "NEW",
  	"stopPrice": "9300",				// please ignore when order type is TRAILING_STOP_MARKET
  	"closePosition": false,   			// if Close-All
  	"symbol": "BTCUSDT",
  	"time": 1579276756075,				// order time
  	"timeInForce": "GTC",
  	"type": "TRAILING_STOP_MARKET",
  	"activatePrice": "9020",			// activation price, only return with TRAILING_STOP_MARKET order
  	"priceRate": "0.3",					// callback rate, only return with TRAILING_STOP_MARKET order
  	"updateTime": 1579276756075,
  	"workingType": "CONTRACT_PRICE",
  	"priceProtect": false,            // if conditional order trigger is protected
	"priceMatch": "NONE",              //price match mode
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0      //order pre-set auot cancel time for TIF GTD order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Order (USER\_DATA)

## API Description

Check an order's status.

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time
  - order create time + 90 days < current time

## HTTP Request

GET `/fapi/v1/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Notes:

> - Either `orderId` or `origClientOrderId` must be sent.
> - `orderId` is self-increment for each specific `symbol`

## Response Example

```json
{
    "avgPrice": "0.00000",
    "clientOrderId": "abc",
    "cumQuote": "0",
    "executedQty": "0",
    "orderId": 1917641,
    "origQty": "0.40",
    "origType": "TRAILING_STOP_MARKET",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "stopPrice": "9300",    // please ignore when order type is TRAILING_STOP_MARKET
    "closePosition": false,   // if Close-All
    "symbol": "BTCUSDT",
    "time": 1579276756075,    // order time
    "timeInForce": "GTC",
    "type": "TRAILING_STOP_MARKET",
    "activatePrice": "9020",   // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",     // callback rate, only return with TRAILING_STOP_MARKET order
    "updateTime": 1579276756075,  // update time
    "workingType": "CONTRACT_PRICE",
    "priceProtect": false            // if conditional order trigger is protected
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Futures TradFi Perps Contract(USER\_DATA)

## API Description

Sign TradFi-Perps agreement contract

## HTTP Request

POST `/fapi/v1/stock/contract`

## Request Weigh

**50**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
   "code": 200,
	"msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# User's Force Orders (USER\_DATA)

## API Description

Query user's Force Orders

## HTTP Request

GET `/fapi/v1/forceOrders`

## Request Weight

**20** with symbol, **50** without symbol

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| autoCloseType | ENUM | NO | "LIQUIDATION" for liquidation orders, "ADL" for ADL orders. |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 50; max 100. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If "autoCloseType" is not sent, orders with both of the types will be returned
> - If "startTime" is not sent, data within 7 days before "endTime" can be queried

## Response Example

```javascript
[
  {
  	"orderId": 6071832819,
  	"symbol": "BTCUSDT",
  	"status": "FILLED",
  	"clientOrderId": "autoclose-1596107620040000020",
  	"price": "10871.09",
  	"avgPrice": "10913.21000",
  	"origQty": "0.001",
  	"executedQty": "0.001",
  	"cumQuote": "10.91321",
  	"timeInForce": "IOC",
  	"type": "LIMIT",
  	"reduceOnly": false,
  	"closePosition": false,
  	"side": "SELL",
  	"positionSide": "BOTH",
  	"stopPrice": "0",
  	"workingType": "CONTRACT_PRICE",
  	"origType": "LIMIT",
  	"time": 1596107620044,
  	"updateTime": 1596107620087
  }
  {
   	"orderId": 6072734303,
   	"symbol": "BTCUSDT",
   	"status": "FILLED",
   	"clientOrderId": "adl_autoclose",
   	"price": "11023.14",
   	"avgPrice": "10979.82000",
   	"origQty": "0.001",
   	"executedQty": "0.001",
   	"cumQuote": "10.97982",
   	"timeInForce": "GTC",
   	"type": "LIMIT",
   	"reduceOnly": false,
   	"closePosition": false,
   	"side": "BUY",
   	"positionSide": "SHORT",
   	"stopPrice": "0",
   	"workingType": "CONTRACT_PRICE",
   	"origType": "LIMIT",
   	"time": 1596110725059,
   	"updateTime": 1596110725071
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# New Order(TRADE)

## API Description

Send in a new order.

## Method

`order.place`

## Request

```javascript
{
    "id": "3f7df6e3-2df4-44b9-9919-d2f38f90a99a",
    "method": "order.place",
    "params": {
        "apiKey": "HMOchcfii9ZRZnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGPHQj1lGdTjR",
        "positionSide": "BOTH",
        "price": 43187.00,
        "quantity": 0.1,
        "side": "BUY",
        "symbol": "BTCUSDT",
        "timeInForce": "GTC",
        "timestamp": 1702555533821,
        "type": "LIMIT",
        "signature": "0f04368b2d22aafd0ggc8809ea34297eff602272917b5f01267db4efbc1c9422"
    }
}
```

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO | Cannot be sent with `closePosition`=`true`(Close-All) |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode; cannot be sent with `closePosition`=`true` |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| stopPrice | DECIMAL | NO | Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| closePosition | STRING | NO | `true`, `false`；Close-All，used with `STOP_MARKET` or `TAKE_PROFIT_MARKET`. |
| activationPrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the latest price(supporting different `workingType`) |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 10 where 1 for 1% |
| workingType | ENUM | NO | stopPrice triggered by: "MARK\_PRICE", "CONTRACT\_PRICE". Default "CONTRACT\_PRICE" |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` or `priceMatch` |
| `MARKET` | `quantity` |
| `STOP/TAKE_PROFIT` | `quantity`, `stopPrice`, `price` or `priceMatch` |
| `STOP_MARKET/TAKE_PROFIT_MARKET` | `stopPrice` |
| `TRAILING_STOP_MARKET` | `callbackRate` |

> - Order with type `STOP`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Order with type `TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Condition orders will be triggered when:
>   - If parameter`priceProtect`is sent as true:
>
>     - when price reaches the `stopPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
>     - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
>   - `STOP`, `STOP_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>   - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>   - `TRAILING_STOP_MARKET`:
>
>     - BUY: the lowest price after order placed `<=`activationPrice`, and the latest price >`= the lowest price \* (1 + `callbackRate`)
>     - SELL: the highest price after order placed >= `activationPrice`, and the latest price <= the highest price \* (1 - `callbackRate`)
> - For `TRAILING_STOP_MARKET`, if you got such error code.
>
>   `{"code": -2021, "msg": "Order would immediately trigger."}`
>
>
>   means that the parameters you send do not meet the following requirements:
>   - BUY: `activationPrice` should be smaller than latest price.
>   - SELL: `activationPrice` should be larger than latest price.
> - If `newOrderRespType` is sent as `RESULT` :
>   - `MARKET` order: the final FILLED result of the order will be return directly.
>   - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.
> - `STOP_MARKET`, `TAKE_PROFIT_MARKET` with `closePosition`=`true`:
>   - Follow the same rules for condition orders.
>   - If triggered， **close all** current long position( if `SELL`) or current short position( if `BUY`).
>   - Cannot be used with `quantity` paremeter
>   - Cannot be used with `reduceOnly` parameter
>   - In Hedge Mode,cannot be used with `BUY` orders in `LONG` position side. and cannot be used with `SELL` orders in `SHORT` position side

## Response Example

```javascript
{
    "id": "3f7df6e3-2df4-44b9-9919-d2f38f90a99a",
    "status": 200,
    "result": {
        "orderId": 325078477,
        "symbol": "BTCUSDT",
        "status": "NEW",
        "clientOrderId": "iCXL1BywlBaf2sesNUrVl3",
        "price": "43187.00",
        "avgPrice": "0.00",
        "origQty": "0.100",
        "executedQty": "0.000",
        "cumQty": "0.000",
        "cumQuote": "0.00000",
        "timeInForce": "GTC",
        "type": "LIMIT",
        "reduceOnly": false,
        "closePosition": false,
        "side": "BUY",
        "positionSide": "BOTH",
        "stopPrice": "0.00",
        "workingType": "CONTRACT_PRICE",
        "priceProtect": false,
        "origType": "LIMIT",
        "priceMatch": "NONE",
        "selfTradePreventionMode": "NONE",
        "goodTillDate": 0,
        "updateTime": 1702555534435
    },
    "rateLimits": [
        {
            "rateLimitType": "ORDERS",
            "interval": "SECOND",
            "intervalNum": 10,
            "limit": 300,
            "count": 1
        },
        {
            "rateLimitType": "ORDERS",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 1200,
            "count": 1
        },
        {
            "rateLimitType": "REQUEST_WEIGHT",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 2400,
            "count": 1
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel Algo Order (TRADE)

## API Description

Cancel an active algo order.

## Method

`algoOrder.cancel`

## Request

```javascript
{
   	"id": "5633b6a2-90a9-4192-83e7-925c90b6a2fd",
    "method": "algoOrder.cancel",
    "params": {
      "apiKey": "HsOehcfih8ZRxnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGOGQj1lGdTjR",
      "algoId": 283194212,
      "clientAlgoId": "DolwRKnQNjoc1E9Bbh03ER",
      "timestamp": 1703439070722,
      "signature": "b09c49815b4e3f1f6098cd9fbe26a933a9af79803deaaaae03c29f719c08a8a8"
    }
}
```

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoId | LONG | NO |  |
| clientAlgoId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `algoId` or `clientAlgoId` must be sent.

## Response Example

```javascript
{
  "id": "unique-cancel-request-id-5678",
  "status": 200,
  "result": {
    "algoId": 2000000002162519,
    "clientAlgoId": "rDMG8WSde6LkyMNtk6s825",
    "code": "200",
    "msg": "success"
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 6
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Cancel Order (TRADE)

## API Description

Cancel an active order.

## Method

`order.cancel`

## Request

```javascript
{
   	"id": "5633b6a2-90a9-4192-83e7-925c90b6a2fd",
    "method": "order.cancel",
    "params": {
      "apiKey": "HsOehcfih8ZRxnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGOGQj1lGdTjR",
      "orderId": 283194212,
      "symbol": "BTCUSDT",
      "timestamp": 1703439070722,
      "signature": "b09c49815b4e3f1f6098cd9fbe26a933a9af79803deaaaae03c29f719c08a8a8"
    }
}
```

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent.

## Response Example

```javascript
{
  "id": "5633b6a2-90a9-4192-83e7-925c90b6a2fd",
  "status": 200,
  "result": {
    "clientOrderId": "myOrder1",
    "cumQty": "0",
    "cumQuote": "0",
    "executedQty": "0",
    "orderId": 283194212,
    "origQty": "11",
    "origType": "TRAILING_STOP_MARKET",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "CANCELED",
    "stopPrice": "9300",
    "closePosition": false,
    "symbol": "BTCUSDT",
    "timeInForce": "GTC",
    "type": "TRAILING_STOP_MARKET",
    "activatePrice": "9020",
    "priceRate": "0.3",
    "updateTime": 1571110484038,
    "workingType": "CONTRACT_PRICE",
    "priceProtect": false,
    "priceMatch": "NONE",
    "selfTradePreventionMode": "NONE",
    "goodTillDate": 0
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 1
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Modify Order (TRADE)

## API Description

Order modify function, currently only LIMIT order modification is supported, modified orders will be reordered in the match queue

## Method

`order.modify`

## Request

```javascript
{
    "id": "c8c271ba-de70-479e-870c-e64951c753d9",
    "method": "order.modify",
    "params": {
        "apiKey": "HMOchcfiT9ZRZnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGPHQj1lGdTjR",
        "orderId": 328971409,
        "origType": "LIMIT",
        "positionSide": "SHORT",
        "price": "43769.1",
        "priceMatch": "NONE",
        "quantity": "0.11",
        "side": "SELL",
        "symbol": "BTCUSDT",
        "timestamp": 1703426755754,
        "signature": "d30c9f0736a307f5a9988d4a40b688662d18324b17367d51421da5484e835923"
    }
}
```

## Request Weight

1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S);
1 on 1min order rate limit(X-MBX-ORDER-COUNT-1M);
0 on IP rate limit(x-mbx-used-weight-1m)

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| symbol | STRING | YES |  |
| side | ENUM | YES | `SELL`, `BUY` |
| quantity | DECIMAL | YES | Order quantity, cannot be sent with `closePosition=true` |
| price | DECIMAL | YES |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.
> - Both `quantity` and `price` must be sent, which is different from dapi modify order endpoint.
> - When the new `quantity` or `price` doesn't satisfy PRICE\_FILTER / PERCENT\_FILTER / LOT\_SIZE, amendment will be rejected and the order will stay as it is.
> - However the order will be cancelled by the amendment in the following situations:
>   - when the order is in partially filled status and the new `quantity` <= `executedQty`
>   - When the order is `GTX` and the new price will cause it to be executed immediately
> - One order can only be modfied for less than 10000 times

## Response Example

```javascript
{
    "id": "c8c271ba-de70-479e-870c-e64951c753d9",
    "status": 200,
    "result": {
        "orderId": 328971409,
        "symbol": "BTCUSDT",
        "status": "NEW",
        "clientOrderId": "xGHfltUMExx0TbQstQQfRX",
        "price": "43769.10",
        "avgPrice": "0.00",
        "origQty": "0.110",
        "executedQty": "0.000",
        "cumQty": "0.000",
        "cumQuote": "0.00000",
        "timeInForce": "GTC",
        "type": "LIMIT",
        "reduceOnly": false,
        "closePosition": false,
        "side": "SELL",
        "positionSide": "SHORT",
        "stopPrice": "0.00",
        "workingType": "CONTRACT_PRICE",
        "priceProtect": false,
        "origType": "LIMIT",
        "priceMatch": "NONE",
        "selfTradePreventionMode": "NONE",
        "goodTillDate": 0,
        "updateTime": 1703426756190
    },
    "rateLimits": [
        {
            "rateLimitType": "ORDERS",
            "interval": "SECOND",
            "intervalNum": 10,
            "limit": 300,
            "count": 1
        },
        {
            "rateLimitType": "ORDERS",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 1200,
            "count": 1
        },
        {
            "rateLimitType": "REQUEST_WEIGHT",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 2400,
            "count": 1
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# New Algo Order(TRADE)

## API Description

Send in a new algo order.

## Method

`algoOrder.place`

## Request

```javascript
{
	"id": "7731f6b5-8d5e-419c-a424-016b0a5fe8d7",
	"method": "algoOrder.place",
	"params": {
		"algoType": "CONDITIONAL",
		"apiKey": "autoApiKey7mM4kPWaRuTUypdTEZKG8U8tDjO64xdBJBrmE1nXU2XSwdxGPyXcYx",
		"newOrderRespType": "RESULT",
		"positionSide": "SHORT",
		"price": "160000",
		"quantity": "1",
		"recvWindow": "99999999",
		"side": "SELL",
		"symbol": "BTCUSDT",
		"timeInForce": "GTC",
		"timestamp": 1762506268690,
		"triggerprice": 120000,
		"type": "TAKE_PROFIT",
		"signature": "ec6e529c69fd8193b19484907bc713114eae06259fcab9728dafd5910f9cac5a"
	}
}
```

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| algoType | ENUM | YES | Only support `CONDITIONAL` |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES | For `CONDITIONAL` algoType, `STOP_MARKET`/`TAKE_PROFIT_MARKET`/`STOP`/`TAKE_PROFIT`/`TRAILING_STOP_MARKET` as order type |
| timeInForce | ENUM | NO | `IOC` or `GTC` or `FOK`, default `GTC` |
| quantity | DECIMAL | NO | Cannot be sent with `closePosition`=`true`(Close-All) |
| price | DECIMAL | NO |  |
| triggerPrice | DECIMAL | NO |  |
| workingType | ENUM | NO | triggerPrice triggered by: `MARK_PRICE`, `CONTRACT_PRICE`. Default `CONTRACT_PRICE` |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| closePosition | STRING | NO | true, false；Close-All，used with `STOP_MARKET` or `TAKE_PROFIT_MARKET`. |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP_MARKET` or `TAKE_PROFIT_MARKET` order. when price reaches the triggerPrice ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the Price Protection Threshold of the symbol. |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode; cannot be sent with `closePosition`=`true` |
| activatePrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the latest price(supporting different `workingType`) |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 10 where 1 for 1% |
| clientAlgoId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| selfTradePreventionMode | ENUM | NO | `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Condition orders will be triggered when:
>   - If parameter`priceProtect`is sent as true:
>
>     - when price reaches the `triggerPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
>     - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
>   - `STOP`, `STOP_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `triggerPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `triggerPrice`
>   - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `triggerPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `triggerPrice`
>   - `TRAILING_STOP_MARKET`:
>
>     - BUY: the lowest price after order placed <= `activatePrice`, and the latest price >= the lowest price \* (1 + `callbackRate`)
>     - SELL: the highest price after order placed >= `activatePrice`, and the latest price <= the highest price \* (1 - `callbackRate`)
> - For `TRAILING_STOP_MARKET`, if you got such error code.
>
>   `{"code": -2021, "msg": "Order would immediately trigger."}`
>
>
>   means that the parameters you send do not meet the following requirements:
>   - BUY: `activatePrice` should be smaller than latest price.
>   - SELL: `activatePrice` should be larger than latest price.
> - `STOP_MARKET`, `TAKE_PROFIT_MARKET` with `closePosition`=`true`:
>   - Follow the same rules for condition orders.
>   - If triggered， **close all** current long position( if `SELL`) or current short position( if `BUY`).
>   - Cannot be used with `quantity` paremeter
>   - Cannot be used with `reduceOnly` parameter
>   - In Hedge Mode,cannot be used with `BUY` orders in `LONG` position side. and cannot be used with `SELL` orders in `SHORT` position side
> - `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.

## Response Example

```javascript
{
  "id": "06c9dbd8-ccbf-4ecf-a29c-fe31495ac73f",
  "status": 200,
  "result": {
    "algoId": 3000000000003505,
    "clientAlgoId": "0Xkl1p621E4EryvufmYre1",
    "algoType": "CONDITIONAL",
    "orderType": "TAKE_PROFIT",
    "symbol": "BTCUSDT",
    "side": "SELL",
    "positionSide": "SHORT",
    "timeInForce": "GTC",
    "quantity": "1.000",
    "algoStatus": "NEW",
    "triggerPrice": "120000.00",
    "price": "160000.00",
    "icebergQuantity": null,
    "selfTradePreventionMode": "EXPIRE_MAKER",
    "workingType": "CONTRACT_PRICE",
    "priceMatch": "NONE",
    "closePosition": false,
    "priceProtect": false,
    "reduceOnly": false,
    "createTime": 1762507264142,
    "updateTime": 1762507264143,
    "triggerTime": 0,
    "goodTillDate": 0
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 1
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New Order(TRADE)

## API Description

Send in a new order.

## Method

`order.place`

## Request

```javascript
{
    "id": "3f7df6e3-2df4-44b9-9919-d2f38f90a99a",
    "method": "order.place",
    "params": {
        "apiKey": "HMOchcfii9ZRZnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGPHQj1lGdTjR",
        "positionSide": "BOTH",
        "price": 43187.00,
        "quantity": 0.1,
        "side": "BUY",
        "symbol": "BTCUSDT",
        "timeInForce": "GTC",
        "timestamp": 1702555533821,
        "type": "LIMIT",
        "signature": "0f04368b2d22aafd0ggc8809ea34297eff602272917b5f01267db4efbc1c9422"
    }
}
```

## Request Weight

**0**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES |  |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO | Cannot be sent with `closePosition`=`true`(Close-All) |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode; cannot be sent with `closePosition`=`true` |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| stopPrice | DECIMAL | NO | Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| closePosition | STRING | NO | `true`, `false`；Close-All，used with `STOP_MARKET` or `TAKE_PROFIT_MARKET`. |
| activationPrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the latest price(supporting different `workingType`) |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 10 where 1 for 1% |
| workingType | ENUM | NO | stopPrice triggered by: "MARK\_PRICE", "CONTRACT\_PRICE". Default "CONTRACT\_PRICE" |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers; default `NONE` |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` or `priceMatch` |
| `MARKET` | `quantity` |
| `STOP/TAKE_PROFIT` | `quantity`, `stopPrice`, `price` or `priceMatch` |
| `STOP_MARKET/TAKE_PROFIT_MARKET` | `stopPrice` |
| `TRAILING_STOP_MARKET` | `callbackRate` |

> - Order with type `STOP`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Order with type `TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).
>
> - Condition orders will be triggered when:
>   - If parameter`priceProtect`is sent as true:
>
>     - when price reaches the `stopPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
>     - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
>   - `STOP`, `STOP_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>   - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:
>
>     - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
>     - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
>   - `TRAILING_STOP_MARKET`:
>
>     - BUY: the lowest price after order placed `<=`activationPrice`, and the latest price >`= the lowest price \* (1 + `callbackRate`)
>     - SELL: the highest price after order placed >= `activationPrice`, and the latest price <= the highest price \* (1 - `callbackRate`)
> - For `TRAILING_STOP_MARKET`, if you got such error code.
>
>   `{"code": -2021, "msg": "Order would immediately trigger."}`
>
>
>   means that the parameters you send do not meet the following requirements:
>   - BUY: `activationPrice` should be smaller than latest price.
>   - SELL: `activationPrice` should be larger than latest price.
> - If `newOrderRespType` is sent as `RESULT` :
>   - `MARKET` order: the final FILLED result of the order will be return directly.
>   - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.
> - `STOP_MARKET`, `TAKE_PROFIT_MARKET` with `closePosition`=`true`:
>   - Follow the same rules for condition orders.
>   - If triggered， **close all** current long position( if `SELL`) or current short position( if `BUY`).
>   - Cannot be used with `quantity` paremeter
>   - Cannot be used with `reduceOnly` parameter
>   - In Hedge Mode,cannot be used with `BUY` orders in `LONG` position side. and cannot be used with `SELL` orders in `SHORT` position side

## Response Example

```javascript
{
    "id": "3f7df6e3-2df4-44b9-9919-d2f38f90a99a",
    "status": 200,
    "result": {
        "orderId": 325078477,
        "symbol": "BTCUSDT",
        "status": "NEW",
        "clientOrderId": "iCXL1BywlBaf2sesNUrVl3",
        "price": "43187.00",
        "avgPrice": "0.00",
        "origQty": "0.100",
        "executedQty": "0.000",
        "cumQty": "0.000",
        "cumQuote": "0.00000",
        "timeInForce": "GTC",
        "type": "LIMIT",
        "reduceOnly": false,
        "closePosition": false,
        "side": "BUY",
        "positionSide": "BOTH",
        "stopPrice": "0.00",
        "workingType": "CONTRACT_PRICE",
        "priceProtect": false,
        "origType": "LIMIT",
        "priceMatch": "NONE",
        "selfTradePreventionMode": "NONE",
        "goodTillDate": 0,
        "updateTime": 1702555534435
    },
    "rateLimits": [
        {
            "rateLimitType": "ORDERS",
            "interval": "SECOND",
            "intervalNum": 10,
            "limit": 300,
            "count": 1
        },
        {
            "rateLimitType": "ORDERS",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 1200,
            "count": 1
        },
        {
            "rateLimitType": "REQUEST_WEIGHT",
            "interval": "MINUTE",
            "intervalNum": 1,
            "limit": 2400,
            "count": 1
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Position Information V2 (USER\_DATA)

## API Description

Get current position information(only symbol that has position or open orders will be returned).

## Method

`v2/account.position`

## Request

```javascript
{
   	"id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "v2/account.position",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "symbol": "BTCUSDT",
        "timestamp": 1702920680303,
        "signature": "31ab02a51a3989b66c29d40fcdf78216978a60afc6d8dc1c753ae49fa3164a2a"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Note**

> - Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.

## Response Example

> For One-way position mode:

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": [
    {
	    "symbol": "BTCUSDT",
	    "positionSide": "BOTH",            // Position Side
	    "positionAmt": "1.000",
	    "entryPrice": "0.00000",
	    "breakEvenPrice": "0.0",
	    "markPrice": "6679.50671178",
	    "unRealizedProfit": "0.00000000",  // Unrealized Profit
	    "liquidationPrice": "0",
	    "isolatedMargin": "0.00000000",
	    "notional": "0",
	    "marginAsset": "USDT",
	    "isolatedWallet": "0",
	    "initialMargin": "0",              // Initial Margin
	    "maintMargin": "0",                // Maintainance Margin
	    "positionInitialMargin": "0",      // Position Initial Margin
	    "openOrderInitialMargin": "0",     // Open Order Initial Margin
	    "adl": 0,
	    "bidNotional": "0",
	    "askNotional": "0",
	    "updateTime": 0                    // Update Time
    }
],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

> For Hedge position mode:

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": [
   {
	    "symbol": "BTCUSDT",
	    "positionSide": "LONG",
	    "positionAmt": "1.000",
	    "entryPrice": "0.00000",
	    "breakEvenPrice": "0.0",
	    "markPrice": "6679.50671178",
	    "unRealizedProfit": "0.00000000",
	    "liquidationPrice": "0",
	    "isolatedMargin": "0.00000000",
	    "notional": "0",
	    "marginAsset": "USDT",
	    "isolatedWallet": "0",
	    "initialMargin": "0",
	    "maintMargin": "0",
	    "positionInitialMargin": "0",
	    "openOrderInitialMargin": "0",
	    "adl": 0,
	    "bidNotional": "0",
	    "askNotional": "0",
	    "updateTime": 0
    },
    {
	    "symbol": "BTCUSDT",
	    "positionSide": "SHORT",
	    "positionAmt": "1.000",
	    "entryPrice": "0.00000",
	    "breakEvenPrice": "0.0",
	    "markPrice": "6679.50671178",
	    "unRealizedProfit": "0.00000000",
	    "liquidationPrice": "0",
	    "isolatedMargin": "0.00000000",
	    "notional": "0",
	    "marginAsset": "USDT",
	    "isolatedWallet": "0",
	    "initialMargin": "0",
	    "maintMargin": "0",
	    "positionInitialMargin": "0",
	    "openOrderInitialMargin": "0",
	    "adl": 0,
	    "bidNotional": "0",
	    "askNotional": "0",
	    "updateTime": 0
    }
  ],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Position Information (USER\_DATA)

## API Description

Get current position information.

## Method

`account.position`

## Request

```javascript
{
   	"id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "method": "account.position",
    "params": {
        "apiKey": "xTaDyrmvA9XT2oBHHjy39zyPzKCvMdtH3b9q4xadkAg2dNSJXQGCxzui26L823W2",
        "symbol": "BTCUSDT",
        "timestamp": 1702920680303,
        "signature": "31ab02a51a3989b66c29d40fcdf78216978a60afc6d8dc1c753ae49fa3164a2a"
    }
}
```

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Note**

> - Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.

## Response Example

> For One-way position mode:

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": [
    {
        "entryPrice": "0.00000",
        "breakEvenPrice": "0.0",
        "marginType": "isolated",
        "isAutoAddMargin": "false",
        "isolatedMargin": "0.00000000",
        "leverage": "10",
        "liquidationPrice": "0",
        "markPrice": "6679.50671178",
        "maxNotionalValue": "20000000",
        "positionAmt": "0.000",
        "notional": "0",
        "isolatedWallet": "0",
        "symbol": "BTCUSDT",
        "unRealizedProfit": "0.00000000",
        "positionSide": "BOTH",
        "updateTime": 0
    }
],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

> For Hedge position mode:

```javascript
{
  "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
  "status": 200,
  "result": [
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.001",
        "entryPrice": "22185.2",
        "breakEvenPrice": "0.0",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "-1.06214947",
        "liquidationPrice": "19731.45529116",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "marginType": "cross",
        "isolatedMargin": "0.00000000",
        "isAutoAddMargin": "false",
        "positionSide": "LONG",
        "notional": "21.12305052",
        "isolatedWallet": "0",
        "updateTime": 1655217461579
    },
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.000",
        "entryPrice": "0.0",
        "breakEvenPrice": "0.0",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "0.00000000",
        "liquidationPrice": "0",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "marginType": "cross",
        "isolatedMargin": "0.00000000",
        "isAutoAddMargin": "false",
        "positionSide": "SHORT",
        "notional": "0",
        "isolatedWallet": "0",
        "updateTime": 0
    }
],
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 20
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

- USDⓈ-M Futures
# Query Order (USER\_DATA)

## API Description

Check an order's status.

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time
  - order create time + 90 days < current time

## Method

`order.status`

## Request

```javascript
{
    "id": "0ce5d070-a5e5-4ff2-b57f-1556741a4204",
    "method": "order.status",
    "params": {
        "apiKey": "HMOchcfii9ZRZnhjp2XjGXhsOBd6msAhKz9joQaWwZ7arcJTlD2hGPHQj1lGdTjR",
        "orderId": 328999071,
        "symbol": "BTCUSDT",
        "timestamp": 1703441060152,
        "signature": "ba48184fc38a71d03d2b5435bd67c1206e3191e989fe99bda1bc643a880dfdbf"
    }
}
```

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Notes:

> - Either `orderId` or `origClientOrderId` must be sent.
> - `orderId` is self-increment for each specific `symbol`

## Response Example

```javascript
{
    "id": "605a6d20-6588-4cb9-afa0-b0ab087507ba",
    "status": 200,
    "result": {
        "avgPrice": "0.00000",
        "clientOrderId": "abc",
        "cumQuote": "0",
        "executedQty": "0",
        "orderId": 1917641,
        "origQty": "0.40",
        "origType": "TRAILING_STOP_MARKET",
        "price": "0",
        "reduceOnly": false,
        "side": "BUY",
        "positionSide": "SHORT",
        "status": "NEW",
        "stopPrice": "9300",    // please ignore when order type is TRAILING_STOP_MARKET
        "closePosition": false,   // if Close-All
        "symbol": "BTCUSDT",
        "time": 1579276756075,    // order time
        "timeInForce": "GTC",
        "type": "TRAILING_STOP_MARKET",
        "activatePrice": "9020",   // activation price, only return with TRAILING_STOP_MARKET order
        "priceRate": "0.3",     // callback rate, only return with TRAILING_STOP_MARKET order
        "updateTime": 1579276756075,  // update time
        "workingType": "CONTRACT_PRICE",
        "priceProtect": false            // if conditional order trigger is protected
    }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# User Data Streams Connect

- The base API endpoint is: **[https://fapi.binance.com](https://fapi.binance.com/)**

- A User Data Stream `listenKey` is valid for 60 minutes after creation.

- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes, if response `-1125` error "This listenKey does not exist." Please use `POST /fapi/v1/listenKey` to recreate `listenKey`.

- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.

- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.

- The connection method for Websocket：
  - Base Url: **wss://fstream.binance.com**
  - User Data Streams are accessed at **/ws/<listenKey>**
  - Example: `wss://fstream.binance.com/ws/XaEAKTsQSRLZAGH9tuIu37plSRsdjmlAVBoNYPUITlTAko1WI22PgmBMpI1rS8Yh`
- For one connection(one user data), the user data stream payloads can guaranteed to be in order during heavy periods; **Strongly recommend you order your updates using E**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Close User Data Stream (USER\_STREAM)

## API Description

Close out a user data stream.

## HTTP Request

DELETE `/fapi/v1/listenKey`

## Request Weight

1

## Request Parameters

None

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Close User Data Stream (USER\_STREAM)

## API Description

Close out a user data stream.

## Method

`userDataStream.stop`

## Request

```javascript
{
  "id": "819e1b1b-8c06-485b-a13e-131326c69599",
  "method": "userDataStream.stop",
  "params": {
    "apiKey": "vmPUZE6mv9SD5VNHk9HlWFsOr9aLE2zvsw0MuIgwCIPy8atIco14y7Ju91duEh8A"
  }
}
```

## Request Weight

**1**

## Request Parameters

None

## Response Example

```javascript
{
  "id": "819e1b1b-8c06-485b-a13e-131326c69599",
  "status": 200,
  "result": {},
   "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# User Data Streams Connect

- The base API endpoint is: **[https://fapi.binance.com](https://fapi.binance.com/)**

- A User Data Stream `listenKey` is valid for 60 minutes after creation.

- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes, if response `-1125` error "This listenKey does not exist." Please use `POST /fapi/v1/listenKey` to recreate `listenKey`.

- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.

- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.

- The connection method for Websocket：
  - Base Url: **wss://fstream.binance.com**
  - User Data Streams are accessed at **/ws/<listenKey>**
  - Example: `wss://fstream.binance.com/ws/XaEAKTsQSRLZAGH9tuIu37plSRsdjmlAVBoNYPUITlTAko1WI22PgmBMpI1rS8Yh`
- For one connection(one user data), the user data stream payloads can guaranteed to be in order during heavy periods; **Strongly recommend you order your updates using E**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Account Configuration Update previous Leverage Update

## Event Description

When the account configuration is changed, the event type will be pushed as `ACCOUNT_CONFIG_UPDATE`
When the leverage of a trade pair changes, the payload will contain the object `ac` to represent the account configuration of the trade pair, where `s` represents the specific trade pair and `l` represents the leverage
When the user Multi-Assets margin mode changes the payload will contain the object `ai` representing the user account configuration, where `j` represents the user Multi-Assets margin mode

## URL PATH

`/private`

## Event Name

`ACCOUNT_CONFIG_UPDATE`

## Response Example

> **Payload:**

```javascript
{
    "e":"ACCOUNT_CONFIG_UPDATE",       // Event Type
    "E":1611646737479,		           // Event Time
    "T":1611646737476,		           // Transaction Time
    "ac":{
    "s":"BTCUSDT",					   // symbol
    "l":25						       // leverage

    }
}

```

> **Or**

```javascript
{
    "e":"ACCOUNT_CONFIG_UPDATE",       // Event Type
    "E":1611646737479,		           // Event Time
    "T":1611646737476,		           // Transaction Time
    "ai":{							   // User's Account Configuration
    "j":true						   // Multi-Assets Mode
    }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Algo Order Update

## Event Description

When new algo order created, order status changed will push such event.
event type is `ALGO_UPDATE`.

**Algo Status**

- `NEW`: This status indicates that the conditional order was successfully placed into the Algo Service but has not yet been triggered.
- `CANCELED`: This status signifies that the conditional order has been canceled.
- `TRIGGERING`: This status suggests that the order has met the triggering condition and has been forwarded to the matching engine.
- `TRIGGERED`: This status means that the order has been successfully placed into the matching engine.
- `FINISHED`: This status shows that the triggered conditional order has been filled or canceled in the matching engine.
- `REJECTED`: This status signifies that the conditional order has been denied by the matching engine, such as in scenarios of margin check failures.
- `EXPIRED`: This status denotes that the conditional order has been canceled by the system. An example would be when a user places a GTE\_GTC Time-In-Force conditional order but then closes all positions on that symbol, resulting in system-led cancellation of the conditional order.

## URL PATH

`/private`

## Event Name

`ALGO_UPDATE`

## Response Example

```javascript
{
  "e":"ALGO_UPDATE",  // Event Type
  "T":1750515742297,  // Transaction Time
  "E":1750515742303,  // Event Time
  "o":{
    "caid":"Q5xaq5EGKgXXa0fD7fs0Ip",  // Client Algo Id
    "aid":2148719,  // Algo Id
    "at":"CONDITIONAL",  // Algo Type
    "o":"TAKE_PROFIT",  //Order Type
    "s":"BNBUSDT",  //Symbol
    "S":"SELL",  //Side
    "ps":"BOTH",  //Position Side
    "f":"GTC",  //Time in force
    "q":"0.01",  //quantity
    "X":"CANCELED",  //Algo status
    "ai":"",  // order id
    "ap": "0.00000", // avg fill price in matching engine, only display when order is triggered and placed in matching engine
    "aq": "0.00000", // execuated quantity in matching engine, only display when order is triggered and placed in matching engine
    "act": "0", // actual order type in matching engine, only display when order is triggered and placed in matching engine
    "tp":"750",  //Trigger price
    "p":"750", //Order Price
    "V":"EXPIRE_MAKER",  //STP mode
    "wt":"CONTRACT_PRICE", //Working type
    "pm":"NONE",  // Price match mode
    "cp":false,  //If Close-All
    "pP":false, //If price protection is turned on
    "R":false,  // Is this reduce only
    "tt":0,  //Trigger time
    "gtd":0,  // good till time for GTD time in force
    "rm": "Reduce Only reject"  // algo order failed reason
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Balance and Position Update

## Event Description

Event type is `ACCOUNT_UPDATE`.

- When balance or position get updated, this event will be pushed.
  - `ACCOUNT_UPDATE` will be pushed only when update happens on user's account, including changes on balances, positions, or margin type.
  - Unfilled orders or cancelled orders will not make the event `ACCOUNT_UPDATE` pushed, since there's no change on positions.
  - "position" in `ACCOUNT_UPDATE`: Only symbols of changed positions will be pushed.
- When "FUNDING FEE" changes to the user's balance, the event will be pushed with the brief message:
  - When "FUNDING FEE" occurs in a **crossed position**, `ACCOUNT_UPDATE` will be pushed with only the balance `B`(including the "FUNDING FEE" asset only), without any position `P` message.
  - When "FUNDING FEE" occurs in an **isolated position**, `ACCOUNT_UPDATE` will be pushed with only the balance `B`(including the "FUNDING FEE" asset only) and the relative position message `P`( including the isolated position on which the "FUNDING FEE" occurs only, without any other position message).
- The field "m" represents the reason type for the event and may shows the following possible types:
  - DEPOSIT
  - WITHDRAW
  - ORDER
  - FUNDING\_FEE
  - WITHDRAW\_REJECT
  - ADJUSTMENT
  - INSURANCE\_CLEAR
  - ADMIN\_DEPOSIT
  - ADMIN\_WITHDRAW
  - MARGIN\_TRANSFER
  - MARGIN\_TYPE\_CHANGE
  - ASSET\_TRANSFER
  - OPTIONS\_PREMIUM\_FEE
  - OPTIONS\_SETTLE\_PROFIT
  - AUTO\_EXCHANGE
  - COIN\_SWAP\_DEPOSIT
  - COIN\_SWAP\_WITHDRAW
- The field "bc" represents the balance change except for PnL and commission.

## URL PATH

`/private`

## Event Name

`ACCOUNT_UPDATE`

## Response Example

```javascript
{
  "e": "ACCOUNT_UPDATE",				// Event Type
  "E": 1564745798939,            		// Event Time
  "T": 1564745798938 ,           		// Transaction
  "a":                          		// Update Data
    {
      "m":"ORDER",						// Event reason type
      "B":[                     		// Balances
        {
          "a":"USDT",           		// Asset
          "wb":"122624.12345678",    	// Wallet Balance
          "cw":"100.12345678",			// Cross Wallet Balance
          "bc":"50.12345678"			// Balance Change except PnL and Commission
        },
        {
          "a":"BUSD",
          "wb":"1.00000000",
          "cw":"0.00000000",
          "bc":"-49.12345678"
        }
      ],
      "P":[
        {
          "s":"BTCUSDT",          	// Symbol
          "pa":"0",               	// Position Amount
          "ep":"0.00000",            // Entry Price
          "bep":"0",                // breakeven price
		  "cr":"200",             	// (Pre-fee) Accumulated Realized
          "up":"0",						// Unrealized PnL
          "mt":"isolated",				// Margin Type
          "iw":"0.00000000",			// Isolated Wallet (if isolated position)
          "ps":"BOTH"					// Position Side
        }，
        {
        	"s":"BTCUSDT",
        	"pa":"20",
        	"ep":"6563.66500",
        	"bep":"0",                // breakeven price
        	"cr":"0",
        	"up":"2850.21200",
        	"mt":"isolated",
        	"iw":"13200.70726908",
        	"ps":"LONG"
      	 },
        {
        	"s":"BTCUSDT",
        	"pa":"-10",
        	"ep":"6563.86000",
        	"bep":"6563.6",          // breakeven price
        	"cr":"-45.04000000",
        	"up":"-1423.15600",
        	"mt":"isolated",
        	"iw":"6570.42511771",
        	"ps":"SHORT"
        }
      ]
    }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Conditional\_Order\_Trigger\_Reject

## Event Description

`CONDITIONAL_ORDER_TRIGGER_REJECT` update when a triggered TP/SL order got rejected.

## URL PATH

`/private`

## Event Name

`CONDITIONAL_ORDER_TRIGGER_REJECT`

## Response Example

```javascript
{
    "e":"CONDITIONAL_ORDER_TRIGGER_REJECT",      // Event Type
    "E":1685517224945,      // Event Time
    "T":1685517224955,      // me message send Time
    "or":{
      "s":"ETHUSDT",      // Symbol
      "i":155618472834,      // orderId
      "r":"Due to the order could not be filled immediately, the FOK order has been rejected. The order will not be recorded in the order history",      // reject reason
     }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: GRID\_UPDATE

## Event Description

`GRID_UPDATE` update when a sub order of a grid is filled or partially filled.
**Strategy Status**

- NEW
- WORKING
- CANCELLED
- EXPIRED

## URL PATH

`/private`

## Event Name

`GRID_UPDATE`

## Response Example

```javascript
{
	"e": "GRID_UPDATE", // Event Type
	"T": 1669262908216, // Transaction Time
	"E": 1669262908218, // Event Time
	"gu": {
			"si": 176057039, // Strategy ID
			"st": "GRID", // Strategy Type
			"ss": "WORKING", // Strategy Status
			"s": "BTCUSDT", // Symbol
			"r": "-0.00300716", // Realized PNL
			"up": "16720", // Unmatched Average Price
			"uq": "-0.001", // Unmatched Qty
			"uf": "-0.00300716", // Unmatched Fee
			"mp": "0.0", // Matched PNL
			"ut": 1669262908197 // Update Time
		   }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Margin Call

## Event Description

- When the user's position risk ratio is too high, this stream will be pushed.
- This message is only used as risk guidance information and is not recommended for investment strategies.
- In the case of a highly volatile market, there may be the possibility that the user's position has been liquidated at the same time when this stream is pushed out.

## URL PATH

`/private`

## Event Name

`MARGIN_CALL`

## Response Example

```javascript
{
    "e":"MARGIN_CALL",    	// Event Type
    "E":1587727187525,		// Event Time
    "cw":"3.16812045",		// Cross Wallet Balance. Only pushed with crossed position margin call
    "p":[					// Position(s) of Margin Call
      {
        "s":"ETHUSDT",		// Symbol
        "ps":"LONG",		// Position Side
        "pa":"1.327",		// Position Amount
        "mt":"CROSSED",		// Margin Type
        "iw":"0",			// Isolated Wallet (if isolated position)
        "mp":"187.17127",	// Mark Price
        "up":"-1.166074",	// Unrealized PnL
        "mm":"1.614445"		// Maintenance Margin Required
      }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Order Update

## Event Description

When new order created, order status changed will push such event.
event type is `ORDER_TRADE_UPDATE`.

## URL PATH

`/private`

**Side**

- BUY
- SELL

**Order Type**

- LIMIT
- MARKET
- STOP
- STOP\_MARKET
- TAKE\_PROFIT
- TAKE\_PROFIT\_MARKET
- TRAILING\_STOP\_MARKET
- LIQUIDATION

**Execution Type**

- NEW
- CANCELED
- CALCULATED - Liquidation Execution
- EXPIRED
- TRADE
- AMENDMENT - Order Modified

**Order Status**

- NEW
- PARTIALLY\_FILLED
- FILLED
- CANCELED
- EXPIRED
- EXPIRED\_IN\_MATCH

**Time in force**

- GTC
- IOC
- FOK
- GTX

**Working Type**

- MARK\_PRICE
- CONTRACT\_PRICE

**Liquidation and ADL:**

- If user gets liquidated due to insufficient margin balance:
  - `c` shows as "autoclose-XXX"，`X` shows as "NEW"
- If user has enough margin balance but gets ADL:
  - `c` shows as “adl\_autoclose”，`X` shows as “NEW”

**Expiry Reason**

- `0`: None, the default value
- `1`: Order has expired to prevent users from inadvertently trading against themselves
- `2`: IOC order could not be filled completely, remaining quantity is canceled
- `3`: IOC order could not be filled completely to prevent users from inadvertently trading against themselves, remaining quantity is canceled
- `4`: Order has been canceled, as it's knocked out by another higher priority RO (market) order or reversed positions would be opened
- `5`: Order has expired when the account was liquidated
- `6`: Order has expired as GTE condition unsatisfied
- `7`: Order has been canceled as the symbol is delisted
- `8`: The initial order has expired after the stop order is triggered
- `9`: Market order could not be filled completely, remaining quantity is canceled

## Event Name

`ORDER_TRADE_UPDATE`

## Response Example

```javascript
{
  "e":"ORDER_TRADE_UPDATE",		   // Event Type
  "E":1568879465651,			       // Event Time
  "T":1568879465650,			       // Transaction Time
  "o":{
    "s":"BTCUSDT",			         // Symbol
    "c":"TEST",				           // Client Order Id
      // special client order id:
      // starts with "autoclose-": liquidation order
      // "adl_autoclose": ADL auto close order
      // "settlement_autoclose-": settlement order for delisting or delivery
    "S":"SELL",					         // Side
    "o":"TRAILING_STOP_MARKET",	 // Order Type
    "f":"GTC",					         // Time in Force
    "q":"0.001",				         // Original Quantity
    "p":"0",					           // Original Price
    "ap":"0",					           // Average Price
    "sp":"7103.04",				       // Stop Price. Please ignore with TRAILING_STOP_MARKET order
    "x":"NEW",					         // Execution Type
    "X":"NEW",					         // Order Status
    "i":8886774,				         // Order Id
    "l":"0",					           // Order Last Filled Quantity
    "z":"0",					           // Order Filled Accumulated Quantity
    "L":"0",					           // Last Filled Price
    "N":"USDT",            	     // Commission Asset
    "n":"0",               	     // Commission
    "T":1568879465650,			     // Order Trade Time
    "t":0,			        	       // Trade Id
    "b":"0",			    	         // Bids Notional
    "a":"9.91",					         // Ask Notional
    "m":false,					         // Is this trade the maker side?
    "R":false,					         // Is this reduce only
    "wt":"CONTRACT_PRICE", 		   // Stop Price Working Type
    "ot":"TRAILING_STOP_MARKET", // Original Order Type
    "ps":"LONG",					       // Position Side
    "cp":false,						       // If Close-All, pushed with conditional order
    "AP":"7476.89",				       // Activation Price, only puhed with TRAILING_STOP_MARKET order
    "cr":"5.0",					         // Callback Rate, only puhed with TRAILING_STOP_MARKET order
    "pP": false,                 // If price protection is turned on
    "si": 0,                     // ignore
    "ss": 0,                     // ignore
    "rp":"0",	   					       // Realized Profit of the trade
    "V":"EXPIRE_TAKER",          // STP mode
    "pm":"OPPONENT",             // Price match mode
    "gtd":0,                     // TIF GTD order auto cancel time
    "er":"0"                     // Expiry Reason
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: STRATEGY\_UPDATE

## Event Description

`STRATEGY_UPDATE` update when a strategy is created/cancelled/expired, ...etc.

**Strategy Status**

- NEW
- WORKING
- CANCELLED
- EXPIRED

**opCode**

- 8001: The strategy params have been updated
- 8002: User cancelled the strategy
- 8003: User manually placed or cancelled an order
- 8004: The stop limit of this order reached
- 8005: User position liquidated
- 8006: Max open order limit reached
- 8007: New grid order
- 8008: Margin not enough
- 8009: Price out of bounds
- 8010: Market is closed or paused
- 8011: Close position failed, unable to fill
- 8012: Exceeded the maximum allowable notional value at current leverage
- 8013: Grid expired due to incomplete KYC verification or access from a restricted jurisdiction
- 8014: Violated Futures Trading Quantitative Rules. Strategy stopped
- 8015: User position empty or liquidated

## URL PATH

`/private`

## Event Name

`STRATEGY_UPDATE`

## Response Example

```javascript
{
	"e": "STRATEGY_UPDATE", // Event Type
	"T": 1669261797627, // Transaction Time
	"E": 1669261797628, // Event Time
	"su": {
			"si": 176054594, // Strategy ID
			"st": "GRID", // Strategy Type
			"ss": "NEW", // Strategy Status
			"s": "BTCUSDT", // Symbol
			"ut": 1669261797627, // Update Time
			"c": 8007 // opCode
		}
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Trade Lite Update

## Event Description

Fast trade stream reduces data latency compared original `ORDER_TRADE_UPDATE` stream. However, it only pushes TRADE Execution Type, and fewer data fields.

## URL PATH

`/private`

## Event Name

`TRADE_LITE`

## Response Example

```javascript
{
  "e":"TRADE_LITE",             // Event Type
  "E":1721895408092,            // Event Time
  "T":1721895408214,            // Transaction Time
  "s":"BTCUSDT",                // Symbol
  "q":"0.001",                  // Original Quantity
  "p":"0",                      // Original Price
  "m":false,                    // Is this trade the maker side?
  "c":"z8hcUoOsqEdKMeKPSABslD", // Client Order Id
      // special client order id:
      // starts with "autoclose-": liquidation order
      // "adl_autoclose": ADL auto close order
      // "settlement_autoclose-": settlement order for delisting or delivery
  "S":"BUY",                   // Side
  "L":"64089.20",              // Last Filled Price
  "l":"0.040",                 // Order Last Filled Quantity
  "t":109100866,               // Trade Id
  "i":8886774,                // Order Id
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: User Data Stream Expired

## Event Description

When the `listenKey` used for the user data stream turns expired, this event will be pushed.

**Notice:**

> - This event is not related to the websocket disconnection.
> - This event will be received only when a valid `listenKey` in connection got expired.
> - No more user data event will be updated after this event received until a new valid `listenKey` used.

## URL PATH

`/private`

## Event Name

`listenKeyExpired`

## Response Example

```javascript
{
    "e": "listenKeyExpired",    // event type
    "E": "1736996475556",       // event time
    "listenKey":"WsCMN0a4KHUPTQuX6IUnqEZfB1inxmv1qR4kbf1LuEjur5VdbzqvyxqG9TSjVVxv"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Keepalive User Data Stream (USER\_STREAM)

## API Description

Keepalive a user data stream to prevent a time out. User data streams will close after 60 minutes. It's recommended to send a ping about every 60 minutes.

## HTTP Request

PUT `/fapi/v1/listenKey`

## Request Weight

**1**

## Request Parameters

None

## Response Example

```javascript
{
    "listenKey": "3HBntNTepshgEdjIwSUIBgB9keLyOCg5qv3n6bYAtktG8ejcaW5HXz9Vx1JgIieg" //the listenkey which got extended
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Keepalive User Data Stream (USER\_STREAM)

## API Description

Keepalive a user data stream to prevent a time out. User data streams will close after 60 minutes. It's recommended to send a ping about every 60 minutes.

## Method

`userDataStream.ping`

## Request

```javascript
{
  "id": "815d5fce-0880-4287-a567-80badf004c74",
  "method": "userDataStream.ping",
  "params": {
    "apiKey": "vmPUZE6mv9SD5VNHk9HlWFsOr9aLE2zvsw0MuIgwCIPy8atIco14y7Ju91duEh8A"
   }
}
```

## Request Weight

**1**

## Request Parameters

None

## Response Example

```javascript
{
  "id": "815d5fce-0880-4287-a567-80badf004c74",
  "status": 200,
  "result": {
    "listenKey": "3HBntNTepshgEdjIwSUIBgB9keLyOCg5qv3n6bYAtktG8ejcaW5HXz9Vx1JgIieg"
  },
  "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Start User Data Stream (USER\_STREAM)

## API Description

Start a new user data stream. The stream will close after 60 minutes unless a keepalive is sent. If the account has an active `listenKey`, that `listenKey` will be returned and its validity will be extended for 60 minutes.

## HTTP Request

POST `/fapi/v1/listenKey`

## Request Weight

**1**

## Request Parameters

None

## Response Example

```javascript
{
  "listenKey": "pqia91ma19a5s61cv6a81va65sdf19v8a65a1a5s61cv6a81va65sdf19v8a65a1"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Start User Data Stream (USER\_STREAM)

## API Description

Start a new user data stream. The stream will close after 60 minutes unless a keepalive is sent. If the account has an active `listenKey`, that `listenKey` will be returned and its validity will be extended for 60 minutes.

## Method

`userDataStream.start`

## Request

```javascript
{
  "id": "d3df8a61-98ea-4fe0-8f4e-0fcea5d418b0",
  "method": "userDataStream.start",
  "params": {
    "apiKey": "vmPUZE6mv9SD5VNHk4HlWFsOr6aKE2zvsw0MuIgwCIPy6utIco14y7Ju91duEh8A"
  }
}
```

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| apiKey | STRING | YES |  |

## Response Example

```javascript
{
  "id": "d3df8a61-98ea-4fe0-8f4e-0fcea5d418b0",
  "status": 200,
  "result": {
    "listenKey": "xs0mRXdAKlIPDRFrlPcw0qI41Eh3ixNntmymGyhrhgqo7L6FuLaWArTD7RLP"
  },
   "rateLimits": [
    {
      "rateLimitType": "REQUEST_WEIGHT",
      "interval": "MINUTE",
      "intervalNum": 1,
      "limit": 2400,
      "count": 2
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Websocket Market Streams

- The connection method for Websocket is：
  - Base Url: **wss://fstream.binance.com**
  - Streams can be access either in a single raw stream or a combined stream
  - Raw streams are accessed at **/ws/<streamName>**
  - Combined streams are accessed at **/stream?streams=<streamName1>/<streamName2>/<streamName3>**
  - Example:
  - `wss://fstream.binance.com/ws/bnbusdt@aggTrade`
  - `wss://fstream.binance.com/stream?streams=bnbusdt@aggTrade/btcusdt@markPrice`
- Combined stream events are wrapped as follows: **{"stream":"<streamName>","data":<rawPayload>}**

- All symbols for streams are **lowercase**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

- The websocket server will send a `ping frame` every 3 minutes. If the websocket server does not receive a `pong frame` back from the connection within a 10 minute period, the connection will be disconnected. Unsolicited `pong frames` are allowed(the client can send pong frames at a frequency higher than every 15 minutes to maintain the connection).

- WebSocket connections have a limit of 10 incoming messages per second.

- A connection that goes beyond the limit will be disconnected; IPs that are repeatedly disconnected may be banned.

- A single connection can listen to a maximum of **1024** streams.

- Considering the possible data latency from RESTful endpoints during an extremely volatile market, it is highly recommended to get the order status, position, etc from the Websocket user data stream.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Aggregate Trade Streams

## Stream Description

The Aggregate Trade Streams push market trade information that is aggregated for fills with same price and taking side every 100 milliseconds. Only market trades will be aggregated, which means the insurance fund trades and ADL trades won't be aggregated.

## URL PATH

`/market`

## Stream Name

`<symbol>@aggTrade`

**Note**:

> Retail Price Improvement(RPI) orders are aggregated into field `q` and without special tags to be distinguished.

## Update Speed

**100ms**

## Response Example

```javascript
{
  "e": "aggTrade",  // Event type
  "E": 123456789,   // Event time
  "s": "BTCUSDT",   // Symbol
  "a": 5933014,		  // Aggregate trade ID
  "p": "0.001",     // Price
  "q": "100",       // Quantity with all the market trades
  "nq": "100",      // Normal quantity without the trades involving RPI orders
  "f": 100,         // First trade ID
  "l": 105,         // Last trade ID
  "T": 123456785,   // Trade time
  "m": true,        // Is the buyer the market maker?
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# All Book Tickers Stream

## Stream Description

Pushes any update to the best bid or ask's price or quantity in real-time for all symbols.

## URL PATH

`/public`

## Stream Name

`!bookTicker`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Update Speed

**5s**

## Response Example

```javascript
{
  "e":"bookTicker",			// event type
  "u":400900217,     		// order book updateId
  "E": 1568014460893,  	// event time
  "T": 1568014460891,  	// transaction time
  "s":"BNBUSDT",     		// symbol
  "b":"25.35190000", 		// best bid price
  "B":"31.21000000", 		// best bid qty
  "a":"25.36520000", 		// best ask price
  "A":"40.66000000"  		// best ask qty
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# All Market Liquidation Order Streams

## Stream Description

The All Liquidation Order Snapshot Streams push force liquidation order information for all symbols in the market.
For each symbol，only the latest one liquidation order within 1000ms will be pushed as the snapshot. If no liquidation happens in the interval of 1000ms, no stream will be pushed.

## URL PATH

`/market`

## Stream Name

`!forceOrder@arr`

## Update Speed

**1000ms**

## Response Example

```javascript
{

	"e":"forceOrder",                   // Event Type
	"E":1568014460893,                  // Event Time
	"o":{

		"s":"BTCUSDT",                   // Symbol
		"S":"SELL",                      // Side
		"o":"LIMIT",                     // Order Type
		"f":"IOC",                       // Time in Force
		"q":"0.014",                     // Original Quantity
		"p":"9910",                      // Price
		"ap":"9910",                     // Average Price
		"X":"FILLED",                    // Order Status
		"l":"0.014",                     // Order Last Filled Quantity
		"z":"0.014",                     // Order Filled Accumulated Quantity
		"T":1568014460893,          	 // Order Trade Time
	}
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# All Market Mini Tickers Stream

## Stream Description

24hr rolling window mini-ticker statistics for all symbols. These are NOT the statistics of the UTC day, but a 24hr rolling window from requestTime to 24hrs before. Note that only tickers that have changed will be present in the array.

## URL PATH

`/market`

## Stream Name

`!miniTicker@arr`

## Update Speed

**1000ms**

## Response Example

```javascript
[
  {
    "e": "24hrMiniTicker",  // Event type
    "E": 123456789,         // Event time
    "s": "BTCUSDT",         // Symbol
    "c": "0.0025",          // Close price
    "o": "0.0010",          // Open price
    "h": "0.0025",          // High price
    "l": "0.0010",          // Low price
    "v": "10000",           // Total traded base asset volume
    "q": "18"               // Total traded quote asset volume
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# All Market Tickers Streams

## Stream Description

24hr rolling window ticker statistics for all symbols. These are NOT the statistics of the UTC day, but a 24hr rolling window from requestTime to 24hrs before. Note that only tickers that have changed will be present in the array.

## URL PATH

`/market`

## Stream Name

`!ticker@arr`

## Update Speed

**1000ms**

## Response Example

```javascript
[
	{
	  "e": "24hrTicker",  // Event type
	  "E": 123456789,     // Event time
	  "s": "BTCUSDT",     // Symbol
	  "p": "0.0015",      // Price change
	  "P": "250.00",      // Price change percent
	  "w": "0.0018",      // Weighted average price
	  "c": "0.0025",      // Last price
	  "Q": "10",          // Last quantity
	  "o": "0.0010",      // Open price
	  "h": "0.0025",      // High price
	  "l": "0.0010",      // Low price
	  "v": "10000",       // Total traded base asset volume
	  "q": "18",          // Total traded quote asset volume
	  "O": 0,             // Statistics open time
	  "C": 86400000,      // Statistics close time
	  "F": 0,             // First trade ID
	  "L": 18150,         // Last trade Id
	  "n": 18151          // Total number of trades
	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Composite Index Symbol Information Streams

## Stream Description

Composite index information for index symbols pushed every second.

## URL PATH

`/market`

## Stream Name

`<symbol>@compositeIndex`

## Update Speed

**1000ms**

## Response Example

```javascript
{
  "e":"compositeIndex",		// Event type
  "E":1602310596000,		// Event time
  "s":"DEFIUSDT",			// Symbol
  "p":"554.41604065",		// Price
  "C":"baseAsset",
  "c":[      				// Composition
  	{
  		"b":"BAL",			// Base asset
  		"q":"USDT",         // Quote asset
  		"w":"1.04884844",	// Weight in quantity
  		"W":"0.01457800",   // Weight in percentage
  		"i":"24.33521021"   // Index price
  	},
  	{
  		"b":"BAND",
  		"q":"USDT" ,
  		"w":"3.53782729",
  		"W":"0.03935200",
  		"i":"7.26420084"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Websocket Market Streams

- The connection method for Websocket is：
  - Base Url: **wss://fstream.binance.com**
  - Streams can be access either in a single raw stream or a combined stream
  - Raw streams are accessed at **/ws/<streamName>**
  - Combined streams are accessed at **/stream?streams=<streamName1>/<streamName2>/<streamName3>**
  - Example:
  - `wss://fstream.binance.com/ws/bnbusdt@aggTrade`
  - `wss://fstream.binance.com/stream?streams=bnbusdt@aggTrade/btcusdt@markPrice`
- Combined stream events are wrapped as follows: **{"stream":"<streamName>","data":<rawPayload>}**

- All symbols for streams are **lowercase**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

- The websocket server will send a `ping frame` every 3 minutes. If the websocket server does not receive a `pong frame` back from the connection within a 10 minute period, the connection will be disconnected. Unsolicited `pong frames` are allowed(the client can send pong frames at a frequency higher than every 15 minutes to maintain the connection).

- WebSocket connections have a limit of 10 incoming messages per second.

- A connection that goes beyond the limit will be disconnected; IPs that are repeatedly disconnected may be banned.

- A single connection can listen to a maximum of **1024** streams.

- Considering the possible data latency from RESTful endpoints during an extremely volatile market, it is highly recommended to get the order status, position, etc from the Websocket user data stream.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Continuous Contract Kline/Candlestick Streams

## Stream Description

**Contract type:**

- perpetual
- current\_quarter
- next\_quarter
- tradifi\_perpetual

**Kline/Candlestick chart intervals:**

s -> seconds; m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

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

## URL PATH

`/market`

## Stream Name

`<pair>_<contractType>@continuousKline_<interval>`

## Update Speed

**250ms**

## Response Example

```javascript
{
  "e":"continuous_kline",	// Event type
  "E":1607443058651,		// Event time
  "ps":"BTCUSDT",			// Pair
  "ct":"PERPETUAL"			// Contract type
  "k":{
    "t":1607443020000,		// Kline start time
    "T":1607443079999,		// Kline close time
    "i":"1m",				// Interval
    "f":116467658886,		// First updateId
    "L":116468012423,		// Last updateId
    "o":"18787.00",			// Open price
    "c":"18804.04",			// Close price
    "h":"18804.04",			// High price
    "l":"18786.54",			// Low price
    "v":"197.664",			// volume
    "n": 543,				// Number of trades
    "x":false,				// Is this kline closed?
    "q":"3715253.19494",	// Quote asset volume
    "V":"184.769",			// Taker buy volume
    "Q":"3472925.84746",	//Taker buy quote asset volume
    "B":"0"					// Ignore
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Contract Info Stream

## Stream Description

ContractInfo stream pushes when contract info updates(listing/settlement/contract bracket update). `bks` field only shows up when bracket gets updated.

## URL PATH

`/market`

## Stream Name

`!contractInfo`

## Update Speed

**Real-time**

## Response Example

```javascript
{
    "e":"contractInfo",          // Event Type
    "E":1669356423908,           // Event Time
    "s":"IOTAUSDT",              // Symbol
    "ps":"IOTAUSDT",             // Pair
    "ct":"PERPETUAL",            // Contract type
    "dt":4133404800000,          // Delivery date time
    "ot":1569398400000,          // onboard date time
    "cs":"TRADING",              // Contract status
    "bks":[
        {
            "bs":1,              // Notional bracket
            "bnf":0,             // Floor notional of this bracket
            "bnc":5000,          // Cap notional of this bracket
            "mmr":0.01,          // Maintenance ratio for this bracket
            "cf":0,              // Auxiliary number for quick calculation
            "mi":21,             // Min leverage for this bracket
            "ma":50              // Max leverage for this bracket
        },
        {
            "bs":2,
            "bnf":5000,
            "bnc":25000,
            "mmr":0.025,
            "cf":75,
            "mi":11,
            "ma":20
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Diff. Book Depth Streams

## Stream Description

Bids and asks, pushed every 250 milliseconds, 500 milliseconds, 100 milliseconds (if existing)

## URL PATH

`/public`

## Stream Name

`<symbol>@depth` OR `<symbol>@depth@500ms` OR `<symbol>@depth@100ms`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Update Speed

**250ms**, **500ms**, **100ms**

## Response Example

```javascript
{
  "e": "depthUpdate", // Event type
  "E": 123456789,     // Event time
  "T": 123456788,     // Transaction time
  "s": "BTCUSDT",     // Symbol
  "U": 157,           // First update ID in event
  "u": 160,           // Final update ID in event
  "pu": 149,          // Final update Id in last stream(ie `u` in last stream)
  "b": [              // Bids to be updated
    [
      "0.0024",       // Price level to be updated
      "10"            // Quantity
    ]
  ],
  "a": [              // Asks to be updated
    [
      "0.0026",       // Price level to be updated
      "100"          // Quantity
    ]
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# RPI Diff. Book Depth Streams

## Stream Description

Bids and asks including RPI orders, pushed every 500 milliseconds

## URL PATH

`/public`

## Stream Name

`<symbol>@rpiDepth@500ms`

**Note**:

> RPI(Retail Price Improvement) orders are included and aggreated in the response message. When the quantity of a price level to be updated is equal to 0, it means either all quotations for this price have been filled/canceled, or the quantity of crossed RPI orders for this price are hidden

## Update Speed

**500ms**

## Response Example

```javascript
{
  "e": "depthUpdate", // Event type
  "E": 123456789,     // Event time
  "T": 123456788,     // Transaction time
  "s": "BTCUSDT",     // Symbol
  "U": 157,           // First update ID in event
  "u": 160,           // Final update ID in event
  "pu": 149,          // Final update Id in last stream(ie `u` in last stream)
  "b": [              // Bids to be updated
    [
      "0.0024",       // Price level to be updated
      "10"            // Quantity
    ]
  ],
  "a": [              // Asks to be updated
    [
      "0.0026",       // Price level to be updated
      "100"          // Quantity
    ]
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# How to manage a local order book correctly

1. Open a stream to **wss://fstream.binance.com/stream?streams=btcusdt@depth**.
2. Buffer the events you receive from the stream. For same price, latest received update covers the previous one.
3. Get a depth snapshot from **[https://fapi.binance.com/fapi/v1/depth?symbol=BTCUSDT&limit=1000](https://fapi.binance.com/fapi/v1/depth?symbol=BTCUSDT&limit=1000)** .
4. Drop any event where `u` is < `lastUpdateId` in the snapshot.
5. The first processed event should have `U````<= ``lastUpdateId``` **AND**`u` >```= ``lastUpdateId```

- U = firstUpdateId (the first update ID) from the WebSocket stream.
- u = finalUpdateId (the last update ID) from the WebSocket stream.
- lastUpdateId = the update ID you got from the REST depth snapshot.

6. While listening to the stream, each new event's `pu` should be equal to the previous event's `u`, otherwise initialize the process from step 3.ß
7. The data in each event is the **absolute** quantity for a price level.
8. If the quantity is 0, **remove** the price level.
9. Receiving an event that removes a price level that is not in your local order book can happen and is normal.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Important WebSocket Change Notice — Base URL Split & Migration

## Background

Due to sustained heavy traffic, we have upgraded the WebSocket URL structure by introducing a **root** plus dedicated entry points for **Public / Market / Private** traffic. This separation improves stability, scalability, and operational isolation across different data types.

## What’s New

- **3 new WebSocket base URLs (Root + routed paths)**
  - Public (high-frequency public market data): `wss://fstream.binance.com/public`
  - Market (regular market data): `wss://fstream.binance.com/market`
  - Private (user data): `wss://fstream.binance.com/private`
- **Two access modes are supported**
  - `ws` mode: streams are composed in the URL path
  - `stream` mode: streams are passed via query (e.g., `?streams=`) — private uses `listenKey/events`
- **Combined streams remain supported**
- **Private supports listenKey + events subscription (multiple listenKeys and multiple events)**

## Examples

### Public / Market: combined subscriptions

- `ws`mode (path-based)

  - `wss://fstream.binance.com/public/ws/bnbusdt@depth/ethusdt@depth`
  - `wss://fstream.binance.com/market/ws/btcusdt@aggTrade/ethusdt@aggTrade`
- `stream`mode (query-based)

  - `wss://fstream.binance.com/market/stream?streams=bnbusdt@aggTrade/btcusdt@markPrice`
  - `wss://fstream.binance.com/public/stream?streams=btcusdt@depth/ethusdt@depth`

### Private: listenKey & events

- `ws`mode (listenKey + events)

  - `wss://fstream.binance.com/private/ws?listenKey=<listenKey1>&events=ORDER_TRADE_UPDATE/ACCOUNT_UPDATE`
- `stream`mode (multiple listenKeys + events)

  - `wss://fstream.binance.com/private/stream?listenKey=<listenKey1>&events=ORDER_TRADE_UPDATE&listenKey=<listenKey2>&events=ACCOUNT_UPDATE`

> JSON `SUBSCRIBE` is also supported; params may include market/public streams and listenKey event items.

## Endpoint & Stream Mapping (Excerpt)

### Public (high-frequency public data)

- Individual Symbol Book Ticker: `<symbol>@bookTicker`
- All Book Tickers: `!bookTicker`
- Partial Book Depth: `<symbol>@depth<levels>` (supports `@500ms` / `@100ms`)
- Diff. Book Depth: `<symbol>@depth` (supports `@500ms` / `@100ms`)

### Market (regular market data)

- Aggregate Trades: `<symbol>@aggTrade`
- Mark Price: `<symbol>@markPrice` or `<symbol>@markPrice@1s`
- Mark Price (All market): `!markPrice@arr` or `!markPrice@arr@1s`
- Kline/Candlestick: `<symbol>@kline_<interval>`
- Continuous Kline: `<pair>_<contractType>@continuousKline_<interval>`
- Mini Ticker: `<symbol>@miniTicker`; All: `!miniTicker@arr`
- Ticker: `<symbol>@ticker`; All: `!ticker@arr`
- Liquidations: `<symbol>@forceOrder`; All: `!forceOrder@arr`
- Composite Index: `<symbol>@compositeIndex`
- Contract Info: `!contractInfo`
- Multi-Assets Mode Asset Index: `!assetIndex@arr` or `<assetSymbol>@assetIndex`

## Compatibility & Migration Guidance

- **Legacy URLs will remain available during a transition period**, but users are strongly encouraged to migrate to the new `/public`, `/market`, `/private` endpoints for improved connection quality and clearer traffic separation.
- Recommended migration order:
1. High-frequency order book & core public feeds → `/public`
2. Regular market feeds (markPrice/kline/ticker, etc.) → `/market`
3. User data feeds (listenKey-based) → `/private`
- Client / SDK recommendations:
  - Split connections by traffic type (separate public/market/private sessions) to reduce per-connection load and jitter.
  - For combined subscriptions, prefer `stream` mode (`?streams=`; private uses listenKey/events).

## Action Required

- Update your WebSocket configuration:
  - Switch base URLs to `/public`, `/market`, `/private`
  - Ensure each stream is subscribed via the correct endpoint category
- After completing migration, gradually reduce reliance on legacy URLs to avoid peak-time connection instability.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Individual Symbol Book Ticker Streams

## Stream Description

Pushes any update to the best bid or ask's price or quantity in real-time for a specified symbol.

## URL PATH

`/public`

## Stream Name

`<symbol>@bookTicker`

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Update Speed

**Real-time**

## Response Example

```javascript
{
  "e":"bookTicker",			// event type
  "u":400900217,     		// order book updateId
  "E": 1568014460893,  		// event time
  "T": 1568014460891,  		// transaction time
  "s":"BNBUSDT",     		// symbol
  "b":"25.35190000", 		// best bid price
  "B":"31.21000000", 		// best bid qty
  "a":"25.36520000", 		// best ask price
  "A":"40.66000000"  		// best ask qty
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Individual Symbol Mini Ticker Stream

## Stream Description

24hr rolling window mini-ticker statistics for a single symbol. These are NOT the statistics of the UTC day, but a 24hr rolling window from requestTime to 24hrs before.

## URL PATH

`/market`

## Stream Name

`<symbol>@miniTicker`

## Update Speed

**2s**

## Response Example

```javascript
  {
    "e": "24hrMiniTicker",  // Event type
    "E": 123456789,         // Event time
    "s": "BTCUSDT",         // Symbol
    "c": "0.0025",          // Close price
    "o": "0.0010",          // Open price
    "h": "0.0025",          // High price
    "l": "0.0010",          // Low price
    "v": "10000",           // Total traded base asset volume
    "q": "18"               // Total traded quote asset volume
  }
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Individual Symbol Ticker Streams

## Stream Description

24hr rolling window ticker statistics for a single symbol. These are NOT the statistics of the UTC day, but a 24hr rolling window from requestTime to 24hrs before.

## URL PATH

`/market`

## Stream Name

`<symbol>@ticker`

## Update Speed

**2000ms**

## Response Example

```javascript
{
  "e": "24hrTicker",  // Event type
  "E": 123456789,     // Event time
  "s": "BTCUSDT",     // Symbol
  "p": "0.0015",      // Price change
  "P": "250.00",      // Price change percent
  "w": "0.0018",      // Weighted average price
  "c": "0.0025",      // Last price
  "Q": "10",          // Last quantity
  "o": "0.0010",      // Open price
  "h": "0.0025",      // High price
  "l": "0.0010",      // Low price
  "v": "10000",       // Total traded base asset volume
  "q": "18",          // Total traded quote asset volume
  "O": 0,             // Statistics open time
  "C": 86400000,      // Statistics close time
  "F": 0,             // First trade ID
  "L": 18150,         // Last trade Id
  "n": 18151          // Total number of trades
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Kline/Candlestick Streams

## Stream Description

The Kline/Candlestick Stream push updates to the current klines/candlestick every 250 milliseconds (if existing).

**Kline/Candlestick chart intervals:**

m -> minutes; h -> hours; d -> days; w -> weeks; M -> months

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

## URL PATH

`/market`

## Stream Name

`<symbol>@kline_<interval>`

## Update Speed

**250ms**

## Response Example

```javascript
{
  "e": "kline",     // Event type
  "E": 1638747660000,   // Event time
  "s": "BTCUSDT",    // Symbol
  "k": {
    "t": 1638747660000, // Kline start time
    "T": 1638747719999, // Kline close time
    "s": "BTCUSDT",  // Symbol
    "i": "1m",      // Interval
    "f": 100,       // First trade ID
    "L": 200,       // Last trade ID
    "o": "0.0010",  // Open price
    "c": "0.0020",  // Close price
    "h": "0.0025",  // High price
    "l": "0.0015",  // Low price
    "v": "1000",    // Base asset volume
    "n": 100,       // Number of trades
    "x": false,     // Is this kline closed?
    "q": "1.0000",  // Quote asset volume
    "V": "500",     // Taker buy base asset volume
    "Q": "0.500",   // Taker buy quote asset volume
    "B": "123456"   // Ignore
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Liquidation Order Streams

## Stream Description

The Liquidation Order Snapshot Streams push force liquidation order information for specific symbol.
For each symbol，only the latest one liquidation order within 1000ms will be pushed as the snapshot. If no liquidation happens in the interval of 1000ms, no stream will be pushed.

## URL PATH

`/market`

## Stream Name

`<symbol>@forceOrder`

## Update Speed

1000ms

## Response Example

```javascript
{

	"e":"forceOrder",                   // Event Type
	"E":1568014460893,                  // Event Time
	"o":{

		"s":"BTCUSDT",                   // Symbol
		"S":"SELL",                      // Side
		"o":"LIMIT",                     // Order Type
		"f":"IOC",                       // Time in Force
		"q":"0.014",                     // Original Quantity
		"p":"9910",                      // Price
		"ap":"9910",                     // Average Price
		"X":"FILLED",                    // Order Status
		"l":"0.014",                     // Order Last Filled Quantity
		"z":"0.014",                     // Order Filled Accumulated Quantity
		"T":1568014460893,          	 // Order Trade Time

	}

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Live Subscribing/Unsubscribing to streams

- The following data can be sent through the websocket instance in order to subscribe/unsubscribe from streams. Examples can be seen below.
- The `id` used in the JSON payloads is an unsigned INT used as an identifier to uniquely identify the messages going back and forth.

## Subscribe to a stream

> **Request**

```json
{
  "method": "SUBSCRIBE",
  "params":
  [
    "btcusdt@aggTrade",
    "btcusdt@depth"
  ],
  "id": 1
}
```

> **Response**

```json
{
  "result": null,
  "id": 1
}
```

## Unsubscribe to a stream

> **Request**

```json
{
  "method": "UNSUBSCRIBE",
  "params":
  [
    "btcusdt@depth"
  ],
  "id": 312
}
```

> **Response**

```json
{
  "result": null,
  "id": 312
}
```

## Listing Subscriptions

> **Request**

```json
{
  "method": "LIST_SUBSCRIPTIONS",
  "id": 3
}
```

> **Response**

```json
{
  "result": [
    "btcusdt@aggTrade"
  ],
  "id": 3
}
```

## Setting Properties

Currently, the only property can be set is to set whether `combined` stream payloads are enabled are not.
The combined property is set to `false` when connecting using `/ws/` ("raw streams") and `true` when connecting using `/stream/`.

> **Request**

```json
{
  "method": "SET_PROPERTY",
  "params":
  [
    "combined",
    true
  ],
  "id": 5
}
```

> **Response**

```json
{
  "result": null,
  "id": 5
}
```

## Retrieving Properties

> **Request**

```json
{
  "method": "GET_PROPERTY",
  "params":
  [
    "combined"
  ],
  "id": 2
}
```

> **Response**

```json
{
  "result": true, // Indicates that combined is set to true.
  "id": 2
}
```

### Error Messages

| Error Message | Description |
| --- | --- |
| {"code": 0, "msg": "Unknown property"} | Parameter used in the `SET_PROPERTY` or `GET_PROPERTY` was invalid |
| {"code": 1, "msg": "Invalid value type: expected Boolean"} | Value should only be `true` or `false` |
| {"code": 2, "msg": "Invalid request: property name must be a string"} | Property name provided was invalid |
| {"code": 2, "msg": "Invalid request: request ID must be an unsigned integer"} | Parameter `id` had to be provided or the value provided in the `id` parameter is an unsupported type |
| {"code": 2, "msg": "Invalid request: unknown variant %s, expected one of `SUBSCRIBE`, `UNSUBSCRIBE`, `LIST_SUBSCRIPTIONS`, `SET_PROPERTY`, `GET_PROPERTY` at line 1 column 28"} | Possible typo in the provided method or provided method was neither of the expected values |
| {"code": 2, "msg": "Invalid request: too many parameters"} | Unnecessary parameters provided in the data |
| {"code": 2, "msg": "Invalid request: property name must be a string"} | Property name was not provided |
| {"code": 2, "msg": "Invalid request: missing field `method` at line 1 column 73"} | `method` was not provided in the data |
| {"code":3,"msg":"Invalid JSON: expected value at line %s column %s"} | JSON data sent has incorrect syntax. |

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Mark Price Stream

## Stream Description

Mark price and funding rate for a single symbol pushed every 3 seconds or every second.

## URL PATH

`/market`

## Stream Name

`<symbol>@markPrice` or `<symbol>@markPrice@1s`

## Update Speed

**3000ms** or **1000ms**

## Response Example

```javascript
  {
    "e": "markPriceUpdate",  	// Event type
    "E": 1562305380000,      	// Event time
    "s": "BTCUSDT",          	// Symbol
    "p": "11794.15000000",   	// Mark price
    "ap": "11794.15000000",   // Mark price moving average
    "i": "11784.62659091",		// Index price
    "P": "11784.25641265",		// Estimated Settle Price, only useful in the last hour before the settlement starts
    "r": "0.00038167",       	// Funding rate
    "T": 1562306400000       	// Next funding time
  }
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Mark Price Stream for All market

## Stream Description

Mark price and funding rate for all symbols pushed every 3 seconds or every second.

**Note**:

> TradFi symbols will be pushed through a seperate message.

## URL PATH

`/market`

## Stream Name

`!markPrice@arr` or `!markPrice@arr@1s`

## Update Speed

**3000ms** or **1000ms**

## Response Example

```javascript
[
  {
    "e": "markPriceUpdate",  	// Event type
    "E": 1562305380000,      	// Event time
    "s": "BTCUSDT",          	// Symbol
    "p": "11185.87786614",   	// Mark price
    "ap": "11185.87786614",   // Mark price moving average
    "i": "11784.62659091",		// Index price
    "P": "11784.25641265",		// Estimated Settle Price, only useful in the last hour before the settlement starts
    "r": "0.00030000",       	// Funding rate
    "T": 1562306400000       	// Next funding time
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Multi-Assets Mode Asset Index

## Stream Description

Asset index for multi-assets mode user

## URL PATH

`/market`

## Stream Name

`!assetIndex@arr` OR `<assetSymbol>@assetIndex`

## Update Speed

**1s**

## Response Example

```javascript
[
    {
      "e":"assetIndexUpdate",
      "E":1686749230000,
      "s":"ADAUSD",           // asset index symbol
      "i":"0.27462452",       // index price
      "b":"0.10000000",       // bid buffer
      "a":"0.10000000",       // ask buffer
      "B":"0.24716207",       // bid rate
      "A":"0.30208698",       // ask rate
      "q":"0.05000000",       // auto exchange bid buffer
      "g":"0.05000000",       // auto exchange ask buffer
      "Q":"0.26089330",       // auto exchange bid rate
      "G":"0.28835575"        // auto exchange ask rate
    },
    {
      "e":"assetIndexUpdate",
      "E":1686749230000,
      "s":"USDTUSD",
      "i":"0.99987691",
      "b":"0.00010000",
      "a":"0.00010000",
      "B":"0.99977692",
      "A":"0.99997689",
      "q":"0.00010000",
      "g":"0.00010000",
      "Q":"0.99977692",
      "G":"0.99997689"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Partial Book Depth Streams

## Stream Description

Top **<levels>** bids and asks, Valid **<levels>** are 5, 10, or 20.

## URL PATH

`/public`

## Stream Name

`<symbol>@depth<levels>` OR `<symbol>@depth<levels>@500ms` OR `<symbol>@depth<levels>@100ms`.

**Note**:

> Retail Price Improvement(RPI) orders are not visible and excluded in the response message.

## Update Speed

**250ms**, **500ms** or **100ms**

## Response Example

```javascript
{
  "e": "depthUpdate", // Event type
  "E": 1571889248277, // Event time
  "T": 1571889248276, // Transaction time
  "s": "BTCUSDT",
  "U": 390497796,     // First update ID in event
  "u": 390497878,     // Final update ID in event
  "pu": 390497794,    // Final update Id in last stream(ie `u` in last stream)
  "b": [              // Bids to be updated
    [
      "7403.89",      // Price Level to be updated
      "0.002"         // Quantity
    ],
    [
      "7403.90",
      "3.906"
    ],
    [
      "7404.00",
      "1.428"
    ],
    [
      "7404.85",
      "5.239"
    ],
    [
      "7405.43",
      "2.562"
    ]
  ],
  "a": [              // Asks to be updated
    [
      "7405.96",      // Price level to be
      "3.340"         // Quantity
    ],
    [
      "7406.63",
      "4.525"
    ],
    [
      "7407.08",
      "2.475"
    ],
    [
      "7407.15",
      "4.800"
    ],
    [
      "7407.20",
      "0.175"
    ]
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Trading Session Stream

## Stream Description

Trading session information for the underlying assets of TradFi Perpetual contracts—covering the U.S. equity market and the commodity market—is updated every second. Trading session information for different underlying markets is pushed in separate messages. Session types for the equity market include "PRE\_MARKET", "REGULAR", "AFTER\_MARKET", "OVERNIGHT", and "NO\_TRADING". Session types for the commodity market include "REGULAR" and "NO\_TRADING".

## URL PATH

`/market`

## Stream Name

`tradingSession`

## Update Speed

**1s**

## Response Example

```javascript
  {
    "e": "EquityUpdate",  	// Event type, can also be CommodityUpdate
    "E": 1765244143062,     // Event time
    "t": 1765242000000,   	// Session start time
    "T": 1765270800000,		  // Session end time
    "S": "OVERNIGHT"        // Session type
  }
```

 

Copyright © 2026 Binance.

---
