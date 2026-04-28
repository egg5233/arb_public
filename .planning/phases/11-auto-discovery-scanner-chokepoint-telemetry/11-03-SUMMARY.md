---
phase: 11-auto-discovery-scanner-chokepoint-telemetry
plan: 03
subsystem: api+pricegaptrader+pg-admin
tags: [chokepoint-migration, hard-cut, registry, validation-extraction, pg-admin, audit-source, t-11-18, t-11-19, t-11-24]

# Dependency graph
requires:
  - phase: 11
    plan: 01
    provides: PriceGap discovery config schema, RegistryReader interface
  - phase: 11
    plan: 02
    provides: "*Registry chokepoint with Add/Update/Delete/Replace, RegistryAuditWriter interface, sentinel errors, SaveJSONWithBakRing, configFileLocks"
provides:
  - "CandidateRegistry interface in internal/api (Replace/Add/Update/Delete/Get/List)"
  - "Server.registry field + SetRegistry setter"
  - "POST /api/config candidate path migrated through s.registry.Replace(ctx, \"dashboard-handler\", next)"
  - "internal/database.PriceGapAuditWriter — *redis.Client adapter satisfying pricegaptrader.RegistryAuditWriter"
  - "pricegaptrader.ValidateCandidates — shared validator extracted from handler-side helper"
  - "cmd/pg-admin candidate subcommands: list / add / delete / replace (all via Registry, source=\"pg-admin\")"
  - "Bootstrap in cmd/main.go constructs ONE *Registry instance"
affects:
  - 11-04 (scanner takes RegistryReader from same Registry instance)
  - 11-05 (Phase 12 promotion swaps consumers from RegistryReader → *Registry)
  - 11-06 (dashboard renders auto_promote_score / max_candidates)

# Tech tracking
tech-stack:
  added:
    - "flag (stdlib) — pg-admin candidate subcommand parsing"
    - "io.Writer-based stdout/stderr injection for testable cmd handlers"
  patterns:
    - "Hard-cut chokepoint migration: capture validated slice locally → release lock → call Registry.Replace AFTER s.cfg.Unlock()"
    - "Defer Registry call across the in-handler lock boundary (T-11-24 deadlock avoidance)"
    - "Static-grep guard test: regexp scan of source files prevents drift back to direct mutation"
    - "Validator extraction with thin alias for backward compat — handlers + pg-admin share one gatekeeper"
    - "Run(args, deps) refactor for testable CLI binaries — main() becomes 5-line wrapper"

key-files:
  created:
    - internal/api/handlers_registry_migration_test.go
    - internal/database/pricegap_audit.go
    - internal/pricegaptrader/validate.go
    - internal/pricegaptrader/validate_test.go
    - cmd/pg-admin/main_registry_test.go
  modified:
    - internal/api/server.go
    - internal/api/handlers.go
    - internal/api/pricegap_handlers.go
    - internal/api/pricegap_handlers_test.go
    - cmd/main.go
    - cmd/pg-admin/main.go
    - cmd/pg-admin/main_test.go

key-decisions:
  - "Defer Registry.Replace until AFTER s.cfg.Unlock() — handler keeps validation under cfg.Lock(); Registry.Replace internally takes its own cfg.Lock() + LockConfigFile() so handler must release first to avoid the T-11-24 deadlock"
  - "Wire Registry via SetRegistry setter (not constructor) — minimises NewServer signature churn and matches existing setter-injection style (SetCloseHandler, SetCapitalAllocator, etc.)"
  - "Add CandidateRegistry interface inside internal/api/server.go (not imported from pricegaptrader) — keeps the test fakeCandidateRegistry at api-package scope without an extra import-cycle"
  - "Build PriceGapAuditWriter inside internal/database (not pricegaptrader) — pricegaptrader has a NARROW RegistryAuditWriter interface and the bridge to *redis.Client lives where the redis driver already imports — keeps Registry's import set lean"
  - "Validator extraction kept handlers.go thin alias — existing tests in pricegap_handlers_test.go and pricegap_candidates_validate_test.go that call validatePriceGapCandidates directly still pass without modification"
  - "Active-position guard stays in handlers.go (NOT pulled into pricegaptrader) — guard reads s.db.GetActivePriceGapPositions which is a server-side concern; pg-admin operator is trusted (machine-local) so pg-admin replace skips the guard intentionally"
  - "pg-admin Run(args, deps) refactor — main() now a 5-line wrapper around Run; Dependencies struct captures Registry + DB + Cfg + Stdout + Stderr so tests inject fakes without forking a process"

