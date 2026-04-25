---
phase: 8
slug: price-gap-tracker-core
status: audited
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-21
revised: 2026-04-25
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + miniredis v2.37.0 for Redis fakes |
| **Config file** | none — existing `go.mod` covers deps |
| **Quick run command** | `go test ./internal/pricegaptrader/... -run TestPriceGap -short` |
| **Full suite command** | `go test ./internal/pricegaptrader/... ./internal/database/... ./internal/config/... -race` |
| **Estimated runtime** | ~45 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green + `go build ./...` must pass
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

Wave 0 (Plans 01–02) ships the interface, model, and Redis persistence tests that later plans assert against. Plans 03–08 add production code covered by unit + miniredis-backed integration tests. Manual-only behaviors are called out in the separate table below.

| Task ID | Plan | Wave | REQ ID | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|--------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | PG-05 | T-08-01 | Config struct default OFF; no write on load | unit | `go test ./internal/config -run TestApplyJSON_PriceGap -count=1` | Y | ✅ |
| 08-01-02 | 01 | 1 | PG-05, PG-OPS-06 | T-08-02 | Defaults safe (PriceGapEnabled=false, empty candidates) | unit | `go test ./internal/config -run TestDefaults_PriceGap -count=1` | Y | ✅ |
| 08-01-03 | 01 | 1 | PG-05 | T-08-01 | PriceGapStore + DelistChecker interfaces compile | unit | `go build ./internal/models/...` | Y | ✅ |
| 08-02-01 | 02 | 2 | PG-04 | T-08-05 | Redis round-trip (save/load/active set) | integration (miniredis) | `go test ./internal/database -run 'TestPriceGapState_PositionRoundTrip\|TestPriceGapState_ActiveSetTransitions' -count=1` | Y | ✅ |
| 08-02-02 | 02 | 2 | PG-04 | T-08-06 | History LIST cap (500 entries) + LTrim | integration (miniredis) | `go test ./internal/database -run TestPriceGapState_HistoryCap -count=1` | Y | ✅ |
| 08-02-03 | 02 | 2 | PG-RISK-03 | T-08-07 | Candidate disable flag get/set/clear | integration (miniredis) | `go test ./internal/database -run TestPriceGapState_DisableFlag -count=1` | Y | ✅ |
| 08-02-04 | 02 | 2 | PG-RISK-03 | T-08-08 | Slippage rolling window append + read last N | integration (miniredis) | `go test ./internal/database -run TestPriceGapState_SlippageWindow -count=1` | Y | ✅ |
| 08-03-01 | 03 | 3 | PG-01 | T-08-12 | Tracker skeleton compiles w/o cross-module import | unit (grep+build) | `go build ./internal/pricegaptrader/... && ! grep -rE "internal/engine\|internal/spotengine" internal/pricegaptrader/*.go` | Y | ✅ |
| 08-03-02 | 03 | 3 | PG-01 | T-08-09, T-08-11 | Detector computes spread bps + 4-bar same-sign persistence | unit | `go test ./internal/pricegaptrader -run 'TestBarRing_\|TestDetectOnce_' -race -count=1` | Y | ✅ |
| 08-03-03 | 03 | 3 | PG-01 | T-08-10 | stubExchange satisfies full Exchange interface (compiles via fakes_test.go shared by all suite) | unit (compile) | `go test ./internal/pricegaptrader -count=1` | Y | ✅ |
| 08-03-04 | 03 | 3 | PG-01 | T-08-09, T-08-11 | 8 detector scenarios (empty, below-T, mixed-sign, pass, stale, insufficient, fires, dedup) | unit | `go test ./internal/pricegaptrader -run 'TestBarRing\|TestDetectOnce' -race -count=1` | Y | ✅ |
| 08-04-01 | 04 | 4 | PG-RISK-01..05 | T-08-13 | 9 typed gate-denial error sentinels | unit (compile+grep) | `go build ./internal/pricegaptrader/... && grep -c "ErrPriceGap" internal/pricegaptrader/errors.go` | Y | ✅ |
| 08-04-02 | 04 | 4 | PG-RISK-01, -02, -04, -05 | T-08-13, T-08-14, T-08-16 | 5 gates composed in fixed D-17 order via injected DelistChecker | unit | `go test ./internal/pricegaptrader -run TestRiskGate_OrderingInvariant -race -count=1` | Y | ✅ |
| 08-04-03 | 04 | 4 | PG-RISK-01, -02, -04, -05 | T-08-14, T-08-15 | 14 gate tests incl boundary + ordering invariant | unit | `go test ./internal/pricegaptrader -run TestRiskGate_ -race -count=1` | Y | ✅ |
| 08-05-01 | 05 | 5 | PG-02 | T-08-17 | Simultaneous IOC happy path (both legs fill) | unit (stubExchange) | `go test ./internal/pricegaptrader -run TestExecution_HappyPath -race -count=1` | Y | ✅ |
| 08-05-02 | 05 | 5 | PG-02 | T-08-18 | Unwind-to-match on asymmetric fill | unit | `go test ./internal/pricegaptrader -run TestExecution_PartialFill_UnwindMatches -race -count=1` | Y | ✅ |
| 08-05-03 | 05 | 5 | PG-02 | T-08-19 | Zero-fill one leg → market-close the other | unit | `go test ./internal/pricegaptrader -run TestExecution_ZeroFillOneLeg_MarketClosesOther -race -count=1` | Y | ✅ |
| 08-05-04 | 05 | 5 | PG-02 | T-08-20 | Circuit-breaker after 5 consecutive PlaceOrder failures | unit | `go test ./internal/pricegaptrader -run TestExecution_CircuitBreakerOpensAfterFiveFailures -race -count=1` | Y | ✅ |
| 08-06-01 | 06 | 6 | PG-03 | T-08-21 | Monitor triggers exit on `\|spread\| ≤ T/2` | unit (fakeClock) | `go test ./internal/pricegaptrader -run TestMonitor_Exit_ReversionTrigger -race -count=1` | Y | ✅ |
| 08-06-02 | 06 | 6 | PG-03 | T-08-22 | Monitor triggers exit on max-hold timer (4h) | unit (fakeClock) | `go test ./internal/pricegaptrader -run TestMonitor_Exit_MaxHoldTrigger -race -count=1` | Y | ✅ |
| 08-06-03 | 06 | 6 | PG-03 | T-08-23 | Exit path retries partial remainder as market | unit | `go test ./internal/pricegaptrader -run TestMonitor_ExitRetriesMarketOnPartialClose -race -count=1` | Y | ✅ |
| 08-06-04 | 06 | 6 | PG-RISK-03 | T-08-24 | Slippage rolling 10-trade 2× auto-disable | unit (miniredis) | `go test ./internal/pricegaptrader -run TestSlippage_ -race -count=1` | Y | ✅ |
| 08-07-01 | 07 | 7 | PG-04, PG-OPS-06 | T-08-25 | Rehydration from Redis rebuilds monitor registry | integration (miniredis+stubExchange) | `go test ./internal/pricegaptrader -run TestRehydrate_ReEnrollsActive -race -count=1` | Y | ✅ |
| 08-07-02 | 07 | 7 | PG-04 | T-08-26 | Orphan detection (zero on-exchange size → mark closed) | integration | `go test ./internal/pricegaptrader -run TestRehydrate_OrphansZeroSizeLeg -race -count=1` | Y | ✅ |
| 08-07-03 | 07 | 7 | PG-OPS-06 | T-08-27 | main.go conditional wire (PriceGapEnabled=false → no goroutines); also covered behaviorally by `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated` in pricegaptrader pkg | unit (static + behavioral) | `go test ./cmd -run TestMainWire_PriceGapDisabled -count=1 && go test ./internal/pricegaptrader -run TestPriceGapEnabled_DefaultOff -count=1` | Y | ✅ |
| 08-07-04 | 07 | 7 | PG-OPS-06 | T-08-28 | Graceful shutdown closes tracker before spot engine (D-03 source-order invariant) | unit (static) | `go test ./cmd -run TestMainWire_ShutdownOrder -count=1` | Y | ✅ |
| 08-08-01 | 08 | 8 | PG-RISK-03 | T-08-31 | pg-admin binary builds + 4 subcommands dispatched | unit (build) | `go build ./cmd/pg-admin/... && go vet ./cmd/pg-admin/...` | Y | ✅ |
| 08-08-02 | 08 | 8 | PG-RISK-03 | T-08-32 | pg-admin enable/disable/positions list round-trip | unit (miniredis) | `go test ./cmd/pg-admin/... -race -count=1` | Y | ✅ |
| 08-08-03 | 08 | 8 | PG-OPS-06 | — | VERSION + CHANGELOG bumped per repo rule | automated grep | `test -s VERSION && grep -q "Phase 8" CHANGELOG.md && grep -q "pg-admin" CHANGELOG.md` | Y | ✅ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*File Exists: N = not yet created (Wave 0 scaffold); flips to Y once plan lands.*

