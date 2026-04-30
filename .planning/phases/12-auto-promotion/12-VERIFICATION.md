---
phase: 12-auto-promotion
verified: 2026-04-30T00:00:00Z
status: human_needed
score: 5/5 success_criteria verified (automated); 4 manual checkpoints pending
re_verification: false
requirements_verified:
  - id: PG-DISC-02
    status: SATISFIED
    evidence: "internal/pricegaptrader/promotion.go (controller); REQUIREMENTS.md:90 marks Phase 12 Complete; CHANGELOG 0.36.0 (2026-04-30)"
human_verification:
  - test: "Discovery dashboard timeline renders newest-first WS push without page reload"
    expected: "LPUSH pg:promote:events '{...}' on staging Redis updates timeline within ≤1 cycle"
    why_human: "Live WS subscription rendering can't be asserted by unit test"
  - test: "Telegram per-event message format and 5-min cooldown behaviour"
    expected: "[PG promote] {symbol} {long}↔{short} (bidirectional) score=N streak=N — single message per event; flap suppressed; distinct candidates pass through"
    why_human: "Requires real Telegram bot token + channel"
  - test: "Cold restart 6-cycle baseline (D-03 in-memory streak)"
    expected: "After daemon kill at counter=5 and restart, controller starts at 0 and needs 6 fresh cycles before fire"
    why_human: "Requires daemon kill + observation across cycles"
  - test: ".bak rotation on auto-promote write"
    expected: "config.json.bak.{ts} appears after first auto-promote with prior content"
    why_human: "Filesystem side-effect requires real promote event"
---

# Phase 12: Auto-Promotion Verification Report

**Phase Goal:** Auto-promote eligible discovery candidates to live tracker positions when streak + score thresholds are met, surface events to dashboard + Telegram, with default-OFF master switch and active-position guard.

**Verified:** 2026-04-30
**Status:** human_needed (all automated checks pass; 4 manual checkpoints remain per VALIDATION.md)
**Re-verification:** No — initial verification.

## Goal Achievement — Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | With `PriceGapDiscoveryEnabled=true` and a candidate scoring ≥ `PriceGapAutoPromoteScore` for ≥6 consecutive cycles, controller appends to `cfg.PriceGapCandidates` via chokepoint with `.bak` rotation | ✓ VERIFIED | `internal/pricegaptrader/promotion.go:179-345` (streak counter, Apply call); `cmd/main.go:502-545` (default-OFF gate); chokepoint via `*Registry.Add` (Phase 11 carried over) |
| 2 | Cap (`PriceGapMaxCandidates`, default 12) enforced — 13th cannot be appended | ✓ VERIFIED | `promotion.go` cap-full silent skip + `cap_full_skips:{symbol}` HASH on `pg:scan:metrics` via `telemetry.go:306-314` (IncCapFullSkip). Test `TestPromotionCapFull` passes |
| 3 | Idempotent dedupe rejects duplicate `(symbol, longExch, shortExch, direction)` tuples | ✓ VERIFIED | Registry.Add returns ErrDuplicate; controller HOLDs streak. Test `TestPromotionDedupe` passes |
| 4 | Auto-demote honors `pg:positions:active` guard | ✓ VERIFIED | `active_position_checker.go:54-78` (default-deny on Redis error); test `TestDemoteBlockedByGuard` passes |
| 5 | Promote/demote events fire Telegram critical alert AND WS broadcast; appear in dashboard timeline | ✓ VERIFIED (code) / human-needed (UX) | `telegram.go:423-455` NotifyPromoteEvent w/ unique cooldown key; `promote_event_sink.go:64+` RPush + LTrim 1000 + WS Broadcast("pg_promote_event"); `web/src/components/Discovery/PromoteTimeline.tsx` (7.2K) renders timeline; manual UX confirmation pending |