patterns-established:
  - "Pattern 1: Defer Registry call across cfg.Lock boundary (capture local → unlock → call → re-RLock for snapshot)"
  - "Pattern 2: Static-source regex test guards against future drift to direct mutation (T-11-18 mitigation)"
  - "Pattern 3: io.Writer-based cmd handlers for in-process CLI testability"
  - "Pattern 4: Sentinel-error-to-exit-code map (3=duplicate, 4=cap, 5=idx, 2=validation/usage)"

requirements-completed:
  - PG-DISC-04

# Metrics
duration: ~50min
completed: 2026-04-28
---

# Phase 11 Plan 03: Hard-Cut Migration to Registry Chokepoint Summary

**Closes Pitfall 2 (three-writer race) before Phase 12 ships. After this plan, ZERO direct mutations of `cfg.PriceGapCandidates` remain anywhere outside `internal/pricegaptrader/registry.go` and `internal/config/config.go`'s single-threaded boot loader. Both writers (dashboard POST `/api/config` and pg-admin CLI) route through the `*Registry` chokepoint with bounded `source` strings (`"dashboard-handler"` vs `"pg-admin"`) for the audit trail.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-04-28
- **Completed:** 2026-04-28
- **Tasks:** 2
- **Files created:** 5 (1 in api, 1 in database, 2 in pricegaptrader, 1 in cmd/pg-admin)
- **Files modified:** 7 (server.go, handlers.go, pricegap_handlers.go, cmd/main.go, cmd/pg-admin/main.go, cmd/pg-admin/main_test.go, pricegap_handlers_test.go)

## Accomplishments

### Task 1 — POST /api/config migration

- `Server.registry CandidateRegistry` field added with `SetRegistry()` setter
- `CandidateRegistry` interface defined in api package (Replace/Add/Update/Delete + Get/List from RegistryReader)
- POST /api/config candidate path: `s.cfg.PriceGapCandidates = next` swapped for deferred `s.registry.Replace(r.Context(), "dashboard-handler", pendingCandidates)` after `s.cfg.Unlock()` (T-11-24 deadlock avoidance)
- Validation + active-position guard PRESERVED in their original positions under `s.cfg.Lock()` — Registry expects pre-validated input per Plan 02 contract
- 5 migration sub-tests proving: routes-through-Registry, validation-not-called-on-fail, guard-not-called-on-conflict, registry-error-returns-500, static-no-direct-mutations regex guard
- `internal/database.PriceGapAuditWriter` adapter wraps `db.Redis()` to satisfy `pricegaptrader.RegistryAuditWriter` (LPush + LTrim)
- `cmd/main.go` constructs ONE `*pricegaptrader.Registry` and injects via `apiSrv.SetRegistry(pgRegistry)` — same instance flows to Phase 12 scanner
- Existing `newPriceGapTestServer` fixture wires a real Registry against the sandboxed CONFIG_FILE so all 12 existing TestPostConfigPriceGapCandidates round-trip tests pass against the production path

### Task 2 — pg-admin candidate subcommands + shared validator

