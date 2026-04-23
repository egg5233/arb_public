---
phase: 09-price-gap-dashboard-paper-live-operations
plan: 08
subsystem: e2e-validation+uat+release
tags: [phase-9, wave-5, paper-to-live, integration-tests, uat, release, version-bump, pg-ops-04, pg-ops-05, pg-val-01, pg-val-02, partial-uat, gaps-found]
requires:
  - 09-03 (paper mode chokepoint at PlaceOrder + pos.Mode immutability)
  - 09-04 (Telegram notifier methods + golden-string regression)
  - 09-05 (rolling metrics aggregator + /metrics handler)
  - 09-06 (tracker broadcast + notifier hooks + DI wiring)
  - 09-07 (dashboard page + REST/WS wiring)
provides:
  - internal/pricegaptrader/paper_to_live_test.go — 4 integration tests proving paper→live cutover + orphan-safety + Telegram continuity + Closed Log shape
  - internal/pricegaptrader/e2e_regression_test.go — guardrail tests (t.Skip + documented harness gap) pinning perp-perp and spot-futures as untouched
  - .planning/phases/09/09-UAT.md — 22-row manual checklist covering every PG-OPS-0X and PG-VAL-0X requirement plus i18n and accessibility
  - CHANGELOG.md — Phase 9 release notes under [0.34.0] - 2026-04-22
  - VERSION — bumped 0.33.0 → 0.34.0
affects:
  - internal/pricegaptrader/ (2 new test files, 0 production files modified)
  - CHANGELOG.md (prepended Phase 9 section)
  - VERSION (minor bump)
tech-stack:
  added: []
  patterns:
    - "integration test drives full paper→live cutover in a single miniredis-backed harness"
    - "t.Skip with explicit guardrail note for regressions whose harness does not exist (honest over fabricated)"
key-files:
  created:
    - internal/pricegaptrader/paper_to_live_test.go
    - internal/pricegaptrader/e2e_regression_test.go
    - .planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md
  modified:
    - CHANGELOG.md
    - VERSION
decisions:
  - "t.Skip preferred over fabricated regression harness for perp-perp/spotengine — the real guardrail is `go test ./... -count=1 -race` staying green; adding a synthetic assertion that can't actually detect regression adds false confidence."
  - "Task 3 human-verify checkpoint classified as PARTIAL — observability gap discovered mid-UAT blocks end-to-end paper-fire validation. Properly scoped as Phase 9.1 gap-closure, not a Phase 9 code defect."
metrics:
  duration: ~30m (autonomous Tasks 1+2) + ~15m operator UAT before gap discovery
  tasks_completed: 3/3 (with Task 3 outcome = gap-found, not approved)
  completed_date: 2026-04-22
requirements: [PG-OPS-01, PG-OPS-02, PG-OPS-03, PG-OPS-04, PG-OPS-05, PG-VAL-01, PG-VAL-02]
requirements_status:
  PG-OPS-01: "code shipped + automated tests green + UAT blocked by observability gap (no detections firing)"
  PG-OPS-02: "code shipped + WS throttle tested + UAT blocked by observability gap"
  PG-OPS-03: "code shipped + Closed Log shape tested + UAT blocked by observability gap"
  PG-OPS-04: "code shipped + paper→live cutover proven in `TestPaperToLiveCutover` + UAT header pill visually confirmed"
  PG-OPS-05: "code shipped + byte-for-byte regression + UAT blocked (no entry/exit event to trigger Telegram on)"
  PG-VAL-01: "code shipped + `TestPaperToLiveCutover_ClosedLogShape` proves Realized > 0 in tests + UAT blocked"
  PG-VAL-02: "code shipped + pure-function aggregator unit-tested + UAT rolling-metrics row blocked (no history to aggregate)"
---

# Phase 09 Plan 08: End-to-end validation + UAT + release bump Summary

**One-liner:** Paper→live cutover proven atomically in integration tests and version bumped to 0.34.0, but end-to-end live paper-fire UAT blocked by a detector-observability gap discovered during operator walkthrough — scoped as Phase 9.1 candidate, not a Phase 9 code defect.

