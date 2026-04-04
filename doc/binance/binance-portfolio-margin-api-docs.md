# Binance Portfolio Margin API Documentation

Source: https://developers.binance.com/docs/

---

## Repo Usage Quick Reference

- Primary repo use: currently limited; this repo mainly uses classic cross-margin plus USDT-M futures rather than portfolio-margin routing
- Base URL in this doc: `https://papi.binance.com`
- Repo symbol format: `BTCUSDT`
- Most relevant sections if portfolio margin work is added:
  - account state and balances
  - order placement / cancel / query
  - margin and risk endpoints
- Important repo note: before using any `papi` endpoint in this codebase, verify the adapter is actually operating in portfolio-margin mode; most current Binance logic is built around `api.binance.com` plus `fapi.binance.com`

Derivatives Trading

# General Info

## General API Information

- The base endpoint is: **[https://papi.binance.com](https://papi.binance.com/)**
- All endpoints return either a JSON object or raw primitive.
- Data is returned in ascending order. Oldest first, newest last.
- All time and timestamp related fields are in UTC milliseconds.
- All data types adopt definition in JAVA.

### HTTP Return Codes

- HTTP `4XX` return codes are used for for malformed requests; the issue is on the sender's side.
- HTTP `403` return code is used when the WAF Limit (Web Application Firewall) has been violated.
- HTTP `429` return code is used when breaking a request rate limit.
- HTTP `418` return code is used when an IP has been auto-banned for continuing to send requests after receiving `429` codes.
- HTTP `5XX` return codes are used for internal errors; the issue is on Binance's side.

1. If there is an error message "Request occur unknown error.", please retry later.
- HTTP `503` return code is used when:

1. If there is an error message "Unknown error, please check your request or try again later." returned in the response, the API successfully sent the request but not get a response within the timeout period.It is important to NOT treat this as a failure operation; the execution status is UNKNOWN and could have been a success;
2. If there is an error message "Service Unavailable." returned in the response, it means this is a failure API operation and the service might be unavailable at the moment, you need to retry later.
3. If there is an error message "Internal error; unable to process your request. Please try again." returned in the response, it means this is a failure API operation and you can resend your request if you need.
4. If the response contains the error message **"Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again." (-1008)**, This indicates the node has exceeded its maximum concurrency and is temporarily throttled. Close-position, reduce-only, and cancel orders are exempt and will not receive this error.

### HTTP 503 Status: Message Variants & Handling

* * *

#### A. “Unknown error, please check your request or try again later.” (Execution status **unknown**)

- **Meaning**: Request accepted but no response before timeout; **execution may have succeeded**.
- **Handling**:

  - **Do not treat as immediate failure**; first verify via **WebSocket updates** or **orderId queries** to avoid duplicates.
  - During peaks, prefer **single orders** over batch to reduce uncertainty.
- **Rate-limit counting**: **May or may not** count, check header to verify rate limit info

* * *

#### B. “Service Unavailable.” (Failure)

- **Meaning**: Service temporarily unavailable; **100% failure**.
- **Handling**: **Retry with exponential backoff** (e.g., 200ms → 400ms → 800ms, max 3–5 attempts).
- **Rate-limit counting**: **not counted**

* * *

##### C. “Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again.” ( **-1008**, Failure)

- **Meaning**: System overload; **100% failure**.
- **Handling**: **Retry with backoff** and **reduce concurrency**;
- **Applicable endpoints**:

  - `POST /fapi/v1/order` / `POST /dapi/v1/order` / `POST /papi/v1/order`
  - `POST /fapi/v1/batchOrders` / `POST /dapi/v1/batchOrders` / `POST papi/v1/batchOrders`
- **Rate-limit counting**: **Not counted** (overload protection).
- **Exception integrated here**: When a request **reduces exposure** (Reduce-only / Close-position: `closePosition = true`, or `positionSide = BOTH` with `reduceOnly = true`, or `LONG+SELL`, or `SHORT+BUY`), it is **not affected or prioritized under -1008** to ensure risk reduction.

  - Covered endpoints: `POST /fapi/v1/order`、`POST /dapi/v1/order`、`POST /papi/v1/order`、`POST /fapi/v1/batchOrders`、`POST /dapi/v1/batchOrders`、`POST /papi/v1/batchOrders` (when parameters satisfy the condition)

### Error Codes and Messages

- Any endpoint can return an ERROR
- Specific error codes and messages defined in Error Codes.

### General Information on Endpoints

- For `GET` endpoints, parameters must be sent as a `query string`.
- For `POST`, `PUT`, and `DELETE` endpoints, the parameters may be sent as a `query string` or in the `request body` with content type `application/x-www-form-urlencoded`. You may mix parameters between both the `query string` and `request body` if you wish to do so.
- Parameters may be sent in any order.
- If a parameter sent in both the `query string` and `request body`, the `query string` parameter will be used.

## LIMITS

- A `429` will be returned when either rate limit is violated.

Binance has the right to further tighten the rate limits on users with intent to attack.

### IP Limits

- Every request will contain `X-MBX-USED-WEIGHT-(intervalNum)(intervalLetter)` in the response headers which has the current used weight for the IP for all request rate limiters defined.
- Each route has a `weight` which determines for the number of requests each endpoint counts for. Heavier endpoints and endpoints that do operations on multiple symbols will have a heavier `weight`.
- When a `429` is received, it's your obligation as an API to back off and not spam the API.
- **Repeatedly violating rate limits and/or failing to back off after receiving 429s will result in an automated IP ban (HTTP status 418).**
- IP bans are tracked and **scale in duration** for repeat offenders, **from 2 minutes to 3 days**.
- **The limits on the API are based on the IPs, not the API keys.**
- Portfolio Margin IP Limit is 6000/min.

It is strongly recommended to use websocket stream for getting data as much as possible, which can not only ensure the timeliness of the message, but also reduce the access restriction pressure caused by the request.

### Order Rate Limits

- Every order response will contain a `X-MBX-ORDER-COUNT-(intervalNum)(intervalLetter)` header which has the current order count for the account for all order rate limiters defined.
- Rejected/unsuccessful orders are not guaranteed to have `X-MBX-ORDER-COUNT-**` headers in the response.
- **The order rate limit is counted against each account**.
- Portfolio Margin Order Limits are 1200/min.

## Endpoint Security Type

- Each endpoint has a security type that determines the how you will interact with it.
- API-keys are passed into the Rest API via the `X-MBX-APIKEY` header.
- API-keys and secret-keys are **case sensitive**.
- API-keys can be configured to only access certain types of secure endpoints. For example, one API-key could be used for TRADE only, while another API-key can access everything except for TRADE routes.
- By default, API-keys can access all secure routes.

| Security Type | Description |
| --- | --- |
| NONE | Endpoint can be accessed freely. |
| TRADE | Endpoint requires sending a valid API-Key and signature. |
| USER\_DATA | Endpoint requires sending a valid API-Key and signature. |
| USER\_STREAM | Endpoint requires sending a valid API-Key and signature. |

## SIGNED (TRADE and USER\_DATA) Endpoint Security

- `SIGNED` endpoints require an additional parameter, signature, to be sent in the `query string` or `request body`.
- Endpoints use `HMAC SHA256` signatures. The `HMAC SHA256` signature is a keyed `HMAC SHA256` operation. Use your `secretKey` as the key and `totalParams` as the value for the HMAC operation.
- The `signature` is not case sensitive.
- Please make sure the `signature` is the end part of your `query string` or `request body`.
- `totalParam`s is defined as the `query string` concatenated with the `request body`.

### Timing security

- A `SIGNED` endpoint also requires a parameter, `timestamp`, to be sent which should be the millisecond timestamp of when the request was created and sent.
- An additional parameter, `recvWindow`, may be sent to specify the number of milliseconds after `timestamp` the request is valid for. If `recvWindow` is not sent, **it defaults to 5000**. `recvWindow` cannot exceed 60000.
- If the server determines that the timestamp sent by the client is more than one second in the future of the server time, the request will also be rejected.

**Serious trading is about timing.** Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With `recvWindow`, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less! The max cannot go beyond 60,000!

### SIGNED Endpoint Examples for POST /papi/v1/um/order

Here is a step-by-step example of how to send a valid signed payload from the
Linux command line using `echo`, `openssl`, and `curl`.

| Key | Value |
| --- | --- |
| apiKey | 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn |
| secretKey | YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG |

| Parameter | Value |
| --- | --- |
| symbol | BTCUSDT |
| side | BUY |
| type | LIMIT |
| timeInForce | GTC |
| quantity | 1 |
| price | 2000 |
| recvWindow | 5000 |
| timestamp | 1611825601400 |

#### Example 1: As a request body

> **Example 1**

> **HMAC SHA256 signature:**

```shell
    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= 7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7
```

> **curl command:**

```shell
    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order' -d 'symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400&signature=7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7'

```

- **requestBody:**

symbol=BTCUSDT
&side=BUY

&type=LIMIT

&timeInForce=GTC

&quantity=1

&price=2000

&recvWindow=5000

&timestamp=1611825601400

#### Example 2: As a query string

> **Example 2**

> **HMAC SHA256 signature:**

```shell
    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= 7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7
```

> **curl command:**

```shell
    (HMAC SHA256)
   $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400&signature=7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7'
```

- **queryString:**

symbol=BTCUSDT

&side=BUY

&type=LIMIT

&timeInForce=GTC

&quantity=1

&price=2000

&recvWindow=5000

&timestamp=1611825601400

#### Example 3: Mixed query string and request body

> **Example 3**

> **HMAC SHA256 signature:**

```shell
   $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTCquantity=0.01&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= fa6045c54fb02912b766442be1f66fab619217e551a4fb4f8a1ee000df914d8e
```

> **curl command:**

```shell
    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC' -d 'quantity=0.01&price=2000&recvWindow=5000&timestamp=1611825601400&signature=fa6045c54fb02912b766442be1f66fab619217e551a4fb4f8a1ee000df914d8e'
```

- **queryString:**

symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC

- **requestBody:**

quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400

Note that the signature is different in example 3.
There is no & between "GTC" and "quantity=1".

### RSA Keys - SIGNED Endpoint Examples for POST /papi/v1/um/order

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
| side | BUY |
| type | LIMIT |
| timeInForce | GTC |
| quantity | 1 |
| price | 2000 |
| recvWindow | 5000 |
| timestamp | 1611825601400 |

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
 curl -H "X-MBX-APIKEY: vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2" -X POST 'https://papi.binance.com/papi/v1/um/order?timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23&signature=aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D'
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
    "https://papi.binance.com/papi/$apiCall?$paramsWithTs&signature=$signature"
```

A sample Bash script containing similar steps is available in the right side.

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Account Balance(USER\_DATA)

## API Description

Query account balance

## HTTP Request

GET `/papi/v1/balance`

## Request Weight

**20**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "asset": "USDT",    // asset name
        "totalWalletBalance": "122607.35137903", // wallet balance =  cross margin free + cross margin locked + UM wallet balance + CM wallet balance
        "crossMarginAsset": "92.27530794", // crossMarginAsset = crossMarginFree + crossMarginLocked
        "crossMarginBorrowed": "10.00000000", // principal of cross margin
        "crossMarginFree": "100.00000000", // free asset of cross margin
        "crossMarginInterest": "0.72469206", // interest of cross margin
        "crossMarginLocked": "3.00000000", //lock asset of cross margin
        "umWalletBalance": "0.00000000",  // wallet balance of um
        "umUnrealizedPNL": "23.72469206",     // unrealized profit of um
        "cmWalletBalance": "23.72469206",       // wallet balance of cm
        "cmUnrealizedPNL": "",    // unrealized profit of cm
        "updateTime": 1617939110373,
        "negativeBalance": "0"
    }
]
```

**OR (when asset sent)**

````javascript
{
    "asset": "USDT",    // asset name
    "totalWalletBalance": "122607.35137903", // wallet balance =  cross margin free + cross margin locked + UM wallet balance + CM wallet balance
    "crossMarginBorrowed": "10.00000000", // principal of cross margin
    "crossMarginFree": "100.00000000", // free asset of cross margin
    "crossMarginInterest": "0.72469206", // interest of cross margin
    "crossMarginLocked": "3.00000000", //lock asset of cross margin
    "umWalletBalance": "0.00000000",  // wallet balance of um
    "umUnrealizedPNL": "23.72469206",     // unrealized profit of um
    "cmWalletBalance": "23.72469206",       // wallet balance of cm
    "cmUnrealizedPNL": "",    // unrealized profit of cm
    "updateTime": 1617939110373,
    "negativeBalance": "0"
}
```
````

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Account Balance(USER\_DATA)

## API Description

Query account balance

## HTTP Request

GET `/papi/v1/balance`

## Request Weight

**20**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "asset": "USDT",    // asset name
        "totalWalletBalance": "122607.35137903", // wallet balance =  cross margin free + cross margin locked + UM wallet balance + CM wallet balance
        "crossMarginAsset": "92.27530794", // crossMarginAsset = crossMarginFree + crossMarginLocked
        "crossMarginBorrowed": "10.00000000", // principal of cross margin
        "crossMarginFree": "100.00000000", // free asset of cross margin
        "crossMarginInterest": "0.72469206", // interest of cross margin
        "crossMarginLocked": "3.00000000", //lock asset of cross margin
        "umWalletBalance": "0.00000000",  // wallet balance of um
        "umUnrealizedPNL": "23.72469206",     // unrealized profit of um
        "cmWalletBalance": "23.72469206",       // wallet balance of cm
        "cmUnrealizedPNL": "",    // unrealized profit of cm
        "updateTime": 1617939110373,
        "negativeBalance": "0"
    }
]
```

**OR (when asset sent)**

````javascript
{
    "asset": "USDT",    // asset name
    "totalWalletBalance": "122607.35137903", // wallet balance =  cross margin free + cross margin locked + UM wallet balance + CM wallet balance
    "crossMarginBorrowed": "10.00000000", // principal of cross margin
    "crossMarginFree": "100.00000000", // free asset of cross margin
    "crossMarginInterest": "0.72469206", // interest of cross margin
    "crossMarginLocked": "3.00000000", //lock asset of cross margin
    "umWalletBalance": "0.00000000",  // wallet balance of um
    "umUnrealizedPNL": "23.72469206",     // unrealized profit of um
    "cmWalletBalance": "23.72469206",       // wallet balance of cm
    "cmUnrealizedPNL": "",    // unrealized profit of cm
    "updateTime": 1617939110373,
    "negativeBalance": "0"
}
```
````

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Account Information(USER\_DATA)

## API Description

Query account information

## HTTP Request

GET `/papi/v1/account`

## Request Weight

**20**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
   "uniMMR": "5167.92171923",        // Portfolio margin account maintenance margin rate
   "accountEquity": "122607.35137903",   // Account equity, in USD value
   "actualEquity": "73.47428058",   //Account equity without collateral rate, in USD value
   "accountInitialMargin": "23.72469206",
   "accountMaintMargin": "23.72469206", // Portfolio margin account maintenance margin, unit：USD
   "accountStatus": "NORMAL"   // Portfolio margin account status:"NORMAL", "MARGIN_CALL", "SUPPLY_MARGIN", "REDUCE_ONLY", "ACTIVE_LIQUIDATION", "FORCE_LIQUIDATION", "BANKRUPTED"
   "virtualMaxWithdrawAmount": "1627523.32459208"   // Portfolio margin maximum amount for transfer out in USD
   "totalAvailableBalance":"",
   "totalMarginOpenLoss":"", // in USD margin open order
   "updateTime": 1657707212154 // last update time
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# BNB transfer (TRADE)

## API Description

Transfer BNB in and out of UM

## HTTP Request

POST `/papi/v1/bnb-transfer`

## Request Weight(IP)

**750**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| amount | DECIMAL | YES |  |
| transferSide | STRING | YES | "TO\_UM","FROM\_UM" |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - The endpoint can only be called 10 times per 10 minutes in a rolling manner

## Response Example

```javascript
{
    "tranId": 100000001       //transaction id
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# CM Notional and Leverage Brackets(USER\_DATA)

## API Description

Query CM notional and leverage brackets

## HTTP Request

GET `/papi/v1/cm/leverageBracket`

## Request Weight

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
        "symbol": "BTCUSD_PERP",
        "brackets": [
            {
                "bracket": 1,   // bracket level
                "initialLeverage": 125,  // the maximum leverage
                "qtyCap": 50,  // upper edge of base asset quantity
                "qtyFloor": 0,  // lower edge of base asset quantity
                "maintMarginRatio": 0.004, // maintenance margin rate
                "cum": 0.0 // Auxiliary number for quick calculation
            },
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Change Auto-repay-futures Status(TRADE)

## API Description

Change Auto-repay-futures Status

## HTTP Request

POST `/papi/v1/repay-futures-switch`

## Request Weight(IP)

**750**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| autoRepay | STRING | YES | Default: `true`; `false` for turn off the auto-repay futures negative balance function |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Change CM Initial Leverage (TRADE)

## API Description

Change user's initial leverage of specific symbol in CM.

## HTTP Request

POST `/papi/v1/cm/leverage`

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
    "maxQty": "1000",
    "symbol": "BTCUSD_200925"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Change CM Position Mode(TRADE)

## API Description

Change user's position mode (Hedge Mode or One-way Mode ) on EVERY symbol in CM

## HTTP Request

POST `/papi/v1/cm/positionSide/dual`

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

# Change UM Initial Leverage(TRADE)

## API Description

Change user's initial leverage of specific symbol in UM.

## HTTP Request

POST `/papi/v1/um/leverage`

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

# Change UM Position Mode(TRADE)

## API Description

Change user's position mode (Hedge Mode or One-way Mode ) on EVERY symbol in UM

## HTTP Request

POST `/papi/v1/um/positionSide/dual`

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

# Fund Auto-collection(TRADE)

## API Description

Fund collection for Portfolio Margin

## HTTP Request

`POST /papi/v1/auto-collection`

## Request Weight(IP)

**750**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - The BNB would not be collected from UM-PM account to the Portfolio Margin account.
> - You can only use this function 500 times per hour in a rolling manner.

## Response Example

```javascript
{
    "msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Fund Collection by Asset(TRADE)

## API Description

Transfers specific asset from Futures Account to Margin account

## HTTP Request

POST `/papi/v1/asset-collection`

## Request Weight(IP)

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - The BNB transfer is not be supported

## Response Example

```javascript
{
    "msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get Auto-repay-futures Status(USER\_DATA)

## API Description

Query Auto-repay-futures Status

## HTTP Request

GET `/papi/v1/repay-futures-switch`

## Request Weight(IP)

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "autoRepay": true  //  "true" for turn on the auto-repay futures; "false" for turn off the auto-repay futures
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get CM Account Detail(USER\_DATA)

## API Description

Get current CM account asset and position information.

## HTTP Request

GET `/papi/v1/cm/account`

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
    "assets": [
        {
            "asset": "BTC",  // asset name
            "crossWalletBalance": "0.00241969",  // total wallet balance
            "crossUnPnl": "0.00000000",  // unrealized profit or loss
            "maintMargin": "0.00000000",    // maintenance margin
            "initialMargin": "0.00000000",  // total intial margin required with the latest mark price
            "positionInitialMargin": "0.00000000",  // positions" margin required with the latest mark price
            "openOrderInitialMargin": "0.00000000",  // open orders" intial margin required with the latest mark price
            "updateTime": 1625474304765 // last update time
         }
     ],
     "positions": [
         {
            "symbol": "BTCUSD_201225",
            "positionAmt":"0",  // position amount
            "initialMargin": "0",
            "maintMargin": "0",
            "unrealizedProfit": "0.00000000",
            "positionInitialMargin": "0",
            "openOrderInitialMargin": "0",
            "leverage": "125",
            "positionSide": "BOTH", // BOTH means that it is the position of One-way Mode
            "entryPrice": "0.0",
            "maxQty": "50",  // maximum quantity of base asset
            "updateTime": 0
        }
     ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get CM Current Position Mode(USER\_DATA)

## API Description

Get user's position mode (Hedge Mode or One-way Mode ) on EVERY symbol in CM

## HTTP Request

GET `/papi/v1/cm/positionSide/dual`

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
  "dualSidePosition": true // "true": Hedge Mode; "false": One-way Mode
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get CM Income History(USER\_DATA)

## API Description

Get CM Income History

## HTTP Request

GET `/papi/v1/cm/income`

## Request Weight

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| incomeType | STRING | NO | "TRANSFER","WELCOME\_BONUS", "FUNDING\_FEE", "REALIZED\_PNL", "COMMISSION", "INSURANCE\_CLEAR", and "DELIVERED\_SETTELMENT" |
| startTime | LONG | NO | Timestamp in ms to get funding from INCLUSIVE. |
| endTime | LONG | NO | Timestamp in ms to get funding until INCLUSIVE. |
| page | INT | NO |  |
| limit | INT | NO | Default 100; max 1000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If `incomeType` is not sent, all kinds of flow will be returned
> - "trandId" is unique in the same "incomeType" for a user
> - The interval between `startTime` and `endTime` can not exceed 200 days:
>
>   - If `startTime` and `endTime` are not sent, the last 200 days will be returned

## Response Example

```javascript
[
    {
        "symbol": "",               // trade symbol, if existing
        "incomeType": "TRANSFER",   // income type
        "income": "-0.37500000",    // income amount
        "asset": "BTC",             // income asset
        "info":"WITHDRAW",          // extra information
        "time": 1570608000000,
        "tranId":"9689322392",      // transaction id
        "tradeId":""                // trade id, if existing
    },
    {
        "symbol": "BTCUSD_200925",
        "incomeType": "COMMISSION",
        "income": "-0.01000000",
        "asset": "BTC",
        "info":"",
        "time": 1570636800000,
        "tranId":"9689322392",
        "tradeId":"2059192"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get Download Id For UM Futures Order History (USER\_DATA)

## API Description

Get download id for UM futures order history

## HTTP Request

GET `/papi/v1/um/order/asyn`

## Request Weight

**1500**

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

# Get Download Id For UM Futures Trade History (USER\_DATA)

## API Description

Get download id for UM futures trade history

## HTTP Request

GET `/papi/v1/um/trade/asyn`

## Request Weight

**1500**

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

# Get Download Id For UM Futures Transaction History (USER\_DATA)

## API Description

Get download id for UM futures transaction history

## HTTP Request

GET `/papi/v1/um/income/asyn`

## Request Weight

**1500**

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

# Get Margin Borrow/Loan Interest History(USER\_DATA)

## API Description

Get Margin Borrow/Loan Interest History

## HTTP Request

GET `/papi/v1/margin/marginInterestHistory`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| archived | STRING | NO | Default: `false`. Set to `true` for archived data from 6 months ago |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

> - Response in descending order
> - The max interval between startTime and endTime is 30 days. It is a MUST to ensure data correctness.
> - If `startTime` and `endTime` not sent, return records of the last 7 days by default
> - If `startTime` is sent and `endTime` is not sent, the records from `startTime` to the present will be returned; if `startTime` is more than 30 days ago, the records of the past 30 days will be returned.
> - If `startTime` is not sent and `endTime` is sent, the records of the 7 days before `endTime` is returned.
> - Type in response has 5 enums:
>   - `PERIODIC` interest charged per hour
>   - `ON_BORROW` first interest charged on borrow
>   - `PERIODIC_CONVERTED` interest charged per hour converted into BNB
>   - `ON_BORROW_CONVERTED` first interest charged on borrow converted into BNB
>   - `PORTFOLIO` Portfolio Margin negative balance daily interest

## Response Example

```javascript
{
  "rows": [
    {
      "txId": 1352286576452864727,
      "interestAccuredTime": 1672160400000,
      "asset": "USDT",
      "rawAsset": “USDT”,
      "principal": "45.3313",
      "interest": "0.00024995",
      "interestRate": "0.00013233",
      "type": "ON_BORROW"
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Account Detail(USER\_DATA)

## API Description

Get current UM account asset and position information.

## HTTP Request

GET `/papi/v1/um/account`

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
    "assets": [
        {
            "asset": "USDT",            // asset name
            "crossWalletBalance": "23.72469206",      // wallet balance
            "crossUnPnl": "0.00000000",    // unrealized profit
            "maintMargin": "0.00000000",        // maintenance margin required
            "initialMargin": "0.00000000",    // total initial margin required with current mark price
            "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
            "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
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
            "entryPrice": "0.00000",    // average entry price
            "maxNotional": "250000",    // maximum available notional with current leverage
            "bidNotional": "0",  // bids notional, ignore
            "askNotional": "0",  // ask notional, ignore
            "positionSide": "BOTH",     // position side
            "positionAmt": "0",         // position amount
            "updateTime": 0           // last update time
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Account Detail V2(USER\_DATA)

## API Description

Get current UM account asset and position information.

## HTTP Request

GET `/papi/v2/um/account`

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
    "assets": [
        {
            "asset": "USDT",            // asset name
            "crossWalletBalance": "23.72469206",      // wallet balance
            "crossUnPnl": "0.00000000",    // unrealized profit
            "maintMargin": "0.00000000",        // maintenance margin required
            "initialMargin": "0.00000000",    // total initial margin required with current mark price
            "positionInitialMargin": "0.00000000",    //initial margin required for positions with current mark price
            "openOrderInitialMargin": "0.00000000",   // initial margin required for open orders with current mark price
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
            "positionSide": "BOTH",     // position side
            "positionAmt": "0",         // position amount
            "updateTime": 0,           // last update time
            "notional": "86.98650000"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Current Position Mode(USER\_DATA)

## API Description

Get user's position mode (Hedge Mode or One-way Mode ) on EVERY symbol in UM

## HTTP Request

GET `/papi/v1/um/positionSide/dual`

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
    "dualSidePosition": true // "true": Hedge Mode; "false": One-way Mode
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# UM Futures Account Configuration(USER\_DATA)

## API Description

Query UM Futures account configuration

## HTTP Request

GET `/papi/v1/um/accountConfig`

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
    "updateTime": 1724416653850,            // reserved property, please ignore
    "multiAssetsMargin": false,
    "tradeGroupId": -1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Futures Order Download Link by Id(USER\_DATA)

## API Description

Get UM futures order download link by Id

## HTTP Request

GET `/papi/v1/um/order/asyn/id`

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
	"s3Link": null,
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
	"s3Link": null,
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# UM Futures Symbol Configuration(USER\_DATA)

## API Description

Get current UM account symbol configuration.

## HTTP Request

GET `/papi/v1/um/symbolConfig`

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
  "isAutoAddMargin": "false",
  "leverage": 21,
  "maxNotionalValue": "1000000",
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Futures Trade Download Link by Id(USER\_DATA)

## API Description

Get UM futures trade download link by Id

## HTTP Request

GET `/papi/v1/um/trade/asyn/id`

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
	"s3Link": null,
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
	"s3Link": null,
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Futures Transaction Download Link by Id(USER\_DATA)

## API Description

Get UM futures Transaction download link by Id

## HTTP Request

GET `/papi/v1/um/income/asyn/id`

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
	"s3Link": null,
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
	"s3Link": null,
  	"notified":false,
  	"expirationTimestamp":-1
  	"isExpired":null,

}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Income History(USER\_DATA)

## API Description

Get UM Income History

## HTTP Request

GET `/papi/v1/um/income`

## Request Weight

**30**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| incomeType | STRING | NO | TRANSFER, WELCOME\_BONUS, REALIZED\_PNL, FUNDING\_FEE, COMMISSION, INSURANCE\_CLEAR, REFERRAL\_KICKBACK, COMMISSION\_REBATE, API\_REBATE, CONTEST\_REWARD, CROSS\_COLLATERAL\_TRANSFER, OPTIONS\_PREMIUM\_FEE, OPTIONS\_SETTLE\_PROFIT, INTERNAL\_TRANSFER, AUTO\_EXCHANGE, DELIVERED\_SETTELMENT, COIN\_SWAP\_DEPOSIT, COIN\_SWAP\_WITHDRAW, POSITION\_LIMIT\_INCREASE\_FEE |
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
        "symbol": "",                   // trade symbol, if existing
        "incomeType": "TRANSFER",   // income type
        "income": "-0.37500000",  // income amount
        "asset": "USDT",                // income asset
        "info":"TRANSFER",          // extra information
        "time": 1570608000000,
        "tranId":"9689322392",      // transaction id
        "tradeId":""                    // trade id, if existing
    },
    {
        "symbol": "BTCUSDT",
        "incomeType": "COMMISSION",
        "income": "-0.01000000",
        "asset": "USDT",
        "info":"COMMISSION",
        "time": 1570636800000,
        "tranId":"9689322392",
        "tradeId":"2059192"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get User Commission Rate for CM(USER\_DATA)

## API Description

Get User Commission Rate for CM

## HTTP Request

GET `/papi/v1/cm/commissionRate`

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
    "symbol": "BTCUSD_PERP",
    "makerCommissionRate": "0.00015",  // 0.015%
    "takerCommissionRate": "0.00040"   // 0.040%
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get User Commission Rate for UM(USER\_DATA)

## API Description

Get User Commission Rate for UM

## HTTP Request

GET `/papi/v1/um/commissionRate`

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
    "takerCommissionRate": "0.0004"   // 0.04%
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Margin Max Borrow(USER\_DATA)

## API Description

Query margin max borrow

## HTTP Request

GET `/papi/v1/margin/maxBorrowable`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "amount": "1.69248805", // account's currently max borrowable amount with sufficient system availability
  "borrowLimit": "60" // max borrowable amount limited by the account level
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Portfolio Margin UM Trading Quantitative Rules Indicators(USER\_DATA)

## API Description

Portfolio Margin UM Trading Quantitative Rules Indicators

## HTTP Request

GET `/papi/v1/um/apiTradingStatus`

## Request Weight

**1** for a single symbol
**10** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

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
        ]
    },
    "updateTime": 1545741270000
}
```

Or (account violation triggered)

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

# Query CM Position Information(USER\_DATA)

## API Description

Get current CM position information.

## HTTP Request

GET `/papi/v1/cm/positionRisk`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| marginAsset | STRING | NO |  |
| pair | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If neither `marginAsset` nor `pair` is sent, positions of all symbols with `TRADING` status will be returned.
> - for One-way Mode user, the response will only show the "BOTH" positions
> - for Hedge Mode user, the response will show "LONG", and "SHORT" positions.

**Note**

> - Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.

## Response Example

- For One-way position mode:

```javascript
[
    {
        "symbol": "BTCUSD_201225",
        "positionAmt": "1",
        "entryPrice": "11707.70000003",
        "markPrice": "11788.66626667",
        "unRealizedProfit": "0.00005866",
        "liquidationPrice": "6170.20509059",
        "leverage": "125",
        "positionSide": "LONG",
        "updateTime": 1627026881327,
        "maxQty": "50",
        "notionalValue": "0.00084827"
    }
]
```

> - For Hedge position mode(only return with position):

```javascript
[
    {
        "symbol": "BTCUSD_201225",
        "positionAmt": "1",
        "entryPrice": "11707.70000003",
        "markPrice": "11788.66626667",
        "unRealizedProfit": "0.00005866",
        "liquidationPrice": "6170.20509059",
        "leverage": "125",
        "positionSide": "LONG",
        "updateTime": 1627026881327,
        "maxQty": "50",
        "notionalValue": "0.00084827"
    },
    {
        "symbol": "BTCUSD_201225",
        "positionAmt": "1",
        "entryPrice": "11707.70000003",
        "markPrice": "11788.66626667",
        "unRealizedProfit": "0.00005866",
        "liquidationPrice": "6170.20509059",
        "leverage": "125",
        "positionSide": "LONG",
        "updateTime": 1627026881327,
        "maxQty": "50",
        "notionalValue": "0.00084827"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Margin Loan Record(USER\_DATA)

## API Description

Query margin loan record

## HTTP Request

GET `/papi/v1/margin/marginLoan`

## Request Weight

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| txId | LONG | NO | the `tranId` in `POST/papi/v1/marginLoan` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| archived | STRING | NO | Default: `false`. Set to `true` for archived data from 6 months ago |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - txId or startTime must be sent. txId takes precedence.
> - Response in descending order
> - The max interval between `startTime` and `endTime` is 30 days.
> - If `startTime` and `endTime` not sent, return records of the last 7 days by default
> - Set `archived` to `true` to query data from 6 months ago

## Response Example

```javascript
{
  "rows": [
    {
        "txId": 12807067523,
        "asset": "BNB",
        "principal": "0.84624403",
        "timestamp": 1555056425000,
        "status": "CONFIRMED"   //one of PENDING (pending execution), CONFIRMED (successfully loaned), FAILED (execution failed, nothing happened to your account);
    }
  ],
  "total": 1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Margin Max Withdraw(USER\_DATA)

## API Description

Query Margin Max Withdraw

## HTTP Request

GET `/papi/v1/margin/maxWithdraw`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "amount": "60"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Margin repay Record(USER\_DATA)

## API Description

Query margin repay record.

## HTTP Request

GET `/papi/v1/margin/repayLoan`

## Request Weight

**10**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| txId | LONG | NO | the tranId in `POST/papi/v1/repayLoan` |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| archived | STRING | NO | Default: `false`. Set to `true` for archived data from 6 months ago |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - txId or startTime must be sent. txId takes precedence.
> - Response in descending order
> - The max interval between `startTime` and `endTime` is 30 days.
> - If `startTime` and `endTime` not sent, return records of the last 7 days by default
> - Set `archived` to `true` to query data from 6 months ago

## Response Example

```javascript
{
     "rows": [
         {
                "amount": "14.00000000",   //Total amount repaid
                "asset": "BNB",
                "interest": "0.01866667",    //Interest repaid
                "principal": "13.98133333",   //Principal repaid
                "status": "CONFIRMED",   //one of PENDING (pending execution), CONFIRMED (successfully execution), FAILED (execution failed, nothing happened to your account)
                "timestamp": 1563438204000,
                "txId": 2970933056
         }
     ],
     "total": 1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Portfolio Margin Negative Balance Interest History(USER\_DATA)

## API Description

Query interest history of negative balance for portfolio margin.

## HTTP Request

`GET /papi/v1/portfolio/interest-history`

## Request Weight

**50**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| size | LONG | NO | Default:10 Max:100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Response in descending order
> - The max interval between startTime and endTime is 30 days. It is a MUST to ensure data correctness.
> - If `startTime` and `endTime` not sent, return records of the last 7 days by default
> - If `startTime` is sent and `endTime` is not sent, the records from `startTime` to the present will be returned; if `startTime` is more than 30 days ago, the records of the past 30 days will be returned.
> - If `startTime` is not sent and `endTime` is sent, the records of the 7 days before `endTime` is returned.

## Response Example

```javascript
[
    {
        "asset": "USDT",
        "interest": "24.4440",               //interest amount
        "interestAccuredTime": 1670227200000,
        "interestRate": "0.0001164",         //daily interest rate
        "principal": "210000"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query UM Position Information(USER\_DATA)

## API Description

Get current UM position information.

## HTTP Request

GET `/papi/v1/um/positionRisk`

## Request Weight

**5**

**Parameters:**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Note**

> - Please use with user data stream `ACCOUNT_UPDATE` to meet your timeliness and accuracy needs.
> - for One-way Mode user, the response will only show the "BOTH" positions
> - for Hedge Mode user, the response will show "LONG", and "SHORT" positions.

## Response Example

> - For One-way position mode:

```javascript
 [
    {
        "entryPrice": "0.00000",
        "leverage": "10",
        "markPrice": "6679.50671178",
        "maxNotionalValue": "20000000",
        "positionAmt": "0.000",
        "notional": "0",
        "symbol": "BTCUSDT",
        "unRealizedProfit": "0.00000000",
        "liquidationPrice": "6170.20509059",
        "positionSide": "BOTH",
        "updateTime": 1625474304765
    }
]
```

> Or For Hedge position mode(only return with position):

```javascript
[
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.001",
        "entryPrice": "22185.2",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "-1.06214947",
        "liquidationPrice": "6170.20509059",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "positionSide": "LONG",
        "notional": "21.12305052",
        "updateTime": 1655217461579
    },
    {
        "symbol": "BTCUSDT",
        "positionAmt": "0.000",
        "entryPrice": "0.0",
        "markPrice": "21123.05052574",
        "unRealizedProfit": "0.00000000",
        "liquidationPrice": "6170.20509059",
        "leverage": "4",
        "maxNotionalValue": "100000000",
        "positionSide": "SHORT",
        "notional": "0",
        "updateTime": 0
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query User Negative Balance Auto Exchange Record (USER\_DATA)

## API Description

Query user negative balance auto exchange record

## HTTP Request

GET `/papi/v1/portfolio/negative-balance-exchange-record`

## Request Weight

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | YES |  |
| endTime | LONG | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

**Note**

> - Response in descending order
> - The max interval between `startTime` and `endTime` is 3 months.

## Response Example

```javascript
{
  "total": 2,
  "rows": [
    {
      "startTime": 1736263046841,
      "endTime": 1736263248179,
      "details": [
        {
          "asset": "ETH",
          "negativeBalance": 18,  //negative balance amount
          "negativeMaxThreshold": 5  //the max negative balance threshold
        }
      ]
    },
    {
      "startTime": 1736184913252,
      "endTime": 1736184965474,
      "details": [
        {
          "asset": "BNB",
          "negativeBalance": 1.10264488,
          "negativeMaxThreshold": 0
        }
      ]
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query User Rate Limit (USER\_DATA)

## API Description

Query User Rate Limit

## HTTP Request

GET `/papi/v1/rateLimit/order`

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
        "interval": "MINUTE",
        "intervalNum": 1,
        "limit": 1200
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Repay futures Negative Balance(USER\_DATA)

## API Description

Repay futures Negative Balance

## HTTP Request

POST `/papi/v1/repay-futures-negative-balance`

## Request Weight(IP)

**750**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    "msg": "success"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# UM Notional and Leverage Brackets (USER\_DATA)

## API Description

Query UM notional and leverage brackets

## HTTP Request

`GET /papi/v1/um/leverageBracket`

## Request Weight

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
        "symbol": "ETHUSDT",
        "notionalCoef": "4.0",
        "brackets": [
            {
                "bracket": 1,   // Notional bracket
                "initialLeverage": 75,  // Max initial leverage for this bracket
                "notionalCap": 10000,  // Cap notional of this bracket
                "notionalFloor": 0,  // Notional threshold of this bracket
                "maintMarginRatio": 0.0065, // Maintenance ratio for this bracket
                "cum":0 // Auxiliary number for quick calculation
            },
        ]
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Public API Definitions

## Terminology

- `baseasseet` refers to the asset that is the `quantity` of a symbol.
- `quoteAsset` refers to the asset that is the `price` of a symbol.
- `Margin` refers to `Cross Margin`
- `UM` refers to `USD-M Futures`
- `CM` refers to `Coin-M Futures`

## ENUM definitions

**Order side (side)**

- BUY
- SELL

**Position side for Futures (positionSide)**

- BOTH
- LONG
- SHORT

**Time in force (timeInForce)**

- GTC - Good Till Cancel
- IOC - Immediate or Cancel
- FOK - Fill or Kill
- GTX - Good Till Crossing (Post Only)

**Stop-Limit Time in force (stopLimitTimeInForce)**

- GTC - Good Till Cancel
- IOC - Immediate or Cancel
- FOK - Fill or Kill

**Side Effect Type (sideEffectType)**

- NO\_SIDE\_EFFECT
- MARGIN\_BUY
- AUTO\_REPAY

**Price Match (priceMatch)**

- NONE: no price match
- OPPONENT: counterparty best price
- OPPONENT\_5: counterparty 5th best price
- OPPONENT\_10: counterparty 10th best price
- OPPONENT\_20: counterparty 20th best price
- QUEUE: the best price on the same side of the order book
- QUEUE\_5: the 5th best price on the same side of the order book
- QUEUE\_10: the 10th best price on the same side of the order book
- QUEUE\_20: the 20th best price on the same side of the order book

**Self-Trade Prevention mode (selfTradePreventionMode)**

- NONE: No Self-Trade Prevention
- EXPIRE\_TAKER: expire taker order when STP trigger
- EXPIRE\_BOTH: expire taker and maker order when STP trigger
- EXPIRE\_MAKER: expire maker order when STP trigger

**Response Type (newOrderRespType)**

- ACK
- RESULT

**Order types (type)**

- LIMIT
- MARKET

**Conditional Order types (strategyType)**

- STOP
- STOP\_MARKET
- LIMIT\_MAKER
- TAKE\_PROFIT
- TAKE\_PROFIT\_MARKET
- TRAILING\_STOP\_MARKET

**Working Type for Futures Conditional Orders (workingType)**

- MARK\_PRICE

**Order status (status)**

- NEW
- CANCELED
- REJECTED
- PARTIALLY\_FILLED
- FILLED
- EXPIRED

**Conditional Order status (strategyStatus)**

- NEW
- CANCELED
- TRIGGERED - conditional order is triggered
- FINISHED - triggered order is filled
- EXPIRED

**Futures Contract type (contractType):**

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

**Rate limiters (rateLimitType)**

- REQUEST\_WEIGHT
- ORDERS

> **REQUEST\_WEIGHT**

```shell
  {
    "rateLimitType": "REQUEST_WEIGHT",
    "interval": "MINUTE",
    "intervalNum": 1,
    "limit": 2400
  }
```

> **ORDERS**

```shell
  {
    "rateLimitType": "ORDERS",
    "interval": "MINUTE",
    "intervalNum": 1,
    "limit": 1200
   }
```

**Rate limit intervals (interval)**

# Filters

Filters define trading rules on a symbol or an exchange.

## Symbol filters

### PRICE\_FILTER

The `PRICE_FILTER` defines the `price` rules for a symbol. There are 3 parts:

- `minPrice` defines the minimum `price`/`stopPrice` allowed; disabled on `minPrice` == 0.
- `maxPrice` defines the maximum `price`/`stopPrice` allowed; disabled on `maxPrice` == 0.
- `tickSize` defines the intervals that a `price`/`stopPrice` can be increased/decreased by; disabled on `tickSize` == 0.

Any of the above variables can be set to 0, which disables that rule in the `price filter`. In order to pass the `price filter`, the following must be true for `price`/`stopPrice` of the enabled rules:

- sell order `price` >= `minPrice`
- buy order `price` <= `maxPrice`
- (`price`-`minPrice`) % `tickSize` == 0

> **ExchangeInfo format:**

```javascript
{
    "filterType": "PRICE_FILTER",
    "minPrice": "0.00000100",
    "maxPrice": "100000.00000000",
    "tickSize": "0.00000100"
}
```

### LOT\_SIZE

The `LOT_SIZE` filter defines the `quantity` (aka "lots" in auction terms) rules for a symbol. There are 3 parts:

- `minQty` defines the minimum `quantity` allowed.
- `maxQty` defines the maximum `quantity` allowed.
- `stepSize` defines the intervals that a `quantity` can be increased/decreased by.

In order to pass the `lot size`, the following must be true for `quantity`:

- `quantity` >= `minQty`
- `quantity` <= `maxQty`
- (`quantity`-`minQty`) % `stepSize` == 0

> **/exchangeInfo format:**

```javascript
{
    "filterType": "LOT_SIZE",
    "minQty": "0.00100000",
    "maxQty": "100000.00000000",
    "stepSize": "0.00100000"
}
```

### PERCENT\_PRICE

The `PERCENT_PRICE` filter defines valid range for a price based on the mark price in Futures and on the average of the previous trades in Cross Margin. For Cross Margin `avgPriceMins` is the number of minutes the average price is calculated over. 0 means the last price is used.

In order to pass the `percent price`, the following must be true for `price`:

- Futures
BUY: `price` <= `markPrice` \_ `multiplierUp`
SELL: `price` >= `markPrice` \_ `multiplierDown`
- Cross Margin
BUY: `price` <= `weightedAveragePrice` \_ `multiplierUp`
SELL: `price` >= `weightedAveragePrice` \_ `multiplierDown`

### MIN\_NOTIONAL

The `MIN_NOTIONAL` filter defines the minimum notional value allowed for an order on a symbol. An order's notional value is the `price` \\* `quantity`. Since `MARKET` orders have no price, the `mark price` is used in Futures and the average price is used over the last `avgPriceMins` for Cross Margin. `avgPriceMins` is the number of minutes the average price is calculated over. 0 means the last price is used.

### MARKET\_LOT\_SIZE

The `MARKET_LOT_SIZE` filter defines the `quantity` (aka "lots" in auction terms) rules for `MARKET` orders on a symbol. There are 3 parts:

- `minQty` defines the minimum `quantity` allowed.
- `maxQty` defines the maximum `quantity` allowed.
- `stepSize` defines the intervals that a `quantity` can be increased/decreased by.

In order to pass the `market lot size`, the following must be true for `quantity`:

- `quantity` >= `minQty`
- `quantity` <= `maxQty`
- (`quantity`-`minQty`) % `stepSize` == 0

> **/exchangeInfo format:**

```javascript
{
  "filterType": "MARKET_LOT_SIZE",
  "minQty": "0.00100000",
  "maxQty": "100000.00000000",
  "stepSize": "0.00100000"
}
```

### MAX\_NUM\_ORDERS

The `MAX_NUM_ORDERS` filter defines the maximum number of orders an account is allowed to have open on a symbol.
Note that both "algo" orders and normal orders are counted for this filter.

> **/exchangeInfo format:**

```javascript
{
  "filterType": "MAX_NUM_ORDERS",
  "limit": 200
}
```

### MAX\_NUM\_ALGO\_ORDERS

The `MAX_NUM_ALGO_ORDERS` filter defines the maximum number of all kinds of algo orders an account is allowed to have open on a symbol.
The algo orders include `STOP`, `STOP_MARKET`, `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`, and `TRAILING_STOP_MARKET` orders.

> **/exchangeInfo format:**

```javascript
{
  "filterType": "MAX_NUM_ALGO_ORDERS",
  "limit": 100
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Test Connectivity

## API Description

Test connectivity to the Rest API.

## HTTP Request

GET `/papi/v1/ping`

## Request Weight

**1**

## Request Parameters

NONE

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Test Connectivity

## API Description

Test connectivity to the Rest API.

## HTTP Request

GET `/papi/v1/ping`

## Request Weight

**1**

## Request Parameters

NONE

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New UM Order (TRADE)

## API Description

Place new UM order

## HTTP Request

POST `/papi/v1/um/order`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES | `LIMIT`, `MARKET` |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode . |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,32}$` |
| newOrderRespType | ENUM | NO | `ACK`, `RESULT`, default `ACK` |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on type:

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
    "symbol": "BTCUSDT",
    "timeInForce": "GTD",
    "type": "MARKET",
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 1693207680000,      //order pre-set auot cancel time for TIF GTD order
    "updateTime": 1566818724722,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# CM Account Trade List(USER\_DATA)

## API Description

Get trades for a specific account and CM symbol.

## HTTP Request

GET `/papi/v1/cm/userTrades`

## Request Weight

**20** with symbol, **40** with pair

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| pair | STRING | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | Trade id to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 50; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `symbol` or `pair` must be sent
> - `symbol` and `pair` cannot be sent together
> - `pair` and `fromId` cannot be sent together
> - `OrderId` can only be sent together with symbol
> - If a `pair` is sent, tickers for all symbols of the `pair` will be returned
> - The parameter `fromId` cannot be sent with `startTime` or `endTime`
> - If `startTime` and `endTime` are both not sent, then the last '24 hours' data will be returned.
> - The time between `startTime` and `endTime` cannot be longer than 24 hours.

## Response Example

```javascript
[
    {
        'symbol': 'BTCUSD_200626',
        'id': 6,
        'orderId': 28,
        'pair': 'BTCUSD',
        'side': 'SELL',
        'price': '8800',
        'qty': '1',
        'realizedPnl': '0',
        'marginAsset': 'BTC',
        'baseQty': '0.01136364',
        'commission': '0.00000454',
        'commissionAsset': 'BTC',
        'time': 1590743483586,
        'positionSide': 'BOTH',
        'buyer': false,
        'maker': false
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# CM Position ADL Quantile Estimation(USER\_DATA)

## API Description

Query CM Position ADL Quantile Estimation

> - Values update every 30s.
> - Values 0, 1, 2, 3, 4 shows the queue position and possibility of ADL from low to high.
> - For positions of the symbol are in One-way Mode or isolated margined in Hedge Mode, "LONG", "SHORT", and "BOTH" will be returned to show the positions' adl quantiles of different position sides.
> - If the positions of the symbol are crossed margined in Hedge Mode:
>   - "HEDGE" as a sign will be returned instead of "BOTH";
>   - A same value caculated on unrealized pnls on long and short sides' positions will be shown for "LONG" and "SHORT" when there are positions in both of long and short sides.

## HTTP Request

GET `/papi/v1/cm/adlQuantile`

## Request Weight

**5**

**Parameters:**

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
    {
        "symbol": "BTCUSD_200925",
        "adlQuantile":
            {
                // if the positions of the symbol are crossed margined in Hedge Mode, "LONG" and "SHORT" will be returned a same quantile value, and "HEDGE" will be returned instead of "BOTH".
                "LONG": 3,
                "SHORT": 3,
                "HEDGE": 0   // only a sign, ignore the value
            }
        },
    {
        "symbol": "BTCUSD_201225",
        "adlQuantile":
            {
                // for positions of the symbol are in One-way Mode
                "LONG": 1,  // adl quantile for "LONG" position in hedge mode
                "SHORT": 2,     // adl qauntile for "SHORT" position in hedge mode
                "BOTH": 0       // adl qunatile for position in one-way mode
            }
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel All CM Open Conditional Orders(TRADE)

## API Description

Cancel All CM Open Conditional Orders

## HTTP Request

DELETE `/papi/v1/cm/conditional/allOpenOrders`

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
    "code": "200",
    "msg": "The operation of cancel all conditional open order is done."
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel All CM Open Orders(TRADE)

## API Description

Cancel all active LIMIT orders on specific symbol

## HTTP Request

DELETE `/papi/v1/cm/allOpenOrders`

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

# Cancel All UM Open Conditional Orders (TRADE)

## API Description

Cancel All UM Open Conditional Orders

## HTTP Request

`DELETE /papi/v1/um/conditional/allOpenOrders`

## Request Weight(Order)

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
    "code": "200",
    "msg": "The operation of cancel all conditional open order is done."
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel All UM Open Orders(TRADE)

## API Description

Cancel all active LIMIT orders on specific symbol

## HTTP Request

DELETE `/papi/v1/um/allOpenOrders`

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

# Cancel CM Conditional Order(TRADE)

## API Description

Cancel CM Conditional Order

## HTTP Request

DELETE `/papi/v1/cm/conditional/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `strategyId` or `newClientStrategyId` must be sent.

## Response Example

```javascript
{
    "newClientStrategyId": "myOrder1",
    "strategyId":123445,
    "strategyStatus":"CANCELED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "11",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD",
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",                 // callback rate, only return with TRAILING_STOP_MARKET order
    "bookTime": 1566818724710,
    "updateTime": 1566818724722,
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel CM Order(TRADE)

## API Description

Cancel an active LIMIT order

## HTTP Request

DELETE `/papi/v1/cm/order`

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
    "avgPrice": "0.0",
    "clientOrderId": "myOrder1",
    "cumQty": "0",
    "cumBase": "0",
    "executedQty": "0",
    "orderId": 283194212,
    "origQty": "2",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "CANCELED",
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1571110484038,
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel Margin Account All Open Orders on a Symbol(TRADE)

## API Description

Cancel Margin Account All Open Orders on a Symbol

## HTTP Request

DELETE `/papi/v1/margin/allOpenOrders`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
[
  {
    "symbol": "BTCUSDT",
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
    "side": "BUY"
  },
  {
    "orderListId": 1929,
    "contingencyType": "OCO",
    "listStatusType": "ALL_DONE",
    "listOrderStatus": "ALL_DONE",
    "listClientOrderId": "2inzWQdDvZLHbbAmAozX2N",
    "transactionTime": 1585230948299,
    "symbol": "BTCUSDT",
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

Derivatives Trading

# Cancel Margin Account OCO Orders(TRADE)

## API Description

Cancel Margin Account OCO Orders

## HTTP Request

DELETE `/papi/v1/margin/orderList`

## Request Weight

**2**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderListId | LONG | NO | Either `orderListId` or `listClientOrderId` must be provided |
| listClientOrderId | STRING | NO | Either `orderListId` or `listClientOrderId` must be provided |
| newClientOrderId | STRING | NO | Used to uniquely identify this cancel. Automatically generated by default |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - Additional notes: Canceling an individual leg will cancel the entire OCO

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
      "stopPrice": "1.00000000"
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
      "side": "SELL"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel Margin Account Order(TRADE)

## API Description

Cancel Margin Account Order

## HTTP Request

DELETE `/papi/v1/margin/order`

## Request Weight

**2**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| newClientOrderId | STRING | NO | Used to uniquely identify this cancel. Automatically generated by default. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent.

## Response Example

```javascript
{
  "symbol": "LTCBTC",
  "orderId": 28,
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

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel UM Conditional Order(TRADE)

## API Description

Cancel UM Conditional Order

## HTTP Request

DELETE `/papi/v1/um/conditional/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `strategyId` or `newClientStrategyId` must be sent.

## Response Example

```javascript
{
    "newClientStrategyId": "myOrder1",
    "strategyId":123445,
    "strategyStatus":"CANCELED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "11",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",                 // callback rate, only return with TRAILING_STOP_MARKET order
    "bookTime": 1566818724710,
    "updateTime": 1566818724722,
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false,
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Cancel UM Order(TRADE)

## API Description

Cancel an active UM LIMIT order

## HTTP Request

DELETE `/papi/v1/um/order`

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
    "avgPrice": "0.00000",
    "clientOrderId": "myOrder1",
    "cumQty": "0",
    "cumQuote": "0",
    "executedQty": "0",
    "orderId": 4611875134427365377,
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "CANCELED",
    "symbol": "BTCUSDT",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1571110484038,
    "selfTradePreventionMode": "NONE",
    "goodTillDate": 0,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Get UM Futures BNB Burn Status (USER\_DATA)

## API Description

Get user's BNB Fee Discount for UM Futures (Fee Discount On or Fee Discount Off )

## HTTP Request

GET `/papi/v1/um/feeBurn`

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

# Margin Account Borrow(MARGIN)

## API Description

Apply for a margin loan.

## HTTP Request

POST `/papi/v1/marginLoan`

## Request Weight(IP)

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| amount | DECIMAL | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    //transaction id
    "tranId": 100000001
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Margin Account New OCO(TRADE)

## API Description

Send in a new OCO for a margin account

## HTTP Request

POST `/papi/v1/margin/order/oco`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| listClientOrderId | STRING | NO | A unique Id for the entire orderList |
| side | ENUM | YES |  |
| quantity | DECIMAL | YES |  |
| limitClientOrderId | STRING | NO | A unique Id for the limit order |
| price | DECIMAL | YES |  |
| limitIcebergQty | DECIMAL | NO |  |
| stopClientOrderId | STRING | NO | A unique Id for the stop loss/stop loss limit leg |
| stopPrice | DECIMAL | YES |  |
| stopLimitPrice | DECIMAL | NO | If provided, stopLimitTimeInForce is required. |
| stopIcebergQty | DECIMAL | NO |  |
| stopLimitTimeInForce | ENUM | NO | Valid values are `GTC/FOK/IOC` |
| newOrderRespType | ENUM | NO | Set the response JSON. |
| sideEffectType | ENUM | NO | NO\_SIDE\_EFFECT, MARGIN\_BUY, AUTO\_REPAY; default NO\_SIDE\_EFFECT. |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

Other Info:

> - Price Restrictions:
>   - `SELL`: Limit Price > Last Price > Stop Price
>   - `BUY`: Limit Price < Last Price < Stop Price
> - Quantity Restrictions:
>   - Both legs must have the same quantity
>   - `ICEBERG` quantities however do not have to be the same.
> - Order Rate Limit
>   - `OCO` counts as 2 orders against the order rate limit.

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
      "stopPrice": "0.960664"
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
      "side": "BUY"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Margin Account Repay(MARGIN)

## API Description

Repay for a margin loan.

## HTTP Request

POST `/papi/v1/repayLoan`

## Request Weight

**100**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| amount | DECIMAL | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
    //transaction id
    "tranId": 100000001
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Margin Account Repay Debt(TRADE)

## API Description

Repay debt for a margin loan.

## HTTP Request

POST `/papi/v1/margin/repay-debt`

## Request Weight(Order)

**3000**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| asset | STRING | YES |  |
| amount | STRING | NO |  |
| specifyRepayAssets | STRING | NO | Specific asset list to repay debt; Can be added in batch, separated by commas |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - The repay asset amount cannot exceed 50000 USD equivalent value for a single request.
> - If `amount` is not sent, all the asset loan will be repaid if having enough specific repay assets.
> - If `amount` is sent, only the certain amount of the asset loan will be repaid if having enough specific repay assets.
> - The system will use the same asset to repay the loan first (if have) no matter whether put the asset in `specifyRepayAssets`

## Response Example

```javascript
{
    "amount": "0.10000000",
	"asset": "BNB",
    "specifyRepayAssets": [
    "USDT",
    "BTC"
	],
    "updateTime": 1636371437000
	"success": true
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Margin Account Trade List (USER\_DATA)

## API Description

Margin Account Trade List

## HTTP Request

GET `/papi/v1/margin/myTrades`

## Weight

**5**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | TradeId to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

**Notes:**

- If `fromId` is set, it will get trades >= that `fromId`. Otherwise most recent trades are returned.
- Less than 24 hours between `startTime` and `endTime`.

## Response:

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
        "time": 1561973357171
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Modify CM Order(TRADE)

## API Description

Order modify function, currently only LIMIT order modification is supported, modified orders will be reordered in the match queue

## HTTP Request

PUT `/papi/v1/cm/order`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| symbol | STRING | YES |  |
| side | ENUM | YES | SELL, BUY |
| quantity | DECIMAL | YES | Order quantity |
| price | DECIMAL | YES |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.
> - Both `quantity` and `price` must be sent
> - When the new `quantity` or `price` doesn't satisfy PRICE\_FILTER / PERCENT\_FILTER / LOT\_SIZE, amendment will be rejected and the order will stay as it is.
> - However the order will be cancelled by the amendment in the following situations:
>   - when the order is in partially filled status and the new `quantity` <= `executedQty`
>   - When the order is `GTX` and the new price will cause it to be executed immediately

## Response Example

```javascript
{
    "orderId": 20072994037,
    "symbol": "BTCUSD_PERP",
    "pair": "BTCUSD",
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
    "side": "BUY",
    "positionSide": "LONG",
    "origType": "LIMIT",
    "updateTime": 1629182711600
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Modify UM Order(TRADE)

## API Description

Order modify function, currently only LIMIT order modification is supported, modified orders will be reordered in the match queue

## HTTP Request

PUT `/papi/v1/um/order`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| symbol | STRING | YES |  |
| side | ENUM | YES | SELL, BUY |
| quantity | DECIMAL | YES | Order quantity |
| price | DECIMAL | YES |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either orderId or origClientOrderId must be sent, and the orderId will prevail if both are sent.
> - Both quantity and price must be sent
> - When the new quantity or price doesn't satisfy PRICE\_FILTER / PERCENT\_FILTER / LOT\_SIZE, amendment will be rejected and the order will stay as it is.
> - However the order will be cancelled by the amendment in the following situations:
>   - when the order is in partially filled status and the new quantity <= executedQty
>   - When the order is GTX and the new price will cause it to be executed immediately

## Response Example

```javascript
{
    "orderId": 20072994037,
    "symbol": "BTCUSDT",
    "status": "NEW",
    "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
    "price": "30005",
    "avgPrice": "0.0",
    "origQty": "1",
    "executedQty": "0",
    "cumQty": "0",
    "cumQuote": "0",
    "timeInForce": "GTC",
    "type": "LIMIT",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "LONG",
    "origType": "LIMIT",
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0      //order pre-set auot cancel time for TIF GTD order
    "updateTime": 1629182711600,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New CM Conditional Order(TRADE)

## API Description

New CM Conditional Order

## HTTP Request

POST `/papi/v1/cm/conditional/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| strategyType | ENUM | YES | "STOP", "STOP\_MARKET", "TAKE\_PROFIT", "TAKE\_PROFIT\_MARKET", and "TRAILING\_STOP\_MARKET" |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode |
| price | DECIMAL | NO |  |
| workingType | ENUM | NO | stopPrice triggered by: "MARK\_PRICE", "CONTRACT\_PRICE". Default "CONTRACT\_PRICE" |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP`/`STOP_MARKET` or `TAKE_PROFIT`/`TAKE_PROFIT_MARKET` orders |
| newClientStrategyId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,36}$` |
| stopPrice | DECIMAL | NO | Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| activationPrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the mark price |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 5 where 1 for 1% |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on type:

| Type | Additional mandatory parameters |
| --- | --- |
| `STOP/TAKE_PROFIT` | `quantity`, `price`, `stopPrice` |
| `STOP_MARKET/TAKE_PROFIT_MARKET` | `stopPrice` |
| `TRAILING_STOP_MARKET` | `callbackRate` |

- Order with type `STOP/TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).

- Condition orders will be triggered when:
  - `STOP`, `STOP_MARKET`:

    - BUY: "MARK\_PRICE" >= `stopPrice`
    - SELL: "MARK\_PRICE" <= `stopPrice`
  - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:

    - BUY: "MARK\_PRICE" <= `stopPrice`
    - SELL: "MARK\_PRICE" >= `stopPrice`
  - `TRAILING_STOP_MARKET`:

    - BUY: the lowest mark price after order placed `<=`activationPrice`, and the latest mark price >`= the lowest mark price \* (1 + `callbackRate`)
    - SELL: the highest mark price after order placed >= `activationPrice`, and the latest mark price <= the highest mark price \* (1 - `callbackRate`)
- For `TRAILING_STOP_MARKET`, if you got such error code. `{"code": -2021, "msg": "Order would immediately trigger."}` means that the parameters you send do not meet the following requirements:
  - BUY: `activationPrice` should be smaller than latest mark price.
  - SELL: `activationPrice` should be larger than latest mark price.
- Condition orders will be triggered when:
  - If parameter`priceProtect`is sent as true:

    - when price reaches the `stopPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
    - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
  - `STOP`, `STOP_MARKET`:

    - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
    - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
  - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:

    - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
    - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`

## Response Example

```javascript
{
    "newClientStrategyId": "testOrder",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "10",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",        // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "timeInForce": "GTC",
    "activatePrice": "9020",    // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",         // callback rate, only return with TRAILING_STOP_MARKET order
    "bookTime": 1566818724710,  // order place time
    "updateTime": 1566818724722
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New CM Order(TRADE)

## API Description

Place new CM order

## HTTP Request

POST `/papi/v1/cm/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES | "LIMIT", "MARKET" |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode. |
| price | DECIMAL | NO |  |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,32}$` |
| newOrderRespType | ENUM | NO | "ACK", "RESULT", default "ACK" |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on `type`:

| Type | Additional mandatory parameters |
| --- | --- |
| `LIMIT` | `timeInForce`, `quantity`, `price` |
| `MARKET` | `quantity` |

- If `newOrderRespType` is sent as `RESULT` :

  - `MARKET` order: the final FILLED result of the order will be return directly.
  - `LIMIT` order with special `timeInForce`: the final status result of the order(FILLED or EXPIRED) will be returned directly.

## Response Example

```javascript
{
    "clientOrderId": "testOrder",
    "cumQty": "0",
    "cumBase": "0",
    "executedQty": "0",
    "orderId": 22542179,
    "avgPrice": "0.0",
    "origQty": "10",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "timeInForce": "GTC",
    "type": "MARKET",
    "updateTime": 1566818724722
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New Margin Order(TRADE)

## API Description

New Margin Order

## HTTP Request

POST `/papi/v1/margin/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES | BUY ; SELL |
| type | ENUM | YES |  |
| quantity | DECIMAL | NO |  |
| quoteOrderQty | DECIMAL | NO |  |
| price | DECIMAL | NO |  |
| stopPrice | DECIMAL | NO | Used with `STOP_LOSS`, `STOP_LOSS_LIMIT`, `TAKE_PROFIT`, and `TAKE_PROFIT_LIMIT` orders. |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. |
| newOrderRespType | ENUM | NO | Set the response JSON. ACK, RESULT, or FULL; MARKET and LIMIT order types default to FULL, all other orders default to ACK. |
| icebergQty | DECIMAL | NO | Used with `LIMIT`, `STOP_LOSS_LIMIT`, and `TAKE_PROFIT_LIMIT` to create an iceberg order |
| sideEffectType | ENUM | NO | `NO_SIDE_EFFECT`, `MARGIN_BUY`, `AUTO_REPAY`,`AUTO_BORROW_REPAY`; default `NO_SIDE_EFFECT`. |
| timeInForce | ENUM | NO | GTC,IOC,FOK |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers |
| autoRepayAtCancel | BOOLEAN | NO | Only when MARGIN\_BUY or AUTO\_BORROW\_REPAY order takes effect, true means that the debt generated by the order needs to be repay after the order is cancelled. The default is true |
| recvWindow | LONG | NO | The value cannot be greater than `60000` |
| timestamp | LONG | YES |  |

## Response Example

```javascript
{
  "symbol": "BTCUSDT",
  "orderId": 28,
  "clientOrderId": "6gCrw2kRUAF9CvJDGP16IP",
  "transactTime": 1507725176595,
  "price": "1.00000000",
  "origQty": "10.00000000",
  "executedQty": "10.00000000",
  "cummulativeQuoteQty": "10.00000000",
  "status": "FILLED",
  "timeInForce": "GTC",
  "type": "MARKET",
  "side": "SELL",
  "marginBuyBorrowAmount": "5",       // will not return if no margin trade happens
  "marginBuyBorrowAsset": "BTC",    // will not return if no margin trade happens
  "fills": [
    {
      "price": "4000.00000000",
      "qty": "1.00000000",
      "commission": "4.00000000",
      "commissionAsset": "USDT"
    },
    {
      "price": "3999.00000000",
      "qty": "5.00000000",
      "commission": "19.99500000",
      "commissionAsset": "USDT"
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New UM Conditional Order (TRADE)

## API Description

Place new UM conditional order

## HTTP Request

POST `/papi/v1/um/conditional/order`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| strategyType | ENUM | YES | "STOP", "STOP\_MARKET", "TAKE\_PROFIT", "TAKE\_PROFIT\_MARKET", and "TRAILING\_STOP\_MARKET" |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode ; cannot be sent with `closePosition`=`true` |
| price | DECIMAL | NO |  |
| workingType | ENUM | NO | stopPrice triggered by: "MARK\_PRICE", "CONTRACT\_PRICE". Default "CONTRACT\_PRICE" |
| priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders |
| newClientStrategyId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,32}$` |
| stopPrice | DECIMAL | NO | Used with `STOP/STOP_MARKET` or `TAKE_PROFIT/TAKE_PROFIT_MARKET` orders. |
| activationPrice | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, default as the mark price |
| callbackRate | DECIMAL | NO | Used with `TRAILING_STOP_MARKET` orders, min 0.1, max 5 where 1 for 1% |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on type:

| Type | Additional mandatory parameters |
| --- | --- |
| `STOP/TAKE_PROFIT` | `quantity`, `price`, `stopPrice` |
| `STOP_MARKET/TAKE_PROFIT_MARKET` | `stopPrice` |
| `TRAILING_STOP_MARKET` | `callbackRate` |

- Order with type `STOP/TAKE_PROFIT`, parameter `timeInForce` can be sent ( default `GTC`).

- Condition orders will be triggered when:
  - `STOP`, `STOP_MARKET`:

    - BUY: "MARK\_PRICE" >= `stopPrice`
    - SELL: "MARK\_PRICE" <= `stopPrice`
  - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:

    - BUY: "MARK\_PRICE" <= `stopPrice`
    - SELL: "MARK\_PRICE" >= `stopPrice`
  - `TRAILING_STOP_MARKET`:

    - BUY: the lowest mark price after order placed `<=`activationPrice`, and the latest mark price >`= the lowest mark price \* (1 + `callbackRate`)
    - SELL: the highest mark price after order placed >= `activationPrice`, and the latest mark price <= the highest mark price \* (1 - `callbackRate`)
- For `TRAILING_STOP_MARKET`, if you got such error code. `{"code": -2021, "msg": "Order would immediately trigger."}` means that the parameters you send do not meet the following requirements:
  - BUY: `activationPrice` should be smaller than latest mark price.
  - SELL: `activationPrice` should be larger than latest mark price.
- Condition orders will be triggered when:
  - If parameter`priceProtect`is sent as true:

    - when price reaches the `stopPrice` ，the difference rate between "MARK\_PRICE" and "CONTRACT\_PRICE" cannot be larger than the "triggerProtect" of the symbol
    - "triggerProtect" of a symbol can be got from `GET /fapi/v1/exchangeInfo`
  - `STOP`, `STOP_MARKET`:

    - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
    - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
  - `TAKE_PROFIT`, `TAKE_PROFIT_MARKET`:

    - BUY: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") <= `stopPrice`
    - SELL: latest price ("MARK\_PRICE" or "CONTRACT\_PRICE") >= `stopPrice`
- `selfTradePreventionMode` is only effective when `timeInForce` set to `IOC` or `GTC` or `GTD`.

- In extreme market conditions, timeInForce `GTD` order auto cancel time might be delayed comparing to `goodTillDate`

## Response Example

```javascript
{
    "newClientStrategyId": "testOrder",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "10",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",        // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "timeInForce": "GTD",
    "activatePrice": "9020",    // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",         // callback rate, only return with TRAILING_STOP_MARKET order
    "bookTime": 1566818724710,  // order place time
    "updateTime": 1566818724722
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false,
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 1693207680000,      //order pre-set auot cancel time for TIF GTD order
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# New UM Order (TRADE)

## API Description

Place new UM order

## HTTP Request

POST `/papi/v1/um/order`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| side | ENUM | YES |  |
| positionSide | ENUM | NO | Default `BOTH` for One-way Mode ; `LONG` or `SHORT` for Hedge Mode. It must be sent in Hedge Mode. |
| type | ENUM | YES | `LIMIT`, `MARKET` |
| timeInForce | ENUM | NO |  |
| quantity | DECIMAL | NO |  |
| reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode . |
| price | DECIMAL | NO |  |
| newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: `^[\.A-Z\:/a-z0-9_-]{1,32}$` |
| newOrderRespType | ENUM | NO | `ACK`, `RESULT`, default `ACK` |
| priceMatch | ENUM | NO | only avaliable for `LIMIT`/`STOP`/`TAKE_PROFIT` order; can be set to `OPPONENT`/ `OPPONENT_5`/ `OPPONENT_10`/ `OPPONENT_20`: /`QUEUE`/ `QUEUE_5`/ `QUEUE_10`/ `QUEUE_20`; Can't be passed together with `price` |
| selfTradePreventionMode | ENUM | NO | `NONE`:No STP / `EXPIRE_TAKER`:expire taker order when STP triggers/ `EXPIRE_MAKER`:expire taker order when STP triggers/ `EXPIRE_BOTH`:expire both orders when STP triggers |
| goodTillDate | LONG | NO | order cancel time for timeInForce `GTD`, mandatory when `timeInforce` set to `GTD`; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Additional mandatory parameters based on type:

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
    "symbol": "BTCUSDT",
    "timeInForce": "GTD",
    "type": "MARKET",
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 1693207680000,      //order pre-set auot cancel time for TIF GTD order
    "updateTime": 1566818724722,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All CM Conditional Orders(USER\_DATA)

## API Description

Query All CM Conditional Orders

## HTTP Request

GET `/papi/v1/cm/conditional/allOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| strategyId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Notes:**

> - These orders will not be found:
>   - order strategyStatus is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 7 days < current time
> - The query time period must be less than 7 days( default as the recent 7 days).

## Response Example

```javascript
[
  {
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"TRIGGERED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD",
    "orderId": 12123343534,    //Normal orderID after trigger if appliable, only have when the strategy is triggered
    "status": "NEW",     //Normal order status after trigger if appliable, only have when the strategy is triggered
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "triggerTime": 1566818724750,
    "timeInForce": "GTC",
    "type": "MARKET",    //Normal order type after trigger if appliable
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3"                // callback rate, only return with TRAILING_STOP_MARKET order
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All CM Orders (USER\_DATA)

## API Description

Get all account CM orders; active, canceled, or filled.

## HTTP Request

GET `/papi/v1/cm/allOrders`

## Request Weight

**20** with symbol, **40** with pair

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| pair | STRING | NO |  |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 50; max 100. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `symbol` or `pair` must be sent.
> - If `orderId` is set, it will get orders >= that orderId. Otherwise most recent orders are returned.
> - These orders will not be found:
>   - order status is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 3 days < current time

## Response Example

```javascript
[
  {
    "avgPrice": "0.0",
    "clientOrderId": "abc",
    "cumBase": "0",
    "executedQty": "0",
    "orderId": 1917641,
    "origQty": "0.40",
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075       // update time
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All Current CM Open Conditional Orders (USER\_DATA)

## API Description

Get all open conditional orders on a symbol. **Careful** when accessing this with no symbol.

## HTTP Request

GET `/papi/v1/cm/conditional/openOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

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
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD",
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3"                // callback rate, only return with TRAILING_STOP_MARKET order
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All Current CM Open Orders(USER\_DATA)

## API Description

Get all open orders on a symbol.

## HTTP Request

`GET /papi/v1/cm/openOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted
**Careful** when accessing this with no symbol.

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| pair | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If the symbol is not sent, orders for all symbols will be returned in an array.

## Response Example

```javascript
[
  {
    "avgPrice": "0.0",
    "clientOrderId": "abc",
    "cumBase": "0",
    "executedQty": "0",
    "orderId": 1917641,
    "origQty": "0.40",
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSD_200925",
    "pair":"BTCUSD",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075        // update time
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All Current UM Open Conditional Orders(USER\_DATA)

## API Description

Get all open conditional orders on a symbol.

## HTTP Request

`GET /papi/v1/um/conditional/openOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted
**Careful** when accessing this with no symbol.

**Parameters:**

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
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",               // callback rate, only return with TRAILING_STOP_MARKET order
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All Current UM Open Orders(USER\_DATA)

## API Description

Get all open orders on a symbol.

## HTTP Request

GET `/papi/v1/um/openOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

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
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSDT",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075，       // update time
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All Margin Account Orders (USER\_DATA)

## API Description

Query All Margin Account Orders

## HTTP Request

GET `/papi/v1/margin/allOrders`

## Weight

**100**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 500. |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

**Notes:**

- If `orderId` is set, it will get orders >= that `orderId`. Otherwise most recent orders are returned.
- For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.

## Response:

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
          "time": 1565769338806,
          "timeInForce": "GTC",
          "type": "TAKE_PROFIT_LIMIT",
          "updateTime": 1565769342148，
          "accountId": 152950866,
          "selfTradePreventionMode": "EXPIRE_TAKER",
          "preventedMatchId": null,
          "preventedQuantity": null
      }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All UM Conditional Orders(USER\_DATA)

## API Description

Query All UM Conditional Orders

## HTTP Request

GET `/papi/v1/um/conditional/allOrders`

## Request Weight

**1** for a single symbol; **40** when the symbol parameter is omitted

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| strategyId | LONG | NO |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - These orders will not be found:
>   - order strategyStatus is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 7 days < current time
> - The query time period must be less than 7 days( default as the recent 7 days).

## Response Example

```javascript
[
  {
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"TRIGGERED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "orderId":12132343435,     //Normal orderID after trigger if appliable, only have when the strategy is triggered
    "status": "NEW",             //Normal order status after trigger if appliable, only have when the strategy is triggered
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "triggerTime": 1566818724750,
    "timeInForce": "GTC",
    "type": "MARKET",     //Normal order type after trigger if appliable
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",                // callback rate, only return with TRAILING_STOP_MARKET order
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query All UM Orders(USER\_DATA)

## API Description

Get all account UM orders; active, canceled, or filled.

- These orders will not be found:
  - order status is `CANCELED` or `EXPIRED`, **AND**
  - order has NO filled trade, **AND**
  - created time + 3 days < current time

## HTTP Request

GET `/papi/v1/um/allOrders`

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

> - If `orderId` is set, it will get orders >= that orderId. Otherwise most recent orders are returned.
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
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSDT",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075,        // update time
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query CM Conditional Order History(USER\_DATA)

## API Description

Query CM Conditional Order History

## HTTP Request

GET `/papi/v1/cm/conditional/orderHistory`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Notes:**

> - Either `strategyId` or `newClientStrategyId` must be sent.
> - `NEW` orders will not be found.
> - These orders will not be found:
>   - order status is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 7 days < current time

## Response Example

```javascript
{
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"TRIGGERED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD",
    "orderId": 12123343534,    //Normal orderID after trigger if appliable，only have when the strategy is triggered
    "status": "NEW",   //Normal order status after trigger if appliable, only have when the strategy is triggered
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "triggerTime": 1566818724750,
    "timeInForce": "GTC",
    "type": "MARKET",     //Normal order type after trigger if appliable
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3"                // callback rate, only return with TRAILING_STOP_MARKET order
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query CM Modify Order History(TRADE)

## API Description

Get order modification history

## HTTP Request

GET `/papi/v1/cm/orderAmendment`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| startTime | LONG | NO | Timestamp in ms to get modification history from INCLUSIVE |
| endTime | LONG | NO | Timestamp in ms to get modification history until INCLUSIVE |
| limit | INT | NO | Default 50, max 100 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.

## Response Example

```javascript
[
    {
        "amendmentId": 5363,    // Order modification ID
        "symbol": "BTCUSD_PERP",
        "pair": "BTCUSD",
        "orderId": 20072994037,
        "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
        "time": 1629184560899,  // Order modification time
        "amendment": {
            "price": {
                "before": "30004",
                "after": "30003.2"
            },
            "origQty": {
                "before": "1",
                "after": "1"
            },
            "count": 3  // Order modification count, representing the number of times the order has been modified
        }
    },
    {
        "amendmentId": 5361,
        "symbol": "BTCUSD_PERP",
        "pair": "BTCUSD",
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
        "symbol": "BTCUSD_PERP",
        "pair": "BTCUSD",
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

# Query CM Order(USER\_DATA)

## API Description

Check an CM order's status.

## HTTP Request

GET `/papi/v1/cm/order`

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
> - These orders will not be found:
>   - order status is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 3 days < current time

## Response Example

```javascript
{
    "avgPrice": "0.0",
    "clientOrderId": "abc",
    "cumBase": "0",
    "executedQty": "0",
    "orderId": 1917641,
    "origQty": "0.40",
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "status": "NEW",
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "positionSide": "SHORT",
    "time": 1579276756075,             // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075        // update time
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Current CM Open Conditional Order(USER\_DATA)

## API Description

Query Current CM Open Conditional Order

## HTTP Request

GET `/papi/v1/cm/conditional/openOrder`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Notes:

> - Either `strategyId` or `newClientStrategyId` must be sent.
> - If the queried order has been triggered, cancelled or expired, the error message "Order does not exist" will be returned.

## Response Example

```javascript
{
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSD",
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3"                 // callback rate, only return with TRAILING_STOP_MARKET order
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Current CM Open Order (USER\_DATA)

## API Description

Query current CM open order

## HTTP Request

GET `/papi/v1/cm/openOrder`

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
> - If the queried order has been filled or cancelled, the error message "Order does not exist" will be returned.

## Response Example

```javascript
[
    {
        "avgPrice": "0.0",
        "clientOrderId": "abc",
        "cumBase": "0",
        "executedQty": "0",
        "orderId": 1917641,
        "origQty": "0.40",
        "origType": "LIMIT",
        "price": "0",
        "reduceOnly": false,
        "side": "BUY",
        "positionSide": "SHORT",
        "status": "NEW",
        "symbol": "BTCUSD_200925",
        "pair": "BTCUSD"
        "time": 1579276756075,              // order time
        "timeInForce": "GTC",
        "type": "LIMIT",
        "updateTime": 1579276756075        // update time
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Current Margin Open Order (USER\_DATA)

## API Description

Query Current Margin Open Order

## HTTP Request

GET `/papi/v1/margin/openOrders`

## Weight

**5**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

**Notes:**

- If the `symbol` is not sent, orders for all symbols will be returned in an array.
- When all symbols are returned, the number of requests counted against the rate limiter is equal to the number of symbols currently trading on the exchange.

## Response:

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
       "time": 1562040170089,
       "timeInForce": "GTC",
       "type": "LIMIT",
       "updateTime": 1562040170089，
       "accountId": 152950866,
       "selfTradePreventionMode": "EXPIRE_TAKER",
       "preventedMatchId": null,
       "preventedQuantity": null
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Current UM Open Conditional Order(USER\_DATA)

## API Description

Query Current UM Open Conditional Order

## HTTP Request

GET `/papi/v1/um/conditional/openOrder`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

Notes:

> - Either `strategyId` or `newClientStrategyId` must be sent.
> - If the queried order has been `CANCELED`, `TRIGGERED` or `EXPIRED`, the error message "Order does not exist" will be returned.

## Response Example

```javascript
{
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"NEW",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "timeInForce": "GTC",
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",               // callback rate, only return with TRAILING_STOP_MARKET order
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Current UM Open Order(USER\_DATA)

## API Description

Query current UM open order

## HTTP Request

GET `/papi/v1/um/openOrder`

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
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSDT",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075，
    "selfTradePreventionMode": "NONE",
    "goodTillDate": 0,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Margin Account's OCO (USER\_DATA)

## API Description

Retrieves a specific OCO based on provided optional parameters

## HTTP Request

GET `/papi/v1/margin/orderList`

## Weight

**5**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| orderListId | LONG | NO | Either orderListId or origClientOrderId must be provided |
| origClientOrderId | STRING | NO | Either orderListId or origClientOrderId must be provided |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response:

```javascript
{
  "orderListId": 27,
  "contingencyType": "OCO",
  "listStatusType": "EXEC_STARTED",
  "listOrderStatus": "EXECUTING",
  "listClientOrderId": "h2USkA5YQpaXHPIrkd96xE",
  "transactionTime": 1565245656253,
  "symbol": "LTCBTC",
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

Derivatives Trading

# Query Margin Account's Open OCO (USER\_DATA)

## API Description

Query Margin Account's Open OCO

## HTTP Request

GET `/papi/v1/margin/openOrderList`

## Weight

**5**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response:

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

Derivatives Trading

# Query Margin Account Order (USER\_DATA)

## API Description

Query Margin Account Order

## HTTP Request

GET `/papi/v1/margin/order`

## Weight

**10**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

**Notes:**

- Either `orderId` or `origClientOrderId` must be sent.
- For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.

## Response:

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
   "time": 1562133008725,
   "timeInForce": "GTC",
   "type": "LIMIT",
   "updateTime": 1562133008725，
   "accountId": 152950866,
   "selfTradePreventionMode": "EXPIRE_TAKER",
   "preventedMatchId": null,
   "preventedQuantity": null
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query Margin Account's all OCO (USER\_DATA)

## API Description

Query all OCO for a specific margin account based on provided optional parameters

## HTTP Request

GET `/papi/v1/margin/allOrderList`

## Weight

**100**

## Parameters:

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| fromId | LONG | NO | If supplied, neither startTime or endTime can be provided |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 500; max 500. |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

## Response:

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

Derivatives Trading

# Query UM Conditional Order History(USER\_DATA)

## API Description

Query UM Conditional Order History

## HTTP Request

GET `/papi/v1/um/conditional/orderHistory`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| strategyId | LONG | NO |  |
| newClientStrategyId | STRING | NO |  |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

**Notes:**

> - Either `strategyId` or `newClientStrategyId` must be sent.
> - `NEW` orders will not be found.
> - These orders will not be found:
>   - order status is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 7 days < current time

## Response Example

```javascript
{
    "newClientStrategyId": "abc",
    "strategyId":123445,
    "strategyStatus":"TRIGGERED",
    "strategyType": "TRAILING_STOP_MARKET",
    "origQty": "0.40",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "stopPrice": "9300",                // please ignore when order type is TRAILING_STOP_MARKET
    "symbol": "BTCUSDT",
    "orderId":12132343435,     //Normal orderID after trigger if appliable，only have when the strategy is triggered
    "status": "NEW",          //Normal order status after trigger if appliable, only have when the strategy is triggered
    "bookTime": 1566818724710,              // order time
    "updateTime": 1566818724722,
    "triggerTime": 1566818724750,
    "timeInForce": "GTC",
    "type": "MARKET",   //Normal order type after trigger if appliable
    "activatePrice": "9020",            // activation price, only return with TRAILING_STOP_MARKET order
    "priceRate": "0.3",               // callback rate, only return with TRAILING_STOP_MARKET order
    "workingType":"CONTRACT_PRICE",
    "priceProtect": false,
    "selfTradePreventionMode": "NONE", //self trading preventation mode
    "goodTillDate": 0
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query UM Modify Order History(TRADE)

## API Description

Get order modification history

## HTTP Request

GET `/papi/v1/um/orderAmendment`

## Request Weight(Order)

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| orderId | LONG | NO |  |
| origClientOrderId | STRING | NO |  |
| startTime | LONG | NO | Timestamp in ms to get modification history from INCLUSIVE |
| endTime | LONG | NO | Timestamp in ms to get modification history until INCLUSIVE |
| limit | INT | NO | Default 500, max 1000 |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - Either `orderId` or `origClientOrderId` must be sent, and the `orderId` will prevail if both are sent.

## Response Example

```javascript
[
    {
        "amendmentId": 5363,    // Order modification ID
        "symbol": "BTCUSDT",
        "pair": "BTCUSDT",
        "orderId": 20072994037,
        "clientOrderId": "LJ9R4QZDihCaS8UAOOLpgW",
        "time": 1629184560899,  // Order modification time
        "amendment": {
            "price": {
                "before": "30004",
                "after": "30003.2"
            },
            "origQty": {
                "before": "1",
                "after": "1"
            },
            "count": 3  // Order modification count, representing the number of times the order has been modified
        },
        "priceMatch": "NONE"
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
        },
        "priceMatch": "NONE"
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
        },
        "priceMatch": "NONE"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query UM Order (USER\_DATA)

## API Description

Check an UM order's status.

## HTTP Request

GET `/papi/v1/um/order`

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

> - These orders will not be found:
> - Either `orderId` or `origClientOrderId` must be sent.
>
>   - order status is `CANCELED` or `EXPIRED`, **AND**
>   - order has NO filled trade, **AND**
>   - created time + 3 days < current time

## Response Example

```javascript
{
    "avgPrice": "0.00000",
    "clientOrderId": "abc",
    "cumQuote": "0",
    "executedQty": "0",
    "orderId": 1917641,
    "origQty": "0.40",
    "origType": "LIMIT",
    "price": "0",
    "reduceOnly": false,
    "side": "BUY",
    "positionSide": "SHORT",
    "status": "NEW",
    "symbol": "BTCUSDT",
    "time": 1579276756075,              // order time
    "timeInForce": "GTC",
    "type": "LIMIT",
    "updateTime": 1579276756075,        // update time
    "selfTradePreventionMode": "NONE",
    "goodTillDate": 0,
    "priceMatch": "NONE"
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query User's CM Force Orders(USER\_DATA)

## API Description

Query User's CM Force Orders

## HTTP Request

GET `/papi/v1/cm/forceOrders`

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
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - If "autoCloseType" is not sent, orders with both of the types will be returned
> - If "startTime" is not sent, data within 7 days before "endTime" can be queried

## Response Example

```javascript
[
  {
    "orderId": 165123080,
    "symbol": "BTCUSD_200925",
    "pair": "BTCUSD",
    "status": "FILLED",
    "clientOrderId": "autoclose-1596542005017000006",
    "price": "11326.9",
    "avgPrice": "11326.9",
    "origQty": "1",
    "executedQty": "1",
    "cumBase": "0.00882854",
    "timeInForce": "IOC",
    "type": "LIMIT",
    "reduceOnly": false,
    "side": "SELL",
    "positionSide": "BOTH",
    "origType": "LIMIT",
    "time": 1596542005019,
    "updateTime": 1596542005050
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query User's Margin Force Orders(USER\_DATA)

## API Description

Query user's margin force orders

## HTTP Request

GET `/papi/v1/margin/forceOrders`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| current | LONG | NO | Currently querying page. Start from 1. Default:1 |
| size | LONG | NO | Default:10 Max:100 |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

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
            "updatedTime": 1558941374745
        }
    ],
    "total": 1
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Query User's UM Force Orders (USER\_DATA)

## API Description

Query User's UM Force Orders

## HTTP Request

GET `/papi/v1/um/forceOrders`

## Request Weight

**20** with symbol, **50** without symbol

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | NO |  |
| autoCloseType | ENUM | NO | `LIQUIDATION` for liquidation orders, `ADL` for ADL orders. |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| limit | INT | NO | Default 50; max 100. |
| recvWindow | LONG | NO | The value cannot be greater than 60000 |
| timestamp | LONG | YES |  |

> - If `autoCloseType` is not sent, orders with both of the types will be returned
> - If `startTime` is not sent, data within 7 days before `endTime` can be queried

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
    "side": "SELL",
    "positionSide": "BOTH",
    "origType": "LIMIT",
    "time": 1596107620044,
    "updateTime": 1596107620087
  }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Toggle BNB Burn On UM Futures Trade (TRADE)

## API Description

Change user's BNB Fee Discount for UM Futures (Fee Discount On or Fee Discount Off ) on _**EVERY symbol**_

## HTTP Request

POST `/papi/v1/um/feeBurn`

## Request Weight

**1**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| feeBurn | STRING | YES | "true": Fee Discount On; "false": Fee Discount Off |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

- The BNB would not be collected from UM-PM account to the Portfolio Margin account.

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

# UM Account Trade List(USER\_DATA)

## API Description

Get trades for a specific account and UM symbol.

## HTTP Request

GET `/papi/v1/um/userTrades`

## Request Weight

**5**

## Request Parameters

| Name | Type | Mandatory | Description |
| --- | --- | --- | --- |
| symbol | STRING | YES |  |
| startTime | LONG | NO |  |
| endTime | LONG | NO |  |
| fromId | LONG | NO | Trade id to fetch from. Default gets most recent trades. |
| limit | INT | NO | Default 500; max 1000. |
| recvWindow | LONG | NO |  |
| timestamp | LONG | YES |  |

> - If `startTime` and `endTime` are both not sent, then the last '7 days' data will be returned.
> - The time between `startTime` and `endTime` cannot be longer than 7 days.
> - The parameter `fromId` cannot be sent with `startTime` or `endTime`.

## Response Example

```javascript
[
    {
        "symbol": "BTCUSDT",
        "id": 67880589,
        "orderId": 270093109,
        "side": "SELL",
        "price": "28511.00",
        "qty": "0.010",
        "realizedPnl": "2.58500000",
        "quoteQty": "285.11000",
        "commission": "-0.11404400",
        "commissionAsset": "USDT",
        "time": 1680688557875,
        "buyer": false,
        "maker": false,
        "positionSide": "BOTH"
    }
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# UM Position ADL Quantile Estimation(USER\_DATA)

## API Description

Query UM Position ADL Quantile Estimation

> - Values update every 30s.
> - Values 0, 1, 2, 3, 4 shows the queue position and possibility of ADL from low to high.
> - For positions of the symbol are in One-way Mode or isolated margined in Hedge Mode, "LONG", "SHORT", and "BOTH" will be returned to show the positions' adl quantiles of different position sides.
> - If the positions of the symbol are crossed margined in Hedge Mode:
>   - "HEDGE" as a sign will be returned instead of "BOTH";
>   - A same value caculated on unrealized pnls on long and short sides' positions will be shown for "LONG" and "SHORT" when there are positions in both of long and short sides.

## HTTP Request

GET `/papi/v1/um/adlQuantile`

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
				// if the positions of the symbol are crossed margined in Hedge Mode, "LONG" and "SHORT" will be returned a same quantile value.
				"LONG": 3,
				"SHORT": 3,
				"BOTH": 0
			}
	},
 	{
 		"symbol": "BTCUSDT",
 		"adlQuantile":
 			{
 				"LONG": 0, 	 	// adl quantile for "LONG" position in hedge mode
 				"SHORT": 0, 	// adl quantile for "SHORT" position in hedge mode
 				"BOTH": 2		// adl quantile for position in one-way mode
 			}
 	}
]
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# User Data Streams Connect

- The base API endpoint is: **[https://papi.binance.com](https://papi.binance.com/)**

- A User Data Stream `listenKey` is valid for 60 minutes after creation.

- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes, if response `-1125` error "This listenKey does not exist." Please use `POST /papi/v1/listenKey` to recreate `listenKey`.

- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.

- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.
\*Connection method for Websocket:
  - Base Url: **wss://fstream.binance.com/pm**
  - User Data Streams are accessed at **/ws/<listenKey>**
  - Example: `wss://fstream.binance.com/pm/ws/pqia91ma19a5s61cv6a81va65sdf19v8a65a1a5s61cv6a81va65sdf19v8a65a1`
- For one connection(one user data), the user data stream payloads can guaranteed to be in order during heavy periods; **Strongly recommend you order your updates using E**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Close User Data Stream(USER\_STREAM)

## API Description

Close out a user data stream.

## HTTP Request

DELETE `/papi/v1/listenKey`

## Request Weight

**1**

## Request Parameters

**None**

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# User Data Streams Connect

- The base API endpoint is: **[https://papi.binance.com](https://papi.binance.com/)**

- A User Data Stream `listenKey` is valid for 60 minutes after creation.

- Doing a `PUT` on a `listenKey` will extend its validity for 60 minutes, if response `-1125` error "This listenKey does not exist." Please use `POST /papi/v1/listenKey` to recreate `listenKey`.

- Doing a `DELETE` on a `listenKey` will close the stream and invalidate the `listenKey`.

- Doing a `POST` on an account with an active `listenKey` will return the currently active `listenKey` and extend its validity for 60 minutes.
\*Connection method for Websocket:
  - Base Url: **wss://fstream.binance.com/pm**
  - User Data Streams are accessed at **/ws/<listenKey>**
  - Example: `wss://fstream.binance.com/pm/ws/pqia91ma19a5s61cv6a81va65sdf19v8a65a1a5s61cv6a81va65sdf19v8a65a1`
- For one connection(one user data), the user data stream payloads can guaranteed to be in order during heavy periods; **Strongly recommend you order your updates using E**

- A single connection is only valid for 24 hours; expect to be disconnected at the 24 hour mark

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Conditional Order Trade Update

## Event Description

When new order created, order status changed will push such event. event type is `CONDITIONAL_ORDER_TRADE_UPDATE`.

**Side**

- BUY
- SELL

**Conditional Order Type**

- STOP
- TAKE\_PROFIT
- STOP\_MARKET
- TAKE\_PROFIT\_MARKET
- TRAILING\_STOP\_MARKET

**Execution Type**

- NEW
- CANCELED
- CALCULATED - Liquidation Execution
- EXPIRED
- TRADE

**Order Status**

- NEW
- CANCELED
- EXPIRED
- TRIGGERED
- FINISHED

**Time in force**

- GTC
- IOC
- FOK
- GTX

## Event Name

`CONDITIONAL_ORDER_TRADE_UPDATE`

## Response Example

```javascript
{
    "e": "CONDITIONAL_ORDER_TRADE_UPDATE", // Event Type
    "T": 1669262908216,                    // Transaction Time
    "E": 1669262908218,                    // Event Time
    "fs": "UM",                            // Event business unit
    "so": {
            "s": "BTCUSDT",                // Symbol
            "c":"TEST",                    // Strategy Client Order Id
            "si": 176057039,               // Strategy ID
            "S":"SELL",                    // Side
            "st": "TRAILING_STOP_MARKET",  // Strategy Type
            "f":"GTC",                     // Time in Force
            "q":"0.001",                   //Quantity
            "p":"0",                       //Price
            "sp":"7103.04",                // Stop Price. Please ignore with TRAILING_STOP_MARKET order
            "os":"NEW",                    // Strategy Order Status
            "T":1568879465650,             // Order book Time
            "ut": 1669262908216,           // Order update Time
            "R":false,                     // Is this reduce only
            "wt":"MARK_PRICE",             // Stop Price Working Type
            "ps":"LONG",                   // Position Side
            "cp":false,                    // If Close-All, pushed with conditional order
            "AP":"7476.89",                // Activation Price, only pushed with TRAILING_STOP_MARKET order
            "cr":"5.0",                    // Callback Rate, only puhed with TRAILING_STOP_MARKET order
            "i":8886774,                   // Order Id
            "V":"EXPIRE_TAKER",         // STP mode
            "gtd":0
        }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Futures Account Configuration Update(Leverage Update)

## Event Description

When the account configuration is changed, the event type will be pushed as `ACCOUNT_CONFIG_UPDATE`
When the leverage of a trade pair changes, the payload will contain the object `ac` to represent the account configuration of the trade pair, where `s` represents the specific trade pair and `l` represents the leverage.

## Event Name

`ACCOUNT_CONFIG_UPDATE`

## Response Example

```javascript
{
    "e":"ACCOUNT_CONFIG_UPDATE",       // Event Type
    "fs": "UM",                       // Event business unit
    "E":1611646737479,                 // Event Time
    "T":1611646737476,                 // Transaction Time
    "ac":{
    "s":"BTCUSD_PERP",                     // symbol
    "l":25                             // leverage

    }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Futures Balance and Position Update

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

## Event Name

`ACCOUNT_UPDATE`

## Update Speed

50ms

## Response Example

```javascript
{
  "e": "ACCOUNT_UPDATE",                // Event Type
  "fs": "UM",                           // Event business unit. 'UM' for USDS-M futures and 'CM' for COIN-M futures
  "E": 1564745798939,                   // Event Time
  "T": 1564745798938 ,                  // Transaction
  "i":"",                           // Account Alias, ignore for UM
  "a":                                  // Update Data
    {
      "m":"ORDER",                      // Event reason type
      "B":[                             // Balances
        {
          "a":"USDT",                   // Asset
          "wb":"122624.12345678",       // Wallet Balance
          "cw":"100.12345678",          // Cross Wallet Balance
          "bc":"50.12345678"            // Balance Change except PnL and Commission
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
          "s":"BTCUSDT",            // Symbol
          "pa":"0",                 // Position Amount
          "ep":"0.00000",            // Entry Price
          "cr":"200",               // (Pre-fee) Accumulated Realized
          "up":"0",                     // Unrealized PnL
          "ps":"BOTH",                   // Position Side
          "bep":"0.00000"            // breakeven price
        }，
        {
            "s":"BTCUSDT",
            "pa":"20",
            "ep":"6563.66500",
            "cr":"0",
            "up":"2850.21200",
            "ps":"LONG",
            "bep":"0.00000"            // breakeven price
         }
      ]
    }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Futures Order update

## Event Description

When new order created, order status changed will push such event. event type is `ORDER_TRADE_UPDATE`.

**Side**

- BUY
- SELL

**Order Type**

- MARKET
- LIMIT
- LIQUIDATION

**Execution Type**

- NEW
- CANCELED
- CALCULATED - Liquidation Execution
- EXPIRED
- TRADE

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

**Liquidation and ADL:**

- If user gets liquidated due to insufficient margin balance:
  - c shows as "autoclose-XXX"，X shows as "NEW"
- If user has enough margin balance but gets ADL:
  - c shows as “adl\_autoclose”，X shows as “NEW”

## Event Name

`ORDER_TRADE_UPDATE`

## Response Example

```javascript
{
  "e":"ORDER_TRADE_UPDATE",     // Event Type
  "fs": "UM",                   // Event business unit. 'UM' for USDS-M futures and 'CM' for COIN-M futures
  "E":1568879465651,            // Event Time
  "T":1568879465650,            // Transaction Time
  "i":"",                   // Account Alias,ignore for UM
  "o":{
    "s":"BTCUSDT",              // Symbol
    "c":"TEST",                 // Client Order Id
      // special client order id:
      // starts with "autoclose-": liquidation order
      // "adl_autoclose": ADL auto close order
      // "settlement_autoclose-": settlement order for delisting or delivery
    "S":"SELL",                 // Side
    "o":"MARKET", // Order Type
    "f":"GTC",                  // Time in Force
    "q":"0.001",                // Original Quantity
    "p":"0",                    // Original Price
    "ap":"0",                   // Average Price
    "sp":"7103.04",					    // Ignore
    "x":"NEW",                  // Execution Type
    "X":"NEW",                  // Order Status
    "i":8886774,                // Order Id
    "l":"0",                    // Order Last Filled Quantity
    "z":"0",                    // Order Filled Accumulated Quantity
    "L":"0",                    // Last Filled Price
    "N":"USDT",             // Commission Asset, will not push if no commission
    "n":"0",                // Commission, will not push if no commission
    "T":1568879465650,          // Order Trade Time
    "t":0,                      // Trade Id
    "b":"0",                    // Bids Notional
    "a":"9.91",                 // Ask Notional
    "m":false,                  // Is this trade the maker side?
    "R":false,                  // Is this reduce only
    "ps":"LONG",                // Position Side
    "rp":"0",                   // Realized Profit of the trade
    "st":"C_TAKE_PROFIT",       // Strategy type, only pushed with conditional order triggered
    "si":12893,                  // StrategyId,only pushed with conditional order triggered
    "V":"EXPIRE_TAKER",         // STP mode
    "gtd":0
  }
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Liability Update

## Event Description

Margin Liability update

## Event Name

`liabilityChange`

## Response Example

```javascript
{
  "e": "liabilityChange",        //Event Type
  "E": 1573200697110,            //Event Time
  "a": "BTC",                    //Asset
  "t": “BORROW”                  //Type
  "T": 1352286576452864727,     //Transaction ID
  "p": "1.03453430",             //Principal
  "i": "0",                      //Interest
  "l": "1.03476851"              //Total Liability
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Margin Account Update

## Event Description

`outboundAccountPosition` is sent any time an account balance has changed and contains the assets that were possibly changed by the event that generated the balance change.

## Event Name

`outboundAccountPosition`

## Response Example

```javascript
{
  "e": "outboundAccountPosition", //Event type
  "E": 1564034571105,             //Event Time
  "u": 1564034571073,             //Time of last account update
  "U": 1027053479517,             // time updateID
  "B": [                          //Balances Array
    {
      "a": "ETH",                 //Asset
      "f": "10000.000000",        //Free
      "l": "0.000000"             //Locked
    }
  ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Margin Balance Update

## Event Description

Margin Balance Update

## Event Name

`balanceUpdate`

## Response Example

```javascript
{
  "e": "balanceUpdate",         //Event Type
  "E": 1573200697110,           //Event Time
  "a": "BTC",                   //Asset
  "d": "100.00000000",          //Balance Delta
  "U": 1027053479517            //event updateId
  "T": 1573200697068            //Clear Time
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: Margin Order Update

## Event Description

Margin orders are updated with the `executionReport` event.

**Execution types:**

- NEW - The order has been accepted into the engine.
- CANCELED - The order has been canceled by the user.
- REJECTED - The order has been rejected and was not processed (This message appears only with Cancel Replace Orders wherein the new order placement is rejected but the request to cancel request succeeds.)
- TRADE - Part of the order or all of the order's quantity has filled.
- EXPIRED - The order was canceled according to the order type's rules (e.g. LIMIT FOK orders with no fill, LIMIT IOC or MARKET orders that partially fill) or by the exchange, (e.g. orders canceled during liquidation, orders canceled during maintenance).
- TRADE\_PREVENTION - The order has expired due to STP trigger.
Check the Public API Definitions for more relevant enum definitions.

## Event Name

`executionReport`

## Response Example

```javascript
{
  "e": "executionReport",        // Event type
  "E": 1499405658658,            // Event time
  "s": "ETHBTC",                 // Symbol
  "c": "mUvoqJxFIILMdfAW5iGSOW", // Client order ID
  "S": "BUY",                    // Side
  "o": "LIMIT",                  // Order type
  "f": "GTC",                    // Time in force
  "q": "1.00000000",             // Order quantity
  "p": "0.10264410",             // Order price
  "P": "0.00000000",             // Stop price
  "d": 4,                        // Trailing Delta; This is only visible if the order was a trailing stop order.
  "F": "0.00000000",             // Iceberg quantity; Will not be visible if not iceberg order
  "g": -1,                       // OrderListId
  "C": "",                       // Original client order ID; Only visible on cancellation of order, the ID of the order being canceled.
  "x": "NEW",                    // Current execution type
  "X": "NEW",                    // Current order status
  "r": "NONE",                   // Order reject reason; Only visible if there is a rejection, will be an error code.
  "i": 4293153,                  // Order ID
  "l": "0.00000000",             // Last executed quantity
  "z": "0.00000000",             // Cumulative filled quantity
  "L": "0.00000000",             // Last executed price
  "n": "0",                      // Commission amount
  "N": null,                     // Commission asset; Only visible when there is a commission amount.
  "T": 1499405658657,            // Transaction time
  "t": -1,                       // Trade ID
  "v": 3,                        // Prevented Match Id; This is only visible if the order expire due to STP trigger.
  "I": 8641984,                  // updateId
  "w": true,                     // Is the order on the book?
  "m": false,                    // Is this trade the maker side?
  "O": 1499405658657,            // Order creation time
  "Z": "0.00000000",             // Cumulative quote asset transacted quantity
  "Y": "0.00000000",             // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
  "Q": "0.00000000",             // Quote Order Quantity; This is only visible if indicated in the order
  "D": 1668680518494,            // Trailing Time; This is only visible if the trailing stop order has been activated.
  "j": 1,                        // Strategy ID; This is only visible if the strategyId parameter was provided upon order placement
  "J": 1000000,                  // Strategy Type; This is only visible if the strategyType parameter was provided upon order placement
  "W": 1499405658657,            // Working Time; This is only visible if the order has been placed on the book.
  "V": "NONE",                   // selfTradePreventionMode
  "u":1,                         // TradeGroupId; This is only visible if the account is part of a trade group and the order expired due to STP trigger.
  "U":37,                        // CounterOrderId; This is only visible if the order expired due to STP trigger.
  "A":"3.000000",                // Prevented Quantity; This is only visible if the order expired due to STP trigger.
  "B":"3.000000"                 // Last Prevented Quantity; This is only visible if the order expired due to STP trigger.
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: OpenOrderLoss Update

## Event Description

Cross margin order margin stream

## Event Name

`openOrderLoss`

## Response Example

```javascript
{
    "e": "openOrderLoss",      //Event Type
    "E": 1678710578788,        // Event Time
    "O": [
        {                    // Update Data
        "a": "BUSD",
        "o": "-0.1232313"       // Amount
        },
        {
        "a": "BNB",
        "o": "-12.1232313"
        }
    ]
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: User Data Stream Expired

## Event Description

When the `listenKey` used for the user data stream turns expired, this event will be pushed.

**Notice:**

- This event is not related to the websocket disconnection.
- This event will be received only when a valid `listenKey` in connection got expired.
- No more user data event will be updated after this event received until a new valid `listenKey` used.

## Event Name

`listenKeyExpired`

## Response Example

```javascript
{
    'e': 'listenKeyExpired',      // event type
    'E': 1576653824250              // event time
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Event: riskLevelChange

## Event Description

- When the user's position risk ratio is too high, this stream will be pushed.
- This message is only used as risk guidance information and is not recommended for investment strategies.
- `RISK_LEVEL_CHANGE`includes following types：`MARGIN_CALL`, `REDUCE_ONLY`, `FORCE_LIQUIDATION`
- In the case of a highly volatile market, there may be the possibility that the user's position has been liquidated at the same time when this stream is pushed out.

## Event Name

`RISK_LEVEL_CHANGE`

## Response Example

```javascript
{
    "e":"riskLevelChange",      // Event Type
    "E":1587727187525,      // Event Time
    "u":"1.99999999",      // uniMMR level
    "s":"MARGIN_CALL",        //MARGIN_CALL, REDUCE_ONLY, FORCE_LIQUIDATION
    "eq":"30.23416728",      // account equity in USD value
    "ae":"30.23416728",      // actual equity without collateral rate in USD value
    "m":"15.11708371"      // total maintenance margin in USD value
}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Keepalive User Data Stream (USER\_STREAM)

## API Description

Keepalive a user data stream to prevent a time out. User data streams will close after 60 minutes. It's recommended to send a ping about every 60 minutes.

## HTTP Request

PUT `/papi/v1/listenKey`

## Request Weight

**1**

## Request Parameters

**None**

## Response Example

```javascript
{}
```

 

Copyright © 2026 Binance.

---

Derivatives Trading

# Start User Data Stream(USER\_STREAM)

## API Description

Start a new user data stream. The stream will close after 60 minutes unless a keepalive is sent. If the account has an active `listenKey`, that `listenKey` will be returned and its validity will be extended for 60 minutes.

## HTTP Request

POST `/papi/v1/listenKey`

## Request Weight

**1**

## Request Parameters

**None**

## Response Example

```javascript
{
  "listenKey": "pqia91ma19a5s61cv6a81va65sdf19v8a65a1a5s61cv6a81va65sdf19v8a65a1"
}
```

 

Copyright © 2026 Binance.

---
