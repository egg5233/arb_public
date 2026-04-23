# Wait-For-Spot-Balance Implementation Plan

**Goal:** Add a 10-second poll after `TransferToSpot` in the rebalance executor so split-account donor withdrawals (binance/bitget etc.) no longer fail silently when the internal futures→spot transfer has not yet reflected in spot balance. Fixes the observed 02:35 2026-04-23 production incident where binance→bingx withdraw was skipped with `netAmount=-0.10`, leaving bingx unfunded and SIGNUSDT entry rejected.

**Architecture:** Insert a single call to a new `waitForSpotBalance(exch, required, timeout)` helper between `TransferToSpot` (allocator.go:1914) and the spot-balance read (allocator.go:1923-1931). The helper polls `GetSpotBalance()` every 1 second until `Available >= required` or the 10-second timeout elapses. On timeout, the donor is dropped (same skip semantics already present at :1967-1970). Unified-account donors skip the entire `TransferToSpot` branch, so the wait is only added inside the split-account `!skipOuterTransfer` block.

**Tech Stack:** Go 1.26, `internal/engine/allocator.go`, `pkg/exchange/exchange.go` (`Exchange` interface already defines `GetSpotBalance`). Go stdlib `time`, `testing`.

---

**Version:** v5
**Date:** 2026-04-23
**Status:** REVIEWING

## Context — Observed incident

2026-04-23 02:35 UTC production log on VPS (v0.33.2 binary):

```
[engine] rebalance: pass1 case(b) top-up rescue-candidate SIGNUSDT bybit/bingx topUp=map[bingx:83.5 bybit:200]
[engine] dryRun: donor binance futures→spot 120.75 (need=120.75)
[engine] dryRun: moved 120.6941 from binance to bingx (netCredit=120.6941)
[engine] dryRun: donor okx futures→spot 199.98 (need=199.98)
[engine] dryRun: moved 199.9809 from okx to bybit (netCredit=199.9809)
[engine] dryRun: end — feasible=true steps=2 totalFee=0.1020
...
[engine] rebalance: binance withdraw fee=0.1000 minWd=10.0000 USDT via APT (planned)
[engine] rebalance: binance spot balance too low to withdraw (netAmount=-0.1000, fee=0.1000)   ← SKIP
[engine] rebalance: batched withdraw okx->bybit net=199.86 fee=0.0020 amount=199.86 via APT
[engine] rebalance: withdraw from okx txid=395562268 -> bybit                                   ← SUCCESS
[engine] rebalance: complete (unfunded=2 skipReasons=2 pendingDeposits=1)
[engine] rebalance: pass1 execute local=false cross=true funded=map[bybit:199.86] unfunded=map[bingx:120.65] skip=map[bingx:planned steps exhausted] kept=0/1
```

Outcome: bybit leg funded (okx→bybit 199.86 landed by 02:45); bingx leg never received its 120 USDT because binance's `TransferToSpot` returned before the spot balance reflected the move, so the immediately-following `GetSpotBalance()` returned 0, which caused `netAmount = 0 - 0.10 = -0.10` and the withdraw was skipped.

2026-04-23 03:15 live diagnostic via `cmd/gatewithdrawdebug` confirmed the binance adapter itself is not the bug — the bug is that `executeRebalanceFundingPlan` treats `TransferToSpot` as synchronously settled when it is actually a fire-and-forget POST.

Codex independent audit (dispatch task `b1f1og3j7`, gpt-5.4 xhigh) confirmed BUG and recommended a `waitForSpotBalance` helper with poll/timeout as the minimal fix. OKX worked by chance because its Withdraw() drains funding directly without a separate futures→spot step, not because the pipeline is safe.

## File Structure

- **Modify** `internal/engine/allocator.go` — add `waitForSpotBalance` helper (near other `e.exchanges[...]` helpers or just above `executeRebalanceFundingPlan`). Insert one call between line 1918 and line 1923.
- **Create** `internal/engine/wait_spot_balance_test.go` — regression test with a stub exchange whose `GetSpotBalance` returns `0` on the first N polls and then returns the expected amount, proving the wait correctly polls and returns success within the timeout.

No interface changes in `pkg/exchange/exchange.go`; `GetSpotBalance()` is already on the `Exchange` interface. No adapter changes needed — this is purely executor-side.

---

## Task 1: Add `waitForSpotBalance` helper to Engine