**Score:** 5/5 success criteria verified by code + tests (manual UX checkpoint #5 pending per `<human_verification>`).

## Decisions Honored (D-01 .. D-18)

| Decision | Status | Evidence |
|---|---|---|
| D-01 streak counter ≥6 | ✓ PASS | `promotion.go:179` promoteStreaks map; counter increments at threshold |
| D-02 strict consecutive | ✓ PASS | reset on miss; tested in `TestPromotionStreakReset` |
| D-03 in-memory streak | ✓ PASS | no Redis key for streak; map only |
| D-04 symmetric demote | ✓ PASS | mirrored 6-cycle counter for demote (CHANGELOG entry, tests) |
| D-05 active-position guard fail-safe | ✓ PASS | `active_position_checker.go:60` returns `(true, err)` on Redis error |
| D-06 mandatory auto-demote | ✓ PASS | demote logic implemented (no manual-only mode) |
| D-07 single switch reuse | ✓ PASS | `cmd/main.go:502` reuses `PriceGapDiscoveryEnabled` (no new flag) |
| D-08 cap-full silent skip + cap_full_skips | ✓ PASS | `telemetry.go:306` IncCapFullSkip per-symbol HASH |
| D-09 no auto-displace | ✓ PASS | code rejects above cap, never displaces |
| D-10 LIST + LTrim 1000 | ✓ PASS | `promote_event_sink.go:64` RPush + LTrim 1000 to `pg:promote:events` |
| D-11 WS pg_promote_event broadcast | ✓ PASS | `promote_event_sink.go:47` `wsEventPromoteEvent = "pg_promote_event"` |
| D-12 REST seed endpoint | ✓ PASS | `internal/api/server.go:197` `GET /api/pg/discovery/promote-events`; `pricegap_discovery_handlers.go:87-105` LRange returns LIST |
| D-13 unique cooldown key | ✓ PASS | `telegram.go:450` `"pg_promote:" + action + ":" + symbol + ":" + longExch + ":" + shortExch + ":" + direction` |
| D-14 message format | ✓ PASS | `telegram.go:423` NotifyPromoteEvent emits `[PG promote/demote] …` (manual confirm pending) |
| D-15 module boundary | ✓ PASS | `promotion.go` imports only `context errors fmt arb/internal/config arb/internal/models arb/pkg/utils`; sink/checker concrete impls keep all internal/api & internal/notify access behind interfaces (`WSBroadcaster`) |
| D-16 chokepoint synchronous Apply | ✓ PASS | `scanner.go:332-347` calls `s.promotion.Apply(ctx, summary)` immediately after `s.telemetry.WriteCycle` in same goroutine |
| D-17 scanner constructor swap + relaxed static test | ✓ PASS | `scanner.go` accepts `*Registry` + `*PromotionController`; `scanner_static_test.go:46-63` forbids only raw `cfg.PriceGapCandidates =` assignments |
| D-18 direction pinned to "bidirectional" | ✓ PASS | `promotion.go:61` `const promoteEventDirection = "bidirectional"` used in 5+ call sites; no `rec.Direction` reads |

## Module Boundary Audit (D-15)

`pricegaptrader/promotion.go` imports: `context`, `errors`, `fmt`, `arb/internal/config`, `arb/internal/models`, `arb/pkg/utils`.
- ✓ No `internal/api` import (uses `WSBroadcaster` interface in `promote_event_sink.go`)
- ✓ No `internal/notify` import (uses `PromoteNotifier` interface)
- ✓ `internal/database` only used in `promote_event_sink.go` + `active_position_checker.go` concrete impls (allowed per D-15: same package)

## i18n Parity Audit

- EN keys with prefix `pricegap.discovery.timeline.`: 13
- zh-TW keys with prefix `pricegap.discovery.timeline.`: 13
- `diff <(grep ... en.ts | sort -u) <(grep ... zh-TW.ts | sort -u)`: **identical**
- Includes 2 retained Phase 11 keys (`title`, `placeholder`) + 11 new Phase 12 keys (actionPromote, actionDemote, scoreLabel, streakLabel, empty, loading, seedError, wsDisconnected, loadMore, sr.promote, sr.demote)
- `parity.test.ts`: 5/5 PASS

## Default-OFF Safety Audit (D-07 / D-11 must_check)

`cmd/main.go:502` — `if cfg.PriceGapDiscoveryEnabled {` block contains:
- `pgPromoteSink := pricegaptrader.NewRedisWSPromoteSink(...)`
- `pgGuard := pricegaptrader.NewDBActivePositionChecker(...)`
- `pgPromotion, perr := pricegaptrader.NewPromotionController(...)`
- `pgScanner := pricegaptrader.NewScanner(..., pgPromotion, ...)`

else-branch (line 543-545): logs both phase-11 and phase-12 disabled. **No PromotionController is constructed when flag is false.** ✓

## Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/pricegaptrader/promotion.go` | controller core | ✓ VERIFIED | 23.3K; 5 interfaces + PromoteEvent + PromotionController + Apply |
| `internal/pricegaptrader/promote_event_sink.go` | Redis+WS sink | ✓ VERIFIED | 3.5K; RPush + LTrim 1000 + WS Broadcast |
| `internal/pricegaptrader/active_position_checker.go` | guard | ✓ VERIFIED | 3.5K; default-deny fail-safe |
| `internal/pricegaptrader/promotion_test.go` | unit tests | ✓ VERIFIED | 14.3K; 14 tests pass |
| `internal/pricegaptrader/promotion_testhelpers_test.go` | fakes | ✓ VERIFIED | 4.0K |
| `internal/notify/telegram.go` NotifyPromoteEvent | Telegram alert | ✓ VERIFIED | line 423-455 |
| `internal/api/pricegap_discovery_handlers.go` | REST seed | ✓ VERIFIED | line 87-105 LRange |
| `internal/api/server.go` route | route registered | ✓ VERIFIED | line 197 |
| `web/src/components/Discovery/PromoteTimeline.tsx` | populated timeline | ✓ VERIFIED | 7.2K; replaces deleted Placeholder |
| `web/src/components/Discovery/PromoteTimelinePlaceholder.tsx` | should be deleted | ✓ VERIFIED | file does not exist |
| `web/src/hooks/usePgDiscovery.ts` | hook ext | ✓ VERIFIED | 11.7K |
| `VERSION` | 0.36.0 | ✓ VERIFIED | reads `0.36.0` |
| `CHANGELOG.md` | dated 0.36.0 entry | ✓ VERIFIED | `## [0.36.0] - 2026-04-30` mentions Phase 12 PG-DISC-02 |

## Key Link Verification

| From | To | Via | Status |
|---|---|---|---|
| `Scanner.RunCycle` | `PromotionController.Apply` | `s.promotion.Apply(ctx, summary)` after `WriteCycle` | ✓ WIRED (scanner.go:344) |
| `PromotionController` | `Registry.Add/Delete` | RegistryWriter interface | ✓ WIRED |
| `PromotionController` | Telegram | PromoteNotifier → TelegramNotifier.NotifyPromoteEvent | ✓ WIRED (cmd/main.go:519) |
| `PromotionController` | WS hub | PromoteEventSink → RedisWSPromoteSink → WSBroadcaster (= *api.Hub) | ✓ WIRED (cmd/main.go:510, server.go Hub() accessor) |
| `PromotionController` | `pg:scan:metrics` cap_full_skips | TelemetrySink.IncCapFullSkip | ✓ WIRED (telemetry.go:314) |
| `cmd/main.go` controller construct | gated by `PriceGapDiscoveryEnabled` | `if cfg.PriceGapDiscoveryEnabled {` block | ✓ WIRED (cmd/main.go:502) |
| `DiscoverySection.tsx` | `PromoteTimeline` | line 26 import + line 133 render | ✓ WIRED |
| Frontend `PromoteTimeline` | REST seed | `usePgDiscovery` hook → GET /api/pg/discovery/promote-events | ✓ WIRED |
| Frontend `PromoteTimeline` | WS push | `usePgDiscovery` switch case `pg_promote_event` | ✓ WIRED (per UI-SPEC + 12-04 SUMMARY) |

## Test Coverage

| Suite | Command | Result |
|---|---|---|
| Phase 12 backend | `go test ./internal/pricegaptrader/ -run "TestPromotion\|TestDemote\|TestPromote\|TestScanner_NoRawCfg" -count=1` | ✓ 14 tests PASS |
| API endpoint + Discovery | `go test ./internal/api/ -run "Promote\|Discovery" -count=1` | ✓ 17 tests PASS |
| Telegram notify | `go test ./internal/notify/ -run "Promote" -count=1` | ✓ PASS |
| i18n parity | `node --test --experimental-strip-types src/i18n/parity.test.ts` | ✓ 5/5 PASS |
| PromoteTimeline FE | `node --test --experimental-strip-types src/components/Discovery/__tests__/PromoteTimeline.test.ts` | ✓ 12/12 PASS |
| Build sanity | `go build ./...` | ✓ PASS |
| Worktrees | `git worktree list` | ✓ main only |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Phase 12 unit tests fire | `go test ./internal/pricegaptrader/ -run "Promote\|Demote"` | 14 pass | ✓ PASS |
| Direction pinned to literal | `grep -c "promoteEventDirection" promotion.go` | 5+ | ✓ PASS |
| Default-OFF gate present | `grep -E "if cfg.PriceGapDiscoveryEnabled" cmd/main.go` | 1 match (line 502) | ✓ PASS |
| Module boundary clean | `grep -rE "internal/api\\\"" promotion.go` | 0 matches | ✓ PASS |
| WS event constant | `grep "pg_promote_event" promote_event_sink.go` | line 47 | ✓ PASS |
| REST endpoint registered | `grep "promote-events" server.go` | line 197 | ✓ PASS |

## Anti-Patterns Found

None. No TODOs/FIXMEs in Phase 12 source files. No empty implementations. No hardcoded empty data flowing to render.

## Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|---|---|---|---|
| PG-DISC-02 | 12-01..12-04 | ✓ SATISFIED | REQUIREMENTS.md:90 marks Phase 12 Complete; full controller + sinks + Telegram + dashboard wired and tested |

## Human Verification Required

Per VALIDATION.md `Manual-Only Verifications`, the following 4 items remain for browser/staging confirmation:

1. **Live WS push → timeline render**
   - Test: Build frontend + Go binary; start arb on staging Redis; manually `LPUSH pg:promote:events '{...}'`; verify timeline updates within 1 cycle without reload; toggle `PriceGapDiscoveryEnabled` and verify scanner+controller stop together.
   - Why human: Real-time WS subscription + visual rendering.

2. **Telegram message wording + cooldown**
   - Test: Configure dev Telegram bot; promote a fake candidate above threshold for 6 cycles; confirm `[PG promote] {symbol} {long}↔{short} (bidirectional) score={n} streak={n}` arrives once; force flap within 5 min (suppressed); force distinct candidate (passes through).
   - Why human: Real bot + channel needed.

3. **Cold restart 6-cycle baseline (D-03)**
   - Test: Run scanner with candidate at counter=5; kill daemon mid-cycle; restart; confirm counter resets to 0 and 6 fresh cycles required before promote.
   - Why human: Requires daemon kill + cycle observation.

4. **`.bak` rotation on auto-promote**
   - Test: After first auto-promote, confirm `config.json.bak.{ts}` exists with prior content.
   - Why human: Real promote event needs to fire end-to-end on staging.

## Gaps Summary

No automated gaps found. All 5 ROADMAP success criteria, all 18 D-decisions, all PG-DISC-02 contractual obligations are honored in code and verified by passing tests. Status is `human_needed` because VALIDATION.md explicitly lists 4 manual-only verifications that cannot be asserted programmatically (live WS rendering, real Telegram delivery, cold-restart timing, filesystem `.bak` side-effect).

---

*Verified: 2026-04-30*
*Verifier: Claude (gsd-verifier)*
