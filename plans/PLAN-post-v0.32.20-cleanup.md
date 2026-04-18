# Plan: Post-v0.32.20 Cleanup — 7 bugs from log audit
Version: v2
Date: 2026-04-18
Status: REVIEWING

## Context

After v0.32.20 deploy on 2026-04-17 16:22 UTC, open-ended Codex log audit over 11 rotate/entry cycles surfaced 7 additional bugs (excluding Bug 5 = allocator symbol cooldown, deferred). All 7 addressed as independent sections. Ships as one patch (v0.32.21).

Ordering by severity:
- **A**: Bug β — bybit unified receiver TransferToFutures blocks PostBalances credit
- **B**: Bug 3 — rebalance top-up sizes too close to L4, entry still rejects
- **C**: Bug 2 — binance -2019 insufficient margin burns 5 IOC retries
- **D**: Bug 4 — override stored when no rebalance transfer happened
- **E**: Bug 6 — BingX 109400 "order not exist" noisy cancel path
- **F**: Bug 1 — Gate.io EnsureOneWayMode startup error not hard-failed
- **G**: Bug 7 — donor capacity log spam (log-only)

Concrete code blocks below all verified by Codex (reading actual source). All line refs Codex-verified.

---

## Section A — Bybit unified receiver credit (Bug β) — PASS at v1

### Problem
`allocator.go:1930-1952` unified recipients confirm deposit via `GetFuturesBalance()` but still unconditionally call `TransferToFutures()` at :1952. Bybit UTA returns `code=131212` (no separate spot wallet). Failure skips credit path (else at :1955-1970), leaves PostBalances/FundedReceivers stale, override drops.

### Change
At `internal/engine/allocator.go` — insert BEFORE the existing `if err := recipientExch.TransferToFutures(...)` at :1952 (preserves non-unified else-path at :1955-1970):

```go
if uc, ok := recipientExch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
    // Deposit already lives in unified pool; GetFuturesBalance() already saw it.
    bi := balances[recipient]
    bi.futures += totalPending
    bi.futuresTotal += totalPending
    balances[recipient] = bi
    if result.FundedReceivers == nil {
        result.FundedReceivers = make(map[string]float64)
    }
    result.FundedReceivers[recipient] += totalPending
    result.CrossTransferHappened = true  // used in Section D
    continue
}
```

Test: add regression test proving unified recipient credited without `TransferToFutures()` call.

---

## Section B — Rebalance L4 headroom (Bug 3)

### Problem
5 sites compute `targetRatio := e.cfg.MarginL4Threshold - marginEpsilon` where `marginEpsilon` is 0.005 (virtually no headroom). Transfer sized to hit L4 exactly → any tiny price move pushes post-trade ratio to 0.81+ → entry rejects.

### Change

#### B1 — New helper at top of `internal/engine/allocator.go`:
```go
func (e *Engine) rebalanceTargetRatio() float64 {
    target := e.cfg.MarginL4Threshold - e.cfg.MarginL4Headroom
    if target <= 0 || target >= e.cfg.MarginL4Threshold {
        target = e.cfg.MarginL4Threshold - marginEpsilon
    }
    return target
}
```

#### B2 — Replace 5 sites: `allocator.go:935, :1357, :1379` + `engine.go:931, :952`

BEFORE (each site):
```go
targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
```

AFTER (each site):
```go
targetRatio := e.rebalanceTargetRatio()
```

#### B3 — Config wiring at `internal/config/config.go`

Add field to `Config` struct (next to existing `MarginL4Threshold`):
```go
MarginL4Headroom float64 // extra cushion below L4 for rebalance deficit sizing (default: 0.05)
```

Add to `jsonRisk` struct:
```go
MarginL4Headroom *float64 `json:"margin_l4_headroom"`
```

Add to defaults block:
```go
MarginL4Headroom: 0.05,
```

Add to `applyJSON`:
```go
if rk.MarginL4Headroom != nil && *rk.MarginL4Headroom > 0 {
    c.MarginL4Headroom = *rk.MarginL4Headroom
}
```

Add to API response map:
```go
risk["margin_l4_headroom"] = c.MarginL4Headroom
```

#### B4 — Test update at `internal/engine/allocator_deficit_test.go`

BEFORE:
```go
const (
    marginEpsilon          = 0.005
    marginSafetyMultiplier = 2.0
    marginL4Threshold      = 0.80
)
targetRatio := marginL4Threshold - marginEpsilon
```

AFTER:
```go
const (
    marginL4Headroom       = 0.05
    marginSafetyMultiplier = 2.0
    marginL4Threshold      = 0.80
)
targetRatio := marginL4Threshold - marginL4Headroom
```

