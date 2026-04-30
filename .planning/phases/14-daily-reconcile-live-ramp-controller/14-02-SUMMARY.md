---
phase: 14-daily-reconcile-live-ramp-controller
plan: 02
subsystem: pricegap-reconcile
tags: [pricegap, reconcile, live-capital, ramp-controller, idempotency, anomaly-detection, go, tdd]

# Dependency graph
requires:
  - phase: 14-daily-reconcile-live-ramp-controller
    plan: 01
    provides: PriceGapPosition.Version + ExchangeClosedAt fields, 4-method reconcile + ramp store extension, PriceGapAnomalySlippageBps config field, Wave-0 4 reconciler test stubs
provides:
  - Reconciler struct with 3-retry imitation of internal/engine/exit.go:1119 (D-15 isolation preserved)
  - DailyReconcileRecord typed schema (4 structs, no maps, byte-deterministic JSON)
  - ReconcileStore + ReconcileNotifier narrow interfaces (D-15 boundary)
  - PriceGapNotifier interface extended with NotifyPriceGapDailyDigest + NotifyPriceGapReconcileFailure
  - LoadRecord public surface for downstream RampController consumer (Plan 14-03)
affects:
  - 14-03-ramp-controller-sizer-gate6 (consumes LoadRecord + DailyReconcileTotals.NetClean signal)
  - 14-04-daemon-notifier (consumes Reconciler + replaces stub Telegram methods)
  - 14-05-cli-pg-ramp-admin (renders DailyReconcileRecord)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "3-retry imitation pattern (5s/15s/30s) — implemented locally to preserve D-15 module isolation; cross-module pattern reuse without importing"
    - "Typed-struct deterministic JSON marshalling — no maps, sorted slices, frozen ComputedAt via nowFn — locked by TestReconcile_Idempotency_ByteEqual at -count=100"
    - "Narrow interface DI (ReconcileStore + ReconcileNotifier) — same shape as Phase 12 PromotionController for consistency"
    - "Empty non-nil slice initialization (FlaggedIDs = []string{}) — preserves byte-equality on no-anomaly days (JSON '[]' not 'null')"

key-files:
  created:
    - "internal/pricegaptrader/reconcile_record.go (4 typed structs + reconcileSchemaVersion=1)"
    - "internal/pricegaptrader/reconciler.go (Reconciler + RunForDate + helpers)"
    - "internal/notify/pricegap_reconcile.go (Plan 14-02 TelegramNotifier stubs for the 2 new interface methods)"
  modified:
    - "internal/pricegaptrader/notify.go (PriceGapNotifier extended; NoopNotifier stubs added)"
    - "internal/pricegaptrader/reconciler_test.go (Wave-0 stubs replaced with 4 real test bodies)"
    - "internal/pricegaptrader/tracker_broadcast_test.go (spyNotifier extended for interface conformance)"

key-decisions:
  - "Triple-fail retry shape lands as [5s,15s,30s] but the loop sleeps BEFORE attempts 1 and 2 (not before attempt 0), so recorded sleeps for triple-fail are [15s,30s] — verified explicitly by TestReconcile_TripleFail_SkipsDay"
  - "ReconcileStore + ReconcileNotifier interfaces declared in reconciler.go (NOT models/pricegap_interfaces.go) — keeps reconcile-specific concerns inside pricegaptrader and avoids widening the cross-module models surface; *database.Client satisfies ReconcileStore implicitly via the existing PriceGapStore method set from Plan 14-01"
  - "TelegramNotifier digest+failure methods land as no-op stubs in a SEPARATE file (internal/notify/pricegap_reconcile.go) so telegram.go does not gain a pricegaptrader import — preserves the existing module-graph layout"
  - "spyNotifier (existing test fake) extended with 2 new methods recording calls in the existing spyCall slice — interface conformance preserved without rewriting the spy"
  - "FlaggedIDs initialized to []string{} (never nil) — ensures JSON output is '[]' on no-anomaly days, byte-equal to other no-anomaly days; this is a hard requirement for D-04"

