---
phase: 08-price-gap-tracker-core
plan: 04
subsystem: pricegaptrader
tags: [pricegap, strategy4, risk-gate, pre-entry, pg-risk-01, pg-risk-02, pg-risk-04, pg-risk-05, d-17]

requires:
  - Tracker skeleton + DelistChecker DI + PriceGapStore DI (from 08-03, 08-01)
  - DetectionResult (from 08-03)
  - Config fields PriceGapBudget / PriceGapMaxConcurrent / PriceGapGateConcentrationPct / PriceGapKlineStalenessSec (from 08-01)
provides:
  - 9 typed ErrPriceGap* sentinels for gate-denial classification
  - (t *Tracker).preEntry — 6-gate deterministic composer in D-17 order
  - GateDecision struct (Approved / Err / Reason) for downstream logging and exit_reason stamping
affects: [08-05, 08-06, 08-07]

tech-stack:
  added: []
  patterns:
    - "Typed sentinel errors via errors.New (matches pkg/exchange ErrRepayBlackout precedent)"
    - "Pure-function gate composition — no adapter I/O inside the gate itself"
    - "Fail-open on Redis-read error for disable flag (Phase 03-02 precedent)"
    - "Injected DelistChecker interface — no free-fn, no package-level indirection variable"

key-files:
  created:
    - internal/pricegaptrader/errors.go
    - internal/pricegaptrader/risk_gate.go
    - internal/pricegaptrader/risk_gate_test.go
  modified: []

key-decisions:
  - "Gate 5 (Gate-concentration cap) only evaluates existing gate-touching active notional when the CURRENT candidate has a Gate leg. Rationale: positions already on the book passed their own entry; the cap's job is to prevent THIS request from pushing the bucket over 50%. If the request doesn't touch Gate, it doesn't add to the bucket and the cap is irrelevant. Plan spec and implementation match."
  - "Per-position cap + budget use strict > (not ≥) — boundary values (requested == MaxPositionUSDT, activeSum+request == Budget) PASS. Tests lock this: TestRiskGate_PerPositionCap_Boundary, TestRiskGate_Budget_JustFits."
  - "Fail-open on disable-flag read error: preEntry logs WARN and skips gate 1 rather than blocking entry. Aligns with Phase 03-02 pattern (don't let Redis hiccups cause missed entries); caller still sees other 5 gates + the detector persistence gate."
  - "Delist gate sequenced AFTER budget/cap gates so cheap pure-math checks fail fast before consulting delist state (even though IsDelisted is O(1) memory read, ordering matches D-17 spec)."
  - "No indirection variable / seam for delist: tests drive the fakeDelistChecker through the tracker's t.delist field, which was already wired in Plan 03. Confirms the D-02 DI design."

patterns-established:
  - "GateDecision carries both a typed Err and a human Reason — downstream can switch on err for flow control and log the Reason for operator diagnostics"
  - "Ordering invariant test: when 2+ gates would fail, the FIRST in D-17 order wins. TestRiskGate_OrderingInvariant locks this as a regression test for any future refactor"

requirements-completed: [PG-RISK-01, PG-RISK-02, PG-RISK-04, PG-RISK-05]

duration: 8min
completed: 2026-04-21
---

# Phase 08 Plan 04: Pre-entry risk gates (PG-RISK-01/-02/-04/-05) Summary

**Wave-4 gatekeeper between detector and execution: 6 deterministic gates composed in D-17 order, returning the first failure as a typed GateDecision. This is the sole authorization check Plan 05 will call before any adapter PlaceOrder.**

## Performance

- **Duration:** ~8 min
- **Tasks:** 3
- **Files created:** 3
- **Files modified:** 0

## Accomplishments

- 9 typed `ErrPriceGap*` sentinels exported for gate-denial classification and downstream log/telemetry switching
- `(t *Tracker).preEntry(cand, requestedNotional, det, activePositions) GateDecision` — composes 6 gates in fixed D-17 order; returns first failure; runs no adapter I/O and makes no external calls beyond the injected `t.db.IsCandidateDisabled` and `t.delist.IsDelisted` seams
- Gate order (D-17): 1. exec-quality disabled → 2. max-concurrent → 3. per-position cap → 4. budget → 5. Gate-concentration (conditional on Gate-leg presence) → 6. delist / halt / staleness
- 14 unit tests with boundary coverage (just-fits vs just-over) plus an ordering-invariant test that locks D-17 against future refactors
- D-02 module boundary preserved — zero imports of `internal/engine/` or `internal/spotengine/` (grep asserted)

## Task Commits

1. **Task 1: typed ErrPriceGap* sentinels** — `ac16680` (feat)
2. **Task 2: 6-gate preEntry composer (risk_gate.go)** — `ec3f266` (feat)
3. **Task 3: 14 risk-gate unit tests + ordering invariant** — `5bfe601` (test)

_Plan metadata commit (SUMMARY + state + roadmap) follows this summary._

## Files Created/Modified

