# Plan: Fix Bybit 131212 and BingX 100437 Rebalance Transfer Bugs

Version: v1
Date: 2026-04-16
Status: DRAFT

## Context

Two production incidents on 2026-04-16 05:45-05:48 UTC during rebalance cycle for ARIAUSDT:

**Bybit 131212**: After binance→bybit deposit of 79.09 USDT confirmed, allocator called `TransferToFutures(FUND→UNIFIED)` which failed with `131212 Insufficient balance`. Log evidence: `GetFuturesBalance` (UNIFIED) went 46.69→125.79 (delta exactly matches deposit), meaning the deposit landed directly in UNIFIED. FUND was empty, so the follow-up FUND→UNIFIED transfer had nothing to move.

**BingX 100437**: After `TransferToSpot` (PFUTURES_FUND) succeeded for 14.04 USDT, withdraw with `walletType=1` failed with `100437 Insufficient balance. Deposit or switch to another account to proceed`. Rollback (FUND_PFUTURES) from the same endpoint succeeded — proving funds were in some fund-like bucket but not visible to walletType=1 withdraw.

Both bugs traced to commit `0d1e286d` (2026-03-28).

Prior investigation: docs/research at dispatch task `a3b1aa13` verified by codex.

## Changes

### 1. internal/engine/allocator.go (Bybit fix — allocator-local)

- `allocator.go:1888-1899` — replace single-bucket baseline capture with dual-bucket capture
- `allocator.go:1909-1954` — replace single-bucket poll with dual-bucket poll; decide whether to call `TransferToFutures` based on which bucket gained the delta

### 2. pkg/exchange/bingx/adapter.go (BingX fix — adapter migration)

- `adapter.go:734-747` — rewrite `TransferToSpot` to use documented endpoint `POST /openApi/api/asset/v1/transfer` with `fromAccount=USDTMPerp toAccount=fund` (behind feature flag; old path retained as fallback)
- `adapter.go:749-762` — rewrite `TransferToFutures` similarly with `fromAccount=fund toAccount=USDTMPerp`
- `adapter.go:764-788` — add post-transfer fund balance poll inside `Withdraw` (or let caller handle — see decision in Risk section)

### 3. internal/config/config.go (feature flag)

- Add `EnableBingXNewTransferAPI bool` field (default `false` for safe rollout)
- JSON tag: `enable_bingx_new_transfer_api`
- Env var: `ENABLE_BINGX_NEW_TRANSFER_API`

### 4. Observability (log both bucket deltas)

- `allocator.go` confirmation loop: log `fundDelta` and `unifiedDelta` on each successful confirmation so we can diagnose future mismatches from logs alone

### 5. Unit tests

- `internal/engine/allocator_deposit_test.go` (new file): tests for dual-bucket detection with fake exchange stub
- `pkg/exchange/bingx/adapter_test.go` (append): tests verifying new endpoint path/params when flag enabled

## Why

### Bybit 131212 root cause (confidence ~95% on fix direction, ~85% on mechanism)

- Allocator code assumes `IsUnified()=true` means deposit lands in the unified/trading pool (`allocator.go:1914` comment: "Unified accounts: deposits land in unified pool, poll via GetFuturesBalance").
- For this user's account, cross-exchange deposits land in UNIFIED directly (likely because Set Deposit Account was configured to UNIFIED, but Bybit provides NO API to query this setting — verified 2026-04-16 that `query-deposit-acct` endpoint returns 404; only `set-deposit-acct` exists).
- Allocator confirms via UNIFIED delta (correct observation), but then unconditionally calls `TransferToFutures` which always moves FUND→UNIFIED (`pkg/exchange/bybit/adapter.go:885-893`). FUND is empty → 131212.

Fix: stop assuming static deposit destination; sample both FUND and UNIFIED pre-deposit, poll both during wait, only call `TransferToFutures` if delta appeared in FUND.

### BingX 100437 root cause (confidence ~85% on fix direction, ~55% on specific "legacy bucket" mechanism)

