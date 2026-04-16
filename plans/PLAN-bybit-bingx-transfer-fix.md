# Plan: Fix Bybit 131212 and BingX 100437 Rebalance Transfer Bugs

Version: v2
Date: 2026-04-16
Status: DRAFT (addressing v1 review findings)

## Review History

- **v1** (2026-04-16 10:45 UTC): codex review — NEEDS-REVISION items 3, 4, 6, 7, 8, 9, 10, 11, 12
  - #3 BingX migration insufficient (soft-poll then withdraw anyway)
  - #4 Feature flag wiring incomplete (missing API handlers + config round-trip tests)
  - #6 `pendingStartBal` should be removed not kept
  - #7 **Highest severity**: removing `IsUnified()` branch unsafe for OKX — OKX unified still requires explicit funding→trading transfer (`pkg/exchange/okx/adapter.go:995-1011`)
  - #8 R6 accounting invariant broken for OKX unified
  - #9 Post-transfer poll policy under-specified (hard gate vs telemetry)
  - #10 `pkg/exchange/bingx/adapter_test.go` doesn't exist — can't "append"
  - #11 Missing dashboard/API exposure + hot-reload wiring via `internal/engine/exchange_manager.go`
  - #12 R3 references nonexistent 100437 retry logic; R5 false claim about 6-exchange safety

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
// New interface (pkg/exchange/exchange.go, alongside existing marker interfaces)
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

**File**: `internal/engine/allocator.go` — donor-side hard-gate poll before withdraw

Add in the donor-processing path after `TransferToSpot` succeeds and before `Withdraw` is called (approximate location: around lines 1577-1596 where `TransferToSpot` is invoked; exact location TBD by inspection):

```go
// After successful TransferToSpot on BingX donor, HARD-GATE poll for fund visibility.
// If poll times out, abort the withdraw and record a skip reason.
if donor == "bingx" {
    required := movedToSpot * 0.99  // 1% tolerance
    startFund := balances[donor].spot  // baseline BEFORE transfer
    pollDeadline := time.Now().Add(15 * time.Second)
    visible := false
    for time.Now().Before(pollDeadline) {
        time.Sleep(2 * time.Second)
        if fb, err := e.exchanges[donor].GetSpotBalance(); err == nil && fb != nil {
            if fb.Available - startFund >= required {
                visible = true
                break
            }
        }
    }
    if !visible {
        e.log.Warn("rebalance: bingx fund balance not visible after transfer (expected +%.4f); aborting withdraw to avoid 100437", movedToSpot)
        result.SkipReasons[bw.recipient] = "bingx fund visibility timeout"
        result.Unfunded[bw.recipient] = bw.netTotal
        // Rollback the futures→spot move
        if rbErr := e.exchanges[donor].TransferToFutures("USDT", fmt.Sprintf("%.4f", movedToSpot)); rbErr != nil {
            e.log.Error("rebalance: bingx rollback after visibility timeout failed: %v", rbErr)
        }
        continue  // skip this withdraw iteration
    }
}
```

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

**File**: `internal/engine/exchange_manager.go`
- Field `bingxNewTransferAPI bool` in snapshot struct (~line 20-23)
- In reload comparison (~line 157-205): detect change in this field
- On change: call `adapter.SetUseNewTransferAPI(newValue)` without full adapter rebuild (it's a runtime-adjustable flag, not a credential)
- Wire initial value in `cmd/main.go newExchange()` and BingX-specific construction (~line 371-379, 518-531)

**Dashboard UI** (`web/src/pages/Config.tsx` or equivalent — exact file TBD):
- Add toggle "Enable BingX New Transfer API (migrate from legacy v3 endpoint)"
- Default OFF
- Save via existing config POST handler

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

**Extend exchange_manager tests** (`internal/engine/exchange_manager_test.go` or similar, exact file TBD):
- Test reload flips the BingX new transfer API flag without full rebuild
- Test flag change is propagated to the live adapter instance

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

## Files Modified Summary

| File | Type | What |
|------|------|------|
| `pkg/exchange/exchange.go` | interface | Add `DepositMayCreditTradingDirectly` marker interface |
| `pkg/exchange/bybit/adapter.go` | method add | Implement `DepositMayCreditTradingDirectly() bool { return true }` |
| `pkg/exchange/gateio/adapter.go` | method add | Implement `DepositMayCreditTradingDirectly() bool { return a.isUnified }` |
| `pkg/exchange/bingx/adapter.go` | field + 2 methods modified | Feature-flagged TransferToSpot/TransferToFutures + SetUseNewTransferAPI |
| `internal/engine/allocator.go` | replace 1888-1954 + add donor hard-gate poll | Dual-bucket detection + BingX poll |
| `internal/config/config.go` | field add | `EnableBingXNewTransferAPI` |
| `internal/api/handlers.go` | wiring | GET/POST `/api/config` includes new field |
| `internal/engine/exchange_manager.go` | snapshot + reload | Propagate flag to live BingX adapter without rebuild |
| `cmd/main.go` | construction | Pass initial flag value to BingX adapter |
| `web/src/pages/Config.tsx` (or equiv) | UI toggle | Dashboard toggle for new transfer API |
| `internal/engine/allocator_deposit_test.go` | new | 6 allocator test cases |
| `pkg/exchange/bingx/adapter_test.go` | **new** (does not currently exist) | 5 adapter test cases |
| `internal/api/config_handlers_test.go` | extend | Round-trip for new field |
| `internal/engine/exchange_manager_test.go` | extend (if exists) | Flag reload without rebuild |

## Open Questions

1. Exact location in allocator for donor-side hard-gate poll — need to inspect `internal/engine/allocator.go:1577-1596` and `:1721-1735` during implementation. Plan assumes post-`TransferToSpot` before `Withdraw`.
2. Dashboard config file — need to confirm `web/src/pages/Config.tsx` path and i18n structure for new toggle label.
3. Whether `exchange_manager.go` reload path supports in-place flag update or requires adapter rebuild. If rebuild needed, fall back to restart-only rollout and document it.

## Review History
- v1: initial draft — codex returned NEEDS-REVISION (items 3, 4, 6, 7, 8, 9, 10, 11, 12)
- v2: **this version** — addresses all 9 findings: narrowed scope with marker interface, removed pendingStartBal, hard-gate BingX poll, full config wiring, corrected R3/R5, new test file for bingx (not append), added exchange_manager hot-reload, per-exchange semantic table, corrected skip-transfer accounting
