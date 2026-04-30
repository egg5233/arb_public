---
phase: 12
plan: 03
subsystem: pricegaptrader, api, cmd
tags: [auto-promotion, scanner-wiring, bootstrap, version, changelog, PG-DISC-02]
requires:
  - .planning/phases/12-auto-promotion/12-01-SUMMARY.md  # PromotionController + 5 narrow interfaces
  - .planning/phases/12-auto-promotion/12-02-SUMMARY.md  # RedisWSPromoteSink + IncCapFullSkip + NotifyPromoteEvent
  - internal/pricegaptrader/promotion.go                  # Plan 01 controller
  - internal/pricegaptrader/promote_event_sink.go         # Plan 02 sink
  - internal/pricegaptrader/scanner.go                    # Phase 11 Plan 04 scanner (modified here)
  - cmd/main.go                                           # bootstrap (modified here)
provides:
  - DBActivePositionChecker (concrete ActivePositionChecker against database.Client)
  - Scanner.RunCycle calls promotion.Apply after telemetry.WriteCycle (D-16)
  - cmd/main.go bootstrap: PromotionController constructed inside `if cfg.PriceGapDiscoveryEnabled` block
  - VERSION 0.36.0 + CHANGELOG.md [0.36.0] entry
  - Server.Hub() accessor (internal/api) — preserves D-15 boundary
affects:
  - internal/pricegaptrader/  # active_position_checker.go (new), scanner.go (signature swap + Apply), scanner_static_test.go (relaxation), scanner_test.go (NewScanner call sites + Test 16/17)
  - internal/api/             # server.go (new Hub() accessor)
  - cmd/                      # main.go (PromotionController bootstrap inside discovery-enabled block)
tech-stack:
  added: []  # zero new deps
  patterns:
    - "narrow interface declared at consumer (WSBroadcaster) — *Hub satisfies via duck typing"
    - "fail-safe guard (IsActiveForCandidate returns true on Redis read error)"
    - "configured-tuple preferred over wire-side roles (PG-DIR-01 Pitfall 5)"
    - "static-test regex relaxation with explicit chokepoint discipline note"
    - "Apply called synchronously after telemetry.WriteCycle (D-16 same goroutine)"
key-files:
  created:
    - internal/pricegaptrader/active_position_checker.go
    - internal/pricegaptrader/active_position_checker_test.go
  modified:
    - internal/pricegaptrader/scanner.go              # *Registry swap + promotion field + Apply call
    - internal/pricegaptrader/scanner_test.go         # NewScanner call-site updates + Test 16 (Apply call) + Test 17 (nil-safe)
    - internal/pricegaptrader/scanner_static_test.go  # relaxed regex (forbid raw cfg.PriceGapCandidates =)
    - internal/api/server.go                          # public Hub() accessor
    - cmd/main.go                                     # PromotionController bootstrap (inside if cfg.PriceGapDiscoveryEnabled)
    - VERSION                                         # 0.35.2 -> 0.36.0
    - CHANGELOG.md                                    # [0.36.0] - 2026-04-30 entry prepended after Unreleased
decisions:
  - "DBActivePositionChecker matches the CONFIGURED tuple (CandidateLongExch / CandidateShortExch from PG-DIR-01) and falls back to wire-side roles (LongExchange / ShortExchange) for legacy pre-Phase-999.1 positions. Direction is NOT a field on PriceGapPosition (only on PriceGapCandidate); auto-promotion only ever creates bidirectional candidates so any active position on the configured tuple blocks demote."
  - "Server.Hub() accessor added in internal/api rather than passing *Server to NewRedisWSPromoteSink so the pricegaptrader package never imports internal/api (D-15 module boundary preserved). *Hub satisfies the narrow WSBroadcaster interface via duck typing — the assertion happens at the cmd/main.go wiring site."
  - "scanner_static_test.go relaxation: removed `\\*Registry\\b` and `registry.(Add|Update|Delete|Replace)\\(` regex checks; new check is a single regex `PriceGapCandidates\\s*=` that forbids only RAW raw assignment. Chokepoint discipline is preserved by PromotionController having no *config.Config mutation surface (it only reads thresholds under cfg.RLock)."
  - "scenarioTwoLeg helper return type changed from *fakeRegistryReader to *Registry (concrete) so the constructor signature change compiles. Tests pass nil for the new promotion parameter — RunCycle's nil-check makes this safe."
  - "Test 16 (TestScanner_RunCycleCallsPromotionApply) runs 10 cycles total (4 to fill the bar ring + 6 to cross promoteStreakThreshold) so the controller actually fires a real promote event — proves the wiring AND the controller integration end-to-end. Test 17 (TestScanner_RunCycleNilPromotionSafe) confirms the nil-safe path."
  - "PromotionController.SetNowFunc called in cmd/main.go to swap the Plan 01 default (returns 0) with time.Now().UnixMilli() so production PromoteEvent.TS values reflect actual fire time. Tests inject deterministic clocks via the same hook."
