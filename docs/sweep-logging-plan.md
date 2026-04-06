# Sweep & Approval 排查 Log 計劃 v3

> Status: **PLAN — 等審查通過後實施**

## 目的
補齊入場掃描路徑所有靜默分支的 log，讓每個計算步驟和失敗原因可追蹤。

---

## 改動清單（8 項）

### 1. ensureFuturesBalance skip 分支 Debug log (manager.go)
**位置**: ensureFuturesBalance 函數 (~line 758-773)

```go
// line 760: futures already sufficient
if futBal.Available >= needed {
    m.log.Debug("ensureFuturesBalance %s: skip — futures=%.2f >= needed=%.2f", exchName, futBal.Available, needed)
    return
}

// line 764: negligible deficit
if deficit < 0.01 {
    m.log.Debug("ensureFuturesBalance %s: skip — deficit=%.4f negligible", exchName, deficit)
    return
}

// line 773: spot balance is zero (line 770 已有 err Warn)
if spotBal.Available <= 0 {
    m.log.Debug("ensureFuturesBalance %s: skip — spot=0, nothing to transfer", exchName)
    return
}
```

### 2. 擴充現有 transfer log (manager.go:785)
**位置**: 現有 auto-transfer Info log，加入 deficit 和 spot 欄位

```go
// BEFORE:
m.log.Info("auto-transfer %s USDT spot→futures on %s (futures=%.2f, needed=%.2f)", amtStr, exchName, futBal.Available, needed)
// AFTER:
m.log.Info("auto-transfer %s USDT spot→futures on %s (futures=%.2f needed=%.2f deficit=%.2f spot=%.2f)",
    amtStr, exchName, futBal.Available, needed, deficit, spotBal.Available)
```

### 3. Sweep mode 標記 (manager.go ~line 165)
**位置**: auto-size else 分支，`const sweepTarget = 1e9` 之後

```go
const sweepTarget = 1e9
m.log.Info("[sweep] auto-size: sweeping spot→futures for %s and %s", opp.LongExchange, opp.ShortExchange)
m.ensureFuturesBalance(...)
```

### 4a. Sweep done marker (manager.go ~line 167)
**位置**: auto-size else 分支內，兩個 ensureFuturesBalance 呼叫之後、else 閉括號之前

```go
        m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
        m.log.Info("[sweep] done for %s and %s", opp.LongExchange, opp.ShortExchange)
    } // end of auto-size else — [sweep] log only fires in auto-size mode
```

### 4b. Post-transfer balance log (manager.go ~line 183)
**位置**: re-fetch balances 之後，在 `if !dryRun { }` 閉括號之前。兩種模式都印（fixed-capital 和 auto-size 都需要確認轉帳結果）

```go
    // After both re-fetch blocks, still inside if !dryRun:
    m.log.Info("post-transfer balances: %s=%.2f %s=%.2f",
        opp.LongExchange, longBal.Available, opp.ShortExchange, shortBal.Available)
} // end of if !dryRun
```

### 5. IsBlacklisted 錯誤 log (engine.go ~line 1715)
**位置**: executeArbitrage 的 approval loop

```go
// BEFORE:
if blocked, err := e.db.IsBlacklisted(opp.Symbol); err == nil && blocked {
    ...
    continue
}
// AFTER:
if blocked, err := e.db.IsBlacklisted(opp.Symbol); err != nil {
    e.log.Warn("IsBlacklisted check failed for %s: %v — treating as not blocked", opp.Symbol, err)
} else if blocked {
    if e.rejStore != nil {  // retain existing nil guard
        e.rejStore.AddOpp(opp, "engine", "symbol blacklisted")
    }
    continue
}
```

### 6. Slot 已滿 skip log (engine.go ~line 1721)
**位置**: `newSlotCandidates >= slots` check

```go
// BEFORE:
if newSlotCandidates >= slots {
    continue
}
// AFTER:
if newSlotCandidates >= slots {
    e.log.Debug("executeArbitrage: slot limit reached (%d/%d), skipping %s", newSlotCandidates, slots, opp.Symbol)
    continue
}
```

### 7. calculateSizeWithPrice remainingSlots<=0 log (manager.go ~line 687-690)
**位置**: calculateSizeWithPrice 函數

```go
// BEFORE:
remainingSlots := m.cfg.MaxPositions - len(active)
if remainingSlots <= 0 {
    return 0
}
// AFTER:
remainingSlots := m.cfg.MaxPositions - len(active)
if remainingSlots <= 0 {
    m.log.Info("[sizing] %s: rejected — no remaining slots (%d/%d)", opp.Symbol, len(active), m.cfg.MaxPositions)
    return 0
}
```

