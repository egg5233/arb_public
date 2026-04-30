# Phase 14 — Deferred Items

Items discovered during Phase 14 plan execution that are out-of-scope for the
current plan and intentionally not addressed here.

## Pre-existing flaky test: TestScanner_RunCycleCallsPromotionApply

- **First observed:** Plan 14-01 SUMMARY (2026-04-30)
- **Re-observed:** Plan 14-02 (2026-04-30) on `go test -count=3` — 2/3 PASS, 1/3 FAIL
- **Failure mode:** event mismatch with exchange labels swapped vs expected
  (`bybit/binance` vs `binance/bybit`) — non-deterministic event ordering
- **Module:** `internal/pricegaptrader/scanner_test.go`
- **Why deferred:** Unrelated to Phase 14 reconcile/ramp work. Plan 14-02
  changed only `reconciler.go`, `reconcile_record.go`, `reconciler_test.go`,
  `notify.go`, `tracker_broadcast_test.go` (interface conformance), and
  `internal/notify/pricegap_reconcile.go` (interface conformance) — none of
  these touch scanner event emission.
- **Triage owner:** Phase 11/12 systemic flake fix (CLAUDE.md `/sdebug` workflow);
  not a Phase 14 surface.
- **Action:** None this phase. Re-run on retry path until systemic fix lands.
