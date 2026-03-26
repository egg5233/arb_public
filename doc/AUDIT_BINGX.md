# BingX Adapter Audit Report

**Date**: 2026-03-26
**Auditor**: Claude (adapter-audit team)
**Files Reviewed**:
- `doc/EXCHANGEAPI_BINGX.md` (authoritative API reference)
- `pkg/exchange/bingx/adapter.go` (1048 lines)
- `pkg/exchange/bingx/client.go` (248 lines)
- `pkg/exchange/bingx/ws.go` (354 lines)
- `pkg/exchange/bingx/ws_private.go` (288 lines)

---

## Summary

| Category | Status | Issues |
|---|---|---|
| Symbol format | PASS | Correct BTC-USDT mapping |
| Endpoint URLs | PASS | All endpoints match docs |
| Request parameters | WARN | 2 issues (clientOrderId casing, leverage side) |
| Response parsing | WARN | 1 issue (clientOrderID casing in open orders) |
| Authentication | WARN | URL-encoding before signing (latent bug) |
| HTTP methods | PASS | DELETE for cancel, correct methods throughout |
| WebSocket | PASS | GZIP + text Ping/Pong handled correctly |
| Listen key management | PASS | Correct endpoints and keepalive interval |
| Order types | PASS | Correct enum values |
| Position/margin mode | PASS | One-way mode with BOTH, CROSSED/ISOLATED |

**Critical issues**: 0
**Warnings**: 4
**Info/Minor**: 3

---

## Detailed Findings

### 1. Symbol Format — PASS

- **Doc**: `BTC-USDT` (hyphen-separated, uppercase)
- **Code**: `toBingXSymbol()` correctly converts `BTCUSDT` -> `BTC-USDT`; `fromBingXSymbol()` reverses via `strings.ReplaceAll("-", "")`
- No issues found.

### 2. Endpoint URLs — PASS

All endpoints match the documented paths:

| Function | Endpoint | Doc Match |
|---|---|---|
| PlaceOrder | `POST /openApi/swap/v2/trade/order` | line 355 |
| CancelOrder | `DELETE /openApi/swap/v2/trade/order` | line 396 |
| GetPendingOrders | `GET /openApi/swap/v2/trade/openOrders` | line 433 |
| GetOrderFilledQty | `GET /openApi/swap/v2/trade/order` | line 456 |
| SetLeverage | `POST /openApi/swap/v2/trade/leverage` | line 491 |
| SetMarginMode | `POST /openApi/swap/v2/trade/marginType` | line 479 |
| LoadAllContracts | `GET /openApi/swap/v2/quote/contracts` | line 114 |
| GetFundingRate | `GET /openApi/swap/v2/quote/premiumIndex` | line 222 |
| GetFuturesBalance | `GET /openApi/swap/v3/user/balance` | line 521 |
| GetPosition(s) | `GET /openApi/swap/v2/user/positions` | line 562 |
| GetFundingFees | `GET /openApi/swap/v2/user/income` | line 617 |
| GetSpotBalance | `GET /openApi/spot/v1/account/balance` | line 655 |
| GetOrderbook | `GET /openApi/swap/v2/quote/depth` | line 182 |
| Listen key (create) | `POST /openApi/user/auth/userDataStream` | line 882 |
| Listen key (extend) | `PUT /openApi/user/auth/userDataStream` | line 883 |
| Listen key (close) | `DELETE /openApi/user/auth/userDataStream` | line 884 |
| Transfer | `POST /openApi/wallets/v1/capital/innerTransfer/apply` | line 704 |

### 3. Request Parameters — WARN

#### WARN-01: `clientOrderID` casing mismatch (adapter.go:174)

- **Code**: `params["clientOrderID"] = req.ClientOid` (uppercase `D`)
- **Doc** (line 375): parameter name is `clientOrderId` (lowercase `d`)
- **Impact**: If BingX is case-sensitive on parameter names, client order IDs would be silently ignored on order placement. Since the bot appears to work in production, BingX may be case-insensitive here, but this deviates from the documented API.
- **Fix**: Change `"clientOrderID"` to `"clientOrderId"`

