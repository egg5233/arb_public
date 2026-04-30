---
phase: 12
plan: 01
subsystem: pricegaptrader
tags: [auto-promotion, controller, interfaces, tdd, PG-DISC-02]
requires:
  - .planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-04-SUMMARY.md  # Scanner CycleSummary/CycleRecord shapes
  - .planning/phases/11-auto-discovery-scanner-chokepoint-telemetry/11-02-SUMMARY.md  # Registry chokepoint + sentinel errors
  - internal/models/pricegap_interfaces.go  # PriceGapCandidate + PriceGapDirectionBidirectional
provides:
  - PromotionController + Apply(ctx, summary CycleSummary) error
  - 5 narrow interfaces (PromoteEventSink, TelemetrySink, PromoteNotifier, ActivePositionChecker, RegistryWriter)
  - PromoteEvent JSON shape (locked for Plan 02 RedisWSPromoteSink and Plan 04 frontend)
  - In-package fakes (fakeRegistry/fakeSink/fakeNotifier/fakeGuard/fakeTelemetrySink) reusable by Plan 03 scanner_test.go
affects:
  - internal/pricegaptrader/  # new files only; no existing files modified
tech-stack:
  added: []  # no new deps
  patterns: [interface-driven DI, in-memory streak counters, fail-safe guard, fire-and-forget notifier]
key-files:
  created:
    - internal/pricegaptrader/promotion.go
    - internal/pricegaptrader/promotion_test.go
    - internal/pricegaptrader/promotion_testhelpers_test.go
  modified: []
decisions:
  - "Pinned candidates (Direction != \"bidirectional\") count toward the global cap (len(currentList) >= maxCands) but are EXCLUDED from the controller's streak universe so they cannot be auto-demoted (D-09 + D-18 compatibility)"
  - "Audit-guard tests strip leading // before substring match so the file's own doc comments describing forbidden patterns (rec.Direction, EndedAt) do not false-positive"
  - "First pass over p.promoteStreaks deletes any key not in the accepted set BEFORE incrementing matched keys — implements D-02 strict-consecutive without depending on map iteration order"
metrics:
  duration: 18min
  completed: 2026-04-30
---

# Phase 12 Plan 01: PromotionController Interfaces and Apply — Summary

PromotionController for auto-promotion / auto-demotion of bidirectional price-gap candidates with per-candidate streak counters, cap-full HOLD semantics, active-position guard fail-safe, and three locked event surfaces (sink/telemetry/notifier).

## What changed

