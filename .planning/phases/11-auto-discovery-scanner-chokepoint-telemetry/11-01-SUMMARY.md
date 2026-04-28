---
phase: 11-auto-discovery-scanner-chokepoint-telemetry
plan: 01
subsystem: config
tags: [config-schema, pricegap, discovery, registry-reader, validation, regexp]

# Dependency graph
requires:
  - phase: 08-price-gap-tracker-core
    provides: PriceGap* config fields, jsonPriceGap nested struct, PriceGapCandidate type, models import path
  - phase: 09-price-gap-dashboard-paper-live-operations
    provides: keepNonZero tripwire pattern, SaveJSON .bak semantics
provides:
  - PriceGapDiscoveryEnabled config switch (default OFF)
  - PriceGapDiscoveryUniverse []string with ≤20 canonical-form validation
  - PriceGapDiscoveryDenylist []string with SYMBOL@exchange tuple format
  - PriceGapDiscoveryIntervalSec int (default 300, floor 60)
  - PriceGapDiscoveryThresholdBps int (default 100, floor 10)
  - PriceGapDiscoveryMinDepthUSDT int (default 1000, floor 0)
  - PriceGapAutoPromoteScore int (default 60, range [50,100])
  - PriceGapMaxCandidates int (default 12, range [1,50])
  - validatePriceGapDiscovery(c *Config) error
  - RegistryReader interface (Get + List only) in pricegaptrader package
affects:
  - 11-02 (Registry implementation will satisfy RegistryReader and add typed mutators)
  - 11-04 (Scanner consumes RegistryReader at compile time, cannot mutate)
  - 11-06 (Dashboard renders auto_promote_score + max_candidates from this schema)
  - 12-01 (Phase 12 swaps consumers to *Registry to enable auto-promotion)

# Tech tracking
tech-stack:
  added:
    - regexp (stdlib) for canonical-form universe + denylist validation
  patterns:
    - "Pointer-optional JSON fields: jsonPriceGap mirrors Config fields with *T to preserve defaults on omission"
    - "Default-then-override load helper: defaults applied before applyJSON overrides for safe-off semantics"
    - "Compile-time read-only contract via stub assertion (`var _ RegistryReader = (*stub)(nil)`)"
    - "Drift-defense grep harness in test file scopes to interface block via regex"

key-files:
  created:
    - internal/config/discovery_test.go
    - internal/pricegaptrader/registry_reader.go
    - internal/pricegaptrader/registry_reader_test.go
  modified:
    - internal/config/config.go

key-decisions:
  - "Reject lowercase universe entries instead of silently normalizing (tighter contract, mirrors Pitfall 1 §unsanitized symbol)"
  - "validatePriceGapDiscovery returns error but is NOT yet wired to fail load — Plan 02 wires the entry point; Plan 01 ships validator + tests"
  - "Default PriceGapAutoPromoteScore=60 to leave headroom above the server floor of 50 (RESEARCH §Security V5)"
  - "Defaults applied in both Load() (zero-value path) and applyJSON() (partial-block path) so any code path produces safe values"
  - "Module-boundary comment paraphrases trading-engine references to satisfy `grep -E 'internal/engine|internal/spotengine'` returning 0"

patterns-established:
  - "Pattern 1: 8 new schema fields mirror jsonPriceGap pointer-optional + default-then-override apply"
  - "Pattern 2: validate{Block} returning error for explicit operator feedback; greppable via `^func validate`"
  - "Pattern 3: Read-only chokepoint via `*Reader` interface stub assertion in package tests"
  - "Pattern 4: Drift-defense grep harness in tests for static interface contracts"

requirements-completed:
  - PG-DISC-01
  - PG-DISC-04

# Metrics
duration: ~25min
completed: 2026-04-28
---

# Phase 11 Plan 01: Auto-Discovery Config Schema + RegistryReader Interface Summary

**Adds the v2.2 Phase 11 config schema (8 new PriceGap discovery fields with validation) and the read-only RegistryReader interface inside `internal/pricegaptrader/`, the additive foundation Plans 02-05 depend on.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-04-28T (worktree spawn)
- **Completed:** 2026-04-28T (post-rebase + 4 commits)
- **Tasks:** 2
- **Files modified:** 4 (3 created, 1 modified)

## Accomplishments

- 8 new `PriceGap*` config fields ship with validation, defaults, JSON load/save round-trip, and 13 unit sub-tests
- `validatePriceGapDiscovery(c *Config) error` enforces D-05 (≤20 universe), D-07 (denylist tuples), and Security V5 score floor 50
- `RegistryReader` interface exposes Get + List only — no mutator methods at compile time
- 5 RegistryReader unit tests (compile-time stub assertion + drift-defense grep harness + module-boundary check)
- Default-OFF preserved: `PriceGapDiscoveryEnabled=false` from default-loaded config
- All 40 existing config tests + 188 combined config/pricegaptrader tests still pass — no regressions

## Task Commits

Each task ran TDD (RED → GREEN):

1. **Task 1 RED: PriceGap discovery schema test** — `dbc806f` (test)
2. **Task 1 GREEN: 8 fields + validator + applyJSON + SaveJSON** — `fb152bc` (feat)
3. **Task 2 RED: RegistryReader interface test** — `bfca670` (test)
4. **Task 2 GREEN: RegistryReader interface** — `eb82b4e` (feat)

_Note: Both tasks REFACTOR phase was a no-op — no cleanup needed beyond GREEN._

