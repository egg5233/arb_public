# Wait-For-Spot-Balance Implementation Plan

**Goal:** Add a 10-second poll after `TransferToSpot` in the rebalance executor so split-account donor withdrawals (binance/bitget etc.) no longer fail silently when the internal futures→spot transfer has not yet reflected in spot balance. Fixes the observed 02:35 2026-04-23 production incident where binance→bingx withdraw was skipped with `netAmount=-0.10`, leaving bingx unfunded and SIGNUSDT entry rejected.

**Architecture:** Insert a single call to a new `waitForSpotBalance(exch, required, timeout)` helper between `TransferToSpot` (allocator.go:1914) and the spot-balance read (allocator.go:1923-1931). The helper polls `GetSpotBalance()` every 1 second until `Available >= required` or the 10-second timeout elapses. On timeout, the donor is dropped (same skip semantics already present at :1967-1970). Unified-account donors skip the entire `TransferToSpot` branch, so the wait is only added inside the split-account `!skipOuterTransfer` block.

**Tech Stack:** Go 1.26, `internal/engine/allocator.go`, `pkg/exchange/exchange.go` (`Exchange` interface already defines `GetSpotBalance`). Go stdlib `time`, `testing`.

---

**Version:** v1
**Date:** 2026-04-23
**Status:** DRAFT

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
// subsequent GetSpotBalance at allocator.go:1923 can read 0 and cause
// netAmount = -fee < 0 at :1967, skipping the downstream withdraw.
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
```

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

Current code at `internal/engine/allocator.go:1913-1923`:

```go
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
                e.log.Info("rebalance: %s futures->spot %s USDT", bestDonor, moveStr)
                if err := e.exchanges[bestDonor].TransferToSpot("USDT", moveStr); err != nil {
                    e.log.Error("rebalance: %s futures->spot failed: %v", bestDonor, err)
                    surplus[bestDonor] = 0
                    continue
                }
                e.recordTransfer(bestDonor, bestDonor+" spot", "USDT", "internal", moveStr, "0", "", "completed", "rebalance-prep")
                movedToSpot = moveAmt

                // Wait for the internal transfer to settle before reading
                // spot balance below. Split-account exchanges return from
                // TransferToSpot before the spot pool reflects the move;
                // without this wait, the GetSpotBalance call below can
                // read 0 and cause netAmount = -fee < 0 at the guard below,
                // silently skipping the donor withdraw. See 2026-04-23
                // 02:35 binance->bingx incident + codex audit b1f1og3j7.
                if err := e.waitForSpotBalance(bestDonor, movedToSpot, 10*time.Second); err != nil {
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
- If `TransferToSpot` was skipped (unified account or spot already sufficient), `movedToSpot` remains `0` and the wait block is never reached.
- `movedToSpot` is the moveAmt we requested, not the actual settled amount; since `TransferToSpot` succeeded per the check above, polling for `>= movedToSpot` is correct: the full amount should eventually appear.

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
    "fmt"
    "sync/atomic"
    "testing"
    "time"

    "arb/pkg/exchange"
)

// delayedSpotStub is a minimal exchange stub whose GetSpotBalance returns
// 0 for the first `delayPolls` calls and then the target amount.
// Only the two methods used by waitForSpotBalance are implemented; other
// interface methods are left unimplemented and will panic if called —
// test must not exercise them.
type delayedSpotStub struct {
    exchange.Exchange
    name       string
    target     float64
    delayPolls int32 // atomic
    calls      int32 // atomic
}

func (s *delayedSpotStub) GetSpotBalance() (*exchange.Balance, error) {
    n := atomic.AddInt32(&s.calls, 1)
    if n <= atomic.LoadInt32(&s.delayPolls) {
        return &exchange.Balance{Available: 0, Total: 0}, nil
    }
    return &exchange.Balance{Available: s.target, Total: s.target}, nil
}

