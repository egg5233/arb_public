# Gate.io Adapter Audit Report

> **Date**: 2026-03-26
> **Auditor**: Adapter Audit Agent
> **Files reviewed**:
> - `pkg/exchange/gateio/adapter.go` (1276 lines)
> - `pkg/exchange/gateio/client.go` (246 lines)
> - `pkg/exchange/gateio/ws.go` (346 lines)
> - `pkg/exchange/gateio/ws_private.go` (256 lines)
> - `pkg/exchange/gateio/adapter_test.go` (155 lines)
> - `doc/EXCHANGEAPI_GATEIO.md` (API reference)

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 2 |
| HIGH     | 2 |
| MEDIUM   | 3 |
| LOW      | 4 |

---

## CRITICAL Issues

### C1. Margin Mode Detection is Inverted

**File**: `adapter.go:314-318` (also `380-384`)

**Problem**: The code checks `cross_leverage_limit > 0` to determine isolated mode, but per the Gate.io API, `cross_leverage_limit > 0` means **cross margin with a leverage cap**. The `leverage` field determines the mode: `"0"` = cross, `> 0` = isolated.

```go
// CURRENT (WRONG):
crossLev, _ := raw.CrossLeverageLimit.Int64()
marginMode := "crossed"
if crossLev > 0 {
    marginMode = "isolated"  // BUG: this is still cross mode!
}
```

**Why it matters**: The bot's own `SetLeverage()` sets `leverage=0, cross_leverage_limit=<value>` for cross mode. So every Gate.io position with a cross leverage limit is **incorrectly reported as "isolated"** when it's actually "crossed". This affects risk management, margin mode validation, and dashboard display.

**Fix**: Check the `leverage` field instead, or use `pos_margin_mode` from the response (which Gate.io provides directly):
```go
// Option A: Check leverage field
lev, _ := strconv.ParseFloat(raw.Leverage, 64)
marginMode := "crossed"
if lev > 0 {
    marginMode = "isolated"
}

// Option B: Use pos_margin_mode from response (most reliable)
// Add to struct: PosMarginMode string `json:"pos_margin_mode"`
```

---

### C2. MinSize/MaxSize in Wrong Units (Contracts vs Base Asset)

**File**: `adapter.go:505-507`

**Problem**: `MinSize` and `MaxSize` are set to raw contract counts, but the engine expects **base asset units**. All other size fields (StepSize, position sizes, order sizes) are properly converted to base units via `quanto_multiplier`.

```go
// CURRENT (WRONG):
MinSize:  float64(c.OrderSizeMin),  // e.g., 1 (contract)
MaxSize:  float64(c.OrderSizeMax),  // e.g., 1000000 (contracts)
StepSize: quantoMult,                // e.g., 0.0001 (base units) ← correct!
```

**Why it matters**: The risk manager at `internal/risk/manager.go:305` rejects orders where `sizeInBase < info.MinSize`. For BTC_USDT (`quanto_multiplier=0.0001`):
- Actual minimum: 1 contract × 0.0001 = **0.0001 BTC**
- Reported MinSize: **1** (treated as 1 BTC by the engine)
- Result: Any BTC order below 1 BTC (~$90,000) is **incorrectly rejected**

Same issue at `internal/engine/engine.go:1924`.

**Fix**:
```go
MinSize: float64(c.OrderSizeMin) * quantoMult,  // base asset units
MaxSize: float64(c.OrderSizeMax) * quantoMult,  // base asset units
```

---

## HIGH Issues

### H1. order_size_min/max Parsed as int64, API Returns Strings

**File**: `adapter.go:477-478`

**Problem**: The API docs specify `order_size_min` and `order_size_max` as **string type** (e.g., `"1"`, `"1000000"`). The Go struct uses `int64`, which would silently parse as `0` for JSON strings.

```go
// CURRENT:
OrderSizeMin int64  `json:"order_size_min"`  // Would be 0 if API sends "1"
OrderSizeMax int64  `json:"order_size_max"`  // Would be 0 if API sends "1000000"
```

