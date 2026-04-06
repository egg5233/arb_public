# :35 EntryScan 修復計劃 v4

> Based on Codex gpt-5.4 reviews: initial (task-mnm5eo6g), plan v1-v3 (task-mnmj7upa, task-mnmjzd5d, bwppcevq9)
> Status: **READY TO IMPLEMENT**
> Updated: 2026-04-06 v5-final — Codex 5 輪審查通過

---

## Phase 1: Critical Fixes

### Fix C1+C2: 可靠 scan 傳遞 + 空掃描處理
**問題**:
1. `scanner.go:779` — 空 merged 直接 return，不發 ScanResult
2. `scanner.go:101` — oppChan buffer=1 + non-blocking send (`scanner.go:970-974`)
3. 空掃描時 `s.opportunities` 不清

**設計考量**: scanner 在 `Start()` goroutine 的 scheduler loop 中 inline 呼叫 `runCycleTagged()` (`scanner.go:208-217`)。blocking send 會造成 scanner 排程 backpressure（下一次 scan 要等上一次被消費才能開始）。但這其實是**正確行為** — 如果 engine 還在處理上一個 scan，新的 scan 等一下是合理的，比 drop 好。只要有 `stopCh` 可在 shutdown 時打斷。

**修復**:
```go
// scanner.go — 新增 helper (在 struct methods 區域)
func (s *Scanner) sendScanResult(result ScanResult) {
    select {
    case s.oppChan <- result:
    case <-s.stopCh:
        s.log.Info("scanner stopped, dropping %s scan", result.Type)
    }
}

// scanner.go:779 — 空掃描路徑
if len(merged) == 0 {
    s.log.Info("No profitable opportunities found")
    s.mu.Lock()
    s.opportunities = nil
    s.mu.Unlock()
    s.sendScanResult(ScanResult{Opps: nil, Type: scanType})
    return
}

// scanner.go:970-974 — 正常路徑 (取代 select/default)
s.sendScanResult(ScanResult{Opps: verified, Type: scanType})
```
**影響範圍**: `internal/discovery/scanner.go` — +1 helper, 修改 2 處
**風險**: 低 — backpressure 比 drop 安全

---

### Fix C3: retrySecondLeg formatSize
**修復**: `engine.go:2065` 和 `engine.go:2117` 改為 `e.formatSize(exchName, symbol, remainingSize)`
**Codex**: APPROVE ✅ (3 次確認)

---

### Fix C4: closeFullyWithRetry rollback 檢查
**修復** — 覆蓋兩個 dust path:
```go
// exit.go — closeFullyWithRetryPriced 的兩個 dust branch 都要修正 totalFilled

// Dust path 1 (~line 2538): minSize check
if minSize > 0 && remaining < minSize {
    e.log.Info("closeFullyWithRetry %s %s: remaining %.6f below minSize — dust", ...)
    totalFilled = totalQty  // ← 新增
    remaining = 0
    break
}

// Dust path 2 (~line 2546): formatSize rounds to zero
if sizeF, _ := strconv.ParseFloat(sizeStr, 64); sizeF <= 0 {
    e.log.Info("closeFullyWithRetry %s %s: formatted size rounds to zero — dust", ...)
    totalFilled = totalQty  // ← 新增
    remaining = 0
    break
}

// exit.go — closeFullyWithRetry 新簽名
func (e *Engine) closeFullyWithRetry(exch exchange.Exchange, symbol string, side exchange.Side, totalQty float64) (remaining float64) {
    filled, _ := e.closeFullyWithRetryPriced(context.Background(), exch, symbol, side, totalQty)
    rem := totalQty - filled
    if rem < 0 {
        rem = 0  // clamp
    }
    return rem
}
```

**Caller 修改策略**: 共 27 處呼叫 `closeFullyWithRetry`，分佈在 `engine.go`、`exit.go`、`consolidate.go`。
**不手動列舉** — 用 grep 找全部 caller（Go 允許忽略 return value，改簽名不會編譯報錯）:
1. 改 `closeFullyWithRetry` 簽名為回傳 `float64` (remaining)
2. `grep -rn "closeFullyWithRetry" internal/engine/` 找全部 27 處 caller
3. 逐一修改每個 caller：
   - **rollback/abort 路徑** → 加 orphan alert
   - **trim 路徑** → log only
   - **consolidate 路徑** → orphan alert（清理殘留倉位失敗更需報警）