- `internal/pricegaptrader/validate.go` extracts `ValidateCandidates([]models.PriceGapCandidate) []string` — D-05..D-11 invariants (symbol regex, exch enum, threshold/maxPosition/slippage ranges, direction enum, duplicate tuple detection)
- 11 unit tests in `validate_test.go` cover every rule independently and the collation property
- `internal/api/pricegap_handlers.go` keeps `validatePriceGapCandidates` as a thin alias for backward compatibility — existing direction-field tests in `pricegap_handlers_test.go` (lines 700-768) untouched
- `internal/api/handlers.go` POST `/api/config` calls `pricegaptrader.ValidateCandidates(next)` directly (acceptance grep ≥1)
- `cmd/pg-admin/main.go` refactored: main() → 5-line wrapper around `Run(args []string, deps Dependencies) int` for testability
- 4 new candidate subcommands routed through Registry with source="pg-admin":
  - `pg-admin candidates list` — JSON dump via Registry.List
  - `pg-admin candidates add --symbol .. --long .. --short .. [--threshold-bps N] [--max-position-usdt N] [--modeled-slippage-bps N] [--direction pinned|bidirectional]`
  - `pg-admin candidates delete --idx N`
  - `pg-admin candidates replace --file <path>`
- Exit-code map: 0 success | 2 validation/usage | 3 duplicate | 4 cap exceeded | 5 idx out of range | 1 generic
- 11 new pg-admin tests cover every subcommand + every sentinel-error path with a fakeRegistry double
- 4 existing pg-admin tests updated to the io.Writer-based handler signatures

## Static Acceptance Grep Results

| Check | Expected | Actual |
|-------|----------|--------|
| `s?\.cfg\.PriceGapCandidates\s*=` in `internal/api/handlers.go` + `internal/api/pricegap_handlers.go` (excluding comments) | 0 | 0 (only a comment that the migration test regex correctly skips) |
| `s\.registry\.Replace` in `internal/api/handlers.go` | ≥1 | 2 (mutator + log line) |
| `pricegaptrader\.NewRegistry` in `cmd/main.go` | 1 | 1 |
| `pricegaptrader\.NewRegistry` in `cmd/pg-admin/main.go` | 1 | 1 |
| `pricegaptrader\.ValidateCandidates` in `internal/api/handlers.go` | ≥1 | 1 |
| `pricegaptrader\.ValidateCandidates` in `cmd/pg-admin/main.go` | ≥1 | 3 |
| `Server.registry` interface field | present | `registry CandidateRegistry` |

## Audit Source Values

| Writer | source | Routes through |
|--------|--------|----------------|
| Dashboard POST /api/config | `"dashboard-handler"` | `s.registry.Replace(...)` |
| pg-admin candidates add | `"pg-admin"` | `Registry.Add(...)` |
| pg-admin candidates delete | `"pg-admin"` | `Registry.Delete(...)` |
| pg-admin candidates replace --file | `"pg-admin"` | `Registry.Replace(...)` |
| (Phase 12 future) | `"scanner-promote"` | `Registry.Add(...)` |

## Method Surface Mapping

| Handler / CLI | Old call site | New call site |
|---------------|---------------|---------------|
| `handlers.go` POST `/api/config` | `s.cfg.PriceGapCandidates = next` | `s.registry.Replace(ctx, "dashboard-handler", next)` |
| `handlers.go` validation | `validatePriceGapCandidates(next)` (local) | `pricegaptrader.ValidateCandidates(next)` (shared) |
| `pg-admin candidates add` | (did not exist) | `reg.Add(ctx, "pg-admin", c)` |
| `pg-admin candidates delete` | (did not exist) | `reg.Delete(ctx, "pg-admin", idx)` |
| `pg-admin candidates replace` | (did not exist) | `reg.Replace(ctx, "pg-admin", next)` |
| `pg-admin candidates list` | (did not exist) | `reg.List()` |

## Lock Ordering Verification (T-11-24)

The migration deliberately defers `Registry.Replace` until AFTER `s.cfg.Unlock()`. Sequence:

```
1. handler: s.cfg.Lock()
2. handler: validate + normalise + guardActivePositionRemoval (under s.cfg.Lock)
3. handler: capture validated `next` to a local variable (no mutation yet)
4. handler: persist OTHER config fields via SaveJSONWithExchangeSecretOverrides (under s.cfg.Lock)
5. handler: s.cfg.Unlock()
6. handler: s.registry.Replace(...)
   ↓
   Registry: cfg.LockConfigFile() (path-level lock)
   Registry: cfg.Lock() (per-instance lock)
   Registry: reload-from-disk (sees the just-saved other fields)
   Registry: in-memory mutation
   Registry: SaveJSONWithBakRing
   Registry: cfg.Unlock() / LockConfigFile release
   Registry: writeAuditLocked (best-effort LPush + LTrim)
7. handler: re-RLock to refresh snapshot for response envelope
```

No deadlock possible — Registry never re-enters handler lock.

## pg-admin In-Process Architecture

Per Plan 11-03 design + 11-RESEARCH.md:
- pg-admin is invoked as a separate OS process from the bot daemon (already the case — `cmd/pg-admin/main.go` is its own binary).
- Plan 02's `configFileLocks` provides in-process serialisation only. Cross-process coordination relies on: (a) Registry's reload-from-disk before every mutation; (b) atomic os.Rename in `SaveJSONWithBakRing`; (c) the daemon's reload-on-next-mutation pattern.
- Plan 02 Task 3's `TestRegistry_ConcurrentCrossInstance` already exercises this exact property (two `*Registry` instances against the same CONFIG_FILE — same shape as bot + pg-admin running concurrently). No additional integration test is needed in Plan 03.

## Decisions Made

- **Lock-deferred Replace** — alternative was making Registry.Replace re-entrant (lock-aware), rejected because it leaks lock ownership across module boundaries. Defer-after-Unlock keeps both the handler's atomic-multi-field pattern AND Registry's self-contained locking intact.
- **CandidateRegistry interface in `api` package** — alternative was importing the interface from `pricegaptrader`, rejected because it would force the api-package fake (`fakeCandidateRegistry`) into pricegaptrader's testhelpers OR duplicate the fake into pricegaptrader_test.go. Local interface keeps the fake at the same package scope as the tests that use it.
- **Active-position guard stays in handlers.go** — pg-admin replace from JSON file is operator-triggered and machine-local; if the operator is removing a candidate with an active position they're presumed to know what they're doing. Registry's reload-from-disk + atomic-rename guarantees consistency. The guard runs only on dashboard requests where the auth surface is broader (any operator with the bearer token).
- **Validator extraction with thin alias** — alternative was deleting the local `validatePriceGapCandidates` and rewriting all 7 existing test sites that call it directly, rejected because it would inflate the diff with churn unrelated to the chokepoint migration. The alias is one line of trivial delegation.
- **PriceGapAuditWriter in `internal/database` (not pricegaptrader)** — alternative was inlining the `redis.Client` import into pricegaptrader, rejected because pricegaptrader's narrow RegistryAuditWriter interface is the whole point of decoupling Registry from the redis driver. The adapter is a 25-line file in the package that already imports redis.
- **`Run(args, deps) int` refactor** — alternative was os.Exec spawning + stdout-parsing test pattern, rejected per 11-RESEARCH.md §"pg-admin in-process vs RPC" recommendation. Run() takes io.Writers + a CandidateRegistry interface so tests inject fakes directly without forking processes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Worktree missing go.sum + web/dist**
- **Found during:** Task 1 GREEN compile phase
- **Issue:** Worktree spawned without gitignored prerequisites; `go vet` failed on `redis/go-redis/v9` resolution and `web/embed.go` failed on missing `dist/`
- **Fix:** Copied `go.sum` and `web/dist` from parent worktree (`/var/solana/data/arb/`)
- **Files modified:** None committed (gitignored)
- **Verification:** `go build ./...` clean
- **Commit:** N/A (gitignored runtime artifacts; same pattern as Plan 01/02)