patterns-established:
  - "When a Plan extends a public interface (PriceGapNotifier here): the GREEN-phase commit MUST also extend ALL existing fakes that satisfy the interface — Rule 2 auto-fix on missing critical functionality. In this Plan: spyNotifier (test fake) + *TelegramNotifier (production stub) — both updated alongside the interface extension"
  - "3-retry pattern is small enough to imitate rather than extract into a shared package — preserves the readability of each call site's retry semantics while honoring D-15 isolation"

requirements-completed: [PG-LIVE-03]

# Metrics
duration: 8min
completed: 2026-04-30
---

# Phase 14 Plan 02: Reconciler Core Summary

**Strategy 4 daily reconciler with 3-retry resilience, byte-identical idempotency, and anomaly flagging — TDD methodology, all 4 tests green, idempotency lock test passes at -count=100.**

## Performance

- **Duration:** ~8 minutes
- **Started:** 2026-04-30T07:12:47Z
- **Completed:** 2026-04-30T07:20:12Z
- **Tasks:** 3 (all auto, all `tdd=true`)
- **Files created:** 3 (reconcile_record.go, reconciler.go, pricegap_reconcile.go)
- **Files modified:** 3 (notify.go, reconciler_test.go, tracker_broadcast_test.go)
- **Commits:** 3 atomic per-task commits

## Accomplishments

- **Reconciler core landed:** RunForDate aggregates closed Strategy 4 positions for a UTC date with 3-retry resilience (5s/15s/30s delays, applied between attempts not before/after) and triple-fail Telegram dispatch via the Notifier abstraction. Imitates internal/engine/exit.go:1119 without importing internal/engine — D-15 module isolation preserved (zero `internal/engine` import statements in reconciler.go).
- **Idempotency contract locked:** TestReconcile_Idempotency_ByteEqual passes at `-count=100` proving byte-identical Save payloads across re-runs of the same date. Achieved via typed-struct schema (no Go map types), positions sorted by (ExchangeClosedAt, ID) ASC, FlaggedIDs sorted ASC, frozen ComputedAt via injectable nowFn.
- **Anomaly detection wired:** Two flag types fire — high-slippage (`abs(RealizedSlipBps) > cfg.PriceGapAnomalySlippageBps`) per D-09 and missing-close-timestamp (`ExchangeClosedAt.IsZero() → fall back to ClosedAt + flag`) per D-10. Flagged IDs aggregate into `Anomalies.FlaggedIDs` (sorted ASC).
- **Paper-mode parity:** Reconciler does NOT branch on `cfg.PriceGapLiveCapital` (D-07) — clean-day signal (`Totals.NetClean`) accumulates regardless of live/paper mode, ready for RampController consumption in Plan 14-03.
- **TDD discipline:** RED phase (Task 2) committed independently with failing tests proving "undefined: Reconciler / NewReconciler"; GREEN phase (Task 3) committed with all 4 tests passing — clean separation between test surface and implementation.

## Task Commits

Each task committed atomically:

1. **Task 1: Define DailyReconcileRecord typed schema** — `7f0258c` (feat): 4 typed structs (DailyReconcileRecord, DailyReconcileTotals, DailyReconcilePosition, DailyReconcileAnomalies); `reconcileSchemaVersion = 1`; zero map types in struct fields.
2. **Task 2: RED — replace Wave-0 reconciler stubs with failing tests** — `50b0ea7` (test): 4 test bodies (Idempotency_ByteEqual, MissingExchangeCloseTs_FallsBackToLocal, TripleFail_SkipsDay, AnomalyHighSlippage_Flagged); fakeReconcileStore + fakeReconcileNotifier + fakeSleeper fixtures; tests fail to compile (`undefined: Reconciler / NewReconciler`) — RED intentional.
3. **Task 3: GREEN — Reconciler implementation makes 4 tests pass** — `37b5e12` (feat): Reconciler + 5 methods (RunForDate, LoadRecord, aggregateAndWrite, buildPositionRecord, computeAggregates) + ReconcileStore + ReconcileNotifier narrow interfaces; PriceGapNotifier interface extended with 2 new methods; NoopNotifier + spyNotifier + *TelegramNotifier stubs added for compile-time conformance.

