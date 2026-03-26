# Bitget Adapter Audit Report

**Date**: 2026-03-26
**Auditor**: Claude (adapter-audit team)
**Files Reviewed**:
- `pkg/exchange/bitget/adapter.go` (1181 lines)
- `pkg/exchange/bitget/client.go` (219 lines)
- `pkg/exchange/bitget/ws.go` (346 lines)
- `pkg/exchange/bitget/ws_private.go` (225 lines)
- `doc/EXCHANGEAPI_BITGET.md` (authoritative API reference)

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 1 |
| HIGH     | 2 |
| MEDIUM   | 4 |
| LOW      | 2 |
| OK       | 10+ |

---

## CRITICAL Issues

### C1. Position field name mismatch: `averageOpenPrice` vs `openPriceAvg`

**File**: `adapter.go:261`
**Impact**: Entry price for all positions may parse as zero/empty.

The adapter expects `averageOpenPrice`:
```go
AverageOpenPrice string `json:"averageOpenPrice"`
```

But the API documentation (and response examples) consistently returns `openPriceAvg`:
```json
"openPriceAvg": "88505.23"
```

This mismatch means `position.AverageOpenPrice` would always be empty string, causing entry price to be zero. This affects PnL calculations, stop-loss placement, and dashboard display.

**Action**: Verify against live API response. If the API truly returns `openPriceAvg`, change the JSON tag to `json:"openPriceAvg"`.

---

## HIGH Issues

### H1. `reduceOnly` parameter uses wrong case

**File**: `adapter.go:94-95`
**Impact**: Reduce-only orders may not be recognized as such, risking unintended position increases.

Code sends:
```go
params["reduceOnly"] = "yes"
```

API documentation specifies uppercase values:
```
reduceOnly: "YES" or "NO" (default). One-way mode only.
```

**Action**: Change `"yes"` to `"YES"`.

### H2. `GetFundingFees` uses wrong `businessType` value

**File**: `adapter.go:982`
**Impact**: Funding fee queries may return empty results or wrong data.

Code sends:
```go
"businessType": "funding_fee",
```

API documentation specifies:
```
contract_settle_fee (funding fee)
```

The documented value for funding fees in the account bill endpoint is `contract_settle_fee`, not `funding_fee`.

**Action**: Change to `"contract_settle_fee"` and verify results match expected funding fee entries.

---

## MEDIUM Issues

### M1. `LoadAllContracts` uses lowercase `productType`

**File**: `adapter.go:365`
**Impact**: Inconsistency; may work if API is case-insensitive, but fragile.

```go
"productType": "usdt-futures",  // lowercase
```

All other methods use the constant `productTypeUSDTFutures = "USDT-FUTURES"` (uppercase). The API docs show uppercase `USDT-FUTURES` as the canonical value.

**Action**: Use the constant `productTypeUSDTFutures` for consistency.

### M2. Contract config field name mismatch: `fundingInterval` vs `fundInterval`

**File**: `adapter.go:529`
**Impact**: `GetFundingInterval()` always falls back to 8h default from contracts endpoint.

Code expects:
```go
FundingInterval string `json:"fundingInterval"`
```

API documentation shows:
```json
"fundInterval": "8"
```

The field in the contracts endpoint is `fundInterval` (without "ing"). This means `GetFundingInterval()` can never parse the value from the contracts API, always returning the 8h default.

**Note**: Low practical impact because `GetFundingRate()` gets interval from `fundingRateInterval` (correct field from the funding rate endpoint), and only falls back to `GetFundingInterval()` if that fails.

**Action**: Change JSON tag to `json:"fundInterval"`.

### M3. `isOneWayMode` missing required `marginCoin` parameter

**File**: `adapter.go:1164`
**Impact**: API call may fail, causing `isOneWayMode()` to return false incorrectly.

```go
raw, err := a.client.Get("/api/v2/mix/account/account", map[string]string{
    "productType": "USDT-FUTURES",
    "symbol":      "BTCUSDT",
})
```

The API docs specify `marginCoin` as **required** for `GET /api/v2/mix/account/account`:
```
| marginCoin | String | Yes | Margin coin, e.g. USDT |
```

