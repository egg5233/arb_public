---
phase: 06-spot-futures-risk-hardening
plan: 02
subsystem: spotengine, api
tags: [maintenance-rate, risk-gate, pre-entry, liquidation, leverage]

# Dependency graph
requires:
  - phase: 06-01
    provides: "getMaintenanceRate() cache+fallback, Config fields for maintenance gate"
provides:
  - "Check 6 (maintenance rate) in checkRiskGate() rejecting overleveraged auto-entries"
  - "ManualOpen maintenance rate warning (log + API response)"
  - "MaintenanceWarning() method on SpotEngine for API handler"
  - "SetSpotMaintenanceWarning() setter on Server for callback wiring"
affects: [06-03, 06-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Survivable drop formula: 1/leverage - maintenance_rate"
    - "Leverage-scaled threshold: 90%/leverage"
    - "Manual bypass with warning log + API response field"

key-files:
  created: []
  modified:
    - internal/spotengine/risk_gate.go
    - internal/spotengine/risk_gate_test.go
    - internal/spotengine/execution.go
    - internal/api/server.go
    - internal/api/spot_handlers.go
    - cmd/main.go

key-decisions:
  - "Used existing isSeparateAccount() helper instead of creating new isUnifiedAccount() -- consistent with codebase"
  - "Maintenance warning in ManualOpen placed before duplicate/capacity checks (1d) since it's non-blocking"
  - "API handler returns maintenance_rate_warning in Data map on success, not in error path"

patterns-established:
  - "Non-blocking risk warnings in manual paths: log + API field, never block"
  - "Check ordering: cheap gates first, maintenance before dry-run"

requirements-completed: [SF-RISK-01]

# Metrics
duration: 10min
completed: 2026-04-05
---

# Phase 06 Plan 02: Pre-Entry Maintenance Rate Gate Summary

**Maintenance rate-aware survivable-drop check as risk gate check 6, preventing GUAUSDT-like overleveraged entries at 3x/30% maintenance (3.3% survivable < 30% threshold)**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-05T14:17:21Z
- **Completed:** 2026-04-05T14:27:24Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 7

## Accomplishments
- Inserted check 6 (maintenance rate gate) in checkRiskGate() between price gap (5) and dry-run (now 7)
- GUAUSDT at 3x leverage with 30% maintenance rate hard-rejected (survivable 3.3% < threshold 30%)
- BTCUSDT at 3x with 0.5% maintenance passes gate (survivable 32.8% >= 30%)
- ManualOpen logs warning but proceeds for manual override (per D-04)
- API handler returns maintenance_rate_warning field in success response
- 6 test functions with 14 test cases covering all leverage/rate combinations

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing maintenance gate tests** - `25efdea` (test)
2. **Task 1 (GREEN): Maintenance gate implementation** - `792edc4` (feat)

_TDD task: RED commit with failing tests, GREEN commit with passing implementation._

## Files Created/Modified
- `internal/spotengine/risk_gate.go` - Check 6 maintenance gate + MaintenanceWarning() method
- `internal/spotengine/risk_gate_test.go` - 6 test functions covering reject/allow/disabled/scaling/default/ordering
- `internal/spotengine/execution.go` - ManualOpen warning log for high-maintenance-rate symbols
- `internal/api/server.go` - spotMaintenanceWarning field + SetSpotMaintenanceWarning setter
- `internal/api/spot_handlers.go` - maintenance_rate_warning in manual open success response
- `cmd/main.go` - Wire SetSpotMaintenanceWarning callback

## Decisions Made
- Used existing `isSeparateAccount()` helper (Binance/Bitget=separate, others=unified) instead of creating new `isUnifiedAccount()` -- follows existing codebase pattern
- Placed maintenance warning in ManualOpen at position 1d (before duplicate/capacity checks) since it's non-blocking and purely advisory
- API returns warning as `maintenance_rate_warning` field in success Data map, not as error -- manual opens succeed regardless

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate return statement**
- **Found during:** Task 1 GREEN phase
- **Issue:** My edit accidentally created two consecutive `return RiskGateResult{Allowed: true}` at end of checkRiskGate()
- **Fix:** Removed the duplicate line
- **Files modified:** internal/spotengine/risk_gate.go
- **Verification:** go vet passes, tests pass
- **Committed in:** 792edc4 (part of GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Trivial self-inflicted edit artifact, no scope change.

## Issues Encountered
None - plan executed as specified.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Check 6 (maintenance rate gate) is ready for runtime liquidation monitoring (Plan 03)
- MaintenanceWarning() reusable for dashboard display (Plan 04)
- All existing spotengine tests (180) continue to pass

---
*Phase: 06-spot-futures-risk-hardening*
*Completed: 2026-04-05*