**Files:**
- Modify: `internal/engine/allocator.go` — add new method on `*Engine`

- [ ] **Step 1: Locate insertion point**

The helper goes on `*Engine`. Pick a stable spot near the other private helpers in `allocator.go` (e.g. right before `executeRebalanceFundingPlan` definition, or at the top of the `allocator.go` helper-method block). Exact line does not matter for callers; the helper is private to this file.

- [ ] **Step 2: Add the helper**

Add:

```go
// waitForSpotBalance polls donor.GetSpotBalance() every 1s until Available >=
// required or timeout elapses. Split-account exchanges (binance, bitget,
// okx classic, gate.io classic) post TransferToSpot asynchronously — the
// REST response confirms acceptance but the spot account balance only
// reflects the move after a short settlement lag. Without this wait, a
// subsequent GetSpotBalance at the caller can read stale balance and cause
// netAmount = -fee < 0 at the guard below, skipping the downstream withdraw.
//
// Returns nil once the target is met. Returns an error on timeout; the
// caller treats the error as a donor skip (surplus -> 0) and continues.
// Unified-account donors bypass the futures->spot step entirely so this
// helper is never called in their path.
func (e *Engine) waitForSpotBalance(donor string, required float64, timeout time.Duration) error {
    exch, ok := e.exchanges[donor]
    if !ok {
        return fmt.Errorf("waitForSpotBalance: donor %s not registered", donor)
    }
    deadline := time.Now().Add(timeout)
    const pollInterval = 1 * time.Second
    var lastAvail float64
    var lastErr error
    for {
        bal, err := exch.GetSpotBalance()
        if err == nil {
            lastAvail = bal.Available
            if bal.Available >= required {
                e.log.Debug("rebalance: %s spot balance reached %.4f (required=%.4f)", donor, bal.Available, required)
                return nil
            }
        } else {
            lastErr = err
        }
        if time.Now().After(deadline) {
            if lastErr != nil {
                return fmt.Errorf("waitForSpotBalance: timeout after %s waiting for %s spot>=%.4f (last err=%v)", timeout, donor, required, lastErr)
            }
            return fmt.Errorf("waitForSpotBalance: timeout after %s waiting for %s spot>=%.4f (last avail=%.4f)", timeout, donor, required, lastAvail)
        }
        time.Sleep(pollInterval)
    }
}

// captureSpotBalanceForTransfer reads the donor's current spot balance.
// Intended to be called BEFORE TransferToSpot so the caller can compute
// `preTransferSpot + sentAmount` as the post-transfer target passed to
// waitForSpotBalance. If GetSpotBalance errors, falls back to snapshotSpot
// (the rebalance-cycle-captured value) — never returns a "reading error"
// in normal operation; only the unknown-donor path can error out.
//
// Correctness note: this MUST happen before TransferToSpot. If it runs
// after, a partially-settled transfer can inflate preTransferSpot and
// cause target = inflated + sentAmount to exceed reality, triggering a
// false timeout.
func (e *Engine) captureSpotBalanceForTransfer(donor string, snapshotSpot float64) (float64, error) {
    exch, ok := e.exchanges[donor]
    if !ok {
        return 0, fmt.Errorf("captureSpotBalanceForTransfer: donor %s not registered", donor)
    }
    preBal, err := exch.GetSpotBalance()
    if err != nil {
        e.log.Debug("rebalance: %s preTransferSpot read failed (%v), fallback to snapshot %.4f", donor, err, snapshotSpot)
        return snapshotSpot, nil
    }
    return preBal.Available, nil
}
```

Design note: the pre-read and wait are TWO separate helpers deliberately, so the call site can interleave them with `TransferToSpot` in the correct order:

```
captureSpotBalanceForTransfer()  -> preTransferSpot    [BEFORE TransferToSpot]
TransferToSpot()                                        [fires the internal transfer]
waitForSpotBalance(preTransferSpot + sentAmount, 10s)  [AFTER TransferToSpot]
```

Merging them into a single helper that does pre-read internally would put the read AFTER `TransferToSpot` (because the helper runs synchronously AFTER its call site), which can double-count fast-settled transfers. Keep them split.

Also ensure `time` import is present at the top of `allocator.go` (it likely is already — verify with a grep).

- [ ] **Step 3: Compile check**

