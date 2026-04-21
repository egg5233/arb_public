---
phase: 07-milestone-polish
plan: 01
subsystem: api, ui
tags: [go, react, i18n, config, dashboard, maintenance-gate]

# Dependency graph
requires:
  - phase: 06-risk-hardening
    provides: SpotFuturesEnableMaintenanceGate, SpotFuturesMaintenanceDefault, SpotFuturesMaintenanceCacheTTL config fields
provides:
  - Dashboard API GET/POST for maintenance gate config fields
  - Config.tsx sf-general tab with toggle + 2 dependent number fields
  - Round-trip test covering API + Redis + config.json persistence
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [config field pipeline: struct -> response -> builder -> update -> persistence -> UI -> i18n]

key-files:
  created:
    - internal/api/config_handlers_test.go (appended TestHandleConfig_MaintenanceGateRoundTrip)
  modified:
    - internal/api/handlers.go
    - web/src/pages/Config.tsx
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts
    - .planning/REQUIREMENTS.md
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "Maintenance gate toggle placed in sf-general tab (top-level engine feature, not exit-specific)"
  - "Server-side validation matches config.go applyJSON: MaintenanceDefault 0 < val < 1.0, CacheTTL >= 1"

patterns-established:
  - "Config field 6-touch pipeline extended: config struct -> API response struct -> response builder -> update struct -> update handler -> Redis persistence -> Config.tsx -> i18n"

requirements-completed: [SF-RISK-01]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 7 Plan 01: Maintenance Gate Dashboard Config Summary

**Wired 3 maintenance gate config fields (toggle, default rate, cache TTL) through dashboard API and Config.tsx sf-general tab with server-side validation and round-trip test**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-06T01:30:39Z
- **Completed:** 2026-04-06T01:35:07Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Dashboard GET /api/config returns enable_maintenance_gate, maintenance_default, maintenance_cache_ttl in spot_futures object
- Dashboard POST /api/config accepts and persists all 3 maintenance gate fields with server-side validation
- Config.tsx sf-general tab shows Maintenance Rate Gate toggle with 2 dependent number fields that dim when gate OFF
- Both EN and ZH-TW locale files have 6 matching i18n keys
- SF-RISK-01 requirement marked Complete in REQUIREMENTS.md

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire maintenance gate fields through API handlers + test** - `c3bb635` (feat)
2. **Task 2: Add maintenance gate toggle and fields to Config.tsx + i18n** - `95c9204` (feat)
3. **Task 3: Update traceability and version** - `57fba09` (chore)

## Files Created/Modified
- `internal/api/handlers.go` - Added 3 fields to configSpotFuturesResponse, spotFuturesUpdate, response builder, update handler, and Redis persistence
- `internal/api/config_handlers_test.go` - Added TestHandleConfig_MaintenanceGateRoundTrip round-trip test
- `web/src/pages/Config.tsx` - Added maintenance gate toggle + 2 number fields in sf-general tab
- `web/src/i18n/en.ts` - Added 6 English translation keys for maintenance gate fields
- `web/src/i18n/zh-TW.ts` - Added 6 Traditional Chinese translation keys for maintenance gate fields
- `.planning/REQUIREMENTS.md` - SF-RISK-01 status updated to Complete
- `CHANGELOG.md` - Added v0.29.0 entry
- `VERSION` - Bumped to 0.29.0

## Decisions Made
- Maintenance gate toggle placed in sf-general tab (alongside engine, auto-entry, dry-run toggles) rather than sf-exit tab -- this is a top-level engine feature affecting both entry and runtime monitoring
- Server-side validation matches config.go applyJSON bounds: MaintenanceDefault must be > 0 and < 1.0, CacheTTL must be >= 1

## Deviations from Plan

None - plan executed exactly as written.

## Threat Flags

None found. All changes are within the existing authenticated dashboard API boundary (T-07-01 mitigated via server-side validation, T-07-02 accepted as non-sensitive).

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 7 Plan 01 is the only plan in Phase 7 -- milestone is complete
- All 21 requirements tracked in REQUIREMENTS.md are now Complete
- v1.0 milestone audit gap (SF-RISK-01 dashboard toggle) is closed

## Self-Check: PASSED

All 9 files verified present. All 3 task commits (c3bb635, 95c9204, 57fba09) verified in git log.

---
*Phase: 07-milestone-polish*
*Completed: 2026-04-06*
