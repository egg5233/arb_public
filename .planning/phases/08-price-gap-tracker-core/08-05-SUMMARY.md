---
phase: 08-price-gap-tracker-core
plan: 05
subsystem: pricegaptrader
tags: [pricegap, strategy4, execution, ioc, circuit-breaker, unwind-to-match, pg-02, d-09, d-10, d-15, d-21]

requires:
  - Tracker skeleton + DelistChecker + PriceGapStore DI (08-03)
  - DetectionResult with SpreadBps/MidLong/MidShort/StalenessSec (08-03)
  - Typed ErrPriceGap* sentinels incl. ErrPriceGapCircuitBreaker (08-04)
  - PriceGapStore.AcquirePriceGapLock / SavePriceGapPosition (08-01, 08-02)
provides:
  - (t *Tracker).openPair — concurrent IOC entry + unwind-to-match + circuit breaker
  - (t *Tracker).placeLeg / closeLegMarket / sizeDecimals helpers
  - (t *Tracker).bumpFailures / resetFailures / isCircuitOpen breaker API
  - stubExchange scripted-fills path (queueFill, orderIDFill map)
  - fakeStore mutex-emulating lock + saved-position recorder
affects: [08-06, 08-07]

tech-stack:
  added: []
  patterns:
    - "Concurrent IOC placement via 2 goroutines + buffered channels (D-09 simultaneous entry)"
    - "Unwind-to-match via MARKET (not IOC) ReduceOnly orders (D-10 §Pitfall 4)"
    - "Consecutive-failure circuit breaker with mutex-guarded counter (D-10)"
    - "Per-symbol Redis lock via AcquirePriceGapLock: \"<symbol>:<long>:<short>\" (D-02)"
    - "Best-effort fill-price stamping from det.MidLong/MidShort because adapter GetOrderFilledQty returns only qty (realized slip deferred to Plan 06 per D-21)"

key-files:
  created:
    - internal/pricegaptrader/execution.go
    - internal/pricegaptrader/execution_test.go
  modified:
    - internal/pricegaptrader/fakes_test.go
    - internal/pricegaptrader/risk_gate_test.go
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "Fill price stamped from det.MidLong/det.MidShort (mid at decision) because pkg/exchange.GetOrderFilledQty returns only (qty, error) — no vwap. Realized slippage is computed at exit in Plan 06 from actual close PnL vs modeled. Documented inline in execution.go."
  - "Unwind cleanup uses MARKET (not IOC) per §Pitfall 4 — unwind must complete to restore delta-neutrality; IOC could leave residual skew."
  - "Circuit breaker bumps on BOTH legs independently — a single openPair invocation with both legs failing adds 2 to the counter, so 3 consecutive double-fail rounds (6 bumps) trip the breaker before round 3 completes."
  - "Lock acquisition runs BEFORE circuit-breaker check would matter for the second tick — forceLockBusy test verifies no orders placed when lock busy (second tick collapses early)."
  - "fakeStore.AcquirePriceGapLock rewritten from (_, false, nil) placeholder into a real per-resource mutex simulator — required to unblock openPair in tests without breaking the 22 pre-existing tests."
  - "Zero orphan-on-failure: both-legs-fail path explicitly short-circuits before unwind to avoid spurious ReduceOnly calls against nonexistent positions."

patterns-established:
  - "PlaceOrderParams has no dedicated vwap-returning method — any fill-price consumer should plan for mid-based estimation + later realized-from-PnL reconciliation"
  - "Test-level lock contention via a single boolean on the store fake (forceLockBusy) rather than full goroutine coordination"

requirements-completed: [PG-02]

duration: 12min
completed: 2026-04-21
---

# Phase 08 Plan 05: Simultaneous IOC entry + unwind-to-match + circuit breaker Summary

**Wave-5 live-trading-critical path: concurrent IOC market orders on both legs, D-10 unwind-to-match partial-fill reconciliation via MARKET (not IOC), zero-fill-on-one-leg → immediate ReduceOnly close of the other, 5-consecutive-failure circuit breaker, per-symbol Redis lock, and D-15 position-ID format. 8 tests cover every failure mode the threat model enumerates.**

## Performance

- **Duration:** ~12 min
- **Tasks:** 3
- **Files created:** 2
- **Files modified:** 4 (+ VERSION + CHANGELOG)

## Accomplishments