**2. [Rule 1 - Bug] handlers.go RegistryError test fixture initially used wrong DB API**
- **Found during:** Task 1 GREEN compile phase
- **Issue:** Migration test used `s.db.SaveActivePriceGapPosition` which doesn't exist; correct method is `SavePriceGapPosition` (which inserts into pg:positions:active automatically when status="open")
- **Fix:** Renamed to `SavePriceGapPosition(pos *models.PriceGapPosition)` in test setup
- **Files modified:** `internal/api/handlers_registry_migration_test.go`
- **Verification:** TestPostConfigCandidates_ActivePositionGuard_RegistryNotCalled passes
- **Commit:** Folded into 4d7be2e (Task 1 GREEN)

**3. [Rule 3 - Blocking] Existing test fixture broke after registry-required wiring**
- **Found during:** Task 1 GREEN running existing TestPostConfigPriceGapCandidates
- **Issue:** Production handler now requires `s.registry != nil` — existing tests using the unmodified `newPriceGapTestServer` fixture started returning 500 "registry not wired"
- **Fix:** Updated `newPriceGapTestServer` to construct a real `*pricegaptrader.Registry` against the sandboxed CONFIG_FILE using `db.PriceGapAudit()` as the audit writer
- **Files modified:** `internal/api/pricegap_handlers_test.go` + new `internal/database/pricegap_audit.go` adapter
- **Verification:** All 12 existing PriceGap candidate tests pass + 5 new migration tests pass = 34 total in `TestPostConfigCandidates*|TestPostConfigPriceGapCandidates|TestPriceGap|TestHandleConfig`
- **Commit:** Folded into 4d7be2e (Task 1 GREEN)

**4. [Rule 3 - Blocking] Existing pg-admin tests broke after io.Writer refactor**
- **Found during:** Task 2 GREEN compile phase
- **Issue:** Existing `cmd/pg-admin/main_test.go` called `cmdEnable(db, sym)`, `cmdDisable(db, sym, reason)`, `cmdPositionsList(db)` — all changed to `(db, sym, stdout, stderr)` to match the testable refactor
- **Fix:** Rewrote test helpers to use bytes.Buffer pairs instead of `os.Pipe()` stdout redirection; updated all 4 existing tests to assert on the buffer content + exit code
- **Files modified:** `cmd/pg-admin/main_test.go`
- **Verification:** 4 existing tests + 11 new candidate tests = 15 passing in `TestPgAdmin_*`
- **Commit:** Folded into 7c439e7 (Task 2 GREEN)

**Total deviations:** 4 (1 worktree-prereq, 3 test-fixture-update). All Rules 1/3 (auto-fix). No architectural changes (no Rule 4).

## Issues Encountered

- The migration grep `s?\.cfg\.PriceGapCandidates\s*=` initially matched a comment line in handlers.go that referenced the old code shape; the static-check test regex deliberately anchors at `^\s*ident\.` (start-of-line, no leading `//`) so it correctly ignores the comment. Verified by running `TestPostConfigCandidates_NoDirectMutationsInPackage` — clean pass.
- `s.cfg.PriceGapCandidates =` ALSO appears in `internal/config/config.go` (the JSON loader on the boot path, line 1462) — explicitly out of scope per the plan (single-threaded boot, not a chokepoint concern).

## User Setup Required

None — no external service configuration. pg-admin remains a CLI tool the operator runs locally.

## Confirmation: No Behavior Change for Live Bot

- `PriceGapDiscoveryEnabled` default: false (Plan 01) — unchanged
- POST /api/config request DTO unchanged — clients see no API surface change
- POST /api/config response shape unchanged — `{"ok": true, "data": <configResponse>}` same as before
- Validation error messages identical — same regex / range / format strings, just sourced from the shared `pricegaptrader.ValidateCandidates`
- Active-position guard 409 message identical
- Auth/CORS middleware unchanged
- Existing scanner / engine / spotengine code untouched
- 966 tests pass across 41 packages (full repo `go test ./... -count=1 -short`); 264 race-clean tests on the affected packages
- Plan 12 (auto-promotion) can now hard-cut from `RegistryReader` to `*Registry` knowing no alternative writer paths exist

