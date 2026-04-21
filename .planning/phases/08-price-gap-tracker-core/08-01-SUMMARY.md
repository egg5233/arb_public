---
phase: 08-price-gap-tracker-core
plan: 01
subsystem: config
tags: [pricegap, strategy4, config, models, dependency-injection]

requires: []
provides:
  - PriceGapPosition struct + ExitReason enum (D-13)
  - PriceGapStore DI interface (D-02 boundary)
  - PriceGapCandidate tuple with ID() helper (PG-05, D-08)
  - SlippageSample struct for rolling window (PG-RISK-03)
  - 11 PriceGap* Config fields with applyJSON mapping (D-22, PG-OPS-06)
affects: [08-02, 08-03, 08-04, 08-05, 08-06, 08-07, 08-08, phase-09]

tech-stack:
  added: []
  patterns:
    - "Interface-based DI mirroring StateStore (internal/models/interfaces.go)"
    - "Pointer-optional applyJSON pattern mirroring jsonSpotFutures"
    - "Safe-off defaults — missing JSON block leaves all fields zero-value"

key-files:
  created:
    - internal/models/pricegap_position.go
    - internal/models/pricegap_interfaces.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go

key-decisions:
  - "ExitReason is a string enum (not int) so Redis JSON persistence stays readable"
  - "SlippageSample lives in pricegap_position.go (same file as ExitReason constants) to keep domain types colocated"
  - "ID() method on PriceGapCandidate uses symbol_longExch_shortExch, matching D-15 position-ID prefix"
  - "Candidate slice applyJSON guards with len(pg.Candidates) > 0 so zero-length slices stay nil (safe-off semantics)"

patterns-established:
  - "PriceGapStore interface: concrete *database.Client will implement in Plan 02 — no leak across layers (D-02)"
  - "applyJSON partial-block semantics: any missing field keeps zero value, matching T-08-03 nil-safe deref"

requirements-completed: [PG-05, PG-OPS-06]

duration: 3min
completed: 2026-04-21
---

# Phase 08 Plan 01: Data contracts + config surface Summary

**Wave-1 foundation for Strategy-4 price-gap tracker: PriceGapPosition model, PriceGapStore DI interface, ExitReason enum, PriceGapCandidate tuple, and 11 default-OFF PriceGap* config fields wired into applyJSON.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-21T09:05:05Z
- **Completed:** 2026-04-21T09:08:15Z
- **Tasks:** 2
- **Files created:** 2
- **Files modified:** 2

## Accomplishments

- PriceGapPosition struct with 16 persisted fields + 4 status constants covering the full lifecycle (pending/open/exiting/closed)
- ExitReason enum: reverted, max_hold, manual, risk_gate, exec_quality, recovered_orphan
- PriceGapStore interface exposes 13 methods across positions, exec-quality disable flag, slippage rolling window, and distributed locks — consumable by Plans 02–06 without touching *database.Client
- 11 PriceGap* Config fields with pointer-optional jsonPriceGap mirror; zero-value defaults guarantee safe-off when price_gap block is absent or partial
- Two unit tests cover both the fully-populated and the missing-block paths under -race

## Task Commits

1. **Task 1: PriceGapPosition + PriceGapStore + ExitReason + PriceGapCandidate + SlippageSample** — `dc1e2e7` (feat)
2. **Task 2: 11 Config fields + jsonPriceGap + applyJSON + 2 tests** — `6b6a2e0` (feat)

_Plan metadata commit (SUMMARY + state) follows this summary._

## Files Created/Modified

- `internal/models/pricegap_position.go` (created) — PriceGapPosition struct, ExitReason constants, SlippageSample, status constants
- `internal/models/pricegap_interfaces.go` (created) — PriceGapCandidate tuple + ID() helper, PriceGapStore DI interface
- `internal/config/config.go` (modified) — 11 new Config fields, jsonPriceGap mirror, applyJSON block, added `arb/internal/models` import
- `internal/config/config_test.go` (modified) — TestApplyJSON_PriceGap (full block), TestDefaults_PriceGap (safe-off)

## Decisions Made

- **String enum for ExitReason** — keeps persisted JSON human-readable and matches spot-futures SpotStatus* precedent
- **Colocated domain types** — SlippageSample + ExitReason live in pricegap_position.go; Candidate + Store live in pricegap_interfaces.go (logical split: data shape vs behavioural contract)
- **`len(pg.Candidates) > 0` guard** — a zero-length JSON array leaves `PriceGapCandidates=nil`, matching safe-off; non-empty array wins
- **No Redis impl yet** — PriceGapStore is a contract only; *database.Client implementation is Plan 02 scope per plan DAG

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria met.

## Issues Encountered

None.

## Verification

- `go build ./...` — succeeds
- `go vet ./internal/config/... ./internal/models/...` — clean
- `go test ./internal/config/... ./internal/models/... -race -count=1` — 20 tests pass (incl. 2 new PriceGap tests)
- D-02 boundary check — `grep -rn "internal/engine|internal/spotengine|internal/database" internal/models/pricegap*.go` returns zero matches
- `git diff config.json` — no changes (CLAUDE.local.md lockdown honored)

## Threat Flags

None — the scope is struct declarations and JSON mapping. All surface introduced is covered by the plan's threat model (T-08-01..T-08-04).

## Next Phase Readiness

- **Plan 02 (Redis persistence)** can start immediately: interface is frozen, concrete impl goes on *database.Client
- **Plans 03–06 (tracker internals)** have their types defined — no scavenger-hunt anti-pattern
- **Phase 9 dashboard** can already surface `PriceGapEnabled` + PriceGapCandidates via existing `/api/config` route (read-side only; toggle wiring is Phase 9 scope)

## Self-Check: PASSED

- `internal/models/pricegap_position.go` — FOUND
- `internal/models/pricegap_interfaces.go` — FOUND
- Commit `dc1e2e7` — FOUND on main
- Commit `6b6a2e0` — FOUND on main
- `go build ./...` — PASS
- `go test ./internal/config/... ./internal/models/... -race -count=1` — PASS (20/20)

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