#### WARN-02: SetLeverage uses `side=BOTH` (adapter.go:389)

- **Code**: `sides := []string{"BOTH"}`
- **Doc** (line 499): side parameter accepts `LONG` or `SHORT` only
- **Impact**: In one-way mode, BingX may accept `BOTH` as an undocumented value. If it doesn't, leverage setting silently fails or errors out. Consider setting both `LONG` and `SHORT` separately for correctness.
- **Production status**: Appears to work, so likely accepted.

### 4. Response Parsing — WARN

#### WARN-03: `clientOrderID` JSON tag mismatch in GetPendingOrders (adapter.go:225)

- **Code**: `ClientOrderID string \`json:"clientOrderID"\``
- **Doc** (line 444): response field is `clientOrderId` (lowercase `d`)
- **Impact**: Go's JSON decoder with explicit tags is **case-sensitive**. If the API returns `clientOrderId`, the field tagged `clientOrderID` will never be populated. All client order IDs from `GetPendingOrders` would be empty strings.
- **Fix**: Change JSON tag to `json:"clientOrderId"`

### 5. Authentication — WARN

#### WARN-04: URL-encoding applied before signature computation (client.go:85)

- **Code**: `parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))`
- **Doc** (line 34,38): "Build signing string: `key=value&key2=value2` (values NOT URL-encoded)" and "Signature is always calculated from the **unencoded** signing string"
- **Impact**: For current parameter values (alphanumeric strings, numbers), `url.QueryEscape` is a no-op, so this works. But if any parameter value ever contains special characters (`[`, `{`, spaces, `+`), the signature would be computed on the encoded form while BingX expects the unencoded form, causing signature failures.
- **Severity**: Latent bug — no impact with current parameter values, but could break if parameters with special characters are added.
- **Fix**: Build the signing string from raw values, then URL-encode separately for the request URL.

### 6. HTTP Methods — PASS

- Cancel order uses `DELETE` — correct per doc line 396
- Place order uses `POST` — correct
- All other methods match documentation

### 7. WebSocket — PASS

- **GZIP compression**: Handled correctly via `gzipDecompress()` with fallback to plain text (ws.go:164-168)
- **Ping/Pong**: Text-based — server sends `"Ping"`, client replies `"Pong"` (ws.go:171-179) — correct per doc line 728,1019
- **Subscription format**: Uses `{id, reqType, dataType}` — correct per doc line 734-739
- **WebSocket URL**: `wss://open-api-swap.bingx.com/swap-market` — correct per doc line 7
- **Book ticker parsing**: Correctly reads `b` (bid) and `a` (ask) fields from `data` — matches doc line 858-860
- **Depth parsing**: Correctly reads `bids`/`asks` arrays — matches doc line 759-761

### 8. Listen Key Management — PASS

- **Create**: `POST /openApi/user/auth/userDataStream` — correct
- **Extend**: `PUT /openApi/user/auth/userDataStream` with `listenKey` param — correct
- **Close**: `DELETE /openApi/user/auth/userDataStream` with `listenKey` param — correct
- **Extend interval**: 30 minutes (`listenKeyExtendInt`) — matches doc recommendation (line 886)
- **Private WS URL**: Appends `?listenKey=` — correct per doc line 873

### 9. Order Types — PASS

- `LIMIT`, `MARKET` — correct (adapter.go:143-152)
- `STOP_MARKET` — correct for stop-loss (adapter.go:828)
- Time-in-force: `GTC`, `IOC`, `FOK`, `PostOnly` — all match doc line 1034
- Order status normalization handles both BingX formats (`NEW`/`Pending`, `FILLED`/`Filled`, etc.) — correct

### 10. Position/Margin Mode — PASS