---

## Wave 0 Requirements

Wave 0 of this phase is synonymous with Plans 01–02 — the interface + Redis persistence foundation that every later plan's tests depend on. Concretely:

- [x] `internal/models/pricegap_interfaces.go` — `PriceGapStore` + `DelistChecker` interfaces (Plan 01 Task 3)
- [x] `internal/models/pricegap_position.go` + `internal/models/pricegap_candidate.go` — domain types (Plan 01)
- [x] `internal/config/config_test.go` — extended with `TestApplyJSON_PriceGap` + `TestDefaults_PriceGap` (Plan 01 Tasks 1–2)
- [x] `internal/database/pricegap_state.go` + `pricegap_state_test.go` — Redis CRUD + miniredis fixtures (Plan 02)
- [x] `internal/pricegaptrader/fakes_test.go` — shared `stubExchange` + `fakeClock` (Plan 03 Task 3)

Wave 0 is complete: every later-plan `<automated>` command resolves against a test file, interface, or Redis fixture created in Plans 01–03.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SOON Binance↔Gate live round-trip | PG-02, PG-03 | Requires live market conditions + small real notional | Enable `PriceGapEnabled=true` with only SOON in candidate list, `PriceGapBudget=100`, watch logs for entry+exit; confirm delta-neutral and closed fully |
| pg-admin disable round-trip | PG-RISK-03 | Validates Redis flag integration with running tracker | `pg-admin disable SOON` → verify next tick log shows candidate blocked with `exec_quality` reason |
| Graceful shutdown with open position | PG-OPS-06 | Requires live process + SIGTERM | Open one position, SIGTERM, restart, verify rehydration from Redis and monitor loop resumes |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready for execute-phase

