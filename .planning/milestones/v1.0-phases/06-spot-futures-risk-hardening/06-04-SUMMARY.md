---
phase: 06-spot-futures-risk-hardening
plan: 04
subsystem: spotengine, ui
tags: [maintenance-rate, discovery, dashboard, i18n, spotengine]

# Dependency graph
requires:
  - phase: 06-01
    provides: ContractInfo.MaintenanceRate populated by all 5 adapters
provides:
  - MaintenanceRate field on SpotArbOpportunity flowing from discovery to dashboard
  - Dashboard maintenance rate column with color-coded risk levels
  - EN and ZH-TW i18n keys for maintenance rate display
affects: [future-risk-scoring, spot-futures-entry]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "lookupMaintenanceRateForDisplay() wraps getMaintenanceRate() with notional from config"
    - "Display-only field pattern: populate in discovery, render in dashboard, no scoring"

key-files:
  created: []
  modified:
    - internal/spotengine/discovery.go
    - internal/spotengine/discovery_test.go
    - web/src/types.ts
    - web/src/pages/Opportunities.tsx
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts
    - VERSION
    - CHANGELOG.md

key-decisions:
  - "lookupMaintenanceRateForDisplay() reuses existing getMaintenanceRate() with planned notional from config, avoiding new cache or API calls"
  - "Maintenance rate column added after Net APR and before Gap in both compact and full table views"
  - "Color coding: red >=10%, amber >=5%, gray <5%, dash for 0/unknown -- provides visual risk assessment without scoring"

patterns-established:
  - "Display-only data enrichment: add field to opportunity struct, populate in all scanner paths, render in dashboard, defer scoring to future observation"

requirements-completed: [SF-RISK-04]

# Metrics
duration: 7min
completed: 2026-04-05
---

# Phase 06 Plan 04: Discovery Display Summary

**MaintenanceRate field flows from exchange adapters through native/CoinGlass discovery to dashboard opportunities table with color-coded risk levels (red/amber/gray), display only per D-15**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-05T14:48:23Z
- **Completed:** 2026-04-05T14:55:48Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- MaintenanceRate field added to SpotArbOpportunity and populated in both native and CoinGlass discovery paths via lookupMaintenanceRateForDisplay()
- Dashboard opportunities table shows maintenance_rate column in both compact (MR) and full-width (Maint %) views with color-coded risk levels
- Both EN and ZH-TW locale files updated with maintenance rate translation keys and tooltips
- VERSION bumped to 0.28.3 with CHANGELOG entry for Phase 6 Plan 04

## Task Commits

Each task was committed atomically:

1. **Task 1: Add MaintenanceRate to SpotArbOpportunity + populate in discovery + test** - `0f01042` (feat)
2. **Task 2: Dashboard maintenance_rate column + i18n + version bump** - `9f1bbac` (feat)

## Files Created/Modified
- `internal/spotengine/discovery.go` - Added MaintenanceRate field to SpotArbOpportunity, lookupMaintenanceRateForDisplay() helper, populated in native + CoinGlass paths
- `internal/spotengine/discovery_test.go` - 3 tests: populated from contracts, fallback to default, not used for scoring
- `web/src/types.ts` - Added maintenance_rate to SpotOpportunity interface
- `web/src/pages/Opportunities.tsx` - MR/Maint % column in compact + full table with red/amber/gray color coding
- `web/src/i18n/en.ts` - 'spot.maintenanceRate': 'Maint %', 'spot.maintenanceRateTooltip'
- `web/src/i18n/zh-TW.ts` - 'spot.maintenanceRate': '維持率', 'spot.maintenanceRateTooltip'
- `VERSION` - 0.28.2 -> 0.28.3
- `CHANGELOG.md` - Phase 6 Plan 04 entry

## Decisions Made
- Reused existing `getMaintenanceRate()` via thin wrapper `lookupMaintenanceRateForDisplay()` rather than adding separate contract loading -- avoids cache duplication and extra API calls
- Maintenance rate column placed after Net APR and before Gap in both table views for logical reading order (yield -> risk -> entry cost)
- Color thresholds (10%/5%) chosen to highlight GUAUSDT-class contracts (30%) in red and moderate-risk in amber

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 6 complete: all 4 plans executed (adapter data, pre-entry gate, runtime monitor + health integration, discovery display)
- Maintenance rate data visible in production dashboard for user observation before deciding on scoring/filtering thresholds
- Future enhancement: once user observes real maintenance rates, can add scoring penalty or filter cutoff in discovery

---
*Phase: 06-spot-futures-risk-hardening*
*Completed: 2026-04-05*