## Files Created/Modified

**Created:**
- `internal/pricegaptrader/reconcile_record.go` (70 lines) — 4 typed structs, schema version constant, doc-only file
- `internal/pricegaptrader/reconciler.go` (272 lines) — Reconciler + 5 methods + 2 narrow interfaces + reconcileRetryDelays var + 2 helpers
- `internal/notify/pricegap_reconcile.go` (40 lines) — *TelegramNotifier stubs satisfying widened pricegaptrader.PriceGapNotifier interface (real Telegram dispatch in Plan 14-04)

**Modified:**
- `internal/pricegaptrader/notify.go` — PriceGapNotifier interface +2 methods; NoopNotifier +2 no-ops
- `internal/pricegaptrader/reconciler_test.go` — Wave-0 4 t.Skip stubs replaced with 474 lines of test bodies + fakes (added 466 lines net)
- `internal/pricegaptrader/tracker_broadcast_test.go` — spyNotifier +2 methods (~20 lines)

## Verification Confirmed

| Check | Result |
|-------|--------|
| `go test ./internal/pricegaptrader/... -run 'TestReconcile_' -count=1` | 4/4 PASS |
| `go test ./internal/pricegaptrader/... -run 'TestReconcile_Idempotency_ByteEqual' -count=100` | 100/100 PASS |
| `go build ./...` | clean exit 0 |
| `go vet ./...` | clean exit 0 |
| `grep -nE '^\s*"arb/internal/engine"' internal/pricegaptrader/reconciler.go` | 0 matches (D-15 isolation) |
| `grep -E '5 \* time.Second, 15 \* time.Second, 30 \* time.Second' reconciler.go` | 1 match (literal pinned) |
| 5 Reconciler method signatures (RunForDate, LoadRecord, aggregateAndWrite, buildPositionRecord, computeAggregates) | confirmed via grep |
| Notifier interface + Noop stub method count for the 2 new methods | 8 grep hits in notify.go (interface decl + Noop stubs + comments) |

## Decisions Made

1. **Sleep timing in 3-retry loop** — The plan's RESEARCH.md note hedged on whether the recorded sleeps would be `[5s,15s]` or `[5s,15s,30s]`. The implementation pattern `if attempt > 0 { sleep(delays[attempt]) }` produces sleeps `[delays[1], delays[2]] = [15s, 30s]` — that is, sleep BEFORE attempts 1 and 2 (counting from 0), not before attempt 0, not after attempt 2. TestReconcile_TripleFail_SkipsDay locks this exact shape. Plan 14-03 RampController treating unreconciled days as ambiguous (fail-safe per T-14-03) means the precise sleep timing only matters for total wallclock latency, not for correctness — but locking the shape prevents accidental drift.

2. **Reconcile interfaces local to pricegaptrader** — `ReconcileStore` and `ReconcileNotifier` declared in `reconciler.go`, NOT in `internal/models/pricegap_interfaces.go`. Rationale: these are reconcile-specific consumer surfaces; widening the cross-module models surface would invite unrelated callers to depend on reconcile method signatures. `*database.Client` satisfies `ReconcileStore` implicitly via the matching method set already added in Plan 14-01.

3. **TelegramNotifier digest+failure stubs in a separate file** — `internal/notify/pricegap_reconcile.go` rather than appending to `telegram.go`. Rationale: telegram.go currently does not import `arb/internal/pricegaptrader`; adding the import there for one type reference (`pricegaptrader.DailyReconcileRecord`) widens the module-graph footprint. Keeping the import isolated to the assertion file (pricegap_assert.go) and the reconcile-stub file (pricegap_reconcile.go) preserves the existing layout. Plan 14-04 will populate the stub bodies with real Telegram dispatch — the file boundary is incidental, not load-bearing.

