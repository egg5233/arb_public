# Plan: Fix Bybit 131212 and BingX 100437 Rebalance Transfer Bugs

Version: v4
Date: 2026-04-16
Status: DRAFT (addressing v3 review findings)

## Review History

- **v1** (2026-04-16 10:45 UTC): codex review — NEEDS-REVISION items 3, 4, 6, 7, 8, 9, 10, 11, 12
- **v2** (2026-04-16 12:55 UTC): codex review — **7 PASS / 4 NEEDS-REVISION**
  - PASS: #1, #2, #5, #6, #8, #9, #10, #12
  - NEEDS-REVISION:
    - #3 BingX hard-gate insertion point still vague (must name exact line in allocator.go:1577-1586 area)
    - #4 Dashboard toggle plan missing i18n updates in `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`
    - #7 Marker interface should go in `pkg/exchange/types.go:267-405` (where other marker interfaces live), not `exchange.go`
    - #11 exchange_manager hot-reload story doesn't match real code: `:18-23, 188-192, 207-221` snapshot creds only; `cmd/main.go:518-531` passes only ExchangeConfig. Must choose: rebuild-on-flag-change OR add concrete interface/snapshot/wiring. Also `exchange_manager_test.go` doesn't exist — say "new file".

## Context

Two production incidents on 2026-04-16 05:45-05:48 UTC during rebalance cycle for ARIAUSDT:

**Bybit 131212**: After binance→bybit deposit of 79.09 USDT confirmed, allocator called `TransferToFutures(FUND→UNIFIED)` which failed with `131212 Insufficient balance`. Log evidence: `GetFuturesBalance` (UNIFIED) went 46.69→125.79 (delta exactly matches deposit), meaning the deposit landed directly in UNIFIED. FUND was empty, so the follow-up FUND→UNIFIED transfer had nothing to move.

**BingX 100437**: After `TransferToSpot` (PFUTURES_FUND) succeeded for 14.04 USDT, withdraw with `walletType=1` failed with `100437 Insufficient balance. Deposit or switch to another account to proceed`. Rollback (FUND_PFUTURES) from the same endpoint succeeded — proving funds were in some fund-like bucket but not visible to walletType=1 withdraw.

Both bugs traced to commit `0d1e286d` (2026-03-28).

Prior investigation: dispatch tasks f5c8a824, 654c4f02, 88f6b368, a3b1aa13 all closed with codex verification.

## Per-Exchange Semantic Table (NEW — addresses v1 #7, #8, #12)

Before the fix we must distinguish what deposits do on each exchange:

| Exchange | IsUnified() | Deposit lands in | TransferToFutures required after deposit? | Implications for allocator |
|----------|-------------|------------------|-------------------------------------------|----------------------------|
| Bybit    | true        | FUND by default; UNIFIED if Set Deposit Account configured | **Only if deposit landed in FUND** | Needs dual-bucket detection |
| Gate.io  | true (dynamic) | Unified shared pool | No — `TransferToFutures` is no-op (`pkg/exchange/gateio/adapter.go:972-994`) | Current flow fine; observation optional |
| OKX      | true (dynamic above 10k USDT) | Funding account (type 6) | **Yes, always** — unified trading still separate from funding (`pkg/exchange/okx/adapter.go:995-1003`) | **Must keep current flow**; do NOT skip transfer |
| Binance  | true (dynamic, portfolio margin) | Spot wallet | Yes | Current flow correct |
| Bitget   | false       | Spot account | Yes | Current flow correct |
| BingX    | false       | Fund account | Yes | Current flow correct; fix is on donor-side (endpoint + post-transfer poll) |

**Conclusion**: dual-bucket skip-transfer logic is Bybit-specific. For all other exchanges the existing single-bucket poll + always-transfer flow is correct.

## Changes

### 1. Bybit-specific dual-bucket detection (v2 — narrower scope)

**File**: `internal/engine/allocator.go` lines 1888-1954