`go build ./internal/engine/`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/engine/allocator.go
git commit -m "engine(allocator): add waitForSpotBalance helper for split-account transfer settlement"
```

---

## Task 2: Call `waitForSpotBalance` after `TransferToSpot`

**Files:**
- Modify: `internal/engine/allocator.go:1918-1923` (insert one block between them)

- [ ] **Step 1: Exact BEFORE**

Current code at `internal/engine/allocator.go:1904-1923` (including the earlier `moveStr` zero-guard):

```go
                moveStr := fmt.Sprintf("%.4f", moveAmt)
                // Post-format zero guard: moveAmt can be a tiny positive (e.g. 0.00003)
                // that passes float checks but rounds to "0.0000" via %.4f. Exchanges
                // reject zero-amount transfers (bitget code=40020).
                if parsed, _ := strconv.ParseFloat(moveStr, 64); parsed <= 0 {
                    e.log.Warn("rebalance: %s moveAmt %.6f rounds to zero string, skipping", bestDonor, moveAmt)
                    surplus[bestDonor] = 0
                    continue
                }
                e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
                if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
                    e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
                    surplus[bestDonor] = 0
                    continue
                }
                e.recordTransfer(bestDonor, bestDonor+" spot", "USDT", "internal", moveStr, "0", "", "completed", "rebalance-prep")
                movedToSpot = moveAmt
            }

            // Unified accounts: spot balance is 0 (same pool as futures).
            // Use GetFuturesBalance to get the real withdrawable amount.
            var donorSpotBal *exchange.Balance
```

- [ ] **Step 2: Exact AFTER**

```go
                moveStr := fmt.Sprintf("%.4f", moveAmt)
                // Post-format zero guard: moveAmt can be a tiny positive (e.g. 0.00003)
                // that passes float checks but rounds to "0.0000" via %.4f. Exchanges
                // reject zero-amount transfers (bitget code=40020).
                parsedMove, _ := strconv.ParseFloat(moveStr, 64)
                if parsedMove <= 0 {
                    e.log.Warn("rebalance: %s moveAmt %.6f rounds to zero string, skipping", bestDonor, moveAmt)
                    surplus[bestDonor] = 0
                    continue
                }

                // Capture pre-transfer spot balance BEFORE TransferToSpot.
                // MUST happen before the transfer; reading after would double-
                // count any portion of the transfer that settled between
                // TransferToSpot's return and this read, causing an inflated
                // target and false timeout. Fallback to donorBal.spot on
                // read error (never fails otherwise; unknown-donor skipped
                // earlier by surrounding logic).
                preTransferSpot, err := e.captureSpotBalanceForTransfer(bestDonor, donorBal.spot)
                if err != nil {
                    e.log.Warn("rebalance: %s %v — skipping donor", bestDonor, err)
                    surplus[bestDonor] = 0
                    continue
                }

                e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
                if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
                    e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
                    surplus[bestDonor] = 0
                    continue
                }
                e.recordTransfer(bestDonor, bestDonor+" spot", "USDT", "internal", moveStr, "0", "", "completed", "rebalance-prep")
                // Use parsedMove (the value actually sent in moveStr) so the
                // wait target matches what the exchange received, not the raw
                // moveAmt float (which may differ by up to 0.00005 due to
                // %.4f rounding and could cause false timeout).
                movedToSpot = parsedMove

                // Wait for the internal transfer to settle before the
                // GetSpotBalance call below. Without this wait, the read
                // can see stale balance and cause netAmount = -fee < 0 at
                // the guard below, silently skipping the donor withdraw.
                // Target = preTransferSpot (captured above) + movedToSpot
                // so a pre-existing spot balance does not cause a false
                // early success. See 2026-04-23 02:35 binance->bingx
                // incident + codex audits b1f1og3j7, bqvv15ukz, bqtfw2xow,
                // ba6l7l0eg, brshav1ag (review findings).
                if err := e.waitForSpotBalance(bestDonor, preTransferSpot+movedToSpot, 10*time.Second); err != nil {
                    e.log.Warn("rebalance: %s %v — skipping donor", bestDonor, err)
                    surplus[bestDonor] = 0
                    continue
                }
            }

            // Unified accounts: spot balance is 0 (same pool as futures).
            // Use GetFuturesBalance to get the real withdrawable amount.
            var donorSpotBal *exchange.Balance