**orphan alert pattern**:
```go
rem := e.closeFullyWithRetry(exch, symbol, side, qty)
if rem > 0 {
    e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", symbol, side, rem, exch.Name())
}
```
**影響範圍**: `exit.go` (簽名 + 2 dust fix), `engine.go` + `exit.go` + `consolidate.go` (27 caller)
**風險**: 中 — 但編譯器確保不遺漏任何 caller

---

## Phase 2: Warnings

### Fix W1: 槓桿/margin-mode 設定失敗
**位置**: `engine.go:2453,2457`
**修復**: 區分 "already set" vs 真正失敗。abort on real failure。
**注意** (Codex 指出): rotation 路徑 `exit.go:2039` 也有相同 pattern，一併修。
**Codex**: APPROVE ✅

### Fix W2: 分佈式鎖複用 AcquireOwnedLock
**實際 API** (`locks.go:68,175`):
```go
// AcquireOwnedLock returns (*OwnedLock, bool, error)
lock, ok, err := e.db.AcquireOwnedLock(lockKey, 5*time.Minute)
if err != nil { ... }
if !ok { ... }
defer lock.Release()  // compare-and-delete, 不會誤刪別人的 lock
```
**呼叫點**: `engine.go:250` (manual open), `engine.go:1669` (entry scan)
**影響範圍**: ~3-5 處改 AcquireLock → AcquireOwnedLock + defer lock.Release()
**風險**: 低

### Fix W3: Auto-size 不跑 ensureFuturesBalance
**實際 config** (`config.go:37`): `CapitalPerLeg float64` (0=auto)，無 `MinCapitalPerLeg`
**Codex 轉帳邏輯審查結果**:
- `ensureFuturesBalance` 只轉 `min(deficit, spotAvailable)`，deficit = needed - futBal.Available
- 沒有 adapter 層的 min/max 限制
- **核心問題**: auto-size (CapitalPerLeg=0) 完全不呼叫 transfer → split-account 交易所的 spot 資金被浪費
- sizing 只看 futures balance → 如果 spot 多 futures 少，倉位會偏小

**修復**: auto-size 時，先把 spot 資金掃進 futures，再做 sizing。目標：最大化可用資金。
```go
// manager.go:153 — 改為:
if !dryRun {
    if needed > 0 {
        bufferedNeed := needed * m.cfg.MarginSafetyMultiplier
        m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, bufferedNeed)
        m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, bufferedNeed)
    } else {
        // Auto-size: sweep all available spot into futures to maximize position sizing.
        // On split-account exchanges (Binance, Bitget, Gate.io), spot balance
        // is invisible to futures sizing — transfer it before approval.
        // ensureFuturesBalance transfers min(deficit, spotAvailable), so passing
        // a large 'needed' effectively sweeps all spot into futures.
        const sweepTarget = 1e9 // effectively unlimited — transfer all available spot
        m.ensureFuturesBalance(opp.LongExchange, longExch, longBal, sweepTarget)
        m.ensureFuturesBalance(opp.ShortExchange, shortExch, shortBal, sweepTarget)
    }
    // Re-fetch balances after potential transfer (existing code below)
}
```
**為什麼 sweepTarget=1e9 安全**: `ensureFuturesBalance` 內部計算 `deficit = needed - futBal.Available`，然後轉 `min(deficit, spotBal.Available)`。所以實際轉帳金額 = 所有可用 spot 餘額，不會超轉。

**實施注意** (Codex 指出):
1. `fmt.Sprintf("%.4f", transferAmt)` 可能四捨五入超過 spotAvailable → 改用 `math.Floor(transferAmt*10000)/10000` 或 `min(transferAmt, spotBal.Available*0.999)` 確保不超轉
2. Bybit/OKX 的 `TransferToFutures` 不是 no-op（code comment 過時），它們執行 funding→trading 轉帳 → sweep 會影響這兩個交易所，但這是**正確行為**（最大化開倉）
**風險**: 低 — 不改 transfer 內部邏輯，只改觸發條件

### Fix W4: 審批用 cached orderbook 沒 age check
**實際 API**:
- PrefetchCache: `Orderbooks map[string]*exchange.Orderbook` (`manager.go:22`)
- Prefetch 存: `adapter.GetDepth(symbol)` 或 `adapter.GetOrderbook(symbol, 20)` → `orderbookSyncMap.Store(exchange+":"+symbol, ob)` (`engine.go:1591-1594`)
- Consume: `cache.Orderbooks[exchange+":"+symbol]` → fallback `exch.GetOrderbook(symbol, 20)` (`manager.go:275-280, 334-339`)

