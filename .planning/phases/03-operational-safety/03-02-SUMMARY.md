---
phase: 03-operational-safety
plan: 02
subsystem: risk
tags: [loss-limits, rolling-window, redis-sorted-set, pre-entry-gate, pnl-tracking]

# Dependency graph
requires:
  - "03-01: TelegramNotifier with cooldown, EnablePerpTelegram config, shared notifier instance"
provides:
  - "LossLimitChecker with RecordClosedPnL and CheckLimits using Redis sorted sets"
  - "24h and 7d rolling PnL windows with automatic 8-day pruning"
  - "Pre-entry gate in executeArbitrage that blocks new entries on loss threshold breach"
  - "PnL event recording at both position close points (depth-fill exit and rotation)"
  - "BroadcastLossLimits WebSocket method for dashboard push"
  - "Config fields: EnableLossLimits, DailyLossLimitUSDT, WeeklyLossLimitUSDT, TelegramCooldownSec"
affects: [03-operational-safety, perp-perp-engine, dashboard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Redis sorted set (ZADD/ZRANGEBYSCORE/ZREMRANGEBYSCORE) for time-windowed event tracking"
    - "Fail-open error handling: query errors do not block entries"
    - "Nil-safe receiver methods on LossLimitChecker"
    - "Net PnL calculation: both wins and losses included in rolling windows"

key-files:
  created:
    - "internal/risk/loss_limit.go"
    - "internal/risk/loss_limit_test.go"
  modified:
    - "internal/config/config.go"
    - "internal/database/state.go"
    - "internal/engine/engine.go"
    - "internal/engine/exit.go"
    - "internal/api/server.go"
    - "cmd/main.go"

key-decisions:
  - "Fail-open on Redis query errors: if loss event query fails, entries are NOT blocked"
  - "Guard DailyLimit/WeeklyLimit > 0 before comparison to avoid blocking when limits are zero"
  - "PP-02 (circuit breaker) acknowledged as dropped per D-05 -- not implemented"
  - "Config struct fields only in this plan; JSON/apply/toJSON/fromEnv deferred to Plan 03-03"

patterns-established:
  - "Redis sorted set for time-windowed PnL tracking with automatic pruning"
  - "Pre-entry gate pattern: check before capacity logic in executeArbitrage"

requirements-completed: [PP-02, PP-03]

# Metrics
duration: 19min
completed: 2026-04-03
---

# Phase 03 Plan 02: Rolling Loss Limit System Summary

**Rolling-window loss limit system using Redis sorted sets with 24h/7d PnL tracking, pre-entry gate in executeArbitrage, and WebSocket broadcast to dashboard**

## Performance

- **Duration:** 19 min
- **Started:** 2026-04-03T00:32:26Z
- **Completed:** 2026-04-03T00:51:01Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Built LossLimitChecker with Redis sorted set backend for PnL event tracking across 24h and 7d rolling windows
- Integrated pre-entry gate in executeArbitrage that blocks new entries when daily or weekly loss thresholds are breached while allowing existing positions to continue being managed
- Added PnL recording at both position close points in exit.go (depth-fill exit and rotation close)
- Added BroadcastLossLimits WebSocket broadcast for dashboard real-time updates
- 7 comprehensive unit tests using miniredis covering daily, weekly, prune, allows, blocks, disabled, and nil-safety scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Add config fields + LossLimitChecker + Redis sorted set ops + unit tests** - `df6fc63` (feat)
2. **Task 2: Wire loss limiter into Engine + pre-entry gate + PnL recording + WS broadcast** - `e669917` (feat)

## Files Created/Modified
- `internal/risk/loss_limit.go` - LossLimitChecker with RecordClosedPnL and CheckLimits (24h/7d rolling windows)
- `internal/risk/loss_limit_test.go` - 7 unit tests using miniredis for all loss limit scenarios
- `internal/config/config.go` - Added EnableLossLimits, DailyLossLimitUSDT, WeeklyLossLimitUSDT, TelegramCooldownSec struct fields
- `internal/database/state.go` - Added keyLossEvents constant, RecordLossEvent (ZADD+prune), GetLossEventsInWindow (ZRANGEBYSCORE)
- `internal/engine/engine.go` - Added lossLimiter field, SetLossLimiter method, pre-entry gate in executeArbitrage
- `internal/engine/exit.go` - Added RecordClosedPnL calls at both position close points
- `internal/api/server.go` - Added BroadcastLossLimits WebSocket method
- `cmd/main.go` - Created and injected LossLimitChecker

## Decisions Made
- Fail-open on Redis query errors: if loss event query fails, entries are NOT blocked (safety over availability for a loss limiter, but Redis failures should not halt trading)
- Guard DailyLimit/WeeklyLimit > 0 before threshold comparison to avoid false blocks when limits are not configured (zero values)
- PP-02 (circuit breaker) acknowledged as dropped per D-05 decision -- no circuit breaker code implemented
- Config struct fields only added in this plan; remaining 5 touch points per field (JSON struct, defaults, applyJSON, toJSON, fromEnv) deferred to Plan 03-03

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Cherry-picked Plan 01 commits into worktree**
- **Found during:** Pre-execution setup
- **Issue:** Plan 02 depends on Plan 01 (telegram field, SetTelegram, EnablePerpTelegram, NotifyLossLimitBreached) but those commits were on a different branch
- **Fix:** Cherry-picked 2 code commits from Plan 01 branch; skipped docs-only commit with conflicts
- **Files modified:** N/A (pre-existing code from Plan 01)
- **Verification:** grep confirmed all Plan 01 symbols present

**2. [Rule 3 - Blocking] Created web/dist placeholder for go:embed**
- **Found during:** Build verification
- **Issue:** Worktree missing web/dist directory required by go:embed
- **Fix:** Created web/dist/.gitkeep placeholder
- **Files modified:** web/dist/.gitkeep
- **Verification:** go build succeeds

**3. [Rule 2 - Missing Critical] Added DailyLimit/WeeklyLimit > 0 guard**
- **Found during:** Task 1 implementation
- **Issue:** Without checking limit > 0, zero-value limits would cause false breach detection (any negative PnL < -0 is true)
- **Fix:** Added `status.DailyLimit > 0` and `status.WeeklyLimit > 0` guards before threshold comparisons
- **Files modified:** internal/risk/loss_limit.go
- **Verification:** TestLossLimitDisabled passes; disabled config does not block entries

---

**Total deviations:** 3 auto-fixed (2 blocking, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness and build. No scope creep.

## Issues Encountered
- go.sum entries missing in worktree -- resolved with `go mod tidy`

## User Setup Required
None - loss limits are gated by EnableLossLimits (default OFF). No external service configuration required.

## Known Stubs
None - all data paths are fully wired. Config fields exist as struct declarations; the JSON/default/apply/toJSON/fromEnv touch points are explicitly deferred to Plan 03-03 by design.

## Next Phase Readiness
- LossLimitChecker ready for Plan 03-03 config wiring (JSON, defaults, apply, toJSON, fromEnv for all safety config fields)
- BroadcastLossLimits ready for frontend consumption when dashboard is extended
- All 7 unit tests passing; risk and engine test suites green

## Self-Check: PASSED

All created files verified (internal/risk/loss_limit.go, internal/risk/loss_limit_test.go). All commits verified (df6fc63, e669917).

---
*Phase: 03-operational-safety*
*Completed: 2026-04-03*