```

Placement notes:
- Insert INSIDE the `if !skipOuterTransfer && donorBal.spot < requiredSpot` block (the same block that already executed `TransferToSpot`). The outer `}` at the unchanged line is the close of that `if` block; the wait must be before it.
- If `TransferToSpot` was skipped (unified account or spot already sufficient), the pre-transfer capture and wait block are never reached.
- Poll for the expected total spot balance after settlement, not just `movedToSpot`. Use the donor's spot balance immediately before `TransferToSpot` plus `movedToSpot`. This avoids a false early pass when the donor already had enough pre-existing spot to satisfy `movedToSpot` but not enough to satisfy the pending withdrawal. Binance and Bitget `TransferToSpot` implementations submit a single transfer and reject non-success codes (see `pkg/exchange/binance/adapter.go:838`, `pkg/exchange/bitget/adapter.go:948`), so the full-or-reject assumption holds.
- If the pre-read `GetSpotBalance` call fails, fall back to the snapshot `donorBal.spot` captured earlier in the rebalance cycle. The wait target may be slightly conservative in that case (waits for a value that's already attainable), but this is safer than polling for only `movedToSpot`.

- [ ] **Step 3: Compile check**

`go build ./internal/engine/`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/engine/allocator.go
git commit -m "engine(allocator): wait for spot balance after TransferToSpot before withdraw"
```

---

## Task 3: Regression test

**Files:**
- Create: `internal/engine/wait_spot_balance_test.go`

- [ ] **Step 1: Write failing test (pre-fix state assumption + post-fix expectation)**

Create `internal/engine/wait_spot_balance_test.go`:

```go
package engine

import (
    "errors"
    "strings"
    "sync/atomic"
    "testing"
    "time"

    "arb/pkg/exchange"
    "arb/pkg/utils"
)

// delayedSpotStub is a minimal exchange stub whose GetSpotBalance controls
// the observed spot balance per-call. Only GetSpotBalance is implemented;
// other Exchange methods are NOT overridden and will panic if called —
// waitForSpotBalance MUST NOT exercise them.
type delayedSpotStub struct {
    exchange.Exchange                  // embed so unused methods are "valid" types; any actual call panics
    preSettle         float64          // balance reported before settlement (pre-existing spot)
    target            float64          // balance once settled (preSettle + moved amount in prod)
    delayPolls        int32            // pre-settlement polls before target becomes visible (atomic)
    calls             int32            // atomic
    errOnPoll         int32            // if >0, this many polls return an error first (atomic)
}

func (s *delayedSpotStub) GetSpotBalance() (*exchange.Balance, error) {
    n := atomic.AddInt32(&s.calls, 1)
    if atomic.LoadInt32(&s.errOnPoll) > 0 {
        atomic.AddInt32(&s.errOnPoll, -1)
        return nil, errors.New("transient network error")
    }
    if n <= atomic.LoadInt32(&s.delayPolls) {
        return &exchange.Balance{Available: s.preSettle, Total: s.preSettle}, nil
    }
    return &exchange.Balance{Available: s.target, Total: s.target}, nil
}

// newWaitTestEngine returns the minimal *Engine fixture required by
// waitForSpotBalance: just exchanges map + logger. Do NOT use
// buildRankFirstEngine — it is overkill for this unit test and pulls
// miniredis + risk manager wiring that is irrelevant here.
func newWaitTestEngine(exchanges map[string]exchange.Exchange) *Engine {
    return &Engine{
        exchanges: exchanges,
        log:       utils.NewLogger("test-wait-spot"),
    }
}

// TestWaitForSpotBalance_EventualSuccess verifies the poll loop succeeds
// when the target balance appears after a few zero-reads.
func TestWaitForSpotBalance_EventualSuccess(t *testing.T) {
    stub := &delayedSpotStub{target: 120.75, delayPolls: 2}
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    start := time.Now()
    err := eng.waitForSpotBalance("binance", 120.75, 10*time.Second)
    elapsed := time.Since(start)

    if err != nil {
        t.Fatalf("expected nil err after delayed balance appears, got: %v", err)
    }
    if atomic.LoadInt32(&stub.calls) < 3 {
        t.Errorf("expected at least 3 GetSpotBalance calls (2 zero + 1 success), got %d", stub.calls)
    }
    if elapsed < 2*time.Second {
        t.Errorf("expected at least 2s elapsed (2 polls), got %s", elapsed)
    }
    if elapsed > 5*time.Second {
        t.Errorf("expected less than 5s elapsed, got %s (should succeed quickly)", elapsed)
    }
}

// TestWaitForSpotBalance_TransientError verifies the poll loop recovers
// after a transient GetSpotBalance error and still succeeds within timeout.
func TestWaitForSpotBalance_TransientError(t *testing.T) {
    stub := &delayedSpotStub{target: 50.0, delayPolls: 0, errOnPoll: 2}
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    start := time.Now()
    err := eng.waitForSpotBalance("binance", 50.0, 10*time.Second)
    elapsed := time.Since(start)

    if err != nil {
        t.Fatalf("expected nil err after transient error recovers, got: %v", err)
    }
    if atomic.LoadInt32(&stub.calls) < 3 {
        t.Errorf("expected at least 3 GetSpotBalance calls (2 err + 1 success), got %d", stub.calls)
    }
    if elapsed > 5*time.Second {
        t.Errorf("expected success within 5s, got %s", elapsed)
    }
}

// TestWaitForSpotBalance_PreExistingSpotBelowTarget verifies the caller's
// expected usage: they must pass target = preTransferSpot + movedToSpot,
// not movedToSpot alone. Models pre-existing spot=100 (below target=200)
// for first 3 polls, then settles to 200. If waitForSpotBalance
// incorrectly returned on the first poll (just checking >0), it would
// wrongly report success at balance=100 < target=200.
func TestWaitForSpotBalance_PreExistingSpotBelowTarget(t *testing.T) {
    stub := &delayedSpotStub{preSettle: 100.0, target: 200.0, delayPolls: 3}
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    err := eng.waitForSpotBalance("binance", 200.0, 10*time.Second)
    if err != nil {
        t.Fatalf("expected eventual success, got err: %v", err)
    }
    if atomic.LoadInt32(&stub.calls) < 4 {
        t.Errorf("expected at least 4 polls (3 pre-settle + 1 settled), got %d", stub.calls)
    }
}

// TestWaitForSpotBalance_Timeout verifies the poll loop returns an error
// after the timeout when the target never appears.
func TestWaitForSpotBalance_Timeout(t *testing.T) {
    stub := &delayedSpotStub{target: 120.75, delayPolls: 9999} // never satisfies
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    start := time.Now()
    err := eng.waitForSpotBalance("binance", 120.75, 3*time.Second) // short timeout for test
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("expected timeout error, got nil")
    }
    if !strings.Contains(err.Error(), "timeout") {
        t.Errorf("expected error message to mention timeout, got: %v", err)
    }
    if elapsed < 3*time.Second {
        t.Errorf("expected at least 3s elapsed (timeout), got %s", elapsed)
    }
    if elapsed > 5*time.Second {
        t.Errorf("expected less than 5s elapsed (timeout should fire promptly), got %s", elapsed)
    }
}

// TestWaitForSpotBalance_UnknownDonor verifies the helper fails fast when
// the donor key is not in e.exchanges (bug guard).
func TestWaitForSpotBalance_UnknownDonor(t *testing.T) {
    eng := newWaitTestEngine(map[string]exchange.Exchange{})
    err := eng.waitForSpotBalance("nonexistent", 100, 1*time.Second)
    if err == nil {
        t.Fatal("expected error for unknown donor, got nil")
    }
    if !strings.Contains(err.Error(), "nonexistent") {
        t.Errorf("expected error to contain donor name, got: %v", err)
    }
}

// singleReadStub returns an error on the first GetSpotBalance call,
// and a fixed value thereafter. Used to exercise the snapshotSpot
// fallback in captureSpotBalanceForTransfer.
type singleReadStub struct {
    exchange.Exchange
    value float64
    calls int32 // atomic
}

func (s *singleReadStub) GetSpotBalance() (*exchange.Balance, error) {
    n := atomic.AddInt32(&s.calls, 1)
    if n == 1 {
        return nil, errors.New("transient pre-read error")
    }
    return &exchange.Balance{Available: s.value, Total: s.value}, nil
}

// fixedReadStub returns a fixed spot balance on every call. Used to
// exercise the happy path of captureSpotBalanceForTransfer.
type fixedReadStub struct {
    exchange.Exchange
    value float64
    calls int32 // atomic
}

func (s *fixedReadStub) GetSpotBalance() (*exchange.Balance, error) {
    atomic.AddInt32(&s.calls, 1)
    return &exchange.Balance{Available: s.value, Total: s.value}, nil
}

// TestCaptureSpotBalanceForTransfer_Success verifies that when the
// GetSpotBalance read succeeds, the helper returns the live Available
// value (not the snapshot).
func TestCaptureSpotBalanceForTransfer_Success(t *testing.T) {
    stub := &fixedReadStub{value: 150.0}
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    pre, err := eng.captureSpotBalanceForTransfer("binance", 100.0) // snapshot=100, live=150
    if err != nil {
        t.Fatalf("expected nil err, got: %v", err)
    }
    if pre != 150.0 {
        t.Errorf("expected pre=150 (live read preferred over snapshot), got %f", pre)
    }
}

// TestCaptureSpotBalanceForTransfer_ReadFallback verifies that when
// GetSpotBalance returns an error, the helper falls back to the
// passed-in snapshotSpot value without returning an error.
func TestCaptureSpotBalanceForTransfer_ReadFallback(t *testing.T) {
    stub := &singleReadStub{value: 999.0} // would return 999 if ever called twice, but test only calls once
    eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

    pre, err := eng.captureSpotBalanceForTransfer("binance", 100.0)
    if err != nil {
        t.Fatalf("expected nil err on fallback, got: %v", err)
    }
    if pre != 100.0 {
        t.Errorf("expected pre=100 (snapshot fallback), got %f", pre)
    }
    if atomic.LoadInt32(&stub.calls) != 1 {
        t.Errorf("expected exactly 1 GetSpotBalance call, got %d", stub.calls)
    }
}

// TestCaptureSpotBalanceForTransfer_UnknownDonor verifies the helper
// fails fast when donor key is not registered.
func TestCaptureSpotBalanceForTransfer_UnknownDonor(t *testing.T) {
    eng := newWaitTestEngine(map[string]exchange.Exchange{})
    _, err := eng.captureSpotBalanceForTransfer("nonexistent", 100.0)
    if err == nil {
        t.Fatal("expected error for unknown donor, got nil")
    }
    if !strings.Contains(err.Error(), "nonexistent") {
        t.Errorf("expected error to contain donor name, got: %v", err)
    }
}
```