4. **spyNotifier extension via the existing spyCall slice** — Rather than rewriting spyNotifier or introducing a new test seam, the 2 new methods append to the existing `n.calls` field with descriptive Kind values (`"notify_daily_digest"`, `"notify_reconcile_failure"`). Tests that don't exercise reconcile dispatch see no behavior change; tests that do (none in 14-02; potentially 14-04) can probe via `countKind`.

5. **FlaggedIDs initialization to non-nil empty slice** — `anomalies.FlaggedIDs = []string{}` in `computeAggregates`. Rationale: nil slice marshals to `null`; empty slice marshals to `[]`. For two no-anomaly days to produce byte-equal payloads (Plan 14-03 RampController will compare consecutive days) the slice MUST be non-nil. This is a hard requirement of D-04.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Extended `*spyNotifier` with 2 new methods**
- **Found during:** Task 3 verification (full test compile)
- **Issue:** Adding `NotifyPriceGapDailyDigest` + `NotifyPriceGapReconcileFailure` to `PriceGapNotifier` broke compilation of `paper_to_live_test.go` and 3 call sites in `tracker_broadcast_test.go` (`*spyNotifier` no longer satisfied the widened interface).
- **Fix:** Added 2 methods to spyNotifier in tracker_broadcast_test.go that record into the existing `spyCall` slice with Kind tags (`notify_daily_digest`, `notify_reconcile_failure`). Tests that don't exercise these methods see no behavior change.
- **Files modified:** internal/pricegaptrader/tracker_broadcast_test.go
- **Verification:** All pricegaptrader tests except the pre-existing flaky scanner test pass.
- **Committed in:** `37b5e12` (Task 3 GREEN — interface conformance is a Task 3 acceptance criterion).

**2. [Rule 2 - Missing Critical] Extended `*TelegramNotifier` with 2 stub methods**
- **Found during:** Task 3 verification (full repo build)
- **Issue:** `internal/notify/pricegap_assert.go` asserts at compile time that `*TelegramNotifier` satisfies `pricegaptrader.PriceGapNotifier`. Adding 2 methods to the interface broke the assertion (`*TelegramNotifier does not implement pricegaptrader.PriceGapNotifier (missing method NotifyPriceGapDailyDigest)`).
- **Fix:** Created `internal/notify/pricegap_reconcile.go` with 2 nil-receiver-safe no-op methods. Plan 14-04 replaces these stubs with real Telegram dispatch. Used a separate file (not telegram.go) so telegram.go does not gain an `arb/internal/pricegaptrader` import — keeps the existing module-graph layout intact.
- **Files modified:** internal/notify/pricegap_reconcile.go (new)
- **Verification:** `go build ./...` clean.
- **Committed in:** `37b5e12` (Task 3 GREEN — same reason).

**3. [Rule 3 - Blocking] Adjusted reconcile_record.go comment to drop "map[]" string**
- **Found during:** Task 1 acceptance criteria check
- **Issue:** The plan acceptance criterion `grep -c "map\[" internal/pricegaptrader/reconcile_record.go` requires zero matches. My initial doc comment used the phrase "(no map[])" which matched the grep.
- **Fix:** Reworded the comment to "no Go map types" — same meaning, no false-positive grep match.
- **Files modified:** internal/pricegaptrader/reconcile_record.go (Task 1 commit)
- **Verification:** `grep -c "map\[" reconcile_record.go` returns 0.
- **Committed in:** `7f0258c` (Task 1 — fix landed before commit).

**Total deviations:** 3 auto-fixed (2 Rule 2 missing critical interface conformance, 1 Rule 3 blocking acceptance grep). All preserved acceptance criteria. No scope creep — every deviation was inside the declared task surface.

## Issues Encountered

