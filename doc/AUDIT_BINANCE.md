# Binance Adapter Audit

**Date**: 2026-03-26
**Files audited**: `pkg/exchange/binance/adapter.go`, `client.go`, `ws.go`, `ws_private.go`
**Reference doc**: `doc/EXCHANGEAPI_BINANCE.md`

## Summary
- Methods audited: 25
- Issues found: 7 (0 critical, 3 medium, 4 low)

## Issues

### [MEDIUM] Method: fetchPositions (GetPosition / GetAllPositions)
- **File**: adapter.go:232
- **Problem**: Uses deprecated v2 endpoint for position data
- **Doc says**: `GET /fapi/v3/positionRisk` (weight 5) — v3 response includes `positionSide`, `breakEvenPrice`, `notional`, `initialMargin`, `maintMargin` but does NOT include `leverage` or `marginType` fields
- **Code does**: `GET /fapi/v2/positionRisk` — relies on v2-specific fields `leverage` and `marginType` in the response
- **Fix**: Cannot simply change to v3 — the v3 response drops `leverage` and `marginType` fields that the code depends on. Two options: (1) stay on v2 and accept deprecation risk, or (2) migrate to v3 and fetch leverage/marginType from `GET /fapi/v1/symbolConfig` separately. Option 1 is pragmatic until Binance announces v2 sunset.

### [MEDIUM] Method: GetFuturesBalance
- **File**: adapter.go:477
- **Problem**: Uses deprecated v2 endpoint for account data
- **Doc says**: `GET /fapi/v3/account` (weight 5)
- **Code does**: `GET /fapi/v2/account`
- **Fix**: Change to `/fapi/v3/account`. The v3 response has identical field names (`totalMarginBalance`, `totalMaintMargin`, `availableBalance`, `assets[].asset`, `assets[].walletBalance`, `assets[].availableBalance`), so this is a drop-in replacement. Lower risk than the position endpoint migration.

### [MEDIUM] Method: TransferToFutures / Withdraw (internal transfer)
- **File**: adapter.go:619, adapter.go:637
- **Problem**: Uses legacy transfer endpoint not documented in current API reference
- **Doc says**: Use `POST /sapi/v1/asset/transfer` with string type codes (`MAIN_UMFUTURE` for spot→futures, `UMFUTURE_MAIN` for futures→spot). Requires "Permits Universal Transfer" API key permission. Weight 900/UID.
- **Code does**: `POST /sapi/v1/futures/transfer` with numeric type codes (`"1"` for spot→futures, `"2"` for futures→spot)
- **Fix**: Migrate to `/sapi/v1/asset/transfer` with `type: "MAIN_UMFUTURE"` (line 619) and `type: "UMFUTURE_MAIN"` (line 637). Ensure API key has Universal Transfer permission enabled.

### [LOW] Method: LoadAllContracts
- **File**: adapter.go:346-349
- **Problem**: Does not filter by `contractType` — loads all TRADING symbols including quarterly/delivery contracts
- **Doc says**: Response includes `contractType` field with values like `PERPETUAL`, `CURRENT_QUARTER`, etc.
- **Code does**: Only filters `status == "TRADING"`, not `contractType == "PERPETUAL"`
- **Fix**: Add filter `if sym.ContractType != "PERPETUAL" { continue }`. Currently harmless since non-perpetual symbols won't match Loris API data, but adds unnecessary entries to the contract map.

### [LOW] Method: createListenKey / keepaliveListenKey
- **File**: ws_private.go:142-158
- **Problem**: Sends unnecessary signature on USER_STREAM endpoints
- **Doc says**: `POST /fapi/v1/listenKey` and `PUT /fapi/v1/listenKey` have security type USER_STREAM — requires API Key header only, no signature needed
- **Code does**: Uses `client.Post()` and `client.Put()` which add `timestamp` and `signature` via `buildQuery()`
- **Fix**: No fix needed — Binance accepts and ignores extra parameters. The extra signature is harmless but wastes minimal compute. Could add `client.PostUnsigned()` for cleanliness but not worth the effort.

### [LOW] Method: PlaceStopLoss / CancelStopLoss
- **File**: adapter.go:888-925
- **Problem**: Uses `/fapi/v1/algoOrder` endpoint not documented in local API reference
- **Doc says**: N/A — endpoint not in `EXCHANGEAPI_BINANCE.md`
- **Code does**: `POST /fapi/v1/algoOrder` with `algoType: "CONDITIONAL"` and `DELETE /fapi/v1/algoOrder` with `algoId`. Code comment explains this was required since 2025-12-09.
- **Fix**: Add `/fapi/v1/algoOrder` documentation to `EXCHANGEAPI_BINANCE.md`. The endpoint is valid per Binance's algo order API. No code change needed.

### [LOW] Cosmetic: Depth stream comment
- **File**: ws.go:73
- **Problem**: Comment says "top-5 orderbook" but code subscribes to `@depth20@100ms` (top 20 levels)
- **Doc says**: Stream `<symbol>@depth<levels>@100ms` with levels 5, 10, 20
- **Code does**: Subscribes to depth20 (20 levels), comment says "top-5"
- **Fix**: Update comment from "top-5 orderbook" to "top-20 orderbook"