**修復**:
```go
// engine.go prefetch — store 端 age check
if ob, ok := adapter.GetDepth(k.symbol); ok {
    if ob.Time.IsZero() || time.Since(ob.Time) < 5*time.Second {
        orderbookSyncMap.Store(k.exchange+":"+k.symbol, ob)
    } else {
        e.log.Debug("prefetch: %s:%s orderbook stale (%.1fs)", k.exchange, k.symbol, time.Since(ob.Time).Seconds())
        // Try REST fallback
        if freshOb, err := adapter.GetOrderbook(k.symbol, 20); err == nil {
            orderbookSyncMap.Store(k.exchange+":"+k.symbol, freshOb)
        }
    }
}

// manager.go consume — read 端 age check
var longOB *exchange.Orderbook
if cache != nil && cache.Orderbooks != nil {
    if cached := cache.Orderbooks[opp.LongExchange+":"+opp.Symbol]; cached != nil {
        if cached.Time.IsZero() || time.Since(cached.Time) < 5*time.Second {
            longOB = cached
        }
        // else: stale, fall through to live fetch
    }
}
if longOB == nil {
    longOB, err2 = longExch.GetOrderbook(opp.Symbol, 20)
    // ... existing error handling
}
// shortOB 同理
```
**風險**: 低

### Fix W5: Capital allocator Commit 驗證 reservation
**實際 API** (`allocator.go:421-428`):
```go
func (a *CapitalAllocator) withVersionRetry(ctx context.Context, fn func(tx *redis.Tx) error) error {
    // ...
    err := a.db.Redis().Watch(ctx, fn, allocatorVersionKey)
}
```
**修復**: 擴展 `withVersionRetry` 支援額外 watch keys，或在 Commit 中直接呼叫 `a.db.Redis().Watch`:
```go
// allocator.go — Commit 方法改為直接使用 Watch
func (a *CapitalAllocator) Commit(res *CapitalReservation, positionID string, exposures map[string]float64) error {
    // ... validation (existing) ...

    ctx := context.Background()
    resKey := a.reservationKey(res.ID)

    var lastErr error
    for range 8 {
        err := a.db.Redis().Watch(ctx, func(tx *redis.Tx) error {
            // Verify reservation still exists (watched — tx fails if key changes)
            exists, err := tx.Exists(ctx, resKey).Result()
            if err != nil {
                return fmt.Errorf("check reservation %s: %w", res.ID, err)
            }
            if exists == 0 {
                return fmt.Errorf("reservation %s expired or released", res.ID)
            }

            // ... existing commit logic (read committed totals, pipeline set+del) ...
        }, allocatorVersionKey, resKey)  // watch BOTH keys

        if err == nil {
            return nil
        }
        if err == redis.TxFailedErr {
            lastErr = err
            continue
        }
        return err
    }
    return fmt.Errorf("commit after 8 retries: %w", lastErr)
}
```
**影響範圍**: `internal/risk/allocator.go` Commit 方法
**風險**: 低-中 — 影響共用 allocator，但是防禦性修復，perp + spot-futures 都受益

---

## Phase 3: Minor

### Fix M1: 掃描分鐘文檔不一致
更新 `engine.go:690-693` + `scanner.go:36-39` 為 config.go 預設值: `:20 rebalance`, `:30 exit`, `:35 rotate`, `:40 entry`
**Codex**: APPROVE ✅

### Fix M2: 未使用的 remainingSlots
刪除 `manager.go:291,521`
**Codex**: APPROVE ✅

---

## 實施順序

| # | Fix | 改動量 | 檔案 |
|---|-----|--------|------|
| 1 | C3 | 2 行 | engine.go |
| 2 | W1 | ~15 行 | engine.go, exit.go |
| 3 | C1+C2 | ~15 行 | scanner.go |
| 4 | W2 | ~10 行 | engine.go |
| 5 | W3 | ~15 行 | manager.go |
| 6 | W4 | ~20 行 | engine.go, manager.go |
| 7 | C4 | ~60 行 | exit.go, engine.go, consolidate.go |
| 8 | W5 | ~20 行 | allocator.go |
| 9 | M1+M2 | ~10 行 | engine.go, scanner.go, manager.go |

**~147 行, 6 檔案, 2 commits**:
1. Phase 1 (C1+C2, C3, C4)
2. Phase 2+3 (W1-W5, M1-M2)