Harness notes:
- Do NOT reuse `buildRankFirstEngine` — it pulls miniredis + risk manager wiring irrelevant to this unit. A minimal `&Engine{exchanges, log}` fixture (via `newWaitTestEngine`) is sufficient.
- `delayedSpotStub` embeds `exchange.Exchange` so unimplemented methods have valid types; any accidental call panics with nil interface dispatch — acceptable because `waitForSpotBalance` only calls `GetSpotBalance()`.
- All assertions use `strings.Contains` for readability.

- [ ] **Step 2: Run tests**

`go test ./internal/engine/ -run 'TestWaitForSpotBalance|TestCaptureSpotBalanceForTransfer' -v -count=1`
Expected: 8/8 PASS. Timings:
- `TestWaitForSpotBalance_EventualSuccess` finishes in 2-3 seconds
- `TestWaitForSpotBalance_TransientError` finishes in 2-3 seconds
- `TestWaitForSpotBalance_PreExistingSpotBelowTarget` finishes in 3-4 seconds
- `TestWaitForSpotBalance_Timeout` finishes in 3-4 seconds
- `TestWaitForSpotBalance_UnknownDonor` finishes in <100ms
- `TestCaptureSpotBalanceForTransfer_Success` finishes in <100ms
- `TestCaptureSpotBalanceForTransfer_ReadFallback` finishes in <100ms
- `TestCaptureSpotBalanceForTransfer_UnknownDonor` finishes in <100ms

- [ ] **Step 3: Full package test**

`go test ./internal/engine/ -count=1`
Expected: all pre-existing tests still PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/engine/wait_spot_balance_test.go
git commit -m "test(engine): regression tests for waitForSpotBalance helper"
```

---

## Task 4: Integration verification

**Files:** None modified.

- [ ] **Step 1: Frontend build** (repo convention — go:embed needs `web/dist`)

`cd web && npm run build && cd ..`

- [ ] **Step 2: Full repo build**

`GOOS=linux GOARCH=amd64 go build -o arb ./cmd/main.go`
Expected: success, ELF Linux amd64.

- [ ] **Step 3: Full repo test**

`go test ./...`
Expected: all tests PASS (pre-existing `cmd/gatecheck` redundant-newline warning is baseline noise, not introduced by this change).

- [ ] **Step 4: Go vet**

`go vet ./...`
Expected: no NEW warnings.

- [ ] **Step 5: Commit verification marker**

`git commit --allow-empty -m "chore: verify full-repo build + test after wait-spot-balance fix"`

---

## Task 5: Version bump + CHANGELOG

**Files:**
- Modify: `VERSION` (`0.33.2` → `0.33.3`)
- Modify: `CHANGELOG.md` (prepend)

- [ ] **Step 1: Bump VERSION**

Change `VERSION` from `0.33.2` to `0.33.3`.

- [ ] **Step 2: Prepend CHANGELOG entry**

Insert above the current `## [0.33.2]` block:

```markdown
## [0.33.3] - 2026-04-23

### Fixed
- **Rebalance transfer timing for split-account donors** — `executeRebalanceFundingPlan` called `TransferToSpot` as fire-and-forget and then immediately read `GetSpotBalance`, which for binance/bitget-class split-account donors returned stale balance before the internal futures→spot settled. The subsequent `netAmount = spot - fee < 0` guard silently skipped the outbound withdraw, leaving recipients (e.g. bingx) unfunded. Observed 2026-04-23 02:35 UTC binance→bingx for SIGNUSDT. Fix:
  - Added `*Engine.waitForSpotBalance(donor, required, timeout)` helper that polls `GetSpotBalance` every 1s until `Available >= required` or the 10s timeout elapses (`internal/engine/allocator.go`).
  - Captured pre-transfer spot balance (`preTransferSpot`) before `TransferToSpot` and wait for `preTransferSpot + movedToSpot` using the *parsed* submitted transfer amount (`parsedMove` from `moveStr`), so neither pre-existing spot nor %.4f float rounding can cause a false success/timeout.
  - Inserted the wait between `TransferToSpot` and the subsequent spot-balance read. On timeout the donor is dropped with the same skip semantics as the existing `netAmount <= 0` guard.
  - Regression tests covering eventual success, transient `GetSpotBalance` error, pre-existing spot below target, timeout, and unknown donor cases (`internal/engine/wait_spot_balance_test.go`).
  - Codex independent audits (local companion tasks `b1f1og3j7`, `bqvv15ukz`, `bqtfw2xow`, gpt-5.4 xhigh) confirmed root cause and iterated the plan.
```

- [ ] **Step 3: Commit**

```bash
git add VERSION CHANGELOG.md
git commit -m "chore: bump v0.33.3 for wait-spot-balance fix"
```

---

## Risk / Side Effects

| Risk | Mitigation |
|------|-----------|
| Poll loop blocks main rebalance goroutine up to 10s per split-donor per recipient | Pass-1 full window is ~60s; at most 2-3 recipients × 10s = 30s worst case; still within budget. Acceptable per per-donor log. |
| `GetSpotBalance` rate limit on rapid 1s polls | 10 polls per 10s × few donors per cycle is well within exchange rate limits (typically 10-20 req/s spot-account allowance on binance/bitget). |
| False positive: spot balance stays stale due to exchange-side indexer lag | 10s covers the p99 of internal transfer reflection; if still zero after 10s the transfer is likely stuck or delayed beyond recovery — donor skip is the correct safe behavior. |
| Unified-account donors should NOT trigger wait | Fix is inside `!skipOuterTransfer` branch, which is false for unified (see line 1857-1860). No change for unified path. |
| `movedToSpot` may be less than what actually landed if `TransferToSpot` was partially honored | We poll for `>= preTransferSpot + movedToSpot`, where `movedToSpot` is parsed from the submitted `moveStr`. Exchanges either honor the request in full or reject; partial fills are not a documented behavior for asset transfers. Even if partial, the later `netAmount <= 0` guard still catches insufficient withdrawable balance and skips gracefully. |
| Clock / sleep interaction with existing unit tests | Tests mock the exchange, not `time.Sleep`; `time.Now()` timing is real but bounded by the test's 10s timeout — no flakiness expected on CI runners with normal scheduling. |

## Out of Scope

- Normalizing `TransferToSpot` across adapters to return transfer-id/status (codex-labelled medium follow-up) — deferred. The wait helper is sufficient for the current bug class.
- Retry on timeout with a different donor — current behavior already falls through to the next donor via the existing loop structure (`continue` on skip).
- gateio APT chain `withdraw_fix_on_chains` lookup issue — separate concern; diagnostic 2026-04-23 04:46 showed adapter works correctly against live API, root cause unclear (possibly transient). Handled in its own follow-up plan if it recurs.
- Extending `waitForSpotBalance` to post-withdraw arrival polling on the recipient side — the current rebalance explicitly does NOT wait for cross-exchange deposits, and that design remains intentional to keep the engine loop responsive.