- Code uses `POST /openApi/api/v3/post/asset/transfer` with `type=PFUTURES_FUND`/`FUND_PFUTURES`. This endpoint is NOT in the current BingX live SPA docs bundle (verified 2026-04-16).
- Documented current endpoint is `POST /openApi/api/asset/v1/transfer` using `fromAccount`/`toAccount` string names (e.g., `USDTMPerp`, `fund`, `spot`).
- Withdraw uses `walletType=1` (Fund Account per `doc/bingx/bingx-spot-api-docs.md:964`).
- Two unresolved alternatives:
  1. v3 transfer credits a different bucket than walletType=1
  2. pure visibility lag (2.2s between transfer and withdraw)
- Either way, migrating to documented endpoint + adding post-transfer poll addresses both.

Fix: migrate endpoint (guarded by feature flag for safe rollout) + add post-transfer fund balance poll before withdraw.

### Observability rationale

Current logs show only one bucket's balance, making it impossible to retroactively distinguish Bybit Set Deposit Account routing vs BingX bucket mismatch. Logging both deltas is zero-risk and lets us diagnose future incidents from logs alone.

## How

### 1. allocator.go — dual-bucket deposit confirmation

Replace lines 1888-1954 pseudocode:

```go
// New type (local to function or file-level helper)
type bucketBaseline struct {
    fund, unified float64
}

// Baseline capture (replaces 1888-1899)
if _, exists := pendingStartBal[bw.recipient]; !exists {
    fundBal, fErr := e.exchanges[bw.recipient].GetSpotBalance()
    futBal, uErr := e.exchanges[bw.recipient].GetFuturesBalance()
    baseline := bucketBaseline{}
    if fErr == nil && fundBal != nil {
        baseline.fund = fundBal.Available
    } else {
        baseline.fund = balances[bw.recipient].spot
    }
    if uErr == nil && futBal != nil {
        baseline.unified = futBal.Available
    }
    pendingBaselines[bw.recipient] = baseline
    pendingStartBal[bw.recipient] = baseline.fund  // preserve old field for existing log
}
pendingDeposits[bw.recipient] += recipientReceives

// Confirmation loop (replaces 1909-1954)
for recipient, totalPending := range pendingDeposits {
    if totalPending <= 0 { continue }
    recipientExch := e.exchanges[recipient]
    baseline := pendingBaselines[recipient]
    threshold := totalPending * 0.9
    e.log.Info("rebalance: waiting for %.2f USDT total deposits on %s (fundStart=%.2f unifiedStart=%.2f)...",
        totalPending, recipient, baseline.fund, baseline.unified)

    var arrivedBucket string  // "fund" or "unified" or ""
    var fundDelta, unifiedDelta float64
    pollDeadline := time.Now().Add(5 * time.Minute)
    for time.Now().Before(pollDeadline) {
        time.Sleep(5 * time.Second)
        fundNow, fErr := recipientExch.GetSpotBalance()
        futNow, uErr := recipientExch.GetFuturesBalance()
        if fErr != nil && uErr != nil { continue }
        if fundNow != nil { fundDelta = fundNow.Available - baseline.fund }
        if futNow != nil { unifiedDelta = futNow.Available - baseline.unified }
        if fundDelta >= threshold {
            arrivedBucket = "fund"; break
        }
        if unifiedDelta >= threshold {
            arrivedBucket = "unified"; break
        }
    }

    if arrivedBucket == "" {
        e.log.Warn("rebalance: deposits on %s not confirmed within 5min (fundDelta=%.2f unifiedDelta=%.2f), skipping",
            recipient, fundDelta, unifiedDelta)
        continue
    }

    e.log.Info("rebalance: deposits confirmed on %s via %s bucket (fundDelta=%.2f unifiedDelta=%.2f)",
        recipient, arrivedBucket, fundDelta, unifiedDelta)

    if arrivedBucket == "fund" {
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
    } else {
        // arrivedBucket == "unified" — deposit already available for trading, skip transfer
        e.log.Info("rebalance: %s deposit already in unified pool, skipping spot->futures", recipient)
        bi := balances[recipient]
        bi.futures += totalPending
        bi.futuresTotal += totalPending
        balances[recipient] = bi
    }
}
```