## Verified OK

### REST Endpoints
- **PlaceOrder** — `POST /fapi/v1/order`: correct endpoint, params (`symbol`, `side`, `type`, `quantity`, `price`, `timeInForce`, `reduceOnly`, `newClientOrderId`), response parsing (`orderId` as int64, `clientOrderId`). Side/type/TIF enum mappings all correct (BUY/SELL, LIMIT/MARKET, GTC/IOC/FOK/GTX).
- **CancelOrder** — `DELETE /fapi/v1/order`: correct params (`symbol`, `orderId`), error code -2011 (CANCEL_REJECTED) properly suppressed.
- **GetPendingOrders** — `GET /fapi/v1/openOrders`: correct params and response field names (`orderId`, `clientOrderId`, `origQty`, `status`).
- **GetOrderFilledQty** — `GET /fapi/v1/order`: correct params (`symbol`, `orderId`), response field `executedQty` correctly parsed.
- **SetLeverage** — `POST /fapi/v1/leverage`: correct params (`symbol`, `leverage`).
- **SetMarginMode** — `POST /fapi/v1/marginType`: correct params (`symbol`, `marginType` as ISOLATED/CROSSED), error code -4046 handled.
- **GetFundingRate** — `GET /fapi/v1/premiumIndex`: correct params, response fields (`lastFundingRate`, `nextFundingTime` as epoch ms). Funding interval fetched from `GET /fapi/v1/fundingInfo` with correct fields (`fundingIntervalHours`, `adjustedFundingRateCap`, `adjustedFundingRateFloor`).
- **GetOrderbook** — `GET /fapi/v1/depth`: correct params (`symbol`, `limit`), response parsing of string arrays for bids/asks.
- **GetSpotBalance** — `GET /sapi/v1/capital/config/getall` via SpotGet: correct response fields (`coin`, `free`, `locked`).
- **GetUserTrades** — `GET /fapi/v1/userTrades`: correct params and response fields (`id`, `orderId`, `price`, `qty`, `commission`, `commissionAsset`, `time`).
- **GetFundingFees** — `GET /fapi/v1/income` with `incomeType: "FUNDING_FEE"`: correct params and response parsing.
- **GetClosePnL** — `GET /fapi/v1/income` for REALIZED_PNL, COMMISSION, FUNDING_FEE: correct aggregation approach.
- **EnsureOneWayMode** — `POST /fapi/v1/positionSide/dual` with `dualSidePosition: "false"`: correct. Error codes -4059 (NO_NEED_TO_CHANGE_POSITION_SIDE) properly handled.
- **CheckPermissions** — `GET /sapi/v1/account/apiRestrictions` on `api.binance.com`: correct spot endpoint, response fields match.
- **Withdraw** — `POST /sapi/v1/capital/withdraw/apply` via SpotPost: correct params (`coin`, `network`, `address`, `amount`), response field `id`.

### WebSocket
- **Price stream (bookTicker)** — Stream name `<symbol_lower>@bookTicker` correct. Combined stream URL format `wss://fstream.binance.com/stream?streams=...` correct. Subscribe/unsubscribe JSON format correct. Payload fields `s`, `b`, `B`, `a`, `A` all match docs.
- **Depth stream** — Stream name `<symbol_lower>@depth20@100ms` correct per docs (levels 5/10/20, speeds 250ms/500ms/100ms). Payload fields `b`/`a` for partial depth correct.
- **Private stream** — ListenKey create (`POST /fapi/v1/listenKey`), keepalive (`PUT /fapi/v1/listenKey`) correct. 30-min keepalive interval is well within the 60-min expiry. WebSocket URL `wss://fstream.binance.com/ws/<listenKey>` correct.
- **ORDER_TRADE_UPDATE parsing** — All field mappings correct: `o.s` (symbol), `o.c` (clientOrderId), `o.S` (side), `o.o` (orderType), `o.X` (orderStatus), `o.i` (orderId), `o.ap` (avgPrice), `o.z` (filledQty), `o.R` (reduceOnly), `o.q` (origQty).

### Authentication (client.go)
- **HMAC-SHA256 signing** — correct algorithm, correct key usage.
- **X-MBX-APIKEY header** — set on all requests.
- **Timestamp** — millisecond epoch, appended before signing.
- **Signature** — appended after query string, not included in signed payload.
- **GET/DELETE** — params in query string. **POST/PUT** — params in form-encoded body. Both correct per Binance API.
- **Error handling** — HTTP 4xx/5xx checked, negative error codes in 200 responses detected.

### Enum Mappings
- **Side**: BUY/SELL — matches docs.
- **Order type**: LIMIT/MARKET — matches docs.
- **Time in force**: GTC/IOC/FOK/GTX — matches docs. GTX correctly mapped from "post_only".
- **Margin type**: ISOLATED/CROSSED — matches docs.
- **Position side**: "false" for one-way mode — matches docs.