## Next Phase Readiness

- Plan 11-04 scanner takes a `RegistryReader` view of the same `pgRegistry` instance constructed in `cmd/main.go` — wire via a new `apiSrv.GetRegistry()` helper or direct injection from main()
- Phase 12 promotion swaps the scanner's `RegistryReader` consumer to `*Registry` and calls `Add(ctx, "scanner-promote", c)` — audit trail captures the third writer with the bounded source enum
- Operator runbook gains 4 new pg-admin verbs that the on-call team can use without touching `config.json` directly (CLAUDE.local.md compliance)

## Self-Check: PASSED

- File `internal/api/handlers_registry_migration_test.go` created: FOUND
- File `internal/database/pricegap_audit.go` created: FOUND
- File `internal/pricegaptrader/validate.go` created: FOUND
- File `internal/pricegaptrader/validate_test.go` created: FOUND
- File `cmd/pg-admin/main_registry_test.go` created: FOUND
- File `internal/api/server.go` modified: FOUND (CandidateRegistry interface + Server.registry field + SetRegistry setter)
- File `internal/api/handlers.go` modified: FOUND (s.registry.Replace at POST /api/config; pricegaptrader.ValidateCandidates call)
- File `internal/api/pricegap_handlers.go` modified: FOUND (validatePriceGapCandidates → thin alias)
- File `cmd/main.go` modified: FOUND (pricegaptrader.NewRegistry + SetRegistry)
- File `cmd/pg-admin/main.go` modified: FOUND (Run/Dependencies refactor + 4 candidates subcommands)
- File `cmd/pg-admin/main_test.go` modified: FOUND (io.Writer signatures)
- File `internal/api/pricegap_handlers_test.go` modified: FOUND (newPriceGapTestServer wires real Registry)
- Commit `f42f551` (Task 1 RED): FOUND
- Commit `4d7be2e` (Task 1 GREEN): FOUND
- Commit `725510d` (Task 2 RED): FOUND
- Commit `7c439e7` (Task 2 GREEN): FOUND
- `go build ./...`: passes
- `go vet ./...`: passes
- `gofmt -l` (all touched files): empty
- `go test ./internal/api/ ./internal/pricegaptrader/ ./internal/config/ ./cmd/pg-admin/... ./cmd/...`: 313 passed
- `go test -race` (api + pricegaptrader + pg-admin): 264 passed
- `go test ./... -count=1 -short`: 966 passed across 41 packages
- Acceptance grep `s?\.cfg\.PriceGapCandidates\s*=` in `internal/api/` (excluding `_test.go`): 0 code mutations (only 1 comment match, ignored by the static test regex)
- Acceptance grep `s\.registry\.Replace` in `internal/api/handlers.go`: ≥1 (2 actual)
- Acceptance grep `pricegaptrader\.NewRegistry` in `cmd/main.go`: 1
- Acceptance grep `pricegaptrader\.NewRegistry` in `cmd/pg-admin/main.go`: 1
- Acceptance grep `pricegaptrader\.ValidateCandidates` in `internal/api/handlers.go`: ≥1 (1 actual)
- Acceptance grep `pricegaptrader\.ValidateCandidates` in `cmd/pg-admin/main.go`: ≥1 (3 actual)
- Acceptance grep `registry\s*\*pricegaptrader\.Registry\|registry\s*CandidateRegistry` in `internal/api/server.go`: ≥1 (1 actual — `registry CandidateRegistry`)
- `config.json` at repo root: NOT touched (CLAUDE.md compliance)

---
*Phase: 11-auto-discovery-scanner-chokepoint-telemetry*
*Plan: 03*
*Completed: 2026-04-28*