### 8. Capital allocator rejected 改 Warn (engine.go ~line 1764)
**位置**: reservePerpCapital 錯誤處理

```go
// BEFORE:
e.log.Info("capital allocator rejected %s: %v", opp.Symbol, err)
// AFTER:
e.log.Warn("capital allocator rejected %s: %v", opp.Symbol, err)
```

### 9. Re-fetch 失敗 log (manager.go ~line 171, 178)
**位置**: approveInternal 的 re-fetch balances 區塊，在 `if !dryRun` 內

```go
// BEFORE (~line 171):
if lb, err := longExch.GetFuturesBalance(); err == nil {
    longBal = lb
    ...
}
// AFTER:
if lb, err := longExch.GetFuturesBalance(); err == nil {
    longBal = lb
    ...
} else {
    m.log.Warn("post-transfer re-fetch %s failed: %v — using pre-transfer balance", opp.LongExchange, err)
}

// BEFORE (~line 178):
if sb, err := shortExch.GetFuturesBalance(); err == nil {
    shortBal = sb
    ...
}
// AFTER:
if sb, err := shortExch.GetFuturesBalance(); err == nil {
    shortBal = sb
    ...
} else {
    m.log.Warn("post-transfer re-fetch %s failed: %v — using pre-transfer balance", opp.ShortExchange, err)
}
```

### 10. activeSymbols skip log (engine.go ~line 1711)
**位置**: executeArbitrage 的 approval loop，symbol 已有持倉時

```go
// BEFORE:
if activeSymbols[opp.Symbol] {
    continue
}
// AFTER:
if activeSymbols[opp.Symbol] {
    e.log.Debug("executeArbitrage: skipping %s — already has active position", opp.Symbol)
    continue
}
```

### 11. Blacklist skip log 補充 (engine.go ~line 1715)
**位置**: Item 5 的 blocked 分支，rejStore 為 nil 時也要 log

```go
} else if blocked {
    if e.rejStore != nil {
        e.rejStore.AddOpp(opp, "engine", "symbol blacklisted")
    }
    e.log.Debug("executeArbitrage: skipping %s — blacklisted", opp.Symbol)
    continue
}
```

---

## 不改的項目
- rebalanceFunds — 已有完整 balance log (engine.go:489)
- createPendingPosition nil guard — 低風險，函數總是回傳非 nil，單獨處理
- commitPerpCapital 失敗後的 recovery — 邏輯改動不在 log 範圍

## 改動範圍（共 12 處）
- `internal/risk/manager.go`: 8 處（3 Debug skip + 1 Info 擴充 + 1 sweep 標記 + 1 sweep done + 1 post-transfer + 1 sizing slots log + 2 re-fetch warn）
- `internal/engine/engine.go`: 4 處（1 IsBlacklisted warn + 1 blacklist skip debug + 1 activeSymbol skip debug + 1 slot skip debug + 1 allocator Info→Warn）
- 不改任何業務邏輯，只加/改 log

## 完整排查鏈（改動後）

```
[entry] balances: binance: total=500 frozen=31 avail=468 spot=23 | gateio: total=83 frozen=0 avail=83
[sweep] auto-size: sweeping spot→futures for binance and gateio
ensureFuturesBalance binance: futures=468 needed=1e9 deficit=... spot=23 transfer=23.33
auto-transfer 23.3300 USDT spot→futures on binance (futures=468 needed=1e9 deficit=... spot=23.33)
ensureFuturesBalance gateio: skip — spot=0, nothing to transfer  (Debug)
[sweep] done for binance and gateio
post-transfer balances: binance=491.86 gateio=83.59
  (or: post-transfer re-fetch binance failed: ... — using pre-transfer balance)  (Warn)
skipping BTCUSDT — already has active position  (Debug)
skipping XYZUSDT — blacklisted  (Debug)
slot limit reached (2/2), skipping ETHUSDT  (Debug)
[sizing] SOLUSDT: rejected — no remaining slots (2/2)
capital allocator rejected BTCUSDT: total cap exceeded  (Warn)
approved LINKUSDT: size=5.0 price=15.00 spread=2.0 bps/h
```

## 風險
- 低 — 只加/改 log，不改業務邏輯
- Item 5 (IsBlacklisted) 改了 if 結構，需確認邏輯等效