- `internal/pricegaptrader/errors.go` (created, 18 lines) — 9 typed sentinels
- `internal/pricegaptrader/risk_gate.go` (created, 105 lines) — preEntry composer + GateDecision struct + candHasGate / positionHasGate helpers
- `internal/pricegaptrader/risk_gate_test.go` (created, 330 lines) — fakeStore + fakeDelistChecker with compile-time interface assertions, 14 tests covering every gate + happy path + ordering invariant

## Decisions Made

- **Gate 5 conditional semantics.** The Gate-concentration cap only runs when `candHasGate(cand)` is true. When the request has no Gate leg, it doesn't contribute to the gate bucket, so the cap can't be tripped by this request. `TestRiskGate_GateConcentration_Uncaps_NoGateLeg` locks this — with existing $3000 gate-leg active positions on a budget of $5000 (cap would be $2500), a binance-bybit request for $1000 still passes because it doesn't touch the bucket.
- **Strict-inequality boundary.** Per-position cap and budget gates use `>`, not `≥`. Boundary tests lock this: `requested == MaxPositionUSDT` passes, `activeSum + request == Budget` passes. The over-by-one variants block.
- **Fail-open on Redis hiccup.** `IsCandidateDisabled` returning a non-nil error logs WARN and falls through to the other 5 gates. Matches the Phase 03-02 operational precedent of not letting Redis transients block trading. Gate 1's safety role is enforced by the flag-write path (Plan 06), not by fail-closed reads.
- **Delist gate at position 6, not earlier.** D-17 specifies delist AFTER budget/cap. Rationale: budget/cap are cheap pure-math checks that should fast-fail before any external state read (even though DelistChecker is O(1) in-memory, keeping the cheapest gates earliest is the pattern).
- **No test seam, no indirection variable.** `t.delist.IsDelisted` goes straight through the field injected in Plan 03's `NewTracker`. Tests pass a `fakeDelistChecker`; production will pass `*discovery.Scanner`. Plan explicitly forbade function patching (`SetDelistCheckForTest`) — confirmed absent via negative grep.

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria met:

- 9 typed errors exported ✓
- All 6 grep checks on risk_gate.go return 1 ✓
- `grep -c "t\.delist\.IsDelisted"` returns 1 ✓
- `grep -q "discovery\.IsDelisted"` returns false (no free-function call) ✓
- `grep -c "func TestRiskGate_"` returns 14 ✓
- `grep -c "fakeDelistChecker"` returns ≥1 ✓
- `grep -q "SetDelistCheckForTest"` returns false (no test-seam indirection) ✓

## Issues Encountered

None. All three tasks built + tested green on first attempt.

## Verification

- `go build ./internal/pricegaptrader/...` — succeeds
- `go vet ./internal/pricegaptrader/...` — clean
- `go build ./...` — succeeds across repo
- `go test ./internal/pricegaptrader -run TestRiskGate_ -race -count=1` — 14/14 pass
- `go test ./internal/pricegaptrader/... -race -count=1` — 22/22 pass (8 detector + 14 risk gate)
- D-02 boundary — `grep -c "internal/engine\|internal/spotengine" internal/pricegaptrader/*.go` returns zero matches
- CLAUDE.local.md honored — config.json untouched, no npm commands

## Threat Flags

None. Surface introduced is fully covered by the plan's threat model:

- T-08-13 (entry without gate check) — mitigated: preEntry is the single authorization seam; Plan 05 tests will assert it's called before PlaceOrder
- T-08-14 (gate ordering swap) — mitigated: `TestRiskGate_OrderingInvariant` locks D-17; any reorder breaks this test
- T-08-15 (Redis read error on disable flag) — accepted per plan: fail-open + WARN log, other 5 gates still execute
- T-08-16 (IsDelisted blocking) — mitigated: DelistChecker backs onto O(1) in-memory scanner cache; no network I/O on hot path

## Next Plan Readiness

- **Plan 05 (entry executor)** has its authorization seam: `preEntry(cand, notional, det, active)` returns `GateDecision`. Entry path should call it with the active snapshot from `t.db.GetActivePriceGapPositions()` and treat any non-nil `Err` as a hard block (log `Reason`, skip the candidate).
- **Plan 06 (monitor/exit + exec-quality)** will write to the disable flag via `SetCandidateDisabled(symbol, reason)` when rolling-window slippage exceeds threshold; Gate 1 here reads it on the next entry attempt.
- **Plan 07 (wiring)** no new constructor signature changes — the Tracker surface frozen in Plan 03 already has `delist` and `db` fields.

## Self-Check: PASSED

- `internal/pricegaptrader/errors.go` — FOUND
- `internal/pricegaptrader/risk_gate.go` — FOUND
- `internal/pricegaptrader/risk_gate_test.go` — FOUND
- Commit `ac16680` — FOUND on main
- Commit `ec3f266` — FOUND on main
- Commit `5bfe601` — FOUND on main
- `go build ./...` — PASS
- `go vet ./internal/pricegaptrader/...` — PASS
- `go test ./internal/pricegaptrader -run TestRiskGate_ -race -count=1` — PASS (14/14)
- `go test ./internal/pricegaptrader/... -race -count=1` — PASS (22/22)

---
*Phase: 08-price-gap-tracker-core*
*Completed: 2026-04-21*
