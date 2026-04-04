---
phase: 05-capital-allocation
plan: 02
subsystem: engine
tags: [capital-allocation, engine-integration, risk-manager, api, scan-cycle]

# Dependency graph
requires:
  - phase: 05-capital-allocation
    provides: Plan 01 core infrastructure (profiles, allocator extensions, config fields)
  - phase: 04-performance-analytics
    provides: ComputeStrategySummary for trailing APR data
provides:
  - Both engines use EffectiveCapitalPerLeg from unified pool when enabled
  - Risk manager sizing uses derived capital per leg
  - ComputeEffectiveAllocation called at EntryScan/discoveryLoop time with trailing APR
  - DynamicStrategyPct available for per-cycle shifting (CA-04)
  - GET /api/allocation endpoint returning CapitalSummary
  - Config POST handles profile selection and custom detection
affects: [05-03-dashboard-allocation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "effectiveCapitalPerLeg() method pattern: check allocator first, fall back to cfg.CapitalPerLeg"
    - "updateAllocation() scan-cycle hook: load history, compute APR, set effective allocation"
    - "Profile custom detection: monitor bundled field writes, auto-switch to custom"
    - "SetCapitalAllocator setter pattern on Server (same as SetAnalyticsStore, SetExchangeScorer)"

key-files:
  created: []
  modified:
    - internal/engine/capital.go
    - internal/engine/engine.go
    - internal/risk/manager.go
    - internal/spotengine/engine.go
    - internal/api/handlers.go
    - internal/api/server.go
    - cmd/main.go

key-decisions:
  - "Use GetHistory(200)/GetSpotHistory(200) for trailing APR instead of nonexistent GetClosedPositions"
  - "Minimum 3 trades per strategy threshold before performance-weighted tilt applies"
  - "spotHasOpps defaults to cfg.SpotFuturesEnabled (conservative heuristic)"
  - "Server gets allocator via setter (SetCapitalAllocator) not constructor, matching existing pattern"
  - "Allocation fields persisted to Redis HASH alongside other config sections"

patterns-established:
  - "effectiveCapitalPerLeg() wrapper: allocator-first with cfg fallback, used by Engine, Manager, SpotEngine"
  - "updateAllocation() scan-cycle integration: called before entry decisions in both engines"
  - "Profile custom detection in handlePostConfig: monitor MaxPositions, Leverage, MaxCostRatio, MinNetYieldAPR, strategy pcts, SizeMultiplier"

requirements-completed: [CA-01, CA-02, CA-03, CA-04]

# Metrics
duration: 13min
completed: 2026-04-04
---

# Phase 5 Plan 02: Engine Integration Summary

**Unified capital allocation wired into perp-perp engine, spot-futures engine, risk manager sizing, scan-cycle APR computation, and REST API with profile handling**

## Performance

- **Duration:** 13 min
- **Started:** 2026-04-04T03:09:02Z
- **Completed:** 2026-04-04T03:22:36Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Both engines and risk manager use effectiveCapitalPerLeg() that reads from allocator when unified capital enabled, with automatic fallback to cfg.CapitalPerLeg
- Performance-weighted allocation (CA-03) computed from trailing APR at every scan cycle via updateAllocation() in both engines
- Dynamic strategy shifting (CA-04) available via dynamicStrategyPct() helper that shifts idle-strategy capital
- GET /api/allocation endpoint returns full CapitalSummary (exposure, by-strategy, by-exchange, effective pcts, pool total, capital per leg)
- Config POST handles named profile selection (applies bundled fields) and auto-detects custom overrides
- All 7 allocation config fields surfaced in config GET/POST and persisted to Redis

## Task Commits

Each task was committed atomically:

1. **Task 1: Engine + risk manager capital integration** - `31fdb4a` (feat)
2. **Task 2: Scan-cycle allocation wiring (CA-03/CA-04)** - `c9da479` (feat)
3. **Task 3: API layer - allocation endpoint, config, profile handling** - `3005c37` (feat)

## Files Created/Modified
- `internal/engine/capital.go` - Added effectiveCapitalPerLeg(), updateAllocation(), dynamicStrategyPct() methods
- `internal/engine/engine.go` - Replaced direct CapitalPerLeg reads, wired updateAllocation at EntryScan
- `internal/risk/manager.go` - Added effectiveCapitalPerLeg() method, replaced 5 CapitalPerLeg references in sizing
- `internal/spotengine/engine.go` - Added updateAllocation(), wired into discoveryLoop, updated capitalForExchange
- `internal/api/handlers.go` - Added allocation response/update structs, handleGetAllocation, profile handling, custom detection
- `internal/api/server.go` - Added allocator field, SetCapitalAllocator setter, /api/allocation route
- `cmd/main.go` - Wired apiSrv.SetCapitalAllocator(allocator)

## Decisions Made
- Used GetHistory(200)/GetSpotHistory(200) instead of nonexistent GetClosedPositions/GetClosedSpotPositions -- same data source, just with a limit parameter
- Set minimum 3 trades per strategy threshold before performance-weighted allocation tilt kicks in (prevents noisy APR from small sample)
- spotHasOpps defaults to cfg.SpotFuturesEnabled as a conservative heuristic (cannot cheaply query SpotEngine's opportunity state from the perp engine)
- Server gets allocator via SetCapitalAllocator setter (not constructor) to match existing dependency injection pattern (SetAnalyticsStore, SetExchangeScorer, etc.)
- Allocation fields persisted to Redis HASH alongside other config sections for consistency

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Used GetHistory/GetSpotHistory instead of nonexistent methods**
- **Found during:** Task 2 (updateAllocation implementation)
- **Issue:** Plan referenced e.db.GetClosedPositions() and e.db.GetClosedSpotPositions() which do not exist in the database package
- **Fix:** Used e.db.GetHistory(200) and e.db.GetSpotHistory(200) which return closed position history with the same data shape needed by ComputeStrategySummary
- **Files modified:** internal/engine/capital.go, internal/spotengine/engine.go
- **Verification:** go build ./... compiles cleanly
- **Committed in:** c9da479 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Trivial method name fix. Same data, different API. No scope creep.

## Issues Encountered
- Pre-existing broken `internal/api/auth_test.go` (untracked file from another agent) prevents `go test ./internal/api/...` from running. The file passes a `*database.Client` to `newAuthStore()` which no longer accepts one. This is out-of-scope and does not affect plan changes. Build (`go build ./...`) passes cleanly. All other package tests pass (239 tests in engine/risk/spotengine).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All backend engine integration complete, ready for Plan 03 (dashboard UI)
- Plan 03 will add dashboard tab for profile selection, allocation display, and capital pool visualization
- Frontend can call GET /api/allocation and POST /api/config with allocation section

## Self-Check: PASSED

All 7 modified files exist. All 3 task commits verified (31fdb4a, c9da479, 3005c37).

---
*Phase: 05-capital-allocation*
*Completed: 2026-04-04*