## Review History

- v1 2026-04-23: DRAFT — initial plan for codex review
- v2 2026-04-23: codex (local companion `bqvv15ukz`) — NEEDS-REVISION with 4 findings:
  - [HIGH] `movedToSpot` alone is wrong wait target; pre-existing spot balance must be added. Fixed: insert pre-read `GetSpotBalance` before `TransferToSpot`, poll for `preTransferSpot + movedToSpot`.
  - [MEDIUM] `newTestEngineWithPass1Harness` does not exist; use minimal `&Engine{exchanges, log}` fixture via `newWaitTestEngine`. Fixed.
  - [LOW] Hand-rolled `contains/indexOf` replaced with `strings.Contains`. Fixed.
  - [LOW] Missing edge-case tests (transient error, pre-existing spot below target). Fixed: added `TestWaitForSpotBalance_TransientError` and `TestWaitForSpotBalance_PreExistingSpotBelowTarget`.
- v3 2026-04-23: codex (local companion `bqtfw2xow`) — NEEDS-REVISION with 3 findings:
  - [MEDIUM] `movedToSpot = moveAmt` uses raw float but actual send is `moveStr` (%.4f rounded); up to 0.00005 drift could cause false timeout. Fixed: use `parsedMove := strconv.ParseFloat(moveStr)` for `movedToSpot`.
  - [LOW] `PreExistingSpotBelowTarget` test didn't actually model pre-existing non-zero balance. Fixed: added `preSettle` field to stub; test now uses preSettle=100, target=200.
  - [LOW] CHANGELOG wording referenced outdated `movedToSpot` target semantics. Fixed: updated Task 5 entry to mention pre-transfer-spot + parsed-move.
- v4 2026-04-23: codex (local companion `ba6l7l0eg`) — NEEDS-REVISION with 3 wording/coverage findings:
  - [LOW] Risk table still described `movedToSpot`-only target. Fixed: updated row to `preTransferSpot + movedToSpot`.
  - [LOW] CHANGELOG hardcoded pre-patch line numbers that will become stale. Fixed: removed `line 1914/1923` from CHANGELOG.
  - [LOW] Coverage gap for pre-read-fallback path. Fixed: extracted `captureAndWaitForSpotTransfer` helper, added `TestCaptureAndWaitForSpotTransfer_PreReadFallback` + `TestCaptureAndWaitForSpotTransfer_UnknownDonor`, Task 2 call site now uses the helper.
- v5 2026-04-23: codex (local companion `brshav1ag`) — NEEDS-REVISION with 1 HIGH logic bug:
  - [HIGH] `captureAndWaitForSpotTransfer` placed pre-read AFTER `TransferToSpot`, which could double-count fast-settled transfers (preRead sees already-credited balance, then adds sentAmount on top → inflated target → false timeout). Fixed: split back into two helpers — `captureSpotBalanceForTransfer` (pre-read only, called BEFORE TransferToSpot) and existing `waitForSpotBalance` (called AFTER). Call site in Task 2 now interleaves them in correct order. Tests updated to `TestCaptureSpotBalanceForTransfer_Success` / `_ReadFallback` / `_UnknownDonor`.

---

## Self-Review (Writer)

**Spec coverage:** Root cause addressed (Task 2). Regression coverage for success/timeout/unknown-donor (Task 3). Build + test gate (Task 4). Version bump (Task 5). All 5 tasks have concrete BEFORE/AFTER or exact command lines.

**Placeholder scan:** No TBD/TODO. Helper code is complete Go. Test code is complete (with a noted simplification option for `strings.Contains`).

**Type consistency:** `waitForSpotBalance(donor string, required float64, timeout time.Duration)` signature uniform across all mentions. `movedToSpot` is already a `float64` in existing code (line 1920). Test stubs implement `GetSpotBalance() (*exchange.Balance, error)` matching the interface.

**Scope discipline:** Only modifies `internal/engine/allocator.go` + adds one test file + bumps VERSION/CHANGELOG. No adapter changes, no interface changes, no dashboard changes.
