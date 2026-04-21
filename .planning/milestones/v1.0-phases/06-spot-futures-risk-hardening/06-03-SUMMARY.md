---
phase: 06-spot-futures-risk-hardening
plan: 03
subsystem: risk
tags: [liquidation-distance, health-monitor, maintenance-rate, cross-engine-dispatch, exit-triggers]

# Dependency graph
requires:
  - phase: 06-spot-futures-risk-hardening plan 01
    provides: getMaintenanceRate(), maintenanceRateProvider interface, SpotFuturesEnableMaintenanceGate config
provides:
  - "Trigger 2b: maintenance-rate-aware liquidation distance in checkExitTriggers"
  - "HealthMonitor spot-futures integration: checkAll fetches both perp and spot positions"
  - "HealthAction.SpotPositions field for cross-engine L4/L5 dispatch"
  - "Engine.SetSpotCloseCallback for SpotEngine dispatch"
  - "dispatchSpotHealthAction helper for goroutine-based spot close"
affects: [spotengine, risk, engine, cmd-main]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cross-engine callback dispatch via SetSpotCloseCallback setter"
    - "Fail-open pattern for spot position fetch in health monitor"
    - "Graduated liquidation distance response (10%/20%/50% of entry threshold)"

key-files:
  created: []
  modified:
    - internal/spotengine/exit_manager.go
    - internal/spotengine/exit_manager_test.go
    - internal/risk/health.go
    - internal/risk/health_test.go
    - internal/engine/engine.go
    - cmd/main.go

key-decisions:
  - "Used callback function type (not interface) for cross-engine dispatch -- matches existing setter pattern"
  - "Fail-open on spot position Redis fetch failure in health monitor -- perp-perp monitoring continues"
  - "L4 selects largest spot position by NotionalUSDT; L5 includes ALL spot positions"
  - "dispatchSpotHealthAction spawns goroutines per position to avoid blocking health action consumer"

patterns-established:
  - "Cross-engine dispatch: Engine.spotCloseCallback func field + SetSpotCloseCallback setter + goroutine dispatch"
  - "Graduated liquidation response: entryThreshold = 0.90/leverage, sub-thresholds at fixed fractions"

requirements-completed: [SF-RISK-02, SF-RISK-03]

# Metrics
duration: 15min
completed: 2026-04-05
---

# Phase 06 Plan 03: Runtime Liquidation Monitor + Health Monitor Integration Summary

**Maintenance-rate-aware liquidation distance trigger with graduated emergency/exit/warn response, and health monitor extended to include spot-futures positions in L4/L5 actions with cross-engine dispatch to SpotEngine**

## Performance

- **Duration:** 15 min
- **Started:** 2026-04-05T14:29:34Z
- **Completed:** 2026-04-05T14:44:37Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Liquidation distance exit trigger 2b fires graduated response based on maintenance-rate-aware estimated liquidation price, preventing GUAUSDT-type incidents
- Health monitor evaluates both perp-perp and spot-futures positions per exchange, with L4/L5 actions carrying SpotPositions for cross-engine dispatch
- Engine.consumeHealthActions routes spot-futures health actions to SpotEngine via registered callback, matching ClosePosition(pos, reason, isEmergency) signature
- 11 new test functions (7 liq distance + 4 health monitor) with 100% pass rate across both suites (257 total tests)

## Task Commits

Each task was committed atomically:

1. **Task 1: Liquidation distance exit trigger 2b** - `90b187d` (feat)
   - TDD RED: `e66e7a2` (test: add failing tests)
2. **Task 2: Health monitor spot-futures integration** - `4c1adbf` (feat)
   - TDD RED: `7e41b39` (test: add failing tests)

## Files Created/Modified
- `internal/spotengine/exit_manager.go` - Trigger 2b: maintenance-rate-aware liquidation distance in Phase 1 safety triggers
- `internal/spotengine/exit_manager_test.go` - 7 test functions for liq distance trigger scenarios
- `internal/risk/health.go` - SpotPositions on HealthAction, checkAll fetches spot positions, L4/L5 include spot candidates
- `internal/risk/health_test.go` - 4 test functions with miniredis for spot-futures health integration
- `internal/engine/engine.go` - spotCloseCallback field, SetSpotCloseCallback setter, dispatchSpotHealthAction helper
- `cmd/main.go` - Wire eng.SetSpotCloseCallback(spotEng.ClosePosition) after both engines init

## Decisions Made
- Used callback function type (`func(pos, reason, isEmergency) error`) instead of interface for cross-engine dispatch -- matches existing Engine setter pattern (SetTelegram, SetLossLimiter, etc.)
- Fail-open on GetActiveSpotPositions Redis error in checkAll -- perp-perp health monitoring continues unblocked (per Phase 3 fail-open pattern)
- L4 reduce selects only the largest spot position by NotionalUSDT as candidate (conservative: don't close all spots on L4)
- L5 emergency includes ALL spot positions on the affected exchange (must close everything at critical level)
- dispatchSpotHealthAction launches goroutines per position to avoid blocking the health action consumer goroutine

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed pos.Size -> pos.FuturesSize in trigger 2b**
- **Found during:** Task 1 (GREEN phase compilation)
- **Issue:** Plan referenced `pos.Size` for notional fallback, but `SpotFuturesPosition` uses `FuturesSize`
- **Fix:** Changed to `pos.FuturesSize` for correct field reference
- **Files modified:** internal/spotengine/exit_manager.go
- **Verification:** Build compiles, all tests pass
- **Committed in:** 90b187d (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor field name correction. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 04 (discovery scoring with maintenance rate display) can proceed
- All runtime risk infrastructure is now in place: pre-entry gate (Plan 02), liq distance monitor (Plan 03 Task 1), health monitor integration (Plan 03 Task 2)
- Config gate `SpotFuturesEnableMaintenanceGate` controls all three subsystems (default OFF for safe rollout)

---
*Phase: 06-spot-futures-risk-hardening*
*Completed: 2026-04-05*

## Self-Check: PASSED
- All 6 modified files exist on disk
- Task 1 commit 90b187d found in git log
- Task 2 commit 4c1adbf found in git log
