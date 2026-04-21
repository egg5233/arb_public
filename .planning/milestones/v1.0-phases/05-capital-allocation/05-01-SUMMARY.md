---
phase: 05-capital-allocation
plan: 01
subsystem: risk
tags: [capital-allocation, risk-profiles, performance-weighting, config]

# Dependency graph
requires:
  - phase: 04-performance-analytics
    provides: ComputeStrategySummary for APR data
provides:
  - Risk profile presets (conservative/balanced/aggressive) with ApplyProfile
  - Unified capital config fields (7 new fields with 6-touch-point)
  - ComputeEffectiveAllocation performance-weighted split algorithm
  - EffectiveCapitalPerLeg derived sizing with manual override
  - DynamicStrategyPct opportunity-based capital shifting
  - Extended CapitalSummary with allocation status
affects: [05-02-engine-integration, 05-03-dashboard-allocation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Profile preset map with ApplyProfile overwrite and ProfileBundledFields custom-detection"
    - "Performance-weighted allocation: 50/50 blend of base split + APR ratio, floor/ceiling clamped"
    - "Derived capital-per-leg: TotalCapitalUSDT / MaxPositions / 2 * SizeMultiplier"
    - "Dynamic shifting: freed = max(0, allocation - committed) for strategy with no opportunities"

key-files:
  created:
    - internal/risk/profiles.go
    - internal/risk/profiles_test.go
  modified:
    - internal/config/config.go
    - internal/risk/allocator.go
    - internal/risk/allocator_test.go

key-decisions:
  - "New allocation JSON section in config (not embedded in fund or risk) -- groups all Phase 5 fields"
  - "Reuse existing clamp function from exchange_scorer.go instead of duplicating"
  - "ComputeEffectiveAllocation is a package-level pure function for easy testing without allocator setup"
  - "SizeMultiplier applied only in EffectiveCapitalPerLeg, not in profile application"

patterns-established:
  - "Risk profile preset map: Profiles[name]ProfilePreset with ApplyProfile(cfg, name)"
  - "ProfileBundledFields() returns JSON paths for custom-detection in handlePostConfig"
  - "SetEffectiveAllocation/DynamicStrategyPct pattern: cache allocation, query with opp flags"

requirements-completed: [CA-01, CA-02, CA-03, CA-04]

# Metrics
duration: 7min
completed: 2026-04-04
---

# Phase 5 Plan 01: Core Capital Allocation Infrastructure Summary

**Risk profile presets, unified capital config (7 fields), performance-weighted allocation algorithm, derived capital-per-leg, and dynamic strategy shifting with 22 new tests**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-04T02:56:09Z
- **Completed:** 2026-04-04T03:03:10Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- 3 named risk profiles (conservative/balanced/aggressive) bundling 7 config parameters each, with ApplyProfile and ProfileBundledFields for custom-detection
- 7 new config fields with full 6-touch-point coverage: EnableUnifiedCapital, TotalCapitalUSDT, RiskProfile, AllocationLookbackDays, AllocationFloorPct, AllocationCeilingPct, SizeMultiplier
- Performance-weighted allocation algorithm that blends profile base split with trailing APR data, clamped to floor/ceiling bounds
- Derived capital-per-leg that computes TotalCapitalUSDT/MaxPositions/2 with SizeMultiplier, respecting manual CapitalPerLeg override
- Dynamic strategy shifting that frees uncommitted capital from strategy with no opportunities, within ceiling bounds
- 22 new tests: 7 profile tests + 15 allocator tests (all passing, 65 total in risk package)

## Task Commits

Each task was committed atomically (TDD: test -> implement):

1. **Task 1: Config fields + Risk profile presets + ApplyProfile** - `cd14eb4` (feat)
2. **Task 2: Allocator dynamic allocation + derived capital + shifting** - `267c9cc` (feat)

## Files Created/Modified
- `internal/risk/profiles.go` - ProfilePreset struct, 3 named profiles, ApplyProfile, ProfileBundledFields
- `internal/risk/profiles_test.go` - 7 tests for profile application, invalid name, bundled fields, config round-trip, defaults
- `internal/config/config.go` - 7 new struct fields, jsonAllocation struct, defaults, applyJSON, toJSON, fromEnv
- `internal/risk/allocator.go` - ComputeEffectiveAllocation, EffectiveCapitalPerLeg, DynamicStrategyPct, SetEffectiveAllocation, extended CapitalSummary, dynamic strategyPct
- `internal/risk/allocator_test.go` - 15 new tests for allocation, derived capital, and shifting logic

## Decisions Made
- New `allocation` JSON section in config rather than splitting across `fund` and `risk` sections -- groups all Phase 5 fields together, follows safety/analytics precedent
- Reused existing `clamp` function from `exchange_scorer.go` rather than duplicating in allocator.go
- Made `ComputeEffectiveAllocation` a package-level pure function (not a method) so it can be tested without Redis/allocator setup
- `SizeMultiplier` is applied only in `EffectiveCapitalPerLeg` derivation, not during profile application to config

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed duplicate clamp function**
- **Found during:** Task 2 (allocator extensions)
- **Issue:** Plan specified adding a `clamp` helper to allocator.go, but `exchange_scorer.go` in the same package already declares an identical `clamp` function, causing a compilation error
- **Fix:** Removed the duplicate from allocator.go and reused the existing declaration
- **Files modified:** internal/risk/allocator.go
- **Verification:** `go build ./internal/risk/` compiles successfully
- **Committed in:** 267c9cc (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Trivial dedup fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All backend infrastructure (profiles, config, allocator) ready for Plan 02 (engine integration)
- Plan 02 will wire EffectiveCapitalPerLeg into both engines' entry paths
- Plan 02 will call ComputeEffectiveAllocation from scan cycle using Phase 4 analytics data
- Plan 03 will add dashboard UI for profile selection and allocation display

---
*Phase: 05-capital-allocation*
*Completed: 2026-04-04*