- **`internal/pricegaptrader/promotion.go`** (new, 583 lines): Declares 5 narrow interfaces (`PromoteEventSink`, `TelemetrySink`, `PromoteNotifier`, `ActivePositionChecker`, `RegistryWriter`), the `PromoteEvent` struct with locked JSON tags per D-10, and the `PromotionController` with `NewPromotionController(...)`, `Apply(ctx, summary CycleSummary) error`, `SetNowFunc`, and `String()`. Direction is pinned to the literal `"bidirectional"` everywhere (D-18) — controller never reads `rec.Direction` (the field doesn't exist on `CycleRecord`).
- **`internal/pricegaptrader/promotion_testhelpers_test.go`** (new, 173 lines, test build only): Five reusable fakes (`fakeRegistry`, `fakeSink`, `fakeNotifier`, `fakeGuard`, `fakeTelemetrySink`). Plan 03's scanner integration test inherits these without re-declaration (same package, `_test.go` suffix).
- **`internal/pricegaptrader/promotion_test.go`** (new, 389 lines): 13 unit tests covering D-01 through D-18: streak increment / reset / hold; promote fire; cap-full silent skip + `IncCapFullSkip`; dedupe; demote streak; guard-blocked + guard-error fail-safe; Telegram key fields; JSON event shape; and 3 audit-guard tests that scan the source for the forbidden patterns (`rec.Direction`, `summary.EndedAt`, `bidirectional` literal count).

## How decisions D-01..D-18 are honored

| Decision | Implementation site |
|----------|--------------------|
| D-01 promote at >=6 consecutive | `if p.promoteStreaks[key] < promoteStreakThreshold { continue }` |
| D-02 strict-consecutive reset | First-pass `delete(p.promoteStreaks, key)` for keys not in `accepted` |
| D-03 in-memory streak storage | `make(map[string]int)` in constructor; cold restart resets |
| D-04 symmetric demote streak | `p.demoteStreaks[key]++` in current-tuples walk |
| D-05 active-position guard fail-safe | `if err != nil { ... HOLD streak } if blocked { ... HOLD streak }` |
| D-06 auto-demote mandatory | No skip-flag; demote runs unconditionally when streak crossed |
| D-07 single switch | NOT checked here (Plan 03 only constructs controller when enabled) |
| D-08 cap-full HOLD at 6 | `if len(currentList) >= maxCands { telemetry.IncCapFullSkip; HOLD }` |
| D-09 no auto-displacement | Controller only adds; never displaces existing tuples |
| D-10 PromoteEvent JSON keys | `json:"ts" / json:"action" / ... / json:"reason"` |
| D-11 Sink fans out to Redis+WS | `PromoteEventSink.Emit` interface (Plan 02 implements) |
| D-12 REST endpoint | NOT in this plan (Plan 02) |
| D-13 Telegram cooldown key fields | All 7 fields passed via `NotifyPromoteEvent(action, symbol, longExch, shortExch, direction, score, streak)` |
| D-14 Telegram format | NOT in this plan (Plan 02 NotifyPromoteEvent impl) |
| D-15 module boundary | Imports limited to context/errors/fmt + arb/internal/{config,models} + arb/pkg/utils |
| D-16 synchronous Apply | No goroutines; Apply called inline by Plan 03 |
| D-17 RegistryWriter interface | Declared here; `*Registry` from registry.go satisfies it |
| D-18 Direction pin | `const promoteEventDirection = "bidirectional"` used 9x in source |

## Contract drifts eliminated (Wave 2/3 audit fix)

- **No `rec.Direction` reads** — verified by `TestPromotion_NoDirectionFromCycleRecord` (CycleRecord has no Direction field per scanner.go:89; reading it would not compile, but the test guards future refactors).
- **No `summary.EndedAt` reads** — verified by `TestPromotion_CycleSummaryFieldNames`. Real field is `CompletedAt`.
- **`recAccepted` test helper omits Direction** — the helper builds `CycleRecord{Symbol, LongExch, ShortExch, Score, WhyRejected: ReasonAccepted}` only, matching the actual struct.
- **`bidirectional` literal pinned >=4 times** — verified by `TestPromotion_BidirectionalLiteralUsed` (actual count: 9).

## Plan 02 / 03 contract compliance

- **Plan 02 RedisWSPromoteSink** implements `PromoteEventSink.Emit(ctx, ev) error` — signature matches.
- **Plan 02 IncCapFullSkip** implements `TelemetrySink.IncCapFullSkip(ctx, symbol) error` — signature matches.
- **Plan 02 NotifyPromoteEvent** implements `PromoteNotifier.NotifyPromoteEvent(action, symbol, longExch, shortExch, direction string, score, streak int)` — signature matches; void return preserves fire-and-forget semantics.
- **Plan 03 wiring** can pass concrete `*Registry`, `*notify.TelegramNotifier`, `*RedisWSPromoteSink`, `*Telemetry`, `*DBActivePositionChecker` to `NewPromotionController` without modification.
- **Plan 04 frontend** receives JSON with the locked field set `{ts, action, symbol, long_exch, short_exch, direction, score, streak_cycles, reason}` — verified by `TestPromoteEventLog` substring match.

## Verification (last run)

```
go build ./...                                              # PASS (full repo)
go vet  ./internal/pricegaptrader/                          # PASS
go test ./internal/pricegaptrader/ -count=1                 # ok ... 2.460s
go test ./internal/pricegaptrader/ -run "TestPromotion|TestDemote" -count=1   # 13 functions PASS
```

## Commits

- `eb3504a` feat(12-01): declare PromotionController interfaces, PromoteEvent struct, and Apply method (D-01..D-18)
- `7c88a22` test(12-01): add shared in-package fakes for PromotionController and Scanner integration tests
- `e224d95` test(12-01): add PromotionController unit tests covering D-01..D-18 (13 cases)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Cap-full pre-seed direction**
- **Found during:** Task 3 first test run.
- **Issue:** `TestPromotionCapFull` pre-seeded ETHUSDT/SOLUSDT with `Direction: "bidirectional"`, which placed them in the controller's bidirectional streak universe. After 6 cycles where they were absent from `summary.Records`, the controller correctly demote-streaked them and fired 2 demote events — but the test asserted no events.
- **Fix:** Changed pre-seed to `Direction: "pinned"`. The cap-full check uses the global `len(currentList)` (correct: pinned candidates also occupy capacity), but `currentTuples` only indexes `Direction == "bidirectional"`, so the pinned seeds occupy cap without being demote-eligible.
- **Files modified:** `internal/pricegaptrader/promotion_test.go`
- **Commit:** `e224d95` (incorporated before commit; not a follow-up commit)

**2. [Rule 1 - Bug] Audit-guard tests false-positive on doc comments**
- **Found during:** Task 3 design.
- **Issue:** Plain `strings.Contains(src, "rec.Direction")` would have matched the file's own doc comment "controller MUST NEVER read rec.Direction" and flunked the audit.
- **Fix:** Audit-guard tests split source by line, trim leading whitespace, skip lines starting with `//`. Only code-level matches trip the assertion.
- **Files modified:** `internal/pricegaptrader/promotion_test.go`
- **Commit:** `e224d95`

### Asks (none)

No checkpoints, no auth gates, no architectural changes. Plan 01 was pure logic with in-package fakes per the plan objective.

## Self-Check

- [x] `internal/pricegaptrader/promotion.go` exists (583 lines)
- [x] `internal/pricegaptrader/promotion_testhelpers_test.go` exists (173 lines)
- [x] `internal/pricegaptrader/promotion_test.go` exists (389 lines)
- [x] commit `eb3504a` present in `git log`
- [x] commit `7c88a22` present in `git log`
- [x] commit `e224d95` present in `git log`
- [x] `go build ./...` succeeds
- [x] `go test ./internal/pricegaptrader/` passes
- [x] `grep -c bidirectional internal/pricegaptrader/promotion.go` returns 9 (>=4 required)
- [x] No code-level `rec.Direction`, `record.Direction`, or `EndedAt` references in promotion.go
- [x] No imports of internal/api, internal/database, or internal/notify in promotion.go

## Self-Check: PASSED