- `openPair(cand, sizeBase, det) (*PriceGapPosition, error)` — the sole live-trading entry seam for Strategy 4. Runs under per-symbol Redis lock; places simultaneous IOC legs in two goroutines; handles every partial-fill topology deterministically.
- Unwind-to-match semantics wired per D-10: the over-filled leg is trimmed with a `ReduceOnly=true` MARKET order so delta-neutrality is restored even when IOC would have left dust.
- Zero-fill-on-one-leg triggers immediate MARKET close of the filled leg (ReduceOnly). No orphaned directional exposure.
- Circuit breaker: `t.failures >= 5` → `t.paused=true`, breaker short-circuits subsequent `openPair` calls with `ErrPriceGapCircuitBreaker` before the Redis lock is taken or any order placed. Only manual reset (Phase 9 scope).
- 8 unit tests covering happy path, partial-fill unwind, zero-fill defensive close, both-legs-fail no-cleanup, circuit-breaker trip + persistence, lock contention, failure-counter reset on success, and D-15 position-ID regex.
- `fakeStore.AcquirePriceGapLock` upgraded from a placeholder stub into a real per-resource mutex simulator — enables Plan 05 tests without regressing the 22 earlier Plan 01/03/04 tests.

## Task Commits

1. **Task 1: openPair + placeLeg + closeLegMarket + circuit-breaker API** — `13dc401` (feat)
2. **Task 2: stubExchange scripted fills (queueFill + orderID → fillScript map)** — `a1baa11` (test)
3. **Task 3: 8 execution tests + fakeStore lock/save upgrades** — `b67b3b3` (test)

_Plan metadata commit (SUMMARY + state + roadmap) follows this summary._

## Files Created/Modified

- `internal/pricegaptrader/execution.go` (created, 240 lines) — openPair, placeLeg, closeLegMarket, bumpFailures/resetFailures/isCircuitOpen, sizeDecimals, roundStep
- `internal/pricegaptrader/execution_test.go` (created, 249 lines) — 8 TestExecution_ cases
- `internal/pricegaptrader/fakes_test.go` (modified) — fillScript struct, queueFill method, orderIDFill map routing in PlaceOrder + GetOrderFilledQty, placedOrders accessor
- `internal/pricegaptrader/risk_gate_test.go` (modified) — fakeStore enhanced with saved[] recorder, heldLocks map mutex simulator, forceLockBusy flag
- `CHANGELOG.md` / `VERSION` → 0.32.40

## Decisions Made

- **Fill-price source = mid at decision.** Adapter's `GetOrderFilledQty(orderID, symbol) (float64, error)` returns only qty — no vwap. `openPair` therefore stamps `LongFillPrice=det.MidLong`, `ShortFillPrice=det.MidShort`. Realized slippage is computed at exit in Plan 06 from actual close PnL vs the modeled entry spread. This is DOCUMENTED inline in execution.go to prevent future readers from mistaking it for a bug.
- **Unwind uses MARKET, not IOC.** Unwind orders carry `ReduceOnly=true` and `Force=""` (plain market). Rationale: §Pitfall 4 — IOC on unwind can leave dust because partial-fill on the unwind itself is possible; MARKET forces completion. The entry legs carry `Force="ioc"` because over-fill would compound the asymmetry problem.
- **fakeStore lock rewrite was required.** The existing `AcquirePriceGapLock` stub returned `("", false, nil)` — i.e. always-busy — which would cause every `openPair` test to exit at the lock gate. Upgraded to a per-resource mutex with a `forceLockBusy` flag for the contention test. This does not regress the 22 prior tests because none of them call `openPair`.
- **Circuit-breaker accounting is per-leg, not per-call.** Each leg's success/failure is independently evaluated — a double-leg-fail `openPair` adds 2 to the counter. This means 3 consecutive double-fail calls = 6 bumps, which is why `TestExecution_CircuitBreakerOpensAfterFiveFailures` asserts trip after the 3rd call. Matches D-10 intent: the counter measures adapter-call failures, not entry attempts.
- **Lock acquired BEFORE leg placement, released via defer.** This guarantees no PlaceOrder is issued without the lock held, and the lock is released even if `openPair` panics. The lock TTL (30s) is a safety net for abnormal termination — normal completion releases immediately.
- **No separate `placedOrders()` name change.** Task 2's spec suggested `GetPlacedOrders()` as the accessor. Chose lowercase `placedOrders()` to match Go's usual lowercase-for-intra-package style; the only callers are tests in the same package.

## Deviations from Plan

**1. [Rule 3 - Blocking Issue] Adjusted to actual adapter signatures**
- **Found during:** Task 1
- **Issue:** Plan 05 spec assumed `GetOrderFilledQty` returns `(qty, vwap, err)` and `LoadAllContracts` returns `map[string]*ContractInfo` (pointer). Actual signatures: `GetOrderFilledQty(orderID, symbol) (float64, error)` and `LoadAllContracts() (map[string]exchange.ContractInfo, error)` (value).
- **Fix:** `placeLeg` now takes `fillPrice` from the caller (mid-at-decision) instead of reading vwap; `sizeDecimals` dereferences value types directly. Behavior intent is preserved: position gets a fill price, contract decimals are respected. Realized slippage still works because Plan 06 computes it from close PnL.
- **Files:** internal/pricegaptrader/execution.go
- **Commit:** 13dc401