- **One-way mode**: Uses `positionSide=BOTH` on all orders — correct for one-way mode
- **SetMarginMode**: Maps `cross` -> `CROSSED`, `isolated` -> `ISOLATED` — correct per doc line 482
- **EnsureOneWayMode**: No-op (position mode set via UI) — acceptable
- **Note**: Doc line 504 shows the endpoint to programmatically set position mode (`POST /openApi/swap/v1/positionSide/dual`), but the adapter doesn't implement this. Since it's a one-time setup, this is acceptable.

---

## Minor / Informational

### INFO-01: GetFundingInterval returns hardcoded 8h (adapter.go:551)

- The `GetFundingInterval` method returns a hardcoded `8 * time.Hour`, but BingX supports 1h, 2h, 4h, 8h intervals per contract.
- `GetFundingRate` already parses `fundingIntervalHours` and returns the correct interval in the `FundingRate` struct, so callers using that path get the right value.
- **Impact**: Low — only affects callers of `GetFundingInterval` specifically, which may not be used if `GetFundingRate` provides the interval.

### INFO-02: Private WS proactive "Pong" in pingLoop (ws_private.go:143-163)

- The private WS has a `pingLoop` that sends `"Pong"` every 10 minutes unprompted.
- The `readLoop` already handles server-initiated `Ping` -> `Pong` reactively.
- Sending unsolicited `"Pong"` messages is unnecessary. The server will just ignore them, but it's technically wrong (should be `"Ping"` if the intent is a keepalive).
- **Impact**: None — server ignores unsolicited Pongs. But if the intent was client-initiated keepalive, it should send `"Ping"`.

### INFO-03: Transfer endpoint parameter names (adapter.go:626-651)

- Code uses `transferAccountType` and `targetAccountType` parameters
- Doc for `/openApi/wallets/v1/capital/innerTransfer/apply` (line 707) lists different parameter names: `userAccountType`, `userAccount`, `walletType`
- The doc's `innerTransfer/apply` is described as "Main Account Internal Transfer" (for transfers to sub-accounts), while the code uses it for self-transfers between accounts
- The standard self-transfer endpoint per doc is `/openApi/api/v3/asset/transfer` (line 675) with `type` enum values like `FUND_SFUTURES`, `SFUTURES_FUND`
- **Impact**: Since transfers work in production, BingX likely accepts these undocumented parameters. But this deviates from the documented API and could break if BingX removes undocumented parameter support.

---

## Positive Observations

1. **Retry logic**: Client has exponential backoff retry with configurable retries and transient error detection (client.go:221-247)
2. **Param copy on retry**: Each retry attempt copies the param map to avoid timestamp mutation issues (client.go:226-229)
3. **Idempotent cancels**: CancelOrder handles 80018 (already filled) and 80016 (not found) gracefully (adapter.go:202-205)
4. **Idempotent leverage/margin**: SetLeverage and SetMarginMode handle "already set" errors (adapter.go:400-404, 427-429)
5. **Negative position size**: Correctly handles BingX one-way mode where short positions have negative `positionAmt` (adapter.go:344-349)
6. **Contract filtering**: LoadAllContracts filters by `status == 1` (online) and USDT suffix (adapter.go:461-468)
7. **Funding rate caps**: GetFundingRate correctly parses `minFundingRate` and `maxFundingRate` (adapter.go:532-541)

---

## Recommended Fixes (Priority Order)

1. **WARN-03** (High): Fix `clientOrderID` JSON tag to `clientOrderId` in GetPendingOrders response struct — client order IDs are currently not parsed
2. **WARN-01** (Medium): Fix `clientOrderID` request parameter to `clientOrderId` in PlaceOrder — ensure client order IDs are sent correctly
3. **WARN-04** (Low): Separate signing string construction from URL encoding in `buildParamString` — prevent future signature failures
4. **WARN-02** (Low): Consider using `LONG`/`SHORT` instead of `BOTH` for SetLeverage, or verify `BOTH` is accepted
