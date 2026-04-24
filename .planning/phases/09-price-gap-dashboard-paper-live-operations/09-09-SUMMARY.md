---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 09
subsystem: pricegaptrader
tags: [gap-closure, observability, config, dashboard, i18n]
gap_closure: true
requirements: [PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-OPS-05, PG-VAL-01, PG-VAL-02]
requires:
  - internal/config/config.go PriceGapPaperMode precedent (Plan 09-01)
  - internal/pricegaptrader/detector.go DetectionResult.Reason populator
  - web/src/pages/PriceGap.tsx paper-mode header toggle (Plan 09-07)
provides:
  - internal/config: PriceGapDebugLog bool field + price_gap.debug_log JSON round-trip (applyJSON + SaveJSON)
  - internal/pricegaptrader: Tracker.shouldLogNoFire(symbol, reason, now) rate-limiter + 60s cooldown constant
  - internal/api: priceGapUpdate.DebugLog + priceGapStateResponse.DebugLog for dashboard round-trip
  - web/src/pages/PriceGap.tsx: debug-log header toggle + debugLog state + toggleDebugLog
  - web/src/i18n/{en,zh-TW}.ts: pricegap.debugLog + pricegap.debugLogTooltip lockstep
affects:
  - internal/pricegaptrader/tracker.go runTick non-fire path — previously silent, now logs one line per (symbol, reason) per 60s when flag ON
tech-stack:
  added: []
  patterns:
    - CLAUDE.local.md new-feature rollout pattern (config bool default OFF + dashboard toggle + config.json persistence)
    - Per-(key, window) rate-limit map (shared conceptually with Telegram notifier cooldown map in Plan 04)
key-files:
  created:
    - internal/pricegaptrader/tracker_debug_log_test.go (10 tests — 180 lines)
    - .planning/phases/09-price-gap-dashboard-paper-live-operations/09-09-SUMMARY.md
  modified:
    - internal/config/config.go (+6 lines: struct field, jsonPriceGap field, applyJSON branch, SaveJSON raw-map write)
    - internal/config/config_test.go (+95 lines: 5 new PriceGapDebugLog tests)
    - internal/pricegaptrader/tracker.go (+43 lines: struct fields, constant, shouldLogNoFire helper, runTick gating — 319 → 362 lines)
    - internal/api/handlers.go (+3 lines: DebugLog field + applyJSON branch)
    - internal/api/pricegap_handlers.go (+2 lines: DebugLog field + state-seed assignment)
    - web/src/pages/PriceGap.tsx (+30 lines: debugLog state, toggleDebugLog, header toggle)
    - web/src/i18n/en.ts (+2 keys)
    - web/src/i18n/zh-TW.ts (+2 keys)
    - VERSION 0.34.1 → 0.34.2
    - CHANGELOG.md (0.34.2 entry prepended)
decisions:
  - "Cooldown = 60s, hard-coded constant. Rationale: 4 configured candidates × 3 non-fire reasons × 2 ticks/min = up to 24 dupe lines/min without a gate. 60s keeps operators informed at ~2-3 lines per candidate per observation window (~3-5 min minimum). Not operator-configurable to keep the config surface minimal — the shipped default is the same flood-guard value the detector test harness uses. If operational experience shows 60s is too coarse (or too chatty), a future plan can promote the constant to a config field."
  - "Log level = .Info (not .Debug). Rationale: utils.Logger.Debug is filtered by default in production log config; .Info is the minimum tier visible in journalctl without extra flags. The cooldown is the spam gate, not the log level. Operators who don't want diagnostic lines keep the flag OFF; those who want them must see them with zero extra configuration."
  - "Log format: `pricegap: no-fire %s reason=%s spread_bps=%.2f staleness_sec=%.1f`. Rationale: single line per event, greppable key=value shape (matches existing pricegap: ... prefix convention used by openPair / exit logs), includes spread_bps + staleness_sec for root-cause triage without re-running the detector."
  - "Flag-check lives in runTick (the caller), not inside shouldLogNoFire. Rationale: keeps the helper deterministic and independently testable (9 of 10 unit tests drive the helper directly); one integration test covers the end-to-end OFF-silence contract."
  - "DebugLog added to priceGapStateResponse (API state seed) and PriceGapState (TS type). Not in the plan's Task 1 acceptance criteria but required for the dashboard toggle to hydrate on page load — otherwise the toggle is always stuck at false after refresh. Rule 2 (auto-add missing critical functionality): the dashboard round-trip is the point of the three-pillar pattern."
  - "Nested JSON form only (`price_gap.debug_log`). Rationale: the flat `price_gap_paper_mode` shortcut exists because Plan 09-02 had a specific dashboard-handler compatibility reason; debug_log has no such requirement, and avoiding the flat shortcut keeps the config schema cleaner."
  - "VERSION 0.34.1 → 0.34.2, not 0.34.0 → 0.34.1 as the plan specified. Rationale: VERSION was already bumped to 0.34.1 by commit e59be3d (reconcile fix) before this plan ran. Patch-level bump for this gap-closure is appropriate regardless of the starting point."
