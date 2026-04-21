---
phase: 05-capital-allocation
plan: 03
subsystem: ui
tags: [capital-allocation, dashboard, react, i18n, config, overview]

# Dependency graph
requires:
  - phase: 05-capital-allocation
    provides: Plan 01 config fields/profiles, Plan 02 API endpoints and engine wiring
provides:
  - Allocation strategy tab in Config page with profile selector and unified capital fields
  - Allocation summary card on Overview page with pool status and strategy splits
  - Full i18n coverage for allocation UI (English + Traditional Chinese)
  - VERSION 0.27.0 and CHANGELOG for Phase 5
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Direct fetch with localStorage token in FC component useEffect for standalone API calls"
    - "Violet color scheme for allocation tab, matching safety (emerald) and risk (amber) color coding"

key-files:
  created: []
  modified:
    - web/src/pages/Config.tsx
    - web/src/pages/Overview.tsx
    - web/src/i18n/en.ts
    - web/src/i18n/zh-TW.ts
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "Direct fetch in Overview.tsx useEffect rather than threading through App props -- keeps allocation data self-contained"
  - "Allocation tab uses violet color scheme to visually distinguish from other tabs"

patterns-established:
  - "Allocation tab pattern: renderAllocationTab with master toggle, profile buttons, grouped NumberFields"

requirements-completed: [CA-01, CA-02, CA-03, CA-04]

# Metrics
duration: 5min
completed: 2026-04-04
---

# Phase 5 Plan 03: Dashboard Allocation UI Summary

**Allocation tab in Config (profile selector, unified capital toggle, bounds) and Overview card (pool total, strategy splits, exchange exposure) with full i18n and version 0.27.0**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-04T03:27:14Z
- **Completed:** 2026-04-04T03:33:10Z
- **Tasks:** 2 of 3 (Task 3 is human-verify checkpoint)
- **Files modified:** 6

## Accomplishments
- 7th "Allocation" strategy tab in Config with violet color scheme, master toggle (EnableUnifiedCapital), 3-button risk profile selector (conservative/balanced/aggressive), capital pool fields (TotalCapitalUSDT, SizeMultiplier), and allocation bounds (floor/ceiling pct, lookback days)
- Allocation summary card on Overview page fetching GET /api/allocation, showing pool total, capital/leg, perp/spot split percentages, committed amounts, and per-exchange exposure breakdown
- 29 new i18n keys in both en.ts and zh-TW.ts covering all allocation config and overview strings
- VERSION bumped to 0.27.0 and CHANGELOG updated with all Phase 5 additions (CA-01 through CA-04)

## Task Commits

Each task was committed atomically:

1. **Task 1: Config.tsx allocation tab + i18n** - `4fca7c2` (feat)
2. **Task 2: Overview allocation card + VERSION + CHANGELOG** - `6694e8b` (feat)
3. **Task 3: Human verification** - checkpoint (pending)

## Files Created/Modified
- `web/src/pages/Config.tsx` - Added allocation strategy type, renderAllocationTab, allocation button in toggle bar, sub-tab exclusion
- `web/src/pages/Overview.tsx` - Added allocation state, fetch effect, allocation summary card with pool/strategy/exchange data
- `web/src/i18n/en.ts` - 29 new allocation keys (cfg.tab.allocation, cfg.alloc.*, overview.alloc.*)
- `web/src/i18n/zh-TW.ts` - 29 matching Traditional Chinese translation keys
- `CHANGELOG.md` - v0.27.0 entry with Phase 5 additions, changes, and notes
- `VERSION` - Bumped from 0.26.0 to 0.27.0

## Decisions Made
- Direct fetch with localStorage token in Overview.tsx useEffect rather than threading allocation data through App.tsx props -- keeps the allocation feature self-contained and avoids modifying App.tsx and OverviewProps
- Allocation tab uses violet color scheme (bg-violet-900/60, text-violet-200) to visually distinguish from risk (amber) and safety (emerald) tabs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all data wired to live API endpoints (/api/allocation and /api/config).

## Next Phase Readiness
- All Phase 5 work complete across 3 plans: infrastructure (Plan 01), engine integration (Plan 02), dashboard UI (Plan 03)
- Human verification checkpoint (Task 3) pending for visual confirmation of UI

## Self-Check: PASSED

All 6 modified files verified. Both task commits verified (4fca7c2, 6694e8b).

---
*Phase: 05-capital-allocation*
*Completed: 2026-04-04*
