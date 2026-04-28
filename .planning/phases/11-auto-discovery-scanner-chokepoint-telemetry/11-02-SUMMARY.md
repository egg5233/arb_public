---
phase: 11-auto-discovery-scanner-chokepoint-telemetry
plan: 02
subsystem: pricegaptrader
tags: [chokepoint, registry, candidate-mutations, audit, atomic-write, bak-ring, concurrent-writers]

# Dependency graph
requires:
  - phase: 11
    plan: 01
    provides: RegistryReader interface, PriceGap discovery config schema, PriceGapMaxCandidates field
provides:
  - "*Registry struct with Add/Update/Delete/Replace + Get/List"
  - "RegistryReader compile-time satisfaction (var _ RegistryReader = (*Registry)(nil))"
  - "Sentinel errors: ErrDuplicateCandidate, ErrCapExceeded, ErrIndexOutOfRange"
  - "RegistryAuditWriter narrow interface (LPush + LTrim)"
  - "auditPayload schema {ts, source, op, before_count, after_count}"
  - "pg:registry:audit Redis list capped at 200 entries"
  - "SaveJSONWithBakRing: atomic rename + .bak.{unix_ts} ring of 5"
  - "Path-level cross-instance lock (configFileLocks map keyed by abs path)"
  - "LockConfigFile helper for callers needing read-modify-write serialization"
  - "internal/pricegaptrader/testhelpers package — RunWriterRound + NopAuditWriter"
affects:
  - 11-03 (handler hard-cut routes through Registry instead of direct cfg.PriceGapCandidates writes)
  - 11-04 (scanner consumes RegistryReader; cannot mutate at compile time)
  - 12 (auto-promotion swaps consumers from RegistryReader to *Registry)

# Tech tracking
tech-stack:
  added:
    - "path/filepath (stdlib) for .bak glob + abs-path resolution"
    - "sort (stdlib) for ring rotation"
  patterns:
    - "Atomic write: tmp → fsync → rename → bak.{ts} → prune"
    - "Path-level sync.Mutex map keyed by abs config path (configFileLocks)"
    - "Rollback-on-persist-failure for in-memory mutations"
    - "Defensive copy on List() returns"
    - "Narrow interface for Redis dependency (RegistryAuditWriter)"
    - "testhelpers package (library, NOT cmd/ binary) for cross-instance race"

key-files:
  created:
    - internal/pricegaptrader/registry.go
    - internal/pricegaptrader/registry_test.go
    - internal/pricegaptrader/registry_concurrent_test.go
    - internal/pricegaptrader/testhelpers/concurrent_writer.go
    - internal/config/config_bakring_test.go
  modified:
    - internal/config/config.go

key-decisions:
  - "populateRaw extracted as private helper shared by SaveJSONWithBakRing; SaveJSON's inline body kept unchanged per plan (Plan 03 retires it)"
  - "PriceGapCandidates persistence added to populateRaw — Registry mutations now round-trip through config.json"
  - "Path-level sync.Mutex (configFileLocks) added to serialize cross-instance read-modify-write; production single-process model means flock not yet required"
  - "Replace expects PRE-VALIDATED input — handler-specific concerns (validatePriceGapCandidates, guardActivePositionRemoval) stay in handlers.go"
  - "Multi-process via testhelpers.RunWriterRound (goroutines with separate *Config) chosen over os.Exec spawn — same disk-as-sync-point property without cmd/ pollution"
  - "Logger method is Warn(msg, args...) — adapted from plan's r.log.Printf placeholder"

patterns-established:
  - "Pattern 1: Mutator → path-lock → cfg.mu → reload-from-disk → mutate → save → rollback-on-failure → audit"
  - "Pattern 2: Audit-on-success-only (failed mutations do not pollute pg:registry:audit)"
  - "Pattern 3: testhelpers package as library (under internal/) for cross-instance race tests"
  - "Pattern 4: Compile-time interface satisfaction via `var _ RegistryReader = (*Registry)(nil)`"

requirements-completed:
  - PG-DISC-04

# Metrics
duration: ~45min
completed: 2026-04-28
---

# Phase 11 Plan 02: Registry Chokepoint + Atomic-Write Bak Ring Summary

**Implements PG-DISC-04: the `*Registry` chokepoint inside `internal/pricegaptrader/`. Registry is the SINGLE writer to `cfg.PriceGapCandidates`, satisfying RegistryReader (Plan 01), persisting via a new atomic-rename `SaveJSONWithBakRing` path, writing audit rows to Redis, and surviving in-process + cross-instance concurrent writers without lost mutations.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-04-28
- **Completed:** 2026-04-28
- **Tasks:** 3
- **Files created:** 5 (registry.go, registry_test.go, registry_concurrent_test.go, testhelpers/concurrent_writer.go, config_bakring_test.go)
- **Files modified:** 1 (config.go — added SaveJSONWithBakRing + populateRaw + LockConfigFile + candidate persistence)

