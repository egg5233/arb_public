# Binance Portfolio Margin (PM) API Documentation Reference

> Scraped from [Binance Developers](https://developers.binance.com/docs/derivatives/portfolio-margin/) on 2026-03-29.
> Base URL: `https://papi.binance.com`

## Overview

Portfolio Margin accounts use `/papi/` endpoints instead of `/fapi/` (classic USDT-M futures).
The signing method is identical (HMAC SHA256), but the base URL and paths differ.

### Endpoint Mapping (Classic → Portfolio Margin)

| Function | Classic (`fapi`) | Portfolio Margin (`papi`) |
|----------|-----------------|--------------------------|
| Place UM order | `POST /fapi/v1/order` | `POST /papi/v1/um/order` |
| Cancel UM order | `DELETE /fapi/v1/order` | `DELETE /papi/v1/um/order` |
| Cancel all UM orders | `DELETE /fapi/v1/allOpenOrders` | `DELETE /papi/v1/um/allOpenOrders` |
| Query UM order | `GET /fapi/v1/order` | `GET /papi/v1/um/order` |
| All UM open orders | `GET /fapi/v1/openOrders` | `GET /papi/v1/um/openOrders` |
| All UM orders | `GET /fapi/v1/allOrders` | `GET /papi/v1/um/allOrders` |
| Account balance | `GET /fapi/v2/account` | `GET /papi/v1/balance` |
| UM positions | `GET /fapi/v2/positionRisk` | `GET /papi/v1/um/positionRisk` |
| Set leverage | `POST /fapi/v1/leverage` | `POST /papi/v1/um/leverage` |
| Set position mode | `POST /fapi/v1/positionSide/dual` | `POST /papi/v1/um/positionSide/dual` |
| Get position mode | `GET /fapi/v1/positionSide/dual` | `GET /papi/v1/um/positionSide/dual` |
| Leverage brackets | `GET /fapi/v1/leverageBracket` | `GET /papi/v1/um/leverageBracket` |
| Commission rate | `GET /fapi/v1/commissionRate` | `GET /papi/v1/um/commissionRate` |
| Account trades | `GET /fapi/v1/userTrades` | `GET /papi/v1/um/userTrades` |
| SL/TP (algo order) | `POST /fapi/v1/algo/order` | `POST /papi/v1/um/conditional/order` |
| Cancel algo order | `DELETE /fapi/v1/algo/order` | `DELETE /papi/v1/um/conditional/order` |
| Force orders | `GET /fapi/v1/forceOrders` | `GET /papi/v1/um/forceOrders` |
| Income history | `GET /fapi/v1/income` | `GET /papi/v1/um/income` |

### Rate Limits
- IP Limit: 6000/min (vs 2400/min for classic)
- Order Limit: 1200/min

---

## Raw Endpoint Documentation


## Section: https://developers.binance.com/docs/derivatives/portfolio-margin/general-info

Portfolio MarginGeneral Info
General Info
General API Information​
The base endpoint is: https://papi.binance.com
All endpoints return either a JSON object or raw primitive.
Data is returned in ascending order. Oldest first, newest last.
All time and timestamp related fields are in UTC milliseconds.
All data types adopt definition in JAVA.
HTTP Return Codes​
HTTP 4XX return codes are used for for malformed requests; the issue is on the sender's side.
HTTP 403 return code is used when the WAF Limit (Web Application Firewall) has been violated.
HTTP 429 return code is used when breaking a request rate limit.
HTTP 418 return code is used when an IP has been auto-banned for continuing to send requests after receiving 429 codes.
HTTP 5XX return codes are used for internal errors; the issue is on Binance's side.
If there is an error message "Request occur unknown error.", please retry later.
HTTP 503 return code is used when:
If there is an error message "Unknown error, please check your request or try again later." returned in the response, the API successfully sent the request but not get a response within the timeout period.It is important to NOT treat this as a failure operation; the execution status is UNKNOWN and could have been a success;
If there is an error message "Service Unavailable." returned in the response, it means this is a failure API operation and the service might be unavailable at the moment, you need to retry later.
If there is an error message "Internal error; unable to process your request. Please try again." returned in the response, it means this is a failure API operation and you can resend your request if you need.
If the response contains the error message "Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again." (-1008), This indicates the node has exceeded its maximum concurrency and is temporarily throttled. Close-position, reduce-only, and cancel orders are exempt and will not receive this error.
HTTP 503 Status: Message Variants & Handling​
A. “Unknown error, please check your request or try again later.” (Execution status unknown)​
Meaning: Request accepted but no response before timeout; execution may have succeeded.
Handling:
Do not treat as immediate failure; first verify via WebSocket updates or orderId queries to avoid duplicates.
During peaks, prefer single orders over batch to reduce uncertainty.
Rate-limit counting: May or may not count, check header to verify rate limit info
B. “Service Unavailable.” (Failure)​
Meaning: Service temporarily unavailable; 100% failure.
Handling: Retry with exponential backoff (e.g., 200ms → 400ms → 800ms, max 3–5 attempts).
Rate-limit counting: not counted
C. “Request throttled by system-level protection. Reduce-only/close-position orders are exempt. Please try again.” (-1008, Failure)​
Meaning: System overload; 100% failure.
Handling: Retry with backoff and reduce concurrency;
Applicable endpoints:
POST /fapi/v1/order / POST /dapi/v1/order / POST /papi/v1/order
POST /fapi/v1/batchOrders / POST /dapi/v1/batchOrders / POST papi/v1/batchOrders
Rate-limit counting: Not counted (overload protection).
Exception integrated here: When a request reduces exposure (Reduce-only / Close-position: closePosition = true, or positionSide = BOTH with reduceOnly = true, or LONG+SELL, or SHORT+BUY), it is not affected or prioritized under -1008 to ensure risk reduction.
Covered endpoints: POST /fapi/v1/order、POST /dapi/v1/order、POST /papi/v1/order、POST /fapi/v1/batchOrders、POST /dapi/v1/batchOrders、POST /papi/v1/batchOrders (when parameters satisfy the condition)
Error Codes and Messages​
Any endpoint can return an ERROR
Specific error codes and messages defined in Error Codes.
General Information on Endpoints​
For GET endpoints, parameters must be sent as a query string.
For POST, PUT, and DELETE endpoints, the parameters may be sent as a query string or in the request body with content type application/x-www-form-urlencoded. You may mix parameters between both the query string and request body if you wish to do so.
Parameters may be sent in any order.
If a parameter sent in both the query string and request body, the query string parameter will be used.
LIMITS​
A 429 will be returned when either rate limit is violated.
Binance has the right to further tighten the rate limits on users with intent to attack.
IP Limits​
Every request will contain X-MBX-USED-WEIGHT-(intervalNum)(intervalLetter) in the response headers which has the current used weight for the IP for all request rate limiters defined.
Each route has a weight which determines for the number of requests each endpoint counts for. Heavier endpoints and endpoints that do operations on multiple symbols will have a heavier weight.
When a 429 is received, it's your obligation as an API to back off and not spam the API.
Repeatedly violating rate limits and/or failing to back off after receiving 429s will result in an automated IP ban (HTTP status 418).
IP bans are tracked and scale in duration for repeat offenders, from 2 minutes to 3 days.
The limits on the API are based on the IPs, not the API keys.
Portfolio Margin IP Limit is 6000/min.
It is strongly recommended to use websocket stream for getting data as much as possible, which can not only ensure the timeliness of the message, but also reduce the access restriction pressure caused by the request.
Order Rate Limits​
Every order response will contain a X-MBX-ORDER-COUNT-(intervalNum)(intervalLetter) header which has the current order count for the account for all order rate limiters defined.
Rejected/unsuccessful orders are not guaranteed to have X-MBX-ORDER-COUNT-** headers in the response.
The order rate limit is counted against each account.
Portfolio Margin Order Limits are 1200/min.
Endpoint Security Type​
Each endpoint has a security type that determines the how you will interact with it.
API-keys are passed into the Rest API via the X-MBX-APIKEY header.
API-keys and secret-keys are case sensitive.
API-keys can be configured to only access certain types of secure endpoints. For example, one API-key could be used for TRADE only, while another API-key can access everything except for TRADE routes.
By default, API-keys can access all secure routes.
Security Type	Description
NONE	Endpoint can be accessed freely.
TRADE	Endpoint requires sending a valid API-Key and signature.
USER_DATA	Endpoint requires sending a valid API-Key and signature.
USER_STREAM	Endpoint requires sending a valid API-Key and signature.
SIGNED (TRADE and USER_DATA) Endpoint Security​
SIGNED endpoints require an additional parameter, signature, to be sent in the query string or request body.
Endpoints use HMAC SHA256 signatures. The HMAC SHA256 signature is a keyed HMAC SHA256 operation. Use your secretKey as the key and totalParams as the value for the HMAC operation.
The signature is not case sensitive.
Please make sure the signature is the end part of your query string or request body.
totalParams is defined as the query string concatenated with the request body.
Timing security​
A SIGNED endpoint also requires a parameter, timestamp, to be sent which should be the millisecond timestamp of when the request was created and sent.
An additional parameter, recvWindow, may be sent to specify the number of milliseconds after timestamp the request is valid for. If recvWindow is not sent, it defaults to 5000. recvWindow cannot exceed 60000.
If the server determines that the timestamp sent by the client is more than one second in the future of the server time, the request will also be rejected.

Serious trading is about timing. Networks can be unstable and unreliable, which can lead to requests taking varying amounts of time to reach the servers. With recvWindow, you can specify that the request must be processed within a certain number of milliseconds or be rejected by the server.

It is recommended to use a small recvWindow of 5000 or less! The max cannot go beyond 60,000!
SIGNED Endpoint Examples for POST /papi/v1/um/order​

Here is a step-by-step example of how to send a valid signed payload from the Linux command line using echo, openssl, and curl.

Key	Value
apiKey	22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn
secretKey	YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG
Parameter	Value
symbol	BTCUSDT
side	BUY
type	LIMIT
timeInForce	GTC
quantity	1
price	2000
recvWindow	5000
timestamp	1611825601400
Example 1: As a request body​

Example 1

HMAC SHA256 signature:

    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= 7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7


curl command:

    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order' -d 'symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400&signature=7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7'
    

requestBody:

symbol=BTCUSDT &side=BUY
&type=LIMIT
&timeInForce=GTC
&quantity=1
&price=2000
&recvWindow=5000
&timestamp=1611825601400

Example 2: As a query string​

Example 2

HMAC SHA256 signature:

    $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= 7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7


curl command:

    (HMAC SHA256)
   $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400&signature=7c12045972f6140e765e0f2b67d28099718df805732676494238f50be830a7d7'


queryString:

symbol=BTCUSDT
&side=BUY
&type=LIMIT
&timeInForce=GTC
&quantity=1
&price=2000
&recvWindow=5000
&timestamp=1611825601400

Example 3: Mixed query string and request body​

Example 3

HMAC SHA256 signature:

   $ echo -n "symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTCquantity=0.01&price=2000&recvWindow=5000&timestamp=1611825601400" | openssl dgst -sha256 -hmac "YtP1BudNOWZE1ag5uzCkh4hIC7qSmQOu797r5EJBFGhxBYivjj8HIX0iiiPof5yG"
    (stdin)= fa6045c54fb02912b766442be1f66fab619217e551a4fb4f8a1ee000df914d8e 


curl command:

    (HMAC SHA256)
    $ curl -H "X-MBX-APIKEY: 22BjeOROKiXJ3NxbR3zjh3uoGcaflPu3VMyBXAg8Jj2J1xVSnY0eB4dzacdE9IWn" -X POST 'https://papi.binance.com/papi/v1/order?symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC' -d 'quantity=0.01&price=2000&recvWindow=5000&timestamp=1611825601400&signature=fa6045c54fb02912b766442be1f66fab619217e551a4fb4f8a1ee000df914d8e'

queryString:

symbol=BTCUSDT&side=BUY&type=LIMIT&timeInForce=GTC

requestBody:

quantity=1&price=2000&recvWindow=5000&timestamp=1611825601400

Note that the signature is different in example 3. There is no & between "GTC" and "quantity=1".

RSA Keys - SIGNED Endpoint Examples for POST /papi/v1/um/order​
This will be a step by step process how to create the signature payload to send a valid signed payload.
We support PKCS#8 currently.
To get your API key, you need to upload your RSA Public Key to your account and a corresponding API key will be provided for you.

For this example, the private key will be referenced as test-prv-key.pem

Key	Value
apiKey	vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2
Parameter	Value
symbol	BTCUSDT
side	BUY
type	LIMIT
timeInForce	GTC
quantity	1
price	2000
recvWindow	5000
timestamp	1611825601400

Step 1: Construct the payload

Arrange the list of parameters into a string. Separate each parameter with a &.

Step 2: Compute the signature:

2.1 - Encode signature payload as ASCII data.

Step 2.2

 $ echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem


2.2 - Sign payload using RSASSA-PKCS1-v1_5 algorithm with SHA-256 hash function.

Step 2.3

$ echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem | openssl enc -base64
aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D


2.3 - Encode output as base64 string.

Step 2.4

$  echo -n 'timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23' | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem | openssl enc -base64 | tr -d '\n'
aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D


2.4 - Delete any newlines in the signature.

Step 2.5

aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D


2.5 - Since the signature may contain / and =, this could cause issues with sending the request. So the signature has to be URL encoded.

Step 2.6

 curl -H "X-MBX-APIKEY: vE3BDAL1gP1UaexugRLtteaAHg3UO8Nza20uexEuW1Kh3tVwQfFHdAiyjjY428o2" -X POST 'https://papi.binance.com/papi/v1/um/order?timestamp=1671090801999&recvWindow=9999999&symbol=BTCUSDT&side=SELL&type=MARKET&quantity=1.23&signature=aap36wD5loVXizxvvPI3wz9Cjqwmb3KVbxoym0XeWG1jZq8umqrnSk8H8dkLQeySjgVY91Ufs%2BBGCW%2B4sZjQEpgAfjM76riNxjlD3coGGEsPsT2lG39R%2F1q72zpDs8pYcQ4A692NgHO1zXcgScTGgdkjp%2Brp2bcddKjyz5XBrBM%3D'


2.6 - curl command

Bash script

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
rawSignature=$(echo -n "$paramsWithTs" \
               | openssl dgst -keyform PEM -sha256 -sign ./test-prv-key.pem \  ### THIS IS YOUR PRIVATE KEY. DO NOT SHARE THIS FILE WITH ANYONE.
               | openssl enc -base64 \
               | tr -d '\n')
signature=$(rawurlencode "$rawSignature")
curl -H "X-MBX-APIKEY: $apiKey" -X $apiMethod \
    "https://papi.binance.com/papi/$apiCall?$paramsWithTs&signature=$signature"


A sample Bash script containing similar steps is available in the right side.

## Section: https://developers.binance.com/docs/derivatives/portfolio-margin/account

Portfolio MarginAccountAccount Balance
Account Balance(USER_DATA)
API Description​

Query account balance

HTTP Request​

GET /papi/v1/balance

Request Weight​

20

Request Parameters​
Name	Type	Mandatory	Description
asset	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Response Example​
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


OR (when asset sent)

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


## Section: https://developers.binance.com/docs/derivatives/portfolio-margin/trade

Portfolio MarginTradeNew Um Order
New UM Order (TRADE)
API Description​

Place new UM order

HTTP Request​

POST /papi/v1/um/order

Request Weight(Order)​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
side	ENUM	YES	
positionSide	ENUM	NO	Default BOTH for One-way Mode ; LONG or SHORT for Hedge Mode. It must be sent in Hedge Mode.
type	ENUM	YES	LIMIT, MARKET
timeInForce	ENUM	NO	
quantity	DECIMAL	NO	
reduceOnly	STRING	NO	"true" or "false". default "false". Cannot be sent in Hedge Mode .
price	DECIMAL	NO	
newClientOrderId	STRING	NO	A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: ^[\.A-Z\:/a-z0-9_-]{1,32}$
newOrderRespType	ENUM	NO	ACK, RESULT, default ACK
priceMatch	ENUM	NO	only avaliable for LIMIT/STOP/TAKE_PROFIT order; can be set to OPPONENT/ OPPONENT_5/ OPPONENT_10/ OPPONENT_20: /QUEUE/ QUEUE_5/ QUEUE_10/ QUEUE_20; Can't be passed together with price
selfTradePreventionMode	ENUM	NO	NONE:No STP / EXPIRE_TAKER:expire taker order when STP triggers/ EXPIRE_MAKER:expire taker order when STP triggers/ EXPIRE_BOTH:expire both orders when STP triggers
goodTillDate	LONG	NO	order cancel time for timeInForce GTD, mandatory when timeInforce set to GTD; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode.
recvWindow	LONG	NO	
timestamp	LONG	YES	

Additional mandatory parameters based on type:

Type	Additional mandatory parameters
LIMIT	timeInForce, quantity, price
MARKET	quantity
If newOrderRespType is sent as RESULT :
MARKET order: the final FILLED result of the order will be return directly.
LIMIT order with special timeInForce: the final status result of the order(FILLED or EXPIRED) will be returned directly.
selfTradePreventionMode is only effective when timeInForce set to IOC or GTC or GTD.
In extreme market conditions, timeInForce GTD order auto cancel time might be delayed comparing to goodTillDate
Response Example​
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


---
## Account-Balance

Account Balance(USER_DATA)
API Description​

Query account balance

HTTP Request​

GET /papi/v1/balance

Request Weight​

20

Request Parameters​
Name	Type	Mandatory	Description
asset	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Response Example​
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


OR (when asset sent)

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


### Tables:
Name | Type | Mandatory | Description
asset | STRING | NO | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## Account-Information

Portfolio MarginAccountAccount Information
Account Information(USER_DATA)
API Description​

Query account information

HTTP Request​

GET /papi/v1/account

Request Weight​

20

Request Parameters​
Name	Type	Mandatory	Description
recvWindow	LONG	NO	
timestamp	LONG	YES	
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## UM-Position-Information

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## Change-Initial-Leverage

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## Change-Position-Mode

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## UM-Notional-and-Leverage-Brackets

Portfolio MarginAccountUm Notional And Leverage Brackets
UM Notional and Leverage Brackets (USER_DATA)
API Description​

Query UM notional and leverage brackets

HTTP Request​

GET /papi/v1/um/leverageBracket

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | NO | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## UM-Commission-Rate

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## New-UM-Order

New UM Order (TRADE)
API Description​

Place new UM order

HTTP Request​

POST /papi/v1/um/order

Request Weight(Order)​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
side	ENUM	YES	
positionSide	ENUM	NO	Default BOTH for One-way Mode ; LONG or SHORT for Hedge Mode. It must be sent in Hedge Mode.
type	ENUM	YES	LIMIT, MARKET
timeInForce	ENUM	NO	
quantity	DECIMAL	NO	
reduceOnly	STRING	NO	"true" or "false". default "false". Cannot be sent in Hedge Mode .
price	DECIMAL	NO	
newClientOrderId	STRING	NO	A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: ^[\.A-Z\:/a-z0-9_-]{1,32}$
newOrderRespType	ENUM	NO	ACK, RESULT, default ACK
priceMatch	ENUM	NO	only avaliable for LIMIT/STOP/TAKE_PROFIT order; can be set to OPPONENT/ OPPONENT_5/ OPPONENT_10/ OPPONENT_20: /QUEUE/ QUEUE_5/ QUEUE_10/ QUEUE_20; Can't be passed together with price
selfTradePreventionMode	ENUM	NO	NONE:No STP / EXPIRE_TAKER:expire taker order when STP triggers/ EXPIRE_MAKER:expire taker order when STP triggers/ EXPIRE_BOTH:expire both orders when STP triggers
goodTillDate	LONG	NO	order cancel time for timeInForce GTD, mandatory when timeInforce set to GTD; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode.
recvWindow	LONG	NO	
timestamp	LONG	YES	

Additional mandatory parameters based on type:

Type	Additional mandatory parameters
LIMIT	timeInForce, quantity, price
MARKET	quantity
If newOrderRespType is sent as RESULT :
MARKET order: the final FILLED result of the order will be return directly.
LIMIT order with special timeInForce: the final status result of the order(FILLED or EXPIRED) will be returned directly.
selfTradePreventionMode is only effective when timeInForce set to IOC or GTC or GTD.
In extreme market conditions, timeInForce GTD order auto cancel time might be delayed comparing to goodTillDate
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
side | ENUM | YES | 
positionSide | ENUM | NO | Default BOTH for One-way Mode ; LONG or SHORT for Hedge Mode. It must be sent in Hedge Mode.
type | ENUM | YES | LIMIT, MARKET
timeInForce | ENUM | NO | 
quantity | DECIMAL | NO | 
reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode .
price | DECIMAL | NO | 
newClientOrderId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: ^[\.A-Z\:/a-z0-9_-]{1,32}$
newOrderRespType | ENUM | NO | ACK, RESULT, default ACK
priceMatch | ENUM | NO | only avaliable for LIMIT/STOP/TAKE_PROFIT order; can be set to OPPONENT/ OPPONENT_5/ OPPONENT_10/ OPPONENT_20: /QUEUE/ QUEUE_5/ QUEUE_10/ QUEUE_20; Can't be passed together with price
selfTradePreventionMode | ENUM | NO | NONE:No STP / EXPIRE_TAKER:expire taker order when STP triggers/ EXPIRE_MAKER:expire taker order when STP triggers/ EXPIRE_BOTH:expire both orders when STP triggers
goodTillDate | LONG | NO | order cancel time for timeInForce GTD, mandatory when timeInforce set to GTD; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode.
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

Type | Additional mandatory parameters
LIMIT | timeInForce, quantity, price
MARKET | quantity

---
## Cancel-UM-Order

Portfolio MarginTradeCancel Um Order
Cancel UM Order(TRADE)
API Description​

Cancel an active UM LIMIT order

HTTP Request​

DELETE /papi/v1/um/order

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
orderId	LONG	NO	
origClientOrderId	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Either orderId or origClientOrderId must be sent.
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
orderId | LONG | NO | 
origClientOrderId | STRING | NO | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## Cancel-All-UM-Open-Orders

Portfolio MarginTradeCancel All Um Open Orders
Cancel All UM Open Orders(TRADE)
API Description​

Cancel all active LIMIT orders on specific symbol

HTTP Request​

DELETE /papi/v1/um/allOpenOrders

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Response Example​
{
    "code": 200, 
    "msg": "The operation of cancel all open order is done."
}


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## Query-UM-Order

Portfolio MarginTradeQuery Um Order
Query UM Order (USER_DATA)
API Description​

Check an UM order's status.

HTTP Request​

GET /papi/v1/um/order

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
orderId	LONG	NO	
origClientOrderId	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	

Notes:

These orders will not be found:
Either orderId or origClientOrderId must be sent.
order status is CANCELED or EXPIRED, AND
order has NO filled trade, AND
created time + 3 days < current time
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
orderId | LONG | NO | 
origClientOrderId | STRING | NO | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## Query-Current-UM-Open-Orders

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## Query-All-UM-Orders

Portfolio MarginTradeQuery All Um Orders
Query All UM Orders(USER_DATA)
API Description​

Get all account UM orders; active, canceled, or filled.

These orders will not be found:
order status is CANCELED or EXPIRED, AND
order has NO filled trade, AND
created time + 3 days < current time
HTTP Request​

GET /papi/v1/um/allOrders

Request Weight​

5

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
orderId	LONG	NO	
startTime	LONG	NO	
endTime	LONG	NO	
limit	INT	NO	Default 500; max 1000.
recvWindow	LONG	NO	
timestamp	LONG	YES	
If orderId is set, it will get orders >= that orderId. Otherwise most recent orders are returned.
The query time period must be less then 7 days( default as the recent 7 days).
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
orderId | LONG | NO | 
startTime | LONG | NO | 
endTime | LONG | NO | 
limit | INT | NO | Default 500; max 1000.
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## UM-Account-Trade-List

Portfolio MarginTradeUm Account Trade List
UM Account Trade List(USER_DATA)
API Description​

Get trades for a specific account and UM symbol.

HTTP Request​

GET /papi/v1/um/userTrades

Request Weight​

5

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
startTime	LONG	NO	
endTime	LONG	NO	
fromId	LONG	NO	Trade id to fetch from. Default gets most recent trades.
limit	INT	NO	Default 500; max 1000.
recvWindow	LONG	NO	
timestamp	LONG	YES	
If startTime and endTime are both not sent, then the last '7 days' data will be returned.
The time between startTime and endTime cannot be longer than 7 days.
The parameter fromId cannot be sent with startTime or endTime.
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
startTime | LONG | NO | 
endTime | LONG | NO | 
fromId | LONG | NO | Trade id to fetch from. Default gets most recent trades.
limit | INT | NO | Default 500; max 1000.
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## New-UM-Conditional-Order

Portfolio MarginTradeNew Um Conditional Order
New UM Conditional Order (TRADE)
API Description​

Place new UM conditional order

HTTP Request​

POST /papi/v1/um/conditional/order

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
side	ENUM	YES	
positionSide	ENUM	NO	Default BOTH for One-way Mode ; LONG or SHORT for Hedge Mode. It must be sent in Hedge Mode.
strategyType	ENUM	YES	"STOP", "STOP_MARKET", "TAKE_PROFIT", "TAKE_PROFIT_MARKET", and "TRAILING_STOP_MARKET"
timeInForce	ENUM	NO	
quantity	DECIMAL	NO	
reduceOnly	STRING	NO	"true" or "false". default "false". Cannot be sent in Hedge Mode ; cannot be sent with closePosition=true
price	DECIMAL	NO	
workingType	ENUM	NO	stopPrice triggered by: "MARK_PRICE", "CONTRACT_PRICE". Default "CONTRACT_PRICE"
priceProtect	STRING	NO	"TRUE" or "FALSE", default "FALSE". Used with STOP/STOP_MARKET or TAKE_PROFIT/TAKE_PROFIT_MARKET orders
newClientStrategyId	STRING	NO	A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: ^[\.A-Z\:/a-z0-9_-]{1,32}$
stopPrice	DECIMAL	NO	Used with STOP/STOP_MARKET or TAKE_PROFIT/TAKE_PROFIT_MARKET orders.
activationPrice	DECIMAL	NO	Used with TRAILING_STOP_MARKET orders, default as the mark price
callbackRate	DECIMAL	NO	Used with TRAILING_STOP_MARKET orders, min 0.1, max 5 where 1 for 1%
priceMatch	ENUM	NO	only avaliable for LIMIT/STOP/TAKE_PROFIT order; can be set to OPPONENT/ OPPONENT_5/ OPPONENT_10/ OPPONENT_20: /QUEUE/ QUEUE_5/ QUEUE_10/ QUEUE_20; Can't be passed together with price
selfTradePreventionMode	ENUM	NO	NONE:No STP / EXPIRE_TAKER:expire taker order when STP triggers/ EXPIRE_MAKER:expire taker order when STP triggers/ EXPIRE_BOTH:expire both orders when STP triggers
goodTillDate	LONG	NO	order cancel time for timeInForce GTD, mandatory when timeInforce set to GTD; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode.
recvWindow	LONG	NO	
timestamp	LONG	YES	

Additional mandatory parameters based on type:

Type	Additional mandatory parameters
STOP/TAKE_PROFIT	quantity, price, stopPrice
STOP_MARKET/TAKE_PROFIT_MARKET	stopPrice
TRAILING_STOP_MARKET	callbackRate

Order with type STOP/TAKE_PROFIT, parameter timeInForce can be sent ( default GTC).

Condition orders will be triggered when:

STOP, STOP_MARKET:
BUY: "MARK_PRICE" >= stopPrice
SELL: "MARK_PRICE" <= stopPrice
TAKE_PROFIT, TAKE_PROFIT_MARKET:
BUY: "MARK_PRICE" <= stopPrice
SELL: "MARK_PRICE" >= stopPrice
TRAILING_STOP_MARKET:
BUY: the lowest mark price after order placed <= activationPrice, and the latest mark price >= the lowest mark price * (1 + callbackRate)
SELL: the highest mark price after order placed >= activationPrice, and the latest mark price <= the highest mark price * (1 - callbackRate)

For TRAILING_STOP_MARKET, if you got such error code. {"code": -2021, "msg": "Order would immediately trigger."} means that the parameters you send do not meet the following requirements:

BUY: activationPrice should be smaller than latest mark price.
SELL: activationPrice should be larger than latest mark price.

Condition orders will be triggered when:

If parameterpriceProtectis sent as true:
when price reaches the stopPrice ，the difference rate between "MARK_PRICE" and "CONTRACT_PRICE" cannot be larger than the "triggerProtect" of the symbol
"triggerProtect" of a symbol can be got from GET /fapi/v1/exchangeInfo
STOP, STOP_MARKET:
BUY: latest price ("MARK_PRICE" or "CONTRACT_PRICE") >= stopPrice
SELL: latest price ("MARK_PRICE" or "CONTRACT_PRICE") <= stopPrice
TAKE_PROFIT, TAKE_PROFIT_MARKET:
BUY: latest price ("MARK_PRICE" or "CONTRACT_PRICE") <= stopPrice
SELL: latest price ("MARK_PRICE" or "CONTRACT_PRICE") >= stopPrice

selfTradePreventionMode is only effective when timeInForce set to IOC or GTC or GTD.

In extreme market conditions, timeInForce GTD order auto cancel time might be delayed comparing to goodTillDate

Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
side | ENUM | YES | 
positionSide | ENUM | NO | Default BOTH for One-way Mode ; LONG or SHORT for Hedge Mode. It must be sent in Hedge Mode.
strategyType | ENUM | YES | "STOP", "STOP_MARKET", "TAKE_PROFIT", "TAKE_PROFIT_MARKET", and "TRAILING_STOP_MARKET"
timeInForce | ENUM | NO | 
quantity | DECIMAL | NO | 
reduceOnly | STRING | NO | "true" or "false". default "false". Cannot be sent in Hedge Mode ; cannot be sent with closePosition=true
price | DECIMAL | NO | 
workingType | ENUM | NO | stopPrice triggered by: "MARK_PRICE", "CONTRACT_PRICE". Default "CONTRACT_PRICE"
priceProtect | STRING | NO | "TRUE" or "FALSE", default "FALSE". Used with STOP/STOP_MARKET or TAKE_PROFIT/TAKE_PROFIT_MARKET orders
newClientStrategyId | STRING | NO | A unique id among open orders. Automatically generated if not sent. Can only be string following the rule: ^[\.A-Z\:/a-z0-9_-]{1,32}$
stopPrice | DECIMAL | NO | Used with STOP/STOP_MARKET or TAKE_PROFIT/TAKE_PROFIT_MARKET orders.
activationPrice | DECIMAL | NO | Used with TRAILING_STOP_MARKET orders, default as the mark price
callbackRate | DECIMAL | NO | Used with TRAILING_STOP_MARKET orders, min 0.1, max 5 where 1 for 1%
priceMatch | ENUM | NO | only avaliable for LIMIT/STOP/TAKE_PROFIT order; can be set to OPPONENT/ OPPONENT_5/ OPPONENT_10/ OPPONENT_20: /QUEUE/ QUEUE_5/ QUEUE_10/ QUEUE_20; Can't be passed together with price
selfTradePreventionMode | ENUM | NO | NONE:No STP / EXPIRE_TAKER:expire taker order when STP triggers/ EXPIRE_MAKER:expire taker order when STP triggers/ EXPIRE_BOTH:expire both orders when STP triggers
goodTillDate | LONG | NO | order cancel time for timeInForce GTD, mandatory when timeInforce set to GTD; order the timestamp only retains second-level precision, ms part will be ignored; The goodTillDate timestamp must be greater than the current time plus 600 seconds and smaller than 253402300799000Mode. It must be sent in Hedge Mode.
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

Type | Additional mandatory parameters
STOP/TAKE_PROFIT | quantity, price, stopPrice
STOP_MARKET/TAKE_PROFIT_MARKET | stopPrice
TRAILING_STOP_MARKET | callbackRate

---
## Cancel-UM-Conditional-Order

Portfolio MarginTradeCancel Um Conditional Order
Cancel UM Conditional Order(TRADE)
API Description​

Cancel UM Conditional Order

HTTP Request​

DELETE /papi/v1/um/conditional/order

Request Weight​

1

Request Parameters​
Name	Type	Mandatory	Description
symbol	STRING	YES	
strategyId	LONG	NO	
newClientStrategyId	STRING	NO	
recvWindow	LONG	NO	
timestamp	LONG	YES	
Either strategyId or newClientStrategyId must be sent.
Response Example​
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


### Tables:
Name | Type | Mandatory | Description
symbol | STRING | YES | 
strategyId | LONG | NO | 
newClientStrategyId | STRING | NO | 
recvWindow | LONG | NO | 
timestamp | LONG | YES | 

---
## UM-Force-Orders

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.

---
## UM-Futures-Account-Balance-V2

Page Not Found

We could not find what you were looking for.

Please contact the owner of the site that linked you to the original URL and let them know their link is broken.