## Files Created/Modified

- `internal/config/config.go` — added 8 fields to Config struct, 8 fields to jsonPriceGap, defaults block in Load(), apply block in applyJSON(), serialization in SaveJSON, validatePriceGapDiscovery validator + 2 compiled regexes
- `internal/config/discovery_test.go` — 12 sub-tests inside TestPriceGapDiscoverySchema covering defaults, floors, format rules, JSON round-trip, keepNonZero tripwire intact
- `internal/pricegaptrader/registry_reader.go` — RegistryReader interface (Get + List), package docstring with module-boundary contract, no implementation
- `internal/pricegaptrader/registry_reader_test.go` — stub implementer + 5 tests including drift-defense grep harness

## Decisions Made

- **Reject lowercase universe** instead of silent normalize — tighter contract, plan explicitly recommended this
- **Score default 60** instead of floor 50 — leaves operator headroom; floor still enforced
- **Validator NOT wired to fail load yet** — Plan 02 owns the load-failure wiring once the registry exists. Plan 01 ships the validator + tests so the contract is testable from day one
- **Module-boundary comment rephrased** — original draft mentioned `internal/engine` / `internal/spotengine` literally in the docstring, which broke the acceptance grep `grep -E 'internal/engine|internal/spotengine'`. Comment now paraphrases the contract to satisfy the grep while preserving intent

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Missing go.sum + web/dist in worktree**
- **Found during:** Task 2 (running `go test ./internal/pricegaptrader/...`)
- **Issue:** Worktree was created without go.sum (gitignored) and web/dist (build artifact); test build failed on `redis/go-redis/v9` resolution and `web/embed.go` failed on missing dist
- **Fix:** Copied go.sum and web/dist from parent worktree (`/var/solana/data/arb/`); both are gitignored so no commit pollution
- **Files modified:** None committed (gitignored prerequisites)
- **Verification:** `go build ./...` and `go vet ./...` pass clean
- **Committed in:** N/A (gitignored runtime artifacts)

**2. [Rule 3 - Blocking] Initial module-boundary test failed on docstring text**
- **Found during:** Task 2 GREEN
- **Issue:** TestRegistryReader_ModuleBoundary regex matched `internal/spotengine` inside the package docstring comment, even though no actual import existed
- **Fix:** Rephrased the comment to reference "live trading engines" without the literal package path; intent preserved, acceptance grep returns 0
- **Files modified:** internal/pricegaptrader/registry_reader.go
- **Verification:** `grep -E 'internal/engine|internal/spotengine' internal/pricegaptrader/registry_reader.go` returns 0; test passes
- **Committed in:** eb82b4e (Task 2 GREEN commit)

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking)
**Impact on plan:** Both deviations fixed without altering plan intent. Module-boundary deviation was self-imposed by literal acceptance-grep wording; fix made the docstring satisfy the grep without losing the contract intent.

## Issues Encountered

- Worktree base diverged from target commit at start (HEAD had 2 commits ahead, missing 5 commits from target). Resolved via `git rebase --onto 02c31f15 HEAD~2 HEAD`.
- pre-existing `web/embed.go` requires `web/dist/` to exist — solved by copying from parent worktree (gitignored).

## User Setup Required

None — no external service configuration required. Operator owns config.json (CLAUDE.md "Do not modify config.json" — intentional read-only constraint for Plan 01).

## Confirmation: No Behavior Change for Live Bot

- `PriceGapDiscoveryEnabled` default: `false` (verified by `TestPriceGapDiscoverySchema/Default_DiscoveryEnabledFalse`)
- No scanner exists yet (Plan 04 task)
- No registry implementer exists yet (Plan 02 task)
- No mutator methods on RegistryReader (verified by drift-defense grep test)
- `keepNonZero` tripwire intact (verified by `TestPriceGapDiscoverySchema/KeepNonZeroTripwireIntact`)
- 40 existing config tests + all pricegaptrader tests still pass

## Next Phase Readiness

- Plan 02 can implement `*Registry` satisfying `RegistryReader` and add typed mutators (Add/Update/Delete/Replace)
- Plan 04 scanner can accept `RegistryReader` as a constructor argument and gain compile-time read-only access
- Plan 06 dashboard can render `auto_promote_score` and `max_candidates` from this schema
- Phase 12 can swap scanner consumers from `RegistryReader` to `*Registry` for auto-promotion

## Self-Check: PASSED

- File `internal/config/config.go` modified: FOUND
- File `internal/config/discovery_test.go` created: FOUND
- File `internal/pricegaptrader/registry_reader.go` created: FOUND
- File `internal/pricegaptrader/registry_reader_test.go` created: FOUND
- Commit `dbc806f` (Task 1 RED): FOUND
- Commit `fb152bc` (Task 1 GREEN): FOUND
- Commit `bfca670` (Task 2 RED): FOUND
- Commit `eb82b4e` (Task 2 GREEN): FOUND
- `go build ./...`: passes
- `go vet ./...`: passes
- `gofmt -l`: empty
- `go test ./internal/config/... ./internal/pricegaptrader/`: 188 passed
- Acceptance grep `internal/engine|internal/spotengine` in registry_reader.go: 0 (clean)
- Acceptance grep `Add(|Update(|Delete(|Replace(` in registry_reader.go: 0 (clean)

---
*Phase: 11-auto-discovery-scanner-chokepoint-telemetry*
*Completed: 2026-04-28*