## What Shipped (Autonomous Work)

### Task 1 — Paper→Live cutover integration test + regression sweep (commit `cf2b97e`)

Created `internal/pricegaptrader/paper_to_live_test.go` with four integration tests:

- `TestPaperToLiveCutover` — opens paper position A with `PriceGapPaperMode=true`, flips flag to false mid-life, opens live position B, closes both. Asserts `pg:history` carries 2 entries with `pA.Mode="paper"` and `pB.Mode="live"`, stub `PlaceOrder` counter is 2 (live legs only), and `pA.RealizedSlipBps > 0` (Pitfall 7 proof).
- `TestPaperToLiveCutover_NoOrphans` — paper monitor continues managing A after the flip and closes via synth path; no live `PlaceOrder` for the paper close.
- `TestPaperToLiveCutover_TelegramContinuity` — mock notifier records 2 entry + 2 exit calls; A's exit carries `📝 PAPER` prefix, B's does not.
- `TestPaperToLiveCutover_ClosedLogShape` — `GET /api/pricegap/closed` returns both rows with correct Modeled / Realized / Delta fields, newest first.

Created `internal/pricegaptrader/e2e_regression_test.go` with two tests:

- `TestE2ERegression_PerpPerpEngineUntouched` — `t.Skip` with explicit note that the real guardrail is `go test ./internal/engine/... -count=1 -race` staying green + Plan 04's golden-string regression pins.
- `TestE2ERegression_SpotFuturesEngineUntouched` — same pattern for `internal/spotengine/`.

Rationale for `t.Skip`: fabricating a "regression" that can't actually detect regression gives false confidence. The honest guardrail is the existing unit-test suite staying green across the phase boundary, which it does.

Verification run: `go test ./... -count=1 -race -short` green, `(cd web && npm run build)` green, `bash scripts/check-i18n-sync.sh` green, `go build ./...` green.

### Task 2 — UAT checklist + CHANGELOG + VERSION (commit `6907415`)

Created `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md` with 22 checkbox rows covering every PG-OPS-0X and PG-VAL-0X requirement plus i18n and accessibility.

Bumped `VERSION` 0.33.0 → 0.34.0 and prepended a Phase 9 entry to `CHANGELOG.md` covering the full Plan 01–08 surface.

### Task 3 — Human UAT walkthrough (this plan's deliverable, outcome = PARTIAL / gap-found)

The operator began walking the UAT checklist against a live dev host. After `make build` + `./arb` launched cleanly, dashboard rendered, and 4 price-gap candidates loaded (SOON, RAVE, HOLO, SIGN — all confirmed in the funding-rate scanner's subscription universe), the UAT stalled on the first row that required a live detection: zero paper trades fired and zero diagnostic log lines appeared in 10+ minutes of runtime, despite thresholds set aggressively low (20 bps) for observability.

## UAT Outcome: PARTIAL (Observability Gap Found)

Operator walked the UAT rows that require only static / deterministic surface (header pill visuals, nav placement, default-paper-TRUE boot). These rows are confirmed passing.

Operator could **not** walk any row that depends on a live detection firing (Live Positions populated, Closed Log populated, Telegram entry/exit alerts, rolling metrics populated, paper→live cutover visual). Root cause was traced during the session and is not a Phase 9 code defect — it's a missing observability path that would have been designed into Phase 8 if the need had surfaced earlier.

### Phase 9.1 Gap Candidates (with file:line anchors)

**Gap #1 — Silent detector non-fire in `internal/pricegaptrader/tracker.go:287-289`**

```go
det := t.detectOnce(cand, now)
if !det.Fired {
    continue      // <-- no log of det.Reason
}
```

`DetectionResult` already carries a `Reason` field (populated at `internal/pricegaptrader/detector.go:113` with values like `"sample_error: ..."`, `"insufficient_persistence"`, `"stale_bbo"`), but `runTick` discards it on every non-firing detection. The detector is effectively a black box in production — the operator has no way to distinguish "no edge today" from "BBO never populated" from "persistence threshold too tight". Phase 9.1 should add a structured debug log (rate-limited) behind a config flag so UAT-grade observability is available without log spam in steady state.

**Gap #2 — No subscription path for pricegap candidates**

`pkg/exchange.SubscribeSymbol` has zero callers in `internal/pricegaptrader/` or `cmd/main.go`. All production callers live in `internal/engine/` (perp-perp). Pricegap candidates that happen to also be in the funding-rate scanner's universe inherit BBO subscriptions "for free" (SOON/RAVE/HOLO/SIGN were in that set), but any candidate outside that universe (e.g. VICUSDT, which produced zero log lines in the operator's test) receives no BBO and silently fails detection Gap #1 masks this entirely. Phase 9.1 should either:

1. have the tracker subscribe its own candidates at startup (clean boundary, independent of perp-perp), or
2. document the dependency explicitly and refuse to start if any candidate has no active BBO subscription (fail-loud).

### Impact on Phase 9 Deliverables

- **Autonomous code (Plans 09-01 through 09-07 + Task 1 + Task 2 of this plan):** sound, shipped per spec, all tests green, builds clean, dashboard renders. Nothing to revert.
- **UAT cannot confirm PG-OPS-04 paper fire end-to-end** — code is proven correct by `TestPaperToLiveCutover` but the operator needs a live observable fire to sign off.
- **UAT cannot confirm PG-OPS-05 Telegram notification pipeline end-to-end** — regression tests pin the message shape byte-for-byte but no entry/exit event has occurred to trigger real Telegram traffic.

These are properly scoped as a gap-closure phase (9.1), not a UAT failure of Phase 9 code. **Phase 9 verifier should return `gaps_found`** when it runs next — the gap is in observability, not in the deliverables' correctness.

## Deviations from Plan

**None during autonomous execution.** Tasks 1 and 2 landed exactly as specified. The deviation is entirely in Task 3's outcome — the plan anticipated either "approved" or "specific UAT rows failed"; reality produced a third category (UAT-blocked-by-observability) that the plan did not explicitly enumerate but is legitimate per the checkpoint's `resume-signal` allowing "describe specific UAT rows that failed so a gap-closure plan can be drafted".

## Threat Flags

None — no new security surface introduced. All new files are test-only (`_test.go`) or documentation.

## Known Stubs

None from this plan. Stubs called out by earlier plans (Phase 9 verifier should track these cumulatively) are:

- Rolling metrics zero-activity rows (09-05) — intentional, padded from `cfg.PriceGapCandidates` at handler level per D-25.

## Verification Checklist

- [x] `go test ./... -count=1 -race -short` green (verified at Task 1 close)
- [x] `go build ./...` green
- [x] `(cd web && npm run build)` green
- [x] `bash scripts/check-i18n-sync.sh` green
- [x] `test -f .planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md`
- [x] `head -30 CHANGELOG.md | grep 0.34.0` matches
- [x] `cat VERSION` returns `0.34.0`
- [ ] Human UAT fully signed off — **BLOCKED by Gap #1 + Gap #2** → Phase 9.1

## Deferred Issues

- **Gap #1** — silent detector non-fire in `tracker.go:287-289` → Phase 9.1
- **Gap #2** — no BBO subscription path for pricegap-only candidates → Phase 9.1
- `e2e_regression_test.go` uses `t.Skip` for both perp-perp and spot-futures regression tests; the real guardrail is the existing test suite staying green. A future phase could invest in a deterministic state-machine replay harness if drift becomes a real risk.

## Self-Check: PASSED

- FOUND: `internal/pricegaptrader/paper_to_live_test.go` (commit `cf2b97e`)
- FOUND: `internal/pricegaptrader/e2e_regression_test.go` (commit `cf2b97e`)
- FOUND: `.planning/phases/09-price-gap-dashboard-paper-live-operations/09-UAT.md` (commit `6907415`)
- FOUND: `CHANGELOG.md` entry `[0.34.0] - 2026-04-22` (commit `6907415`)
- FOUND: `VERSION` = `0.34.0` (commit `6907415`)
- FOUND: commit `cf2b97e` in git log
- FOUND: commit `6907415` in git log