**Approach change (addresses v1 #7, #8)**: do NOT remove the `IsUnified()` branch universally. Add a new narrow marker interface for exchanges where "deposit may land directly in trading account" (currently only Bybit). Non-matching exchanges use existing logic unchanged.

```go
// New interface — place in pkg/exchange/types.go (alongside SpotMarginExchange,
// TradingFeeProvider, FlashRepayer at lines 267-405, where all other optional/marker
// interfaces live — v3 fix per codex #7).
type DepositMayCreditTradingDirectly interface {
    // Returns true if this exchange can credit incoming deposits directly
    // into the trading/unified account, bypassing the separate funding/spot
    // bucket that TransferToFutures normally drains.
    DepositMayCreditTradingDirectly() bool
}
```

**Adapter wiring**:
- `pkg/exchange/bybit/adapter.go`: add `func (a *Adapter) DepositMayCreditTradingDirectly() bool { return true }`
- `pkg/exchange/gateio/adapter.go`: add `func (a *Adapter) DepositMayCreditTradingDirectly() bool { return a.isUnified }` — unified shared pool also treats deposit as directly usable; transfer is no-op
- No additions needed for binance/okx/bitget/bingx — absence of method means `false` via type assertion

**Allocator poll logic** (replaces existing lines 1888-1954):

```go
type bucketBaseline struct {
    fund    float64  // GetSpotBalance / funding account
    trading float64  // GetFuturesBalance / unified trading account
}

// ... in the baseline capture loop:
if _, exists := pendingBaselines[bw.recipient]; !exists {
    baseline := bucketBaseline{}
    // Trading balance (always available for unified detection)
    if fb, err := e.exchanges[bw.recipient].GetFuturesBalance(); err == nil && fb != nil {
        baseline.trading = fb.Available
    }
    // Fund balance (always available; used by non-unified path and dual-bucket detection)
    if sb, err := e.exchanges[bw.recipient].GetSpotBalance(); err == nil && sb != nil {
        baseline.fund = sb.Available
    }
    pendingBaselines[bw.recipient] = baseline
}
pendingDeposits[bw.recipient] += recipientReceives

// ... in the confirmation loop:
for recipient, totalPending := range pendingDeposits {
    if totalPending <= 0 { continue }
    recipientExch := e.exchanges[recipient]
    baseline := pendingBaselines[recipient]
    threshold := totalPending * 0.9
    
    // Determine if this exchange can skip transfer when delta appears in trading
    canSkipTransfer := false
    if probe, ok := recipientExch.(interface { DepositMayCreditTradingDirectly() bool }); ok {
        canSkipTransfer = probe.DepositMayCreditTradingDirectly()
    }
    
    e.log.Info("rebalance: waiting for %.2f USDT on %s (fundStart=%.2f tradingStart=%.2f canSkipTransfer=%v)...",
        totalPending, recipient, baseline.fund, baseline.trading, canSkipTransfer)
    
    arrivedIn := ""  // "fund" | "trading" | ""
    var fundDelta, tradingDelta float64
    pollDeadline := time.Now().Add(5 * time.Minute)
    
    for time.Now().Before(pollDeadline) {
        time.Sleep(5 * time.Second)
        fundDelta, tradingDelta = 0, 0
        if fb, err := recipientExch.GetFuturesBalance(); err == nil && fb != nil {
            tradingDelta = fb.Available - baseline.trading
        }
        if sb, err := recipientExch.GetSpotBalance(); err == nil && sb != nil {
            fundDelta = sb.Available - baseline.fund
        }
        if fundDelta >= threshold {
            arrivedIn = "fund"; break
        }
        if canSkipTransfer && tradingDelta >= threshold {
            arrivedIn = "trading"; break
        }
    }
    
    if arrivedIn == "" {
        e.log.Warn("rebalance: deposits on %s not confirmed within 5min (fundDelta=%.2f tradingDelta=%.2f), skipping",
            recipient, fundDelta, tradingDelta)
        continue
    }
    
    e.log.Info("rebalance: deposits confirmed on %s via %s bucket (fundDelta=%.2f tradingDelta=%.2f)",
        recipient, arrivedIn, fundDelta, tradingDelta)
    
    if arrivedIn == "trading" {
        // Deposit already in trading account — skip transfer
        // (only reachable when canSkipTransfer=true, i.e., Bybit with Set Deposit Account, or Gate unified shared pool)
        e.log.Info("rebalance: %s deposit already in trading pool, skipping spot->futures", recipient)
        bi := balances[recipient]
        bi.futures += totalPending
        bi.futuresTotal += totalPending
        // NOTE: do NOT debit bi.spot — spot was never credited for this path (addresses v1 #8)
        balances[recipient] = bi
        continue
    }
    
    // arrivedIn == "fund" — normal transfer path
    transferStr := fmt.Sprintf("%.4f", totalPending)
    if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
        e.log.Error("rebalance: %s spot->futures failed: %v", recipient, err)
    } else {
        e.log.Info("rebalance: %s spot->futures %s USDT (rebalance deposit)", recipient, transferStr)
        e.recordTransfer(recipient+" spot", recipient, "USDT", "internal", transferStr, "0", "", "completed", "rebalance-recv")
        bi := balances[recipient]
        bi.futures += totalPending
        bi.futuresTotal += totalPending
        bi.spot -= totalPending
        if bi.spot < 0 { bi.spot = 0 }
        balances[recipient] = bi
    }
}
```

**Behavior verification per exchange**:
- Bybit (canSkipTransfer=true): deposits via Set Deposit Account→UNIFIED will show tradingDelta ≥ threshold → skip transfer → fixes 131212.
- Bybit (deposit to FUND): shows fundDelta → normal transfer path (unchanged from current behavior).
- OKX (canSkipTransfer=false): deposits always land in funding → fundDelta shows first → normal transfer (current behavior preserved).
- Gate unified (canSkipTransfer=true): tradingDelta shows (shared pool) → skip transfer (matches existing no-op semantics).
- Binance/Bitget/BingX: canSkipTransfer=false → fund/spot poll → normal transfer (unchanged).

### 2. Remove `pendingStartBal` (addresses v1 #6)

Replace all references with `pendingBaselines[recipient].fund` (or `.trading` for unified path). Grep confirmed only uses are in the deposit confirmation block (`internal/engine/allocator.go:1381, 1830-1851`). Delete the variable and its declaration.

### 3. BingX endpoint migration + HARD-GATE post-transfer poll (addresses v1 #3, #9, #12)

**Decision on v1 #9**: post-transfer poll is a **HARD GATE**, not telemetry. If fund balance doesn't reflect the transfer within timeout, do NOT proceed to withdraw. Set a distinct skip reason. Addresses v1 #3 (no silent fallback).

**File**: `pkg/exchange/bingx/adapter.go`

```go
// Adapter struct field (existing struct)
type Adapter struct {
    // ... existing fields
    useNewTransferAPI bool  // set via SetUseNewTransferAPI
}

func (a *Adapter) SetUseNewTransferAPI(enabled bool) {
    a.useNewTransferAPI = enabled
}

// TransferToSpot — migrate to documented v1 endpoint when flag enabled
func (a *Adapter) TransferToSpot(coin string, amount string) error {
    if a.useNewTransferAPI {
        params := map[string]string{
            "fromAccount": "USDTMPerp",
            "toAccount":   "fund",
            "asset":       coin,
            "amount":      amount,
        }
        _, err := a.client.Post("/openApi/api/asset/v1/transfer", params)
        if err != nil {
            return fmt.Errorf("bingx TransferToSpot (v1): %w", err)
        }
        return nil
    }
    // Legacy path (default)
    params := map[string]string{
        "type":   "PFUTURES_FUND",
        "asset":  coin,
        "amount": amount,
    }
    _, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
    if err != nil {
        return fmt.Errorf("bingx TransferToSpot (legacy): %w", err)
    }
    return nil
}

// TransferToFutures — symmetric migration
func (a *Adapter) TransferToFutures(coin string, amount string) error {
    if a.useNewTransferAPI {
        params := map[string]string{
            "fromAccount": "fund",
            "toAccount":   "USDTMPerp",
            "asset":       coin,
            "amount":      amount,
        }
        _, err := a.client.Post("/openApi/api/asset/v1/transfer", params)
        if err != nil {
            return fmt.Errorf("bingx TransferToFutures (v1): %w", err)
        }
        return nil
    }
    params := map[string]string{
        "type":   "FUND_PFUTURES",
        "asset":  coin,
        "amount": amount,
    }
    _, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
    if err != nil {
        return fmt.Errorf("bingx TransferToFutures (legacy): %w", err)
    }
    return nil
}
```

**File**: `internal/engine/allocator.go` — donor-side HARD-GATE poll (v3 fix per codex #3 — exact insertion point named)

**Exact insertion point**: immediately after `movedToSpot = moveAmt` on `allocator.go:1585`, BEFORE the donor-balance read at `:1588-1596` which builds the withdraw request. This is inside the per-donor `for` loop, so rollback and `continue` only affect this donor iteration.

The withdraw itself happens later in the batched path at `allocator.go:1721-1746` — but the hard-gate must fire IMMEDIATELY after `TransferToSpot` so that `donorSpotBal` at :1593/:1595 reflects the actual settled balance, not a pending one. That prevents the batched withdraw from being built against phantom balance.

```go
// Insert at allocator.go after line 1585 (movedToSpot = moveAmt), before line 1587 (blank line + line 1588 Unified accounts comment).
//
// HARD-GATE (addresses v1 #3, #9 and v2 #3): BingX donor fund visibility must
// match the spot transfer before we proceed to build the withdraw. If poll
// times out, we rollback futures→spot and mark this donor unusable for this
// rebalance cycle.
if bestDonor == "bingx" && movedToSpot > 0 {
    pollDeadline := time.Now().Add(15 * time.Second)
    required := movedToSpot * 0.99  // 1% tolerance for fee truncation
    baselineFund := balances[bestDonor].spot  // snapshot BEFORE the transfer above
    visible := false
    for time.Now().Before(pollDeadline) {
        time.Sleep(2 * time.Second)
        if fb, err := e.exchanges[bestDonor].GetSpotBalance(); err == nil && fb != nil {
            if fb.Available-baselineFund >= required {
                visible = true
                break
            }
        }
    }
    if !visible {
        e.log.Warn("rebalance: bingx fund not visible after futures->spot %.4f; rolling back", movedToSpot)
        if rbErr := e.exchanges[bestDonor].TransferToFutures("USDT", fmt.Sprintf("%.4f", movedToSpot)); rbErr != nil {
            e.log.Error("rebalance: bingx rollback after visibility timeout failed: %v", rbErr)
        }
        surplus[bestDonor] = 0
        movedToSpot = 0
        continue  // skip this donor for this rebalance cycle
    }
}
```

Note: the `continue` matches the existing error-handling pattern at `:1581-1583` and `:1598-1601`. The later batched-withdraw stage at `:1721-1746` will simply not see this donor (zeroed surplus + movedToSpot=0).

### 4. Feature flag wiring — full stack (addresses v1 #4, #11)

**File**: `internal/config/config.go`
- Add field `EnableBingXNewTransferAPI bool` with `json:"enable_bingx_new_transfer_api"` tag
- Add env var parsing: `ENABLE_BINGX_NEW_TRANSFER_API`
- Default: `false`

**File**: `internal/api/handlers.go`
- In GET `/api/config` response builder (~line 519-620): include `enable_bingx_new_transfer_api` field
- In POST `/api/config` parser (~line 829-972): accept and persist the field
- In config-to-Redis persistence path (~line 1518-1560): include field

**File**: `internal/api/config_handlers_test.go`
- Extend `TestConfigRoundTrip` (or similar) at ~line 528-620 to include `enable_bingx_new_transfer_api` in both GET and POST roundtrip

**File**: `internal/engine/exchange_manager.go` (v3 fix per codex #11 — concrete wiring chosen)

**Design decision**: **rebuild-on-flag-change**, NOT in-place mutation. Rationale:
- Existing snapshot pattern at `:18-23` already treats any snapshot-field change as "rebuild the adapter". Adding the flag to the snapshot matches this convention exactly.
- Mutating a live adapter mid-request would require mutex on every transfer call path, which is scope creep for a one-flag feature.
- Rebuild is cheap (no connection pools inside `bingx.NewAdapter`).

Concrete changes:

1. **Snapshot struct** (`:18-23`):
   ```go
   type adapterSnapshot struct {
       apiKey, secretKey, passphrase string
       enabled                       bool
       bingxUseNewTransferAPI        bool  // v3: only meaningful for bingx
   }
   ```

2. **Snapshot capture for bingx** (`:188-192`):
   ```go
   case "bingx":
       return adapterSnapshot{
           apiKey: m.cfg.BingXAPIKey, secretKey: m.cfg.BingXSecretKey,
           enabled:                m.cfg.IsExchangeEnabled("bingx"),
           bingxUseNewTransferAPI: m.cfg.EnableBingXNewTransferAPI,
       }
   ```

3. **Snapshot equality check** (existing reload logic, inspect and update): ensure the new field is compared so a flag flip triggers rebuild.

4. **buildExchangeConfig** (`:198-205`): pass the flag through the new field of `exchange.ExchangeConfig` (requires adding `BingXUseNewTransferAPI bool` to `pkg/exchange/types.go` ExchangeConfig struct):
   ```go
   func buildExchangeConfig(name string, snap adapterSnapshot) exchange.ExchangeConfig {
       return exchange.ExchangeConfig{
           Exchange:               name,
           ApiKey:                 snap.apiKey,
           SecretKey:              snap.secretKey,
           Passphrase:             snap.passphrase,
           BingXUseNewTransferAPI: snap.bingxUseNewTransferAPI,  // v3
       }
   }
   ```

5. **BingX adapter construction** — no separate SetUseNewTransferAPI method needed; adapter reads from the config struct it's already passed:
   ```go
   // In pkg/exchange/bingx/adapter.go NewAdapter (existing):
   func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
       return &Adapter{
           // ... existing fields
           useNewTransferAPI: cfg.BingXUseNewTransferAPI,  // v3
       }
   }
   ```

6. **cmd/main.go** (v4 fix per codex #11 — explicit cold-start change required):

   Hot reload path goes through `internal/engine/exchange_manager.go:138-205 → buildExchangeConfig` ✓. But **cold start does NOT use buildExchangeConfig** — it uses `exchangeConfig(cfg, name)` at `cmd/main.go:537-580`. If we only update the reload path, the flag is silently dropped on fresh process start and only activates after the first config reload.

   Required change in `cmd/main.go:571-576` (bingx case in `exchangeConfig`):
   ```go
   case "bingx":
       return exchange.ExchangeConfig{
           Exchange:               "bingx",
           ApiKey:                 cfg.BingXAPIKey,
           SecretKey:              cfg.BingXSecretKey,
           BingXUseNewTransferAPI: cfg.EnableBingXNewTransferAPI,  // v4
       }
   ```

**Dashboard UI** (v4 fix per codex #4 — correct `cfg.*` namespace):

Existing convention per `web/src/i18n/en.ts:300, :407, :456` and `web/src/pages/Config.tsx:537, :1729-1730, :1756`:
- Labels: `cfg.field.<camelCaseFieldName>`
- Descriptions: `cfg.desc.<camelCaseFieldName>`

v4 keys:
- `cfg.field.enableBingxNewTransferAPI` = "Use BingX new transfer API (v1 endpoint)"
- `cfg.desc.enableBingxNewTransferAPI` = "Migrates TransferToSpot / TransferToFutures to documented /openApi/api/asset/v1/transfer endpoint. Leave OFF unless tested."

Add both keys to both locale files (`web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts`).

`web/src/pages/Config.tsx` — add toggle component referencing these keys via `t('cfg.field.enableBingxNewTransferAPI')` / `t('cfg.desc.enableBingxNewTransferAPI')`. Default OFF.

### 5. Observability (v1 unchanged)

- Log both `fundDelta` and `tradingDelta` at confirmation (already shown in code above)
- One INFO line per confirmation event (not per-poll iteration) to keep log volume bounded

### 6. Tests (addresses v1 #10)

**New file `internal/engine/allocator_deposit_test.go`**:
- Test 1: Bybit deposit lands in FUND → TransferToFutures called, balance mutation correct
- Test 2: Bybit deposit lands in UNIFIED (canSkipTransfer=true) → TransferToFutures NOT called, futures balance credited without spot debit
- Test 3: OKX deposit lands in funding (canSkipTransfer=false for OKX) → TransferToFutures called, normal path
- Test 4: Deposit never arrives → timeout, no transfer, unfunded recorded
- Test 5: Partial delta below threshold → continues polling
- Test 6: Gate unified deposit → shows up in trading pool → skip transfer (no-op path)

**New file `pkg/exchange/bingx/adapter_test.go`** (codex confirmed this file does NOT exist — create, do not "append"):
- Test A: `TransferToSpot` with `useNewTransferAPI=true` POSTs to `/openApi/api/asset/v1/transfer` with correct `fromAccount`/`toAccount`
- Test B: `TransferToSpot` with `useNewTransferAPI=false` (default) POSTs to `/openApi/api/v3/post/asset/transfer` with `type=PFUTURES_FUND`
- Test C-D: Symmetric for `TransferToFutures`
- Test E: `Withdraw` always includes `walletType=1` (already in place, regression guard)

**Extend `internal/api/config_handlers_test.go`** (addresses v1 #10):
- Round-trip test for `enable_bingx_new_transfer_api` field (GET → POST → GET)

**New file `internal/engine/exchange_manager_test.go`** (v3 fix per codex #11 — file does NOT exist, create fresh):
- Test: snapshot captures `bingxUseNewTransferAPI`
- Test: config flip triggers rebuild (adapter instance pointer differs before/after)
- Test: non-flag changes do NOT trigger rebuild (preserve existing snapshot equality semantics)
- Test: BingX adapter's `useNewTransferAPI` field reflects the config value post-rebuild

### 7. Risk table (v2 — rewritten, addresses v1 #12)

| ID | Risk | Mitigation |
|----|------|-----------|
| R1 | Bybit dual-bucket race: auto-move happens between polls | 90% threshold + canSkipTransfer=true branch handles either ordering; observe both deltas so either case leads to correct outcome |
| R2 | BingX new endpoint migration breaks something else | Feature flag default `false`, manual VPS test before enabling, logs show chosen path |
| R3 | BingX post-transfer poll times out legitimately | **Hard gate** — abort withdraw, rollback futures→spot, record skip reason. Do NOT proceed. (v1 mentioned nonexistent "100437 retry logic" — removed.) |
| R4 | allocator.go structural change affects other paths | Grep confirms only rebalance allocator uses this deposit-confirm loop; spot-futures engine has own flows |
| R5 | IsUnified semantics leak | **Retained IsUnified branch**. Only narrow `DepositMayCreditTradingDirectly` marker added for Bybit + Gate unified. OKX explicitly opts out. Verified per-exchange in the semantic table above. |
| R6 | skip-transfer accounting invariant | Only active when `canSkipTransfer=true` (Bybit + Gate unified). For these, spot was never credited, so not debiting spot is correct. OKX and others always go through the fund branch which debits spot correctly. |
| R7 | Observability log spam | INFO only at confirmation event (once per recipient); DEBUG for per-poll iterations if needed |

## Testing Plan (v2)

1. **Unit tests** (Phase 4): new `allocator_deposit_test.go` + new `bingx/adapter_test.go` + extended `config_handlers_test.go` + extended exchange_manager tests
2. **Compile check**: `go build ./cmd/main.go`
3. **Live smoke test on VPS (post-Phase 5)**:
   - Deploy with `EnableBingXNewTransferAPI=false` (default)
   - Trigger manual rebalance targeting Bybit recipient
   - Verify: new log lines show both bucket deltas
   - Verify: if UNIFIED delta appears, transfer SKIPPED with new log line
   - Verify: no 131212 error
   - Set flag true via dashboard toggle
   - Trigger rebalance with BingX donor
   - Verify: BingX transfer uses new endpoint path
   - Verify: hard-gate poll visible in logs
   - Verify: if poll succeeds, withdraw proceeds normally
4. **Regression check**: manually trigger rebalance against binance/okx/gate/bitget recipients to confirm unchanged behavior

## Rollout (v2)

1. Merge with `EnableBingXNewTransferAPI=false` (default)
2. Live test Bybit fix on VPS (HIGH priority — Bybit rebalance currently broken)
3. After 24h stable, enable `EnableBingXNewTransferAPI=true` via dashboard
4. Monitor logs for hard-gate timeouts and new-endpoint success rate
5. If stable for 1 week, remove legacy path (v0.34.x)

## Files Modified Summary (v3)

| File | Type | What |
|------|------|------|
| `pkg/exchange/types.go` | interface add (line 267-405 area) | Add `DepositMayCreditTradingDirectly` marker interface + extend `ExchangeConfig` with `BingXUseNewTransferAPI bool` |
| `pkg/exchange/bybit/adapter.go` | method add | Implement `DepositMayCreditTradingDirectly() bool { return true }` |
| `pkg/exchange/gateio/adapter.go` | method add | Implement `DepositMayCreditTradingDirectly() bool { return a.isUnified }` |
| `pkg/exchange/bingx/adapter.go` | field add + 2 methods modified | `useNewTransferAPI` field read from `ExchangeConfig`; feature-flagged `TransferToSpot`/`TransferToFutures` paths |
| `internal/engine/allocator.go` | replace `:1888-1954` + insert at `:1586` | Dual-bucket detection + BingX donor hard-gate poll |
| `internal/config/config.go` | field + env + load wiring (~line 63-83, 723-780, 1344-1390) | `EnableBingXNewTransferAPI` |
| `internal/api/handlers.go` | GET/POST wiring (~line 519-620, 829-980, 1518-1560) | `/api/config` response/update includes new field; Redis persistence |
| `internal/engine/exchange_manager.go` | snapshot + buildExchangeConfig (`:18-23, 188-192, 198-205`) | Capture flag, pass through to adapter; rebuild on flag change |
| `cmd/main.go` | edit `:571-576` (bingx case in `exchangeConfig`) | Add `BingXUseNewTransferAPI: cfg.EnableBingXNewTransferAPI` to cold-start ExchangeConfig |
| `web/src/pages/Config.tsx` | UI toggle | Dashboard toggle using `t('cfg.field.enableBingxNewTransferAPI')` and `cfg.desc.*` (default OFF) |
| `web/src/i18n/en.ts` | i18n keys | `cfg.field.enableBingxNewTransferAPI` + `cfg.desc.enableBingxNewTransferAPI` |
| `web/src/i18n/zh-TW.ts` | i18n keys | Same two keys (TW) |
| `internal/engine/allocator_deposit_test.go` | **new** | 6 allocator test cases |
| `pkg/exchange/bingx/adapter_test.go` | **new** | 5 adapter test cases |
| `internal/api/config_handlers_test.go` | extend | Round-trip test for new field |
| `internal/engine/exchange_manager_test.go` | **new** | 4 snapshot/reload test cases |

## Open Questions (v3 — all resolved)

1. ~~Exact BingX hard-gate location~~ → resolved: insert at `allocator.go:1586` (after `movedToSpot = moveAmt`, before donor-balance read)
2. ~~Dashboard config path~~ → `web/src/pages/Config.tsx` confirmed; i18n files `web/src/i18n/{en,zh-TW}.ts` added to plan
3. ~~exchange_manager reload design~~ → resolved: **rebuild-on-flag-change** (matches existing snapshot pattern, simpler than in-place mutation)

## Review History
- v1: initial draft — codex NEEDS-REVISION (items 3, 4, 6, 7, 8, 9, 10, 11, 12)
- v2: addressed 9 findings — codex NEEDS-REVISION (items 3, 4, 7, 11) — 7 PASS
- v3: addressed 4 findings — codex NEEDS-REVISION (items 4, 11) — 10 PASS
- v4: **this version** — addresses final 2 findings:
  - #4: i18n namespace corrected from `config.*` to `cfg.field.*` / `cfg.desc.*` (matches existing convention at `web/src/i18n/en.ts:300, :407, :456`)
  - #11: explicit cold-start wiring added — edit `cmd/main.go:571-576` bingx case in `exchangeConfig()`; hot reload path via `buildExchangeConfig` is separate and already covered in v3
