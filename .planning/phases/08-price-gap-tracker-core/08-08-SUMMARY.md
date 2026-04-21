---
phase: 08-price-gap-tracker-core
plan: 08
subsystem: pricegaptrader
tags: [pricegap, strategy4, cli, pg-admin, operator-tools, d-20, pg-risk-03, phase-8-close]

requires:
  - *database.Client.SetCandidateDisabled / ClearCandidateDisabled / IsCandidateDisabled (08-02)
  - *database.Client.GetActivePriceGapPositions / SavePriceGapPosition (08-02)
  - cfg.PriceGap* fields loaded from config.json (08-01)
  - models.PriceGapPosition shape (08-01/02)
provides:
  - cmd/pg-admin binary — 4 subcommands: status | enable <sym> | disable <sym> [reason] | positions list
  - Reversal path for PG-RISK-03 exec-quality auto-disable (pre-Phase-9 dashboard)
  - Operator workflow: restart-free disable/enable of candidates, Redis-only state surface
affects: [phase-9 — dashboard will supersede this CLI]

tech-stack:
  added: []
  patterns:
    - "Operator CLI as a thin wrapper over *database.Client methods — no new service layer, no HTTP, no config mutation"
    - "Stdout capture via os.Pipe swap for testing fmt.Printf-based CLIs without refactoring to io.Writer injection"
    - "text/tabwriter for human-readable tabular output where the row count is operator-scale (dozens, not thousands)"

key-files:
  created:
    - cmd/pg-admin/main.go
    - cmd/pg-admin/main_test.go
  modified:
    - CHANGELOG.md
    - VERSION

key-decisions:
  - "VERSION bumped 0.32.44 -> 0.33.0 (minor): the plan suggested 0.32.0 from stale memory, but actual disk state was 0.32.44. Plan Task 3 authorised 'bump to the next minor version from what's actually on disk' as the fallback — applied. Minor bump is appropriate because Phase 8 ships a new subsystem (pricegaptrader) + a new binary (pg-admin) behind a default-off flag."
  - "Used config.Load() + database.New(addr,pass,db) rather than the planner's sketched config.Load()->(cfg,err) + database.NewClient(cfg). The package API on disk is the former; the planner's sketch was a reasonable proxy but not literal. Rule 3 deviation (blocking — wrong signature) applied inline with no semantic change."
  - "Added a -h|--help|help alias to the main dispatch (exit 0) separate from the unknown-subcommand path (exit 2). The plan's acceptance criteria required usage on no-args with non-zero exit, which is preserved; the help alias is an ergonomic addition that does not affect any acceptance check."
  - "Stdout capture helper uses os.Pipe + goroutine-free io.ReadAll. The main.go fmt.Printf-based API was preserved verbatim; wrapping it in an io.Writer injection would have been a larger refactor with no functional benefit."
  - "cmdStatus iterates cfg.PriceGapCandidates (not Redis SCAN on pg:candidate:disabled:*) — the config is the canonical candidate list, and Redis flags are orthogonal state on that set. Avoids ghost-candidate output where a stale disabled flag lingers for a candidate that has since been removed from config."

patterns-established:
  - "Thin CLI-over-Redis pattern for operator tools: main.go Args dispatch, subcommand func per verb, tabwriter for list output, fatal helper on any DB error, fmt.Fprintln to stderr for usage. Reusable shape for future admin CLIs (spot-admin, engine-admin, etc.)."
  - "Pipe-based stdout capture for CLI subcommand tests — avoids invasive io.Writer rewiring and keeps the main.go code path identical between test and production."

decisions:
  - summary: "Phase 8 closes at v0.33.0 with the tracker subsystem default-off and pg-admin as the only operator surface. Phase 9 will add dashboard UI, runtime toggle, paper-mode, and Telegram alerts."
  - summary: "config.json remains untouched by pg-admin per CLAUDE.local.md rule. All tracker operator state lives in Redis DB 2 pg:* namespace."
  - summary: "Default-off invariant preserved: perp-perp and spot-futures engines are byte-for-byte unaffected when PriceGapEnabled=false."

metrics:
  duration: "~12 minutes"
  completed: "2026-04-21"