// TestWaitForSpotBalance_EventualSuccess verifies the poll loop succeeds
// when the target balance appears after a few polls.
func TestWaitForSpotBalance_EventualSuccess(t *testing.T) {
    eng := newTestEngineWithPass1Harness(t) // reuse Task 3's harness
    stub := &delayedSpotStub{name: "binance", target: 120.75, delayPolls: 2}
    eng.exchanges["binance"] = stub

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

// TestWaitForSpotBalance_Timeout verifies the poll loop returns an error
// after the timeout when the target never appears.
func TestWaitForSpotBalance_Timeout(t *testing.T) {
    eng := newTestEngineWithPass1Harness(t)
    stub := &delayedSpotStub{name: "binance", target: 120.75, delayPolls: 9999} // never satisfies
    eng.exchanges["binance"] = stub

    start := time.Now()
    err := eng.waitForSpotBalance("binance", 120.75, 3*time.Second) // short timeout for the test
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("expected timeout error, got nil")
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
    eng := newTestEngineWithPass1Harness(t)
    err := eng.waitForSpotBalance("nonexistent", 100, 1*time.Second)
    if err == nil {
        t.Fatal("expected error for unknown donor, got nil")
    }
    if want := "nonexistent"; err != nil && !contains(err.Error(), want) {
        t.Errorf("expected error message to contain %q, got %q", want, err.Error())
    }
}

func contains(s, sub string) bool {
    return len(s) >= len(sub) && (s == sub || len(sub) == 0 || fmt.Sprintf("%v", s) != "" && (len(s) > 0 && indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub {
            return i
        }
    }
    return -1
}
```

Helper discovery note: `newTestEngineWithPass1Harness` exists in `internal/engine/rebalance_topup_test.go` (added in the top-up routing fix). It returns an `*Engine` whose `exchanges` map is mutable — extending it with `delayedSpotStub` is the established pattern. If the harness does not already expose `exchanges` publicly to tests in the same package, no change is needed because test files in the same package can access unexported fields directly.

For `contains` / `indexOf` — if `strings` is already imported in the test file or a nearby existing test helper, prefer `strings.Contains`. The above hand-rolled version is only to avoid a new import if the test file is otherwise import-light; feel free to simplify to `strings.Contains(err.Error(), "nonexistent")`.

- [ ] **Step 2: Run tests**

`go test ./internal/engine/ -run 'TestWaitForSpotBalance' -v -count=1`
Expected: 3/3 PASS. Timings:
- `EventualSuccess` finishes in 2-3 seconds
- `Timeout` finishes in 3-4 seconds
- `UnknownDonor` finishes in <100ms

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
- **Rebalance transfer timing for split-account donors** — `executeRebalanceFundingPlan` called `TransferToSpot` as fire-and-forget and then immediately read `GetSpotBalance`, which for binance/bitget-class split-account donors returned `0` before the internal futures→spot settled. The subsequent `netAmount = spot - fee < 0` guard silently skipped the outbound withdraw, leaving recipients (e.g. bingx) unfunded. Observed 2026-04-23 02:35 UTC binance→bingx for SIGNUSDT. Fix:
  - Added `*Engine.waitForSpotBalance(donor, required, timeout)` helper that polls `GetSpotBalance` every 1s until `Available >= required` or the 10s timeout elapses (`internal/engine/allocator.go`).
  - Inserted `waitForSpotBalance(bestDonor, movedToSpot, 10*time.Second)` call between `TransferToSpot` (line 1914) and the spot-balance read (line 1923). On timeout the donor is dropped with the same skip semantics as the existing `netAmount <= 0` guard.
  - Regression tests covering eventual success, timeout, and unknown-donor cases (`internal/engine/wait_spot_balance_test.go`).
  - Codex independent audit (dispatch task `b1f1og3j7`, gpt-5.4 xhigh) confirmed the root cause before fix.
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
| `movedToSpot` may be less than what actually landed if `TransferToSpot` was partially honored | We poll for `>= movedToSpot`, which is the amount we requested. Exchanges either honor the request in full or reject; partial fills are not a documented behavior for asset transfers. Even if partial, the next guard at line 1967 (`netAmount <= 0`) still catches it and skips gracefully. |
| Clock / sleep interaction with existing unit tests | Tests mock the exchange, not `time.Sleep`; `time.Now()` timing is real but bounded by the test's 10s timeout — no flakiness expected on CI runners with normal scheduling. |

## Out of Scope

- Normalizing `TransferToSpot` across adapters to return transfer-id/status (codex-labelled medium follow-up) — deferred. The wait helper is sufficient for the current bug class.
- Retry on timeout with a different donor — current behavior already falls through to the next donor via the existing loop structure (`continue` on skip).
- gateio APT chain `withdraw_fix_on_chains` lookup issue — separate concern; diagnostic 2026-04-23 04:46 showed adapter works correctly against live API, root cause unclear (possibly transient). Handled in its own follow-up plan if it recurs.
- Extending `waitForSpotBalance` to post-withdraw arrival polling on the recipient side — the current rebalance explicitly does NOT wait for cross-exchange deposits, and that design remains intentional to keep the engine loop responsive.

## Review History

- v1 2026-04-23: DRAFT — initial plan for codex review

---

## Self-Review (Writer)

**Spec coverage:** Root cause addressed (Task 2). Regression coverage for success/timeout/unknown-donor (Task 3). Build + test gate (Task 4). Version bump (Task 5). All 5 tasks have concrete BEFORE/AFTER or exact command lines.

**Placeholder scan:** No TBD/TODO. Helper code is complete Go. Test code is complete (with a noted simplification option for `strings.Contains`).

**Type consistency:** `waitForSpotBalance(donor string, required float64, timeout time.Duration)` signature uniform across all mentions. `movedToSpot` is already a `float64` in existing code (line 1920). Test stubs implement `GetSpotBalance() (*exchange.Balance, error)` matching the interface.

**Scope discipline:** Only modifies `internal/engine/allocator.go` + adds one test file + bumps VERSION/CHANGELOG. No adapter changes, no interface changes, no dashboard changes.