Supporting structural changes:
- Add `pendingBaselines map[string]bucketBaseline` alongside `pendingStartBal`/`pendingDeposits`
- Keep `pendingStartBal` for backward compat with any other reader (only `bucketBaseline.fund` path uses it now)

### 2. bingx/adapter.go — endpoint migration

Feature-flagged migration:

```go
// Adapter has access to config via existing pattern (check how other flags are wired).
// If not already available, pass flag via Adapter struct field populated at construction.

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
    // Legacy path (default until flag enabled in production)
    params := map[string]string{
        "type":   "PFUTURES_FUND",
        "asset":  coin,
        "amount": amount,
    }
    _, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
    if err != nil {
        return fmt.Errorf("bingx TransferToSpot: %w", err)
    }
    return nil
}

// Symmetric change for TransferToFutures:
// new: fromAccount=fund, toAccount=USDTMPerp
// legacy: type=FUND_PFUTURES
```

**Post-transfer poll (callers side, not inside adapter)**:

The allocator's withdraw path (upstream of `TransferToSpot` call for BingX donor) should add a short fund balance check before withdrawing. Exact location: in the rebalance donor-processing path where `TransferToSpot` is called before `Withdraw`. Pseudocode:

```go
// After TransferToSpot succeeds on BingX:
if donor == "bingx" {
    pollDeadline := time.Now().Add(15 * time.Second)
    for time.Now().Before(pollDeadline) {
        time.Sleep(2 * time.Second)
        if fb, err := e.exchanges[donor].GetSpotBalance(); err == nil {
            if fb.Available >= movedAmount*0.99 {
                break
            }
        }
    }
}
```

**Decision**: the post-transfer poll belongs in the allocator's batched-withdraw orchestration, not inside the adapter. Adapter stays thin. Exact location TBD during implementation — need to inspect the withdraw pipeline around `allocator.go:1855` area and co-locate with existing futures→spot tracking.

### 3. config.go — feature flag

Follow existing pattern (see `EnablePoolAllocator`, `EnableLiqTrendTracking`):

```go
// In Config struct (appropriate section)
EnableBingXNewTransferAPI bool `json:"enable_bingx_new_transfer_api"`

// In Load() env var mapping
if v := os.Getenv("ENABLE_BINGX_NEW_TRANSFER_API"); v != "" {
    c.EnableBingXNewTransferAPI = v == "true" || v == "1"
}

// Default: false (unset)
```

Wire flag into BingX adapter at construction (in `cmd/main.go` `newExchange` switch):
```go
case "bingx":
    adapter := bingx.NewAdapter(...)
    adapter.SetUseNewTransferAPI(cfg.EnableBingXNewTransferAPI)
    return adapter
```

Or pass via constructor — whichever matches existing adapter pattern.

### 4. Tests

New file `internal/engine/allocator_deposit_test.go`:

```go
// Fake exchange with scripted balance progression
type fakeDepositExchange struct {
    fundBalances    []float64  // progression
    unifiedBalances []float64
    transferCalled  bool
    pollIdx         int
}

// Test cases:
// 1. Deposit lands in FUND → TransferToFutures called, unified delta matches post-transfer
// 2. Deposit lands in UNIFIED → TransferToFutures NOT called, unified balance already has funds
// 3. Deposit never arrives → timeout, no transfer attempt, position marked unfunded
// 4. Partial delta (below threshold) → continues polling
```

New tests in `pkg/exchange/bingx/adapter_test.go`:

```go
// Use mock client (existing test pattern):
// 1. TransferToSpot with useNewTransferAPI=true sends fromAccount=USDTMPerp toAccount=fund
// 2. TransferToSpot with useNewTransferAPI=false sends type=PFUTURES_FUND (legacy)
// 3. Symmetric for TransferToFutures
// 4. Verify request path is /openApi/api/asset/v1/transfer when flag on
```