metrics:
  duration: 25min
  completed: 2026-04-24T11:00:00Z
---

# Phase 9 Plan 09: Debug-Log Gap Closure Summary

Closes Verification Gap #1 (tracker.go:287-289 silently discarding `det.Reason`) by adding a flag-gated, rate-limited non-fire log line in `runTick`. Follows the CLAUDE.local.md new-feature rollout pattern: config bool default OFF, dashboard toggle, config.json persistence. All 10 new pricegaptrader tests + 5 new config tests green; 111 pricegaptrader tests total green; API tests green; i18n sync green at 758 keys; frontend build green; go build green.

## Task-by-task outcomes

### Task 1 — Config field `PriceGapDebugLog` with JSON round-trip

TDD red → green in one cycle. Added struct field (line 279), `jsonPriceGap.DebugLog` pointer field (line 318), `applyJSON` branch (line 1343), and `SaveJSON` raw-map write (line 1756). 5 new tests cover default-OFF, applyJSON true/false, absent-preserves, and SaveJSON round-trip. `config.json` untouched. 25 config tests total green.

Commit: `356b40c feat(09-09): add PriceGapDebugLog config field with JSON round-trip`

### Task 2 — Rate-limited non-fire reason logger

Added `debugLogMu sync.Mutex` + `lastNoFireLogged map[string]time.Time` to the Tracker struct, package constant `priceGapNoFireLogCooldown = 60 * time.Second`, and helper `(t *Tracker) shouldLogNoFire(symbol, reason string, now time.Time) bool`. Wired into `runTick` replacing the bare `if !det.Fired { continue }` with a flag + cooldown-gated `.Info` line. 10 new tests:

1. `TestShouldLogNoFire_EmptyReasonRejected` — defensive empty-string gate
2. `TestShouldLogNoFire_FirstCallEmits` — first call returns true + records ts
3. `TestShouldLogNoFire_WithinCooldownSuppressed` — t+30s, t+59s suppressed
4. `TestShouldLogNoFire_AfterCooldownEmits` — t+60s, t+120s both emit (strict-less-than)
5. `TestShouldLogNoFire_DifferentReasonSameSymbol` — 3 reasons on SOONUSDT independent
6. `TestShouldLogNoFire_DifferentSymbolSameReason` — 3 symbols on stale_bbo independent
7. `TestShouldLogNoFire_FloodSuppressed` — 100 rapid calls → exactly 1 emit
8. `TestRunTick_DebugLog_FlagOffAndOn/FlagOff_NoMapGrowth` — 10 ticks flag OFF → 0 map entries
9. `TestRunTick_DebugLog_FlagOffAndOn/FlagOn_OneEntryWithinCooldown` — 10 ticks flag ON → 1 map entry

Commit: `b956565 feat(09-09): rate-limited non-fire reason logger in tracker.runTick`

### Task 3 — Dashboard toggle + i18n + VERSION/CHANGELOG

`web/src/pages/PriceGap.tsx`: `debugLog` useState, `toggleDebugLog` callback (POSTs `{ price_gap: { debug_log: next } }`), header `<label>` next to Paper/Live pill with amber `bg-amber-500/15 text-amber-400` tint when ON. `PriceGapState` type gains `debug_log: boolean`. `priceGapUpdate` in `internal/api/handlers.go` gains `DebugLog *bool` + apply branch (Rule 2 — handler wiring was implicitly required for the toggle to function). `priceGapStateResponse` gains `DebugLog bool` for seed hydration. i18n keys `pricegap.debugLog` + `pricegap.debugLogTooltip` added to en.ts and zh-TW.ts; sync script green at 758 keys. VERSION 0.34.1 → 0.34.2. CHANGELOG entry documents Gap #1 closure.

Commit: `6f4de44 feat(09-09): dashboard debug-log toggle + i18n + VERSION bump`

## Log format chosen (decision record)

```
pricegap: no-fire SOONUSDT reason=stale_bbo spread_bps=247.83 staleness_sec=92.4
```

- Prefix `pricegap:` matches existing pricegaptrader log convention (openPair, closePair, gate-blocked)
- `no-fire` is a stable grep anchor distinct from every other pricegap event
- `reason=` carries the detector's verbatim string (`sample_error: ...`, `insufficient_persistence`, `stale_bbo`)
- `spread_bps=` + `staleness_sec=` give root-cause context without re-running the detector
- `.Info` level so journalctl surfaces it by default; cooldown is the spam gate

## Cooldown duration rationale

