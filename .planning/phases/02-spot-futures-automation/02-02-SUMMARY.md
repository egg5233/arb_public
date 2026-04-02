---
phase: 02-spot-futures-automation
plan: 02
subsystem: spotengine
tags: [exit-guards, min-hold, settlement-window, basis-gate, spread-gate, spot-futures]

# Dependency graph
requires:
  - phase: 02-spot-futures-automation
    plan: 01
    provides: 9 config fields for exit/entry guards (SpotFuturesEnable*, SpotFuturesMin*, SpotFuturesMax*, SpotFuturesExit*)
provides:
  - Min-hold gate blocking yield-based exits for young positions
  - Settlement window guard deferring exits during 8h funding settlement
  - Exit spread gate deferring exits when unwind slippage exceeds threshold
  - Entry basis gate rejecting entries when spread-based basis proxy too wide
  - isInSettlementWindow/isInSettlementWindowAt helpers for settlement detection
  - estimateUnwindSlippage helper for futures orderbook spread estimation
  - calculateEntryBasis helper for entry basis proxy calculation
affects: [02-spot-futures-automation, spot-futures-dashboard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Emergency-first exit trigger ordering: safety triggers before guard gates before yield triggers"
    - "Guard gate pattern: if cfg.Enable* { check(); return if blocked } -- complete no-op when disabled"
    - "Fail-closed error handling on basis gate: reject entry on orderbook errors"
    - "spreadStubExchange/basisStubExchange test patterns for separate bid/ask orderbook stubs"

key-files:
  created:
    - internal/spotengine/exit_guards_test.go
    - internal/spotengine/basis_spread_test.go
  modified:
    - internal/spotengine/exit_manager.go
    - internal/spotengine/risk_gate.go

key-decisions:
  - "Restructured checkExitTriggers priority: Phase 1 (emergency/safety), Phase 2 (guard gates), Phase 3 (yield triggers)"
  - "Used futures orderbook bid-ask spread as conservative basis proxy -- no separate spot price feed needed"
  - "Basis gate is fail-closed: GetOrderbook errors reject entry (safer than allowing potentially bad entry)"
  - "isInSettlementWindow uses hour%8 modular arithmetic for 00:00/08:00/16:00 UTC settlement detection"

patterns-established:
  - "Exit trigger 3-phase priority: emergency first, guards second, yield last"
  - "Guard gate no-op pattern: check Enable* flag first, skip entire gate when false"
  - "spreadStubExchange for orderbook spread tests with separate bid/ask"

requirements-completed: [SF-06, SF-07]

# Metrics
duration: 13min
completed: 2026-04-02
---

# Phase 02 Plan 02: Exit/Entry Safeguards Summary

**Exit guards (min-hold, settlement window, spread gate) and entry basis gating protecting spot-futures automation pipeline, with emergency-first trigger priority ordering**

## Performance

- **Duration:** 13 min
- **Started:** 2026-04-02T08:01:04Z
- **Completed:** 2026-04-02T08:14:04Z
- **Tasks:** 2
- **Files modified:** 4 (exit_manager.go, risk_gate.go, exit_guards_test.go, basis_spread_test.go)

## Accomplishments
- Restructured checkExitTriggers with emergency-first priority: price spike and margin health always checked before any guard gate, ensuring emergencies can never be blocked by min-hold or settlement windows
- Added 3 exit guards (min-hold, settlement window, exit spread gate) that only block yield-based triggers while being complete no-ops when their Enable* toggle is false
- Added entry basis gate as check 5 in checkRiskGate, fail-closed on errors, before dry-run (renumbered to check 6)
- 74 new tests across 2 test files (475+302 lines) covering guard behavior, emergency bypass, settlement window detection, slippage estimation, basis calculation

## Task Commits

Each task was committed atomically:

1. **Task 1: Add exit guards to checkExitTriggers** - `0194d3f` (feat)
2. **Task 2: Add basis gate to checkRiskGate** - `3744fcc` (feat)

## Files Created/Modified
- `internal/spotengine/exit_manager.go` - Restructured checkExitTriggers (emergency > guards > yield), added isInSettlementWindow, estimateUnwindSlippage helpers
- `internal/spotengine/risk_gate.go` - Added basis gate as check 5, calculateEntryBasis helper, renumbered dry-run to check 6
- `internal/spotengine/exit_guards_test.go` - 475 lines: TestMinHoldGate, TestSettlementGuard, TestEmergencyBypass, TestExitSpreadGate, TestEstimateUnwindSlippage, TestIsInSettlementWindow
- `internal/spotengine/basis_spread_test.go` - 302 lines: TestBasisGateEntry, TestBasisGateBeforeDryRun, TestBasisGateDryRunPassthrough, TestCalculateEntryBasis, TestCalculateEntryBasis_ExchangeNotFound

## Decisions Made
- Restructured checkExitTriggers to 3-phase priority (emergency/safety first, guard gates second, yield triggers last) per RESEARCH Pitfall 3 -- ensures emergency exits can never be blocked by guards
- Used futures orderbook bid-ask spread as conservative basis proxy per RESEARCH Open Question 1 -- for USDT pairs, spread is a reasonable proxy and the 0.5% default threshold provides sufficient buffer
- Basis gate is fail-closed on GetOrderbook errors -- rejects entry rather than allowing a potentially bad entry
- Settlement window uses simple hour%8 modular arithmetic for standard 8h settlements (00:00, 08:00, 16:00 UTC)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree missing go.sum and web/dist directory (same as Plan 01). Fixed with `mkdir -p web/dist && touch web/dist/placeholder && go mod tidy`.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all guards are fully wired to config fields and operational.

## Next Phase Readiness
- All exit safeguards and entry basis gating in place -- prerequisite for enabling auto-entry (SF-05 in Plan 03)
- Guard config fields (Enable* toggles) all default to false -- safe to deploy without affecting existing behavior
- Plan 03 (auto-entry enablement) can now safely enable auto-entry knowing exits are properly guarded

## Self-Check: PASSED

All files exist, all commits verified.

---
*Phase: 02-spot-futures-automation*
*Completed: 2026-04-02*