**Action**: Add `"marginCoin": "USDT"` to the params.

### M4. `GetUserTrades` uses undocumented endpoint

**File**: `adapter.go:919`
**Impact**: Endpoint may change without notice; no documented contract.

Code uses:
```go
"/api/v2/mix/order/fill-history"
```

The documented fill endpoint is:
```
GET /api/v2/mix/order/fills
```

`fill-history` is not in the local API docs. It may be a valid endpoint but there's no documented contract for it.

**Action**: Verify if `/api/v2/mix/order/fills` would serve the same purpose. If `fill-history` is intentionally used for a different scope (e.g., longer history window), add it to the local docs.

---

## LOW Issues

### L1. Ping interval more aggressive than documented

**File**: `ws.go:332`
**Impact**: None (conservative is fine, may use slightly more bandwidth).

Code pings every 20 seconds. Docs specify every 30 seconds. This is safe (more conservative), not a bug.

### L2. PlaceOrder lacks `tradeSide` for hedge-mode compatibility

**File**: `adapter.go:74-96`
**Impact**: None currently, since bot enforces one-way mode via `EnsureOneWayMode()`.

The `PlaceOrder` method never sets `tradeSide` (`open`/`close`), which is required in hedge-mode. Since the bot calls `EnsureOneWayMode()` before trading, this isn't a live issue but would break if position mode changes.

---

## Verified Correct

| Area | Detail | Status |
|------|--------|--------|
| **Response wrapper** | All methods check `code == "00000"` | OK |
| **productType** (general) | Uses constant `"USDT-FUTURES"` for most endpoints | OK |
| **Endpoint URLs** | All `/api/v2/mix/` and `/api/v2/spot/` paths match docs | OK |
| **Authentication headers** | `ACCESS-KEY`, `ACCESS-SIGN`, `ACCESS-TIMESTAMP`, `ACCESS-PASSPHRASE` | OK |
| **REST timestamp** | `time.Now().UnixMilli()` (milliseconds) | OK |
| **WS login timestamp** | `time.Now().Unix()` (seconds) | OK |
| **HMAC-SHA256 sign** | `timestamp + method + requestPath + body`, Base64 encoded | OK |
| **GET sign includes query** | requestPath constructed with `?queryString` before signing | OK |
| **WebSocket URLs** | `wss://ws.bitget.com/v2/ws/public` and `/private` | OK |
| **WS subscribe format** | `{op: "subscribe", args: [{instType, channel, instId}]}` | OK |
| **WS login format** | `{op: "login", args: [{apiKey, passphrase, timestamp, sign}]}` | OK |
| **WS login sign** | `timestamp + "GET" + "/user/verify"` | OK |
| **feeDetail parsing** | Correctly parsed as array, accesses `[0].TotalFee` | OK |
| **Enum: side** | `buy`, `sell` | OK |
| **Enum: orderType** | `limit`, `market` | OK |
| **Enum: force** | `ioc`, `fok`, `gtc`, `post_only` | OK |
| **Enum: marginMode** | `isolated`, `crossed` (maps "cross" -> "crossed") | OK |
| **Enum: posMode** | `one_way_mode`, `hedge_mode` | OK |
| **Cancel idempotency** | Accepts codes 43011, 43025 as success | OK |
| **SetMarginMode idempotency** | Accepts code 40872 (already set) | OK |
| **Retry logic** | Exponential backoff on 429 and 40900 | OK |
| **Transfer account types** | `usdt_futures`, `spot` | OK |
| **Orderbook depth** | Parses `[price, size]` arrays correctly | OK |
| **WS private order channel** | Subscribes to `orders` with `instId: "default"` | OK |
| **WS order status parsing** | Uses `accBaseVolume` and `priceAvg` correctly | OK |

---

## Recommendations

1. **Immediate**: Fix C1 (position field name) - verify against live response, then fix JSON tag
2. **Immediate**: Fix H1 (reduceOnly case) - change to `"YES"`
3. **Immediate**: Fix H2 (businessType) - change to `"contract_settle_fee"`
4. **Soon**: Fix M1-M4 (consistency and missing params)
5. **Document**: Add `/api/v2/mix/order/fill-history` to local API docs if it's intentionally used
