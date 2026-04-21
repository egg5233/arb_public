---
phase: 03-operational-safety
plan: 01
subsystem: notify
tags: [telegram, notifications, cooldown, perp-perp, api-errors, emergency-close]

# Dependency graph
requires: []
provides:
  - "TelegramNotifier with per-event-type cooldown (5-min window)"
  - "4 new notification methods: SL triggered, emergency close perp, consecutive API errors, loss limit breached"
  - "Perp-perp engine telegram field with SetTelegram injection"
  - "API error counter (per-exchange, triggers at 3 consecutive failures)"
  - "Shared TelegramNotifier instance across both engines"
  - "EnablePerpTelegram config struct field (default OFF)"
affects: [03-operational-safety, perp-perp-engine]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-event-type cooldown with sync.Mutex-protected map[string]time.Time"
    - "checkCooldownAt(key, now) for testable time injection"
    - "Shared notifier injection via SetTelegram + constructor parameter"
    - "API error counter with recordAPIError/recordAPISuccess pattern"

key-files:
  created: []
  modified:
    - "internal/notify/telegram.go"
    - "internal/notify/telegram_test.go"
    - "internal/engine/engine.go"
    - "internal/engine/engine_test.go"
    - "internal/spotengine/engine.go"
    - "internal/config/config.go"
    - "cmd/main.go"

key-decisions:
  - "Cooldown logic uses injectable time parameter (checkCooldownAt) for deterministic testing"
  - "Notifier does not gate on config -- callers (engine) check EnablePerpTelegram before calling"
  - "API error counter fires notification at exactly count==3, not on every subsequent failure"
  - "EnablePerpTelegram struct field only in this plan; JSON/apply/toJSON/fromEnv deferred to Plan 03-03"

patterns-established:
  - "Per-event-type cooldown: unique string key per event category, 5-min dedup window"
  - "Shared service injection: create in cmd/main.go, inject via Set* or constructor parameter"

requirements-completed: [PP-01]

# Metrics
duration: 8min
completed: 2026-04-03
---

# Phase 03 Plan 01: Perp-Perp Telegram Notifications Summary

**4 Telegram notification methods with per-event-type cooldown wired into perp-perp engine for SL triggers, L4/L5 emergency closes, and consecutive API errors**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-03T00:06:00Z
- **Completed:** 2026-04-03T00:14:10Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Extended TelegramNotifier with cooldown mechanism and 4 new perp-perp notification methods
- Wired notifications into perp-perp engine at SL trigger, L4/L5 emergency close, and PlaceOrder error paths
- Refactored both engines to share a single TelegramNotifier instance (created in cmd/main.go)
- Added EnablePerpTelegram config field (default OFF) gating all perp-perp notifications
- Added comprehensive tests: cooldown, nil-safety, API error counter, counter reset, counter disabled

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend TelegramNotifier with cooldown and perp-perp notification methods** - `4dbcc5a` (feat)
2. **Task 2: Wire TelegramNotifier into perp Engine + API error counter + shared instance** - `84b084d` (feat)

## Files Created/Modified
- `internal/notify/telegram.go` - Added cooldownMu/lastSent fields, checkCooldown/checkCooldownAt, NotifySLTriggered, NotifyEmergencyClosePerp, NotifyConsecutiveAPIErrors, NotifyLossLimitBreached
- `internal/notify/telegram_test.go` - Added TestCooldown, TestCooldownIndependent, expanded TestNilSafeNotify, TestNotifySLTriggeredFormat, TestNotifyLossLimitBreachedFormat
- `internal/engine/engine.go` - Added telegram field, apiErrMu/apiErrCounts, SetTelegram, recordAPIError/recordAPISuccess, notification calls at SL/L4/L5/depth-fill points
- `internal/engine/engine_test.go` - Added TestAPIErrorCounter, TestAPIErrorCounterReset, TestAPIErrorCounterDisabled
- `internal/spotengine/engine.go` - Changed NewSpotEngine to accept telegram parameter instead of creating internally
- `internal/config/config.go` - Added EnablePerpTelegram bool field
- `cmd/main.go` - Create shared TelegramNotifier, inject via SetTelegram and NewSpotEngine parameter

## Decisions Made
- Cooldown logic uses injectable time parameter (checkCooldownAt) for deterministic testing without real time waits
- Notifier does not gate on config -- the engine checks EnablePerpTelegram before calling notification methods, keeping the notifier reusable
- API error counter fires Telegram notification at exactly count==3, not on every subsequent failure, to avoid spam
- EnablePerpTelegram added as struct field only in this plan; remaining 5 config touch points (JSON, default, apply, toJSON, fromEnv) deferred to Plan 03-03 per plan spec

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree missing web/dist directory for go:embed -- created placeholder directory to allow compilation and testing

## User Setup Required
None - no external service configuration required. Notifications are gated by EnablePerpTelegram (default OFF).

## Next Phase Readiness
- TelegramNotifier ready for Plan 03-02 (loss limit notification) and Plan 03-03 (config wiring)
- EnablePerpTelegram struct field exists, ready for full config integration in Plan 03-03
- Both engines confirmed working with shared notifier instance

## Self-Check: PASSED

---
*Phase: 03-operational-safety*
*Completed: 2026-04-03*