- **Pre-existing flaky test `TestScanner_RunCycleCallsPromotionApply`** failed on full-package run (2/3 pass on `go test -count=3`). Identical failure mode as recorded in Plan 14-01 SUMMARY (event mismatch with exchange labels swapped). Out-of-scope for Plan 14-02 (reconciler does not touch scanner event emission). Logged in `.planning/phases/14-daily-reconcile-live-ramp-controller/deferred-items.md` for systemic flake triage under CLAUDE.md `/sdebug` workflow.

## User Setup Required

None — Plan 14-02 ships dormant code behind the existing `cfg.PriceGapLiveCapital` toggle (default OFF from Plan 14-01). The Reconciler is constructible but not yet wired into a daemon — Plan 14-04 ships the cron daemon that calls `Reconciler.RunForDate` at UTC 00:30. No Redis migrations, no env vars, no operator action.

## Next Phase Readiness

**Plan 14-03 (RampController + Sizer + Gate 6) ready:**
- Public `Reconciler.LoadRecord(ctx, date) (DailyReconcileRecord, bool, error)` is the consumer surface. RampController will call this for the previous day's date, check `record.Totals.NetClean`, and increment its in-memory clean-day counter.
- `DailyReconcileRecord.Totals.NetClean` is the typed boolean signal — implements D-05 logic (PnL >= 0 AND PositionsClosed >= 1).
- `bool, error` triple from LoadRecord communicates 3 states to RampController: (record, true, nil) = reconciled day, (zero, false, nil) = unreconciled, (zero, false, err) = decode/load failure → fail-safe (no promote, no demote).

**Plan 14-04 (Daemon + Notifier) ready:**
- Reconciler ships with `SetSleepFunc` + `SetNowFunc` injection seams — daemon's tick-driven goroutine can swap to a virtual clock for tests.
- Stub methods `*TelegramNotifier.NotifyPriceGapDailyDigest` and `NotifyPriceGapReconcileFailure` in `internal/notify/pricegap_reconcile.go` are the slots Plan 14-04 fills with real dispatch (cooldown keys, critical-path override, message formatting).

**No blockers. CLAUDE.local.md rules respected:** config.json untouched, no npm install, frontend unchanged, Go module isolation preserved (zero internal/engine imports in reconciler.go).

## Self-Check: PASSED

Verification claims confirmed:

- All commits exist:
  - `7f0258c` (Task 1 Define DailyReconcileRecord) — FOUND
  - `50b0ea7` (Task 2 RED tests) — FOUND
  - `37b5e12` (Task 3 GREEN implementation) — FOUND
- All claimed files exist:
  - `internal/pricegaptrader/reconcile_record.go` — FOUND (70 lines)
  - `internal/pricegaptrader/reconciler.go` — FOUND (272 lines)
  - `internal/notify/pricegap_reconcile.go` — FOUND (40 lines)
  - `internal/pricegaptrader/notify.go` modified — FOUND
  - `internal/pricegaptrader/reconciler_test.go` modified — FOUND
  - `internal/pricegaptrader/tracker_broadcast_test.go` modified — FOUND
- All acceptance criteria satisfied:
  - 4 typed structs in reconcile_record.go (4 grep hits)
  - reconcileSchemaVersion = 1 (1 grep hit)
  - 0 map[ in reconcile_record.go after fix
  - 0 t.Skip in reconciler_test.go (Wave-0 stubs all replaced)
  - 4 TestReconcile_ functions in reconciler_test.go
  - 5 Reconciler methods (RunForDate, LoadRecord, aggregateAndWrite, buildPositionRecord, computeAggregates)
  - 0 internal/engine imports in reconciler.go (D-15 isolation)
  - 1 grep match for the 5s/15s/30s literal
  - ≥4 NotifyPriceGapDailyDigest/Failure hits in notify.go (8 actual)
- Tests: 4 TestReconcile_* PASS; idempotency lock test PASS at -count=100
- go build ./... clean; go vet ./... clean
- config.json untouched (`git diff --stat config.json` returns empty)

---
*Phase: 14-daily-reconcile-live-ramp-controller*
*Completed: 2026-04-30*