60s hard-coded:
- 4 candidates × 3 reasons × 2 ticks/min = up to 24 dupe lines/min without a gate
- At 60s cooldown: ~2 lines/min per (candidate, reason) worst case; realistic observation shows 1 line per candidate per few minutes as reasons cycle
- Not operator-configurable to keep the surface minimal — promote to config field only if operational experience demands it

## Deviations from plan

### Rule 2 — Auto-add missing critical functionality

**1. [Rule 2 — Security/Correctness] Wired `DebugLog` through API layer**

- **Found during:** Task 3 reading of existing paper-mode plumbing
- **Issue:** The plan only called out `internal/config/config.go` for round-trip, but the dashboard's POST `/api/config` flows through `internal/api/handlers.go:priceGapUpdate`. Without adding `DebugLog *bool` there, the toggle would POST successfully (Content-Type accepted), `applyJSON` would never see the key (handler's typed struct drops unknown fields on nested binding), and cfg.PriceGapDebugLog would stay zero — silent failure.
- **Fix:** Added `DebugLog *bool` to `priceGapUpdate` + apply branch; added `DebugLog bool` to `priceGapStateResponse` for seed hydration; added `debug_log: boolean` to the TS `PriceGapState` interface; wired `setDebugLog(d.debug_log || false)` into the seed path.
- **Files modified:** `internal/api/handlers.go`, `internal/api/pricegap_handlers.go`, `web/src/pages/PriceGap.tsx`
- **Commit:** `6f4de44`

### Version drift from plan

**2. [Context] VERSION starting at 0.34.1, not 0.34.0**

- **Found during:** Task 3 VERSION check
- **Issue:** Plan wrote "bump to 0.34.1 for gap-closure patch" assuming VERSION=0.34.0. HEAD already carried 0.34.1 from commit e59be3d (reconcile fix) which landed after the plan was written.
- **Fix:** Bumped to 0.34.2 instead. CHANGELOG entry written as 0.34.2 accordingly; the 0.34.1 reconcile-fix entry remains untouched above the new entry.
- **Files modified:** `VERSION`, `CHANGELOG.md`
- **Commit:** `6f4de44`

## Known stubs

None. All new code is wired end-to-end: config field → API handler → dashboard toggle → runTick.

## Threat flags

None. No new network endpoint, no auth path, no file access pattern, no schema change. The added `/api/config` key is behind the existing Bearer-token auth gate. The added log output is `.Info` level → systemd journal only (no remote shipping).

## Verification against plan's success criteria

- [x] Gap #1 closed: det.Reason reaches production logs, gated by config flag, rate-limited per (symbol, reason)
- [x] New feature rollout pattern honored: default OFF + dashboard toggle + config.json persistence
- [x] i18n lockstep preserved (en ↔ zh-TW, 758 keys)
- [x] Version + CHANGELOG updated (0.34.1 → 0.34.2, new entry prepended)
- [x] All automated acceptance criteria satisfied
- [x] `go test ./internal/config/... ./internal/pricegaptrader/... -count=1 -race -short` green
- [x] `bash scripts/check-i18n-sync.sh` green
- [x] `(cd web && npm run build) && go build ./...` green (frontend-first build order)
- [x] `git diff config.json` empty (CLAUDE.local.md hard rule)

## Self-Check: PASSED

Created files:
- FOUND: /var/solana/data/arb/internal/pricegaptrader/tracker_debug_log_test.go
- FOUND: /var/solana/data/arb/.planning/phases/09-price-gap-dashboard-paper-live-operations/09-09-SUMMARY.md

Commits:
- FOUND: 356b40c (Task 1 — config field)
- FOUND: b956565 (Task 2 — rate-limited logger)
- FOUND: 6f4de44 (Task 3 — dashboard toggle + i18n + VERSION bump)

Acceptance checks (spot-sampled):
- FOUND: `grep -c 'PriceGapDebugLog' internal/config/config.go` ≥ 3 (struct + applyJSON + SaveJSON reference path)
- FOUND: `grep -c '"debug_log"' internal/config/config.go` ≥ 1 (json tag)
- FOUND: `grep -c 'pricegap: no-fire' internal/pricegaptrader/tracker.go` ≥ 1
- FOUND: `grep -c 'shouldLogNoFire' internal/pricegaptrader/tracker.go` ≥ 2
- FOUND: `grep -c 'priceGapNoFireLogCooldown' internal/pricegaptrader/tracker.go` ≥ 1
- FOUND: `cat VERSION` outputs 0.34.2
- FOUND: CHANGELOG.md 0.34.2 entry with Gap #1 closure
- FOUND: `git diff config.json` empty

Unblocks: Plan 09-10 (Gap #2 — BBO subscription wiring) and Plan 09-11 (UAT walkthrough of the 14 blocked rows).
