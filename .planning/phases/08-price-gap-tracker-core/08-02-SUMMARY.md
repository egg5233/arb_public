---
phase: 08-price-gap-tracker-core
plan: 02
subsystem: database
tags: [pricegap, strategy4, redis, persistence, locks]

requires:
  - PriceGapPosition / SlippageSample / ExitReason (from 08-01)
  - PriceGapStore interface (from 08-01)
provides:
  - "*database.Client satisfies models.PriceGapStore (11 methods, compile-time assertion)"
  - "pg:* Redis namespace (pg:positions, pg:positions:active, pg:history, pg:candidate:disabled:<sym>, pg:slippage:<cid>)"
  - "Distributed locks under arb:locks:pg:<resource> with compare-and-delete release"
affects: [08-03, 08-04, 08-05, 08-06, 08-08, phase-09]

tech-stack:
  added: []
  patterns:
    - "Pipeline-atomic save (HSET + active SET mutation mirrors spot_state.go lines 44-55)"
    - "LPush + LTrim cap pattern for history (500) and slippage window (10)"
    - "Token-based distributed lock: SetNX with random hex token + releaseLockScript (CAS)"
    - "Compile-time interface assertion: var _ models.PriceGapStore = (*Client)(nil)"

key-files:
  created:
    - internal/database/pricegap_state.go
    - internal/database/pricegap_state_test.go
  modified: []

key-decisions:
  - "Token-based locks (not plain bool AcquireLock) — interface contract requires token, and token CAS prevents a stale holder from deleting a reacquired lease"
  - "Reuse releaseLockScript from locks.go — same CAS semantics as existing arb:locks:<res> callers"
  - "priceGapLockPrefix = \"arb:locks:pg:\" — sub-prefix under the existing arb:locks: root so operators grepping for locks see one namespace"
  - "GetSlippageWindow clamps n: n<=0 or n>cap both return the full 10-entry cap — avoids silently truncating callers that forget to bound their request"
  - "GetPriceGapPosition returns explicit 'not found' error on redis.Nil (mirrors GetPosition in state.go); GetActivePriceGapPositions skips orphan SET members silently"

patterns-established:
  - "pg:* namespace isolation verified: zero grep hits for 'pg:' in state.go or spot_state.go"
  - "Interface assertion forces any future refactor breaking PriceGapStore to fail compile, not just runtime"

requirements-completed: [PG-04, PG-RISK-03]

duration: 8min
completed: 2026-04-21
---

# Phase 08 Plan 02: Redis persistence (pg:*) Summary

**Wave-2 foundation for Strategy-4 price-gap tracker: *database.Client now implements all 11 PriceGapStore methods with pipeline-atomic writes, namespace isolation under pg:, and token-based distributed locks under arb:locks:pg:.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-21T (sequential after 08-01)
- **Tasks:** 2
- **Files created:** 2
- **Files modified:** 0

## Accomplishments

- `internal/database/pricegap_state.go` — 11 PriceGapStore methods on *Client, with pipeline-atomic position save, pipeline-atomic history LPush+LTrim, candidate disable flag Set/Is/Clear, slippage window append+get (cap 10), and token-CAS distributed locks under `arb:locks:pg:`
- Compile-time interface assertion `var _ models.PriceGapStore = (*Client)(nil)` ensures any breaking refactor fails compile
- `internal/database/pricegap_state_test.go` — 7 miniredis-backed tests exercising round-trip, active-set transitions (open → closed → removed), 600-push LTrim cap, disable Set/Is/Clear cycle, 15-sample newest-first slippage window, lock prefix + second-acquire rejection + wrong-token CAS no-op, interface assertion
- Namespace isolation verified: `grep -r "pg:"` in state.go and spot_state.go returns zero matches — no collision with arb:* or arb:spot_*

## Task Commits

1. **Task 1: Implement pg:* Redis CRUD** — `802366a` (feat)
2. **Task 2: miniredis round-trip tests** — `6aa0f71` (test)

_Plan metadata commit (SUMMARY + state) follows this summary._