threat_model:
  - { id: T-08-31, category: EoP, disposition: accept, note: "pg-admin bypasses tracker safety by design — operator must be able to force-enable a candidate after auto-disable. Binary is accessible only to users with server shell access (same trust boundary as config.json edits)." }
  - { id: T-08-32, category: Tampering, disposition: mitigate, note: "Tracker reads pg:candidate:disabled:<sym> on every preEntry tick. Race with a concurrent pg-admin enable/disable is at worst one stale tick of mismatch (~polling interval). Redis SET/DEL are atomic, no torn writes possible." }
  - { id: T-08-33, category: InfoDisclosure, disposition: accept, note: "positions list exposes notional/status/timing — server-local CLI, same trust boundary as shell." }
---

# Phase 8 Plan 8: pg-admin Operator CLI Summary

## One-liner
Ships `cmd/pg-admin` — a Redis-only operator CLI (`status | enable | disable | positions list`) that provides the reversal path for PG-RISK-03 exec-quality auto-disable until Phase 9 dashboard ships. Closes Phase 8 at v0.33.0.

## What Shipped

### Task 1 — cmd/pg-admin/main.go (commit `19040b2`)
- 148-line Go binary; single `package main` file with `main()` dispatching on `os.Args[1]` to one of four subcommand handlers:
  - `status` — prints enabled flag, budget, max-concurrent, gate concentration pct, max-hold min, candidate count, active position count, then scans `cfg.PriceGapCandidates` and lists any with a `pg:candidate:disabled:<symbol>` flag.
  - `enable <symbol>` — calls `db.ClearCandidateDisabled(symbol)`; symbol is upper-cased before writing.
  - `disable <symbol> [reason...]` — calls `db.SetCandidateDisabled(symbol, reason)`; reason defaults to `"manual"`; extra args are space-joined into the reason string.
  - `positions list` — calls `db.GetActivePriceGapPositions()` and renders via `text/tabwriter` (columns: ID, SYMBOL, LONG, SHORT, NOTIONAL, ENTRY_BPS, STATUS, OPENED); prints `(no active positions)` when empty.
- Usage printed on `-h|--help|help` (exit 0) or unknown subcommand / missing args (exit 2).
- Fatal helper writes to stderr + exits 1 on any Redis error.
- `defer db.Close()` guarantees clean connection teardown.

### Task 2 — cmd/pg-admin/main_test.go (commit `d8a76fd`)
Four tests, miniredis-backed real `*database.Client`, stdout captured via `os.Pipe` swap:
1. **TestPgAdmin_EnableClearsFlag** — seed `pg:candidate:disabled:SOON="auto:exec_quality"`, call `cmdEnable(db,"SOON")`, assert `mr.Exists` returns false and stdout contains `"enabled"`.
2. **TestPgAdmin_DisableSetsFlag** — empty miniredis, call `cmdDisable(db,"DRIFT","manual test")`, assert `mr.Get("pg:candidate:disabled:DRIFT")` contains `"manual test"` and stdout contains both `"DRIFT"` and `"manual test"`.
3. **TestPgAdmin_PositionsList_Empty** — no active set, call `cmdPositionsList(db)`, assert stdout contains `"(no active positions)"` and no `pg_` row markers.
4. **TestPgAdmin_PositionsList_OnePosition** — seed one full `PriceGapPosition` via `SavePriceGapPosition`, call `cmdPositionsList(db)`, assert all key fields (ID, symbol, exchanges, `5000.00` notional) present and empty marker absent.

All 4 green under `-race -count=1` in 1.066s.