---

## Validation Audit 2026-04-25

Re-audit performed by `/gsd-validate-phase` after Phase 8 shipped (v0.34.10) and downstream rolling fixes.

**Live test counts:**
- `go test ./internal/pricegaptrader/ -count=1` → all green (~155 tests, package suite)
- `go test ./internal/database/ -run TestPriceGapState -count=1` → all green
- `go test ./internal/config/ -run TestPriceGap -count=1` → all green
- `go test ./cmd/pg-admin/ -count=1` → 4/4 green
- `go test ./cmd/ -run TestMainWire -count=1` → 2/2 green (NEW)
- Aggregate: **177 tests pass across 4 packages** (pricegaptrader+database+config+pg-admin), plus 2 new in cmd/

**Gaps found:** 2
1. `08-07-04` (D-03 shutdown order, `TestMainWire_ShutdownOrder`) — verification map specified the test but no file existed in `cmd/`.
2. `08-07-03` (`TestMainWire_PriceGapDisabled`) — same: behavioral coverage existed in pricegaptrader pkg via `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated`, but the prescribed `cmd/` static guard was missing.

**Resolved:** 2/2 — added `cmd/shutdown_order_test.go` containing two static source-order assertions:
- `TestMainWire_ShutdownOrder` — fails if `pgTracker.Stop()` is reordered after `spotEng.Stop()` in `cmd/main.go`.
- `TestMainWire_PriceGapDisabled` — fails if `pgTracker.Start()` escapes the `if cfg.PriceGapEnabled` guard or the disabled-path log line is removed.

Static-grep is the natural test type here: a behavioral test would require mocking the entire `main()` bootstrap (exchanges, Redis, allocator, scanner, snapshot writer, API server), which is impractical and duplicates `TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated`. The two new tests pin the structural invariant — any future refactor that reorders shutdown or removes the gate fails CI.

**Escalated:** 0

**Manual-only items (unchanged):** 3 — live SOON round-trip, pg-admin↔tracker integration, graceful shutdown with open position. All require running binary + live exchanges; rationale documented above.

**Files modified by audit:**
- `cmd/shutdown_order_test.go` (NEW)
- `.planning/phases/08-price-gap-tracker-core/08-VALIDATION.md` (this file)

**Compliance:** `nyquist_compliant: true` — every Per-Task row now has either an automated command that resolves to a real green test, or an explicit Manual-Only justification.