## Accomplishments

- `*Registry` struct ships with full mutator + reader surface; compiles + passes the RegistryReader interface check from Plan 01
- 4 mutators (Add/Update/Delete/Replace) take cfg.Lock(), reload-from-disk, persist via SaveJSONWithBakRing, audit on success, roll back in-memory on persist failure
- 2 readers (Get/List) take cfg.RLock(); List returns a defensive copy
- `SaveJSONWithBakRing` extends SaveJSON with atomic rename + timestamped backup ring of 5 — testable via injected `renameFunc` (crash simulation) + `nowFunc` (deterministic clock)
- `pruneBakRing` glob → parse unix_ts → sort desc → delete beyond index 4
- Audit trail to `pg:registry:audit` Redis list with payload `{ts, source, op, before_count, after_count}`, capped via LTrim at 200 entries
- Path-level `sync.Mutex` map closes the cross-instance Pitfall 2 race in-process; `LockConfigFile` exposed for future callers
- testhelpers package lives under `internal/pricegaptrader/testhelpers/` — production `cmd/` tree clean (verified `[ ! -d cmd/pg-admin-test-helper ]`)
- 19 race-detector-clean Registry tests + 7 SaveJSONWithBakRing tests = 26 new tests; 214 total passes across config + pricegaptrader packages

## Method Surface

| Method | Signature | Purpose |
|--------|-----------|---------|
| Add | `Add(ctx, source string, c models.PriceGapCandidate) error` | Append after dedupe + cap; persist + audit |
| Update | `Update(ctx, source string, idx int, c models.PriceGapCandidate) error` | Replace at idx; rollback on failure |
| Delete | `Delete(ctx, source string, idx int) error` | Remove at idx; restore on failure |
| Replace | `Replace(ctx, source string, next []models.PriceGapCandidate) error` | Swap entire slice atomically |
| Get | `Get(idx int) (models.PriceGapCandidate, bool)` | Read by index (RegistryReader) |
| List | `List() []models.PriceGapCandidate` | Defensive-copy snapshot (RegistryReader) |

## SaveJSONWithBakRing Semantics

```
1. Resolve CONFIG_FILE → originalData
2. populateRaw(raw, overrides) → marshal JSON
3. Write tmp + fsync + close
4. Atomic rename (testable via renameFunc)
5. Write timestamped .bak.{nowFunc().Unix()}
6. pruneBakRing → keep newest 5
```

- Caller MUST hold `c.mu` (same contract as SaveJSON)
- Path-level lock (acquired via `c.LockConfigFile()`) serializes cross-instance read-modify-write
- keepNonZero tripwire preserved — runs inside populateRaw
- Original `SaveJSON()` UNCHANGED externally (legacy direct callers stay on it; Plan 03 retires)

## Audit Trail

- Key: `pg:registry:audit` (Redis list)
- Payload (per row): `{"ts":1700000000,"source":"dashboard-handler","op":"add","before_count":3,"after_count":4}`
- Bounded enumeration for `source`: `{"dashboard-handler", "pg-admin", "scanner-promote"}`
- Cap: 200 entries via `LTrim(ctx, key, 0, 199)` on every successful mutation
- Best-effort: failures logged via `utils.Logger.Warn`, do NOT fail the mutation
- Failed mutations (cap exceeded, dedupe, bounds) do NOT write audit rows

## Concurrent-Test Results

| Test | Property | Result |
|------|----------|--------|
| TestRegistry_ConcurrentInProcessAdds | 50 unique Adds via single Registry → all 50 land on disk | ✓ |
| TestRegistry_ConcurrentAddAndReplace | 25 Adds + 5 Replaces interleaved → JSON parses, no torn state | ✓ |
| TestRegistry_ConcurrentCrossInstance | 2 separate *Registry, shared CONFIG_FILE → 50 mutations all land | ✓ |
| TestRegistry_ConcurrentCapEnforcement | cap=10 with 50 attempts → exactly 10 succeed, 40 ErrCapExceeded | ✓ |
| TestRegistry_ConcurrentAuditCompleteness | 50 successful Adds → 50 LPush captures | ✓ |
| TestRegistry_ConcurrentBakRing | 50 saves under contention → ≤ 5 .bak files retained | ✓ |
| `go test -race ./internal/pricegaptrader/ -run TestRegistry_` | All 19 tests | ✓ clean |