### Task 3 — VERSION + CHANGELOG (commit `28dce03`)
- `VERSION`: 0.32.44 → 0.33.0 (minor bump; planner's 0.32.0 was based on stale memory — plan authorised next-minor-from-disk as fallback).
- `CHANGELOG.md`: new top entry `## [0.33.0] - 2026-04-21` consolidating Phase 8 contents:
  - pricegaptrader subsystem (default OFF)
  - 11 new PriceGap* config fields
  - pg:* Redis namespace
  - 6 pre-entry risk gates
  - cmd/pg-admin CLI (D-20)
  - Circuit breaker on 5 consecutive PlaceOrder failures
  - Rehydration with orphan detection
  - Notes: default-off isolation, Phase 9 follow-ups (dashboard, runtime toggle, paper mode, Telegram).

## Verification

| Check | Result |
|-------|--------|
| `go build ./cmd/pg-admin/...` | PASS |
| `go vet ./cmd/pg-admin/...` | CLEAN |
| `go build ./...` (full repo) | PASS |
| `go test ./cmd/pg-admin/... -race -count=1` | PASS (4/4 tests) |
| Smoke: `pg-admin` (no args) | exit=2, usage printed to stderr |
| Smoke: `pg-admin --help` | exit=0, usage printed |
| `git diff` on `config.json` across Phase 8 Plan 8 commits | empty (config.json never touched) |
| 11 grep-based acceptance checks (Tasks 1+2+3) | 11/11 PASS |

Acceptance criteria walk:
- `grep -c "package main" cmd/pg-admin/main.go` = 1 ✓
- `grep -c 'case "status":'` = 1 ✓, `"enable":` = 1 ✓, `"disable":` = 1 ✓, `"positions":` = 1 ✓
- `grep -c "ClearCandidateDisabled" cmd/pg-admin/main.go` = 1 ✓
- `grep -c "SetCandidateDisabled" cmd/pg-admin/main.go` = 1 ✓
- `grep -c "GetActivePriceGapPositions" cmd/pg-admin/main.go` = 1 (+ 1 in cmdStatus) — met
- `grep -c "func TestPgAdmin_" cmd/pg-admin/main_test.go` = 4 ✓
- `grep -c "0.33.0" CHANGELOG.md` ≥ 1 ✓
- `grep -c "Phase 8" CHANGELOG.md` ≥ 1 ✓
- `grep -c "pricegaptrader" CHANGELOG.md` ≥ 1 ✓
- `grep -c "pg-admin" CHANGELOG.md` ≥ 1 ✓

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking signature mismatch] config/database constructor signatures differ from plan sketch**
- **Found during:** Task 1
- **Issue:** Plan Task 1 pseudo-code uses `cfg, err := config.Load()` and `db, err := database.NewClient(cfg)`. Actual package API is `cfg := config.Load()` (no error) and `db, err := database.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)`.
- **Fix:** Used the actual signatures. `cmd/balance/main.go` and `cmd/main.go` both use `database.New(addr,pass,db)` — same pattern.
- **Files modified:** cmd/pg-admin/main.go
- **Commit:** `19040b2`

**2. [Rule 3 — VERSION bump anchor] Planner's 0.32.0 vs actual 0.32.44 on disk**
- **Found during:** Task 3
- **Issue:** Plan Task 3 recommends 0.32.0 citing memory "current=0.31.2"; actual disk VERSION is 0.32.44 (Phase 8 Plans 1–7 already minted 0.32.37 → 0.32.44).
- **Fix:** Bumped to 0.33.0 — plan Task 3 pre-authorised "If VERSION differs, bump it to the next minor version from what's actually on disk."
- **Files modified:** VERSION
- **Commit:** `28dce03`

### Minor additions (outside deviation rules)

- `-h|--help|help` alias: returns exit 0 instead of exit 2. Unknown-subcommand and no-args paths still exit 2 per plan acceptance. Pure ergonomic addition; no behavior change to any test path.

## Authentication Gates

None — pg-admin operates on local Redis; no external auth required.

## Deferred Issues

None for this plan. Carried from Plan 07: `cmd/gatecheck/main.go:48` pre-existing `fmt.Println` vet warning (unrelated to Phase 8). Recorded in `.planning/phases/08-price-gap-tracker-core/deferred-items.md`.

## Known Stubs

None — all 4 subcommands are fully wired to live `*database.Client` methods.

## Phase 8 Closure

With Plan 08-08 committed, **Phase 8 is complete**: 8/8 plans shipped. VERSION stamped at **v0.33.0**. Tracker subsystem is code-complete, test-green, default-OFF, and byte-for-byte isolated from perp-perp + spot-futures. Operator reversal path (`pg-admin`) ships so PG-RISK-03 auto-disable is not a trap. Phase 9 carries: dashboard UI, runtime toggle, paper-mode, Telegram alerts, `GET /api/pricegap/positions` endpoint.

## Self-Check: PASSED

- FOUND: cmd/pg-admin/main.go
- FOUND: cmd/pg-admin/main_test.go
- FOUND: .planning/phases/08-price-gap-tracker-core/08-08-SUMMARY.md (this file)
- VERIFIED: VERSION == 0.33.0
- VERIFIED: CHANGELOG.md contains "[0.33.0]" header and "Phase 8" + "pricegaptrader" + "pg-admin" keywords
- FOUND commit 19040b2 (Task 1 — pg-admin main.go)
- FOUND commit d8a76fd (Task 2 — pg-admin tests)
- FOUND commit 28dce03 (Task 3 — VERSION + CHANGELOG)
- VERIFIED: config.json untouched across all 3 commits