metrics:
  duration: 25min
  completed: 2026-04-30
---

# Phase 12 Plan 03: Live Wiring of PromotionController into Scanner — Summary

This plan turns the dormant Plan 01 controller + Plan 02 sinks into a live integration: `Scanner.RunCycle` now calls `promotion.Apply` synchronously after `telemetry.WriteCycle`, the active-position guard reads `pg:positions:active` SMEMBERS via `*database.Client`, and `cmd/main.go` constructs the controller inside the existing `if cfg.PriceGapDiscoveryEnabled` block so the default-OFF safety is preserved. Version bumps to 0.36.0 with a changelog entry covering the full Phase 12 backend (Plans 01-03); Plan 04 swaps the dashboard placeholder card.

## What changed

### internal/pricegaptrader/active_position_checker.go (NEW, 84 lines)

`DBActivePositionChecker` implements the `ActivePositionChecker` interface declared in Plan 01. `IsActiveForCandidate` calls `db.GetActivePriceGapPositions()` which reads `pg:positions:active` SMEMBERS + dereferences the `pg:positions` HASH. For each position it compares `(Symbol, LongExch, ShortExch)`:

- Prefers `CandidateLongExch / CandidateShortExch` (PG-DIR-01 stamps the CONFIGURED tuple, distinct from wire-side roles under inverse fires)
- Falls back to `LongExchange / ShortExchange` for legacy pre-Phase-999.1 positions

On Redis read error returns `(true, err)` — D-05 fail-safe. The controller treats both `(true, nil)` and `(_, err)` as "blocked" and HOLDs the demote streak at threshold.

**Field-name discovery during execution:** `models.PriceGapPosition` uses `LongExchange / ShortExchange` (NOT `LongExch / ShortExch` — those are `PriceGapCandidate` field names). The plan correctly anticipated this and instructed me to USE THE ACTUAL NAMES — done.

### internal/pricegaptrader/active_position_checker_test.go (NEW, 5 tests)

- `TestDBActivePositionChecker_MatchesConfiguredTuple` — happy path, position with matching configured tuple → blocked=true
- `TestDBActivePositionChecker_NoMatchOnDifferentExchanges` — different short → not blocked
- `TestDBActivePositionChecker_NoMatchOnDifferentSymbol` — different symbol → not blocked
- `TestDBActivePositionChecker_EmptyActiveSet` — no positions → not blocked
- `TestDBActivePositionChecker_LegacyPositionUsesWireSideRoles` — empty CandidateLongExch/CandidateShortExch → falls back to wire-side roles → blocked=true (Pitfall 5)

All 5 tests use the existing `newE2EClient` miniredis helper from `tracker_test.go`.

### internal/pricegaptrader/scanner.go (MODIFIED)

- Struct field `registry RegistryReader` -> `registry *Registry` (D-17 widening)
- New struct field `promotion *PromotionController` (nil-safe)
- `NewScanner` signature: new `promotion *PromotionController` parameter inserted before `log`
- `RunCycle`: after `_ = s.telemetry.WriteCycle(ctx, summary)` succeeds, calls `s.promotion.Apply(ctx, summary)` if non-nil. Apply errors logged best-effort; cycle never aborts (Pitfall 6)

### internal/pricegaptrader/scanner_static_test.go (MODIFIED — relaxation)

The original `TestScanner_NoRegistryMutators` is replaced by `TestScanner_NoRawCfgCandidatesAssignment`. The single regex `PriceGapCandidates\s*=[^=]` (the `[^=]` excludes equality comparisons) forbids only raw `cfg.PriceGapCandidates = ...` and `s.cfg.PriceGapCandidates = ...` patterns. The `*Registry` field type and `registry.Add / registry.Delete / registry.List` calls are now permitted because they are the legitimate chokepoint route via `PromotionController`. The doc block at the top of the file explains the rationale.

### internal/pricegaptrader/scanner_test.go (MODIFIED)