**Impact**: If Gate.io sends these as strings, both values silently become 0, meaning MinSize=0 (no minimum check) and MaxSize=0. This interacts with C2 above.

**Note**: The system works in production, so Gate.io may currently send these as numbers. But API behavior can change.

**Fix**: Use `json.Number` or `string` and parse explicitly:
```go
OrderSizeMin string `json:"order_size_min"`
OrderSizeMax string `json:"order_size_max"`
// Then: sizeMin, _ := strconv.ParseInt(c.OrderSizeMin, 10, 64)
```

---

### H2. IOC Partial Fills Don't Trigger Order Callback

**File**: `ws_private.go:219-241`

**Problem**: The `onFill` callback is only invoked when `status == "filled"`. Gate.io IOC orders that **partially fill** have `finish_as: "ioc"` (not `"filled"`), so the code sets `status = "finished"` (pass-through from `o.Status`), and the callback is **never triggered**.

```go
// Status mapping only handles "filled" and "cancelled":
if o.FinishAs == "filled" {
    status = "filled"
} else if o.FinishAs == "cancelled" {
    status = "cancelled"
}
// "ioc" finish_as → status remains "finished" → onFill NOT called

// Callback guard:
if upd.Status == "filled" && upd.FilledVolume > 0 ... {
    (*ws.onFill)(upd)  // Never reached for IOC partial fills
}
```

**Impact**: The bot uses IOC orders for execution. Partially filled IOC orders (~common in volatile markets) don't trigger the fast WS notification. The system falls back to REST polling (`GetOrderFilledQty`), which works but adds latency.

**Fix**: Map `finish_as: "ioc"` with `filledVol > 0` to `status = "filled"`:
```go
if o.FinishAs == "filled" || (o.FinishAs == "ioc" && filledVol > 0) {
    status = "filled"
} else if o.FinishAs == "cancelled" {
    status = "cancelled"
}
```

---

## MEDIUM Issues

### M1. EnsureOneWayMode Uses Query Param Instead of JSON Body

**File**: `adapter.go:1264`

**Problem**: API docs specify `POST /futures/{settle}/dual_mode` with JSON body `{"dual_mode": false}`. The code sends `dual_mode=false` as a **query parameter** with an empty body.

```go
// CURRENT:
a.client.Post("/futures/usdt/dual_mode?dual_mode=false", "")

// EXPECTED per docs:
a.client.Post("/futures/usdt/dual_mode", `{"dual_mode":false}`)
```

**Impact**: May work in practice (Gate.io seems to accept both), but relies on undocumented behavior. Error handling catches common failure modes.

---

### M2. WebSocket Depth Subscribe Format Incorrect

**File**: `ws.go:280-291`

**Problem**: The `futures.order_book` subscribe message uses non-standard top-level fields (`accuracy`, `limit`) instead of embedding parameters in the `payload` array as documented.

```go
// CURRENT:
"payload":  []string{symbol},
"accuracy": "0",
"limit":    5,

// EXPECTED per docs:
"payload": []string{symbol, "5", "0"}  // [contract, level, interval_ms]
```

**Impact**: May work if Gate.io accepts both formats, but the payload-array format is the documented approach.

---

### M3. PlaceOrder Sends Size as int64, Docs Specify String

**File**: `adapter.go:151`

**Problem**: Since API v4.106.0, all size fields are **string type**. The code sends `size` as a JSON integer.

```go
// CURRENT:
"size": size,  // int64 → JSON number: 88

// RECOMMENDED:
"size": strconv.FormatInt(size, 10),  // string: "88"
```

**Impact**: Currently works (Gate.io accepts both), but future API changes could break this.

---

## LOW Issues

### L1. book_ticker WS Uses Field "s" Instead of "contract"

**File**: `ws.go:181`