## Risk

### R1. Bybit dual-bucket poll race condition

If user's Bybit auto-routes deposits via a transient FUND → UNIFIED hop (e.g., deposit briefly appears in FUND then quickly moves), the 5-second polling interval might catch it in FUND, trigger TransferToFutures, which then races with Bybit's own auto-move.

**Mitigation**: threshold check requires ≥90% of deposit amount in the bucket. If auto-move happens faster than 5s poll, we'll likely see delta in UNIFIED (post-move state) and skip TransferToFutures. Safe either way.

### R2. BingX endpoint migration breaks something else

The old endpoint may still work for non-rebalance paths (e.g., spot-futures engine). Feature flag keeps old path default.

**Mitigation**:
- Default `EnableBingXNewTransferAPI=false`
- Manual test on VPS with flag enabled before making default
- Log which path was used on every transfer so we can diff behavior

### R3. BingX post-transfer poll timeout too short

15-second poll may not catch eventual consistency delays beyond that window.

**Mitigation**: if poll times out but no error, still attempt withdraw — existing 100437 retry logic can catch actual failures. Poll is a soft guard, not a hard gate.

### R4. allocator.go structural change affects other paths

The deposit confirmation loop is used only by the rebalance allocator. Other engines (spot-futures) have their own transfer flows.

**Mitigation**: grep confirms only `allocator.go` has this pattern. Other callsites use direct `TransferToFutures` without the poll-and-confirm wrapper.

### R5. IsUnified() branch removal affects non-Bybit exchanges

Current code treats only Bybit/Gate as `IsUnified()=true`. Gate.io's `TransferToFutures` is documented as a no-op (per codex verification). Removing the `IsUnified` branch and always running dual-bucket detection is safe because:
- Gate unified will see delta in "futures" (shared pool) → arrivedBucket="unified" → skip TransferToFutures → correct
- Non-unified exchanges (binance/okx/bitget/bingx) have distinct FUND/SPOT → arrivedBucket="fund" → TransferToFutures called → correct

**Mitigation**: verify each of the 6 exchanges' `GetSpotBalance` / `GetFuturesBalance` semantics in adapter code before implementation. Document each exchange's mapping in a comment.

### R6. Side effect on `balances[recipient]` accounting

Currently the code credits `bi.futures += totalPending` and debits `bi.spot -= totalPending` post-TransferToFutures. The new "skip transfer" path credits `bi.futures` but doesn't debit `bi.spot` (because spot was never debited — deposit bypassed it).

**Mitigation**: ensure the new branch's balance accounting matches observed reality. Add an invariant assertion or log for `bi.spot < 0` cases.

### R7. Observability log spam

Adding fund/unified delta logs on every rebalance could add noise.

**Mitigation**: use INFO only for confirmation event (once per recipient per rebalance), DEBUG for per-poll iterations.

## Testing Plan

1. **Unit tests** (Phase 4): allocator_deposit_test.go + bingx/adapter_test.go
2. **Compile check** (Phase 4): `go build ./cmd/main.go`
3. **Live smoke test** (post-Phase 5):
   - Deploy to VPS with `EnableBingXNewTransferAPI=false`
   - Trigger a manual rebalance; verify Bybit path logs both bucket deltas
   - Verify Bybit fix: expect log line showing UNIFIED delta, no 131212
   - Flip flag to `true` and trigger rebalance involving BingX donor
   - Verify BingX new endpoint succeeds; no 100437
4. **Regression check**: ensure other 4 exchanges (binance/okx/gate/bitget) still work via existing paths

## Rollout

1. Merge with `EnableBingXNewTransferAPI=false` (default)
2. Live test Bybit fix on VPS (high priority — currently broken)
3. Enable `EnableBingXNewTransferAPI=true` after confirming no regressions
4. If new BingX path stable for 24h, remove legacy path in v0.33.x

## Review History
- v1: initial draft — pending codex review