- `fakeRegistryReader` (read-only stub) replaced by `newTestRegistry(cfg)` helper that builds a real `*Registry` against an empty `*config.Config`
- All 9 `NewScanner(...)` call sites updated: pass `nil` for the new promotion parameter
- `scenarioTwoLeg` return signature: `*fakeRegistryReader` -> `*Registry`
- Removed unused `arb/internal/models` import (no longer referenced)
- New Test 16 `TestScanner_RunCycleCallsPromotionApply` — runs 10 cycles (4 ring-fill + 6 streak-threshold) with a real `PromotionController` wired to in-package fakes; asserts an Add call landed on the registry AND a PromoteEvent was emitted on the sink with the correct tuple
- New Test 17 `TestScanner_RunCycleNilPromotionSafe` — confirms nil promotion does not panic

### internal/api/server.go (MODIFIED)

New public accessor: `func (s *Server) Hub() *Hub` (~10 lines). Returns the private `s.hub` so cmd/main.go can pass it into `pricegaptrader.NewRedisWSPromoteSink`. `*Hub` satisfies the narrow `pricegaptrader.WSBroadcaster` interface (declared in `promote_event_sink.go`) via its existing `Broadcast(eventType string, data interface{})` method. The pricegaptrader package still does NOT import internal/api — duck typing keeps the boundary clean.

### cmd/main.go (MODIFIED)

The existing `if cfg.PriceGapDiscoveryEnabled { ... NewScanner ... }` block (lines 502-510) was extended:

```go
if cfg.PriceGapDiscoveryEnabled {
    pgPromoteSink := pricegaptrader.NewRedisWSPromoteSink(db, apiSrv.Hub())
    pgGuard := pricegaptrader.NewDBActivePositionChecker(db)
    pgPromotion, perr := pricegaptrader.NewPromotionController(
        cfg, pgRegistry, pgGuard, pgPromoteSink, tg, pgTelemetry,
        utils.NewLogger("pg-promotion"),
    )
    if perr != nil { log.Error("FATAL: ..."); os.Exit(1) }
    pgPromotion.SetNowFunc(func() int64 { return time.Now().UnixMilli() })

    pgScanner := pricegaptrader.NewScanner(
        cfg, pgRegistry, exchanges, pgTelemetry, pgPromotion,
        utils.NewLogger("pg-scanner"),
    )
    pgTracker.SetScanner(pgScanner)
    log.Info("[phase-12] auto-promotion controller enabled (threshold=%d, max_candidates=%d)",
        cfg.PriceGapAutoPromoteScore, cfg.PriceGapMaxCandidates)
}
```

`tg` is the shared `*notify.TelegramNotifier` constructed earlier in main.go (line 226); its `NotifyPromoteEvent` method (Plan 02) satisfies the narrow `PromoteNotifier` interface and is nil-safe per Phase 6 convention. `apiSrv.Hub()` is the new accessor added above. `db` and `pgRegistry` are already in scope.

The else-branch logs `[phase-12] auto-promotion disabled (PriceGapDiscoveryEnabled=false)` so operators see why the controller isn't running.

### VERSION