**Problem**: API docs show `"contract": "BTC_USDT"` in book_ticker result, but the code reads `"s"`. This works in production, suggesting Gate.io's actual WS response uses abbreviated field names not fully documented.

**Risk**: Low — the code works. Just a documentation vs. implementation discrepancy to be aware of.

---

### L2. TransferToFutures Sends Extra "settle" Field

**File**: `adapter.go:846`

**Problem**: The transfer body includes `"settle": "usdt"` which is not in the `/wallet/transfers` API spec. The documented fields are: `currency`, `from`, `to`, `amount`.

**Impact**: Likely silently ignored by Gate.io. Harmless but unnecessary.

---

### L3. Size Fields in Various Responses Parsed as int64

**Files**: `adapter.go:210,253,289` and `ws_private.go:188-189`

**Problem**: Multiple response parsers use `int64` for size/left fields. Gate.io API v4.106.0+ notes these as string type, and decimal sizes are supported with `X-Gate-Size-Decimal: 1` header.

**Impact**: Currently works for integer-only contracts. Could break if Gate.io migrates to string-only responses or if decimal contracts are introduced.

---

### L4. GetUserTrades Limit Max Should Be 1000

**File**: `adapter.go:1018-1019`

**Problem**: Code caps limit at 100, but Gate.io docs say the default limit is 100 and max is 1000.

```go
// CURRENT:
if limit <= 0 || limit > 100 {
    limit = 100 // Gate.io max is 100  ← comment is wrong
}
```

**Impact**: Not a bug per se, but the comment is incorrect and the function could support up to 1000 records per API docs.

---

## Passing Checks

| Check | Status | Notes |
|-------|--------|-------|
| Symbol format (BTCUSDT ↔ BTC_USDT) | ✅ PASS | `toGateSymbol`/`fromGateSymbol` correct |
| Settle parameter ("usdt" in paths) | ✅ PASS | All endpoints use `/futures/usdt/` |
| Size sign convention (+long/-short) | ✅ PASS | PlaceOrder, GetPosition, GetPendingOrders all correct |
| Endpoint URL prefix | ✅ PASS | All use `/futures/{settle}/` correctly |
| Response parsing (direct JSON) | ✅ PASS | No wrapper object expected |
| Authentication (HMAC-SHA512) | ✅ PASS | Signature format, headers (KEY/SIGN/Timestamp) all correct |
| quanto_multiplier conversion | ✅ PASS | Consistently applied in PlaceOrder, GetPosition, GetOrderFilledQty, GetOrderbook, WS depth, GetUserTrades |
| WebSocket URL | ✅ PASS | `wss://fx-ws.gateio.ws/v4/ws/usdt` correct |
| WS channel names | ✅ PASS | `futures.book_ticker`, `futures.order_book`, `futures.orders`, `futures.ping` |
| WS auth signature | ✅ PASS | `channel=<ch>&event=<ev>&time=<ts>` with HMAC-SHA512 |
| WS private subscription | ✅ PASS | Uses `!all` wildcard for all-contract order updates |
| Cross margin leverage setup | ✅ PASS | `leverage=0, cross_leverage_limit=<value>` correctly sets cross mode |
| Unified account detection | ✅ PASS | Falls back to classic on failure |
| Error handling & retry | ✅ PASS | Exponential backoff, retries on transient errors |
| Client order ID (text field) | ✅ PASS | Correctly uses `t-` prefix |
| Order cancellation idempotency | ✅ PASS | ORDER_NOT_FOUND treated as success |

---

## Recommended Priority

1. **Fix C1** (margin mode detection) — Simple fix, high correctness impact
2. **Fix C2** (MinSize/MaxSize units) — Simple fix, affects order sizing
3. **Fix H2** (IOC partial fill callback) — Add `"ioc"` to status mapping
4. **Fix H1** (string parsing for size fields) — Use `string`/`json.Number` types
5. **Fix M1** (dual_mode body format) — Low risk but easy to correct