**2. [Rule 3 - Blocking Issue] fakeStore.AcquirePriceGapLock upgraded from placeholder**
- **Found during:** Task 3 (first attempt to test openPair)
- **Issue:** Placeholder returned `(_, false, nil)` → every openPair call would exit at lock gate.
- **Fix:** Implemented per-resource mutex with `forceLockBusy` flag for contention testing.
- **Files:** internal/pricegaptrader/risk_gate_test.go
- **Commit:** b67b3b3

No Rule 4 architectural changes. No Rule 1/2 security bugs found outside scope.

## Issues Encountered

- Adapter signature mismatch between plan spec and real code (documented above) — adjusted per Rule 3.
- fakeStore lock placeholder caused initial test failures — fixed in Task 3 commit.

## Verification

- `go build ./...` — succeeds
- `go vet ./internal/pricegaptrader/...` — clean
- `go test ./internal/pricegaptrader -run TestExecution_ -race -count=1` — 8/8 pass
- `go test ./internal/pricegaptrader/... -race -count=1` — 30/30 pass (8 detector + 14 risk gate + 8 execution)
- D-02 boundary — `grep -rE "internal/engine|internal/spotengine" internal/pricegaptrader/*.go` returns only `tracker.go:1` (the literal comment text "MUST NOT import internal/engine/"), no actual imports
- `grep -c 'Force:\s*"ioc"' internal/pricegaptrader/execution.go` — 2 matches (both legs)
- `grep -c "ReduceOnly: true" internal/pricegaptrader/execution.go` — 1 match (closeLegMarket)
- `grep -c "t.failures >= 5" internal/pricegaptrader/execution.go` — 1 match
- `grep -c "SavePriceGapPosition" internal/pricegaptrader/execution.go` — 1 match
- `grep -c "AcquirePriceGapLock" internal/pricegaptrader/execution.go` — 1 match
- `grep -c "pg_%s_%s_%s_%d" internal/pricegaptrader/execution.go` — 1 match
- `grep -c "func TestExecution_" internal/pricegaptrader/execution_test.go` — 8 matches
- CLAUDE.local.md honored — config.json untouched, no npm commands
- VERSION bumped 0.32.39 → 0.32.40, CHANGELOG entry added

## Threat Flags

None. Surface introduced is fully covered by the plan's threat model:

- **T-08-17** (runaway orders) — mitigated: `TestExecution_CircuitBreakerOpensAfterFiveFailures` asserts no orders placed after trip
- **T-08-18** (asymmetric legs) — mitigated: `TestExecution_PartialFill_UnwindMatches` asserts ReduceOnly SELL closes the over-fill to match
- **T-08-19** (duplicate exec across ticks) — mitigated: `TestExecution_LockHeld_SecondCallBlocks` asserts lock-busy exits before any PlaceOrder
- **T-08-20** (zero-fill strand) — mitigated: `TestExecution_ZeroFillOneLeg_MarketClosesOther` asserts ReduceOnly SELL closes the filled leg
- **T-08-21** (no persistence) — mitigated: `TestExecution_HappyPath` asserts `store.saved` is len==1 with Status=open

## Next Plan Readiness

- **Plan 06 (monitor + exit + exec-quality)** has its entry seam: `openPair` returns a fully-persisted `*PriceGapPosition` with Status=open, OpenedAt timestamp, EntrySpreadBps, ModeledSlipBps. Plan 06 spawns the per-position monitor goroutine keyed off `pos.ID` using the `monitors` map already declared in `tracker.go`.
- **Realized slippage** is computed in Plan 06 from the close PnL vs EntrySpreadBps — the mid-based fill-price stamping in this plan is intentional and sufficient for that computation.
- **Plan 07 (wiring)** — no constructor signature changes. `Tracker` surface (exchanges, db, delist, cfg) unchanged since Plan 03.
- **Phase 9 (dashboard)** — can read `*PriceGapPosition.Status` / `NotionalUSDT` via existing `/api/positions` patterns once Plan 06 adds the spot-engine-style read endpoint.

## Self-Check: PASSED

- `internal/pricegaptrader/execution.go` — FOUND
- `internal/pricegaptrader/execution_test.go` — FOUND
- Commit `13dc401` — FOUND on main
- Commit `a1baa11` — FOUND on main
- Commit `b67b3b3` — FOUND on main
- `go build ./...` — PASS
- `go vet ./internal/pricegaptrader/...` — PASS
- `go test ./internal/pricegaptrader -race -count=1` — PASS (30/30)
- D-02 boundary grep — PASS (only literal comment string matches; no imports)

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
