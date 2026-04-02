---
phase: 02-spot-futures-automation
plan: 01
subsystem: spotengine
tags: [loris, funding-rates, discovery, config, spot-futures, native-scanner]

# Dependency graph
requires:
  - phase: 01-spot-futures-exchange-expansion
    provides: 5 exchange margin adapters, SpotArbOpportunity struct, borrow rate cache
provides:
  - Native Loris-based discovery scanner replacing CoinGlass as primary data source
  - 9 new config fields for exit guards, basis gate, settlement guard, min-hold
  - pollLoris() method for HTTP polling of Loris API
  - runNativeDiscoveryScan() producing ranked Dir A + Dir B opportunities
  - CoinGlass fallback via runCoinGlassFallback() when Loris unavailable
affects: [02-spot-futures-automation, spot-futures-exit, spot-futures-dashboard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Loris rate normalization: raw/8 = bps/h, * 8760/10000 = APR"
    - "pollLorisFromURL() test helper pattern for HTTP mock injection"
    - "Both Dir A and Dir B generated per symbol+exchange (dual opportunity pattern)"

key-files:
  created:
    - internal/spotengine/native_scanner_test.go
  modified:
    - internal/config/config.go
    - internal/spotengine/discovery.go

key-decisions:
  - "Loris URL shared as package const, test helper uses URL injection via pollLorisFromURL()"
  - "runDiscoveryScan() is now a router: native when enabled, CoinGlass fallback otherwise"
  - "Both Dir A and Dir B generated for every positive-funding symbol+exchange pair"

patterns-established:
  - "6-touch-point config pattern: struct field, JSON tag, default, apply, toJSON, fromEnv"
  - "Native scanner with HTTP mock test pattern using httptest.NewServer"

requirements-completed: [SF-04]

# Metrics
duration: 9min
completed: 2026-04-02
---

# Phase 02 Plan 01: Native Discovery Scanner Summary

**Native Loris-based scanner replacing CoinGlass Chrome scraper, with net yield calculation (fundingAPR - borrowAPR - feeAPR) and 9 new config fields for Phase 2 exit/entry guards**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-02T07:45:48Z
- **Completed:** 2026-04-02T07:55:13Z
- **Tasks:** 1
- **Files modified:** 5 (config.go, discovery.go, native_scanner_test.go, CHANGELOG.md, VERSION)

## Accomplishments
- Native Loris scanner polls api.loris.tools/funding, normalizes 8h-equiv rates to APR, generates both Dir A (borrow_sell_long) and Dir B (buy_spot_short) opportunities per symbol+exchange
- CoinGlass Chrome scraper preserved as automatic fallback when Loris is unavailable
- 9 new config fields with full 6-touch-point coverage (struct, JSON, defaults, apply, toJSON, fromEnv)
- 10 unit tests covering Dir A/B generation, normalization, fallback, ranking, filter status, config defaults, source field, exchange filtering

## Task Commits

Each task was committed atomically:

1. **Task 1: Add 9 config fields + build native Loris scanner** - `7089328` (feat)

## Files Created/Modified
- `internal/config/config.go` - 9 new SpotFutures* config fields with all 6 touch-points
- `internal/spotengine/discovery.go` - pollLoris(), runNativeDiscoveryScan(), runCoinGlassFallback(), runDiscoveryScan() router
- `internal/spotengine/native_scanner_test.go` - 10 test functions (545 lines) covering native scanner behavior
- `CHANGELOG.md` - v0.24.6 entry documenting native scanner and config fields
- `VERSION` - bumped to 0.24.6

## Decisions Made
- Used URL injection pattern (pollLorisFromURL) for test mocking rather than interface-based HTTP client -- simpler, matches existing codebase style
- runDiscoveryScan() refactored to router function: delegates to native or CoinGlass based on SpotFuturesNativeScannerEnabled config flag
- Both Dir A and Dir B are generated for every positive-funding symbol+exchange pair; ranking by NetAPR naturally surfaces the better direction
- Existing CoinGlass test (TestRunDiscoveryScan_KeepsActivePositionWithNonPositiveFunding) continues to work via CoinGlass fallback path since test uses zero-value Config (NativeScannerEnabled=false)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree missing go.sum entries and web/dist directory. Fixed with `mkdir -p web/dist && touch web/dist/placeholder && go mod tidy`. Not a code issue -- infrastructure setup for worktree.
- Plan referenced `rate.YearlyRate` but the MarginInterestRate struct only has HourlyRate/DailyRate. Used existing pattern `rate.HourlyRate * 24 * 365` which matches all other code in the file.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Native scanner ready for consumption by discoveryLoop -> attemptAutoEntries pipeline (Plan 02)
- Config fields for exit guards (min-hold, settlement, basis, spread) are defined but not yet enforced -- Plan 02 will wire them
- CoinGlass fallback tested and working for reliability

---
*Phase: 02-spot-futures-automation*
*Completed: 2026-04-02*
