# Plan: Fix Bybit 131212 and BingX 100437 Rebalance Transfer Bugs

Version: v5 (MINIMAL — supersedes v1-v4)
Date: 2026-04-16
Status: DRAFT

## Why v5 replaces v4

v4 was over-engineered (~300 lines + UI toggle + i18n + feature flag + exchange_manager rebuild + 4 test files). Codex review at task `dcb2f822` concluded:
- v4 dashboard/feature-flag infrastructure is "rollout productization work, not bugfix essence"
- Bybit fix can be a minimal try-catch (functionally equivalent to v4's dual-bucket abstraction)
- BingX fix needs endpoint migration + localized visibility handling, but no global retry code addition

v5 ships only what the incident requires.

## Bug summary (verified across 8+ prior dispatches)

**Bybit 131212** (2026-04-16 05:46:43 UTC): Cross-exchange deposit of 79.09 USDT arrived in Bybit UNIFIED account (user has Set Deposit Account pointing to UNIFIED). Our allocator confirmed arrival via `GetFuturesBalance()` (correct observation), then unconditionally called `TransferToFutures` which moves FUND→UNIFIED. FUND was empty (deposit never touched it) → 131212 "insufficient balance". **Funds were already tradable; the failed transfer was redundant.**

**BingX 100437** (2026-04-16 05:45:17 UTC): Donor path. `TransferToSpot` succeeded via undocumented legacy endpoint `/openApi/api/v3/post/asset/transfer` (type=`PFUTURES_FUND`). Then `Withdraw(walletType=1)` failed with 100437. Rollback `FUND_PFUTURES` succeeded — proving funds are in some fund-like bucket that walletType=1 can't see. Documented modern endpoint is `/openApi/api/asset/v1/transfer` with explicit `fromAccount`/`toAccount` string names.

## Changes

### 1. Bybit benign-131212 handler (~10 lines)

**File**: `internal/engine/allocator.go`
**Location**: lines 1938-1954 (the `recipientExch.TransferToFutures(...)` call after deposit confirmation)

**Before (current)**:
```go
transferStr := fmt.Sprintf("%.4f", totalPending)
if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
    e.log.Error("rebalance: %s spot->futures failed: %v", recipient, err)
} else {
    // ... balance accounting
}
```

**After (v5)**:
```go
transferStr := fmt.Sprintf("%.4f", totalPending)
if err := recipientExch.TransferToFutures("USDT", transferStr); err != nil {
    // Bybit 131212 special case: deposit may have auto-routed to UNIFIED
    // via user's Set Deposit Account setting. If trading balance already
    // holds the expected amount, treat the redundant FUND->UNIFIED failure
    // as benign and proceed.
    if strings.Contains(err.Error(), "131212") {
        if bal, berr := recipientExch.GetFuturesBalance(); berr == nil && bal != nil {
            if bal.Available >= startBal + totalPending*0.9 {
                e.log.Info("rebalance: %s 131212 benign — deposit already in trading pool (bal=%.2f)", recipient, bal.Available)
                err = nil  // treat as success
            }
        }
    }
    if err != nil {
        e.log.Error("rebalance: %s spot->futures failed: %v", recipient, err)
    }
}
if err == nil {
    e.log.Info("rebalance: %s spot->futures %s USDT (rebalance deposit)", recipient, transferStr)
    e.recordTransfer(recipient+" spot", recipient, "USDT", "internal", transferStr, "0", "", "completed", "rebalance-recv")
    // balance accounting (unchanged)
    bi := balances[recipient]
    bi.futures += totalPending
    bi.futuresTotal += totalPending
    bi.spot -= totalPending
    if bi.spot < 0 { bi.spot = 0 }
    balances[recipient] = bi
}
```

**Why this is enough**:
- The expensive part (cross-exchange withdraw + on-chain deposit) already succeeded
- Bybit UNIFIED is trading-usable as-is
- 131212 from a FUND→UNIFIED call when UNIFIED already has funds is operationally benign
- No new interface, no marker abstraction, no flag — just a targeted error-code recovery

**Safety**: the 131212 branch only triggers on the exact error string AND requires UNIFIED balance verification before treating as success. If UNIFIED doesn't have the funds (real "sum is truly insufficient"), error is still propagated.

### 2. BingX endpoint migration (~20 lines)

**File**: `pkg/exchange/bingx/adapter.go`
**Location**: lines 734-762 (`TransferToSpot`, `TransferToFutures`)

**Before (current)**:
```go
func (a *Adapter) TransferToSpot(coin string, amount string) error {
    params := map[string]string{
        "type":   "PFUTURES_FUND",
        "asset":  coin,
        "amount": amount,
    }
    _, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
    // ...
}

func (a *Adapter) TransferToFutures(coin string, amount string) error {
    params := map[string]string{
        "type":   "FUND_PFUTURES",
        "asset":  coin,
        "amount": amount,
    }
    _, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
    // ...
}
```

**After (v5)**:
```go
func (a *Adapter) TransferToSpot(coin string, amount string) error {
    params := map[string]string{
        "fromAccount": "USDTMPerp",
        "toAccount":   "fund",
        "asset":       coin,
        "amount":      amount,
    }
    _, err := a.client.Post("/openApi/api/asset/v1/transfer", params)
    if err != nil {
        return fmt.Errorf("bingx TransferToSpot: %w", err)
    }
    return nil
}

func (a *Adapter) TransferToFutures(coin string, amount string) error {
    params := map[string]string{
        "fromAccount": "fund",
        "toAccount":   "USDTMPerp",
        "asset":       coin,
        "amount":      amount,
    }
    _, err := a.client.Post("/openApi/api/asset/v1/transfer", params)
    if err != nil {
        return fmt.Errorf("bingx TransferToFutures: %w", err)
    }
    return nil
}
```

**Why this is enough**:
- Documented endpoint per `doc/bingx/bingx-other-api-docs.md:106-149` and BingX live SPA bundle
- Uses explicit account names (`USDTMPerp`, `fund`) that match `walletType=1` semantics
- Direct replacement — same signature, same caller behavior
- No feature flag because: the new endpoint is the official one; rolling back to legacy would revert to known-broken

**Safety**: if new endpoint returns error, it propagates the same way legacy errors do. No behavior change for successful calls.

### 3. Localized post-transfer poll — BingX donor path (~15 lines)

**File**: `internal/engine/allocator.go`
**Location**: insert after line 1585 (`movedToSpot = moveAmt`), before line 1588 (donor balance read)

Only applies when donor is BingX. Protects against visibility lag between `TransferToSpot` return and fund balance becoming withdrawable.

```go
// After successful TransferToSpot on BingX donor, verify fund visibility
// before building the withdraw to avoid 100437 caused by balance lag.
if bestDonor == "bingx" && movedToSpot > 0 {
    pollDeadline := time.Now().Add(15 * time.Second)
    required := movedToSpot * 0.99
    // Baseline snapshot BEFORE the transfer above; we only added movedToSpot,
    // so delta against pre-transfer balance must cover required.
    baselineFund := balances[bestDonor].spot
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
            e.log.Error("rebalance: bingx rollback failed: %v", rbErr)
        }
        surplus[bestDonor] = 0
        movedToSpot = 0
        continue
    }
}
```

**Why this is enough**:
- Only touches BingX donor path; other exchanges unaffected
- Hard-gates withdraw so 100437 from balance lag never happens
- Matches existing continue-on-error pattern at `:1581-1583, :1597-1600`

### 4. Observability (~2 lines)

**File**: `internal/engine/allocator.go`
**Location**: line 1927 (existing `"all deposits confirmed"` log)

Extend log to include trading-pool balance for diagnosis:

```go
e.log.Info("rebalance: all deposits confirmed on %s (fundBal=%.2f, startBal=%.2f)", recipient, spotBal.Available, startBal)
```

(Change: add the startBal context; the full dual-bucket log comes naturally from the 131212 handler's INFO line.)

### 5. Tests (~60 lines, 2 files)

**New file `internal/engine/allocator_bybit_131212_test.go`**:
- Test 1: TransferToFutures fails with 131212 but UNIFIED has the expected delta → treated as success, balance accounting correct
- Test 2: TransferToFutures fails with 131212 and UNIFIED does NOT have delta → error propagated
- Test 3: TransferToFutures fails with a non-131212 error → error propagated unchanged

**New file `pkg/exchange/bingx/adapter_transfer_test.go`**:
- Test A: `TransferToSpot` POSTs to `/openApi/api/asset/v1/transfer` with `fromAccount=USDTMPerp`, `toAccount=fund`
- Test B: `TransferToFutures` POSTs to same endpoint with `fromAccount=fund`, `toAccount=USDTMPerp`

Use existing mock HTTP client pattern if available; otherwise table-driven test with `httptest.NewServer`.

## Files Modified

| File | Change | Lines |
|------|--------|-------|
| `internal/engine/allocator.go` | Bybit 131212 handler + BingX donor post-transfer poll + observability | ~30 |
| `pkg/exchange/bingx/adapter.go` | Replace legacy endpoint in TransferToSpot + TransferToFutures | ~20 |
| `internal/engine/allocator_bybit_131212_test.go` | **new** | ~40 |
| `pkg/exchange/bingx/adapter_transfer_test.go` | **new** | ~40 |
| **Total** | | **~130** |

## What's NOT in v5 (deferred/rejected)

| Feature | Rationale for not including |
|---------|----------------------------|
| Feature flag for BingX endpoint | New endpoint is the official documented one; fallback would revert to known-broken path. No grey-area rollout needed. |
| Dashboard toggle + i18n | Nothing to toggle. Users don't need to see this. |
| exchange_manager rebuild wiring | No flag to react to. |
| `DepositMayCreditTradingDirectly` marker interface | Over-abstraction for one incident path. The try-catch is functionally equivalent and doesn't leak a new exchange-semantic concept. |
| Remove `pendingStartBal` | Not touched by v5; leave existing code alone (minimize churn). |
| Per-exchange deposit landing semantic table | Useful for future reference, but the v5 code doesn't rely on it. |

## Risk Table

| ID | Risk | Mitigation |
|----|------|-----------|
| R1 | 131212 handler treats unrelated insufficiency as success | Additional `GetFuturesBalance` check requires actual balance ≥ threshold; false positives only happen if UNIFIED coincidentally has unrelated funds matching the deposit, which is unlikely |
| R2 | BingX new endpoint differs in unknown way | Direct replacement, docs verified in `doc/bingx/*.md` and live SPA bundle |
| R3 | Visibility poll times out on real failures | Hard gate with rollback preserves donor state; unfunded recipient is logged |
| R4 | Rollback TransferToFutures after BingX visibility timeout itself fails | Already-handled pattern at `:1819-1847` |

## Testing Plan

1. Unit tests for all 5 scenarios (2 files, ~5 test cases)
2. `go build ./cmd/main.go` — verify compiles
3. Deploy to VPS; trigger manual rebalance with Bybit recipient → observe 131212-benign path logs
4. Trigger rebalance with BingX donor → observe new endpoint + visibility poll logs
5. Regression check: one rebalance with binance/okx/gate/bitget recipient/donor to confirm no side effects

## Rollout

Single release — no staged flag rollout. Deploy directly after Phase 5 (post-implementation review) passes.

## Review History
- v1–v4: progressively reviewed by codex; eventually ALL PASS at v4 but user flagged v4 as over-engineered
- v5: **this version** — reduced to minimum viable fix per codex dispatch `dcb2f822` recommendation