## Files Created/Modified

- `internal/database/pricegap_state.go` (created, 250 lines) — 11 methods + 6 key constants + interface assertion
- `internal/database/pricegap_state_test.go` (created, 329 lines) — 7 tests, miniredis fixture helper

## Decisions Made

- **Token-based lock API.** The plan called for `AcquirePriceGapLock(resource, ttl) (token, ok, err)` but the existing `AcquireLock` on *Client returns only `(ok, err)` with no token. Rather than change the shared `AcquireLock` signature (risk of breaking perp-perp callers) or degrade the interface, I implemented a fresh SetNX with a random 16-byte hex token and reused `releaseLockScript` from `locks.go` for the compare-and-delete release. This matches the semantics of `AcquireOwnedLock` without inheriting its goroutine-based renewal — pricegap lock hold times are short (entry + exit rounds) and a fixed TTL is sufficient.
- **`priceGapLockPrefix = "arb:locks:pg:"`.** Sub-prefix under the existing `arb:locks:` root rather than a fresh `pg:locks:` namespace so operators inspecting distributed locks see one consistent place. The `pg:` sub-prefix is still distinct from perp-perp's `arb:locks:<symbol>` (T-08-08).
- **Slippage window clamp.** `GetSlippageWindow(cid, n)` clamps `n<=0` and `n>cap` to the full 10-entry cap. This avoids surprising callers who pass a large `n` expecting "give me everything" — matches the permissive spirit of `GetHistory(limit)` in state.go.
- **Orphan-member tolerance.** `GetActivePriceGapPositions` silently skips SET members whose HASH value is missing (mirrors the `ids vs vals` pattern in `GetActiveSpotPositions`). The tracker's recovery path calls `RemoveActivePriceGapPosition` explicitly.

## Deviations from Plan

**[Rule 1 — Fix] Plan sample code used `err.Error() == "redis: nil"` for missing-key detection.** Replaced with the idiomatic `err == redis.Nil` sentinel comparison (go-redis/v9 canonical check). Plan's string-match version would have worked in practice but is brittle — any future wrapping of the error would silently turn the missing-key branch into a real-error branch. Commit `802366a` uses the sentinel.

No other deviations.

## Issues Encountered

None. Tests passed first run; build clean on first compile.

## Verification

- `go build ./internal/database/...` — succeeds
- `go vet ./internal/database/...` — clean
- `go test ./internal/database -run TestPriceGapState -race -count=1` — 7/7 pass
- `go test ./internal/database -race -count=1` — 13/13 pass (entire package, no regressions)
- `go build ./...` — full repo builds
- Compile-time assertion `var _ models.PriceGapStore = (*Client)(nil)` present in pricegap_state.go:35
- Namespace isolation — `grep -n "pg:" internal/database/state.go internal/database/spot_state.go` returns zero matches
- CLAUDE.local.md honored — no config.json changes, no npm install commands

## Threat Flags

None. All pg:* surface is covered by the plan's threat model (T-08-05 key collision, T-08-06 history cap, T-08-07 slippage DoS, T-08-08 lock namespace).

## Next Plan Readiness

- **Plan 03 (scanner/validator)** can inject `*database.Client` as `models.PriceGapStore` — no more stub store needed
- **Plan 04 (tracker entry/exit)** can persist positions and query active set
- **Plan 05 (risk gate)** can use Set/Is/Clear + slippage window for exec-quality breach detection (PG-RISK-03)
- **Plan 06 (control loop)** can take `AcquirePriceGapLock("SOONUSDT", 30s)` around entry to serialize cross-scan races
- **Plan 08 (pg-admin CLI)** can list active + history + clear disable flags through the same interface

## Self-Check: PASSED

- `internal/database/pricegap_state.go` — FOUND
- `internal/database/pricegap_state_test.go` — FOUND
- Commit `802366a` — FOUND on main
- Commit `6aa0f71` — FOUND on main
- `go build ./...` — PASS
- `go test ./internal/database -race -count=1` — PASS (13/13)

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
