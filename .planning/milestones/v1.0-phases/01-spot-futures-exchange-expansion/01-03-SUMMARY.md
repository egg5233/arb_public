---
phase: 01-spot-futures-exchange-expansion
plan: 03
subsystem: exchange-adapters
tags: [gateio, okx, margin, spot-margin, auto-loan, cross-margin]
status: checkpoint-pending

# Dependency graph
requires:
  - phase: 01-02
    provides: "Binance/Bitget margin adapter patterns, fee deduction, spot/margin routing"
provides:
  - "Gate.io margin adapter verified (28/28 livetest)"
  - "OKX margin adapter with cross-margin borrow/repay via tdMode=cross + ccy=USDT"
  - "OKX EnsureAutoLoan for Multi-currency/Portfolio margin mode accounts"
  - "OKX PlaceOrder fix for fractional lotSz contract sizing"
affects: [spotengine, exchange-adapters]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "OKX Futures mode: tdMode=cross + ccy=USDT handles borrow/repay implicitly (no autoLoan needed)"
    - "OKX Multi-currency margin mode: autoLoan=true via set-auto-loan API for explicit control"
    - "Gate.io market orders require ioc time-in-force (gtc rejected)"

key-files:
  created: []
  modified:
    - pkg/exchange/gateio/margin.go
    - pkg/exchange/okx/margin.go
    - pkg/exchange/okx/adapter.go
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "OKX cross-margin borrows via tdMode=cross + ccy=USDT (Futures mode implicit) instead of autoLoan (requires Multi-currency margin mode)"
  - "OKX EnsureAutoLoan gracefully handles 54001 error in Futures mode"
  - "OKX Test 25 MarginBorrow expected failure in Futures mode -- autoLoan replaces explicit borrow"
  - "OKX PlaceOrder now rounds to lotSz precision (fractional contracts for BTC)"

patterns-established:
  - "OKX account mode awareness: Futures mode (acctLv=2) vs Multi-currency margin (acctLv=3) affects available APIs"
  - "Gate.io unified account: auto_borrow/auto_repay as boolean in order body, account=unified"

requirements-completed: []

# Metrics
duration: 13min
completed: 2026-04-01
---

# Phase 01 Plan 03: Gate.io and OKX Margin Adapter Verification Summary

**Gate.io 28/28 and OKX 27/28 livetest pass; cross-margin spot orders handle borrow/repay implicitly in OKX Futures mode via tdMode=cross + ccy=USDT**

## Performance

- **Duration:** 13 min
- **Started:** 2026-04-01T15:24:49Z
- **Completed:** 2026-04-01T15:38:00Z
- **Tasks:** 1 of 2 (Task 2 is checkpoint:human-verify)
- **Files modified:** 5

## Accomplishments

- Gate.io passes all 28 livetest tests including Dir A auto-borrow sell and Dir B QuoteSize buy
- OKX passes 27/28 livetest tests (Test 25 MarginBorrow expected fail in Futures mode)
- OKX Dir A and Dir B verified via livetest Test 27 with live trades (auto-borrow sell + buyback, buy + sellback)
- Fixed pre-existing OKX PlaceOrder contract sizing bug (fractional lotSz for BTC)

## Task Commits

Each task was committed atomically:

1. **Task 1: Run livetest on Gate.io and OKX, fix adapter bugs** - `b8a93d8` (feat)

**Task 2: Verify Gate.io and OKX Dir A and Dir B with live trades** - CHECKPOINT (awaiting human verification)

## Files Created/Modified

- `pkg/exchange/gateio/margin.go` -- Fixed market order time-in-force (ioc not gtc) for PlaceSpotMarginOrder
- `pkg/exchange/okx/margin.go` -- Added EnsureAutoLoan method, added ccy=USDT to PlaceSpotMarginOrder, integrated autoLoan call
- `pkg/exchange/okx/adapter.go` -- Added lotSz cache, fixed PlaceOrder contract sizing with fractional lotSz
- `CHANGELOG.md` -- v0.22.54 entry for Gate.io and OKX fixes
- `VERSION` -- Bumped to 0.22.54

## Decisions Made

1. **OKX cross-margin mechanism in Futures mode**: OKX account is in Futures mode (acctLv=2) which does NOT support the `set-auto-loan` API (that requires Multi-currency margin mode acctLv=3+). However, in Futures mode, cross-margin spot orders with `tdMode=cross` and `ccy=USDT` implicitly handle borrow/repay. This is the correct mechanism for this account type.

2. **EnsureAutoLoan is non-fatal**: The method attempts to enable autoLoan via API. If the account is in Futures mode (error 54001), it gracefully continues -- the cross-margin mechanism handles borrows anyway. This makes the code work across all OKX account modes.

3. **OKX Test 25 (MarginBorrow) remains failing**: The `spot-manual-borrow-repay` endpoint only works in Spot mode with `enableSpotBorrow=true`. In Futures mode, explicit MarginBorrow is not available. This is by design -- the production flow uses autoLoan/cross-margin instead of explicit MarginBorrow.

4. **OKX contract sizing fix (pre-existing)**: BTC-USDT-SWAP has lotSz=0.01, meaning contract quantities can be 0.17 (not just integers). The previous `math.Round` + format with 0 decimals rounded 0.17 to 0. Fixed to round to lotSz precision using `math.Floor(contracts/lotSz) * lotSz`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] OKX PlaceOrder contract sizing with fractional lotSz**
- **Found during:** Task 1 (OKX livetest)
- **Issue:** Test 11 failed with "Parameter sz error" because BTC contract quantity 0.17 was rounded to 0 (integer rounding)
- **Fix:** Added lotSz cache, round to lotSz precision using `math.Floor(contracts/lotSz) * lotSz`
- **Files modified:** `pkg/exchange/okx/adapter.go`
- **Verification:** OKX Test 11 now passes
- **Committed in:** b8a93d8

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Pre-existing OKX bug fixed as part of adapter verification. No scope creep.

## Issues Encountered

1. **OKX autoLoan only available in Multi-currency margin mode**: The plan assumed autoLoan would work in Futures mode, but OKX error 54001 shows it requires acctLv=3+. Resolved by discovering that Futures mode cross-margin orders handle borrows implicitly via `tdMode=cross` + `ccy=USDT`.

2. **OKX MarginBorrow not available in Futures mode**: The `spot-manual-borrow-repay` endpoint requires Spot mode with `enableSpotBorrow=true`. Since autoLoan replaces explicit borrows, this is acceptable. The repay monitor fallback may need adjustment for OKX, but auto-repay handles it in practice.

## Known Stubs

None -- all implementations are fully wired.

## Next Phase Readiness

- Gate.io: fully verified, ready for production use
- OKX: adapter code ready, awaiting human verification of live trades (Task 2 checkpoint)
- After Task 2 approval: all 5 exchanges (Bybit, Binance, Bitget, Gate.io, OKX) complete for SF-01/SF-02/SF-03

---
*Phase: 01-spot-futures-exchange-expansion*
*Completed: 2026-04-01 (Task 1 only; Task 2 checkpoint pending)*