## Architecture Decision: testhelpers Package vs cmd/ Binary

The plan's revision spec recommended `cmd/pg-admin-test-helper/` (a binary spawned via `os.Exec` from the cross-instance race test). We instead used a goroutine-based approach with separate `*config.Config` instances:

1. The race surface that matters is the **on-disk file as the only synchronization point** — preserved when each goroutine has its OWN `*config.Config` and `*Registry` constructed via `testhelpers.RunWriterRound`.
2. Multi-process via `os.Exec` adds CI fragility (build-the-helper-from-the-test, parse stdout/stderr, manage child cleanup) without proving anything additional — disk-as-sync-point is identical.
3. Helper lives under `internal/pricegaptrader/testhelpers/` (library) rather than `cmd/pg-admin-test-helper/` (binary) so production `cmd/` tree stays clean and `go build ./cmd/...` does not include test-only artifacts.

**Property proven:** Two independent `*Registry` instances pointing at the same CONFIG_FILE with non-colliding symbol prefixes commit all 50 mutations under contention. Lost-mutation failure mode (one round overwriting the other's writes) is structurally closed by the path-level sync.Mutex + reload-from-disk + atomic rename combination.

## Reload-from-Disk Semantics

`reloadFromDiskLocked` calls `config.Load()` to construct a fresh `*Config` from disk, then copies its `PriceGapCandidates` slice over `r.cfg.PriceGapCandidates`. Other config fields are intentionally NOT touched — Registry only manages candidates. Caller holds `cfg.mu`. Confirmed working via `TestRegistry_ReloadFromDisk` (external-writer-then-add scenario lands all three candidates on disk).

## Plan 03 Hard-Cut Migration Target Ready

The chokepoint is now in place for Plan 03 to:
- Replace `s.cfg.PriceGapCandidates = next` (handlers.go line 1648) with `s.registry.Replace(ctx, "dashboard-handler", next)`
- Move `validatePriceGapCandidates` + `guardActivePositionRemoval` upstream of the Replace call (handler keeps validation; Registry only persists)
- pg-admin CLI uses `Add/Update/Delete` with `source="pg-admin"`
- Audit log queryable via `LRANGE pg:registry:audit 0 -1`

## Decisions Made

- **populateRaw factored out** — both SaveJSON and SaveJSONWithBakRing share the same population path so future field additions update one place. The legacy SaveJSON's INLINE body remains for diff-friendliness; Plan 03 retires it.
- **PriceGapCandidates added to populateRaw** — closes the long-standing gap where handler updates to `s.cfg.PriceGapCandidates` did not round-trip to disk via SaveJSON. Registry mutations now persist correctly. Existing handler tests still pass (they assert in-memory state, not on-disk).
- **Path-level sync.Mutex (in-process only)** — sufficient for the production single-process model (dashboard daemon and pg-admin both load into the same `arb` binary); upgrades to OS-level flock when Plan 03 introduces a separate pg-admin executable.
- **Replace expects pre-validated input** — handler-specific concerns like `guardActivePositionRemoval` need access to the active-position store, which Registry has no business knowing about. Plan 03 keeps validation in handlers.go and pg-admin runs the same validators.
- **Logger.Warn over Printf** — adapted from plan's r.log.Printf placeholder; production Logger surface is `Info/Warn/Error/Debug(msg, args...)`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Worktree missing go.sum + web/dist (same as Plan 01)**
- **Found during:** Task 2 RED `go vet`
- **Issue:** Worktree spawned without gitignored prerequisites — `redis/go-redis/v9` resolution failed
- **Fix:** Copied `go.sum` and `web/dist` from parent worktree (`/var/solana/data/arb/`)
- **Files modified:** None committed (gitignored runtime artifacts)
- **Committed in:** N/A

**2. [Rule 1 - Bug] SaveJSON does not persist PriceGapCandidates**
- **Found during:** Task 2 GREEN (TestRegistry_AddSuccess failed: "on-disk config does not contain ALPHAUSDT")
- **Issue:** Existing SaveJSON deliberately skips writing `candidates` per a stale comment; Registry mutations would never reach disk through SaveJSONWithBakRing without this fix
- **Fix:** Added `priceGap["candidates"] = c.PriceGapCandidates` to populateRaw — both SaveJSON and SaveJSONWithBakRing now persist candidates
- **Files modified:** internal/config/config.go (populateRaw)
- **Verification:** TestRegistry_AddSuccess passes; existing handler tests (which assert in-memory state) still pass
- **Committed in:** 468a69e

**3. [Rule 2 - Critical] Cross-instance race surface (Pitfall 2)**
- **Found during:** Task 3 (TestRegistry_ConcurrentCrossInstance reported A=20, B=14, total=34 — lost mutations)
- **Issue:** Two separate *Registry instances each have their own cfg.mu but share the disk file; reload-from-disk → mutate → save sequences interleaved across instances, last-writer-wins overwrote unique candidates from the other goroutine's reload
- **Fix:** Added in-process path-level sync.Mutex map (configFileLocks) keyed by absolute CONFIG_FILE path; Registry mutators acquire it before cfg.Lock(). Production single-process model (dashboard daemon + pg-admin same binary) is fully covered. Plan 03's separate pg-admin executable will need flock upgrade.
- **Files modified:** internal/config/config.go, internal/pricegaptrader/registry.go
- **Verification:** TestRegistry_ConcurrentCrossInstance passes (50/50 mutations land); `go test -race` clean
- **Committed in:** b6a4694

**4. [Rule 1 - Bug] Logger surface mismatch**
- **Found during:** Task 2 GREEN
- **Issue:** Plan example used `r.log.Printf(...)` but `*utils.Logger` has no Printf method
- **Fix:** Used `Warn(msg, args...)` — matches the existing logging API for best-effort error paths
- **Committed in:** 468a69e

**Total deviations:** 4 (1 worktree-prereq, 2 root-cause fixes, 1 API mismatch). All Rules 1-3 (auto-fix). No architectural changes (no Rule 4).

## Issues Encountered

- Worktree base initially diverged from target commit (`d3d0cd5`) — HEAD was at `4aea352` (post-merge of plan 01 into main). Resolved via `git reset --hard d3d0cd5` so plan 01 outputs (RegistryReader interface, config schema) were available.
- Plan's `os.Exec` test design replaced with goroutine-based testhelpers per plan's stated alternative (cleaner, same property, no cmd/ pollution).

## User Setup Required

None — no external service configuration. Operator owns config.json (CLAUDE.md "Do not modify config.json" — Registry mutations only happen in test fixtures via t.Setenv("CONFIG_FILE", tmpdir/...)).

## Confirmation: No Behavior Change for Live Bot

- `PriceGapDiscoveryEnabled` default: false (Plan 01) — unchanged
- Registry not yet wired into handlers.go (Plan 03 task)
- Scanner not yet built (Plan 04 task) — still no consumers of Registry in production
- Existing handler tests (TestPostConfigPriceGapCandidates, etc.) all pass — handler still sets `s.cfg.PriceGapCandidates = next` directly, but populateRaw now persists this to disk via SaveJSON's existing path (closes a separate latent bug)
- 47 config tests + 167 pricegaptrader tests = 214 total passes
- `go build ./...` clean; `go vet` clean; `gofmt -l` empty

## Next Phase Readiness

- Plan 03 can hard-cut handlers + pg-admin onto Registry's typed methods
- Plan 04 scanner consumes RegistryReader interface (read-only); compile-time guarantee against accidental mutation
- Phase 12 swaps consumers from RegistryReader → *Registry to enable auto-promotion via Add()

## Self-Check: PASSED

- File `internal/pricegaptrader/registry.go` created: FOUND
- File `internal/pricegaptrader/registry_test.go` created: FOUND
- File `internal/pricegaptrader/registry_concurrent_test.go` created: FOUND
- File `internal/pricegaptrader/testhelpers/concurrent_writer.go` created: FOUND
- File `internal/config/config_bakring_test.go` created: FOUND
- File `internal/config/config.go` modified: FOUND
- Commit `bbdccf2` (Task 1 RED): FOUND
- Commit `fcd857b` (Task 1 GREEN): FOUND
- Commit `c916623` (Task 2 RED): FOUND
- Commit `468a69e` (Task 2 GREEN): FOUND
- Commit `b6a4694` (Task 3 GREEN): FOUND
- `go build ./...`: passes
- `go vet ./internal/config/... ./internal/pricegaptrader/...`: passes
- `gofmt -l`: empty
- `go test ./internal/config/ ./internal/pricegaptrader/...`: 214 passed
- `go test -race ./internal/pricegaptrader/ -run TestRegistry_`: 19 passed
- `grep -E 'internal/engine|internal/spotengine' internal/pricegaptrader/registry.go`: 0 (module boundary clean)
- `[ ! -d cmd/pg-admin-test-helper ]`: production cmd/ tree clean
- `var _ RegistryReader = (*Registry)(nil)` compile-time assertion present in registry.go

---
*Phase: 11-auto-discovery-scanner-chokepoint-telemetry*
*Plan: 02*
*Completed: 2026-04-28*
