---
phase: 8
slug: price-gap-tracker-core
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-21
revised: 2026-04-21
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
| 08-01-01 | 01 | 1 | PG-05 | T-08-01 | Config struct default OFF; no write on load | unit | `go test ./internal/config -run TestApplyJSON_PriceGap -count=1` | N | ⬜ |
| 08-01-02 | 01 | 1 | PG-05, PG-OPS-06 | T-08-02 | Defaults safe (PriceGapEnabled=false, empty candidates) | unit | `go test ./internal/config -run TestDefaults_PriceGap -count=1` | N | ⬜ |
| 08-01-03 | 01 | 1 | PG-05 | T-08-01 | PriceGapStore + DelistChecker interfaces compile | unit | `go build ./internal/models/...` | N | ⬜ |
| 08-02-01 | 02 | 2 | PG-04 | T-08-05 | Redis round-trip (save/load/active set) | integration (miniredis) | `go test ./internal/database -run TestPricegapState_SaveLoad -count=1` | N | ⬜ |
| 08-02-02 | 02 | 2 | PG-04 | T-08-06 | History LIST cap (500 entries) + LTrim | integration (miniredis) | `go test ./internal/database -run TestPricegapState_HistoryCap -count=1` | N | ⬜ |
| 08-02-03 | 02 | 2 | PG-RISK-03 | T-08-07 | Candidate disable flag get/set/clear | integration (miniredis) | `go test ./internal/database -run TestPricegapState_DisableFlag -count=1` | N | ⬜ |
| 08-02-04 | 02 | 2 | PG-RISK-03 | T-08-08 | Slippage rolling window append + read last N | integration (miniredis) | `go test ./internal/database -run TestPricegapState_SlippageWindow -count=1` | N | ⬜ |
| 08-03-01 | 03 | 3 | PG-01 | T-08-12 | Tracker skeleton compiles w/o cross-module import | unit (grep+build) | `go build ./internal/pricegaptrader/... && ! grep -rE "internal/engine\|internal/spotengine" internal/pricegaptrader/` | N | ⬜ |
| 08-03-02 | 03 | 3 | PG-01 | T-08-09, T-08-11 | Detector computes spread bps + 4-bar same-sign persistence | unit | `go test ./internal/pricegaptrader -run 'TestBarRing_\|TestDetectOnce_' -race -count=1` | N | ⬜ |
| 08-03-03 | 03 | 3 | PG-01 | T-08-10 | stubExchange satisfies full Exchange interface | unit (compile) | `go test ./internal/pricegaptrader -run TestFakes -count=1` | N | ⬜ |
| 08-03-04 | 03 | 3 | PG-01 | T-08-09, T-08-11 | 8 detector scenarios (empty, below-T, mixed-sign, pass, stale, insufficient, fires) | unit | `go test ./internal/pricegaptrader -run 'TestBarRing\|TestDetectOnce' -race -count=1` | N | ⬜ |
| 08-04-01 | 04 | 4 | PG-RISK-01..05 | T-08-13 | 9 typed gate-denial error sentinels | unit (compile+grep) | `go build ./internal/pricegaptrader/... && grep -c "ErrPriceGap" internal/pricegaptrader/errors.go` | N | ⬜ |
| 08-04-02 | 04 | 4 | PG-RISK-01, -02, -04, -05 | T-08-13, T-08-14, T-08-16 | 5 gates composed in fixed D-17 order via injected DelistChecker | unit | `go build ./internal/pricegaptrader/...` | N | ⬜ |
| 08-04-03 | 04 | 4 | PG-RISK-01, -02, -04, -05 | T-08-14, T-08-15 | 14 gate tests incl boundary + ordering invariant | unit | `go test ./internal/pricegaptrader -run TestRiskGate_ -race -count=1` | N | ⬜ |
| 08-05-01 | 05 | 5 | PG-02 | T-08-17 | Simultaneous IOC happy path (both legs fill) | unit (stubExchange) | `go test ./internal/pricegaptrader -run TestExecute_HappyPath -race -count=1` | N | ⬜ |
| 08-05-02 | 05 | 5 | PG-02 | T-08-18 | Unwind-to-match on asymmetric fill | unit | `go test ./internal/pricegaptrader -run TestExecute_UnwindToMatch -race -count=1` | N | ⬜ |
| 08-05-03 | 05 | 5 | PG-02 | T-08-19 | Zero-fill one leg → market-close the other | unit | `go test ./internal/pricegaptrader -run TestExecute_ZeroFill -race -count=1` | N | ⬜ |
| 08-05-04 | 05 | 5 | PG-02 | T-08-20 | Circuit-breaker after 5 consecutive PlaceOrder failures | unit | `go test ./internal/pricegaptrader -run TestExecute_CircuitBreaker -race -count=1` | N | ⬜ |
| 08-06-01 | 06 | 6 | PG-03 | T-08-21 | Monitor triggers exit on `|spread| ≤ T/2` | unit (fakeClock) | `go test ./internal/pricegaptrader -run TestMonitor_ExitOnReversion -race -count=1` | N | ⬜ |
| 08-06-02 | 06 | 6 | PG-03 | T-08-22 | Monitor triggers exit on max-hold timer (4h) | unit (fakeClock) | `go test ./internal/pricegaptrader -run TestMonitor_ExitOnMaxHold -race -count=1` | N | ⬜ |
| 08-06-03 | 06 | 6 | PG-03 | T-08-23 | Exit path retries partial remainder as market | unit | `go test ./internal/pricegaptrader -run TestMonitor_PartialRetry -race -count=1` | N | ⬜ |
| 08-06-04 | 06 | 6 | PG-RISK-03 | T-08-24 | Slippage rolling 10-trade 2× auto-disable | unit (miniredis) | `go test ./internal/pricegaptrader -run TestSlippage_AutoDisable -race -count=1` | N | ⬜ |
| 08-07-01 | 07 | 7 | PG-04, PG-OPS-06 | T-08-25 | Rehydration from Redis rebuilds monitor registry | integration (miniredis+stubExchange) | `go test ./internal/pricegaptrader -run TestRehydrate_ActiveSet -race -count=1` | N | ⬜ |
| 08-07-02 | 07 | 7 | PG-04 | T-08-26 | Orphan detection (zero on-exchange size → mark closed) | integration | `go test ./internal/pricegaptrader -run TestRehydrate_OrphanCleanup -race -count=1` | N | ⬜ |
| 08-07-03 | 07 | 7 | PG-OPS-06 | T-08-27 | main.go conditional wire (PriceGapEnabled=false → no goroutines) | unit | `go test ./cmd -run TestMainWire_PriceGapDisabled -count=1` | N | ⬜ |
| 08-07-04 | 07 | 7 | PG-OPS-06 | T-08-28 | Graceful shutdown closes tracker before spot engine | unit | `go test ./cmd -run TestMainWire_ShutdownOrder -count=1` | N | ⬜ |
| 08-08-01 | 08 | 8 | PG-RISK-03 | T-08-31 | pg-admin binary builds + 4 subcommands dispatched | unit (build) | `go build ./cmd/pg-admin/... && go vet ./cmd/pg-admin/...` | N | ⬜ |
| 08-08-02 | 08 | 8 | PG-RISK-03 | T-08-32 | pg-admin enable/disable/positions list round-trip | unit (miniredis) | `go test ./cmd/pg-admin/... -race -count=1` | N | ⬜ |
| 08-08-03 | 08 | 8 | PG-OPS-06 | — | VERSION + CHANGELOG bumped per repo rule | manual | `test -s VERSION && grep -q "Phase 8" CHANGELOG.md && grep -q "pg-admin" CHANGELOG.md` | N | ⬜ |

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