`0.35.2` -> `0.36.0` (minor bump for new feature; semver consistent with Phase 11's 0.35.0 minor bump for the auto-discovery scanner).

### CHANGELOG.md

New `## [0.36.0] - 2026-04-30` section between `## [Unreleased]` and `## [0.35.2]`. Three subsections (Added, Changed, Safety) summarize all of Phase 12 (Plans 01-03 backend) plus the rollback procedure pointing operators at `POST /api/config` (NEVER `config.json` direct edit per CLAUDE.local.md).

## How decisions D-05, D-15, D-16, D-17 are honored

| Decision | Implementation site |
|----------|--------------------|
| D-05 active-position guard fail-safe | `DBActivePositionChecker.IsActiveForCandidate` returns `(true, err)` on Redis read error; tested via `TestDBActivePositionChecker_*` |
| D-15 module boundary | `pricegaptrader` does NOT import `internal/api` — `WSBroadcaster` declared inside `promote_event_sink.go`; `*api.Hub` satisfies it at the cmd/main.go wiring site via the new `Hub()` accessor |
| D-16 synchronous Apply | `Scanner.RunCycle` calls `s.promotion.Apply(ctx, summary)` AFTER `s.telemetry.WriteCycle(ctx, summary)` returns — same goroutine, no spawn |
| D-17 RegistryWriter chokepoint | Scanner field swapped `RegistryReader` -> `*Registry`; `*Registry` satisfies `RegistryWriter` (Plan 01 interface) via its existing `Add`/`Delete`/`List` methods |
| D-18 Direction pin | The controller passes `"bidirectional"` to all sinks; `DBActivePositionChecker` matches on `(Symbol, LongExch, ShortExch)` only (Direction is not a `PriceGapPosition` field) |

## Default-OFF safety verification

The PromotionController is constructed ONLY inside the `if cfg.PriceGapDiscoveryEnabled` block in cmd/main.go (line 502-547). When the flag is `false`:

- No `pgPromotion` variable exists in scope
- No call to `NewPromotionController` is made
- `pgScanner` is also never constructed (existing Phase 11 behavior)
- `pgTracker.SetScanner` is never called → `pgTracker.Start()` runs with the legacy single-cycle path

The Phase 11 scanner constructor change (now requires `*Registry`) is itself default-OFF compatible because the constructor is only called inside the same `if` block.

## Verification (last run)

```
go build ./...                                                                     # PASS
go vet  ./...                                                                      # PASS
go build -o /tmp/arb-test ./cmd                                                    # PASS (26MB binary)
go test ./internal/pricegaptrader/ -count=1                                        # 249 PASS, 2 SKIP
go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/...     # 371 PASS across 4 packages
grep -r "internal/api" internal/pricegaptrader/*.go                                # zero non-test matches
git worktree list                                                                  # only main checkout
```

## Commits

- `9d39ac4` feat(12-03): wire PromotionController into Scanner.RunCycle + add ActivePositionChecker (D-16, D-17, D-05)
- `5296250` feat(12-03): wire PromotionController into cmd/main.go bootstrap + bump v0.36.0

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `models.PriceGapPosition` field-name reality vs plan example**
- **Found during:** Task 1 design (before writing code).
- **Issue:** The plan ACTION block sketched the matcher as `if p.Symbol == c.Symbol && p.LongExch == c.LongExch && p.ShortExch == c.ShortExch && p.Direction == c.Direction`. Actual struct field names on `models.PriceGapPosition` (verified `internal/models/pricegap_position.go:42-56`) are `LongExchange / ShortExchange` for wire-side roles and `CandidateLongExch / CandidateShortExch` for the PG-DIR-01 configured tuple. There is NO `Direction` field on `PriceGapPosition` (only on `PriceGapCandidate`).
- **Fix:** Used the configured tuple (`CandidateLongExch / CandidateShortExch`) preferentially with fallback to wire-side roles for legacy pre-Phase-999.1 positions (Pitfall 5 in PG-DIR-01). Direction omitted from the comparison entirely — auto-promotion only ever creates bidirectional candidates per D-18, and any active position on the configured tuple regardless of direction MUST block the demote.
- **Files:** `internal/pricegaptrader/active_position_checker.go`, `internal/pricegaptrader/active_position_checker_test.go` (test 5 specifically covers the legacy-fallback path).
- **Plan acknowledged this risk** at line 191: "If the actual `models.PriceGapPosition` field names differ ... USE THE ACTUAL NAMES — do NOT invent." So this is a flagged plan-internal expectation, not a true deviation.

**2. [Rule 3 - Blocking] No public `Hub()` accessor on `*api.Server`**
- **Found during:** Task 2 first compile attempt would have failed (`apiSrv.Hub` undefined).
- **Issue:** The plan ACTION block assumed `apiSrv.Hub()` existed. `s.hub` is a private field on `*api.Server`; no accessor was defined.
- **Fix:** Added `func (s *Server) Hub() *Hub { return s.hub }` in `internal/api/server.go` adjacent to the existing `BroadcastPositionUpdate` method. `*Hub` already satisfies the narrow `pricegaptrader.WSBroadcaster` interface via its `Broadcast(string, interface{})` method (declared in `internal/api/ws.go:96`). The accessor is documented as the preserved D-15 boundary mechanism.
- **Files:** `internal/api/server.go`.
- **Plan-internal expectation noted** at line 319: "If `apiSrv.Hub()` is not the actual hub accessor, find the correct one (likely `apiSrv.GetHub()` or a public field). DO NOT invent; read the file." → Found that NO accessor existed; the cleanest path was to ADD `Hub()` rather than expose `s.hub` as a field. This is the smallest possible API surface addition (one method).

**3. [Rule 2 - Critical] PromotionController.SetNowFunc called from bootstrap**
- **Found during:** Task 2 design.
- **Issue:** Plan 01's `defaultNowMsImpl` returns 0 — production PromoteEvent.TS values would all be unix epoch 0 (1970-01-01) without an explicit `SetNowFunc`. Plan 03's plan ACTION block did NOT include this wiring.
- **Fix:** Added `pgPromotion.SetNowFunc(func() int64 { return time.Now().UnixMilli() })` immediately after `NewPromotionController` succeeds.
- **Files:** `cmd/main.go`.
- **Justification:** Plan 01's SUMMARY explicitly noted "Production callers (Plan 03) call SetNowFunc with the real time.Now-based source after construction" → this is the intended wiring; the omission from Plan 03's ACTION block was a plan oversight, not a deliberate skip.

**4. [Rule 1 - Bug] Test 16 streak-threshold off-by-one**
- **Found during:** Task 1 first test run (`go test -run TestScanner_RunCycleCallsPromotionApply` failed with "expected at least one registry.Add call from Apply").
- **Issue:** Initially set `cfg.PriceGapAutoPromoteScore=1` and ran only 4 cycles, expecting an Add call. Actual `promoteStreakThreshold = 6` (Plan 01 D-01) means 6 consecutive accepted cycles are required AFTER the persistence ring fills. So 4 cycles gives 4 ring-fill cycles + 0 accepted = 0 streak. Need 4 + 6 = 10 cycles total.
- **Fix:** Changed `runCycles(sc, ctx, 4, ...)` to `runCycles(sc, ctx, 10, ...)`. Test now passes.
- **Files:** `internal/pricegaptrader/scanner_test.go`.
- **Commit:** `9d39ac4` (incorporated before commit; not a follow-up).

### Asks (none)

No checkpoints, no auth gates, no architectural changes. The Plan 03 path was clean wiring after Plans 01 and 02 had built the components correctly.

## Threat Flags

None — all new surface areas are explicitly registered in the plan threat-models (T-12-13 .. T-12-18). T-12-13 mitigation (no raw `cfg.PriceGapCandidates =` assignment) is enforced by `TestScanner_NoRawCfgCandidatesAssignment`. T-12-14 mitigation (default-OFF gate) is enforced by the `if cfg.PriceGapDiscoveryEnabled` placement of `NewPromotionController`. T-12-16 mitigation (fail-safe guard) is enforced by `DBActivePositionChecker.IsActiveForCandidate` returning `(true, err)` and asserted by the controller-level test in `promotion_test.go` (Plan 01).

## Self-Check

- [x] `internal/pricegaptrader/active_position_checker.go` exists and contains `type DBActivePositionChecker struct`
- [x] `internal/pricegaptrader/active_position_checker.go` calls `g.db.GetActivePriceGapPositions()`
- [x] `internal/pricegaptrader/scanner.go` field is `registry  *Registry` (concrete)
- [x] `internal/pricegaptrader/scanner.go` field `promotion *PromotionController` exists
- [x] `internal/pricegaptrader/scanner.go` calls `s.promotion.Apply(ctx, summary)` AFTER `s.telemetry.WriteCycle`
- [x] `internal/pricegaptrader/scanner_static_test.go` contains regex `PriceGapCandidates\s*=`
- [x] `internal/pricegaptrader/scanner_static_test.go` does NOT contain `\*Registry\b` forbidden-token regex
- [x] `cmd/main.go` contains `NewPromotionController(`
- [x] `cmd/main.go` contains `NewRedisWSPromoteSink(db, apiSrv.Hub())`
- [x] `cmd/main.go` contains `NewDBActivePositionChecker(db)`
- [x] `cmd/main.go` contains `[phase-12] auto-promotion controller enabled` log line
- [x] `cmd/main.go` `NewPromotionController` is INSIDE `if cfg.PriceGapDiscoveryEnabled` block (line 502-547)
- [x] `internal/api/server.go` contains `func (s *Server) Hub() *Hub`
- [x] `VERSION` contains exactly `0.36.0`
- [x] `CHANGELOG.md` contains `## [0.36.0] - 2026-04-30`
- [x] `CHANGELOG.md` contains `Phase 12 (PG-DISC-02) auto-promotion controller`
- [x] `CHANGELOG.md` contains `cap_full_skips:{symbol}`
- [x] `CHANGELOG.md` contains `pg_promote_event`
- [x] `CHANGELOG.md` contains `Default OFF`
- [x] `git log` contains commit `9d39ac4` (Task 1)
- [x] `git log` contains commit `5296250` (Task 2)
- [x] `go build ./...` exits 0
- [x] `go vet ./...` exits 0
- [x] `go build -o /tmp/arb-test ./cmd` exits 0 (26MB binary built; `/tmp/arb-test` removed after verification)
- [x] `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/notify/...` 371 PASS, 0 FAIL
- [x] `grep -r "internal/api" internal/pricegaptrader/*.go` zero non-test matches (D-15 boundary)
- [x] no worktree branches dangling — `git worktree list` shows only main

## Self-Check: PASSED