---

## Section C — Binance -2019 early-abort + refetch+downsize (Bug 2)

### Problem
First-leg IOC returns `code=-2019 Margin is insufficient`. Current code: retry 5× same size → circuit breaker trips → trade aborted. Fix: on first margin error, refetch live balance, downsize once, retry; if refetched size too small, abort immediately (don't waste remaining retries).

### Change

#### C1 — New helper in `internal/engine/engine.go` near other engine helpers:
```go
func (e *Engine) refetchMarginCappedSize(exch exchange.Exchange, longExchange, shortExchange, symbol string, leverage, price, minSize, minChunkUSDT, currentSize float64) (float64, *exchange.Balance, error) {
    liveBal, err := exch.GetFuturesBalance()
    if err != nil {
        return 0, nil, err
    }
    downsized := (liveBal.Available * leverage) / price
    downsized = e.commonTradeableSize(longExchange, shortExchange, symbol, downsized)
    if downsized <= 0 || downsized >= currentSize || downsized < minSize || downsized*price < minChunkUSDT {
        return 0, liveBal, fmt.Errorf("refetched size below minimum (avail=%.2f price=%.6f)", liveBal.Available, price)
    }
    return downsized, liveBal, nil
}
```

#### C2 — Short-first first leg at `internal/engine/engine.go:3762-3772`

BEFORE:
```go
if err != nil {
    shortConsecFails++
    e.recordAPIError(opp.ShortExchange, err)
    if isMarginError(err) {
        e.log.Warn("depth tick: short IOC margin insufficient for %s, invalidating cache", opp.Symbol)
        lastBalCheck = time.Time{}
    }
    e.log.Warn("depth tick: short IOC failed (%d/%d): %v", shortConsecFails, maxConsecFails, err)
    continue
}
```

AFTER:
```go
if err != nil && isMarginError(err) {
    e.log.Warn("depth tick: short IOC margin insufficient for %s, refetching live balance", opp.Symbol)
    lastBalCheck = time.Time{}
    downsized, liveBal, resizeErr := e.refetchMarginCappedSize(shortExch, opp.LongExchange, opp.ShortExchange, opp.Symbol, leverage, bidPrice, minSize, e.cfg.MinChunkUSDT, size)
    if liveBal != nil {
        cachedShortBal = liveBal
    }
    if resizeErr != nil {
        e.log.Warn("depth tick: short IOC margin insufficient for %s after refetch, aborting entry: %v", opp.Symbol, resizeErr)
        break fillLoop
    }
    size = downsized
    shortSizeStr = e.formatSize(opp.ShortExchange, opp.Symbol, size)
    e.log.Warn("depth tick: short IOC margin insufficient for %s, retrying once with downsized size=%s", opp.Symbol, shortSizeStr)
    shortOID, err = shortExch.PlaceOrder(exchange.PlaceOrderParams{Symbol: opp.Symbol, Side: exchange.SideSell, OrderType: "limit", Price: shortPriceStr, Size: shortSizeStr, Force: "ioc"})
}
if err != nil {
    shortConsecFails++
    e.recordAPIError(opp.ShortExchange, err)
    e.log.Warn("depth tick: short IOC failed (%d/%d): %v", shortConsecFails, maxConsecFails, err)
    continue
}
```

#### C3 — Long-first first leg at `internal/engine/engine.go:3999-4011`

BEFORE:
```go
if err != nil {
    longConsecFails++
    e.recordAPIError(opp.LongExchange, err)
    if isMarginError(err) {
        e.log.Warn("depth tick: long IOC margin insufficient for %s, invalidating cache", opp.Symbol)
        lastBalCheck = time.Time{}
    }
    e.log.Warn("depth tick: long IOC failed (%d/%d): %v", longConsecFails, maxConsecFails, err)
    continue
}
```

AFTER:
```go
if err != nil && isMarginError(err) {
    e.log.Warn("depth tick: long IOC margin insufficient for %s, refetching live balance", opp.Symbol)
    lastBalCheck = time.Time{}
    downsized, liveBal, resizeErr := e.refetchMarginCappedSize(longExch, opp.LongExchange, opp.ShortExchange, opp.Symbol, leverage, askPrice, minSize, e.cfg.MinChunkUSDT, size)
    if liveBal != nil {
        cachedLongBal = liveBal
    }
    if resizeErr != nil {
        e.log.Warn("depth tick: long IOC margin insufficient for %s after refetch, aborting entry: %v", opp.Symbol, resizeErr)
        break fillLoop
    }
    size = downsized
    longSizeStr = e.formatSize(opp.LongExchange, opp.Symbol, size)
    e.log.Warn("depth tick: long IOC margin insufficient for %s, retrying once with downsized size=%s", opp.Symbol, longSizeStr)
    longOID, err = longExch.PlaceOrder(exchange.PlaceOrderParams{Symbol: opp.Symbol, Side: exchange.SideBuy, OrderType: "limit", Price: longPriceStr, Size: longSizeStr, Force: "ioc"})
}
if err != nil {
    longConsecFails++
    e.recordAPIError(opp.LongExchange, err)
    e.log.Warn("depth tick: long IOC failed (%d/%d): %v", longConsecFails, maxConsecFails, err)
    continue
}
```

Second-leg retry paths at `:3912-3924` and `:4145-4156` remain UNCHANGED (second-leg margin errors require rollback/match logic, not downsize).

---

## Section D — Override storage gate (Bug 4)

### Problem
`rebalanceFunds` stores allocator override even when no real transfer occurred. Next entry scan hits "all overrides stale" → tier-3 fallback → capital unchanged.

### Change

#### D1 — Extend `rebalanceExecutionResult` at `internal/engine/allocator.go:78-85`

BEFORE:
```go
type rebalanceExecutionResult struct {
    PostBalances    map[string]rebalanceBalanceInfo
    Unfunded        map[string]float64
    SkipReasons     map[string]string
    FundedReceivers map[string]float64
}
```

AFTER:
```go
type rebalanceExecutionResult struct {
    PostBalances          map[string]rebalanceBalanceInfo
    Unfunded              map[string]float64
    SkipReasons           map[string]string
    FundedReceivers       map[string]float64
    LocalTransferHappened bool
    CrossTransferHappened bool
}
```

#### D2 — Local spot→futures success at `allocator.go:1320-1327`

BEFORE:
```go
if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
    e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
} else {
    bi := balances[name]
    bi.futures += extra
    bi.spot -= extra
    bi.futuresTotal += extra
    balances[name] = bi
}
```

AFTER (add one line):
```go
if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
    e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
} else {
    bi := balances[name]
    bi.futures += extra
    bi.spot -= extra
    bi.futuresTotal += extra
    balances[name] = bi
    result.LocalTransferHappened = true
}
```

#### D3 — Local spot→futures at `allocator.go:1408-1417`

BEFORE:
```go
if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
    e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
} else {
    transferAmt -= actualTransfer
    bi := balances[name]
    bi.futures += actualTransfer
    bi.spot -= actualTransfer
    bi.futuresTotal += actualTransfer
    balances[name] = bi
}
```

AFTER (add one line):
```go
if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
    e.log.Error("rebalance: %s spot->futures failed: %v", name, err)
} else {
    transferAmt -= actualTransfer
    bi := balances[name]
    bi.futures += actualTransfer
    bi.spot -= actualTransfer
    bi.futuresTotal += actualTransfer
    balances[name] = bi
    result.LocalTransferHappened = true
}
```

#### D4 — Cross-exchange receiver credit success

At the point where recipient balance is committed after successful deposit + TransferToFutures (non-unified path) OR unified-pool credit from Section A. Set `result.CrossTransferHappened = true`:

```go
balances[recipient] = bi
if result.FundedReceivers == nil {
    result.FundedReceivers = make(map[string]float64)
}
result.FundedReceivers[recipient] += totalPending
result.CrossTransferHappened = true
```

(Section A's unified branch already sets this; also set in the existing non-unified credit success block.)

#### D5 — Override store gate at `internal/engine/engine.go:~632-646`

Insert short-circuit BEFORE the `kept := e.keepFundedChoices(...)` line:

```go
if !result.LocalTransferHappened && !result.CrossTransferHappened {
    e.allocOverrideMu.Lock()
    e.allocOverrides = nil
    e.allocOverrideMu.Unlock()
    e.log.Info("rebalance: no transfers executed, skipping allocator override store")
    e.log.Info("rebalance: complete")
    return
}
kept := e.keepFundedChoices(allocSel.choices, result.PostBalances, result.FundedReceivers)
```

---

## Section E — BingX 109400 idempotent cancel (Bug 6)

### Problem
BingX `CancelOrder` returns `code=109400 order not exist` when the order already completed (filled/cancelled by exchange). Current handler logs WARN and falls through. Real fix: treat 109400 as idempotent at the adapter layer (same pattern as existing 80018/80016 handling).

### Change

#### E1 — `pkg/exchange/bingx/adapter.go:243-255`

BEFORE:
```go
func (a *Adapter) CancelOrder(symbol, orderID string) error {
    params := map[string]string{"symbol": toBingXSymbol(symbol), "orderId": orderID}
    _, err := a.client.Delete("/openApi/swap/v2/trade/order", params)
    if err != nil {
        if apiErr, ok := err.(*APIError); ok {
            if apiErr.Code == 80018 || apiErr.Code == 80016 {
                return nil
            }
        }
        return fmt.Errorf("bingx CancelOrder: %w", err)
    }
    return nil
}
```

AFTER (add 109400 to idempotent code list):
```go
func (a *Adapter) CancelOrder(symbol, orderID string) error {
    params := map[string]string{"symbol": toBingXSymbol(symbol), "orderId": orderID}
    _, err := a.client.Delete("/openApi/swap/v2/trade/order", params)
    if err != nil {
        if apiErr, ok := err.(*APIError); ok {
            if apiErr.Code == 80018 || apiErr.Code == 80016 || apiErr.Code == 109400 {
                return nil
            }
        }
        return fmt.Errorf("bingx CancelOrder: %w", err)
    }
    return nil
}
```

#### E2 — `internal/engine/engine.go:4485-4490` — add comment explaining normalization

BEFORE (handler block):
```go
if err := exch.CancelOrder(symbol, orderID); err != nil {
    e.log.Warn("confirmFill: cancel %s on %s: %v", orderID, exch.Name(), err)
}
time.Sleep(200 * time.Millisecond)
restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
```

AFTER (add comment, no behavior change):
```go
// BingX 109400 is normalized to nil by its adapter, so "already gone"
// falls through to the REST query below instead of spamming WARNs.
if err := exch.CancelOrder(symbol, orderID); err != nil {
    e.log.Warn("confirmFill: cancel %s on %s: %v", orderID, exch.Name(), err)
}
time.Sleep(200 * time.Millisecond)
restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
```

---

## Section F — Gate.io EnsureOneWayMode hard-fail (Bug 1)

### Problem
`cmd/main.go:223-229` logs EnsureOneWayMode errors and continues. Bot proceeds with unknown account mode — if gateio is in hedge mode, trades will fail silently or create wrong positions. Fix: hard-fail startup.

### Change

`cmd/main.go:223-229`

BEFORE:
```go
for name, exch := range exchanges {
    if err := exch.EnsureOneWayMode(); err != nil {
        log.Error("EnsureOneWayMode on %s: %v", name, err)
    } else {
        log.Info("%s: one-way mode confirmed", name)
    }
}
```

AFTER:
```go
for name, exch := range exchanges {
    if err := exch.EnsureOneWayMode(); err != nil {
        log.Error("FATAL: EnsureOneWayMode on %s failed: %v", name, err)
        os.Exit(1)
    }
    log.Info("%s: one-way mode confirmed", name)
}
```

Verify `os` import is present in `cmd/main.go`; add if missing.

---

## Section G — Donor capacity log downgrade (Bug 7) — PASS at v1

### Change
`internal/engine/allocator.go:784` — change INFO to DEBUG.

BEFORE:
```go
e.log.Info("rebalance: donor %s capacity: futures=%.2f total=%.2f marginRatio=%.4f maxTransferOut=%.2f futuresAvail=%.2f surplus=%.2f unified=%v hasPos=%v",
    name, bal.futures, bal.futuresTotal, bal.marginRatio, bal.maxTransferOut, futuresAvail, surplus, isUnified, bal.hasPositions)
```

AFTER:
```go
e.log.Debug("rebalance: donor %s capacity: futures=%.2f total=%.2f marginRatio=%.4f maxTransferOut=%.2f futuresAvail=%.2f surplus=%.2f unified=%v hasPos=%v",
    name, bal.futures, bal.futuresTotal, bal.marginRatio, bal.maxTransferOut, futuresAvail, surplus, isUnified, bal.hasPositions)
```

---

## Files Touched (summary)

| File | Sections |
|------|----------|
| `internal/engine/allocator.go` | A, B2 (3 sites), D1-D4, G |
| `internal/engine/engine.go` | B2 (2 sites), C1-C3, D5, E2 |
| `internal/engine/allocator_deficit_test.go` | B4 |
| `internal/config/config.go` | B3 |
| `cmd/main.go` | F |
| `pkg/exchange/bingx/adapter.go` | E1 |

## Version
Bump to `v0.32.21`.

## Review History
- v1: Codex — A, G PASS; B, C, D, E, F NEEDS-REVISION (missing 3 sites for B; wrong line numbers for C; missing struct fields for D; bingx has no GetOrderStatus — use existing idempotency pattern for E; F fix already present, only startup-log needs hardening).
- v2: REVIEWING — all concrete BEFORE/AFTER code supplied by Codex v1 direct-ask; 5 sites for B centralized; 2 sites for C; struct extension + 2 success sites for D; 1-line adapter change for E; os.Exit for F.
