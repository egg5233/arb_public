---
phase: 04-performance-analytics
plan: 03
subsystem: analytics
tags: [sqlite, api, config, pnl-decomposition, slippage, snapshot-writer]

# Dependency graph
requires:
  - phase: 04-01
    provides: "SQLite time-series store (store.go) and aggregator functions (aggregator.go)"
provides:
  - "EnableAnalytics config toggle with 6-touch-point convention"
  - "GET /api/analytics/pnl-history and GET /api/analytics/summary API endpoints"
  - "SnapshotWriter background goroutine for non-blocking position close recording"
  - "BackfillFromHistory for SQLite population from Redis on first startup"
  - "PnL decomposition (ExitFees, BasisGainLoss) persisted in reconcilePnL"
  - "BBO slippage capture at entry in both executeTrade paths"
affects: [04-04]

# Tech tracking
tech-stack:
  added: []
  patterns: [config 6-touch-point for analytics toggle, buffered channel snapshot writer, interface-based engine injection]

key-files:
  created:
    - internal/analytics/snapshot.go
    - internal/api/analytics_handlers.go
  modified:
    - internal/config/config.go
    - internal/api/server.go
    - internal/api/handlers.go
    - internal/engine/engine.go
    - internal/engine/exit.go
    - internal/spotengine/engine.go
    - internal/spotengine/exit_manager.go
    - cmd/main.go

key-decisions:
  - "Analytics routes always registered (return 503 when disabled) to avoid frontend 404s"
  - "SnapshotWriter uses buffered channel (size 100) with non-blocking send — analytics never blocks trading"
  - "BBO slippage captured after fill completion (not before) using current WS BBO — approximate but zero-risk"
  - "BasisGainLoss formula: reconciledPnL - reconciledFunding - rotationPnL + totalFees (isolates price movement)"
  - "Snapshot writer injected via interface to avoid circular imports between engine and analytics packages"

patterns-established:
  - "Interface injection pattern for optional engine dependencies (snapshotWriter interface{...})"
  - "Non-blocking analytics recording via buffered channels — drop events on full with warning log"
  - "Config toggle gates all initialization: no SQLite file, no goroutine, no backfill when disabled"

requirements-completed: [PP-04, AN-01, AN-02, AN-03, AN-05, AN-06]

# Metrics
duration: 19min
completed: 2026-04-04
---

# Phase 04 Plan 03: Analytics Backend Wiring Summary

**EnableAnalytics 6-touch config toggle, dual API endpoints for PnL history and summaries, SnapshotWriter with buffered non-blocking channel, PnL decomposition (ExitFees/BasisGainLoss) in reconcilePnL, BBO slippage capture at entry**

## Performance

- **Duration:** 19 min
- **Started:** 2026-04-03T22:46:49Z
- **Completed:** 2026-04-03T23:06:00Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- Added EnableAnalytics config toggle following 6-touch-point convention (struct, JSON apply, defaults, applyJSON, toJSON, applyEnv) with default OFF
- Created analytics REST API: GET /api/analytics/pnl-history (time-series from SQLite) and GET /api/analytics/summary (strategy summaries + exchange metrics from Redis)
- Built SnapshotWriter with buffered channel (100) and BackfillFromHistory for first-startup SQLite population
- Hooked PnL decomposition into reconcilePnL: ExitFees and BasisGainLoss persisted alongside RealizedPnL
- Captured BBO slippage at entry in both executeTrade and executeTradeV2 paths
- Connected snapshot writer to both perp and spot engines for position close events
- All 291 existing tests remain green, go vet clean

## Task Commits

Each task was committed atomically:

1. **Task 1: Config toggle, API endpoints, snapshot writer, main.go wiring** - `0a42ae7` (feat)
2. **Task 2: PnL decomposition and BBO slippage capture** - `ef304c1` (feat)

## Files Created/Modified
- `internal/config/config.go` - Added EnableAnalytics and AnalyticsDBPath with 6-touch-point convention
- `internal/api/analytics_handlers.go` - New file: handleGetAnalyticsPnLHistory and handleGetAnalyticsSummary handlers
- `internal/api/server.go` - Added analyticsStore field, SetAnalyticsStore setter, analytics route registration
- `internal/api/handlers.go` - Added analyticsUpdate struct, configAnalyticsResponse, POST/GET config support
- `internal/analytics/snapshot.go` - New file: SnapshotWriter goroutine, RecordPerpClose/RecordSpotClose, BackfillFromHistory
- `internal/engine/engine.go` - Added snapshotWriter interface field, SetSnapshotWriter setter, BBO slippage capture in both entry paths
- `internal/engine/exit.go` - Persist ExitFees/BasisGainLoss in tryReconcilePnL, RecordPerpClose after history update
- `internal/spotengine/engine.go` - Added snapshotWriter interface field, SetSnapshotWriter setter
- `internal/spotengine/exit_manager.go` - RecordSpotClose call after history write in completeExit
- `cmd/main.go` - Analytics import, guarded initialization block with store/writer/backfill, spot engine wiring

## Decisions Made
- Analytics routes are always registered but return 503 when analyticsStore is nil (disabled) -- avoids frontend 404 errors
- SnapshotWriter uses non-blocking send on buffered channel (100) -- drops events with warning log if full, trading never blocked
- BBO slippage is captured after fills complete using current WS BBO, not pre-fill snapshot -- approximate but zero risk to trading path
- BasisGainLoss formula isolates price movement: reconciledPnL - reconciledFunding - rotationPnL + totalFees
- Snapshot writer injected via interface (not concrete type) to avoid circular imports between engine and analytics

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Merge conflict resolution for config.go defaults**
- **Found during:** Task 1 (config toggle setup)
- **Issue:** Worktree was behind dev branch; Wave 1 outputs (analytics store, aggregator, model fields) not present
- **Fix:** Merged dev into worktree branch, resolved one conflict in config.go defaults section (kept HEAD's pool allocator fields)
- **Files modified:** internal/config/config.go (merge resolution)
- **Verification:** go build passes after merge

**2. [Rule 1 - Bug] Variable name conflict in executeTrade**
- **Found during:** Task 2 (BBO slippage capture)
- **Issue:** New `var slippage` conflicted with existing `slippage := e.cfg.SlippageBPS / 10000.0` in same scope
- **Fix:** Renamed to `entrySlippage` to avoid redeclaration
- **Files modified:** internal/engine/engine.go
- **Verification:** go build succeeds, go vet clean

**3. [Rule 2 - Missing Critical] Config POST handler and response for analytics**
- **Found during:** Task 1 (config toggle setup)
- **Issue:** Plan only specified 6 touch points in config.go but didn't mention the API handler side (POST config, GET config response, configUpdate struct) which is required for dashboard toggle
- **Fix:** Added analyticsUpdate struct, configAnalyticsResponse, POST handler support, and buildConfigResponse integration
- **Files modified:** internal/api/handlers.go
- **Verification:** Build passes, config round-trips correctly

---

**Total deviations:** 3 auto-fixed (1 blocking, 1 bug, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - analytics is disabled by default. Enable via config JSON, env var, or dashboard toggle.

## Known Stubs
None - all functions fully implemented with real logic.

## Next Phase Readiness
- Analytics backend fully wired: store, API, config toggle, engine hooks, snapshot writer
- Ready for Plan 04 (frontend dashboard) to consume /api/analytics/pnl-history and /api/analytics/summary
- Enable analytics by setting enable_analytics=true in config.json or ENABLE_ANALYTICS=true env var

## Self-Check: PASSED

All 10 created/modified files verified on disk. Both task commits (0a42ae7, ef304c1) verified in git history.
